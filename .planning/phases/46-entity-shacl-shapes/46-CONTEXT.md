# Phase 46: Entity SHACL Shapes - Context

**Gathered:** 2026-10-04
**Status:** Ready for planning
**Milestone:** v1.6 — SPARQL Schema Detection + Entity SHACL Shapes

<domain>
## Phase Boundary

给实体类 ABox（`InsuranceProduct` 险种 / `Organization` 机构——kind 枚举就这两类，D-NZK-04）交付 SHACL shapes（必填属性 + cardinality 级约束），boot 时经既有 quarantine 分流接受校验：违规实体进隔离区、不进主索引，boot 不阻断（D-SHACL-02 保持）；`/admin/governance` 校验报告可呈现实体违规且与 TBox 层可区分；`verify-phase-46.sh` 全绿。

**Requirements:** SHACL-01（shapes 随仓库交付 + 路径可配置零硬编码）、SHACL-02（启动校验覆盖实体 shapes + quarantine 流 + boot 存活）、SHACL-03（governance 报告分层呈现，涉 UI 先 UI-SPEC）。

**讨论中升级为范围内（顺手修复）：** `induction.go:383` 的 D-PROV-09 rollback 重校验硬编码了不存在的 `data/shacl/shapes.ttl` 路径——rollback 重校验目前在静默 no-op 且无视 `SKILLFORGE_SHACL_SHAPES`。D-46-02 将它与 boot 的 shapes 加载统一到同一 env，属 SHACL-01「路径可配置」标准的既有违规修复，非新能力。

**In scope:** 实体 NodeShapes（新 entities.ttl）+ 引擎 `sh:severity` 支持（validate.go）+ shapes 加载路径列表化（main.go + induction.go）+ Violation 分层盖戳（validate.go DTO）+ FE 徽章/区头计数（governance 三文件）+ `46-UI-SPEC.md`（UI-SPEC 先行，hard constraint）+ dangling_edge_count 只读字段 + verify-phase-46.sh。
**Out of scope:** 级联隔离入边（deferred）、entityCode 必填/黄灯（语料成熟后再议）、SPARQL 1.1 parser、D-SPARQL-07 comment hardening、emitter 谓词扩充、图谱空心节点渲染重设计、隔离区自动修复策略（40-CONTEXT 既有 deferred）。

</domain>

<decisions>
## Implementation Decisions

### 约束强度（Area 1 — 根决策）
- **D-46-01（C2 安静全绿 — severity 政策进 shapes 文件）：**
  - **引擎**：`validate.go` 的 `parsePropertyShape` 新增 `sh:severity` 解析分支；`:428-434` 的硬编码推导（`path==sourcePath→warning, else error`）替换为「解析值或默认 error」。W3C 语义映射：`sh:Violation`（未声明时的默认）→ error 隔离，`sh:Warning`/`sh:Info` → warning 放行——D-SHACL-02 语义保持。默认=error 保证既有 8 个 TBox shapes 行为字节不变。
  - **实体 shapes 约束清单**：只把提取器无条件保证的属性设为必填@error——`rdf:type` hasValue 自检 + `rdfs:label` minCount 1（abox.go:210-211 无条件发出）；`skf:entityCode` / `skf:parentOrganization` / `skf:belongsToTier` 一律 **if-present 质量检查**（datatype/class，无 minCount）。live 数据 127 实体（71 险种 + 56 机构）首跑**零隔离、health=ok**。
  - **否决业务理想必填的原因（数据实证）**：`entityCode` 仅在 LLM 找到编码时发出（abox.go:213-215，live 缺失 88-89%）、`parentOrganization` 仅同批精确名匹配（abox.go:232-236，缺失 98% 且根机构语义性无父级）——minCount 1 @ error 会在首跑隔离 112/127 个合法实体，governance 永红、3D 图谱/ontology 视图/hybrid retrieval 同步缩水。
  - **sourcePath 特例退役**：D-PROV-10 的 `path==sourcePath→warning` 引擎硬编码（一次性特例，无法扩展）退役——在 shapes.ttl 的两个 sourcePath PropertyShape 上显式声明 `sh:severity sh:Warning`，行为不变、政策归位数据。
  - **否决 C1（entityCode minCount 1 @ warning）的原因**：112 条常驻警告把 D-SHACL-03 三色映射的「黄」钉死——黄区现在是 provenance-missing（Run ECL 可行动项）的专属语义，永黄 = 告警疲劳。

