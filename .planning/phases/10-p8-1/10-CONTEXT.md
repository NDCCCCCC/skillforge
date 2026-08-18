# Phase 10: 多轮对话记忆（P8.1）- Context

**Gathered:** 2026-08-18
**Status:** Ready for planning
**Requirements:** MEM-01, MEM-02, MEM-03, MEM-04, MEM-05 (5 v0.8 requirements) — 5 Success Criteria (ROADMAP §Phase 10)

<domain>
## Phase Boundary

用户可跨页面刷新进行多轮追问——agent 结合消毒后的会话历史正确解析指代并作答；citation 白名单代码级锁定为本轮检索，历史/摘要文本永远不能成为证据；长会话超预算按 project→then→summarize 顺序压缩。

**前置条件（Phase 9 NO-GO 裁决，D9-V1/V2）：** 本 phase 必须前置信任地板逃逸强化——拒答判定不得依赖历史上下文、历史不得携带拒答语义进 prompt；memory 发布前须重跑 spike 硬门回到四指标 <5% / refusal delta <10pp（`cmd/spike-history` 即重门机制）。

**本 phase 交付：**
- sqlite 历史投影喂 agent（`internal/history` 将 session 投影为 `[]schema.Message` 喂 VerdictPipeline）
- 投影消毒产品化（D9-B1/B2/B3 契约 + verdict JSON 确定性渲染 + 旧拒答中性占位）
- query-only 拒答分类器（拒答判定与历史结构性解耦——逃逸方案核心）
- query rewrite 前置（指代消解进检索）
- citation 白名单代码强制（本轮 Run 检索 = 唯一证据源）
- token 预算哑实现 + 预算事件可观测（data-decision 扩展）
- FE 刷新后完整恢复（新 history API + initialMessages + 现有卡片组件）
- 末位 plan 全量 35 例 live 重跑 spike 硬门（发布验收）

**不交付（deferred）：**
- Eino ADK Runner 迁移 + summarization middleware 启用（Phase 11——历史注入先落 VerdictPipeline，投影输出已是 Message 形态故搬迁成本低）
- frozen eval 基线 v2 重锁（v0.8 milestone 收尾统一做）
- HITL 审批门（Phase 11）、DeepAgent 规划（Phase 12）、子 agent 扇出（Phase 13）

</domain>

<decisions>
## Implementation Decisions

### A. 信任地板防漂移机制（Phase 9 逃逸方案具体化）

- **D10-A1: 拒答判定 = query-only 拒答分类器（独立 LLM 前置调用）。** 输入 = query + 本轮检索 chunks——物理上看不到历史；判定通过后才走历史增强生成。保留语义拒答能力（PII 类）；代价 = 每轮多一次轻量 LLM 调用（~1s 量级）。这是 D9-V2 条款 (c) 的直接落地。**拒绝：** 代码级全接管（丢失语义拒答——检索意外命中时 PII 类漏拒）；双保险（实现面最大且优先级/重叠处理需额外定义）。现有代码级短路（SC-2 0-chunks / score<0.3）保持不动——它天然不看历史，作为分类器之外的结构性保底。
- **D10-A2: 历史中的旧拒答轮 = 中性占位模板。** 拒答轮投影时渲染为固定模板（D9-L2 filler 同风格：固定字符串、零拒答语义词如「拒绝/不能/无权」），保留对话连续性（「刚才为什么没答」追问不突兀）。模板措辞需红测验证不触发同类漂移。**拒绝：** 完全丢弃拒答轮（agent 对拒答失忆，答非所问）；保留完整拒答文本（spike 已证明这是 -95pp 漂移源头）。
- **D10-A3: spike 硬门重跑 = 末位 plan 全量 35 例。** 最后一个 plan = live 重跑全量（10 恶意 + 15 善意 + 10 长会话，真 GLM + Milvus，同 seed=42），四指标 + refusal delta 全部回门才宣告 phase 完成。与 Phase 9 同门同口径，结果直接可比；成本 ~$0.05 + ~75min 可接受。
- **D10-A4: frozen eval 基线 = spike 门独立把关，旧 lock 保档不动。** 旧 lock（sha256 50bb1688）与数字（faithfulness 0.972 / refusal 95%）保档不动；Phase 10 拒答行为变化由 spike 硬门单独把关；frozen CI gate 若因拒答路径变更而红则记录豁免理由。基线 v2 重锁推迟到 v0.8 milestone 收尾统一做。**拒绝：** Phase 10 建新基线 v2（末位 plan 多一个 live 验收项，且 v0.8 内还有 Phase 11-13 行为变更，现在锁 v2 仍会过时）；双基线并行（维护面翻倍，v1 意义存疑）。

