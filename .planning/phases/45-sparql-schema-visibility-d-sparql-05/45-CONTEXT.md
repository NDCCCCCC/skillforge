# Phase 45: SPARQL Schema Visibility (D-SPARQL-05) - Context

**Gathered:** 2026-10-03
**Status:** Ready for planning
**Milestone:** v1.6 — SPARQL Schema Detection + Entity SHACL Shapes
**Origin:** Phase 44 D-SPARQL-05 defer（44-CONTEXT `<deferred>` → v1.6 首 phase 落地）

<domain>
## Phase Boundary

把 `internal/ontology/store/store.go` 的 `isSchemaTriple` 切换为 subject-based detection：skill 个体（`rdf:type skf:Skill` 的 subject）及其 `hasReadAccess` triples 在 skill-scoped 视角（`/api/sparql` 带 X-Skill-Name、MCP tool call）升格为 schema 层免过滤——当前它们被 D-TIER-03 fail-closed 过滤（44-RESEARCH Path B 推导：skill_sql 不在 `visibleSubjects` → hasReadAccess EXCLUDED）。同时 D-TIER-03 fail-closed 语义零回退：无 `belongsToTier` 的 ABox 实体（note_003 类）仍被过滤，Phase 40/44 既有回归全绿。

**Requirements:** SPARQL-01（subject-based detection）、SPARQL-02（fail-closed 不回退）、SPARQL-03（判定单点化）。

**Roadmap SC1 措辞澄清（代码事实）：** ROADMAP SC1 说 hasReadAccess "此前对 admin baseline 不可见"——实际 nil-tier `QueryScoped` early-return 直达 `s.Query()`（store.go:492-516），不经 `isSchemaTriple`，admin baseline 当前已可见全部 triples。真实受益者是 **skill-scoped 视角** + schema 判定语义统一。本 CONTEXT 以代码事实为准。

**In scope:** store.go 的 isSchemaTriple/QueryScoped 改造 + 回归测试（admin fixture 计数 + store 单元层）+ emitter conformance 测试 + SC3 parity 测试 + verify-phase-45.sh。
**Out of scope:** SPARQL 1.1 parser（Phase 41 backlog）、D-SPARQL-07 comment hardening（v1.6 Future SPARQL-F01）、前端改动、Phase 46 SHACL 工作、emitter 谓词扩充（allowlist 演进属未来决策）。

</domain>

<decisions>
## Implementation Decisions

### 判定范围（Area 2 — 根决策）
- **D-45-01（β — subject ∩ predicate allowlist）:** `isSchemaTriple` 新语义 = 保留现有 `rdf:type`+`schemaClasses[O]` 臂 + OWL 四谓词臂（domain/range/subClassOf/subPropertyOf），新增：`schemaSubjects[t.S] && metaPredicates[t.P]`。`schemaSubjects` = 携带 `rdf:type → schemaClasses` triple 的 subject 集合；`metaPredicates` allowlist = emitter 实际发出的 13 个 schema 个体元数据谓词：`skillName`、`skillDescription`、`skillTemperature`、`poweredBy`、`hasReadAccess`、`hasWriteAccess`、`requiresRefusal`、`tierName`、`categoryName`、`subCategoryName`、`agentTypeName`、`refusalPhraseText`、`belongsToTier`——**明确排除 `label`/`comment`**（D-SPARQL-03 裁决保持：schema 个体的 label/comment 在 tier 视角仍不可见）。
  - **否决字面 α 的原因：** ROADMAP SC1 括号里的 Option α（subject ∈ schemaSubjects → 全部 triples 免过滤）会打破 Phase 44 两个 pinned 测试（`store_label_passthrough_test.go:64`、`:175-208`），即隐性推翻 D-SPARQL-03 既有决策。β 在不动任何 pinned fail-closed 决策的前提下达成 SC1。
  - **防漂移：** 新增 emitter conformance 测试——emitter 产出谓词 ⊆ allowlist；emitter render() 新增谓词而 allowlist 滞后时测试红灯（变静默半可见为显式失败）。
  - **泄漏面上界：** emitter bug 误发 `rdf:type skf:Skill` 的 ABox 实体最多泄 allowlist 谓词；NoteChunk/TableSchema 按构造不入 schemaSubjects（schemaClasses 注释 + hermetic gate 双重锁定），SPARQL-02 无虞。

