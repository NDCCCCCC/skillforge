---
phase: 05-skill-platform-full-knowledge-boundaries
reviewed: 2026-08-13T15:30:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - web/src/store/app-store.ts
  - web/src/hooks/useSkillReload.ts
  - web/src/App.tsx
  - web/src/components/chat/ChatInput.tsx
  - web/src/components/layout/SkillManagerMenu.tsx
  - web/src/components/layout/RightChat.tsx
  - web/src/components/layout/LeftSidebar.tsx
  - web/src/__tests__/ChatInput.test.tsx
  - web/src/__tests__/LeftSidebar.test.tsx
  - web/src/__tests__/SkillManagerMenu.test.tsx
  - web/src/__tests__/useSkillReload.test.tsx
findings:
  critical: 2
  warning: 5
  info: 4
  total: 11
status: issues_found
---

# Phase 5: Code Review Report (Plan 05-07)

**Reviewed:** 2026-08-13T15:30:00Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

This plan implements the Phase 5 UAT #2 gap-closure (D5-IA-01/03/04/06) — a frontend-only 4-section LeftSidebar rewrite, session-scoped useChat, live Skill surface, and wizard-only SkillManagerMenu. The structural changes (store contract, IA rewrite, manifest.description path) are correct and the plan's own grep gates pass cleanly.

However, the most important user-facing claim — that "Vercel AI SDK v5 retains per-id message state in its in-memory chatStore" — is **factually wrong**, surfaced by tracing the actual `@ai-sdk/react@4.0.62` source. The `useChat({ id })` hook recreates the underlying `Chat` instance whenever the `id` prop changes, losing all messages. This invalidates D5-IA-03's "messages are in-memory per session until refresh" promise and breaks the user-visible session-switching feature. The second critical issue is a fragility in `useSkillReload.tick()` that wipes the entire skill list on any transient empty `/api/skills` response.

Beyond the two criticals, the IA and store-contract additions are sound. The five warnings are defense-in-depth gaps, test fragility, and accessibility concerns. None of the four test files exercise the actual session-switch behavior, so the critical bug is unverified by tests.

## Critical Issues

### CR-01: Session-scoped useChat loses messages when activeSessionId changes

**File:** `web/src/App.tsx:62` (and the supporting comment at lines 56-60)

**Issue:** The implementation comment claims *"Vercel AI SDK v5 retains per-id message state in its in-memory chatStore while the hook stays mounted; switching activeSessionId swaps which in-memory conversation is rendered."* This is incorrect. Verified in `node_modules/@ai-sdk/react/dist/index.js` (line ~321):

```js
const shouldRecreateChat =
  "chat" in options && options.chat !== chatRef.current ||
  "id" in options && options.id != null && chatRef.current.id !== options.id;
if (shouldRecreateChat) {
  chatRef.current = "chat" in options ? options.chat : new Chat(chatOptions);
}
```

When `id` is provided and the new value differs from the previous Chat's id, the hook allocates a **new `Chat(chatOptions)`**. `chatOptions` does not include `messages`, so the new Chat starts empty. The previous Chat's messages become unreachable. There is no "chatStore" in either `@ai-sdk/react` or `ai` (verified by searching exports for `chatStore`, `allChats`, `registry` — none exist; the only `Store` key the AI SDK uses is `getRealtimeStoreKey` in `useRealtime`, which is unrelated).

User-visible impact: type messages in session A, click 新建会话, click back to session A — its messages are gone. This directly violates D5-IA-03's promise that "messages are in-memory and lost on refresh **only**" (i.e., messages must persist across session switches). The plan's `grep 'id: activeSessionId' web/src/App.tsx` gate passes because the line exists, but the runtime behavior is broken.

**Fix:** Maintain a per-session `Chat` map in a `useRef` and pass the existing instance via the `chat` prop (which the hook preserves instead of recreating). The `id` prop must NOT be passed when `chat` is provided, otherwise the recreation branch fires.

```tsx
// web/src/App.tsx (sketch)
const chatMapRef = useRef<Map<string, Chat<UIMessage>>>(new Map());
const chat = useMemo(() => {
  const existing = chatMapRef.current.get(activeSessionId);
  if (existing) return existing;
  const created = new Chat({ id: activeSessionId, transport });
  chatMapRef.current.set(activeSessionId, created);
  return created;
}, [activeSessionId, transport]);
const { messages, sendMessage, status } = useChat({ chat }); // no `id` prop
```

Add an integration test that mounts `<App>`, sends a message in session A, switches to B, sends another, switches back to A, and asserts session A's messages are still present. This is the test that would have caught the bug.

---

### CR-02: useSkillReload wipes all skills on any transient empty /api/skills response

**File:** `web/src/hooks/useSkillReload.ts:137-149`

