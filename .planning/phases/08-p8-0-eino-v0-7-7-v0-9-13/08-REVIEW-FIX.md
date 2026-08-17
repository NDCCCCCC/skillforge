---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
fixed_at: 2026-08-17T17:52:45Z
review_path: .planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/08-REVIEW.md
iteration: 2
findings_in_scope: 11
fixed: 11
skipped: 0
status: all_fixed
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-08-17T17:52:45Z
**Source review:** .planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/08-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 11（fix_scope=all：1 Critical + 2 Warning + 8 Info）
- Fixed: 11
- Skipped: 0

> Iteration 说明：iteration 1（fix_scope=critical_warning）已完成 CR-01 / WR-01 / WR-02 并合回 master（commit f8ee4f6 / a2cf689 / 53f6ca5），本报告将其原样收录。Iteration 2 只处理剩余 8 个 Info 发现（IN-01..IN-08），在隔离 worktree（临时分支 `gsd-reviewfix/08-34046`）中逐一原子提交，最终 fast-forward 合回 `master`。
>
> 验证：本轮全部修复完成后，worktree 内 `go test ./... -count=1` 全绿；`go test -tags=contract ./integration/contract/ -count=1` 全绿（含重写后的 TestContractVersionPin）。

## Fixed Issues

### CR-01: agent.go systemPrompt 仍使用已废弃的 Status 枚举，与 verdict.ValidStatuses 漂移

**Status:** fixed: requires human verification（iteration 1 标记——逻辑/prompt 类修复，语法与单测均通过，但 prompt 语义对齐与 Validate 门控位置建议人工复核）
**Files modified:** `internal/agent/agent.go`, `internal/agent/agent_test.go`, `eval/frozen/compliance.lock.json`
**Commit:** f8ee4f6（iteration 1）
**Applied fix:**
- 将 `internal/agent/agent.go` 的 `systemPrompt` 对齐到 260817-vsm 迁移后的 `verdict.ValidStatuses`（ok/needs_input/refused/error），补齐 `answer` / `compliance_status` 字段指令。
- `(*Agent).Run` verdict parse 后新增 `verdict.Validate` 门控。
- 同步更新 agent_test.go stub 与 `eval/frozen/compliance.lock.json` 的 `prompt_hash`（frozen lock 同步测试 `TestShippedFrozenLockIsInSync` 要求）。

### WR-01: WithRunEmit(ctx, nil) 的文档语义与实现相反

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** a2cf689（iteration 1）
**Applied fix:** 采用审查建议选项 (a)——修正 godoc 为真实语义（nil 是 no-op，已注入的 override 被保留）。未引入哨兵值（选项 b），当前无调用方需要清除语义。

### WR-02: verdict JSON parse 失败一律误发 forced_stop 决策

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** 53f6ca5（iteration 1）
**Applied fix:** parse 失败分支改为仅在最终消息仍携带 pending tool calls（`len(out.ToolCalls) > 0`，即 MaxStep 截断的结构化信号）时发射 forced_stop；干净收尾的 malformed JSON 只走普通 error 路径。（对审查建议做了智能调整：原建议的 `sc.current() >= ComputeMaxStep(...)` 条件因 hop 计数与 pregel 步数单位不同恒为 false，详见 iteration 1 报告。）

### IN-01: agent.go 多处注释引用不存在的 "runtime type assertion"

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** a8121f5
**Applied fix:** 两处失效注释均改为真实机制描述——约束由 `AgentConfig.ChatModel` 字段类型（`model.ToolCallingChatModel`）在编译期强制，不存在也无需 runtime assertion：(1) NewAgent godoc 的 "verified at the type assertion below"；(2) 编译期断言旁的 NOTE 段（原称 "NewAgent enforces that via a runtime type assertion … AgentConfig.ChatModel field accepts the wider model.ChatModel interface"，两处均与事实相反）。纯注释修改，`go build ./internal/agent/` 通过。

### IN-02: LastRetrieved godoc 承诺 "Returns nil"，实际返回空的非-nil slice

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** ac3ca91
**Applied fix:** 采用 "either/or" 中的**改 godoc** 选项（而非改成 `return nil`）。理由：当前实现的 nil-vs-empty 区分有语义价值——nil 表示 "Run 从未调用"，空非-nil 表示 "跑过但零命中（refusal-by-construction）"；若改为零命中返回 nil，两种状态将无法区分。godoc 现在明确写明：nil 仅当 Run 从未调用；零命中返回空的**非-nil** slice，调用方必须用 `len() == 0` 而非 nil 检查判断零命中。无行为变更，无调用方受影响。

