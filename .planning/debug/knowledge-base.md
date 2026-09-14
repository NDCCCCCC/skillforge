# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## subprogress-missing-retrieve — VerdictPipeline 缺失 retrieve subprogress chunk（4-chunk 契约回归）
- **Date:** 2026-09-14
- **Error patterns:** data-subprogress, retrieve chunk missing, "should have 4 item(s), but has 3", TestVerdictPipeline_SubprogressOrder, emitSubprogress, subprogress order, [{found_candidates authority 2} {generate 0} {self_verify 0}]
- **Root cause:** 回归 —— c264fac（refactor 33-01 压缩 VerdictPipeline 行数）删除了 6366eff 引入的 `emitSubprogress(Step:"retrieve", Tier:tiers[0])` 发射行，生产只剩 3 个 data-subprogress chunk；测试锚定的 4-chunk 契约与 FE 类型（verdict.ts SubprogressStepName）/渲染测试/fixture 一致，属测试正确、实现回归。注：live 侧验证曾被 stale-process 干扰延迟（`pkill -x` 因 Linux comm 截断到 15 字符必然匹配失败，:8888 旧进程未被杀掉，端口存在性检查被骗过）；按 PID 重启后，最终 4-chunk 序列（retrieve → found_candidates → generate → self_verify）经 curl 抓 SSE 帧实证确认。
- **Fix:** 在 internal/server/pipeline.go 的 `len(chunks)==0` 拒答闸门之后、found_candidates 发射之前，逐字恢复被删的 `emitSubprogress(c, wire.SubprogressStep{Step: "retrieve", Tier: tiers[0]})` 行
- **Files changed:** internal/server/pipeline.go
---
