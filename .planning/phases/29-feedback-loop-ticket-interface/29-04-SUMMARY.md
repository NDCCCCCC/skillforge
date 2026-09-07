---
phase: 29-feedback-loop-ticket-interface
plan: 04
subsystem: api
tags: [sse, wire-contract, feedback-loop, turn-id, history-restore, d15-paired-commit]

# Dependency graph
requires:
  - phase: 29-01
    provides: AppendTurn (int64, error) + SessionMessage.ID read-back + writeAuditAndTurn (int64, error) + the 12 annotated emission windows
  - phase: 29-02
    provides: Store.VotedVerdictsBySession (map[int64]string) + PutFeedback UNIQUE(session_id, turn_id)
provides:
  - data-turn wire frame (24th ValidTypes entry) + wire.TurnPayload{turnId} + TS mirror TurnPayload with TS-only votedVerdict? field (D15 paired commit 271a5bb)
  - emitTurnID(c, turnID) helper — turnID<=0 emits zero bytes (no fabricated identity), emit errors WARN-only
  - 11 live-stream emission points wired (pipeline 6 + chat 3 + runner 2) with write-first reorder on refusal/error paths
  - history endpoint embeds turn_id + voted_verdict (D-29-03 one-request restore echo)
  - FE chat-restore synthesizes data-turn parts from history rows (same consumption point as live stream, 29-07 renders from it)
