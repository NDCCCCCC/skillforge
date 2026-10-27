---
status: complete
completed: 2026-10-27
---
# Summary: 261027-ebw — found_candidates 步骤诚实标注 top-5 截断

## What changed

- `wire.SubprogressStep` 新增可选 `Total`（omitempty）= 截断前候选集大小。
- `retriever` 新增可选能力接口 `CandidateCounter`；`embedded.Client` 实现
  `CountCandidates`，复用与 `Search` 完全相同的 `candidates()` 过滤器，
  保证计数与 Search 实际排序的候选集永不分歧。
- `pipeline.go`：类型断言 counter，支持则把 Total 附到 found_candidates 事件；
  不支持则 omitempty（不伪造）。
- 前端 `DataSubprogressPart`：`total > count` 时渲染
  「找到 5 条候选（候选集共 N 条，已按相关度取前 5 条）」，否则文案不变。

## Verification

- `go test ./internal/wire ./internal/retriever/...` 全过（round-trip、
  omitempty、CountCandidates 含 Search 一致性不变量）。
- `go test ./internal/server -run "Subprogress|ClassifierRefuses"` 全过
  （Total 双向断言）。
- `internal/server` 全量有一个存量 flaky：
  `TestAdminPipelineRunGoroutineRecoversFromPanic`（IN-05 panic 模拟在
  goroutine 恢复路径）—— 基线 stash 后全量同样失败，与本次改动无关。
- `web`: DataSubprogressPart 8/8 通过，`tsc --noEmit` 干净。

## Deferred / notes

- topK=5 字面量仍留在 `pipeline.go`（本次只解决"诚实显示"，不调检索深度）。
- Milvus/Mongo retriever 未实现 CandidateCounter（可选接口，按需补）。
