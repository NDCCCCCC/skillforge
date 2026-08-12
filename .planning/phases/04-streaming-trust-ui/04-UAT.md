---
status: complete
phase: 04-streaming-trust-ui
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md, 04-UI-SPEC.md]
started: 2026-08-11T20:50:00Z
updated: 2026-08-12T03:40:00Z
verification_mode: automated (chrome-devtools MCP) + unit-test evidence
dev_url: http://localhost:5177/ (Vite v5.4.21)
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start — Dev Server Boots
expected: `npm run dev` boots Vite cleanly; app URL renders the SkillForge shell (dark theme, no white-screen / unstyled HTML / console crash). Confirms Wave-2 locked-file reconciliation (theme.css Tailwind tokens + fontsource fonts + `@/*` alias active).
result: pass
evidence: Vite booted (5177); page title `SkillForge — Skill 驱动的企业合规审查`; app shell rendered; no blocking console errors (only a harmless 404, likely favicon). a11y snapshot shows full three-panel structure.

### 2. Three-Panel Layout Renders (D-02)
expected: Three columns — LeftSidebar (SESSIONS/FILES/SKILLS), CenterPanel (empty hero `等待合规审查结果` + body), RightChat (empty hero `开始一次合规审查` + 3 suggested-prompt chips + ChatInput with `发送`). Topbar `SkillForge` + collapse buttons; statusbar `就绪`.
result: pass
evidence: a11y snapshot — topbar `SkillForge` + `Collapse left/right panel`; left = `资料库 · SOURCES` with 4 tiers (权威/历史/个人/草稿) + `会话 1`; center tabs `审查结论/引用原文/Skill 定义` + empty hero; right = `开始一次合规审查` + 3 prompt chips + textbox + `发送` (disabled) ; statusbar `就绪`. (Left labels read `资料库 · SOURCES` rather than literal `SESSIONS/FILES/SKILLS` — sessions/files/skills all present, labeled per sketch sources concept. Minor spec-label drift, not a functional gap.)

### 3. Theme + Fonts Active (denova-dark)
expected: Dark `#0f1419` bg, Inter font, Tailwind v4 `@theme` status tokens compiled.
result: pass
evidence (objective, computed styles): `body.backgroundColor=rgb(15,20,25)`=#0f1419; `body.color=rgb(232,234,237)`=#e8eaed; `body.fontFamily="Inter Variable", Inter, system-ui...` (fontsource loaded); `--color-status-compliant=#22c55e`; `--color-status-out-of-scope=#5b7a99`; `--color-bg=#0f1419`; 3 stylesheets active. Wave-2 locked-file reconciliation confirmed.

### 4. Send Query → Verdict Card (D-06)
expected: VerdictCard with status-colored border, status label + `置信度 {N}%` + confidence bar, body, `{N} 条引用` footer.
result: pass
evidence: Sent `测试问题` → streamed verdict: `合规` + `置信度 93%` + `1 条引用` + answer text + citation chip `[权威] ... c-001`. Two tool calls (tool-search_authority, tool-rerank_bge) rendered with input/output JSON (ToolPart FEUI-01 ✓).

### 5. Reasoning Preview Above Answer (D-08)
expected: Reasoning ABOVE answer, height-capped, `推理` label + `展开全部` toggle.
result: pass
evidence: a11y order — `推理` label + `展开全部` button + reasoning body appear BEFORE tool calls and answer text (D-08 ordering invariant held). `展开全部` toggle present (capped state). (`推理中.../生成中...` streaming suffix not captured — only visible mid-stream; structure correct.)

### 6. Citation Click → CenterPanel Chunk-Source (D-01, FEUI-03)
expected: Click chip → CenterPanel chunk-source: TierBadge + `版本: {v}` + `生效日期: {date}` + verbatim mono `<pre>`.
result: pass
evidence: Clicked citation chip → center content switched to `[权威]` + `版本: v3.2` + `生效日期: 2026-01-15` + verbatim chunk text. ChunkSourceView contract met.

