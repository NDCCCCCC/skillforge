---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
reviewed: 2026-08-17T17:04:29Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - integration/contract/contract_test.go
  - integration/contract/no_legacy_interrupt_test.go
  - internal/agent/agent.go
  - internal/agent/callbacks.go
  - internal/agent/retrieve_tool.go
  - internal/store/sqlite/checkpoint.go
  - internal/store/sqlite/checkpoint_resume_test.go
  - internal/store/sqlite/checkpoint_test.go
  - internal/store/sqlite/schema.go
  - scripts/spiked/e2e.go
  - scripts/spiked/wire.go
findings:
  critical: 1
  warning: 2
  info: 8
  total: 11
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-08-17T17:04:29Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

本次审查覆盖 Phase 8 的 11 个文件：eino v0.7.7→v0.9.14 + eino-ext openai v0.1.5→v0.1.13 升级（contract_test.go 重钉版本、agent/spiked 文件的 godoc API 引用对齐）、sqlite CheckPointStore（checkpoint.go + schema.go + checkpoint_test.go）、legacy-interrupt 仓库级 lint gate（no_legacy_interrupt_test.go）、以及 interrupt→persist→restart→resume 全链路回归测试（checkpoint_resume_test.go）。

新增代码质量总体扎实：checkpoint.go 全程 `?` 参数化（并有 SQL 注入回归测试锁定）、compile-time 接口断言（`var _ compose.CheckPointStore`）、幂等 upsert、Close/reopen 真实重启语义、resume 测试用双进程模拟 + history-keyed 脚本模型，设计均到位。版本 pin 与 go.mod 实际内容一致（eino v0.9.14 / openai v0.1.13 / hertz v0.10.6 / milvus-sdk v2.4.2 已逐一核对）。

但审查发现 **1 个 Critical**：`internal/agent/agent.go` 的 systemPrompt 仍指令 LLM 输出已废弃的中文 Status 枚举（合规/不合规/需补充/unanswered），与 260817-vsm 迁移后的 `verdict.ValidStatuses`（ok/needs_input/refused/error）直接漂移。该缺陷虽是跨 phase 迁移遗留（非本 phase 引入——本 phase 对 agent.go 的改动仅 10 行注释对齐，已核对 diff），但本 phase 的 contract test 新增了"legacy 中文 status 必须 fail Validate"的断言，使仓库内生产 prompt 与自己的契约测试形成自相矛盾；且 agent 生产路径（USE_AGENT_PATH=true）不调用 Validate 直接落审计库，影响是实打实的（详见 CR-01）。另有 2 个 Warning（`WithRunEmit` nil 语义与文档相反；parse 失败一律误发 forced_stop）和 8 个 Info。

## Critical Issues

### CR-01: agent.go systemPrompt 仍使用已废弃的 Status 枚举，与 verdict.ValidStatuses 漂移（audit 脏数据 + critique gating 失效 + 与本 phase 契约测试自相矛盾）

**File:** `internal/agent/agent.go:690-699`
**Issue:**
`systemPrompt` 指令 LLM 输出 `{"status": "合规"|"不合规"|"需补充"|"unanswered", ...}`，且不要求 `answer` / `compliance_status` 字段。而 260817-vsm 迁移后：

- `internal/verdict/validate.go:47-52` 的 `ValidStatuses` 只有 `{ok, needs_input, refused, error}`；`Validate` 对 "合规" 等一律返回 `invalid status`。
- 生产默认路径 `internal/server/pipeline.go` 的 `systemInstructionsTemplate` 已迁移到新枚举（含 `answer` + `compliance_status`），唯独 agent.go 的 prompt 未迁移——这是确凿的漂移，不是两套并存约定。

实际影响（agent 路径 `USE_AGENT_PATH=true` 为生产代码，`internal/server/chat.go` runAgentPath 已核对）：

1. **审计脏数据**：`runAgentPath` 拿到 verdict 后不经过 `verdict.Validate`，直接 `writeAuditAndTurn` 落库——audit 表 `verdict_json` 列会存入枚举外的 Status（"合规"/"unanswered"），eval 侧车和任何按 `ValidStatuses` 分桶的看板都会错分。
2. **D3-05 critique gating 静默失效**：`internal/agent/critique.go:111-113` 的门控条件是 `v.Status != verdict.DowngradeStatus`（`DowngradeStatus = "needs_input"`，见 grounding.go:192）。prompt 让 LLM 输出的"需补充"永远不等于 "needs_input"，所以"需补充类裁决必须跑 critique"的规则在 agent 路径永不触发。
3. **UI 渲染退化**：`chat.go` `conclusionText` Branch 2 优先取 `ComplianceStatus`（agent prompt 不产出该字段，恒为空），回退到 `v.Status` → 用户会看到「结论：需补充（置信度 X%）」或「结论：unanswered（…）」这类混排；而新枚举的 "refused"（PII 拒答路径）在 agent prompt 中根本不存在。
4. **仓库内自相矛盾**：本 phase 的 `integration/contract/contract_test.go:303-309`（TestContractStructuredOutputSchema 3b）明确断言 `{"合规","不合规","需补充","unanswered"}` 必须 fail `verdict.Validate`——契约测试在驱逐这些值，而同仓库的 agent.go 生产 prompt 在强制产生这些值。