### B. 历史注入落点 + Runner 时序

- **D10-B1: 历史注入落 VerdictPipeline（生产默认路径）。** 投影输出 `[]schema.Message` 在 pipeline 内渲染进 LLM 请求。改动集中、全套既有回归套件做安全网、spike 重门同口径。Phase 11 Runner 迁移时历史注入搬一次家（`projection.go` 输出已是 Message 形态，搬迁成本低）。**Runner 迁移本身归 Phase 11**（HITL Interrupt/Resume 前置）。**拒绝：** 直接迁 ADK Runner（非默认路径未过生产验证 + 同时改两件事调试面翻倍）；迁移预留抽象层（投影输出已是 Message 形态，抽象层多余）。
- **D10-B2: 历史消息结构 = 多轮 messages 前缀。** 消毒后历史轮次作为原生 messages 前缀（user/assistant 交替）进入 LLM 请求；最后一条 user = query + chunks DATA（现有 `userQueryTemplate` 不变）；system 保持单条不变。GLM 对原生多轮 messages 的指代解析遵循远好于文本拼接。`streamAndAssemble` 从双消息扩展为「前缀 + 双消息」。**拒绝：** 单 user 三段式（历史是文本不是对话，指代解析弱——spike 注入近似此形态）；system 摘要框（丢轮次结构 + 指令遵守劣化）。
- **D10-B3: 历史窗口 = token 预算驱动。** 预算内尽量多轮，超限从最旧轮截断（project 阶段）；3 轮不再是硬限而是 spike 验证过的保底口径。哑预算先行（RF-6：GLM tokenizer 不可配置——字符近似估算）；summarization middleware 留待 Phase 11 Runner 迁移后启用（ROADMAP SC-5 顺序 project→then→summarize 保持）。末位重门若长会话子集不过门则回退收紧窗口。**拒绝：** 固定 3 轮硬限（长会话只能记住最近 3 轮，MEM-05 的 project 阶段意义弱化）；轮数 env 可配置（默认仍保守 3 轮，长会话体验问题未解决）。
- **D10-B4: 预算/压缩事件可观测 = data-decision chunk 扩展。** 复用现有 data-decision SSE 通道（D16 观测基底）新增 decision 类型（如 `history_budget_truncated` / `history_projected`），FE 可选渲染 + 后端 log 同步。零新 wire 类型（Go/TS 两侧不动 D15 白名单结构），审计链完整。**拒绝：** 仅后端 log（用户不可见，「可观测」只剩运维查日志）；新 SSE chunk 类型（wire 双侧同步 + D15 扩容，维护面增加）。

### C. 指代解析策略（MEM-01）

- **D10-C1: 追问轮检索 query = query rewrite 前置。** 追问轮先轻量 LLM 改写：看历史 + 追问 → 独立完整 query（「那第二条呢」→「XX 制度第二条的内容」）→ 用改写 query 走检索 + 拒答分类器；生成层仍给原 query + 历史前缀。首轮/无指代时跳过 rewrite。会话检索质量的标准解（conversational query rewriting）。**拒绝：** 原 query 直检（「那第二条呢」检索 miss → SC-2 拒答——用户问上轮答过的内容却被拒，MEM-01 SC 直接不过）；零成本拼接（embedding 被无关文本污染，召回不可控）。
- **D10-C2: 执行顺序 = 先 rewrite 后分类。** 追问轮链路：rewrite（看历史，产出独立 query）→ 改写 query 检索 → 拒答分类器（看改写 query + 本轮 chunks）→ 通过才生成。分类器看到语义完整 query 判定准；**rewrite 输出仅用于检索/分类，不进证据链**——恶意历史即使污染改写文本，检索不到 chunk 仍走拒答（结构性兜底）。**拒绝：** 先分类后 rewrite（指代未消解时分类器无法判定，误判率高）；分类器双 query 输入（两 query 不一致时判定口径含糊）。
- **D10-C3: 改写结果 = 透明提示。** 改写 query 经 data-decision chunk 下发（如 `decision=history_query_rewritten, query=<改写文本>`），FE 在消息气泡上方渲染小字提示（「已理解为：XX 制度第二条」）。用户能发现改写误解并纠正（重问）；与 D16 决策可观测同一通道；audit 行同步留档。**拒绝：** 静默内部（改写误解时用户只见莫名拒答/跑偏答案，信任型产品黑盒决策与平台调性冲突）；仅审计留档（UI hint: yes 要求可感知）。