### 7. Refusal Card Distinction (D-07)
expected: RefusalCard (not VerdictCard): out-of-scope 🚫 blue-gray / I-don't-know ❓ gray; no confidence bar / status header.
result: pass
evidence: Not UI-triggerable — MockChatTransport plays a single fixed DEFAULT_FIXTURE regardless of skill/query (verified in transport/index.ts + MockChatTransport.ts), so refusal fixtures can't be reached via normal interaction. Component correctness pinned by `RefusalCard.test.tsx` (8 assertions: out-of-scope vs I-don't-know, suggestion chip, NEVER renders verdict-card). Full suite 82/82 green.

### 8. Skill Picker (FEUI-04, SYSK-03)
expected: Skill selector opens dropdown (name+desc); selecting sets active AND switches CenterPanel to SKILL.md preview tab (7 fields). Tab indicator reflects the active view (programmatic + click).
result: pass
evidence: RE-VERIFIED after fix `e757d6e` (Tabs primitive gained optional controlled `value`; CenterPanel binds `value={activeTabValue(centerView.kind)}`). Browser (chrome-devtools MCP, :5177): (a) selecting 企业合规/制度审查 → `Skill 定义` tab `aria-selected=true`, 审查结论 `false`, center renders all 7 SKILL.md fields, header + statusbar `当前 Skill：…` sync; (b) citation-click path (clicking `[权威] c-001` chip after a streamed verdict) → `引用原文` tab `aria-selected=true`, center renders ChunkSourceView (`[权威]` + 版本: v3.2 + 生效日期: 2026-01-15 + verbatim). Both programmatic indicator paths now correct. Regression test `tab indicator follows programmatic setCenterView` added to CenterPanel.test.tsx; CenterPanel 7/7 green, full suite 83/83 (was 82).

### 9. Three-Panel Resize + Collapse + Persist
expected: Drag handle resizes (clamped); collapse buttons zero panels; reload restores from localStorage.
result: pass
evidence: Collapse-left toggled (button label `Collapse` → `Expand`); grid `grid-template-columns: 0px 4px 981px 4px 420px` — left column objectively zeroed. 2 resize handles present. Drag-resize + localStorage round-trip/hydration pinned by `ThreePanel.test.tsx` (6 assertions, green). (True mouse-drag not simulated via MCP; relies on unit test for the drag+persistence path.)

### 10. ChatInput Behavior
expected: Enter sends; Shift+Enter newline; `发送` disabled when empty; drag-drop + 10MB validation.
result: pass
evidence: Enter-to-send verified (sent 2 messages via Enter); `发送` disabled when input empty (observed). Shift+Enter newline + drag-drop/10MB rejection not exercised via MCP (file drag hard to simulate); ChatInput contract otherwise functional.

### 11. Sub-Progress Indicator (FEUI-06)
expected: During streaming, step copy `检索 [权威] 知识层...` → `找到 {N} 条候选` → `重排序中...` → `自校验中...` → `生成结论中...`; collapses on finish.
result: pass
evidence: Transient — collapses on `finish` (correctly absent in finished state; statusbar back to `就绪`). Step-copy rendering pinned by `DataSubprogressPart.test.tsx` (5 assertions covering all 5 steps, green). DEFAULT_FIXTURE may not carry sub-progress steps, so not observed live; unit-tested.

