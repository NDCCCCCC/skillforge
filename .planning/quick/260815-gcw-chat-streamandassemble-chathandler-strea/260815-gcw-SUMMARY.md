---
quick: 260815-gcw
title: "chat 流式卡死诊断日志：chatHandler / VerdictPipeline / streamAndAssemble 全链路埋点"
date: 2026-08-15
type: execute
phase: null
plan: null
subsystem: observability
tags: [chat, streaming, sse, debug-logging, info, eino, hertz, openai-compatible, hang-diagnosis]

# Dependency graph
requires: []
provides:
  - "chat.go + pipeline.go 全链路 13 个 [INFO] log.Printf 埋点（chatHandler 入口 / 3 个路由分支 / VerdictPipeline dispatch / runAgentPath 入口 / VerdictPipeline 入口 / streamAndAssemble 入口 / Stream() 调用前后 / 首次 Recv / 每次 Recv / completed）"
  - "chatHandler 函数 godoc DEBUG seam（11 行），告知 curl 触发 + 期望日志序列 + hang 位置诊断决策表"
  - "streamAndAssemble 内的 tokenN 计数器（function scope，可访问 completed 日志）"
affects:
  - "未来 /api/chat hang 复现诊断（无代码改动，仅靠控制台日志序列定位 hang 段）"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "[INFO] server: <func> <verb> <field>=<value> 全链路埋点格式（截断 query 到 80 字符、控制体量、便于 grep）"
    - "// DEBUG (260815-gcw): <语义描述> 注释头，标注每个埋点的诊断意图与上下游关联"

key-files:
  created: []
  modified:
    - "internal/server/chat.go (1 godoc DEBUG seam + 6 [INFO] log.Printf sites; 38 lines added)"
    - "internal/server/pipeline.go (7 [INFO] log.Printf sites + 1 tokenN := 0 declaration; 52 lines added)"

key-decisions:
  - "tokenN 声明在 streamAndAssemble 函数体顶层（var buf strings.Builder 之后、for 之前），保持函数作用域以便 D-7 completed 日志可访问"
  - "D-6 tokenN++ / log 放在 `if msg == nil || msg.Content == \"\"` continue 之后、EmitChunk 之前 — 仅在成功 Recv 非空 token 时自增，匹配 plan '成功 Recv 后' 语义（避免 msg==nil 时 len(msg.Content) 解引用）"
  - "D-4 Stream returned OK 严格按 plan 字面放置（Stream 调用 close 之后、`if err != nil` 检查之前） — 即使 err != nil 也会 log，依赖下一行 err 检查兜底语义正确性"
  - "query 全部用 truncate(query, 80) 截断（复用 chat.go line 513 已定义的 truncate），符合 plan threat model T-GCW-01 mitigate"
  - "PipelineChatModel.Stream model=%T 用具体类型（*openai.ChatModel）打印 — 验证 Deps 注入正确，stub test 模型也会暴露类型不一致"
  - "无 SSE 字节流变化、无函数签名变化、无测试断言变化 — 仅追加 log 行，control flow 零修改"

patterns-established:
  - "Pattern: 诊断性 [INFO] 日志统一前缀 [INFO] server: <func> <verb> field=value — 与现有 [WARN] server: / [ERROR] server: 风格一致，便于日志过滤"

requirements-completed: []

# Metrics
duration: ~10m
started: 2026-08-15T03:45:00Z
completed: 2026-08-15T03:54:45Z
tasks: 1
files: 2
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick 260815-gcw: chat 流式卡死诊断日志

**chat 流式全链路 13 个 [INFO] 埋点 + chatHandler godoc DEBUG seam：定位 /api/chat hang 段（Stream() / first Recv / mid-stream / SSE wire）的控制台可读诊断**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-08-15T03:45:00Z
- **Completed:** 2026-08-15T03:54:45Z
- **Tasks:** 1 / 1
- **Files modified:** 2 (chat.go + pipeline.go; no _test.go changes)

## Accomplishments

