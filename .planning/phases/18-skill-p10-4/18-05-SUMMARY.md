---
phase: 18-skill-p10-4
plan: 05
subsystem: agent/fanout + eval
tags: [gap-closure, fan-05, fan-06, audit-seam, data-channel, hermetic, lock-test]
requires:
  - 18-03 (dataChannelFanOutTool + CR-02 装配——GAP-1 的审计盲区本体)
  - eval/golden/fanout.jsonl + eval/frozen/fanout.lock.json（Phase 18 冻结数据，GAP-2 的死数据本体）
provides:
  - "FanOutAuditSink seam（internal/agent 本地声明，结构兼容 runtime.SubAgentAuditWriter）+ dataChannelFanOutTool 审计行写入（dispatched 决策 JSON / fallback 枚举，runPath data_channel/fanout/<skillID>）"
  - "RunnerBuildConfig.FanOutAudit threading + server fanOutAuditAdapter（store.Store 接口适配 + PlatformFloorVersion + 编译期断言）"
  - "GoldenCase 四断言字段解析 + EvaluateFanOutExpectation（FAN-06 monthly_report 断言入口）"
  - "internal/eval/fanout_lock_test.go（plan 18-02/18-03 声明件：case_count/category/阈值/actual=null/双 sha256 约定结构锁）"
affects:
  - "FAN-05 closure: 分类决策 + 理由可从 audit 表单独重放（D18-C2/C4 落地）"
  - "FAN-06 closure: 冻结 expected_fan_out 被断言代码消费；lock 漂移可检测——操作员 live 重跑（human_verification[1]）现在有消费 seam 可用"
key-files:
  created:
    - internal/eval/fanout_lock_test.go
  modified:
    - internal/agent/fanout_tools.go
    - internal/agent/fanout_tools_test.go
    - internal/agent/runner_setup.go
    - internal/server/runner_setup.go
    - cmd/spike-fanout/main.go
    - cmd/spike-fanout/main_test.go
decisions:
  - "FanOutAuditSink 在 internal/agent 本地声明而非 import internal/runtime——runtime→agent 依赖不可反向；Go 结构类型使 server 适配器同时满足两者"
  - "fallback 审计 reason 复用 D18-C4 fallback_reason 枚举原值（invalid_json/missing_field/boundary_not_in_template/num_subtasks_mismatch/classifier_declined_fan_out），不造新枚举"
  - "ExpectedFallback 用 bool 而非任务书草稿的 string——冻结 JSONL 实际携带 JSON 布尔，string 字段会让 loadFanoutGolden 对全部 4 个 monthly_report 行硬错"
  - "server 适配器走 Deps.Store 的 store.Store 接口（非 *sqlite.SQLiteStore 草稿形态），并补 PlatformFloorVersion——AuditRow 契约要求每条 INSERT 路径携带 floor 版本"
  - "EvaluateFanOutExpectation 采用 verifier 提供的『新增独立断言入口』选项：spike-fanout 入口不可达 data-channel 分类器，不伪造 live 决策；hermetic 测试对真实冻结行钉死语义"
metrics:
  duration: ~45 min
  completed_date: 2026-08-29
---

# Phase 18 Plan 05 Summary: fan-out 分类器审计行 + expected_fan_out 断言消费

**Plan:** 18-05 — 18-VERIFICATION GAP-1（FAN-05）/ GAP-2（FAN-06）closure
**Status:** COMPLETE
**Executed:** 2026-08-29
**Commits:** `7425ba1`（GAP-1）+ `bc552aa`（GAP-2）

## One-liner

两个 gap 同根因（数据通道最后一公里未接）：分类决策/理由只存在于临时工具 JSON（无审计行）+ 冻结 expected_fan_out 无断言消费（GoldenCase 静默丢字段、lock 测试缺文件）。本轮以窄接口审计 seam + 服务器 store 适配器关闭 GAP-1，以 GoldenCase 字段解析 + EvaluateFanOutExpectation 断言入口 + fanout_lock_test.go 结构锁关闭 GAP-2。

