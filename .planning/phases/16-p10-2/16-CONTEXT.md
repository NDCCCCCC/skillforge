# Phase 16: KPI 聚合通道（P10.2）- Context

**Gathered:** 2026-08-24
**Status:** Ready for planning
**Source:** `/gsd-discuss-phase 16` — Default mode (4 questions per area, 4 areas, 16 locked decisions)
**Requirements:** DATA-03, DATA-07, DATA-08, DATA-09, DATA-12, KPI-01, KPI-02 (7 v1.0 requirements) — 5 Success Criteria (ROADMAP §Phase 16)

<domain>
## Phase Boundary

agent 与原始数据之间只存在一条受控通道——数据人员用受控文件定义 KPI（注册期预编译），运行时 agent 只能按名调用、引擎层无 SQL 生成面；每次调用的数据引用 `{func, args, as_of, result_snapshot}` 冻结进审计行。

**本 phase 交付：**
- KPI 注册表：YAML 受控文件定义（数据人员可写）+ 启动期 fail-closed 预编译成内存 registry（运行时与 agent 不可增改——无注册后门）
- KPI 查询表达：SQL 模板 + 命名参数占位，注册期静态校验（SELECT-only / 参数全覆盖 / 引用数据集必须已注册 / 禁跨数据集 join）
- `reads: ["data:aggregated_only"]` manifest 通道模式 + 通道守卫（与 tier reads 互斥；加载期交叉验证 + 运行时 Tool 拼装双层，镜像 D11-B1 `writes:` 执法）
- 单一 `kpi_call` 工具（sibling to `retrieve`）：按名调用 + 参数 schema 从定义文件编译；无匹配 → 工具层结构化拒答 `{reason_code: no_matching_kpi, available_kpis}`
- 数据引用：结构化快照 `{func, args, as_of, rows, k_min_applied}` + D16 data-decision 通道扩 data-reference part；as_of 系统自动生成（数据集 period 水位 + 调用时刻）
- `kpi_calls` 独立 INSERT-only 审计表（全量结果集冻结——数据清理后凭审计行可完整回放）
- k_min 查询侧防线：行级抑制 + suppressed 标记（Phase 15 配置面的执行侧落地）
- 差分成本门：参数枚举值域声明即白名单（细粒度参数组合在注册期被拒）

**不交付（deferred）：**
- KPI 仪表盘 Skill UI（sparkline / hover 溯源 / 下钻）— Phase 17
- 再识别 eval 断言（聚合 join + KPI 差分进冻结集）— Phase 17（本 phase 的 k_min 行为与枚举白名单是其断言对象）
- admin KPI 只读视图 — Phase 20 ADM-03（本 phase 语义上锁定「registry 只读 + kpi_calls 可 SQL 直查」即可）
- 混合 Skill（tier + data 并读）— v1 互斥（D16-C1），业务场景明确后再开
- 运行时差分计数防线（跨调用快照差集检测）— Phase 17 eval 验证枚举白名单是否足够后决策
- 声明式度量 DSL（metricflow 形态）/ per-Skill KPI 子集 / KPI 结果缓存 TTL — 拒绝或默认不做（见 decisions）

</domain>

<decisions>
## Implementation Decisions

### A. KPI 定义载体与 DSL（Area 1）

