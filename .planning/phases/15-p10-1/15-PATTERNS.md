# Phase 15: 数据导入管线 + 导入即脱敏（P10.1）- Pattern Map

**Mapped:** 2026-08-23
**Files analyzed:** 22（新建 14 + 修改 8）
**Analogs found:** 17 / 22（4 项机制类新代码无仓内 analog，见「No Analog Found」——全部已由 15-RESEARCH §Architecture Patterns 给出实测骨架）

> 本文件是 planner 的直接输入：每个 plan 的 action 应引用下述 analog 文件与行号的代码形态。
> 所有行号基于 2026-08-23 的 master 工作区实测精读（非二手引证）。
> Phase 14 资产（duckdb 三件套 / datasets 注册表 / isAdminRequest）全部在本 session 重新核对，与 14-PATTERNS.md 一致。

**对上游线索的两处修正（已核实）：**
1. `isAdminRequest` 在 **`internal/server/audit.go:114-129`**（不是 `internal/store/sqlite/audit.go`——那是 store 层 INSERT 实现）。
2. `ValidateDatasetID` 实际在 `internal/duckdb/connector.go:109-117`（regexp 定义 :104、`errInvalidDatasetID` :106、`AttachDataset`+filepath.Clean :124-135）。

---

## File Classification

| 新建/修改文件 | Role | Data Flow | 最近 Analog | 匹配度 |
|---------------|------|-----------|-------------|--------|
| `internal/data/rules.go` [新] | service（loader+validator） | file-I/O + transform | `internal/skill/validate.go` + `internal/skill/loader.go` | exact |
| `internal/data/normalize.go` [新] | utility（常量+纯函数） | transform | `internal/runtime/fanout_runtime.go` 55-130（常量表 + Resolve 兜底） | role-match |
| `internal/data/desensitize.go` [新] | utility（纯函数引擎） | transform | 无仓内 analog（crypto stdlib；RESEARCH Pattern 2） | none |
| `internal/data/decode.go` [新] | utility（流式解码） | streaming | 无仓内 analog（RESEARCH Pattern 1 骨架 + Code Examples 全文） | none |
| `internal/data/dump.go` [新] | service（accept-list 解析器） | transform | 无仓内 analog（RESEARCH Pitfall 2 三层文法 + 3.53.0 fixtures） | none |
| `internal/data/importer.go` [新] | service（编排） | batch / streaming | `internal/duckdb/staging.go` 105-179（步骤编排+补偿叙事） | role-match |
| `internal/data/preview.go` [新] | store（进程内会话） | request-response | `internal/runtime/fanout_runtime.go` 170-260（SharedChunkAccumulator） | exact |
| `internal/data/batch.go` [新] | service（幂等+审计编排） | batch | `internal/duckdb/staging.go` + `internal/store/sqlite/datasets.go` | exact（组合） |
| `internal/duckdb/staging.go` [改：新增 SupersedeStagedDataset] | service（file-I/O） | file-I/O | 自身 105-179（CommitStagedDataset——supersede 的同构基座） | 改造点 |
| `internal/duckdb/importer_write.go`（可选，或并入 importer.go）[新] | service（批量写） | batch | `internal/duckdb/appender_throughput_test.go` 56-84（仓内唯一 Appender 用法形态） | exact（用法） |
| `internal/store/sqlite/schema.go` [改：datasets ALTER 扩列 + import_batches 建表] | migration（DDL） | DDL 幂等迁移 | 自身 231-262（ALTER-gated）+ 176-186（plan_todos 新表先例） | exact |
| `internal/store/sqlite/datasets.go` [改：dataset+period 查询] | model/store | CRUD | 自身 31-97 | 改造点 |
| `internal/store/sqlite/import_batches.go` [新] | model/store | CRUD（INSERT-only） | `internal/store/sqlite/datasets.go`（put/list 全套形态） | exact |
| `internal/store/store.go` [改：ImportBatchRow + 接口扩展] | model（行结构+接口） | — | 自身 DatasetRow 266-284 + 接口 533-540 | 改造点 |
| `internal/server/server.go` [改：三选项 + 路由 +4] | route | request-response | 自身 295-348（`server.Default` :296 + 路由注册表） | 改造点 |
| `internal/server/import_api.go` [新] | controller（HTTP handler） | request-response（multipart 流式） | `internal/server/platform_prompt.go` 52-77 + `internal/server/audit.go` 191-276 | exact |
| `datarules/{dataset}.yaml` [新] | config（声明式规则） | — | `skills/<name>/SKILL.md` frontmatter YAML（loader.go 240-265 解析纪律） | role-match |
| `integration/contract/import_gates_test.go` [新] | test（结构性禁令门） | batch | `integration/contract/platform_floor_bypass_test.go` 36-126 | exact |
| `internal/data/testdata/*.sql` + 四编码 CSV [新] | test fixture | — | RESEARCH §Code Examples `.dump` ground truth（sqlite3 3.53.0 实测输出） | partial |
| `scripts/verify-phase-15.sh` [新] | gate 脚本 | batch | `scripts/verify-phase-14.sh` 55-74（run_stage）+ 191-198（source-grep 段） | exact |
| `Makefile` [改：verify-phase-15 target] | config（构建门） | — | 自身 45-53（verify-phase-14 target 形态） | 改造点 |
| `cmd/server/main.go` [改：导入 deps 装配 + env] | config（装配） | — | 自身 118-143（envOr 群）+ 354（Deps 构造）+ 463-464 | 改造点 |

