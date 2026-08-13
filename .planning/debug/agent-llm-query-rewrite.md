---
status: resolved-encoding-rootcause
trigger: "Phase 7 live 验证发现 RAG 检索对应该回答的查询（如'年假有几天'）返回 unanswered（拒答）。"
created: 2026-08-13
updated: 2026-08-14
---

# Debug: agent-llm-query-rewrite

## Symptoms

- **Expected:** `POST /api/chat {query:"年假有几天",skill:"compliance"}` → 带引用裁决（annual_leave.md 在权威层语料里，应召回 + 给出 status/confidence/citations）。
- **Actual:** `data-policy status="unanswered" confidence=0 citations=[]`。agent 把 query 改写成「出差住宿费」/「出差费用报销」→ 召回 travel_expense.md（不相关）→ LLM unanswered。第二次提示词改动后甚至出现 `parse verdict: unexpected end of JSON` 回归（LLM 输出「消息乱码」自然语言而非 JSON verdict）。
- **Error:** 无 crash；现象是功能性的（拒答该答的查询）。第二次提示词改动引入 verdict JSON parse error。
- **Timeline:** 2026-08-13 首次在 live Milvus（10.62.10.34:19530，D11 etcd 损坏已解决）端到端验证 Phase 3 agent 路径时暴露。Phase 2 的 VerdictPipeline（代码强制检索）当时 6/6 SC 活体绿；Phase 3 runAgentPath（LLM 自主检索）从未 live 验证直到今天。
- **Reproduction:**
  - 起后端：`RETRIEVER_BACKEND=milvus MILVUS_ADDRESS=10.62.10.34:19530 BGE_M3_URL=http://10.62.0.123:11434/v1/embeddings BGE_M3_MODEL=bge-m3 GLM_API_KEY=<key> go run ./cmd/server`
  - 触发：`curl -N POST localhost:8888/api/chat -d '{"query":"年假有几天","skill":"compliance"}'`
  - 对照（work）：`query:"出差住宿费超过500元合规吗"` → 需补充 + 0.85 + 引用条文（因为原 query 跟检索意图一致，LLM 没跑偏）。

## Current Focus

- **hypothesis (CONFIRMED via probe + live-verified):** 根因是 agent 路径 runAgentPath（internal/agent/agent.go）把检索决策 + query 生成 + 相关性/拒答判断都交给 LLM（react loop）。GLM-5.2 在每一层都漂移：改写 query 跑偏（年假→出差住宿费），提示词工程压不住（两次强化无效 + 引入回归），强化「必须基于 chunk 回答」后又破坏了拒答护城河（量子力学 → 需补充 0.1 而非拒答）。Phase 2 VerdictPipeline 用代码强制原 query 检索 → chunks 注入 prompt → LLM 只出 verdict JSON，无此问题。
- **direction (SWITCHED to A per user):** 先前的 pre-retrieve 方向（commits 0171680/d3178ed，在 (*Agent).Run 代码强制原 query 检索一次 + 注入 user message）已被用户 + orchestrator **LIVE-VERIFIED FAILED**。用户明确选择方向 A：把生产默认路径切回 Phase 2 VerdictPipeline，runAgentPath 保留在树内但不在默认路径上。
- **next_action (DONE):** 实现方向 A。(1) 路由：cmd/server/main.go 把 agent 构造挂在 `USE_AGENT_PATH=true`（opt-in，默认 unset → Deps.Agent 为 nil → chatHandler 走 VerdictPipeline）；chat.go 既有的 `if s.Agent != nil` 分支语义不变。(2) 审计/会话持久化：VerdictPipeline 之前不写 audit/session 行，现已复用 writeAuditAndTurn 在每个 refusal/verdict 发射点写入，error 分支由 chatHandler 写入（refusal_reason="internal_error"），镜像 runAgentPath 的三个写入点。(3) 回归测试：3 个新测试锁定默认路径「相关 query→带引用裁决+audit 行」与「越界 query→拒答+audit 行（refusal_reason）」。(4) runAgentPath + pre-retrieve 代码原样保留为 opt-in 实验路径；系统提示词保持原样（已回退）。atomic commits per step。
- **expecting (PENDING USER LIVE VERIFY):** 用户重启 server（默认即 VerdictPipeline，无需设 USE_AGENT_PATH）+ curl「年假有几天」→ 期望带引用裁决；「量子力学原理」→ 期望拒答（护城河 intact）。单元 + race 测试全绿；live 验证需用户在 Milvus+GLM 环境执行。

