---
quick: 260815-gcw
title: "chat 流式卡死诊断日志：chatHandler / VerdictPipeline / streamAndAssemble 全链路埋点"
date: 2026-08-15
type: execute
depends_on: []
files_modified:
  - internal/server/chat.go
  - internal/server/pipeline.go
autonomous: true
requirements: []
user_setup: []

must_haves:
  truths:
    - "重启后端 + curl /api/chat 后控制台可见 chatHandler 入口日志（query / skill / sessionID）"
    - "控制台可见路由分支日志（runAgentPath vs VerdictPipeline）"
    - "控制台可见 VerdictPipeline 入口日志（query / skill-name / sessionID）"
    - "控制台可见 streamAndAssemble 入口日志（prompt bytes）"
    - "控制台可见 PipelineChatModel.Stream() 调用前后日志"
    - "控制台可见首次 Recv() 调用日志 + 每次 Recv() 计数"
    - "streamAndAssemble 后段 EmitChunk 错误 + text-delta-draft-end 后有日志"
    - "现有 chat_test.go / pipeline_test.go 测试断言不变、全部通过"
  artifacts:
    - path: "internal/server/chat.go"
      provides: "chatHandler / runAgentPath / routeThenRun 入口日志埋点"
      contains: "log.Printf.*chatHandler enter"
    - path: "internal/server/pipeline.go"
      provides: "VerdictPipeline / streamAndAssemble 全链路埋点"
      contains: "log.Printf.*streamAndAssemble"
  key_links:
    - from: "internal/server/chat.go"
      to: "internal/server/pipeline.go"
      via: "VerdictPipeline 调用前的分支日志"
      pattern: "dispatching to VerdictPipeline"
    - from: "internal/server/pipeline.go"
      to: "PipelineChatModel.Stream"
      via: "Stream 调用前后日志"
      pattern: "calling PipelineChatModel.Stream"
---

<objective>
为 chat 流式链路加诊断日志，解决"前端拿到 data-subprogress 但拿不到 text-delta-draft、后端控制台完全无 /api/chat 日志"的盲区。日志覆盖 chatHandler 入口、路由分支、VerdictPipeline 入口、streamAndAssemble 入口、PipelineChatModel.Stream() 调用前后、首次 Recv()、每次 Recv() 计数、EmitChunk 错误、text-delta-draft-end 后共 8+ 个埋点。重启后端 + curl /api/chat 即可定位 Stream() 阻塞根因（GLM key 过期 / endpoint 失效 / chatModelOpts 不兼容 / 网络 hang）。

Purpose: 当前生产盲区 — 用户已确认走真后端（VITE_REAL_BACKEND=1），前端拿到 3 个 data-subprogress 但没有 text-delta-draft，且后端控制台对 /api/chat 完全静默。无日志无法判断 hang 发生在 PipelineChatModel.Stream() 入参构造、HTTP 建连、首次 Recv()、流中 Recv() 哪一段。本计划不修 bug，仅加日志把链路可见化，让下一次复现可直接读控制台诊断。

Output: chat.go 与 pipeline.go 新增约 14 处 [INFO] log.Printf 埋点；不改变控制流、不改变函数签名、不改变返回值、不改变测试断言。
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/quick/260815-cos-chat-c-llm-streaming-grounding-hover-ui/260815-cos-SUMMARY.md
</context>

<interfaces>
From internal/server/chat.go (line numbers from the live read):

- Line 128: chatHandler 入口（首行没有任何 log）
- Line 141: UTF-8 校验失败 400
- Line 147: BindAndValidate + req 解码
- Line 162-169: query 提取（lastUserQuery fallback）
- Line 170-172: req.Skill 默认值（空 → compliance）
- Line 181-185: sessionID 解析（X-Session-Id header 或 newSessionID）
- Line 204-207: Agent path 分支
- Line 215-236: Registry+Router 分支 vs Loader fallback
- Line 238: VerdictPipeline 调用
- Line 341: runAgentPath 入口（成功路径无 INFO）

From internal/server/pipeline.go:

- Line 138: VerdictPipeline 入口（首行没有任何 log）
- Line 388: streamAndAssemble 入口（首行没有任何 log）
- Line 394: PipelineChatModel.Stream() 调用
- Line 404: Recv() 循环（首次进入无日志）
- Line 417-423: text-delta-draft EmitChunk（已有 [WARN] 但成功路径无计数）
- Line 428-433: text-delta-draft-end（已有 [WARN] 但成功 emit 后无日志）

