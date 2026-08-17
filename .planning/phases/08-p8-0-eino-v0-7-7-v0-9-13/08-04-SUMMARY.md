---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
plan: 04
subsystem: persistence + agent-runtime integration (SC-5 real-interrupt round-trip test)
tags: [checkpoint-store, adk-runner, interrupt-resume, hitl-foundation, hermetic-test, schema-registername, sc-5]
requires:
  - "SQLiteStore.Get/Set satisfying compose.CheckPointStore (Phase 8 / Plan 08-02, commits 9a557b1 + e6a580a)"
  - "TestNoLegacyInterruptAPI ban gate — this plan's test must not trip it (Phase 8 / Plan 08-03, commit fea6114)"
  - "eino v0.9.14 dependency base (Phase 8 / Plan 08-01, commit 36845c0)"
provides:
  - "SC-5 real-interrupt round-trip locked by TestInterruptResumeRoundTrip: tool-layer StatefulInterrupt → adk Runner Action.Interrupted (InterruptContexts[0].ID) → checkpoint durably in sqlite → instance-level 'process restart' (Close + same-DSN reopen + NEW agent + NEW runner) → ResumeWithParams(Targets) → scripted final answer with the injected decision visible in the execution flow"
  - "Registration-surface proof: custom state *approvalDecision registered via schema.RegisterName[T](stableName) survives the serialize → sqlite → deserialize round trip with semantics intact (original pending state recovered via GetInterruptState, injected decision received via GetResumeContext)"
  - "Phase 11 direct technical basis: write-approval tool shape (StatefulInterrupt carrying approval state + resume-target branching) proven end-to-end on the real store — Pitfall 4 detonation surface finally exercised"
affects:
  - "Phase 11 HITL（写工具审批——resume 定位、状态注册、跨重启存活的全部 API 面已被本测试预演）"
  - "Phase 10/12/13 interrupt 消费面（官方事件链的仓内首个工作样例）"
tech-stack:
  added: []
  patterns:
    - "两进程 stub 剧本的 history-keyed 脚本：scripted ChatModel 按「对话历史是否已含 Tool 消息」分支而非实例本地轮次计数——进程 2 的新实例拿到恢复的历史才会走终答分支（计数器重置的天然解法）"
    - "工具层中断三分支形状（照官方 myStatefulTool + Phase 11 写工具镜像）：未中断→StatefulInterrupt 携带状态；已恢复+是目标→消费 GetResumeContext 数据；已恢复+非目标→re-interrupt 保状态（leaf 契约）"
key-files:
  created:
    - internal/store/sqlite/checkpoint_resume_test.go
  modified: []
decisions:
  - "D8-06: SC-5「进程重启」= 实例级重建（新 DB handle + 新 ChatModelAgent + 新 Runner，同一 DSN 文件），stub 剧本按对话历史分支而非轮次计数；中断 ID 取 InterruptContexts[0] 并断言其地址末段指向审批工具（root-cause 叶子）"
metrics:
  duration: 5m
  completed: 2026-08-17
---

# Phase 8 Plan 04: SC-5 真实中断运行往返测试 Summary

**One-liner:** hermetic 全链路测试 `TestInterruptResumeRoundTrip`（0.17s）：stub ChatModel 剧本 → 工具层 `tool.StatefulInterrupt` 携带 `*approvalDecision`（经 `schema.RegisterName` 双注册）→ adk Runner `Action.Interrupted` 捕获 `InterruptContexts[0].ID` → checkpoint 落入 08-02 SQLiteStore（Get existed=true 实证）→ Close + 同 DSN 重开 + 全新 Agent/Runner「进程重启」→ `ResumeWithParams(Targets)` 注入决策 → 恢复到中断点继续执行，终答文本携带注入的 Reason 且重启侧工具同时取回原始 pending 状态与注入决策；phase hermetic 五门全绿收口。

## What Was Built

### Task 1: TestInterruptResumeRoundTrip（commit `3c42c20`，440 行，仅此一文件）

`internal/store/sqlite/checkpoint_resume_test.go`（`package sqlite_test` 黑盒，license header 1-15 行照 checkpoint_test.go 逐字，hermetic——唯一 I/O 是 t.TempDir() 内的 sqlite 文件）：

**(1) 自定义状态类型 + 注册面**：`type approvalDecision struct{ Approved bool; Reason string }`，同文件 `func init()` 内 `schema.RegisterName[*approvalDecision]("skillforge_test_approval_decision")` —— 双注册（gob + v0.9 InternalSerializer registry，schema/serialization.go:83-90），**零裸 gob 注册**（acceptance grep=0 实证）；"skillforge_" 前缀命名空间为 Phase 11 真实审批状态预留。