### IN-03: 版本 pin 用子串匹配，可被 pre-release 版本绕过

**Status:** fixed
**Files modified:** `integration/contract/contract_test.go`
**Commit:** a5c1f16
**Applied fix:** 四处 `strings.Contains(body, "<module> <version>")` 改为行级精确匹配：新增 `assertExactPin` 闭包，用 `(?m)^\s*<module>\s+<version>(\s*//[^\n]*)?\r?$` 锚定正则（允许 `// indirect` 尾注释，拒绝任何 `-rc1`/`-alpha` 等 pre-release 后缀）。四个 pin（eino v0.9.14 / hertz v0.10.6 / eino-ext openai v0.1.13 / milvus-sdk-go v2.4.2）全部走该 helper。已运行 `go test -tags=contract ./integration/contract/ -run TestContractVersionPin` 通过。

### IN-04: e2e.go truncate 按字节截断，中文输入会产生无效 UTF-8

**Status:** fixed
**Files modified:** `scripts/spiked/e2e.go`
**Commit:** 9a6abae
**Applied fix:** `truncate` 改为 rune-safe（与 `internal/agent/critique.go:227` 的同名 helper 逻辑一致）：保留 `len(s) <= n` 字节级快速路径（字节数 ≤ n 时 rune 数必然 ≤ n，直接返回原串安全），超长时按 `[]rune` 截断。godoc 补充说明 n 按 rune 计。`go test ./scripts/spiked/` 通过。

### IN-05: e2e.go 两处 godoc 失真

**Status:** fixed
**Files modified:** `scripts/spiked/e2e.go`
**Commit:** 4fffc51
**Applied fix:** (a) 删除 systemPrompt godoc 中逐字重复的粘贴事故段落（原 84-94 行是 95-106 行的半截重复，在 "Production (02-03 cmd/server)" 处断句后从头重贴），去重后保留完整单份；(b) `drainFinalContent` godoc 的 "drained to completion in the deferred goroutine" 改为真实行为——首个 error 被记录、循环继续消费至 iterator 关闭后在同一循环内返回（函数内不存在任何 deferred goroutine）。

### IN-06: probeCancellation 存在假通过窗口

**Status:** fixed: requires human verification（测量探针的控制流变更——逻辑类修复，建议人工确认 inconclusive 失败模式符合预期）
**Files modified:** `scripts/spiked/e2e.go`
**Commit:** 1f7f915
**Applied fix:** 采用审查建议的 "or" 分支中的**前置检查**方案（而非引入可挂起 mock——后者需要重构 mock 层，超出 Info 级修复范围）：20ms settle 后、`cancel()` 前 select 检查 `done` 是否已关闭；若迭代器已自然耗尽则返回 "iterator completed before cancel; probe inconclusive" 错误，使 probe 响亮失败而非以 elapsed≈0ms 假通过。选择理由：probe 当前只对真实 GLM 端点（慢上游）运行，契约套件以 `GLM_API_KEY=unhappy` 强制 e2e fail-fast、并不真正跑 probe，因此前置检查不会让现有绿色路径变红。godoc 同步补充该行为说明。`go build` + `go vet` + `go test ./scripts/spiked/` 通过。

### IN-07: wire.go 次要健壮性问题

**Status:** fixed
**Files modified:** `scripts/spiked/wire.go`
**Commit:** b36dd21
**Applied fix:** (a) mock handler 的 `body, _ := io.ReadAll(r.Body)` 改为检查错误——读失败时 `http.Error(w, …, 500)` 并 return，不再以空 body 静默继续；(b) `runWire(ctx, args []string)` 签名改为 `runWire(ctx context.Context, _ []string)`，与同文件 `runE2E` 的忽略标注风格一致。`go vet` + `go test ./scripts/spiked/` 通过。

### IN-08: checkpoint_resume_test.go 对 Address 空切片直接索引，缺防护

**Status:** fixed
**Files modified:** `internal/store/sqlite/checkpoint_resume_test.go`
**Commit:** 18fcc47
**Applied fix:** `ic.Address[len(ic.Address)-1].ID` 索引前补 `require.NotEmpty(t, ic.Address, "InterruptContext must carry a non-empty address …")`——未来 eino 版本若返回空 Address，将得到可读断言失败而非 panic。`go vet` + `go test ./internal/store/sqlite/` 通过。

## Skipped Issues

None — 所有 11 个发现（跨两轮 iteration）均已修复。

---

_Fixed: 2026-08-17T17:52:45Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
