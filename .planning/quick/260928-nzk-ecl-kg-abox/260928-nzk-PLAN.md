---
phase: quick-260928-nzk
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/knowledge/ecl/stub.go
  - internal/knowledge/ecl/extract.go
  - internal/knowledge/ecl/extract_test.go
  - internal/knowledge/ecl/entity_iri.go
  - internal/knowledge/ecl/entity_iri_test.go
  - internal/knowledge/ecl/abox.go
  - internal/knowledge/ecl/abox_test.go
  - "data/ontology/*.ttl — TBox v4..v7, written by the live server on approve (not hand-edited)"
autonomous: false
requirements: [D-NZK-01, D-NZK-02, D-NZK-03, D-NZK-04, D-NZK-05, D-NZK-06, D-NZK-07]
user_setup: []

must_haves:
  truths:
    - "TBox v7 exists on the live server: the 4 properties involvesInsuranceProduct / involvesOrganization / parentOrganization / entityCode are APPROVED through the human gate (ONT-04, D-NZK-03) — no self-approval happened"
    - "The LLM extraction prompt requests entities (D-NZK-04) with tables/related_notes spec lines byte-identical to the old prompt; illegal kind / empty / >12-rune names are DROPPED (not errored); exact-name dedupe keep-first (D-NZK-01)"
    - "After a pipeline re-run, the KG ABox contains entity individuals with shape: rdf:type skf:InsuranceProduct|Organization + rdfs:label + optional skf:entityCode + skf:belongsToTier (inherited note tier) + skf:parentOrganization (Organization only, resolvable) + note→entity edges (D-NZK-05)"
    - "POST /api/sparql with a skill that reads sql-guides returns individuals for rdf:type skf:InsuranceProduct (车险/农险/...) and skf:Organization; a skill without the tier sees none — D-TIER-03 fail-closed preserved (D-NZK-02)"
    - "TestStubVsGoldenF1 stays green (macro tables F1 ≥ 0.8, related F1 ≥ 0.8; baseline 0.9846 / 1.0 intact) and SHACL violation count does not increase after the re-run (D-NZK-06/07)"
    - "Re-running ApplyNote for the same note converges: zero duplicate entity individuals (InsertUnique on entity-owned triples)"
  artifacts:
    - path: "internal/knowledge/ecl/stub.go"
      provides: "ExtractedEntity type + Extraction.Entities field + sanitizeEntities drop-gates (kind enum, name length, dedupe)"
      contains: "ExtractedEntity"
    - path: "internal/knowledge/ecl/extract.go"
      provides: "Extended extractionPromptTmpl (entities spec appended, tables/related lines untouched) + llmExtractionShape.Entities + over-cap union entity dedupe"
      contains: "entities"
    - path: "internal/knowledge/ecl/entity_iri.go"
      provides: "EntityIRI(ns, name) — deterministic local name: ASCII→PascalCase, CJK→e+sha256[:12], same-batch collision ordinal"
    - path: "internal/knowledge/ecl/abox.go"
      provides: "BuildTriples entity shapes + ApplyNote entity capture/sweep/InsertUnique/onChange-once"
      contains: "involvesInsuranceProduct"
  key_links:
    - from: "internal/knowledge/ecl/extract.go extractionPromptTmpl"
      to: "llmExtractionShape.Entities"
      via: "prompt appends entities JSON spec; extractBody sanitizes before ValidateExtraction"
      pattern: "sanitizeEntities"
    - from: "internal/knowledge/ecl/abox.go ApplyNote"
      to: "store.Store InsertUnique"
      via: "entity-owned triples (type/label/code/tier/parent) are globally shared individuals — InsertUnique, note→entity edges Insert"
      pattern: "InsertUnique"
    - from: "entity belongsToTier object"
      to: "{ns}{note.Tier} Tier individual IRI"
      via: "inherits the note's tier — satisfies SHACL global belongsToTierShape (object must be skf:Tier), never an LLM-supplied value"
      pattern: "belongsToTierPred"
    - from: "POST /api/sparql"
      to: "Store.QueryScoped tier filter"
      via: "X-Skill-Name header; entity visible iff its belongsToTier tier is in the skill's hasReadAccess"
      pattern: "InsuranceProduct"
