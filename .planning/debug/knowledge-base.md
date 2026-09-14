# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## hitl-approval-snapshot-500 — approval approve 端点 500 "audit write failed"（27db382 重构丢失 snapshot fallback）
- **Date:** 2026-09-14
- **Error patterns:** "skill snapshot required (D11-C2)", "audit write failed", TestDataDecisionExtensionNoNewChunkTypes, executeApprovalResume, MarkApproved, manifestForSkill, activeSkillSnapshot, zero snapshot, Loader-less Deps, approval 500
- **Root cause:** 重构回归 —— 27db382（refactor 33-04）把 approvalPostHandler 拆成 decode/validate/execute/drain 四 helper 时，executeApprovalResume 为 audit 决议行重新推导 snapshot：`activeSkillSnapshot(manifestForSkill(s.Loader, row.SkillID))`，丢了拆分前 expire-check 块中 `rowManifest == nil → s.ActiveManifest` 的 fallback（当时注释明确覆盖「tests construct Deps without a Loader」场景）。测试 Deps 不设 Loader（repo 全部 server 测试既定模式）→ manifestForSkill 返回 nil → 零值 snapshot → AuditWriter.MarkApproved 的 D11-C2 snap.IsZero() 守卫拒绝 → 500。对外文案 "audit write failed" 是下游症状：同一请求 expire-check 用 fallback 判 fresh、audit 写入用零值 snapshot 被拒，两个推导不一致。plan 侧同构测试绿是因为 plan_api.go:365-377 一直有该 fallback。**教训：同一 handler 内同一 manifest 有多处推导时，拆分重构极易在某处丢 fallback 链 —— 新增推导点必须镜像既有 fallback 链，或收敛为单一 helper。**
- **Fix:** executeApprovalResume 把 fallback 推导上移到 audit 写入之前（snap 从 fallback 后的 manifest 推导，与 expire-check / plan_api.go / 拆分前一致），删除 resume 处重复推导。不改对外错误文案（无测试断言 "audit write failed" 字面量；入口闸门保证 ActiveManifest 非 nil，补 fallback 后零值 snapshot 路径不可达）。commit 93570d7（+14/-5）。
- **Files changed:** internal/server/approval_api.go
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

## sql-guides-needs-input — 纯 personal-role KB 的 Skill 被确定性降级为 needs_input@0.89 + 零引用 + compliance 条款卡片
- **Date:** 2026-09-15
- **Error patterns:** needs_input 置信度 89%, compliance_status "needs_input", citations 空, review_required true, 条款修订建议卡片, confidence 0.89, ApplyDowngrade, confidence floor, sql-guides, personal role KB, citableChunksByID, grounding closure empty, 引用被剥空
- **Root cause:** 双层。主因：VerdictPipeline 的 grounding closure 单一来源 citableChunksByID 只收 authority-role chunk（KBM-03 / 8cbe270 citability gate），而 sql-guides 是首个 reads 全为 personal-role 的 Skill（394c2be）→ 闭包恒为空 → ApplyDowngrade 确定性把任何带引用的 verdict 改写为 needs_input@0.89（恰好 ConfidenceFloor 封顶）+ review_required=true + 引用全部剥空 + compliance_status="needs_input"（提示词禁止 LLM 输出、仅 ApplyDowngrade 传播规则会写入的指纹）。设计缺口：26-06 只对 agent 路径落地"空闭包→干净拒绝"（T-26-06-3 accepted），默认 VerdictPipeline 从未处理该形态，落在最差中间态（检索+生成全跑完 → 静默降级）。次因（级联）：emitRedline 闸门只看 Status==needs_input，无 Skill 维度 → 对 operational 型 Skill 也发 compliance 条款改写卡片。
- **Fix:** operational-mode 判定（pipeline.go 新增 readsCitableTier + allChunksByID）：Skill 的 resolved reads 不含 citable-role KB 时 grounding closure = 本次检索全部 chunk；含 authority tier 的 Skill 闭包保持 citable-only 字节不变（KBM-03 护城河不动）。emitRedline +operational bool 参数，首行闸门（先于 cost-control gate）：operational Skill 永不触发条款改写。混合读取 Skill 行为不变（TestVerdictPipeline_MixedReadsKeepCitableOnlyClosure 锁定）。
- **Files changed:** internal/server/pipeline.go, internal/server/redline.go, internal/server/redline_test.go, internal/server/pipeline_operational_test.go
- **Pattern:** 诊断指纹——audit verdict 里 confidence 恰好等于 ConfidenceFloor（0.89）+ citations=[] + compliance_status 取到提示词禁止的值，三者同时出现 = ApplyDowngrade 确定性降级，非 LLM 行为；用对照组 audit 行（正常 needs_input 是 0.95+有引用）即可区分。环境陷阱：Hertz netpoll 监听 [::]:8888 时不调度 IPv4-mapped 127.0.0.1 连接，curl 探针必须打 http://[::1]:8888 或 localhost，否则零字节挂起。
---
