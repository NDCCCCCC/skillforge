---
phase: 05-skill-platform-full-knowledge-boundaries
verified: 2026-08-13T15:55:00Z
status: gaps_found
score: 13/15 must-haves verified (8 ROADMAP SCs + 7/7 Plan-05-07 wiring must-haves; CR-01 session-message-retention FAILED; CR-02 setSkills wipe = WARN)
overrides_applied: 0
re_verification:
  previous_status: passed (Aug 12) — OVERRIDDEN by code review Aug 13 (commit 8462409) that surfaced 2 critical findings not present in the original verification
  previous_score: 8/8
  gaps_closed: []
  regressions: []
  driver: Code review (05-REVIEW.md commit 8462409, 11 files reviewed) verified two critical claims against actual @ai-sdk/react@4.0.62 source. CR-01 falsified the "per-id chatStore" assumption that anchored Plan 05-07's session model. CR-02 surfaced a transient-empty API response fragility in tick() that wipes the live skill list.
gaps:
  - truth: "Switching sessions swaps the active useChat instance; each session's messages are independent and in-memory (lost on refresh)"
    status: failed
    reason: "useChat({ id: activeSessionId }) in @ai-sdk/react@4.0.62 allocates a new Chat() every time the id changes — verified directly in node_modules/@ai-sdk/react/dist/index.js:326-328. The plan's claim of 'Vercel AI SDK v5 per-id chatStore semantics' is not accurate for the installed React hook. Switching sessions loses ALL previously-streamed messages (and from any prior in-flight stream), not just on refresh. Test suite (209 vitest, 8 new LeftSidebar tests + 4 new ChatInput tests) does NOT exercise a session switch — no App-level test mounts <App>, sends a message, switches activeSessionId, and asserts message persistence. The user-visible feature 'messages retained across session switches' is broken in production."
    artifacts:
      - path: "web/src/App.tsx"
        issue: "Line 62: `useChat({ id: activeSessionId, transport })` — id changes cause Chat() recreation via shouldRecreateChat branch (lib line 326-328)."
      - path: "web/node_modules/@ai-sdk/react/dist/index.js"
        issue: "Lines 326-328: shouldRecreateChat evaluates true when chatRef.current.id !== options.id; new Chat(chatOptions) is allocated with no messages. No chatStore exists in @ai-sdk/react — searched the package for chatStore, allChats, registry, messageCache, perId — none exist."
      - path: "web/src/__tests__/"
        issue: "No App.test.tsx; no test exercises session switching with message retention assertion. ChatInput.test.tsx + LeftSidebar.test.tsx only check store state changes (createSession, setActiveSession), never that messages persist across an activeSessionId flip."
    missing:
      - "Maintain a `useRef<Map<string, Chat<UIMessage>>>` in App.tsx (or equivalent) so each session retains its Chat instance; pass `chat` to useChat instead of `id` (the chat prop is preserved while id is recreating)."
      - "Or, accept the broken state as MVP and update the must_have to 'messages do NOT persist across session switches; each switch starts a fresh useChat instance'."
      - "Add an integration test that mounts <App>, sends a message in session A, switches to session B via LeftSidebar click, and asserts session A's messages are still present (or that the new behavior is intentional)."
  - truth: "Skill list is stable across transient empty /api/skills responses"
    status: failed
    reason: "useSkillReload.tick() calls useAppStore.getState().setSkills(summaries) UNCONDITIONALLY on every poll. A single transient empty response (server restart, brief backend hiccup, schema-migration window) wipes store.skills — LeftSidebar 技能 block empties, ChatInput selector shows '尚未安装任何 Skill', RightChat badge shows '未选择 Skill', currentSkillId becomes a dangling reference. Plan 05-07's manifest.description test (useSkillReload.test.tsx:113-157) happens to queue the same response every call (mock returns `responses[0]` regardless of callIdx) so the bug is masked in that test. The other two useSkillReload tests (version bump, status flip) use the callIdx queue pattern with `responses[idx] ?? []` — a third empty poll DOES call setSkills([]), but the assertions only inspect skillReloadStatus, not skills, so the wipe is unobserved."
    artifacts:
      - path: "web/src/hooks/useSkillReload.ts"
        issue: "Line 148: `if (!cancelled) useAppStore.getState().setSkills(summaries)` — no guard against transient empty list when current skills list is non-empty."
      - path: "web/src/__tests__/useSkillReload.test.tsx"
        issue: "Lines 138-141: manifest test mocks fetch to return `responses[0]` always (no callIdx counter), so the wipe never fires in this test. Lines 41-47 + 89-95: version/status tests DO exercise the wipe, but don't assert against skills."
    missing:
      - "Guard: skip setSkills dispatch when summaries.length === 0 AND current.length > 0 (treat as transient glitch per the bug fix sketch in 05-REVIEW.md lines 99-103)."
      - "Extend the useSkillReload manifest test to use vi.waitFor against useAppStore.getState().skills rather than wall-clock time, OR queue enough responses to outlast the wait window."