## Evidence

- timestamp: 2026-08-13 probe `scripts/retrieve_probe`（用生产 bge+milvus 绕过 LLM 直接 embed+search 权威层 top-3）：
  - query="年假" → annual_leave.md「年假折算与转结规则」score=**0.6027** ✅ 精准命中
  - query="出差住宿费" → travel_expense.md score=0.7555 ✅
  - query="竞业限制" → non_compete.md score=0.6398 ✅
  - query="量子力学" → non_compete.md（误命中）score=**0.2397** ✅ 低分（<0.3 阈值，正确被挡）
  - 结论：区分度优秀（相关 0.60-0.76 vs 无关 0.24，3 倍）；0.3 阈值合理；**推翻 Phase 2 记录的"所有 doc 0.29-0.40"**（那是旧 Milvus+Mongo 环境）。
- timestamp: 2026-08-13 live curl /api/chat：
  - "年假有几天" → retrieve query 被改成「出差住宿费」/「出差费用报销」→ unanswered（应答却拒答）
  - "出差住宿费超过500元合规吗" → retrieve query 未改写 → 需补充+0.85+引用 ✅
  - "量子力学原理" → unanswered ✅（确实没有）
- timestamp: 2026-08-13 提示词尝试 1（agent.go systemPrompt 加 MANDATORY FIRST STEP + retrieve_tool.go Desc 禁止改写）：retrieve 触发了，但年假 query 仍被改成「出差住宿费」。
- timestamp: 2026-08-13 提示词尝试 2（agent.go systemPrompt 加 CRITICAL RETRIEVE QUERY RULE 点名年假≠出差费用）：年假 query 仍被改写成「出差住宿费」，且新增 verdict JSON parse error（LLM 输出「抱歉，消息乱码」自然语言）。**回归**。
- timestamp: 2026-08-13 fix 应用后单元测试（`go test ./internal/agent/`）：全部通过，含新加的 TestRunPreRetrievesOriginalQuery（模拟 LLM 不合作场景：maxToolCalls=0，retriever 在 call 0 返回 annual_leave，断言 verdict 有非空 citations + LastRetrieved 暴露 chunk 42）。`go test -race ./internal/agent/` 干净。`go test ./...` 全仓绿。
- timestamp: 2026-08-13 **pre-retrieve 方向 LIVE-VERIFIED FAILED（用户 + orchestrator）**：
  - pre-retrieve 确实召回了正确的 doc（probe：原 query「年假有几天」→ annual_leave.md score 0.65 top-1）。
  - 但用原系统提示词时，LLM 忽略 user-message 预载的 chunks → 仍 unanswered。
  - 用强化提示词（「必须基于 present 的 chunk 出 verdict；不得拒答」）时，LLM：(a) 对年假引用了**错误**的 chunk（引用了反腐败「商业贿赂」而非 annual_leave），(b) **停止拒答**越界 query（量子力学 → 需补充 0.1 而非拒答）—— 拒答护城河被打破。
  - 该强化系统提示词已被 orchestrator `git checkout --` 回退到原样（工作树：系统提示词为原样；pre-retrieve 代码来自 commit 0171680 仍在但已因切换默认路径而成为死代码）。
  - 根因是架构性的：runAgentPath 把检索决策 + query 生成 + 相关性/拒答判断都交给 LLM，每一层都漂移。Phase 2 VerdictPipeline 用代码强制原 query 检索无此问题；其在 Milvus 上的检索质量被 scripts/retrieve_probe 独立确认（原 query → 正确 doc，相关 0.6-0.76 vs 无关 0.24）。
