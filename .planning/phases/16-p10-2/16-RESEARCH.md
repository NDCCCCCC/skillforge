# Phase 16: KPI 聚合通道（P10.2）- Research

**Researched:** 2026-08-24
**Domain:** Go 后端 — KPI 受控定义/预编译注册表/agent 单工具通道/审计快照（Eino tool 边界 + DuckDB 执行 + sqlite 审计）
**Confidence:** HIGH（领域全部为仓库内已验证先例的直接镜像；唯一外部行为 `json_serialize_sql` 已对照官方文档核实，残留不确定点列入 Wave 0 探针）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**A. KPI 定义载体与 DSL（Area 1）**

- **D16-A1: YAML 受控文件 = 唯一真源。** KPI 定义文件由数据人员编写（镜像 `datarules/loans.yaml` 先例：数据人员可写 + loader 加载期 fail-closed 校验），git 版本化 + diff 评审纪律；签名 = 文件内容哈希（与 D15-A4 rules_sha256 同构）。拒绝：Go 代码函数（每个 KPI 要开发者介入，与 KPI-01「数据人员定义」相悖）、sqlite kpi_function 表 + admin 管理（admin 可写 = 注册后门面，SC-1 明说「无注册后门」；真源进 DB 破坏「文件是唯一真源」纪律）。
- **D16-A2: SQL 模板 + 命名参数占位。** 定义文件直接写 SELECT + `:param` 占位；注册期静态校验：只允许单条 SELECT、参数全覆盖（SQL 内占位符与 params 声明一一对应）、引用数据集必须已在 datasets 注册表、禁止跨数据集 join（D14-D1 物理隔离的应用层呼应）。表达力最强（窗口函数/复杂聚合都能写）；SQL 生成面收敛在 loader 预编译层，agent 永远只看到 name + params。拒绝：声明式度量/维度（引擎拼 SQL 本身也是要审计的 SQL 生成面，v1 抽象成本高）、混合形态（两套校验 + 多兼容一种类型，D15-D4 同构理由）。
- **D16-A3: 参数类型 + 枚举值域声明。** `params:` 段声明类型与值域（如 `period: {type: string, pattern: <月份正则>}`；`status: {type: enum, values: [正常, 逾期]}`），loader 编译成 JSON schema 挂到 kpi_call 工具边界——参数校验 fail-closed，越域参数在进 SQL 绑定前被拒。枚举值域同时是差分成本门的声明面（D16-D2）。拒绝：仅类型不约束值域（枚举校验变运行时行为，与 DATA-08「加载期拒绝而非运行时行为」精神相悖）、period 一等参数特殊化（本质是 pattern 约束的 string，无需特殊分支）。
- **D16-A4: 内存 registry + 启动期编译。** 启动时 loader 校验通过后编译成内存 registry `{name, sql, param schema, 引用数据集, 聚合粒度元数据, 签名哈希}`；结构性测试锁「运行时无任何路径修改 registry」。admin 只读视图 = 文件加载结果 + kpi_calls 审计行组合（最后调用时间从审计行查），无需新持久表。拒绝：sqlite 快照表（文件改了表没同步 = 两真源漂移）、运行时按需读文件（每次调用多 IO 且不可变量难锁）。

**B. kpi_call 工具形态（Area 2）**

- **D16-B1: 单一 kpi_call 工具。** `kpi_call(name, params)` 一个工具（ARCHITECTURE Q2 建议形态），KPI 数量增长不膨胀 Tool 列表；「知道有哪些 KPI」靠工具 description 内嵌 registry 目录（启动期生成 name/description/参数摘要）。与 `retrieve` 同构——一个工具闭包一个通道。拒绝：每 KPI 一个工具（20+ KPI 时 tool-calling 精度反降，Tool Registry 拼装复杂化）。
- **D16-B2: 无匹配 = 工具层结构化错误。** `kpi_call` 对未知 KPI 名返回 `{reason_code: no_matching_kpi, human_explanation, available_kpis: [...]}`（DATA-12 原文语义），agent 拿到结构化错误自然转述拒答（可用目录辅助用户重问）。无匹配是工具调用结果而非 query 分类问题——信息最准的层拒答。拒绝：短路 verdict 拒答管线（需 callback 层拦截工具结果，复杂度高且丢失可用目录信息）、query 前置分类（匹配判断与参数解析强耦合，前置层不准）。
- **D16-B3: 结构化快照 + data-reference part。** kpi_call 返回 `{func, args, as_of, rows, k_min_applied}`；D16 data-decision SSE 通道扩一个 data-reference part（**不扩 wire union 类型**，复用现有 `data-*` part 机制）——数字溯源链从工具层就结构化，Phase 17 GEVL-10（答案数字与快照一致）与 sparkline 以此为地基。拒绝：纯文本结果（快照只在审计行、SSE 流无结构化引用）、wire 层全留 Phase 17（数据引用 wire 契约无 hermetic 锁，Phase 17 改动面反大）。
- **D16-B4: as_of 系统自动生成。** as_of 是结果元数据不是查询参数：系统从 datasets 注册表读所查数据集最新 period 水位 + 调用时刻自动生成，agent 不可伪造（铁律⑦「永不声称拥有实时数据」的结构性落地）。历史时点回放走审计行快照（KPI-02），不来重新查询。拒绝：agent 传 as_of 参数（可伪造新鲜话术；且 supersede 后旧 period 已删查不到）。

**C. 通道守卫执法（Area 3）**

- **D16-C1: reads 互斥（二选一）。** 含 `data:aggregated_only` 的 reads 不得同时声明 tier（反之亦然）——ARCHITECTURE Q2「at most ONE data:* pattern」延伸到 tier 维度。tier（Milvus 知识）与 data（DuckDB 数据）是两套存储/两套证据体系（citation 白名单 vs 数据引用快照），混读让 grounding/审计双轨复杂化；D14-D1 物理隔离哲学同构。混合 Skill（既查制度又查数）记 deferred，业务明确后再开。
- **D16-C2: 加载期交叉验证 + 拒绝。** Skill 加载时（loader/registry 层）交叉验证：reads 含 `data:aggregated_only` → 工具集必须 kpi_call（不含 retrieve）；reads 是 tier → 含 retrieve 不含 kpi_call。违规 = Skill 加载失败（启动期 + 热重载期都拒），错误信息点名冲突字段——DATA-08 原文「跨通道违规 = 加载期拒绝而非运行时行为」的直接落地，镜像 7-field validator 先例。拒绝：仅运行时静默过滤（违规 Skill 无声存在，配置错误不可见）。
- **D16-C3: 双层镜像 D11-B1。** 加载期 fail（D16-C2）+ 运行时 BuildToolsForSkill 按 reads 模式拼 Tool 集（越通道工具不出现 → LLM 看不到 = prompt-inject 也调不到）。加载期拒新违规，运行时兜底已激活 session 的存量会话与热重载窗口——与 `writes:` 执法（D11-B1 运行时拼 + D11-B3 编译期允许表）完全同构，先例代码直接镜像。
- **D16-C4: 平台级共享 KPI 目录。** 所有 `data:aggregated_only` Skill 看到同一 KPI 目录（singleton registry，manifest 不提供 per-Skill KPI 子集声明）。同 KPI 不同 Skill 必同结果——否则「这个 Skill 说 4.2%，那个说 4.5%」信任崩塌（FEATURES differentiator 原文）；定制发生在调用层（Skill steps 选哪些 KPI），不在定义层。拒绝：per-Skill kpi_allowlist 子集（定义层出现差异面，审计对账复杂化）。

**D. k_min 防线 + 差分门 + 审计（Area 4）**

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

### Deferred Ideas (OUT OF SCOPE)

