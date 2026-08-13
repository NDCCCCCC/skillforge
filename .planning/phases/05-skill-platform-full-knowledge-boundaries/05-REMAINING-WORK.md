# Phase 5 — Remaining Work Plan (post #9 + OOS-1)

**Created:** 2026-08-13 (handoff for a fresh session)
**Purpose:** Self-contained plan to finish the remaining Phase 5 UAT issues. A cold-start session can resume from this file alone.

---

## 0. Status Snapshot

### ✅ DONE (committed, independently verified) — do NOT redo

| Issue | Commit | What | Verification |
|---|---|---|---|
| **#9 (blocker)** wizard commit not registering | `af1ca9a` | `wizardCommitHandler` now rescans `Loader` + sets `Registered`; FE gates toast on `registered`; `Loader.cached` race-guarded with `sync.RWMutex` | go build/test/`-race`/vet green; vitest 194/194; `TestWizardCommitHandler_RegistersAndRefreshesLoader` |
| **OOS-1 (blocker)** GLM verdict fence-parse | `d62fe98` | New shared `internal/llm.ExtractJSON`; all 4 parse sites re-routed (agent.go, callbacks.go, critique.go, pipeline.go) | go build/test/`-race`/vet green; 25-case table + `agent.Run` regression |
| **#2 (major) right-panel IA** | `7cc252e` `58e1bb5` `277d7f3` `b050967` `9dbbe4d` `0ac41d8` (Plan 05-07) | LEFT-sidebar consolidation: chat-input Skill selector popover + LeftSidebar 4-section rewrite (新建会话 + 会话 + 文件 + 技能) + SkillManagerMenu wizard-only + RightChat minimal badge header + session-scoped useChat + live GET /api/skills data path reading `item.manifest.description` | tsc clean; vitest 209/209; vite build succeeds; 0 backend files changed |

Resolved debug sessions: `.planning/debug/resolved/wizard-commit-registration.md`, `.planning/debug/resolved/glm-verdict-fence-parse.md`.

### ⏳ REMAINING

