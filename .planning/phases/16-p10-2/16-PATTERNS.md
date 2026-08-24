# Phase 16: KPI 聚合通道（P10.2）- Pattern Map

**Mapped:** 2026-08-24
**Files analyzed:** 26（新建 12 + 修改 14，其中 2 项 planner 终裁可选）
**Analogs found:** 23 / 26（3 项无仓内 analog，见「No Analog Found」——全部已由 16-RESEARCH §Architecture Patterns / §Code Examples 给出实测骨架）

> 本文件是 planner 的直接输入：每个 plan 的 action 应引用下述 analog 文件与行号的代码形态。
> 所有行号基于 2026-08-24 的 master 工作区实测精读（非二手引证）。
> 先例资产（datarules loader / D11-B1 双层执法 / import_batches / retrieve.go 工厂）全部在本 session 重新核对。

**对上游线索的三处修正（已核实）：**
1. **D16-C3 运行时装配点的真实位置**：16-RESEARCH Pattern 4 写「`agent.go:180-256` NewAgent 目前无条件构建 retrieveTool——这是要改的装配分支点」。实测：`internal/agent/agent.go:204-256` 的 NewAgent 是 **`USE_AGENT_PATH=true` 实验路径**（`cmd/server/main.go:271-288` 的 useAgentPath gate + 注释原文 "opt-in experimental path" 证实）。**生产主路径是 `internal/agent/runner_setup.go:196-201`**（BuildRunner：`BuildToolsForSkill(ctx, ...)` 之后 `tools = append(tools, retrieveTool)` 无条件追加）。通道分支的震中在 runner_setup.go；agent.go 可同步改（实验路径 data Skill 不支持亦可，planner 定，但注释必须写明）。
2. **main.go 装配段实际行号**：datarules env 群在 `cmd/server/main.go:186-195`（`SKILLFORGE_DATARULES_DIR` 等），importer 完整装配在 `:301-323`（duckdb.New → NewAttachLRU → data.NewImporter → [WARN] fail-closed 分支）。KPI loader 装配照这两段形态。
3. **ValidTypes 计数**：`internal/wire/chunk.go:334` 注释写 "Count: 22"、`internal/wire/types.go:24` 写 23——实际 map（chunk.go:355-383）数出 23 键。Phase 16 **不新增 chunk 类型**（D16-B3 复用 data-decision），计数锁（chunk_test.go）不受影响；顺手校正注释可选、非必须。

---

## File Classification

| 新建/修改文件 | Role | Data Flow | 最近 Analog | 匹配度 |
|---------------|------|-----------|-------------|--------|
| `internal/kpi/define.go` [新] | model（YAML 结构体） | — | `internal/data/rules.go:43-75`（Rules/ColumnRule + 词表常量） | exact |
| `internal/kpi/loader.go` [新] | service（受控文件 loader） | file-I/O + transform | `internal/data/rules.go:85-108`（LoadRules 全形态） | exact |
| `internal/kpi/registry.go` [新] | store（不可变 registry + 只读接口） | — | `internal/agent/hitl/registry.go:76-121`（package-init 常量 + lock-free 读） | exact |
| `internal/kpi/validate_text.go` [新] | utility（Layer 1 纯 Go 校验） | transform | `internal/skill/validate.go:50-147` + `internal/data/rules.go:130-210` | exact（组合） |
| `internal/kpi/validate_sql.go` [新] | service（json_serialize_sql AST 走查） | transform | 无仓内 analog（16-RESEARCH §Code Examples + Wave 0 探针） | none |
| `internal/kpi/executor.go` [新] | service（Call(ctx,name,params) 编排） | request-response | `internal/agent/tools/retrieve.go:113-173`（闭包形态）+ `internal/duckdb/staging.go:83-104`（编排 godoc） | role-match（组合） |
| `internal/kpi/kmin.go` [新] | utility（行级抑制纯函数） | transform | `internal/data/desensitize.go:28-39`（KMinDefault 语义链；行为语义是行替换——刻意偏差） | partial |
| `internal/agent/tools/kpi_tool.go` [新] | component（eino Tool 工厂） | request-response | `internal/agent/tools/retrieve.go` 全文件 | exact |
| `internal/store/sqlite/kpi_calls.go` [新] | store（INSERT-only 审计） | CRUD（INSERT-only） | `internal/store/sqlite/import_batches.go` 全文件 | exact |
| `kpis/loans.yaml` [新] | config（受控声明文件） | — | `datarules/loans.yaml` | exact |
| `internal/server/kpi_admin.go` [新，可选] | controller（admin 只读端点） | request-response | `internal/server/platform_prompt.go:52-77`（经 15-PATTERNS §15 引证：token 门 + 403/404 + JSON envelope） | exact |
| `scripts/verify-phase-16.sh` [新] | gate 脚本 | batch | `scripts/verify-phase-15.sh`（经 15-PATTERNS §18 引证：run_stage + source-grep 段） | exact |
| `internal/skill/manifest.go` [改：validReadPatterns + ChannelKind] | model（词表/通道标记） | — | 自身 validAgentTypes 词表 :160-173 + Reads 字段 :60 | 改造点 |
| `internal/skill/validate.go` [改：reads XOR 互斥] | middleware（加载期校验） | — | 自身 reads 段 :72-91 + writes known-target 段 :129-145 | 改造点 |
| `internal/skill/tier.go` [改：ResolveTiers 前置分支] | utility | transform | 自身 ResolveTiers :120-148 | 改造点 |
| `internal/agent/runner_setup.go` [改：BuildRunner 通道分支] | config（运行时装配） | — | 自身 :176-201（BuildToolsForSkill + retrieveTool 追加——D16-C3 震中） | 改造点 |
| `internal/agent/agent.go` [改：NewAgent 实验路径，可选同步] | config（实验装配） | — | 自身 :224（USE_AGENT_PATH opt-in 路径） | 改造点 |
| `internal/agent/callbacks.go` [改：工具名分流] | middleware（decision 回调） | event-driven | 自身 toolHandler.OnStart :138-148（误标源） | 改造点 |
| `internal/store/store.go` [改：KPICallRow + 接口] | model（行结构+接口） | — | 自身 ImportBatchRow :297-328 + 接口 :604-621 | 改造点 |
| `internal/store/sqlite/schema.go` [改：kpi_calls DDL additive] | migration（DDL） | — | 自身 import_batches 块 :219-243 + dataset_deletes :249-254 | 改造点 |
| `internal/wire/types.go` [改：DecisionKPIReference] | model（wire 枚举） | — | 自身 DecisionFanout* :78-111（P13-L7 additive 先例） | 改造点 |
| `internal/wire/chunk.go` [改：DecisionPayload.DataRef] | model（wire payload） | — | 自身 DecisionPayload :157-170（RunPath/TransferFrom/TransferTo additive 字段） | 改造点 |
| `web/src/types/wire-types.ts` [改：TS 镜像 lockstep] | model（TS 镜像） | — | 自身 DecisionFanoutDispatched :78-83（"only the decision STRING enum widens"） | 改造点 |
| `cmd/server/main.go` [改：KPI loader 装配] | config（装配） | — | 自身 :186-195（env 群）+ :301-323（importer 装配段） | 改造点 |
| `Makefile` [改：verify-phase-16 target] | config（构建门） | — | 自身 verify-phase-15 target（15-PATTERNS §18 引证） | 改造点 |

