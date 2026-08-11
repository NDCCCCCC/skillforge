# Phase 3: Agentic Retrieval & Trust Evaluation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `03-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-08-11
**Phase:** 3-Agentic-Retrieval-Trust-Evaluation
**Areas discussed:** Agent 自主决策架构, Self-RAG 自验证机制, Eval 护城河 & 数据来源, 审计日志 + 会话记忆存储

---

## Agent 自主决策架构 (ARAG-01 + ARAG-02)

### Q1 — Agent 自主(ARAG-01) 与 拒答构造式(D-09) 如何共存？

| Option | Description | Selected |
|--------|-------------|----------|
| C 混合 | 全 tool-calling agent(检索为 tool、多跳原生、兑现 AGNT-02) + Go 信任地板(全 hop tier 匹配 chunk=0 → 强制拒答) + GroundingCheck | ✓ |
| A 分层 | 薄 LLM 路由器分类意图 + 底层现有确定性检索/拒答 | |
| B 全 tool-calling | 检索为 tool 的全 agent 循环；moat 部分依赖 LLM 自律拒答 (D-01 风险) | |
| 你决定 | | |

**User's choice:** C 混合
**Notes:** 决定整条流水线形状。C 既最大化 Eino ADK agent 工程卖点，又完整保留 D-09 确定性护城河。

### Q2 — 多跳(ARAG-02)怎么触发、怎么封顶？

| Option | Description | Selected |
|--------|-------------|----------|
| Agent 自判 + 硬封顶 2 | ADK agent 自判信息不足→再跳；Go 硬封 max_hops=2(1 初始+1 再跳, 满足 SC-2)；agent 自然重写查询 | ✓ |
| Go 覆盖信号强制再跳 | hop-1 后 Go 检测低覆盖→强制 hop-2 | |
| 双触发 | agent + Go 都能触发 | |
| 你决定 | | |

**User's choice:** Agent 自判 + 硬封顶 2

---

## Self-RAG 自验证机制 (ARAG-03)

### Q1 — Self-RAG 走哪种机制？

| Option | Description | Selected |
|--------|-------------|----------|
| C 分层 | Go GroundingCheck 恒为硬地板；可选 LLM critique 只追加 review_required(抓语义反转)，不能 override 地板 | ✓ |
| A 纯确定性 Go 门 | 前移成自验证 pass；抓不到"逐字命中却语义反转"盲点 | |
| B LLM-as-judge | 第二趟 LLM 逐条判；LLM 审自己 = D-01 不可依赖 | |
| 你决定 | | |

**User's choice:** C 分层
**Notes:** 关键动机 = 子串 grounding 的已知盲点（chunk 说 X，agent 引用支持 ¬X，子串过得了但 claim 错）。LLM critique 专补此洞。

### Q2 — LLM critique 何时跑？

| Option | Description | Selected |
|--------|-------------|----------|
| 门控跑 | 仅 Go 地板通过 + 置信度不确定带(0.6–0.9)或 需补充/多跳 时跑；高置信单跳/拒答不跑 | ✓ |
| 总跑 | 每非拒答 verdict 都跑 (~2x LLM 成本) | |
| 你决定 | | |

**User's choice:** 门控跑

---

## Eval 护城河 & 数据来源 (GEVL-04/05 + D2-15)

### Q1 — 50-case golden set 怎么来？

| Option | Description | Selected |
|--------|-------------|----------|
| 纯手写合成 | 扩展 Phase 2 corpus，构造对抗/拒答/多跳三类(GEVL-05)；确定、可复现、零授权风险 | ✓ |
| 合成 + 少量真实 | | |
| 你决定 | | |

**User's choice:** 纯手写合成

### Q2 — Ragas judge 用哪个模型？

| Option | Description | Selected |
|--------|-------------|----------|
| GLM 默认 + 可配位 | GLM(内网唯一可达) 默认 + judge_model 配置位 + 人工 spot-check 反自评偏差 | ✓ |
| 双模型交叉 | | |
| GLM 单评判 | | |
| 你决定 | | |

**User's choice:** GLM 默认 + 可配位

### Q3 — Sidecar 形态 + D2-15 校准范围？

| Option | Description | Selected |
|--------|-------------|----------|
| CLI 批处理 + 产出校准数据 | Go JSONL → Python 离线 Ragas → JSON 报告(SC-6) + 给 0.3 阈值实测建议值(D2-15)；CI 门 GEVL-07 留 Phase 6 | ✓ |
| HTTP 常驻服务 | | |
| 你决定 | | |

**User's choice:** CLI 批处理 + 产出校准数据

---

## 审计日志 + 会话记忆存储 (GEVL-03 + AGNT-05)

### Q1 — 存储怎么定？（用户先反问：DB 选择是否影响企业级定位）

Claude 先回答了定位问题（v1 产品能力不影响；但有观感/信号效应；照搬 D11 抽象可四全其美），再重问。

| Option | Description | Selected |
|--------|-------------|----------|
| SQLite 默认 + 抽象 | store.Store 接口 + SQLite v1 默认 + PG Phase 6 drop-in (照搬 D11) | ✓ |
| 直接 PostgreSQL | | |
| 纯 SQLite 不抽象 | | |
| 你决定 | | |

**User's choice:** SQLite 默认 + 抽象
**Notes:** 用户提出的"企业级定位"反问是本区域的关键 moment——Claude 给出"D11 抽象 = 工程成熟度信号 > 无脑选 PG"的判断后用户采纳。

### Q2 — 会话记忆生命周期？

| Option | Description | Selected |
|--------|-------------|----------|
| 持久化到 SQLite | | |
| 内存 map | | |
| TTL ephemeral | | |
| 你决定 | ✓ |

**User's choice:** 你决定 → Claude discretion：持久化到 store（session_id 分区隔离）

---

## Claude's Discretion

- **会话记忆生命周期** (AGNT-05)：持久化到 store（session_id 分区；SC-9 并发验证零泄露；重启保留；为 Phase 6 retention 打基础）
- **Trust-floor 计数细节**：什么算"全 hop tier 匹配 chunk = 0"（跨 hop 去重、低分非零是否计）
- **Critique 不确定带阈值**：0.6–0.9 为示意值，由 eval 数据校准
- **Golden-set 三类比例**：researcher 提议，应超配 refusal-required（moat 头牌行为）
- **`prompt_hash` 口径** (GEVL-03)：rendered system prompt + skill id + model ver 的哈希
- **KNOW-02 tier 强制层级**：Go indexer 层（backend-agnostic，与 D7 一致）
- **`store.Store` 接口方法集**：planner 设计（PutAudit / GetSession / AppendTurn / SQL 查询助手）
- **`max_hops=2` 封顶语义**：到顶必须出答或拒答，无第三跳

## Deferred Ideas

- CI 门 GEVL-07（faithfulness<0.9 阻断）→ Phase 6
- Eval dashboard (GEVL-06) → Phase 6
- HTTP 常驻 eval 服务 → Phase 6
- PostgreSQL 生产后端实现 → Phase 6（本阶段留 PG-ready 抽象）
- retention / redaction → Phase 6 SC-5（D3-10 已为其打基础）
- 真实用户采样 golden set → Phase 6
- BGE-reranker（若 golden set 检索精度不足）→ researcher 验证，实现可能在本阶段或延期
- LLM-generated suggested_alternative_query（非拒答场景）→ Phase 5（D2-14）
- 多 Skill 语义路由 → Phase 5
- counter-evidence / redline / policy version (CMPL-06/07/08) → Phase 5
