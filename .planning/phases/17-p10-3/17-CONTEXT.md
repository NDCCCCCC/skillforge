# Phase 17: KPI 仪表盘 Skill（P10.3）- Context

**Gathered:** 2026-08-27
**Status:** Ready for planning
**Source:** `/gsd-discuss-phase 17` — Default mode (4 areas × ~4 questions = 16 decisions)
**Requirements:** DASH-01, DASH-02, DASH-03, DASH-04, DASH-05, GEVL-09, GEVL-10 (7 v1.0 requirements) — 5 Success Criteria (ROADMAP §Phase 17)

<domain>
## Phase Boundary

业务人员用自然语言在聊天里问数字问题（「上月贷款余额多少」「2026-Q2 逾期率是多少」），KPI Dashboard Skill 经 `kpi_call` 工具调用作答——答案含**数值 + KPI 定义 + 计算时间窗 + as_of + 数据引用**（hover 显示通道名 `data:aggregated_only`），内嵌 sparkline 迷你趋势（每点独立引用），下钻永不触达原始行。GEVL-09（KPI 调用准确率，50 例程序判定）+ GEVL-10（答案数字与所引 KPI 快照逐个一致）+ 再识别断言（聚合 join + KPI 差分）进冻结集。

**本 phase 交付：**
- 单一 `kpi-dashboard` Skill（`examples/skills/kpi-dashboard/SKILL.md`）：reads `["data:aggregated_only"]`，steps 显式路由表（业务词→KPI 名映射，按业务域分组）+ 模板组合提示 + `kpi_call` 工具调用
- 多周期 KPI 函数支持：参数 `periods: [period, ...]` 一次返回多行（FEATURES Cap 3 落地）
- sparkline 渲染：纯内联 SVG 手写（零新增依赖）+ 消息内嵌，UI 组件 `<Sparkline>` 在 chat 消息主体内
- data-reference part 数组化：`{references: [{point_index, period, value, k_min_applied?}, ...]}` 单一 SSE part 装载 sparkline 所有点引用；沿用 D16 data-decision 通道，不扩 wire union
- 多 KPI 答案形态：LLM 自然语言文本 + 每个数字挂一个独立 data-reference part（SSE 多 part 流式），UI 文本 inline + hover 复用 DASH-02 hover popup
- 下钻交互：消息内点 chip「看月度趋势」→ Skill 重调同 KPI 多周期版（同 KPI 函数，参数从单 period 变 `periods: [最近 N 月]`），无需新 SSE part 类型
- GEVL-09 50 例 golden set：合成 mini KPI 注册表（3-5 KPI）+ DuckDB 合成 1 万行伪数据，50 例自然语言查询 query 期望 KPI 名+数值（去千分位/单位 + 容差 ±0.5% + 允许表达列表三步判定）
- GEVL-10 引用一致性：答案数字必须与所引 KPI 快照 rows 逐个匹配（同三步判定）+ as_of 必出现在答案文本
- 再识别冻结子集：独立 `eval/frozen/reid.jsonl`（不混入 GEVL-09 主集），Pitfall 11（跨数据集 join / 同数据集多 KPI 差集攻击）+ Pitfall 22（KPI 差分还原个体）两类用例
- 新冻结锁 `eval/frozen/kpi.lock.json`：5 字段 pin（model_ver / prompt_hash / seed / eval_set_version / skill_name）+ KPI 注册表内容哈希
- Skill `kpi-dashboard` 自动加载：`internal/skill/loader.go` 启动期 fsnotify 扫描 `examples/skills/`，与现有 `examples/skills/compliance/` 同级

**不交付（deferred）：**
- 业务月报 Skill（多 KPI 编排报告形态）— Phase 18
- 静态报告门户 + cron 调度 — Phase 19
- 导入管理 UI / KPI 只读视图 / Skill 管理入口 — Phase 20 admin
- 混合 Skill（tier 知识 + data 数据并读）— v1 互斥（D16-C1）
- per-Skill KPI 子集 — 拒绝（D16-C4）
- 运行时差分计数防线 — Phase 17 eval 验证枚举白名单是否足够后决策（D16-D2 拒绝项）
- 真实企业脱敏数据 — v1.1+（本 phase 合成数据足够）
- KPI 注册表改动 — Phase 16 已锁，本 phase 只读消费
- 多轮趋势组合（趋势之上的趋势）— v1.1+（复杂度高）