### schemaSubjects 生命周期（Area 1）
- **D-45-02（查询时派生，无持久状态）:** `schemaSubjects` 不作为 Store 持久字段——`QueryScoped` 入口每次调用从 `byP[rdfType]` 派生局部集合（<10k triples 下数百次 map lookup，sub-ms，低于既有 `s.all` 全表扫描成本）。
  - **否决 eager "Insert 时填充" 的原因：** ROADMAP/44-RESEARCH 字面方案存在已验证的覆盖缺口——snapshot boot 的 TBox merge 走 `InsertUnique`（restore.go:86）、RollbackTBox 走 `DeleteSubject`（induction.go:345-353）、SHACL quarantine 是活跃路径（main.go:937-941 + shapes.ttl 显式 target schema 类）。4-mutator hook 漏一即静默 SPARQL-02 违规，且 fresh-store 测试全绿、仅 snapshot-present boot 复现（本仓库已踩过的派生态漂移模式，参照 Milvus reseed 写放大）。
  - **对 ROADMAP SC1 括号文字的偏离说明：** "schemaSubjects 集合 Insert 时填充" 是实现策略描述；查询时派生语义等价且构造性消除漂移。planner 应在 PLAN.md 记录此偏离 rationale。
  - 写路径零改动：InsertUnique / DeleteSubject / Quarantine / rollback / LoadSnapshot（重放 Insert）全部自动正确。

### SC3 单点化形态（Area 3）
- **D-45-03（判定单点已达成 + parity 测试）:** `isSchemaTriple` 已是全仓唯一实现（store.go:604），tier 路径单一调用点（store.go:550）。**不改查询路径**：nil-tier early-return（store.go:492-516）与 admin 无 skill 的裸 `Query()`（admin_ontology.go:218）维持 D-TIER-01 "admin baseline = 全可见" 刻意语义——判定对 nil 路径恒为 no-op，统一入口的"演化自动覆盖"收益接近零，却有 dedup 语义变化扰动 Phase 44 计数敏感 fixture 的真实风险。
  - **SC3 验证形态：** 端到端 parity 测试——同一 fixture 下 `/api/sparql`（X-Skill-Name）与 `/api/admin` KG query 对 schema triple 的判定一致；SC3 达成以测试断言为准。
  - isSchemaTriple 的现实演化方向（增删 schemaClasses/allowlist 条目）自动同时作用于两个端点（均经 tier 路径单一调用点）。

### 断言更新与回归范围（Area 4）
- **D-45-04（双层回归）:**
  - **admin 层**（admin_platform_test.go）：`TestSparqlQueryScoped_TierFilter` 15→17（10 schema + 7 ABox）+ 新增正向断言：结果含 `(skill_sql, hasReadAccess, sql-guides)`；`TestSparqlQueryScoped_FailClosedNoBelongsToTier` Path B 8→10（isSchemaTriple 升格落点：schema passthrough 先于 S/P/O 过滤，store.go:538-554）+ 新增正向 hasReadAccess 断言。**负向断言（note_002/note_003 不可见）不变——红线。** fixtureKG godoc 块（Path B 计数推导）重写。
  - **store 层**（新文件，建议 `store_schema_subject_test.go`）：一测双断言——restrictive tier 下 skill 个体 `hasReadAccess` 可见 **且** 同 subject `label` 仍不可见（同点锁定 D-SPARQL-05 正向 + D-SPARQL-03 优先级红线）。
  - **Path A（nil-tier）断言全部不变**（22/4/1 等）——early-return 不经 isSchemaTriple。
  - **既有红线清单（一个都不能动）：** `TestStore_SchemaIndividualLabelTierVisibility`、`TestStore_LabelAndCommentNotSchemaPassthrough`、`TestIsSchemaTriple_LabelAndCommentAreABox` 既有断言、Phase 40 store 套件（store_test.go:592-744）note_*/tbl_* 计数。
  - **verify-phase-45.sh（新建）：** WSL `CGO_ENABLED=1`：build+vet → `go test ./internal/ontology/store/...` → `go test ./internal/server/ -run "TestSparqlQueryScoped|TestHybridSearchScoped"` → 全量 `go test ./internal/...`；fail-fast。store 包纯 Go 但 internal/server 测试经 sqlite→mattn 驱动必须 CGO。
