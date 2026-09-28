---
phase: quick-260928-nzk
plan: "01"
subsystem: knowledge/ecl + ontology
tags: [ecl, kg, abox, entities, tbox, ontonlogy-proposals, sparql, tier-isolation]
requires:
  - TBox v3 (skf:InsuranceProduct + skf:Organization, prop-0001/0002, approved 2026-09-28)
  - ONT-04 human proposal gate (POST /api/admin/ontology/proposals/*)
  - Phase 39 ECL extraction contract (extractionPromptTmpl, ValidateExtraction)
  - Phase 40 ApplyNote ordering pin (STATE 39-04) + D-TIER-02 belongsToTier emission
  - Phase 41 SPARQL endpoint (/api/sparql, X-Skill-Name tier isolation, D-TIER-03)
provides:
  - "TBox v7: skf:involvesInsuranceProduct / skf:involvesOrganization / skf:parentOrganization / skf:entityCode (all human-approved via prop-0003..0006)"
  - "Extraction contract entities field with anti-hallucination drop-gates (kind enum, ≤12-rune name, exact-name dedupe keep-first)"
  - "Entity-aware extraction prompt (tables/related spec byte-identical, entities spec appended)"
  - "EntityIRI deterministic local names (ASCII PascalCase / CJK sha256[:12]) + collision ordinals"
  - "ABox entity triple emission with InsertUnique idempotency, extended orphan sweep with parent-target guard, second-chance parent resolution"
  - "Live KG entity individuals (65 InsuranceProduct + 55-57 Organization) tier-gated via inherited note tier"
affects:
  - internal/knowledge/ecl (extractor contract)
  - KG ABox shape (new ent_* individuals)
  - /api/sparql consumers (entity individuals once D-NZK-DEF-01 is resolved)
tech-stack:
  added: []
  patterns:
    - "Entry-wise entity drop-gates (sanitizeEntities) vs whole-extraction hard error (ValidateExtraction) — D-NZK-04 by design"
    - "Deterministic entity IRI = identity (D-NZK-01): same exact name ⇒ same IRI ⇒ cross-note InsertUnique dedupe"
    - "Entity-owned triples InsertUnique vs note-owned edges Insert in ApplyNote (extends the table precedent)"
key-files:
  created:
    - internal/knowledge/ecl/entity_iri.go
    - internal/knowledge/ecl/entities_test.go
    - internal/knowledge/ecl/entity_iri_test.go
  modified:
    - internal/knowledge/ecl/stub.go
    - internal/knowledge/ecl/extract.go
    - internal/knowledge/ecl/abox.go
    - internal/knowledge/ecl/abox_test.go
decisions:
  - "D-NZK-04 entity gates drop entries entry-wise (kind enum / ≤12 runes / trim / exact-name dedupe keep-first) — one hallucinated entity must not dead-letter a valid extraction"
  - "Entity identity = exact trimmed name (D-NZK-01): deterministic IRI (ASCII PascalCase, CJK e+sha256[:12]), same-batch collision ordinals -2/-3"
  - "Entity belongsToTier always inherits the note's server-derived tier (T-NZK-03) — never LLM-supplied; SHACL global belongsToTierShape stays satisfied"
  - "ApplyNote split: entity-owned triples InsertUnique (globally shared individuals), involves-edges Insert (note-owned); orphan sweep extended with the parent-target guard; second-chance parent via store Organization rdf:type check"
  - "Live chat provider: LOCAL Qwen endpoint (10.62.10.36:8888) was down (connection refused); launched with LOCAL_* unset → GLM (glm-5.3-flash), the provider CONTEXT D-NZK-06 presumes (额度已充裕)"
  - "D-NZK-DEF-01 (out-of-scope discovery): live SPARQL skill→tier resolution is broken for ALL skills (hasReadAccess objects are cat_* KBCategory individuals; resolver passes 'cat_sql-guides' as the tier name; QueryScoped matches Tier names) → tier-scoped queries return schema-only for every live skill. NOT fixed here (Phase 41/44-owned trust-boundary code; adjacent deferred decision D-SPARQL-05); documented in deferred-items.md for a human decision"
metrics:
  duration: "~2h20m (2026-09-28T09:39Z → ~11:59Z, including two live pipeline runs ~20 min each and one environment-forced restart)"
  completed: 2026-09-28
  tasks: 4
  files: 7
---

# Quick Task 260928-nzk: ECL entity extraction → KG ABox Summary

Batch extraction of business entities (险种 InsuranceProduct / 机构 Organization) from sql-guides notes into the KG ABox, with the 4 supporting TBox properties (v7) approved through the ONT-04 human gate — entities are now queryable, tier-gated vocabulary instead of opaque note text.

## Tasks

### Task 1 — TBox v7 via the human approve gate (checkpoint:human-verify) ✅

- Baseline `current_version: 3` recorded; 4 proposals generated one-per-property via `POST /api/admin/ontology/proposals/generate` (prop-0003 ObjectProperty 涉及险种, prop-0004 ObjectProperty 涉及机构, prop-0005 ObjectProperty 上级机构, prop-0006 DatatypeProperty 实体业务代码) — all with correct kinds, first try, status pending.
- **Execution STOPPED at the ONT-04 gate; zero self-approvals.** After the human approved the recommended custom_local_names, the 4 approve calls were submitted: versions 3→4→5→6→7, all pinned: `involvesInsuranceProduct`, `involvesOrganization`, `parentOrganization`, `entityCode` (proposed_iris `http://skillforge.local/ontology#…`).
- Verified: `/api/admin/ontology/changelog` → `current_version: 7`; all 4 proposals `approved` with the pinned local names. No commit (server-owned TTLs under gitignored `/data/`).

### Task 2 — Extractor extension (TDD) ✅

- RED `5ca04ef` → GREEN `cd8b0ba`.
- `stub.go`: `ExtractedEntity` (name/kind/code?/parent?) + `Extraction.Entities` (`entities,omitempty` — stub/golden drafter stays nil-entities, D-NZK-06) + `sanitizeEntities` (kind enum drop, empty/>12-rune drop via utf8.RuneCountInString, trim, exact-name dedupe keep-first; Parent deliberately not validated here).
- `extract.go`: entities spec + 4 new rules APPENDED to `extractionPromptTmpl` — the original tables line, related_notes line and all 4 Rules bullets are byte-identical (pinned by test); `llmExtractionShape.Entities`; sanitize BEFORE `ValidateExtraction` (tables gate semantics untouched); over-cap union dedupes entities keep-first mirroring `seenTables`.
- Golden gate: `TestStubVsGoldenF1` green at baseline — tables F1 0.9846 / related 1.0 (≥ 0.8 both).

### Task 3 — ABox emission (TDD) ✅

- RED `f2fe4a8` → GREEN `01d4a48`.
- `entity_iri.go`: `EntityIRI(ns,name) = ns + "ent_" + {ASCII ^[0-9A-Za-z_]+$ → PascalCase | else e+sha256[:12]}`; godoc explains why `emitter.SanitizeLocal` alone is insufficient (CJK collapses to `___`, fusing distinct names). stdlib only (T-NZK-SC).
- `abox.go`: 4 predicate helpers + `rdfsLabelIRI`; `assignEntityIRIs` (defense-in-depth re-sanitize + collision ordinals); BuildTriples emits per entity `rdf:type {ns}{Kind}`, `rdfs:label`, optional `entityCode`, `belongsToTier {ns}{note.Tier}` (never LLM-supplied, T-NZK-03), kind-specific note→entity edge, `parentOrganization` only for same-batch exact-name Organization matches (self-parent dropped). ApplyNote: captures involves-objects, sweep extended (no involves-edge AND no parent-target-edge ⇒ DeleteSubject; parent targets survive), entity-owned triples InsertUnique vs involves-edges Insert, second-chance parent via store `rdf:type Organization` check; STATE 39-04 ordering + onChange-once untouched.

### Task 4 — E2E live gate ✅ (one pre-existing blocker surfaced, see Deviations)

| Gate | Result |
|---|---|
| Hermetic `go build ./...` + ecl/ontology tests (CGO, WSL) | GREEN (all packages) |
| Server restart per runbook (CGO binary, .env + explicit `SKILLFORGE_MCP_SERVICE_SKILL=simple-qa`) | Boot clean; TBox v7 UpgradeIfNeeded no-op (208 TTL triples); snapshot restore clean |
| SHACL baseline → after | 7 errors / 7 quarantined → 7 / 7 (identical pre-existing `sub_sql-guides_*` rows) — zero new violations (D-NZK-07 ✅) |
| Pipeline re-run | Run 1: 25/26 (note 018 gate-rejected `wuh_dw.etl_policy`; retries exhausted). Run 3: **26/26, zero failures** |
| Entity individuals in KG (admin) | 65 InsuranceProduct + 55 Organization; labels 车险/农险/非车险/车损险/武汉中心支公司…; full shapes verified (type/label/code/tier/parent/involves edges + provenance) |
| Tables regression spot-check | note_033 → `tbl_etl_policy`; note_039 → 5 `etl_*` refs — tables extraction intact |
| Idempotency (verification 7) | After a complete second application: 0 duplicate type triples, 65/65 and 55/55 distinct IRIs, 119 labels = 119 distinct IRIs (zero fusion). Count drift (57→55 orgs) is GLM extraction variance, correctly swept |
| SPARQL negative isolation (D-NZK-02 fail-closed) | HOLDS — a skill without sql-guides sees zero entity rows |
| SPARQL positive visibility (D-NZK-02) | **BLOCKED by pre-existing D-NZK-DEF-01** (below) — affects ALL ABox data equally, not entities |

## Deviations from Plan

### Environment handling (not plan deviations, documented for operators)

1. **Chat provider switch:** the configured LOCAL Qwen endpoint (`http://10.62.10.36:8888/v1`) was down (host pingable, port refused). Run 1 dead-lettered all 26 notes with transport errors (no prompt regression — zero completions attempted). Relaunched with `LOCAL_API_KEY/LOCAL_BASE_URL/LOCAL_MODEL` unset so the GLM path (`glm-5.3-flash` via open.bigmodel.cn) is selected — the provider CONTEXT D-NZK-06 presumes ("GLM 额度已充裕"; STATE notes the prior session's 临时切 GLM). Also: `.env` has CRLF and does NOT contain `SKILLFORGE_MCP_SERVICE_SKILL` — it must be exported explicitly at launch (D-TRUST-02), and sourcing needs `source <(tr -d '\r' < .env)`.
2. **WSL server liveness:** background server processes don't survive detached `nohup` in this environment; the server runs as an attached background task (`/home/ninedrunk/skillforge-server-nzk`, log `/home/ninedrunk/sf-nzk.log`). One harness-side kill interrupted run 2 mid-extract (KG unaffected — applies are debounced-snapshotted; run 3 re-proved everything). `/tmp` proved volatile across calls; binaries/logs live under `/home/ninedrunk/`.
3. **Note 018 transient gate rejection (run 1):** GLM emitted schema-qualified `wuh_dw.etl_policy`; the D-Ext-01 corpus-contract gate rejected it 3/3 attempts (by-design anti-hallucination behavior). Run 3 extracted it cleanly — model variance, not a prompt regression (tables spec byte-identical).

### Auto-fixed Issues

None — the code compiled and behaved as planned; all RED gates failed for exactly the pinned reasons.

### Out-of-scope discovery requiring a human decision (Rule 4 — NOT fixed)

**D-NZK-DEF-01 (see `deferred-items.md` in this directory):** live `POST /api/sparql` returns schema-only rows for EVERY skill — `resolveSkillTiersFromKG` (internal/server/admin_platform.go:604) passes the `hasReadAccess` object through verbatim, and live objects are `cat_sql-guides` (KBCategory individuals), while `QueryScoped` matches Tier names (`sql-guides`). Result: tier-scoped queries hide ALL ABox rows (notes/tables too) for every live skill; fail-closed holds (nothing leaks), but the positive half of D-NZK-02 cannot be demonstrated over SPARQL until resolved. Entities ARE correctly in the KG with correct belongsToTier objects (verified via the admin KG query). Suggested minimal fix + live-wiring regression test documented in deferred-items.md; adjacent deferred decision D-SPARQL-05 (v1.6) should absorb it.

## Known Stubs

None — no placeholder data paths; every entity individual is LLM-extracted from the corpus and written through the gated pipeline.

## Threat Flags

None new — all mitigations in the plan's threat register are implemented and tested (T-NZK-01 drop-gates, T-NZK-02 hash IRIs, T-NZK-03 note-tier inheritance, T-NZK-04 human gate honored, T-NZK-SC stdlib-only).

## Verification Checklist (plan §verification)

1. ✅ WSL build + ecl/ontology tests fully green
2. ✅ `/api/admin/ontology/changelog` → current_version 7 with the 4 pinned names (entries v4..v7)
3. ⚠️ Entity individuals with label/code/tier/parent verified via the admin KG query; the `/api/sparql` positive path is blocked by pre-existing D-NZK-DEF-01 (all ABox rows, all skills)
4. ✅ Restricted skill sees zero entity triples via `/api/sparql` (fail-closed)
5. ✅ TestStubVsGoldenF1 PASS (0.9846 / 1.0, baseline preserved)
6. ✅ SHACL violations 7 → 7 (no increase)
7. ✅ Second full pipeline application: zero duplicate entity individuals (65/65, 55/55 distinct IRIs; 119/119 distinct labels)

## Commits

| Task | Commit | Content |
|---|---|---|
| 2 RED | `5ca04ef` | test(quick-260928-nzk): failing entity extraction gate tests |
| 2 GREEN | `cd8b0ba` | feat(quick-260928-nzk): entity extraction gates + prompt extension |
| 3 RED | `f2fe4a8` | test(quick-260928-nzk): failing entity ABox emission tests |
| 3 GREEN | `01d4a48` | feat(quick-260928-nzk): entity ABox emission with idempotent apply |
| 1, 4 | — | No code commits (server-side TBox data on gitignored /data/; live verification) |

## Self-Check: PASSED

All 7 created/modified key files exist on disk; all 4 task commits (2× RED, 2× GREEN) found in git log. Duration ~2h18m (09:39Z → 11:57Z).