</domain>

<decisions>
## Implementation Decisions

### A. KPI Skill 编排 + 路由策略（DASH-01/02）

- **D17-A1: Skill steps 显式路由表。** `kpi-dashboard/SKILL.md` 的 `steps:` 写一个按业务域分组的路由表：
  ```
  routes:
    loans:
      - triggers: [贷款余额, 未偿还本金, loan balance, outstanding]
        kpi: KPI_LOAN_BALANCE
      - triggers: [不良率, NPL ratio]
        kpi: KPI_NPL_RATIO
    customers:
      - triggers: [客户数, 活跃客户, customer count]
        kpi: KPI_CUSTOMER_COUNT
  ```
  Skill 作者控制映射点（diff 评审纪律），LLM 读 steps 自行选 KPI。拒绝：description 强化（token 重 + 路由准确率依赖 description 质量）、router 子 agent（违反 D16-C4 singleton 哲学 + 1 KPI 1 Skill 增殖）。
- **D17-A2: 单一 KPI Dashboard Skill。** 一个 Skill 服务所有数字问答。steps 路由表按业务域分组（loans/customers/risk），防止线性膨胀。理由：用户不必挑 Skill → 零摩擦；与 D16-C4 singleton registry 哲学一致；v1 demo KPI 数量有限。Phase 18 月报 Skill 同样适用单 Skill 形态。拒绝：按域拆 Skill（路由多一层）、shell + 子 Skill 组合（多一层 Skill 加载 + eval 套件）。
- **D17-A3: 歧义 = LLM 自决 + 标注不定可追问。** 「贷款余额」多 KPI 可解时 LLM 选最可能解，answer 中加 `confidence_note` 字段（"该问题多个 KPI 可解，按 X 业务口径回答；若你想问 Y 可追问"）。零阻断交互。拒绝：拒答列候选（阻断式摩擦）、调多 KPI 对照（成本 + eval 复杂）。
- **D17-A4: Answer shape = LLM 自然语言 + data-reference parts。** 不强制 verdict-like 结构化 envelope。Skill 输出自然语言文本，含数值 + KPI 定义 + 时间窗 + as_of（"上月（2026-07）贷款余额 4,231.5 亿元，按 KPI_LOAN_BALANCE 计算（as_of 2026-08-01）"）；每个数字挂一个独立 data-reference part，UI inline 高亮 + hover popup。理由：单值答案 vs 多 KPI 答案结构差异大；data-reference part 已承载结构化溯源层；与现有 chat 习惯一致。

### B. Sparkline 渲染 + 多点 data-reference wire（DASH-03）

- **D17-B1: 纯内联 SVG 手写。** `<Sparkline points={[{x, y, value, period}]} onPointHover={...} />` 输出 SVG `<polyline>` + `<circle>` per point。Tailwind class 控制 color/size。零新增依赖；bundle 零增量；hover/click 原生事件。拒绝：recharts（chat 内"杀鸡用牛刀"，带 AxisLabel 等冗余面）、react-sparklines（多一依赖 + 定制能力有限，无法每点独立引用）。
- **D17-B2: data-reference part 数组化。** 沿用 D16 data-decision 通道，不扩 UIMessageChunk union。data-reference part 新增 `references: [{point_index, period, value, k_min_applied?}, ...]` 字段，单 part 装载 sparkline 所有点引用。UI 逐点 hover 拿对应 index 的引用展示 popup。理由：SSE part 计数少（sparkline = 1 part 不是 N）、点引用点对点（kpi_calls 审计 1 调用 = N 点可追）、零 wire union 扩张。拒绝：嵌套 sparkline wrapper part（扩 union 触发 wire-type 编译期报警）、多 part 串行 emit（N 个 SSE 帧 + 乱序风险）。
- **D17-B3: 单 KPI 多周期调用，一次 emit。** kpi_call 调一次多周期 KPI 函数（参数 `periods: ["2025-08", "2025-09", ..., "2026-07"]`），返回 N 行，data-reference part 一次性 emit 包含 N 个 references。理由：1 kpi_calls 审计行 = 1 调用 = N 点可追、调用代价低、点序天然有序。拒绝：调多单点 KPI 拼凑（N 调用代价 + N 审计 + 流式乱序）、单 KPI 接受 period 数组参数（实为同一形态，FEATURES Cap 3 落地）。
- **D17-B4: Per-point hover 复用 DASH-02 popup。** 同一引用 popup 显示 value + period + 共享 as_of + 通道名 `data:aggregated_only` + KPI 函数名 + args。零分叉：单值 hover 与 sparkline 点 hover 同一组件，UI 行为一致。