- **D16-A1: YAML 受控文件 = 唯一真源。** KPI 定义文件由数据人员编写（镜像 `datarules/loans.yaml` 先例：数据人员可写 + loader 加载期 fail-closed 校验），git 版本化 + diff 评审纪律；签名 = 文件内容哈希（与 D15-A4 rules_sha256 同构）。拒绝：Go 代码函数（每个 KPI 要开发者介入，与 KPI-01「数据人员定义」相悖）、sqlite kpi_function 表 + admin 管理（admin 可写 = 注册后门面，SC-1 明说「无注册后门」；真源进 DB 破坏「文件是唯一真源」纪律）。
- **D16-A2: SQL 模板 + 命名参数占位。** 定义文件直接写 SELECT + `:param` 占位；注册期静态校验：只允许单条 SELECT、参数全覆盖（SQL 内占位符与 params 声明一一对应）、引用数据集必须已在 datasets 注册表、禁止跨数据集 join（D14-D1 物理隔离的应用层呼应）。表达力最强（窗口函数/复杂聚合都能写）；SQL 生成面收敛在 loader 预编译层，agent 永远只看到 name + params。拒绝：声明式度量/维度（引擎拼 SQL 本身也是要审计的 SQL 生成面，v1 抽象成本高）、混合形态（两套校验 + 多兼容一种类型，D15-D4 同构理由）。
- **D16-A3: 参数类型 + 枚举值域声明。** `params:` 段声明类型与值域（如 `period: {type: string, pattern: <月份正则>}`；`status: {type: enum, values: [正常, 逾期]}`），loader 编译成 JSON schema 挂到 kpi_call 工具边界——参数校验 fail-closed，越域参数在进 SQL 绑定前被拒。枚举值域同时是差分成本门的声明面（D16-D2）。拒绝：仅类型不约束值域（枚举校验变运行时行为，与 DATA-08「加载期拒绝而非运行时行为」精神相悖）、period 一等参数特殊化（本质是 pattern 约束的 string，无需特殊分支）。
- **D16-A4: 内存 registry + 启动期编译。** 启动时 loader 校验通过后编译成内存 registry `{name, sql, param schema, 引用数据集, 聚合粒度元数据, 签名哈希}`；结构性测试锁「运行时无任何路径修改 registry」。admin 只读视图 = 文件加载结果 + kpi_calls 审计行组合（最后调用时间从审计行查），无需新持久表。拒绝：sqlite 快照表（文件改了表没同步 = 两真源漂移）、运行时按需读文件（每次调用多 IO 且不可变量难锁）。

### B. kpi_call 工具形态（Area 2）

- **D16-B1: 单一 kpi_call 工具。** `kpi_call(name, params)` 一个工具（ARCHITECTURE Q2 建议形态），KPI 数量增长不膨胀 Tool 列表；「知道有哪些 KPI」靠工具 description 内嵌 registry 目录（启动期生成 name/description/参数摘要）。与 `retrieve` 同构——一个工具闭包一个通道。拒绝：每 KPI 一个工具（20+ KPI 时 tool-calling 精度反降，Tool Registry 拼装复杂化）。
- **D16-B2: 无匹配 = 工具层结构化错误。** `kpi_call` 对未知 KPI 名返回 `{reason_code: no_matching_kpi, human_explanation, available_kpis: [...]}`（DATA-12 原文语义），agent 拿到结构化错误自然转述拒答（可用目录辅助用户重问）。无匹配是工具调用结果而非 query 分类问题——信息最准的层拒答。拒绝：短路 verdict 拒答管线（需 callback 层拦截工具结果，复杂度高且丢失可用目录信息）、query 前置分类（匹配判断与参数解析强耦合，前置层不准）。
- **D16-B3: 结构化快照 + data-reference part。** kpi_call 返回 `{func, args, as_of, rows, k_min_applied}`；D16 data-decision SSE 通道扩一个 data-reference part（**不扩 wire union 类型**，复用现有 `data-*` part 机制）——数字溯源链从工具层就结构化，Phase 17 GEVL-10（答案数字与快照一致）与 sparkline 以此为地基。拒绝：纯文本结果（快照只在审计行、SSE 流无结构化引用）、wire 层全留 Phase 17（数据引用 wire 契约无 hermetic 锁，Phase 17 改动面反大）。
- **D16-B4: as_of 系统自动生成。** as_of 是结果元数据不是查询参数：系统从 datasets 注册表读所查数据集最新 period 水位 + 调用时刻自动生成，agent 不可伪造（铁律⑦「永不声称拥有实时数据」的结构性落地）。历史时点回放走审计行快照（KPI-02），不来重新查询。拒绝：agent 传 as_of 参数（可伪造新鲜话术；且 supersede 后旧 period 已删查不到）。

### C. 通道守卫执法（Area 3）

