# Phase 10: 多轮对话记忆（P8.1） - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-18
**Phase:** 10-多轮对话记忆（P8.1）
**Areas discussed:** 信任地板防漂移机制, 历史注入落点 + Runner 时序, 指代解析策略（MEM-01）, 刷新后 FE 历史恢复（MEM-02 UI）

---

## 信任地板防漂移机制

### Q1: 拒答判定与历史怎么解耦？

| Option | Description | Selected |
|--------|-------------|----------|
| query-only 拒答分类器 | 拒答前置为独立 LLM 调用：输入 = query + 本轮检索 chunks（物理上看不到历史），先判该不该拒，通过后才走历史增强生成。保留语义拒答能力（PII 类）；代价 = 每轮多一次轻量 LLM 调用。D9-V2 条款 (c) 的直接落地。 | ✓ |
| 代码级全接管 | 拒答 100% 代码判定：SC-2 短路 + score<0.3 扩展覆盖所有拒答场景，LLM verdict 层删除 refused 职责。零额外调用零漂移，但丢失语义拒答（检索意外命中时 PII 类漏拒）。 | |
| 双保险 | 分类器（语义拒答）+ 代码短路（结构性拒答）都在历史注入前判定；LLM 生成层的 refused 职责移除。最稳但实现面最大。 | |

**User's choice:** query-only 拒答分类器
**Notes:** spike 证明 prompt 层写规则不足以防 LLM 在历史注入后停止拒答，需要结构性隔离。

### Q2: 历史中已经发生过的拒答轮，投影后以什么形态进入 prompt？

| Option | Description | Selected |
|--------|-------------|----------|
| 完全丢弃拒答轮 | 拒答轮不入历史。最彻底的「历史不携带拒答语义」；代价 = 「刚才为什么拒绝」追问时 agent 失忆。 | |
| 中性占位模板 | 拒答轮渲染为固定模板（D9-L2 filler 同风格，零拒答语义词）。保留对话连续性；模板措辞需红测验证。 | ✓ |
| 保留完整拒答文本 | 拒答轮照常经 refusalText 渲染入历史（含拒答理由）。对话体验最好；但 spike 已证明这是漂移源头，需完全依赖分类器前置守门。 | |

**User's choice:** 中性占位模板
**Notes:** 固定字符串、零拒答语义词（拒绝/不能/无权）进入；与 D9-L2 filler 模板同风格。

### Q3: spike 硬门重跑怎么安排？

| Option | Description | Selected |
|--------|-------------|----------|
| 末位 plan 全量重跑 | 最后一个 plan = live 重跑全量 35 例（10 恶意 + 15 善意 + 10 长会话，真 GLM + Milvus，同 seed=42）。与 Phase 9 同门同口径；成本 ~$0.05 + ~75min。 | ✓ |
| 中途验证 + 末位终跑 | 强化机制落地后先跑 25 例短历史中途验证方向，末位再全量。方向错误早发现，但多一次 live 成本。 | |
| 只跑 followup 子集 | 只重跑 15 例 followup（拒答漂移直接触发场景）。成本最低，但回归验证不完整。 | |

**User's choice:** 末位 plan 全量重跑
**Notes:** 四指标 + refusal delta 全部回门才宣告 phase 完成。

### Q4: 拒答分类器改变拒答行为路径后，frozen eval 基线怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| Phase 10 建新基线 v2 | 分类器落地后重跑 cmd/eval --frozen，以有版本的新 lock 留档。行为变化有档可查；末位 plan 多一个 live 验收项。 | |
| spike 门独立把关 | 旧 lock 保档不动；行为变化由 spike 硬门单独把关，CI gate 红则记录豁免理由；v2 重锁推迟到 v0.8 收尾。 | ✓ |
| 双基线并行 | v1 + v2 双 lock 可跑。可回归对比但维护面翻倍，v1 意义存疑。 | |

**User's choice:** spike 门独立把关
**Notes:** T-8-12 禁覆盖旧 lock；v2 重锁推迟到 v0.8 milestone 收尾统一做。

---

