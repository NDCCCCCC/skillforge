---
phase: 45-sparql-schema-visibility-d-sparql-05
plan: 01
subsystem: ontology-store
tags: [sparql, tier-filtering, access-control, rdf-store, tdd]
requires:
  - Phase 44 D-SPARQL-01..08 (SPARQL test fix + red-line pinned tests)
  - store.go QueryScoped Path B schema-passthrough loop (pre-existing)
provides:
  - isSchemaTriple 4th arm — schemaSubjects[t.S] && metaPredicates[t.P] (D-45-01 β)
  - metaPredicates 13-predicate allowlist (label/comment deliberately excluded)
  - Store.schemaSubjectSet() query-time derivation (D-45-02, zero write-path hooks)
  - store-layer dual-assertion regression (hasReadAccess visible AND label hidden, same subject)
affects:
  - 45-02 (emitter conformance test pins metaPredicates; parity test; verify-phase-45.sh)
  - /api/sparql, /api/admin kg/query skill=, MCP skf_query — all via the single QueryScoped Path B call site
tech-stack:
  added: [] # zero new dependencies (go.mod unchanged)
  patterns:
    - const-map allowlist beside schemaClasses (same compile-time literal pattern)
    - query-time derived set + single-judgment call site (SPARQL-03 single-point)
    - RED→GREEN TDD pair across server fixture assertions and store implementation
key-files:
  created:
    - internal/ontology/store/store_schema_subject_test.go
  modified:
    - internal/ontology/store/store.go
    - internal/ontology/store/store_label_passthrough_test.go
    - internal/server/admin_platform_test.go
decisions:
  - D-45-01 β arm implemented as locked — subject ∩ predicate allowlist; literal Option α rejected (would break D-SPARQL-03 pinned tests)
  - D-45-02 query-time derivation implemented (deviation from ROADMAP SC1 "Insert 时填充" wording recorded in plan objective) — 4-mutator drift class eliminated constructively
  - D-45-03 untouched — Path A early-return byte-identical; admin baseline semantics frozen
  - D-45-05 wording policy honored — all new rationale cites D-SPARQL-05 / D-45-04; zero "miscount" strings added
metrics:
  duration: 9 min
  completed: 2026-10-03T02:30:31Z
  tasks: 3
  files: 4
---

# Phase 45 Plan 01: SPARQL Schema Visibility β Arm Summary

**One-liner:** Landed D-SPARQL-05 Option β in the store — `isSchemaTriple` gained a subject∩predicate allowlist arm (13 metaPredicates, label/comment fail-closed) with query-time `schemaSubjects` derivation, making skill individuals' `hasReadAccess` triples tier-visible (admin counts 17/10) while all D-SPARQL-03 red lines pass with zero assertion edits.

## What Was Built

**Task 1 (RED, commit `7931106`)** — `internal/server/admin_platform_test.go` assertion choreography:
- `TestSparqlQueryScoped_TierFilter`: 15 → **17** (8 schema + 2 hasReadAccess β-visible + 4 note_001 + 3 tbl_payment_t), positive membership assertion for `(skill_sql, hasReadAccess, sql-guides)`
- `TestSparqlQueryScoped_FailClosedNoBelongsToTier` Path B: 8 → **10** (schema passthrough precedes the S/P/O filter; β adds both hasReadAccess rows), same positive assertion
- fixtureKG godoc rewritten (item 3, Path B totals, Path B S=note_003 case, subject-exclusion list) per D-45-04/D-45-05 — no "EXCLUDED from Path B" residue, no "miscount"/"D-SPARQL-08" strings
- Negative loops (note_002/note_003) and all Path A assertions byte-identical

**Task 2 (GREEN, commit `0c3f398`)** — `internal/ontology/store/store.go` β implementation:
- `metaPredicates` — 13-predicate allowlist (skillName, skillDescription, skillTemperature, poweredBy, hasReadAccess, hasWriteAccess, requiresRefusal, tierName, categoryName, subCategoryName, agentTypeName, refusalPhraseText, belongsToTier) declared beside `schemaClasses`; label/comment deliberately excluded (D-SPARQL-03)
- `(*Store).schemaSubjectSet()` — query-time derivation from `byP[rdfType] ∩ schemaClasses`; no persistent state, zero write-path hooks (InsertUnique / DeleteSubject / Quarantine / rollback / LoadSnapshot correct by construction)
- `isSchemaTriple(t Triple, schemaSubjects map[string]bool)` — arms 1–3 byte-identical; new arm 4 `schemaSubjects[t.S] && metaPredicates[t.P]`
- `QueryScoped` Path B: derivation once after visibleSubjects, before the `s.all` loop; single tier-path call site updated (SPARQL-03 single-point preserved); Path A early-return untouched
- Ripple: 8 pinned call sites in `store_label_passthrough_test.go` pass `nil`; zero assertion edits (D-45-04 red-line policy comment added)