- **D-45-05（rationale 政策）:** D-SPARQL-08 政策延续，但 45 的计数变化是**预期行为变更**（非 test-author miscount）——godoc rationale 引用 D-SPARQL-05 / 本 CONTEXT 决策编号，不写 "miscount"。

### Claude's Discretion
- schemaSubjects 派生的具体形态（QueryScoped 内联 vs helper 函数）。
- 新测试文件命名与 conformance 测试的断言写法（emitter 谓词枚举方式）。
- parity 测试落点（store 级 + handler 级两层均可，研究者建议至少 handler 级端到端）。
- emitter 谓词清单的常量定义位置（store.go 与 emitter.go 的归属——allowlist 判定在 store，枚举真源在 emitter，由 conformance 测试钉住）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 44 来源与推演（本 phase 的直接前置）
- `.planning/phases/44-sparql-test-fix/44-RESEARCH.md` §2/§5/§6 — fixture 22-triple 逐行审计、Option α/β 原文、Decision locks；Path B 数学是 45 断言更新的基线
- `.planning/phases/44-sparql-test-fix/44-CONTEXT.md` — D-SPARQL-01..08 全部锁定决策；`<deferred>` 是本 phase 来源
- `.planning/phases/44-sparql-test-fix/44-01-PLAN.md:459-473` — WSL CGO 命令链（verify-phase-45.sh 的运行命令模板）

### 信任语义（LOCKED，不可削弱）
- `.planning/ROADMAP.md` — Phase 45 Goal/SC1-4 + Phase 40 D-TIER-01/03/06/07 定义（:196 附近）
- `.planning/REQUIREMENTS.md` — SPARQL-01/02/03 定义
- `internal/ontology/store/store.go:477-577` — QueryScoped 源码（early-return 492-516、schema passthrough 538-554）
- `internal/ontology/store/store.go:579-659` — isSchemaTriple 全文 docstring + schemaClasses
- `internal/ontology/store/store_test.go:569-744` — Phase 40 hermetic gate（必须保持绿）

### 实现落点（侦察确认的事实）
- `internal/server/admin_platform.go:178-292` — SparqlQueryScoped + /api/sparql handler（X-Skill-Name 强制、tiers==nil 即 401）
- `internal/server/admin_ontology.go:200-224` — /api/admin KG query handler（无 skill 走裸 Query()）
- `internal/server/platform/mcp/server.go:240-280` — MCP tool call 经 QueryScoped
- `internal/ontology/store/restore.go:86` — snapshot boot TBox merge 走 InsertUnique（D-45-02 的核心证据）
- `internal/knowledge/ecl/induction.go:345-353` — RollbackTBox 走 DeleteSubject
- `internal/ontology/emitter/`（emitter.go:223-318 render）— skill 个体 13 谓词的真源（allowlist 枚举来源）

### 既有测试资产（回归红线）
- `internal/server/admin_platform_test.go` — Phase 44 修复后 22-triple fixture + 各测试断言
- `internal/ontology/store/store_label_passthrough_test.go` — D-SPARQL-03/06 pinned 测试（:45/:133/:175-208 全部是红线）
- `verify-phase-41.sh` — verify 脚本分区 + fail-fast 模式参照