- **混合 Skill（tier 知识 + data 数据并读）** — v1 互斥（D16-C1）；既查制度又查数的业务场景明确后再开（两套引用体系混排需先解决 grounding/审计双轨）
- **运行时差分计数防线**（跨调用快照差集检测）— Phase 17 再识别 eval 验证枚举白名单是否足够后决策（D16-D2 拒绝项）
- **KPI 仪表盘 Skill**（聊天数字问答 / sparkline / hover 溯源 / 下钻 / GEVL-09/10）— Phase 17
- **再识别 eval 断言进冻结集**（聚合 join + KPI 差分场景）— Phase 17
- **admin KPI 只读视图 + 定时任务视图** — Phase 20 ADM-03（本 phase 交付 kpi_calls 可 SQL 直查的面）
- **声明式度量 DSL（metricflow 形态）** — v1.1+（D16-A2 拒绝项；若 KPI 数量爆炸再评估）
- **per-Skill KPI 子集声明** — 拒绝（D16-C4；定制在调用层不在定义层）
- **KPI 结果缓存 TTL**（FEATURES optional，default OFF）— 不做（金融新鲜性默认 always live；需要时 per-KPI 加）
- **data.lock.json 冻结锁收录范围** — Phase 20 收尾决策
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DATA-03 | 聚合查询 API：agent 永不对原始行 SELECT，只调用预定义 KPI 函数（名+参数） | §Architecture Patterns「Pattern 1/2」：kpi_call 工具闭包（镜像 retrieve.go）+ executor 预编译 SQL 执行；§Code Examples named-param 绑定 |
| DATA-07 | reads 值域扩展 `data:aggregated_only`（v1.0 仅此一个 data 模式），与 tier reads 互斥 | §Architecture Patterns「Pattern 4」：manifest Validate() 分支 + ResolveTiers 前置识别 + XOR 校验（扩展点已在 internal/skill/validate.go / tier.go 确认） |
| DATA-08 | 跨通道违规 = platform 层 guardrail，加载期拒绝而非运行时行为 | §Pattern 4 双层执法（加载期 D16-C2 + 运行时 BuildToolsForSkill D16-C3），镜像 D11-B1/B3 先例（internal/agent/hitl_tools.go 已确认） |
| DATA-09 | agent 只能调用预定义 KPI；无 SQL 生成、无 KPI 定义面 | §Pattern 3「无 SQL 生成面」+ TestNoSQLGenerationSurface 结构性测试形态（D8-04 WalkDir 禁令门复刻） |
| DATA-12 | 拒答带结构化 reason `{reason_code, human_explanation}` | §Pattern 2 no_matching_kpi 工具层结构化错误（D16-B2；retrieveArgs/Response JSON 形态先例已确认） |
| KPI-01 | KPI 由受控文件定义、运行时不可增改、admin 只读 | §Pattern 1 受控 YAML + 启动期编译 + 只读接口面（TestKPIRegistryImmutableAtRuntime 锁法） |
| KPI-02 | 每次调用冻结 `{func, args, as_of, result_snapshot}` 进审计行，可回放 | §Pattern 6 kpi_calls INSERT-only DDL（镜像 import_batches 先例）+ as_of 自动生成（datasets 注册表 period 水位） |
</phase_requirements>

## Summary

Phase 16 的本质是**四次复刻仓库内已验证的先例，再叠加一个外部能力点**。四次复刻：(1) KPI 定义文件 = `datarules/loans.yaml` + `internal/data/rules.go` LoadRules 的直接镜像（受控 YAML + sha256 + fail-closed 校验 + symlink guard）；(2) 通道守卫 = `writes:` 执法（D11-B1 运行时拼 Tool + D11-B3 加载期允许表）在 reads 维度的镜像；(3) kpi_calls 表 = `import_batches` dedicated INSERT-only 表先例的照抄（additive DDL，新表无需 PRAGMA 门）；(4) kpi_call 工具 = `internal/agent/tools/retrieve.go` 的 sibling（一个闭包一个通道，闭包捕获 registry 快照而非 ResolvedTiers）。这些先例的代码形态、测试锁形态、装配点全部在本仓库中现场确认过，planner 可以按「找到镜像对象 → 换语义」直接派任务。

外部能力点是 **SQL 注册期静态校验**。本研究确认了 DuckDB 的 `json_serialize_sql()` 内置函数可作为加载期 SQL 解析器：它把 SQL 文本解析成 JSON AST（`{"error":false,"statements":[{"node":{"type":"SELECT_NODE","from_table":{"type":"BASE_TABLE","table_name":...},...}}]}`），语法错误时返回 `{"error":true,"error_type":"parser",...}` 而非抛异常——loader 必须显式检查 `error:true` 才能 fail-closed，这是一个容易踩的反直觉点。AST 中的 `from_table.type=="BASE_TABLE"` + `table_name` 字段是「引用数据集必须已注册 + 禁跨数据集 join」两条规则的执行依据（递归走 AST 收集全部表引用，全部必须落在 `ds_{声明的dataset}` 单一别名下）。该函数属于 JSON 模块，而 go-duckdb v2.4.3 静态链接了 JSON 扩展，hermetic boot（autoinstall/autoload=false）下依然可用。

**核心工程约束是 hermetic/WSL 双轨测试分类。** 纯 Go 层（YAML schema 校验、`:param` 占位符文本覆盖检查、枚举/pattern 编译、通道 XOR、wire payload、kpi_calls DDL）在 Windows 直接跑；凡触碰 `internal/duckdb`（CGO）的层（json_serialize_sql 解析校验、预编译执行、k_min 抑制、快照回放）必须在 WSL Ubuntu-22.04 跑——与 D15-01-1 对 internal/data 的分类完全一致。设计推论：KPI loader 的校验逻辑应拆成「纯函数校验核心（hermetic）+ 引擎探针（WSL）」两层，让 80% 的 fail-closed 分支在 Windows 快速锁死。

**Primary recommendation:** 新建 `internal/kpi` 包（loader + registry + executor），SQL 静态校验用双层拆分（纯 Go 文本检查 hermetic + `json_serialize_sql` AST 检查 WSL-gated），kpi_call 以 retrieve.go 为模板闭包拼装进 BuildToolsForSkill 通道分支，kpi_calls 表照 import_batches 抄 additive DDL——零新依赖，全部复用 go.mod 已锁版本。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| KPI 定义解析/静态校验/编译 | Backend loader（新 `internal/kpi`） | — | 受控文件纪律：加载期 fail-closed 是唯一校验窗口（D16-A1/A2） |
| SQL 执行（预编译查询） | Backend DuckDB engine（`internal/kpi` executor over `internal/duckdb` AttachLRU） | — | 数据集已懒 ATTACH READ_ONLY，KPI 查询零新连接基建 |
| kpi_call 工具暴露/通道守卫 | Agent runtime（`internal/agent` BuildToolsForSkill 通道分支） | Skill loader（加载期 XOR 拒绝，D16-C2） | LLM 看不到的工具调不到——运行时拼装是物理防线 |
| reads 值域/互斥校验 | Skill loader（`internal/skill/validate.go`） | — | 7-field validator 是 reads 语义的唯一所有者（manifest.go 确认） |
| as_of 生成 | Backend（datasets 注册表读面 `store.ListDatasets`） | — | period 水位是运行时状态，必须从 sqlite registry 读（D16-B4） |
| k_min 行级抑制 | Backend executor（Go 层后过滤，本研究推荐） | — | 行替换语义（D16-D1：行保留、值替换）与 SQL `HAVING`（行消失）不同构，见 Pitfall 4 |
| 审计快照冻结 | Storage（`internal/store/sqlite` kpi_calls） | — | dedicated INSERT-only 表；大 JSON 快照不混 chat 审计主表（D16-D4） |
| SSE 数据引用溯源 | Wire（`internal/wire` DecisionPayload additively）+ server SSE emitter | — | 复用 data-decision 通道，不扩 chunk union（D15 wire-type 编译告警不破） |
| 差分白名单 | 定义文件 `params:` 枚举声明（数据人员） | loader 编译进 JSON schema | 注册期执法，运行时零额外计数（D16-D2） |

## Standard Stack

### Core