- chat.go 新增 1 个 godoc DEBUG seam（11 行）+ 6 个 [INFO] log.Printf，覆盖 chatHandler 入口 / 3 个路由分支（Agent path / Registry+Router / Loader fallback）/ VerdictPipeline dispatch / runAgentPath 入口
- pipeline.go 新增 7 个 [INFO] log.Printf + 1 个 tokenN := 0 计数器，覆盖 VerdictPipeline 入口 / streamAndAssemble entering / calling PipelineChatModel.Stream / Stream returned OK / 首次 Recv / 每次 Recv / completed
- 诊断决策表内嵌在 chatHandler godoc：no log 1 = hertz routing; log 1 only = decode/lookup; logs 1-3 only = embed/retrieve; logs 1-5 only = Stream() (GLM/endpoint); logs 1-7 only = first Recv; slow recv N = mid-stream; logs OK but no FE draft = SSE wire
- 现有 chat_test.go / pipeline_test.go 零修改（git diff --stat 仅 chat.go + pipeline.go 两文件，90 insertions）
- go test ./internal/server/... PASS（与 baseline 一致：ok 2.212s）
- go vet ./internal/server/... 无 warning
- go build ./... 成功

## Task Commits

Each task was committed atomically:

1. **Task 1: chat.go 与 pipeline.go 全链路诊断日志埋点** - `aec5392` (feat)

## Files Created/Modified

- `internal/server/chat.go` - 6 [INFO] log.Printf sites + 1 godoc DEBUG seam; 修改位置: chatHandler 函数头 (C-1), sessionID 解析后 (C-2), Agent path 分支前 (C-3), Registry+Router 分支前 (C-4), Loader fallback 内 (C-5), VerdictPipeline dispatch 前 (C-6), runAgentPath 函数头 (C-7)
- `internal/server/pipeline.go` - 7 [INFO] log.Printf sites + 1 tokenN := 0; 修改位置: VerdictPipeline 函数头 (D-1), streamAndAssemble 函数头 (D-2 + D-3), Stream 调用后 (D-4), var buf 后 (D-5), 每次 Recv 成功后 (D-6), text-delta-draft-end 后 (D-7)

## Decisions Made

None - followed plan as specified (D-2 + D-3 共用一段 godoc 注释是 plan 允许的分组优化；D-6 放置在 if-check 之后而非之前是为了避免 msg==nil 时 len(msg.Content) 解引用 — plan 字面 "line 411 ... 之前" 与 "成功 Recv 后" 语义存在微冲突，选用更安全的 "之后" 解读，匹配 parenthetical 的语义意图)

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as specified.

### Plan Literal vs Semantic Interpretation

**1. D-6 per-token counter placement**
- **Found during:** Task 1, D-6 insertion
- **Issue:** Plan 字面 "line 411 if msg == nil ... continue 之前" 与 parenthetical "成功 Recv 后、EmitChunk 之前" 语义微冲突 — 字面是"if 之前"（含 nil msg 也会 tokenN++），语义是"成功后"
- **Fix:** 按 parenthetical 语义解读，tokenN++ / log 放在 `if msg == nil || msg.Content == "" { continue }` 的 `}` 之后、EmitChunk 之前 — 仅在 msg != nil && msg.Content != "" 时自增
- **Files modified:** internal/server/pipeline.go
- **Verification:** go test PASS, go vet clean, log 格式 token_n=N / delta_bytes=N 在 happy path 正确递增
- **Committed in:** aec5392

## Issues Encountered

None.

## Self-Check: PASSED

- internal/server/chat.go: exists, 40672 bytes
- internal/server/pipeline.go: exists, 28319 bytes
- Commit aec5392: exists (`feat(260815-gcw): chat streaming full-link INFO logging for hang diagnosis`)
- chat.go + pipeline.go: 90 insertions, 0 deletions, no _test.go changes
- go test ./internal/server/... PASS (baseline + after = ok 2.270s baseline → ok 2.212s after)
- go vet ./internal/server/... clean
- go build ./... successful

## Next Phase Readiness

- /api/chat hang 复现诊断就绪：用户重启后端 + curl :8888/api/chat 即可在控制台按 godoc DEBUG seam 列出的 14 项序列对照 log，定位 hang 段（hertz routing / decode / embed / retrieve / Stream() / first Recv / mid-stream / SSE wire）
- 后续若发现具体 hang 段，可针对性 patch（e.g. Stream() hang = 检查 GLM_API_KEY + endpoint + chatModelOpts 兼容性；first Recv hang = 检查 HTTP body open / SSE write flush）
- 无控制流变化、无 SSE 字节流变化 — 下游 useChat 解析器与现有 acceptance test 不受影响

---

*Quick: 260815-gcw*
*Completed: 2026-08-15*