- **D16-C1: reads 互斥（二选一）。** 含 `data:aggregated_only` 的 reads 不得同时声明 tier（反之亦然）——ARCHITECTURE Q2「at most ONE data:* pattern」延伸到 tier 维度。tier（Milvus 知识）与 data（DuckDB 数据）是两套存储/两套证据体系（citation 白名单 vs 数据引用快照），混读让 grounding/审计双轨复杂化；D14-D1 物理隔离哲学同构。混合 Skill（既查制度又查数）记 deferred，业务明确后再开。
- **D16-C2: 加载期交叉验证 + 拒绝。** Skill 加载时（loader/registry 层）交叉验证：reads 含 `data:aggregated_only` → 工具集必须 kpi_call（不含 retrieve）；reads 是 tier → 含 retrieve 不含 kpi_call。违规 = Skill 加载失败（启动期 + 热重载期都拒），错误信息点名冲突字段——DATA-08 原文「跨通道违规 = 加载期拒绝而非运行时行为」的直接落地，镜像 7-field validator 先例。拒绝：仅运行时静默过滤（违规 Skill 无声存在，配置错误不可见）。
- **D16-C3: 双层镜像 D11-B1。** 加载期 fail（D16-C2）+ 运行时 BuildToolsForSkill 按 reads 模式拼 Tool 集（越通道工具不出现 → LLM 看不到 = prompt-inject 也调不到）。加载期拒新违规，运行时兜底已激活 session 的存量会话与热重载窗口——与 `writes:` 执法（D11-B1 运行时拼 + D11-B3 编译期允许表）完全同构，先例代码直接镜像。
- **D16-C4: 平台级共享 KPI 目录。** 所有 `data:aggregated_only` Skill 看到同一 KPI 目录（singleton registry，manifest 不提供 per-Skill KPI 子集声明）。同 KPI 不同 Skill 必同结果——否则「这个 Skill 说 4.2%，那个说 4.5%」信任崩塌（FEATURES differentiator 原文）；定制发生在调用层（Skill steps 选哪些 KPI），不在定义层。拒绝：per-Skill kpi_allowlist 子集（定义层出现差异面，审计对账复杂化）。

### D. k_min 防线 + 差分门 + 审计（Area 4）

- **D16-D1: k_min 行级抑制 + 标记。** KPI 结果中聚合行数 < k_min（数据集级 override 或全局 `KMinDefault=5`）：该行值替换为 suppressed 标记，其余行正常返回；快照如实记录被抑制行与 `k_min_applied` 字段。业务可用性最好（大盘不因个别小桶全废），被抑制行仍可审计回放；Phase 17 再识别 eval 断言以此行为为准。拒绝：整体拒答（一个小桶拖死整个查询；agent 学会避开敏感参数反而丢失审计面）、动态桶合并（桶边界漂移与 D15-D1「跨期边界稳定」冲突，月环比失真）。
- **D16-D2: 枚举值域即差分白名单。** 定义文件 `params:` 的枚举值域声明（D16-A3）就是细粒度参数组合白名单——数据人员控制参数粒度（如 region 只开放省级枚举不开放城市），注册期就把细粒度组合拒之门外，运行时无需额外差分计数。改粒度 = 改文件 = 走 git 评审，与「无注册后门」同构。拒绝：+ 运行时差分计数（跨调用状态 + 检测算法 v1 复杂度高；agent 可换 session 绕过——枚举白名单是否足够由 Phase 17 再识别 eval 断言验证后再决策）。
- **D16-D3: result_snapshot 全量结果集冻结。** 审计行冻结 KPI 返回的全量结果集（rows + suppressed 标记 + as_of + k_min_applied）——「数据清理后凭审计行可回放当时答案」的最强解释。KPI 是聚合结果，行数 = 参数枚举组合数，天然有界体积可控，回放无信息损失。拒绝：top-N 截断（回放不完整，KPI-02 打折）、仅存查询引用（数据被 supersede/删除后无法重查，违背 KPI-02 原文）。
- **D16-D4: 独立 kpi_calls INSERT-only 表。** KPI 调用审计落 dedicated 表 `{func, args, as_of, result_snapshot, k_min_applied, ts, session/skill}`（镜像 D15-02-1 import_batches 先例）。ADM-03 KPI 视图（最后调用时间）可 SQL 直查无需 JOIN；大 JSON 快照不混 chat 审计主表；T-3-AUDIT INSERT-only 模式照复。拒绝：audit 主表新 action type + meta 兄弟表（跨表 JOIN 查视图；大快照与轻量 meta 混用）。

