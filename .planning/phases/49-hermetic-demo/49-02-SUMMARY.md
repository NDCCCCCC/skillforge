# 49-02 EXECUTION SUMMARY — Stub LLM Path + Triple Annotation

Phase 49 · plan 49-02 · wave 1 · 依赖 49-01（embedded 后端，已落地）。

## 结果

全部 task 完成，验证全绿：

- `go build ./...` ✅
- `go test ./internal/server/stubchat/ ./internal/verdict/ ./cmd/server/ -count=1` ✅（三包全 ok）
- `cd web && npx vitest run` ✅（86 文件 / 697 用例全过）
- plan 硬约束复核：`internal/server/pipeline.go` 零改动（`git diff --stat` 为空）；`grep -c "STUB LLM MODE"` = 1；`envRequired("BGE_M3_URL")` 保留 = 1。

## files_modified（12）

| # | 文件 | 变更 |
|---|------|------|
| 1 | `internal/verdict/verdict.go` | Verdict 加 `Mode string \`json:"mode,omitempty"\`` + `const ModeStub = "stub"`。真模型不写字段，wire 向后兼容。 |
| 2 | `internal/server/stubchat/extract.go`（新建） | 证据块抽取：`extractRefBlocks` 按 pipeline.go:781 的 `[ref-N] tier=X source=Y\n<chunk>\n---\n` 格式行锚定解析；`firstSpan` 取 top chunk 首个 8–200 rune 句段（跳空行/#/---）；`renderVerdict` 产出 Verdict——有块：`status=ok, confidence=0.5, Span=Answer=firstSpan, citation 指向 blocks[0]`；无块/空 span：`refused, confidence=0, citations=[]`（非 nil），两种均 `ReviewRequired=true, Mode="stub"`。citations 过与真模型完全相同的 grounding 校验（span 是 chunk 原文子串），零豁免。 |
| 3 | `internal/server/stubchat/stub.go`（新建） | `type Stub struct{}` 实现 `model.BaseChatModel`（Stream + Generate；`var _ model.BaseChatModel = (*Stub)(nil)`）。Stream 按 64 rune 分片、`schema.StreamReaderFromArray` 返回——**绝不为空**（≥1 token，规避 pipeline 零 token 判 ERROR）；Generate 返回完整 JSON。输出确定性：同一 prompt 字节级一致（含并发测试）。**教训：确定性输出勿用 goroutine+channel 流**（曾用 schema.Pipe+goroutine，WSL 下出现原因不明的首片后 EOF 截断，改 StreamReaderFromArray 后彻底消失）。 |
| 4 | `internal/server/stubchat/stub_test.go`（新建） | 10 个测试：抽取表驱动（ref-7/ref-9 双块夹具）、Stream/Generate 字节一致、空证据块 refused、伪造 span 被 ApplyDowngrade 压制（≤ ConfidenceFloor）、citations 过 GroundingCheck、Confidence≤0.5+ReviewRequired、Stream 非空、并发流字节一致、userOf 从后向前找。 |
| 5 | `cmd/server/main.go` | ①地雷 2（D-11）：`glmKey := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))`（惰性；缺 key 不再 Fatalf）；`stubActive := glmKey == ""` 时打醒目 `[WARN] cmd/server: STUB LLM MODE — …`（四要素：无 key/无 LLM 生成/confidence≤0.5+review_required 恒 true/mode="stub"；不打印 key 值；全文恰 1 处该子串）。②`BGE_M3_URL` envRequired 必填不动（D-01/D-11）。③装配矩阵：`var pipelineChatModel model.BaseChatModel; var toolChatModel model.ToolCallingChatModel`；stub 分支 `pipelineChatModel = stubchat.New()`，真模型分支两者 = cm（原逻辑逐字保留）。消费点：adk Model→pipelineChatModel；agent.NewAgent ChatModel→toolChatModel；induction.NewManager、ecl.NewLLMExtractor→pipelineChatModel；rewriter/classifier 改三分支 switch（stub 时保持 nil 并打 default-deny WARN——分类器会误解析 stub 的 verdict JSON 致全部查询被拒，此分支是保命的）；wizard 变量 stub 时 nil（端点 503，既有 nil 姿态）；Deps：Runner/Wizard/ChatModel→toolChatModel（nil 时 emitRedline 既有 nil-gate 自动跳过）、`PipelineChatModel→pipelineChatModel`（唯一插入点）。④49-01 的 `buildRetriever` embedded 分支未动。 |
| 6 | `cmd/server/backend_switch_test.go` | `TestGlmKeyLineStillEagerFor49_02` → `TestGlmKeyLineIsLazyForStubMode`（按其注释承诺同步）：断言 `envRequired("GLM_API_KEY")` 出现 0 次、glmKey 行含 `envOr("LOCAL_API_KEY"` + `os.Getenv("GLM_API_KEY")`、`envRequired("BGE_M3_URL")` 仍存在。 |
| 7 | `cmd/server/main_test.go` | `TestBootAssemblyOrder` 的 wizard 断言 `skill.NewWizard(cm,…)` → `skill.NewWizard(toolChatModel,…)`（锁的本意是 kbReg 注入，随 49-02 变量改名同步；plan 外文件，最小一行）。 |
| 8 | `web/src/types/verdict.ts` | `VERDICT_MODES = ['stub'] as const`、`VerdictMode`、`Verdict.mode?: VerdictMode`、`isStubVerdict()`（独立守卫，**不改** `isVerdict`——真模型 verdict 仍通过，status-only 检查不破坏）。 |
| 9 | `web/src/types/verdict.test.ts`（新建） | 6 用例：VERDICT_MODES 仅 stub、真模型无 mode 字段、isStubVerdict 仅 mode:"stub" 为真、isVerdict 兼容有/无 mode、垃圾输入拒绝、既有 COMPLIANCE_STATUSES 未动。 |
| 10 | `web/src/components/center/VerdictCard.tsx` | 两个渲染面（chat-stream inline stripe + CenterPanel 全卡）均加 `data-testid="stub-mode-badge"` 的 `STUB` 徽标（D-08③渲染分支）。 |
| 11 | `web/src/__tests__/VerdictCard.test.tsx` | 加 2 用例：mode="stub" 两个面都有徽标；无 mode 不渲染徽标。 |
| 12 | `README.md` | `### No API key? Stub LLM mode` 小节（Try it 与 Architecture 之间）：确定性 stub 说明、三重标注清单、可观察后果（verbatim 引用/拒绝更频繁/STUB 徽标+复核旗标）、`BGE_M3_URL` 仍必填 + 设 `GLM_API_KEY`/`LOCAL_API_KEY` 恢复真模型。 |

## D-10 核验

- Confidence ≤ 0.5 且 ReviewRequired=true：stubchat 全部测试断言 ✅；README 声明 ✅。
- citations 走真 grounding 校验零豁免 ✅（GroundingCheck 对 stub citation 与真模型同路径）。
- Stream 非空 ✅（专用测试 + StreamReaderFromArray 保证 ≥1 token）。
- pipeline.go 零改动 ✅；前端 `isVerdict` 未破坏 ✅（既有 697 用例 + 新用例双确认）。

## 偏离记录

- `cmd/server/main_test.go`（#7）不在 plan files_modified 内：`TestBootAssemblyOrder` 硬编码锁 `skill.NewWizard(cm,…)` 字面量，49-02 改名后必须同步，否则 cmd/server 永远红。改动为单行断言字符串，语义（kbReg 注入）不变。
- Stream 实现从初稿的 goroutine+schema.Pipe 换成 `schema.StreamReaderFromArray`：确定性输出下管道+goroutine 在 WSL 出现首片后 EOF 截断且无法稳定归因（详见上表 #3 教训行）；新实现行为契约不变（分片、非空、字节一致）且彻底稳定。

未做 git 提交；未写 .planning 状态文件。