**测试文件**（随实现落地，归对应 plan；名字与 CONTEXT §Specific test lock requirements 一一对应，hermetic/WSL 分类照 16-RESEARCH Validation 节）：
- `internal/kpi/`：TestKPIRegistryImmutableAtRuntime / TestNoSQLGenerationSurface（宿主可放 internal/agent）/ TestKPILoadFailClosed / TestNoMatchingKPIRefusal / TestEnumParamFailClosed / TestKMinRowSuppression / TestKPISnapshotFrozen(WSL) / TestASofAutoGenerated / TestSameKPIAcrossSkills(WSL) / TestProbeJSONSerializeSQL(WSL Wave 0)
- `internal/skill/`：TestChannelGuardXOR（含 reloadOne 路径用例）
- `internal/agent/`：TestToolsForChannel
- `internal/store/...`：TestKPICallsDDL / TestKPICallsInsertOnly
- `internal/wire/`：chunk_test.go 计数锁复核 + DataRef 形态用例

---

## Pattern Assignments

### 1. `internal/kpi/define.go`（model：YAML 结构体）+ 11. `kpis/loans.yaml`（config）[新]

**Analog:** `internal/data/rules.go:43-75` + `datarules/loans.yaml` 全文件——D16-A1「镜像 datarules 先例」的直接模板。

**受控文件结构体形态**（rules.go:43-60——godoc 写 CONTROLLED file 纪律 + 审计锚语义，KPIFile/KPIDef/ParamSpec 照此密度写）：

```go
// Rules is the in-memory form of a datarules/{dataset}.yaml rule file ...
// The file is a CONTROLLED file: versioned in git, changed via PR review
// (the primary defense against a passthrough misdeclaration — T-15-06),
// and its content digest (rules_sha256) rides every import-batch audit
// row so "which rule version desensitized this batch" is always
// provable (D15-A4).
type Rules struct {
	Dataset string                `yaml:"dataset"`
	Tier    string                `yaml:"tier"`
	KMin    int                   `yaml:"k_min"`
	Columns map[string]ColumnRule `yaml:"columns"`
}
```

**词表常量段**（rules.go:66-70——RuleHash/RulePassthrough/RuleBucketFixed 三常量 + default-deny 注释；ParamSpec 的 type 词表 `string|enum` 照此）：

```go
const (
	RuleHash        = "hash"
	RulePassthrough = "passthrough"
	RuleBucketFixed = "bucket_fixed"
)
```

**受控文件本体形态**（datarules/loans.yaml 全文——每行行内注释写约束与决策锚；kpis/loans.yaml 按 16-RESEARCH Pattern 1 的 YAML schema 提案写：dataset / description / k_min / kpis[]（name+description+sql+count_column+params））：

```yaml
dataset: loans          # 必须与注册名 __ 前缀一致（禁 "__" 子串）
tier: 历史               # 权威|历史|个人（TierEnum 词表）
k_min: 10               # 可选 override（缺省 0 = 用全局 KMinDefault=5）
```

注意 kpis/loans.yaml 与 datarules/loans.yaml 的 dataset 名同用 `loans`（逻辑数据集名，datasets 注册表 `dataset` 列同值——as_of 水位与 k_min_override 都按它查）。

---

### 2. `internal/kpi/loader.go`（service：受控文件 loader）[新]

**Analog:** `internal/data/rules.go:85-108` LoadRules——symlink guard → ReadFile → sha256 → yaml.Unmarshal → Validate 五段式，KPI loader 第四次复刻。

**LoadRules 全形态**（rules.go:85-107——逐行照抄换语义）：

```go
func LoadRules(path string) (*Rules, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("lstat %s: %w", path, err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmtError("LoadRules", fmt.Errorf(
			"%s is a symlink — rule files are regular controlled files only (V12 symlink-swap guard)", path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("read %s: %w", path, err))
	}
	sum := sha256.Sum256(raw)
	rulesSHA := hex.EncodeToString(sum[:])
	var m Rules
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("unmarshal YAML in %s: %w", path, err))
	}
	if err := ValidateRules(&m); err != nil {
		return nil, "", fmtError("LoadRules", fmt.Errorf("validate %s: %w", path, err))
	}
	return &m, rulesSHA, nil
}
```

**KPI loader 差异点**（相对 LoadRules）：
- 目录加载（`LoadKPIs(dir)`）而非单文件：WalkDir 收 `kpis/*.yaml`（镜像 `internal/skill/loader.go:89-157` Scan 的 WalkDir + 首错短路 + "partial state unspecified" 语义）；**10 个文件 1 个坏 = 整体失败**（16-RESEARCH Anti-Pattern「部分加载」禁令）。
- 校验双层：Layer 1 纯 Go（validate_text.go，hermetic）+ Layer 2 json_serialize_sql（validate_sql.go，WSL）——LoadRules 是单层，这是刻意扩展。
- `:param` → `$param` 重写发生在 Layer 1/Layer 2 全部通过之后、编译进 registry 之前（Pitfall 3；重写后重跑占位符覆盖检查双重确认）。
- 签名哈希口径（Discretion 8）：建议目录聚合哈希（全部文件内容按文件名排序后串接 sha256——与「registry 整体原子替换」语义一致）。

---

### 3. `internal/kpi/registry.go`（store：不可变 registry + 只读接口）[新]

**Analog:** `internal/agent/hitl/registry.go:76-121`——「package init 后只读、无 mutex、lock-free 读」的不可变 registry 形态，TestKPIRegistryImmutableAtRuntime 的锁法依据。

**不可变 map 形态**（hitl/registry.go:76-101——注释原文逐条复刻到 kpi registry）：

```go
// hitlHandlers is the D11-B3 map from write-target name to the
// InvokableTool that handles the write. The map is READ-ONLY after
// package init — there is no mutex; reads are lock-free. If a future
// Skill hot-reload needs to swap handlers at runtime, the swap is a
// wholesale map replacement under a write lock (out of scope for
// 11-02).
var hitlHandlers = map[string]tool.InvokableTool{ ... }
```