### Claude's Discretion

- **KPI 定义文件目录约定与 YAML schema 具体字段形态**（`kpi/` vs `datarules/` 内子目录；文件头 metadata：name/description/dataset/tier 声明语法）
- **SQL 模板静态校验的具体规则集**（SELECT-only 判定算法、占位符语法（`:name`）、DuckDB 方言校验深度、聚合函数要求（KPI 必须聚合非逐行））
- **kpi_call 工具 description 的目录生成口径**（KPI 多时的截断/排序、参数摘要格式）
- **data-reference part 的具体 wire payload 字段**（D16-B3 约束内：复用 data-decision 通道、不扩 union）
- **k_min 抑制的实现位置**（SQL 层 `HAVING count >= k_min` vs Go 层后过滤——行为语义不变即可）
- **kpi_calls 表具体 DDL 列集**（session/skill 关联列、索引；D16-D4 语义不变）
- **通道守卫违规报错文案**
- **registry 签名哈希口径**（单文件哈希 vs 目录聚合哈希）
- **KPI 定义文件热重载行为**（镜像 SYSK-04 Skill fsnotify 先例 vs 仅启动期加载——注意与 D16-A4「运行时不可变」的关系：热重载 = 新 registry 原子替换，不是原地增改）
- **admin 只读 API 端点是否本 phase 交付**（KPI-01 的 UI 归 Phase 20；只读查询面最小形态 planner 定）
- **period 数组参数（跨期比较一次调用）**（FEATURES Cap 3 提及 array-typed period；planner 按参数 schema 表达力决定，非本 phase 硬需求）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 研究文档（形态已研究到实现级）
- `.planning/research/FEATURES.md` Capability 2 — KPI 聚合接口 table stakes / differentiators / anti-features（singleton registry、快照冻结、no_matching_kpi、NL2SQL 与拖拽构建器为 anti-feature）
- `.planning/research/ARCHITECTURE.md` Q2 — kpi_call Tool archetype 定案（sibling to retrieve、platform-defined、`Call(ctx, name, paramsJSON)` 签名、BuildToolsForSkill seam 分支、manifest validReadPatterns + at most ONE data:* pattern）
- `.planning/research/PITFALLS.md` Pitfall 14 — KPI 注册后门（agent 运行时注册 KPI 绕过审批；注册表文件级运行时不可变）
- `.planning/research/PITFALLS.md` Pitfall 12 — 跨 skill 通道违规（`data:` 通道加载期守卫，镜像 writes: 执法）
- `.planning/research/PITFALLS.md` Pitfall 22 — KPI 差分再识别（两快照差分还原小群体；参数白名单 + 差分成本门）
- `.planning/research/PITFALLS.md` Pitfall 11 — 聚合 join 再识别（k_min=5 推荐值；Phase 15 交付配置面，本 phase 交付查询侧执行）

### 需求 + 路线图锚
- `.planning/REQUIREMENTS.md` §Financial Data Layer — DATA-03/07/08/09/12 原文（DATA-07 v1.0 仅 `data:aggregated_only`；DATA-12 reason code 具体化为 `no_matching_kpi`）
- `.planning/REQUIREMENTS.md` §KPI — KPI-01（受控文件定义/运行时不可增改/admin 只读）/ KPI-02（{func, args, as_of, result_snapshot} 冻结进审计行）原文
- `.planning/ROADMAP.md` §Phase 16 — Goal / 5 SC / Pitfalls addressed / Research flags（Q2 KPI 工具接口形态 + FEATURES Cap 2 DSL 形态 plan 期定稿——本 CONTEXT 已定稿）