- timestamp: 2026-08-13 方向 A 实现 + 验证：
  - `go build ./...` 绿；`go test ./...` 全仓绿；`go test -race ./internal/server/` 绿。
  - 3 个新回归测试通过（TestVerdictPipelineDefault_PersistsAuditAndCitedVerdict / _RefusalPathPersistsAudit / _NoChunksRefusalPersistsAudit）。
  - 既有 agent 路径测试（chat_session_test.go 直接构造 Deps.Agent）未改动仍绿 —— 确认 runAgentPath 保留为可构造的 opt-in 路径。

## Eliminated

- hypothesis: embedding 区分度低导致召回不到 —— **ELIMINATED**（probe 证明原 query 召回精准 0.60）。
- hypothesis: Milvus 对接 bug —— **ELIMINATED**（seed/search/检索全 live 通过，probe 正常返回）。
- hypothesis: 0.3 检索阈值不当 —— **ELIMINATED**（无关 query 0.24 < 0.3 正确挡，相关 0.6+ 正确过）。
- hypothesis: 提示词工程能纠正 LLM 改写 —— **ELIMINATED**（两次强化无效，反引入回归）。
- hypothesis: pre-retrieve（代码强制原 query 检索 + 注入 user message）能在 agent 路径上救回 —— **ELIMINATED（LIVE-VERIFIED FAILED）**：召回对了，但 LLM 或忽略预载 chunks（原提示词）或乱引用 + 破坏拒答护城河（强化提示词）。根因是 agent 路径让 LLM 主导太多层。

## Resolution

- **root_cause:** agent 路径 runAgentPath 把检索决策 + query 生成 + 相关性/拒答判断都交给 LLM（react loop）。GLM-5.2 在每一层都漂移（年假→出差住宿费改写 → 召回错文档 → unanswered）。提示词工程压不住；pre-retrieve 救不回（LLM 忽略预载 chunks 或强化后破坏拒答护城河）。Phase 2 VerdictPipeline 用代码强制原 query 检索 → chunks 注入 prompt → LLM 只出 verdict JSON，结构上消除此脆弱点。
- **fix (APPLIED — 方向 A, commits 9400175 + 1cef0f3 + 4183c43):**
  - **路由（commit 9400175）：** cmd/server/main.go 把 agent 构造挂在 `USE_AGENT_PATH=true`（opt-in，默认 unset）。默认时 `*server.Deps.Agent` 为 nil，chatHandler 既有的 `if s.Agent != nil` 分支不进入 → 走 VerdictPipeline。.env.example 文档化了该 flag。runAgentPath + pre-retrieve 代码（commits 0171680/d3178ed）原样保留为 opt-in 实验路径；系统提示词保持原样（强化版已回退）。既有 chat_session_test.go 测试直接构造 Deps.Agent，仍走 runAgentPath，未改动。
  - **审计/会话持久化（commit 1cef0f3）：** VerdictPipeline 之前不写 audit/session 行（只有 runAgentPath 调 writeAuditAndTurn）。现已复用 chat.go 既有的 writeAuditAndTurn 在 VerdictPipeline 的每个发射点写入，镜像 runAgentPath 的三个写入点：4 个 refusal 发射点（空 tiers / 0 chunks / 低分 / LLM-unanswered）写 refusal_reason；verdict 发射点写 refusal_reason="" + verdict_json + chunk_ids；chatHandler 的 error 分支（embed/retrieve/LLM drain/parse 真错误）写 refusal_reason="internal_error"。VerdictPipeline 签名增加 (sessionID, skillID string)。写入仍为 best-effort（T-3-AUDIT-WRITE accept）。
  - **回归测试（commit 4183c43）：** internal/server/pipeline_default_test.go 3 个测试锁定默认路径：相关 query（stub retriever seed annual_leave @0.65，镜像 live probe）→ 带引用裁决 + 1 audit 行 + (user,assistant) session turn + runner 调用恰好 1 次；越界 query（score 0.24，镜像 量子力学→0.2397）→ low_retrieval_score 拒答 + audit 行 + runner 调用 0 次；0 chunks → no_authorized_source 拒答 + audit 行。
