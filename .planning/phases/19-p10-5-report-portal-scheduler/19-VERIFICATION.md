---
phase: 19-p10-5-report-portal-scheduler
verified: 2026-08-29T12:55:58Z
status: passed
score: 9/9 must-haves verified
overrides_applied: 0
re_verification: Yes — 第二轮 gap closure（19-05，commit ccd8824）
re_verification_meta:
  previous_status: gaps_found
  previous_score: 8/9
  gaps_closed:
    - "PORT-04 — executeReportRun 新增 audit report.ReportAuditSink 参数，RunReportJob 把 storeDB 直通传入（report_job.go:83/:143/:197）；编译期断言 var _ report.ReportAuditSink = (store.Store)(nil) 锁定接口关系；TestExecuteReportRunWiresAuditSink 实测 PASS 证明 URL=/reports/<period>/v1.html 且 reports 行含 version/digest/template_version"
    - "PORT-03 — cmd/server/main.go:392 补 sched.SetKPIExecutor(kpiExecutor)（先于 :402 的 sched.Start），cron 路径 KPI 快照接线闭合"
  gaps_remaining: []
  regressions: []
gaps: []
---

# Phase 19: Static Report Portal + Shared Scheduler Verification Report

**Phase Goal:** Static report portal + shared scheduler for automated report regeneration with drift detection, eval gate, TTL cleanup
**Verified:** 2026-08-29T12:55:58Z
**Status:** passed
**Re-verification:** Yes — 第二轮 gap closure（19-05，commit `ccd8824`；第一轮 19-04 commit `c28ca46` 修复不完整，本轮补齐）
**Score:** 9/9 must-haves verified

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `GET /reports` lists all published reports with skill/period/version/time/status | VERIFIED | `reportsListHandler` + `server.go:486` 路由；`ListReportRuns` 返回真实审计行（三轮回归确认） |
| 2 | Skill manifest accepts `report_schedule:` cron expression validated at load time | VERIFIED | `manifest.go:80` 字段 + `loader.go:197` `cron.ParseStandard` 加载期校验（回归确认） |
| 3 | Scheduler starts with server and registers cron jobs for all Skills with schedules | VERIFIED | `scheduler.go:80` Start 迭代 `loader.Skills()`；`cron.WithChain(SkipIfStillRunning, Recover)`；main.go `:392` 装配 → `:402` Start 顺序正确 |
| 4 | Every scheduled run produces a `report_runs` audit row with run_id and status | VERIFIED | `report_job.go:112` `PutReportRun` 覆盖 success/failed；`writeBlockedRun` 覆盖 blocked/skipped |
| 5 | Drift is detected by comparing KPI snapshots between consecutive runs | VERIFIED | `drift.go:30` + `report_job.go:88`；本轮 GAP-2 修复后 cron 路径 `kpiExecutor` 已接线（main.go:392），`len(currSnapshots)>0` 守卫不再因 nil executor 恒跳过；DRIFT 徽标 `ReportPortal.tsx:161-167` |
| 6 | Manual rerender via `POST /api/reports/rerender` triggers the same run path as cron | VERIFIED | `rerender.go:66` 传 `deps.KPIExecutor` + `deps.Store`（经 RunReportJob 直通，自动成为 audit sink）；202 + 异步执行 |
| 7 | Scheduler checks eval gate before firing a cron run; blocked runs get `eval_gate_blocked` status | VERIFIED | `report_job.go:50` `evalGate.Check`；blocked → `writeBlockedRun("eval_gate_blocked")` |
| 8 | TTL cleanup job DETACHs DuckDB datasets before deleting; only anonymized datasets are cleaned | VERIFIED | `cleanup_job.go` tier=="anonymized" 检查 + DETACH-before-delete 顺序 |
| 9 | Version persistence —每次发布新增 vN.html 不覆盖 | VERIFIED | `publish.go:198-228` `getNextVersion` 扫描 `reports/<period>/` 取 MAX+1（永不覆盖）；`report_job.go:197` Publish 收到实 audit sink → reports 行写入 version/digest/template_version（PORT-04 持久化链闭合）；`TestExecuteReportRunWiresAuditSink` 实测断言 `URL=/reports/2026-08/v1.html` + 行字段 |