## 历史注入落点 + Runner 时序

### Q1: Phase 10 的历史注入落在哪条管线上？

| Option | Description | Selected |
|--------|-------------|----------|
| VerdictPipeline | 落现有生产默认路径，投影输出在 pipeline 内渲染进 prompt。改动集中、回归套件做安全网、spike 重门同口径。Phase 11 Runner 迁移时搬一次家（投影输出已是 Message 形态）。 | ✓ |
| 直接迁 ADK Runner | 切生产默认到 ADK Runner，历史作为 messages 喂 Agent.Run，summarization middleware 可用。一步到位但 Runner 路径未过生产验证 + 同时改两件事调试面翻倍。 | |
| 落 VP + 迁移预留层 | VerdictPipeline 落地 + 顶部抽象薄接口。额外设计成本，但投影输出已是 Message 形态——抽象层可能多余。 | |

**User's choice:** VerdictPipeline
**Notes:** Runner 迁移本身归 Phase 11（ROADMAP Phase 11 Goal 原文「Runner 迁移落在简单 ReAct archetype 上」）。

### Q2: 消毒后的历史以什么消息结构进入 LLM 请求？

| Option | Description | Selected |
|--------|-------------|----------|
| 多轮 messages 前缀 | 历史轮次作为原生 messages 前缀（user/assistant 交替），最后一条 user = query + chunks DATA 不变，system 单条不变。GLM 原生多轮遵循最好。 | ✓ |
| 单 user 三段式 | 历史拼接成文本块塞进现有 user 消息。改动最小但指代解析弱——spike 注入近似此形态。 | |
| system 摘要框 | 历史压缩为摘要句拼进 system。token 最省但丢轮次结构。 | |

**User's choice:** 多轮 messages 前缀
**Notes:** streamAndAssemble 从双消息扩展为「前缀 + 双消息」。

### Q3: Phase 10 发布时，历史窗口策略是什么？

| Option | Description | Selected |
|--------|-------------|----------|
| token 预算驱动 | 预算内尽量多轮，超限从最旧轮截断（project 阶段）；3 轮降为保底口径。长会话体验自然；末位重门长会话子集不过门则回退收紧。 | ✓ |
| 固定 3 轮硬限 | 保持 D9-B3 不变。最保守（spike 口径）但长会话只记住 3 轮。 | |
| 轮数可配置 | env 可调，默认 3。灵活但默认仍保守，长会话体验问题未解决。 | |

**User's choice:** token 预算驱动
**Notes:** 哑预算先行（RF-6 字符近似）；summarize 阶段留待 Phase 11 middleware。

### Q4: token 预算触发截断/压缩时，事件怎么暴露？

| Option | Description | Selected |
|--------|-------------|----------|
| data-decision 扩展 | 复用 D16 通道新增 decision 类型（history_budget_truncated 等），FE 可选渲染 + log 同步。零新 wire 类型。 | ✓ |
| 仅后端 log | 结构化一行日志。最轻但用户不可见。 | |
| 新 SSE chunk 类型 | 独立 data-history-budget 类型 + FE 提示条。感知最 explicit 但 wire 双侧维护面增加。 | |

**User's choice:** data-decision 扩展
**Notes:** 与 ARAG-01 决策可观测同一通道，审计链完整。

---

## 指代解析策略（MEM-01）

### Q1: 追问轮的检索 query 怎么消解指代？

| Option | Description | Selected |
|--------|-------------|----------|
| query rewrite 前置 | 追问轮先轻量 LLM 改写（看历史 → 独立完整 query）→ 改写 query 走检索 + 分类器；生成层仍给原 query + 历史前缀。首轮跳过。 | ✓ |
| 原 query 直检 | 检索用原 query，指代解析全靠生成层。「那第二条呢」检索 miss → SC-2 拒答，MEM-01 SC 不过。 | |
| 零成本拼接 | 原 query + 最近 assistant 轮文本拼接检索。便宜但 embedding 被污染。 | |

**User's choice:** query rewrite 前置
**Notes:** conversational query rewriting 是标准解；spike followup 子类 context_recall -87% 部分根因即检索 miss。

