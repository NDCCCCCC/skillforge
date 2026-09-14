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

## feedback-admin-gate-order — feedback admin-gate 源序断言锚定过期字面量（Phase 35 helper 抽取未同步测试）
- **Date:** 2026-09-14
- **Error patterns:** TestFeedbackDispositionAdminGate, admin_gate_is_the_FIRST_statement_of_every_admin_handler, isAdminRequest, requireAdmin, source-order regex, assert.Regexp mismatch, putFeedbackAuditRow, "must run the admin-token gate as its FIRST statement"
- **Root cause:** bb3128c（Phase 35 REPEATHTTP-02）把 14 处 inline admin gate 模板抽取为 helper `requireAdmin(c)`（audit.go:142，内部第一步仍是 `isAdminRequest` 检查 + 集中写标准 403 envelope），但未同步更新 feedback_test.go:821（Phase 29-05 / 5898fc7 引入）的源序正则 —— 正则仍锚定全仓库已不存在的字面量 `if !isAdminRequest\(c\) \{`，对三个 feedback admin handler 全部失配。gate 语义从未坏（handlers 第一条可执行语句是 `if !requireAdmin(c) {`，6/6 行为子测试一直 PASS）。诊断陷阱：失败输出里的 putFeedbackAuditRow 源码回显是 red herring —— testify `assert.Regexp` 失败时打印整个源文件字符串，看到的只是长输出的一个窗口，不是正则跨函数匹配。
- **Fix:** 仅改测试：正则锚点换成现行 `if !requireAdmin\(c\) \{`（保留 brace 与 gate 间只允许注释行的严格结构），并新增第二条源序断言锁定 audit.go 的 `requireAdmin` 自身第一条语句是 `if isAdminRequest(c) {` —— 两半锁死「token 检查先于任何副作用/数据访问」的 defense-in-depth 契约，未删除未放宽。
- **Files changed:** internal/server/feedback_test.go
- **Pattern:** helper 抽取类重构（gate/body 模板 → helper）必须同步 grep 所有锚定函数体字面量的结构断言（源序 lock、regex 注入类测试）；否则行为测试照常 PASS，只有结构断言 born-fail。
---
