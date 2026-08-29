---
phase: 20-p10-6-admin-console
padded_phase: "20"
plan_count: 0
requirements: [ADM-01, ADM-02, ADM-03, ADM-04, ADM-05, ADM-06]
canonical_refs:
  - .planning/PROJECT.md
  - .planning/REQUIREMENTS.md (ADM-01..ADM-06)
  - .planning/STATE.md
  - .planning/phases/06-eval-dashboard-production-hardening/06-VERIFICATION.md (redaction mechanism from Phase 6 to be reused in ADM-06)
  - .planning/phases/18-skill-p10-4/18-CONTEXT.md (fan-out audit row structure for admin view reference)
  - .planning/ROADMAP.md (Phase 20 goal, success criteria, PITFALLS reference: P20 audit growth monitoring, P18 4th frozen lock)
decisions:
  - id: layout
    label: 布局形态
    decision: Tab 切换
    rationale: 横向 Tab 栏四视图复用 /eval dashboard 视觉样式，改造成本最低
  - id: milestone-wrap
    label: 里程碑收尾形态
    decision: 纯文本备忘录写入 20-SUMMARY.md
    rationale: 不额外产文件，在 phase SUMMARY 里一段总结 + operator gates ledger
  - id: eval-interaction
    label: Skill eval 结果交互
    decision: 悬浮提示
    rationale: hover 查看概况，不占列表行高空间
  - id: audit-monitoring
    label: 审计体量监控
    decision: 容量进度条
    rationale: 每张表（audit / report_runs / kpi_calls）格子数阈值，超阈值变红 badge
scope: |
  Phase 20 = admin four-view (/admin) + milestone wrap only.
  No new auth model (reuses X-Admin-Token constant-time comparison from /eval).
  No new redaction — reuses Phase 6 redaction mechanism.
  No editing of KPI definitions or schedule files — UI is read-only; files are sole source of truth.
  Audit growth monitoring is a UI indicator only — INSERT-only invariant is locked elsewhere.
  4th frozen lock (data.lock.json) is a documentation + lock file decision, no new code.
not_in_scope: |
  Authentication model (reused from /eval X-Admin-Token).
  Redaction mechanism (reused from Phase 6).
  Editing of KPI definitions or schedules (read-only UI, files are sole truth).
  Audit archival/retention policy beyond growth monitoring.
tech_stack: Go + Eino + React + shadcn/Tailwind (same as rest of project)
impl_principles:
  - X-Admin-Token constant-time comparison (no new auth)
  - actor_type:admin audit tag on all admin mutations
  - KPI definitions read-only; schedule files read-only
  - audit growth capacity bars (per-table count/threshold, red badge on breach)
  - Tab-based four-view layout (reuse /eval dashboard visual style)
  - eval results shown as hover tooltips, not inline badges
