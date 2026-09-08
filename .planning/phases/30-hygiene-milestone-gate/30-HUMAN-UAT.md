---
status: partial
phase: 30-hygiene-milestone-gate
source: [30-VERIFICATION.md]
started: 2026-09-08T10:30:00+08:00
updated: 2026-09-08T10:30:00+08:00
---

## Current Test

[awaiting human testing]

## Tests

### 1. 27-UAT 全项 operator 亲自跑（Phase 27 doc-upload runbook）
expected: `.planning/phases/27-document-upload-pipeline/27-UAT.md` Tests 1-7 全 PASS（上传/解析/chunk/向量/入库/审计可见/重复去重）
result: [pending]

### 2. 28-UAT 全项 operator 亲自跑（Phase 28 dynamic-knowledge-graph runbook）
expected: `.planning/phases/28-dynamic-knowledge-graph/28-UAT.md` Tests 1-18 全 PASS（KB 图谱动态节点/edge/邻居/路径/性能/降级）
result: [pending]

### 3. 29-UAT 全项 operator 亲自跑（Phase 29 feedback/ticket runbook）
expected: `.planning/phases/29-feedback-loop-ticket-interface/29-UAT.md` Tests 1-7 全 PASS
result: [pending]

### 4. 29-HUMAN-UAT 全项 operator 亲自跑（Phase 29 human-verification）
expected: `.planning/phases/29-feedback-loop-ticket-interface/29-HUMAN-UAT.md` Tests 1-9 全 PASS（含 Phase 27→28→29 联合验收 = FBK-05 信任语义实机确认）
result: [pending]

### 5. HYG-03 live eval 冻结重放
expected: `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go run ./cmd/eval --frozen --gate-check"` exit 0 + faithfulness ≥ 0.9 + refusal_accuracy ≥ 0.95；pre-flight 探测 Milvus/BGE/GLM；断线记环境 STOP 不刷分
result: [pending]

### 6. Phase 30 8 个加固修复回归（operational smoke）
expected: CR-01 SSE emit 不再双写 audit/turn；WR-01 refs 失败重试自动写入；WR-02 queued 行进程重启后被 sweep；WR-03 跨域重定向被阻断 + secret 未泄漏；WR-04 归因只命中 verdict 行；WR-05 公共端点 429 触发 + `local-session-*` 400；WR-06 迟到 votedVerdict 锁定生效；HYG-02 5-stage gate exit 0
result: [pending]

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0
blocked: 0

## Gaps

- 30-REVIEW.md 报告 `internal/server/knowledge.go:341/413` 两处疑似 bug——经 orchestrator 直读确认：**两条均误读**（line 343 单 `c.JSON`、line 413 条件已是 `requestedHops > maxHopsLimit`）。该文件属于先于 Phase 30 启动就已存在的 uncommitted dirty file，不在 30 的 16 files 改动范围；其 diff 单独看可能是合理的或与 reviewer/verifier 看的快照基线不同。30 的 8 个加固修复（CR-01 + WR-01..06）独立验证全部 PASS（见 30-VERIFICATION.md 12/12 must-haves）。
- HYG-03 live 层（`cmd/eval --frozen --gate-check`）依赖内网服务（Milvus/BGE/GLM），按 `.claude/memory/env-intranet-service-flakiness.md` 纪律——live 前探测，断线记环境 STOP 不刷分。