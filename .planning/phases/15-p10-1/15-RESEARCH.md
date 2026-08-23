# Phase 15: 数据导入管线 + 导入即脱敏（P10.1）- Research

**Researched:** 2026-08-23
**Domain:** Go 数据导入管线（CSV/SQL dump/multipart 流式）+ 确定性脱敏（HMAC-SHA256 + 定宽桶）+ DuckDB Appender 批量写入 + 幂等/审计
**Confidence:** HIGH（全部核心 API 已对 module cache / 本机实测验证；零新增外部依赖）

---

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

**A. 脱敏规则载体（Area 1）**

- **D15-A1: 混合载体——代码 + 文件。** 标准 PII 识别规则（手机号/证件号/姓名等字段类型判定 + 哈希前规范化链）在 Go 代码（`internal/data/`，随仓库版本化）；每数据集的列→规则映射在声明式文件（如 `data/rules/{dataset}.yaml`），数据人员可写，平台加载期校验 fail-closed。与 Phase 16 KPI 注册表「受控文件」哲学对齐。拒绝：纯 Go 代码（数据人员每加一列依赖开发者——Pitfall 21 原推荐，摩擦过大）、纯声明式文件（识别规则质量风险进文件面）。
- **D15-A2: 显式 passthrough 白名单。** 规则文件可显式声明 passthrough 列（update_ts、状态码等非敏感技术列）；未声明且无规则的列 = 拒绝导入（fail-closed 不破）。每列都有显式决定，审计性质保持，同时避免技术列也要写规则的膨胀。拒绝：全部未知列拒（规则文件膨胀）、自动保守脱敏（类型猜测 = Pitfall 21 D2 放行变体）。
- **D15-A3: 规则修正 = 删数据集重导。** 某列已按 passthrough 导入、事后发现敏感 → 删除数据集 + 从企业源文件重新导入。源始终在企业侧，SkillForge 内无 raw 不可恢复是设计而非缺陷；路径唯一且物理正确。拒绝：二次脱敏迁移。
- **D15-A4: 规则集哈希进审计。** 每批导入的审计行携带当时生效的规则集内容哈希（rules_sha256）——「这批数据按哪版规则脱敏」可回放可举证。拒绝：semver 版本号、不携带。

**B. 预览会话与 raw 生命周期（Area 2）**

- **D15-B1: 双传 + 哈希桥接。** 预览调用：上传 → 流式读 + 计算 preview → **raw 即弃**，服务器只存预览会话（内容哈希 + 调整后桶边界 + 样本脱敏版）。提交调用：重传同一文件，按内容哈希匹配预览会话 → 按新边界全量脱敏落库。raw 两次都在单请求生命周期内消失，**磁盘上从无 raw**。代价：大文件双传输（月度导入低频，接受）。附带红利：**预览的 A 不会提交成 B**。拒绝：staging 暂存 raw + TTL、上传即脱敏 + 改名提交。
- **D15-B2: API push 即提交。** 机器通道 push 直接走完整脱敏管线 + fail-closed 规则检查，任一列无规则 → 整批结构化拒绝；可选 dry-run 规则校验接口供调用方预检。拒绝：强制两阶段、即提交 + 回执报告。
- **D15-B3: 永不执行 dump。** SQL dump 通道解析 sqlite `.dump` 格式（CREATE TABLE + INSERT 行），只提取 schema 和行数据，走与 CSV 同一条脱敏管线；dump 中任何其他语句（DROP/UPDATE/PRAGMA/ATTACH/触发器…）→ 整批拒绝。拒绝：沙箱执行后拷出。
- **D15-B4: 原文→脱敏对照展示。** 预览响应给前 100 行每列的原文值与拟脱敏值对照。**实现约束：预览响应不落盘、不进常规日志。** 拒绝：只展示脱敏后。

**C. 幂等冲突语义（Area 3）**

- **D15-C1: 新批替旧批。** 同 (dataset, period) 不同内容哈希（月度修正重导）→ 新批原子替换该 period 旧数据（staging 写完 → 原子交换，失败旧数据完好——D14-D4 模式复用）；新批审计行携带 `supersedes_batch_id` 链式可追溯。拒绝：拒绝须先删、双版本并存。
- **D15-C2: period 显式必填参数。** 导入请求必须显式携带 period（如 `"2026-07"`），格式加载期校验。拒绝：自动推断、无 period 整替。
- **D15-C3: API 删除端点随本 phase 交付。** `DELETE /api/import/dataset/:name`（X-Admin-Token 常量时间比较门，复用 /eval 模式）+ 删除动作写 INSERT-only 审计行。拒绝：手动文件删除、延到 Phase 20。
- **D15-C4: 全量快照语义。** 每批导入 = 该 (dataset, period) 的全量数据，替换落库。拒绝：支持增量追加。

**D. 桶化默认策略 + k_min（Area 4）**

- **D15-D1: 定宽桶默认。** 金额类连续敏感字段默认定宽桶（如 1000 一桶）。**跨期边界稳定 → 月环比趋势可比**（Phase 16 KPI 聚合前提）。每字段可在规则文件里选其他方案。拒绝：数量级桶、分位数桶。
- **D15-D2: 桶边界调整仅本批生效。** 预览时调整的桶边界只影响本批导入（存进预览会话 → 提交时应用）；规则文件不变。拒绝：可回写规则文件。
- **D15-D3: k_min 全局默认 5 + 数据集级 override。** 代码常量全局默认 `k_min=5` + 数据集级 override 随导入存入 datasets 注册表。拒绝：仅全局常量、每集必填。
- **D15-D4: v1 L2 仅数值桶化。** L2 规则类型只有数值桶化。分类敏感字段（地区/职业）要么 L1 哈希、要么 passthrough + 依赖 k_min 查询防线。分类泛化映射表留 v1.1。

### Claude's Discretion

- 预览会话过期/TTL 形态（哈希桥接下过期会话无害——只能匹配同一内容；纯卫生问题，planner 定）
- multipart 大小上限与流式缓冲形态（STACK-VERIFICATION §2.8 容量纪律内落）
- 预览响应不落盘的实现细节（日志过滤/中间件形态——D15-B4 约束的落地）
- HMAC 规范化链具体形态（NFC/trim/全角空格/手机号 +86——ROADMAP 明说 plan 期定稿，STACK-VERIFICATION §3.2 已给设计）
- GBK/BOM 解码管线实现（§3.1 已设计：剥 BOM → utf8.Valid → GB18030 解码 → csv.Reader，照做）
- 规则文件具体 schema（YAML 字段形态、passthrough 列声明语法、桶方案参数化）
- datasets 注册表列扩展（period、k_min override、supersedes 链、rules_sha256 落哪列——D14-D2 表基础上 additive）
- 批次审计行的表形态（新表 vs 复用 audit 表新 action type——T-3-AUDIT INSERT-only 模式沿用）
- dry-run 规则校验端点形态（D15-B2 的可选预检）
- SQL dump 解析器对 .dump 格式的容错边界（多语句事务块、引号转义等具体解析规则）

### Deferred Ideas (OUT OF SCOPE)

- 分类泛化规则（L2 映射表，城市→省）— v1.1
- 增量追加导入模式 — v1.1
- 二次脱敏迁移（对已存脱敏数据再变换）— v1.1
- KPI 聚合通道 + `data:aggregated_only` 守卫 — Phase 16
- 再识别 eval 断言（聚合 join + KPI 差分）— Phase 17
- 导入管理 UI（批次视图）— Phase 20 admin
- data.lock.json 第 5 冻结锁决策 — Phase 20 收尾

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DATA-01 | 三通道导入（CSV 含 GBK/GB18030 + BOM 解码 / SQL dump / API push）+ tier + schema 元数据 | §Code Examples 解码管线（x/text GB18030 已验证）；.dump 语法 ground truth（本机 sqlite3 3.53.0 实测）；Hertz 流式 multipart 三选项已验证（§Pitfall 1）；Appender catalog 寻址已验证 |
| DATA-02 | 导入即强制脱敏（L1 哈希前规范化 + L2 桶化），raw 不落库 | §HMAC 规范化链定稿（NFC/NFKC 全角实测证据）；§HMAC 构造推荐（站点盐→数据集盐推导）；D15-A1/A2 fail-closed 载体已定型 |
| DATA-15 | 两阶段导入：预览（前 100 行 + 拟脱敏 + 拟 tier + 桶边界可调）→ 确认提交 | §预览会话形态（in-memory + 哈希桥接）；「raw 从不落盘」与 Hertz 默认 multipart 行为的冲突及解法（§Pitfall 1——pre-parse 会把 raw 写进 OS 临时文件，必须禁用） |
| DATA-16 | 幂等重导入：内容哈希 + (dataset, period) 去重 | §(dataset, period) → 注册表映射方案（复合名复用 ValidateDatasetID 语法）；§supersede 算法（backup-rename 链，复用 CommitStagedDataset） |
| DATA-17 | 每批 INSERT-only 审计行证明 raw 已弃 | §批次审计表形态两个选项与推荐；D14-D2 datasets 表 additive 扩列路径（ALTER-gated 先例） |

