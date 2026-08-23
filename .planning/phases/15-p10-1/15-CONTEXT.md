# Phase 15: 数据导入管线 + 导入即脱敏（P10.1）- Context

**Gathered:** 2026-08-23
**Status:** Ready for planning
**Source:** `/gsd:discuss-phase 15` — Default mode (4 questions per area, 4 areas, 16 locked decisions)
**Requirements:** DATA-01, DATA-02, DATA-15, DATA-16, DATA-17 (5 v1.0 requirements) — 5 Success Criteria (ROADMAP §Phase 15)

<domain>
## Phase Boundary

数据人员可把 CSV / SQL dump / API 三通道数据经两阶段（预览→确认提交）导入为脱敏数据集——L1 HMAC + L2 桶化在导入时一次完成，原始文件导入即弃且可审计证明（「导入即脱敏，原始文件即弃」）。

**本 phase 交付：**
- 三通道导入：CSV（UTF-8 / UTF-8-BOM / GBK / GB18030 自动检测解码）、SQL dump（sqlite 方言 copy-table，**永不执行 dump 内 SQL**）、API push（multipart 流式，**push 即提交**）
- L1 HMAC（站点盐 + 数据集盐 + 字段域分离；哈希前规范化）+ L2 定宽桶（fail-closed 规则检查）
- 两阶段预览/提交（双传 + 内容哈希桥接；磁盘上从无 raw 文件）
- 幂等重导入（同 (dataset, period) 同内容哈希 → 无操作去重；不同内容 → 新批原子替旧批 + supersedes 链式审计）
- 批次审计（INSERT-only，含 rules_sha256 + raw 弃置证明）
- 数据集删除 API 端点（X-Admin-Token 门 + 删除审计行）
- k_min 配置面（全局默认 5 + 数据集级 override 存 datasets 注册表）
- DuckDB 写入一律 Appender/批量路径（Phase 14 已建 ≥10 万行/s 基准门）

**不交付（deferred）：**
- KPI 聚合通道 / `data:aggregated_only` 通道守卫（Phase 16）
- 再识别 eval 断言（Phase 17——k_min 在本 phase 只交付配置面）
- 导入管理 UI（Phase 20 admin 四视图）
- data.lock.json 冻结锁决策（Phase 20 收尾，第 5 冻结锁）
- 分类泛化规则（城市→省映射表）与增量追加模式（v1.1）

</domain>

<decisions>
## Implementation Decisions

### A. 脱敏规则载体（Area 1）

- **D15-A1: 混合载体——代码 + 文件。** 标准 PII 识别规则（手机号/证件号/姓名等字段类型判定 + 哈希前规范化链）在 Go 代码（`internal/data/`，随仓库版本化）；每数据集的列→规则映射在声明式文件（如 `data/rules/{dataset}.yaml`），数据人员可写，平台加载期校验 fail-closed。与 Phase 16 KPI 注册表「受控文件」哲学对齐。拒绝：纯 Go 代码（数据人员每加一列依赖开发者——Pitfall 21 原推荐，摩擦过大）、纯声明式文件（识别规则质量风险进文件面）。
- **D15-A2: 显式 passthrough 白名单。** 规则文件可显式声明 passthrough 列（update_ts、状态码等非敏感技术列）；未声明且无规则的列 = 拒绝导入（fail-closed 不破）。每列都有显式决定，审计性质保持，同时避免技术列也要写规则的膨胀。拒绝：全部未知列拒（规则文件膨胀）、自动保守脱敏（类型猜测 = Pitfall 21 D2 放行变体）。
- **D15-A3: 规则修正 = 删数据集重导。** 某列已按 passthrough 导入、事后发现敏感 → 删除数据集 + 从企业源文件重新导入。源始终在企业侧，SkillForge 内无 raw 不可恢复是设计而非缺陷；路径唯一且物理正确。拒绝：二次脱敏迁移（引入「脱敏数据二次变换」新概念，桶化已损精度无法修复，语义复杂）。
- **D15-A4: 规则集哈希进审计。** 每批导入的审计行携带当时生效的规则集内容哈希（rules_sha256）——「这批数据按哪版规则脱敏」可回放可举证，规则文件改动对批次的影响可追溯。与 D14-B1/B3 平台地板版本进审计同构。拒绝：semver 版本号（内容一致性靠纪律，忘 bump 即漂移）、不携带（规则-批次对账关系丢失）。

