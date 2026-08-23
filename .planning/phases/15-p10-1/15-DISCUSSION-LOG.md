# Phase 15: 数据导入管线 + 导入即脱敏（P10.1）- Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-23
**Phase:** 15-p10-1
**Areas discussed:** 脱敏规则载体, 预览会话与 raw 生命周期, 幂等冲突语义, 桶化默认策略 + k_min

---

## 脱敏规则载体

### Q1: 脱敏规则库放哪里？

| Option | Description | Selected |
|--------|-------------|----------|
| 混合：代码+文件 | 标准 PII 识别规则（手机号/证件号/姓名等字段类型判定 + 哈希前规范化链）在 Go 代码；每数据集的列→规则映射在声明式文件（data/rules/{dataset}.yaml），数据人员可写，加载期校验 fail-closed。与 Phase 16 KPI 受控文件哲学一致。 | ✓ |
| 纯 Go 代码仓库化 | 全部规则（含字段类型识别）都在 internal/data/rules.go，加规则 = 代码提交 + PR 评审。审计链最硬，但数据人员每新增一列都要开发者介入（Pitfall 21 原推荐）。 | |
| 纯声明式文件 | 规则全部在数据人员可写的声明式文件里（含字段类型识别规则），平台只做加载期校验。自主性最大，但识别规则质量风险进文件面。 | |

**User's choice:** 混合：代码+文件（推荐）
**Notes:** → D15-A1

### Q2: fail-closed 的粒度：未知列怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| 显式 passthrough 白名单 | 规则文件可显式声明 passthrough 列（非敏感技术列）；未声明且无规则的列 = 拒绝导入。每列都有显式决定，fail-closed 语义不破。 | ✓ |
| 全部未知列拒 | 任何不在规则库里的列一律拒绝，包括时间戳/状态码。最严格，但金融数据技术列多，规则文件膨胀。 | |
| 自动保守脱敏 | 未知列按类型猜测自动落 L2 桶或 L1 哈希。无需人工干预，但「猜测」是 Pitfall 21 D2 放行的变体。 | |

**User's choice:** 显式 passthrough 白名单（推荐）
**Notes:** → D15-A2

### Q3: 规则修正的语义（passthrough 列事后发现敏感）？

| Option | Description | Selected |
|--------|-------------|----------|
| 删数据集重导 | 源始终在企业侧，SkillForge 无 raw 不可恢复是设计而非缺陷。路径唯一且物理正确，v1 不加二次脱敏复杂度。 | ✓ |
| 支持二次脱敏 | 对已存储脱敏数据做二次脱敏迁移。少一次重导，但引入「二次变换」新概念，桶化已损精度无法修复。 | |
| 不处理 | v1 不定义此流程。风险：出事时无预案，临时操作可能破坏审计链。 | |

**User's choice:** 删数据集重导（推荐）
**Notes:** → D15-A3

### Q4: 批次审计行是否携带规则集版本？

| Option | Description | Selected |
|--------|-------------|----------|
| 规则集哈希进审计 | rules_sha256 携带在每批导入审计行，「这批数据按哪版规则脱敏」可回放可举证。与 D14-B1/B3 同构。 | ✓ |
| semver 版本号 | 人工 bump，人可读，但版本号与内容一致性靠纪律维持。 | |
| 不携带 | 审计行只记 DATA-17 原文字段。规则-批次对账关系丢失。 | |

**User's choice:** 规则集哈希进审计（推荐）
**Notes:** → D15-A4

---

## 预览会话与 raw 生命周期

### Q1: 预览→提交的机制：raw 在两次调用之间怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| 双传 + 哈希桥接 | 预览：流式读 + 计算 preview → raw 即弃，只存预览会话（内容哈希 + 桶边界 + 样本脱敏版）。提交：重传同文件按哈希匹配 → 全量脱敏落库。磁盘从无 raw；附带「预览的 A 不会提交成 B」。代价：大文件双传输。 | ✓ |
| staging 暂存 raw + TTL | 预览上传的 raw 保留在 data/staging/，提交时直接用。省一次传输，但 Pitfall 13 明言 GC 定时器不是安全边界。 | |
| 上传即脱敏 + 改名提交 | 上传即流式脱敏写 staging .duckdb，预览看脱敏结果，提交 = 原子改名。raw 最短命，但桶边界调整后无法重算——与 DATA-15 冲突。 | |