</phase_requirements>

---

## Summary

Phase 15 的地基已由 Phase 14 全部打好：`internal/duckdb/` 三件套（connector boot-queries / AttachLRU 懒挂载 / CommitStagedDataset 原子提交）、sqlite `datasets` 注册表（`INSERT OR REPLACE` on name）、Appender 吞吐基准门（≥10 万行/s）。本 phase 的新增面是：`internal/data/` 新 package（规则文件加载 + 脱敏引擎 + 三通道导入编排 + 预览会话 + 幂等/审计）、Hertz 导入 API（multipart 流式）、以及规则文件目录。**零新增外部依赖**——`x/text v0.28.0`（GB18030 + NFC/NFKC）、`gopkg.in/yaml.v3`（规则文件）、`crypto/hmac`（stdlib）全部已在 go.mod。

研究的四个 research flag 全部收口，且每个都带本机实测/模块源码证据：(1) **Q1 package 布局**——`internal/data/` 为域逻辑层，依赖 `internal/duckdb`（引擎适配）与 `internal/store/sqlite`（注册表），方向镜像 agent→store，无环；注意 ARCHITECTURE.md Q1 原稿的 sqlite `dataset_row` 表设计已被 Phase 14 的「每数据集一 .duckdb 文件」决策取代，以 Phase 14 实际代码为准。(2) **Q7 分类器位置**——REQUIREMENTS Out of Scope 已禁 LLM 自动分类 + D15-A1/A2 已锁静态声明，无运行时分类器；「分类」= 规则文件列名 → Go 规则库的确定性查表。(3) **§3.1 GBK/BOM**——定稿为「剥 BOM → 首块 utf8.Valid 探测 → UTF-8 严格校验流 / GB18030 解码流」，含流式变体（multipart 不能回读，整文件 `utf8.Valid` 不可行）。(4) **§3.2 HMAC 规范化**——本机 Go 实测证实 `strings.TrimSpace` 自动覆盖全角空格 U+3000、NFC 不折叠全角数字而 NFKC 会，据此定稿三档规范化 profile（person_name = NFC+trim；phone_cn = NFKC+去分隔符+去 +86；id_card_cn = NFKC+大写）。

两个必须带入 plan 的冲突发现：**(a) Hertz 默认 multipart 行为违反「raw 从不落盘」**——默认路径把 >16MB 的文件 part 溢写到 OS 临时文件，且默认 body 上限只有 4MB；必须 `WithStreamBody(true)` + `WithDisablePreParseMultipartForm(true)` + 提高 `WithMaxRequestBodySize`（三者均已在 hertz v0.10.6 源码验证，官方 doc comment 明示前两者需成对使用）。**(b) CONTEXT 提议的 `data/rules/` 目录与 `.gitignore` 的 `data/` 整目录忽略冲突**——规则文件需要版本化（D15-A1「受控文件」），应放版本化路径（如仓库根 `datarules/`）或给 gitignore 加例外。此外本机 sqlite3 3.53.0 CLI 实测的 `.dump` ground truth 显示：真实 dump 必然含 `PRAGMA foreign_keys=OFF` / `BEGIN TRANSACTION` / `COMMIT` 样板句、可能含 `CREATE INDEX` / `sqlite_sequence` 块 / `unistr()` 字面量——D15-B3 的「其他语句整批拒绝」必须配套一个显式跳过样板句清单，否则每个真实 dump 都会被拒。

**Primary recommendation:** 建 `internal/data/` 新 package（rules/normalize/desensitize/decode/dump/importer/preview/batch 八文件分层），导入端点挂 `internal/server`，写入路径 = 专用 staging Connector + `NewAppenderFromConn`，提交/替换路径复用 `CommitStagedDataset`（supersede 用 backup-rename 三步链），规则文件放版本化目录 `datarules/{dataset}.yaml`，Hertz 侧三选项成对开启 + 分路由 Content-Length 守卫。

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CSV 字节解码（BOM/GBK/GB18030） | `internal/data/decode.go`（导入管线内部） | — | 频道特定变换，属于 ingest 域；x/text 是纯库无 I/O |
| sqlite .dump 解析（永不执行） | `internal/data/dump.go` | — | accept-list 文法扫描器是域逻辑；D15-B3 锁定不借 sqlite 引擎执行 |
| multipart 流式读取 | `internal/server/import_api.go`（Hertz handler） | stdlib `mime/multipart` | HTTP 帧解析属 server 层；`multipart.NewReader(BodyStream(), boundary)` 即 stdlib |
| 规则文件加载 + fail-closed 校验 | `internal/data/rules.go` | `datarules/*.yaml` | 加载期校验镜像 Skill 7-field validator；文件是数据面，校验是代码面（D15-A1） |
| L1 规范化 + HMAC | `internal/data/normalize.go` + `desensitize.go` | — | 纯函数、可确定性测试（TestHMACNormalizationJoinability） |
| L2 定宽桶 | `internal/data/desensitize.go` | — | 纯函数；边界跨期稳定是 D15-D1 契约 |
| 预览会话（哈希桥接） | `internal/data/preview.go`（进程内） | — | 单实例部署已锁（STACK-VERIFICATION §2.8）；过期会话无害（D15-B1） |
| staging .duckdb 写入 | `internal/duckdb`（专用 Connector + Appender） | `internal/data/importer.go` 编排 | 引擎适配已由 Phase 14 定型；导入批持独立 Connector 写 staging 文件 |
| 原子提交 / supersede 替换 | `internal/duckdb/staging.go`（扩展） | `internal/data/batch.go` 编排 | D14-D4 模式；supersede 需要 backup-rename 扩展（见 Pattern 5） |
| datasets 注册表 + 批次审计行 | `internal/store/sqlite`（additive 迁移） | `internal/store/store.go` 接口 | OLTP 元数据 + INSERT-only 审计的既有先例 |
| 导入/删除 API 端点 | `internal/server`（路由注册表 + isAdminRequest） | — | X-Admin-Token 门复用 audit.go:114-129 |
| k_min 配置面 | `internal/data`（全局常量）+ datasets 注册表（override 列） | Phase 16 读取 | D15-D3；本 phase 只存不查 |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/text` | **v0.28.0**（go.mod 实测，STACK-VERIFICATION 写 v0.22.0 已过时） | `encoding/simplifiedchinese.GB18030.NewDecoder()` + `unicode/norm.NFC/NFKC` | GB18030 是 GBK 超集，一个解码器通吃；norm 是 Unicode 规范化唯一标准库级实现。模块缓存已验证（gbk.go:18 `GB18030 encoding.Encoding`、normalize.go:38 `NFC Form = iota`）[VERIFIED: module cache] |
| `gopkg.in/yaml.v3` | v3.0.1（go.mod 既有 direct dep） | 规则文件 `datarules/{dataset}.yaml` 解析 | 已是直接依赖（eino 之外最早引入）；零新增 [VERIFIED: go.mod] |
| stdlib `encoding/csv` | go1.25.0 | CSV 解析（动态 schema，运行时列名发现） | STACK-VERIFICATION §2.5 已裁决拒绝 gocsv [CITED: STACK-VERIFICATION §2.5] |
| stdlib `crypto/hmac` + `crypto/sha256` | go1.25.0 | L1 确定性脱敏哈希 | stdlib 唯一正解；never hand-roll [CITED: STACK-VERIFICATION §0 #6] |
| stdlib `mime/multipart` | go1.25.0 | multipart 流式分 part 读取 | `multipart.NewReader(io.Reader, boundary)` 是流式原语；Hertz 自己内部也用它（request.go:271）[VERIFIED: module cache] |
| `github.com/marcboeker/go-duckdb/v2` | v2.4.3 | staging .duckdb 写入（Appender） | Phase 14 已引入；Appender 是 SC-1 批量路径唯一形态 [VERIFIED: module cache] |
| `github.com/mattn/go-sqlite3` | v1.14.50 | datasets 注册表 / 批次审计扩展列 | Phase 14 已切换 [VERIFIED: go.mod] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | v1.11.1 | 测试断言（仓内既有惯例） | 全部新测试 |
| Hertz `server.WithStreamBody` / `WithDisablePreParseMultipartForm` / `WithMaxRequestBodySize` | hertz v0.10.6 | 流式大文件上传三选项 | 导入端点前置（见 Pitfall 1）[VERIFIED: module cache option.go:40/152/254] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|-----------|
| 手写 .dump accept-list 解析器 | 用 mattn/go-sqlite3 在 :memory: 执行 dump 后 SELECT 读出 | **被 D15-B3 明确拒绝**（「沙箱执行后拷出 = 执行任意 SQL」）。即使加 sqlite authorizer 回调也属于决策禁止面 |
| 手写 .dump 解析器 | 引 vitess sqlparser 等通用 SQL parser | 重依赖换窄需求；~200 行定向 INSERT-VALUES tokenizer 攻击面更小；违反零新增依赖姿态 |
| x/text GB18030 单解码器 | chardet 库自动探测编码 | GBK/GB18030/UTF-8 三形态由「utf8.Valid 探测 + GB18030 兜底」覆盖，无需探测库（§3.1 设计） |
| HMAC-SHA256 | SHA-256 裸哈希 / Argon2 | 裸哈希无盐可被彩虹表打；Argon2 是密码哈希（慢），确定性 join 场景需要快哈希 |
| in-memory 预览会话 | sqlite 预览会话表 | 单实例部署已锁；重启丢会话无害（只能匹配同一内容哈希）；表是过度设计 |

**Installation:**

```bash
# 零新增依赖——全部已在 go.mod。无需 go get。
```

**Version verification:** 2026-08-23 实测 `go.mod`：x/text v0.28.0 / yaml.v3 v3.0.1 / go-duckdb v2.4.3 / mattn v1.14.50 / hertz v0.10.6 / testify v1.11.1；Go 1.25.0 windows/amd64。

## Package Legitimacy Audit

> 本 phase **不安装任何新包**。上表全部为既有依赖的复用，无新 registry 写入面，无 slopcheck 审计对象。唯一新「外部输入面」是数据人员上传的 CSV/dump 文件本身——其防线是 fail-closed 规则校验 + accept-list dump 解析（见 Security Domain）。

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| （无新增包） | — | — | — | — | — | N/A |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## Architecture Patterns

### System Architecture Diagram

```
数据人员 (浏览器/curl/脚本)                    企业源文件 (CSV / sqlite .dump)
      │                                            │
      ▼                                            ▼