---

<objective>
Extend the ECL extraction pipeline to batch-extract business entities (险种 InsuranceProduct / 机构 Organization) from sql-guides notes and write them into the KG ABox, with the 4 supporting TBox properties approved through the human proposal gate.

Purpose: notes currently reference business vocabulary (车险×211, 分公司×120...) only as opaque text; entity individuals in the ABox make that vocabulary queryable and tier-gated through SPARQL / hybrid retrieval.

Output: TBox v7 (4 approved properties), entity-aware extractor + ABox writer with tests, live pipeline re-run producing tier-gated entity individuals with golden-F1 and SHACL gates green.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/quick/260928-nzk-ecl-kg-abox/260928-nzk-CONTEXT.md
@.planning/STATE.md

Server: live on :8888 (WSL), admin token from the server env (SKILLFORGE_ADMIN_TOKEN — check .env / the WSL process env; all /api/admin/* calls need the X-Admin-Token header). TBox current version expected 3 (v3 gained skf:InsuranceProduct + skf:Organization today via prop-0001/0002).
BE builds/tests are WSL-only: `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go build ./... && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ecl/... ./internal/ontology/... -count=1'`

<interfaces>
<!-- Extracted from the codebase 2026-09-28. Use these directly — no re-exploration. -->

From internal/knowledge/ecl/stub.go (extraction contract + gates):
```go
type ExtractedTable struct { Name string `json:"name"`; Kind string `json:"kind"` }  // kind: update|select|insert|create
type RelatedNote   struct { SourceIdx string `json:"source_idx"`; Relation string `json:"relation"` }
type Extraction    struct { Tables []ExtractedTable `json:"tables"`; RelatedNotes []RelatedNote `json:"related_notes"` }
var tableNamePattern = regexp.MustCompile(`^etl_\w+$`)                                 // stub.go:57
func ValidateExtraction(ex Extraction) error                                          // tables-only hard-error gate (keep semantics)
```

From internal/knowledge/ecl/extract.go (prompt + LLM path):
```go
const extractionPromptTmpl = `...`      // :61 — output shape {"tables":[...],"related_notes":[...]}; Rules has 4 bullets
type llmExtractionShape struct { Tables []ExtractedTable `json:"tables"`; RelatedNotes []RelatedNote `json:"related_notes"` }
func (e LLMExtractor) ExtractNote(ctx, note) (Extraction, error)
  // under DefaultSectionCap(40000 runes): one call; over cap: per-section calls, union with
  // tables deduped by name keep-first, related deduped by source_idx+relation
func (e LLMExtractor) extractBody(...)  // temp 0, JSONObjectResponseFormat, llm.ExtractJSON, strict unmarshal, ValidateExtraction
```

From internal/knowledge/ecl/abox.go (ABox shapes + pinned ordering):
```go
func NoteIRI(ns, sourceIdx string) string          // {ns}note_{idx}
func TableIRI(ns, name string) string              // {ns}tbl_ + emitter.SanitizeLocal(name)
const rdfTypeIRI = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
func referencesTablePred(ns) / belongsToTierPred(ns) string   // {ns}referencesTable / {ns}belongsToTier
func BuildTriples(ns string, note Note, ex Extraction) []store.Triple   // pure — no store access
func ApplyNote(st *store.Store, ns string, note Note, ex Extraction, onChange func()) error
  // pinned (STATE 39-04): validate-before-mutate → capture refs → DeleteSubject(note) →
  // orphan sweep → Insert/InsertUnique → onChange exactly once; tables: rdf:type TableSchema
  // via InsertUnique, everything else Insert; solely-referenced tables swept
```

From internal/ontology/store/store.go:
```go
func (s *Store) Insert(t Triple)        // blind append under write-lock
func (s *Store) InsertUnique(t Triple) bool  // dedupes on (S,P,O) under ONE lock acquisition
func (s *Store) DeleteSubject(subject string) int
func (s *Store) Query(s, p, o string) []Triple   // "" = wildcard
type Triple struct { S, P, O string; Provenance *Provenance }  // O holds literal text as-is for literal triples
```

From internal/ontology/induction/proposal.go:
```go
type Proposal struct { ID string; Kind ProposalKind /* Class|ObjectProperty|DatatypeProperty */; ...; LocalName string; Status ProposalStatus /* pending|approved|rejected */ }
```

Proposal loop API (X-Admin-Token required; server on :8888):
- POST /api/admin/ontology/proposals/generate  body {"text": "..."} → 200 {"proposal": {...}} (ONE proposal per call, status pending; LocalName is LLM/backend-generated PascalCase)
- GET  /api/admin/ontology/proposals → 200 {"proposals": [...]}
- POST /api/admin/ontology/proposals/:id/approve body {"custom_local_name": "..."} → 200 {"proposal", "tbox_version"} (version bumps per approve)
- GET  /api/admin/ontology/changelog → 200 {"current_version", "entries": [...]}

SPARQL endpoint (Phase 41): POST /api/sparql, header X-Skill-Name (NOT admin token):
```go
type SparqlPattern struct { S, P, O string; Limit, Offset int; Filter string }  // "" = wildcard; limit default 50 max 500
```
NS default: http://skillforge.local/ontology#. rdf:type predicate: http://www.w3.org/1999/02/22-rdf-syntax-ns#type.

SHACL (internal/ontology/shacl/shapes.ttl): global skf:belongsToTierShape (:26-29) requires every belongsToTier object to be a skf:Tier individual. Classes without a NodeShape (InsuranceProduct / Organization today) are skipped by collectIndividuals — zero new violations by construction (D-NZK-07).

Eval gate: TestStubVsGoldenF1 in internal/knowledge/ecl/extract_golden_test.go — golden set eval/ecl-golden/sql-guides.jsonl (pinned row count), harnessF1Threshold = 0.8 BOTH metrics. Baseline: macro tables 0.9846 / related 1.0. Golden set and stub stay UNCHANGED this task (entities are not scored — D-NZK-06).
</interfaces>
</context>

<tasks>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 1: TBox v7 — 4 property proposals through the human approve gate (D-NZK-03, ONT-04)</name>
  <files>none (server-side data/ontology/*.ttl are written by the live server on approve)</files>
  <action>
  All calls against http://localhost:8888 with header X-Admin-Token (token from the server's env — check .env or the WSL server process env).

  1. Baseline: GET /api/admin/ontology/changelog → record current_version (expect 3) so post-approve math is verifiable.
  2. For EACH of the 4 properties (D-NZK-03) call POST /api/admin/ontology/proposals/generate with {"text": ...} — one generate per property (the API returns one proposal per call). Text must state the property kind, domain→range, and bilingual meaning, e.g. for the first: ObjectProperty 中文名「涉及险种」involvesInsuranceProduct, domain NoteChunk, range InsuranceProduct, 表示一条知识笔记提及某个保险产品. The four:
     - involvesInsuranceProduct — ObjectProperty, NoteChunk → InsuranceProduct, 涉及险种
     - involvesOrganization — ObjectProperty, NoteChunk → Organization, 涉及机构
     - parentOrganization — ObjectProperty, Organization → Organization, 上级机构
     - entityCode — DatatypeProperty, 实体业务代码, xsd:string
     Verify each response proposal: Kind matches the requested kind and Status is "pending". If the LLM proposes a wrong Kind, retry generate with sharper text (max 2 retries per property). Record the 4 proposal IDs. The backend LocalName may differ — the approve step's custom_local_name pins the final name.
  3. STOP. Emit CHECKPOINT (do NOT self-approve — ONT-04 is non-negotiable). Include: the 4 proposal IDs, their proposed kinds, and the recommended custom_local_names: involvesInsuranceProduct / involvesOrganization / parentOrganization / entityCode. Ask the human to confirm (or correct) the names; the human may also approve in the /admin/ontology UI instead.
  4. AFTER the orchestrator resumes with approval: POST /api/admin/ontology/proposals/:id/approve with {"custom_local_name": "<confirmed name>"} for each proposal. Expect 200 with tbox_version incrementing (3→4→5→6→7 across the 4 approvals).
  5. Verify: GET /api/admin/ontology/changelog → current_version == 7; GET /api/admin/ontology/proposals → the 4 proposals are approved with the pinned local names.
  </action>
  <what-built>4 pending TBox property proposals (ObjectProperty ×3 + DatatypeProperty ×1) generated via the live proposal API, awaiting human approval to reach TBox v7</what-built>
  <how-to-verify>
  1. GET http://localhost:8888/api/admin/ontology/proposals (with X-Admin-Token) shows the 4 new pending proposals with correct kinds.
  2. Confirm the custom_local_names: involvesInsuranceProduct, involvesOrganization, parentOrganization, entityCode.
  3. Reply "approved" (executor then submits the 4 approve calls), or approve in the /admin/ontology UI yourself and say so.
  </how-to-verify>
  <resume-signal>Type "approved" (optionally with name corrections) — or state you approved via the UI</resume-signal>
  <verify>
    <automated>curl -s -H "X-Admin-Token: $SKILLFORGE_ADMIN_TOKEN" http://localhost:8888/api/admin/ontology/changelog | grep -o '"current_version":[^,}]*'  # == 7, and proposals list shows 4 approved with pinned local names</automated>
  </verify>
  <done>TBox v7 live with skf:involvesInsuranceProduct, skf:involvesOrganization, skf:parentOrganization, skf:entityCode — every one human-approved, zero self-approvals</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Extractor extension — entities field, drop-gates, dedupe (D-NZK-04, D-NZK-01)</name>
  <files>internal/knowledge/ecl/stub.go, internal/knowledge/ecl/extract.go, internal/knowledge/ecl/extract_test.go</files>
  <behavior>
    - sanitizeEntities drops an entity whose kind is not exactly "InsuranceProduct" or "Organization" (illegal kind = drop that entry, NOT a whole-extraction error — D-NZK-04 differs from the tables hard-error gate by design)
    - sanitizeEntities drops entities with empty name after TrimSpace, or name longer than 12 runes (utf8.RuneCountInString)
    - sanitizeEntities trims leading/trailing whitespace from surviving names
    - sanitizeEntities dedupes by exact trimmed name, keep-first (D-NZK-01; 车险 and 汽车险 remain two entities — no aliasing)
    - extractionPromptTmpl still contains the ORIGINAL tables line, related_notes line, and all 4 original Rules bullets byte-identical; entities spec + new rules are APPENDED (D-NZK-04 prompt-stability requirement)
    - ExtractNote with a fake Generator returning {"tables":[],"related_notes":[],"entities":[{"name":"车险","kind":"InsuranceProduct","code":"01"}]} parses entities through to the Extraction
    - ExtractNote with a fake returning an entity of kind "Product" (illegal) yields Extraction with that entity dropped and NO error
    - Over-cap (per-section) path: same entity name returned by two section calls appears once in the union (keep-first)
  </behavior>
  <action>
  1. stub.go: add `ExtractedEntity` struct next to RelatedNote: Name string `json:"name"`, Kind string `json:"kind"`, Code string `json:"code,omitempty"`, Parent string `json:"parent,omitempty"`. Add `Entities []ExtractedEntity` with json tag `"entities,omitempty"` to Extraction. Doc comments cite D-NZK-04.
  2. stub.go: add `sanitizeEntities(raw []ExtractedEntity) []ExtractedEntity` implementing the gates in <behavior>. Parent is NOT validated here (needs store/batch context — Task 3 owns it). Dedupe key = trimmed name.
  3. extract.go: extend extractionPromptTmpl — the JSON shape gains a third line for entities: {"name": "<≤12字实体名>", "kind": "InsuranceProduct" | "Organization", "code": "<可选业务代码>", "parent": "<可选上级机构名,仅Organization>"}. Append new Rules bullets (DO NOT touch existing text): extract 险种 and 机构 names appearing in the note; kind is exactly one of the two classes; name ≤12 chars, written exactly as in the note; code only when the note states a business code; parent only for Organization and only when an immediate parent is stated; emit [] when none. Few-shot wording is Claude's discretion (CONTEXT).
  4. extract.go: llmExtractionShape gains `Entities []ExtractedEntity json:"entities"`. In extractBody, after unmarshal set ex.Entities = sanitizeEntities(shape.Entities) BEFORE ValidateExtraction (tables gate semantics unchanged).
  5. extract.go ExtractNote over-cap union: dedupe entities across section calls by name keep-first, mirroring the seenTables pattern.
  6. Golden set is NOT touched (entities unscored per D-NZK-06); StubExtract stays nil-entities.
  7. Tests per <behavior> in extract_test.go (or a new entities_test.go in package ecl) — write tests FIRST (RED), then implement (GREEN).
  </action>
  <verify>
    <automated>wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ecl/... -count=1'</automated>
  </verify>
  <done>All entity gates enforced with drop semantics; prompt tables/related parts byte-identical; every pre-existing ecl test still green</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: ABox emission — entity triples, IRI sanitization, idempotent apply (D-NZK-05, D-NZK-01/02)</name>
  <files>internal/knowledge/ecl/entity_iri.go, internal/knowledge/ecl/entity_iri_test.go, internal/knowledge/ecl/abox.go, internal/knowledge/ecl/abox_test.go</files>
  <behavior>
    - EntityIRI is deterministic: same name → same IRI across notes (identity = exact name, D-NZK-01); pure-ASCII name → PascalCase local name; CJK name → "e" + first 12 hex chars of sha256(name); result prefixed ns + "ent_"
    - Same-batch collision: two DISTINCT names mapping to one IRI get ordinal suffixes (-2, -3...) within one BuildTriples call
    - BuildTriples per entity emits: entity rdf:type {ns}{Kind}; rdfs:label "<name>"; skf:entityCode "<code>" only when code non-empty; skf:belongsToTier {ns}{note.Tier} only when note.Tier != ""; note→entity edge {ns}involvesInsuranceProduct or {ns}involvesOrganization; parentOrganization edge ONLY when parent exactly matches another Organization entity in the same batch
    - Parent referencing a non-existent (not-in-batch) name: parent triple dropped, the entity itself kept (D-NZK-04)
    - ApplyNote re-run for the same note with the same extraction: zero new entity triples (entity-owned triples InsertUnique, note→entity edges rebuilt via Insert after DeleteSubject)
    - ApplyNote orphan sweep extended: a captured entity IRI with zero remaining involvesInsuranceProduct/involvesOrganization references AND zero parentOrganization references pointing at it is DeleteSubject'd; an entity still referenced by another note survives; an entity referenced ONLY as someone's parentOrganization target survives
    - ApplyNote emits a parentOrganization triple via InsertUnique when the parent is not in-batch but already exists in the store as an Organization individual (rdf:type check)
    - onChange fires exactly once per successful apply (existing invariant preserved)
  </behavior>
  <action>
  1. New file entity_iri.go: `EntityIRI(ns, name string) string` = ns + "ent_" + entityLocalName(name). entityLocalName: if regexp ^[0-9A-Za-z_]+$ matches → deterministic PascalCase transform; else "e" + hex.EncodeToString(sha256(name))[:12]. Godoc explains D-NZK-01 (exact-name identity ⇒ deterministic IRI ⇒ cross-note InsertUnique dedupe works) and why emitter.SanitizeLocal alone is insufficient (it strips CJK to empty). No new external dependency (crypto/sha256 is stdlib).
  2. abox.go: add predicate helpers involvingInsuranceProductPred/involvesOrganizationPred/parentOrganizationPred/entityCodePred(ns) (same style as referencesTablePred) + `rdfsLabelIRI = "http://www.w3.org/2000/01/rdf-schema#label"` const.
  3. BuildTriples: after the existing table loop, re-run sanitizeEntities on ex.Entities (defense in depth — same gate as Task 2), assign batch IRIs via a local map (collision ordinal per <behavior>), emit the per-entity triples + note→entity edges with the note's existing prov record. Parent same-batch resolution = exact trimmed-name match against batch entities of kind Organization.
  4. ApplyNote: extend step 2 capture to also collect object IRIs of both involves-predicates on the note; extend step 4 orphan sweep to entities (sweep only when NO involves-edge AND NO parentOrganization target-edge remains — the parent-target guard). Step 5 insertion split: entity-owned triples (rdf:type with O ∈ {ns+InsuranceProduct, ns+Organization}, rdfs:label, entityCode, belongsToTier, parentOrganization where subject is an ent_ IRI) → InsertUnique; note→entity edges → Insert. Second-chance parent resolution per <behavior> using st.Query(parentIRI, rdfTypeIRI, ns+"Organization"). Ordering and onChange-once invariant untouched (STATE 39-04 pin).
  5. belongsToTier object is ALWAYS ns + note.Tier (server-derived from kbDir, never LLM-supplied) — satisfies the SHACL global belongsToTierShape; label/code are literal O values stored as plain strings (same as TBox individual tierName literals).
  6. Tests per <behavior> in entity_iri_test.go + abox_test.go — RED first, then GREEN.
  </action>
  <verify>
    <automated>wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ecl/... ./internal/ontology/... -count=1'</automated>
  </verify>
  <done>Entity triple shape matches D-NZK-05 exactly; ApplyNote is idempotent for entities; sweep never orphans parent targets; all ecl + ontology tests green</done>
</task>

<task type="auto">
  <name>Task 4: E2E live gate — pipeline re-run, KG/SPARQL visibility, golden F1 + SHACL no-regression (D-NZK-06, D-NZK-07)</name>
  <files>none (live verification against the WSL server on :8888)</files>
  <action>
  Prerequisite: Tasks 1-3 committed and TBox v7 live.

  1. Hermetic gate: run the WSL build+test command from <context> — must be fully green including TestStubVsGoldenF1 (the golden F1 gate ≥ 0.8 both metrics; golden set untouched so the 0.9846 / 1.0 baseline must hold).
  2. Rebuild + restart the server per env-wsl-backend-runbook (WSL, CGO_ENABLED=1). On boot verify: no errors, TBox version still 7 (UpgradeIfNeeded strict no-op), snapshot restore clean.
  3. SHACL baseline: GET /api/admin/governance/shacl → record the violation count BEFORE the pipeline run.
  4. POST /api/admin/pipeline/run → 200 {"status":"started"}; poll GET /api/admin/pipeline/status until the extract stage is done. Check the failures list: mass dead-letters = stop and diagnose (a prompt regression would surface here); a couple of pre-existing failures → note them.
  5. Entity visibility: POST /api/sparql with X-Skill-Name of a skill that reads sql-guides — body {"p":"http://www.w3.org/1999/02/22-rdf-syntax-ns#type","o":"http://skillforge.local/ontology#InsuranceProduct","limit":50} → expect individuals (车险/农险/健康险 among the labels); repeat with o = ...#Organization. Spot-check one entity's rdfs:label / entityCode / belongsToTier / parentOrganization triples via {"s":"<entity IRI>"}.
  6. Tier isolation (D-NZK-02): same InsuranceProduct query with X-Skill-Name of a skill WITHOUT sql-guides read access → zero entity triples (fail-closed via belongsToTier, D-TIER-03 untouched).
  7. SHACL after: violations count ≤ baseline (D-NZK-07: new classes have no NodeShape so they cannot violate; the only live risk is belongsToTier objects — which Task 3 constrains to Tier individual IRIs).
  8. Live quality spot-check (D-NZK-06): pick 2 notes, confirm their referencesTable triples are still present in the KG after the prompt change (tables extraction did not regress).
  9. Commit sequence with prefix feat(quick-260928-nzk) — one commit per task boundary (extractor / abox / e2e artifacts if any).
  </action>
  <verify>
    <automated>wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ecl/... -run "TestStubVsGoldenF1" -count=1 -v' && curl -s -X POST http://localhost:8888/api/sparql -H "X-Skill-Name: sql-guides" -H "Content-Type: application/json" -d '{"p":"http://www.w3.org/1999/02/22-rdf-syntax-ns#type","o":"http://skillforge.local/ontology#InsuranceProduct","limit":10}'</automated>
  </verify>
  <done>Entities live in the KG and tier-gated via SPARQL; golden F1 gate green at baseline; SHACL violation count not increased; pipeline completed without new mass failures; work committed</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| LLM → Extraction.entities | Model output crosses into the extraction contract (prompt injection / hallucinated entities) |
| Entity name → KG IRI | Untrusted text becomes a KG identifier local name |
| Entity belongsToTier | Tier attachment decides cross-skill visibility (D-TIER-03 trust boundary) |
| Human approve gate | Nothing enters the TBox without explicit human approval (ONT-04) |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-NZK-01 | Tampering | sanitizeEntities (stub.go) | mitigate | kind enum hard-filter, name trim + ≤12-rune cap, exact-name dedupe keep-first — illegal entries DROPPED, never mint ABox individuals |
| T-NZK-02 | Tampering | EntityIRI / entityLocalName | mitigate | deterministic sha256[:12] (CJK) or PascalCase (ASCII) local names — no raw LLM text reaches an IRI; same technique as TableIRI's sanitizer |
| T-NZK-03 | Elevation (tier bypass) | entity belongsToTier emission | mitigate | object is ALWAYS {ns}{note.Tier} (server-derived from kbDir basename), never LLM-supplied; satisfies SHACL global belongsToTierShape; SPARQL tier filter stays fail-closed |
| T-NZK-04 | Repudiation | TBox proposal approve | mitigate | ONT-04 blocking human checkpoint in Task 1 — executor generates proposals but CANNOT approve; approve calls run only after orchestrator-resumed human confirmation |
| T-NZK-05 | Denial of Service | hallucinated entity flood | accept | per-note entity volume bounded by prompt instruction (≤12-char names, dedupe); sweep removes unreferenced entities on re-run; KG is memory-map (spike 005d, 9× faster than Kuzu) |
| T-NZK-SC | Tampering | no new package installs | mitigate | stdlib-only (crypto/sha256, regexp, strings) — zero npm/pip/cargo surface this task |
</threat_model>

<verification>
1. `wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go build ./... && CGO_ENABLED=1 /usr/local/go/bin/go test ./internal/knowledge/ecl/... ./internal/ontology/... -count=1'` — fully green
2. GET /api/admin/ontology/changelog → current_version == 7 with the 4 pinned property names in the entries
3. POST /api/sparql (allowed skill) → InsuranceProduct + Organization individuals with label/code/tier/parent triples
4. POST /api/sparql (restricted skill) → zero entity triples (fail-closed)
5. TestStubVsGoldenF1 PASS (≥ 0.8 both metrics; baseline 0.9846 / 1.0 preserved)
6. SHACL report: violation count ≤ pre-run baseline
7. Pipeline re-run idempotent: second run adds zero duplicate entity individuals
</verification>

<success_criteria>
- TBox v7 with involvesInsuranceProduct / involvesOrganization / parentOrganization / entityCode, every property human-approved (no self-approval)
- Extraction contract extended with entities; drop-gates + dedupe live in both the single-call and over-cap union paths; prompt tables/related spec byte-identical
- ABox entity triple shape exactly per D-NZK-05; entity individuals InsertUnique-idempotent across notes and re-runs; orphan sweep with parent-target guard
- Live: 车险/农险/... + 机构 individuals visible to sql-guides skills via /api/sparql, invisible to others
- Golden F1 gate and SHACL no-new-violations gate both green after the pipeline re-run
</success_criteria>

<output>
Create `.planning/quick/260928-nzk-ecl-kg-abox/260928-nzk-SUMMARY.md` when done
</output>