affects: [29-05 (feedback vote endpoint consumes the emitted turnId), 29-07 (FE FeedbackRow consumes data-turn parts), 29-09 (stage-3 ValidTypes double-guard)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "write-first reorder: persist audit + turn BEFORE any data-turn/terminal emit so a failed emit can never lose the audit row (GEVL-03) or broadcast an unassigned id"
    - "absence-lock tests: RED-passing tests that pin zero-emission contracts (turnID==0, agent-error branch) so GREEN cannot over-emit"

key-files:
  created:
    - internal/server/feedback_wire.go
    - internal/server/feedback_wire_test.go
  modified:
    - internal/wire/chunk.go
    - internal/wire/chunk_test.go
    - web/src/types/wire-types.ts
    - internal/server/pipeline.go
    - internal/server/chat.go
    - internal/server/runner_setup.go
    - internal/server/pipeline_history_test.go
    - internal/server/chat_session_test.go
    - internal/server/handlers_session_history.go
    - internal/server/handlers_session_history_test.go
    - web/src/lib/chat-restore.ts
    - web/src/lib/chat-restore.test.ts

key-decisions:
  - "D-29-04-A: data-turn emitted AFTER counter-evidence/redline annotations and BEFORE text-end on the verdict path — the plan's <behavior> matrix (authoritative for tests) placed it after the annotation frames; the <action>'s 'immediately after writeAuditAndTurn' wording was reconciled in favor of the behavior contract, satisfying both 'turn 写完 → 发帧' and '先于 finish'"
  - "D-29-04-B: verdict-path early return on emitErr skips data-turn — emitErr means the SSE stream is already dead, so a vote key could not reach the client anyway; audit row still exists (write-first)"
  - "D-29-04-C: agent-error branch (chat.go:449) keeps its original audit-after-terminal order and emits NO data-turn — no finish frame means useChat never registers the assistant message, so there is no FeedbackRow to vote on; a turn id would fabricate a vote target (T-29-06)"
  - "D-29-04-D: TurnPayload.votedVerdict exists on the TS mirror ONLY (optional) — filled exclusively by the history synthesis, never marshalled by any Go emitter; declared in the union payload so the chat-restore literal passes TS excess-property checking"
  - "D-29-04-E: runner drain-error branch is reorder-locked by the ==11 call-site count rather than a frame-order test — that branch only fires after client disconnect/ctx cancel, so its frame order is structurally unobservable over HTTP"

patterns-established:
  - "Pattern: write-first emission — every response shape persists (audit + turn) before emitting identity or terminal frames"
  - "Pattern: counted-store test fake (embeds store.Store, counts + scripts one method) for exactly-once batch-read locks"

requirements-completed: []  # FBK-01/04 remain Pending at phase level — vote endpoint (29-05) + FE feedback UI (29-07) complete them

# Metrics
duration: 32min
completed: 2026-09-08
---

# Phase 29 Plan 04: data-turn wire 帧 + history 双字段回显 Summary

**FBK-01 的 turn id 双暴露路径打通：`data-turn` 成为第 24 个 wire 类型并在 11 个发射点于 finish 前发出（覆盖全部 5 条拒绝形态，写入先于发射），history 恢复一次请求同时带回 `turn_id` + `voted_verdict`，Go/TS 两侧 D15 配对提交。**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-07T19:53:41Z
- **Completed:** 2026-09-07T20:22:54Z
- **Tasks:** 3/3 (Task 1 auto + Task 2 TDD + Task 3 TDD)
- **Files modified:** 14 (2 created, 12 modified)

## Accomplishments

- **Task 1 (commit 271a5bb, D15 paired):** `internal/wire/chunk.go` ValidTypes 注册 `"data-turn"`（计数注释 23→24，snake_case `data_turn` 持续被拒 T-1-11）+ `wire.TurnPayload{turnId}`（与 DecisionPayload 同区）；`web/src/types/wire-types.ts` 同 commit 增加 `TurnPayload {turnId; votedVerdict?}` 与 `| { type: 'data-turn'; id?: string; data: TurnPayload }` union 成员；新文件 `internal/server/feedback_wire.go` 的 `emitTurnID(c, turnID)`——turnID<=0 零字节输出（冲突/降级绝不编造 id，T-29-06），marshal/emit 错误上抛给调用方记 WARN（绝不 500，镜像终止帧的 best-effort 姿态）。
- **Task 2 (commits 4471b88 RED + affeaa1 GREEN):** 11 个发射点接入并重排——pipeline.go 6（5 条拒绝路径改为 `writeAuditAndTurn → emitTurnID → return emitRefusal(...)`，verdict 路径捕获 turnID 后在 counter-evidence/redline 之后、text-end 之前发射）+ chat.go 3（pipeline-error 分支、agent 拒绝、agent verdict 全部 write-first）+ runner_setup.go 2（drain-error 与 drain-complete）。chat.go:449 agent-error 分支保持原序不发帧并就地注释（无 finish → useChat 不注册消息 → 无 FeedbackRow）。`grep -c emitTurnID == 11` 精确锁定。帧序 hermetic 测试：verdict 流 data-turn 位于 counter_evidence 注释帧之后、text-end/finish 之前且 turnId == GetSession 落库 assistant 行 id；4 条可触达拒绝形状 + 空 tiers out_of_scope（直调 VerdictPipeline 夹具）断言 data-turn 先于 emitRefusal 的 text-start；turnID==0（nil Store）零 data-turn 帧且其余帧序不变。
- **Task 3 (commits 2f5e032 RED + 5ba4cfc GREEN):** `historyEntry` 增加 `turn_id`/`voted_verdict`（assistant-only + omitempty）；handler 在 GetSession 之后恰一次调用 `VotedVerdictsBySession`（counting fake 锁 ==1），读失败降级为无 voted_verdict + WARN、绝不 500；`renderRow(m, votedVerdict)` 四个 assistant 分支（refusal / verdict-JSON / RenderConclusion 失败 / 纯文本）均填 `TurnID=m.ID`，user 分支不填。FE `chat-restore.ts`：`SessionHistoryMessage` 增加 snake_case `turn_id?`/`voted_verdict?`，assistant 行按 truthy turn_id 合成 `{ type: 'data-turn', data: { turnId, votedVerdict? } }` part（与 live 流同一消费点，29-07 只认这一种形状）。

## Verification (plan-level)

- `wsl go build ./...` OK；`go test ./internal/wire/ ./internal/server/... -count=1` 全绿（wire + server 13.6s + classifier + rewrite，既有 GEVL-03 / 帧序 / 审计测试零回归）
- `npx vitest run wire-contract chat-restore` → 15/15 PASS；`npx tsc --noEmit` 干净（D15 `_driftCheck` 零改动仍有效）
- grep 证据：`chunk.go:480 "data-turn": true`；`wire-types.ts:454 data-turn` union 成员；`emitTurnID(` 恰 11 处（pipeline 6 / chat 3 / runner 2，feedback_wire.go 定义处不计）；handler 内 `.VotedVerdictsBySession(` 恰 1 处调用

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] internal/wire/chunk_test.go 4 处 ValidTypes 计数锁 23→24**
- **Found during:** Task 1
- **Issue:** `TestValidTypes_Twenty` + `TestValidTypesUnchangedAfterFanout/BudgetGate/DataRefWire` 断言 `len(ValidTypes)==23`，注册 data-turn 后 Task 1 的 verify 命令（`-run TestValidTypes...`）必炸
- **Fix:** 四处断言升至 24 并注明 Plan 29-04 增量；`TestValidTypes_Twenty` 补 data-turn 注册断言
- **Files modified:** internal/wire/chunk_test.go
- **Commit:** 271a5bb

