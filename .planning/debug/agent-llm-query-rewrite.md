---
status: fixed-pending-live-verify
trigger: "Phase 7 live 验证发现 agent RAG 检索对应该回答的查询（如'年假有几天'）返回 unanswered（拒答）。根因已用 scripts/retrieve_probe 实测锁定：embedding/Milvus/检索全健康，唯一问题是 agent 路径 runAgentPath 让 GLM-5.2 自主改写 retrieve query（年假→出差住宿费），召回错文档。"
created: 2026-08-13
updated: 2026-08-13
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

- **hypothesis (CONFIRMED via probe):** 根因是 agent 路径 runAgentPath（internal/agent/agent.go）让 LLM 自主生成 retrieve query，GLM-5.2 改写跑偏。embedding/Milvus/检索全健康（原 query「年假」精准召回 annual_leave score 0.6027）。提示词工程压不住（两次强化无效 + 引入回归）。
- **next_action (DONE):** 实现 fix = 在 (*Agent).Run 代码强制用原 user query embed+search 一次（结果进 ChunkAccumulator + 注入 user message DATA envelope），不依赖 LLM 改写；回退两次提示词改动（git checkout）；加 agent_test.go 回归（TestRunPreRetrievesOriginalQuery）。atomic commit 完成。
- **expecting (PENDING USER LIVE VERIFY):** 修复后「年假有几天」→ 带引用裁决（召回 annual_leave）；agent 仍可在保底结果上 refine。单元测试已绿；live 验证需用户重启 server + curl。

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

## Eliminated

- hypothesis: embedding 区分度低导致召回不到 —— **ELIMINATED**（probe 证明原 query 召回精准 0.60）。
- hypothesis: Milvus 对接 bug —— **ELIMINATED**（seed/search/检索全 live 通过，probe 正常返回）。
- hypothesis: 0.3 检索阈值不当 —— **ELIMINATED**（无关 query 0.24 < 0.3 正确挡，相关 0.6+ 正确过）。
- hypothesis: 提示词工程能纠正 LLM 改写 —— **ELIMINATED**（两次强化无效，反引入回归）。

## Resolution

- **root_cause:** agent 路径 runAgentPath 把检索决策 + query 生成交给 LLM（react loop），GLM-5.2 改写 query 跑偏（年假→出差住宿费），召回错文档。Phase 2 VerdictPipeline 用代码强制原 query 检索无此问题；Phase 3 换 agent 路径引入此脆弱点。
- **fix (APPLIED in commits 0171680 + d3178ed):** 在 `(*Agent).Run`（internal/agent/agent.go）代码强制用原 user query embed+search 一次（保底），结果进 `ChunkAccumulator`（acc.Add）+ 注入 user message 的 DATA envelope（`buildUserMessageBody` 复用 `formatChunksAsData` 的 `[CHUNK N] ... \n---\n` framing）。agent 仍可在此基础上 refine/补检（retrieve_compliance_chunks 工具不变；accumulator 按 chunk ID 去重，重复命中无副作用）。失败非致命：embed/search error 时 log + 回退到 agent-only 路径（修复前行为）。两次提示词改动已 `git checkout --` 回退（工作树清理，非独立 commit）。
- **verification:**
  - (1) ✅ 单元回归 `TestRunPreRetrievesOriginalQuery`（agent_test.go）：fake model maxToolCalls=0 + 已 seed fake retriever，断言 Run 返回非空 citations + LastRetrieved 含 annual_leave chunk。绿。
  - (2) ✅ 全仓 `go test ./...` + `go test -race ./internal/agent/`：绿。
  - (3) ⏳ **PENDING USER**：live 验证。需用户重启 server + curl「年假有几天」→ 期望带引用裁决。
- **files_changed:**
  - `internal/agent/agent.go` — 新增 `preRetrieveOriginalQuery` helper；`(*Agent).Run` 调用顺序调整（pre-retrieve → buildAgentMessages）；`buildAgentMessages` 签名增加 `preRetrieveChunks []retriever.Hit`；新增 `buildUserMessageBody`；imports 加 `log`、`time`。
  - `internal/agent/agent_test.go` — 新增 `TestRunPreRetrievesOriginalQuery`；既有 4 个测试的 `ret.CallCount()` 期望 +1（pre-retrieve 在 loop 外多加一次 Search）：TestAgentRetrievesAndAnswers (1→2)、TestMultiHopCitationAggregation (2→3)、TestMaxStepBlocksThirdHop (2→3)、TestDecisionSSEEmission 仅注释；TestMaxStepBlocksThirdHop chunksPerCall 扩展为 3 entry（覆盖 pre-retrieve）。
  - `internal/agent/retrieve_tool.go` — 无变化（提示词改动已回退）。
  - `scripts/retrieve_probe/main.go` — 已有的诊断脚本，commit 0171680 一并入库。

## Notes

- 用户在 live Milvus 上验证（10.62.10.34:19530 + BGE 10.62.0.123:11434）。修复后需用户重启 server + curl 验证（GLM_API_KEY 在用户进程，调试器/测试用 fake model 不需要 key）。
- 用户项目 CLAUDE.md 要求代码改动走 GSD workflow；本 debug session 即为该 workflow。atomic commit + 回归测试是用户明确要求（选 b）。
- 提示词改动（agent.go + retrieve_tool.go）当前在工作树未提交，应作为本修复的一部分回退（git checkout 或重写回原样）。**DONE**：`git checkout --` 回退，工作树干净。
- Live verification command（用户执行）：
  ```bash
  # 1. 重启 server（用生产 env）
  RETRIEVER_BACKEND=milvus MILVUS_ADDRESS=10.62.10.34:19530 \
    BGE_M3_URL=http://10.62.0.123:11434/v1/embeddings BGE_M3_MODEL=bge-m3 \
    GLM_API_KEY=<key> go run ./cmd/server

  # 2. 在另一个 terminal 触发回归 query
  curl -N localhost:8888/api/chat -d '{"query":"年假有几天","skill":"compliance"}'

  # 期望：data-policy status=合规|需补充 + confidence>0 + citations 含 annual_leave
  # （修复前：status=unanswered + confidence=0 + citations=[]）

  # 3. 对照 query（应保持工作）
  curl -N localhost:8888/api/chat -d '{"query":"出差住宿费超过500元合规吗","skill":"compliance"}'

  # 4. 反向回归 query（应保持拒答）
  curl -N localhost:8888/api/chat -d '{"query":"量子力学原理","skill":"compliance"}'
  ```