**Task 3 (commit `1479534`)** — `internal/ontology/store/store_schema_subject_test.go`:
- `TestStoreSchemaSubject_HasReadAccessVisibleLabelHidden` — one test, two assertions, same subject: `(skill_x, hasReadAccess, cat_sql)` tier-visible (D-SPARQL-05 positive) AND `(skill_x, label, "secret")` fail-closed in the SAME Path B result (D-SPARQL-03 red line); pred-scoped and Path A sanity assertions included

## Verification Results

- RED gate: exactly `TierFilter` + `FailClosedNoBelongsToTier` failed (17≠15, 10≠8); other 9 SPARQL tests passed
- GREEN gate: `go vet ./internal/ontology/...` clean; store suite fully green (incl. 3 D-SPARQL-03 red-line tests + Phase 40 suite); `TestSparqlQueryScoped` 11/11 PASS (TierFilter=17, FailClosed Path B=10); `TestHybridSearchScoped` 9/9 PASS (WSL `CGO_ENABLED=1` throughout)
- Ripple integrity: `grep -c "isSchemaTriple("` in passthrough test = 8, all with `nil`; diff shows only `, nil` additions + policy comment — assertion verdicts/messages untouched
- Production call sites: exactly one `isSchemaTriple(t, schemaSubjects)` (store.go Path B); no second schema-classification implementation introduced

## Deviations from Plan

**1. [Rule 1 - Bug] Task 3 pred-scoped assertion corrected 1 → 2 rows**
- **Found during:** Task 3 first run (test failed against my own new test's expectation)
- **Issue:** Plan `<behavior>` expected `QueryScoped("", hasReadAccess, "", ["sql-guides"])` to return exactly 1 row. Mechanically impossible: schema passthrough bypasses the S/P/O pattern filter entirely, so the `rdf:type` row (required by construction — the β arm needs the subject to carry `rdf:type → schemaClasses`) always co-passes with any β-visible row. The same mechanism is pinned by the passing admin FailClosed Path B=10 assertion.
- **Fix:** Assertion changed to Len 2 (rdf:type + hasReadAccess) + positive membership + label-absence check, with godoc explaining the structural co-existence. Production code untouched.
- **Files modified:** internal/ontology/store/store_schema_subject_test.go
- **Commit:** 1479534

**2. [Out-of-scope discovery] Pre-existing emitter test failure — NOT fixed (scope boundary)**
- `TestEmitSubCategoryInstanceCount` fails (`SubCategory instances = 6, want 7`). Reproduced identically at pre-plan commit `5bf9c35` in a throwaway worktree; 45-01's diff cannot reach the emitter package (last touched Phase 39 `b789bfb`). Logged as **D-45-DEF-01** in `deferred-items.md`. Consequence: `go test ./internal/ontology/...` (whole-tree) is red on this one test; the plan-modified packages (store, server SPARQL/Hybrid) are fully green. 45-02's `verify-phase-45.sh` P2 partition will surface it — triage recommendation recorded in deferred-items.md.

No auth gates occurred.

## Known Stubs

None. All data paths are fully wired; no placeholder code introduced.

## Threat Flags

None — no security-relevant surface beyond the plan's `<threat_model>` was introduced. The β arm's leak bound is exactly as modeled (T-45-01 mitigated by label/comment exclusion + red lines; T-45-02 gate lands in 45-02; T-45-04 mitigated by query-time derivation).

## Requirement Traceability Note

Plan frontmatter lists `requirements: [SPARQL-01, SPARQL-02]`, but 45-02 (wave 2) claims the same IDs plus SPARQL-03 (conformance test, parity test, verify script are the verification layer of those requirements). REQUIREMENTS.md checkboxes deliberately left unchecked after 45-01 — 45-02's executor should `requirements mark-complete SPARQL-01 SPARQL-02 SPARQL-03` at phase close.

## Self-Check: PASSED

- [x] internal/ontology/store/store.go — FOUND (β arm + metaPredicates + schemaSubjectSet)
- [x] internal/ontology/store/store_label_passthrough_test.go — FOUND (8 nil-arg call sites)
- [x] internal/ontology/store/store_schema_subject_test.go — FOUND (new, package store)
- [x] internal/server/admin_platform_test.go — FOUND (17/10 + positive assertions + godoc rewrite)
- [x] Commit 7931106 — FOUND
- [x] Commit 0c3f398 — FOUND
- [x] Commit 1479534 — FOUND
