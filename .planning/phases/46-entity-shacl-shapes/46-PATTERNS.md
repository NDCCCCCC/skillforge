# Phase 46: Entity SHACL Shapes - Pattern Map

**Mapped:** 2026-10-04
**Files analyzed:** 12（3 新建 + 9 修改/扩展；另 3 个「预期零改动」咽喉点单独列出）
**Analogs found:** 12 / 12（本 phase 以「自身文件就地扩展」为主——修改文件的 analog 即其现状代码；新建文件的 analog 为同目录/同家族既有文件）

> 本 phase 是 Phase 40 自研 SHACL 引擎的延伸，**零新依赖**。绝大多数改动 = 在既有结构上加「一个 parse 分支 / 一个字段 / 一个徽章」。以下每个 assignment 都给出「现状代码（复制基座）+ 扩展点」。所有行号取自 HEAD dede743 工作树实测。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/ontology/shacl/entities.ttl` **(new)** | config（声明式 shapes 资产） | batch（boot/rollback 时 parse） | `internal/ontology/shacl/shapes.ttl` | exact（同目录同家族） |
| `internal/ontology/shacl/validate.go` (mod) | service（纯 Go 校验引擎） | transform（KG → Report） | 自身现状（:92-108/:118-135/:333-437/:428-434） | self |
| `internal/ontology/shacl/shapes.ttl` (mod) | config（声明式 shapes 资产） | batch | 自身现状（:4 注释 / :192-197 / :215-220） | self |
| `internal/ontology/shacl/shacl_test.go` (mod) | test（stdlib testing，无 testify） | fixture→assert | 自身现状（:144-151 fixture idiom / :696-753 补丁点） | self |
| `cmd/server/main.go` (mod) | config/boot 装配 | batch（启动编排） | 自身现状（:925-951 boot 链 + :173-178 envOr） | self |
| `internal/ontology/induction/induction.go` (mod) | service（rollback 管线） | batch | 自身现状（:382-390 修复点） | self |
| `internal/ontology/store/store.go` (**预期零改动**) | model（内存三元组库） | CRUD | —（仅作 D-46-04 否决证据引用） | n/a |
| `internal/server/admin_governance.go` (**预期零改动**) | controller（HTTP handler） | request-response | 自身现状（:45-63 passthrough 证据） | self |
| `web/src/components/admin/governance/types.ts` (mod) | model（FE DTO 镜像） | transform（JSON→类型） | 自身现状（:12-46 扩展点） | self |
| `web/src/components/admin/governance/governanceApi.ts` (**预期零改动**) | service（fetch 封装） | request-response | 自身现状（:56-60 null-guard 咽喉红线） | self |
| `web/src/components/admin/governance/ShaclReportView.tsx` (mod) | component（报告视图） | request-response | 自身现状（:15-26 徽章 idiom / :89-153 红区） | self |
| `web/src/components/admin/governance/GovernanceTab.tsx` (mod) | component（tab 壳 + HealthBanner） | request-response | 自身现状（:81-84 shapes meta） | self |
| `verify-phase-46.sh` **(new)** | test（验证门禁脚本） | batch（fail-fast 分区） | `verify-phase-45.sh`（+ `verify-phase-40.sh:30-35`） | exact |
| `internal/ontology/induction` 回归测试（新增，落 `induction_test.go` 或新文件） | test | fixture→assert | `induction_test.go`（`TestRollbackLatency` :241 就近先例） | role-match |

## Pattern Assignments

### 1. `internal/ontology/shacl/entities.ttl` (new — config/batch)

**Analog:** `internal/ontology/shacl/shapes.ttl`（唯一且精确的家族模板）

**文件头注释模式**（shapes.ttl:1-7）——决策编号入注释、env 可配置声明、命名空间声明：
```turtle
# SHACL Shape Declarations for SkillForge Knowledge Graph
# D-SHACL-01: Pure-Go SHACL validator using standard TTL shapes (goRDFlib turtle.Parse)
# D-SHACL-04: 8 NodeShapes covering all classes emitted by Phase 38/39 emitter
# Configurable path via SKILLFORGE_SHACL_SHAPES env (default data/ontology/shacl.ttl)
```
（注意 :4 的「default data/ontology/shacl.ttl」是 D-46-02 列明的陈旧描述，**不要照抄**；entities.ttl 头注释应引用 D-46-01/D-46-02 并写「路径经 SKILLFORGE_SHACL_SHAPES 列表配置」。）

**@prefix 块**（shapes.ttl:9-13）——entities.ttl 需要 sh/skf/rdf/rdfs 四个（不需要 xsd，按 D-46-01 约束清单不用 datatype）：
```turtle
@prefix sh:   <http://www.w3.org/ns/shacl#> .
@prefix skf:  <http://skillforge.local/ontology#> .
@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
```

**NodeShape 骨架模板**——最接近的既有形态是 `NoteChunkShape`（shapes.ttl:177-185）：`rdf:type hasValue` 自检 + 分节注释风格：
```turtle
skf:NoteChunkShape a sh:NodeShape ;
    sh:targetClass skf:NoteChunk ;
    sh:property skf:NoteChunk_type , skf:NoteChunk_referencesTable , skf:NoteChunk_sourcePath .

