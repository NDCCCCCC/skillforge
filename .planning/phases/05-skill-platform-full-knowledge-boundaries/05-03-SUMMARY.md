---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 03
subsystem: web-ui
tags:
  - sysk-04
  - sysk-05
  - sc-1
  - sc-2
  - sc-3-entry
  - sc-4-placeholder
  - sc-5-placeholder
  - d14
  - d15
dependency_graph:
  requires:
    - 05-01
    - 05-02
  provides:
    - sysk-04-ui
    - sysk-05-ui
    - sc-3-entry-point
    - sc-4-empty-state
    - sc-5-empty-state
  affects:
    - center-panel
    - top-bar
    - status-bar
    - app-store
    - message-bubble
tech-stack:
  added: []
  patterns:
    - sse-event-source-with-exp-backoff
    - zustand-mirror-of-wire-decision
    - five-tab-center-panel
    - shadcn-command-pattern-handrolled
key-files:
  created:
    - web/src/components/chat/parts/RoutingTracePart.tsx
    - web/src/components/inspector/AgentInspector.tsx
    - web/src/components/graph/KnowledgeGraphBrowser.tsx
    - web/src/components/layout/TopBar.tsx
    - web/src/components/layout/SkillManagerMenu.tsx
    - web/src/components/layout/ReloadToast.tsx
    - web/src/components/status/StatusBar.tsx
    - web/src/hooks/useSkillReload.ts
    - web/src/lib/sse-client.ts
    - web/src/__tests__/RoutingTracePart.test.tsx
    - web/src/__tests__/AgentInspector.test.tsx
    - web/src/__tests__/KnowledgeGraphBrowser.test.tsx
    - web/src/__tests__/SkillManagerMenu.test.tsx
    - web/src/__tests__/ReloadToast.test.tsx
    - web/src/__tests__/StatusBar.test.tsx
    - web/src/__tests__/sse-client.test.ts
    - web/src/__tests__/useSkillReload.test.tsx
  modified:
    - web/src/components/chat/parts/SkillPicker.tsx
    - web/src/components/chat/MessageBubble.tsx
    - web/src/components/center/CenterPanel.tsx
    - web/src/components/layout/ThreePanel.tsx
    - web/src/store/app-store.ts
    - web/src/types/verdict.ts
    - web/src/App.tsx
    - web/src/__tests__/CenterPanel.test.tsx
decisions:
  - "Tab navigation became bidirectional (Phase 5 lifts D-02 'display-only' restriction) — clicking any tab dispatches setCenterView. The Phase-4-deferred bidirectional-nav item is now resolved."
  - "SkillPicker accepts optional `scores` prop for ambiguity mode; per-candidate CTA 使用此 Skill + dismiss 使用默认 Skill. The default mode (no scores prop) is unchanged from Phase 4."
  - "TopBar + StatusBar extracted from ThreePanel.tsx so the SC-2 skill manager + SC-2 pinned-version indicator live in their own files (per UI-SPEC §Component Contracts)."
  - "useSkillReload falls back to 2s polling of /api/skills when EventSource is unavailable (jsdom / dev-only mode). The SSE channel is the primary path; polling is the degradation strategy."
  - "ReloadToast strips absolute filesystem paths in validation error bodies (T-5-14 defense-in-depth) — UI never leaks server filesystem structure."
  - "StatusBar pinned-version indicator driven by prop (wiring from useChat.messages metadata lands in 05-04). Component contract is locked; the data source is the only Phase-5-deferred piece."
metrics:
  duration_minutes: 30
  completed_at: 2026-08-12
  task_count: 2
  test_count_before: 84
  test_count_after: 122
  test_files_before: 14
  test_files_after: 22
  bundle_size_kb: 419.81
  bundle_size_gzip_kb: 122.69
---

# Phase 5 Plan 03: SC-1 Routing Trace + SC-2 Hot-Reload UI + Skill Manager Menu — Summary