全部为 go.mod 已锁依赖——**本 phase 零新包**。

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/marcboeker/go-duckdb/v2` | v2.4.3 | KPI 预编译 SQL 执行 + `json_serialize_sql` 加载期解析 | 已锁（go.mod 确认）；named params `$foo` + `sql.Named` 已在 module cache statement_test.go:207-218 验证；JSON 扩展静态链接（README:371） |
| `github.com/cloudwego/eino` | v0.9.14 | kpi_call 工具 schema（`schema.ParameterInfo` 含 `Enum []string`） | 已锁；tool.go:254-263 确认 Enum + SubParams 字段，KPI 名枚举可表达 |
| `gopkg.in/yaml.v3` | v3.0.1 | KPI 定义文件解析 | datarules loader 同款（rules.go 先例） |
| `github.com/mattn/go-sqlite3` | v1.14.50 | kpi_calls 表 | 已锁；Windows MSYS2 gcc 可链接（仅 go-duckdb 不能） |
| `github.com/stretchr/testify` | v1.11.1 | 测试断言 | 全仓测试标准 |
| `github.com/fsnotify/fsnotify` | （已锁） | 仅当选择 KPI 热重载时使用 | skill Registry 先例（registry.go:37-162 debounce 1s 模式）；v1 建议不用（见 Open Questions Q1） |

### Supporting

无。DuckDB CLI / Python 侧车等均不需要——SQL 校验用 go-duckdb 进程内完成。

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `json_serialize_sql` AST 校验 | 纯 Go 正则/字符串检查 | 正则无法可靠识别嵌套子查询里的表引用与 join——「禁跨数据集 join」必须有真解析器；纯 Go 检查保留为第一层快速失败（hermetic） |
| `json_serialize_sql` AST 校验 | 第三方 Go SQL parser（如 znacho/sqlparser） | 新依赖 + DuckDB 方言覆盖不全（窗口函数、QUALIFY 等）；内置函数零依赖且方言绝对一致 |
| `sql.Named` 绑定 | 手工字符串替换参数 | 绝对禁止——拼接即 SQL 注入面（T-3-SQLI）；见 Don't Hand-Roll |

**Installation:** 无（`go mod tidy` 不新增任何条目）。

**Version verification:** 2026-08-24 已在仓库 go.mod 现场确认：eino v0.9.14 / go-duckdb v2.4.3 / mattn v1.14.50 / testify v1.11.1 / yaml.v3 v3.0.1。

## Package Legitimacy Audit

**本 phase 不安装任何外部包。** 所有依赖为 Phase 3-15 已锁定并经过 slopcheck 时代审计的 go.mod 存量（上表逐条现场核实）。无新增 npm/pip/cargo 条目，无需运行 slopcheck gate。

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
数据人员（git 评审纪律）
   │  编写/更新
   ▼
kpis/*.yaml（受控文件：dataset + kpis[] + params 枚举/pattern）      datarules/*.yaml（Phase 15 先例）
   │
   │ 启动期（cmd/server/main.go 装配点，镜像 datarules 装配）
   ▼
┌─────────────────────────── internal/kpi loader ───────────────────────────┐
│ Layer 1 纯 Go 校验（hermetic）：YAML schema / :param↔params 覆盖 /          │
│   枚举编译 / 名称唯一 / dataset 声明存在                                   │
│ Layer 2 引擎校验（WSL）：json_serialize_sql → AST 走查                      │
│   ├─ error:true → 拒（语法错）                                             │
│   ├─ len(statements)≠1 或 node.type≠SELECT_NODE → 拒（非单条 SELECT）      │
│   └─ BASE_TABLE 收集 → 表引用 ⊄ ds_{声明dataset}.* → 拒（未注册/跨集 join） │
│ 通过 → `:param` 重写为 `$param` → 编译进内存 Registry（不可变，含签名哈希） │
└───────────────────────────────────────────────────────────────────────────┘
   │                                                     ▲
   │ Skill 加载（internal/skill/validate.go）             │ datasets 注册表（sqlite）
   ▼                                                     │ （period 水位 / k_min_override /
reads: ["data:aggregated_only"] ──XOR── reads: ["权威",...]│  存在性，as_of 生成读这里）
   │ 混合 → Skill 加载失败（点名冲突字段）                  │
   ▼                                                     │
┌───────────── 运行时（每次会话，BuildToolsForSkill 通道分支）──────────────┴──┐
│ data 通道 Skill → Tools = {kpi_call}（无 retrieve）                          │
│ tier 通道 Skill → Tools = {retrieve} + writes 允许的 HITL 工具（无 kpi_call）│
└──────────────────────────────────────────────────────────────────────────────┘
   │ agent 调用 kpi_call(name, params)
   ▼
┌──────────────────── internal/kpi executor（kpi_call 闭包内）────────────────┐
│ ① name 不在 registry → 结构化错误 {reason_code: no_matching_kpi,            │
│    human_explanation, available_kpis}（D16-B2，agent 转述拒答）              │
│ ② params 对编译好的 param schema 校验 → 越域/缺参 → 结构化错误（fail-closed）│
│ ③ as_of = {period: 数据集最新 period 水位, ts: 调用时刻}（系统生成，D16-B4） │
│ ④ AttachLRU.Get(dataset) → 预编译 SQL + sql.Named 绑定 → 执行                │
│ ⑤ Go 层 k_min 后过滤：count < k_min 的行 → 值替换为 suppressed 标记（D16-D1）│
│ ⑥ 快照 {func, args, as_of, rows, k_min_applied} 三路输出 ↓                   │
└──────────────────────────────────────────────────────────────────────────────┘
   │                    │                          │
   │ 工具结果返回 agent  │ SSE data-decision        │ INSERT-only
   ▼                    ▼ （data-reference part，  ▼
agent 综合作答          │  DecisionPayload 扩字段） kpi_calls 表
（结论必带 as_of 引用，  │                          （数据清理后凭行回放，
 铁律③⑦）               ▼                           KPI-02）
                        FE（Phase 17 溯源 UI 地基）
```

### Recommended Project Structure

```
internal/
├── kpi/                      # 新包（Phase 16 核心）
│   ├── define.go             # KPIFile / KPIDef / ParamSpec YAML 结构体
│   ├── loader.go             # LoadKPIs(dir) — 双层校验 → Registry（镜像 rules.go）
│   ├── registry.go           # 不可变 Registry + 只读接口（Lookup/Catalog/Signature）
│   ├── executor.go           # Call(ctx, name, params) — 校验→绑定→执行→k_min→快照
│   ├── validate_text.go      # Layer 1 纯 Go 校验（hermetic 可测）
│   ├── validate_sql.go       # Layer 2 json_serialize_sql AST 走查（WSL-gated 测）
│   ├── kmin.go               # 行级抑制后过滤（纯函数，hermetic 可测）
│   └── *_test.go             # 11 个 test lock（hermetic/WSL 分类见 Validation 节）
├── skill/
│   ├── manifest.go           # 扩展：reads 的 data: 模式识别（ChannelKind）
│   ├── validate.go           # 扩展：XOR 互斥校验（D16-C1/C2）
│   └── tier.go               # ResolveTiers 前置分支（data: 模式不走 tier 解析）
├── agent/
│   ├── hitl_tools.go         # BuildToolsForSkill 扩通道分支（D16-C3 挂点）
│   ├── agent.go              # NewAgent 按 reads 模式拼 retrieve vs kpi_call
│   └── tools/kpi_tool.go     # BuildKPICallTool（镜像 retrieve.go 形态）
├── store/
│   ├── store.go              # Store 接口 + KPICallRow（镜像 DatasetRow）
│   └── sqlite/
│       ├── schema.go         # kpi_calls DDL（additive，进 schemaSQL 即可）
│       └── kpi_calls.go      # PutKPICall INSERT-only（镜像 import_batches.go）
├── wire/
│   ├── types.go              # DecisionKPIReference 新 Decision 值（可选）
│   └── chunk.go              # DecisionPayload 扩 DataRef 字段（additive）
└── data/                     # 不动（k_min 读取面 KMinDefault/datasets.k_min_override 现成）

kpis/                         # 新受控文件目录（仓库根，与 datarules/ 平级）
└── loans.yaml                # 示例：loan_overdue_rate 等 2-3 个 KPI
```

### Pattern 1: 受控 YAML 文件 + fail-closed loader（第 4 次复刻）

**What:** KPI 定义 = git 内 YAML 文件；loader 启动期全量校验，任何一条不过 = 整体加载失败（服务拒绝启动），绝不部分加载。
**When to use:** `internal/kpi/loader.go` 直接镜像 `internal/data/rules.go` 的 LoadRules（Lstat symlink guard → ReadFile → sha256 → yaml.Unmarshal → Validate fail-closed）。

**YAML schema 提案（Discretion 1 的建议答案）：**

```yaml
# kpis/loans.yaml — 与 datarules/loans.yaml 平级的受控文件
dataset: loans            # 必须匹配 datasets 注册表中的 dataset 字段（D15-C2 period 载体）
description: 贷款业务核心指标（数据组维护）
k_min: 0                  # 0 = 用数据集 k_min_override，再 0 用全局 KMinDefault=5（与 datarules 同语义）
kpis:
  - name: loan_overdue_rate_by_region
    description: 各省份贷款逾期率（百分比，分母为贷款笔数）
    sql: |
      SELECT region AS region,
             count(*) AS row_count,
             round(100.0 * sum(CASE WHEN status = '逾期' THEN 1 ELSE 0 END) / count(*), 2) AS overdue_rate
      FROM ds_loans.loans
      WHERE period = :period
      GROUP BY region
    count_column: row_count          # k_min 阈值判定的计数控（kmin.go 消费）
    params:
      period:
        type: string
        pattern: '^2[0-9]{3}-(0[1-9]|1[0-2])$'   # 月份正则
      region:
        type: enum
        values: [华东, 华北, 华南, 西南, 西北, 东北]  # 差分白名单（D16-D2）：只开大区不开省份
        required: false              # 缺省 = 全部大区
```