### C. 下钻 UX + 多 KPI 组合编排（DASH-04/05）

- **D17-C1: 下钻 = 点 chip → Skill 重调多周期 KPI。** 数字下方内嵌 chip「看月度趋势」「看同比」，点 chip → Skill 重调同 KPI 多周期版（参数从单 period 变 `periods: [最近 12 月]`）。Skill 决定 chip 语义，UI 只渲染。理由：1 Skill + 1 工具路径，与 chat 驱动下钻同逻辑；Skill 作者控制哪些 chip 出现。拒绝：in-place 展开面板（需新 SSE part 类型 + skill 状态机）、chat 追问预填（用户多一步）、双路并存（用户纠结）。
- **D17-C2: 多 KPI 答案 = 文本 inline + hover。** 多 KPI 答案主体是自然语言文本，每个数字挂独立 data-reference part inline 高亮 + hover。布局选择：与现有 chat 文本主导体验一致；引用点对点；零 wire union 扩张。拒绝：多 KPI 卡片网格（与 chat 习惯不一致）、主体文本 + 补充区 sparkline（两段划分需明确动画/分隔逻辑）。
- **D17-C3: Skill steps 模板提示 + LLM tool-call 编排。** Skill steps 列出常见业务组合模式提示（「如三指标组合『余额+不良+收入』可同时调以下 KPI…」），但**不**硬约束调用顺序。LLM 依靠 kpi_call tool call 实现多次调用。理由：Skill 作者表达业务偏好（哪些 KPI 组合合理）但保留 LLM 灵活度；与 Phase 16 「kpi_call 单一 Tool」哲学一致。拒绝：硬约束调用顺序（Skill 步骤变重，违反 kpi_call 哲学）、Skill 不加组合提示（业务偏好无法表达）。

### D. GEVL-09/10 eval 实现 + 再识别断言

- **D17-D1: 合成数据 + 合成 KPI。** phase 交付时合成 mini KPI 注册表（3-5 KPI：KPI_LOAN_BALANCE / KPI_NPL_RATIO / KPI_INTEREST_INCOME / KPI_CUSTOMER_COUNT / KPI_LOAN_BALANCE_TREND 多周期版）+ DuckDB 合成 1 万行伪贷款/客户数据进 `data/staging/testdata.duckdb`。理由：phase 代码可脱敏验证、零外部依赖；EvalSuiteFixture 不污染真实生产数据。拒绝：复用 loans.yaml + 脱敏数据（依赖 Phase 15 资产质量）、真实企业数据（需外部运维，本 phase 不可阻断）。
- **D17-D2: 3 步组合数值相等判定。** Golden set 每例预定义 `expected_kpi`（函数名+参数）+ `expected_value`（KPI 快照数值）+ `allowed_expressions: ["4,231.5亿", "423150000000", ...]`。判定三步：1）去千分位/单位前缀后数值与 expected_value 差 ≤ 0.5% ✓；2）如未通过看是否命中 allowed_expressions 列表 ✓；3）都不过 FAIL。理由：兼顾严谨（数学容差）+ 表达宽容（业务人员说"4,231 亿"也行）+ 调试可读（allowed_expressions 数组就是断言白名单）。拒绝：严格字符串相等（LLM 表达变种轻易 FAIL）、仅容差（业务表达进不了 eval）。
- **D17-D3: 独立 reid 子集 + golden 互不干扰。** 独立 `eval/frozen/reid.jsonl`（不混入 GEVL-09 主 50 例）。每例 = 攻击 query（跨数据集 join / KPI 差集）+ 期望结果（`should_refuse: true` 或 `all_values_suppressed: true` 或 `min_suppression_count >= k_min`）。理由：REID 护航与准确率两套指标语义互不干扰、REID 子集可独立升级。拒绝：混入 GEVL-09 主集（拉低调用准确率指标）、unit test 独立交付（不算 GEVL-09/10 可复现指标，不进 audit lock）。
- **D17-D4: 新独立 `kpi.lock.json`。** `eval/frozen/kpi.lock.json` pin 5 字段：`model_ver` / `prompt_hash` / `seed` / `eval_set_version` / `skill_name` + 额外 `kpi_registry_sha256`（注册表内容哈希，含所有 KPI 定义文件聚合哈希）。与 compliance/fanout/hitl/platform-floor/deep 同级。理由：phase 锁逻辑隔离、Phase 18/19 收尾可独立决定是否合并 data.lock.json。拒绝：扩 compliance.lock.json（不同 Skill 多样，混锁臃肿）、新建 data.lock.json 预占位（提前进位置关系设定，Phase 18 同步评估负担）。