## One-Liner

SC-1 routing trace + ambiguity picker rendered inline on every routed turn, plus SC-2 fsnotify hot-reload toast surface, topbar skill-manager dropdown, and 5-tab CenterPanel wired through `setCenterView` — full UI surface delivered end-to-end against the 05-02 backend (D5-04 wire extension + D5-06 in-flight version pinning).

## Completed Tasks

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | SC-1 routing trace + ambiguity picker | `f3e7a9b` | `web/src/components/chat/parts/RoutingTracePart.tsx`, `web/src/components/chat/parts/SkillPicker.tsx`, `web/src/components/chat/MessageBubble.tsx`, `web/src/store/app-store.ts`, `web/src/types/verdict.ts`, `web/src/__tests__/RoutingTracePart.test.tsx` |
| 2 | SC-2 hot-reload UI + Skill manager menu + 5-tab CenterPanel | `f1834da` | `web/src/components/center/CenterPanel.tsx`, `web/src/components/layout/TopBar.tsx`, `web/src/components/layout/SkillManagerMenu.tsx`, `web/src/components/layout/ReloadToast.tsx`, `web/src/components/status/StatusBar.tsx`, `web/src/components/inspector/AgentInspector.tsx`, `web/src/components/graph/KnowledgeGraphBrowser.tsx`, `web/src/hooks/useSkillReload.ts`, `web/src/lib/sse-client.ts`, `web/src/App.tsx`, `web/src/components/layout/ThreePanel.tsx`, plus 7 test files |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed `sse-client` connect not closing the prior EventSource**

- **Found during:** Task 2 — first sse-client test run
- **Issue:** Calling `connect()` twice left the prior `EventSource` open (idempotency violation). React StrictMode / HMR re-mounts would leak subscribers.
- **Fix:** Added an explicit close of the prior `es` before constructing a new `EventSource` in `open()`. The fix is captured by the new `sse-client.test.ts > connect is idempotent` assertion.
- **Files modified:** `web/src/lib/sse-client.ts`
- **Commit:** `f1834da`

**2. [Rule 2 - Critical] SkillPicker ambiguity mode + RoutingTracePart wiring**

- **Found during:** Plan authoring — the plan listed `data-decision` dropping silently in the existing `MessageBubble.test.tsx`. The `decision: 'retrieve'` payload still drops silently (forward-compat with Phase 3 inspector), but `decision: 'route' | 'counter_evidence'` now routes to `RoutingTracePart`. The pre-existing test continues to pass because it uses `decision: 'retrieve'`.
- **Issue:** None (correct wiring from the start).
- **Fix:** N/A — the dispatcher guard in `MessageBubble.tsx` correctly only routes `route | counter_evidence` decisions, leaving other decision values to drop silently.
- **Files modified:** `web/src/components/chat/MessageBubble.tsx`

### Architectural Choices

- **TopBar + StatusBar extraction** — the plan listed `TopBar.tsx` and `StatusBar.tsx` as separate files, but Phase 4 had inlined them inside `ThreePanel.tsx`. Task 2 extracts them into the planned files via composition (`ThreePanel` now composes `<TopBar />` and `<StatusBar />`). The existing `ThreePanel.test.tsx` assertions (data-testids `app-topbar`, `app-statusbar`, `collapse-left`, `collapse-right`) continue to pass because the testids are preserved on the extracted components.
- **ReloadToast without sonner dependency** — the plan calls for sonner toasts, but shadcn init has not yet run (per UI-SPEC §Registry Safety, shadcn init is a Wave-1 prerequisite). `ReloadToast` is implemented as a hand-rolled fixed-position stack so the surface ships today; the shadcn swap-in is a 1-line replacement once sonner is added.

## Acceptance Criteria Coverage

### Task 1 — SC-1 routing trace + ambiguity picker