### Q2: rewrite、检索、拒答分类器的执行顺序怎么定？

| Option | Description | Selected |
|--------|-------------|----------|
| 先 rewrite 后分类 | rewrite（看历史）→ 改写 query 检索 → 分类器（看改写 query + 本轮 chunks）→ 生成。分类器判定准；rewrite 输出不进证据链。 | ✓ |
| 先分类后 rewrite | 分类器先看原始追问——指代未消解无法判定，误判率高。 | |
| 分类器双 query 输入 | 分类器同时拿原 + 改写 query。最稳但 prompt 复杂度上升。 | |

**User's choice:** 先 rewrite 后分类
**Notes:** rewrite 输出仅用于检索/分类，不进证据链——恶意历史污染改写文本时，检索不到 chunk 仍拒答（结构性兜底）。

### Q3: 改写后的 query 要不要暴露给用户？

| Option | Description | Selected |
|--------|-------------|----------|
| 透明提示 | data-decision 下发 + FE 小字提示（「已理解为：XX 制度第二条」）。用户能发现误解并纠正；audit 留档。 | ✓ |
| 静默内部 | 纯内部机制。界面干净但误解时用户无法诊断。 | |
| 仅审计留档 | audit 可查但 FE 不渲染。UI hint: yes 要求下价值有限。 | |

**User's choice:** 透明提示
**Notes:** 信任型产品（合规场景）黑盒决策与平台调性冲突。

---

## 刷新后 FE 历史恢复（MEM-02 UI）

### Q1: 页面刷新后，会话历史在 FE 怎么呈现？

| Option | Description | Selected |
|--------|-------------|----------|
| 完整恢复 | 新 API 返回结构化历史，useChat initialMessages 恢复（D14），现有 VerdictCard/CitationCluster 渲染，引用可点。与 live 消息视觉一致。 | ✓ |
| 后端记忆前端白板 | 刷新后空聊天，新追问仍正确。实现最小但与 UI hint 矛盾。 | |
| 简化文本恢复 | 纯文本气泡 + 「历史对话（简化展示）」标注。实现中等但视觉割裂。 | |

**User's choice:** 完整恢复
**Notes:** session_turn 存 raw verdictJSON（数据完整），投影层确定性渲染；FE 组件全部现成。

### Q2: 刷新/切换回旧会话时，FE 当前 Skill 与会话绑定的 Skill 怎么对齐？

| Option | Description | Selected |
|--------|-------------|----------|
| 自动切到会话 Skill | history API 返回 skill_id，SkillPicker 同步选中，不一致时切换并提示。检索库始终与会话对齐。 | ✓ |
| 保持当前 Skill | 历史属于 Skill A、检索走 Skill B 的库，指代解析跨库错位。 | |
| 不匹配则阻断 | 弹提示要求手动切。安全但体验繁琐。 | |

**User's choice:** 自动切到会话 Skill
**Notes:** 与后端 chat 请求带 skill 字段的现有契约一致。

---

## Claude's Discretion

- 中性占位模板具体措辞（须过红测：恶意注入下不触发拒答漂移）
- 哑预算 token 估算口径（字符近似 + 余量常数，末位重门校准）
- rewrite 失败/超时降级（best-effort 用原 query，不阻断）
- rewrite 与分类器 prompt 形态（轻量、结构化输出）
- history API 路由命名/分页/限长
- 投影层渲染函数拆分（渲染收敛在 internal/history，存储层不动）
- data-decision 新类型命名（保持语义清晰即可）

## Deferred Ideas

- Phase 11 Runner 迁移 + summarization middleware 启用（历史注入搬家到 Agent.Run messages）
- frozen eval 基线 v2 重锁（v0.8 收尾统一）
- resume 注入文本白名单强制复用（Phase 11）
- 子 agent 输出白名单（Phase 13）
- 哑预算 tokenizer 精确化（RF-6 完全解决后）
- Phase 7 遗留（07-02 T3 / 07-05 T2/T3 / 07-06 — 用户决策推迟到 v0.8 后）