**(2) stub ChatModel**（multiTurnStubChatModel 形状：`var _ model.ToolCallingChatModel` 断言 + mutex 计数 + Stream not-implemented + WithTools 返回自身）：**剧本按对话历史分支**——历史无 Tool 消息 → 返回对 `request_write_approval` 的 ToolCall（中断触发）；历史含 Tool 消息 → 返回终答 `审批决策已执行:` + 末条 Tool 消息内容（echo 设计使「resume 注入的决策到达执行流」在终答文本里可断言）。history-keyed 是两进程设计的必然：进程 2 的新实例计数器归零但拿到恢复的历史，计数剧本会重发 ToolCall 死循环。

**(3) 中断工具**（照官方 `myStatefulTool` + Phase 11 写工具镜像）：未中断 → `tool.StatefulInterrupt(ctx, info, &approvalDecision{false, "awaiting human decision"})`；已恢复 → `GetInterruptState[*approvalDecision]` 取回持久化状态 + `GetResumeContext[*approvalDecision]` 取注入决策，输出 `approved=%v; reason=%s`；非 resume 目标 → re-interrupt（leaf 契约，adk/runner.go:137-147）。mutex 保护全部字段 + `snapshot()` 访问器。

**(4) 全链路**：进程 1（`sqlite.New(file DSN)` → `adk.NewChatModelAgent(ToolsConfig 挂工具)` → `adk.NewRunner(CheckPointStore: s1)` → `Query(query, WithCheckPointID("test-cp-roundtrip"))` → 照 drainFinalContent 形状 drain + `ev.Action.Interrupted` 分支捕获 ID 并断言地址末段=审批工具）→ drain 后 `s1.Get` existed=true + payload 非空（持久化实证）→ `s1.Close()` → **进程重启**（同 DSN `sqlite.New` + 全新 model/tool/agent/runner 四实例）→ `ResumeWithParams(Targets{interruptID: &approvalDecision{true, "human approved via resume"}})` → drain 断言：终答含 `approved=true` + 注入 Reason；工具 2 侧 `restoredState = 原始 pending 状态`（Approved=false, "awaiting human decision"——序列化往返语义完好）且 `resumedDecision = 注入决策`；`firstPass=0`（无新中断）、`model2.callCount>=1`、进程 1 工具 `resumed=nil`（隔离实证）。

事件链全部按 module cache 实读核验：adk/runner.go:310-341（中断事件构造 + checkpoint 持久化）、adk/interrupt.go:219-242（resume 经 store.Get 回读 + gob 解码 + PopulateInterruptState）、components/tool/interrupt.go:44-185（Interrupt/StatefulInterrupt/GetInterruptState/GetResumeContext 精确签名）、官方先例 adk/interrupt_test.go:1145 TestChatModelAgentToolInterrupt + :1547 parallel resume + myStatefulTool 三分支。

### Task 2: phase hermetic 五门全绿（验证性任务，零提交——无回归 → 无归因 fix）

| 门 | 命令 | 结果 |
|----|------|------|
| 1 | `go build ./... && go vet ./...` | exit 0（gofmt -l 空） |
| 2 | `go test ./... -count=1` | 全绿（agent / bge / eval / knowledge / llm / milvus / mongo / server / skill / store / **store/sqlite 0.835s 含新测试** / verdict / wire / eval_snapshot / spiked / cmd/eval 全 ok） |
| 3 | `go test ./integration/contract -tags=contract -count=1 -timeout 10s -v` | 8 项全 PASS（3.300s）——**TestContractVersionPin 0.00s**（新 pin）+ **TestNoLegacyInterruptAPI 0.03s**（禁令门双绿）+ WireFormat 2.74s / MilvusFilter / StructuredOutputSchema / CitationGrounding / RunsUnder10s 0.08s / FullSpikeSuite 0.28s |
| 4 | `go test ./internal/store/sqlite -run "TestCheckpoint\|TestInterruptResumeRoundTrip" -v -count=1` | 7 面全 PASS（0.627s）——六面 store 语义 + **TestInterruptResumeRoundTrip 0.17s** |
| 5 | `go test ./internal/server -run TestCancelPropagationMidTool -v -count=1` | `--- PASS (0.07s)`，warm-up-to-return **50.07ms** <200ms（SC-8 计时断言未放松） |

## Must-Haves Truths 验证