### B. 预览会话与 raw 生命周期（Area 2）

- **D15-B1: 双传 + 哈希桥接。** 预览调用：上传 → 流式读 + 计算 preview → **raw 即弃**，服务器只存预览会话（内容哈希 + 调整后桶边界 + 样本脱敏版）。提交调用：重传同一文件，按内容哈希匹配预览会话 → 按新边界全量脱敏落库。raw 两次都在单请求生命周期内消失，**磁盘上从无 raw**（不是「落盘后删除」——Pitfall 13 的最强解释）。代价：大文件双传输（月度导入低频，接受）。附带红利：**预览的 A 不会提交成 B**（与 D11-C4「批的 A 不会执行成 B」同构）。拒绝：staging 暂存 raw + TTL（GC 定时器不是安全边界，举证变弱）、上传即脱敏 + 改名提交（桶边界调整后无法重算，与 DATA-15 冲突）。
- **D15-B2: API push 即提交。** 机器通道 push 直接走完整脱敏管线 + fail-closed 规则检查，任一列无规则 → 整批结构化拒绝；可选 dry-run 规则校验接口供调用方预检。两阶段预览是人用通道（CSV/SQL dump）的能力，机器调用方不需要人看预览。拒绝：强制两阶段（纯摩擦）、即提交 + 回执报告（本质是提交后回执不是预览）。
- **D15-B3: 永不执行 dump。** SQL dump 通道解析 sqlite `.dump` 格式（CREATE TABLE + INSERT 行），只提取 schema 和行数据，走与 CSV 同一条脱敏管线；dump 中任何其他语句（DROP/UPDATE/PRAGMA/ATTACH/触发器…）→ 整批拒绝。与平台铁律「永不编写/执行/建议 SQL」同构，攻击面最小。拒绝：沙箱执行后拷出（等于执行任意 SQL，与平台叙事相抵）。
- **D15-B4: 原文→脱敏对照展示。** 预览响应给前 100 行每列的原文值与拟脱敏值对照（如「张三 → a3f8…」），数据人员可验证映射与规范化正确性（手机号 +86 归一对不对、姓名列是否漏脱）。数据人员本就持有 raw 文件，回传不扩大泄露面。**实现约束：预览响应不落盘、不进常规日志。** 拒绝：只展示脱敏后（映射错误不易发现）。

### C. 幂等冲突语义（Area 3）

- **D15-C1: 新批替旧批。** 同 (dataset, period) 不同内容哈希（月度修正重导）→ 新批原子替换该 period 旧数据（staging 写完 → 原子交换，失败旧数据完好——D14-D4 模式复用）；新批审计行携带 `supersedes_batch_id` 链式可追溯；KPI 查询永远看到最新。旧数据可删——KPI 快照冻结在审计行，回放不受影响（v1.0 不变量保证）。拒绝：拒绝须先删（月度修正是常规操作，每次两步是持续摩擦）、双版本并存（as_of 选择面复杂化，存储双倍，v1 过度设计）。
- **D15-C2: period 显式必填参数。** 导入请求必须显式携带 period（如 `"2026-07"`），格式加载期校验。数据人员对自己的数据期负责；自动推断错月份 = 数据放错期且难发现（KPI 时间窗错位是静默错误）。拒绝：自动推断（对每家企业 schema 都是猜测）、无 period 整替（与 DATA-16 去重键冲突，滚动多月数据集无法表达）。
- **D15-C3: API 删除端点随本 phase 交付。** `DELETE /api/import/dataset/:name`（X-Admin-Token 常量时间比较门，复用 /eval 模式）+ 删除动作写 INSERT-only 审计行。理由：D15-A3 修正路径需要它；不变量「删除只作用于脱敏数据集 + 审计可查」要求删除是平台动作而非 `rm`。Phase 20 admin UI 只是给它穿 UI。拒绝：手动文件删除（注册表与文件系统不一致 + 无审计）、延到 Phase 20（15-19 期间无安全删除路径）。
- **D15-C4: 全量快照语义。** 每批导入 = 该 (dataset, period) 的全量数据，替换落库。与 DATA-16 内容哈希去重天然自洽（同全量重传 → 同哈希 → 无操作）；金融月表导出本就是全量；部分数据误当全量会被预览行数对比暴露。拒绝：支持增量追加（增量批内容哈希每批必不同，(dataset, period) 去重键失效，幂等语义复杂化）。