### Shapes 文件组织（Area 2）
- **D-46-02（b 路径列表 — 延伸既有机制 + 修复 rollback no-op）：**
  - 实体 shapes 独立文件 `internal/ontology/shacl/entities.ttl` 随仓库交付（与 shapes.ttl 同目录同家族；TBox/ABox 生命周期分离，运营者可从 env 列表去掉一条路径即独立禁用实体校验）。
  - `SKILLFORGE_SHACL_SHAPES` 延伸为**路径列表**（`filepath.SplitList`，os 路径列表分隔符）；默认值 = 既有 shapes.ttl + 新 entities.ttl。这是 ROADMAP SC1「延伸既有 SKILLFORGE_SHACL_SHAPES 机制」的字面落地。运行时是 WSL Linux（env-wsl-backend-runbook），无 Windows `;`/`:` 分隔符歧义，文档写明即可。
  - **`induction.go:383` rollback 重校验改读同一 env**（当前硬编码 `data/shacl/shapes.ttl`——目录不存在，warn-and-return-0 静默 no-op）：修复 D-PROV-09 rollback/boot shapes parity 缺口，属零硬编码标准的既有违规修复。
  - 多文件 parse 的错误聚合不得静默跳过缺失路径（「不允许静默吞错」标准）；`Report.ShapesPath` 保持单 string（join 显示），FE 类型零变化。
  - `shapes.ttl:4` 陈旧头注释（声称默认 `data/ontology/shacl.ttl`，与 main.go:930 实际不符）一并修正。
  - **否决 (a) 单文件追加**：实体校验无法独立禁用（env 是全有全无的逃生门）；**否决 (b2) 第二 env**：偏离 ROADMAP 字面 + rollback 需正确组合两个 env（组合漂移风险）。

### 违规分层呈现（Area 3 — SHACL-03）
- **D-46-03（A 后端盖戳）：**
  - 校验循环在 stamping 点（validate.go:120-126，`v.Entity` 赋值处）给 `Violation` 新增 `target_class` + `layer` 字段（`omitempty`）：`ns.TargetClass` 循环内现成可用，确定性派生、可测试。layer 枚举 = `"tbox"` | `"entity"`；`checkGlobalReferences`（:139-146/:694-718）的跨实体完整性违规用 `"global"` 哨兵，避免按实体误分类。
  - UI = **标签问题而非结构问题**：红/黄区已按实体行组织（ShaclReportView.tsx :107-145 / :173-206）——每行违规加 layer/class 徽章 + 红黄区头部 per-layer 计数；**不加新分区、不加分页 tab**（D-SHACL-03 三色区语义不动）。
  - FE 咽喉点 null-guard（governanceApi.ts:57-60，260928-jxb 治理白屏先例）——报告虽为 in-memory 每次boot 重生成（store.go:251-262，无陈旧兼容问题），null-guard 先例仍是红线。
  - **`46-UI-SPEC.md` 先行（hard constraint）**：仓库惯例是每 web phase 独立全量 UI-SPEC（10/10 先例，无 delta-spec 先例）；内容精简——Design System/Color/A11y 引用 40-UI-SPEC.md，正文只 spec 徽章、区头计数文案（实体违规 vs TBox 违规）、global 哨兵显示；显式声明与 40-UI-SPEC §ShaclReportView 冲突时以本 spec 为准（v0.7.7 审计教训：spec 漂移成审计发现）。
  - **否决 FE-only 分组**：`Violation.entity` 是实例 IRI（`skf:note_N`）非类名，前缀启发式 = `OntologyGraphView.tsx:21-25` 已登记的硬编码债同款，违反零硬编码。