### D. FE 刷新后历史恢复（MEM-02 UI）

- **D10-D1: 恢复形态 = 完整恢复。** 新 API（如 `GET /api/session/:id/history`）返回结构化历史（role + 渲染文本 + 解析后 verdict），FE 经 useChat `initialMessages` 恢复（D14 约束：不进 Zustand）；历史消息用现有 VerdictCard / CitationCluster 组件渲染，citation chip 可点开 chunk 详情——与 live 消息视觉一致。数据完整（session_turn 存 raw verdictJSON，可确定性渲染）。**拒绝：** 后端记忆前端白板（用户看到空白以为记忆丢失，与 UI hint「刷新后连续性为可感知界面行为」矛盾）；简化文本恢复（同一会话两种视觉形态 + 引用不可点，体验割裂）。
- **D10-D2: Skill 归属 = 自动切到会话 Skill。** 恢复历史时 FE 自动切换到该 session 绑定的 Skill（history API 返回 skill_id，SkillPicker 同步选中；不一致时切换并提示）。检索库始终与会话上下文对齐。**拒绝：** 保持当前 Skill（历史属于 Skill A、检索走 Skill B 的库，指代解析跨库错位）；不匹配则阻断（多一步手动操作，体验繁琐）。

### Claude's Discretion

- **中性占位模板具体措辞** —— D10-A2 锁定「固定字符串 + 零拒答语义词」；具体文案由实施定，须过红测（恶意注入用例下验证不触发拒答漂移）。
- **哑预算 token 估算口径** —— RF-6 明示 GLM tokenizer 不可配置；字符近似（如中文 1 字符 ≈ 1 token 保守估算）+ 预留检索 chunks 与输出的余量，具体常数由实施 + 末位重门校准。
- **rewrite 失败/超时降级** —— best-effort 用原 query 检索（历史照常进生成层），不报错不阻断。
- **rewrite 与拒答分类器的 prompt 形态** —— 轻量调用（低 max_tokens、结构化输出），具体 prompt 由实施定；分类器输出需结构化（拒/不拒 + 理由枚举）。
- **history API 路由命名/分页/限长** —— 具体形态由 planner 定（长会话可限最近 N 轮 + 游标，与 D10-B3 预算口径对齐）。
- **投影层渲染位置** —— verdictJSON → conclusionText 渲染与 refusalReason → 中性占位的转换落在 `internal/history` 投影层（session_turn 保持 raw 存储，对 eval/调试友好）；具体函数拆分由实施定。
- **data-decision 新类型的命名** —— `history_query_rewritten` / `history_budget_truncated` / `history_projected` 为示意，实施时可调整（保持 decision 字符串语义清晰即可）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 9 裁决输入（本 phase 的直接前置）
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-06-SUMMARY.md` §Escape plan — 逃逸方案原文（拒答判定不依赖历史 / 历史不携带拒答语义 / query-only 分类器候选）+ NO-GO 数字（refusal -95pp、SC-3 零穿透 0/10 PASS）
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-CONTEXT.md` — D9-B1/B2/B3 投影契约 + D9-V1/V2/V3 裁决 + spike 设计全集
- `.planning/seeds/spike-history-injection-trust-floor.md` — 假设 H1/H2/H3 原文（H2 = 拒答上下文漂移，已证实）
- `.planning/research/PITFALLS.md` §Pitfall 1（H1-H3 + 修复策略）/ §Pitfall 2（tier 穿透）/ §Pitfall 9（摘要压缩 trigger）

