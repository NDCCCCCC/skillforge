---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 08
subsystem: frontend
tags: [cr-01, cr-02, wr-01, wr-02, wr-03, wr-04, wr-05, in-01, in-02, in-03, in-04, chat-session, skill-reload, defense-in-depth, gap-closure]
dependency_graph:
  requires: ['05-07']
  provides: ['session-message-retention', 'transient-empty-skill-resilience', 'loud-failure-banner', 'stripFsPath-utility', 'deleteSession-ui', 'stale-currentSkillId-clear']
  affects: ['App.tsx', 'ChatInput.tsx', 'LeftSidebar.tsx', 'ReloadToast.tsx', 'useSkillReload.ts', 'app-store.ts', 'useSkillReload.test.tsx', 'App.test.tsx', 'ChatInput.test.tsx', 'LeftSidebar.test.tsx', 'SkillManagerMenu.test.tsx']
tech-stack:
  added: []
  patterns: ['useRef Map per-session Chat instance', 'transient-empty guard on setSkills', 'consecutive-failure counter for loud banner', 'shared stripFsPath utility', 'multi-line error flattening + 160-char truncation for tooltips', 'stopPropagation on per-row delete button', 'FROZEN_TS test fixture pattern']
key-files:
  created:
    - web/src/__tests__/App.test.tsx
    - web/src/lib/stripFsPath.ts
  modified:
    - web/src/App.tsx
    - web/src/store/app-store.ts
    - web/src/hooks/useSkillReload.ts
    - web/src/components/chat/ChatInput.tsx
    - web/src/components/layout/LeftSidebar.tsx
    - web/src/components/layout/ReloadToast.tsx
    - web/src/__tests__/ChatInput.test.tsx
    - web/src/__tests__/LeftSidebar.test.tsx
    - web/src/__tests__/SkillManagerMenu.test.tsx
    - web/src/__tests__/useSkillReload.test.tsx
decisions:
  - "CR-01: per-session Chat<UIMessage> map keyed by session id, kept in a useRef. Pass `chat` prop to useChat (NOT `id`); both together would trigger the shouldRecreateChat branch in @ai-sdk/react/dist/index.js:326-328."
  - "D14 invariant preserved: Zustand store has no `messages` field. Each Chat<UIMessage> instance in chatMapRef holds its own message stream."
  - "CR-02: transient-empty guard on setSkills dispatch — skip if summaries.length===0 && current.length>0. Preserves the live skill list across a single backend hiccup."
  - "IN-04: consecutive-failure counter scoped to the useEffect closure. Resets on the first OK response. Surfaces after 3 consecutive non-OK responses (including network failures)."
  - "WR-01: stripFsPath lifted from ReloadToast into web/src/lib/stripFsPath.ts (verbatim export). Single source of truth for filesystem-path stripping."
  - "WR-04: multi-line errors are flattened to single-line + truncated to 160 chars + ellipsis in the title attribute. CSS rendering is OS-stable."
  - "IN-01: each session row carries a × delete button (opacity-0, group-hover:opacity-100). event.stopPropagation prevents the row's setActiveSession from also firing."
  - "IN-02: 暂无会话 branch retained as defense-in-depth (deleteSession can theoretically produce an empty list)."
  - "IN-03: boot session renamed from '新会话' to '会话 1' for consistent naming with createSession()'s '会话 N' pattern."
  - "WR-05: FROZEN_TS = 1700000000000 declared at module scope in ChatInput/LeftSidebar/SkillManagerMenu test files. Replaces Date.now() snapshots in test fixtures."
metrics:
  duration: 14m
  completed: 2026-08-13T13:36:00Z
---

# Phase 5 Plan 8: Gap Closure (CR-01 + CR-02 + 5 WR + 4 IN) Summary

## One-liner

Per-session Chat<UIMessage> map + transient-empty skill-list guard + opportunistic cleanup (stripFsPath, deleteSession UI, offline banner, FROZEN_TS) — closes both critical findings from the 05-07 review and 7 of 9 cleanup items.

## CR-01 fix — session message retention