POST /api/import/preview (multipart)     POST /api/import/commit (multipart, 重传)
POST /api/import/push    (multipart, 机器通道 = 即提交)
DELETE /api/import/dataset/:name (X-Admin-Token)
      │
      ▼
internal/server/import_api.go
  ├─ isAdminRequest (删除端点；导入端点 planner 定是否同样挂门)
  ├─ multipart.NewReader(BodyStream(), MultipartFormBoundary())   ← 流式，raw 不落盘
  │    WithStreamBody + WithDisablePreParseMultipartForm 已在 server.New 开启
      ▼
internal/data/importer.go  ──────────── 编排（三通道共用后半段）
  ├─ CSV:   decode.go  剥BOM → utf8探测 → GB18030/NewDecoder/UTF8 → csv.Reader
  ├─ dump:  dump.go    accept-list 解析 (CREATE TABLE + INSERT VALUES；样板句跳过；他句整批拒)
  └─ push:  与 CSV 同路径 (multipart part 即文件流)
      ▼
rules.go  加载 datarules/{dataset}.yaml → 列集合比对
  ├─ 列在文件无规则且未 passthrough → 结构化拒绝 (fail-closed, D15-A2)
  └─ rules_sha256 = sha256(规则文件字节) (D15-A4)
      ▼
desensitize.go  每行每列:
  ├─ L1: normalize.go profile → HMAC-SHA256(数据集盐, field域‖0x00‖值) → hex
  ├─ L2: 定宽桶 floor(value/width)*width (D15-D1)
  └─ passthrough 原样
      ▼
预览分支                                提交分支
  ├─ 前100行原文↔脱敏对照                  ├─ 内容哈希 == 预览会话哈希? 否→拒 (D15-B1)
  ├─ 会话{content_sha256, 桶边界, TTL}      ├─ 幂等检查: 同(dataset,period)同哈希 → 返回原 batch_id
  ├─ raw 即弃 (请求结束即不可达)            ├─ supersede: LRU.Detach → 旧文件 rename→staging 备份
  └─ 响应不落盘不落日志 (D15-B4)            │         → 新文件 rename→datasets → 注册表 → 删备份
                                           ▼
                              internal/duckdb 专用 staging Connector
                                CREATE TABLE (脱敏后 schema)
                                NewAppenderFromConn → AppendRow×N → Flush → Close (WAL 刷盘)
                                           ▼
                              CommitStagedDataset (既有) / SupersedeStagedDataset (新)
                                           ▼
                              sqlite: datasets 注册表 (additive 列) + import_batches 审计行
                                {batch_id, raw_size_bytes, desensitized_row_count,
                                 discard_ts, dataset, period, rules_sha256,
                                 supersedes_batch_id, content_sha256}
```

读者可沿「multipart 上传 → 解码 → 规则门 → 脱敏 → staging Appender → 原子提交 → 审计行」追踪主用例；预览分支在 desensitize 后分叉、raw 即弃。

### Recommended Project Structure

```
internal/data/                  # NEW package（Q1 定论——域逻辑层）
├── rules.go                    # 规则文件 schema + loader + fail-closed 校验 + rules_sha256
├── normalize.go                # L1 哈希前规范化 profiles（person_name / phone_cn / id_card_cn）
├── desensitize.go              # L1 HMAC + L2 定宽桶引擎（纯函数）
├── decode.go                   # BOM/GBK/GB18030/UTF-8 解码管线（CSV/push 通道）
├── dump.go                     # sqlite .dump accept-list 解析器（D15-B3）
├── importer.go                 # 三通道导入编排（流式 → 脱敏 → staging 写）
├── preview.go                  # 预览会话 store（in-memory + mutex + TTL）
└── batch.go                    # batch_id 生成 / 幂等检查 / supersede 编排 / 审计行写入

internal/duckdb/
├── staging.go                  # 既有 CommitStagedDataset；新增 SupersedeStagedDataset（Pattern 5）
└── importer_write.go           # (可选) staging Connector + Appender 写入 helper（若不放 importer.go）

internal/store/sqlite/
├── schema.go                   # datasets 表 additive ALTER 列 + import_batches 新表（既有迁移循环追加）
├── datasets.go                 # 既有 PutDataset/ListDatasets；扩展查询（by dataset+period）
└── import_batches.go           # NEW — INSERT-only 批次审计行

internal/server/
├── server.go                   # server.Default 加三选项；路由表 +4 行
└── import_api.go               # preview / commit / push / dry-run / delete handlers

datarules/                      # NEW 版本化目录（gitignore data/ 冲突的解法——见 Open Q2）
└── {dataset}.yaml              # 数据人员可写的列→规则映射（D15-A1）