# rdf:type check (should always be present for any individual in KG)
skf:NoteChunk_type a sh:PropertyShape ;
    sh:path rdf:type ;
    sh:hasValue skf:NoteChunk ;
    sh:message "skf:NoteChunk must have rdf:type skf:NoteChunk"@en .
```

**minCount 必填模式**（shapes.ttl:87-91，KBCategory_name——label 约束照此，但**不加 datatype**，见 Pitfall 2）：
```turtle
skf:KBCategory_name a sh:PropertyShape ;
    sh:path skf:categoryName ;
    sh:minCount 1 ;
    sh:datatype xsd:string ;
    sh:message "skf:KBCategory must have at least one skf:categoryName (xsd:string)"@en .
```

**if-present class 检查模式**（shapes.ttl:93-96，KBCategory_belongsToTier——无 minCount，只有 sh:class；实体 belongsToTier/parentOrganization 照此）：
```turtle
skf:KBCategory_belongsToTier a sh:PropertyShape ;
    sh:path skf:belongsToTier ;
    sh:class skf:Tier ;
    sh:message "skf:KBCategory must have skf:belongsToTier pointing to a skf:Tier"@en .
```

**engine 侧对接约束（决定 TTL 写法）：**
- `parsePropertyShape`（validate.go:333-437）逐谓词扫描；`sh:severity` 新分支加入后，**severity 必须写在每个 `sh:PropertyShape` 内**（引擎不读 NodeShape 级 severity——Pitfall 3）。
- PropertyShape 可为 IRI 引用（`sh:property skf:Xxx_name`，shapes.ttl 主风格）或 blank node（`sh:property [ ... ]`，测试 fixture 风格）——交付文件用 IRI 引用风格。
- `sh:message` 以 `@en` 结尾（全部既有 shape 如此）。
- 约束对象真源：`internal/knowledge/ecl/stub.go:99-105` `legalEntityKinds` = `{InsuranceProduct, Organization}`；TBox 类声明在 `data/ontology/skillforge_tbox.ttl:253-274`（InsuranceProduct/Organization 类 + involvesX/parentOrganization/entityCode 属性）。
- 发射契约（决定必填 vs if-present）：`internal/knowledge/ecl/abox.go:209-236`——`rdf:type` :210 与 `rdfs:label` :211 无条件发出（→ minCount 1 @ error 安全）；`skf:entityCode` :213-215 条件发出；`skf:belongsToTier` :220-222 仅 note.Tier 非空；`parentOrganization` :232-236 仅同批精确名匹配（→ 均 if-present）。

---

### 2. `internal/ontology/shacl/validate.go` (mod — service/transform)

**Analog:** 自身现状。四处改动点逐一给出现状锚。

**改动点 A — DTO 扩展**（现状 :54-83）：`omitempty` 惯例已有先例（`SourcePath` :60）；新字段 `TargetClass`/`Layer`（Violation）与 `DanglingEdgeCount`（QuarantinedEntity）照此加：
```go
// Violation records one SHACL constraint violation.
type Violation struct {
	Severity    ViolationSeverity `json:"severity"`
	Entity      string            `json:"entity"`
	Constraint  string            `json:"constraint"`
	Detail      string            `json:"detail"`
	SourcePath  string            `json:"source_path,omitempty"`
	LineStart   int               `json:"line_start,omitempty"`
	LineEnd     int               `json:"line_end,omitempty"`
}