**Score:** 9/9 truths verified

### Re-verification 记录（第二轮，19-05）

**Commit:** `ccd8824`（fix(19-05): wire audit sink + KPI executor into scheduler run paths，PORT-03/PORT-04）

**逐项核实（读代码 + 本机重跑测试，未采信 SUMMARY 陈述）：**

1. **PORT-04 audit sink threading** ✓
   - `report_job.go:143` — `executeReportRun(..., audit report.ReportAuditSink)` 新参数
   - `report_job.go:83` — `RunReportJob` 把已持有的 `storeDB` 直通传入（store.Store 方法集含 PutReport）
   - `report_job.go:197` — `report.Publish(..., skillID, audit)`：nil 决策被撤销，approvalAuditID 保持空串（自动化运行无 HITL 审批）但 reports 行照写
   - `report_job_test.go:42` — 编译期断言 `var _ report.ReportAuditSink = (store.Store)(nil)` 锁定窄接口关系
   - `report_job_test.go:60-78` — `TestExecuteReportRunWiresAuditSink`：断言 `url == "/reports/2026-08/v1.html"`、reports 行 `Version=1`、`ApprovalAuditID` 为空、`FilePath/ResultDigest/TemplateVersion` 非空
   - `report_job_test.go:83-91` — `TestExecuteReportRunNilAuditFailsClosed`：nil sink → error 含 "audit sink not wired"（CR-04 契约在 scheduler 层保持可见）
2. **PORT-03 boot 装配** ✓
   - `cmd/server/main.go:392` — `sched.SetKPIExecutor(kpiExecutor)`，位于 `:402 sched.Start(ctx)` 之前，带注释说明此前 cron 路径空快照 + drift 静默失效的后果
   - 类型正确性经双侧签名交叉验证：`SetKPIExecutor(exec *kpi.Executor)`（scheduler.go:65，包编译通过）× `kpiExecutor = kpi.NewExecutor(...)` 返回 `*kpi.Executor`（main.go:375 / executor.go:205）
3. **一致性检查** ✓
   - `executeReportRun` 生产调用方唯一（report_job.go:83）；`rerender.go:66` 签名未变，手动路径经 storeDB 直通自动获得 audit sink——cron 与手动路径行为一致
   - 测试用 `chdirTemp`（t.TempDir + os.Chdir，无 t.Parallel）隔离发布写入，不污染仓库工作树

**19-04（第一轮）与 19-05（第二轮）closure 对比：** 19-04 把 Publish 调用接进来但传 nil sink（与 publish.go CR-04 fail-closed 契约矛盾，运行期必败）；19-05 修正为实 sink threading 并补齐 boot 接线与测试覆盖。两轮合计，`executeReportRun` 端到端（快照 → 模板 → Publish → URL 回填 → 审计行）已有直接测试覆盖，不再可能以"编译通过"假象掩盖运行期失败。

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/kpi/executor.go` | `GetAllKPISnapshots` method | VERIFIED | executor.go:721-741：迭代 `reg.Catalog()`，逐 KPI `Call`，取 `Snapshot.Rows[0]`，单点失败 log+skip，CGO-free |
| `internal/scheduler/scheduler.go` | `kpiExecutor` field + `SetKPIExecutor` + 装配调用 | VERIFIED | scheduler.go:42/:65 + main.go:392 调用点（本轮闭合） |
| `internal/scheduler/report_job.go` | executeReportRun 完整实现 + 实 audit sink | VERIFIED | :143 audit 参数、:83 storeDB 直通、:197 Publish 实 sink；`TestExecuteReportRunWiresAuditSink` 端到端验证 |
| `internal/scheduler/report_job.go` | outputURL 回填 report_runs | VERIFIED | :107 `OutputURL: outputURL`，URL 来自 Publish 真实返回值（测试断言 `/reports/2026-08/v1.html`） |
| `internal/scheduler/report_job_test.go` | executeReportRun 测试覆盖 | VERIFIED | 两个新测试 + 编译期断言（本轮新增） |
| `internal/server/rerender.go` | 手动路径完整依赖 | VERIFIED | :66 deps.Store（兼 audit sink）+ deps.KPIExecutor |
| `internal/report/publish.go` | 版本递增 + fail-closed 审计 | VERIFIED | :198-228 MAX+1 扫描；:162-166 nil-sink 拒绝（契约未动，调用方已修正） |
| `cmd/server/main.go` | scheduler 装配完整 | VERIFIED | :387-402 Store/EvalGate/KPIExecutor/Detacher/TTLCleanup 全部接线后 Start |

（19-01/19-02/19-03 的其余 artifacts——report_runs.go、schema.go、manifest.go、in_flight.go、eval_gate.go、drift.go、cleanup_job.go、report_portal.go、ReportPortal.tsx——三轮回归抽查未发现变化，维持 VERIFIED。）

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/server/main.go` | `scheduler.go` | `SetKPIExecutor(kpiExecutor)` | WIRED | main.go:392（本轮闭合），先于 :402 Start |
| `report_job.go` | `report/publish.go` | Publish 第 9 参 audit sink | WIRED | report_job.go:197 传 `audit`（storeDB 直通）；编译期断言 report_job_test.go:42 |
| `report_job.go` | `kpi/executor.go` | `kpiExec.GetAllKPISnapshots(ctx)` | WIRED | report_job.go:147；cron 路径 kpiExec 自本轮起非 nil |
| `report_job.go` | `report/publish.go` | `report.Publish(...)` | WIRED | report_job.go:197，测试证明运行期成功返回 URL |
| `scheduler.go` | `skill/loader.go` | `loader.Skills()` | WIRED | scheduler.go:86 |
| `report_portal.go` | `store/report_runs.go` | `ListReportRuns()` | WIRED | 回归确认 |
| `ReportPortal.tsx` | `report_portal.go` / `rerender.go` | fetch / POST | WIRED | 回归确认 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `report_job.go`（cron 路径） | `currSnapshots` | `s.kpiExecutor`（main.go:392 已接线） | Yes — GetAllKPISnapshots 经既有 Call（Phase 16/17 全测试覆盖）取真实值 | FLOWING（本轮闭合） |
| `report_job.go`（手动路径） | `currSnapshots` | `deps.KPIExecutor` | Yes | FLOWING |
| `report_job.go` | `outputURL` | `report.Publish` 返回值（实 audit sink） | Yes — 测试断言真实 URL + reports 行字段 | FLOWING（本轮闭合） |
| `ReportPortal.tsx` | `runs` | `GET /reports` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 三包编译 | `CGO_ENABLED=0 go build ./internal/scheduler/... ./internal/kpi/... ./internal/report/...` | BUILD OK | ✓ PASS |
| scheduler 全量测试（含 2 个新测试） | `CGO_ENABLED=0 go test ./internal/scheduler/ -count=1 -v` | ok 0.247s，8/8 PASS：`TestExecuteReportRunWiresAuditSink`、`TestExecuteReportRunNilAuditFailsClosed`、drift 3 项、eval_gate 3 项 | ✓ PASS |
| Publish 审计契约回归 | `CGO_ENABLED=0 go test ./internal/report/ -run "TestPublish" -count=1`（第一轮已验） | 契约未变，调用方已修正 | ✓ PASS |
| cmd/server 本机编译 | `go build ./cmd/server/...` | 失败于 cgo.exe（本 shell 无 gcc；已知 D15-01-1 环境约束，服务端二进制按 81b975f 惯例 WSL 侧构建） | ? SKIP（环境约束；改动行类型经双侧签名交叉验证） |