scripts/verify-phase-15.sh      # NEW — 照 verify-phase-14.sh run_stage 形态
```

依赖方向（镜像 agent→store 先例）：`internal/data` → `internal/duckdb` + `internal/store`；`internal/duckdb` → `internal/store`（staging.go 已 import store.DatasetRow）。**无环**。

### Pattern 1: 流式解码管线（§3.1 定稿——含 multipart 不可回读的流式变体）

**What:** STACK-VERIFICATION §3.1 的四步设计（剥 BOM → utf8.Valid → GB18030 → csv.Reader）在 multipart 流上的落地。
**When to use:** CSV 与 API push 通道的入口。

```go
// Source: STACK-VERIFICATION §3.1 设计 + x/text v0.28.0 模块缓存 API 实测
//
// 关键事实（全部已验证）:
//   - GB18030 是 GBK 超集 → 一个解码器通吃两种 [CITED: STACK-VERIFICATION §3.1]
//   - Decoder.Reader(io.Reader) io.Reader 存在（encoding/encoding.go:78）→ 流式可行
//   - utf8.Valid 只能作用于完整 []byte；multipart 流不可回读 → 整文件探测不可行
//
// 推荐流式形态:
//   1) bufio.Reader 读前 3 字节，== \xEF\xBB\xBF 则丢弃（BOM；不剥则首列名带 BOM，
//      规则文件列名匹配失败 = fail-closed 误拒）
//   2) 再 Peek 64KB 做 utf8.Valid 探测:
//      - 通过   → 余流包 strict UTF-8 校验（utf8.DecodeRune 循环或
//                 transform.NewReader(r, unicode.BOMOverride{})）；
//                 中途遇无效字节 → 整批拒绝（fail-closed：混合编码文件本就是坏数据）
//      - 不通过 → GB18030.NewDecoder().Reader(流)（x/text 解码器对非法序列返回 error，
//                 同样 fail-closed 而非 best-effort 乱码）
//   3) 解码后的 io.Reader 喂 encoding/csv.Reader（LazyQuotes=false 严格模式）
//   4) 全程同时 TeeReader 进 sha256 哈希器（内容哈希是 D15-B1/B4/C1 的桥）
//
// 顺序铁律: utf8 先于 GB18030——GB18030 对任意字节几乎都能"成功"解码出乱码，
// 先试 GBK 会把 UTF-8 文件静默变 mojibake（§3.1 已点名此坑）。
```

### Pattern 2: HMAC 构造与盐推导（§3.2 定稿所需证据 + 推荐形态）

**What:** 站点盐 + 数据集盐 + 字段域分离的落地构造。
**When to use:** desensitize.go 的 L1 路径。

```go
// Source: STACK-VERIFICATION §3.2 设计 + 本机 Go 1.25/x/text v0.28.0 实测（2026-08-23）
//
// 实测证据（决定规范化链形态）:
//   strings.TrimSpace("张三　") == "张三"        // unicode.IsSpace 覆盖全角空格 ✓
//   norm.NFC.String("１３８") == "１３８"            // NFC 不折叠全角数字
//   norm.NFKC.String("１３８") == "138"              // NFKC 折叠全角数字/符号
//
// 推荐规范化 profile（三级，全部进 normalize.go 的代码规则库）:
//   person_name: NFC → TrimSpace                （NFKC 过猛: ①→1 类兼容折叠会并值）
//   phone_cn:    NFKC → 去分隔符(- 空格) → 去国家码前缀(+86/86/86-) → 校验11位
//   id_card_cn:  NFKC → strings.ToUpper → 去空白   （全角数字+x→X 都归一）
//   默认(未声明 profile 的 hash 列): NFC → TrimSpace
//
// 推荐 HMAC 构造（datasetSalt 不落库、确定性推导——无需注册表盐列、无需迁移）:
//   siteSalt   = os.Getenv("SKILLFORGE_DATA_SALT")   // 未设 → 导入端点 fail-closed 拒绝
//                                                        （镜像 isAdminRequest unset 语义）
//   datasetSalt = HMAC-SHA256(key=siteSalt, msg="dsalt:"+dataset)
//   l1Hash      = HMAC-SHA256(key=datasetSalt,
//                             msg="field:"+column+"\x00"+normalizedValue)
//   输出: hex 编码全 64 字符入库（预览展示截断仅为 UI；存储不截断——截断降抗碰撞性）
//
// 推导式盐 vs 随机盐(存注册表) 等价安全（registry 本地 sqlite 可读），推导式少一列少一迁移；
// 若 planner 选随机盐: 盐必须 per-dataset（非 per-period！见 Pitfall 4）。
//
// 字段域分离（§3.2 锁定）: 同值不同列 → 不同哈希。跨数据集同值 → 不同哈希
// （per-dataset 盐）=「拒绝跨数据集推断个体」铁律 ④ 的密码学落地——这是特性不是缺陷，
// 文档叙事应明说。
```

### Pattern 3: staging 写入（专用 Connector + Appender）

**What:** 导入批对 staging .duckdb 的批量写入形态。
**When to use:** importer.go 落库段。

```go
// Source: internal/duckdb/connector.go（既有）+ go-duckdb v2.4.3 appender.go 模块源码
//
// 已验证 API:
//   NewAppender(driverConn driver.Conn, catalog, schema, table string) (*Appender, error)
//   NewAppenderFromConn(driverConn, schema, table)  // catalog="" 即默认库
//   行为: AppendRow 批量缓存，auto-flush 阈值或 Flush()/Close() 时落表
//   不支持列类型仅: INVALID / UHUGEINT / BIT / ANY / BIGNUM（type.go:52-58）
//   → staging 表用 VARCHAR/BIGINT/DOUBLE/DECIMAL/TIMESTAMP 全部安全
//
// 形态: 每导入批一个专用 Connector（DSN = data/staging/{batch}.duckdb）:
//   c, _ := duckdb.New(ctx, duckdb.Config{DSN: stagingPath, MaxOpenConns: 1})
//   defer c.Close()                       // Close 双关 = WAL 刷盘（Pitfall 5，connector.go:182-198）
//   conn, _ := c.Connector().Connect(ctx) // 需暴露 connector 访问（attach_lru.go:73 先例）
//   ExecContext: CREATE TABLE data (...)  // 脱敏后 schema（类型见 Open Q4）
//   app, _ := duckdb.NewAppenderFromConn(conn, "main", "data")
//   循环 app.AppendRow(...)               // 值来自 desensitize 输出，非 raw
//   app.Flush(); app.Close()
//   → CommitStagedDataset（既有）
//
// 注意: 导入写路径不经过 AttachLRU（那是读路径懒挂载）；staging 文件是 Connector 的
// default DB，无需 ATTACH。禁止逐行 INSERT 循环（SC-1；appender_throughput_test.go 基准门
// AppenderMinRowsPerSec=100_000 锁定，connect 器常量 + DUCKDB_APPENDER_MIN_RPS env 覆盖先例）。
```

### Pattern 4: datasets 注册表 (dataset, period) 映射 + additive 扩列

**What:** 幂等键落注册表的形态。
**When to use:** schema.go 迁移 + batch.go。

```go
// Source: internal/store/sqlite/schema.go:206-214（既有 DDL）+ ALTER-gated 迁移先例（C 共享模式）
//
// 既有表: datasets(name TEXT PRIMARY KEY, tier, schema_json, file_path,
//                  content_sha256, batch_id, created_at)
// 既有写路径: putDatasetSQL = INSERT OR REPLACE INTO datasets(...)  （datasets.go:31）
// 既有约束: CommitStagedDataset 强制 Row.Name == 文件名 id（staging.go:112-116）
//
// 推荐映射: 注册名 = "{dataset}__{period}"（如 "loans__2026-07"）
//   - ValidateDatasetID 语法 ^[A-Za-z0-9_-]{1,64}$ 对该形态合法（_ 和 - 都在白名单）
//   - 文件名/ATTACH 别名/注册表 PK 三位一体自动成立——CommitStagedDataset、AttachLRU、
//     删除端点全部零改动复用
//   - 幂等查询: WHERE dataset=? AND period=?（additive 列）
// additive ALTER 列（照 alterAuditAddPlatformFloorVersionSQL 先例, NOT NULL DEFAULT ''）:
//   dataset TEXT NOT NULL DEFAULT ''        -- 逻辑数据集名（__ 前缀）
//   period  TEXT NOT NULL DEFAULT ''        -- "2026-07"（加载期格式校验 YYYY-MM）
//   k_min_override INTEGER NOT NULL DEFAULT 0   -- 0 = 用全局默认 5（D15-D3）
// 索引-after-ALTER 纪律: 引用新列的 CREATE INDEX 必须独立 const 在迁移循环之后
//   ExecContext（D14-E1 契约——idx_audit_run_path 事故先例）
```

### Pattern 5: supersede 三步链（失败旧数据完好的正确形态）

**What:** D15-C1「新批原子替换，失败旧数据完好」的实现算法。
**When to use:** staging.go 新增 SupersedeStagedDataset。

```go
// Source: internal/duckdb/staging.go（D14-D4 既有语义推导）+ Windows 锁边界（Pitfall 4/§3.5）
//
// 为什么不能直接 os.Rename(new, target) 覆盖: 覆盖后旧字节即灭。若第 4 步注册表写失败，
// 旧数据已物理丢失——违反 D15-C1「失败旧数据完好」。备份链才有补偿可退。
//
// 步骤（全部在 LRU.Detach(id) 成功之后）:
//   1) os.Rename(target, backup)    // 旧文件 → data/staging/.superseded-{id}-{ts}.duckdb
//                                    // （同卷原子；staging 只含脱敏数据，短暂留备份无害）
//   2) os.Rename(stagingNew, target)// 新文件就位（既有 rename 语义）
//   3) Registry.PutDataset(新行)     // INSERT OR REPLACE on name —— 同名天然替换
//   失败补偿: 2 失败 → rename(backup, target) 还原; 3 失败 → 双 rename 还原
//   成功尾步: os.Remove(backup)      // 已 DETACH，Windows 删得动
//   审计行 supersedes_batch_id = 旧 batch_id（从注册表旧行读出后再替换）
//
// 既有 CommitStagedDataset 的 ErrDatasetExists 守卫对 supersede 是反语义（它拒已存在目标）
// ——supersede 需要独立入口（同文件新函数），复用 ValidateDatasetID/PUT 路径/补偿叙事。
```

### Pattern 6: 批次审计行（表形态两选项）

**What:** DATA-17 + D15-A4 审计行的落表。
**When to use:** import_batches.go（推荐）或 audit 表新 action。

```sql
-- 选项 A（推荐）: 专用 INSERT-only 表——字段形状与 audit 表（24+ 列 chat 域）不重叠，
-- ADM-02 批次列表视图直接查它；INSERT-only 纪律用包注释 + 测试锁（镜像 audit.go:28-31）:
CREATE TABLE IF NOT EXISTS import_batches (
    batch_id                TEXT PRIMARY KEY,   -- crypto/rand hex
    dataset                 TEXT NOT NULL,
    period                  TEXT NOT NULL,
    tier                    TEXT NOT NULL,
    content_sha256          TEXT NOT NULL,
    rules_sha256            TEXT NOT NULL,      -- D15-A4
    raw_size_bytes          INTEGER NOT NULL,   -- DATA-17
    desensitized_row_count  INTEGER NOT NULL,   -- DATA-17
    discard_ts              TEXT NOT NULL,      -- DATA-17（raw 弃置时刻 = 流读完时刻）
    supersedes_batch_id     TEXT NOT NULL DEFAULT '',  -- D15-C1 链
    created_at              TEXT NOT NULL
);
-- 可选联动: audit 表写一行 action_data_imported（ARCHITECTURE.md §2 先例名）携带
-- batch_id 引用，让 /api/audit 时间线能看到导入事件——两表 INSERT-only 不冲突。