关键设计点（每条都对应一条 locked decision）：
- `FROM ds_loans.loans` —— **表引用必须带 `ds_{dataset}` 别名前缀**。这不是风格要求：AttachLRU 以 `ds_{id}` 别名 ATTACH（attach_lru.go:94 确认），AST 走查时「全部表引用落在单一 `ds_{声明dataset}` 前缀下」就是「禁跨数据集 join」的机械化判定。
- `count_column` —— k_min 行级抑制需要知道每行的聚合计数。SELECT 必须暴露一个计数控（loader 校验：count_column 必须出现在 select_list 别名中）。
- `params` 的 enum/pattern 在 loader 编译成正则/集合，进 executor 的参数校验闭包（进 SQL 绑定前 fail-closed，D16-A3）。
- **拒绝示例（loader 必须拒）**：`FROM ds_loans.loans JOIN ds_cards.cards`（跨数据集）；`INSERT INTO ...`（非 SELECT）；`:region` 占位但 params 未声明 region（覆盖不全）；params 声明 `status` 但 SQL 无 `:status`（声明冗余——D16-A2「一一对应」双向校验）。

### Pattern 2: kpi_call 工具（retrieve 的 sibling）

**What:** 一个闭包一个通道；闭包捕获 registry 只读接口 + executor。
**When to use:** `internal/agent/tools/kpi_tool.go` 镜像 `retrieve.go` 的 BuildRetrieveTool 形态。

```go
// 形态骨架（Source: internal/agent/tools/retrieve.go 既有模式 + eino v0.9.14 schema）
func BuildKPICallTool(reg *kpi.Registry, exec *kpi.Executor, catalog string) tool.BaseTool {
    return utils.NewTool(
        "kpi_call",
        "按名调用预定义 KPI（聚合指标查询）。可用 KPI 目录：\n"+catalog+ // 启动期生成（D16-B1）
            "\n参数：name=KPI 名（须在目录内）；params=该 KPI 声明的参数对象。",
        func(ctx context.Context, params map[string]any) (string, error) {
            name, _ := params["name"].(string)
            // ① 无匹配 → 结构化错误（D16-B2，不是 Go error——是工具结果）
            // ② 参数对编译 schema 校验 fail-closed（D16-A3）
            // ③ 快照 {func, args, as_of, rows, k_min_applied} JSON 返回
            ...
        },
        // schema.ToolInfo: name 参数带 Enum = registry 全部 KPI 名（eino ParameterInfo.Enum 已确认支持）
    )
}
```

目录生成口径（Discretion 3 建议答案）：按 KPI 名字母序、每行 `name — description（参数摘要：period:月份, region:枚举6值）`；KPI 超过约 30 条时截断到 30 并附「完整目录见 no_matching_kpi 返回」——v1 loans 场景 2-3 条，远不及阈值。

**注意——decision callback 误标风险**：`internal/agent/callbacks.go` 的 decision 回调把「Tool OnStart」标为 `DecisionRetrieve`。kpi_call 加入后若不区分工具名，SSE 时间线会把 KPI 调用误标为 retrieve（见 Pitfall 6）。

### Pattern 3: 「无 SQL 生成面」结构性测试（D8-04 复刻）

**What:** 用 WalkDir + 源码走查断言 agent 可达路径无 SQL 字符串构造，比接口约定更强。
**When to use:** `TestNoSQLGenerationSurface`。

```go
// 骨架：agent 可达包（internal/agent、internal/runtime、internal/server/chat 层）
// 中禁止出现 SQL DML/DQL 关键字字面量；SQL 只允许存在于 internal/kpi、
// internal/duckdb、internal/store（各自有别的测试锁）与 _test.go。
var forbidden = []string{"SELECT ", "INSERT ", "ATTACH ", "DETACH ", "CREATE TABLE"}
filepath.WalkDir("internal/agent", func(...) error {
    if strings.HasSuffix(path, "_test.go") { return nil }
    src := os.ReadFile(path)
    for _, kw := range forbidden {
        if strings.Contains(src, kw) { t.Errorf("%s 含 SQL 生成面 %q", path, kw) }
    }
})
// 辅助接口断言：kpi.Executor 对 agent 侧只暴露 Call(ctx, name, paramsJSON)，
// 不暴露任何 Query(sql string) 形态方法（golang.org/x/tools 不需要——reflect 断言接口集即可）
```

先例：D8-04 TestNoLegacyInterruptAPI / verify-phase-15.sh Stage 08 contract structural bans（现场确认存在）。

### Pattern 4: reads 通道 XOR 双层执法（D11-B1 镜像）

**What:** 加载期拒绝新违规 + 运行时按通道拼 Tool 集。
**When to use:** 两处改动——

**加载期（internal/skill/validate.go，现场确认的扩展点）：**

```go
// Validate() 内，在现有 reads 校验位置扩展：
const ReadDataAggregatedOnly = "data:aggregated_only"
// 1. 识别：reads 中含 data:aggregated_only → channel = data
// 2. XOR（D16-C1）：len(reads) > 1 → 报错点名：
//    "reads 同时声明 data 通道与 tier 读取（[data:aggregated_only 权威]）——两者互斥，混合 Skill 见 D16-C1 deferred"
// 3. tier 分支才走 ResolveTiers（tier.go 现按 "/" 切分，"data:aggregated_only" 无 "/" 会撞 TierEnum 报错
//    ——必须在 ResolveTiers 之前分支，或在 tier.go 首行识别 data: 前缀直接返回空 tiers + channel 标记）
```

**运行时（internal/agent，BuildToolsForSkill / NewAgent 装配）：**

```go
// D16-C3：按 manifest reads 模式拼 Tool 集（镜像 writes: 与 AllowedWriteTargets 求交集的形态）
if manifest.HasDataChannel() {
    tools = []tool.BaseTool{kpiCallTool}          // data 通道：仅 kpi_call，无 retrieve
} else {
    tools = []tool.BaseTool{retrieveTool}          // tier 通道：retrieve + HITL 写工具（现状）
    tools = append(tools, BuildToolsForSkill(manifest, reg)...)
}
```

现状确认：`agent.go:180-256` NewAgent 目前无条件构建 retrieveTool——这是要改的装配分支点；`hitl_tools.go` BuildToolsForSkill 已是按 manifest 动态拼装的先例（writes 维度），reads 维度照抄其形态。skill Registry 热重载每次 reloadOne 都重跑 parseAndValidate（现场确认）→ 加载期守卫在热重载窗口自动生效，无需额外代码。

### Pattern 5: as_of 自动生成（D16-B4）

**What:** executor 调用时从 sqlite datasets 注册表读该 KPI 声明 dataset 的最新 period 水位，拼接调用时刻。

```go
// as_of 结构（同时是 data-reference part 与 kpi_calls 行的字段口径）：
type ASOf struct {
    Period string `json:"period"` // 数据集最新 period 水位（datasets.period，D15-C2 写入）
    Ts     string `json:"ts"`     // 调用时刻 RFC3339（结果元数据：这个答案基于 X 期数据、于 Y 时刻算出）
}
```

period 水位读取面：`store.DatasetRow.Period`（store.go:255-329 现场确认）——注意 datasets 按 name 主键、`dataset` 字段是 D15-C2 业务数据集名；KPI 文件声明的是 `dataset:`（业务名），查询时取该 dataset 的最新 period 行（同一 dataset 可有多 period 批次行，取 `ORDER BY created_at DESC` 第一条的 period——与 LatestImportBatch 同构）。**executor 的 registry 接口应只声明 `LatestPeriod(ctx, dataset) (string, error)` 窄接口**（interface isolation，staging.go DatasetRegistry 先例），避免 kpi 包依赖整个 store.Store。

### Pattern 6: kpi_calls INSERT-only 表（import_batches 照抄）

**What:** dedicated 审计表，无 UPDATE/DELETE 路径。
**DDL 提案（Discretion 6 建议答案）：**

