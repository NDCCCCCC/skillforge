---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 07
subsystem: skill-platform
tags: [sc-1, sysk-04, ia-rewrite, sessions, live-skills, left-sidebar, chat-input-selector, tdd]
requires: [05-01, 05-02, 05-03]
provides: [chat-input-skill-selector, left-sidebar-sessions, left-sidebar-skills, skillmanager-wizard-only, session-scoped-usechat, live-skills-data-path]
gap_closure: true
closes_uat: '#2'
---

# Plan 05-07 — D5-IA Left-Sidebar Consolidation + Live-Skill Surface

## Objective

Close the LAST open Phase-5 UAT gap (#2 left-sidebar / right-panel information architecture) by implementing the four LOCKED §4 decisions D5-IA-01/03/04/06. Frontend-only gap closure — no backend file is touched. Reconcile the UI information architecture with the sketch source-of-truth and fix the load-bearing wiring bug where the Skill surfaces read a static fixture instead of the live `GET /api/skills` endpoint.

## What shipped

### Frontend (TypeScript / React 19) — 10 files

| File | Provides |
|------|----------|
| `web/src/store/app-store.ts` | Additive: `SkillSummary` + `SessionMeta` types; `skills: SkillSummary[]` + `setSkills(list)`; `sessions: SessionMeta[]` + `activeSessionId: string` + `createSession(): string` + `setActiveSession(id): void` + `deleteSession(id): void`. Default seeds one '新会话' so `activeSessionId` is never empty at boot. `deleteSession` guarantees `activeSessionId` is never empty (falls back to first remaining session or mints a fresh one). D14 file-level comment forbidding a `messages` field preserved verbatim. |
| `web/src/hooks/useSkillReload.ts` | Tick now also normalizes the live `/api/skills` response into `SkillSummary[]` and dispatches `setSkills` every poll. **Description is read from `item.manifest?.description`** — the BLOCKING CONSTRAINT per `.continue-here.md` (reading `item.description` directly yields undefined). The list itself is NOT a reload signal — only version/status diffs emit toasts. |
| `web/src/App.tsx` | `useChat({ id: activeSessionId, transport })` — Vercel AI SDK v5 retains per-id message state in its in-memory chatStore while the hook stays mounted. Switching `activeSessionId` swaps which in-memory conversation is rendered (D5-IA-03). No backend `ListSessions` / `/api/sessions` added. |
| `web/src/components/chat/ChatInput.tsx` | `data-testid="chat-input-current-skill"` is now a clickable Skill selector popover reading `useAppStore.skills`. Clicking a row calls `useAppStore.setCurrentSkill(skill.id)` and closes the popover. Empty `store.skills` renders a muted 尚未安装任何 Skill row (no crash). The `__fixtures__/skills.json` import + `SKILLS` constant + `currentSkill` fixture lookup are removed. `modeLabel` branch preserved (wizard mode replaces the trigger with `模式：${modeLabel}`). |
| `web/src/components/layout/SkillManagerMenu.tsx` | Wizard-only dropdown. Per-skill `<ul>` + 全部重载 button (D5-IA-05) + `reloadPath` prop + `handleReloadAll` + `__fixtures__/skills.json` import REMOVED. The new-skill entry (`data-testid="skill-manager-new-skill"`) is the sole entry point. Zero fetch calls issued by this component. |
| `web/src/components/layout/RightChat.tsx` | Header is now a minimal read-only badge (`data-testid="right-chat-badge"`) showing the active session name + the current Skill name. `SessionPicker` + default-mode `SkillPicker` imports + header usage are removed. The `SkillPicker` COMPONENT file stays — `RoutingTracePart` still imports it for SC-1 ambiguity mode. |
| `web/src/components/layout/LeftSidebar.tsx` | 4 stacked labeled sections (D5-IA-02/03/04): 新建会话 button (calls `createSession()`), 会话 block (list of `store.sessions`, clickable rows → `setActiveSession`, active row `bg-primary-dim` + border-l-primary), 文件 block (preserved 4-tier SOURCES list), 技能 block (list of `store.skills` with status badges — 已加载 / 加载失败（旧版可用） per UI-SPEC §Copywriting Contract Phase-4-deferred items — and active row marker). Zero fetch calls. |

### Tests — 4 files

| File | Coverage |
|------|----------|
| `web/src/__tests__/useSkillReload.test.tsx` | Added new case: polled response `[{id:'s1', name:'S1', version:1, status:'ok', manifest:{description:'desc-from-manifest'}}]` populates `useAppStore.getState().skills[0].description === 'desc-from-manifest'` (proves `item.manifest.description` path; reading `item.description` would yield undefined). Existing version-bump + status-flip cases preserved. |
| `web/src/__tests__/ChatInput.test.tsx` | 4 new cases: 未选择 Skill default copy; trigger opens popover listing skills from `store.skills` with name + description; clicking a row sets `currentSkillId` and closes popover; empty `skills` shows 尚未安装任何 Skill row. |
| `web/src/__tests__/SkillManagerMenu.test.tsx` | Rewritten: trigger opens dropdown; 新建 Skill（向导） button present + sets `wizardMode.active` + `centerView.kind === 'skill-md'`; `skill-manager-reload-all` is NOT rendered (D5-IA-05); `skill-manager-item` rows are NOT rendered (moved to LeftSidebar); outside-click closes. The 加载失败 / 全部重载 tests deleted. |
| `web/src/__tests__/LeftSidebar.test.tsx` | 8 new cases: 新建会话 button rendered; clicking creates session + new session is active; 2 sessions rendered with s1 marked active (border-l-primary / bg-primary-dim); clicking s2 calls `setActiveSession`; 1 ok skill rendered with name + 已加载 + active marker; failed skill renders 加载失败（旧版可用） with title containing error; empty skills shows 尚未安装任何 Skill (no crash); 文件 block preserves 4 SOURCES tiers (权威/历史/个人/草稿). |

## TDD Gate Sequence

Per the task-level `tdd="true"` flag, each task followed RED → GREEN. Six commits land in this order:

```
7cc252e test(05-07): add failing test for store.skills manifest.description path  [Task 1 RED]
58e1bb5 feat(05-07): store contract + live-skills data path + session-scoped useChat  [Task 1 GREEN]
277d7f3 test(05-07): add ChatInput selector tests + rewrite SkillManagerMenu tests  [Task 2 RED]
b050967 feat(05-07): chat-input Skill selector + SkillManagerMenu wizard-only + RightChat minimal header  [Task 2 GREEN]
9dbbe4d test(05-07): add LeftSidebar 4-section tests (sessions/files/skills)  [Task 3 RED]
0ac41d8 feat(05-07): LeftSidebar 4-section rewrite (sessions + files + skills)  [Task 3 GREEN]
```

No REFACTOR commits were necessary — the GREEN iterations covered all correctness concerns (notably the click-target fix in Task 3 where the `data-testid` had to move from the `<li>` to the `<button>` so `fireEvent.click(row)` targeted the actual onClick).

## Deviations from Plan

**1. [Rule 1 - Bug] Click target mismatch in LeftSidebar session rows (initial GREEN)**
- **Found during:** Task 3 first GREEN run — `clicking the s2 row calls setActiveSession — activeSessionId becomes s2` failed (expected 's2', received 's1').
- **Issue:** Original draft put `onClick` on the inner `<button>` but `data-testid="left-sidebar-session"` on the wrapping `<li>`. `fireEvent.click(s2Row)` clicked the `<li>` (no handler) instead of the button.
- **Fix:** Moved `data-testid` to the `<button>` itself so the click target is the data-testid element. Same restructuring for skill rows.
- **Files modified:** `web/src/components/layout/LeftSidebar.tsx`
- **Commit:** 0ac41d8 (absorbed into GREEN; no separate REFACTOR commit because it was a behavior fix, not a cleanup)

No other deviations. Plan executed exactly as written for the wire shape (item.manifest.description), store contract, and component IA.

## Verification

```bash
$ cd web && npx tsc --noEmit
# (no output — clean)

$ cd web && npx vitest run
 Test Files  31 passed (31)
      Tests  209 passed (209)
# Baseline 196 → +1 (useSkillReload manifest test) + 4 (ChatInput) + 8 (LeftSidebar) = 209

$ cd web && npx vite build
✓ built in 5.43s
dist/assets/index-06H8ir3O.js  459.17 kB │ gzip: 133.10 kB
# Build succeeds — no dead imports from removed __fixtures__/skills.json deps

$ git diff --name-only b134b95..HEAD | grep -E '^(internal|cmd)/'
NO BE FILES CHANGED — frontend-only verified
```

Acceptance grep gates (all pass):

| Check | Result |
|-------|--------|
| `grep messages web/src/store/app-store.ts` | Only in D14 file-level comment (5 lines). No `messages` field on AppState/AppActions. |
| `grep 'manifest?.description\|manifest.description' web/src/hooks/useSkillReload.ts` | 4 hits (1 in interface, 3 in normalization path) |
| `grep 'item.description' web/src/hooks/useSkillReload.ts` | 2 hits — both in comments documenting the bug. 0 in normalization path. |
| `grep -E 'createSession\|setActiveSession\|SkillSummary\|activeSessionId' web/src/store/app-store.ts` | All 4 symbols present |
| `grep 'id: activeSessionId' web/src/App.tsx` | Line 62 inside `useChat({...})` call |
| `grep dangerouslySetInnerHTML web/src/components/{chat/ChatInput,layout/SkillManagerMenu,layout/RightChat,layout/LeftSidebar}.tsx` | 0 hits |
| `grep '__fixtures__/skills.json' web/src/components/chat/ChatInput.tsx web/src/components/layout/SkillManagerMenu.tsx` | 0 hits |
| `grep 'skill-manager-reload-all\|reloadPath\|api/skills/reload' web/src/components/layout/SkillManagerMenu.tsx` | 0 hits (only 2 comment lines explaining the route was removed) |
| `grep 'skill-manager-new-skill' web/src/components/layout/SkillManagerMenu.tsx` | 1 hit |
| `grep 'chat-input-skill-option' web/src/components/chat/ChatInput.tsx` | 1 hit |
| `grep 'right-chat-badge' web/src/components/layout/RightChat.tsx` | 1 hit |
| `grep 'SessionPicker\|SkillPicker' web/src/components/layout/RightChat.tsx` | 0 hits (only comment lines) |
| `grep -E 'left-sidebar-new-session\|left-sidebar-session\|left-sidebar-skill' web/src/components/layout/LeftSidebar.tsx` | All 3 present |
| `grep -E 'createSession\|setActiveSession\|setCurrentSkill' web/src/components/layout/LeftSidebar.tsx` | All 3 present |
| `grep -E '权威\|历史\|个人\|草稿' web/src/components/layout/LeftSidebar.tsx` | 4 hits (SOURCES tiers preserved) |

## Threat Model Compliance

| Threat | Mitigation | Evidence |
|--------|-----------|----------|
| T-5-IA-01 (XSS on skill fields) | React default text escaping only | 0 `dangerouslySetInnerHTML` in any touched file |
| T-5-IA-02 (path injection from currentSkillId) | Client passes only the id to existing `/api/chat` path | No path construction from `currentSkillId` in any touched file |
| T-5-IA-03 (session ID injection) | UI-local Zustand only; no backend roundtrip; refresh wipes all | `sessions` array + `activeSessionId` are client-side only |
| T-5-IA-04 (filesystem paths in error text) | Error string rendered in `title` attribute only | LeftSidebar uses `title={...skill.error}` on the failed-skill row |

## Auth Gates

None. No authentication required for this plan — purely frontend UI surface changes.

## Known Stubs

None. All UI surfaces now read live `GET /api/skills` via `store.skills` populated by `useSkillReload`. The dead `/api/skills/reload` 404 is gone per D5-IA-05. The `__fixtures__/skills.json` static fixture is removed from ChatInput + SkillManagerMenu (the file itself is still on disk and may be used by other components — see `SkillPicker.test.tsx` and `SkillPicker.tsx` ambiguity mode — but no plan-touched component imports it).

## Self-Check

```
FOUND: 7cc252e (test RED 1)
FOUND: 58e1bb5 (feat GREEN 1)
FOUND: 277d7f3 (test RED 2)
FOUND: b050967 (feat GREEN 2)
FOUND: 9dbbe4d (test RED 3)
FOUND: 0ac41d8 (feat GREEN 3)
FOUND: 05-07-PLAN.md (plan)
FOUND: web/src/store/app-store.ts (modified)
FOUND: web/src/hooks/useSkillReload.ts (modified)
FOUND: web/src/hooks/useSkillReload.test.tsx (modified)
FOUND: web/src/App.tsx (modified)
FOUND: web/src/components/chat/ChatInput.tsx (modified)
FOUND: web/src/__tests__/ChatInput.test.tsx (created)
FOUND: web/src/components/layout/SkillManagerMenu.tsx (modified)
FOUND: web/src/__tests__/SkillManagerMenu.test.tsx (modified)
FOUND: web/src/components/layout/RightChat.tsx (modified)
FOUND: web/src/components/layout/LeftSidebar.tsx (modified)
FOUND: web/src/__tests__/LeftSidebar.test.tsx (created)
```
## Self-Check: PASSED