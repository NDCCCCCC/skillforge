---
phase: quick-260928-nzk
verified: 2026-09-28T12:04:10Z
status: passed
score: 5/6 must-haves verified (1 deferred — blocked by pre-existing D-NZK-DEF-01)
overrides_applied: 0
deferred:
  - truth: "POST /api/sparql with a skill that reads sql-guides returns entity individuals (positive half of D-NZK-02)"
    addressed_in: "v1.6 follow-up (D-SPARQL-05) — documented in deferred-items.md"
    evidence: "Pre-existing Phase 41 defect D-NZK-DEF-01: resolveSkillTiersFromKG (internal/server/admin_platform.go:604) passes cat_* KBCategory IRIs that Store.QueryScoped cannot match -> schema-only rows for EVERY skill. Fail-closed holds (negative probe passes); defect predates this task and affects all ABox data equally, not entities. Orchestrator-directed deferral."
human_verification: []
---

# Quick Task 260928-nzk: ECL entity extraction → KG ABox — Verification Report

**Task Goal:** 扩展 ECL 抽取器批量抽取业务实体(险种/机构)写入 KG ABox
**Verified:** 2026-09-28T12:04:10Z
**Status:** passed (1 deferred item, orchestrator-directed)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | TBox v7 with involvesInsuranceProduct / involvesOrganization / parentOrganization / entityCode approved through the human gate | ✓ VERIFIED | Live changelog: `current_version: 7`, entries v4-v7 each `who: induction:llm` adding exactly the 4 property IRIs (timestamps 09:43:34Z→09:43:59Z). KG rdf:type: 3× owl:ObjectProperty + entityCode× owl:DatatypeProperty. ONT-04 checkpoint was human-supervised during execution (no self-approval is process-level; changelog + orchestrator supervision are the available evidence). |
| 2 | LLM prompt requests entities with tables/related spec byte-identical; illegal kind / empty / >12-rune names DROPPED; exact-name dedupe keep-first | ✓ VERIFIED | `internal/knowledge/ecl/stub.go:128-147` sanitizeEntities (kind enum `legalEntityKinds`, `utf8.RuneCountInString > 12` drop, TrimSpace, keep-first dedupe). `extract.go:67-87` prompt: entities spec + 4 entity rules strictly APPENDED after the original 4 Rules bullets. `git show cd8b0ba` — zero template lines removed (additive-only). sanitize runs BEFORE ValidateExtraction (`extract.go:242`). Over-cap union dedupes via `seenEntities` (`extract.go:164,202-210`). |
| 3 | KG ABox contains entity individuals with rdf:type + rdfs:label + entityCode? + belongsToTier + parentOrganization? + note→entity edges | ✓ VERIFIED | Live KG: 65 InsuranceProduct + 55 Organization individuals. 车险 (`ent_ec27e8dd085bc`) full shape: type + label 车险 + belongsToTier `…#sql-guides`, all with provenance. 2 parentOrganization edges (→ 武汉中心支公司 `ent_e4f27761d1552`, which carries type/label/entityCode 420100/tier). Involves edges present (65 IP + 55 Org subjects covered). |
| 4 | POST /api/sparql positive visibility of entities for a sql-guides skill | ⊘ DEFERRED | Reproduced live: sql-guides query returns 23 schema-only rows, 0 entity rows — exactly D-NZK-DEF-01 (pre-existing Phase 41 resolver defect, fail-closed, affects ALL ABox rows). Verified via the admin KG query path instead (truth 3). Blocked-by-DEF-01, deferred to D-SPARQL-05 (v1.6) per orchestrator instruction; NOT a gap of this task. |
| 5 | TestStubVsGoldenF1 green at baseline; SHACL violation count does not increase | ✓ VERIFIED | Re-ran verbose: PASS, macro tables P=0.9780 R=1.0000 F1=0.9846 / related 1.0 (exact baseline). Live SHACL: `error_count: 7`, quarantined 7 — all pre-existing `sub_sql-guides_*` rows (identical to baseline claim). |
| 6 | Re-running ApplyNote for the same note converges: zero duplicate entity individuals | ✓ VERIFIED | Code: entity-owned triples (S prefix `ent_`) via InsertUnique, involves-edges via Insert (`abox.go:333-347`); sweep with parent-target guard (`abox.go:319-326`). Live: 65/65 + 55/55 distinct ent_ subjects (0 duplicate type triples); 119 labels = 119 distinct subjects = 119 distinct names (zero fusion). |