### 前置 phase 决策（直接继承）
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-CONTEXT.md` — D11-B1 运行时动态拼 Tool whitelist / D11-B3 单文件 registry source of truth（D16-C3 镜像对象）
- `.planning/phases/15-p10-1/15-CONTEXT.md` — D15-A1 受控文件哲学（datarules 先例）/ D15-A4 规则哈希进审计（签名同构）/ D15-D1 定宽桶跨期稳定（k_min 行为不得破坏）/ D15-D3 k_min 全局 5 + 数据集 override（本 phase 执行侧消费）
- `.planning/phases/14-p10-0-cgo-trust-floor/14-CONTEXT.md` — D14-D1 每数据集一 .duckdb 物理隔离（跨数据集 join 禁令的结构性背景）

### 已交付核心资产（直接扩展）
- `internal/agent/hitl/registry.go` — AllowedWriteTargets + handlers 单文件 source of truth（kpi_call 注册面镜像模式）
- `internal/agent/tools/retrieve.go` — 现有唯一读工具（kpi_call 的 sibling 模板：闭包一个通道）
- `internal/skill/manifest.go` + `internal/skill/loader.go` — reads 值域扩展点（validReadPatterns + 通道互斥校验落这里）
- `internal/duckdb/attach_lru.go` + `internal/duckdb/connector.go` — 懒 ATTACH READ_ONLY + LRU（KPI 查询执行面现成）
- `internal/data/desensitize.go` `KMinDefault` + datasets 注册表 `k_min_override` — k_min 读取面（Phase 15 交付）
- `datarules/loans.yaml` + `internal/data/rules.go` — 受控 YAML + fail-closed loader 先例（KPI 定义文件直接镜像）
- `internal/store/sqlite/import_batches.go` — dedicated INSERT-only 表先例（kpi_calls 镜像）
- `internal/agent/runner_sse.go`（D16 data-decision chunk 通道）— data-reference part 扩展点
- `internal/platformfloor/` — 平台铁律①（只按名调 KPI）③（结论带 as_of + 数据引用）⑦（永不声称实时）——kpi_call 行为与地板文案必须一致

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/duckdb/` attach_lru + connector** — Phase 14/15 交付的懒 ATTACH READ_ONLY 连接模式：KPI 查询按定义文件引用的数据集 ATTACH 后执行预编译 SQL，无需新连接基建
- **`internal/agent/tools/retrieve.go` 工厂模式** — kpi_call 复刻同形态：闭包捕获 registry（对应 retrieve 闭包捕获 ResolvedTiers）
- **`internal/skill/` fsnotify Registry + 7-field validator** — KPI 定义文件加载/校验/热重载的镜像对象
- **`internal/store/sqlite/import_batches.go`** — INSERT-only 专用表 + TestBatchAuditRowShape 测试锁模式，kpi_calls 照抄

### Established Patterns
- **D11-B1/B3 双层执法** — 加载期允许表校验 + 运行时动态拼 Tool；D16-C2/C3 直接镜像（reads 维度换 writes 维度）
- **fail-closed 加载期校验** — Skill validator / KNOW-02 / datarules loader 三先例；KPI loader 第四次复刻
- **T-3-AUDIT INSERT-only + parameterized** — kpi_calls 表照复；快照 JSON blob 参数化写入
- **D8-04 WalkDir 禁令门** — 可复刻为「agent 路径无 SQL 生成面」结构性断言（SQL 字符串只允许出现在 kpi/ 定义文件与 loader 校验代码中）

### Integration Points
- **`internal/skill/manifest.go`** — reads 值域扩 `data:aggregated_only` 模式 + 互斥校验
- **BuildToolsForSkill seam**（ARCHITECTURE Q2 点名）— `reads` 含 data 通道时 append kpi_call（whitelist.go 形态）
- **kpi_calls 表 DDL** — sqlite additive 迁移（D15-02 模式）
- **SSE data-reference part** — D16 data-decision 通道扩展，不扩 wire union（D15 wire-type compile alarm 不破）
- **cmd/server/main.go** — KPI loader 启动装配（镜像 datarules/registry 装配点）

</code_context>

<specifics>
## Specific Ideas