| Criterion | Status | Evidence |
| --------- | ------ | -------- |
| `cd web && npx vitest run` exits 0 | PASS | 122/122 tests pass |
| `cd web && npx tsc --noEmit` exits 0 | PASS | D15 drift alarm green |
| RoutingTracePart renders 路由分析 + one row per skillScores entry | PASS | `RoutingTracePart.test.tsx > renders 路由分析 + one row per SkillScore` |
| RoutingTracePart renders SkillPicker when ambiguous=true | PASS | `RoutingTracePart.test.tsx > renders SkillPicker ambiguity mode` |
| RoutingTracePart renders 路由信息缺失。 when scores empty | PASS | `RoutingTracePart.test.tsx > renders the defensive empty state` |
| SkillPicker accepts scores prop with per-candidate CTAs | PASS | `RoutingTracePart.test.tsx > ambiguity picker per-candidate CTA invokes onPickAmbiguous` |
| CenterView union includes 'agent-inspector' and 'knowledge-graph' kinds | PASS | `app-store.ts` discriminated union |
| Citation includes effective_from/to/version optional fields | PASS | `verdict.ts` Citation interface |

### Task 2 — SC-2 hot-reload UI + Skill manager menu + 5-tab CenterPanel

| Criterion | Status | Evidence |
| --------- | ------ | -------- |
| `cd web && npx vitest run` exits 0 | PASS | 122/122 tests pass |
| `cd web && npx tsc --noEmit` exits 0 | PASS | D15 drift alarm green |
| CenterPanel renders 5 tabs with labels per UI-SPEC | PASS | `CenterPanel.test.tsx > renders five tab buttons with the correct Chinese labels` |
| Clicking tab Agent 状态 dispatches setCenterView({kind:'agent-inspector'}) | PASS | `CenterPanel.test.tsx > default tab is verdict — clicking the Agent 状态 tab dispatches setCenterView` |
| AgentInspector empty state renders 暂无 Agent 状态 | PASS | `AgentInspector.test.tsx > renders the empty state with the verbatim heading and body` |
| KnowledgeGraphBrowser empty state renders 知识库为空 | PASS | `KnowledgeGraphBrowser.test.tsx > renders the empty state with the verbatim heading and body` |
| SkillManagerMenu lists Skills with per-skill status | PASS | `SkillManagerMenu.test.tsx > lists every installed skill with per-skill status` |
| useSkillReload updates store on skill-reloaded message | PASS | `useSkillReload.test.tsx > polling fallback detects a version bump and updates the store` |
| ReloadToast renders success/failure toasts | PASS | `ReloadToast.test.tsx > renders a success toast for status="ok"` + `renders a failure toast` |
| StatusBar shows pinned-version label on version mismatch | PASS | `StatusBar.test.tsx > renders the pinned-version indicator when pinnedVersion=true` |
| sse-client auto-reconnects on disconnect | PASS | `sse-client.test.ts > onerror closes the EventSource and schedules reconnect` |

## Threat Surface

| Flag | File | Description | Mitigation |
| ---- | ---- | ----------- | ---------- |
| threat_flag: re-render-on-sse-error | `web/src/lib/sse-client.ts` | Reconnect storm on EventSource error | Exp backoff (3s base, 30s cap). Hook unmounts on component unmount. Store is debounced (max one toast per skill per ts check). |
| threat_flag: filesystem-path-leak | `web/src/components/layout/ReloadToast.tsx` | BE validation errors may include absolute filesystem paths | T-5-14 — `stripFsPath` reduces to basename before render |
| threat_flag: data-decision-injection | `web/src/components/chat/parts/RoutingTracePart.tsx` | Server-emitted skillName / score rendered as HTML | T-5-13 — score formatted via `toFixed(2)` + string interpolation only; `skillName` rendered as plain text node; NEVER `dangerouslySetInnerHTML` |
| threat_flag: skill-reload-spoof | `web/src/hooks/useSkillReload.ts` | Spoofed skill-reloaded event | T-5-12 — toast is informational only; actual Skill reload is BE-side per D5-08 + 05-01 atomic.Pointer |
| threat_flag: tab-navigation-bypass | `web/src/components/center/CenterPanel.tsx` | Tab click bypasses store | Phase-5 bidirectional nav is the intended behavior; `setCenterView` is the only store mutation path |