**Issue:** `tick()` builds a `summaries` array from `list` and dispatches `useAppStore.getState().setSkills(summaries)` unconditionally. If the API returns `[]` for a single tick (server restart, brief backend hiccup, schema migration, or even a one-frame race during a deploy), every Skill is removed from the store — the LeftSidebar 技能 block goes empty, the ChatInput selector shows "尚未安装任何 Skill", the RightChat badge shows "未选择 Skill", and the active `currentSkillId` becomes a stale reference. The skeletons are restored on the next non-empty poll, but the gap is visible (and any active skill id is silently lost).

The same fragility appears in the test: `web/src/__tests__/useSkillReload.test.tsx:113-157` (the manifest.description test) only queues one response and waits 60ms while polling every 30ms. The second poll hits `responses[1] ?? []` and calls `setSkills([])`. The test happens to pass because `setTimeout(60)` may not always land after the second tick in this CI, but the assertion is timing-dependent.

**Fix:** Skip the `setSkills` dispatch when the new list is empty AND the existing skills list is non-empty (assume empty is a transient glitch). Optionally, also skip when the list shape is invalid (defensive against schema changes).

```ts
// web/src/hooks/useSkillReload.ts
const summaries: SkillSummary[] = list
  .filter((item): item is SkillListItem & { id: string } => typeof item.id === 'string')
  .map((item) => ({ /* ... */ }));

if (!cancelled) {
  const current = useAppStore.getState().skills;
  // Don't wipe the existing list on a transient empty response.
  if (summaries.length > 0 || current.length === 0) {
    useAppStore.getState().setSkills(summaries);
  }
}
```

Also extend the manifest test to either queue enough responses to cover the full wait window, or `await` a specific `setSkills` call via a subscription rather than wall-clock time.

---

## Warnings

### WR-01: LeftSidebar does not sanitize filesystem paths in failed-skill title attribute

**File:** `web/src/components/layout/LeftSidebar.tsx:151`

**Issue:** The `title` attribute is built as `title={failed && skill.error ? \`加载失败：${skill.error}\` : undefined}`. The ReloadToast component applies `stripFsPath()` (lines 49-56) to defense-in-depth against T-5-IA-04 (an absolute filesystem path leaking into the UI). The LeftSidebar does not. The threat model claims T-5-IA-04 is mitigated by "title only" rendering — but a tooltip on hover is still visible to the user, and a malicious YAML reload error containing a server path (e.g., `/var/lib/skillforge/secrets/...`) would be exposed.

**Fix:** Reuse the same `stripFsPath` helper. Either lift it into a shared utility (`web/src/lib/stripFsPath.ts`) or duplicate the inline implementation in LeftSidebar:

```tsx
// web/src/components/layout/LeftSidebar.tsx
function stripFsPath(s: string): string {
  const idx = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'));
  return idx < 0 ? s : s.slice(idx + 1);
}
// ...
title={failed && skill.error ? `加载失败：${stripFsPath(skill.error)}` : undefined}
```

---

### WR-02: Stale currentSkillId is not cleared when the selected skill is removed

**File:** `web/src/components/chat/ChatInput.tsx:62-64`

**Issue:** `currentSkill = skills.find((s) => s.id === currentSkillId)` returns `undefined` when the backend stops returning a previously-selected skill (e.g., Skill was deleted or failed to reload). The trigger displays "未选择 Skill", but `currentSkillId` is still set in the store. The popover rendered on click will show no row as `currentSkillId`-active, because no row matches `skill.id === currentSkillId`. The user sees no signal that their previous selection is invalid. Worse, when they send a message, the backend presumably receives a stale skill id (assuming the chat transport threads it through).

**Fix:** Add a `useEffect` that clears `currentSkillId` when it no longer references an existing skill:

```tsx
// web/src/components/chat/ChatInput.tsx
useEffect(() => {
  if (currentSkillId && !currentSkill) {
    useAppStore.getState().setCurrentSkill(null);
  }
}, [currentSkillId, currentSkill]);
```

---

### WR-03: Test fragility — manifest test only queues one response

**File:** `web/src/__tests__/useSkillReload.test.tsx:113-157`

**Issue:** The manifest test queues only one response and waits 60ms. The polling interval is 30ms, so the second tick fires at ~30ms and receives `responses[1] ?? []` (which is `[]`), invoking `setSkills([])`. The assertion runs after `setTimeout(60)` — by then, the second tick has likely completed (since `fetch` resolves synchronously in the mock), so `skills` may be `[]`. The test passes in this CI environment but is timing-dependent.

**Fix:** Either queue enough responses to cover the test's wait window, or use `vi.waitFor` against the actual store state instead of wall-clock time:

```ts
// Option A — fit the queue to the wait window
const responses = Array(5).fill([{ id: 's1', name: 'S1', version: 1, status: 'ok', manifest: { description: 'desc-from-manifest' } }]);

// Option B — wait for the assertion to hold
await vi.waitFor(() => {
  const skills = useAppStore.getState().skills;
  expect(skills).toHaveLength(1);
  expect(skills[0].description).toBe('desc-from-manifest');
});
```

---

### WR-04: Error string in title attribute can produce malformed tooltips

**File:** `web/src/components/layout/LeftSidebar.tsx:151`

