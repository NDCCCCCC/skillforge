# Quick Task 260928-nzk: 扩展 ECL 抽取器批量抽取业务实体(险种/机构)写入 KG ABox - Context

**Gathered:** 2026-09-28
**Status:** Ready for planning

<domain>
## Task Boundary

扩展 ECL 抽取管线,从 sql-guides 语料批量抽取业务实体(险种/机构)写入 KG ABox。TBox v3 已有 `skf:InsuranceProduct` + `skf:Organization` 两个类(2026-09-28 经提案闭环批准,prop-0001/0002)。

语料证据(已核实):
- 险种词表: 车险×211、农险×66、健康险×36、意外险×15、学平险×6、雇主责任险×5、团客非车×2、家财险×1
- 险种代码 = SQL `product_code` 列(alias 险种代码),语料无系统化代码→名称映射表
- 机构层级信号: 分公司×120、部门组×111、科室×54、中心支公司×44、支公司×24、总公司×3(主要在文档标题与正文)
- 现有抽取合同: `extractionPromptTmpl`(extract.go:61)输出 `{tables, related_notes}`,去重惯例 = 按名保留首个

</domain>

<decisions>
## Implementation Decisions (2026-09-28 讨论锁定)

### D-NZK-01: 实体身份 = 精确名匹配
同名合并(与 tables 按名去重、related_notes 按 source_idx+relation 去重惯例一致)。车险/汽车险 视为两个实体。不做别名归一(无别名表维护成本)。

### D-NZK-02: 实体 belongsToTier = 继承来源文档 tier
实体 individual 发 `belongsToTier <来源note的tier>`。tier 过滤查询可见,D-TIER-03 fail-closed 语义自然覆盖,SPARQL 换 skill 视角能看到实体。

### D-NZK-03: TBox 全量 4 属性,走提案闭环(ONT-04 不可绕过)
本期需人工 approve 4 个新属性提案(LLM generate → 人工 custom_local_name 改名 → approve):
- `skf:involvesInsuranceProduct` (ObjectProperty: NoteChunk → InsuranceProduct,中文 涉及险种)
- `skf:involvesOrganization` (ObjectProperty: NoteChunk → Organization,中文 涉及机构)
- `skf:parentOrganization` (ObjectProperty: Organization → Organization,中文 上级机构)
- `skf:entityCode` (DatatypeProperty: 实体业务代码,xsd:string)
Executor 负责调 generate API 产出提案;**approve 步骤必须暂停交人工**(AskUserQuestion 确认 custom_local_name 后由 executor 代提交,或用户在 UI 点)。

### D-NZK-04: 抽取合同扩展
`Extraction` JSON 增 `entities` 字段:
```json
{"tables": [...], "related_notes": [...],
 "entities": [{"name": "<≤12字实体名>", "kind": "InsuranceProduct" | "Organization", "code": "<可选业务代码>", "parent": "<可选上级机构名,仅Organization>"}]}
```
校验门(沿用反幻觉哲学):
- kind 枚举硬校验(仅两个已批准类),非法 kind 丢弃该条
- name 非空、≤12 rune、去首尾空白;精确名去重(keep first)
- parent 必须引用同批次或已有 Organization individual,否则丢弃 parent 关系(保留实体)
- prompt 中 tables/related_notes 部分保持**原文不动**(降低 tables F1 扰动风险),仅追加 entities 字段说明

### D-NZK-05: ABox 三元组形状
每个实体 individual(note 级聚合后):
- `<iri> rdf:type skf:<Kind>` (IRI = ns + sanitized local name;PascalCase/拼音化由确定性函数生成,冲突时追加序号)
- `<iri> rdfs:label "车险"`
- `<iri> skf:entityCode "01"` (仅当抽到)
- `<iri> skf:belongsToTier <source note tier individual IRI>` (D-NZK-02)
- `<iri> skf:parentOrganization <parentIri>` (仅 Organization)
- `<noteIri> skf:involvesInsuranceProduct <iri>` / `<noteIri> skf:involvesOrganization <iri>` (note→实体边)
ApplyNote 顺序沿用 validate-before-mutate → capture refs → DeleteSubject(note) → 孤儿清扫 → Insert/InsertUnique → onChange;实体 individual 用 InsertUnique(跨 note 幂等),note→实体边随 note 重建。

### D-NZK-06: eval 门重跑
执行完跑 golden set eval(39-04 门: macro tables F1 ≥ 0.8, related F1 ≥ 0.8)。entities 字段不参与打分,但 prompt 变更后 tables/related 抽取质量不得退步。GLM 额度已充裕。

### D-NZK-07: SHACL 最小同步
本期**不**为两个新类新增 NodeShape(未声明 shape 的类不参与校验,零违规)。但必须验证:新 ABox 不触发既有全局 shapes 违规(尤其 belongsToTier → Tier 全局约束:实体 tier 继承发的 object 必须是已存在的 skf:Tier individual)。SHACL 完整 shapes 留 v1.6。

### Claude's Discretion
- 实体 IRI local-name 生成函数的具体实现(拼音/PascalCase/序号消歧)
- entities 抽取的 few-shot 示例措辞
- manager 并发/重试对实体抽取的复用方式(沿用 extract worker pool ≤3)
- 提案 generate 的 text 措辞(需含 D-NZK-03 四属性的中英文语义)

</decisions>

<specifics>
## Specific Notes

- 抽取 prompt 模板在 `internal/knowledge/ecl/extract.go:61`(extractionPromptTmpl);Extraction struct 在 :79-80
- stub.go:57 `tableNamePattern = ^etl_\w+$` 反幻觉门只管 tables;entities 门单独实现(D-NZK-04)
- ABox 形状参考 abox.go BuildTriples(现有 note/table 三元组);ApplyNote 顺序 pin 在 STATE.md 39-04 决策
- 实体 visibleSubjects 判定走 belongsToTier → store.go:528-536 现有机制,无需改 store
- SHACL belongsToTier 全局 shape: object 必须 skf:Tier(shapes.ttl:30-34)——实体继承 tier 时 object 用 Tier individual IRI(ns+tierName),不是字面量
- eval 门: cmd/eval 或 eval/ 侧;39-09 政策 A writes-only,macro tables 0.9846 / related 1.0 是当前基线
- 端到端验证: 重跑 ECL 管线 → KG 查询 rdf:type skf:InsuranceProduct 能看到 车险/农险/... individuals → SPARQL 换 skill 视角验证 tier 可见性

</specifics>

<canonical_refs>
## Canonical References

- `internal/knowledge/ecl/extract.go` — 抽取合同 + prompt 模板 + LLM 调用形状
- `internal/knowledge/ecl/stub.go` — stub 抽取器(表名合同)
- `internal/knowledge/ecl/abox.go` — BuildTriples 现有三元组形状
- `internal/knowledge/ecl/manager.go` — ApplyNote 顺序 + worker pool + onChange
- `internal/ontology/induction/proposal.go` — Proposal 结构(kind/local_name/custom_local_name)
- `internal/ontology/shacl/shapes.ttl` — 全局 property shapes(belongsToTier 约束)
- `.planning/STATE.md` § Decisions 39-04 — ApplyNote ordering pin + F1 conventions
</canonical_refs>