**Fix:**
将 agent.go 的 systemPrompt 对齐 pipeline.go 的 `systemInstructionsTemplate`（单一来源，理想做法是把该模板下沉到共享的 prompt.go，agent.go 注释里本就标注了这个迁移目标）：

```go
const tmpl = `You are a strict compliance auditor. ...

Return JSON in this exact shape:
  {"status": "ok"|"needs_input"|"refused"|"error", "confidence": 0.0-1.0,
   "citations": [{"chunk_id":"...", "span":"...", "tier":"..."}],
   "review_required": true|false,
   "answer": "<factual queries only>",
   "compliance_status": "<compliant|non_compliant|not_applicable; omit when not a compliance review>"}

If retrieve_compliance_chunks returns 0 chunks, set status="refused", confidence=0.0, citations=[], review_required=true.`
```

同时在 `(*Agent).Run` 的 verdict parse 之后补一道 `verdict.Validate` 校验（或至少在 SUMMARY 中显式记录为何 agent 路径豁免），防止枚举再度漂移时静默落库。

## Warnings

### WR-01: WithRunEmit(ctx, nil) 的文档语义与实现相反——声称"清除 override"，实际保留 override

**File:** `internal/agent/agent.go:404-411`
**Issue:**
godoc 声明 "Passing nil emit clears the override (Run falls back to AgentConfig.Emit)"，但实现是：

```go
func WithRunEmit(ctx context.Context, emit DecisionEmitFunc) context.Context {
	if emit == nil {
		return ctx // ← 返回原 ctx，已有的 override 原样保留
	}
	return context.WithValue(ctx, emitCtxKey{}, emit)
}
```

`context.WithValue` 机制上无法"删除"一个值，所以 `WithRunEmit(ctx, nil)` 实际是 no-op。若某个中间层（router / middleware）先注入了自己的 emitter，随后 handler 试图通过 `WithRunEmit(ctx, nil)` 回退到 `AgentConfig.Emit`，会静默继续使用中间层的 emitter——对 SSE 流式路径而言是行为错误而非文档瑕疵。

**Fix:**
二选一：(a) 修正 godoc 为 "Passing nil emit is a no-op; a previously injected override is preserved (context.WithValue cannot remove values)"；(b) 若确实需要清除语义，引入哨兵值：

```go
type clearedEmit struct{}
func WithRunEmit(ctx context.Context, emit DecisionEmitFunc) context.Context {
	if emit == nil {
		return context.WithValue(ctx, emitCtxKey{}, clearedEmit{}) // Run 识别哨兵并回退 cfg.Emit
	}
	return context.WithValue(ctx, emitCtxKey{}, emit)
}
// Run 中:
if v, ok := ctx.Value(emitCtxKey{}).(DecisionEmitFunc); ok && v != nil { emit = v }
```

### WR-02: verdict JSON parse 失败一律误发 forced_stop 决策——错误归因错误，UI 误导

**File:** `internal/agent/agent.go:545-549`
**Issue:**
```go
if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &v); err != nil {
	// Emit a forced_stop decision if we hit MaxStep mid-tool-call
	emitForcedStop(emit, sc)
	...
}
```
注释限定条件是 "if we hit MaxStep mid-tool-call"，但代码对**任何** parse 失败都调用 `emitForcedStop`——包括模型在干净收尾后输出了 malformed JSON 的情况（这与步数毫无关系）。`DecisionForcedStop` 会作为 data-decision SSE chunk 推给前端，UI 据此渲染"agent 在 N hop 后放弃"，而真实原因是模型输出无法解析。可观测性数据（decision 序列）因此失真，排障时会把方向带偏。

**Fix:**
仅在步数确实触顶时才发 forced_stop；否则走普通 error 路径（或引入独立的 `DecisionParseError`）：

```go
if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &v); err != nil {
	if sc != nil && sc.current() >= ComputeMaxStep(a.cfg.MaxHops) {
		emitForcedStop(emit, sc) // 真正的 MaxStep 截断
	}
	return verdict.Verdict{}, nil, fmt.Errorf("agent run: parse verdict: %w (raw: %s)", err, truncate(raw, 200))
}
```

## Info

### IN-01: agent.go 多处注释引用不存在的 "runtime type assertion"

**File:** `internal/agent/agent.go:186-193, 340-350`
**Issue:**
NewAgent godoc（"verified at the type assertion below"）与编译期断言旁注释（"NewAgent enforces that via a runtime type assertion (intentionally not compile-time … the AgentConfig.ChatModel field accepts the wider model.ChatModel interface"）描述的机制不存在：`AgentConfig.ChatModel` 字段类型就是 `model.ToolCallingChatModel`，约束完全由编译器保证，代码中没有任何 runtime assertion，字段也不是"更宽的 model.ChatModel"。属前次重构遗留的失效注释。
**Fix:** 改为"约束由字段类型在编译期强制，无需 runtime assertion"，删去对 model.ChatModel 宽接口的描述。

