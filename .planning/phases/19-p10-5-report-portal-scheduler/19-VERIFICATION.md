---
phase: 19-p10-5-report-portal-scheduler
verified: 2026-08-29T12:44:03Z
status: gaps_found
score: 8/9 must-haves verified
overrides_applied: 0
re_verification: Yes — after gap closure 19-04 (commit c28ca46)
re_verification_meta:
  previous_status: gaps_found
  previous_score: 8/9
  gaps_closed: []
  gaps_remaining:
    - "PORT-04 — report.Publish 仍无法在自动化路径上成功返回：executeReportRun 以 nil audit sink 调用 Publish，触发 publish.go 的 fail-closed 契约报错，output_url 仍恒为空串"
    - "PORT-03 — SetKPIExecutor 在 cmd/server/main.go 装配时从未调用，cron 路径 KPI 快照恒为空（手动重渲染路径正常）"
  regressions: []
gaps:
  - truth: "Version persistence —每次发布新增 vN.html 不覆盖（PORT-04）"
    status: failed
    reason: "19-04 把 executeReportRun 从 stub 改为真实实现并接入了 report.Publish（结构性接线成立），但 report_job.go:190 以 nil 作为第 9 个参数（audit ReportAuditSink）调用 Publish。publish.go:162-166 的 fail-closed 契约（CR-04）对 nil audit 直接返回错误（且错误发生在 vN.html 文件已写入之后）。因此生产路径上每次自动化运行：孤儿 vN.html 落盘 → Publish 返回 error → executeReportRun 返回 (\"\", err) → RunReportJob 写 status=\"failed\"、OutputURL=\"\"。上一轮记录的可观测失败（output_url 恒为空串）依旧存在，只是根因从 stub 变成了 nil audit sink。19-04 SUMMARY 把『传 nil audit』列为决策，但该决策与 publish.go 自身的 fail-closed 契约及其测试 TestPublishRequiresAuditSink（publish_test.go:126-135，本机实测 PASS 证明该调用形态必报错）直接矛盾。执行侧只跑了 go build + scheduler 测试，而 scheduler 包仅有 drift_test.go / eval_gate_test.go，对 executeReportRun 零覆盖，因此未能发现。"
    artifacts:
      - path: internal/scheduler/report_job.go
        issue: "line 190: report.Publish(..., nil) 第 9 参 audit sink 为 nil，Publish 必然返回 error，outputURL 恒为空串"
      - path: internal/report/publish.go
        issue: "lines 162-166 fail-closed 契约正确（非缺陷）——缺陷在调用方传 nil；每次失败运行还遗留无 reports 表行关联的孤儿 vN.html 文件"
    missing:
      - "executeReportRun 需接收并传递真实的 audit sink：RunReportJob 已持有 storeDB store.Store，且 SQLiteStore.PutReport（internal/store/sqlite/reports.go:46）已满足 ReportAuditSink 接口，把 storeDB（或窄化后的 sink）传入 executeReportRun 作为 Publish 的第 9 参即可"
      - "为 scheduler 包新增覆盖 executeReportRun 端到端（stub engine + stub audit）的测试，防止再次回归"
  - truth: "每次运行有审计行 + 全量 KPI 快照（PORT-03）— cron 路径 KPI 快照恒为空"
    status: partial
    reason: "SetKPIExecutor（scheduler.go:65）在全仓库无任何调用点。cmd/server/main.go:387-397 装配 Scheduler 时调用了 SetStore/SetEvalGate/SetDetacher/SetTTLCleanup，唯独漏掉 sched.SetKPIExecutor(kpiExecutor)——而 kpiExecutor 在 main.go:375 已构建完成，接线成本为一行。后果：cron 路径 scheduler.go:148 传入 nil executor → executeReportRun 跳过 GetAllKPISnapshots → kpi_snapshots 恒为 {}，且 report_job.go:87 的 len(currSnapshots)>0 守卫使漂移检测（PORT-06）在自动化路径上静默失效。手动重渲染路径正常（main.go:534 → server.Deps.KPIExecutor → rerender.go:66，真实 KPI 数据流通）。"
    artifacts:
      - path: cmd/server/main.go
        issue: "scheduler 装配块（387-397 行）缺少 sched.SetKPIExecutor(kpiExecutor) 调用"
    missing:
      - "在 main.go 的 sched.Start(ctx) 之前补一行 sched.SetKPIExecutor(kpiExecutor)"
---

# Phase 19: Static Report Portal + Shared Scheduler Verification Report