**User's choice:** 双传 + 哈希桥接（推荐）
**Notes:** → D15-B1

### Q2: API push（机器到机器）通道是否也走两阶段？

| Option | Description | Selected |
|--------|-------------|----------|
| push 即提交 | 机器通道直接走完整脱敏管线 + fail-closed 规则检查；可选 dry-run 规则校验。两阶段预览是人用通道的能力。 | ✓ |
| 也强制两阶段 | 对机器调用方是纯摩擦，异步推送方状态机更复杂。 | |
| 即提交 + 回执报告 | 提交后回执带脱敏统计。本质是提交后回执不是预览。 | |

**User's choice:** push 即提交（推荐）
**Notes:** → D15-B2

### Q3: SQL dump 通道的安全边界？

| Option | Description | Selected |
|--------|-------------|----------|
| 永不执行 dump | 解析 .dump 格式（CREATE TABLE + INSERT），只提取 schema 和行数据走同一脱敏管线；其他语句 → 整批拒绝。与平台铁律「永不执行未知 SQL」同构。 | ✓ |
| 沙箱执行后拷出 | dump 灌进一次性内存 sqlite 执行再拷出。能处理更复杂 dump，但等于执行任意 SQL，与平台叙事相抵。 | |

**User's choice:** 永不执行 dump（推荐）
**Notes:** → D15-B3

### Q4: 「拟脱敏效果」的展示形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 原文→脱敏对照 | 前 100 行每列同时给原文值与拟脱敏值，可验证映射与规范化正确性。数据人员本就持有 raw，回传不扩大泄露面；约束：预览响应不落盘、不进常规日志。 | ✓ |
| 只展示脱敏后 | 只含脱敏后前 100 行 + 规则标识 + 桶边界示例。egress 面最小，但映射错误不易发现。 | |

**User's choice:** 原文→脱敏对照（推荐）
**Notes:** → D15-B4

---

## 幂等冲突语义

### Q1: 同 (dataset, period) 但内容哈希不同怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| 新批替旧批 | 原子替换 + supersedes_batch_id 链式审计；KPI 查询永远最新；旧数据可删（KPI 快照冻结在审计行）。月度修正一步到位。 | ✓ |
| 拒绝，须先删 | 两步操作显式性最强，但月度修正是常规操作，持续摩擦。 | |
| 双版本并存 | 多批次按 batch_id 并存，可回滚，但查询语义复杂化 + 存储双倍。 | |

**User's choice:** 新批替旧批（推荐）
**Notes:** → D15-C1

### Q2: period（数据期）怎么声明？

| Option | Description | Selected |
|--------|-------------|----------|
| 显式必填参数 | 导入请求显式携带 period（如 "2026-07"），格式加载期校验。杜绝自动推断错期的静默污染。 | ✓ |
| 自动推断 | 平台从日期列推断。少填一个参数，但错期静默污染 (dataset, period) 键。 | |
| 无 period 整替 | 与 DATA-16 去重键冲突，滚动多月数据集无法表达。 | |

**User's choice:** 显式必填参数（推荐）
**Notes:** → D15-C2

### Q3: 数据集删除的操作面？

| Option | Description | Selected |
|--------|-------------|----------|
| API 删除端点 | DELETE /api/import/dataset/:name（X-Admin-Token 常量时间比较门）+ INSERT-only 删除审计行。D15-A3 修正路径需要；不变量要求删除是平台动作而非 rm。 | ✓ |
| 手动文件删除 | 无端点开发量，但注册表与文件不一致 + 删除无审计。 | |
| 延到 Phase 20 | 15-19 期间无安全删除路径，临时操作破坏审计链。 | |

**User's choice:** API 删除端点（推荐）
**Notes:** → D15-C3