**None.** All 11/11 Phase-5 UAT tests pass. §1 (eval/judge consolidation) + §2 (#7 hot-reload bridge) + §3 (#4 graph chunk-content) + §4 (#2 right-panel IA) are all DONE.

---

## 1. eval/judge.go — consolidate onto llm.ExtractJSON (OOS-1 follow-up, tiny) ✅ DONE (commit `d1af0f6`, quick task 260813-1i6)

- **Severity:** low (not on user-facing verdict path) — but silently degrades eval metrics if a judge model wraps output in prose+fences.
- **Symptom:** `internal/eval/judge.go:362` carries its OWN `stripJSONDelim` copy (TrimPrefix/Suffix based) with the same prose-wrapped blind spot that OOS-1 just fixed elsewhere. `eval/faithfulness_test.go:156` pins only 4 basic shapes.
- **Root cause:** confirmed (OOS-1 session doc, out-of-scope note).
- **Artifacts:** `internal/eval/judge.go:135` (call site), `:362` (def); `internal/eval/faithfulness_test.go:147-158` (TestStripJSONDelim).
- **Fix:** replace `stripJSONDelim` usage at judge.go:135 with `llm.ExtractJSON`; delete the local `stripJSONDelim` def; expand `TestStripJSONDelim` (or move it into `internal/llm/extract_test.go`'s table) to cover prose-wrapped + no-newline + CRLF. ~10 LOC + tests.
- **Verification:** `go test ./internal/eval/ ./internal/llm/`; `go test -race ./internal/eval/`; confirm judge path parses a prose-wrapped fixture.
- **GSD entry:** `/gsd-quick eval-judge-extractjson-consolidation`
- **Size:** ~15 min.

## 2. UAT #7 (major) — Hot-reload toast bridge severed ✅ DONE (commits 3d77870 + 3265178 BE, 09ae711 FE; quick task 260813-1y0)

- **Severity:** major. Completes the Skill hot-reload subsystem together with #9 (which fixed NEW-skill registration; this is the EXISTING-skill reload notification bridge).
- **Symptom:** Touch `skills/<name>/SKILL.md` on disk → no ReloadToast. Browser console: `404 [38×]` (EventSource hammering `/api/skills/events`). Broken-YAML failure toast also unreachable.
- **Root cause (confirmed in 05-UAT.md):** signal path severed end-to-end.
  1. SSE: `GET /api/skills/events` is NOT registered (`server.go:191-196` lists only chat/skills/wizard/graph) → 404 reconnect storm.
  2. Polling fallback (`useSkillReload.ts:120`) diffs `item.version`, but `GET /api/skills` returns `[]*skill.Manifest` (`manifest.go:57-73` = 7 schema fields, NO version). `Version int64` lives only on `registry.Entry` (`registry.go:58`), bumped on reload (`registry.go:333`) but NEVER serialized to HTTP.
  3. Failure path: poller always emits `status:'ok'` (`useSkillReload.ts:125`).
  - NOTE: backend reload machinery itself works (fsnotify → registry Version bump → router cache invalidation) — but recall from #9 that `cmd/server/main.go` does NOT construct `Deps.Registry`, so the watcher is fictional in production. **This must be reconciled:** either wire `Deps.Registry` + `Watch(ctx)` in main.go (then SSE events have a source), or implement reload via the `Loader.Scan()` path that #9 made live.
- **Artifacts:** `internal/server/server.go:191-196` (routes), `internal/server/skills.go` (`listSkillsHandler`), `internal/skill/manifest.go:57-73`, `internal/skill/registry.go:58,333`, `cmd/server/main.go:221-231` (Deps — Registry not set), `web/src/hooks/useSkillReload.ts:120,125`.
- **Proposed fix (two viable designs — DECIDE before coding):**
  - **(A) SSE primary:** register `GET /api/skills/events` (Hertz `pkg/protocol/sse`); wire `Deps.Registry`+`Watch` in main.go; emit `skill-reloaded`/`skill-failed` events on fsnotify. Most faithful to the original design.
  - **(B) Polling fallback only (simpler, matches #9's approach):** serialize `registry.Entry.Version` into the `GET /api/skills` response (wrap `{manifest, version, status}`) so `useSkillReload`'s diff detects increments; have the server's fsnotify→`Loader.Scan()` path bump a version counter. Drop the dead SSE EventSource.
  - Either way: fix the failure-path toast (emit `status:'failed'` on broken YAML).
- **Verification:** automated `chrome-devtools` + `fs`: touch a valid SKILL.md → ReloadToast within ~1s; append broken YAML → error toast. Unit-test the version-increment + status emission. Add `go test` for the SSE/manifest-version path.
- **GSD entry:** `/gsd-debug hot-reload-toast-bridge` (bug-shaped, root cause known) OR Phase 5 gap-closure.
- **Size:** medium (~1–1.5h). **Decision required:** design A vs B.

## 3. UAT #4 (major) — Knowledge Graph chunk shows "(chunk not found in fixture)" ✅ DONE (commits 8939a74 BE + aec582e FE, quick task 260813-dno)

- **Severity:** major.
- **Symptom:** Click a chunk in the SC-5 Knowledge Graph → ChunkSourceView opens (tier + 当前有效 + 生效日期) but body renders `(chunk not found in fixture)`.
- **Root cause (confirmed):** `web/src/lib/chunk-lookup.ts:48` `NOT_FOUND_TEXT` fallback — content lookup resolves ONLY fixture/verdict citations. Backend exposes NO chunk-content endpoint (routes: chat/skills/wizard/graph). Graph endpoint is metadata-only per T-5-22 → graph-browsed chunks have no client-side content.
- **Artifacts:** `web/src/lib/chunk-lookup.ts`, `internal/server/server.go` (routes), `internal/server/knowledge.go` (graph handler), `internal/knowledge` (chunk store).
- **DECISION required first:** SC-5 browse contract — is full-text view from the graph in-scope (needs a content endpoint) or metadata-only (then chunk click must NOT promise a verbatim view)? UAT gap explicitly flags this.
  - If in-scope: add `GET /api/knowledge/chunk/:id` returning the chunk's verbatim text + metadata; teach `chunk-lookup.ts` to fetch from it for graph-browsed (non-cited) chunks. Reuse the existing knowledge store (the graph already enumerates chunk IDs).
  - If out-of-scope: make graph chunk click metadata-only (no ChunkSourceView body promise) — smaller change, but weakens SC-5.
- **Verification:** click graph chunk → verbatim text renders; unit-test the new endpoint + lookup path.
- **GSD entry:** Phase 5 gap-closure (or `/gsd-quick` if metadata-only decision).
- **Size:** medium (~1h) if content endpoint; small if metadata-only. **Decision required:** in-scope vs metadata-only.

## 4. UAT #2 (major) — Right-panel / Skill Manager information architecture ✅ DONE (Plan 05-07, 2026-08-13)

- **Severity:** major. **Largest + most design-dependent.**
- **Symptom:** Skill selector is a top-right dropdown; user wants it in the chat-input area (near the 未选择 Skill hint). Session list should be a drawer that toggles with the chat view. The session/skill dropdown should become a 新建会话 button. ALSO: `SkillManagerMenu.tsx` reads from a STATIC fixture `__fixtures__/skills.json`, NOT `/api/skills` (so newly-installed skills — including #9's wizard output — never appear in the menu).
- **Root cause (confirmed):** IA diverges from the sketch source-of-truth (sessions + file tree + skill list belong in the LEFT sidebar; right panel is "chat with Skill"). Plus the SkillManagerMenu static-fixture bug.
- **Resolution:** Plan 05-07 implements the locked LEFT-sidebar consolidation per sketch-findings layout-three-panel rule #7.
  - **D5-IA-01** ChatInput hosts the Skill selector popover reading `store.skills` (which carries `item.manifest.description` per the BLOCKING CONSTRAINT — `useSkillReload` dispatches `setSkills` on every poll). Clicking a row calls `useAppStore.setCurrentSkill(id)`.
  - **D5-IA-02** LeftSidebar has 4 stacked labeled sections: 新建会话 button (calls `createSession()`) + 会话 block + 文件 block + 技能 block.
  - **D5-IA-03** Session list is UI-local Zustand only (no backend `ListSessions` / `/api/sessions` added); each session is an independent `useChat({id})` instance (Vercel AI SDK v5 per-id chatStore); messages retained per-id while mounted, lost on refresh.
  - **D5-IA-04** SkillManagerMenu (topbar) is wizard-only: per-skill list + 全部重载 button + `reloadPath` prop + `__fixtures__/skills.json` import REMOVED. Skill browse + status moved to LeftSidebar 技能 block.
  - **D5-IA-05** Dead POST `/api/skills/reload` 404 route is no longer hit (the 全部重载 button is gone). Polling-only reload (Plan §2 decision B) surfaces via `useSkillReload` + `ReloadToast`.
  - RightChat header is now a minimal read-only badge (`right-chat-badge`); `SessionPicker` + default-mode `SkillPicker` USAGE is removed (the `SkillPicker` COMPONENT file stays — `RoutingTracePart` still imports it for SC-1 ambiguity mode).
- **Artifacts:** `web/src/store/app-store.ts`, `web/src/hooks/useSkillReload.ts`, `web/src/App.tsx`, `web/src/components/chat/ChatInput.tsx`, `web/src/components/layout/SkillManagerMenu.tsx`, `web/src/components/layout/RightChat.tsx`, `web/src/components/layout/LeftSidebar.tsx`, `web/src/__tests__/{useSkillReload,ChatInput,SkillManagerMenu,LeftSidebar}.test.{tsx,x}`.
- **Verification:** tsc --noEmit clean; vitest 209/209 (8 new LeftSidebar + 4 new ChatInput + 1 new useSkillReload manifest-description case + rewritten SkillManagerMenu suite); vite build succeeds; `git diff --name-only b134b95..HEAD` shows 0 files under `internal/` or `cmd/`. See `05-07-SUMMARY.md` for full deviation/acceptance log.
- **Size (actual):** ~30 minutes, 6 commits (3 RED tests + 3 GREEN impls), no REFACTOR commits needed (GREEN iterations covered all correctness concerns).

---

## 3. Recommended Sequencing & Rationale

1. **eval/judge.go consolidation** (§1) — tiny, closes the OOS-1 thread cleanly, high confidence. Do first.
2. **#7 hot-reload bridge** (§2) — completes the Skill hot-reload subsystem alongside #9; root cause known; needs one design pick (SSE vs polling).
3. **#4 graph chunk-content** (§3) — focused; needs one scope pick (content endpoint vs metadata-only).
4. **#2 right-panel IA** (§4) — largest, design-dependent; do last, after the IA decision.

Rationale: knock out the clear-cut, known-root-cause items first (§1→§3), defer the design-heavy IA work (§4) to the end where it can be scoped deliberately. §2 and §4 share the `/api/skills/reload` + `SkillManagerMenu` wiring — coordinate so #7's reload-route decision feeds #2.

**All four items are now DONE (§1 commit `d1af0f6`, §2 commits `3d77870` + `3265178` BE + `09ae711` FE, §3 commits `8939a74` BE + `aec582e` FE, §4 Plan 05-07 commits `7cc252e` `58e1bb5` `277d7f3` `b050967` `9dbbe4d` `0ac41d8`). Phase 5 is feature-complete. Plan §0 REMAINING table is empty.**

**Status (2026-08-13):** §1 + §2 + §3 + §4 ALL DONE. All 11/11 Phase-5 UAT tests pass. The Phase-5 frontend shipped feature-complete (the underlying `_fixtures_/skills.json` static-fixture bug + the `/api/skills/reload` 404 + the right-panel dropdown IA are all retired in lockstep by Plan 05-07's left-sidebar consolidation).

**Decisions — RESOLVED 2026-08-13 (locked, do not re-ask):**
- §2: **polling-only (B)** — serialize `registry.Entry.Version` into the `GET /api/skills` response (wrap `{manifest, version, status}`); reuse #9's live `Loader.Scan()` path to bump the version; delete the dead SSE EventSource; emit `status:failed` on broken YAML.
- §3: **full-text in-scope** — add `GET /api/knowledge/chunk/:id` returning verbatim text + metadata; teach `chunk-lookup.ts` to fetch from it for graph-browsed (non-cited) chunks.
- §4: **LEFT-sidebar consolidation** (per sketch source-of-truth) — reconcile with `05-UI-SPEC.md` BEFORE implementing; wire `SkillManagerMenu` to live `GET /api/skills` (fixes static-fixture bug + the `/api/skills/reload` 404 — coordinate with §2).

---

## 4. Hygiene — update tracking artifacts (do as part of the first new-session commit)

These are currently STALE (the code is fixed but the docs weren't updated to keep the #9/OOS-1 commits atomic):
- `05-UAT.md` — Test 9 `result: issue` → `fixed` (commit `af1ca9a`); OOS-1 `status: open` → `fixed` (commit `d62fe98`); the #9 Gap `status: failed` → `fixed`. (If not already edited in this session.)
- `STATE.md` — Session Continuity note: #9 + OOS-1 resolved (commits `af1ca9a`, `d62fe98`); Phase 5 UAT now 9/11 effectively pass (2 open: #2, #4; #7 hot-reload bridge open). Pre-existing unrelated drift in STATE.md/config.json to reconcile separately.
- As each remaining item is fixed, flip its UAT result + this plan's status.

---

## 5. New-Session Resume Instructions

1. **Read first:** this file (`05-REMAINING-WORK.md`), then `05-UAT.md` (current test results + gaps), then `.planning/STATE.md`.
2. **Resolved context (only if needed):** `.planning/debug/resolved/{wizard-commit-registration,glm-verdict-fence-parse}.md`.
3. **Confirm the baseline is still green before starting:**
   ```bash
   go build ./... && go vet ./... && go test ./...
   cd web && npx tsc --noEmit && npx vitest run
   ```
4. **Pick an item** per §3 ordering. Surface the gating decision (§2/§3/§4) to the user before coding.
5. **Entry points:** `/gsd-quick` (§1), `/gsd-debug` or gap-closure (§2, §3), `/gsd-discuss-phase` → UI impl (§4).
6. **Branch:** `branching_strategy: none` — commits land on `master` per project convention; `commit_docs: true`.
7. **Verification standard** (match the #9/OOS-1 bar): build + vet + full `go test` + `-race` on touched packages; FE `tsc` + `vitest`; independent review of the load-bearing logic, not just the agent's self-report.

---

*Plan authored 2026-08-13 after resolving blockers #9 (af1ca9a) + OOS-1 (d62fe98).*