- **verification:**
  - (1) ✅ `go build ./...` 绿。
  - (2) ✅ `go test ./...` 全仓绿（含 internal/agent 既有 pre-retrieve 测试 TestRunPreRetrievesOriginalQuery —— 该代码仍在树内）。
  - (3) ✅ `go test -race ./internal/server/` 绿。
  - (4) ✅ 3 个新回归测试通过。
  - (5) ⏳ **PENDING USER**：live 验证。默认即 VerdictPipeline（无需设 USE_AGENT_PATH），用户重启 server + curl「年假有几天」→ 期望带引用裁决；「量子力学原理」→ 期望拒答。
- **files_changed:**
  - `cmd/server/main.go` — USE_AGENT_PATH env 解析；agent 构造挂在 flag 后（默认不构造，Deps.Agent=nil）；启动日志标明当前 /api/chat 路径。
  - `internal/server/pipeline.go` — VerdictPipeline 签名增加 (sessionID, skillID string)；4 个 refusal 发射点 + verdict 发射点调用 s.writeAuditAndTurn（emit 后、return 前；verdict 路径在 emit-error return 前写入以保证 GEVL-03）。
  - `internal/server/chat.go` — chatHandler 路由注释更新（agent 路径 opt-in，VerdictPipeline 默认）；VerdictPipeline 调用传 sessionID+req.Skill；error 分支调用 writeAuditAndTurn(refusal_reason="internal_error")；writeAuditAndTurn docstring 更新覆盖 VerdictPipeline 用法。
  - `internal/server/pipeline_default_test.go` — 新增 3 个回归测试。
  - `.env.example` — 文档化 USE_AGENT_PATH（默认 unset → VerdictPipeline）。
  - `internal/agent/agent.go` / `internal/agent/retrieve_tool.go` — **无变化**（pre-retrieve 代码 + 系统提示词保持原样，作为 opt-in 实验路径）。

## Post-Resolution (2026-08-14): 真正根因 = 客户端编码（非后端 bug）

切到 VerdictPipeline（方向 A）后，live 验证「年假有几天」**仍然** unanswered。这与 probe（独立 Go 进程，UTF-8 字面量）召回 annual_leave 0.65 矛盾。临时 `[DEBUG-VP]` log 显示 server 召回 anti_corruption_clause.md 0.3094（错 chunk），且**4 次完全一致**（非随机）→ 排除了「embedding 不稳」假设。

**决定性实验**：用 `printf` 强制 UTF-8 body + `-H "Content-Type: application/json"` curl → VerdictPipeline 召回 annual_leave 0.65 → `需补充 0.8 + 引用「结转上限 5 天」` ✅。三个回归 query 全绿（年假 需补充 0.8 / 差旅 需补充 0.95 / 量子力学 low_retrieval_score 拒答）。

**根因**：用户在 PowerShell（GBK codepage 936）里 curl 中文，字节被编成 GBK 发出。Hertz 的 sonic JSON 解码把无效 UTF-8 字节 sanitize 成 U+FFFD → query 变成 U+FFFD 串 → embed 出垃圾向量 → 低分召回 anti_corruption（近似随机向量的 top-1，跟 probe 的「量子力学→anti_corruption 0.26」是同一现象）。证据闭环：(1) `[DEBUG-VP]` query 乱码 = server **真收到**乱码字节，非终端显示问题；(2) 4 次稳定 = GBK 编码确定性；(3) probe 对 = Go 源码字面量 UTF-8 不经 shell/curl；(4) 差旅偶尔 work = 那次用了 UTF-8 shell。