**测试文件**（随实现落地，归对应 plan；名字与 CONTEXT §Specific test lock requirements 一一对应）：
- `internal/data/`：TestDecode* / TestDump* / TestHMACNormalizationJoinability / TestUnknownColumnFailClosed / TestRawNeverOnDisk / TestPreviewCommitHashBridge / TestIdempotentReimport / TestBucketBoundaryStableAcrossPeriods（+ 建议补 TestHashStableAcrossPeriods，RESEARCH Pitfall 4）
- `internal/duckdb/`：TestSupersedeAtomicSwap（注入点必须是「rename 后注册表失败」与「rename 本身失败」双面——RESEARCH Pitfall 3）
- `internal/store/sqlite/`：TestBatchAuditRowShape
- `internal/server/`：TestImportStreaming（>16MB part 不触 TempDir）/ TestDeleteDatasetAudited（复刻 audit_test.go TestAdminToken_* 四用例骨架）

---

## Pattern Assignments

### 1. `internal/data/rules.go`（service：loader + fail-closed validator）[新]

**Analog:** `internal/skill/validate.go`（校验器）+ `internal/skill/loader.go`（文件加载）——D15-A1「加载期校验镜像 Skill 7-field validator」的直接模板。

**Fail-closed 校验形态**（validate.go 50-91——错误信息命名字段+规则、首个失败短路、default-deny）：

```go
func Validate(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil (SYSK-02: 7-field SKILL.md required)")
	}
	// 1. description — non-empty intent-routing text (SKILL_SCHEMA field 1)
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("description is required and must be non-empty (SKILL_SCHEMA field 1)")
	}
	...
	// 3. reads — non-empty (default-deny), no "*" wildcard (P-SKILL-3)
	if len(m.Reads) == 0 {
		return fmt.Errorf("reads is required and must be non-empty (default-deny: P-SKILL-3 / SYSK-07)")
	}
```

rules.go 照此写 `ValidateRules`：列规则 ∈ {hash, passthrough, bucket_fixed}、hash 列 profile 必须是 Go 规则库已注册名（防「规则名幻觉」）、bucket_fixed 必须 width>0、tier ∈ TierEnum 词表、k_min ≥ 0。**注意与 Skill 校验的一个刻意差异**：CONTEXT 要求未知列「结构化拒绝并列出列名」——CSV 列集合比对发生在导入时（rules.go 提供列集合查询，importer.go 比对），文件自身校验仍是首错短路。

**已知目标词表校验先例**（validate.go 66-71 + 135-144——枚举白名单 + 「不要在这里加，去唯一真源加」的注释纪律）：

```go
	if !validAgentTypes[m.Agent] {
		return fmt.Errorf("agent type %q is not supported (SKILL_SCHEMA field 2; valid: chat, deep, ...)", m.Agent)
	}
	...
	known := hitl.KnownTargets()
	for _, w := range m.Writes {
		if !containsString(known, w) {
			return fmt.Errorf("writes contains unknown target %q (known targets: %v); "+
				"the target must appear in hitl.AllowedWriteTargets — "+
				"do NOT add it here, update internal/agent/hitl/registry.go instead (D11-B3 shared source)",
				w, known)
		}
	}
```

rules.go 的 profile 名校验照此把唯一真源放 normalize.go（`KnownProfiles()`），错误信息指向它。

**文件加载 + 解析纪律**（loader.go 202-215——ReadFile → yaml.Unmarshal → Validate 的三段式 + 错误带路径）：

```go
	raw, err := os.ReadFile(path)
	if err != nil { return nil, nil, fmt.Errorf("read %s: %w", path, err) }
	...
	var m Manifest
	if err := yaml.Unmarshal(front, &m); err != nil {
		return nil, nil, fmt.Errorf("unmarshal YAML in %s: %w", path, err)
	}
	if err := Validate(&m); err != nil {
		return nil, nil, fmt.Errorf("validate %s: %w", path, err)
	}
```

规则文件是整文件 YAML（无 frontmatter），比 loader 简单：`os.ReadFile` → 记录字节（rules_sha256 = sha256 of raw bytes）→ `yaml.Unmarshal(raw, &rules)` → `ValidateRules`。**V12 symlink guard（loader.go 180-200）建议复刻**——规则文件同样是「受控文件被原子替换」面。

**目录事实（Pitfall 6 已核实）**：`.gitignore:27-28` 确实整目录忽略 `data/`（注释原文 "Data-support runtime state (D14-D4)"）。规则文件必须放版本化路径（推荐仓库根 `datarules/`），与 staging.go:32-33 注释「runtime state, never committed」的叙事一致。

---

### 2. `internal/data/normalize.go`（utility：规范化 profile 表）[新]

**Analog:** `internal/runtime/fanout_runtime.go` 55-130——常量表 + 带 godoc 的 source-grep 锁 + Resolve 兜底三件套。

**常量即文档 + grep 锁定目标**（fanout_runtime.go 63 与 88-89）：

```go
const SubAgentMaxIteration = 5       // "the literal is the call-site source-grep target ... NOT a config knob."
...
// Source-grep locked via TestSharedChunkAccumulatorLRUCap.
const SharedChunkAccumulatorMaxEntries = 20
```

normalize.go 的三档 profile（RESEARCH Pattern 2 实测定稿）照此形态：