### Claude's Discretion

- **多周期 KPI 函数 `periods` 参数 schema**（FEATURES Cap 3 落地：`{type: array, items: {type: string, pattern: <月份正则>}, minItems: 2, maxItems: 24}`；loader 静态校验）
- **Sparkline 维度/配色 token**（SVG viewBox/width/height 默认值、stroke/circle 颜色与 Tailwind v4 token 映射）
- **Chip 文案生成策略**（KPI 定义文件可声明 `drill_chips: [{label, periods_template}]`；无声明则默认生成「看月度趋势」chip）
- **Sparkline 维度自适应**（viewBox 内自动 scale 还是固定坐标轴 + Y 轴动态 padding）
- **kpi_calls 审计行的多周期快照格式**（rows 数组按 period 排序 vs 按调用顺序）
- **eval/frozen/reid.jsonl 与 kpi.lock.json 的关联方式**（REID 子集引用主 KPI 注册表 sha256，单独 lock vs 合并主锁的 sub-section）
- **`examples/skills/kpi-dashboard/SKILL.md` 与 examples/skills/compliance/ 的差异**（无 verdict envelope 字段、温度默认 0、guardrails 侧重"绝不暴露非聚合数字"）
- **GEVL-09 50 例 query 文案构造**（合成 vs LLM 改写 vs 模板化）
- **Skill steps 路由表 schema**（YAML 结构 vs Markdown 表；YAML 更程序可读）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 研究文档（实现级已研究到 wire/UX）
- `.planning/research/FEATURES.md` Capability 3（sparkline 渲染形态，ROADMAP Phase 17 明确引用）— note: FEATURES.md 当前版本 Capability 3 = Planning Layer（Phase 12 已交付），sparkline 形态需结合 Phase 6 recharts 经验与 D17-B1 决策重新评估（已 lock）
- `.planning/research/PITFALLS.md` Pitfall 11 — 聚合 join 再识别（k_min=5 推荐值；本 phase 收口冻结断言）
- `.planning/research/PITFALLS.md` Pitfall 22 — KPI 差分再识别（参数白名单 + 差分成本门；本 phase 收口再识别子集）
- `.planning/research/PITFALLS.md` Pitfall 19 — eval 审计漂移（每 phase 贡献 data-eval golden，本 phase 建立首块）
- `.planning/research/STACK.md` — recharts (Phase 6 已在用；本 phase D17-B1 决定不复用)
- `.planning/research/ARCHITECTURE.md` — Q2 kpi_call Tool archetype（同 Phase 16 D16-B1）

### 需求 + 路线图锚
- `.planning/REQUIREMENTS.md` §KPI 仪表盘 Skill (DASH) — DASH-01..05 原文（chat 数字问答 / sparkline / 下钻 / 多 KPI 组合）
- `.planning/REQUIREMENTS.md` §Evaluation Metrics — GEVL-09（KPI 调用准确率 50 例）+ GEVL-10（faithfulness 数据维度扩展）
- `.planning/REQUIREMENTS.md` §v1.0 Requirements — DATA-18（data.lock.json 冻结锁，Phase 19 收尾，本 phase 贡献 kpi.lock.json）
- `.planning/ROADMAP.md` §Phase 17 — Goal / 5 SC / Pitfalls / Research flags（sparkline 渲染形态 + golden set 50 例构造来源 plan 期定，已 lock）

