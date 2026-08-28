---
status: partial
phase: 17-p10-3
source: [17-VERIFICATION.md]
started: 2026-08-27
updated: 2026-08-28
---

## Current Test

[awaiting human testing]

## Tests

### 1. SC-1 端到端聊天数字问答（单值路径）
expected: 启动 server + 前端，聊天输入「上月贷款余额是多少」→ 答案含数值 + KPI 定义 + 计算时间窗 + as_of；数字带虚线下划线，hover 显示 5 字段 popup（含 来源：data:aggregated_only）
result: [2026-08-28 PASS via chrome-devtools live] glm-5.3-flash + 真实 DuckDB 聚合：4,248,934,525.84 元（KPI_LOAN_BALANCE 2026-07，832 行，k_min=5 应用）；hover popup 5 字段全渲染（数值/点 1 / 1、数据截至 2026-08-27T16:09:29Z、来源：data:aggregated_only、函数：KPI_LOAN_BALANCE）。备注：「上月」相对词按工具拒答→LLM 诚实降级（D16-B2 活体验证），SKILL.md 已加 period 换算规则
why_human: 需要活体 LLM + server 运行时；UI hover 呈现无法程序断言

### 2. 修复后视觉走查（DASH-03 闭合后）
expected: 多周期 KPI 回答中查看 sparkline 渲染（N>1 references）→ svg 趋势线内嵌消息，逐点 circle 可见；单值路径 hover tooltip 不回归。注意：逐点 hover popup 属已知 stub（override），不作为本项通过条件
result: [2026-08-28 PASS via chrome-devtools live] 「看最近7个月的贷款余额月度趋势」→ KPI_LOAN_BALANCE_TREND periods=[2026-01..2026-07] → 1 个 data-decision part references[7] → svg[data-testid=sparkline] polyline + 7 circles 内嵌消息；趋势表格 + 环比 + as_of 引用行齐备；单值 hover tooltip 不回归
why_human: 视觉布局 + CSS 渲染；commit 8740805 改变了用户可见 UI，只有测试看过、人眼未看过

### 3. Eval 运行 + 冻结锁签署（operator 步骤）
expected: `go run ./cmd/eval --frozen=kpi --dry-run` 填充 prompt_hash / lock_sha256，然后跑 live eval 50 例 → kpi_call_accuracy >= 0.9（lock thresholds），metrics 填充，operator_signed_at 签署
result: [2026-08-28 BLOCKED-WITH-NOTES] live 50 例未跑，不签署（no-falsify）。GLM 端点实际可用（glm-5.3-flash 已在 chat E2E 工作），但 cmd/eval 链路存在 3 个断点：(a) 环境级：cmd/eval 的 retriever 构造不认 RETRIEVER_BACKEND=none（cmd/server 已加，cmd/eval 是独立一份）→ 内网 Milvus 断即 fatal；(b) 代码级：runKPIMode 的 snapRows 为空占位（judge 第三步无数值快照可比，只剩 allowed_expressions 路径）；(c) 代码级：ag.Run 用 compliance manifest（firstManifest），不路由 kpi-dashboard runner；且 golden set expected_value 为合成假设值，与 testdata 实际聚合值（如 4,248,934,525.84）未对齐。闭环路径：gevl-09 live 化（eval 复用 BuildRunner 数据通道 + 按 golden case 注入真实快照 + 重算 golden 期望值）→ 归 Phase 18 前置或 gap closure 决策
why_human: 依赖内网 GLM 端点可用性（env-intranet-service-flakiness 先例）；签署是 operator 职责

### 4. WSL leg（Stage 06 补腿）
expected: `MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/kpi/wsl/... -count=1'` → PASS（含 7 KPIs + ValidateAgainstKPI 路由名断言）
result: [2026-08-28 PASS] 7/7 tests green (TestKPIEngineIntegration / TestNameLexiconParity / TestKPILoadFailClosedLayer2WSL x3 / TestProbeJSONSerializeSQL x6 / TestKPISnapshotFrozen / TestSameKPIAcrossSkills). Phase 16 legacy 2 blocked-with-notes in internal/kpi/wsl closed by 17-09 fixes.
why_human: Windows MSYS2 gcc 无法链 duckdb_static（D15-01-1 既定先例）；17-09-SUMMARY 声称已过但验证器无法在本 host 复跑

## Summary

total: 4
passed: 3
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps
