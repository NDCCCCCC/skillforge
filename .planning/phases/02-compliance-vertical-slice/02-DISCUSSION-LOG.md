# Phase 2: Compliance Vertical Slice - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-11
**Phase:** 2-Compliance-Vertical-Slice
**Areas discussed:** 裁决生成严格度, Citation span 格式, Skill 注册与 API, 合规语料来源

---

## 裁决生成严格度 (Verdict Generation & Grounding)

| Option | Description | Selected |
|--------|-------------|----------|
| 程序化校验 | LLM 出裁决后，Go 代码逐条验证每个 citation 的 span 确实出现在检索到的 chunk 原文（归一化匹配），不符的 claim 剔除。最强幻觉防御、确定性、可单测。 | ✓ |
| 两步 LLM 自检 | LLM 出草稿→第二遍 LLM 逐条核对 chunk 文本，剔除未支撑项。比单次强，但 GLM 自检 GLM 非独立，可能漏检；成本~2x。 | |
| 单次调用 | 检索 chunk 注入 prompt，模型自引 citation，一次出裁决。最轻最快（最接近 spike 003），但依赖模型诚实，正是 P-COMPL-1 风险。 | |

**User's choice:** 程序化校验 (推荐)
**Notes:** 锁定了 P-COMPL-1 幻觉护城河的实现方式——确定性 Go 校验而非 LLM 自检，契合"语义确定性正解"哲学。

**Sub-decision — 未支撑 claim 处理:**

| Option | Description | Selected |
|--------|-------------|----------|
| 降级为需补充 | 保留有据的 合规/不合规 结论，把未支撑 claim 标 review_required、置信度封顶<0.9，沿用现成 需补充 状态。 | ✓ |
| 整体拒答 | 任何一条 claim 未支撑→整个裁决转为结构化拒答。最保守，但可能过于激进。 | |
| 静默剔除 | 删掉该 claim，其余正常输出。用户无感知，审计上不推荐。 | |

**User's choice:** 降级为需补充 (推荐)

---

## Citation span 格式 (Citation Contract)

> 领域 1 选了"程序化校验"，直接锁定 span 必须是逐字引用（否则 Go 无法做子串匹配）。本领域剩两个子决策。

**Sub-decision — citation 对象字段:**

| Option | Description | Selected |
|--------|-------------|----------|
| 最小集 {chunk_id,span,tier} | 恰好满足 SC-3。Phase 4 citation 面板用 chunk_id 回查原文 + tier badge。score/effective_date 留给 Phase 3/5。 | ✓ |
| + retrieval score | 同时带检索相似度分，便于 GEVL-01 拒答阈值埋点。TierSearchResult 已有 Score，成本低。 | |
| + score + effective_date | 为 Phase 5 CMPL-08 预埋，但需现在改 chunk schema，超出纵向切片范围。 | |

**User's choice:** 最小集 {chunk_id,span,tier} (推荐)

**Sub-decision — 匹配严格度:**

| Option | Description | Selected |
|--------|-------------|----------|
| 归一化匹配 | 统一空白+全角→半角标点+去首尾后，再做子串包含。避免 CN 全角/半角假阴性，仍是精确子串检查。 | ✓ |
| 精确子串 | runes 完全相等才认。最严格，但 CN 全角/半角混用易假阴性。 | |
| 模糊(编辑距离) | 允许少量字符差异。最宽松，但可能放过近似臆造，削弱护城河。 | |

**User's choice:** 归一化匹配 (推荐)

---

## Skill 注册与 API (Skill Registration & API)

**Sub-decision — 注册模型:**

| Option | Description | Selected |
|--------|-------------|----------|
| 文件驱动 | skills/*/SKILL.md 唯一真源，启动扫盘加载，GET 列出，POST 仅 dry-run 校验。"丢个文件即注册"字面成真；Phase 5 fsnotify 自然延续。 | ✓ |
| API+文件双通道 | POST 写盘+注册，文件落盘也触发同一 watcher。DX 更全但双写需处理一致性。 | |
| API-only | POST 持久化到 DB/config。背离职文件型 Skill 哲学与热加载承诺。 | |

**User's choice:** 文件驱动 (推荐)

**Sub-decision — 查询端点形态:**

| Option | Description | Selected |
|--------|-------------|----------|
| POST /api/chat | 通用聊天端点，skill 在 body 指定。为 Phase 5 SYSK-05 语义路由预留(URL 不变)。契合 Vercel AI SDK v5 useChat 默认端点。 | ✓ |
| POST /api/skills/{id}/query | 显式 per-skill 端点。语义清晰，但 Phase 5 加多 Skill 路由时 URL 设计会变。 | |
| POST /api/compliance/query | 硬编码单 Skill 端点。Phase 2 最快，但 Phase 5 必须重构。最短视。 | |

**User's choice:** POST /api/chat (推荐)
**Notes:** 同时确定新建 cmd/server Hertz 常驻服务，cmd/spiked 保留作回归二进制。

---

## 合规语料来源 (Compliance Corpus)

**Sub-decision — 语料来源:**

| Option | Description | Selected |
|--------|-------------|----------|
| 手写小型权威语料 | 5-10 篇制度/约 30 chunks，覆盖 spike 5 查询含拒答用例。确定→eval 可复现、安全、可 demo。Phase 3 再扩到 50 cases。 | ✓ |
| 真实法规文本 | 劳动法/公司报销制度原文。最真实，但有许可/准确性顾虑(STATE 开放问题#2)。 | |
| 合成但逼真 | 生成一本假"公司制度手册"含重叠条款。比手写快但质量/覆盖难控。 | |

**User's choice:** 手写小型权威语料 (推荐)

**Sub-decision — 目录约定:**

| Option | Description | Selected |
|--------|-------------|----------|
| 独立 knowledge/ 分层 | knowledge/authority/*.md 为权威层真源，skills/compliance/ 只放 SKILL.md + reads 声明。indexer 扫 knowledge/{tier}/ 入 Milvus。延续 denova 真源分离。 | ✓ |
| skills/compliance/data/ | Skill 自带语料。Skill 与数据耦合，多 Skill 共享权威层会重复。 | |
| 保留 testdata/docs/ 扩展 | testdata 语义=测试夹具，非 demo 语料；Phase 7 还要迁。 | |

**User's choice:** 独立 knowledge/ 分层 (推荐)

---

## Claude's Discretion

用户选择"直接生成 CONTEXT"，以下细节交由 researcher/planner + Claude 裁量（已写入 CONTEXT.md D2-13/14/15 与 Claude's Discretion 段）：

- **拒答分类法** (D2-13)：reason 闭合集合 = `{no_authorized_source, low_retrieval_score, out_of_scope}`；结构化拒答 shape 按 ARAG-05。
- **suggested_alternative_query 生成方式** (D2-14)：LLM 生成 + 规则兜底。
- **检索分拒答阈值** (D2-15)：Phase 2 用保守占位值(cosine < 0.3)，Phase 3 实测校准；review_required 置信度阈值 SC-5 已钉死 < 0.9。
- spike 资产晋升到生产包的具体布局；`reads: authority/**` → tier `权威` 的映射；grounding 是否覆盖"无 citation 的新 claim"；5 查询集合（建议复用 spike 002/003）。

## Deferred Ideas

- 多 Skill 语义路由 (SYSK-05) — Phase 5
- 自主检索决策 / 多跳 / 完整 Self-RAG (ARAG-01/02/03) — Phase 3
- 流式信任 UI (FEUI-*) — Phase 4
- 版本追踪 / 反证 / redline (CMPL-06/07/08) — Phase 5
- Ragas 评估管线 + 50-case 金标集 + 审计 SQL (GEVL-03/04/05) — Phase 3/6
- 热加载 (SYSK-04, fsnotify) — Phase 5

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` (score 0.6, 关键词误报) — Phase 1 已完成；SYSK-08 是其在 Phase 2 的延续，不重新 spike。