### D. 桶化默认策略 + k_min（Area 4）

- **D15-D1: 定宽桶默认。** 金额类连续敏感字段默认定宽桶（如 1000 一桶：「0-999 / 1000-1999」）。可解释性最强（KPI 引用可说明）；**跨期边界稳定 → 月环比趋势可比**（Phase 16 KPI 聚合在桶化列上 group-by 的前提）；长尾稀疏桶由 k_min 防线兜底。每字段可在规则文件里选其他方案。拒绝：数量级桶（同桶内差异 10 倍，下钻粒度粗）、分位数桶（桶边界依赖本批数据 → 跨期漂移 → 月环比失真）。
- **D15-D2: 桶边界调整仅本批生效。** 预览时调整的桶边界只影响本批导入（存进预览会话 → 提交时应用，D15-B1 哈希桥接的一部分）；规则文件不变。想固化新边界 → 显式改规则文件（文件是唯一真源，rules_sha256 审计语义干净）。拒绝：可回写规则文件（API 自动改版本化文件破坏人工评审纪律）。
- **D15-D3: k_min 全局默认 5 + 数据集级 override。** 代码常量全局默认 `k_min=5`（Pitfall 11 推荐值）+ 数据集级 override 随导入存入 datasets 注册表（Phase 16 KPI 查询时读取）。金融严敏数据集可调 10 无需动代码。拒绝：仅全局常量（无法差异化）、每集必填（摩擦大且容易全填 5 应付）。
- **D15-D4: v1 L2 仅数值桶化。** L2 规则类型只有数值桶化（定宽/数量级可参数化）。分类敏感字段（地区/职业）要么 L1 哈希、要么 passthrough + 依赖 k_min 查询防线（Pitfall 11 的 zip3 场景由 k_min 挡）。分类泛化映射表留 v1.1。拒绝：v1 含分类泛化（每张映射表都是新维护面 + 预览/审计/规则哈希多兼容一种类型）。

### Claude's Discretion

- **预览会话过期/TTL 形态**（哈希桥接下过期会话无害——只能匹配同一内容；纯卫生问题，planner 定）
- **multipart 大小上限与流式缓冲形态**（STACK-VERIFICATION §2.8 容量纪律内落）
- **预览响应不落盘的实现细节**（日志过滤/中间件形态——D15-B4 约束的落地）
- **HMAC 规范化链具体形态**（NFC/trim/全角空格/手机号 +86——ROADMAP 明说 plan 期定稿，STACK-VERIFICATION §3.2 已给设计）
- **GBK/BOM 解码管线实现**（§3.1 已设计：剥 BOM → utf8.Valid → GB18030 解码 → csv.Reader，照做）
- **规则文件具体 schema**（YAML 字段形态、passthrough 列声明语法、桶方案参数化）
- **datasets 注册表列扩展**（period、k_min override、supersedes 链、rules_sha256 落哪列——D14-D2 表基础上 additive）
- **批次审计行的表形态**（新表 vs 复用 audit 表新 action type——T-3-AUDIT INSERT-only 模式沿用）
- **dry-run 规则校验端点形态**（D15-B2 的可选预检）
- **SQL dump 解析器对 .dump 格式的容错边界**（多语句事务块、引号转义等具体解析规则）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 研究文档（实现级设计已给全）
- `.planning/research/STACK-VERIFICATION.md` §3.1 — GBK/GB18030 + BOM 解码管线（剥 BOM → utf8.Valid → GB18030 NewDecoder → csv.Reader；零新增依赖）
- `.planning/research/STACK-VERIFICATION.md` §3.2 — L1 HMAC 哈希前规范化（NFC → trim 含全角空格 → 字段特定规则；站点盐+数据集盐+字段域分离）
- `.planning/research/STACK-VERIFICATION.md` §2.8 — Appender 批量纪律（月批千万行级逐行 = 小时级、批量 = 分钟级）+ 部署纪律
- `.planning/research/PITFALLS.md` Pitfall 13 — raw 文件生命周期（GC 不是安全边界；错误路径清理；D15-B1 是其最强解）
- `.planning/research/PITFALLS.md` Pitfall 21 — 脱敏规则漂移（fail-closed + 规则注册表 + schema diff 报告；D15-A1/A2 落地其推荐）
- `.planning/research/PITFALLS.md` Pitfall 11 — 聚合 join 再识别（k_min=5 推荐值；本 phase 只交付配置面，断言在 Phase 17）
- `.planning/research/FEATURES.md` Capability 1 — 导入管线 table stakes + differentiators（导入时脱敏是平台执行不是 Skill 声明；审计证明 raw 弃置）
- `.planning/research/ARCHITECTURE.md` Q1 — 数据层 package 布局（`internal/data/` NEW package 定论）

