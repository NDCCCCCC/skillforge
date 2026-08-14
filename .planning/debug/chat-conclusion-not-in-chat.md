---
status: awaiting_human_verify
trigger: "chat 最终结果不出现在 chat 对话流里，而是出现在页面中间的'结论'tab 中（用户：'chat逻辑很奇怪'）"
created: 2026-08-15
updated: 2026-08-15
---

# Debug: chat-conclusion-not-in-chat

## Symptoms

- **Expected:** 最终结论文本以 assistant 消息形式流式出现在右侧 chat 对话流里；页面中间的「结论」tab 只承载结构化字段（status / confidence / citations）。
- **Actual:** chat 对话流里有过程内容（流式输出阶段），但没有最终结论；最终结果只出现在页面中间的「结论」tab 里。
- **Error:** 无 crash / 无报错信息（用户未提及）。
- **Timeline:** 一直都是这样 —— 自该 UI 实现以来即如此（Phase 4 streaming-trust-ui），不是回归，是设计分流问题。
- **Reproduction:** 打开前端 → 发送合规查询（如「年假有几天」）→ 观察右侧 chat 流（有过程无结论）与页面中部「结论」tab（结论只出现在这里）。

## Current Focus

- **hypothesis:** CONFIRMED + FIX APPLIED（见 Resolution）。
- **test:** 已完成：2 个新回归测试 + 全仓 go test + web vitest 全绿。
- **expecting:** 用户在真实环境（重启 Go server + 前端 VITE_REAL_BACKEND=1）发送「年假有几天」→ chat 流出现「结论：…（置信度 …%）。依据 权威 tier 条款：「…」。」文本；结论 tab 仍显示结构化卡。
- **next_action:** await human verify（用户 live 确认后 → archive_session）。

reasoning_checkpoint:
  hypothesis: "VerdictPipeline 从不产生结论文本：prompt 禁止非 JSON 输出（'NO other text'）、Verdict 无文本字段、emitVerdictStream 只发占位空格 text-delta —— 结论只存在于 data-policy 结构化 payload；FE 忠实渲染（结论 tab + 气泡内 VerdictCard），但 chat 流 text part 永远没有可读结论。refusal 路径同类：emitRefusal 是 data-only 流（无 text part），而 Phase 4 fixture（refusal-out-of-scope.json line 9-11）明确带 text part —— FE 契约要求两条路径都有可读文本。"
  confirming_evidence:
    - "chat.go:622 Delta: \" \" 占位符（注释自述 placeholder）"
    - "pipeline.go systemPromptTemplate: 'NO other text. ONLY the JSON object.'"
    - "multi-part.json / refusal-out-of-scope.json 均带结论文本 text-delta（mock 模式 chat 有结论）；用户 web/.env VITE_REAL_BACKEND=1 走真实后端 → 结论消失"
    - "pipeline.go:302-306 自述 live-verified：缺 text-end/finish 的消息不注册 —— emitRefusal 恰好只发 data-policy+finish-step"
  falsification_test: "若 chat 流已有结论文本或 FE 丢弃了文本 chunk，假设即伪 —— 已直接读码排除（ai@7 parser line 7418 把 data chunk push 进 parts；text part 只收到一个空格）"
  fix_rationale: "根因=『结论文本不存在于流中』。修复=在 SSE 发射层（emitVerdictStream/emitRefusal）用已验证的结构化字段确定性推导中文结论文本并以 text-delta 发出，同时给 refusal 路径补 text-start/delta/end+finish 消息锚。不动 LLM prompt/Verdict schema/eval golden set（保持确定性与 eval 门禁），FE 零改动（fixture 契约本就如此）。"
  blind_spots: "未在真实 Milvus+GLM 环境端到端验证（需用户 human-verify）；结论文本为模板推导而非 LLM 自由生成 —— 若用户期望 LLM 生成的自然语言总结，需要后续 Phase 扩展 Verdict schema（超本次范围）。"

## Evidence

- timestamp: 2026-08-15 (session 2)
  checked: `internal/server/pipeline.go` systemPromptTemplate (lines 72-93) + Step 8/10
  found: 系统提示词强制「Output ONLY a JSON object… NO other text. ONLY the JSON object.」；Verdict 结构里没有文本字段；结论只以 data-policy chunk 发出。
  implication: LLM 层面被禁止产生结论文本 —— 文本在后端根本不存在，不是被前端丢弃。

- timestamp: 2026-08-15 (session 2)
  checked: `internal/server/chat.go` emitVerdictStream (lines 611-662)
  found: text-delta 固定 `Delta: " "`（注释自述「placeholder」）；text-start/text-end 只是为了 useChat 注册消息。data-policy 承载整个 Verdict JSON。
  implication: chat 流 text part = 一个空格；结构化结论走 data-policy。