deferred: []
human_verification:
  - test: "Switch between two sessions in the live UI: type and send a message in session A, click 新建会话 in the left sidebar, type and send a message in session B, click back to session A in the 会话 list. Observe whether session A's previously-streamed messages are present."
    expected: "If CR-01 fix is missing (current state), session A's messages are gone — only session B's messages remain. The assistant message bubble and any stream history vanishes when activeSessionId flips."
    why_human: "Cannot verify programmatic message-loss for useChat's internal Chat instance from a static check alone — requires the React runtime. The vitest suite does not mount <App> at the level needed."
  - test: "Watch the LeftSidebar 技能 block and the chat-input Skill selector while the backend restarts (or while /api/skills returns [] for a single tick — easiest: open DevTools → Network, block the GET /api/skills response for one cycle)."
    expected: "Skills vanish from both UI surfaces for one poll cycle, then reappear. currentSkillId remains valid even when the list temporarily empties."
    why_human: "Timing-sensitive behavior not exercised by unit tests; visual UI confirmation required."
---

# Phase 5: Skill Platform, Full Knowledge Boundaries — Verification Report (RE-VERIFICATION)

**Phase Goal:** Promote SkillForge from "one Skill" to a true Skill-driven platform — semantic routing across multiple Skills, hot reload of in-flight Skills, a Skill Wizard that generates a `SKILL.md` from natural language, the full 4-tier knowledge boundary system with conflict surfacing, and an Agent state inspector.

**Verified:** 2026-08-13T15:55:00Z (RE-VERIFICATION, after code review surfaced CR-01 + CR-02)

**Status:** gaps_found

**Mode:** mvp

---

## 1. Verification Scope

This is a **re-verification** triggered by `05-REVIEW.md` (commit 8462409, 2026-08-13T15:30, 11 files standard-depth review). The original verification (2026-08-12) marked Phase 5 PASS at 8/8 ROADMAP SCs. The review traced two critical claims to actual `@ai-sdk/react@4.0.62` source and FE regression tests, finding both to be FALSE in production code despite the green test suite.

The previous VERIFICATION's `passed` verdict was anchored on the assumption that "Vercel AI SDK v5 retains per-id message state in its in-memory chatStore." That assumption is not accurate for the installed React hook (`@ai-sdk/react@4.0.62`), where `shouldRecreateChat` allocates a new `Chat()` on every id change with empty messages.

This re-verification confirms:
- ROADMAP SCs 1-8 are still structurally PASS at the BE level (the platform capability is built).
- Plan 05-07 must_have "Switching sessions swaps the active useChat instance; each session's messages are independent and in-memory (lost on refresh)" is **NOT met** at runtime — directly conflicts with @ai-sdk/react@4.0.62 behavior verified at the source level.
- Plan 05-07 must_have "Skill surfaces read the LIVE GET /api/skills" has a related fragility (CR-02) — the data path works, but a transient empty response wipes the live list.

---

## 2. ROADMAP Success Criteria — Goal-Backward Matrix