### 环境约束
- `env-wsl-backend-runbook`（memory）— WSL Go 构建；CGO_ENABLED=1 需 Linux gcc emutls ABI
- `env-intranet-service-flakiness`（memory）— 全套件 hermetic，无 live LLM/Milvus 依赖

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `schemaClasses` map（store.go:641-659）— 常量判定集合是既有模式（12 类硬编码），metaPredicates allowlist 沿用同一模式
- `byP[rdfType]` 索引 — schemaSubjects 查询时派生的数据源，O(桶大小)
- `LoadSnapshot` 重放 Insert（store.go:383-385）— snapshot 恢复在派生方案下零额外代码
- `store_label_passthrough_test.go` — store 包内新测试文件的组织先例（局部 testNS 常量、package store）

### Established Patterns
- 判定单点 + 调用点单点：isSchemaTriple 唯一实现、QueryScoped:550 唯一 tier 路径调用
- "路径差异 = 端点语义"：nil early-return（D-TIER-01）与 dedup 差异（D-SPARQL-02）都是注释明示的刻意设计
- 常量判定集合硬编码 + docstring 决策编号引用（D-TIER-03/D-SPARQL-03 均内嵌于 godoc）
- 断言计数变更配 godoc rationale 块（D-SPARQL-08 政策，45 延续但措辞为预期行为变更）

### Integration Points
- 唯一 prod 改动面：`internal/ontology/store/store.go`（isSchemaTriple + QueryScoped 入口派生）
- 测试改动面：admin_platform_test.go（2 处计数 + 正向断言 + godoc）、store 包新文件、emitter conformance 测试、verify-phase-45.sh
- 无前端改动、无 API contract 变化、无 DB schema 变化、无 emitter 行为变化（conformance 测试只读 emitter 产出）
- Hybrid Retrieval（admin_platform.go:388-473）经 QueryScoped 收益自动生效：skill 节点 label/tier 收集含 skill 元数据

</code_context>

<specifics>
## Specific Notes

- **实现落点已由决策组合唯一确定：** D-45-01（allowlist）+ D-45-02（查询时派生）→ isSchemaTriple 需要接收 schemaSubjects 参数（或 QueryScoped 内联判定）——FailClosed Path B 预期 8→10 由"schemas passthrough 先于 S/P/O 过滤"的现有循环结构决定（store.go:538-554）。planner 勿再引入 visibleSubjects 注入变体（它给出 8 不变，与 D-45-04 断言清单冲突）。
- **emitter 13 谓词清单**（emitter.go:223-318 静态可枚举）：skillName/skillDescription/skillTemperature/poweredBy/hasReadAccess/hasWriteAccess/requiresRefusal/tierName/categoryName/subCategoryName/agentTypeName/refusalPhraseText/belongsToTier。planner 实现时应以 emitter 代码逐行核对，勿直接信任本清单。
- **TierFilter 计数推导（45 后预期）**：17 = 10 schema（4 class defs + 4 individuals rdf:type + 2 hasReadAccess）+ 7 ABox（note_001×4 + tbl_payment_t×3）。skill 个体的 hasReadAccess 升格后，skill_sql/skill_compliance 除 rdf:type 外的全部边仅 hasReadAccess（在 allowlist 内）→ 全可见。
- **测试执行环境**：store 包可 CGO_ENABLED=0；internal/server 必须 WSL CGO_ENABLED=1（Windows cgo.exe exit 2）。verify-phase-45.sh 全程 WSL 包装。

</specifics>

<deferred>
## Deferred Ideas

- **schemaSubjects 持久缓存 + generation 计数器** — 仅当 KG 增长到 100k+ triples 使每次派生可测量时再考虑（当前被查询时派生严格支配）。
- **class-XOR 不变量（boot 时拦截误类型实体）** — Area 2 讨论中的 α 变体；若未来 emitter mis-type 成为现实问题再立项。
- **D-SPARQL-07 comment hardening** — 已在 REQUIREMENTS.md Future SPARQL-F01，不入本 phase。
- **SPARQL 1.1 parser** — Phase 41 backlog，v1.6+ 候选。

None of these arose as new scope creep — discussion stayed within phase boundary.

</deferred>

---

*Phase: 45-sparql-schema-visibility-d-sparql-05*
*Context gathered: 2026-10-03 via advisor-mode research (4 parallel researchers, code-verified)*