`web/src/App.tsx` was using `useChat({ id: activeSessionId, transport })`. @ai-sdk/react@4.0.62's `shouldRecreateChat` branch (verified at `node_modules/@ai-sdk/react/dist/index.js:326-328`) allocates a new `Chat()` on every `id` change, wiping all messages. The fix:

- Import `Chat` from `@ai-sdk/react` (verified public export at `index.d.ts:329`).
- Maintain `chatMapRef = useRef<Map<string, Chat<UIMessage>>>(new Map())` in `LiveApp`.
- Derive the current Chat via `useMemo([activeSessionId, transport])`: get existing from the map, or `new Chat({ id, transport })` and store it.
- Call `useChat({ chat })` (NOT `{ id, chat }` — both together still trigger recreation).

D14 invariant preserved: the Zustand store still has no `messages` field; per-session `Chat<UIMessage>` instances hold messages in their internal state.

`web/src/__tests__/App.test.tsx` (NEW) — 4 integration cases:
1. Mounts and shows the initial session.
2. Sends a message in session A; bubble renders.
3. **CR-01 round-trip** — sends SENTINEL-MESSAGE-A in session A, creates session B (asserts A's message hidden while B is active), clicks back to session A (asserts A's message still present). Proves message retention across activeSessionId flips.
4. No `console.error` from React for the chat-prop-without-id-prop shape.

Round-trip case passes 5/5 consecutive runs — timing-resilient.

## CR-02 fix — transient-empty skill-list guard

`web/src/hooks/useSkillReload.ts` was unconditionally dispatching `setSkills(summaries)` on every tick. A transient `[]` response wiped the live skill list. The fix:

- Read `useAppStore.getState().skills` BEFORE the dispatch.
- Skip the `setSkills` call when `summaries.length === 0 && currentSkills.length > 0`. Baseline cold-start (both empty) still applies.

`web/src/__tests__/useSkillReload.test.tsx` — added the transient-empty test: seed `skills=[s1]`, mock fetch to return `[]` on every call, wait 120ms; assert `useAppStore.getState().skills.length === 1 && id === 's1'`. The seeded skill survives across multiple empty polls.

## IN-04 — loud-failure banner after 3 consecutive non-OK responses

`web/src/hooks/useSkillReload.ts` now tracks a `consecutiveFailures` counter in the effect closure. Increments on `!res.ok` (network failures included). Resets to 0 on the first OK response. At counter >= 3, dispatches `useAppStore.setSkillServiceOffline(true)`. On reset, dispatches `setSkillServiceOffline(false)`.

`web/src/store/app-store.ts` — additive extension:
- `AppState.skillServiceOffline?: boolean` (default `false`).
- `AppActions.setSkillServiceOffline(offline: boolean): void`.

`web/src/components/layout/LeftSidebar.tsx` — renders `<div data-testid="skill-service-offline-banner">无法连接到 Skill 服务 — 显示的内容可能已过时</div>` above the 技能 block when the flag is true.

`web/src/__tests__/LeftSidebar.test.tsx` — added the offline-banner test: set `skillServiceOffline=true`, assert the banner is present with the correct text.

`web/src/__tests__/useSkillReload.test.tsx` — added the 3-consecutive-failure test: mock fetch to return `ok: false` on every call, wait for the counter to reach the threshold, assert `useAppStore.getState().skillServiceOffline === true`.

## WR-01 + WR-04 — stripFsPath lifted + multi-line error flattening

`web/src/lib/stripFsPath.ts` (NEW) — single `export function stripFsPath(s: string | undefined): string | undefined` lifted verbatim from `ReloadToast.tsx`. Same semantics: `Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'))` then `slice(idx + 1)`.

`web/src/components/layout/ReloadToast.tsx` — inline `stripFsPath` REMOVED; imports from the shared module.

`web/src/components/layout/LeftSidebar.tsx` — imports `stripFsPath` and applies it to failed-skill `title` attributes via a local `flattenError(s)` helper:
- `stripFsPath` → basename only.
- `\r?\n` → ` ` (single space).
- Truncate to 160 chars + `'…'`.

`web/src/__tests__/LeftSidebar.test.tsx` — added the stripFsPath test: seed a failed skill with error `'/var/lib/secrets/x: missing temperature\n  at parse\n  at validate'`, assert the title contains the basename + flattened text, no `/var/lib/secrets`, no `\n`.