```go
// person_name = NFC → TrimSpace（NFKC 过猛：①→1 兼容折叠会并值）
// phone_cn    = NFKC → 去分隔符 → 去 +86 前缀 → 校验 11 位
// id_card_cn  = NFKC → ToUpper → 去空白（全角数字 + x→X 都归一）
// 默认 hash 列 = NFC → TrimSpace
const ProfilePersonName = "person_name"   // 等字符串常量 + KnownProfiles() 唯一真源
```

**Resolve 兜底形态**（fanout_runtime.go 119-130——零值回退包常量）可镜像为 profile 缺省解析。x/text API 已在模块缓存实测（`unicode/norm.NFC/NFKC`、`strings.TrimSpace` 覆盖 U+3000 全角空格——RESEARCH 实测证据）。

---

### 3. `internal/data/desensitize.go` + 4. `internal/data/decode.go` + 5. `internal/data/dump.go` [新]

**无仓内 analog**（grep 实测：仓内生产代码零 `GB18030/simplifiedchinese/unicode/norm` 引用、零 SQL 解析器、零 crypto 构造）。planner 直接采用 15-RESEARCH 的实测骨架，不再在仓内找参照：

- **desensitize.go** → RESEARCH Pattern 2：`datasetSalt = HMAC-SHA256(siteSalt, "dsalt:"+dataset)` → `l1Hash = HMAC-SHA256(datasetSalt, "field:"+column+"\x00"+value)`，hex 全 64 字符入库不截断；L2 `floor(value/width)*width`。站点盐 env 未设 → 导入 fail-closed（镜像 isAdminRequest 的 unset-拒语义，见 Shared Pattern A）。
- **decode.go** → RESEARCH Pattern 1 + §Code Examples `decodeStream` 全文（bufio.Peek 剥 BOM → 64KB utf8.Valid 探测 → UTF-8 严格流 / GB18030 Decoder.Reader 流 → csv.Reader；**顺序铁律：UTF-8 先于 GB18030**）。
- **dump.go** → RESEARCH Pitfall 2 三层文法：跳过白名单（`PRAGMA foreign_keys=OFF` / `BEGIN TRANSACTION` / `COMMIT` / `sqlite_sequence` 三件套 / `CREATE INDEX`——Open Q5）/ 接受（`CREATE TABLE` + `INSERT ... VALUES`）/ 其余整批拒。fixtures 用 RESEARCH §Code Examples 的 3.53.0 实测输出直接落 `testdata/`。

**纯函数测试形态锚**：这三件都是确定性纯函数/流变换，测试直接照 CONTEXT 测试锁清单写（无 mock 面）；`TeeReader` 进 sha256 哈希器在 decode 层做（内容哈希是 B1/B4/C1 的桥）。

---

### 6. `internal/data/importer.go`（service：三通道编排）[新]

**Analog:** `internal/duckdb/staging.go` CommitStagedDataset（105-179）——「前置校验 → 前置守卫 → 带补偿的落盘序列 → 尾步注册」的编排叙事 + godoc 写全语义清单。

**编排 godoc 形态**（staging.go 83-104——步骤语义逐条写进函数文档，失败行为写明）：

```go
// CommitStagedDataset promotes a completed staging file into a committed
// dataset (D14-D4): atomic same-volume rename + registry row write with one
// commit semantic.
//
// Semantics:
//   - DatasetsDir is created if missing (MkdirAll — idempotent).
//   - An existing target file aborts with ErrDatasetExists BEFORE any
//     mutation (replay/id-collision guard; ...).
//   - On ANY failure the staging file is removed (best-effort) ...
```

importer.go 照此把三通道（CSV / dump / push）共用后半段的「解码 → 规则门 → 逐行脱敏 → staging Appender → commit/supersede → 审计行」写成一份语义清单；通道差异只在前段（decode.go vs dump.go vs part 直通）。

**staging 写入的连接形态**（appender_throughput_test.go 56-84——仓内唯一 Appender 调用序列，直接照抄进 importer_write.go）：

```go
	c, err := New(ctx, Config{DSN: stagingPath, MaxOpenConns: 1})   // 每导入批一个专用 Connector
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()                // Close 双关 = WAL 刷盘

	_, err = c.DB().ExecContext(ctx, `CREATE TABLE wide (...)`)     // 脱敏后 schema
	driverConn, err := c.connector.Connect(ctx)                     // attach_lru.go:73 同款先例：
	                                                                 // 通过 c.connector 拿 driver.Conn
	app, err := duckdb.NewAppender(driverConn, "", "main", "wide")  // catalog="" 即默认库
	for i := 0; i < rows; i++ {
		require.NoError(t, app.AppendRow(...))                      // 值来自 desensitize 输出
	}
	require.NoError(t, app.Flush())
	... app.Close()
```

**性能契约**（appender_throughput_test.go 44-49）：`AppenderMinRowsPerSec = 100_000` 基准门 + `DUCKDB_APPENDER_MIN_RPS` env 覆盖先例已锁；导入路径禁止逐行 INSERT（新 grep 门，见 §18）。

---

### 7. `internal/data/preview.go`（store：进程内预览会话）[新]

**Analog:** `internal/runtime/fanout_runtime.go` 170-260 SharedChunkAccumulator——mutex + map + 容量上限的进程内 store 全套形态（被 TestSharedChunkAccumulatorLRUCap 锁定的成熟先例；attach_lru.go 50-58 是同款第二实现）。

**结构体 + 构造器兜底**（fanout_runtime.go 170-175 + 186-195）：