**Score:** 5/6 truths verified + 1 deferred (truth 4, blocked by pre-existing D-NZK-DEF-01)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | SPARQL positive entity visibility (positive half of D-NZK-02) | v1.6 / D-SPARQL-05 | `deferred-items.md` D-NZK-DEF-01: `resolveSkillTiersFromKG` passes `cat_sql-guides` where `QueryScoped` matches Tier names → all ABox rows hidden for every live skill. Fail-closed negative probe HOLDS (calculator sees zero ABox rows — verified live). Defect predates this task (Phase 41 surface, fail-closed direction); fix changes trust-boundary resolution semantics — human decision documented. |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/knowledge/ecl/stub.go` | ExtractedEntity + Extraction.Entities + sanitizeEntities gates | ✓ VERIFIED | ExtractedEntity:48-53, Entities `entities,omitempty`:63, sanitizeEntities:128-147 — substantive, wired |
| `internal/knowledge/ecl/extract.go` | Prompt entities spec appended + llmExtractionShape.Entities + sanitize-before-validate + union dedupe | ✓ VERIFIED | :77-84 appended spec, :95 shape field, :242 sanitize call, :164/:202-210 union dedupe |
| `internal/knowledge/ecl/entity_iri.go` | EntityIRI deterministic (ASCII PascalCase / CJK sha256[:12]) | ✓ VERIFIED | :59-71; stdlib only; godoc explains SanitizeLocal insufficiency |
| `internal/knowledge/ecl/abox.go` | Entity triple shapes + ApplyNote capture/sweep/InsertUnique/onChange-once | ✓ VERIFIED | 4 predicate helpers :66-88, assignEntityIRIs ordinals :105-125, BuildTriples entities :201-237, ApplyNote :278-377 |
| Test files (entities_test.go 229L, entity_iri_test.go 85L, abox_test.go 715L, extract_test.go 290L) | RED→GREEN coverage | ✓ VERIFIED | 28 test functions; all green under WSL CGO hermetic run |

Commits `5ca04ef` / `cd8b0ba` / `f2fe4a8` / `01d4a48` all verified in git history with matching messages.

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| extract.go extractionPromptTmpl | llmExtractionShape.Entities | sanitizeEntities before ValidateExtraction | ✓ WIRED | extract.go:242 |
| abox.go ApplyNote | store.Store InsertUnique | entity-owned triples InsertUnique, involves-edges Insert | ✓ WIRED | abox.go:333-347 |
| entity belongsToTier object | {ns}{note.Tier} Tier IRI | server-derived, never LLM text | ✓ WIRED | abox.go:221 `O: ns + note.Tier`; live: 119/119 ent_ tier triples have IRI objects, single distinct tier `sql-guides`, 0 non-IRI |
| pipeline → KG | live entity individuals | real extraction with provenance | ✓ WIRED (Level 4 data-flow) | Entity triples in live KG carry provenance records — written through the gated pipeline, not hand-seeded |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Hermetic build + ecl/ontology tests | WSL `go build ./... && CGO_ENABLED=1 go test ./internal/knowledge/ecl/... ./internal/ontology/... -count=1` | exit 0 — ecl, emitter, induction, shacl, store all `ok` | ✓ PASS |
| Golden F1 gate | `go test -run TestStubVsGoldenF1 -count=1 -v` | PASS; tables F1 0.9846 / related 1.0 | ✓ PASS |
| TBox v7 + property types | GET changelog + kg/query per property | current_version 7; 3 ObjectProperty + 1 DatatypeProperty | ✓ PASS |
| Entity counts | kg/query rdf:type per kind | 65 InsuranceProduct, 55 Organization | ✓ PASS |
| Entity shape | kg/query s=ent_ec27e8dd085bc (车险) | type + label + belongsToTier(IRI) + provenance | ✓ PASS |
| Idempotency | kg/query counts + distinct-subject analysis | 65/65, 55/55, 119/119 distinct; 0 duplicates | ✓ PASS |
| SHACL no-regression | GET /api/admin/governance/shacl | error_count 7, quarantined 7 (all pre-existing sub_sql-guides_*) | ✓ PASS |
| SPARQL negative isolation (D-NZK-02 fail-closed) | POST /api/sparql X-Skill-Name: calculator | 0 entity rows, 0 ABox rows (schema-only 23) | ✓ PASS |
| SPARQL positive visibility | POST /api/sparql X-Skill-Name: sql-guides | schema-only 23 rows, 0 entity rows | ⊘ DEFERRED (D-NZK-DEF-01) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| D-NZK-01 | 260928-nzk-PLAN | 实体身份 = 精确名匹配 (exact-name dedupe, deterministic IRI) | ✓ SATISFIED | sanitizeEntities keep-first; EntityIRI deterministic; live 119 distinct labels |
| D-NZK-02 | 260928-nzk-PLAN | belongsToTier 继承 note tier; fail-closed SPARQL | ✓ SATISFIED (negative half) / ⊘ positive half deferred (DEF-01) | abox.go:221; calculator probe 0 rows; positive blocked pre-existing |
| D-NZK-03 | 260928-nzk-PLAN | 4 TBox 属性走提案闭环 + 人工 approve | ✓ SATISFIED | Changelog v4-v7, 4 approved proposals with pinned local names |
| D-NZK-04 | 260928-nzk-PLAN | entities 合同 + drop-gates + parent resolution | ✓ SATISFIED | stub.go:48-147; extract.go; parent same-batch + second-chance store check (abox.go:349-370) |
| D-NZK-05 | 260928-nzk-PLAN | ABox 三元组形状 | ✓ SATISFIED | BuildTriples:201-237; live 车险/武汉中心支公司 shapes match spec exactly |
| D-NZK-06 | 260928-nzk-PLAN | golden set eval 门重跑 | ✓ SATISFIED | 0.9846 / 1.0 at baseline, re-run this verification |
| D-NZK-07 | 260928-nzk-PLAN | SHACL 最小同步, 零新增违规 | ✓ SATISFIED | error_count 7 → 7 unchanged; new classes contribute nothing |

No orphaned requirements — all 7 plan requirements map to implemented, verified behavior.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | TBD/FIXME/TODO/HACK/PLACEHOLDER/stub-returns scanned across all 8 phase files | — | Zero hits |

### Human Verification Required

None — all verification items were programmatically checkable and passed. The ONT-04 human approve gate was already executed during the task (blocking checkpoint completed with orchestrator supervision).

### Gaps Summary

No gaps. All six must-have truths either verified (5) or deferred by orchestrator direction (1 — SPARQL positive visibility, blocked by the pre-existing Phase 41 D-NZK-DEF-01 resolver defect that hides ALL ABox rows from every live skill; the fail-closed security half holds and was verified live). Code, tests, live KG data, TBox state, golden F1 gate, SHACL gate, and idempotency evidence are all consistent with the SUMMARY claims — every executor claim cross-checked against reality held.

---

_Verified: 2026-09-28T12:04:10Z_
_Verifier: Claude (gsd-verifier)_