### 需求 + 路线图锚
- `.planning/REQUIREMENTS.md` §Financial Data Layer — DATA-01/02 原文；§v1.0 Requirements — DATA-15/16/17 原文 + 贯穿性不变量（审计 INSERT-only / 清理只作用脱敏数据集 / KPI 快照冻结）
- `.planning/ROADMAP.md` §Phase 15 — Goal / 5 SC / Pitfalls addressed / Research flags（「规范化的具体形态 plan 期定稿」）

### 前置 phase 决策（直接继承）
- `.planning/phases/14-p10-0-cgo-trust-floor/14-CONTEXT.md` — D14-D1（每数据集一 .duckdb）/ D14-D2（sqlite datasets 注册表）/ D14-D3（懒 ATTACH LRU）/ D14-D4（staging + 原子改名——D15-C1 supersede 直接复用）

### 已交付核心资产（直接扩展）
- `internal/duckdb/connector.go` — NewConnector boot-queries（hermetic 常量）+ `SetMaxIdleConns(0)` + WAL 刷盘纪律
- `internal/duckdb/attach_lru.go` — 懒 ATTACH + LRU（D14-D3，Phase 15 导入后数据集读取直接用）
- `internal/duckdb/staging.go` — staging 原子改名 + 目录约定（`data/staging/` + `data/datasets/`，D14-D4；**Phase 15 导入管线直接消费**）
- `internal/store/sqlite/schema.go:206` — datasets 注册表 DDL（D14-D2；本 phase additive 扩列）
- `internal/store/sqlite/audit.go` — T-3-AUDIT INSERT-only + `?`-parameterized 先例（批次审计行复刻）
- `internal/server/` X-Admin-Token 常量时间比较门模式（/eval 先例——D15-C3 删除端点复用）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/duckdb/` 三件套**（connector + attach_lru + staging）— Phase 14 刚交付；导入落库的连接模式、数据集读取、staging 原子改名全部现成，Phase 15 只需在 staging 之上建导入编排
- **`internal/store/sqlite/schema.go` datasets 表** — 注册表已建（D14-D2），本 phase additive 加列（period / k_min_override / supersedes 链 / rules_sha256 等 planner 定）
- **Appender 基准门**（`appender_throughput_test.go`，≥10 万行/s）— 导入写入路径的性能契约已锁，Phase 15 不允许逐行 INSERT 循环有现成测试模式可参照
- **X-Admin-Token 中间件模式**（/eval + Phase 14 admin 预览端点）— 删除端点 + 导入 API 认证直接复用

### Established Patterns
- **T-3-AUDIT INSERT-only** — 批次审计行、supersede 链、删除审计行全部复刻；永不 UPDATE/DELETE
- **D14-D4 staging 原子改名** — 导入提交（staging .duckdb → datasets/）与 supersede 替换（新文件写 staging → 原子交换）同一模式两处复用
- **fail-closed 加载期校验** — Skill 7-field validator / KNOW-02 tier-mandatory 的镜像：规则文件校验失败 = 拒绝导入，不是运行时降级
- **D8-04 WalkDir 禁令门** — 可复刻为「导入路径禁止逐行 INSERT」或「预览响应不落盘」类结构性断言

### Integration Points
- **导入 API 挂 `internal/server`**（Hertz 路由注册表 server.go 模式）；multipart 流式读（Hertz 原生）
- **新 package `internal/data/`**（ARCHITECTURE Q1 定论）— 规则库 + 脱敏管线 + 导入编排
- **x/text v0.22.0 已在依赖树** — GB18030 解码 + NFC 规范化零新增依赖
- **audit 主表 vs 新表**：批次审计行的落表形态 planner 裁决（Claude's Discretion 已记）

</code_context>

<specifics>
## Specific Ideas

- **「预览的 A 不会提交成 B」** — D15-B1 哈希桥接与 D11-C4「批的 A 不会执行成 B」（HITL 冻结参数快照）同一哲学的导入层延伸：提交请求的内容哈希必须与预览会话逐字节匹配，否则拒绝
- **「库内无可恢复原始值」的最强解释** — raw 从不落盘（双传模式），而非「落盘后删除」；这是 Pitfall 13 的结构性解，不依赖 GC/TTL/清扫器
- **raw 即弃 + 源在企业侧 = 删数据集重导是唯一修正路径** — 这不是缺陷是设计（D15-A3）；文档叙事要把这一点讲成信任卖点而非运维负担
- **定宽桶的月环比可比性** — Phase 16 KPI 在桶化列上 group-by，桶边界跨期漂移（分位数桶）会让趋势失真——D15-D1 的选择是为 Phase 17 的趋势 sparkline 铺路
- **k_min 本 phase 只交付配置面** — 存进 datasets 注册表，Phase 16 查询时读、Phase 17 eval 断言收口；不要在本 phase 提前实现查询侧防线

## Specific test lock requirements

- **TestPreviewCommitHashBridge**：预览会话 A 的哈希 + 提交文件 B（内容不同）→ 拒绝提交（预览的 A 不会提交成 B）
- **TestRawNeverOnDisk**：预览 + 提交全流程后，data/staging/ 与 data/datasets/ 无任何 raw 格式文件（CSV/dump 原件）；磁盘上只有 .duckdb 脱敏数据集
- **TestUnknownColumnFailClosed**：CSV 含规则库外新列且未 passthrough 声明 → 整批结构化拒绝（含列名错误信息）
- **TestDumpNonInsertStatementRejected**：dump 含 DROP/UPDATE/PRAGMA/ATTACH 任一语句 → 整批拒绝（D15-B3）
- **TestHMACNormalizationJoinability**：「张三」「`张三␣`（尾空格）」「+8613800000000」「138-0000-0000」规范化后恒同哈希（§3.2 join 生命线）
- **TestIdempotentReimport**：同 (dataset, period) 同内容重导 → 返回已有 batch_id，无新增数据行、无副作用（DATA-16）
- **TestSupersedeAtomicSwap**：新批替换失败（注入写失败）→ 旧数据集完好可查；成功 → supersedes_batch_id 链在审计行可查
- **TestBucketBoundaryStableAcrossPeriods**：同字段同值在不同 period 批次落同一定宽桶（D15-D1 跨期可比性）
- **TestBatchAuditRowShape**：每批导入审计行含 `{batch_id, raw_size_bytes, desensitized_row_count, discard_ts, dataset, rules_sha256}`（DATA-17 + D15-A4）
- **TestDeleteDatasetAudited**：删除端点产出 INSERT-only 审计行；datasets 注册表与文件系统状态一致（D15-C3）

</specifics>

<deferred>
## Deferred Ideas

- **分类泛化规则**（L2 映射表，城市→省）— v1.1（D15-D4 拒绝项；k-匿名经典手段，但映射表维护成本高）
- **增量追加导入模式** — v1.1（D15-C4 拒绝项；内容哈希幂等语义不兼容）
- **二次脱敏迁移**（对已存脱敏数据再变换）— v1.1（D15-A3 拒绝项；删数据集重导是 v1 唯一路径）
- **KPI 聚合通道 + `data:aggregated_only` 守卫** — Phase 16
- **再识别 eval 断言**（聚合 join + KPI 差分）— Phase 17
- **导入管理 UI（批次视图）** — Phase 20 admin
- **data.lock.json 第 5 冻结锁决策** — Phase 20 收尾

None — discussion stayed within phase scope（无 scope creep 项）

</deferred>

---

*Phase: 15-p10-1*
*Context gathered: 2026-08-23 via /gsd:discuss-phase 15 (Default mode, 4 areas × 4 questions = 16 decisions)*