## WR-02 — stale currentSkillId auto-clear

`web/src/components/chat/ChatInput.tsx` — added a `useEffect([currentSkillId, skills])` that calls `useAppStore.getState().setCurrentSkill(null)` when `currentSkillId !== null && !skills.some(s => s.id === currentSkillId)`. The trigger re-renders '未选择 Skill' fallback because `currentSkill = skills.find(...)` is `undefined`.

`web/src/__tests__/ChatInput.test.tsx` — added the 5th test: seed `currentSkillId='s1'`, render, then `useAppStore.setState({ skills: [] })`, wait for the effect to clear, assert `useAppStore.getState().currentSkillId === null` and trigger shows '未选择 Skill'.

## WR-03 — manifest test fragility hardened

`web/src/__tests__/useSkillReload.test.tsx` — the manifest test (lines 113-157) was:
- Mocking fetch to return `responses[0]` blindly (masking the wipe — the second tick would see `[]`).
- Using `setTimeout(60)` (wall-clock timing).

Replaced with:
- A `callIdx` counter returning `responses[idx] ?? responses[0]` (so the second tick does not spuriously return `[]`).
- `await vi.waitFor(() => { ... }, { timeout: 1000 })` against the actual store state.

All 4 polling tests still pass; the new structure is timing-resilient.

## WR-05 — FROZEN_TS replaces Date.now() in test fixtures

`ChatInput.test.tsx`, `LeftSidebar.test.tsx`, `SkillManagerMenu.test.tsx`:
- Declared `const FROZEN_TS = 1700000000000;` at module scope.
- Replaced `createdAt: 0` and `createdAt: 1` with `createdAt: FROZEN_TS` and `createdAt: FROZEN_TS + 1` in the test session seeds.
- Replaced the `'新会话'` boot session name with `'会话 1'` to match the production rename.
- Added `skillServiceOffline: false` to the reset blocks so test order is independent.

## IN-01 — × delete-session button per row

`web/src/components/layout/LeftSidebar.tsx` — each session `<li>` now renders a sibling `<button data-testid="left-sidebar-session-delete">` with:
- `opacity-0 group-hover:opacity-100` (visible only on hover/focus).
- `onClick: (e) => { e.stopPropagation(); useAppStore.getState().deleteSession(s.id); }` — `stopPropagation` prevents the parent row's `setActiveSession` from also firing.
- `aria-label={`删除会话 ${s.name}`}`.
- The `<li>` parent has `className="relative group"` so the hover trigger propagates.

`web/src/__tests__/LeftSidebar.test.tsx` — added the delete-button test: seed 2 sessions, click × on s2, assert `sessions.length === 1` and `activeSessionId === 's1'`.

## IN-02 — 暂无会话 branch retained

Acknowledged in the plan as defense-in-depth. `deleteSession` can theoretically produce an empty list (in practice the boot session + the `deleteSession` fallback prevent it). The branch stays as a safety net.

## IN-03 — boot session name '会话 1'

`web/src/store/app-store.ts` — `sessions: [{ id: INITIAL_SESSION_ID, name: '会话 1', ... }]` (was `'新会话'`). Consistent with `createSession()`'s `会话 N` naming pattern.

## Verification

| Check | Result |
|---|---|
| `cd web && npx tsc --noEmit` | exit 0 |
| `cd web && npx vitest run` | 32 files / **219 tests pass** (209 baseline + 10 new: 4 App + 1 ChatInput + 2 useSkillReload + 3 LeftSidebar) |
| `cd web && npx vite build` | 460.62 KB / 133.58 KB gzip; built in 9.64s |
| `git diff --name-only master..HEAD` | 12 files — all under `web/src/`. **No backend files modified** (`internal/`, `cmd/` clean). |
| `npx vitest run src/__tests__/App.test.tsx` x5 | 5/5 consecutive runs pass (CR-01 round-trip timing-resilient) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test assertion correctness] WR-04 test initially asserted collapsed whitespace**
- **Found during:** Task 3 verification
- **Issue:** The flatten-error test asserted `'x: missing temperature at parse'` (single space) but the actual output had `'x: missing temperature   at parse'` (3 spaces: 1 from \n→space + 2 original indent). The input string had `'\n  at parse'` which produces `'  at parse'` after the newline collapse.
- **Fix:** Compress all whitespace in the assertion (`title.replace(/\s+/g, ' ')`) before the `toContain` check. The flattenError implementation is correct — multi-space runs are not collapsed (intentional: only newlines are normalized for OS-stable tooltip rendering).
- **Files modified:** `web/src/__tests__/LeftSidebar.test.tsx`
- **Commit:** f045004