- timestamp: 2026-08-15 (session 2)
  checked: `web/src/App.tsx` (lines 158-172) + `web/src/components/center/CenterPanel.tsx` + `web/src/components/chat/MessageBubble.tsx` + `web/src/components/chat/parts/DataPolicyPart.tsx` + ai@7.0.59 dist/index.js:7408-7418
  found: 前端从 messages 反向扫 data-policy part → currentVerdict → CenterPanel「结论」tab 渲染 VerdictCard；MessageBubble 也会内联渲染 VerdictCard（ai v7 对无 id 的 data chunk 仍 push 进 message.parts，line 7418，非 transient）。FE 路由本身没有 bug —— 它忠实渲染了后端发的全部内容。
  implication: 问题不在前端路由；是后端从未发送结论文本。

- timestamp: 2026-08-15 (session 2)
  checked: `web/src/__fixtures__/multi-part.json` (DEFAULT_FIXTURE) vs 真实后端流
  found: mock fixture 在 text-delta 里带结论文本（「依据《合同审批管理办法》第 5 条…当前流程符合该制度。」lines 29-30）；真实后端流（USE_AGENT_PATH unset, Router nil）= text-start → text-delta " " → source-document×N → data-policy → [counter_evidence/redline] → text-end → finish-step → finish。
  implication: dev/mock 下 chat 有结论文本（Phase 4 UI 按此设计），切到真实后端（web/.env VITE_REAL_BACKEND=1）后结论消失 —— 正是用户观察到的「chat逻辑很奇怪」。

- timestamp: 2026-08-15 (session 2)
  checked: `cmd/server/main.go` (Router=nil at line 257 skill.New("skills/", nil), Agent=nil 默认) + `web/.env` (VITE_REAL_BACKEND=1)
  found: 用户实际运行 = VerdictPipeline 生产默认路径 + 真实后端。
  implication: 复现路径锁定。

## Eliminated

- hypothesis: 前端把 data-policy chunk 只路由到结论 tab、不落入 useChat 消息流
  evidence: MessageBubble case 'data-policy' → DataPolicyPart → VerdictCard 在气泡内联渲染；ai@7 parser（dist/index.js:7418）把无 id 的 data chunk push 进 message.parts；App.tsx currentVerdict 也来自同一 messages。前端没有丢弃任何东西 —— 是后端发的 text part 只有一个占位空格，且全系统不存在结论文本。
  timestamp: 2026-08-15 (session 2)

- hypothesis: runAgentPath（USE_AGENT_PATH=true）缺 text-end/finish 导致消息未注册
  evidence: 用户环境未设 USE_AGENT_PATH（.env.example 注释 + 前序 debug session agent-llm-query-rewrite 的结论：默认 VerdictPipeline）；VerdictPipeline 路径已正确闭合 text-end+finish-step+finish（pipeline.go:312-322）。
  timestamp: 2026-08-15 (session 2)

## Resolution

root_cause: VerdictPipeline 设计上不产生自然语言结论文本：systemPromptTemplate 强制 LLM 只输出 Verdict JSON（"NO other text"），Verdict schema 无文本字段，emitVerdictStream 的 text-delta 是占位空格 " "；refusal 路径（emitRefusal）更是 data-only 流（无 text part，按项目自证的 live Phase 7 行为，消息在 useChat 中不注册）。结论只存在于 data-policy 结构化 payload，FE 把它镜像到中间「结论」tab —— chat 流里永远没有可读结论。Phase 4 mock fixtures（multi-part.json / refusal-out-of-scope.json）都带结论文本 text-delta，所以 mock/dev 有结论、真实后端（VITE_REAL_BACKEND=1）没有 —— 前后端契约裂缝，非回归。

fix: 在 SSE 发射层确定性推导结论文本（不动 LLM prompt / Verdict schema / eval golden set / FE）：
- `internal/server/chat.go` — emitVerdictStream 的 text-delta 从占位空格改为 conclusionText(v)（状态 + 置信度 + 首条已 grounding 的引用 span 原文 + 需复核提示）。
- `internal/server/chat.go` — emitRefusal 补全 text-start/text-delta(refusalText)/text-end 消息锚 + finish 终止符；refusalText 把 reason 码映射为中文可读句 + 改写建议。
- `internal/server/chat.go` — runAgentPath 尾部补 text-end + finish（与 VerdictPipeline Step-11 同构，修复 opt-in agent 路径消息不注册的同类隐患）。
- `internal/server/chat_test.go` — 新增 TestVerdictStreamCarriesConclusionText + TestRefusalStreamCarriesTextPart 回归测试。

verification: go build ./... + go vet 通过；go test ./... 全绿（含 2 个新回归测试）；web vitest 233/233 全绿（FE 零改动，fixture 契约本就要求 text part）。live 端到端（真实 Milvus+GLM 环境 + 浏览器）待用户验证。

files_changed: [internal/server/chat.go, internal/server/chat_test.go]