### 需求与范围
- `.planning/REQUIREMENTS.md` §MEM-01..05 — 5 条需求原文（指代解析 / 跨刷新投影 / 白名单代码强制 / 投影消毒 / 预算压缩）
- `.planning/ROADMAP.md` §Phase 10 — Goal / 5 Success Criteria / Pitfalls addressed / RF-6
- `.planning/PROJECT.md` §Core Value — 贯穿性不变量：citation 白名单 = 本轮检索，代码强制

### 已交付核心资产（直接扩展）
- `internal/history/projection.go` — `Project(prev []store.SessionMessage) []schema.Message` + `scrubChunkRefs`（D9-B1/B2/B3 已实现）；本 phase 扩展为完整包（verdict 渲染 + 中性占位 + 预算窗口）
- `internal/history/projection_test_fixtures.go` — 7 条导出 fixture 契约（非 test-only，D9-B4）
- `internal/store/sqlite/session.go` — `GetSession` / `AppendTurn`（SC-9 session_id 分区已锁）；session_turn 存 raw verdictJSON / 裸 refusalReason
- `internal/store/sqlite/schema.go` — session_turn DDL（UNIQUE(session_id, role, ts) + idx_session_turn_sid）
- `internal/server/pipeline.go` — VerdictPipeline（生产默认）：`buildPrompt` 双模板 / `retrievalScoreThreshold=0.3` 代码短路 / SC-2 0-chunks 拒答；本 phase 的注入落点
- `internal/server/chat.go` — `chatHandler` 路由 / `runAgentPath`（GetSession 已调用但不喂 agent 的现状）/ `writeAuditAndTurn`（verdictJSON 入 session_turn 的路径）
- `cmd/spike-history/` — 重门二进制（main / runner / long / spike_row / summary）；末位 plan 直接复用

### 不变量层（本 phase 不得破坏）
- `internal/verdict/verdict.go` + `citation.go` + `grounding.go` — D2-01 grounding moat / D2-04 Citation{ChunkID, Span, Tier} / Normalize 规范化
- `internal/server/chat_session_test.go` — 2-session isolation 范式（TestSessionIsolation50x5 重跑绿的基准形态）
- `eval/frozen/compliance.lock.json` — v0.8 基线 pin（sha256 50bb1688，D10-A4 保档不动）

### eval 基础设施（末位重跑用）
- `eval/golden/compliance.jsonl` — 75 例（50 baseline + 10 adversarial + 15 followup，category 字段区分）
- `internal/eval/` — Judge / 4 Metrics / LoadGolden（spike runner 复用）
- `eval/README.md` — D6-15 layered tolerance（live 1e-3 层）