```sql
-- additive 迁移：新表进 schemaSQL 即可（CREATE TABLE IF NOT EXISTS 幂等；
-- 全新表无需 PRAGMA table_info 门——那是给已有表加列用的，14-09 UAT 先例）
CREATE TABLE IF NOT EXISTS kpi_calls (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              TEXT    NOT NULL,             -- 调用时刻（as_of.Ts 同值）
    session_id      TEXT    NOT NULL,             -- 会话关联（audit 表同名列先例）
    skill_id        TEXT    NOT NULL,             -- 调用方 Skill
    func            TEXT    NOT NULL,             -- KPI 名（KPI-02 原文 func 字段）
    args_json       TEXT    NOT NULL,             -- 参数快照（含 suppressed 标记所需的实际入参）
    as_of_period    TEXT    NOT NULL,             -- as_of.period
    k_min_applied   INTEGER NOT NULL,             -- 本次执行生效的 k_min 值
    result_snapshot TEXT    NOT NULL,             -- 全量结果集 JSON：[{region, row_count, overdue_rate|suppressed}, ...]
    signature       TEXT    NOT NULL DEFAULT ''   -- registry 签名哈希（回放时证明定义版本，D16-A4）
);
CREATE INDEX IF NOT EXISTS idx_kpi_calls_func       ON kpi_calls(func);
CREATE INDEX IF NOT EXISTS idx_kpi_calls_session_id ON kpi_calls(session_id);
CREATE INDEX IF NOT EXISTS idx_kpi_calls_ts         ON kpi_calls(ts);
```

`PutKPICall` 照 `putImportBatchSQL` 抄：单条 INSERT、参数化、无任何 UPDATE/DELETE 函数；测试锁 `TestKPICallsInsertOnly`（镜像 TestImportBatchInsertOnly 形态：source-grep 断言 kpi_calls.go 无 UPDATE/DELETE 字面量）。

### Pattern 7: data-reference SSE part（不扩 wire union）

**What:** 复用 `data-decision` chunk（discriminator 不变，ValidTypes 23 不变），DecisionPayload **additively** 加字段 + 新 Decision 值。
**When to use:** P13-L7 先例（RunPath/TransferFrom/TransferTo 就是这么加的，chunk.go:157-170 现场确认）。

```go
// internal/wire/chunk.go — additive 扩展（P13-L7 同构）：
type DecisionPayload struct {
    // ...现有字段不动...
    DataRef *DataReference `json:"dataRef,omitempty"` // 仅 decision == "kpi_reference" 时非空
}
type DataReference struct {          // Discretion 4 建议答案
    Func        string          `json:"func"`
    Args        map[string]any  `json:"args,omitempty"`
    ASOfPeriod  string          `json:"asOfPeriod"`
    ASOfTs      string          `json:"asOfTs"`
    RowCount    int             `json:"rowCount"`             // 含被抑制行
    Suppressed  int             `json:"suppressed"`           // 被抑制行数
    KMinApplied int             `json:"kMinApplied"`
    // 注意：rows 本体不进 SSE（体积 + agent 已从工具结果拿到）；
    // FE Phase 17 sparkline 数据从 kpi_calls 审计行取（admin 只读面）
}
// internal/wire/types.go — 新 Decision 值（string 枚举 additive，永不 repurpose）：
DecisionKPIReference = "kpi_reference"
```

发射点：executor 快照产生后由 kpi_call 闭包经 Emit 链发一枚 `Step=<当跳>, Decision=kpi_reference, DataRef=...` 的 data-decision chunk。TS 镜像（web/src/types/wire-types.ts）加同名 case + 字段（D15 lockstep；FE 渲染本 phase 不做，Phase 17 消费）。

### Anti-Patterns to Avoid

- **admin 可写 KPI 注册面**——任何「运行时新增 KPI」的 HTTP 端点都是 SC-1 明说的注册后门。admin 面本 phase 只读（GET 列表从 registry + kpi_calls 查）。
- **executor 返回 error 表示 no_matching_kpi**——D16-B2 锁定它是「工具调用结果」不是 Go error：agent 要拿到 available_kpis 转述给用户，Go error 只会变成 eino 的工具失败噪声。
- **SQL 字符串拼接参数**——哪怕参数已过枚举校验也必须 `sql.Named` 绑定（纵深防御：schema 校验管值域，绑定管注入，两层独立失效不串联）。
- **k_min 用 HAVING 实现**——`HAVING count>=5` 让行消失，而 D16-D1 要求行保留、值替换 suppressed（快照要如实记录被抑制行）。语义不同构，见 Pitfall 4。
- **部分加载**——10 个 KPI 文件 1 个坏 = 整体失败。绝不「加载 9 个跳过 1 个」（静默降级 = 数据人员不知道自己的 KPI 没生效）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL 语法/表引用校验 | 自写 SQL tokenizer/parser | DuckDB `json_serialize_sql()` + AST JSON 走查 | SQL 方言坑（窗口函数、QUALIFY、CTE）无穷；引擎自己的解析器零漂移 `[CITED: duckdb.org docs]` |
| 参数转义/防注入 | 字符串替换/escape 函数 | `sql.Named` + `$param` 占位 | go-duckdb v2.4.3 原生支持（statement_test.go 验证）；绑定即免疫注入 |
| SSE 新 chunk 类型 | 新 discriminator 进 wire union | data-decision 通道 + DecisionPayload additive 字段 | ValidTypes=23 计数锁 + D15 TS 镜像编译告警不破；P13-L7 先例 |
| INSERT-only 审计 | 通用审计框架 | 照 import_batches.go 抄 60 行 | 先例含测试锁形态；框架是过度设计 |
| YAML frontmatter/schema 解析 | 自写解析 | yaml.v3 + 规则函数 | rules.go 第四次复刻 |
| 工具目录注入 LLM | 运行时查 registry 拼 prompt | 启动期生成 description 快照（D16-B4 同构：闭包捕获编译期产物） | 运行时拼接 = registry 可变性的隐性后门；捕获快照与不可变语义自洽 |

**Key insight:** 这个 phase 的每一块都有仓库内已验证镜像对象。计划的实质是「换语义的复刻」——独创面只有 json_serialize_sql AST 走查一处，且它是声明式规则集（见 Code Examples）。

## Common Pitfalls

### Pitfall 1: `json_serialize_sql` 对非法 SQL 不抛异常——返回 `error:true` JSON
**What goes wrong:** loader 用 `db.QueryRow("SELECT json_serialize_sql(?)", sql).Scan(&ast)` 永远不报 Go 错误；语法错的 SQL 也「成功」返回，AST JSON 里 `error:true`。若不显式检查，坏 SQL 混进 registry。
**Why it happens:** 官方文档示例明确：`SELECT json_serialize_sql('TOTALLY NOT VALID SQL')` 返回 `{"error":true,"error_type":"parser","error_message":"syntax error at or near \"TOTALLY\"",...}` `[CITED: duckdb.org/docs/lts/data/json/sql_to_and_from_json.html]`。
**How to avoid:** Layer 2 校验第一行就 unmarshal 外层信封，`error==true` → fail-closed 拒载（错误文案带 error_message 定位行号 position）。
**Warning signs:** TestKPILoadFailClosed 的「语法错」分支通过得太容易（没有真的走到 error:true 检查）。

### Pitfall 2: 启动顺序死锁——空 datasets 注册表 vs fail-closed 数据集存在性校验
**What goes wrong:** D16-A2 要求「引用数据集必须已在 datasets 注册表」，但全新部署 datasets 注册表为空（数据靠运行后 admin 导入写入 sqlite）。启动期严格校验 = 全新环境服务器永远起不来 = 导不了数据。
**Why it happens:** datasets 注册表是运行时状态（sqlite），KPI 文件是 git 内容——两个世界的启动时序没有天然交集。
**How to avoid:** 推荐「bootstrap 窗口」语义：datasets 注册表**为空**时跳过存在性检查（AST/语法/参数校验照跑），**非空**时严格 fail-closed；每次启动重校验，导入新数据集后重启即收紧。备选：admin 导入 commit 成功后触发一次 KPI 重校验（复杂，v1 不建议）。
**Warning signs:** 全新 clone + `make run` 起不来；或为绕过它把校验整体降级成 warning。

### Pitfall 3: `:param` 占位符与 DuckDB 原生 `$param` 语法不兼容
**What goes wrong:** go-duckdb named parameters 用 `$foo` 占位 + `sql.Named("foo", v)` 绑定；定义文件写 `:period` 直接交给引擎 → Parser error。
**Why it happens:** `:param` 是 CONTEXT 选定的文件语法（数据人员可读性），不是 DuckDB 语法 `[VERIFIED: go-duckdb v2.4.3 module cache statement_test.go:207-218 TestPrepareQueryNamed 用 $baz]`。
**How to avoid:** loader 编译期做一次受控重写：`:name` → `$name`（正则仅限 `[a-zA-Z_][a-zA-Z0-9_]*` 捕获组，重写后再过一遍占位符覆盖检查，双重确认）。重写后的 `$param` 版本才是 registry 里存的 sql。
**Warning signs:** executor 阶段才报 parser error（说明重写漏了或检查顺序反了）。