-- 选项 B: 复用 audit 表 action 枚举 + frozen_args BLOB 装结构化字段。
-- 代价: 批次字段无法 SQL 直查（ADM-02 视图要 JSON 解包）；audit 列继续膨胀。
```

### Pattern 7: 导入 API 端点与 Hertz 三选项

**What:** server.New 的装配改动 + handler 形态。
**When to use:** server.go + import_api.go。

```go
// Source: hertz v0.10.6 option.go 官方 doc comment（WithStreamBody 原文, :244-252）:
//   "Note: multipart form requests are still pre-parsed even when streaming is
//    enabled (the body is fully consumed into memory or a temp file before the
//    handler runs). To stream multipart bodies as well, also pass
//    WithDisablePreParseMultipartForm(true)."
//
// server.New 改动（现仅 server.Default(WithHostPorts(addr))，server.go:296）:
//   h := server.Default(
//       server.WithHostPorts(addr),
//       server.WithStreamBody(true),                      // 流式 body（8KB 预读上限）
//       server.WithDisablePreParseMultipartForm(true),    // 禁 multipart 预解析（raw 不落盘关键）
//       server.WithMaxRequestBodySize(<env 可配，见 Open Q1>),
//   )
// 兼容性已验证: 仓内无任何 FormFile/MultipartForm 调用（grep 实测）；
//   流模式下 c.Bind 路径经 BodyE() 排空流（request.go:697-700），既有 JSON handler 不受影响。
//
// handler 流式读 part（stdlib 原语，hertz 内部同款 request.go:271）:
//   mr := multipart.NewReader(c.Request.BodyStream(), c.Request.MultipartFormBoundary())
//   for { part, err := mr.NextPart(); ... io.Copy(解码管线, io.TeeReader(part, hasher)) }
//
// 分路由体量守卫（防止大上限被挪用到 /api/chat 做 DoS）:
//   全局 middleware 在 c.Next() 前检查声明 Content-Length——非导入路由超过 8MB 直接 413。
//   （流模式下 body 只预读 8KB（ext/common.go:61 maxContentLengthInStream=8KB），
//    middleware 先于 handler 排空，守卫有效。）
//
// 删除端点: 复用 isAdminRequest（audit.go:114-129）+ LRU.Detach → os.Remove(文件)
//   → 注册表行删除/标记 → INSERT-only 审计行（TestDeleteDatasetAudited）。
```

### Anti-Patterns to Avoid

- **`ctx.Request.MultipartForm()` / `c.FormFile()` 在导入路径使用** — 触发 Hertz 预解析路径：>16MB part 溢写 OS 临时文件（`DefaultMaxInMemoryFileSize=16MB`，consts/default.go:51）= raw 落盘，直接违反 D15-B1「磁盘上从无 raw」与 Pitfall 13。必须 `NextPart()` 流式。
- **读整个文件进内存再 `utf8.Valid(whole)`** — 月批 GB 级会打爆内存；用首块探测 + 流式严格校验（Pattern 1）。
- **supersede 用覆盖式 rename** — 注册表写失败时旧数据已灭（见 Pattern 5）。
- **每 (dataset, period) 一个盐** — 同客户跨期哈希不一致 → 月环比 join 断裂（Pitfall 4）。
- **逐行 `INSERT INTO ... VALUES`** — SC-1 明令禁止；STACK-VERIFICATION §2.8：千万行逐行 = 小时级。
- **dump 解析器「宽松跳过不认识的语句」** — 默认应是拒绝；只有显式样板句白名单可跳（Pitfall 2 证据）。
- **把规则文件放 `data/rules/`** — `data/` 整目录 gitignore（.gitignore:28），规则文件失去版本化 = D15-A1「受控文件」哲学破灭。

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| GBK/GB18030 解码 | 自写码表转换 | `x/text/encoding/simplifiedchinese.GB18030.NewDecoder()` | GB18030 码表数千映射；x/text 是 Go 官方扩展库、已在依赖树 |
| Unicode 规范化 | 自写 NFC | `x/text/unicode/norm.NFC/NFKC` | 规范化算法有版本化 Unicode 数据依赖，手写必漂移 |
| multipart 解析 | 自写 boundary 切分 | stdlib `mime/multipart.Reader` | RFC 2046 边界转义/续传边界极琐碎；stdlib 已被 hertz 自身复用 |
| CSV 解析 | 自写分隔符切分 | stdlib `encoding/csv.Reader` | 引号嵌套/跨行字段/CRLF 全有坑；STACK-VERIFICATION 已裁决 |
| HMAC | 自写 hash(盐‖值) 拼接 | stdlib `crypto/hmac` | 拼接式构造有 length-extension 与域混淆攻击面；HMAC 是标准构造 |
| 原子提交/替换 | 两阶段 copy + 标志文件 | `os.Rename` 同卷 + CommitStagedDataset/三步链 | OS rename 即原子边界（RESEARCH Don't Hand-Roll 先例，Phase 14 已落地） |
| 批量写入 | 逐行 INSERT / 自写批量事务 | go-duckdb Appender | 基准门 ≥10 万行/s 已锁；Appender 内部走 DuckDB 列存批量路径 |
| dataset ID 净化 | 自写过滤器 | `duckdb.ValidateDatasetID`（connector.go:112） | ATTACH 注入防线（T-14-12）已有先例实现 |
| admin 鉴权 | 新比较逻辑 | `isAdminRequest`（audit.go:114-129） | 常量时间比较 + unset 拒绝语义已定型 |

**Key insight:** 本 phase 的「新代码」几乎全部是**编排与策略**（规则文件 schema、规范化 profile、supersede 算法、文法白名单）；所有**机制类**问题（解码/解析/哈希/原子性/批量/鉴权）都有已验证的标准件。计划的任务切分应体现这条线——机制复用任务小而确定，策略任务大而需测试锁定。

---

## Common Pitfalls

### Pitfall 1: Hertz 默认 multipart 路径违反「raw 从不落盘」+ 4MB 默认上限
**What goes wrong:** 上传 50MB CSV → 默认 `MaxRequestBodySize=4MB`（option.go:40 实测）直接 413；即便调大，默认路径 Hertz 在 handler 前预解析 multipart，>16MB 的 part 溢写 OS 临时文件（consts/default.go:51）——raw 文件落在了 `/tmp`，Pitfall 13 原样复现，「磁盘上从无 raw」的可审计声明被推翻。
**Why it happens:** WithStreamBody 的官方文档明示「multipart 仍会被预解析」——两选项必须成对开启（option.go:244-252 doc 原文）。
**How to avoid:** `WithStreamBody(true)` + `WithDisablePreParseMultipartForm(true)` + `WithMaxRequestBodySize(提升)` 三件套；handler 用 `multipart.NewReader(BodyStream(), MultipartFormBoundary())`。仓内已 grep 验证无既有 multipart 消费者，全局开启安全。
**Warning signs:** 集成测试里上传 >16MB 文件后 `os.TempDir()` 出现残留；TestRawNeverOnDisk 若只扫 data/ 目录抓不到 /tmp 泄漏——测试要同时扫 staging/datasets/TempDir。

### Pitfall 2: 真实 sqlite .dump 必含样板句——朴素 D15-B3 解析器会拒掉每个合法 dump
**What goes wrong:** 本机 sqlite3 3.53.0 实测（2026-08-23）：`.dump` 输出**必然**含 `PRAGMA foreign_keys=OFF;` / `BEGIN TRANSACTION;` / `COMMIT;`；普通表 dump 常含 `CREATE INDEX`；AUTOINCREMENT 表追加 `PRAGMA writable_schema=ON/OFF` + `sqlite_sequence` 的 CREATE/DELETE/INSERT 块。若解析器按「非 CREATE TABLE/INSERT 即拒」实现，第一个真实 dump 就被拒。
**Why it happens:** D15-B3 的拒绝清单（DROP/UPDATE/PRAGMA/ATTACH/触发器）是语义意图；但 `PRAGMA foreign_keys=OFF` 是工具链样板而非攻击载荷。
**How to avoid:** 三层文法（Pitfall 表内已给全证据）：**跳过白名单** = `PRAGMA foreign_keys=OFF` / `PRAGMA writable_schema=ON|OFF` / `BEGIN TRANSACTION` / `COMMIT` / `CREATE INDEX ...`（无害元数据，建议跳过，见 Open Q5）/ `sqlite_sequence` 三件套（内部簿记）；**接受** = `CREATE TABLE` + `INSERT INTO <表> VALUES(...)`；**整批拒** = 其余一切。另两个实测转义事实：字符串内单引号 `''` 转义；BLOB 字面量 3.53.0 输出**小写** `x'6162'`（老版本 `X'..'` 大写——两者都收）；TEXT 含控制字符时 3.53.0 输出 `unistr('...
...')`（v1 可整批拒并提示导出端清理，或解码该形态——planner 定，见 Open Q5）。
**Warning signs:** TestDumpNonInsertStatementRejected 的用例集若只写 `DROP TABLE` 却没写「合法 dump 全量通过」的对照用例，就是没防住这个坑。

### Pitfall 3: supersede 覆盖式替换丢失旧数据
（详见 Pattern 5。）**Warning signs:** TestSupersedeAtomicSwap 注入的是「rename 后注册表失败」而非「rename 本身失败」时，覆盖式实现会漏测。

### Pitfall 4: 盐的作用域选错破坏跨期 join
**What goes wrong:** 盐按 (dataset, period) 或按批生成 → 同一客户 2026-06 与 2026-07 哈希不同 → Phase 16/17 月环比、趋势 sparkline 全部断裂；且 supersede 重导后哈希漂移。
**Why it happens:** 「数据集盐」直觉上随数据集实例走，而实例被理解为 (dataset, period) 文件。
**How to avoid:** 盐 per-dataset（逻辑数据集名，跨 period 跨批次恒定）——推荐确定性推导 `datasetSalt = HMAC(siteSalt, "dsalt:"+dataset)`（无存储、无迁移、无遗忘问题）。站点盐 env 未设 → 导入 fail-closed。**运维警示进文档：站点盐变更 = 全部既有哈希与新导入不可 join（需全量重导）**。
**Warning signs:** TestBucketBoundaryStableAcrossPeriods 的姊妹测试「TestHashStableAcrossPeriods」（CONTEXT 测试锁未列，建议补）。

### Pitfall 5: 全角字符与 NFC 的隐性交互（§3.2 定稿核心证据）
**What goes wrong:** 金融系统导出的手机号常是全角 `１３８...` 或 `＋８６...`。NFC 不折叠全角（本机实测）→ 朴素 NFC 链下全角/半角同号不同哈希；反过来全局用 NFKC 又过猛（`①→1`、`(一)→(1)` 类兼容折叠会合并本不同的值）。
**How to avoid:** 分 profile：person_name 用 NFC+TrimSpace；phone_cn/id_card_cn 用 NFKC（数字字母小字母表上 NFKC 安全）+ 字段特定归一（去分隔符、+86、大写）。TrimSpace 已覆盖全角空格 U+3000（实测），无需自写 rune 剥离。

### Pitfall 6: 规则文件目录与 gitignore 冲突
**What goes wrong:** CONTEXT 提议 `data/rules/{dataset}.yaml`，但 `.gitignore:28` 忽略整个 `data/`（staging.go:32-33 注释明示「runtime state, never committed」）——规则文件静默失去版本化，D15-A4 的 rules_sha256 将指向无版本历史的文件，「哪版规则」不可回放。
**How to avoid:** 规则文件放版本化路径（推荐仓库根 `datarules/`；或 `data/rules/` + gitignore 例外 `!data/rules/`，但混运行态目录不推荐）。planner 定（Open Q2）。

### Pitfall 7: Appender 不支持类型 + staging 表类型策略真空
**What goes wrong:** staging 表 CREATE TABLE 选了 BIT/ANY 等类型 → `NewAppender` 直接 errAppenderCreation（type.go:52-58 实测清单）。更深一层：CSV 一切皆字符串，若 staging 表全 VARCHAR，Phase 16 KPI 的 SUM/AVG 要靠 CAST——类型推断策略是绕不开的决策。
**How to avoid:** 明确类型策略（Open Q4）：推荐「规则文件可选 `duckdb_type` 声明 + 缺省按首 100 行（预览数据）推断 INTEGER/DOUBLE/VARCHAR」，推断只发生在预览阶段且结果存 schema_json（提交阶段不再猜，保证幂等重导同型）。

### Pitfall 8: 预览响应/原文对照进日志（D15-B4 约束）
**What goes wrong:** 任何 `log.Printf("%v", payload)` 或未来加的 access-log middleware 落请求体 → 原文对照进日志文件 = raw 半持久化。
**How to avoid:** handler 构造响应后只记录 batch 级摘要（行数/哈希/列名），永不打行值；仓内现状无 body-logging middleware（server.go 路由面实测），新增 middleware 时须排除 /api/import/*。可复刻 D8-04 WalkDir 禁令门形态做「import_api.go 内禁 log 行值」结构性断言（grep 门）。

### Pitfall 9: Windows 文件锁——删除/替换前忘 DETACH
Phase 14 已立（staging.go:104 注释 + §3.5）：数据集被 ATTACH 时 os.Remove/os.Rename 在 Windows 直接失败。supersede 与删除端点的**第一步**都是 `AttachLRU.Detach(ctx, id)`。TestDeleteDatasetAudited 应含「先查询过数据集（触发懒 ATTACH）再删除」的用例。

### Pitfall 10: GB18030 兜底吞掉 UTF-8 检测顺序错误
GBK 字节空间几乎完备——UTF-8 字节流喂 GB18030 解码器「成功」产出乱码而非报错。探测顺序必须 UTF-8 优先（§3.1 铁律已写）。**Warning signs:** 测试集里有「UTF-8 中文 CSV 走 GB18030 路径产出乱码但导入成功」用例即为红旗。

---

## Code Examples

### GB18030/UTF-8 流式解码骨架（Pattern 1 展开）

```go
// Source: x/text v0.28.0 模块缓存 API（encoding.go:56-78 Decoder.Bytes/String/Reader;
//         simplifiedchinese/gbk.go:18 GB18030）——签名实测
import (
    "golang.org/x/text/encoding/simplifiedchinese"
    "golang.org/x/text/unicode/norm"
)

func decodeStream(r io.Reader) (io.Reader, error) {
    br := bufio.NewReader(r)
    // ① BOM
    if head, _ := br.Peek(3); len(head) == 3 && bytes.Equal(head, []byte{0xEF,0xBB,0xBF}) {
        if _, err := br.Discard(3); err != nil { return nil, err }
    }
    // ② 首块 UTF-8 探测（64KB）
    sniff, _ := br.Peek(64 * 1024) // Peek 不消费；短文件返回更短切片
    if len(sniff) > 0 && utf8.Valid(sniff) {
        return br, nil // UTF-8 路径（可再包严格校验 transformer，见 Pattern 1）
    }
    // ③ GB18030 流式解码（非法序列返回 error → fail-closed）
    return simplifiedchinese.GB18030.NewDecoder().Reader(br), nil
}
```

### .dump ground truth fixtures（本机 sqlite3 3.53.0 实测输出，直接进测试）

```sql
-- 基础表（引号转义 + 小写 blob + 中文 + NULL）:
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE t(a INTEGER PRIMARY KEY, name TEXT, bal REAL, data BLOB, note TEXT);
INSERT INTO t VALUES(1,'张三''quote',1234.56,x'616263',NULL);
CREATE INDEX idx_t_name ON t(name);
COMMIT;
-- AUTOINCREMENT 表（额外样板块）:
PRAGMA writable_schema=ON;
CREATE TABLE IF NOT EXISTS sqlite_sequence(name,seq);
DELETE FROM sqlite_sequence;
INSERT INTO sqlite_sequence VALUES('u',2);
PRAGMA writable_schema=OFF;
-- TEXT 含换行（3.53.0 输出 unistr 形态）:
INSERT INTO t VALUES(2,unistr('line1
line2'),0.5,NULL,'it''s');
```

### 规则文件 schema 提案（D15-A1/A2/D1/D4 全决策映射）

```yaml
# datarules/loans.yaml —— 数据人员可写；loader 加载期 fail-closed 校验
dataset: loans          # 必须与注册名 __ 前缀一致
tier: 历史               # 权威|历史|个人（TierEnum 词表）
k_min: 10               # 可选 override（缺省用全局 5）
columns:
  customer_name: { rule: hash, profile: person_name }
  mobile:        { rule: hash, profile: phone_cn }
  cert_no:       { rule: hash, profile: id_card_cn }
  balance:       { rule: bucket_fixed, width: 1000 }   # D15-D1 默认形态
  overdue_days:  { rule: bucket_fixed, width: 30 }
  update_ts:     { rule: passthrough }                  # D15-A2 显式白名单
  status:        { rule: passthrough }
# 校验规则（loader）:
#   - CSV 列 ∈ columns 且 rule ∈ {hash, passthrough, bucket_fixed} 且 profile/bucket 参数齐
#   - hash 列的 profile 必须是 Go 规则库已注册名（未知 profile → 拒，防「规则名幻觉」）
#   - CSV 列 ∉ columns → 结构化拒绝并列出列名（TestUnknownColumnFailClosed）
#   - columns 有而 CSV 无 → 亦拒（schema 漂移双向检查，Pitfall 21 schema diff 报告）
# rules_sha256 = hex(sha256(该 YAML 文件字节)) —— D15-A4
```

### 幂等/审计/删除的 sqlite 侧查询

```sql
-- 幂等检查（batch.go，? 参数化——T-3-SQLI 纪律）:
SELECT batch_id FROM import_batches
WHERE dataset = ? AND period = ? ORDER BY created_at DESC LIMIT 1;
-- 同 content_sha256 → 返回既有 batch_id，无副作用（DATA-16）
-- 不同 content_sha256 → supersede 流程（D15-C1），supersedes_batch_id = 上面这行
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 查询时脱敏（masking policy at query time，Snowflake/BigQuery 模式） | **导入时脱敏**（raw 物理不持有） | GDPR Art.25 privacy-by-design 逐年被监管实例化 | SkillForge 的差异化立场（FEATURES Cap 1：desensitization irreversible **because raw was discarded**）；Phase 15 是该立场的物理落地 |
| TTL/GC 清扫 raw 文件 | 双传 + 哈希桥接（raw 单请求生命周期） | 本 phase（D15-B1） | GC 定时器不是安全边界（Pitfall 13）——结构上从无 raw 比清扫可审计性强 |
| 密码学上「加盐哈希存注册表」 | 确定性盐推导（HMAC 链式派生） | — | 少一列少一迁移；等价安全（本地 sqlite 本可读） |

**Deprecated/outdated（本 phase 相关）:**
- ARCHITECTURE.md Q1 原稿的 `dataset_row` sqlite 表 + 「NO new connection pool」设计——**已被 Phase 14 的每数据集 .duckdb + DuckDB Connector 模式取代**，以 Phase 14 实际交付代码为准（本文件全部 Pattern 均基于实际代码）。ARCHITECTURE.md 是 2026-08-21 研究稿，Phase 14（08-22/23）晚于它。
- STACK-VERIFICATION 记录的 x/text v0.22.0——go.mod 现为 **v0.28.0**（API 面不变，已实测）。

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DuckDB 接受 `ds_loans__2026-07` 形态的 ATTACH 别名（Go 侧 ValidateDatasetID 语法已验证；DuckDB catalog 标识符侧未实测） | Pattern 4 | 低——标识符仅字母数字下划线连字符；Wave 0 一条 3 行探针测试即可锁 |
| A2 | 站点盐用单 env var `SKILLFORGE_DATA_SALT`（名字与 fail-closed 语义是提案，planner 可改名） | Pattern 2 | 低——命名面决策；语义（未设拒绝导入）建议保留 |
| A3 | 老 sqlite3 版本 dump 输出大写 `X'..'` blob 且 TEXT 换行直接内嵌（非 unistr）——仅 3.53.0 实测，历史版本行为来自格式沿革知识 | Pitfall 2 | 低——解析器 x/X 双收 + 换行内嵌本就必须支持（csv 行界无关的 SQL 语句级扫描）；fixture 用 3.53.0 实测输出锁定 |
| A4 | 复合名 `loans__2026-07` 的 `__` 分隔符无歧义（dataset 名本身允许含 `_`，理论上 `loans_` + `_2026-07` 可碰撞） | Pattern 4 | 极低——加载期校验 dataset 名禁连续下划线即可消除；或换 `--` 分隔（同在白名单） |

**其余全部 claim 已 VERIFIED（module cache/本机实测/repo grep）或 CITED（既有研究文档）。**

## Open Questions (RESOLVED)

> 2026-08-23 修订（checker research_resolution）：全部六问已由 15-01…15-07 plans 采纳定稿，非未决；逐问决议指针如下。

1. **全局上传上限的具体数值与配置面**
   - What we know: 默认 4MB 必须提高；流模式下预读仅 8KB，内存安全；容量假设月批 100 万-5000 万行（STACK-VERIFICATION §2.8）
   - What's unclear: 上限定多少（512MB？2GB？）、是否 env 可配（`IMPORT_MAX_BODY_BYTES`?）、分路由 Content-Length 守卫的非导入路由阈值
   - Recommendation: env 可配 + 合理默认（如 2GB 导入路由 / 8MB 其余）；planner 定数值，handler 内再加逻辑上限（行数与 raw_size_bytes 双限）
   - **→ RESOLVED**：15-06 Task 1（SKILLFORGE_IMPORT_MAX_BODY_BYTES 默认 2147483648 / SKILLFORGE_MAX_BODY_BYTES 默认 8388608 + 非导入路由 Content-Length 守卫 413）+ 15-05 Task 1（ImportMaxRowsPerBatch = 50_000_000 行数限——字节/行数双限落地）
2. **规则文件目录名**
   - What we know: `data/` 全目录 gitignore；D15-A1 要版本化受控文件
   - Recommendation: 仓库根 `datarules/`（CONTEXT 用了「如」，属可调范围）；若用户坚持 data/rules 需 gitignore 例外
   - **→ RESOLVED**：仓库根 `datarules/`（15-01 Task 2 落 loans.yaml + README；15-07 Stage 09 `git ls-files datarules/` 非空锁——版本化面有门）
3. **批次审计表形态（选项 A 专用表 vs 选项 B audit 复用）**
   - Recommendation: A（专用 `import_batches` + 可选 audit 联动行）；两者都满足 INSERT-only，A 对 ADM-02 视图友好
   - **→ RESOLVED**：选项 A——专用 `import_batches` 表（15-02 交付 ImportBatchRow / PutImportBatch / LatestImportBatch + 锁名 TestBatchAuditRowShape；15-05 writeAuditRow 消费）
4. **staging 表列类型策略**
   - Recommendation: 规则文件可选 `duckdb_type` + 预览期首 100 行推断（INTEGER/DOUBLE/VARCHAR 三档），推断结果冻结进 schema_json 保证幂等重导同型（Pitfall 7）
   - **→ RESOLVED**：15-01 Task 2 ColumnRule.DuckDBType 可选字段 + 15-05 Task 1 预览期首 100 行推断（BIGINT/DOUBLE/VARCHAR 三档——DuckDB 整型列名定 BIGINT）+ 结果冻结进 PreviewSession 与 schema_json（幂等重导同型）
5. **dump 容错边界两处定夺**: `CREATE INDEX` 跳过 vs 拒（推荐跳过——实测每个普通表 dump 都含）；`unistr()` 字面量解码 vs 整批拒（v1 推荐拒并结构化提示「导出端含控制字符」，解码留 v1.1）
   - **→ RESOLVED**：CREATE INDEX 跳过（跳过白名单）+ unistr() v1 整批拒——15-04 Task 1 三层文法（TestDumpControlCaseRealDumpPasses / TestDumpUnistrRejected 锁定）
6. **导入端点是否也挂 X-Admin-Token**
   - What we know: 删除端点 D15-C3 明确挂；preview/commit/push 的鉴权面 CONTEXT 未锁（「数据人员」角色 v1.0 无用户体系）
   - Recommendation: 三端点同挂 admin token（镜像 /eval 姿态，v1.0 单 token 已是 Out of Scope 表立场）；planner/user 确认
   - **→ RESOLVED**：preview/commit/push 三端点同挂 isAdminRequest——15-06 Task 2（「Open Q6 采纳」注记 + TestImportEndpointsAdminGated 锁定）

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| MSYS2 gcc (ucrt64) | CGO 编译（go-duckdb/mattn） | ✓ | 16.1.0（`/c/msys64/ucrt64/bin/gcc` 实测） | — |
| Go | 全部 | ✓ | 1.25.0 windows/amd64 | — |
| x/text | GB18030/NFC 解码 | ✓ | v0.28.0（go.mod） | — |
| yaml.v3 | 规则文件 | ✓ | v3.0.1（go.mod） | — |
| sqlite3 CLI | 生成 .dump ground-truth fixture | ✓ | 3.53.0（`/d/Program Files/Sqlite3/`） | fixture 直接入库后不再需要 |
| Docker | 无（本 phase 全 hermetic，无容器构建需求） | ✗（Phase 14 已记录） | — | 不需要 |
| Milvus / BGE / GLM 内网服务 | **无依赖**（导入管线不触 LLM/向量层） | 未探测（无需） | — | — |

**Missing dependencies with no fallback:** 无。
**Missing dependencies with fallback:** 无。

> 环境注记：本 phase 研究与实现均不依赖内网间歇服务（Milvus/BGE/GLM）——全 hermetic 路径，不受 `env-intranet-service-flakiness` 记忆约束影响。

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | go `testing` + stretchr/testify v1.11.1（仓内既有） |
| Config file | 无独立配置（Go 惯例；CGO 门 = Makefile `check-cgo`） |
| Quick run command | `go test ./internal/data/... ./internal/duckdb/... ./internal/store/sqlite/... -count=1` |
| Full suite command | `go test ./... -count=1`（Makefile `test-unit`） |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DATA-01 | CSV 四编码解码正确（UTF-8/BOM/GBK/GB18030 fixture 往返） | unit | `go test ./internal/data/ -run TestDecode -count=1` | ❌ Wave 0 |
| DATA-01 | dump 接受白名单语句 + 拒 DROP/UPDATE/ATTACH | unit | `go test ./internal/data/ -run TestDump -count=1` | ❌ Wave 0 |
| DATA-01 | multipart 流式（>16MB part 不触 TempDir） | integration | `go test ./internal/server/ -run TestImportStreaming -count=1` | ❌ Wave 0 |
| DATA-02 | 规范化 join 恒等（张三/尾空格/+86/全角） | unit | `go test ./internal/data/ -run TestHMACNormalizationJoinability -count=1` | ❌ Wave 0 |
| DATA-02 | 未知列 fail-closed | unit | `go test ./internal/data/ -run TestUnknownColumnFailClosed -count=1` | ❌ Wave 0 |
| DATA-02 | 全流程后磁盘无 raw | integration | `go test ./internal/data/ -run TestRawNeverOnDisk -count=1` | ❌ Wave 0 |
| DATA-15 | 哈希桥接（预览 A + 提交 B → 拒） | unit | `go test ./internal/data/ -run TestPreviewCommitHashBridge -count=1` | ❌ Wave 0 |
| DATA-16 | 同 (dataset,period) 同哈希 → 无副作用 | integration | `go test ./internal/data/ -run TestIdempotentReimport -count=1` | ❌ Wave 0 |
| DATA-16 | supersede 注入失败旧数据完好 + 成功链可查 | integration | `go test ./internal/duckdb/ -run TestSupersedeAtomicSwap -count=1` | ❌ Wave 0 |
| DATA-17 | 批次审计行字段形状 | integration | `go test ./internal/store/sqlite/ -run TestBatchAuditRowShape -count=1` | ❌ Wave 0 |
| SC-1 | Appender 路径（无逐行 INSERT） | 既有门 + 新 grep 门 | `go test ./internal/duckdb/ -run TestDuckDBAppenderThroughput -count=1`（已有 ✓）+ WalkDir 式禁令 | ✓ 基准门 / ❌ 禁令门 Wave 0 |
| D15-D1 | 桶边界跨期稳定 | unit | `go test ./internal/data/ -run TestBucketBoundaryStableAcrossPeriods -count=1` | ❌ Wave 0 |
| D15-C3 | 删除端点审计 + 注册表/文件一致 | integration | `go test ./internal/server/ -run TestDeleteDatasetAudited -count=1` | ❌ Wave 0 |

（测试名与 CONTEXT §Specific test lock requirements 一一对应；`TestRawNeverOnDisk` 需扩展扫描 `os.TempDir()`——见 Pitfall 1 警示。）

### Sampling Rate
- **Per task commit:** `go test ./internal/data/... ./internal/duckdb/... ./internal/store/sqlite/... -count=1`（<30s 目标；新包测试保持 hermetic）
- **Per wave merge:** `go test ./... -count=1` + `cd web && npx tsc --noEmit`（虽无 FE 改动，守既有 0 错误基线）
- **Phase gate:** `scripts/verify-phase-15.sh`（run_stage 形态复刻 verify-phase-14.sh：build+vet → race 段 → 全量测试 → 冻结字节段 → Appender 基准段 → WalkDir/grep 禁令段 → Makefile target）

### Wave 0 Gaps
- [ ] `internal/data/` 包尚不存在——首个 plan 建包骨架 + normalize.go（纯函数最先可测）
- [ ] `.dump` ground-truth fixtures（`internal/data/testdata/*.sql`）——用本文件 Code Examples 的 3.53.0 实测输出直接落盘；再生成命令：`sqlite3 t.db "CREATE...; INSERT..."; sqlite3 t.db .dump`
- [ ] 四编码 CSV fixtures（testdata，含 GBK/GB18030/BOM 各一）
- [ ] grep/WalkDir 禁令门：导入代码禁逐行 INSERT、禁 MultipartForm/FormFile、import_api 禁行值日志（复刻 D8-04 形态）
- [ ] `scripts/verify-phase-15.sh` + Makefile target

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no（v1.0 无用户体系；admin token 即姿态） | — |
| V3 Session Management | no | — |
| V4 Access Control | yes | `isAdminRequest` 常量时间比较（crypto/subtle，audit.go:114-129 复用）；未设 env = 全拒 |
| V5 Input Validation | yes | 三层：dataset ID 白名单（ValidateDatasetID 复用）；period 格式加载期校验（YYYY-MM）；列集合 fail-closed 比对（D15-A2）+ dump accept-list 文法（D15-B3） |
| V6 Cryptography | yes | stdlib crypto/hmac + sha256；盐经 env 注入不进仓库；**never hand-roll**（Don't Hand-Roll 表） |
| V12 File Upload | yes | 流式不落临时文件（三选项）；体量双限（Content-Length 守卫 + 行数/raw_size 逻辑限）；解码 fail-closed |

### Known Threat Patterns for 数据导入管线（Go + Hertz + DuckDB）

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| ATTACH 别名/路径注入（T-14-12 延续） | Tampering | `ValidateDatasetID` 白名单 + filepath.Clean（connector.go:98-135 既有）；导入侧所有用户输入的 dataset 名过同一函数 |
| dump 内嵌恶意 SQL（DROP/ATTACH/触发器） | Tampering/Elevation | 永不执行（D15-B3）——解析器只认 CREATE TABLE/INSERT VALUES 形状，其余整批拒；TestDumpNonInsertStatementRejected 锁 |
| multipart 预解析把 raw 写 OS 临时文件 | Information Disclosure | WithDisablePreParseMultipartForm + NextPart 流式（Pitfall 1）；TestRawNeverOnDisk 扫 staging/datasets/TempDir |
| 超大上传 DoS（大上限波及 /api/chat） | DoS | 流模式 8KB 预读 + 分路由 Content-Length middleware + 导入逻辑双限 |
| 预览原文对照进日志/落盘 | Information Disclosure | D15-B4：响应不落盘、行值不进日志；grep 禁令门（Pitfall 8） |
| 规则文件提权（passthrough 滥声明敏感列） | Elevation | 规则文件 = 受控文件走 PR 评审（D15-A1 哲学）；rules_sha256 进审计行使滥声明可追责；无运行时自动放行路径 |
| 哈希字典攻击（常见姓名穷举） | Information Disclosure | 站点盐+数据集盐（彩虹表不可预制）；确定性 join 的固有残余风险——文档明示（PIPL/GDPR 伪匿名化定位），非 v1 修复项 |
| 删除端点未授权调用 | Tampering | X-Admin-Token 常量时间门 + INSERT-only 删除审计行（D15-C3） |

---

## Sources

### Primary (HIGH confidence — 本 session 实测/模块源码)
- go module cache `github.com/marcboeker/go-duckdb/v2@v2.4.3` — appender.go（NewAppender 签名/auto-flush）、type.go:52-58（不支持类型清单）
- go module cache `github.com/cloudwego/hertz@v0.10.6` — common/config/option.go:40（4MB 默认）、app/server/option.go:147-154/244-256（DisablePreParseMultipartForm / WithStreamBody 官方 doc）、protocol/consts/default.go:49-51（16MB 内存阈值）、protocol/request.go（BodyStream/MultipartFormBoundary/BodyE 排空）、protocol/http1/ext/common.go:61（8KB 预读）、protocol/http1/req/request.go:255-310（ReadBodyStream 尺寸执法）
- go module cache `golang.org/x/text@v0.28.0` — encoding/simplifiedchinese/gbk.go:18（GB18030）、encoding/encoding.go:56-78（Decoder API）、unicode/norm/normalize.go:38（NFC Form）
- 本机实测 Go 程序（2026-08-23）— TrimSpace 剥 U+3000 ✓ / NFC 不折叠全角 ✓ / NFKC 折叠全角 ✓
- 本机 sqlite3 3.53.0 CLI 实测（2026-08-23）— 两份 `.dump` ground truth（样板句/转义/小写 blob/unistr/sqlite_sequence）
- repo grep/精读 — internal/duckdb 三件套、schema.go:206-214 datasets DDL、datasets.go putDatasetSQL、server.go:295-341 路由面（无 multipart 消费者）、audit.go isAdminRequest（14-PATTERNS 引证）、.gitignore:28、Makefile targets、go.mod

### Secondary (MEDIUM confidence — 既有研究文档引证)
- `.planning/research/STACK-VERIFICATION.md` §2.1/§2.5/§2.7/§2.8/§3.1/§3.2/§3.4/§3.5
- `.planning/research/PITFALLS.md` Pitfall 11/13/21
- `.planning/research/FEATURES.md` Capability 1（table stakes/differentiators/anti-features）
- `.planning/research/ARCHITECTURE.md` Q1/Q7（Q1 已按 Phase 14 实际代码修正引用）
- `.planning/phases/14-p10-0-cgo-trust-floor/14-CONTEXT.md`（D14-D1..D4）+ `14-PATTERNS.md`（analog 全表）

### Tertiary (LOW confidence)
- 无（无 WebSearch-only 结论进入本文件）

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 全部既有依赖，版本/签名实测
- Architecture: HIGH — Q1/Q7 由锁定决策+实际代码双重锚定；ARCHITECTURE.md 原稿与 Phase 14 的矛盾已识别并裁决
- Pitfalls: HIGH — Pitfall 1/2/5 均有本机实测数字/输出背书，非推测
- HMAC 规范化定稿: HIGH — 三条关键行为（U+3000/NFC/NFKC）全部实证

**Research date:** 2026-08-23
**Valid until:** 2026-09-22（依赖面冻结；hertz/duckdb 版本未动则结论持续成立）
