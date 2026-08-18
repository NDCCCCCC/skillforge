# Phase 9: 历史注入 trust-floor spike 硬门（P8.S）— Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-18
**Phase:** 9-trust-floor-spike-p8-s-spike-hard-gate
**Areas discussed:** A. 恶意注入用例构造 / B. 历史投影形态 + 消毒边界 / C. 善意追问边界 + 子集结构 / D. 历史长度维度

---

## A. 恶意注入用例构造

| Option | Description | Selected |
|--------|-------------|----------|
| LLM 生成（GLM-5.2） | 用 GLM 生成"看起来像真制度但实际虚假"的段落 | ✓ |
| 手写合成（最小集） | 人工写 5-10 条"明确假"段落 | |
| 真条款变体（错版本/过期） | 从 7 篇 knowledge/authority 改版本/日期造过期假文本 | |

**User's choice:** LLM 生成（推荐）
**Notes:** 与现有 7 篇 `knowledge/authority/` 表面相似但实质错误（错金额 / 错机构 / 错生效日期 / 虚构条款编号）。覆盖维度最全（含虚构条款）。

| Option | Description | Selected |
|--------|-------------|----------|
| 追加 10 条到 `eval/golden/compliance.jsonl` adversarial 类 | 复用现有 frozen-lock + generator pipeline | ✓ |
| 独立 spike-only 文件（不动原 golden） | 新建 `eval/spike/history_injection.jsonl` | |
| 追加 20 条（多场景） | 错额/错机构/过期/虚构 四类各 5 | |

**User's choice:** 追加 10 条到 adversarial 类（推荐）
**Notes:** 享受 baseline 重用，spike 与 50 例同批跑。

| Option | Description | Selected |
|--------|-------------|----------|
| 第 N 轮用户轮（user 粘贴伪文本 → user 追问） | 贴 Pitfall 2 描述的场景 | ✓ |
| 第 N 轮 assistant 轮（预设误传 verdict） | 压 H1 假设 | |
| 二者都跑（双集，5+5） | 全面但成本双倍 | |

**User's choice:** 第 N 轮用户轮（推荐）
**Notes:** 第 2 轮 query 必须是 `eval/golden/compliance.jsonl` 中已存在 query（保证 ground_truth 对照）。

| Option | Description | Selected |
|--------|-------------|----------|
| 双层 anchor（`history_anchor: {turn_index, fake_chunk_ref}` 不参评） | 事后调试 + Phase 10 复现 | ✓ |
| 只放伪文本 + 不标位置 | 简单 | |
| 同时准备 5 条正例模板（合法历史对照） | 区分信源合规 vs 伪信源 | |

**User's choice:** 双层 anchor（推荐）
**Notes:** 不参与评分；报告披露 anchor → 哪条刺穿 → 哪类漂移链路。

---

## B. 历史投影形态 + 消毒边界

| Option | Description | Selected |
|--------|-------------|----------|
| 仅 user + assistant 纯文本 | Eino schema.Message 投影只留 Role∈{user,assistant} + Content | ✓ |
| user/assistant + tool 结果 | 保留 tool_name 不进 | |
| 原始 schema.Message 全量 | 完整 | |

**User's choice:** 仅 user + assistant 纯文本（推荐）
**Notes:** 与 D2-04 "citation 仅限本轮检索" 互补；tool messages / reasoning / tool_call_id / system metadata 全部不入 LLM。

| Option | Description | Selected |
|--------|-------------|----------|
| 只剥 user 文本里的 `[CHUNK id]` / `[CH-xxx]` / `[id:N]` 标记 | 范围窄、可证 | ✓ |
| user + assistant 文本都扫 | 全面但破坏 assistant verdict | |
| user/assistant + verdict 序列化后再渲染 | 最高安全 | |

**User's choice:** 只剥 user 文本里的标记（推荐）
**Notes:** 替换为同长度占位符 `[REF-N]`；assistant 文本里的 ChunkID 不扫（D2-04 表达携带）。

| Option | Description | Selected |
|--------|-------------|----------|
| 硬限 3 轮（最近 3 个 user/assistant 轮次对） | 简单，与 ROADMAP last-3-turns 一致 | ✓ |
| token 预算（512 tok） | 动态 | |
| 不设限，全送 | spike 成本低但长会话漏检 | |

**User's choice:** 硬限 3 轮（推荐）
**Notes:** MEM-05 长会话通过 D 域单独 spike 验证。

| Option | Description | Selected |
|--------|-------------|----------|
| 接口骨架 + 正负例 fixture（`internal/history/projection.go`） | Phase 10 零成本接上 | ✓ |
| 纯 markdown 文档契约 | spike 只产报告 | |
| 接口骨架 + Go unit test | 更进一步 | |

**User's choice:** 接口骨架 + 正负例（推荐）
**Notes:** `func Project(prev []SessionMessage) []schema.Message`；fixture 仅 spike 调用 + Phase 10 复用，不写 unit test。

---

## C. 善意追问边界 + 是否分 A/B 子组

