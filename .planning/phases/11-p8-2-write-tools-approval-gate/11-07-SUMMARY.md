---
phase: 11
plan: 07
subsystem: frontend
tags: [hitl, approval-rules, audit-timeline, resume-banner, zustand, react]
requires:
  - 11-03 (audit endpoint)
  - 11-04 (approval dual endpoint)
  - 11-05 (approval-rules endpoints)
  - 11-06 (approval slice + ApprovalCard — executed CONCURRENTLY; interleaved commits)
provides:
  - ApprovalRulesPanel (LeftSidebar/SessionDrawer 审批规则 section + revoke X)
  - AuditTimelineTab (DetailPanel 6th tab 审计轨迹, read-only)
  - SkillSwitchGuard (SkillPicker aria-disabled while approval.pending)
  - useResumeBanner (HITL-03 cross-refresh parallel fetch + 3s banner)
affects:
  - web/src/store/app-store.ts (CenterView + audit-timeline only; approval slice is 11-06's)
tech-stack:
  added: []
  patterns:
    - "snake_case BE envelope → camelCase FE row mapping in lib/sessions.ts (fetchApprovalRules / fetchPendingApproval / fetchAuditRows)"
    - "Optimistic unmount with 120ms fade-out + inline toast (no confirm dialog)"
    - "Fail-closed audit banner (non-2xx → 审计链异常 role=alert, never an empty list)"
key-files:
  created:
    - web/src/components/ApprovalRulesPanel.tsx (193 lines)
    - web/src/components/AuditTimelineTab.tsx (196 lines)
    - web/src/hooks/use-resume-banner.ts (113 lines)
    - web/src/__tests__/ApprovalRulesPanel.test.tsx (145 lines)
    - web/src/__tests__/AuditTimelineTab.test.tsx (116 lines)
    - web/src/__tests__/ResumeBanner.test.tsx (147 lines)
  modified:
    - web/src/lib/sessions.ts (+HITL fetchers)
    - web/src/components/chat/parts/SkillPicker.tsx (D11-B4 guard)
    - web/src/components/layout/SessionDrawer.tsx (审批规则 section mount)
    - web/src/components/layout/DetailPanel.tsx (6th tab 审计轨迹)
    - web/src/store/app-store.ts (CenterView += audit-timeline)
    - web/src/App.tsx (useResumeBanner invocation)
key-decisions:
  - "ApprovalRulesPanel lands at the foot of SessionDrawer — the conceptual LeftSidebar was removed in the Open Design B2 redesign; SessionDrawer is the surviving left-side surface (D11-D4 intent preserved)"
  - "fetchApprovalRules fires scope=session always + scope=skill&skill_id when a current Skill is known, then merges/dedupes — the BE 400s on scope=skill without skill_id"
  - "useResumeBanner owns the state=auto_passed branch (11-06's hydrateFromGet covers active + expired only); terminal auto-passed + banner set directly, no pending card"
  - "Resume banner copy is a pure derivation (RESUME_BANNER_TEXT keyed by banner.status) — 11-06's ResumeBanner shape carries no text field"
requirements-completed:
  - HITL-03
  - HITL-08
duration: 17 min
completed: 2026-08-20
---

# Phase 11 Plan 07: FE Rules Panel + Audit Timeline + SkillSwitchGuard + Cross-Refresh Resume Summary

HITL-08 + HITL-03 FE surface: operator-visible approval rules (view + revoke X with audit transparency), a read-only audit timeline tab with per-action status bullets and filter chips, a SkillPicker freeze while an approval is pending (D11-B4), and the cross-refresh resume flow (parallel GET /approval + /approval-rules + /history on mount + session change; pending / expired / auto-passed / fail-closed branches; 3s auto-dismiss banner).

## Execution

- Start: 2026-08-20T04:42:27Z — End: 2026-08-20T04:59:22Z (17 min)
- 8 tasks, 12 files (6 created + 6 modified), 7 commits
- Ran CONCURRENTLY with 11-06 (commit interleave: f30800a 11-06 slice landed between my Task 1 design + Task 5; 9de58ff ApprovalCard between Task 5 + 6). File ownership held: no 11-06-owned file (ApprovalCard.tsx, transport, approval slice actions) modified by this plan.

## Commits

- da1c219 feat(11-07): ApprovalRulesPanel — read-only rules list + revoke X + HITL fetchers
- 4ed31df feat(11-07): AuditTimelineTab — read-only audit timeline + filter chips
- c9cfa15 feat(11-07): SkillPicker SkillSwitchGuard (D11-B4)
- aebf6b0 feat(11-07): wire 审批规则 section + 审计轨迹 tab (D11-D4)
- 5708e54 feat(11-07): cross-refresh resume banner hook (HITL-03)
- cf91585 test(11-07): 12 hermetic vitest tests — rules panel + audit tab + resume banner

## Test Results

- Full suite: **50 files / 372 tests — all PASS** (`npx vitest run`)
- Targeted filter (`-t 'ApprovalRulesPanel|AuditTimelineTab|ResumeBanner|SkillPicker|SessionDrawer|DetailPanel'`): 42 passed across 8 files
- New tests (12): TestApprovalRulesPanelEmptyState / TestApprovalRulesPanelRevoke (+ failure branch) / TestApprovalRulesPanelReactiveUpdate / TestAuditTimelineTabRendering / TestAuditTimelineTabReadOnly / TestAuditTimelineTabFilterChips / fail-closed banner / TestApprovalCardResumeBanner (active + auto_passed) / TestApprovalCardExpiredState / TestApprovalCardAuditFailClosed
- `npx tsc --noEmit` exits 0
- ESLint: NOT configured in this repo (no eslint.config.js; no lint script) — tsc is the effective lint gate; the plan's eslint verify steps are inapplicable

## Invariant Guards (all green)

- D-04: `useChat.*transport` in new files → 0 hits
- D15: `type: 'data-decision'` in new files → 0 hits (no new SSE chunk types; resume is HTTP-only)
- T-3-AUDIT: `PUT|DELETE` in AuditTimelineTab.tsx → 0 hits; TestAuditTimelineTabReadOnly asserts no button/anchor inside a row
- HITL-08 viewable+revocable: `approval-rules-revoke` defined in ApprovalRulesPanel; panel mounted in SessionDrawer
- HITL-03: `fetchPendingApproval` invoked in use-resume-banner.ts; parallel fetch wired in App.tsx
- 11-06 boundary: git log for this plan touches neither ApprovalCard.tsx nor the transport

## Deviations from Plan

1. **[Rule 2 — missing critical] Approval slice shape consumed from 11-06's landed implementation, not the plan's sketch.** 11-06 committed its slice (f30800a) mid-execution: snake_case `PendingApprovalRow`, `hydrateFromGet(row, ctx)`, `ResumeBanner` without a text field. My initial duplicate type block was reverted; all 11-07 code reads the real slice. Banner copy lives in `RESUME_BANNER_TEXT` (use-resume-banner.ts).
2. **[Rule 2 — stale plan assumption] SessionDrawer has no 会话/文件/技能/创建 4-section IA.** The Open Design B2 redesign deleted the old LeftSidebar; SessionDrawer is the surviving left-side surface. `<ApprovalRulesPanel />` mounts at the drawer's foot (the "5th section" intent).
3. **[Rule 2 — BE wire truth] Audit rows carry `target` + `prompt_hash`, not `toolName`/`args_hash`** (per 11-04's audit_api.go). Rows render target (mono) + `hash {prompt_hash[:8]}`.
4. **[Rule 2 — BE wire truth] GET /approval returns `{approvals: [...]}` (no 204 path); timestamps are RFC3339** — fetchers parse via Date.parse; Go zero-time → lastUsedAt null （未使用）.
5. **[Rule 1 — testability] Success toast auto-dismiss is 3s (same as error), decoupled from the 120ms row fade-out.** The plan tied toast dismissal to the 120ms timer, which makes the toast unassertable and unreadable; the fade-out stays 120ms per UI-SPEC.
6. **[Rule 2] Fetchers + types live in lib/sessions.ts from the start** (plan Task 6 called for moving them there; no interim duplication in the panel).
7. **[Rule 2] fetchApprovalRules merges session+skill scopes** — BE requires `skill_id` for `scope=skill`, so `scope=session|skill` as a single call is impossible; two calls merged + deduped by rule_id.
8. **[N/A] ESLint verification steps skipped** — no ESLint config exists in this repo.
9. **[Concurrency] app-store.ts committed in aebf6b0 contains ONLY the CenterView += audit-timeline hunk** (11-06's slice was already committed by them in f30800a).

**Total deviations:** 9 (8 auto-fixed, 1 not-applicable). **Impact:** none on the HITL-03/HITL-08 contracts — all acceptance criteria verified green against the real BE wire shapes.

## Authentication Gates

None.

## Issues Encountered

- Concurrent 11-06 execution mid-flight: (a) their uncommitted slice briefly coexisted with my design — resolved by reverting my duplicate and consuming their committed shape; (b) their in-flight ApprovalCard.test.tsx had a transient tsc error during my Task 5 — they fixed it before my final gate (full tsc exits 0). No action needed.

## Self-Check: PASSED

- All 6 created files exist on disk; all 8 tasks' acceptance criteria verified (tsc 0, full vitest 372/372, grep guards green).
- `git log --oneline --grep="11-07"` returns 6 production/test commits + this SUMMARY commit.

## Next Step

**11-08 ready to start** (Wave 5 — operator-run E2E acceptance gate). The FE surfaces it consumes: ApprovalCard (11-06) + ApprovalRulesPanel + AuditTimelineTab + SkillSwitchGuard + cross-refresh resume banner are all live on the store + endpoints.