### 隔离的悬挂边（Area 4）
- **D-46-04（a+c 保持 + 计数）：**
  - boot 隔离循环（main.go:937-941）**subject-only 保持不动**——「接入既有 quarantine 流」的范围纪律。SC2「不出现在正常 KG 查询结果中」在 a+c 下正式解读为 **subject 位置不可见**；对象位悬挂边可见但带计数（见下）。
  - 报告新增只读 `dangling_edge_count`（每 quarantined entity）：对入边谓词 `involvesInsuranceProduct` / `involvesOrganization` / `parentOrganization` 反向 Query 计数，governance UI 展示——满足运营者信任需求，零机制风险。
  - **否决级联隔离的原因（机制实证）**：`store.Quarantine` 按 `t.S` 键控（store.go:196）——级联 note 的 involves 边会误把**健康 note** 记入 `QuarantinedEntities()` 并污染 TripleCount；破坏记录在案的恢复路径（40-CONTEXT：Run ECL 经 abox.go:363-369 二次父解析要求父级 rdf:type 在主索引）；隔离后 debounced snapshot 只序列化 `s.all`（store.go:370-374）会把删除永久化。与 D-TIER-06「图不碎裂」精神相悖；40-CONTEXT 已把「隔离区自动修复策略」归为未来增强。
  - 症状定位（研究者取证）：悬挂边是**视觉混淆**（ontology 图空心 IRI 节点/无 tier、SPARQL 对象位空行），非数据损坏；provenance 视图不渲染 involves 边，hybrid retrieval 只走 referencesTable 不受影响。

### Claude's Discretion
- env 默认列表值的确切形态、多文件 parse 错误聚合的具体结构（约束：不静默吞缺失路径）。
- entities.ttl 内部组织（沿用 shapes.ttl 的分节注释风格）与 PropertyShape 命名。
- `dangling_edge_count` 字段落点（`QuarantinedEntity` struct 内 vs Report 顶层）与入边谓词枚举的实现位置（Go 常量 vs shapes 派生；零硬编码标准约束的是 IRI 命名空间与 shapes 路径，不强制本 helper）。
- layer 枚举确切字符串、徽章样式与区头文案细节（46-UI-SPEC / ui-phase 域）。
- SC2 注入测试的形态（单测 fixture KG vs boot 级测试）与 verify-phase-46.sh 的测试编组（照 45 模式：fail-fast + anti-vacuous PASS 计数；shacl 包纯 Go 可 CGO_ENABLED=0，internal/server 测试需 WSL CGO_ENABLED=1）。
- 首跑全绿的断言方式（建议：以 BuildTriples 构造性保证为据做 hermetic fixture 断言，live 快照核对留 operator 域）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap / Requirements
- `.planning/ROADMAP.md` Phase 46（:101-118）— Goal + SC1-4 + UI hint: yes
- `.planning/REQUIREMENTS.md` :15-19 — SHACL-01/02/03 需求原文

### 信任语义与既有 SHACL 决策（LOCKED，不可削弱）
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-CONTEXT.md` — D-SHACL-01..04（引擎/分级隔离/三色/shapes 全覆盖）、D-PROV-09（rollback 重校验）、D-PROV-10（sourcePath warning）+ `<deferred>` 隔离区自动修复
- `.planning/phases/45-sparql-schema-visibility-d-sparql-05/45-CONTEXT.md` — D-45-01..05（rdfs:label 非 schema passthrough 的判定语义——实体 label 在 tier 视角不可见的背景；SHACL boot 校验走全量 Query 无交互）
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md` — 46-UI-SPEC 的引用基座（Design System/Color/A11y + §ShaclReportView）