**结论：后端 VerdictPipeline + BGE-M3 embed + Milvus + 检索 + 拒答护城河全部正确。** 前端用 fetch/JSON（UTF-8），真实用户永远不受影响。这是命令行测试的编码陷阱，不是后端代码缺陷。

> 注：agent 路径 runAgentPath 的 LLM query-rewrite 漂移（年假→出差住宿费，probe 证据）仍是**独立的真实问题**，保留为 agent 路径的已知缺陷（opt-in 实验路径，默认不走）。但它**不是**默认路径 unanswered 的根因——默认路径 unanswered 纯粹是编码。

**加固（已实现 + 测试全绿）**：
- `internal/server/chat.go`：chatHandler 在 `BindAndValidate` **之前**对 raw body 做 `utf8.Valid` 校验，无效 → 400 `"request body must be valid UTF-8"`。必须在 BindAndValidate 前——sonic 在解码时已把无效字节 sanitize 成 U+FFFD，之后无法检测（单元测试 `TestChatHandlerRejectsNonUTF8Query` 探明此行为：放 query 后校验时 sonic 已转 U+FFFD，校验失效，落到 findSkill 的 out_of_scope 而非 400）。
- `internal/server/chat_utf8_test.go`：`TestChatHandlerRejectsNonUTF8Query`（手搓 GBK body → 断言 400）+ `TestChatHandlerAcceptsUTF8Query`（UTF-8 → 不拒，正对照）。
- `internal/server/pipeline.go`：删除临时 `[DEBUG-VP]` 诊断 log。
- 验证：`go build ./...` 绿；`go test ./...` 全仓绿；`go test -race ./internal/server/` 绿；`go vet` 干净。

**Live 验证（PENDING USER）**：重启 server + 用 GBK/PowerShell 中文 curl → 期望 400；用 UTF-8 curl → 期望正常裁决（已确认）。

## Notes

- 用户在 live Milvus 上验证（10.62.10.34:19530 + BGE 10.62.0.123:11434）。修复后需用户重启 server + curl 验证（GLM_API_KEY 在用户进程，调试器/测试用 fake model 不需要 key）。默认即 VerdictPipeline，**不要**再改系统提示词。
- 用户项目 CLAUDE.md 要求代码改动走 GSD workflow；本 debug session 即为该 workflow。atomic commit per logical step（路由 / 审计持久化 / 测试 / debug 文件）是用户明确要求。
- Live verification command（用户执行，默认路径无需设 USE_AGENT_PATH）：
  ```bash
  # 1. 重启 server（用生产 env；不设 USE_AGENT_PATH → 默认 VerdictPipeline）
  RETRIEVER_BACKEND=milvus MILVUS_ADDRESS=10.62.10.34:19530 \
    BGE_M3_URL=http://10.62.0.123:11434/v1/embeddings BGE_M3_MODEL=bge-m3 \
    GLM_API_KEY=<key> go run ./cmd/server
  # 启动日志应见：[OK] cmd/server: /api/chat path = VerdictPipeline (default; USE_AGENT_PATH unset)

  # 2. 回归 query（修复前：unanswered；修复后：带引用裁决）
  curl -N localhost:8888/api/chat -d '{"query":"年假有几天","skill":"compliance"}'

  # 3. 对照 query（应保持工作）
  curl -N localhost:8888/api/chat -d '{"query":"出差住宿费超过500元合规吗","skill":"compliance"}'

  # 4. 反向回归 query（应保持拒答 —— 拒答护城河 intact）
  curl -N localhost:8888/api/chat -d '{"query":"量子力学原理","skill":"compliance"}'

  # 5. （可选）确认审计行已写入（VerdictPipeline 默认路径现在写 audit）
  curl -N localhost:8888/api/audit?skill=compliance
  ```
- 若未来要重新启用 agent 路径做实验：`USE_AGENT_PATH=true go run ./cmd/server`（runAgentPath + pre-retrieve 代码仍在树内，agent 单元测试仍绿）。重新启用前必须先解决「LLM 主导太多层」的架构问题，否则会重现本次 live-verified failure。