**防御性拷贝 + 排序返回**（hitl/registry.go:116-121——Catalog()/Lookup() 照此；错误信息稳定 + 调用方无法毒化 registry）：

```go
func KnownTargets() []string {
	out := make([]string, len(AllowedWriteTargets))
	copy(out, AllowedWriteTargets)
	sort.Strings(out)
	return out
}
```

**KPI registry 差异点**：hitl registry 是 package-init 常量，KPI registry 是**启动期编译产物**（LoadKPIs 返回值）——不可变性来自「结构体只导出读方法、map 不导出」，形态参照 `internal/skill/loader.go:52-78`（Loader.cached + RWMutex + Scan 原子换 slice：`l.cached = out` 整体替换而非 append，concurrent 读者持旧 slice 不受扰）。若选热重载（Discretion 9），照 `internal/skill/registry.go` 的 reloadOne → 新 registry 原子替换模式；v1 建议仅启动期加载（16-RESEARCH Open Q1 倾向）。接口面只暴露 `Lookup(name)` / `Catalog()` / `Signature()` / `Names()`——无任何 Add/Update/Set 方法（TestKPIRegistryImmutableAtRuntime 用 reflect 断言导出方法集）。

---

### 4. `internal/kpi/validate_text.go`（utility：Layer 1 纯 Go 校验）[新]

**Analog:** `internal/data/rules.go:130-210` ValidateRules（第四复刻的直接母版）+ `internal/skill/validate.go:50-147`（词表校验形态）。

**fail-closed 校验形态**（rules.go:169-201——switch + 词表 default-deny + 错误命名字段/规则/决策锚；ParamSpec 的 type/values/pattern/required 校验照此写）：

```go
	for _, name := range names {
		cr := m.Columns[name]
		switch cr.Rule {
		case RuleHash:
			if cr.Profile != "" && !containsString(KnownProfiles(), cr.Profile) {
				return fmtError("ValidateRules", fmt.Errorf(
					"column %q: hash rule profile %q is unknown (known profiles: %v); ... do NOT invent profile names ... (D15-A1 single source of truth)",
					name, cr.Profile, KnownProfiles()))
			}
		...
		default:
			return fmtError("ValidateRules", fmt.Errorf(
				"column %q: rule %q is not supported (valid rules: ...)",
				name, cr.Rule))
		}
	}
```

Layer 1 校验规则集（全部 hermetic 可测）：YAML schema 必填段、KPI name 唯一且过 `duckdb.ValidateDatasetID` 同款词法（`internal/duckdb/connector.go:109-117`，15-PATTERNS 已引证）、`:param` 占位符 ↔ params 声明**双向**一一对应（rules.go:259-268 ValidateColumnSet 的双向 diff 报错形态可借用——占位符多了/声明少了分别列出）、enum values 非空、pattern 可编译（`regexp.Compile` 失败即拒）、count_column 出现在 SELECT 别名中（文本层预检，AST 层复核）、SQL 含 GROUP BY 或聚合函数的启发式（Pitfall 7 规则①）。

**首错短路 vs 全量 diff 的分工先例**（rules.go:126-129 注释原文）：文件自身校验首错短路（镜像 Skill validator），集合比对类（列/占位符）一次性列全 diff——照抄这个分工。

---

### 5. `internal/kpi/validate_sql.go`（service：json_serialize_sql AST 走查）[新]

**无仓内 analog**（grep 实测：仓内零 SQL parser）。planner 直接采用 16-RESEARCH §Code Examples 的 `sqlEnvelope`/`sqlStmt`/`sqlNode` 骨架 + 四条 fail-closed 规则（envelope.Error / len(statements)≠1 / node.type≠SELECT_NODE / BASE_TABLE 收集判 `^ds_{declaredDataset}\.`）。

**两条已核实的执行面事实**（写进代码注释）：
- 表引用前缀 `ds_{id}` 是 AttachLRU 的 ATTACH 别名（`internal/duckdb/attach_lru.go:91-94` 注释原文 "ATTACHes the dataset READ_ONLY under the ds_{id} alias"）——AST 表引用判定以此为机械依据。
- Wave 0 探针（TestProbeJSONSerializeSQL，WSL）先 lock A1/A2/A3 再写走查逻辑（16-RESEARCH Open Q2：valid SELECT / invalid SQL / INSERT / 跨别名 JOIN / 空 FROM 五用例）。

---

### 6. `internal/kpi/executor.go`（service：Call 编排）[新]

**Analog:** `internal/agent/tools/retrieve.go:113-173`（闭包捕获不可变快照）+ `internal/duckdb/staging.go:83-104`（步骤语义写进 godoc）+ `internal/duckdb/staging.go:59-65`（窄接口隔离）。

**闭包捕获不可变量**（retrieve.go:124-131——注释原文「Snapshot the tier slice ONCE at construction」是 KNOW-03 闭包的叙事模板；kpi executor/tool 闭包捕获编译期 registry 快照同构）：

```go
	// Snapshot the tier slice ONCE at construction. This is the KNOW-03
	// closure: the agent LLM has no way to mutate the tiers that
	// Retriever.Search receives because the tool function does not read
	// from any LLM-supplied field that maps to tiers ...
	tiers := m.ResolvedTiers()
```

**窄接口隔离**（staging.go:59-65——executor 的 as_of 生成依赖照此只声明 `LatestPeriod(ctx, dataset) (string, error)`，不让 kpi 包 import 整个 store.Store）：

```go
// DatasetRegistry is the narrow write+read seam the staging commit path
// needs from the sqlite store (interface isolation: staging depends on the
// two dataset operations, not the whole store.Store).
type DatasetRegistry interface {
	PutDataset(ctx context.Context, row store.DatasetRow) error
}
```

**编排 godoc 形态**（staging.go:83-104 CommitStagedDataset——语义清单逐条写：前置校验/失败行为/补偿；executor.Call 的 ①无匹配→结构化结果 ②参数 fail-closed ③as_of 系统生成 ④AttachLRU+sql.Named 执行 ⑤kmin 后过滤 ⑥三路输出 快照照此写成一份 godoc）：

```go
// CommitStagedDataset promotes a completed staging file into a committed
// dataset (D14-D4): atomic same-volume rename + registry row write with one
// commit semantic.
//
// Semantics:
//   - DatasetsDir is created if missing (MkdirAll — idempotent).
//   - An existing target file aborts with ErrDatasetExists BEFORE any
//     mutation (replay/id-collision guard; ...).
```