**Phase Goal:** Static report portal + shared scheduler for automated report regeneration with drift detection, eval gate, TTL cleanup
**Verified:** 2026-08-29T12:44:03Z
**Status:** gaps_found
**Re-verification:** Yes — after gap closure 19-04 (commit `c28ca46`)
**Score:** 8/9 must-haves verified

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `GET /reports` lists all published reports with skill/period/version/time/status | VERIFIED | 回归确认：`reportsListHandler` + `server.go:486` 路由未变；`ListReportRuns` 返回真实审计行 |
| 2 | Skill manifest accepts `report_schedule:` cron expression validated at load time | VERIFIED | 回归确认：`manifest.go:80` 字段 + `loader.go:197` `cron.ParseStandard` 校验未变 |
| 3 | Scheduler starts with server and registers cron jobs for all Skills with schedules | VERIFIED | 回归确认：`scheduler.go:80` Start 迭代 `loader.Skills()` 注册非空 `ReportSchedule`；`cron.WithChain(SkipIfStillRunning, Recover)`（scheduler.go:52）未变 |
| 4 | Every scheduled run produces a `report_runs` audit row with run_id and status | VERIFIED | 回归确认：`report_job.go:112` `PutReportRun` 对 success/failed 均写行；`writeBlockedRun` 覆盖 blocked/skipped；19-04 后失败运行也会落审计行（status="failed"） |
| 5 | Drift is detected by comparing KPI snapshots between consecutive runs | VERIFIED | 回归确认：`DetectDrift`（drift.go:30）在 `report_job.go:88` 被调用，机制在快照存在时工作（手动重渲染路径已验证数据流通）。注意：cron 路径因 GAP-2 快照恒空而静默跳过，该问题归入 PORT-03 gap 追踪，不在本 truth 重复扣分 |
| 6 | Manual rerender via `POST /api/reports/rerender` triggers the same run path as cron | VERIFIED | 回归确认 + 增强：`rerender.go:66` 现传入 `deps.KPIExecutor`（main.go:534 装配），手动路径 KPI 数据真实流通 |
| 7 | Scheduler checks eval gate before firing a cron run; blocked runs get `eval_gate_blocked` status | VERIFIED | 回归确认：`report_job.go:50` `evalGate.Check`；blocked → `writeBlockedRun("eval_gate_blocked")` 未变 |
| 8 | TTL cleanup job DETACHs DuckDB datasets before deleting; only anonymized datasets are cleaned | VERIFIED | 回归确认：`cleanup_job.go` tier 检查 + DETACH-before-delete 顺序未变 |
| 9 | Version persistence —每次发布新增 vN.html 不覆盖 | FAILED | 19-04 结构性接线成立（executeReportRun 非 stub、`report.Publish` 被调用、`OutputURL` 字段接入审计行），但 `report_job.go:190` 以 nil audit sink 调用 Publish，触发 `publish.go:162-166` fail-closed 报错 → 每次运行 `status="failed"`、`output_url=""`，与上一轮相同的可观测失败。本机实测：`TestPublishRequiresAuditSink` PASS 证明该调用形态必报错 |

