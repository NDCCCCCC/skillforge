---
phase: 45-sparql-schema-visibility-d-sparql-05
plan: 02
subsystem: ontology-verification
tags: [sparql, conformance-test, parity-test, verification-gate, anti-drift]
requires:
  - 45-01 β arm (isSchemaTriple 4th arm + metaPredicates + schemaSubjectSet)
  - Phase 44 red-line suites (D-SPARQL-03 pinned tests)
provides:
  - TestEmitterPredicatesWithinAllowlist — D-45-01 anti-drift gate (emitter predicates on schema individuals ⊆ {rdf:type} ∪ metaPredicates)
  - TestSchemaJudgmentParity — SPARQL-03 / D-45-03 handler-level parity across /api/sparql and /api/admin on one fixtureKG
  - verify-phase-45.sh — SC4 fail-fast phase gate (4 partitions, WSL CGO chain, anti-vacuous PASS counts)
  - store_export_test.go — test-only seam letting an external store test import the emitter (cycle-safe)
affects:
  - Phase 46 (Entity SHACL Shapes) — inherits the owl:Class-vs-skf:Class declaration quirk note + the gate
  - verify-phase-45.sh is the phase verifier's entry point (P2/P4 carry the D-45-DEF-01 named waiver)
tech-stack:
  added: [] # zero new dependencies (go.mod unchanged)
  patterns:
    - external test package + export_test.go seam (import-cycle escape hatch)
    - named-waist waiver in a suite gate (unique --- FAIL set must be empty or exactly the documented pre-existing failure)
    - exit code paired with minimum --- PASS count (verify-phase-39 anti-vacuous pattern)
key-files:
  created:
    - internal/ontology/store/store_emitter_conformance_test.go
    - internal/ontology/store/store_export_test.go
    - verify-phase-45.sh
  modified:
    - internal/server/admin_ontology_test.go