### 引擎与装配（实现落点）
- `internal/ontology/shacl/validate.go` — 引擎全文：`parsePropertyShape` :333-437（sh:severity 分支插入点 + :428-434 待退役的 sourcePath 硬编码）、stamping 循环 :118-135（layer 盖戳点）、global refs :139-146/:694-718（global 哨兵）、Report/Violation structs :55-83、`parseShapes` 单路径假设 :218-331（列表化改造点）
- `internal/ontology/shacl/shapes.ttl` — 既有 8 TBox NodeShapes（约束风格参照）+ :4 陈旧头注释（修正点）+ :192-197/:215-220 sourcePath shapes（补显式 sh:severity）
- `cmd/server/main.go:930-957` — env 读取 + Validate + quarantine 分流 + SetShaclReport boot 链
- `internal/ontology/induction/induction.go:383` — rollback 重校验硬编码（D-46-02 修复点；:386-388 warn-and-return-0 no-op 路径）
- `internal/ontology/store/store.go` — Quarantine :196（subject 键控）、QuarantinedEntities :237-249、report cache :251-262、Snapshot :370-374（只序列化 s.all——级联否决的关键证据）

### 实体侧（shapes 的约束对象）
- `internal/knowledge/ecl/abox.go:165-250` — BuildTriples：实体 triple 生成条件（label 无条件 :210-211 / entityCode 条件 :213-215 / belongsToTier :220-222 / parentOrganization 同批匹配 :232-236）；:333-347 Run ECL 重建 involves 边（恢复路径）；:363-369 二次父解析前提
- `internal/knowledge/ecl/stub.go:99-105` — legalEntityKinds 枚举（kind 集合 = {InsuranceProduct, Organization} 的真源）
- `data/ontology/skillforge_tbox.ttl:253-273` — TBox v7：InsuranceProduct/Organization 类 + involvesX/parentOrganization 属性 + entityCode DatatypeProperty
- `data/ontology/skillforge_kg.json` — live 快照（71 险种 + 56 机构 = 127 实体；研究者计数依据）

### FE（分层呈现落点）
- `web/src/components/admin/governance/ShaclReportView.tsx` — 红区 :89-153 / 黄区 :156-211（徽章+区头计数的插入点）
- `web/src/components/admin/governance/types.ts:12-46` + `governanceApi.ts:57-60` — DTO 镜像 + null-guard 咽喉点（260928-jxb 白屏先例）
- `web/src/components/graph/OntologyGraphView.tsx:21-25` — 硬编码 kind 反先例（FE-only 方案否决依据）

### 读取路径取证（悬挂边决策依据）
- `internal/server/admin_platform.go:189-252` — SparqlQueryScoped：可见性只查 triple 的 subject（悬挂入边在对象位可见的机制）
- `internal/server/admin_governance.go:45-63` — shacl handler（503-if-nil）；:137/:187-201 provenance 视图边循环走主索引

### 验证参照
- `.planning/phases/45-sparql-schema-visibility-d-sparql-05/45-02-PLAN.md` 附近 + `verify-phase-45.sh` — verify 脚本分区/fail-fast/anti-vacuous 模式与 WSL CGO 命令链
- `verify-phase-40.sh:35` — shacl 包测试编组先例（`go test ./internal/ontology/shacl/...`）

### 环境约束
- `env-wsl-backend-runbook`（memory）— WSL Go 构建；CGO_ENABLED=1 需 Linux gcc emutls ABI
- `env-intranet-service-flakiness`（memory）— 全套件 hermetic，无 live LLM/Milvus 依赖

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **7-constraint 引擎 switch**（D-SHACL-01 哲学）：`sh:severity` = 同量级的一个 parse 分支 + 推导替换（~15-20 行单文件）；severity 已从 `propertyConstraint` 流入全部 `check*` 函数（:214/:503/:518…），解析后自动生效
- **parsedShapes 全 slice 字段**（:186-196）：多文件 merge = 纯 append，列表化改造成本低
- **报告 in-memory 缓存**（store.go:251-262）：每次 boot/rollback 由同一 binary 重生成——DTO 加字段无陈旧兼容问题
- **BuildTriples 构造性保证**：hermetic fixture KG（BuildTriples 产物 + 注入违规实体）即可断言首跑全绿 + SC2 隔离分流，无需 live 依赖
- **governanceApi.ts:57-60 null-guard 咽喉点** + 40-UI-SPEC 引用基座