### 前置 phase 决策（直接继承）
- `.planning/phases/16-p10-2/16-CONTEXT.md` — D16-A1 YAML KPI 注册表 / D16-A2 SQL 模板 / D16-A3 枚举值域 / D16-A4 启动期编译 / D16-B1 单一 kpi_call / D16-B2 no_matching_kpi / D16-B3 data-decision 通道 + data-reference part / D16-B4 as_of 系统自动生成 / D16-C1 reads 互斥 / D16-C2/C3 加载期+运行时双层 / D16-C4 singleton registry / D16-D1 k_min 行级抑制 / D16-D2 枚举白名单 / D16-D3 全量快照冻结 / D16-D4 kpi_calls INSERT-only 表
- `.planning/phases/15-p10-1/15-CONTEXT.md` — D15-A1 受控 YAML 哲学 / D15-A4 规则哈希进审计 / D15-D1 定宽桶跨期稳定 / D15-D3 k_min 全局 5 + 数据集 override
- `.planning/phases/14-p10-0-cgo-trust-floor/14-CONTEXT.md` — D14-D1 每数据集一 .duckdb 物理隔离 / D14-D2 sqlite datasets 注册表 / D14-D4 staging 原子改名
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-CONTEXT.md` — D11-B1 运行时动态拼 Tool whitelist（D16-C3 镜像对象）

### 已交付核心资产（直接扩展）
- `internal/kpi/registry.go` + `internal/kpi/loader.go` + `internal/kpi/define.go` + `internal/kpi/validate_sql.go` + `internal/kpi/lexicon.go` — Phase 16 完整 KPI 注册/校验基建（D17 本 phase 直接消费；只读不改）
- `internal/kpi/kmin.go` — k_min 行级抑制实现（GEVL-09/10 golden 测试验证行为）
- `internal/skill/loader.go` + `internal/skill/registry.go` + `internal/skill/manifest.go` — Skill 加载/注册表/fsnotify + reads 值域扩 `data:aggregated_only` 模式（kpi-dashboard Skill 落点）
- `internal/agent/tools/retrieve.go` — kpi_call 镜像工厂模式（多周期参数扩展示例）
- `internal/store/sqlite/kpi_calls.go` — kpi_calls INSERT-only 表（GEVL-10 引用一致性 + kpi.lock.json 关联）
- `internal/agent/runner_sse.go`（D16 data-decision chunk 通道）— data-reference part `references[]` 数组扩展点
- `web/src/components/chat/MessageBubble.tsx` — data-decision part 渲染现成模式（sparkline 组件落点）
- `eval/frozen/compliance.jsonl` + `compliance.lock.json` — golden set + lock 模式（D17-D2/D4 镜像）
- `examples/skills/compliance/SKILL.md` — Skill 7-field YAML 模板（kpi-dashboard Skill 镜像）
- `internal/data/desensitize.go` `KMinDefault` + datasets 表 `k_min_override` — k_min 读取面（GEVL-09 测试验证）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/kpi/` 完整六件套**（registry/loader/define/validate_sql/lexicon/kmin）— Phase 16 完整交付；本 phase 直接消费，无需新增 KPI 注册基建；多周期 `periods` 参数只需在 define.go schema 段加 `type: array` 分支
- **`internal/agent/runner_sse.go` data-decision 通道** — 现成 SSE 通道机制；data-reference part 加 `references:[]` 字段是字段扩展，不动 wire union（Phase 4 wire-type 编译期报警不动）
- **`internal/store/sqlite/kpi_calls.go`** — INSERT-only 审计表 + TestBatchAuditRowShape 模式；GEVL-10 引用一致性测试可直查 `kpi_calls` 行比对快照
- **`internal/skill/` 7-field validator + fsnotify Registry** — kpi-dashboard SKILL.md 加载 + 热重载现成；步骤路由表只需在 SKILL.md YAML 内合法字段
- **`examples/skills/compliance/SKILL.md`** — 7-field 模板镜像对象（kpi-dashboard 同级放置）；temperature=0 + reads=`data:aggregated_only` + writes=`[]` 形态一致
- **`web/src/components/chat/MessageBubble.tsx` data-decision 分支** — sparkline 组件落点：识别 data-reference part 的 `references[]` 数组 + 渲染 inline 数字 hover + sparkline 嵌入