### Pitfall 4: k_min 的 HAVING 陷阱与 D15-D1 定宽桶交互
**What goes wrong:** 用 `HAVING count(*) >= 5` 实现 k_min——被抑制的行**从结果里消失**而不是「值替换为 suppressed 标记」（D16-D1 原文），快照无法如实记录被抑制行，且与「其余行正常返回」语义冲突。
**Why it happens:** SQL 直觉写法与锁定语义不同构；另外动态合并相邻小桶（common bucketing fix）会破坏 D15-D1「桶边界跨期稳定」——月环比失真。
**How to avoid:** SQL 原样返回全部行 + `count_column` 计数；Go 层 `kmin.go` 纯函数后过滤：`count < kmin` 的行把**值列**替换为 `"suppressed"` 字符串（行与计数保留）。k_min 取值链：文件 k_min > 0 ? : 数据集 k_min_override > 0 ? : KMinDefault=5（三段链与 datarules/datasets 现有语义对齐，desensitize.go:28-39 现场确认）。
**Warning signs:** TestKMinRowSuppression 断言行数变少（错——行数必须不变）。

### Pitfall 5: 决策回调把 kpi_call 误标为 retrieve
**What goes wrong:** `internal/agent/callbacks.go` 的 decision 回调按「Tool OnStart 就发 DecisionRetrieve」工作（代码注释原文）。kpi_call 上线后 KPI 调用在 SSE 时间线上全部显示为「检索知识库」。
**Why it happens:** Phase 3 只有 retrieve 一个工具时这是对的；现在有两个读工具了。
**How to avoid:** 回调里按 RunInfo/tool 名分流：kpi_call 的 OnStart 发 `DecisionKPIReference`（或至少不发 retrieve）；这也是 Pattern 7 data-reference part 的发射时机耦合点——planner 要把「回调分流」与「DataRef 发射」放在同一个任务里。
**Warning signs:** E2E SSE 流里 KPI 调用显示 retrieve decision。

### Pitfall 6: 通道 XOR 漏掉热重载窗口
**What goes wrong:** 加载期校验只写在首次 Scan，热重载路径（reloadOne → parseAndValidate）行为不一致。
**Why it happens:** 校验逻辑放错层（放 Scan 循环而不是 parseAndValidate/Validate 内部）。
**How to avoid:** XOR 校验进 `Validate()` 本体（7-field validator 内）——skill Registry 的 reloadOne 每次重跑 parseAndValidate（loader.go:48 注释现场确认），自动覆盖热重载。
**Warning signs:** TestChannelGuardXOR 只测首次 Load 不测 reloadOne。

### Pitfall 7: 快照体积失控破坏「全量冻结」
**What goes wrong:** KPI 返回 10 万行（如逐客户明细伪装成 KPI——GROUP BY 唯一键），result_snapshot JSON 巨大，sqlite 行级膨胀。
**Why it happens:** 定义文件 SQL 没有聚合（违反「KPI 必须聚合非逐行」）或枚举值域太大（region 枚举了全国 2000 个县）。
**How to avoid:** Layer 1 校验加两条结构性规则：① SQL 必须含 GROUP BY 或聚合函数（min/max/sum/count/avg 任一出现在 select_list）——注意这是**启发式**（GROUP BY 唯一键绕过），所以 ② loader 加行数上限断言参数（如 max_rows: 5000，默认常量），executor 结果超限 = 结构化错误（该 KPI 定义不合格，数据人员改文件）。枚举值域本身由 git 评审纪律把关（D16-D2）。
**Warning signs:** kpi_calls 表体积增速异常；TestKPISnapshotFrozen 用超宽结果集跑超时。

## Code Examples

### Named-parameter 绑定（go-duckdb v2.4.3 实测形态）

```go
// Source: go-duckdb v2.4.3 module cache — statement_test.go:207-218（TestPrepareQueryNamed）
// registry 存的预编译 SQL（loader 已把 :period 重写为 $period）：
//   SELECT region, count(*) AS row_count, ... FROM ds_loans.loans
//   WHERE period = $period GROUP BY region
rows, err := db.QueryContext(ctx, compiledSQL,
    sql.Named("period", args.Period))   // 参数已过枚举/pattern 校验（进绑定前 fail-closed）
```

### json_serialize_sql 加载期校验（Layer 2 骨架）

```go
// Source: duckdb.org/docs/lts/data/json/sql_to_and_from_json.html（官方文档示例结构）
// 外层信封：{"error":bool, "statements":[...]}；语法错不抛异常、error:true。
type sqlEnvelope struct {
    Error       bool          `json:"error"`
    ErrorMsg    string        `json:"error_message"`
    Statements  []sqlStmt     `json:"statements"`
}
type sqlStmt struct {
    Node sqlNode `json:"node"`
}
type sqlNode struct {
    Type      string        `json:"type"`       // "SELECT_NODE" / 其它（拒）
    FromTable *sqlFrom      `json:"from_table"` // {"type":"BASE_TABLE","table_name":"tbl"} 或 {"type":"EMPTY"}
    // AST 递归走查：from_table、join 树、子查询（select_list 里的子 SELECT、
    // from_table.type=="SUBQUERY"）全部表引用收集后统一判定
}
// 校验规则集（全部 fail-closed）：
//  1. envelope.Error == true → 拒（带 error_message）
//  2. len(statements) != 1 → 拒（只允许单条）
//  3. statements[0].node.type != "SELECT_NODE" → 拒（SELECT-only）
//  4. 递归收集 BASE_TABLE.table_name（含 join/子查询）；任一不匹配
//     ^ds_{declaredDataset}\. → 拒（未注册数据集或跨数据集 join）
//  注：文档确认函数语义是「serialize semicolon-separated SELECT statements」；
//  对 INSERT/DELETE 等的返回形态文档未明示——规则 2/3 无论返回 error 还是
//  其它 node.type 都会拒（Wave 0 探针 lock 精确行为，见 Open Questions Q2）
```

### 通道 XOR 校验（internal/skill/validate.go 扩展骨架）

```go
// Source: internal/skill/validate.go 现有 Validate() + tier.go ResolveTiers 现场形态
const ReadDataAggregatedOnly = "data:aggregated_only"

// Validate() 内（reads 校验位置替换/扩展）：
hasData := false
for _, r := range m.Reads {
    if r == ReadDataAggregatedOnly { hasData = true }
}
if hasData {
    if len(m.Reads) > 1 {
        return fmt.Errorf(
            "skill %q: reads 声明 data 通道（data:aggregated_only）的同时声明了 tier 读取 %v ——两者互斥（D16-C1），混合 Skill 是 v1 deferred",
            m.Name, m.Reads)
    }
    // data 通道：不解析 tiers；resolvedTiers 留空；ChannelKind = data
    // （ResolveTiers 不被调用——"data:aggregated_only" 无 "/" 会在 TierEnum 撞错）
} else {
    // 现有路径不变：ResolveTiers 校验 tier 值域
}
```

### data-reference part 发射（wire additive）

