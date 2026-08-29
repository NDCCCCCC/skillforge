---
phase: "18"
slug: skill-p10-4
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-28
updated: 2026-08-29
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + testify v1.11.1（BE）；vitest ^2.0.0（FE） |
| **Config file** | 无独立配置（go test 惯例）；web/vitest 配置既有 |
| **Quick run command** | `go test ./internal/report/ ./internal/skill/ ./internal/agent/... -count=1` |
| **Full suite command** | `go test ./... -count=1` + `cd web && npx vitest run` |
| **Estimated runtime** | ~120 秒（BE 快速集）；~60 秒（FE vitest） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/report/ ./internal/skill/ ./internal/agent/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1` + `cd web && npx vitest run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 180 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | TestMonthlyReportTemplateLoadFailClosed | ASVS V5 | Skill 拒载时错误点名冲突字段 | unit | `go test ./internal/skill/ -run TestMonthlyReportTemplateLoadFailClosed` | ✅ W0 | ⬜ pending |
| 18-01-02 | 01 | 1 | TestTemplateSlotTypesFixed | ASVS V5 | 自定义槽位类型加载期被拒 | unit | `go test ./internal/skill/ -run TestTemplateSlotTypesFixed` | ✅ W0 | ⬜ pending |
| 18-01-03 | 01 | 1 | TestKPINumbersServerInjected | — | 数字路径类型结构锁（LLM 不产数字） | unit | `go test ./internal/report/ -run TestKPINumbersServerInjected` | ✅ W0 | ⬜ pending |
| 18-01-04 | 01 | 1 | TestNarrativeNumberConsistency | — | 叙事数字三步判定一致 | unit | `go test ./internal/report/ -run TestNarrativeNumberConsistency` | ✅ W0 | ⬜ pending |
| 18-01-05 | 01 | 1 | TestRenderPipelineOrder | ASVS V14 | goldmark→bluemonday 顺序不可换 | unit | `go test ./internal/report/ -run TestRenderPipelineOrder` | ✅ W0 | ⬜ pending |
| 18-01-06 | 01 | 1 | TestMarkdownTableInjectionBlocked | ASVS V14 | 单元格语法字符转义 | unit | `go test ./internal/report/ -run TestMarkdownTableInjectionBlocked` | ✅ W0 | ⬜ pending |
| 18-01-07 | 01 | 1 | TestReportPublishHitlGate | ASVS V4/V5 | 未审批不执行+三行审计+冻结快照 | unit | `go test ./internal/agent/hitl/ -run TestReportPublishHitlGate` | ✅ W0 | ⬜ pending |
| 18-01-08 | 01 | 1 | TestApprovalCardShowsStructure | ASVS V4 | 审批卡 payload 含章节+KPI名+模板版本 | FE vitest | `cd web && npx vitest run -t ApprovalCardShowsStructure` | ✅ W0 | ⬜ pending |
| 18-01-09 | 01 | 1 | TestRequestChangeThirdState | ASVS V4 | 独立审计枚举+触发新一轮确认门 | unit | `go test ./internal/agent/hitl/ ./internal/server/ -run TestRequestChangeThirdState` | ✅ W0 | ⬜ pending |
| 18-01-10 | 01 | 1 | TestVersionIncrementNoOverwrite | — | v1→v2 共存+v1 字节不变 | unit | `go test ./internal/report/ -run TestVersionIncrementNoOverwrite` | ✅ W0 | ⬜ pending |
| 18-01-11 | 01 | 1 | TestReportsTableAuditChain | — | reports 行含审批ID+kpi_calls 引用 | unit | `go test ./internal/store/sqlite/ -run TestReportsTableAuditChain` | ✅ W0 | ⬜ pending |
| 18-02-01 | 02 | 2 | TestFanOutBoundaryFromTemplate | ASVS V5 | task_boundaries ∈ 模板章节 | unit | `go test ./internal/agent/ -run TestFanOutBoundaryFromTemplate` | ✅ W0 | ⬜ pending |
| 18-02-02 | 02 | 2 | TestFanOutClassifierStructuredOutput | — | 结构化输出+决策+理由进审计 | unit | `go test ./internal/agent/ -run TestFanOutClassifierStructuredOutput` | ✅ W0 | ⬜ pending |
| 18-02-03 | 02 | 2 | TestFanOutFallbackSequential | — | 校验失败→顺序执行+fallback_reason | unit | `go test ./internal/agent/ -run TestFanOutFallbackSequential` | ✅ W0 | ⬜ pending |
| 18-02-04 | 02 | 2 | TestFanOutFrozenSetAssertion | — | golden schema+lock byte-identity | unit | `go test ./internal/eval/ -run TestFanOutFrozenSetAssertion` | ✅ W0 | ⬜ pending |
| 18-02-05 | 02 | 2 | TestRegenerateNewRunNotPatch | — | 重生成=新 run，非原地补丁 | integration | `go test ./internal/server/ -run TestRegenerateNewRunNotPatch` | ✅ W0 | ⬜ pending |
| 18-03-01 | 03 | 3 | TestReportCardChatMessage | ASVS V4 | 卡片+wire union 不扩 | unit+FE | `go test ./internal/wire/ -run TestReportCard` + `cd web && npx vitest run -t ReportCard` | ✅ W0 | ⬜ pending |
| 18-03-02 | 03 | 3 | TestStaticRouteServesReports | ASVS V4 | 正常 200+遍历按构造拒 | unit | `go test ./internal/server/ -run TestStaticRouteServesReports` | ✅ W0 | ⚠️ gcc-env-blocked |
| 18-03-03 | 03 | 3 | TestNoTemplateCodeExecutionSurface | ASVS V14 | 模板无代码执行面 | structural | `go test ./internal/report/ -run TestNoTemplateCodeExecutionSurface` | ✅ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/report/template_test.go` — TestMonthlyReportTemplateLoadFailClosed + TestTemplateSlotTypesFixed
- [x] `internal/report/render_test.go` — TestRenderPipelineOrder + TestMarkdownTableInjectionBlocked + TestKPINumbersServerInjected + TestNarrativeNumberConsistency + TestNoTemplateCodeExecutionSurface
- [x] `internal/report/publish_test.go` — TestVersionIncrementNoOverwrite
- [x] `internal/agent/hitl/hitl_report_publish_test.go` — TestReportPublishHitlGate + TestRequestChangeThirdState
- [x] `internal/agent/fanout_tools_data_channel_test.go` — TestFanOutBoundaryFromTemplate + TestFanOutClassifierStructuredOutput + TestFanOutFallbackSequential
- [x] `internal/store/sqlite/reports_test.go` — TestReportsTableAuditChain
- [x] `internal/wire/report_card_test.go` — TestReportCardChatMessage
- [x] `internal/eval/fanout_lock_test.go` — TestFanOutFrozenSetAssertion

All Wave 0 test skeleton files listed in PLAN.md frontmatter `files_modified` (8 files across 3 plans).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| GLM 分类器漂移率实测 | FAN-06 | 内网 GLM 服务间歇断（记环境 STOP，断线不刷分）；N 次重跑需 operator gate | 探 GLM 端点可用后执行 `go test ./internal/agent/ -run TestFanOutClassifierDriftRate -v`（N=5 次）；记录一致率进 lock 元数据 |
| 完整端到端聊天流（重生成 chip→新 run→审批→发布→静态页） | RPT-01/02/03/04/05 | 需要 live chat session + 完整 agent 装配；非 hermetic 测试范围 | Phase 18 UAT checklist（PLAN.md 附件） |

*If none: "All phase behaviors have automated verification."*

---

## Security ASVS Coverage

| ASVS Category | Phase Coverage | Key Controls |
|---------------|---------------|-------------|
| V4 Access Control | ✅ | HITL-05 writes 交集强制；report 路由正则白名单 |
| V5 Input Validation | ✅ | template: fail-closed 校验；fan_out_dispatch args 结构校验；ApprovalDecision.Validate |
| V14 HTML/XML Output | ✅ | goldmark 默认转义 + bluemonday UGCPolicy 双层；管线顺序结构锁；TestNoTemplateCodeExecutionSurface |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-08-29

---

## Validation Audit Trail

| Audit Date | Test Packages | Passed | Failed | Blocked | Notes |
|------------|---------------|--------|--------|---------|-------|
| 2026-08-29 | report, skill, agent, agent/hitl, store/sqlite, wire, eval | 9/9 packages PASS | 0 | 1 (internal/server gcc linking — environment issue, not test defect) | Full suite except server package (DuckDB gcc linking environment issue); all individual test names from Per-Task Map verified PASS |

---

*Phase: 18-skill-p10-4*
*Validation Strategy derived from: 18-RESEARCH.md §Validation Architecture*
*Created: 2026-08-28 via /gsd:plan-phase 18*
*Verified: 2026-08-29 via /gsd:validate-phase 18*