### FE 资产（D10-D1/D2 落点）
- `web/src/App.tsx` — useChat + `id: activeSessionId`（in-memory chatStore，本 phase 加 initialMessages 恢复）
- `web/src/stores/app-store.ts` — D14 禁 messages 字段约束
- `web/src/components/`（VerdictCard / CitationCluster / SkillPicker 等）— 完整恢复复用的渲染组件
- `web/src/lib/real-chat-transport.ts` — RealChatTransport（SSE → UIMessageChunk 解析层）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/history/projection.go`** — 投影契约已实现（Project + scrubChunkRefs）；本 phase 在此扩展 verdictJSON 确定性渲染、中性占位模板、token 预算窗口。7 条导出 fixture 是行为契约。
- **`GetSession` / `AppendTurn`** — sqlite 会话读写已就位（SC-9 分区）；数据流 = writeAuditAndTurn 每轮写 audit 行 + session turn 对（assistant 存 raw verdictJSON / refusalReason）。
- **data-decision SSE 基底（D16）** — `wire.DecisionPayload` + FE 解析通道现成；D10-B4/C3 只加 decision 类型字符串，零 wire 结构变更。
- **`cmd/spike-history/`** — 末位重跑直接复用（11-flag CLI + RunEntry 双路径 + summary 裁决），无需新建。
- **VerdictCard / CitationCluster / chunk-lookup（FE）** — 完整恢复的渲染组件全部现成；VerdictJSON 从 history API 返回后走现有渲染管线。
- **拒答短路面（SC-2 / score<0.3）** — 代码级拒答已存在且天然不看历史；分类器是其上游的语义拒答层，两者互补不冲突。

### Established Patterns
- **D9-B1/B2/B3 投影契约** — role 过滤（仅 user/assistant 纯文本）/ user 侧 CHUNK 扫描 / 窗口截断（本 phase 从 3 轮硬限改为预算驱动，D10-B3）。
- **D14 Zustand 禁 messages** — 恢复历史必须走 useChat initialMessages，不得引入 store.messages。
- **D-04 transport-swap / D15 wire 类型编译期告警** — FE 改动不得破坏 transport-swap 测试与 SkillForgeChunk 联合类型。
- **D11/D18 interface-first** — history 包扩展沿用「接口在父包、实现在子包」模式。
- **D6-15 layered tolerance** — 末位重跑走 live 层（真 GLM judge，1e-3）。

### Integration Points
- **`chatHandler` → `VerdictPipeline`**（`internal/server/pipeline.go`）— 历史注入、rewrite、拒答分类器都挂在这条链上：rewrite → 检索 → 分类器 → buildPrompt（多轮 messages 前缀）→ streamAndAssemble。
- **`streamAndAssemble`** — 现为单 system + 单 user 双消息；扩展为「历史前缀 + 双消息」。
- **新 history API** — `GET /api/session/:id/history`（命名可调），返回 role + 渲染文本 + 解析后 verdict + skill_id；Hertz 路由挂 `internal/server/`。
- **audit 行扩展** — rewrite 后的 query / 分类器决策可入 data-decision 流 + log（audit 表 schema 不动，T-3-AUDIT INSERT-only）。

</code_context>

<specifics>
## Specific Ideas

- **分类器物理隔离是原则不是实现细节** —— 分类器的输入清单（query + 本轮检索 chunks）在代码层面杜绝历史进入；任何后续需求（如「给分类器看上轮答案」）都应被视为违反 D9-V2 条款 (a) 而拒绝。
- **rewrite 输出不进证据链** —— 改写文本只喂检索和分类器；生成层的证据仍然只有本轮 chunks（MEM-03 白名单不变量的自然推论）。
- **session_turn 保持 raw 存储、渲染在投影层** —— 存储层不动（eval/调试需要 raw verdictJSON）；verdictJSON → conclusionText、refusalReason → 中性占位的确定性渲染全部收敛在 `internal/history` 投影层——这也是 GEVL-08 消毒规则的既定口径。
- **GLM 多轮 messages 原生遵循 > 文本拼接** —— D10-B2 的依据；Phase 11 Runner 迁移后 messages 形态直接复用，无二次设计。
- **每轮独立检索 + 判定是系统既有形态** —— 历史只影响指代解析（rewrite）与生成上下文（messages 前缀）；「用户补充信息后重问」自然走完整新管线（重新检索重新判定），无需特殊逻辑。
- **spike 重跑结果与 Phase 9 逐例可比** —— 同 seed=42、同 35 例、同门；恶意组锚点（history_anchor 字段）让任何残余穿透可 1:1 定位到具体 turn。

</specifics>

<deferred>
## Deferred Ideas

- **Phase 11 Runner 迁移 + summarization middleware 启用** — 历史注入从 VerdictPipeline 搬到 ADK `Agent.Run` messages；MEM-05 的 summarize 阶段真正启用（本 phase 只有 project + 哑预算截断）。
- **frozen eval 基线 v2 重锁** — v0.8 milestone 收尾统一做（Phase 11-13 行为变更全部落地后）。
- **resume 注入文本的白名单强制复用** — Phase 11 resume 回放整条消息列表时，复用 MEM-03 的本轮检索白名单机制（不变量已声明，实施归 Phase 11）。
- **子 agent 输出的白名单** — Phase 13 FAN-* 的输出同样不能成为证据（不变量已声明，实施归 Phase 13）。
- **哑预算的 tokenizer 精确化** — RF-6 完全解决（GLM tokenizer 可配置或官方计数 API）后替换字符近似；非本 phase 阻塞项。
- **Phase 7 遗留（07-02 T3 / 07-05 T2/T3 / 07-06）** — 用户决策推迟到 v0.8 后，本 phase 不触碰。

</deferred>

---

*Phase: 10-p8-1*
*Context gathered: 2026-08-18*