- **「agent 永不写 SQL 为结构性保证，非 prompt 约束」**（SC-3 原文）——落地形态：kpi_call 闭包外 agent 可达代码无 SQL 构造面；SQL 只存在于定义文件模板（注册期校验后常量）与 loader；用 WalkDir 类结构性测试锁死
- **铁律①③⑦ 与 kpi_call 行为必须自洽** — 地板说「只按名调 KPI」而通道守卫保证 LLM 物理上只有 kpi_call 可调；地板说「结论带 as_of」而 as_of 由系统生成 agent 不可伪造——提示词承诺与结构性 enforcement 互为印证，这是平台叙事的核心卖点
- **「同 KPI 不同 Skill 必同结果」是 singleton registry 的验收语义** — 测试锁：同 KPI 同参数跨 Skill 调用结果（含 as_of 同期）逐字节一致
- **k_min 行级抑制与 D15-D1 定宽桶的配合** — 桶化列的稀疏桶（小桶）由 k_min 抑制兜底，桶边界本身跨期稳定不因 k_min 动态合并——两条防线各司其职
- **无匹配拒答带 available_kpis 目录** — 拒答不只有 reason code 还有建设性信息（DATA-12 的 human_explanation 精神）；agent 转述时用户能立刻改问

## Specific test lock requirements

- **TestKPIRegistryImmutableAtRuntime**：运行时无任何代码路径可增改 registry（接口面只读 + 结构性断言）
- **TestNoSQLGenerationSurface**：agent 可达路径无 SQL 字符串构造（WalkDir/接口断言——SQL 只在 kpi/ 定义文件与 loader）
- **TestKPILoadFailClosed**：语法错 / 非 SELECT / 引用未注册数据集 / 跨数据集 join / 参数未覆盖 → 启动期加载失败（fail-closed 各分支）
- **TestChannelGuardXOR**：reads 混 tier + data:aggregated_only → Skill 加载失败，错误点名冲突字段（D16-C1/C2）
- **TestToolsForChannel**：data 通道 Skill 的 Tool 集 = {kpi_call}（无 retrieve）；tier 通道 = {retrieve}（无 kpi_call）（D16-C3 运行时层）
- **TestNoMatchingKPIRefusal**：未知 KPI 名 → `{reason_code: no_matching_kpi, human_explanation, available_kpis}`（D16-B2）
- **TestEnumParamFailClosed**：枚举外参数值在工具边界被拒，不进 SQL 绑定（D16-A3/D2）
- **TestKMinRowSuppression**：小桶行值替换为 suppressed 标记 + k_min_applied 记录 + 其余行正常返回（D16-D1）
- **TestKPISnapshotFrozen**：result_snapshot 全量冻结；数据集删除/supersede 后凭 kpi_calls 行完整回放当时结果（D16-D3 / KPI-02）
- **TestASofAutoGenerated**：as_of 来自 datasets 注册表最新 period 水位 + 调用时刻；参数 schema 中无 as_of 字段（D16-B4）
- **TestSameKPIAcrossSkills**：同 KPI 同参数两个 Skill 调用，结果（同期 as_of）逐字节一致（D16-C4）

</specifics>

<deferred>
## Deferred Ideas

- **混合 Skill（tier 知识 + data 数据并读）** — v1 互斥（D16-C1）；既查制度又查数的业务场景明确后再开（两套引用体系混排需先解决 grounding/审计双轨）
- **运行时差分计数防线**（跨调用快照差集检测）— Phase 17 再识别 eval 验证枚举白名单是否足够后决策（D16-D2 拒绝项）
- **KPI 仪表盘 Skill**（聊天数字问答 / sparkline / hover 溯源 / 下钻 / GEVL-09/10）— Phase 17
- **再识别 eval 断言进冻结集**（聚合 join + KPI 差分场景）— Phase 17
- **admin KPI 只读视图 + 定时任务视图** — Phase 20 ADM-03（本 phase 交付 kpi_calls 可 SQL 直查的面）
- **声明式度量 DSL（metricflow 形态）** — v1.1+（D16-A2 拒绝项；若 KPI 数量爆炸再评估）
- **per-Skill KPI 子集声明** — 拒绝（D16-C4；定制在调用层不在定义层）
- **KPI 结果缓存 TTL**（FEATURES optional，default OFF）— 不做（金融新鲜性默认 always live；需要时 per-KPI 加）
- **data.lock.json 冻结锁收录范围** — Phase 20 收尾决策

None — discussion stayed within phase scope（无 scope creep 项）

</deferred>

---

*Phase: 16-p10-2*
*Context gathered: 2026-08-24 via /gsd-discuss-phase 16 (Default mode, 4 areas × 4 questions = 16 decisions)*
