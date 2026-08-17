---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
fixed_at: 2026-08-18T02:10:00+08:00
review_path: .planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/08-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-08-18T02:10:00+08:00
**Source review:** .planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (critical_warning scope: CR-01, WR-01, WR-02; 8 个 IN-* Info 发现不在范围内)
- Fixed: 3
- Skipped: 0

> 说明：本次修复曾因 API quota 中断后恢复执行。CR-01 与 WR-01 在中断前已完成并提交；WR-02 由恢复后的运行完成。三个修复均在隔离 worktree（临时分支 `gsd-reviewfix/08-31553`）中提交，最终通过 fast-forward 合回 `master`。

## Fixed Issues

### CR-01: agent.go systemPrompt 仍使用已废弃的 Status 枚举，与 verdict.ValidStatuses 漂移

**Status:** fixed: requires human verification（逻辑/prompt 类修复，语法与单测均通过，但 prompt 语义对齐与 Validate 门控位置建议人工复核）
**Files modified:** `internal/agent/agent.go`, `internal/agent/agent_test.go`, `eval/frozen/compliance.lock.json`
**Commit:** f8ee4f6
**Applied fix:**
- 将 `internal/agent/agent.go` 的 `systemPrompt` 从废弃的中文 Status 枚举（合规/不合规/需补充/unanswered）对齐到 260817-vsm 迁移后的 `verdict.ValidStatuses`（ok/needs_input/refused/error），并补齐 `answer` / `compliance_status` 字段指令，与 `internal/server/pipeline.go` 的 `systemInstructionsTemplate` 保持一致。
- 在 `(*Agent).Run` 的 verdict parse 之后新增 `verdict.Validate` 门控——`runAgentPath`（internal/server/chat.go）落审计库前不再会写入枚举外 Status，D3-05 critique gating（`v.Status != verdict.DowngradeStatus`）恢复有效。
- 同步更新 `internal/agent/agent_test.go` 的 stub 输出到新枚举。
- 伴随更新 `eval/frozen/compliance.lock.json` 的 `prompt_hash`：frozen lock pin 的是 `agent.RenderSystemPrompt(manifest)` 的渲染结果（`internal/eval/frozen_test.go::TestShippedFrozenLockIsInSync`），CR-01 修改 prompt 后必须重算 hash，否则该同步测试会立即失败。已验证 `TestShippedFrozenLockIsInSync` PASS。

### WR-01: WithRunEmit(ctx, nil) 的文档语义与实现相反

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** a2cf689
**Applied fix:** 采用审查建议的选项 (a)——修正 godoc 为真实语义："Passing nil emit is a no-op; a previously injected override is preserved (context.WithValue cannot remove values)"。未引入哨兵值（选项 b），因为当前代码库没有需要"清除 override"语义的调用方，保持最小改动。

### WR-02: verdict JSON parse 失败一律误发 forced_stop 决策

**Status:** fixed
**Files modified:** `internal/agent/agent.go`
**Commit:** 53f6ca5
**Applied fix:** parse 失败分支不再无条件调用 `emitForcedStop`，改为仅在最终消息仍携带 pending tool calls（`out != nil && len(out.ToolCalls) > 0`）时发射——这是"循环在 tools 节点执行前结束（MaxStep 截断）"的结构化信号。干净收尾但输出 malformed JSON（ToolCalls 为空）的情况现在只走普通 error 路径，decision 序列不再失真。

**对审查建议的智能调整（非盲套）：** 审查建议的门控条件是 `sc.current() >= ComputeMaxStep(a.cfg.MaxHops)`，但 `stepCounter` 计的是 retrieve 工具调用次数（hop 数，MaxHops=2 时上限为 2，第 3 次 retrieve 已被 TestMaxStepBlocksThirdHop 结构性锁死），而 `ComputeMaxStep` 返回的是 pregel 步数预算（2N+1=5）——两者单位不同，该条件恒为 false，会让 forced_stop 变成死代码。实际验证 eino v0.9.14 源码（`compose/graph_run.go:250`）：硬性 MaxStep 超限会以 `compose.ErrExceedMaxSteps` 从 `Generate` 返回，走 agent.go 的 error 分支而非 parse 分支。因此采用 `len(out.ToolCalls) > 0` 作为跨 eino 版本都正确的截断信号，并在注释中记录了 v0.9.14 的实际行为。

**已知残留观察（超出本发现范围，未改动）：** 在 eino v0.9.14 上，真正的 MaxStep 截断走 error 分支，该分支按既有注释"故意不发射 forced_stop"（原意是针对 ctx cancellation）——即当前版本真实截断时 UI 反而收不到 forced_stop。这属于欠发射（under-emission），与 WR-02 的误发射（mis-emission）是不同问题，如需修复应单独立项（可在 error 分支对 `errors.Is(err, compose.ErrExceedMaxSteps)` 特判发射）。

**验证:** `go build ./...` + `go vet ./internal/agent/ ./internal/server/` 通过；`go test ./internal/agent/` 全部通过。

## Skipped Issues

None — 所有范围内发现均已修复。

---

_Fixed: 2026-08-18T02:10:00+08:00_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