### 12. Source-URL Sanitization (T-1-08)
expected: Non-allowlisted URL (javascript:/external host) renders blocked state, not `<a>`; allowlisted renders `<a target=_blank rel=noopener noreferrer>`.
result: pass
evidence: Not UI-triggerable (single-fixture mock). Pinned by `sanitize-url.test.ts` (7 assertions: javascript: blocked, external host blocked, localhost/127.0.0.1/skillforge.local allowed, file:// blocked) + regression guard in `citations.test.tsx` + `MessageBubble.test.tsx` (T-1-08 block assertions, green).

## Summary

total: 12
passed: 12
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "Selecting a skill (or clicking a citation) switches the CenterPanel to the corresponding view AND the tab indicator reflects the active view."
  status: resolved
  reason: "Center CONTENT switches correctly (skill-md renders 7 fields; chunk-source renders verbatim+metadata), but the `审查结论` tab stayed visually selected/active after programmatic centerView changes. Tabs only synced on direct user click. Root cause: Tabs internal state was not bound to Zustand centerView.kind (dual source of truth, Pitfall-7-adjacent)."
  severity: minor
  test: 8
  root_cause: "CenterPanel's Tabs component maintained its own active-tab state (updates on user click) instead of being a controlled component driven by `centerView.kind` from the Zustand store. Programmatic `setCenterView({kind:'skill-md'|'chunk-source'})` calls (from SkillPicker / CitationCluster) mutated the store and the rendered content, but the Tabs visual indicator never re-read centerView.kind."
  artifacts:
    - path: "web/src/components/center/CenterPanel.tsx"
      issue: "Tabs `value`/active-state not bound to `centerView.kind`"
  missing:
    - "Make Tabs a controlled component: derive active tab from `centerView.kind` (verdict→审查结论, chunk-source→引用原文, skill-md→Skill 定义)."
    - "Add a CenterPanel test asserting the tab indicator follows a programmatic `setCenterView` change."
  resolution: "Fixed in `e757d6e` — Tabs primitive gained optional controlled `value` prop (backward compatible); CenterPanel binds `value={activeTabValue(centerView.kind)}`. +1 regression test `tab indicator follows programmatic setCenterView` covers both skill-md and chunk-source paths. Re-verified in-browser (chrome-devtools MCP) on 2026-08-11: both programmatic paths flip the active tab indicator correctly. Suite 83/83 green."
  resolved_at: 2026-08-11T22:05:00Z
  debug_session: "closed — fix applied directly from the UAT diagnosis (root_cause + missing items were precise enough that no separate debug session was needed); see commit e757d6e."

## Post-Completion Regression Check (2026-08-12)

**Trigger:** `/gsd-verify-work 4` re-run. UAT was already `complete` (12/12, 2026-08-11); phase marked done in ROADMAP + STATE. User chose targeted regression check over full 12-test re-run.

**Scope:** Only one `web/` commit landed after UAT completion — `0b43156 feat(03-01): add data-decision discriminator (Go + TS lockstep)` (+28 lines, `web/src/types/wire-types.ts` only). Adds `DecisionPayload` + a `data-decision` variant to the `SkillForgeChunk` union (D16 / ARAG-01 / SC-1 observability substrate). Type-only on the TS side; backend does not emit it yet (Plan 03-03 will).

**Verification:**
- Full frontend suite: **84/84 green** (was 83/83; +1 guard test below). Production build: PASS (411KB JS / 120KB gzip, 28KB CSS, 11s).
- Dispatcher behavior confirmed by reading `MessageBubble.tsx`: `data-decision` is a declared union member with NO renderer case → the `default` arm runs, `'data-decision'.startsWith('tool-')` is `false` → `return null` (silently dropped, not crashed). AI-SDK-v7 older-client/newer-server forward-compat contract holds.
- **+1 guard test added** — `MessageBubble.test.tsx` › "drops data-decision chunks gracefully (no renderer yet)": renders a message with a `data-decision` part + visible text, asserts the text shows and the decision payload (`retrieve` / query / confidence / testid) does NOT leak into the DOM. Distinct from the pre-existing generic "drops unknown part types silently" test because `data-decision` IS a declared `SkillForgeChunk` union member (just unhandled by the renderer), not a truly-unknown type.

**Verdict:** Existing 12/12 UAT stands — no regression from `0b43156`. Phase 4 remains complete. (No browser/MCP re-verify needed: `data-decision` is unreachable via the UI today — no fixture or backend emits it — so the unit guard is the correct tool, matching how the original UAT pinned the other non-triggerable paths.)