### Q4: 批次数据形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 全量快照语义 | 每批 = 该 (dataset, period) 全量数据替换。与 DATA-16 内容哈希去重自洽；部分数据误当全量会被预览行数对比暴露。 | ✓ |
| 支持增量追加 | 增量批内容哈希每批必不同，(dataset, period) 去重键失效，幂等语义复杂化。 | |

**User's choice:** 全量快照语义（推荐）
**Notes:** → D15-C4

---

## 桶化默认策略 + k_min

### Q1: 连续敏感字段的默认桶方案？

| Option | Description | Selected |
|--------|-------------|----------|
| 定宽桶默认 | 如 1000 一桶。可解释性最强、跨期边界稳定 → 月环比趋势可比；长尾稀疏桶由 k_min 兜底；每字段可选其他方案。 | ✓ |
| 数量级桶默认 | 自适应范围大，但同桶内差异 10 倍，KPI 下钻粒度粗。 | |
| 分位数桶默认 | 信息损失均匀，但桶边界依赖本批数据 → 跨期漂移 → 月环比失真。 | |

**User's choice:** 定宽桶默认（推荐）
**Notes:** → D15-D1

### Q2: 预览期调整的桶边界，作用域？

| Option | Description | Selected |
|--------|-------------|----------|
| 仅本批生效 | 调整存进预览会话，提交时应用；规则文件不变。想固化 → 显式改规则文件（唯一真源）。 | ✓ |
| 可回写规则文件 | API 自动改版本化文件破坏「文件唯一真源 + 人工评审」纪律，rules_sha256 语义复杂化。 | |

**User's choice:** 仅本批生效（推荐）
**Notes:** → D15-D2

### Q3: k_min 的默认值与配置粒度？

| Option | Description | Selected |
|--------|-------------|----------|
| 全局默认+集级覆盖 | 代码常量 k_min=5（Pitfall 11 推荐值）+ 数据集级 override 存 datasets 注册表（Phase 16 查询时读）。 | ✓ |
| 仅全局常量 | 最简单，但严敏与粗粒度数据集无法差异化。 | |
| 每集必填 | 强制显式思考，但摩擦大且容易全填 5 应付。 | |

**User's choice:** 全局默认+集级覆盖（推荐）
**Notes:** → D15-D3

### Q4: v1 是否包含分类泛化（如城市→省）？

| Option | Description | Selected |
|--------|-------------|----------|
| v1 仅数值桶化 | 分类敏感字段要么 L1 哈希、要么 passthrough + 依赖 k_min 查询防线。映射表维护成本高，留 v1.1。 | ✓ |
| 含分类泛化 | k-匿名手段更完整，但每张映射表都是新维护面 + 预览/审计/规则哈希多兼容一种类型。 | |

**User's choice:** v1 仅数值桶化（推荐）
**Notes:** → D15-D4

---

## Claude's Discretion

- 预览会话过期/TTL 形态（哈希桥接下过期无害，纯卫生问题）
- multipart 大小上限与流式缓冲形态
- 预览响应不落盘的实现细节（日志过滤/中间件）
- HMAC 规范化链具体形态（NFC/trim/全角/手机号——ROADMAP 定为 plan 期定稿）
- GBK/BOM 解码管线实现（§3.1 已设计，照做）
- 规则文件具体 schema（YAML 字段、passthrough 声明语法、桶方案参数化）
- datasets 注册表列扩展（period / k_min_override / supersedes / rules_sha256 落哪列）
- 批次审计行的表形态（新表 vs 复用 audit 表新 action type）
- dry-run 规则校验端点形态
- SQL dump 解析器容错边界

## Deferred Ideas

- 分类泛化规则（L2 映射表）→ v1.1
- 增量追加导入模式 → v1.1
- 二次脱敏迁移 → v1.1
- KPI 聚合通道 + `data:aggregated_only` 守卫 → Phase 16
- 再识别 eval 断言 → Phase 17
- 导入管理 UI → Phase 20
- data.lock.json 第 5 冻结锁决策 → Phase 20 收尾