```go
type SharedChunkAccumulator struct {
	mu     sync.Mutex
	ll     *list.List               // doubly-linked; front = newest, back = oldest
	byID   map[string]*list.Element // chunk ID → list element
	max    int                      // max entries (default SharedChunkAccumulatorMaxEntries)
}
func NewSharedChunkAccumulator(maxEntries int) *SharedChunkAccumulator {
	if maxEntries <= 0 { maxEntries = SharedChunkAccumulatorMaxEntries }
	...
}
```

preview.go 差异点：map 键 = 内容哈希（D15-B1 哈希桥接的匹配键），payload = `{content_sha256, 调整后桶边界, 样本脱敏版, 创建时刻}`；上限语义从 LRU 改 TTL/容量过期（CONTEXT discretion——过期会话无害，只能匹配同一内容）。**快照返回纪律**（fanout_runtime.go 240-248 `Hits()` 注释「snapshot — the caller may iterate concurrently」）照搬。

---

### 8. `internal/data/batch.go`（service：幂等 + supersede + 审计编排）[新]

**Analog:** `internal/duckdb/staging.go`（supersede 语义基座）+ `internal/store/sqlite/datasets.go`（注册表读写）。

**幂等查询的 SQL 形态**（datasets.go 26-35——`?` 全参数化 + const SQL 字符串 + 注释写幂等理由，batch.go 全部 SQL 照此）：