decisions:
  - D-45-01 anti-drift gate landed as conformance test — emitter render() predicates on schema-individual subjects must stay within {rdf:type} ∪ metaPredicates or the gate red-lights
  - Conformance test lives in package store_test (external) + store_export_test.go seam — emitter transitively imports store (emitter/upgrade.go → induction → store), so the plan's in-package premise was impossible; external test packages may cycle through the package under test
  - Parity test wires Ontology.Induction via newOntologyDeps then swaps KG to fixtureKG — requireOntology 503s kg/query on nil Induction; kg/query reads only Ontology.KG
  - verify-phase-45.sh P2/P4 carry a named-waist D-45-DEF-01 waiver — partition passes only when the unique --- FAIL set is empty or exactly TestEmitSubCategoryInstanceCount; any other failure fails the gate
  - P3 threshold pinned at ≥ 19 --- PASS (10 TestSparqlQueryScoped + 9 TestHybridSearchScoped per plan; live tree yields 20 with the def01 file's 11th SPARQL test — threshold keeps rename-detection headroom)
metrics:
  duration: 24 min
  completed: 2026-10-03T03:01:43Z
  tasks: 3
  files: 4
---

# Phase 45 Plan 02: SPARQL Schema Visibility Verification Layer Summary

**One-liner:** Landed the phase verification layer on 45-01's β semantics — D-45-01 emitter conformance gate (13-family fixture, label/comment negative pin), SPARQL-03 handler-level parity test proving /api/sparql and /api/admin make identical schema/ABox judgments on one fixtureKG, and verify-phase-45.sh (SC4, 4 fail-fast partitions under WSL CGO_ENABLED=1, exit 0 with the D-45-DEF-01 named waiver).

## What Was Built

**Task 1 (commit `417ef99`)** — `internal/ontology/store/store_emitter_conformance_test.go` (+ `store_export_test.go` seam):
- `TestEmitterPredicatesWithinAllowlist`: renders a SKILL.md fixture exercising ALL 13 metaPredicates families (multi-line description, agent, temperature, reads/writes globs over sql-guides/authority/draft KB dirs incl. a 取数口径 subdirectory, refusal phrase), parses the TTL through the PRODUCTION path (`store.New().LoadFromTurtle`), scopes by the SAME `schemaSubjectSet()` derivation Path B uses, and fails listing any triple where a schema-individual subject carries a predicate outside {rdf:type} ∪ metaPredicates
- Anti-vacuous coverage check: every metaPredicates key must appear in the parsed output — an impoverished fixture cannot pass silently
- Negative pin: skf: AND rdfs: label/comment must never enter metaPredicates (D-SPARQL-03 authoritative over D-45-01)
- owl:Class-vs-skf:Class declaration quirk (production class-def triples tier-invisible) recorded comment-only for Phase 46 — no assertion, per RESEARCH Open Question 1

**Task 2 (commit `118eebd`)** — `internal/server/admin_ontology_test.go`:
- `TestSchemaJudgmentParity` (SPARQL-03 / D-45-03): one `fixtureKG()` store wired into both handlers; Request A `POST /api/sparql` (X-Skill-Name: sql) and Request B `GET /api/admin/ontology/kg/query?p=<hasReadAccess>&skill=sql` (X-Admin-Token) both return the (skill_sql, hasReadAccess, sql-guides) β-visible row, neither contains any note_003 row (D-TIER-07 fail-closed on both surfaces), plus row-count parity
- File-local `invokeParity` registers `ontologyRoutes` + `POST /api/sparql` on ONE engine; `invokeOntology`/`ontologyRoutes` byte-unchanged (0 deletions in diff — T-45-06 anti-second-implementation guard held)
- Case-1 direct-Tier fixture wiring keeps the BL-01 resolver divergence out of the parity signal (45-RESEARCH A4)

**Task 3 (commit `e610bf1`)** — `verify-phase-45.sh` (repo root, mode 0755):
- P1 build+vet → P2 ontology suite → P3 targeted SPARQL+Hybrid (exit code AND ≥ 19 `--- PASS`, anti-vacuous) → P4 full `./internal/...` suite; every gate command inside WSL Ubuntu-22.04 with CGO_ENABLED=1 (Pitfall 6)
- `grep -c verify_main` = 0 — the stale Phase-44 harness is never built, run, or referenced (D-45-DEF-02 exclusion; recorded below as backlog)
- D-45-DEF-01 named waiver: P2/P4 pass only when the unique `--- FAIL` set is empty or exactly `TestEmitSubCategoryInstanceCount` (pre-existing, proven at `5bf9c35`); a `fail_detail` diagnostic prints the exact offending set on any mismatch

## Verification Results

- `go test ./internal/ontology/store/ -run TestEmitterPredicatesWithinAllowlist -v -count=1` → PASS; full store suite green
- `go test ./internal/server/ -run TestSchemaJudgmentParity -v -count=1` → PASS; all 6 pre-existing `TestAdminOntology*` tests green (harness untouched)
- Combined `TestEmitterPredicatesWithinAllowlist|TestStoreSchemaSubject` → PASS
- `bash verify-phase-45.sh` → exit 0: P1 green, P2 green (1 waived), P3 20/19 `--- PASS`, P4 green (1 waived)
- `git diff` on admin_ontology_test.go: 134 insertions, 0 deletions — invokeOntology/ontologyRoutes untouched
- verify_main.go: no commits touch it; still untracked working-tree noise (D-45-DEF-02)

## Deviations from Plan

**1. [Rule 3 - Blocking] Task 1 conformance test moved to external package `store_test` + `store_export_test.go` seam**
- **Found during:** Task 1 first verify run
- **Issue:** Plan asserted "emitter imports nothing from store; verified at HEAD 216182f" — true directly but false transitively: `emitter/upgrade.go` imports `induction`, which imports `store`. An in-package (`package store`) test file importing emitter hits `import cycle not allowed in test`.
- **Fix:** Test file declares `package store_test` (external test packages may legally cycle through the package under test); the unexported members it needs (metaPredicates, schemaSubjectSet, all, ns, rdfType) are exposed via `store_export_test.go` — an in-package `_test.go` seam compiled only under `go test`, so production API surface is unchanged. File name, test name, and every plan assertion preserved verbatim.
- **Files modified:** internal/ontology/store/store_emitter_conformance_test.go (package clause + seam calls), internal/ontology/store/store_export_test.go (new)
- **Commit:** 417ef99

**2. [Rule 3 - Blocking] Task 2 parity deps wire Ontology.Induction via newOntologyDeps, then swap KG to fixtureKG**
- **Found during:** Task 2 design (pre-implementation code read)
- **Issue:** Plan's literal setup `deps := &Deps{Ontology: &OntologyDeps{KG: fixtureKG()}}` would make Request B return 503: `requireOntology` (admin_ontology.go:81) fails-closed on nil `Induction`. The kg/query handler reads only `Ontology.KG`.
- **Fix:** `ont := newOntologyDeps(t); ont.KG = fixtureKG()` — Induction is a real temp-dir manager (satisfies the gate), KG is the parity fixture (satisfies the plan's "do not use testMiniTTL KG" mandate; the Manager's stale KG reference is never read by kg/query).
- **Files modified:** internal/server/admin_ontology_test.go
- **Commit:** 118eebd

**3. [Rule 3 - Blocking] Task 3 P2/P4 need a named D-45-DEF-01 waiver to ever exit 0**
- **Found during:** Task 3 baseline dry-run (pre-implementation)
- **Issue:** Plan expects P2 (`go test ./internal/ontology/...`) and P4 (`go test ./internal/...`) green, but D-45-DEF-01 (`TestEmitSubCategoryInstanceCount`, 6 vs 7) is a proven pre-existing failure explicitly out of scope ("do NOT fix the emitter bug"). As literally specified, the gate can never exit 0.
- **Fix:** `waiver_ok` — a partition passes only when its unique `--- FAIL` set is empty or EXACTLY `TestEmitSubCategoryInstanceCount`; any other failure prints its log tail + the full offending set and exits 1. The gate stays honest (new failures red-light; the waived test disappearing also red-lights nothing today — it would simply make logs cleaner) while acknowledging documented debt. Matches the phase constraint's "documented triage" instruction and deferred-items.md.
- **Files modified:** verify-phase-45.sh
- **Commit:** e610bf1

**4. [Transient observation] First gate run: P4 waiver mismatch did not reproduce**
- The initial `bash verify-phase-45.sh` run failed P4's waiver check; its `tail -30` showed only the single expected D-45-DEF-01 failure. Three subsequent full-suite runs (two standalone, one gate) show exactly one clean `--- FAIL` line. Root cause of the one-off could not be pinned (details were above the old tail window); the script now prints the unique `--- FAIL` set via `fail_detail` on any future mismatch, so a recurrence will name itself. No code change beyond the diagnostics.

No auth gates occurred.

## Known Stubs

None. All three artifacts are fully wired; no placeholder code introduced.

## Backlog Notes (recorded, not fixed — scope discipline)

- **D-45-DEF-02 (verify_main.go)**: `internal/server/platform/verify_main.go` remains runtime-stale since Phase 44 (pre-44 fixture, owl:Class IRIs, label-as-schema assertions), untracked, invisible to `go test`, excluded from verify-phase-45.sh per plan (Pitfall 9). BL-05 backlog candidate for a follow-up quick task (fix or delete).
- **D-45-DEF-01 (emitter SubCategory count)**: still open in `deferred-items.md` — now surfaced by every verify-phase-45.sh P2/P4 execution as a WAIVED line until triaged (likely stale count vs TBox v7) or fixed by a quick task.

## Threat Flags

None — no security-relevant surface beyond the plan's `<threat_model>` was introduced. T-45-02 (allowlist drift) is now mitigated by the landed conformance test; T-45-06 (second implementation) held — the parity test only observes real handler responses, harness functions untouched; T-45-07 (vacuous gate) mitigated by the P3 pass-count pairing.

## Requirement Traceability

SPARQL-01 (anti-drift conformance), SPARQL-02 (fail-closed red lines re-proven in every P2/P3 gate execution), SPARQL-03 (single-point judgment demonstrated by parity test) — all three marked complete in REQUIREMENTS.md via `requirements mark-complete` at plan close (per 45-01 SUMMARY's handoff note).

## Self-Check: PASSED

- [x] internal/ontology/store/store_emitter_conformance_test.go — FOUND (package store_test, TestEmitterPredicatesWithinAllowlist)
- [x] internal/ontology/store/store_export_test.go — FOUND (test-only seam)
- [x] internal/server/admin_ontology_test.go — FOUND (+134/-0, TestSchemaJudgmentParity + invokeParity)
- [x] verify-phase-45.sh — FOUND (mode 100755 in git, 4 partitions, 0 verify_main refs)
- [x] Commit 417ef99 — FOUND
- [x] Commit 118eebd — FOUND
- [x] Commit e610bf1 — FOUND
- [x] bash verify-phase-45.sh — exit 0, 4 partitions PASS