**Issue:** Even after WR-01's path sanitization, multi-line validation errors (e.g., YAML tracebacks, Go panics with stack frames) can leak into the title attribute. React HTML-escapes the content, but the browser renders the tooltip as a single line — long errors are truncated or wrap unpredictably depending on the OS. The `title` attribute is also not localized, not screen-reader-friendly, and is hidden from keyboard users.

**Fix:** Prefer a custom popover/dialog with the full error text rendered as `<pre>`-formatted content, accessible to keyboard and screen reader users. At minimum, replace `\n` with a single space and truncate to a reasonable length before composing the title.

---

### WR-05: OriginalState snapshot captures `Date.now()` at module load

**Files:** `web/src/__tests__/ChatInput.test.tsx:17`, `web/src/__tests__/LeftSidebar.test.tsx:20`, `web/src/__tests__/SkillManagerMenu.test.tsx:20`

**Issue:** `const originalState = useAppStore.getState()` captures `sessions: [{ id: 'session-1', name: '新会话', createdAt: Date.now() }]` at module load. The `createdAt` is a real timestamp, not a stable test fixture. While the test files override `sessions` in `beforeEach`/`afterEach`, if any test has a code path that reads `originalState.sessions[0].createdAt` without overriding, the value drifts each test run. More importantly, this is a subtle test isolation anti-pattern — readers might think the snapshot is fully reset, but it isn't.

**Fix:** Don't snapshot top-level state. Alternatively, freeze the `createdAt` to a constant in the snapshot:

```ts
// In a test setup file
const FROZEN_TS = 0;
const originalState = useAppStore.getState();
originalState.sessions[0].createdAt = FROZEN_TS;
```

Or just rely on per-test overrides and don't snapshot the time-sensitive fields.

---

## Info

### IN-01: `deleteSession` action is unreachable from any UI

**File:** `web/src/store/app-store.ts:226-244`

The `deleteSession` action is defined in the `AppActions` contract but no UI surface (LeftSidebar, RightChat, ChatInput, SkillManagerMenu) invokes it. It is dead code in v1. Either wire it up (e.g., a small × button on each LeftSidebar session row) or remove it from the contract until a future plan needs it.

---

### IN-02: Empty sessions branch in LeftSidebar is unreachable

**File:** `web/src/components/layout/LeftSidebar.tsx:78-79`

The store guarantees `sessions.length > 0` at all times (initial seed + `deleteSession` fallback). The `暂无会话` branch is therefore dead code. It is a defensive fallback that adds visual noise (extra conditional in the JSX) without runtime value. Could be removed for simplicity, or kept as a safety net with a comment.

---

### IN-03: Inconsistent session naming convention

**File:** `web/src/store/app-store.ts:182, 218`

The boot session is named "新会话" (Chinese for "new session"). Newly created sessions via `createSession` are named `会话 N` (e.g., "会话 2"). This naming inconsistency is visible in the LeftSidebar. Cosmetic, but the right sidebar's `right-chat-badge` shows the raw name. Consider naming the boot session "会话 1" or making the create flow's name also "新会话" + a counter.

---

### IN-04: `if (!res.ok) return;` in useSkillReload silently swallows errors

**File:** `web/src/hooks/useSkillReload.ts:129`

The polling loop silently discards non-OK responses. If the backend is down for hours, the user gets no feedback — the UI keeps showing the last good list. Consider a single toast or banner after N consecutive failures, or at least emitting an error to a telemetry channel. Not a bug per se, but it violates the principle of loud failures.

---

## Threat Model Compliance

| Threat | Claimed mitigation | Verdict |
|--------|-------------------|---------|
| T-5-IA-01 (XSS) | React default text escaping | Confirmed — no `dangerouslySetInnerHTML` in any touched file |
| T-5-IA-02 (path injection via currentSkillId) | Client passes only id to existing `/api/chat` | Confirmed — no path construction from currentSkillId in touched files |
| T-5-IA-03 (session ID spoofing) | Client-only Zustand | Confirmed — but see CR-01: the session-scoped useChat does NOT retain per-session messages, so the security property holds but the user-facing feature is broken |
| T-5-IA-04 (filesystem paths in error text) | title attribute only | **Partial** — ReloadToast strips paths, LeftSidebar does not (see WR-01) |

| Quality Gate | Result |
|--------------|--------|
| Plan's 14 grep gates | All pass |
| `npx tsc --noEmit` | Exit 0 |
| `vitest run` | 31 files / 209 tests pass |
| `vite build` | 466.50 KB / 135.02 KB gzip |
| D14 invariant (no `messages` field in store) | Held |
| `item.manifest?.description` path | Held (3 normalization uses, 0 direct `item.description` reads) |

The critical bugs surfaced here are orthogonal to the gated checks — the grep gates validate wiring shape, not runtime behavior. CR-01 specifically requires a runtime test that switches sessions and inspects messages, which is missing from the test suite.

---

_Reviewed: 2026-08-13T15:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