```go
// Source: internal/wire/chunk.go DecisionPayload + P13-L7 additive 先例（现场确认）
// 见 §Pattern 7 完整定义。发射侧（kpi_call 闭包 / decision 回调分流后）：
emit(DecisionRecord{
    Step:     sc.current(),
    Decision: wire.DecisionKPIReference,   // "kpi_reference" —— 新枚举值
    DataRef:  &wire.DataReference{Func: name, ASOfPeriod: asOf.Period, ...},
})
// ValidTypes 计数不变（23）：chunk discriminator 仍是 data-decision，
// chunk_test.go 的计数锁不破——P12-L3 同款证明义务由 wire 包测试承担
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| NL2SQL（agent 现写 SQL 查数） | 预定义受控 KPI + 按名调用 | 业界共识（金融场景尤其）；本仓 FEATURES Cap 2 定为 anti-feature | SQL 生成面收敛到注册期；agent 面 zero-SQL |
| 每 KPI 一个工具 | 单一 kpi_call + description 目录 | ARCHITECTURE Q2（本研究现场复核） | Tool 列表稳定，KPI 增长不动 agent 配置 |
| 运行时注册/管理面（DB 存 KPI 定义） | 文件唯一真源 + 启动期编译 | dbt/metricflow 生态同款「定义即代码」哲学 | git 评审 = KPI 变更审批流；无注册后门 |
| metricflow 形态声明式度量 DSL | 裸 SQL 模板 + 命名参数 | D16-A2 拒绝 DSL | v1 表达力最大、校验面最小；DSL 留 v1.1+ |

**Deprecated/outdated（本仓语境）:**
- ARCHITECTURE.md Q2 早期草图的 `kpi_function` sqlite 表设计——**已被 D16-A1 明确拒绝**（admin 可写=注册后门）。planner 只按 YAML 文件真源规划，忽略 Q2 里的表设计残留（Q2 的 Tool archetype 部分仍有效）。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `json_serialize_sql` 对 INSERT/DELETE 等非 SELECT 语句返回 error:true 或非 SELECT_NODE node（两条判定路径都拒） | Pattern 7 / Code Examples | 低——规则 2/3 双保险；Wave 0 探针 3 行测试 lock 真实行为 |
| A2 | go-duckdb v2.4.3 静态链接的 JSON 扩展含 `json_serialize_sql`（README 只列 JSON 模块名，未逐函数列举） | Standard Stack / Pattern 1 | 低——同左侧：Wave 0 探针直接验证；若缺失则 Layer 2 改为 `PREPARE` 编译探测（语句能 PREPARE 即语法有效，但表引用走查需降级为 Layer 1 文本规则） |
| A3 | AST 的 join/子查询表引用会以 BASE_TABLE 节点出现在可走查位置（官方示例只展示了单表 FROM；join 树的具体 JSON 形态未在文档示例中出现） | Code Examples | 中——走查函数可能要适配真实 join JSON 形态；Wave 0 探针建议用一条含 JOIN 的 SQL 打印真实 AST 再定走查代码 |
| A4 | datasets.period 水位取「该 dataset 最新 created_at 行的 period」语义（与 LatestImportBatch 同构） | Pattern 5 | 低——planner 期定稿即可；测试用 fixture 锁语义 |
| A5 | bootstrap 窗口方案（空 datasets 注册表跳过存在性检查）是可接受的 D16-A2 落地形态 | Pitfall 2 / Open Questions Q1 | 中——若用户要求绝对严格（空注册表也拒），全新部署需先导数据再起服，需用户确认启动顺序约定 |

**除 A1-A5 外，本研究的全部实现级断言均来自仓库代码现场读取（VERIFIED）或官方文档（CITED），无需用户确认。**

## Open Questions

1. **datasets 为空时的启动语义（Pitfall 2 / A5）**
   - What we know: D16-A2 要求注册期校验「引用数据集必须已注册」；全新部署注册表必然为空。
   - What's unclear: 严格 fail（先导数据才能起服）还是 bootstrap 窗口（空表跳过存在性检查）。
   - Recommendation: bootstrap 窗口 + 每次启动重校验 + 测试锁「空注册表不拒载但非空注册表严格拒」。planner 可直接采纳；若不采纳需在 plan 里写明部署顺序约定。

2. **`json_serialize_sql` 精确行为 lock（A1/A2/A3）**
   - What we know: 官方文档确认 SELECT 序列化、error:true 信封、BASE_TABLE 节点。
   - What's unclear: 非 SELECT 语句返回形态；join 树的真实 JSON 形态；静态链接是否含该函数。
   - Recommendation: Wave 0 一个 3-5 用例的 WSL-gated 探针测试（valid SELECT / invalid SQL / INSERT / 跨别名 JOIN / 空 FROM），先跑通再写走查逻辑。这是本 phase 唯一的真未知。

3. **admin 只读 API 端点是否本 phase 交付（Discretion 10）**
   - What we know: KPI-01 的 UI 归 Phase 20；D16-A4 说「admin 只读视图 = 文件加载结果 + kpi_calls 组合，无需新持久表」。
   - What's unclear: 最小只读面（GET /api/admin/kpis 列表）是否本 phase 出。
   - Recommendation: 出最小形态（一个 GET 端点，从 registry + kpi_calls 聚合，只读）——它是 TestSameKPIAcrossSkills 与运营排障的现成探针，成本一个 plan task；不做 UI。

4. **kpi_call 的 decision 发射与 retrieve 回调分流（Pitfall 5）**
   - What we know: 现有回调按 Tool OnStart 发 DecisionRetrieve；kpi_call 必须分流。
   - What's unclear: 分流逻辑放 callbacks.go（按工具名）还是 kpi_tool 闭包自己 emit。
   - Recommendation: callbacks.go 按工具名分流（一处改，时间线语义集中管理）；DataRef 快照字段由闭包经 context 传递或闭包直接 emit——planner 按 eino callback handler 的 RunInfo 可得性定。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go（Windows host） | hermetic 层编译与测试 | ✓ | go1.25.0 windows/amd64 | — |
| MSYS2 gcc（Windows） | sqlite CGO（kpi_calls 测试） | ✓ | 16.1.0 | — |
| WSL Ubuntu-22.04 | go-duckdb CGO（Layer 2 / executor / k_min / 回放测试） | ✓ | go1.25.0 linux/amd64 + gcc 11.4.0 | — |
| go-duckdb v2.4.3 | SQL 执行 + json_serialize_sql | ✓（go.mod 锁定） | v2.4.3 | — |
| datasets 测试 fixture | executor/k_min/回放测试 | ✓ | Phase 15 已有 staging→commit 测试管线（复用其造数形态） | — |
| Milvus/BGE/GLM（live 服务） | 本 phase 无依赖（hermetic 全覆盖） | — | — | 不适用——本 phase 零 live 依赖 |

**Missing dependencies with no fallback:** 无。
**Missing dependencies with fallback:** 无。

**WSL 调用口径（沿 Phase 15 既有形态，verify-phase-15.sh 现场确认）：**
```bash
MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c \
  'export PATH="$PATH:/usr/local/go/bin" && cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/kpi/ -run TestXxx -count=1 -timeout 300s'
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go 标准 `testing` + testify v1.11.1（全仓既有） |
| Config file | 无集中配置——各包 `_test.go`（沿仓库现状） |
| Quick run command（Windows hermetic） | `go test ./internal/kpi/ ./internal/skill/ ./internal/wire/ -run '<快速子集>' -count=1` |
| Full suite command（WSL） | `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && go test ./... -count=1 -race -timeout 900s'` |