```go
// putDatasetSQL is the single write path for the datasets registry
// (D14-D2). INSERT OR REPLACE keyed on name: ... Every value is a `?`
// placeholder — the row never touches fmt.Sprintf (T-3-SQLI / T-14-16).
const putDatasetSQL = `
INSERT OR REPLACE INTO datasets (
    name, tier, schema_json, file_path, content_sha256, batch_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
`
```

**batch_id 生成**：`crypto/rand` hex（RESEARCH Pattern 6 DDL 注释已定），仓内无先例但属 stdlib 一行。

**supersede 编排调序**（详见下一节 staging.go 改造——batch.go 负责读旧注册行拿 `supersedes_batch_id` → 调 SupersedeStagedDataset → 写 import_batches 审计行，两步间失败的补偿语义复刻 staging.go 165-177 的「aggregate error + operator triage」叙事）。

---

### 9. `internal/duckdb/staging.go` 新增 `SupersedeStagedDataset`（service，file-I/O）[改]

**Analog:** 自身 CommitStagedDataset（105-179）——同文件新函数，复用 ValidateDatasetID / 前置守卫 / rename 语义 / 补偿叙事四处。

**必须复刻的前置段**（staging.go 105-145——参数校验 + LRU 先 Detach + 目标存在性检查）：

```go
func CommitStagedDataset(ctx context.Context, opts CommitOptions) error {
	if opts.Registry == nil { return fmtError("CommitStagedDataset", errors.New("registry is required")) }
	if err := ValidateDatasetID(opts.DatasetID); err != nil { return err }
	if opts.Row.Name != opts.DatasetID {
		return fmtError("CommitStagedDataset", fmt.Errorf(
			"row name %q does not match dataset id %q — the registry row and file name are one identity", ...))
	}
	...
	// Release the engine-side handle first if this id is ATTACHed (idempotent).
	if opts.LRU != nil {
		if err := opts.LRU.Detach(ctx, opts.DatasetID); err != nil { ... }
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", ErrDatasetExists, target)   // ← 对 supersede 是反语义：supersede 恰要目标已存在
	}
```

**SupersedeStagedDataset 差异点**（RESEARCH Pattern 5 算法，以既有代码为基座推导）：
- 既有 `ErrDatasetExists` 守卫（:135-137）反向——supersede 前置是「目标必须存在且已注册」（否则退化为普通 commit）。
- 三步链：① `os.Rename(target, backup)`（backup 落 `data/staging/.superseded-{id}-{ts}.duckdb`——同卷，DefaultStagingDir 常量 :38 已有）→ ② `os.Rename(stagingNew, target)` → ③ `Registry.PutDataset(新行)`；② 失败 → 还原 backup；③ 失败 → 双 rename 还原。补偿失败的错误文案照 staging.go 169-172 的「UNREGISTERED; triage manually」风格。
- 尾步 `os.Remove(backup)` 前提是已 DETACH（:127-130 先例 + Windows 锁注释 :102-104）。
- 成功路径 defer 清理标志位（:149-154 `renamed` 模式）照搬，扩展为多阶段标志。

**目录常量直接可用**（staging.go 34-42）：`DefaultStagingDir = "data/staging"` / `DefaultDatasetsDir = "data/datasets"` + 同卷原子性注释。

---

### 10. `internal/store/sqlite/schema.go`（migration：datasets 扩列 + import_batches 建表）[改]

**Analog:** 自身三处先例——ALTER-gated 加列（231-262 + 301-321）、新表进 schemaSQL 尾部（plan_todos 155-186）、索引-after-ALTER 纪律（282-296）。

**datasets additive 扩列**（照 255-258 的 Phase 14 先例形态——NOT NULL DEFAULT '' + 常量 + 注册进循环）：

```go
	// Phase 14 / Plan 14-02 — PLAT-10 platform-floor version column.
	// NOT NULL DEFAULT '' mirrors the budget_layer precedent; empty
	// string denotes pre-floor era rows.
	alterAuditAddPlatformFloorVersionSQL = "ALTER TABLE audit ADD COLUMN platform_floor_version TEXT NOT NULL DEFAULT ''"
```

新增（追加进 `datasets` 的独立 ALTER 列表——照 auditLifecycleColumns 循环 :301-316 形态建 `datasetImportColumns`）：

```go
	alterDatasetsAddDatasetSQL  = "ALTER TABLE datasets ADD COLUMN dataset TEXT NOT NULL DEFAULT ''"         // 逻辑数据集名（__ 前缀）
	alterDatasetsAddPeriodSQL   = "ALTER TABLE datasets ADD COLUMN period TEXT NOT NULL DEFAULT ''"          // "2026-07"
	alterDatasetsAddKMinOverrideSQL = "ALTER TABLE datasets ADD COLUMN k_min_override INTEGER NOT NULL DEFAULT 0"  // 0 = 全局默认 5
```

同步改 schemaSQL 内 `CREATE TABLE datasets`（:206-214）使新库直出同列——「schemaSQL 是真源，ALTER 只补旧库」的分工注释在 :217-224 已写明。

**import_batches 新表**（照 plan_todos 155-186 的「独立 INSERT-only 表 + 注释块写 schema rationale」形态，DDL 用 RESEARCH Pattern 6 选项 A 的 11 列）：

```go
-- plan_todos 先例的注释块开头（schema.go 155-160）：
-- plan_todos (Phase 12 / Plan 12-02 / P12-L5 / P12-L4): the SEPARATE
-- INSERT-only audit table that backs the DeepAgent write_todos Tool ...
-- This table is INTENTIONALLY DISTINCT from audit.action — replay-query
-- is the hot path ... a dedicated table keeps the SELECT lean.
```

**索引-after-ALTER 纪律**（schema.go 287-296 注释原文——UAT 14-09 事故先例，新列索引必须独立 const 且在迁移循环之后执行）：

```go
// Executed in migrate() AFTER the audit ALTERs — same ordering contract
// as createAuditToolCallIDPendingUniqueIdxSQL (UAT 14-09: on a legacy
// DB the run_path column is added by an ALTER, so an index created
// before the ALTER loop dies with "no such column: run_path").
```

`idx_import_batches_dataset_period`（幂等查询热路径）照 `idx_audit_run_path`（:294-296）注册在 migrate() 尾部。

**migrate() 主流程零改动**（:360-408 追加式设计——只加常量、加循环项、加尾部 ExecContext）。

---

### 11. `internal/store/sqlite/datasets.go` + 12. `internal/store/sqlite/import_batches.go` + 13. `internal/store/store.go` [改/新]

**Analog:** `internal/store/sqlite/datasets.go` 全文件（97 行）——put/list 双函数 + const SQL + fmtError + Scan 同步的最小 store 文件模板。import_batches.go 直接整文件复刻此形态：

- `putImportBatchSQL`（INSERT-only——无 OR REPLACE，batch_id 是 PK）+ `getLatestBatchSQL`（幂等检查：`WHERE dataset = ? AND period = ? ORDER BY created_at DESC LIMIT 1`，RESEARCH §Code Examples）+ `listImportBatchesSQL`（ADM-02 预留）。
- Scan/RFC3339 时间往返形态照 datasets.go 71-96（`time.Parse(time.RFC3339Nano, createdAt)` → `.UTC()`）。

**store.go 行结构 + 接口扩展**（store.go DatasetRow 266-284 + 接口 533-540 的 PutDataset/ListDatasets 段）：

```go
// DatasetRow is one registered analytic dataset (Phase 14 / D14-D2). ...
type DatasetRow struct {
	Name string        // the same allowlist grammar the duckdb ATTACH layer enforces
	Tier string
	SchemaJSON string
	FilePath string
	ContentSHA256 string
	BatchID string
	CreatedAt time.Time
}
```

- `DatasetRow` 尾部 additive 加 `Dataset / Period / KMinOverride` 三字段（注释写「Phase 15 / D15-C2 / D15-D3」+ 默认值语义）。
- 新 `ImportBatchRow` 结构体照 DatasetRow 的注释密度写（每字段一句语义 + 哪个决策锚）。
- `Store` 接口尾部追加 `PutImportBatch / LatestImportBatch / ListImportBatches`（照 :533-540 的方法注释形态——一句用途 + 调用方）。
- **注意**：`internal/duckdb/staging.go:54-56` 已有窄接口先例 `DatasetRegistry`（interface isolation——staging 只依赖 PutDataset）。batch.go 若需避免 import store 全接口，同款窄接口照抄。

---

### 14. `internal/server/server.go`（route：三选项 + 路由）[改]

**Analog:** 自身 New()（295-348）。

**Hertz 选项注入点——全仓唯一 server 构造处**（server.go:296，本次改造的震中）：

```go
func New(addr string, deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(addr))       // ← 改为四选项：
	h.POST("/api/chat", deps.chatHandler)                 //   WithStreamBody(true) +
	...                                                    //   WithDisablePreParseMultipartForm(true) +
	                                                       //   WithMaxRequestBodySize(<env 可配>)
```

RESEARCH Pattern 7 已核对 hertz v0.10.6 option.go 官方 doc：前两者必须成对（StreamBody 单开时 multipart 仍预解析）。仓内 grep 实测无 `FormFile/MultipartForm/BodyStream` 任何消费者——全局开启对既有 JSON handler 无影响（流模式下 Bind 经 BodyE() 排空）。

**路由注册形态**（server.go 297-341——每路由一行 + 按_phase_分组注释块）：

```go
	h.GET("/api/audit", deps.auditHandler)
	// Phase 14 Plan 02 (PLAT-10 / D14-B2) — admin preview of the effective
	// system prompt for a Skill. ...
	h.GET("/api/admin/platform-prompt", deps.platformPromptHandler)
```

新增照此追加一个注释块 + 4-5 行：`POST /api/import/preview` / `POST /api/import/commit` / `POST /api/import/push` /（可选 dry-run）/ `DELETE /api/import/dataset/:name`。**分路由 Content-Length 守卫 middleware**（防大上限波及 /api/chat，RESEARCH Pattern 7）也在此注册——在 `h.POST("/api/chat", ...)` 之前 `h.Use(...)`。

**Deps 装配**（server.go 343-346 的「nil 时从 Store 初始化」先例——import 端点的 deps 字段照此 nil-posture，tests 可不注入）：

```go
	if deps.AuditWriter == nil && deps.Store != nil {
		deps.AuditWriter = hitl.NewAuditWriter(deps.Store, agent.PlatformFloorVersion)
	}
```

**main.go 侧接线**（cmd/server/main.go:118 envOr / :354 Deps 构造 / :463-464 sqlite.New）：`SKILLFORGE_DATA_SALT`（未设 → 导入端点 fail-closed，警告不退出——与 `SKILLFORGE_ADMIN_TOKEN` unset 同语义）+ 上限 env + datarules 目录路径，全部走 envOr 群形态。

---

### 15. `internal/server/import_api.go`（controller：multipart 流式 handler）[新]

**Analog:** `internal/server/platform_prompt.go` 52-77（Phase 14 最新 admin handler——token 门 + 403/404 + JSON envelope）+ `internal/server/audit.go` 191-276（参数校验 400 + 503 nil-posture + 500 generic envelope）。

**Token 门 + handler 骨架**（platform_prompt.go 52-77——删除端点直接照此，D15-C3）：

```go
func (s *Deps) platformPromptHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{"error": "admin token required"})
		return
	}
	skillID := string(c.Query("skill"))
	m := s.findSkill(skillID)
	if m == nil {
		c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
		return
	}
	... c.JSON(http.StatusOK, ...)
}
```

**错误 envelope 与 nil-posture**（audit.go 191-197 + 258-267——503 依赖未装配 / 400 参数 / 500 generic + WARN 日志）：

```go
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "audit store not initialized (set Deps.Store at server construction)",
		})
		return
	}
	...
	if err != nil {
		log.Printf("[WARN] server: audit query failed: %v", err)      // ← 导入 handler 的日志只打 batch 级摘要，
		c.JSON(http.StatusInternalServerError, graphResponseError{     //    永不打行值（D15-B4 / Pitfall 8）
			Error: "audit query failed — see server logs",
		})
		return
	}
```

`graphResponseError` 定义在 `internal/server/knowledge.go:64-66`（`{Error string \`json:"error"\`}`——全 server 统一 envelope，直接复用不新造）。

**multipart 流式读**（无仓内消费者，grep 已证；RESEARCH Pattern 7 给的形态为准）：

```go
	mr := multipart.NewReader(c.Request.BodyStream(), c.Request.MultipartFormBoundary())
	for { part, err := mr.NextPart(); ... io.Copy(解码管线, io.TeeReader(part, hasher)) }
```

**删除端点动作序列**（D15-C3，复用件已全部核实）：`isAdminRequest` → `ValidateDatasetID(name)`（connector.go:112）→ `LRU.Detach(ctx, name)`（attach_lru.go:136-150，幂等）→ `os.Remove(文件)` → 注册表行处理 → INSERT-only 审计行。**先查询后删除用例**（触发懒 ATTACH 再删）锁 Windows 锁序（RESEARCH Pitfall 9 / TestDeleteDatasetAudited）。

**测试形态**：复刻 `audit_test.go` 的 TestAdminToken_* 四用例（UnsetEnv→403 / WrongTokenTimingSafe / CorrectToken / ConstantTimeCompareIsUsed——14-PATTERNS §10 已引证骨架）。

---

### 16. `datarules/{dataset}.yaml` [新]

**Analog:** `skills/<name>/SKILL.md` 的声明式受控文件形态（loader.go:32-45「File-driven registration (D2-07): the single source of truth ... is the filesystem」叙事 + 「Validation errors are NOT silently swallowed」纪律）。

规则文件 = 整文件 YAML（无 frontmatter，比 SKILL.md 少一层）；schema 按 RESEARCH §Code Examples 提案（dataset / tier / k_min / columns 列表）。命名约束：dataset 名禁连续下划线（消除 `loans__2026-07` 复合名歧义，RESEARCH A4）——该校验进 rules.go 的 ValidateRules。

---

### 17. `integration/contract/import_gates_test.go`（test：结构性禁令门）[新]

**Analog:** `integration/contract/platform_floor_bypass_test.go`（D8-04 WalkDir 门形态——Phase 14 刚交付的第三例）。

**必复刻骨架**（platform_floor_bypass_test.go 36-61 + 93-126——scan roots 列表 + skip dirs + `//go:build contract` tag + WalkDir 收集）：

```go
//go:build contract

var platformFloorScanRoots = []string{ "internal/agent", ..., "internal/platformfloor" }
var platformFloorSkipDirs  = map[string]bool{ ".claude": true, ".git": true, ... }

func TestNoBypassOfPlatformFloorCompose(t *testing.T) {
	root := findModuleRoot(t)
	// WalkDir 收集 .go 文件 → 断言集（文件自身跳过：d.Name() == "platform_floor_bypass_test.go"）
```

新门断言集（RESEARCH Wave 0 gaps + Pitfall 8）：对 `internal/server/import_api.go` + `internal/data/` 扫描——① 禁 `.MultipartForm(`/`.FormFile(`（触发预解析路径）；② 禁逐行 `INSERT INTO`（导入写路径只许 Appender）；③ import_api.go 内禁 log 行值（结构性断言：只许 batch 级摘要字段名）。文件自身带禁令字面量须自跳过（:117-119 先例）。

---

### 18. `scripts/verify-phase-15.sh` + `Makefile` [新/改]

**Analog:** `scripts/verify-phase-14.sh`（260 行全套）+ Makefile:45-53。

**run_stage helper 逐字复刻**（verify-phase-14.sh 55-74——只改 phase 号与 log 前缀）：

```bash
run_stage() {
  local name="$1"; shift
  stage_num=$((stage_num + 1))
  local nn; nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-14-stage-${nn}.log"    # → phase-15
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  ...
}
```

**Phase 15 版段序建议**（复刻 14 的 10 段骨架，替换中间段）：gcc 门（:99-100 原样）→ build+vet → race 全量 → sqlite 套件 → duckdb 套件 + TestSupersedeAtomicSwap → **internal/data 新套件**（CONTEXT 测试锁 10 项按包分组 -run 正则——照 :159-163 的多测试名 `|` 连接形态）→ **contract 禁令门**（:162-163 的 `-tags=contract` 调用形态原样）→ **source-grep 段**（:191-198 形态，新锁：`k_min` 默认常量 / 上限常量 / `WithDisablePreParseMultipartForm` 存在性 / HMAC 构造不手拼）→ 冻结字节段（Phase 15 无新 lock.json——若不动 prompt 面，4 锁 sha256 原样重申，照 :222-234）。

**Windows 主机警示照抄头部**（:17-22——go-duckdb 不能在 Windows-native gcc 链接，emutls ABI；Phase 15 全套 gate 仍须 WSL2/Linux 跑）。

**Makefile**（:45-53 形态 + :9 .PHONY 追加）：

```makefile
# verify-phase-14 is the Phase 14 HERMETIC gate (Plan 14-07). ...
verify-phase-14:
	bash scripts/verify-phase-14.sh
# → verify-phase-15 同形态（含 internal/data 套件 + import 禁令门的说明注释）
```

---

## Shared Patterns

### A. X-Admin-Token 常量时间门（V4 访问控制）
**Source:** `internal/server/audit.go:114-129`
**Apply to:** 删除端点（D15-C3 必挂）+ preview/commit/push 三端点（RESEARCH Open Q6 推荐同挂——planner 终裁）。复用 `isAdminRequest(c)`，**不新造比较逻辑**；unset env = 全拒（demo-OOB 姿态）。站点盐 `SKILLFORGE_DATA_SALT` 未设 → 导入端点 fail-closed 拒绝，**镜像同一 unset 语义**。

```go
func isAdminRequest(c *app.RequestContext) bool {
	expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	if expected == "" { return false }
	var got string
	c.Request.Header.VisitAll(func(k, v []byte) { if string(k) == "X-Admin-Token" { got = string(v) } })
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

### B. Handler 错误 envelope（503/400/403/404/500）
**Source:** `internal/server/knowledge.go:64-66`（graphResponseError）+ `audit.go:191-276` + `platform_prompt.go:52-77`
**Apply to:** import_api.go 全部 handler。日志纪律：WARN 级打 err（不含行值），响应给 generic 消息。

### C. ALTER-gated 幂等迁移 + 索引-after-ALTER
**Source:** `internal/store/sqlite/schema.go:231-262`（ALTER 常量）+ `:301-321`（列注册循环）+ `:282-296`（索引时序契约，UAT 14-09 事故注释）
**Apply to:** datasets 三新列 + `idx_import_batches_dataset_period`。schemaSQL 是新库真源，ALTER 只补旧库；引用新列的索引必须独立 const 且在迁移循环后执行。

### D. INSERT-only 审计纪律（T-3-AUDIT）
**Source:** `internal/store/sqlite/schema.go:28-31`（包注释「INSERT + SELECT paths only — NO row-modification path」）+ `datasets.go:26-35`（全 `?` 占位）
**Apply to:** import_batches.go（批次行/删除行只 INSERT 永不 UPDATE/DELETE）+ 删除动作的审计行。SQL 一律 const 字符串 + `?` 参数，永不 fmt.Sprintf 用户值。

### E. Fail-closed 加载期校验（default-deny + 首错短路 + 错误指名字段）
**Source:** `internal/skill/validate.go:50-147` + `loader.go:42-45`（「Validation errors are NOT silently swallowed」）
**Apply to:** rules.go 的 ValidateRules（列集合双向比对在导入时做并**列出全部未知列名**——TestUnknownColumnFailClosed）+ dump.go 三层文法（默认拒，白名单显式跳）+ period YYYY-MM 格式校验。

### F. fmtError 包装 + sentinel error
**Source:** `internal/duckdb/connector.go:200-207`（`duckdb.%s: %w` 镜像 sqlite 形态）+ `staging.go:44-48`（ErrDatasetExists sentinel + errors.Is 可判）
**Apply to:** internal/data 全部新文件（`data.%s: %w` 形态）+ staging.go 新函数。supersede 的失败分类（哪步失败→哪类补偿）靠 sentinel 区分，测试注入点据此设计。

### G. 结构性常量 source-grep 锁
**Source:** `fanout_runtime.go:63`（常量注释「literal is the call-site source-grep target ... NOT a config knob」）+ `appender_throughput_test.go:44-49` + `verify-phase-14.sh:191-198`（grep 段）
**Apply to:** `KMinDefault = 5`（D15-D3）、预览 TTL、上传上限默认值、规范 profile 名表——全部字面量进代码 + verify-phase-15.sh grep 锁定；env 只做覆盖（DUCKDB_APPENDER_MIN_RPS :91-96 先例），不进 config 文件。

### H. 同卷原子 rename + 失败补偿 + Windows 先 DETACH
**Source:** `internal/duckdb/staging.go:147-177`（rename + PutDataset 失败回滚 + aggregate error 叙事）+ `:124-130`（pre-commit Detach）+ `:102-104`（「MUST NOT os.Remove ... without a prior Detach」）
**Apply to:** SupersedeStagedDataset 三步链（备份-就位-注册）与删除端点（Detach → Remove）。测试注入面：rename 前后 + 注册表写三处（Pitfall 3 双面）。

### I. 进程内 mutex+map store + 快照返回
**Source:** `internal/runtime/fanout_runtime.go:170-260`（SharedChunkAccumulator）/ `internal/duckdb/attach_lru.go:50-58`（同款第二例）
**Apply to:** preview.go 预览会话 store（键 = 内容哈希；TTL/容量兜底；Get 返回快照）。

### J. WalkDir 结构性禁令门（D8-04 系）
**Source:** `integration/contract/platform_floor_bypass_test.go:36-126`（scan roots + skip dirs + `//go:build contract` + 门文件自跳过）
**Apply to:** import_gates_test.go 三断言（禁 MultipartForm/FormFile、禁逐行 INSERT、禁行值日志）。

---

## No Analog Found

| 文件 | Role | Data Flow | 原因与替代 |
|------|------|-----------|------------|
| `internal/data/decode.go` | utility | streaming | 仓内生产代码零 GB18030/x-text 引用（grep 实测；mongo.go:245 仅一条「avoid unicode/norm」注释）。用 15-RESEARCH Pattern 1 + §Code Examples `decodeStream` 骨架（x/text v0.28.0 API 已模块缓存实测）——**utf8 先于 GB18030 的顺序铁律**写进包注释。 |
| `internal/data/normalize.go` | utility | transform | 仓内无 Unicode 规范化先例。RESEARCH Pattern 2 三档 profile 已带本机实测证据（TrimSpace 剥 U+3000 / NFC 不折叠全角 / NFKC 折叠）——照做即测试即锁（TestHMACNormalizationJoinability）。 |
| `internal/data/desensitize.go` | utility | transform | 仓内无 crypto 构造先例。stdlib `crypto/hmac` + RESEARCH Pattern 2 盐推导链（datasetSalt 不落库）；**禁止手拼 hash(盐‖值)**（Don't Hand-Roll 表）。 |
| `internal/data/dump.go` | service | transform | 仓内无 SQL 解析器（平台铁律「永不执行」本就排除 sqlite 引擎路径）。RESEARCH Pitfall 2 三层文法 + §Code Examples 的 3.53.0 实测 fixture；~200 行定向 tokenizer，默认拒 + 显式白名单跳过。 |
| multipart 流式 handler 段 | controller | streaming | 仓内零 multipart 消费者（grep 实测——这同时证明全局 Hertz 三选项安全）。RESEARCH Pattern 7 的 `multipart.NewReader(BodyStream(), MultipartFormBoundary())` 形态 + Pitfall 1 三件套。 |

## Metadata

**Analog search scope:** internal/duckdb、internal/store（+sqlite）、internal/server、internal/skill、internal/runtime、internal/reprohash（经 14-PATTERNS 引证）、integration/contract、cmd/server、scripts、Makefile、.gitignore、eval fixtures
**Files scanned（本 session 精读）:** connector.go / staging.go / attach_lru.go / appender_throughput_test.go / schema.go / datasets.go / store.go / server.go / audit.go(server) / platform_prompt.go / knowledge.go(:58-77) / validate.go / loader.go / fanout_runtime.go(:55-264) / platform_floor_bypass_test.go / verify-phase-14.sh / Makefile / .gitignore / main.go（grep 定位）
**Pattern extraction date:** 2026-08-23
**关键提醒（planner 必读）:**
① `server.Default` 全仓唯一构造点在 server.go:296——三选项落这里，同时必须新增分路由 Content-Length 守卫防大上限波及 /api/chat（RESEARCH Pitfall 1）；
② supersede 是 staging.go 的**同文件新函数**而非改 CommitStagedDataset——既有 ErrDatasetExists 守卫对 supersede 是反语义，两入口并存（RESEARCH Pattern 5 已论证）；
③ 注册名 `{dataset}__{period}` 复合名让 CommitStagedDataset / AttachLRU / 删除端点零改动复用，但要求 rules.go 校验 dataset 名禁连续下划线（RESEARCH A4）；
④ 规则文件目录**不可**放 `data/rules/`（.gitignore:28 实测整目录忽略）——放仓库根 `datarules/`（Open Q2 推荐已核实成立）；
⑤ TestRawNeverOnDisk 必须同时扫 `data/staging/` + `data/datasets/` + `os.TempDir()`（Hertz 预解析泄漏在 /tmp，Pitfall 1 警示）；
⑥ 全套 verify gate 仍受 Windows emutls 限制（verify-phase-14.sh:17-22）——Phase 15 的 duckdb 段照旧只能 WSL2/Linux 跑，internal/data 纯函数测试段可在 Windows 本地跑（planner 可据此拆 stage）。
