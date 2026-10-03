# Phase 45: SPARQL Schema Visibility (D-SPARQL-05) - Research

**Researched:** 2026-10-03
**Domain:** Go in-memory RDF triple store tier-filtering (`internal/ontology/store/store.go`) + SPARQL/admin/MCP query surfaces (`internal/server`)
**Confidence:** HIGH (every CONTEXT file:line claim re-verified against current working tree at HEAD 24710bf; all count derivations re-derived line-by-line; environment probes executed)

## Summary

Phase 45 is a **single-function semantic change with a precisely calculable test blast radius**. The `isSchemaTriple` function (store.go:604) gains a fourth arm — `schemaSubjects[t.S] && metaPredicates[t.P]` — where `schemaSubjects` is derived at query time from `byP[rdfType]` (D-45-02) and `metaPredicates` is a 13-predicate allowlist (D-45-01). The only production call site is store.go:550 inside `QueryScoped` Path B; the nil-tier early-return (store.go:492-517) never consults it, so Path A counts are provably frozen. I re-derived both expected count changes from the actual 22-triple fixture: **TierFilter 15→17** and **FailClosed Path B 8→10**, with the +2 in both cases being the two `hasReadAccess` triples (skill_sql→sql-guides AND skill_compliance→authority — schema passthrough is tier-agnostic, which the planner must encode in the positive assertions).

All CONTEXT file:line claims were verified EXCEPT two corrections: (1) `RollbackTBox`'s DeleteSubject sweep is at `internal/ontology/induction/induction.go:345-353`, NOT `internal/knowledge/ecl/induction.go` (the CONTEXT path is wrong; line range is right); (2) the emitter's TBox class declarations emit `owl:Class` (OWL vocabulary IRI), not `skf:Class` — meaning the 44-DECISIONS claim "Production TBox emitter uses skf:Class" is factually wrong, and production class-def triples are currently invisible in tier views (pre-existing quirk; out of scope, but it constrains the conformance-test design). The 13-predicate allowlist was independently re-enumerated from emitter.go:219-318 and matches CONTEXT's list EXACTLY.

Three production-behavior nuances emerged that the fixture does not exercise and the planner must know: KBCategory/SubCategory metadata is **already visible today** via the ABox path (their `belongsToTier` puts them in `visibleSubjects` → D-TIER-06 all-edges); the real production delta is skill/SkillAgent/RefusalPhrase/tierName metadata; and `verify_main.go` (phase-41 standalone harness) is runtime-stale since Phase 44 but compiles and is NOT in the phase-45 gate.

**Primary recommendation:** Implement β exactly as locked — `metaPredicates` map next to `schemaClasses` (store.go:641 pattern), `schemaSubjectSet()` helper derived once per Path-B invocation, `isSchemaTriple(t Triple, schemaSubjects map[string]bool)` signature change with mechanical `nil`-arg updates at the 8 pinned-test call sites — then land the four-layer test plan (admin counts + positive assertions, store dual-assertion test, emitter conformance test, handler-level parity test) and a new fail-fast `verify-phase-45.sh` using the 44-01-PLAN WSL CGO command chain.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-45-01（β — subject ∩ predicate allowlist）:** `isSchemaTriple` 新语义 = 保留现有 `rdf:type`+`schemaClasses[O]` 臂 + OWL 四谓词臂（domain/range/subClassOf/subPropertyOf），新增：`schemaSubjects[t.S] && metaPredicates[t.P]`。`schemaSubjects` = 携带 `rdf:type → schemaClasses` triple 的 subject 集合；`metaPredicates` allowlist = emitter 实际发出的 13 个 schema 个体元数据谓词：`skillName`、`skillDescription`、`skillTemperature`、`poweredBy`、`hasReadAccess`、`hasWriteAccess`、`requiresRefusal`、`tierName`、`categoryName`、`subCategoryName`、`agentTypeName`、`refusalPhraseText`、`belongsToTier`——**明确排除 `label`/`comment`**（D-SPARQL-03 裁决保持：schema 个体的 label/comment 在 tier 视角仍不可见）。
  - 否决字面 α 的原因：Option α（subject ∈ schemaSubjects → 全部 triples 免过滤）会打破 Phase 44 两个 pinned 测试（`store_label_passthrough_test.go:64`、`:175-208`），即隐性推翻 D-SPARQL-03 既有决策。
  - 防漂移：新增 emitter conformance 测试——emitter 产出谓词 ⊆ allowlist；emitter render() 新增谓词而 allowlist 滞后时测试红灯。
  - 泄漏面上界：emitter bug 误发 `rdf:type skf:Skill` 的 ABox 实体最多泄 allowlist 谓词；NoteChunk/TableSchema 按构造不入 schemaSubjects。
- **D-45-02（查询时派生，无持久状态）:** `schemaSubjects` 不作为 Store 持久字段——`QueryScoped` 入口每次调用从 `byP[rdfType]` 派生局部集合（<10k triples 下 sub-ms）。写路径零改动：InsertUnique / DeleteSubject / Quarantine / rollback / LoadSnapshot（重放 Insert）全部自动正确。**planner 应在 PLAN.md 记录对 ROADMAP SC1 括号文字（"Insert 时填充"）的偏离 rationale。**
- **D-45-03（判定单点已达成 + parity 测试）:** `isSchemaTriple` 已是全仓唯一实现（store.go:604），tier 路径单一调用点（store.go:550）。**不改查询路径**：nil-tier early-return（store.go:492-516）与 admin 无 skill 的裸 `Query()`（admin_ontology.go:218）维持 D-TIER-01 语义。SC3 验证形态：端到端 parity 测试——同一 fixture 下 `/api/sparql`（X-Skill-Name）与 `/api/admin` KG query 对 schema triple 的判定一致；SC3 达成以测试断言为准。
- **D-45-04（双层回归）:**
  - admin 层（admin_platform_test.go）：`TestSparqlQueryScoped_TierFilter` 15→17 + 新增正向断言 `(skill_sql, hasReadAccess, sql-guides)`；`TestSparqlQueryScoped_FailClosedNoBelongsToTier` Path B 8→10 + 新增正向 hasReadAccess 断言。**负向断言（note_002/note_003 不可见）不变——红线。** fixtureKG godoc 块重写。
  - store 层（新文件，建议 `store_schema_subject_test.go`）：一测双断言——restrictive tier 下 skill 个体 `hasReadAccess` 可见 **且** 同 subject `label` 仍不可见。
  - Path A（nil-tier）断言全部不变（22/4/1 等）。
  - 既有红线（一个都不能动）：`TestStore_SchemaIndividualLabelTierVisibility`、`TestStore_LabelAndCommentNotSchemaPassthrough`、`TestIsSchemaTriple_LabelAndCommentAreABox` 既有断言、Phase 40 store 套件（store_test.go:592-744）note_*/tbl_* 计数。
  - verify-phase-45.sh（新建）：WSL `CGO_ENABLED=1`：build+vet → `go test ./internal/ontology/store/...` → `go test ./internal/server/ -run "TestSparqlQueryScoped|TestHybridSearchScoped"` → 全量 `go test ./internal/...`；fail-fast。