### Established Patterns
- severity 单字段流经全部 check*（引擎改动面天然收敛）
- boot 顺序 LoadSnapshot → SHACL validate → quarantine 分流 → SetShaclReport（新步骤不进链，只扩 validate 语义）
- 判定/机制单点 + docstring 决策编号引用（D-TIER-03/D-SPARQL-03/D-SHACL-02 先例）
- verify 脚本：fail-fast 分区 + 正则 -run 配对最低 --- PASS 计数（anti-vacuous，45 模式）
- FE snake_case DTO 镜像 per 40-UI-SPEC；jxb null-guard 先例

### Integration Points
- `SKILLFORGE_SHACL_SHAPES`：boot（main.go:930）与 rollback（induction.go:383）两个消费点统一到同一 env 列表
- `Violation` DTO：validate.go → admin_governance.go handler（透传）→ types.ts → ShaclReportView（徽章/区头）
- `QuarantinedEntity` struct（validate.go:66-70 + store.go:230-233）：dangling_edge_count 候选落点
- entities.ttl 与 shapes.ttl 同目录：单 binary 仓库交付，env 缺省即可用

</code_context>

<specifics>
## Specific Notes

- **live 数据已过 ROADMAP 数字**：快照现为 71 险种 + 56 机构（127 实体；ROADMAP 写的 65/38 是 v1.5 时点）。任何「先跑一遍看隔离多少」的验证都该按 127 口径。
- **SC2 验收语义（D-46-04 下的正式解读）**：「违规实体不出现在正常 KG 查询结果中」= subject 位置不可见（QueryScoped 可见性按 subject 判定，store.go:564）；对象位悬挂入边可见 + `dangling_edge_count` 披露。planner 写断言时按此口径，勿写「对象位也不可见」的过度断言。
- **引擎改动的回归红线**：默认 severity=error 必须保证既有 8 TBox shapes 行为字节不变（shapes.ttl 上显式补 sourcePath 的 `sh:severity sh:Warning` 后，sourcePath warning 行为保持）；Phase 40 shacl_test.go 全部既有断言一条不能动。
- **rollback parity 测试**：D-46-02 修复后需断言 rollback 重校验真的读到 env 列表（当前 no-op 无测试覆盖——修完后加回归）。
- **UI-SPEC 顺序**：`/gsd:ui-phase 46` 必须在 `/gsd:plan-phase 46` 之前（v1.4 hard constraint，涉 web）。
- 研究者证据行号已内嵌各 decision——planner 实现前应逐行核对，勿直接信任本清单（同 45-CONTEXT 惯例）。

</specifics>

<deferred>
## Deferred Ideas

- **级联隔离（object-keyed 抑制机制）** — 仅当 SC2 被正式重释为「对象位也不可见」且接受新隔离机制 + 恢复语义重设计时再立项（D-46-04 否决记录在案）。
- **C1（entityCode minCount 1 @ warning 黄灯）** — 语料重抽取/编码覆盖率上来后再考虑；届时 112 条警告的告警疲劳前提可能消失。
- **skf:layer shapes 标注（Area 3 Option C）** — 仅当 layer 分类学超出 tbox/entity/global（如 per-Skill shapes 层）再考虑。
- **图谱空心节点渲染修复** — 被隔离实体的入边在 ontology/3D 图上呈空心 IRI 节点；FE 打磨候选，非本 phase。
- **入边谓词枚举 shapes 化** — dangling 计数若需泛化到任意关系属性，再考虑用 global property shapes 派生枚举（当前 Go 常量即可）。

None of these arose as unrequested scope creep — 讨论全程在 phase boundary 内，修复 induction.go:383 属零硬编码标准的既有违规。

</deferred>

---

*Phase: 46-entity-shacl-shapes*
*Context gathered: 2026-10-04 via advisor-mode research (4 parallel researchers, code-verified)*