| # | Success Criterion (ROADMAP.md:127-133) | Status | Evidence |
|---|---|---|---|
| **SC-1** | With two Skills installed, an ambiguous query routes to the right Skill by description semantic match — trace shows score + selected Skill (SYSK-05) | **VERIFIED** (code-level; bundled sample count observation in §6) | `internal/skill/router.go:37` `AmbiguityLowFloor=0.4`, `:47` `AmbiguityMargin=0.08`; cosine via `vecmath.Cosine`; `internal/server/chat.go:132` guard; routing decision emitted BEFORE VerdictPipeline. Router tests green (`go test ./internal/skill/`). |
| **SC-2** | Editing a SKILL.md on disk triggers fsnotify reload; next query after 1s debounce uses the new behavior (SYSK-04) | **VERIFIED** | `internal/skill/registry.go:42` `ReloadDebounce = 1*time.Second`; `:90` `atomic.Pointer[map[string]*Entry]` lock-free snapshot; `:131` `fsnotify.NewWatcher` directory watch; `:373` `time.AfterFunc` debounce. BE has `internal/server/skills.go` exposing registry-backed GET /api/skills with `version` field for diff-based polling (POST /api/skills/reload 404 retired per 05-07). `TestRegistry_Reload_Debounce` passes; `TestRegistry_Reload_ValidationFailure_KeepsOld` passes. (Symlink test skipped on Windows — code compiled in.) |
| **SC-3** | Skill Wizard takes NL input → emits validated `SKILL.md` passing Phase 2 validator (SYSK-06) | **VERIFIED** | BE: `internal/server/skills.go:230` `wizardHandler` (preview) + `:230+` `wizardCommitHandler` (atomic write + Loader rescan — fix af1ca9a closed UAT #9 registration loop); `internal/skill/wizard.go:114` `Wizard.Generate` (LLM + structured output + retry); `server.go:202-203` route registration; V12 path-traversal guard. FE: 5-step interview via `useWizard.ts`. UAT #8/9/10 all pass live. |
| **SC-4** | Live Agent State Inspector shows current turn's tool calls / retrievals / reasoning (AGNT-06) | **VERIFIED** | `internal/server/chat.go:178-230` emits `data-decision` chunks BEFORE VerdictPipeline. FE: `web/src/components/inspector/AgentInspector.tsx` builds an Accordion from `useChat.messages` parts (D14: messages live in useChat, store stays metadata-only). `AgentInspector.test.tsx` 8/8 pass; UAT #6 confirms live via direct POST /api/chat (MockChatTransport dev limitation acknowledged). |
| **SC-5** | Knowledge graph browser shows tier → doc → chunk; clicking a chunk reveals its citations (KNOW-06) | **VERIFIED** (after 2026-08-13 fix) | Graph endpoint live: `internal/server/knowledge.go:75` `graphHandler`. FE: `web/src/components/graph/KnowledgeGraphBrowser.tsx` 3-level Accordion + filter. **Chunk-content gap closed by UAT #4 fix (commits 8939a74 BE + aec582e FE)**: GET `/api/knowledge/chunk/:id` returns verbatim text; `chunk-lookup.ts` async-fetches on fixture-miss; `ChunkSourceView` renders body. `KnowledgeGraphBrowser.test.tsx` green. |
| **SC-6** | Policy conflict (权威 vs 历史) surfaces counter-evidence as annotation, NOT grounding (CMPL-06, D5-13) | **VERIFIED** — structural | `internal/server/annotate.go:71` signature `emitCounterEvidence(ctx, c, qvec, now)` — **no chunksByID parameter**. Compiler rejects any leak. `:76` queries `["历史"]` only. `pipeline.go:267-269` invokes AFTER `emitVerdictStream`. FE `CounterEvidenceCard.test.tsx` 9/9 pass; `MessageBubble.tsx:33-77` dispatcher routes `decision="counter_evidence"` to `CounterEvidenceCard`. The moat IS the signature. |
| **SC-7** | Conditional redline rewrites on 不合规/需补充 only (CMPL-07, D5-14) | **VERIFIED** — structural | `internal/server/redline.go:108-110`: first executable line is the gate `if v.Status != verdict.NonCompliantStatus && v.Status != verdict.DowngradeStatus { return nil }` — BEFORE any LLM call, BEFORE any wire emit. `TestRedline_Conditional` covers `compliant_skips_LLM`, `non_compliant_fires_LLM`, `needs_info_fires_LLM`. FE `RedlineCard.test.tsx` 12/12 pass; `MessageBubble.tsx:87-96` dispatcher. |
| **SC-8** | Querying chunks whose `effective_to < today` yields zero (CMPL-08) | **VERIFIED** | `pipeline.go:155` `now := time.Now().Unix()` computed ONCE at pipeline entry (T-5-10 / D5-15); `:156` passed to `Retriever.Search(qvec, tiers, 5, now)`; backend applies `effective_to < today` as PRE-filter (Milvus + Mongo). Zero results for expired rows, NEVER a post-filter fallback. |

**Score: 8/8 ROADMAP Success Criteria VERIFIED at the structural level.** The platform capabilities (routing, hot reload, wizard, inspector, graph, counter-evidence, redline, version filtering) are all present in code with confirmed wiring.

---

## 3. Plan 05-07 Must-Haves (D5-IA-01/03/04/06 — UAT #2 Gap Closure)

Loaded from `05-07-PLAN.md` frontmatter:

| # | Must-Have Truth | Status | Evidence |
|---|---|---|---|
| 05-07-T1 | Skill selector lives in chat-input area (NOT a separate top-right dropdown) | **VERIFIED** | `web/src/components/chat/ChatInput.tsx` `chat-input-current-skill` is the clickable trigger; popover lists `store.skills`; click sets `currentSkillId`. Old fixture-based lookup REMOVED. |
| 05-07-T2 | Selecting a Skill sets `useAppStore.currentSkillId` | **VERIFIED** | `ChatInput.test.tsx:77-90` asserts `useAppStore.getState().currentSkillId === 's1'` after click. Green. |
| 05-07-T3 | Session list always visible in left sidebar 会话 block (NOT a drawer, NOT in chat header) | **VERIFIED** | `LeftSidebar.tsx` 4-section: 新建会话 button + 会话 block + 文件 block + 技能 block. `RightChat.tsx` SessionPicker usage REMOVED from header (now in `LeftSidebar.tsx` as a `<ul>`). `SkillManagerMenu.test.tsx` rewritten; `LeftSidebar.test.tsx` 8/8 pass. |
| 05-07-T4 | Clicking 新建会话 at the top of the left sidebar creates a new session and makes it active | **VERIFIED** | `LeftSidebar.test.tsx:51-60` asserts `sessions.length` increases by 1, `activeSessionId` changes to the new id. Green. `app-store.ts:216-224` `createSession()` does append + setActive. |
| 05-07-T5 | **Switching sessions swaps the active useChat instance; each session's messages are independent and in-memory (lost on refresh)** | **FAILED** | See §4 — CR-01. `useChat({ id: activeSessionId })` in `@ai-sdk/react@4.0.62` recreates `Chat` on every id change (verified at `node_modules/@ai-sdk/react/dist/index.js:326-328`). Messages lost on EACH switch, not only on refresh. No test exercises this behavior. |
| 05-07-T6 | Topbar Skill 管理 menu shows ONLY 新建 Skill（向导） entry | **VERIFIED** | `SkillManagerMenu.tsx` is now wizard-only: per-skill `<ul>` + 全部重载 button + reloadPath + `__fixtures__/skills.json` import REMOVED. Test `SkillManagerMenu.test.tsx:6 cases` asserts `skill-manager-reload-all` is null and `skill-manager-item` rows are absent. |
| 05-07-T7 | All Skill surfaces read LIVE `GET /api/skills` (item.manifest.*), NOT `__fixtures__/skills.json` | **VERIFIED** — with WARN (CR-02) | `useSkillReload.ts:142` normalizes `item.manifest?.description` (per BLOCKING CONSTRAINT); `setSkills` dispatched every tick. `SkillManagerMenu.tsx` + `ChatInput.tsx` + `LeftSidebar.tsx` 技能 block all read `store.skills`. Fixture imports = 0. WARN: `setSkills` is unconditional on transient empty responses (CR-02). |
| 05-07-T8 | No HTTP request to `/api/skills/reload` (dead 404 route retired, 全部重载 removed) | **VERIFIED** | `grep "api/skills/reload" web/src/components/layout/SkillManagerMenu.tsx` returns 0 hits. `SkillManagerMenu.test.tsx` asserts `queryByTestId('skill-manager-reload-all')` is null. |
| 05-07-T9 | RightChat header is read-only badge (SessionPicker + default SkillPicker removed) | **VERIFIED** | `RightChat.tsx` header is `data-testid="right-chat-badge"` with active session name + current skill name. `SkillPicker` COMPONENT file retained (RoutingTracePart ambiguity mode) but USAGE in header is gone. |
| 05-07-T10 | LeftSidebar 4-stacked sections (新建会话 / 会话 / 文件 / 技能) | **VERIFIED** | `LeftSidebar.test.tsx:8/8` covers all 4 sections including 新建会话 button + 4-tier SOURCES 权威/历史/个人/草稿 + 已加载/加载失败（旧版可用） status badges. |

**Score: 9/10 must-haves VERIFIED.** One FAILED (T5 session messages).

---

## 4. Critical Findings (from 05-REVIEW.md commit 8462409)

### 4.1 CR-01 — Session-scoped useChat loses messages on every switch

**Source-level evidence (this re-verification independently confirms the reviewer's finding):**

```js
// web/node_modules/@ai-sdk/react/dist/index.js:314-330
const chatOptions = {
  ...options,
  transport: { /* ... wrapped to use latestRef ... */ },
  // ... onToolCall, onData, onFinish, onError wrappers ...
};
const chatRef = useRef2(
  "chat" in options ? options.chat : new Chat(chatOptions)
);
const shouldRecreateChat =
  "chat" in options && options.chat !== chatRef.current ||
  "id" in options && options.id != null && chatRef.current.id !== options.id;
if (shouldRecreateChat) {
  chatRef.current = "chat" in options ? options.chat : new Chat(chatOptions);
}
```

`web/src/App.tsx:62` passes `useChat({ id: activeSessionId, transport })`. When `activeSessionId` changes:
1. `shouldRecreateChat` evaluates to `true` (because `chatRef.current.id !== options.id`).
2. `chatRef.current = new Chat(chatOptions)` is allocated.
3. `chatOptions` does NOT include `messages` — the constructor `Chat({ ...init })` (line 259) takes `messages` separately, but the `chatOptions` spread only puts in transport/callbacks. The new Chat has empty messages.
4. All previously-streamed messages in the previous session become unreachable.

There is **no** chatStore, allChats, registry, messageCache, or perId mechanism in `@ai-sdk/react` (verified via grep across the package: `chatStore|allChats|registry|messageCache|perId` returns 0 hits in `node_modules/@ai-sdk/react/src/` and `dist/`). The "Vercel AI SDK v5 retains per-id message state" claim in `web/src/App.tsx:8-11` and `:56-60` is FACTUALLY WRONG for the installed React hook.

**Note:** "AI SDK v5" in PROJECT.md / Phase 4 communications refers to the UIMessageChunk **wire format** (product-code-name), NOT the npm package version. The installed React SDK is `@ai-sdk/react@4.0.62` (verified via `node_modules/@ai-sdk/react/package.json`). The original verification read "AI SDK v5" as the npm version and assumed a chatStore semantics that does not exist in v4.

**Test coverage gap:** No `App.test.tsx` exists. `LeftSidebar.test.tsx` + `ChatInput.test.tsx` only exercise store mutations — `createSession`, `setActiveSession`, `setCurrentSkill`. None of them mount `<App>` and assert that messages persist across an `activeSessionId` flip. The bug is unverified by tests because no test touches the runtime behavior.

**User-visible impact:** Type messages in session A → click 新建会话 (creates session B + flips activeSessionId) → click the session A row in the LeftSidebar to switch back → session A's messages are gone. The user sees an empty chat bubble where they had conversation a moment ago. This directly violates the must_have "each session's messages are independent and in-memory (lost on refresh **only**)".

**Fix sketch (per 05-REVIEW.md lines 64-77):** Maintain `useRef<Map<string, Chat<UIMessage>>>` keyed by session id; pass the existing Chat instance via `chat` prop instead of `id`. The `chat` prop is preserved while `id` triggers recreation. Add an integration test that actually exercises the switch.

### 4.2 CR-02 — useSkillReload.tick() wipes the live skill list on transient empty response

**File:** `web/src/hooks/useSkillReload.ts:147-149`

```ts
if (!cancelled) {
  useAppStore.getState().setSkills(summaries);
}
```

When `summaries.length === 0` (transient response), this dispatch replaces the live `store.skills` with `[]` even when the prior list had entries. Visible symptoms:
- LeftSidebar 技能 block (line 144+) renders the empty branch "尚未安装任何 Skill".
- ChatInput selector (`chat-input-current-skill`) shows the muted empty row (line 92-97 ChatInput.test.tsx).
- RightChat badge shows "未选择 Skill" (`RightChat.tsx` `right-chat-badge` fallback).
- `useAppStore.currentSkillId` is silently stale — it points to an id that is no longer in `store.skills`.

**Test mask:** `web/src/__tests__/useSkillReload.test.tsx:138-141` manifest test mocks fetch to return `responses[0]` always (no `callIdx` counter), so the wipe never fires in this test. Other two tests (`TestRegistry_Reload_Debounce` analog lines 33-67 + `ok→failed` lines 69-111) DO use the `responses[idx] ?? []` pattern that returns `[]` for index-out-of-range — but the assertions only inspect `skillReloadStatus`, not `skills`. The wipe is unobserved.

**Fix sketch (per 05-REVIEW.md lines 92-103):** Skip `setSkills` dispatch when `summaries.length === 0 && current.length > 0`. Extend the manifest test to either queue enough responses OR use `vi.waitFor()` against the actual store state.

---

## 5. Phase 5 Requirement ID Traceability

Per `REQUIREMENTS.md` lines 153-237 (Phase 5 column), 8 requirement IDs map to this phase:

| Requirement | Title | ROADMAP SC | Status |
|---|---|---|---|
| **SYSK-04** | Hot-reload Skills | SC-2 | Satisfied |
| **SYSK-05** | Multi-Skill routing | SC-1 | Satisfied (capability shipped; only 1 sample bundled — see §6) |
| **SYSK-06** | Skill Wizard | SC-3 | Satisfied (UAT #8/9/10 PASS live) |
| **AGNT-06** | Agent state inspector | SC-4 | Satisfied |
| **KNOW-06** | Knowledge graph browser | SC-5 | Satisfied (post 8939a74/aec582e fix) |
| **CMPL-06** | Counter-evidence ≠ grounding | SC-6 | Satisfied (structural signature moat) |
| **CMPL-07** | Conditional redline | SC-7 | Satisfied (first-line gate structural) |
| **CMPL-08** | Effective-to filtering | SC-8 | Satisfied (PRE-filter structural) |

All 8 Phase 5 requirement IDs accounted for. No orphans.

---

## 6. Observations (non-blocking, for the record)

### 6.1 Bundled sample Skill count (SC-1 wording)

**Observation:** `skills/` ships with exactly one sample (`skills/compliance/SKILL.md`). SC-1's strict text references "two Skills installed (compliance + general QA)". Reuse model: a second sample can be added via the Skill Wizard (SC-3) in <5 minutes without code changes.

**Assessment:** Non-blocking. Routing capability is structurally verified across N Skills; the demo corpus happens to be 1 sample.

### 6.2 SC-5 "reveals its citations" wording

**Observation:** ROADMAP SC-5 says "clicking a chunk reveals its citations." `ChunkSourceView` renders the chunk's verbatim text + TierBadge + VersionBadge + 生效日期 + 失效日期 — i.e., what is citable ABOUT the chunk. It does NOT show reverse citations (which verdicts cited this chunk).

**Assessment:** Non-blocking. `REQUIREMENTS.md:37` KNOW-06 = "User can browse the knowledge graph (tier + doc + chunk relationships) in the UI" — met. UI-SPEC §SC-5 explicitly narrows to "navigate to ChunkSourceView." If the original wording intended reverse-citation tracking, that is a separate data path not in Phase 5 scope.

### 6.3 MockChatTransport dev limitation (SC-4 live verification)

**Observation:** `web/src/App.tsx:54` defaults `env: 'dev'` on Vite's PROD flag — Vite dev uses `MockChatTransport`, which has no `data-decision` parts in fixtures. The dev UI cannot demonstrate SC-4 (routing trace → Agent Inspector Accordion) without overriding `env=prod` or reading direct `/api/chat` on :8888.

**Assessment:** Non-blocking. Backend POST `/api/chat` does emit `data-decision` + `data-policy` + `data-redline` + finish-step live (re-confirmed via curl in UAT #6/11). The dev-mode mock is a documented limitation, not a Phase 5 regression.

### 6.4 Pre-existing / unfixed warnings (05-REVIEW.md)

The review surfaced 5 warnings (WR-01 through WR-05) and 4 info findings (IN-01 through IN-04). None are blockers for the Phase 5 goal but are logged for future cleanup:

- WR-01 (LeftSidebar title attribute doesn't sanitize FS paths)
- WR-02 (stale currentSkillId not cleared when selected skill is removed)
- WR-03 (useSkillReload manifest test fragility — overlaps with CR-02)
- WR-04 (title attribute multi-line error rendering)
- WR-05 (test fixture `originalState` snapshots `Date.now()`)
- IN-01 (deleteSession unreachable from any UI)
- IN-02 (LeftSidebar 暂无会话 branch unreachable)
- IN-03 (inconsistent session naming)
- IN-04 (silent error swallowing on non-OK /api/skills)

### 6.5 build / typecheck / tests

| Check | Command | Result |
|---|---|---|
| Go build | `go build ./...` | **exit 0** |
| Go test | `go test ./...` | **all packages green** (registry reload tests pass; counter-evidence + redline tests pass; pipeline tests pass) |
| TS typecheck | `cd web && npx tsc --noEmit` | **exit 0** |
| Frontend vitest | `cd web && npx vitest run` | **31 files / 209 tests PASS** (no coverage for session message retention) |

---

## 7. Gaps Summary

Two critical findings render the Phase 5 goal PARTIALLY achieved at the runtime level, even though every SC and every structural guarantee is met:

1. **CR-01 (BLOCKER, FAILED):** The must_have "Switching sessions swaps the active useChat instance; each session's messages are independent and in-memory (lost on refresh)" is NOT met. The user-visible session-switching feature is broken: every switch wipes prior messages, not just refresh. This is a real user-facing defect, not a stub or hole — the wiring exists, the store contract exists, the SDK just doesn't preserve messages the way the plan said. Fix requires implementation rework (useRef chat-map + `chat` prop + integration test).

2. **CR-02 (WARNING, FAILED):** The must_have "All Skill surfaces read LIVE GET /api/skills" works on the happy path. But `useSkillReload.tick()` unconditionally dispatches `setSkills(summaries)` — transient empty responses wipe the live list. Fragility rather than user-visible failure in the brief window, but the test suite does not catch it (manifest test happens to return the same response every call, masking the wipe).

**Phase 5 goal status:** The "Skill-driven platform thesis" (semantic routing, hot reload, wizard authoring, 4-tier boundaries, counter-evidence, redline, agent inspector) IS delivered at structural level. The user-facing session-switching feature (Plan 05-07's contribution to UAT #2) is broken at runtime. Without a fix, the platform is functional but session-management is misrepresented to operators (the LeftSidebar UI implies per-session message persistence, the must_have claims it, and the SDK does not deliver it). This gap deserves a follow-up plan.

**Recommended remediation (one plan, code-shaped):**
- Fix CR-01 with `useRef<Map<string, Chat>>` keyed by activeSessionId, pass `chat` prop to useChat. Add `App.test.tsx` integration test that mounts <App>, sends a message, switches sessions, asserts message retention.
- Fix CR-02 with the empty-list guard. Extend the useSkillReload manifest test to use `vi.waitFor` against the actual store state.
- Address the 5 warnings + 4 info findings opportunistically while the relevant files are touched.

---

_Verified: 2026-08-13T15:55:00Z_
_Verifier: Claude (gsd-verifier, re-verification mode)_
_Drivers: 05-REVIEW.md commit 8462409 surfaced 2 critical findings; this verification independently traced both against actual code._