关键助手：
- chat.go line 513: truncate(s, n) 已有，复用做 query 截断
- chat.go line 96: defaultSkill = "compliance" 常量
- pipeline.go line 402: var buf strings.Builder — tokenN 声明位置
- chat.go line 432: writeAuditAndTurn 签名（不修改）
</interfaces>

<tasks>

<task type="auto">
  <name>Task 1: chat.go 与 pipeline.go 全链路诊断日志埋点
<files>internal/server/chat.go, internal/server/pipeline.go</files>

<action>
**步骤 A — 跑全量测试确认基线 PASS**

运行 go test ./internal/server/... -count=1 -timeout 60s。

预期：所有现有测试 PASS。chat_test.go / pipeline_test.go 用 stub ChatModel，不真调 GLM；新 log.Printf 走 stderr 不影响 SSE 字节流；useChat 解析器不受影响。

**步骤 B — 跑 go vet 确认基线干净**

运行 go vet ./internal/server/...

**步骤 C — chat.go 加 godoc DEBUG 注释 + 6 个 log.Printf 埋点**

C-1：在 chatHandler 函数首行后（line 128 之后、UTF-8 校验之前）插入 godoc DEBUG 注释块，告知用户怎么 curl 触发 + 期望日志序列 + hang 位置诊断决策表。这是 11 行 godoc，不影响运行时。

C-2：chatHandler 入口日志。在 line 170 与 line 185 全部完成后、line 204 之前一次性 log（query / skill / sessionID 此时全部就绪）。代码形如 log.Printf("[INFO] server: chatHandler enter query=... skill=... sess=...")。注意复用 chat.go line 513 已定义的 truncate 函数把 query 截到 80 字符。

C-3：Agent path 分支日志。在 line 204 if s.Agent != nil 之前。形如 log.Printf("[INFO] server: chatHandler routing to runAgentPath (USE_AGENT_PATH=true) skill=... sess=...")。

C-4：Registry+Router 分支日志。在 line 216 if s.Registry != nil ... 之前。形如 log.Printf("[INFO] server: chatHandler routing to Registry+Router skill=... sess=...")。

C-5：Loader fallback 日志。在 line 226 manifest = s.findSkill(req.Skill) 之后、line 227 nil check 之前。形如 log.Printf("[INFO] server: chatHandler routing to VerdictPipeline (Loader) skill=... sess=...")。

C-6：VerdictPipeline dispatch 日志。在 line 238 if err := s.VerdictPipeline(...) 之前。形如 log.Printf("[INFO] server: chatHandler dispatching to VerdictPipeline skill=... sess=...")。

C-7：runAgentPath 入口日志。在 line 341 函数首行之后。形如 log.Printf("[INFO] server: runAgentPath enter query=... skill=... sess=...")。

**步骤 D — pipeline.go 加 8 个 log.Printf 埋点**

D-1：VerdictPipeline 入口。line 138 函数首行之后、// Step 1: resolve tiers 之前。形如 log.Printf("[INFO] server: VerdictPipeline enter query=... skill_name=... skill_id=... sess=...")。注意复用 truncate 函数。

D-2：streamAndAssemble 入口。line 388 函数首行之后、stream, err := s.PipelineChatModel.Stream(...) 之前。形如 log.Printf("[INFO] server: streamAndAssemble entering prompt_bytes=N opts_count=N")。注意 sessionID 不在 streamAndAssemble 签名里 —— 只打 prompt 与 opts 体量。

D-3：PipelineChatModel.Stream() 调用前。紧贴 line 394 之前。形如 log.Printf("[INFO] server: streamAndAssemble calling PipelineChatModel.Stream model=%T opts=N")。用 %T 打印具体模型类型（*openai.ChatModel）以验证 Deps 注入正确。

D-4：Stream 返回 OK。line 397 if err != nil 之前、defer stream.Close() 之前。形如 log.Printf("[INFO] server: streamAndAssemble PipelineChatModel.Stream returned OK, entering Recv loop")。