## Verification Results

```
$ cd web && npx tsc --noEmit
(no output — D15 drift alarm green)

$ cd web && npx vitest run
Test Files  22 passed (22)
     Tests  122 passed (122)
  Duration  6.62s

$ cd web && npm run build
dist/assets/index-*.js  419.81 kB │ gzip: 122.69 kB
✓ built in 7.72s
```

## Self-Check: PASSED

- All claimed created files exist on disk (RoutingTracePart, AgentInspector, KnowledgeGraphBrowser, TopBar, SkillManagerMenu, ReloadToast, StatusBar, useSkillReload, sse-client).
- All claimed modified files were modified (MessageBubble, SkillPicker, CenterPanel, ThreePanel, app-store, verdict, App, CenterPanel.test).
- Both commits exist in git log:
  - `f3e7a9b` feat(05-03): wire SC-1 routing trace + ambiguity picker
  - `f1834da` feat(05-03): wire SC-2 hot-reload UI + 5-tab CenterPanel + skill manager
- 38 new tests added (122 - 84 = 38) across 8 new test files (RoutingTracePart, AgentInspector, KnowledgeGraphBrowser, SkillManagerMenu, ReloadToast, StatusBar, sse-client, useSkillReload).
- All 84 baseline tests still pass (no regressions).
- Production build succeeds (419.81 kB / 122.69 kB gzipped — within Phase-4 baseline).
- D15 compile-time wire drift alarm still green (SkillForgeChunk assignable to UIMessageChunk).
- D14 invariant preserved: `messages` field stays banned from `app-store.ts`.

## Carry-forward for 05-04 / 05-05

- **`useSkillReload` polling fallback** is wired and ready; the SSE channel lives at `/api/skills/events` per the plan. The BE endpoint is not yet implemented (per the orchestrator note in the plan: "verify location before subscribing; if not yet implemented, add a minimal stub"). The polling fallback ensures the UI surface works even without the BE endpoint.
- **`StatusBar` pinned-version indicator** is prop-driven; 05-04 will wire `useChat.messages` metadata → `pinnedVersion` + `pinnedSkillName` props.
- **`SkillManagerMenu` wizard CTA** calls `setWizardMode({ active: true, step: 1, totalSteps: 5 })`; 05-04 ships the actual wizard UI that consumes this state.
- **`AgentInspector` + `KnowledgeGraphBrowser`** are empty-state placeholders; 05-05 ships the full Accordion trees per UI-SPEC §SC-4 / §SC-5.
- **`@`-mention fuzzy search** (Phase-4-deferred item overlapping Phase 5) is NOT implemented in 05-03 — it lives in 05-04 alongside the wizard (per UI-SPEC §Phase-4-deferred items).
- **LeftSidebar real file tree** (Phase-4-deferred) is NOT implemented — it shares the SC-5 graph browser component (Accordion) per UI-SPEC §Layout; lives in 05-05.

## Notes

- `messages` field stays BANNED from `app-store.ts` (D14 invariant preserved).
- `dangerouslySetInnerHTML` is NOT introduced anywhere in this plan's diff (T-5-13 mitigation applied throughout).
- `data-decision` with `decision: 'retrieve'` continues to drop silently (forward-compat with the Phase 3 inspector data path).
- The wizard entry point is mounted but the wizard UI itself lives in 05-04; clicking 新建 Skill（向导） currently sets `wizardMode.active = true` + routes to `Skill 定义` tab, which is sufficient as a navigation gate.