// QuarantinedEntity is an entity that has at least one error-level violation.
type QuarantinedEntity struct {
	Entity      string      `json:"entity"`
	TripleCount int         `json:"triple_count"`
	Violations  []Violation `json:"violations"`
}
```

**改动点 B — Validate 签名 variadic 化 + ShapesPathsFromEnv helper**（现状 :92-108）：
```go
func Validate(kg *store.Store, shapesPath string) (*Report, error) {
	return ValidateWithTrigger(kg, shapesPath, "boot")
}

func ValidateWithTrigger(kg *store.Store, shapesPath string, trigger string) (*Report, error) {
	parsed, err := parseShapes(shapesPath)
	if err != nil {
		return nil, fmt.Errorf("shacl: parse shapes: %w", err)
	}
	report := &Report{
		ShapesPath:  shapesPath,
		...
```
- `...string` variadic 让 13 处既有测试调用 `Validate(kg, tmpPath)` 零改动编译通过。
- 多文件 merge 的结构基础：`parsedShapes.NodeShapes` 是纯 slice（:186-196），逐文件 `append` 即可；`parseShapes(path)`（:217-331）签名可保持单路径，由 Validate 层循环。
- 缺失路径不得静默吞：现状 `os.ReadFile` 失败 → `fmt.Errorf("read shapes file: %w", err)`（:219-221）→ Validate 整体返回 error → main.go:932-934 WARN + 整轮校验（含 quarantine）跳过——多文件沿用「任一失败即整体失败」语义（`errors.Join` 聚合）。
- env helper 放 shacl 包单点（boot/rollback 共用，杜绝组合漂移）：
```go
// RESEARCH Pattern 2 形态（filepath.SplitList 空串返回空 slice——先判空回退默认列表）：
func ShapesPathsFromEnv() []string {
    if v := os.Getenv("SKILLFORGE_SHACL_SHAPES"); v != "" {
        return filepath.SplitList(v)
    }
    return []string{ /* 默认双文件：shapes.ttl + entities.ttl */ }
}
```

**改动点 C — severity 解析分支替换 sourcePath 硬编码**（退役点 :428-434）：
```go
	// D-PROV-10: provenance-missing (skf:sourcePath on NoteChunk/TableSchema) is warning-only.
	// Detect by path = skf:sourcePath.
	if c.Path == pf("sourcePath") {
		c.Severity = SeverityWarning
	} else {
		c.Severity = SeverityError
	}
```
新分支照既有约束解析分支的循环模板写（同函数内 :353-361 minCount 分支为模板）；`propertyConstraint.Severity` 字段已存在（:214）且已流入全部 check*（:503/:517/:533...），解析后自动生效。`sh()` helper（:727）现成。**同 commit 必须在 shapes.ttl:193-197/:216-220 两个 sourcePath PropertyShape 显式补 `sh:severity sh:Warning`，并给 `shacl_test.go:720-725` 的 fixture 补同一行**（否则 TestShaclViolation 翻红——Pitfall 1）。

**改动点 D — layer/target_class 盖戳**（stamping 循环现状 :117-135）：
```go
	// Walk each NodeShape.
	for _, ns := range parsed.NodeShapes {
		// Collect all individuals of the target class.
		individuals := collectIndividuals(kg, ns.TargetClass)

		for _, entity := range individuals {
			for _, prop := range ns.Properties {
				violations := evaluateProperty(kg, entity, prop, parsed)
				for _, v := range violations {
					v.Entity = entity
					if v.Severity == SeverityError {
```
`v.Entity = entity`（:126）即盖戳点；`ns.TargetClass` 循环内现成。layer 派生推荐 parse 期源文件追踪——`nodeShape` struct（:198-201）加 Layer 字段，parseShapes 列表化后逐文件 append 时按 basename 打 `entity`/`tbox` 戳。

**global 哨兵点**（checkGlobalReferences :692-718）——跨实体违规的 Violation 构造处直接写 `Layer: "global"`：
```go
		if !existsType(kg, target, ns("type"), pf("TableSchema")) {
			violations = append(violations, Violation{
				Severity:   SeverityError,
				Entity:     tr.S,
				Constraint: "sh:class",
				Detail:     fmt.Sprintf("skf:referencesTable target <%s> is not a skf:TableSchema instance", target),
			})
		}
```

**改动点 E — dangling_edge_count**（构建 quarantine 列表现状 :148-157，计数插在 kg 全量可查的校验期内）：
```go
	// Build quarantined list from entities that have at least one error.
	for entity, violations := range entityErrors {
		tripleCount := len(kg.Query(entity, "", ""))
		report.Quarantined = append(report.Quarantined, QuarantinedEntity{
			Entity:      entity,
			TripleCount: tripleCount,
			Violations:  violations,
		})
		report.ErrorCount += len(violations)
	}
```
反向计数直接用 `kg.Query("", pred, entity)`（byO 索引，store.go:271-306）；谓词常量用既有 `pf(...)` helper（:729）。就近部分 analog：`store.Adjacent1Reverse`（store.go:323-328）已示范同款反查 idiom。

**包 docstring 惯例**（:17-29）——引擎级决策编号写在这里，新增 severity 决策引用照此追加：
```go
// Package shacl implements a pure-Go SHACL validation engine.
//
// D-SHACL-01: Zero third-party Go SHACL library dependencies. ...
// Seven constraint types are implemented via switch cases: sh:minCount,
// sh:maxCount, sh:class, sh:datatype, sh:nodeKind, sh:pattern, sh:in.
// Adding a new constraint type requires only a new case branch.
//
// D-SHACL-02: Violations are split into error (structural / quarantined)
// and warning (optional property missing / provenance-missing). ...
```

---

### 3. `internal/ontology/shacl/shapes.ttl` (mod — retrofit)

**Analog:** 自身现状。三处：

- **:4 陈旧头注释修正**：`# Configurable path via SKILLFORGE_SHACL_SHAPES env (default data/ontology/shacl.ttl)` 与 main.go:930 实际默认 `internal/ontology/shacl/shapes.ttl` 不符——D-46-02 列明修正，顺带更新 D-SHACL-04 描述。
- **:192-197 与 :215-220 两个 sourcePath PropertyShape 补显式 severity**（retrofit 后行为不变，政策从引擎硬编码归位数据）：
```turtle
# D-PROV-10: skf:sourcePath is optional — provenance-missing is a WARNING
skf:NoteChunk_sourcePath a sh:PropertyShape ;
    sh:path skf:sourcePath ;
    sh:maxCount 1 ;
    sh:nodeKind sh:Literal ;
    sh:message "skf:sourcePath on skf:NoteChunk is optional; if present must be a literal (warning-level)"@en .
```
- **知悉项**：:227-232 `skf:IRIPropertyShape` 是无 `sh:targetClass` 的第 9 个 NodeShape（隐性全域 shape）——新测试勿把「8 NodeShapes」当文件全集断言（Pitfall 10）。

---

### 4. `internal/ontology/shacl/shacl_test.go` (mod — test)

**Analog:** 自身现状。测试 fixture idiom 全套照抄：

**URI 常量块**（:30-65）——实体测试的新常量（InsuranceProduct/Organization/entityCode/label/predicates）加在这里：
```go
const (
	rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	skfTier = "http://skillforge.local/ontology#Tier"
	...
	skfBelongsToTier     = "http://skillforge.local/ontology#belongsToTier"
```

**temp-file fixture idiom**（:144-151，全部 13 个测试共用）：
```go
	tmp, err := os.CreateTemp("", "shacl-test-*.ttl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.WriteString(fixture)
	tmp.Close()
	defer os.Remove(tmpPath)

	parsed, err := parseShapes(tmpPath)
```

**KG 构造 idiom**（:204-206 等）——`store.New()` + 逐条 `Insert(store.Triple{S,P,O})`，字面量带引号（`"tier-a"`）——新 fixture 沿用引号字面量，规避 Pitfall 2 的裸值分类问题：
```go
	kg := store.New()
	kg.Insert(store.Triple{S: "http://skillforge.local/ontology#kb1", P: rdfType, O: skfKBCategory})
```

**TestShaclViolation fixture 单行补丁点**（:705-726；断言 :741-752 零改动）：
```go
    sh:property [
        sh:path skf:sourcePath ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:message "provenance-missing (warning-level, D-PROV-10)"
    ] .
```
→ 在 `sh:datatype` 行后补 `sh:severity sh:Warning ;`（镜像 retrofitted shapes.ttl）。断言 `WarningCount==1 / 0 quarantined / health=warn` 原样保留。

**SC2 隔离测试模板**（TestShaclQuarantine :757-827）——注入违规 → 断言进 Quarantined + TripleCount + Violations 结构：
```go
	found := false
	for _, q := range report.Quarantined {
		if q.Entity == nsNote002 {
			found = true
			if q.TripleCount == 0 {
				t.Errorf("quarantined entity note_002 has 0 triples, want 2")
			}
```
（SC2 subject 不可见断言加 `kg.Query(entity, "", "")` 为空 + 再 Validate 报告稳定——Pitfall 6 的「稳定循环」口径；对象位可见的悬挂边照 D-46-04 如实断言计数。）

**global refs 测试模板**（TestGlobalReferencesConstraint :833-876）——fixture 内附 `skf:referencesTableShape a sh:PropertyShape` 全局 shape 即触发 checkGlobalReferences——layer=global 哨兵测试照此。

---

### 5. `cmd/server/main.go` (mod — boot 装配)

**Analog:** 自身现状 :925-951。改动只落 env 读取 + Validate 调用两行，链路顺序不动：
```go
	shaclShapesPath := envOr("SKILLFORGE_SHACL_SHAPES", filepath.Join("internal", "ontology", "shacl", "shapes.ttl"))
	shaclReport, shaclErr := shacl.Validate(ontologyKG, shaclShapesPath)
	if shaclErr != nil {
		// Non-fatal: log and continue. Boot proceeds without blocking on SHACL.
		log.Printf("[WARN] cmd/server: SHACL validation: %v", shaclErr)
	} else {
		// D-SHACL-02: error-level entities → quarantine; warnings stay in KG.
		for _, q := range shaclReport.Quarantined {
			for _, t := range ontologyKG.Query(q.Entity, "", "") {
				ontologyKG.Quarantine(t)
			}
		}
		// Cache the report so GET /api/admin/governance/shacl can serve it.
		ontologyKG.SetShaclReport(shaclReport)
```
- 改法：`shaclShapesPath := envOr(...)` → `shaclShapesPaths := shacl.ShapesPathsFromEnv()`；`shacl.Validate(ontologyKG, shaclShapesPath)` → `shacl.Validate(ontologyKG, shaclShapesPaths...)`。隔离循环 / SetShaclReport / health 日志零改动（D-46-04：boot 隔离循环 subject-only 保持不动）。
- **时序红线**：boot 快照写在 validate 之前（:921 WriteSnapshot vs :931 Validate）——不要移动任何顺序、不要新增 post-quarantine 快照触发（Pitfall 6 稳定循环 vs 永久化）。
- env 读取既有惯例 `envOr`（:173-178）：
```go
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
```
（本 phase 默认值收敛进 `shacl.ShapesPathsFromEnv()` 单点，main.go 不再自带默认路径字面量。）

---

### 6. `internal/ontology/induction/induction.go` (mod — rollback parity)

**Analog:** 自身现状 :382-390（修复目标本体）：
```go
	// Step 11: Run SHACL revalidation and return violation count (D-PROV-09).
	shapesPath := filepath.Join(m.dataDir, "..", "shacl", "shapes.ttl")
	report, err := shacl.ValidateWithTrigger(m.kg, shapesPath, "rollback")
	if err != nil {
		// SHACL failure is not a rollback failure — log and return 0 violations
		fmt.Fprintf(os.Stderr, "[WARN] induction: post-rollback SHACL validation: %v\n", err)
		return next, 0, nil
	}
	return next, report.ErrorCount, nil
```
- `filepath.Join(m.dataDir, "..", "shacl", "shapes.ttl")` = 不存在的 `data/shacl/shapes.ttl` → 恒走 warn-and-return-0 no-op。修法：`shacl.ValidateWithTrigger(m.kg, shacl.ShapesPathsFromEnv()..., "rollback")`。
- warn-and-return-0 姿态保留（D-PROV-09 语义：SHACL 失败不算 rollback 失败）；修复后加回归测试钉住「真读 env」——就近先例 `induction_test.go`（`TestRollbackLatency` :241 起的 rollback 测试族，`TestQueuePersistsAcrossRestart` :212 示范 Manager 级 temp-dir 装配 idiom）。

---

### 7. `web/src/components/admin/governance/types.ts` (mod — FE DTO 镜像)

**Analog:** 自身现状 :12-46。扩展点与 snake_case + optional 镜像惯例：
```typescript
export interface ShaclViolation {
  severity: ViolationSeverity;
  /** entity URI; '(global)' for shape-level violations */
  entity: string;
  /** sh:minCount | sh:class | provenance-missing | … */
  constraint: string;
  /** engine message, rendered verbatim */
  detail: string;
  /** SC-1: source file path */
  source_path?: string;
  /** D-PROV-05: 1-based line number */
  line_start?: number;
  line_end?: number;
}

export interface QuarantinedEntity {
  entity: string;
  triple_count: number;
  violations: ShaclViolation[];
}
```
- 新字段按 46-UI-SPEC §Component 1 逐字：`layer?: ViolationLayer`（type = `'tbox' | 'entity' | 'global'`）、`target_class?: string`、`QuarantinedEntity.dangling_edge_count?: number`。
- **`ShaclReport` 零变化**（`shapes_path` 保持单 string :41）。
- 注释惯例：每个字段带决策编号 JSDoc（`/** D-46-03: ... */`）——照 `source_path?` 的 `/** SC-1: ... */` 先例。

---

### 8. `web/src/components/admin/governance/ShaclReportView.tsx` (mod — component)

**Analog:** 自身现状。四处插入点：

**徽章 idiom**（:14-26）——LayerBadge 照 ViolationBadge 逐字结构（色板换 46-UI-SPEC §Color 表）：
```tsx
// Severity badge colors per UI-SPEC §Color
const SEVERITY_BADGE: Record<string, string> = {
  error: 'bg-status-non-compliant/10 text-status-non-compliant',
  warning: 'bg-status-needs-info/10 text-status-needs-info',
};

function ViolationBadge({ severity }: { severity: ShaclViolation['severity'] }) {
  return (
    <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', SEVERITY_BADGE[severity])}>
      {severity === 'error' ? '错误' : '警告'}
    </span>
  );
}
```
（tbox=`bg-primary-dim text-primary` / entity=`bg-surface-3 text-text-muted` / global=`border border-dashed border-border text-text-muted`；unknown layer 渲染原值同 entity 色。target_class 用 neutral chip `rounded bg-surface-2 px-1.5 py-0.5 text-[11px] text-text-muted` + mono IRI。）

**红区表（6 列 → 7 列）**：表头 :96-105、行体 :107-145。「悬挂边」列插在「三元组」（:120-121）之后；cell 照既有行号列的 `!= null` 惯例（:138-142）——`dangling_edge_count != null ? count : '—'`，0 如实渲染 0；header 加 `title="指向该隔离实体的入边数量（在 KG 查询对象位仍可见）"`。约束 cell（:122-128）扩为 layer 徽章 → target_class chip → constraint mono → `+{extra}`，layer 缺省不渲染徽章（legacy 兼容）：
```tsx
                      {/* 约束 */}
                      <td className="px-3 py-1.5 font-mono">
                        {first.constraint}
                        {extra > 0 && (
                          <span className="ml-1 text-text-muted">+{extra}</span>
                        )}
                      </td>
```

**区头计数**：红区 `<h2>` :91-93 / 黄区 :158-160 追加 `<span className="ml-2 font-mono text-xs font-normal text-text-muted" data-testid="governance-{quarantine|warnings}-layer-counts">TBox n · 实体 m · 跨实体 k</span>`（仅非零 layer、固定顺序）：
```tsx
          <h2 className="mb-2 text-sm font-semibold text-status-non-compliant">
            隔离区（error 级违规实体）
          </h2>
```

**hint 文案替换**（:149-151）：
```tsx
          <p className="mt-2 text-xs text-text-muted">
            隔离实体不进入主索引；可修复数据后通过 Run ECL 重建，或待下次启动重新校验。
          </p>
```
→ 46-UI-SPEC §Copywriting 新文案（subject 位置语义 + 悬挂边指引 + 原恢复路径）。黄区表 5 列不变（:162-170）。

---

### 9. `web/src/components/admin/governance/GovernanceTab.tsx` (mod — 可选微调)

**Analog:** 自身现状 :81-84（HealthBanner shapes meta——唯一触碰点）：
```tsx
      <div className="ml-auto text-xs text-text-muted font-mono">
        校验时点 {new Date(report.validated_at).toLocaleString()}
        （{report.trigger === 'boot' ? '启动时' : '回滚后'}）· shapes {report.shapes_path}
      </div>
```
→ shapes 值包一层 `<span className="inline-block max-w-[280px] truncate align-bottom" title={report.shapes_path}>`（46-UI-SPEC §Component 5，显示保护）。其余（Tab 注册 :22-26、view switcher :114-131、backfill CTA :64-80）全部不动。

---

### 10. `verify-phase-46.sh` (new — test 门禁)

**Analog:** `verify-phase-45.sh`（最新且最完整；`verify-phase-40.sh:30-35` 提供 shacl 包 targeted -run 先例）。

**骨架**（45 全套照搬）：`set -uo pipefail` + `MSYS_NO_PATHCONV=1`（:30-32）、WSL wrapper（:34-38）、色 helpers（:40-44）、`pass_lines` anti-vacuous 计数（:49-51）、P1-P4 分区：
```bash
REPO=/mnt/d/CODE/ClaudeCode/ai-agent
GO=/usr/local/go/bin/go

# wsl <command> — run a gate command inside WSL at the repo root.
wsl() { wsl.exe -d Ubuntu-22.04 -- bash -c "cd $REPO && $1"; }

# pass_lines <log> — number of --- PASS lines in a -v go test log
# (verify-phase-39.sh anti-vacuous pattern).
pass_lines() { grep -c -- '--- PASS' "$1" || true; }
```

**P1-P4 结构**（45:66-129）：P1 `CGO_ENABLED=1 build ./... + vet`；P2 ontology 套件；P3 targeted `-run` + 最低 `--- PASS` 计数（正则配对、阈值注释写明组成）；P4 `./internal/...` 全量。Phase 46 的 P3 正则建议形态（RESEARCH Test Map）：`-run 'TestShapesPaths|TestSeverity|TestViolationLayer|TestDanglingEdges|TestEntityQuarantine|TestRollbackShaclParity'`。

**shacl 包 targeted 先例**（verify-phase-40.sh:30-35）：
```bash
go vet ./internal/ontology/shacl/... ./internal/ontology/store/... ./internal/ontology/induction/... 2>&1 | head -5 || fail "vet failed"
go test ./internal/ontology/shacl/... -v -run "TestShaclViolation|TestShaclQuarantine" 2>&1 | tail -5 || fail "SHACL violation/quarantine tests failed"
```
**CGO 注意**：shacl 包纯 Go 可 `CGO_ENABLED=0` 快跑；P1/P4 全量必须 `CGO_ENABLED=1`（WSL Linux gcc）。45 的 waiver_ok 机制（:53-61）仅当 46 存在已记录 pre-existing failure 时才需要——45 的 D-45-DEF-01 waiver 是否仍适用由 planner 按 merge 时点核实。

---

### 11. rollback parity 回归测试（internal/ontology/induction — new test）

**Analog:** `induction_test.go` 既有函数族（TestApproveIncrementsTBoxVersion :109、TestQueuePersistsAcrossRestart :212、TestRollbackLatency :241——Manager 级 temp-dir 装配 + 断言返回值）。断言口径：rollback 触发后 `ValidateWithTrigger(..., "rollback")` 真的从 env 列表 parse shapes（当前 no-op 零覆盖——修复后补钉）。

## 预期零改动的咽喉点（改动即红线违规）

| File | 证据 | 红线 |
|------|------|------|
| `internal/ontology/store/store.go` | `Quarantine` :189-225（subject 键控）、`QuarantinedEntities` :235-249、report cache :251-262、`Snapshot` :366-374（只序列化 `s.all`）、QueryScoped 可见性 :550-583（ABox 需 visible subject，:563-566） | dangling_edge_count 落 `shacl.QuarantinedEntity`（validate.go:66-70）；**不动 store**——级联隔离被否决的机制证据恰是 :196（`entity := t.S`）与 :370-374 |
| `internal/server/admin_governance.go` | handler :45-63 `c.JSON(http.StatusOK, r)` 直接透传 `*shacl.Report` | DTO 加字段自动透传，handler 零改动 |
| `web/src/components/admin/governance/governanceApi.ts` | null-guard 咽喉 :56-60（260928-jxb 白屏先例；既有测试 `governanceApi.test.ts:29-86` 钉住） | 新字段全为标量 → 咽喉点不动；PR 出现此文件改动即偏离（唯一例外：引入新数组字段才扩 normalize） |

## Shared Patterns

### 1. Apache-2.0 版权头
**Source:** `internal/ontology/shacl/validate.go:1-15`（Go 全文版）、`types.ts:1-2`（TS 两行版：`// Copyright 2026 SkillForge Authors` + `// SPDX-License-Identifier: Apache-2.0`）
**Apply to:** 全部新建/扩展 Go 与 TS 文件；`verify-phase-46.sh` 照 verify-45.sh:2 的单行版（`# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0`）。

### 2. 决策编号 docstring 引用
**Source:** `validate.go:17-29`（包级）、`store.go:189-192`（函数级 `D-SHACL-02:` 行）、`main.go:925-929`（块注释）、`governanceApi.ts:57-59`（quick-260928-jxb bug 引用）
**Apply to:** severity 分支、layer 盖戳、ShapesPathsFromEnv、entities.ttl 头注释——每处新逻辑标注 D-46-01..04（+ 保留 D-PROV-10 语义引用）。

### 3. BE DTO → FE snake_case 镜像 + omitempty/optional 对偶
**Source:** `validate.go:55-83` ↔ `types.ts:12-46`
**Apply to:** `TargetClass` → `target_class?`、`Layer` → `layer?`、`DanglingEdgeCount` → `dangling_edge_count?`；Go 零值省略（omitempty）= FE optional（`!= null` 判空渲染，照行号列 :138-142 惯例）。

### 4. severity 单字段流经 check*（引擎改动面天然收敛）
**Source:** `propertyConstraint.Severity`（validate.go:214）→ 全部 check 函数消费（:503/:517/:533/:549/:576/:624/:649/:669/:685）
**Apply to:** severity 解析分支是唯一 Go 改动点；**不要**在各 check* 内加任何 severity 逻辑。

### 5. 测试 fixture 三件套
**Source:** `shacl_test.go:144-151`（CreateTemp TTL）/ `:204-206`（store.New + Insert）/ `:226-235`（Validate + 断言）
**Apply to:** severity 三态、layer 盖戳、entities.ttl parse、dangling 计数、SC2 隔离全部新测试。首跑全绿断言用 BuildTriples 构造性 fixture（abox.go:165-239 的发射条件即保证），**不写 live 快照计数断言**（Pitfall 7）。

### 6. env 单点读取 + os 分隔符
**Source:** `main.go:173-178`（envOr）、RESEARCH Pattern 2（`filepath.SplitList`；空串 → 空 slice，必须先判空回退默认列表）
**Apply to:** `SKILLFORGE_SHACL_SHAPES` 的唯一消费入口 = `shacl.ShapesPathsFromEnv()`；boot（main.go:930）与 rollback（induction.go:383）两点同源。

### 7. FE console idiom（mono + testid + truncate-with-title）
**Source:** ViolationBadge（ShaclReportView.tsx:20-26）、`data-testid` 全覆盖（:90/:113/:157/:176）、truncate+title（:130/:134）
**Apply to:** LayerBadge、区头计数 span、悬挂边列、HealthBanner shapes meta——全部沿用既有 class 组合，零新 token。

## No Analog Found

| File | Role | Data Flow | Reason | Fallback |
|------|------|-----------|--------|----------|
| `dangling_edge_count` 反向计数 helper（validate.go 内新函数） | utility | query-aggregate | shacl 包内无反查计数先例 | 部分 analog：`store.Adjacent1Reverse`（store.go:323-328）同款 `Query("", pred, obj)` idiom；谓词 IRI 家族见 abox.go:224-236 |

其余全部文件均有 exact / self analog。

## Metadata

**Analog search scope:** `internal/ontology/{shacl,store,induction,ecl}`、`cmd/server`、`internal/server`、`web/src/components/admin/governance`、repo root `verify-phase-*.sh`、`data/ontology`
**Files read in full:** validate.go (799), shapes.ttl (232), shacl_test.go (876), admin_governance.go (220), types.ts (94), governanceApi.ts (117), ShaclReportView.tsx (214), GovernanceTab.tsx (149), verify-phase-45.sh (138), 46-UI-SPEC.md (319)
**Files read targeted:** induction.go (:350-409), main.go (:885-969 + envOr :173-178), store.go (:185-384, :540-589), abox.go (:160-259), stub.go (:90-114), skillforge_tbox.ttl (:240-284)
**Pattern extraction date:** 2026-10-04（HEAD dede743）