D-5：首次 Recv 前 + tokenN 声明。line 403 for { 之前、var buf strings.Builder 之后。声明 tokenN := 0，然后 log.Printf("[INFO] server: streamAndAssemble about to call first Recv (timeout governed by ctx)")。

D-6：每次 Recv 计数。line 411 if msg == nil ... continue 之前（即成功 Recv 后、EmitChunk 之前）。形如 tokenN++; log.Printf("[INFO] server: streamAndAssemble recv token_n=N delta_bytes=N")。

D-7：text-delta-draft-end 后完成日志。line 428-433 之后、return buf.String(), nil 之前。形如 log.Printf("[INFO] server: streamAndAssemble completed total_tokens=N buf_bytes=N")。

**步骤 E — 最终验证**

运行：
- go test ./internal/server/... -count=1 -timeout 60s
- go vet ./internal/server/...
- go build ./...
- git diff --stat internal/server/

确认：仅 chat.go + pipeline.go 修改；chat.go 新增约 7 行（1 godoc 注释块 + 6 log.Printf）；pipeline.go 新增约 8 行（7 log.Printf + 1 tokenN 声明）；无 _test.go 修改。

注意事项：
- 不修改任何 _test.go 文件
- 不引入新依赖（log 包已在两文件 import 中）
- godoc 注释用 // DEBUG: 前缀让 IDE/GoDoc 高亮
- truncate 函数在 chat.go line 513 已定义，复用即可
- manifest.Name 字段如果为空字符串，log 会显示 "" —— 这是预期行为（Phase 2 Skill 仅有一个 Name="compliance"）
</action>

<verify>
<automated>go test ./internal/server/... -count=1 -timeout 60s && go vet ./internal/server/... && go build ./... && git diff --stat internal/server/</automated>
</verify>

<done>
- chat.go 新增 1 个 godoc DEBUG 注释块 + 6 个 [INFO] log.Printf 埋点
- pipeline.go 新增 6 个 [INFO] log.Printf 埋点 + 1 个 tokenN 声明
- go test ./internal/server/... 全 PASS（与 baseline 一致）
- go vet ./internal/server/... 无 warning
- go build ./... 成功
- git diff --stat 仅显示 chat.go + pipeline.go 两个文件
- 现有 chat_test.go / pipeline_test.go 文件零修改
- 无控制流变化、无函数签名变化、无返回值变化
</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| 用户 → 后端控制台 | log.Printf 输出到 stderr/stdout，**不含**敏感字段（query / skill / sessionID）。T-1-13 (API key 泄露) 不涉及 — log 不打印 env。 |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-GCW-01 | Information Disclosure | log.Printf query 内容 | mitigate | query 是用户输入的合规审查问题，不含 PII/credential；truncate 到 80 字符兜底 |
| T-GCW-02 | Information Disclosure | log.Printf sessionID | accept | sessionID 是 128-bit 随机 hex，无 PII；日志仅本地 stderr，不外发 |
| T-GCW-03 | Tampering | log 注入（CRLF 在 query 字段） | mitigate | Go log.Printf 自动转义控制字符；审计 grep 安全 |
| T-GCW-04 | Denial of Service | log 体量 | mitigate | 每次 /api/chat 约 14 条 log，每条约 80-120 字节；高频请求下磁盘 IO 可忽略；无文件落盘 |
| T-GCW-SC | Tampering | npm/pip/cargo installs | mitigate | **零安装** — 仅修改 .go 源代码，不引入依赖；slopcheck N/A |
</threat_model>

<verification>
- 全量测试 PASS: go test ./internal/server/... -count=1
- go vet ./internal/server/... 无 warning
- go build ./... 成功
- git diff --stat 仅 chat.go + pipeline.go 两个文件变更
- chat.go 新增 log 行数约 7（1 godoc 注释块 + 6 log.Printf）
- pipeline.go 新增 log 行数约 8（6 log.Printf + 1 tokenN 声明 + 1 调整 buf 块）
- 现有 chat_test.go / pipeline_test.go 文件零修改
- 无 _test.go 文件被修改
</verification>

<success_criteria>
- 用户重启后端 + curl /api/chat 可在控制台看到从 chatHandler 入口 → 路由分支 → VerdictPipeline 入口 → streamAndAssemble 入口 → Stream() 调用 → Stream 返回 → 首次 Recv → 每次 Recv 计数 → 完成的完整 14 个埋点日志
- 如果 hang 发生在 Stream() 调用前/后/首次 Recv 前/每次 Recv 中 — 通过日志行号精确锁定 hang 位置
- 现有测试全部 PASS（chat_test.go / pipeline_test.go）
- 无控制流变化、无函数签名变化、无返回值变化、无 SSE 输出字节流变化
</success_criteria>

<output>
Create `.planning/quick/260815-gcw-chat-streamandassemble-chathandler-strea/260815-gcw-SUMMARY.md` when done
</output>
