# Phase 45 — Deferred Items

## Discovered during 45-01 execution (2026-10-03)

| # | Item | Type | Evidence | Disposition |
|---|------|------|----------|-------------|
| D-45-DEF-01 | `internal/ontology/emitter` `TestEmitSubCategoryInstanceCount` fails: `SubCategory instances = 6, want 7` | Pre-existing test failure (out of 45-01 scope) | Reproduced identically at pre-plan commit `5bf9c35` in a throwaway worktree (`/d/CODE/ClaudeCode/p45-precheck`), 45-01's diff touches only `internal/ontology/store/*` and `internal/server/admin_platform_test.go`; emitter last modified in Phase 39 (`b789bfb`) | NOT fixed (scope boundary: only auto-fix issues caused by the current task's changes). Surface to phase verifier + 45-02: `verify-phase-45.sh` partition P2 (`go test ./internal/ontology/...`) will be red on this test until triaged — likely a stale count vs the v7 TBox (6 vs 7 SubCategory instances; STATE.md records TBox v3→v7 during 260928-nzk with "4 属性人工批准"). Candidate for a 45-02 assertion-choreography fix or a quick task. |
| D-45-DEF-02 | `internal/server/platform/verify_main.go` runtime-stale since Phase 44 (pre-44 fixture, owl:Class IRIs, raw counts, label-as-schema assertions) | Pre-existing stale harness | 45-RESEARCH Corrections + Pitfall 9; untracked in working tree | Deferred by plan (Open Question 2 RESOLVED): excluded from verify-phase-45.sh; BL-05 backlog candidate; do not fix in Phase 45. |

## Untracked working-tree noise (not produced by 45-01; left alone)

- `internal/server/skillforge-server-test-*` directories (5) — leftover hertz test temp dirs from earlier sessions.
- `internal/server/platform/verify_main.go` — see D-45-DEF-02.

## D-45-DEF-03 (2026-10-03, code-review WR-01/WR-02 + IN-02 backlog candidates)

Code review (45-REVIEW.md, 0 critical / 3 warning / 3 info) surfaced pre-existing behavioral issues amplified-but-not-caused by the β arm:

- **WR-01 — Path B schema passthrough precedes S/P/O pattern filter** (store.go:556-561 before 568-577): pattern-scoped queries return non-matching schema rows and inflated Count (pred=hasReadAccess returns 10, only 2 match). Pre-existing; β amplified 8→10. Consumer risk: /api/sparql + MCP skf_query false-positive existence checks. Candidate: apply pattern filter to schema rows too (count semantics change — needs its own decision record; count-sensitive fixtures pinned at 17/10 would move).
- **WR-02 — `kg/query?skill=<unresolvable>` fails open** (admin_ontology.go:214-216): nil tiers → QueryScoped Path A → full unfiltered KG, while sibling /api/sparql handler 401s on nil. Admin-gated (no escalation) but breaks fail-closed posture; SC3 parity guarantee holds only for resolvable skills. Candidate: align with 401-on-nil-tier semantics.
- **IN-02 — IRI family mismatch (undocumented half)**: isSchemaTriple OWL arm checks `skf:domain`/`skf:range` but emitter emits `rdfs:domain`/`rdfs:range` (arm never matches in production); dead checks `skf:Property`/`skf:NamedIndividual`. Fail-closed direction (over-filtering). Phase 46 should fix the whole IRI family (owl:Class half already documented as D-45-DEF adjacent), not just owl:Class.
- IN-01 (store_export_test.go seam returns mutable aliases, schemaSubjectSet called without documented RLock) — test-only, safe today; note for future seam consumers.
- IN-03 (β discloses cross-skill tier topology, e.g. skill_compliance→authority visible in sql-guides views) — intended per D-SPARQL-05; recorded as visible product decision.