**执行面连接**（attach_lru.go:95-129 Get——懒 ATTACH 已现成，executor 直接 `lru.Get(ctx, datasetID, path)` 后在同一 dedicated session 上跑预编译 SQL；`sql.Named` 绑定形态照 16-RESEARCH §Code Examples go-duckdb statement_test.go 实测引用）。period 水位读取（A4 语义）：datasets 按 name 主键、同一 dataset 可多 period 批次行，取 `ORDER BY created_at DESC` 第一条的 period——与 `getLatestBatchSQL`（import_batches.go:44-52）同构。

**as_of 结构 + 三路输出**：照 16-RESEARCH Pattern 5 的 `ASOf{Period, Ts}` 定义（同时是 wire DataRef 与 kpi_calls 行的字段口径）；⑥ 三路输出 = 工具 JSON 返回 agent / SSE data-decision（DataRef）/ PutKPICall 审计行——三处字段同源（单一 Snapshot 结构序列化，防漂移）。

---

### 7. `internal/kpi/kmin.go`（utility：行级抑制纯函数）[新]

**Analog:** `internal/data/desensitize.go:28-39`（KMinDefault 语义链）——**语义链同源、行为刻意不同构**。

**k_min 取值链锚点**（desensitize.go:28-39——注释原文已预言 Phase 16 消费："v1 ships the constant, Phase 16 enforces it"）：

```go
// KMinDefault is the Phase 15 GLOBAL default for the k-anonymity query
// floor (D15-D3). A dataset may override it per-import via the rule
// file's k_min field (0 = use this default; the override is stored in
// the datasets registry, Phase 16 reads it). ...
// values below k_min are suppressed at query time by the query-side
// defense line — v1 ships the constant, Phase 16 enforces it).
const KMinDefault = 5
```

取值链三段：KPI 文件 k_min > 0 ? : datasets.k_min_override（`internal/store/store.go` DatasetRow.KMinOverride 字段，schema.go:216 列）> 0 ? : KMinDefault——与 datarules/datasets 既有语义对齐。

**行为偏差（必须写进 godoc + 测试断言）**：kmin.go 是 **Go 层后过滤纯函数**——`count < kmin` 的行把**值列**替换为 `"suppressed"`（行与计数保留），**不是** SQL `HAVING`（行消失）也不是整体拒答（D16-D1）。TestKMinRowSuppression 断言**行数不变**（16-RESEARCH Pitfall 4 警示：断言行数变少 = 实现错了）。输入 = rows + count_column 名 + kmin 值；输出 = 处理后 rows + suppressed 计数 + k_min_applied。

---

### 8. `internal/agent/tools/kpi_tool.go`（component：kpi_call 工具）[新]

**Analog:** `internal/agent/tools/retrieve.go` 全文件（221 行）——「一个工具闭包一个通道」的 sibling 模板。

**工具名常量 + 参数/返回结构**（retrieve.go:53 + 74-88——json tag 小写约定 + 结构体 godoc 写 wire 契约）：

```go
const RetrieveToolName = "retrieve_compliance_chunks"
type retrieveArgs struct {
	Query string `json:"query"`
}
type retrieveResponse struct {
	Chunks string `json:"chunks"`
	Count  int    `json:"count"`
}
```

kpi_tool 照此：`KPICallToolName = "kpi_call"`；args = `{name string, params map[string]any}`；response = 快照 JSON `{func, args, as_of, rows, k_min_applied}`。**no_matching_kpi 是工具结果不是 Go error**（D16-B2 / 16-RESEARCH Anti-Pattern：返回 `{reason_code: "no_matching_kpi", human_explanation, available_kpis}` 的正常 JSON——retrieve.go:145-149 的 Go error 形态只留给 infra 失败）。

**ToolInfo + ParamsOneOf 形态**（retrieve.go:133-143——name 参数带 `Enum: registry 全部 KPI 名`（eino v0.9.14 `schema.ParameterInfo.Enum` 已在 16-RESEARCH Standard Stack 现场核实）、params 子对象用 SubParams）：

```go
	desc := &schema.ToolInfo{
		Name: RetrieveToolName,
		Desc: "Retrieve compliance-regulation chunks from the knowledge tiers this Skill is authorized to read ...",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "The compliance question or refined search query ...",
				Required: true,
			},
		}),
	}
```

**desc 内嵌目录**（D16-B1）：description 字符串 = 前缀文案 + 启动期生成的 catalog（字母序、每行 `name — description（参数摘要）`、~30 条截断）——catalog 是**启动期编译快照**进闭包（16-RESEARCH Don't Hand-Roll 表：运行时拼 prompt = registry 可变性隐性后门）。

**构造器前置 panic + utils.NewTool**（retrieve.go:113-122 + 168-172——nil 参数 panic "caller bug" 约定 + `utils.NewTool(desc, invoke)` 泛型推断返回 JSON 形态）：

```go
	return utils.NewTool(desc, invoke)
```

---

### 9. `internal/store/sqlite/kpi_calls.go`（store：INSERT-only 审计）[新]

**Analog:** `internal/store/sqlite/import_batches.go` 全文件（175 行）——整文件复刻换列集。

**const SQL + 单 INSERT 写路径**（import_batches.go:30-39 + 70-89——全 `?` 占位、永不 fmt.Sprintf、时间 UTC RFC3339Nano、fmtError 包装）：

