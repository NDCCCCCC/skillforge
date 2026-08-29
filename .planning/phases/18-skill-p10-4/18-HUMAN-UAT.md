---
status: partial
phase: 18-skill-p10-4
source: [18-VERIFICATION.md]
started: 2026-08-29
updated: 2026-08-29
---

## Current Test

[awaiting human testing — 2 operator-gate items, blocked by D15-01-1 CGO host + intranet GLM service availability]

## Tests

### 1. 完整 live 月报流程端到端
expected: 启动 server，聊天输入「出 2026-08 月报」→ write_todos 计划清单先行可见 → plan_approve 确认门批准 → fan_out_dispatch 分类（审计行 action_fanout_dispatched 落库）→ report_publish 审批卡（结构摘要 + 模板版本 + 要求修改按钮）→ 批准 → /reports/2026-08/v1.html 可访问 → 报告卡片渲染 → 点「带修改重生成」chip 预填 → 新一轮 run 确认门显示「基于 v1 版本」；同 period 二次发布产生 v2.html 且 v1.html 字节不变
result: [pending]
why_human: 需要 live GLM + CGO 可链接的主机（本机 CGO_ENABLED=1 在 cgo 编译步失败，D15-01-1 既有记录），端到端流式行为与视觉效果无法程序化验证

### 2. FAN-06 操作员 live 重跑
expected: GAP-2 修复后（bc552aa 断言入口 + EvaluateFanOutExpectation），用 live LLM 后端重跑 monthly_report 4 用例，回填 monthly_report_fanout_rate_actual 并重签 lock；fan-out 率 ≥ 0.95 阈值，漂移可检测
result: [pending]
why_human: 需要内网 GLM/BGE 服务（间歇可用，见 env-intranet-service-flakiness），且 harness 无法在本机执行 live 决策（不伪造原则）

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