**Score:** 8/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/kpi/executor.go` | `GetAllKPISnapshots` method | VERIFIED | executor.go:721-741 真实实现：迭代 `reg.Catalog()`，逐 KPI `Call`，取 `Snapshot.Rows[0]`，单点失败 log+skip，包保持 CGO-free |
| `internal/scheduler/scheduler.go` | `kpiExecutor` field + `SetKPIExecutor` setter | VERIFIED (存在) / 接线断裂 | scheduler.go:42 字段、:65 setter 存在；但 setter 全仓库零调用点（见 GAP-2） |
| `internal/scheduler/report_job.go` | executeReportRun 真实实现（KPI executor + report.Publish + URL 回填） | STUB→真实，但运行期必败 | :140-196 结构完整（快照→模板→slotTexts→MapSnapshot→Publish）；:190 nil audit sink 使 Publish 运行期必报错（见 GAP-1） |
| `internal/scheduler/report_job.go` | `RunReportJob` 捕获 outputURL 写入审计行 | VERIFIED (plumbing) | :83 接收返回值、:107 `OutputURL: outputURL` 字段接线正确——只因上游 Publish 必败而恒为空串 |
| `internal/server/rerender.go` | 手动路径传 KPIExecutor | VERIFIED | :66 传入 `deps.KPIExecutor`；loader 传 nil → 回落最小模板（可接受，GAP-1 修复后手动路径即可产出带 KPI 的报告） |
| `internal/store/sqlite/reports.go` | `PutReport`（ReportAuditSink 实现，修复路径已具备） | VERIFIED | :46 已实现——修复 GAP-1 无需新增接口，把 RunReportJob 已有的 storeDB 传入即可 |

（19-01/19-02/19-03 的其余 artifacts——report_runs.go、schema.go、manifest.go、in_flight.go、eval_gate.go、drift.go、cleanup_job.go、report_portal.go、ReportPortal.tsx——本轮回归抽查关键点未发现变化，维持 VERIFIED。）

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `report_job.go` | `kpi/executor.go` | `kpiExec.GetAllKPISnapshots(ctx)` | WIRED | report_job.go:144 调用；但 cron 路径 kpiExec 为 nil（GAP-2），仅手动路径数据流通 |
| `report_job.go` | `report/publish.go` | `report.Publish(...)` | PARTIAL | report_job.go:190 调用存在，但 nil audit sink → 运行期必返回 error |
| `report_job.go` | `report/publish.go` | Publish 第 9 参 audit sink | NOT_WIRED | 传 nil；SQLiteStore.PutReport 已实现该接口却未被传入 |
| `cmd/server/main.go` | `scheduler.go` | `SetKPIExecutor(kpiExecutor)` | NOT_WIRED | setter 存在但装配块（main.go:387-397）无调用；server.Deps.KPIExecutor（main.go:534）只覆盖手动路径 |
| `scheduler.go` | `skill/loader.go` | `loader.Skills()` | WIRED | scheduler.go:86 未变 |
| `report_portal.go` | `store/report_runs.go` | `ListReportRuns()` | WIRED | 未变 |
| `ReportPortal.tsx` | `report_portal.go` / `rerender.go` | fetch / POST | WIRED | 未变 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `report_job.go`（手动路径） | `currSnapshots` | `deps.KPIExecutor` → GetAllKPISnapshots | Yes | FLOWING（较上轮 STUB 有实质改善） |
| `report_job.go`（cron 路径） | `currSnapshots` | `s.kpiExecutor` = nil（SetKPIExecutor 未调用） | No — `if kpiExec != nil` 跳过 | DISCONNECTED（GAP-2） |
| `report_job.go` | `outputURL` | `report.Publish` 返回值 | No — Publish 因 nil audit 必报错，result 为 nil | DISCONNECTED（GAP-1，与上轮同症状、新根因） |
| `ReportPortal.tsx` | `runs` | `GET /reports` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 三包编译 | `CGO_ENABLED=0 go build ./internal/scheduler/... ./internal/kpi/... ./internal/report/...` | BUILD OK | ✓ PASS |
| scheduler 测试 | `CGO_ENABLED=0 go test ./internal/scheduler/... -count=1` | ok 0.193s | ✓ PASS（但不含 executeReportRun 覆盖） |
| Publish nil-audit 契约（证明 GAP-1） | `CGO_ENABLED=0 go test ./internal/report/ -run "TestPublishRequiresAuditSink\|TestPublishWritesAuditRow" -count=1 -v` | 两测试均 PASS——TestPublishRequiresAuditSink 证明与 report_job.go:190 相同的 nil-audit 调用形态必返回 "audit sink not wired" 错误且 result 为 nil | ✗ 证明集成路径运行期必败 |

### Probe Execution

SKIPPED — 无 `scripts/*/tests/probe-*.sh` 约定探针，PLAN/SUMMARY 亦未声明探针；以上 Behavioral Spot-Checks 已覆盖可编程验证面。端到端启动服务器触发重渲染依赖内网 Milvus/GLM/BGE 栈，留给下一轮 gap 修复后的验证。

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|---------|
| PORT-01 | /reports 门户列出全部已发布报告，可筛选 | VERIFIED | `report_portal.go` + `server.go:486`（回归确认） |
| PORT-02 | `report_schedule` = manifest 字段，加载期校验，热重载生效 | VERIFIED | `manifest.go:80` + `loader.go:197` + `scheduler.go:86`（回归确认） |
| PORT-03 | 每次定时重渲染产生审计行 + 全量 KPI 快照（含 `kpi_snapshots[]`、`output_url` 字段） | PARTIAL → 计入 GAP | 审计行 ✓（report_job.go:112）；但 cron 路径 `kpi_snapshots` 恒空（GAP-2）、`output_url` 恒空（GAP-1），两个显式声明字段不达标 |
| PORT-04 | 版本留存——每次重渲染新增 run 目录永不覆盖，历史报告可追溯 | FAILED | 文件层 vN 递增逻辑正确（publish.go:198-228）且每次运行确会写入新 vN.html；但 nil-audit 使运行必标 failed、output_url 空、无 reports 表行 → "历史报告可追溯" 不成立（GAP-1） |
| PORT-05 | 手动触发「立即重渲染」复用运行路径 | VERIFIED | `rerender.go` 202 + 异步 RunReportJob；本轮还接入 deps.KPIExecutor（增强） |
| PORT-06 | 漂移检测 ±5% 阈值 + drift 徽标 | VERIFIED | `drift.go:30` + `report_job.go:88` + `ReportPortal.tsx` DRIFT 徽标（回归确认；cron 路径受 GAP-2 影响静默跳过，已在该 gap 记录） |
| PORT-07 | 并发运行守卫 (skill, period) | VERIFIED | `in_flight.go` + `report_job.go:62`（回归确认） |
| DATA-18 | TTL 清理先 DETACH 后删文件，只作用脱敏数据集 | VERIFIED | `cleanup_job.go`（回归确认） |
| GEVL-11 | eval 门阻塞指标回退 Skill 的 cron 重渲染 | VERIFIED | `eval_gate.go` + `report_job.go:50`（回归确认） |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/kpi/executor.go | 728 | 注释声称 "no audit row"，实际经 `e.Call` 每次会写 kpi_calls 审计行（SessionID/SkillID 为空） | ℹ️ Info | 行为本身符合审计纪律（T-16-06-05），仅注释与实现不符；建议下一轮顺手修正注释 |
| （其余 3 个 19-04 修改文件） | — | 无 TBD/FIXME/XXX/PLACEHOLDER/stub 模式 | — | 干净 |

### 19-04 Gap Closure 评估（Re-verification 记录）

**Commit:** `c28ca46`（feat(19-04): wire KPI executor and report.Publish into scheduler to close PORT-04）

**已落实的部分：**
1. `GetAllKPISnapshots`（executor.go:721）真实实现且被 executeReportRun 调用 ✓
2. `executeReportRun` 从 stub 变为完整实现（模板解析、slotTexts、MapSnapshot、Publish 调用链）✓
3. `RunReportJob` 签名扩展 + `OutputURL` 字段回填 plumbing ✓
4. 手动重渲染路径接入 `deps.KPIExecutor`，手动路径 KPI 数据真实流通 ✓
5. 三包 `CGO_ENABLED=0` 编译通过 ✓

**未达成的部分（gap 未闭合）：**
1. **GAP-1（PORT-04 本体）**：`report_job.go:190` 第 9 参传 nil audit sink → `publish.go:162-166` fail-closed 必报错 → output_url 恒空、status 恒 failed。上一轮的可观测失败原样保留，仅根因由 stub 换成 nil-audit。执行侧声明"传 nil audit"为决策，但与 publish.go 的 CR-04 契约及包内测试直接矛盾；且 scheduler 包测试对 executeReportRun 零覆盖（仅 drift_test.go / eval_gate_test.go），导致 `go test` 通过造成"已修复"假象。
2. **GAP-2（PORT-03 cron 路径）**：`SetKPIExecutor` 全仓库零调用点；main.go:387-397 装配块遗漏一行 `sched.SetKPIExecutor(kpiExecutor)`（executor 在 main.go:375 已就绪）。cron 自动化路径 KPI 快照恒空，连带 PORT-06 漂移检测在该路径静默失效。

**修复成本评估：** GAP-1 = 把 RunReportJob 已持有的 storeDB 传入 executeReportRun 作为 Publish 的 audit sink（SQLiteStore.PutReport 已实现接口）+ 补一个 executeReportRun 端到端测试；GAP-2 = main.go 一行接线。两项合计预期极小改动量。

### Human Verification Required

无新增人工验证项——两个 gap 均为可编程验证的代码级缺陷。待 gap 修复后，建议下轮验证加入：启动服务器（内网栈可用时）手动触发一次重渲染，确认 `GET /reports` 行的 output_url 可点击打开对应 vN.html（浏览器渲染效果属人工确认范畴）。

## Gaps Summary

**2 gaps（1 个继承未闭合 + 1 个新发现），根因相互独立：**

**GAP-1（继承自上轮，根因更换）：PORT-04 版本留存。** 19-04 把 Publish 调用接进来了，但传了 nil audit sink，而 publish.go 的 fail-closed 契约（CR-04，防"文件落盘而审计表空"）使该调用在运行期必然失败。每次自动化运行留下孤儿 vN.html、审计行标 failed、output_url 恒空。修复：传入 storeDB 作为 audit sink + 补 executeReportRun 测试覆盖。

**GAP-2（新发现）：PORT-03 cron 路径 KPI 快照恒空。** SetKPIExecutor 定义了但从未被调用——scheduler 装配块遗漏接线。cron 路径（本 phase 的主交付路径"共享调度器自动重渲染月报"）拿不到 KPI 数据，漂移检测随之静默失效；仅手动重渲染路径数据流通。修复：main.go 一行。

两个 gap 修复后 PORT-03/PORT-04 才能真正闭合；届时本 phase 9/9 truths 可达 passed。

---

_Verified: 2026-08-29T12:44:03Z_
_Verifier: Claude (gsd-verifier)_
