---
quick_id: 260811-uaf
slug: fix-centerpanel-tab-indicator-sync-with-
description: Fix CenterPanel tab indicator sync with centerView.kind
date: 2026-08-11
phase_context: "04-streaming-trust-ui post-UAT minor fix (test 8)"
status: planned
---

# Quick Task 260811-uaf — Fix CenterPanel tab indicator ↔ centerView.kind sync

## Origin

Resumes the fix scoped in `.planning/HANDOFF.json` + `.planning/phases/04-streaming-trust-ui/.continue-here.md`.
The `/gsd:quick resume` target never existed (prior session paused after `init.quick`, before the
planner wrote the task dir), so this is a fresh RUN using the handoff as the plan source.

## Problem (UAT 04-UAT.md → Gaps, test 8, severity minor)

CenterPanel content switches correctly on programmatic `setCenterView` (SkillPicker → `skill-md`,
CitationCluster → `chunk-source`), but the `审查结论` tab indicator stays visually active. Tabs only
sync on direct user click. **Root cause:** `<Tabs defaultValue="verdict" />` is uncontrolled —
`defaultValue` seeds `useState` once; the Tabs primitive has no controlled `value` prop, so it never
re-reads `centerView.kind`. Dual source of truth (Pitfall-7-adjacent).

## Fix (scoped per handoff: controlled component + 1 test)

Bind the Tabs active state to `centerView.kind` so the indicator follows programmatic store changes.

### Task 1 — Tabs: add optional controlled `value` prop
- **File:** `web/src/components/ui/tabs.tsx`
- **Action:** Add optional `value?: string`. When provided, Tabs is controlled: `activeValue = value ?? internalValue`;
  on click, skip `setInternalValue` when controlled (still fire `onValueChange` for callers that want it).
  Make `defaultValue` optional. Backward compatible — sole existing caller passes `defaultValue` only.
- **Verify:** TypeScript compiles; existing uncontrolled behavior unchanged.
- **Done:** `value` prop accepted; controlled vs uncontrolled branch present.

### Task 2 — CenterPanel: bind Tabs `value` to `centerView.kind`
- **File:** `web/src/components/center/CenterPanel.tsx`
- **Action:** Add `activeTabValue(kind)` helper (`chunk-source`→`chunk-source`, `skill-md`→`skill-md`,
  `empty`|`verdict`→`verdict`). Replace `<Tabs defaultValue="verdict" .../>` with
  `<Tabs value={activeTabValue(centerView.kind)} .../>`. Do NOT wire `onValueChange` (see Decision).
- **Verify:** render shows the active tab matching `centerView.kind`.
- **Done:** No `defaultValue` on CenterPanel's Tabs; `value` derived from store.

### Task 3 — Test: programmatic setCenterView → indicator follows
- **File:** `web/src/__tests__/CenterPanel.test.tsx`
- **Action:** Add 1 `it` block: after `setCenterView({kind:'skill-md'})`, the `Skill 定义` tab is
  `aria-selected="true"`; after `setCenterView({kind:'chunk-source', ...})`, the `引用原文` tab is active.
  Covers both test-8 programmatic paths (SkillPicker + citation click).
- **Verify:** `cd web && npm test -- --run` → **83/83** (baseline 82 + 1).
- **Done:** New test green; full suite green.

## Decision: controlled-value only, NOT bidirectional nav

The handoff's "and/or dispatch setCenterView on tab click" is permissive. This task ships the
**controlled-value direction only** (closes test 8) and does NOT wire `onValueChange → setCenterView`:

1. Existing test `CenterPanel.test.tsx:94` pins D-02 ("clicking the second tab does NOT change
   centerView"). Wiring onValueChange would break that pin and require a test rewrite.
2. `Verdict` has no `id` field (`centerView.verdictId` has **0 consumers**), and a bare chunk-source
   tab click has no chunk payload. Full bidirectional nav has no clean payload for 2 of 3 tabs.
3. `ChunkSourceView` has no empty state — UI-SPEC 169's `点击聊天中的引用以查看权威条款原文。` copy
   is unimplemented. Full nav would pull that empty-state work into scope (scope creep).

Net effect: the store becomes the single source of truth for BOTH the tab indicator AND the body,
eliminating the dual-source-of-truth smell. Tab clicks become visual no-ops — consistent with the
existing D-02 design (navigation flows through SkillPicker / CitationCluster, not the tabs).

## Deferred

- Bidirectional tab navigation (tab click → setCenterView) with payloads.
- UI-SPEC 169 chunk-source empty-state copy; verify 170 skill-md empty state end-to-end.
- Recorded in SUMMARY as a deferred item for a future phase task.

## Out of scope

- ROADMAP.md changes (quick tasks are separate from planned phases).
- Backend / Go changes.
