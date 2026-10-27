---
status: executing
---
# Quick Task: 261027-ebw — found_candidates 步骤诚实标注 top-5 截断

## Problem

进度条「找到 N 条候选」的 N 几乎恒等于 5，因为 `internal/server/pipeline.go:256`
把检索深度硬编码为 topK=5，且 UI 文案让用户误以为"恰好 5 条相关候选"。
低相关拒绝时（如"反洗钱提取"）同样显示「找到 5 条候选」，与「范围内无依据」自相矛盾。

## Plan

1. `wire.SubprogressStep` 增加 `Total int` 字段（`json:"total,omitempty"`），
   语义：候选集总大小（截断前）。
2. `embedded.Search` 能拿到候选总数（`len(cands)`），但 retriever 接口不改 ——
   在 pipeline 侧无法直接取。改为：embedded 返回的 hits 上带候选总数？
   最小侵入方案：retriever.Hit 不动；新增接口方法侵入大。
   → 采用：`embedded.Client` 增加导出方法不必需；pipeline 里 `Count: len(chunks)` 保持，
   `Total` 由 `Search` 的一个新包装返回。评估后定稿：
   - 方案 A（选定）：给 `retriever.Retriever` 接口增加可选能力接口
     `CandidateCounter interface { CountCandidates(ctx, tiers, now) (int, error) }`，
     pipeline 类型断言，支持则带 Total，否则不带（omitempty）。
   - embedded.Client 实现 CountCandidates（复用 candidates()）。
3. 前端 `DataSubprogressPart.renderStep`：`found_candidates` 且 `total` 存在且
   `total > count` 时渲染「找到 5 条候选（候选集共 N 条，已按相关度取 top-5）」；
   其余情况不变。
4. 测试：
   - Go：wire round-trip 补 Total；pipeline_subprogress 测试补 Total 断言；
     embedded CountCandidates 单测。
   - FE：DataSubprogressPart 测试补 total 截断/未截断/无 total 三种渲染。
5. 全量相关测试通过后原子提交，更新 STATE.md。

## Verification

- `go test ./internal/...` 相关包通过
- `cd web && npx vitest run src/__tests__/DataSubprogressPart.test.tsx` 通过