### Established Patterns
- **D11-B1/B3 双层执法** — Skill steps 模板提示（"哪些 KPI 组合合理"，静态声明）+ LLM tool-call 实现（动态编排）= 镜像 D11-B1 的两层结构
- **D16-B3 data-decision part 不扩 union** — D17-B2 数组化是该原则的扩展落地（字段扩展而非类型扩展）；wire-type 编译期报警保持绿
- **T-3-AUDIT INSERT-only + parameterized** — kpi_calls 表照复；多周期快照 rows 数组参数化写入
- **fail-closed 加载期校验** — Skill 7-field validator + datarules loader + KPI loader 三先例；kpi-dashboard 加载期校验 fail-closed 规则延用
- **D14-D4 staging 原子改名** — testdata.duckdb 落 `data/staging/testdata.duckdb` 用同一 staging 模式；合成数据导入走 Phase 15 已交付的 import 管线（同源）
- **D8-04 WalkDir 禁令门** — 可复刻为「kpi-dashboard Skill 路径无 SQL 构造面」结构性断言（与 D16 TestNoSQLGenerationSurface 同构）
- **eval/frozen/compliance.jsonl + lock.json 模式** — D17 golden set + lock.json 镜像；TestShippedFrozenLockIsInSync 字节身份验证模式照搬

### Integration Points
- **`internal/skill/loader.go` Skill 扫描** — `examples/skills/` 下新增 `kpi-dashboard/SKILL.md` 自动被 fsnotify 拾取（Phase 5 已交付）
- **`internal/skill/manifest.go` reads 值域校验** — `data:aggregated_only` 模式已注册（D16-C2 已落地）；kpi-dashboard Skill 加载期校验自动过
- **kpi_calls 表 DDL** — sqlite 表已建（D16-D4）；多周期 `periods` 参数调用自然生成多行 rows 快照
- **SSE data-reference part `references[]` 数组扩展** — `internal/agent/runner_sse.go` 字段添加；web `MessageBubble.tsx` 渲染分支添加；wire union 不动
- **`<Sparkline>` React 组件** — 新建于 `web/src/components/chat/Sparkline.tsx`；接收 points[] + onPointHover handler；纯 SVG 输出
- **chip 渲染** — web `MessageBubble.tsx` data-reference part 渲染时，根据 KPI 定义文件 `drill_chips` 字段渲染内嵌 chip；onClick 触发新一轮 chat 消息预填「请调 KPI_X periods=...」（用户点发送确认；或自动发送由 Claude's discretion 决定）
- **`eval/frozen/kpi.lock.json`** — 新建；与现有 5 锁同级，pin 字段含 KPI 注册表 sha256
- **`eval/frozen/reid.jsonl`** — 新建子集；每行攻击 query + 期望结果

</code_context>

<specifics>
## Specific Ideas

- **"业务人员聊天里问数字 → 数字 + 全出处 + sparkline + 下钻" 是平台叙事的核心卖点** — 数字可溯源（DASH-02）+ 趋势可视化（DASH-03）+ 下钻可控（DASH-04）是「数据支撑业务」的最直观表达，与 seed 文件 KPI 仪表盘 Skill 的定位一致
- **「sparkline 每点独立引用」是引用一致性的最强解释** — 答案中每个数字 hover 都拿到一致引用，与合规 Skill 的"逐字引用"同构哲学
- **Skill steps 显式路由表 = "Skill 作者的 diff 评审纪律" 延伸到数据问答层** — 与 D15-A1 受控文件哲学一致：作者表达业务偏好，LLM 灵活执行
- **合成数据 + 合成 KPI 让 phase 17 完全自包含可验证** — 不依赖企业数据 + 不依赖生产 KPI 注册表；任何开发者 fork 后能跑 GEVL-09/10 全套门
- **GEVL-09/10 + REID 三套冻结锁的并存** — 是 Pitfall 19（eval 审计漂移）的首块防御：每 phase 贡献 data-eval golden，Phase 19 收尾决策合并或保持独立
- **多周期 KPI 函数的 periods 数组参数** — 是 FEATURES Cap 3 的落地形态；一次调用 N 行 = 1 审计行 = 1 data-reference part 装载 N references，性能/可追性最佳
- **「同一 KPI 不同 Skill 必同结果」（D16-C4）的复现** — 单一 kpi-dashboard Skill 下此不变量自动满足；Phase 18 月报 Skill 复用 KPI 必同结果
- **data-reference part `references[]` 数组化是 wire 演进的最小路径** — 不扩 union 即可承载 sparkline + 多点引用；保留 Phase 4 wire-type 编译期报警护栏