```go
// putImportBatchSQL is the single write path for the import_batches audit
// table (DATA-17 + D15-A4 + D15-C1). batch_id is the PRIMARY KEY; replays
// surface as constraint errors, never silent replacement (T-15-13).
const putImportBatchSQL = `
INSERT INTO import_batches (
    batch_id, dataset, period, tier, content_sha256, rules_sha256,
    raw_size_bytes, desensitized_row_count, discard_ts, supersedes_batch_id,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (s *SQLiteStore) PutImportBatch(ctx context.Context, row store.ImportBatchRow) error {
	_, err := s.db.ExecContext(ctx, putImportBatchSQL,
		row.BatchID, ...,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("PutImportBatch", err)
	}
	return nil
}
```

PutKPICall 照此（16-RESEARCH Pattern 6 的 11 列 DDL 提案：id AUTOINCREMENT / ts / session_id / skill_id / func / args_json / as_of_period / k_min_applied / result_snapshot / signature）。**无 UPDATE/DELETE 函数**；测试锁 TestKPICallsInsertOnly 镜像 TestImportBatchInsertOnly 形态（source-grep 断言 kpi_calls.go 无 UPDATE/DELETE 字面量）。查询面（admin 只读）：`ListKPICalls(func)` / `LatestKPICallByFunc` 照 getLatestBatchSQL/listImportBatchesSQL（:44-62）的 `ORDER BY created_at DESC LIMIT 1` 形态。

---

### 10. `internal/store/sqlite/schema.go` kpi_calls DDL + `internal/store/store.go` KPICallRow [改]

**Analog:** 自身三处先例——import_batches 建表块（:219-243）、dataset_deletes 的 AUTOINCREMENT 形态（:249-254）、索引-after-ALTER 时序契约（:346-353 + :490-496）。

**注释块写 schema rationale**（schema.go:219-230——"SEPARATE INSERT-only ... INTENTIONALLY DISTINCT from audit.action ... dedicated table keeps the SELECT lean" 叙事；kpi_calls 注释块照写：大 JSON 快照不混 chat 审计主表 + ADM-03 SQL 直查 + KPI-02 回放语义）：

```sql
-- import_batches (Phase 15 / Plan 15-02 / DATA-17): the SEPARATE
-- INSERT-only batch-audit table that proves raw data was discarded ...
-- This table is INTENTIONALLY DISTINCT from audit.action — batch-audit
-- queries are the hot path for the admin import view (ADM-02), and a
-- dedicated table keeps the SELECT lean and SQL-first.
--
-- INSERT-only discipline: rows are appended by PutImportBatch; there
-- is NO UPDATE / DELETE path. Replays of the same batch_id surface as
-- a PRIMARY KEY constraint error, never silent replacement.
CREATE TABLE IF NOT EXISTS import_batches (
    batch_id                TEXT PRIMARY KEY,
    ...
    created_at              TEXT NOT NULL
);
```

kpi_calls 是**全新表进 schemaSQL 尾部**即可（CREATE TABLE IF NOT EXISTS 幂等；**无需** ALTER/PRAGMA 门——那是给已有表加列用的）。三个索引（func / session_id / ts）照 `idx_import_batches_dataset_period`（:351-353）独立 const + migrate() 尾部 ExecContext。

**store.go 行结构 + 接口扩展**（ImportBatchRow :297-328 每字段一句语义注释 + 接口方法 :604-616 一句用途+调用方——KPICallRow 与 `PutKPICall(ctx, row) error` / `ListKPICalls` 照抄密度）：

```go
// ImportBatchRow is one INSERT-only audit record for a data-import batch
// (Phase 15 / Plan 15-02 / DATA-17). It carries the proof that raw data was
// discarded (DiscardTS), the rule-set hash applied (RulesSHA256, D15-A4),
// and the supersede chain (SupersedesBatchID, D15-C1).
type ImportBatchRow struct {
	// BatchID is the crypto/rand hex identifier of the batch.
	BatchID string
	...
}
```

---

### 12. `internal/skill/manifest.go` + `internal/skill/validate.go` + `internal/skill/tier.go`（reads 通道扩展）[改]

**Analog:** 自身——validate.go 的 reads 校验段（:72-91）+ writes known-target 段（:129-145）+ manifest.go 的 validAgentTypes 词表（:160-173）+ tier.go ResolveTiers（:120-148）。

**XOR 校验插入点**（validate.go:72-91 现状——3 号 reads 校验与 3a ResolveTiers 委托之间就是 D16-C2 分支落点；错误文案照 writes 段的「指名冲突值 + 指向唯一真源」风格）：

```go
	// 3. reads — non-empty (default-deny), no "*" wildcard (P-SKILL-3)
	if len(m.Reads) == 0 {
		return fmt.Errorf(
			"reads is required and must be non-empty (default-deny: P-SKILL-3 / SYSK-07)",
		)
	}
	for _, r := range m.Reads {
		if r == "*" { ... }
	}
	// 3a. reads — every top-level dir must be in TierEnum (SYSK-07 +
	// D2-12). Delegate to ResolveTiers so there is exactly ONE tier-enum
	// source of truth in this package.
	if _, err := ResolveTiers(m.Reads); err != nil {
		return fmt.Errorf("reads: %w", err)
	}
```

错误文案镜像对象（validate.go:135-145——点名冲突值 + 列合法集 + "do NOT add it here, update ... instead" 指向唯一真源；XOR 报错照此点名 `reads` 全集 + D16-C1 锚 + deferred 提示）：

```go
	known := hitl.KnownTargets()
	for _, w := range m.Writes {
		if !containsString(known, w) {
			return fmt.Errorf(
				"writes contains unknown target %q (known targets: %v); "+
					"the target must appear in hitl.AllowedWriteTargets — "+
					"do NOT add it here, update internal/agent/hitl/registry.go instead (D11-B3 shared source)",
				w, known,
			)
		}
	}
```

**词表/通道标记形态**（manifest.go:160-173——`validReadPatterns` 或 `ReadDataAggregatedOnly` 常量 + ChannelKind 判定方法照 validAgentTypes 的「hard filter + 运行时 defense-in-depth 复查」注释叙事；Manifest 上加 `HasDataChannel()` 方法而非新 yaml 字段——reads 值域扩展，schema 不动）。

**ResolveTiers 前置分支**（tier.go:126-135 现状——`"data:aggregated_only"` 无 `/`，top = 整串撞 TierEnum 必报错；16-RESEARCH Pattern 4 定案：**在 Validate 调 ResolveTiers 之前分支**（data 通道跳过 3a），或在 ResolveTiers 首行识别 `data:` 前缀返回空 tiers——planner 选一，测试 TestChannelGuardXOR 两条路径都锁）：

```go
	for _, r := range reads {
		top := r
		if i := strings.Index(r, "/"); i >= 0 {
			top = r[:i]
		}
		tier, ok := TierEnum[top]
		if !ok {
			return nil, fmt.Errorf(
				"reads %q: unknown tier dir %q (known: %v)",
				r, top, knownTierDirs(),
			)
		}
```

**热重载窗口自动覆盖**（Pitfall 6）：XOR 校验进 `Validate()` 本体——`internal/skill/loader.go:47-51`（"the per-file parse+validate+tiers body was extracted into the unexported helper parseAndValidate so the new Registry.reloadOne can reuse the same logic"）+ `internal/skill/registry.go:84`（"V12 ASVS symlink guard runs on every reloadOne (parseAndValidate ...)"）证实 reloadOne 每次重跑 parseAndValidate → 加载期守卫零额外代码覆盖热重载。**校验绝不能写在 Scan 循环里**。

---

### 13. `internal/agent/runner_setup.go` BuildRunner 通道分支 + `internal/agent/hitl_tools.go`（D16-C3 运行时层）[改]

**Analog:** 自身 runner_setup.go:176-201——D16-C3 震中（见文首修正 1）。

**现状装配段**（runner_setup.go:191-201——注释原文 "Always include the read-only retrieve Tool (it is not a write Tool and is not gated by the writes: frontmatter)"——这句话对 data 通道 Skill 变为**反语义**，通道分支替换此段）：

```go
	tools, err := BuildToolsForSkill(ctx, cfg.Manifest, cfg.SessionID, cfg.SkillID, ruleMatcher)
	if err != nil {
		return nil, fmt.Errorf("agent.BuildRunner: BuildToolsForSkill: %w", err)
	}

	// Always include the read-only retrieve Tool (it is not a write
	// Tool and is not gated by the writes: frontmatter). The Tool
	// closes over the manifest's resolved tier slice (KNOW-03 — the
	// LLM cannot override tiers via Tool arguments).
	retrieveTool := agenttools.BuildRetrieveTool(cfg.Embedder, cfg.Retriever, cfg.Manifest)
	tools = append(tools, retrieveTool)
```

改后形态（16-RESEARCH Pattern 4 骨架）：`manifest.HasDataChannel()` → tools = {kpiCallTool}（**不含** retrieve、**不**跑 BuildToolsForSkill 的 writes 交集——data 通道 v1 无写工具）；else → 现状两段不变。**BuildRunner 签名需要新 deps**（kpi registry/executor 经 RunnerBuildConfig 传入，nil 时 data 通道 fail 报错——镜像 :157-174 的 fail-fast 前置校验群）。

**pure-function whitelist 的镜像叙事**（hitl_tools.go:31-36——"read-only Skills see ZERO write Tool schemas ... regardless of how the LLM is prompted or what the user pastes into the chat"，D16-C3 的通道版注释照此力度写；hitl_tools.go 本体大概率不动——通道分支在它的调用侧，除非 planner 选择把分支收进 BuildToolsForSkill）：

```go
// BuildToolsForSkill is the D11-B1 pure-function whitelist recompute.
// Per turn (per Runner construction), the LLM-visible Tool list is
// intersection(AllowedWriteTargets, manifest.Writes) — read-only
// Skills see ZERO write Tool schemas (HITL-05 load-bearing invariant:
// a Skill with writes:[] has no way to reach a write Tool, regardless
// of how the LLM is prompted or what the user pastes into the chat).
```

**测试形态锚**（hitl_tools_test.go:148-153——readOnly vs writey 双 manifest 对比断言；TestToolsForChannel 照此写 data vs tier 双 manifest：`names(tools)` 集合相等断言）。

**两处次要装配点**（planner 终裁）：
- `internal/agent/agent.go:224`（USE_AGENT_PATH=true 实验路径）——同步分支或声明实验路径仅支持 tier（注释写明）。
- `internal/runtime/deep_agent.go:171-174`——deep archetype 系统提示词硬编码提及 `retrieve_compliance_chunks`；data 通道 Skill 走 chat archetype 即不受影响，deep+kpi 组合 v1 不支持（在 validate.go 或 dispatch 层拒，planner 定——建议 validate 层拒，fail-closed 传统）。

---

### 14. `internal/agent/callbacks.go` 工具名分流 [改]

**Analog:** 自身 toolHandler.OnStart :138-148——Pitfall 5 误标源。

**现状（误标源）**（callbacks.go:138-148——Tool OnStart 无条件发 DecisionRetrieve；注释 :44-46 原文 "**retrieve** — Tool OnStart fired (the agent invoked retrieve_compliance_chunks on this hop)"——Phase 3 单工具时代的假设）：

```go
	toolHandler := &template.ToolCallbackHandler{
		OnStart: func(ctx context.Context, _ *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			sc.inc()
			query := ""
			if input != nil {
				query = tryParseQuery(input.ArgumentsInJSON)
			}
			emit(sc.current(), DecisionRetrieve, "", query, 0.0)
			return ctx
		},
	}
```

改法（16-RESEARCH Open Q4 推荐）：OnStart 按 `RunInfo` 的工具名分流——`kpi_call` → 发 `DecisionKPIReference`（不发 retrieve）；其余不变。**与 DataRef 发射同任务交付**（Pitfall 5 明示：回调分流与 data-reference part 发射是同一耦合点）。DataRef 快照字段从工具结果传递：`DecisionEmitFunc` 签名（agent.go:135）只有 5 个标量参数——planner 需扩展签名或闭包直发（扩签名会动 server 侧 emitDecision 包装 `internal/server/runner_setup.go:192-198` runnerEmitDecision——它是现成发射 seam，见下条）。

---

### 15. `internal/wire/types.go` + `internal/wire/chunk.go` + `web/src/types/wire-types.ts`（data-reference part）[改]

**Analog:** 自身——types.go DecisionFanout* 常量（:78-111）+ chunk.go DecisionPayload 的 P13-L7 additive 字段（:157-170）+ TS 镜像常量群（wire-types.ts:65-83）。

**Decision 枚举 additive 先例**（types.go:78-111——注释原文 "The discriminator (kebab-case data-decision) is unchanged. The DecisionPayload.Decision string enum widens by 3 literals — same D16 overload pattern"；`DecisionKPIReference = "kpi_reference"` 照此追加 + 同密度注释）：

```go
	// ... The TS mirror at
	// web/src/types/wire-types.ts adds the matching 3 string literals
	// under D15 lockstep.
	DecisionFanoutDispatched = "fanout_dispatched"
	DecisionFanoutCompleted  = "fanout_completed"
	DecisionFanoutFailed     = "fanout_failed"
```

**Payload additive 字段先例**（chunk.go:160-162——RunPath/TransferFrom/TransferTo 就是 P13-L7 这么加的；`DataRef *DataReference json:"dataRef,omitempty"` 照此追加到 DecisionPayload 尾部，现有字段一行不动）：

```go
type DecisionPayload struct {
	Step           int             `json:"step"`
	Decision       string          `json:"decision"`
	RunPath        string          `json:"runPath,omitempty"`      // P13-L7 — fanout_* events only; slash-separated agent path
	TransferFrom   string          `json:"transferFrom,omitempty"` // P13-L7 — fanout_* events only; parent run ID
	TransferTo     string          `json:"transferTo,omitempty"`   // P13-L7 — fanout_* events only; sub_query_id
	...
}
```

DataReference 结构照 16-RESEARCH Pattern 7 定义（Func/Args/ASOfPeriod/ASOfTs/RowCount/Suppressed/KMinApplied——rows 本体不进 SSE）。

**发射 seam**（internal/server/runner_setup.go:192-198——runnerEmitDecision 是现成的 per-request data-decision 发射点，data-reference part 经它或同形态闭包发；`wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})`）：

```go
func runnerEmitDecision(c *app.RequestContext, p wire.DecisionPayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal decision: %w", err)
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})
}
```

**TS 镜像**（wire-types.ts:78-83——`export const DecisionFanoutDispatched = 'fanout_dispatched' as const;` + 注释 "only the decision STRING enum widinds"；`DecisionKPIReference` 常量 + DecisionPayload 接口加 `dataRef?` 字段，D15 lockstep 同 commit；FE 渲染本 phase 不做）。

**ValidTypes 红线**：chunk.go:355-383 的 map **一行不动**（不扩 union）；chunk_test.go 计数锁自动保持。

---

### 16. `cmd/server/main.go` KPI loader 装配 [改]

**Analog:** 自身 datarules 两段——env 群（:186-195）+ importer 装配（:301-323）。

**env 群形态**（main.go:186-195——注释块写 fail-closed 语义 + envOr 默认值；`SKILLFORGE_KPIS_DIR`（默认 `kpis`）照此）：

```go
	// Phase 15 — data-import pipeline configuration.
	// SKILLFORGE_DATA_SALT is the L1 HMAC site salt. Unset → import endpoints
	// are disabled (503 fail-closed), but the rest of the server starts.
	dataSalt := envOr("SKILLFORGE_DATA_SALT", "")
	importMaxBody := parseBytesEnvOrDefault("SKILLFORGE_IMPORT_MAX_BODY_BYTES", server.ImportMaxBodyDefault)
	nonImportMaxBody := parseBytesEnvOrDefault("SKILLFORGE_MAX_BODY_BYTES", server.NonImportMaxBodyDefault)
	datarulesDir := envOr("SKILLFORGE_DATARULES_DIR", "datarules")
