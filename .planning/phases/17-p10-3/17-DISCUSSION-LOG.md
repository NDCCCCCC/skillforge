# Phase 17: KPI 仪表盘 Skill (P10.3) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-27
**Phase:** 17-p10-3
**Areas discussed:** A (KPI Skill 编排 + 路由策略), B (Sparkline + wire), C (下钻 + 多 KPI 编排), D (GEVL-09/10 + 再识别)

---

## Area A — KPI Skill 编排 + 路由策略

| Option | Description | Selected |
|--------|-------------|----------|
| Skill steps 显式路由表（推荐） | Skill 的 `steps:` 写一个显式路由表（业务词 → KPI name）；LLM 读 steps 自行路由 | ✓ |
| kpi_call description 强化 + LLM 推理 | description 内嵌完整路由键 + 近义词；LLM 依靠 tool-calling 推理 | |
| 独立 router 子 agent（Conditional） | ConditionalAgent 先做意图分类再调对应 Skill | |
| 路由表在 description + Skill 简单透传 | 两者合二：description 内嵌完整路由表，Skill steps 只调 kpi_call | |

**User's choice:** Skill steps 显式路由表（推荐）
**Locked:** D17-A1

| Option | Description | Selected |
|--------|-------------|----------|
| 单一 KPI Dashboard Skill | 一个 Skill 服务所有数字问答，steps 路由表按业务域分组 | ✓ (claude's pick) |
| 按域拆分（loans / customers / risk 等） | 多 Skill，router 先选 Skill | |
| 一个 shell Skill + 业务域子 Skill 组合 | Shell 路由 + 子 Skill 执行 | |

**User's choice:** 使用最佳方案（=单一 KPI Dashboard Skill）
**Locked:** D17-A2 — 单一 KPI Dashboard Skill，按业务域分组路由表防止线性膨胀

| Option | Description | Selected |
|--------|-------------|----------|
| LLM 决策 + 标注不定可追问（推荐） | LLM 自决走最可能，加 `confidence_note` 字段可追问 | ✓ |
| 拒答 + 列出候选 KPI 让用户选 | 多 KPI 可解时拒答列候选 | |
| 步骤内加 verifier 子步骤调取多个对照 | 调多 KPI 对照 + 选兼容值 | |

**User's choice:** LLM 决策 + 标注不定可追问（推荐）
**Locked:** D17-A3

**Implicit A4:** Answer shape = LLM 自然语言 + 内联 KPI 定义/时间窗/as_of + 每个数字挂一个 data-reference part（不强制 verdict envelope）

---

## Area B — Sparkline 渲染 + 多点 data-reference wire

| Option | Description | Selected |
|--------|-------------|----------|
| 纯内联 SVG 手写（推荐） | 手写 SVG 元素（polyline/circle），零依赖，bundle 零增量 | ✓ |
| 复用现有 recharts | 复用 Phase 6 eval TrendPanel 已在用的 recharts | |
| 轻量 sparkline 专用库 | ad-hoc 轻量库如 react-sparklines | |

**User's choice:** 纯内联 SVG 手写（推荐）
**Locked:** D17-B1

| Option | Description | Selected |
|--------|-------------|----------|
| data-reference 数组化（推荐） | 扩 schema 携带 `references: [{point_index, ...}, ...]`，单 part 装载所有点 | ✓ |
| 嵌套 sparkline wrapper part | 新增 `{type: 'sparkline', points, references}` part 类型 | |
| 多 part 连续 emit（每点一个） | 每个数据点 emit 独立 data-reference part | |

**User's choice:** data-reference 数组化（推荐）
**Locked:** D17-B2 — 不扩 wire union，仅字段扩展

| Option | Description | Selected |
|--------|-------------|----------|
| 单 KPI 多周期调用，一次 emit（推荐） | 一次多周期 KPI（periods 数组），单 data-reference part 一次性 emit | ✓ |
| 调多 KPI 函数拼凑 + 串行 emit | 调多个单点 KPI，N 个独立 data-reference part 串行 | |
| 单 KPI + 周期参数（FEATURES Cap 3） | KPI 接受多 period 一次查询，返回 N 行 | |

**User's choice:** 单 KPI 多周期调用，一次 emit（推荐）
**Locked:** D17-B3 — `periods: array` 参数即 FEATURES Cap 3 落地

**Implicit B4:** Per-point hover 复用 DASH-02 hover 模式（同一 popup 显示 value + period + 共享 as_of + 通道名）

---

## Area C — 下钻 UX + 多 KPI 组合

| Option | Description | Selected |
|--------|-------------|----------|
| 点 chip → Skill 重新调多周期 KPI（推荐） | 点 chip → Skill 重调同 KPI 多周期版 | ✓ |
| 展开面板（in-place） | 同 message 内 in-place 展开 mini-panel | |
| 聊天追问（自然语言驱动） | 预填文本到 chat input 需用户点发送 | |
| 双路：chip + 追问均可 | 两个 chip 出现 + 预填 | |

**User's choice:** 点 chip → Skill 重新跳多周期 KPI（推荐）
**Locked:** D17-C1

| Option | Description | Selected |
|--------|-------------|----------|
| 文本 inline + hover（推荐） | 主体自然语言文本，每个数字内联 + hover popup | ✓ |
| 多 KPI 卡片网格（Grid） | 每个 KPI 一个卡片，网格状排列 | |
| 主体文本 + 补充区 sparkline | 主体文本 + 详细 KPI 区 sparkline/表 | |

**User's choice:** 文本 inline + hover（推荐）
**Locked:** D17-C2

| Option | Description | Selected |
|--------|-------------|----------|
| Skill steps 模板提示 + LLM tool-call（推荐） | steps 列出组合模式提示，LLM 靠 tool-call 实现 | ✓ |
| Skill steps 硬约束调用顺序 | Skill steps 把多 KPI 组合变为可控状态机 | |
| Skill 不加组合提示，纯靠 LLM | Skill 不设组合提示 | |

**User's choice:** Skill steps 模板提示 + LLM tool-call（推荐）
**Locked:** D17-C3 — 镜像 D11-B1 的两层结构

---

## Area D — GEVL-09/10 + 再识别

| Option | Description | Selected |
|--------|-------------|----------|
| 合成数据 + 合成 KPI（推荐） | 合成 mini KPI 注册表 + 1 万行伪数据 | ✓ |
| 复用真实 loans.yaml + 脱敏合成数据 | 复用 Phase 15/16 资产 | |
| 真实企业脱敏数据 | 需外部金融 IT 部门 | |

**User's choice:** 合成数据 + 合成 KPI（推荐）
**Locked:** D17-D1

| Option | Description | Selected |
|--------|-------------|----------|
| 3 步组合判定（推荐） | 去格式化 + 容差 ±0.5% + allowed_expressions 三步判定 | ✓ |
| 严格字符串相等（不接受转换） | LLM 输出与期望严格匹配 | |
| 仅容差 ±0.5%，不限表达 | 只定义容差，不限表达 | |

**User's choice:** 3 步组合判定（推荐）
**Locked:** D17-D2

| Option | Description | Selected |
|--------|-------------|----------|
| 独立 reid 子集 + golden 互不干扰（推荐） | 独立 `eval/frozen/reid.jsonl`，与 GEVL-09 主集分离 | ✓ |
| 混入 GEVL-09 主集 | 攻击例进 50 例主集 | |
| 不进 frozen，unit test 独立交付 | REID 走 unit test | |

**User's choice:** 独立 reid 子集 + golden 互不干扰（推荐）
**Locked:** D17-D3

| Option | Description | Selected |
|--------|-------------|----------|
| 新独立 kpi.lock.json（推荐） | 新建 `eval/frozen/kpi.lock.json`，与现有 5 锁同级 | ✓ |
| 扩 compliance.lock.json + 新 action | 扩 compliance.lock.json schema 加 KPI 子段 | |
| 新建 data.lock.json（预留多 phase 合并） | 新建 data.lock.json 为多 phase 共用 | |

**User's choice:** 新独立 kpi.lock.json（推荐）
**Locked:** D17-D4

---

## Claude's Discretion

- 多周期 KPI 函数 `periods` 参数 schema（FEATURES Cap 3 落地细节）
- Sparkline 维度/配色 token + viewBox 自适应
- Chip 文案生成策略（KPI 定义文件 `drill_chips` 字段）
- kpi_calls 审计行的多周期快照格式
- eval/frozen/reid.jsonl 与 kpi.lock.json 的关联方式
- examples/skills/kpi-dashboard/SKILL.md 字段细节
- GEVL-09 50 例 query 文案构造方法
- Skill steps 路由表 schema（YAML 结构 vs Markdown 表）

## Deferred Ideas

- 多轮趋势组合（趋势之上）— v1.1+
- 真实企业脱敏数据 — v1.1+
- 业务月报 Skill — Phase 18
- 静态报告门户 + cron — Phase 19
- KPI 差分运行时动态检测 — Phase 17 eval 验证后决策
- 混合 Skill（tier + data）— v1 互斥
- admin KPI 只读视图 + 实时 dashboard — Phase 20
- data.lock.json 跨 phase 合并决策 — Phase 19
- Skill steps 路由表 schema 标准化 — Claude's discretion
- Skill steps verifier 子步骤 — A3 拒绝项