## Specific test lock requirements

- **TestKPISkillStepsRoutingTable**：单测 Skill 加载时 steps.routes 路由表格式合法 + 触发短语非空 + KPI 名在 registry 内
- **TestDrillChipReinvokesMultiPeriodKPI**：点 chip → 新一轮 chat 消息预填「调 KPI_X periods=[最近 12 月]」+ LLM tool-call 实际触发 + 新 data-reference part 含 N references
- **TestSparklinePureSVG**：snapshot 测试 `<Sparkline points=...>` 输出 SVG 元素序列（polyline + N circles），零依赖
- **TestDataReferenceArrayPart**：data-reference part 含 references[] 数组时 SSE 序列化正确；wire union 类型不变（编译期验证）
- **TestPerPointHoverSameReference**：sparkline 点 hover 弹出 popup 与单数字 hover 同源（同一组件 + 同一字段集）
- **TestMultiKPISingleMessage**：Skill steps 调 3 个 KPI → 答案文本含 3 个数字 + 3 个独立 data-reference part inline；UI 同时渲染 3 个 hover
- **TestGEVL09GoldenSet50**：50 例 golden 全过 3 步判定（去格式化 + 容差 ±0.5% + allowed_expressions）
- **TestGEVL10ReferenceConsistency**：答案中数字 = KPI 快照 rows 数值（D2 三步判定）+ as_of 必出现 + 通道名 `data:aggregated_only` 必出现
- **TestReidFrozenSubset**：reid.jsonl 攻击 query 全过期望结果（拒答 / 全 suppressed / min suppression count ≥ k_min）
- **TestKPILockByteIdentity**：kpi.lock.json 5 字段 + KPI 注册表 sha256 与 TestShippedFrozenLockIsInSync 镜像模式
- **TestMultiPeriodKPIPeriodsArray**：periods 数组参数 1 调用返回 N 行 rows + 1 kpi_calls 审计行 + 1 data-reference part references[]=N
- **TestReadsChannelGuardXOR**（D16-C1 镜像）：kpi-dashboard Skill reads 含 `data:aggregated_only` → Tool 集 = {kpi_call}（无 retrieve）；通道守卫违规加载期拒绝
- **TestNoSQLGenerationSurface**（D16 镜像）：kpi-dashboard Skill 可达路径无 SQL 构造（WalkDir 结构性断言）
- **TestDrillDownNeverReachesRaw**：下钻 chip 重调 KPI 必走 kpi_call 工具，无 raw 数据访问路径

</specifics>

<deferred>
## Deferred Ideas

- **多轮趋势组合**（"趋势之上的趋势"——例如月度趋势对比季度趋势）— v1.1+（复杂度高，需要 multi-period recursive 调用规划）
- **真实企业脱敏数据驱动 eval** — v1.1+（需外部 IT 部门提供 + 运维纪律）
- **业务月报 Skill**（Phase 17 KPI 调用层之上的报告编排）— Phase 18
- **静态报告门户 + cron 调度** — Phase 19
- **KPI 差分攻击的运行时动态检测**（跨调用快照差集检测）— Phase 17 eval 验证枚举白名单是否足够后决策（D16-D2 拒绝项）
- **混合 Skill（tier 知识 + data 数据并读）** — v1 互斥（D16-C1）；业务明确后再开
- **admin KPI 只读视图 + 实时 KPI dashboard 页面** — Phase 20 ADM-03（kpi_calls 可 SQL 直查的面已够）
- **data.lock.json 跨 phase 合并决策** — Phase 19 收尾（DATA-18）
- **Skill steps 路由表 schema 标准化**（YAML vs Markdown 表）— Claude's discretion；planner 定
- **Skill steps 加 verifier 子步骤调取多个 KPI 对照** — A3 拒绝项；用户偏好 LLM 自决 + 标注不定可追问

None — discussion stayed within phase scope（无 scope creep 项）

</deferred>

---

*Phase: 17-p10-3*
*Context gathered: 2026-08-27 via /gsd-discuss-phase 17 (Default mode, 4 areas × 4 questions = 16 decisions)*