```

**装配段形态**（main.go:301-323——[OK]/[WARN]/[FAIL] 日志纪律 + fail-closed 分支；KPI loader 装配照此：LoadKPIs 失败 = `log.Fatalf`（受控文件坏 = 服务不起，与 Skill Scan :198-200 同姿态）；Layer 2 校验需要 duckdb 连接——与 importer 共用 :memory: Connector/AttachLRU 或独立开（planner 定，注意 hermetic 启动：Layer 2 依赖 CGO 在 Windows 本地跑不了——`go run cmd/server` 在 Windows 的行为要在 plan 里写明，或 Layer 2 延后到首个 data Skill 加载时执行（bootstrap 窗口语义配套，Pitfall 2））：

```go
	var importer *data.Importer
	if dataSalt != "" {
		ddb, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
		...
		importer = data.NewImporter(data.ImporterConfig{
			SiteSalt: dataSalt,
			RulesDir: datarulesDir,
		}, st, lru)
		log.Printf("[OK] cmd/server: import pipeline wired (salt present, rules=%s)", datarulesDir)
	} else {
		log.Printf("[WARN] cmd/server: SKILLFORGE_DATA_SALT unset — import endpoints disabled (fail-closed)")
	}
```

**admin 只读端点（可选，12 号文件）**：GET `/api/admin/kpis` 照 platform_prompt.go:52-77 形态（`isAdminRequest` 门 + 403/404 + JSON envelope——15-PATTERNS §15 已引证全文）；路由注册照 server.go 路由表「按 phase 分组注释块 + 每路由一行」形态（15-PATTERNS §14 引证）。

---

### 17. `scripts/verify-phase-16.sh` + `Makefile` [新/改]

**Analog:** `scripts/verify-phase-15.sh`（15-PATTERNS §18 已全文引证 run_stage helper / gcc 前置 / WSL 警示 / source-grep 段形态）。

Phase 16 版差异段：internal/kpi 分包套件（hermetic 段 Windows 可跑 + WSL 段探针/executor/回放）、`TestNoSQLGenerationSurface` 结构禁令门（forbidden 关键字 `SELECT `/`INSERT `/`ATTACH `/`DETACH `/`CREATE TABLE` 对 `internal/agent` + `internal/server`（chat 路径）扫描，`_test.go` 跳过——16-RESEARCH Pattern 3 骨架）、source-grep 锁新条目（KMinDefault 消费引用 / `data:aggregated_only` 常量唯一真源 / DecisionKPIReference 字面量）。WalkDir 门形态母版：`integration/contract/platform_floor_bypass_test.go`（15-PATTERNS §17 引证：scan roots + skip dirs + `//go:build contract` tag + 门文件自跳过）。