| Truth | 状态 | 证据 |
|-------|------|------|
| 一次 Run 中断：stub 剧本下 adk Runner 发出 Action.Interrupted，ID 从 InterruptContexts[0].ID 捕获 | ✅ | drain 捕获 + `require.NotEmpty(interruptID)` + 地址末段断言指向 `request_write_approval` |
| 中断后 checkpoint bytes 已持久化到 sqlite（store Get existed=true） | ✅ | drain 后 `s1.Get("test-cp-roundtrip")` → existed=true、payload 非空（重启前、同句柄实证） |
| 「进程重启」后 ResumeWithParams(Targets) 恢复执行到脚本化终答 | ✅ | s1.Close → 同 DSN 重开 → 新 Agent/Runner → ResumeWithParams → 终答 `审批决策已执行:approved=true; reason=human approved via resume` |
| 自定义状态类型经 schema.RegisterName[T](name) 注册，中断往返后状态语义完好 | ✅ | init() 注册（grep=3 / 裸 gob grep=0）；重启侧工具取回原始 pending 状态（Approved=false + pendingReason 逐字相等）+ 注入决策（Approved=true + resumeReason）双断言 |

## Verification（plan `<verification>` 三项）

| # | 命令 | 结果 |
|---|------|------|
| 1 | `go test ./internal/store/sqlite -run "TestCheckpoint\|TestInterruptResumeRoundTrip" -v -count=1` | 全绿（Task 2 门 4） |
| 2 | 五门 phase 级命令全绿 | 全部 exit 0（Task 2 表） |
| 3 | 禁令门 + VersionPin 在 contract 套件内同跑绿 | 门 3：TestNoLegacyInterruptAPI 0.03s + TestContractVersionPin 0.00s 双 PASS |

### Acceptance grep 门（Task 1 全过）

`schema.RegisterName`=3（≥1）/ `gob.RegisterName`=0 / `ResumeWithParams`=5（≥1）/ `WithCheckPointID`=1（≥1）/ `adk.NewRunner`=2（≥2，两独立构造）/ `sqlite.New` 实调用 2 次（同 DSN）/ `react.NewAgent`=0 / `go vet ./internal/store/sqlite` exit 0 / 禁令门单跑 PASS / 文件 440 行（≥100）/ 提交 `git show --stat` 仅含本文件。

## Deviations from Plan

**1. [Rule 3 - 阻塞问题] `-race` 加跑在本机不可用（环境限制，非代码问题）**
- **Found during:** Task 1 验证后附加 diligence 加跑
- **Issue:** `go test -race` 需要 cgo/gcc，本 Windows 机器无 gcc（`cgo: C compiler "gcc" not found`）。
- **Fix:** 不适用——plan 的 verify 命令本就不含 `-race`；竞态安全由构造保证（工具全部字段 mutex 保护 + `snapshot()` 访问器 + 迭代器 drain 在 channel close 之后读取，happens-before 成立）。
- **Files modified:** 无。
- **Commit:** 无（无代码变更）。

其余：plan executed exactly as written——工具层中断 API 走 `components/tool/interrupt.go` 实读签名（无需 fallback 到「最近 adk 原生中断面」的逃生条款）；ChatModelAgent + 工具层 Interrupt 在 v0.9.14 接线无障碍（官方 adk/interrupt_test.go:1145/1547 同形状先例背书）。

## Auth Gates

None — 纯 hermetic（stub model + t.TempDir sqlite），无 live stack 依赖（live 重锁属 08-05 human-verify checkpoint）。

## Known Stubs

None — 无 UI / 无占位数据。stub ChatModel 与 stub 工具是**测试替身**（hermetic 设计要求），非产品代码 stub。

## Threat Flags

None — 无新增安全面。威胁登记表 T-8-09（mitigate：schema.RegisterName 双注册 + grep 门禁裸 gob + round-trip 测试即延迟引爆检出器）与 T-8-10（mitigate：三重建「重启」+ store 持久化/终答/注入值三重证据链）全部按 plan 落实；T-8-SC（accept）维持——零新包（标准库 + 既有 eino/testify）。

## Requirements Mark-Complete 决策

plan frontmatter `requirements: [PLAT-05]` **不 mark-complete**：PLAT-05 覆盖整个 Phase 8 CheckPointStore 面（store 骨架 ✅ 08-02 + 注册面 ✅ + 真实中断往返 ✅ 本 plan + live 重锁 = 08-05 human-verify 未完成）。与 08-03（PLAT-04）处置一致——REQUIREMENTS 勾选留待 08-05 收口。

## Self-Check: PASSED

- 文件存在性：internal/store/sqlite/checkpoint_resume_test.go — FOUND（git tracked @ 3c42c20，440 行）
- 提交存在性：`3c42c20` FOUND（git log 验证）；Task 2 验证性任务零提交（plan 明确「无代码文件；产出 = SUMMARY 门证据」）
- 五门输出留存于本文件「Task 2」表