### Phase Requirements → Test Map（11 个 test lock 的 hermetic/WSL 分类）

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| KPI-01 | registry 运行时不可变（接口只读 + 结构断言） | unit（hermetic） | `go test ./internal/kpi/ -run TestKPIRegistryImmutableAtRuntime -count=1` | ❌ Wave 0 |
| DATA-09 | agent 可达路径无 SQL 生成面（WalkDir 禁令） | structural（hermetic） | `go test ./internal/agent/ -run TestNoSQLGenerationSurface -count=1` | ❌ Wave 0 |
| DATA-09/D16-A2 | KPI 加载 fail-closed（语法/非 SELECT/未注册/跨集 join/参数未覆盖） | unit — Layer 1 hermetic + Layer 2 WSL 双段 | hermetic: `go test ./internal/kpi/ -run TestKPILoadFailClosed -count=1`；Layer 2 分支: WSL 同命令（tag 或子测试名区分） | ❌ Wave 0 |
| DATA-07/08 | reads XOR 互斥 → Skill 加载失败点名冲突字段 | unit（hermetic） | `go test ./internal/skill/ -run TestChannelGuardXOR -count=1` | ❌ Wave 0 |
| DATA-08/D16-C3 | data 通道 Tool 集={kpi_call}，tier 通道={retrieve} | unit（hermetic） | `go test ./internal/agent/ -run TestToolsForChannel -count=1` | ❌ Wave 0 |
| DATA-12/D16-B2 | 未知 KPI 名 → no_matching_kpi + available_kpis | unit（hermetic——无匹配分支不需要引擎） | `go test ./internal/kpi/ -run TestNoMatchingKPIRefusal -count=1` | ❌ Wave 0 |
| D16-A3/D2 | 枚举外参数在工具边界被拒，不进 SQL 绑定 | unit（hermetic——校验在绑定前） | `go test ./internal/kpi/ -run TestEnumParamFailClosed -count=1` | ❌ Wave 0 |
| D16-D1 | 小桶行值替换 suppressed + k_min_applied + 其余行正常 | unit — kmin.go 纯函数 hermetic；executor 集成 WSL | hermetic: `go test ./internal/kpi/ -run TestKMinRowSuppression -count=1`；executor 段 WSL | ❌ Wave 0 |
| KPI-02/D16-D3 | 快照全量冻结；数据集删除后凭审计行回放 | integration（WSL——需真 DuckDB 数据集 + sqlite） | WSL: `go test ./internal/kpi/ ./internal/store/... -run TestKPISnapshotFrozen -count=1` | ❌ Wave 0 |
| D16-B4 | as_of 系统生成（period 水位+时刻）；参数 schema 无 as_of | unit（hermetic——LatestPeriod 用 stub 窄接口） | `go test ./internal/kpi/ -run TestASofAutoGenerated -count=1` | ❌ Wave 0 |
| D16-C4 | 同 KPI 同参数跨 Skill 逐字节一致 | integration（WSL——真执行） | WSL: `go test ./internal/kpi/ -run TestSameKPIAcrossSkills -count=1` | ❌ Wave 0 |
| KPI-02（表） | kpi_calls DDL + INSERT-only 纪律 | unit（hermetic——sqlite 经 MSYS2 gcc 可链接） | `go test ./internal/store/... -run 'TestKPICallsDDL\|TestKPICallsInsertOnly' -count=1` | ❌ Wave 0 |
| D16-B3 | data-reference part wire 形态 + ValidTypes 计数不破 | unit（hermetic） | `go test ./internal/wire/ -count=1` | 既有 chunk_test.go 计数锁扩展 ❌ Wave 0 |
| A1/A2/A3 | json_serialize_sql 行为探针（Wave 0 首任务） | probe（WSL） | WSL: `go test ./internal/kpi/ -run TestProbeJSONSerializeSQL -count=1 -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** Windows hermetic 子集 `go build ./... && go test ./internal/kpi/ ./internal/skill/ ./internal/wire/ ./internal/agent/ -count=1 -run '<本任务相关>'`
- **Per wave merge:** WSL 全量 `go test ./... -count=1 -race -timeout 900s`
- **Phase gate:** `make verify-phase-16`（新 verify 脚本，镜像 verify-phase-15.sh 的 Stage 结构：gcc 前置 → build+vet → 全量 -race → kpi/skill/wire/agent 分包套件 → 结构禁令 → 源码 grep 锁）

### Wave 0 Gaps
- [ ] `internal/kpi/define.go loader.go registry.go executor.go validate_text.go validate_sql.go kmin.go` 空骨架 + 包声明（全部测试的宿主）
- [ ] `internal/kpi/probe_test.go` — TestProbeJSONSerializeSQL（WSL 探针，lock A1/A2/A3 后再写 validate_sql.go 走查逻辑）
- [ ] `internal/skill/validate_test.go` 增 TestChannelGuardXOR 用例（含 reloadOne 路径）
- [ ] `internal/store/sqlite/kpi_calls.go` + schema.go DDL + TestKPICallsDDL/TestKPICallsInsertOnly
- [ ] `internal/wire/` DecisionPayload.DataRef + DecisionKPIReference + chunk_test 计数锁复核
- [ ] `kpis/loans.yaml` 示例受控文件（2-3 个 KPI，含 enum + pattern 参数各一）——executor 测试的 fixture 即真实交付物

*(测试框架本身无缺口——go test + testify 既有设施全覆盖)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | 本 phase 不动认证（admin 只读面沿既有 admin 边界） |
| V3 Session Management | no | session_id 仅作为 kpi_calls 关联列写入 |
| V4 Access Control | yes | 通道守卫 = 工具级访问控制（加载期 XOR 拒绝 + 运行时按 reads 拼 Tool 白名单——LLM 看不到即调不到）；registry 无任何运行时写路径 |
| V5 Input Validation | yes | 三层：kpi_call 边界参数 schema（enum/pattern，fail-closed）→ `sql.Named` 绑定（注入免疫）→ SQL 模板注册期 AST 校验（SELECT-only/单数据集） |
| V6 Cryptography | no | 不引入新密码学（签名哈希是 sha256 完整性用途，非密码学边界） |
| V12 Logging/Monitoring | yes | kpi_calls INSERT-only 审计（全量快照冻结，防抵赖回放）；与 T-3-AUDIT 同纪律 |

### Known Threat Patterns for KPI 聚合通道（Go + DuckDB + LLM agent）

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL 注入（经 LLM 工具参数） | Tampering | 参数永远 `sql.Named` 绑定 + 值域 schema 前置拒绝 + SQL 模板注册期编译（agent 无拼接面） |
| Prompt injection 诱导越通道（data Skill 试图调 retrieve / 要求原始行） | Elevation | 运行时 Tool 集按通道拼装（越通道工具不存在）+ 铁律①②（platformfloor 文案与结构互证） |
| KPI 差分再识别（两次快照差分还原小群体） | Information Disclosure | 枚举值域即白名单（注册期拒细粒度参数）+ k_min 行级抑制；Phase 17 eval 断言验证 |
| 注册后门（运行时新增/篡改 KPI 定义） | Tampering | 文件唯一真源 + registry 编译期不可变 + 结构性测试锁 + admin 只读 |
| as_of 伪造（声称数据比实际新） | Repudiation | as_of 系统生成（period 水位+时刻），参数 schema 无 as_of 字段；快照冻结可对账 |
| 审计快照泄漏敏感明细 | Information Disclosure | KPI 定义强制聚合（GROUP BY/聚合函数校验 + 行数上限）+ k_min 抑制进快照 |
| 跨数据集 join 再识别（Pitfall 11 场景） | Information Disclosure | AST 走查全部表引用必须落单一 `ds_{dataset}` 前缀；D14-D1 物理隔离兜底 |

## Sources

### Primary (HIGH confidence — 仓库现场读取)
- `internal/data/rules.go` + `datarules/loans.yaml` — 受控 YAML loader 先例（symlink guard/sha256/fail-closed 全形态）
- `internal/agent/tools/retrieve.go` — kpi_call sibling 模板（闭包/ToolInfo/JSON 形态）
- `internal/agent/hitl_tools.go` + `internal/agent/hitl/registry.go` — D11-B1/B3 双层执法镜像对象
- `internal/skill/manifest.go validate.go loader.go tier.go registry.go` — reads 扩展点 + XOR 落点 + 热重载路径
- `internal/store/sqlite/schema.go import_batches.go` + `internal/store/store.go` — additive DDL 模式 + DatasetRow period/k_min 面
- `internal/duckdb/connector.go attach_lru.go staging.go` — ds_{id} 别名 ATTACH、LRU、窄接口先例
- `internal/wire/types.go chunk.go` + `internal/agent/callbacks.go` — Decision additive 模式 + 回调分流点
- `internal/platformfloor/platform_floor.go` — 铁律①③⑦ 文案锚
- `internal/data/desensitize.go` — KMinDefault=5 + k_min 语义链
- go.mod（2026-08-24 读）— 版本锁定确认；Windows/WSL go1.25.0 双端探测确认

### Secondary (MEDIUM-HIGH confidence — 官方文档/模块缓存)
- `[CITED: duckdb.org/docs/lts/data/json/sql_to_and_from_json.html]` — json_serialize_sql 签名/参数、error:true 信封、BASE_TABLE/table_name 节点、SELECT 语义（2026-08-24 全文读取）
- `[VERIFIED: go-duckdb v2.4.3 module cache statement_test.go:207-218]` — `$foo` + sql.Named 绑定
- `[VERIFIED: go-duckdb v2.4.3 README:371]` — ICU/JSON/Parquet/Autocomplete 静态链接
- `[VERIFIED: eino v0.9.14 tool.go:254-263]` — ParameterInfo Enum/SubParams
- `.planning/phases/15-p10-1/15-RESEARCH.md` — Validation Architecture 结构先例
- `.planning/research/FEATURES.md Cap 2 / ARCHITECTURE.md Q2 / PITFALLS.md 11/12/14/22` — 上一层研究定稿（Q2 的 kpi_function 表设计已被 D16-A1 取代——以 CONTEXT 为准）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 零新包，全部 go.mod 锁定存量，现场核对版本
- Architecture: HIGH — 四个镜像先例的代码形态全部现场读取；装配点（validate.go/agent.go/schema.go/chunk.go）逐一定位
- Pitfalls: HIGH — 除 json_serialize_sql 三条行为细节（A1/A2/A3，均有 Wave 0 探针兜底）外，全部 pitfall 来自仓库既有代码语义推演
- Validation: HIGH — hermetic/WSL 分类沿 D15-01-1 既有边界，11 个 test lock 全部映射到命令与宿主文件

**Research date:** 2026-08-24
**Valid until:** 2026-09-23（仓库内先例不漂移则长期有效；外部依赖仅 json_serialize_sql 一处，DuckDB LTS 文档稳定）