### Probe Execution

SKIPPED — 无 `scripts/*/tests/probe-*.sh` 约定探针，PLAN/SUMMARY 未声明探针。Behavioral Spot-Checks 已覆盖可编程验证面。

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|---------|
| PORT-01 | /reports 门户列出全部已发布报告，可筛选 | VERIFIED | `report_portal.go` + `server.go:486` |
| PORT-02 | `report_schedule` = manifest 字段，加载期校验，热重载生效 | VERIFIED | `manifest.go:80` + `loader.go:197` + `scheduler.go:86` |
| PORT-03 | 每次定时重渲染产生审计行 + 全量 KPI 快照（含 `kpi_snapshots[]`、`output_url`） | VERIFIED | 审计行 report_job.go:112；`kpi_snapshots` 经 main.go:392 接线的 executor 真实采集；`output_url` 经实 audit sink 的 Publish 真实回填（ccd8824） |
| PORT-04 | 版本留存——每次重渲染新增 run 目录永不覆盖，历史报告可追溯 | VERIFIED | `getNextVersion` MAX+1 永不覆盖（publish.go:198-228）；reports 行写入 version/digest/template_version（实 sink，report_job_test.go:69-77 断言）+ report_runs.output_url → 历史可追溯（ccd8824） |
| PORT-05 | 手动触发「立即重渲染」复用运行路径 | VERIFIED | `rerender.go` 202 + 异步 RunReportJob（storeDB 直通兼 audit sink） |
| PORT-06 | 漂移检测 ±5% 阈值 + drift 徽标 | VERIFIED | `drift.go:30` + `report_job.go:88`；cron 路径快照源已接线，守卫不再恒跳过 |
| PORT-07 | 并发运行守卫 (skill, period) | VERIFIED | `in_flight.go` + `report_job.go:62` |
| DATA-18 | TTL 清理先 DETACH 后删文件，只作用脱敏数据集 | VERIFIED | `cleanup_job.go` tier 检查 + DETACH-first |
| GEVL-11 | eval 门阻塞指标回退 Skill 的 cron 重渲染 | VERIFIED | `eval_gate.go` + `report_job.go:50` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/kpi/executor.go | 728 | 注释声称 "no audit row"，实际经 `e.Call` 每次写 kpi_calls 审计行（SessionID/SkillID 为空） | ℹ️ Info | 行为符合审计纪律（T-16-06-05），仅注释与实现不符，建议顺手修正 |
| （19-05 全部改动文件） | — | 无 TBD/FIXME/XXX/PLACEHOLDER/stub 模式 | — | 干净 |

### Human Verification Required

无门禁性人工验证项——9/9 truths 均以代码阅读 + 本机通过的测试完成程序化验证。

（信息性备注，非门禁项：真实环境的端到端冒烟——WSL 构建二进制启动服务器、内网 Milvus/DuckDB 数据集就绪后手动触发一次重渲染、浏览器打开 portal 中 output_url 确认 v1.html 渲染效果——属项目既有 live-UAT 惯例范畴，且依赖内网服务可用性。本轮验证遵循"不启动服务器"的验证流程约束，未覆盖此面。）

## Gaps Summary

无残留 gap。第一轮验证记录的 PORT-04（executeReportRun stub）经 19-04（c28ca46）结构性接线但被 nil-audit 否决，最终由 19-05（ccd8824）以实 sink threading + boot 装补齐 + 测试覆盖三项闭合；PORT-03 的 cron 路径 KPI 快照断链同轮闭合。9/9 truths VERIFIED，phase goal 达成，可进入下一 phase。

---

_Verified: 2026-08-29T12:55:58Z_
_Verifier: Claude (gsd-verifier)_