**2. [Rule 3 - Blocking issue] web/package-lock.json was a partial lockfile**
- **Found during:** Task 1 setup (npm install)
- **Issue:** `web/node_modules/` was absent; the only thing the project had was `web/package-lock.json` containing 5 dependencies from the spike. The full test environment required ~700 lockfile additions.
- **Fix:** Ran `npm install` in `web/`, then committed the populated `web/package-lock.json` as a separate `chore` commit (no source code touched). The test commands now succeed out-of-the-box.
- **Files modified:** `web/package-lock.json`
- **Commit:** bc12e61

### Files NOT in plan (additive)

- `web/package-lock.json` was not in the plan's `files_modified` list but became necessary for the test environment to work. Committed separately as a chore.

### Auth gates

None — no auth steps required.

## Known Stubs

None introduced. The boot session's `Date.now()` in `app-store.ts:194` is acceptable (only the test fixtures needed FROZEN_TS).

## Test Results

### App.test.tsx (NEW — 4 cases)
- (1) mounts and shows the initial session
- (2) sends a message in session A; bubble renders
- (3) **CR-01 round-trip** — session A messages persist across session switches
- (4) does not crash when the chat prop is set without an id prop

### useSkillReload.test.tsx (4 + 2 = 6 cases)
- mounts without throwing
- version bump detection
- status flip (ok→failed) with keep-old
- manifest.description (WR-03 hardened)
- **(NEW) CR-02 transient-empty guard** — empty responses don't wipe live list
- **(NEW) IN-04 offline banner** — 3 consecutive failures surfaces the banner

### ChatInput.test.tsx (4 + 1 = 5 cases)
- 4 original cases
- **(NEW) WR-02 stale currentSkillId clear**

### LeftSidebar.test.tsx (8 + 3 = 11 cases)
- 8 original cases
- **(NEW) IN-01 × delete-session button**
- **(NEW) IN-04 offline banner**
- **(NEW) WR-01 + WR-04 stripFsPath + multi-line flattening**

### SkillManagerMenu.test.tsx (6 cases, unchanged behavior)
- FROZEN_TS applied to test fixtures for consistency

## Self-Check: PASSED

- All 12 modified files exist in the worktree.
- 4 task commits exist: `88e1a10`, `78f5bcd`, `f045004`, `bc12e61`.
- `tsc --noEmit` exit 0; `vitest run` 219/219 pass; `vite build` succeeds.
- No backend files modified.

## Impact

- **CR-01 closed:** session message retention works. Switching sessions does NOT wipe prior messages. `useChat({ chat })` is the right pattern for `@ai-sdk/react@4.0.62`.
- **CR-02 closed:** transient empty `/api/skills` responses no longer wipe the live skill list. `currentSkillId` and the LeftSidebar 技能 block survive a single backend hiccup.
- **7 of 9 cleanup items closed:** WR-01, WR-02, WR-03, WR-04, WR-05, IN-01, IN-03, IN-04. IN-02 acknowledged as defense-in-depth (kept as-is).
- **Threat surface:** `stripFsPath` covers all FS path leaks (T-5-IA-04). `currentSkillId` clears on staleness (T-5-IA-02). No client-side path construction from user input. New `skillServiceOffline` boolean is the desired loud-failure signal (T-5-IA-05 accept).
- **D14 invariant:** preserved — no `messages` field in the Zustand store. Per-session Chat instances hold messages in their internal state.
- **D5-IA-03 invariant:** preserved — messages are in-memory and lost on refresh (no backend ListSessions).