### IN-02: LastRetrieved godoc 承诺 "Returns nil"，实际返回空的非-nil slice

**File:** `internal/agent/agent.go:286-317`
**Issue:**
godoc 说 "Returns nil if … the most recent Run produced zero tier-matched chunks"，但实现 `out := make([]RetrievedChunk, 0, len(hits))` 在零命中时返回长度为 0 的非-nil slice。按文档写 `if chunks == nil` 判断的调用方（Plan 03-06 eval sidecar 是既定消费方）会判断失效。
**Fix:** 二选一：零命中时显式 `return nil`，或修正 godoc 为 "Returns an empty (non-nil) slice"。

### IN-03: 版本 pin 用子串匹配，可被 pre-release 版本绕过

**File:** `integration/contract/contract_test.go:98-114`
**Issue:**
`strings.Contains(body, "github.com/cloudwego/eino v0.9.14")` 会把 `v0.9.14-rc1`（甚至 `v0.9.14-alpha`）误判为合规 pin——pre-release 的语义风险恰是本 gate 想拦的（本 phase 决策明确拒绝 v0.10.0-alpha.x）。
**Fix:** 改为行级解析（`require github.com/cloudwego/eino v0.9.14` 精确匹配），或匹配后追加 `"github.com/cloudwego/eino v0.9.14\n"` / 排除 `-` 后缀。

### IN-04: e2e.go truncate 按字节截断，中文输入会产生无效 UTF-8

**File:** `scripts/spiked/e2e.go:418-423`
**Issue:**
`s[:n]` 以 byte 截断，用于截断中文 query（n=80）与 LLM 输出时可能切断多字节 rune，trace / stderr 输出产生乱码。同仓库 `internal/agent/critique.go:227` 的同名 helper 是 rune-safe 的，两处行为不一致（注释还声称逻辑 copied）。
**Fix:** 复用 rune-safe 版本：`r := []rune(s); if len(r) <= n { return s }; return string(r[:n]) + "..."`。

### IN-05: e2e.go 两处 godoc 失真：注释段落整段重复；drainFinalContent 引用不存在的 "deferred goroutine"

**File:** `scripts/spiked/e2e.go:84-106, 381-389`
**Issue:**
(a) systemPrompt 的 godoc 中 "enforces the JSON output schema. Adapted from spike 003 …" 整段出现两次（84-98 与 95-106 逐字重复），应为粘贴事故；(b) `drainFinalContent` godoc 说 "the iterator is still drained to completion in the deferred goroutine"，但函数内没有任何 deferred goroutine——它只是继续循环到结束。
**Fix:** 删除重复段落；将 drain 描述改为实际行为（循环内继续消费直到 iterator 关闭）。

### IN-06: probeCancellation 存在假通过窗口——迭代器在 20ms 内自然完成时测得 elapsed≈0

**File:** `scripts/spiked/e2e.go:345-378`
**Issue:**
实现是 `time.Sleep(20ms)` 后 `cancel()` 再计时。若 mock 上游极快、迭代器在 sleep 期间已自然耗尽，`done` 早已 close，`cancel()` 后 select 立即返回 `elapsed≈0ms`——probe"通过"但根本没有测量取消传播。真实 GLM 端点较慢所以实际运行有效，但该 spike 也承担 contract 套件的自动化回归职责（`runSpikedWithBinary` 场景），此时测量可能退化为常数通过。
**Fix:** 在 select 前检查 `done` 是否已关闭（已关闭则报 "iterator completed before cancel; probe inconclusive"），或改用可挂起的 mock 强制请求滞留至 cancel 之后。

### IN-07: wire.go 次要健壮性问题——io.ReadAll 错误忽略、runWire 的 args 参数未使用

**File:** `scripts/spiked/wire.go:133-135, 176-177`
**Issue:**
(a) mock handler 中 `body, _ := io.ReadAll(r.Body)` 丢弃错误，读失败时会以空 body 通过 wire 断言之外的路径静默继续；(b) `runWire(ctx, args []string)` 的 `args` 从未使用（同文件 `runE2E` 的签名用 `_ []string` 标注了忽略，风格不一致）。
**Fix:** (a) `if _, err := io.ReadAll(r.Body); err != nil { http.Error(w, …, 500); return }`（仍捕获成功读到的 body）；(b) 签名改为 `runWire(ctx context.Context, _ []string) error`。

### IN-08: checkpoint_resume_test.go 对 Address 空切片直接索引，缺防护

**File:** `internal/store/sqlite/checkpoint_resume_test.go:336`
**Issue:**
`ic.Address[len(ic.Address)-1].ID` 假定 `InterruptContexts[0].Address` 非空；`require.NotEmpty(t, ev.Action.Interrupted.InterruptContexts)` 只保护外层切片。若未来 eino 版本对某类 interrupt 返回空 Address，此处会 panic 而不是给出可读的断言失败（panic 虽会被 testing 框架捕获为失败，但丢失上下文）。
**Fix:** 索引前补 `require.NotEmpty(t, ic.Address, "InterruptContext must carry a non-empty address")`。

---

_Reviewed: 2026-08-17T17:04:29Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