---

## Shared Patterns

### A. 受控 YAML + fail-closed loader（第四次复刻）
**Source:** `internal/data/rules.go:85-108`（LoadRules：Lstat symlink guard → ReadFile → sha256 → Unmarshal → Validate）+ `internal/skill/loader.go:42-45`（"Validation errors are NOT silently swallowed"）
**Apply to:** internal/kpi/loader.go 全部加载路径 + kpis/*.yaml 校验。首错短路 + 错误带文件路径 + 指名字段/规则/决策锚；目录加载绝不部分加载。

### B. D11-B1/B3 双层执法 → reads 维度镜像（D16-C2/C3）
**Source:** `internal/skill/validate.go:129-145`（加载期词表拒绝，错误指向唯一真源）+ `internal/agent/hitl_tools.go:90-167`（运行时 pure-function 白名单交集）+ `internal/agent/runner_setup.go:191-201`（装配点）
**Apply to:** manifest validReadPatterns（加载期 XOR）+ BuildRunner 通道分支（运行时拼 Tool 集）。测试形态：hitl_tools_test.go:148-153 双 manifest 对比。

### C. INSERT-only dedicated 审计表（T-3-AUDIT）
**Source:** `internal/store/sqlite/import_batches.go`（const SQL 全 `?` + 单 INSERT + fmtError）+ `internal/store/sqlite/schema.go:219-243`（注释块写 rationale + PRIMARY KEY 防重放）
**Apply to:** kpi_calls.go + schema.go DDL + store.go KPICallRow。无 UPDATE/DELETE 函数；source-grep 测试锁。

### D. eino 工具闭包（一个闭包一个通道）
**Source:** `internal/agent/tools/retrieve.go:113-173`（构造器 panic 前置 + 闭包捕获不可变快照 + ToolInfo/ParamsOneOf + utils.NewTool）
**Apply to:** kpi_tool.go。闭包捕获 registry/executor/catalog 编译期快照（对应 retrieve 捕获 ResolvedTiers 的 KNOW-03 叙事）；结构化拒答走工具结果不走 Go error。

### E. wire additive（不扩 union）
**Source:** `internal/wire/types.go:78-111`（Decision 枚举 additive 常量）+ `internal/wire/chunk.go:157-170`（DecisionPayload additive 字段，P13-L7）+ `web/src/types/wire-types.ts:65-83`（TS lockstep）+ `internal/server/runner_setup.go:192-198`（runnerEmitDecision 发射 seam）
**Apply to:** DecisionKPIReference + DataRef 字段 + TS 镜像同 commit。ValidTypes map 一行不动；「never repurpose an existing value」纪律照抄注释。

### F. 窄接口隔离（interface isolation）
**Source:** `internal/duckdb/staging.go:59-65`（DatasetRegistry——只依赖所需操作不依赖整个 store.Store）
**Apply to:** executor 的 `LatestPeriod(ctx, dataset)` 依赖接口；kpi 包不 import store 全接口。

### G. WalkDir 结构性禁令门（D8-04 系）
**Source:** `integration/contract/platform_floor_bypass_test.go`（15-PATTERNS §17 引证：scan roots + skip dirs + `//go:build contract` + 门文件自跳过）
**Apply to:** TestNoSQLGenerationSurface——agent 可达包（internal/agent、internal/server chat 层）禁 SQL DML/DQL 字面量；SQL 只允许存在于 kpis/ 定义文件、internal/kpi、internal/duckdb、internal/store 与 _test.go。

### H. fmtError 包装 + sentinel error
**Source:** `internal/data/rules.go:88`（`fmtError("LoadRules", ...)`——`data.%s: %w` 形态）+ `internal/duckdb/staging.go:57,70`（ErrDatasetExists sentinel）
**Apply to:** internal/kpi 全部新文件（`kpi.%s: %w` 形态）。no_matching_kpi **不是** sentinel error——它是工具层 JSON 结果（Anti-Pattern 明示）。

### I. 结构性常量 source-grep 锁
**Source:** `internal/data/desensitize.go:33-39`（KMinDefault——"The literal is the call-site source-grep target ... NOT a config knob"）
**Apply to:** k_min 取值链常量引用、max_rows 默认上限、catalog 截断阈值 30——全部字面量进代码 + verify-phase-16.sh grep 段锁定。

### J. 平台铁律与结构互证（叙事锚）
**Source:** `internal/platformfloor/platform_floor.go:100-112`——铁律①「永不编写、执行或建议 SQL；只按名调用已注册 KPI」/ ③「数据结论必须附带 as_of 与数据引用」/ ⑦「永不声称拥有实时数据；只报告 as_of」
**Apply to:** kpi_call 行为验收口径（as_of 系统生成 = 铁律⑦结构性落地；通道守卫 = 铁律①结构性落地）。plan 的验收叙事直接引用这三条原文，不改 platformfloor 文案（Phase 16 不动该文件——文案与结构本就自洽）。

---

## No Analog Found

| 文件 | Role | Data Flow | 原因与替代 |
|------|------|-----------|------------|
| `internal/kpi/validate_sql.go` | service | transform | 仓内零 SQL parser（grep 实测；铁律①本就排除 agent 侧 SQL 面）。用 16-RESEARCH §Code Examples 的 envelope/AST 骨架 + 四条 fail-closed 规则；**Wave 0 探针（TestProbeJSONSerializeSQL，WSL）先 lock A1/A2/A3 真实行为再写走查逻辑**——join 树 JSON 形态（A3）必须用含 JOIN 的 SQL 打印真实 AST 后再定递归函数。 |
| `internal/kpi/executor.go` 执行段 | service | request-response | 仓内无「预编译 SQL + sql.Named 参数化查询」生产先例（importer 是 Appender 写路径，不是查询路径）。go-duckdb `$foo` + `sql.Named` 形态已在 module cache statement_test.go:207-218 实测（16-RESEARCH 引证）；AttachLRU session 复用是现成执行面。 |
| catalog 生成 + kpi_call description 注入 | utility | transform | 仓内无「启动期把 registry 目录编译进工具 description」先例。16-RESEARCH Pattern 2 给出口径（字母序 / 每行 name — description（参数摘要）/ 30 条截断）；铁律：**编译期快照进闭包，运行时零拼接**（Don't Hand-Roll 表）。 |

## Metadata

**Analog search scope:** internal/kpi（不存在，grep 证空）、internal/data、internal/skill、internal/agent（+tools/hitl）、internal/store（+sqlite）、internal/wire、internal/duckdb、internal/server、internal/runtime、internal/platformfloor、cmd/server、web/src/types、scripts、integration/contract
**Files scanned（本 session 精读）:** rules.go / loans.yaml(datarules) / retrieve.go / hitl_tools.go / hitl/registry.go / import_batches.go / manifest.go / validate.go / loader.go / tier.go / registry.go(grep 定位) / chunk.go / types.go / agent.go(:120-300) / runner_setup.go(agent :130-236 + server :180-207) / callbacks.go / schema.go(:219-255 + 索引段) / store.go(grep 定位) / desensitize.go(:1-70) / attach_lru.go(:1-130) / staging.go(:25-105) / platform_floor.go(:100-112) / main.go(:186-323) / wire-types.ts(:58-83)
**Pattern extraction date:** 2026-08-24
**关键提醒（planner 必读）:**
① **D16-C3 震中是 `internal/agent/runner_setup.go:196-201`**（生产 BuildRunner），不是 RESEARCH 写的 agent.go:180-256（那是 USE_AGENT_PATH 实验路径，main.go:271-288 gate 证实）——见文首修正 1；
② **TestNoSQLGenerationSurface 的 forbidden 列表要先跑一遍现状基线**——`internal/runtime/deep_agent.go:171-174` 的提示词含 "retrieve" 字样但不含 SQL 关键字（已核），`internal/server` 是否已有 SQL 字面量需 plan 时实测（audit.go 的 SELECT 查询在 store 层不在 server 层，预期干净；若 server 层有则 scan roots 排除法调整）；
③ **callbacks.go 分流与 DataRef 发射必须同任务**（Pitfall 5 耦合点），且 DecisionEmitFunc 5 标量签名（agent.go:135）装不下 DataRef——扩签名会波及 server 侧 emitDecision/runnerEmitDecision 包装，planner 在该任务里一并处理；
④ **Layer 2 校验的启动时序**（Pitfall 2）：json_serialize_sql 需要 duckdb 连接（CGO/WSL），Windows 本地 `go run cmd/server` 起不动 Layer 2——bootstrap 窗口 + 延迟校验或 hermetic 降级形态必须在 plan 里显式决策（16-RESEARCH Open Q1/Q2 的推荐可直接采纳）；
⑤ **deep archetype 提示词硬编码 retrieve**（deep_agent.go:171-174）——data 通道 Skill + deep 组合 v1 应在 validate 层拒（fail-closed 传统），避免提示词承诺与工具集不符；
⑥ kpis/ 目录与 datarules/ 平级放仓库根（.gitignore:27-28 只忽略 data/，kpis/ 不受影响——15-PATTERNS §1 目录事实已核）；
⑦ 测试 hermetic/WSL 分类照 16-RESEARCH Validation 节执行——kpi 包 80% 校验分支（Layer 1/XOR/wire/DDL/no_matching/enum/kmin 纯函数/as_of stub）Windows 直跑，executor/快照回放/跨 Skill 一致性/probe 五类 WSL。