**2. [Rule 1 - Bug] Task 3 Go 测试断言类型错配（int64 vs float64）**
- **Found during:** Task 3 GREEN
- **Issue:** 首版 GREEN 测试把 `historyEntry.TurnID`（int64）与 `float64(turnN)` 比较，wire 输出实际正确（raw JSON 已含 `"turn_id":4`）但断言全炸
- **Fix:** 改为 int64 字面量比较；键缺席断言保留 raw-JSON map 检查（真正锁 omitempty）
- **Files modified:** internal/server/handlers_session_history_test.go
- **Commit:** 5ba4cfc

### Plan-internal conflicts resolved

- Task 2 `<behavior>`（data-turn 位于 counter-evidence/redline 之后）与 `<action>`（writeAuditAndTurn 紧接 emitTurnID）冲突 → 按 behavior 契约实现（见 key-decisions D-29-04-A），turn 写完 → 发帧 → 终止帧的严格时序不受影响

### Coverage notes (honest, not deviations)

- runner drain-error 分支无 HTTP 层帧序测试：该分支仅在客户端断开/ctx 取消后触发，帧序对任何客户端都不可观测——由 ==11 调用点计数 + write-first 重排锁定（D-29-04-E，测试文件头注已落档）
- 两条"缺席锁"测试（turnID==0 零发射、agent-error 分支零发射）在 RED 阶段即 PASS——它们守护的是 GREEN 不许过度发射的反向契约

## TDD Gate Compliance

- Task 2: RED(4471b88, 5 个行为测试断言失败) → GREEN(affeaa1) 门齐全
- Task 3: RED(2f5e032; Go 侧为编译形状 RED——historyEntry 尚无 TurnID 字段，FE 侧为断言形状 RED) → GREEN(5ba4cfc) 门齐全
- Task 1 为 type="auto"（非 TDD 任务），随同提交 feedback_wire_test.go 即写即绿

## Known Stubs

None — 本 plan 无 UI 消费面（29-07 落 FeedbackRow 渲染）；data-turn part 的 FE 合成逻辑已有测试锁定。

## Self-Check: PASSED

- FOUND: internal/server/feedback_wire.go / feedback_wire_test.go（Task 1 创建）
- FOUND: internal/wire/chunk.go:480 `"data-turn": true`；web/src/types/wire-types.ts:454 union 成员
- FOUND: 全部 6 个 commit（271a5bb / 4471b88 / affeaa1 / 2f5e032 / 5ba4cfc + 本文档 docs commit）
- FOUND: emitTurnID 11 处调用点 == 验收的精确等式；VotedVerdictsBySession 恰 1 处调用