- **D-45-05（rationale 政策）:** 计数变化是**预期行为变更**（非 test-author miscount）——godoc rationale 引用 D-SPARQL-05 / D-45-04，不写 "miscount"。

### Claude's Discretion

- schemaSubjects 派生的具体形态（QueryScoped 内联 vs helper 函数）。
- 新测试文件命名与 conformance 测试的断言写法（emitter 谓词枚举方式）。
- parity 测试落点（store 级 + handler 级两层均可，研究者建议至少 handler 级端到端）。
- emitter 谓词清单的常量定义位置（allowlist 判定在 store，枚举真源在 emitter，由 conformance 测试钉住）。

### Deferred Ideas (OUT OF SCOPE)

- schemaSubjects 持久缓存 + generation 计数器（仅当 KG 100k+ triples）。
- class-XOR 不变量（boot 时拦截误类型实体）。
- D-SPARQL-07 comment hardening（REQUIREMENTS Future SPARQL-F01）。
- SPARQL 1.1 parser（Phase 41 backlog）。

**Out of scope (phase boundary):** SPARQL 1.1 parser、D-SPARQL-07 comment hardening、前端改动、Phase 46 SHACL 工作、emitter 谓词扩充（allowlist 演进属未来决策）。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SPARQL-01 | `isSchemaTriple` 切换为 subject-based detection——skill 个体及其 `hasReadAccess` triples 可见 | β 第四臂实现配方（Architecture Patterns §Pattern 1）+ fixture 计数推导（17/10 已逐 triple 验证）+ store 层双断言测试设计 |
| SPARQL-02 | D-TIER-03 fail-closed 不回退：无 `belongsToTier` 的 ABox 实体仍被过滤，既有 tier 回归全绿 | β 构造性保证：NoteChunk/TableSchema ∉ schemaClasses → ABox notes 永不入 schemaSubjects；schema passthrough 先于 pattern filter 的现有循环结构（store.go:543-554）使负向断言自然保持；红线测试清单已逐一核对在 β 下全绿 |
| SPARQL-03 | schema 可见性规则单点化——`/api/sparql` 与 `/api/admin` 共享同一判定 | grep 证实 `schemaClasses`/`isSchemaTriple` 仅存在于 store 包、唯一 tier 调用点 store.go:550；三个端点（/api/sparql、/api/admin kg/query、MCP skf_query）均汇入 QueryScoped；parity 测试配方（handler 级 ut.PerformRequest 模式已定位） |
</phase_requirements>

## Code-Verified Fact Base (CONTEXT claims audit)

Every claim re-verified against working tree at HEAD `24710bf`, 2026-10-03. This is the load-bearing section for planning.

### Verified as stated

| CONTEXT claim | Verdict | Evidence |
|---------------|---------|----------|
| QueryScoped 源码 store.go:477-577 | ✓ | nil-tier early-return 492-517（D-SPARQL-02 dedup 仅 all-wildcard 分支）；allowedTierSet 519-523；visibleSubjects 525-536（walk `byP[ns+"belongsToTier"]`，TrimPrefix 比较）；schema passthrough 538-554（**先于** visibleSubjects 检查、**先于** S/P/O pattern filter）；:550 唯一 isSchemaTriple 调用 |
| isSchemaTriple 全文 store.go:579-659 | ✓ | 实现 :604-633；schemaClasses :641-659（12 类硬编码 map，含注释 NoteChunk/TableSchema 排除） |
| admin_platform.go:178-292 SparqlQueryScoped + handler | ✓ | 共享函数 :189-252（validate→default 50/cap 500→QueryScoped→filter→paginate）；handler :260-292（X-Skill-Name 强制，nil tiers→401）；路由 server.go:704（NOT under /api/admin，无 X-Admin-Token） |
| admin_ontology.go:200-224 KG query handler | ✓ | :205-224；`skill` param → `resolveSkillTiers`（同文件姊妹 resolver）→ QueryScoped；无 skill → 裸 `Query()` :218（D-TIER-01）；路由 server.go:677（admin token 组） |
| MCP tool call 经 QueryScoped | ✓ | `internal/server/platform/mcp/server.go` handleQuery :231-291；:261 QueryScoped；skill 未找到 → 空 results（fail-closed） |
| restore.go:86 snapshot boot TBox merge 走 InsertUnique | ✓ | Restore :48-89；:85-87 temp store → `s.InsertUnique(t)` 循环 |
| RollbackTBox 走 DeleteSubject | ✓（路径修正） | 实际位置 `internal/ontology/induction/induction.go` :304-358；DeleteSubject sweep :345-353。**CONTEXT 写的 `internal/knowledge/ecl/induction.go` 是错误路径**（ecl 包只有 abox.go 用 DeleteSubject 于 ApplyNote :307-325） |
| SHACL quarantine 活跃路径 main.go:937-941 | ✓ | boot 校验 :930-951；Quarantine 循环 :937-941；shapes 路径 env `SKILLFORGE_SHACL_SHAPES` :930 |
| emitter.go:223-318 render = 13 谓词真源 | ✓（独立重枚举） | 见下节 |
| store_label_passthrough_test.go :45/:133/:175-208 红线 | ✓ | 三测试全文核对：β 下全部断言天然保持（label/comment ∉ allowlist） |
| store_test.go:569-744 Phase 40 hermetic gate | ✓ | 8 个 QueryScoped 测试逐一对 β 推演：全部保持绿（testTTL 含 3 个 skill 个体 + 1 个 cat，无测试断言 hasReadAccess 不可见） |
| fixture 22-triple + TierFilter 15 + FailClosed Path B 8 | ✓ | fixtureKG :129-163 = 22 unique（4 class defs + 2 skill rdf:type + 2 hasReadAccess + 2 tier rdf:type + 4 note_001 + 3 note_002 + 2 note_003 + 3 tbl）；TierFilter 断言 15 @ :262；FailClosed nil=2 @ :278 / Path B=8 @ :292；Path A 全套 22/4/22/22/22/22、Filter 1/1/0 |
| `testNS` == store `ns` | ✓ | 两者均为 `http://skillforge.local/ontology#`（store.go:42 / admin_platform_test.go:30 / store_label_passthrough_test.go:29）——fixture 的 8 个 schema triple 判定与生产同构 |

### Corrections to CONTEXT / upstream docs