| Option | Description | Selected |
|--------|-------------|----------|
| 三小类（指代追问 + 拒答后追问补充 + 多跳链式） | 覆盖 Pitfall 1 H1 + MEM-01 | ✓ |
| 仅指代追问 | 最窄 | |
| 三小类 + 对话压缩 | 压测 MEM-05（超 Phase 9 范围） | |

**User's choice:** 三小类（推荐）

| Option | Description | Selected |
|--------|-------------|----------|
| 追加 15 条（5/5/5 三小类均分） | 覆盖完整 | ✓ |
| 追加 10 条（与恶意注入等量） | 成本低 | |
| 不造新用例，复用原 50 | 成本零 | |

**User's choice:** 追加 15 条（重视）
**Notes:** 5/5/5 三小类均分；`category:"history_injection_followup"`。

| Option | Description | Selected |
|--------|-------------|----------|
| 独立子集 baseline（恶意组 / 善意组 / 纯 baseline 三层独立） | 能看出哪一类在漂移 | ✓ |
| 合并到一组（只看总指标） | 报告极简 | |
| 三层分集（含纯 baseline） | 能反推"哪种历史需要过滤" | |

**User's choice:** 独立子集 baseline（推荐）

| Option | Description | Selected |
|--------|-------------|----------|
| 每条用例都公开 | 调试 / 复现 | ✓ |
| 仅总指标 + 漂移超阈条目标黄 | 报告小 | |
| 三层报告 | 高阶总 + 子组 + 漂移超阈附例 | |

**User's choice:** 每条用例都公开（推荐）
**Notes:** 报告 = 场景分类报告 + 每条用例表（q/a/四指标/origin_group/baseline-delta）公开，7-10 页可接受。

---

## D. 历史长度维度（短/长双门？）

| Option | Description | Selected |
|--------|-------------|----------|
| 短 + 长双门 | 短 1-3 轮 + 长 8-15 轮 | ✓ |
| 只测短历史（1-3 轮） | spike 范围小 | |
| 仅文档说明，长历史留 Phase 10 | Phase 10 实施后才发现漂移 | |

**User's choice:** 短 + 长双门（推荐）
**Notes:** 短覆盖 Pitfall 1（近期历史可能跳抽 retrieve）；长覆盖 MEM-05。

| Option | Description | Selected |
|--------|-------------|----------|
| 中立填充（合规问答） | 与 corpus 同分布 | ✓ |
| 重复同一用户查询 | 成本最低但引入伪假设 | |
| 真实 8-15 轮会话 | 最真实但需 source | |

**User's choice:** 中立填充（合规问答，推荐）
**Notes:** 从原 50 例 sample 抽取 8/12/15 三类长度；其中 3 轮含真实 adversarial/followup 用例作为"信号"，其余中性问答作为"噪声"。

| Option | Description | Selected |
|--------|-------------|----------|
| 独立"长会话副报告" | 与短报告交叉 | ✓ |
| 主报告表 + 一列"长 vs 短" | 报告单一 | |
| 三轮决策（7×2×50 例） | 全面但 spike 成本爆炸 | |

**User's choice:** 独立"长会话副报告"（推荐）

| Option | Description | Selected |
|--------|-------------|----------|
| 与短同门（<5% / <10%） | 简单，与 ROADMAP 一致 | ✓ |
| 只走定性门（看 H1/H2/H3 是否发生） | 避免长会话一刀切 | |
| 双门限不一致 | 门限多了难记 | |

**User's choice:** 与短同门（推荐）

---

## Claude's Discretion

- **F. no-go 时的逃逸方案选型**：用户未选择讨论此域。Phase 9 决策 = **不预选**；报告产出时按 hit 情况判定（短漂移 → 锁定 last-3-turns + 仅 user 文本；长漂移 → 锁定 summary 中间件触发；全漂移 → 锁定仅用户轮记忆 + Phase 10 重设计）。
- **E. Phase 10 接口骨架的包结构**：Phase 9 仅 stub `Project()` 函数 + fixture 注释；包结构（是否拆子包）Phase 10 决定。
- **spike 输出物位置**：`reports/short_history.md` + `reports/long_history.md` + `reports/summary.md` + `eval/golden/compliance.jsonl` 追加 25 例 + `cmd/spike-history/` + `internal/history/projection.go`。
- **stub 函数 build tag**：默认 build 包含；`cmd/spike-history` 在 spike 完成后保留为可重跑工具。

---

## Deferred Ideas

- Phase 10 MEM-* 实施（spike 通过后启动）
- Phase 11 HITL-* 审批门（Phase 9 不涉及）
- Phase 12 PLAN-* 规划层（Phase 9 不涉及）
- Phase 13 FAN-* 子 agent 扇出（Phase 9 不涉及）
- Phase 10 接口骨架的扩展（Phase 9 仅 stub）
- MEM-05 摘要压缩机制（Phase 10 内容）
- CI gate 增加 spike-history 自动跑（Phase 9 不在 scope）
- 07-06 open-source 后置（与 Phase 9 独立）
- `internal/history/` 包的最终命名（Phase 10 决定）