## Tasks Completed

| Task | Description | Commit | Status |
|------|-------------|--------|--------|
| Task 1 | fanout_tools.go：FanOutAuditSink seam + audit 字段 + InvokableRun 六路径审计写入（成功=dispatched+决策 JSON；fallback×5=枚举；g.Wait 错误不写） | `7425ba1` | PASS |
| Task 2 | runner_setup.go（agent/server）：FanOutAudit config threading + fanOutAuditAdapter 装配（store.Store 接口 + floor 版本 + 编译期断言） | `7425ba1` | PASS |
| Task 3 | fanout_tools_test.go：stub sink 三测试（dispatched 恰一行含可重放 JSON / fallback×5 恰一行枚举 / nil sink 无 panic 行为不变），既有测试零回归 | `7425ba1` | PASS |
| Task 4 | spike-fanout main.go：GoldenCase +4 断言字段（*bool expected_fan_out）+ EvaluateFanOutExpectation | `bc552aa` | PASS |
| Task 5 | internal/eval/fanout_lock_test.go（NEW）+ main_test.go 解析/断言矩阵测试 | `bc552aa` | PASS |

## Verification

- `CGO_ENABLED=0 go test ./internal/agent/ -run 'FanOut' -count=1` → ok（3 新测试 + 既有 FanOut/SectionTitles/MergeSnapshots 全过）
- `CGO_ENABLED=0 go test ./internal/eval/ -run 'FanOut' -count=1` → ok（TestFanOutLockSelfConsistent + TestFanOutLockSHA256Recomputable）
- `CGO_ENABLED=0 go test ./cmd/spike-fanout/ -count=1` → ok（TestMonthlyReportGoldenCasesParseAssertions + TestEvaluateFanOutExpectation + 既有全量）
- `CGO_ENABLED=0 go build ./cmd/spike-fanout/` → OK；`go vet ./internal/agent/ ./cmd/spike-fanout/ ./internal/eval/` → clean
- **internal/server 类型检查 = operator leg**：本机 CGO_ENABLED=0 败于 go-duckdb/mapping 依赖编译（非本次改动面），CGO_ENABLED=1 败于 D15-01-1 emutls。改动为纯增量 adapter + config 字段 + import，由同文件既有 store/agent import 类型保证

## Deviations

1. **ExpectedFallback bool（非任务书草稿的 string）**：冻结 JSONL 实际值是 JSON 布尔（rpt-fanout-03 true / 其余 false）；按 string 实现会使 loadFanoutGolden 对 4 行全错。
2. **server 适配器字段类型 store.Store 接口**（任务书草稿为 *sqlite.SQLiteStore）：Deps.Store 本就是接口字段；适配器额外携带 PlatformFloorVersion（AuditRow 契约要求）。
3. **观察项（非本次修复，建议后续 plan 跟进）**：ChatHandler 构造 RunnerBuildConfig 时未传 SkillID（既有缺口），故生产 runPath 现为 `data_channel/fanout/`（skill 段空，仍过 sqlite RunPath 非空守卫）；审计行 SkillID 列由适配器以 chatReq.Skill 填充，可按列重放。传 SkillID 会同时改变 tier-channel HITL 规则作用域（BuildToolsForSkill WithContext），超出本 plan 爆破半径，未动。
4. **gofmt 顺带修复**：本次修改的 4 个 Go 文件被 gofmt 重排（含 fanout_tools.go 既有 section_results 对齐漂移）——仅限修改文件，未触碰全仓其他 gofmt 偏差文件。

## Operator Legs（继承 18-VERIFICATION human_verification）

- live 月报端到端（CGO 可链接主机 + GLM）——不变
- FAN-06 操作员 live 重跑：GAP-2 关闭后 monthly_report 4 用例可解析可断言；重跑后回填 monthly_report_fanout_rate_actual 并重签 lock 时需同步更新 TestFanOutLockSelfConsistent 的 actual==null 结构锁（测试失败信息已注明）