1. **RollbackTBox 路径**：`internal/ontology/induction/induction.go:345-353`（CONTEXT 写 `internal/knowledge/ecl/induction.go` — 错）。ECL 侧另有第三个 DeleteSubject 调用簇：`internal/knowledge/ecl/abox.go:307,316,325`（ApplyNote replace 路径）。查询时派生方案下三者均自动正确。
2. **44-DECISIONS D-SPARQL-04 的"Production TBox emitter uses skf:Class"是错的**：emitter.go:219 写 `skf:%s a owl:Class`（owl: 前缀 = `http://www.w3.org/2002/07/owl#Class`），而 isSchemaTriple 检查 `ns+"Class"`（skf 命名空间）。**后果：生产中 class 声明 triples 在 tier 视角当前不可见**（走 fail-closed ABox 路径）。这是 pre-existing quirk，Phase 45 不修、也不应修（不在 boundary 内）——但它约束 conformance 测试设计（见 Pitfall 9）。
3. **CONTEXT Integration Points 中 "Hybrid Retrieval 经 QueryScoped 收益自动生效：skill 节点 label/tier 收集含 skill 元数据" 表述过强**：HybridSearchScoped 的 KG 扩展起点是 `rdf:type NoteChunk/TableSchema` 查询（admin_platform.go:389-390），skill 个体从不进入 KGNodes 集合——β 对 hybrid 节点集无实际影响。9 个 HybridSearchScoped 测试在 β 下平凡保持绿（已逐一推演）。CONTEXT 说的 :388-473 调用点本身无误（389/390/440/456/473 经 grep 证实）。

### Emitter 13-predicate allowlist — independent re-enumeration (CONTEXT 明确要求勿直接信任其清单；已核对，清单准确)

From `internal/ontology/emitter/emitter.go` render() (:196-321), predicates emitted **on schema-individual subjects** (subjects carrying `rdf:type → schemaClasses`):

| # | Predicate (resolved IRI = ns+local) | Emitted on | Emitter line |
|---|-------------------------------------|-----------|--------------|
| 1 | `skf:tierName` | Tier individuals | 257 |
| 2 | `skf:categoryName` | KBCategory individuals | 262 |
| 3 | `skf:belongsToTier` | KBCategory → Tier；SubCategory → cat_* | 262, 264 |
| 4 | `skf:subCategoryName` | SubCategory individuals | 264 |
| 5 | `skf:agentTypeName` | SkillAgent individuals | 281 |
| 6 | `skf:skillName` | Skill individuals | 286 |
| 7 | `skf:skillDescription` | Skill individuals | 286 |
| 8 | `skf:skillTemperature` | Skill individuals | 286 |
| 9 | `skf:poweredBy` | Skill → SkillAgent | 286 |
| 10 | `skf:hasReadAccess` | Skill → cat_* | 290 |
| 11 | `skf:hasWriteAccess` | Skill → cat_* | 296 |
| 12 | `skf:requiresRefusal` | Skill → phrase_* | 300 |
| 13 | `skf:refusalPhraseText` | RefusalPhrase individuals | 317 |

**Verdict: exactly 13, matches CONTEXT's allowlist verbatim.** emitter 还发出（但不在 schema-individual subjects 上，故不入 allowlist 语义域）：`rdf:type`（一切个体 + class 声明）、`rdfs:label`/`rdfs:comment`（仅 class 声明，:219）、`rdfs:domain`/`rdfs:range`（仅 property 声明，:237/252）。注意这些是 **rdfs:/owl: 词汇表 IRI**，不是 `ns+"label"` 等 skf: IRI——现有 isSchemaTriple 的 OWL 四谓词臂（检查 `ns+"domain"` 等）对生产 emitter 输出实际是死臂（pre-existing，不动）。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Schema/ABox triple 判定 | API/Backend（store 包） | — | 判定是数据层信任语义，单点 isSchemaTriple；任何端点侧复刻都违反 SPARQL-03 |
| Tier 解析（skill→tiers） | API/Backend（server 包） | — | resolveSkillTiersFromKG（/api/sparql）与 resolveSkillTiers（/api/admin、MCP）双 resolver 并存（BL-01 dedup 积压，不在本 phase） |
| SPARQL 查询面（/api/sparql） | API/Backend | — | X-Skill-Name 隔离，401 fail-closed |
| Admin KG query 面 | API/Backend | — | X-Admin-Token；skill param 可选 |
| MCP skf_query 面 | API/Backend | — | 服务级 SKILLFORGE_MCP_SERVICE_SKILL |
| 测试/构建编排 | Build env（WSL CGO） | — | internal/server 经 mattn/go-sqlite3 必须 CGO_ENABLED=1 |

无 Browser/CDN/DB tier 参与——本 phase 零前端改动、零 DB schema 变化、零 API contract 变化。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | go1.25.5 | 全部测试 | 项目既有约定（store 层 stdlib + testify 混用） |
| stretchr/testify | go.mod 既有 | assert 断言 | admin_platform_test.go / store_label_passthrough_test.go 既有模式 |
| cloudwego/hertz `ut` | 既有 | handler 级 HTTP 测试 | invokeOntology 模式（admin_ontology_test.go:96-112）已验证 |
| tggo/goRDFlib | 既有 | conformance 测试中 TTL 解析 | store.LoadFromTurtle 封装 |

**本 phase 零新增外部依赖。** 无 npm/pip/cargo 安装；Package Legitimacy Audit 不适用（见下）。

### Package Legitimacy Audit

不适用——本 phase 不安装任何外部包（纯 Go 内部改动 + 测试）。go.mod 既有依赖零变化。slopcheck 门禁无对象。

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────────┐
                        │            Query surfaces (read-only)        │
                        │                                              │
  POST /api/sparql ────►│ adminPlatformSparqlHandler                   │
  (X-Skill-Name, 401    │   └─ resolveSkillTiersFromKG ─┐              │
   if absent/unknown)   │                               │              │
                        │                               ▼              │
  GET /api/admin/       │ adminOntologyKGQueryHandler   │              │
  ontology/kg/query ───►│   └─ resolveSkillTiers ───────┤              │
  (X-Admin-Token,       │                               │  tiers []string
   skill= optional)     │   └─ (no skill → bare Query,  │              │
                        │      D-TIER-01 admin baseline)│              │
                        │                               ▼              │
  MCP skf_query ───────►│ mcp handleQuery ─ resolveSkillTiers           │
  (service skill)       │                               │              │
                        └───────────────────────────────┼──────────────┘
                                                        ▼
                        ┌──────────────────────────────────────────────┐
                        │  Store.QueryScoped(subj,pred,obj,tiers)      │
                        │                                              │
                        │  Path A: len(tiers)==0                       │
                        │    └─► dedup walk / Query()   [NO isSchema]  │
                        │                                              │
                        │  Path B: tiers non-empty                     │
                        │    1. allowedTierSet                         │
                        │    2. visibleSubjects ← byP[belongsToTier]   │
                        │    3. schemaSubjects ← byP[rdfType] ∩        │
                        │       schemaClasses   ◄── NEW (D-45-02,      │
                        │           derived once per call)             │
                        │    4. for each triple (dedup by S|P|O):      │
                        │       isSchemaTriple(t, schemaSubjects)?     │
                        │         YES → emit unconditionally  ◄── β    │
                        │         NO  → t.S ∈ visibleSubjects?         │
                        │                 YES → pattern filter → emit  │
                        │                 NO  → DROP (fail-closed)     │
                        └──────────────────────────────────────────────┘
                                                        ▲
                        ┌───────────────────────────────┴──────────────┐
                        │  Store mutators (write paths — ZERO changes) │
                        │  Insert / InsertUnique / DeleteSubject /     │
                        │  Quarantine / LoadSnapshot(Insert replay) /  │
                        │  Restore TBox merge / RollbackTBox /         │
                        │  ApplyNote / boot SHACL quarantine           │
                        └──────────────────────────────────────────────┘
```

Reader trace: a skill-scoped SPARQL request enters via X-Skill-Name → resolver yields tier names → QueryScoped Path B → per-triple schema judgment (now subject-aware) → filtered page. The judgment function is shared by all three surfaces; a change to `schemaClasses` or `metaPredicates` propagates to every endpoint through the single call site at store.go:550.

### Pattern 1: β arm + query-time derivation (the implementation recipe)

**What:** Add `metaPredicates` allowlist (const map, same pattern as schemaClasses), derive `schemaSubjects` once at Path-B entry, extend `isSchemaTriple` with a fourth arm.

**Exact shape (recommended under Claude's Discretion — helper-function form):**

```go
// store.go — placed immediately after schemaClasses (same const-map pattern).
// D-45-01 (Phase 45 / D-SPARQL-05): metadata predicates the TBox emitter
// emits on schema-individual subjects. A triple whose subject is a schema
// individual AND whose predicate is allowlisted here passes tier filtering
// unconditionally. label/comment are DELIBERATELY EXCLUDED (D-SPARQL-03
// stays authoritative: schema-individual labels are ABox annotations,
// fail-closed). Pinned by the emitter conformance test — if render()
// starts emitting a new predicate on schema individuals, that test fails
// until this allowlist is consciously extended.
var metaPredicates = map[string]bool{
	ns + "skillName":         true,
	ns + "skillDescription":  true,
	ns + "skillTemperature":  true,
	ns + "poweredBy":         true,
	ns + "hasReadAccess":     true,
	ns + "hasWriteAccess":    true,
	ns + "requiresRefusal":   true,
	ns + "tierName":          true,
	ns + "categoryName":      true,
	ns + "subCategoryName":   true,
	ns + "agentTypeName":     true,
	ns + "refusalPhraseText": true,
	ns + "belongsToTier":     true,
}

// schemaSubjectSet derives the set of subjects carrying
// (rdf:type, schemaClasses[O]) triples. D-45-02: query-time derivation,
// no persistent state — InsertUnique/DeleteSubject/Quarantine/rollback/
// LoadSnapshot all stay correct by construction (zero write-path hooks to
// forget). Cost: one walk of the byP[rdfType] bucket, sub-ms at KG scale.
// Caller must hold at least RLock.
func (s *Store) schemaSubjectSet() map[string]bool {
	out := make(map[string]bool)
	for _, idx := range s.byP[rdfType] {
		if schemaClasses[s.all[idx].O] {
			out[s.all[idx].S] = true
		}
	}
	return out
}
```

```go
// Signature change: isSchemaTriple gains the derived set.
// schemaSubjects may be nil (nil map lookups are false) — all existing
// pinned-test call sites pass nil mechanically.
func isSchemaTriple(t Triple, schemaSubjects map[string]bool) bool {
	// Arm 1 (unchanged): rdf:type → OWL vocabulary declarations.
	// Arm 2 (unchanged): rdf:type → schemaClasses[O].
	// Arm 3 (unchanged): ns+"domain"/"range"/"subClassOf"/"subPropertyOf".
	// Arm 4 (NEW, D-45-01 β):
	if schemaSubjects[t.S] && metaPredicates[t.P] {
		return true
	}
	return false
}
```

QueryScoped Path B, after visibleSubjects build and before the loop:

```go
// D-45-02: derive schema-individual subjects once per invocation.
schemaSubjects := s.schemaSubjectSet()
...
if isSchemaTriple(t, schemaSubjects) {   // was: isSchemaTriple(t)
```

**Why helper-function form:** CONTEXT lists it as discretion; the helper keeps the QueryScoped body readable, costs nothing (one bucket walk vs the existing full `s.all` walk), and is unit-testable in isolation.

**When to use:** exactly as locked. Do NOT explore eager Insert-time population (D-45-02 rejected it — 4-mutator hook coverage gap, snapshot-boot-only drift) or visibleSubjects-based variants (they yield 8-not-10, contradicting D-45-04 — see Pitfall 7).

### Pattern 2: Signature-change ripple handling (pinned test call sites)

`TestIsSchemaTriple_LabelAndCommentAreABox` calls `isSchemaTriple(t)` directly **8 times** (store_label_passthrough_test.go:52, 64, 73, 82, 91, 100, 109, 118). The signature change requires updating these call sites to `isSchemaTriple(t, nil)`. **All 8 existing assertions remain valid with nil** (nil map lookups are false; label/comment ∉ metaPredicates keeps the two False assertions true regardless of the set passed; the rdf:type/schemaClasses and OWL-predicate arms are untouched). Update calls mechanically; touch zero assertions; add a one-line comment citing D-45-04 red-line policy.

### Pattern 3: Handler-level parity test harness (SC3)

Existing pattern to copy — `invokeOntology` (admin_ontology_test.go:96-112): `server.Default(server.WithHostPorts(":0"))` + `ontologyRoutes(h, deps)` + `ut.PerformRequest`. For parity:
- `/api/sparql`: POST with header `X-Skill-Name: sql-guides` (NOT under admin token group — server.go:704), body `{"p":"...hasReadAccess"}`.
- `/api/admin/ontology/kg/query`: GET with header `X-Admin-Token` + `?skill=sql&p=...hasReadAccess` (admin_ontology.go:214-216).
- Fixture: reuse `fixtureKG()` (same package). Note the two resolvers differ (BL-01) — use direct Tier-object wiring (`hasReadAccess → sql-guides` as a Tier individual) so BOTH resolvers resolve identically (Case 1 in both implementations), keeping the parity assertion about the isSchemaTriple judgment, not resolver divergence.
- Assert: the `(skill_sql, hasReadAccess, sql-guides)` triple appears in BOTH responses; note_003 rows appear in NEITHER.

### Anti-Patterns to Avoid

- **Eager schemaSubjects maintenance:** D-45-02 explicitly rejected — restore.go InsertUnique / RollbackTBox DeleteSubject / SHACL Quarantine / ApplyNote DeleteSubject would each need a hook; miss one → silent SPARQL-02 violation visible only on snapshot-present boots (the repo's known derived-state-drift failure mode, cf. Milvus reseed write amplification).
- **visibleSubjects-based visibility for hasReadAccess:** produces 8-not-10 in FailClosed Path B (skill_compliance→authority would stay hidden under a sql-guides view), contradicting D-45-04's locked assertion list. CONTEXT `<specifics>` explicitly forbids this variant.
- **Adding label/comment to the allowlist "for symmetry":** breaks `TestStore_SchemaIndividualLabelTierVisibility` (:175-208) and `TestIsSchemaTriple_LabelAndCommentAreABox` — both pinned red lines.
- **Second schema-classification implementation** (e.g., a handler-side check for the parity test): SPARQL-03 requires the test to *observe* the shared judgment, not re-implement it.
- **Changing Path A or the nil-tier early-return:** D-45-03 locked "不改查询路径"; Path A counts are the regression canary (all frozen).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TTL parsing in conformance test | regex/substring scan of emitted Turtle | `store.LoadFromTurtle` (goRDFlib turtle parser) | Prefix expansion (`skf:` → full IRI) is exactly what the store does in production; a text scan would test the wrong representation |
| Schema visibility per endpoint | copy of isSchemaTriple in server pkg | the single store.go:604 function | SPARQL-03 is the requirement |
| HTTP handler tests | manual net/http server | hertz `ut.PerformRequest` + `invokeOntology` pattern | proven in admin_ontology_test.go; hermetic |
| Dedup in tests | manual expected-slice sorting | existing `seen` map semantics already in QueryScoped Path B | Path B output order follows `s.all` insertion order; assert with membership helpers, not index positions |

**Key insight:** everything this phase needs already exists in-repo as a proven pattern; the only genuinely new code is ~15 lines of store.go plus tests.

## Expected Count Derivations (verified, per D-45-04)

Fixture `schemaSubjects` under β = {`Class`, `ObjectProperty`, `Skill`, `Tier`} (class-def subjects — their rdf:type objects ARE ns+"Class" etc., which IS in schemaClasses) ∪ {`skill_sql`, `skill_compliance`, `sql-guides`, `authority`}. The only metaPredicates triples in the fixture: 2 × hasReadAccess.

| Test | Before | After | Derivation |
|------|--------|-------|------------|
| `TierFilter` (wildcard, ["sql-guides"]) | 15 | **17** | 8 existing schema + **2 hasReadAccess (skill_sql→sql-guides AND skill_compliance→authority — passthrough is tier-agnostic)** + 7 ABox (note_001×4 + tbl×3). note_002/note_003 negative assertions unchanged |
| `FailClosedNoBelongsToTier` Path B (S=note_003, ["sql-guides"]) | 8 | **10** | schema passthrough precedes S/P/O filter (store.go:543-554) → same +2 hasReadAccess bypass the S filter |
| `FailClosedNoBelongsToTier` nil | 2 | 2 | Path A, untouched |
| AllWildcard / SubjectFilter / Pagination / Truncated / LimitCap / DefaultLimit / Filter | 22/4/22/22/22/22/1,1,0 | unchanged | Path A never consults isSchemaTriple (early-return store.go:492-517) |
| store Phase 40 suite (store_test.go:569-744) | green | green | testTTL has 3 skill individuals + 1 cat_authority; no existing assertion requires hasReadAccess/categoryName invisibility under restrictive tiers; notes/tables ∉ schemaClasses by construction |
| HybridSearchScoped ×9 | green | green | skill individuals never enter hybrid KGNodes (expansion roots are NoteChunk/TableSchema rdf:type only) |

**Positive assertions to add (per D-45-04):** TierFilter results contain `(ns+"skill_sql", ns+"hasReadAccess", ns+"sql-guides")`; FailClosed Path B contains a hasReadAccess row. Note the 17/10 counts include skill_compliance→authority — if the planner writes the positive assertion as "exactly one hasReadAccess row" it will be wrong; assert on the specific skill_sql triple and let the count carry the rest.

## Common Pitfalls

### Pitfall 1: Trusting CONTEXT's induction.go path
**What goes wrong:** planner points a task at `internal/knowledge/ecl/induction.go` — file doesn't exist.
**Why:** transcription error in 45-CONTEXT canonical_refs.
**How to avoid:** use `internal/ontology/induction/induction.go:345-353` (verified). ECL's DeleteSubject cluster is `internal/knowledge/ecl/abox.go:307-325`.
**Warning signs:** task edits a file that fails to open.

### Pitfall 2: Breaking pinned tests while updating their call sites
**What goes wrong:** "fixing" the 8 `isSchemaTriple(t)` calls by also "updating" assertions to match new behavior.
**Why:** D-45-05 wording policy — these assertions are NOT miscounts; they are red lines that β is specifically constructed to keep passing.
**How to avoid:** change signature → pass `nil` → run `go test ./internal/ontology/store/` → zero assertion edits. If any red-line assertion fails, the implementation is wrong, not the test.
**Warning signs:** a diff that touches assertion lines in store_label_passthrough_test.go.

### Pitfall 3: Miscounting TierFilter 17
**What goes wrong:** asserting 16 (forgetting skill_compliance→authority is also schema-visible) or writing "exactly one hasReadAccess" assertions.
**Why:** schema passthrough is tier-agnostic by design; BOTH hasReadAccess triples appear in every tier view.
**How to avoid:** use the derivation table above; positive assertion targets the skill_sql triple specifically.
**Warning signs:** count assertions of 16, or loops asserting `len(hasReadAccessRows)==1`.

### Pitfall 4: Deriving schemaSubjects inside the triple loop
**What goes wrong:** O(n²) behavior / repeated allocation per triple.
**Why:** naive inline placement of the derivation inside `for _, t := range s.all`.
**How to avoid:** derive once after visibleSubjects, before the loop.
**Warning signs:** benchmark/sanity check shows QueryScoped regression; code review spots derivation in loop body.

### Pitfall 5: Conformance test asserting "all emitted predicates ⊆ 13-allowlist"
**What goes wrong:** the conformance test red-lights immediately on `rdf:type` and `rdfs:label`/`rdfs:domain` (TBox declarations) — those are emitted but legitimately outside the allowlist.
**Why:** class-declaration subjects are NOT in schemaSubjects (their rdf:type objects are OWL vocabulary IRIs, not ns+"Class" — see Corrections #2), so the leak-surface invariant is scoped to schema-INDIVIDUAL subjects only.
**How to avoid:** conformance assertion = for every triple `t` in the parsed emitter output where `schemaSubjects[t.S]`: `t.P ∈ {rdfType} ∪ metaPredicates`. Optionally add a negative pin: `label`/`comment` (both rdfs: and skf: forms) ∉ metaPredicates (locks D-SPARQL-03 priority).
**Warning signs:** conformance test failing on day one with rdfs: predicates.

### Pitfall 6: Running internal/server tests without WSL CGO
**What goes wrong:** Windows cgo.exe exit status 2 (44-RESEARCH §1) — no test runs at all, or false confidence from CGO_ENABLED=0 skipping sqlite-linked code.
**Why:** internal/server → mattn/go-sqlite3 (go.mod:16); production build path is CGO.
**How to avoid:** all gate commands under `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 ...'` (verified working: go1.25.5 linux/amd64 + gcc 11.4.0 present).
**Warning signs:** `cgo.exe: exit status 2` in output; CGO_ENABLED=0 in a server-package command.

### Pitfall 7: Choosing the visibleSubjects-based variant
**What goes wrong:** FailClosed Path B stays 8 (skill_compliance→authority still hidden), colliding with D-45-04's 10 and the positive hasReadAccess assertion.
**Why:** visibleSubjects only contains subjects with belongsToTier matching allowedTiers — skills have none.
**How to avoid:** β as locked; CONTEXT `<specifics>` explicitly forbids this variant.
**Warning signs:** plan tasks mentioning "add skill subjects to visibleSubjects".

### Pitfall 8: Writing "miscount" in godoc rationale
**What goes wrong:** violating D-45-05 — Phase 45's count changes are intended behavior changes, not test-author errors.
**How to avoid:** godoc cites `D-SPARQL-05 (v1.6 Phase 45) / D-45-04 (45-CONTEXT)` and describes the new semantics.
**Warning signs:** the strings "miscount" or "D-SPARQL-08" appearing in the 45 assertion-update rationale.

### Pitfall 9: "Fixing" verify_main.go's stale assertions in this phase
**What goes wrong:** scope creep into a pre-existing runtime-stale harness.
**Why:** `internal/server/platform/verify_main.go` still runs a PRE-Phase-44 fixture (owl:Class IRIs :59-62, raw-count 23 :426, "label is schema" assertions :441-464). It is invoked only by verify-phase-41.sh:53 (not the phase-45 gate), is `package main` (invisible to `go test`), and compiles cleanly (it never calls isSchemaTriple directly — only `kg.QueryScoped`, whose signature is unchanged).
**How to avoid:** leave it; note as pre-existing backlog. Do NOT add `go run verify_main.go` to verify-phase-45.sh.
**Warning signs:** a plan task editing verify_main.go, or verify-45 referencing it.

### Pitfall 10: Forgetting the fixtureKG godoc rewrite
**What goes wrong:** godoc block (admin_platform_test.go:39-124) still says hasReadAccess "EXCLUDED from Path B" — doc/code contradiction that will mislead the next phase (46 SHACL).
**How to avoid:** D-45-04 requires the godoc rewrite in the same commit as the assertion changes; new Path B derivation 17/10 written there per D-45-05 wording.
**Warning signs:** diff updates assertions but not lines 60-65 / 94-115 of the godoc.

## Code Examples

### Emitter conformance test shape (store package, imports emitter — no cycle; verified emitter imports only yaml)

```go
// internal/ontology/store/store_emitter_conformance_test.go
package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skillforge/skillforge/internal/ontology/emitter"
)

// TestEmitterPredicatesWithinAllowlist pins D-45-01's anti-drift gate:
// every predicate the emitter emits on a schema-individual subject must be
// in {rdf:type} ∪ metaPredicates. A render() change that adds a predicate
// on skill/cat/tier individuals fails HERE (explicitly) instead of silently
// becoming tier-visible or silently half-invisible.
func TestEmitterPredicatesWithinAllowlist(t *testing.T) {
	// 1. temp skillsRoot with one SKILL.md (reads/writes/agent/temperature/
	//    refusal phrases → exercises all 13 predicate families) + kbRoot dirs.
	// 2. ttl, err := emitter.New(skillsRoot, kbRoot, emitter.DefaultNS).Emit()
	// 3. st, err := LoadFromTurtle wrapper — parse via (s := New(); s.LoadFromTurtle(ttl))
	// 4. subjects := st.schemaSubjectSet()  // same derivation as Path B
	// 5. for each triple: if subjects[t.S] {
	//        assert t.P == rdfType || metaPredicates[t.P]
	//    }
	// 6. negative pin: !metaPredicates[ns+"label"] && !metaPredicates[ns+"comment"]
	//    (D-SPARQL-03 priority lock)
}
```

Notes: test file lives in `package store` (needs unexported `metaPredicates`/`schemaSubjectSet`); emitter imports nothing from store so the test-only import cannot cycle. Use `emitter.DefaultNS` so resolved IRIs match `ns`.

### Store dual-assertion test shape (D-45-04 store layer)

```go
// internal/ontology/store/store_schema_subject_test.go
// One test, two assertions, same subject (D-45-04):
//   insert (skill_x, rdf:type, skf:Skill), (skill_x, hasReadAccess, cat_a),
//          (skill_x, label, "secret"), plus one sql-guides note for a
//          non-empty visible set.
//   Path B (restrictive tier):
//     POSITIVE (D-SPARQL-05): hasReadAccess row present.
//     NEGATIVE (D-SPARQL-03 red line): label row absent — same subject.
```

### Parity test shape (SC3, handler level)

```go
// internal/server/admin_platform_parity_test.go (or extend admin_ontology_test.go)
// fixtureKG(); two requests over ut harness:
//   A: POST /api/sparql {"p": ns+"hasReadAccess"} + X-Skill-Name: sql-guides
//   B: GET  /api/admin/ontology/kg/query?p=<ns+hasReadAccess>&skill=sql-guides
//      + X-Admin-Token (t.Setenv SKILLFORGE_ADMIN_TOKEN)
// Assert: both return the (skill_sql, hasReadAccess, sql-guides) triple;
// neither returns any note_003 row. Same store, same judgment, two surfaces.
```

(Resolver caveat: fixtureKG wires `hasReadAccess → sql-guides` where sql-guides IS a Tier individual — Case-1 wiring that both `resolveSkillTiersFromKG` (admin_platform.go:623) and `resolveSkillTiers` (admin_ontology.go) resolve identically. Do not use cat-object wiring in the parity fixture or resolver divergence (BL-01) pollutes the SC3 signal.)

### verify-phase-45.sh skeleton (44-01-PLAN:456-474 command chain + verify-phase-39 anti-vacuous PASS counts + verify-phase-41 fail-fast sections)

```bash
#!/usr/bin/env bash
set -uo pipefail  # fail-fast per partition; partitions in order
REPO=/mnt/d/CODE/ClaudeCode/ai-agent
wsl() { wsl.exe -d Ubuntu-22.04 -- bash -c "cd $REPO && $1"; }

# P1 build + vet (CGO production path)
wsl 'CGO_ENABLED=1 go build ./... && CGO_ENABLED=1 go vet ./internal/ontology/... ./internal/server/...'
# P2 store suite (schema-subject + conformance + red lines + Phase 40 gate)
wsl 'CGO_ENABLED=1 go test ./internal/ontology/... -count=1'
# P3 targeted SPARQL + Hybrid (pair exit code with minimum PASS count — anti-vacuous)
wsl 'CGO_ENABLED=1 go test ./internal/server/ -count=1 -run "TestSparqlQueryScoped|TestHybridSearchScoped" -v | tee /tmp/p45.log' \
  && grep -c "^--- PASS" /tmp/p45.log | assert_min 19   # 10 SPARQL + 9 Hybrid
# P4 full hermetic suite
wsl 'CGO_ENABLED=1 go test ./internal/... -count=1'
```

(Adapt the assert_min helper to the verify-phase-39.sh grep-count pattern; MSYS_NO_PATHCONV=1 if Windows path mangling appears — verify-phase-40.sh precedent.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| label/comment always-schema predicate gate | removed (D-SPARQL-03), ABox fail-closed | Phase 44 (`2ac4855`) | β's allowlist must NOT reintroduce them |
| nil-tier path returned raw duplicates | dedup on all-wildcard Path A (D-SPARQL-02) | Phase 44 (`7b69f45`) | Path A = 22 baseline; frozen in 45 |
| fixture owl:Class IRIs + 23 raw | testNS Class IRIs + 22 unique (D-SPARQL-04) | Phase 44 (`9660177`, `788a06b`) | the 17/10 derivations stand on this fixture |
| subject-blind schema gate | β subject∩predicate (this phase) | Phase 45 | skill metadata tier-visible; D-SPARQL-03 priority preserved |

**Deprecated/outdated:** `verify_main.go`'s mental model (label-as-schema, raw counts) — stale since Phase 44; do not use as a semantic reference. 44-DECISIONS' "emitter uses skf:Class" claim — wrong vs emitter.go:219; use this RESEARCH's Corrections section instead.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | store package tests run green under plain `go test` (pure Go, goRDFlib only) so P2 needs no special env beyond WSL uniformity | verify script design | LOW — worst case P2 also needs CGO=1, which the script sets anyway |
| A2 | FE vitest suite unaffected (no FE file touches SPARQL result counts in an asserting way) | scope | LOW — D-45-04 gate excludes FE; if an FE test asserts triple counts it would surface in a later full-suite run, not this phase's gate |
| A3 | Two new store test files + one server test file naming (`store_schema_subject_test.go`, `store_emitter_conformance_test.go`, parity file) — names are CONTEXT-discretion suggestions, not requirements | test design | NONE — cosmetic |
| A4 | `resolveSkillTiers` (admin_ontology.go) resolves fixtureKG's direct-Tier wiring identically to `resolveSkillTiersFromKG` | parity test | MEDIUM if wrong — parity assertion would conflate resolver divergence with judgment divergence; mitigated by verified Case-1 code path in both (direct Tier-object lookup) and the explicit fixture guidance above |

All other claims in this research are code-verified (see Code-Verified Fact Base) — no user confirmation needed beyond the locked CONTEXT decisions.

## Open Questions (RESOLVED)

> Resolution recorded 2026-10-03 by plan-checker: all three adopted into plans per RESEARCH's own recommendations.

1. **Should the conformance test also pin the emitter's class-declaration IRI mismatch (owl:Class vs skf:Class)?** (RESOLVED → 45-02 T1 comment-only quirk note)
   - What we know: production class-def triples are currently tier-invisible (Corrections #2); fixing this is out of scope but a conformance comment should note it so Phase 46 doesn't "discover" it as new.
   - Recommendation: comment only; no assertion (would fail today).
2. **verify_main.go / verify-phase-41.sh staleness** (RESOLVED → excluded from gate; record BL-05 candidate in 45 SUMMARY, do not fix)
   - What we know: runtime-broken since Phase 44; not in any active gate; compiles clean.
   - Recommendation: record as backlog item (BL-05 candidate) in the phase SUMMARY; do not fix in 45 (scope discipline).
3. **Parity test file placement** (RESOLVED → extend admin_ontology_test.go, 45-02 T2)
   - Recommendation: extend the existing ontology test file (reuses `invokeOntology`, `newOntologyDeps`, token env helpers) with a parity test that uses `fixtureKG()`; either placement is acceptable per CONTEXT discretion.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| WSL Ubuntu-22.04 | all CGO test/build gates | ✓ (probed) | — | none — Windows cgo.exe broken (44-RESEARCH §1) |
| Go (linux/amd64, in WSL) | build/vet/test | ✓ (probed) | go1.25.5 | — |
| gcc (in WSL) | CGO_ENABLED=1 linking (emutls ABI) | ✓ (probed) | 11.4.0 | — |
| Go (windows/amd64) | non-CGO convenience runs (store pkg only) | ✓ | go1.25.5 | cgo broken — CGO_ENABLED=0 only |
| mattn/go-sqlite3 | internal/server test compilation | ✓ | v1.14.50 (go.mod:16) | — |
| testify, hertz ut, goRDFlib | tests | ✓ | go.mod pinned | — |
| Live LLM/Milvus/BGE | none | not required | — | suite is hermetic (env-intranet-service-flakiness memory) |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none.

WSL probe output (2026-10-03): `wsl.exe -d Ubuntu-22.04 -- bash -lc 'go version; gcc --version...'` → go1.25.5 linux/amd64, gcc 11.4.0, repo reachable at `/mnt/d/CODE/ClaudeCode/ai-agent` (both `/mnt/d/code` and `/mnt/d/CODE` work — case-insensitive mount).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing + testify (existing; go1.25.5) |
| Config file | none (go.mod module `github.com/skillforge/skillforge`; no test-framework config needed) |
| Quick run command | `wsl.exe -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 go test ./internal/ontology/store/ -count=1'` |
| Full suite command | `wsl.exe -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/CODE/ClaudeCode/ai-agent && CGO_ENABLED=1 go test ./internal/... -count=1'` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SPARQL-01 | skill 个体 hasReadAccess tier-可见 | unit (store) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestStoreSchemaSubject -v -count=1` | ❌ new (`store_schema_subject_test.go`) |
| SPARQL-01 | admin 层计数 17/10 + 正向断言 | unit (server) | `CGO_ENABLED=1 go test ./internal/server/ -run "TestSparqlQueryScoped_TierFilter|TestSparqlQueryScoped_FailClosedNoBelongsToTier" -v -count=1` | ✅ (update assertions in admin_platform_test.go) |
| SPARQL-01 (anti-drift) | emitter 谓词 ⊆ allowlist | unit (store, conformance) | `CGO_ENABLED=1 go test ./internal/ontology/store/ -run TestEmitterPredicatesWithinAllowlist -v -count=1` | ❌ new (`store_emitter_conformance_test.go`) |
| SPARQL-02 | fail-closed 不回退（负向断言 + Path A 冻结 + 红线三测试） | unit (regression) | `CGO_ENABLED=1 go test ./internal/ontology/store/... -count=1` + `... ./internal/server/ -run "TestSparqlQueryScoped\|TestHybridSearchScoped" -count=1` | ✅ (existing, must stay green unchanged) |
| SPARQL-03 | 双端点 schema 判定一致 | integration (handler, hermetic) | `CGO_ENABLED=1 go test ./internal/server/ -run TestSchemaJudgmentParity -v -count=1` | ❌ new (placement per Open Question 3) |
| SC4 | verify-phase-45.sh 全绿 | gate script | `bash verify-phase-45.sh` (WSL CGO chain, 4 fail-fast partitions) | ❌ new |

### Sampling Rate
- **Per task commit:** store suite quick command (≤30s, CGO_OK) + touched-package vet
- **Per wave merge:** targeted SPARQL+Hybrid gate with minimum-PASS count (10 SPARQL + 9 Hybrid)
- **Phase gate:** `bash verify-phase-45.sh` all partitions green before `/gsd:verify-work`

### Wave 0 Gaps
- None for framework/infrastructure (test harness, WSL toolchain, fixtures all exist and were probed).
- The three new test files + verify script are phase deliverables (normal tasks), not Wave-0 blockers.

*(No gaps: existing test infrastructure covers all phase requirements; new files are in-scope deliverables.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | unchanged (X-Admin-Token / X-Skill-Name / MCP service skill — out of phase scope) |
| V3 Session Management | no | stateless read endpoints |
| V4 Access Control | **yes — this phase IS an access-control change** | single enforcement point `QueryScoped` Path B; fail-closed default (subject ∉ visibleSubjects → drop); allowlist-bound schema passthrough; red-line regression suite as the control test |
| V5 Input Validation | yes (existing) | limit/offset validation + 500 cap (SparqlQueryScoped :191-202); MCP hard cap 500 (T-41-03); no new inputs in 45 |
| V6 Cryptography | no | n/a |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tier data leakage via schema-predicate abuse (the label-leak class Phase 44 fixed) | Information Disclosure | β allowlist EXCLUDES label/comment (D-SPARQL-03); conformance test red-lights new emitter predicates; red-line tests pin the boundary |
| Emitter mis-typing an ABox entity as schema class | Information Disclosure (bounded) | accepted upper bound per D-45-01: leaks at most allowlist predicates; NoteChunk/TableSchema ∉ schemaClasses by construction + schemaClasses comment + SHACL shapes (Phase 46) as future second layer |
| Empty-tiers fail-open (resolver returns empty non-nil → admin baseline) | Elevation / Info Disclosure | resolvers return nil not empty (fail-closed 401); pinned by def01 tests (admin_platform_def01_test.go:130-134) — do not disturb |
| Derived-state drift (persistent cache vs mutators) | Tampering (silent) | D-45-02 query-time derivation eliminates the class entirely |

**Security enforcement note:** `security_enforcement` is not disabled in .planning/config.json; this section is included. The phase's security posture change is exactly the D-45-01 leak-surface bound — no auth/crypto/session work.

## Sources

### Primary (HIGH confidence — direct code inspection at HEAD 24710bf, 2026-10-03)
- `internal/ontology/store/store.go` — :40-45 ns/rdfType consts; :117-150 Insert/InsertUnique; :162-187 DeleteSubject; :193-225 Quarantine; :273-306 Query; :377-387 LoadSnapshot; :477-577 QueryScoped; :579-633 isSchemaTriple; :641-659 schemaClasses
- `internal/server/admin_platform_test.go` — :29-37 consts; :39-166 fixtureKG + godoc; :168-358 SPARQL test assertions
- `internal/ontology/store/store_label_passthrough_test.go` — :45-120, :133-160, :175-208 red lines
- `internal/ontology/store/store_test.go` — :29-47 testTTL; :569-744 Phase 40 gate
- `internal/ontology/emitter/emitter.go` — :196-321 render (13-predicate enumeration); :46 DefaultNS
- `internal/ontology/store/restore.go` — :48-89 Restore/InsertUnique merge
- `internal/ontology/induction/induction.go` — :304-358 RollbackTBox
- `internal/knowledge/ecl/abox.go` — :307-325 ApplyNote DeleteSubject cluster
- `cmd/server/main.go` — :930-951 boot SHACL + quarantine
- `internal/server/admin_ontology.go` — :150-198 resolveSkillTiers; :205-224 KG query handler
- `internal/server/admin_platform.go` — :189-252 SparqlQueryScoped; :260-292 handler; :389-473 hybrid KG calls; :623-666 resolveSkillTiersFromKG
- `internal/server/platform/mcp/server.go` — :231-291 skf_query
- `internal/server/server.go` — :677, :704 route registrations
- `internal/server/platform/verify_main.go` — full read (staleness audit)
- `verify-phase-41.sh` — :53 verify_main invocation; partition structure
- `internal/server/admin_ontology_test.go` — :96-112 invokeOntology harness; :168-187 KG query endpoint test
- grep audits: `isSchemaTriple` (1 prod call site + 8 test sites), `schemaClasses` (store-internal only), `QueryScoped` (4 prod call sites), `DeleteSubject`/`RollbackTBox` (all mutators)

### Secondary (HIGH-MEDIUM)
- `.planning/phases/45-sparql-schema-visibility-d-sparql-05/45-CONTEXT.md` — locked decisions (D-45-01..05), discretion areas, deferred list
- `.planning/phases/44-sparql-test-fix/44-RESEARCH.md` §2/§5 — fixture audit lineage, Option α/β origin
- `.planning/phases/44-sparql-test-fix/44-DECISIONS.md` — D-SPARQL-01..08 (one claim corrected, see Corrections #2)
- `.planning/phases/44-sparql-test-fix/44-01-PLAN.md:456-483` — WSL CGO command chain
- `.planning/REQUIREMENTS.md` — SPARQL-01/02/03 definitions
- `.planning/STATE.md` — Phase 43/44 history, hermetic gate environment (matches probe)

### Tertiary (LOW — flagged, no action needed)
- None. No web sources were required; the entire phase domain is in-repo.

## Metadata

**Confidence breakdown:**
- Implementation recipe (β + query-time derivation): HIGH — every touched line read; signature ripple enumerated; no library unknowns
- Count derivations (17/10, Path A frozen): HIGH — re-derived triple-by-triple from the actual fixture against the actual loop structure
- Red-line preservation: HIGH — all pinned tests traced under β semantics; β excludes label/comment by construction
- Environment gates: HIGH — WSL toolchain probed live this session
- Production-behavior nuances (cat_*/sub_* already-visible, class-def invisibility, hybrid no-op): HIGH as code facts; their intentional-status is MEDIUM (pre-existing behavior, no decision doc claims it) — flagged for planner awareness only, no action in scope

**Research date:** 2026-10-03
**Valid until:** 2026-11-02 (30 days — stable; no upstream dependency churn; re-verify only if store.go/emitter.go change before planning)
