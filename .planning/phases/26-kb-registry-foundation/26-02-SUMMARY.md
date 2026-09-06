---
phase: 26-kb-registry-foundation
plan: 02
subsystem: knowledge-registry
tags: [kb-registry, sqlite, additive-migration, seed-merge, tdd, kb-m-01, kb-m-02]
requires:
  - "internal/kb Registry interface + Store seam (26-01)"
  - "knowledge_bases.yaml controlled seed (26-01)"
provides:
  - "sqlite knowledge_bases mutable config table (additive D14-E1 migration, role CHECK backstop, migrate-tail role index)"
  - "SQLiteStore implementation of kb.Store (LoadBases / UpsertBase / SetRetrievalEnabled)"
  - "kb.Open production assembly: seed fail-closed -> structure-only upsert -> merged store-backed snapshot"
affects:
  - internal/server (26-05 admin endpoints + audit rows ride kb.Store mutations)
  - internal/knowledge + internal/retriever (26-03 consume the Open-ed registry)
  - cmd/server (26-09 wires kb.Open; this plan deliberately did not touch it)
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "additive migration: new table in schemaSQL + index const executed at the migrate() tail after the ALTER loops (D14-E1)"
    - "mutable config table + insert-only audit rows written by the caller (datasets precedent for the config-is-not-evidence split)"
    - "seed merge: structure belongs to config, the switch belongs to ops (Pitfall 6 verdict, test-locked both directions)"
    - "field-diff write guard so an unchanged seed never writes at boot"
key-files:
  created:
    - internal/store/sqlite/kb_registry.go
    - internal/store/sqlite/kb_registry_test.go
    - internal/kb/open.go
    - internal/kb/open_test.go
  modified:
    - internal/store/sqlite/schema.go
decisions:
  - "D26-02-A: kb.Open returns the Registry interface (26-01 deviation #1 handoff), and builds it with the in-package newSnapshot(merged, st) so the production registry is store-backed — a read-only New() registry would make 26-05's Create/Deactivate fail with ErrReadOnly"
  - "D26-02-B: knowledge_bases is a mutable config table (datasets precedent); change evidence is the CALLER's insert-only audit row in the existing audit table (26-05) — no new audit table, no delete path anywhere"
  - "D26-02-C: Open re-reads the store after the seed upserts (plan step 5) so the snapshot and its Signature reflect persisted reality rather than an in-memory guess"
  - "D26-02-D: SetRetrievalEnabled fails closed on an unknown slug via RowsAffected (an admin action against a missing base must be visible, not a silent 0-row success)"
metrics:
  duration: ~11 min
  completed: 2026-09-06
  tasks: 2
  commits: 4
  test_functions: 15
  files: 5
  insertions: 942
status: complete
---

# Phase 26 Plan 02: KB Registry Persistence + Production Assembly Summary

**One-liner:** sqlite `knowledge_bases` mutable config table behind an additive D14-E1 migration plus the `SQLiteStore` `kb.Store` adapter and `kb.Open`'s seed/runtime merge — an admin deactivation now survives a restart, a PR's new base reaches existing deployments, and an admin-created base is never recycled by the seed.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `knowledge_bases` additive DDL (role CHECK at the storage layer) + `idxKnowledgeBasesRole` at the migrate() tail + `SQLiteStore`'s three `kb.Store` verbs with the compile-time `var _ kb.Store` assertion | `530d128` (RED), `1217cdb` (GREEN) |
| 2 | `kb.Open(ctx, seedPath, Store) (Registry, error)`: 5-step merge semantics with the structure-only, field-diff-guarded seed upsert and a store re-read before snapshotting | `9acd54d` (RED), `e33f300` (GREEN) |

Trust semantics locked by the 15 new tests (7 sqlite under WSL CGO, 8 kb hermetic on Windows):

- **deactivation is never resurrected** — `TestOpenSeedUpsertStructureOnly`: the seed renames `historical`, the persisted `retrieval_enabled=false` survives, and the store received exactly one upsert carrying the persisted switch, never the seed's initial `true` (Pitfall 6a).
- **deactivate, never destroy, at the storage layer** — `kb_registry.go` has zero `DELETE FROM knowledge_bases`; `TestSetRetrievalEnabled` flips the switch in isolation and stamps `updated_at` without touching `created_at`.
- **seed structure changes reach existing deployments** — `TestOpenNewSeedEntryReachesExistingDeploy` (Pitfall 6b) and `TestOpenFreshClone` (empty store + seed = 4 entries, 4 upserts).
- **the seed is not a garbage collector** — `TestOpenAdminCreatedRowSurvives`.
- **absent seed is the one legal degraded state** — `TestOpenSeedAbsent` (populated store passes untouched with zero writes; empty store yields an empty registry, no error — boot's zero-Skills guard stays the backstop).
- **fail-closed assembly** — `TestOpenSeedMalformed` (broken seed names the file; store failure carries the driver error) and `TestOpenContextCancelled` (AGNT-04).
- **closed role vocabulary backstopped by sqlite** — `TestRoleCheckConstraint` refuses `role='admin'` with a `CHECK constraint` error, not merely a Go validation.
- **migration discipline** — `TestMigrateIdempotent` (second migrate on the same connection is a no-op) and `TestExistingDBUpgrades` (pre-Phase-26 DB gains the table + `idx_knowledge_bases_role` via `New()` with the legacy row intact).
- **round-trip fidelity** — `TestKBRegistryRoundTrip` / `TestUpsertBaseIdempotentUpdate` (every field including switch and both timestamps; ON CONFLICT updates in place).

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| WSL `go test ./internal/store/sqlite/ -run 'TestKBRegistry\|TestUpsertBase\|TestSetRetrieval\|TestLoadBases\|TestRoleCheck\|TestMigrateIdempotent\|TestExistingDBUpgrades' -count=1 -v` | PASS (7/7) |
| WSL `go test ./internal/store/sqlite/ ./internal/kb/ -count=1` | PASS (both packages) |
| Windows `go test ./internal/kb/ -count=1` | PASS (hermetic, no CGO) |
| `go vet ./internal/store/sqlite/` / `go vet ./internal/kb/` | CLEAN |
| `go build ./internal/...` | OK (no internal package broke) |
| frozen eval lock / golden set | untouched — `git diff fe6ff1b..HEAD -- eval/` is empty; the two entries `git status --porcelain eval/` shows (`calibration.json`, `report.json`) are untracked artifacts that pre-date this plan (present in the session-start git status), not modifications |

Acceptance criteria all hold: `var _ kb.Store = (*SQLiteStore)(nil)` present; role column carries the CHECK; the index const is ExecContext'ed at schema.go:742, after the ALTER loop (line 636) — D14-E1 ordering intact; `grep -c "UPDATE knowledge_bases"` == 1 and `grep -c "DELETE FROM knowledge_bases"` == 0; 12 `?` placeholders (no string-built SQL); `grep -c "retrieval_enabled\|RetrievalEnabled" internal/kb/open.go` == 4; `TestOpenSeedUpsertStructureOnly` + `TestOpenNewSeedEntryReachesExistingDeploy` both present and green.

## Deviations from Plan

**1. [Rule 3 - Blocking, inherited from 26-01] `Open` returns `Registry`, not `*Registry`**
- **Found during:** Task 2 planning (the 26-01 SUMMARY handoff flagged it in advance)
- **Issue:** plan `<interfaces>` line 69 declares `func Open(...) (*Registry, error)`; `Registry` is the interface (the impl is the unexported `snapshot`), so that signature is not legal Go.
- **Fix:** `Open(ctx, seedPath, st Store) (Registry, error)`.
- **Commit:** `e33f300`

**2. [Rule 2 - Missing critical wiring] The Open-ed registry is store-backed, not `New(rows)`**
- **Found during:** Task 2
- **Issue:** the plan's step 5 says `New(rows)`; `New` builds a read-only registry (`Create`/`Deactivate` → `ErrReadOnly`). Consumed literally, 26-05's `s.KBRegistry.Create(req)` / `Deactivate` would fail closed and KBM-02's runtime management would not exist in production — a mutation that cannot persist must not happen at all, by 26-01's own posture.
- **Fix:** step 5 builds `newSnapshot(merged, st)` — the in-package constructor that wires the Store, which 26-01's SUMMARY explicitly licensed ("It can build the in-package `*snapshot` directly"). The re-read (plan step 5, kept) still sources the snapshot from the store, so only the mutator wiring differs from the letter of the plan.
- **Commit:** `e33f300`

**3. [Form] `memStore` map-backed fake in `open_test.go` instead of reusing `registry_test.go`'s `fakeStore`**
- The plan's step 5 (re-read the store after the upserts) needs a Store that APPLIES writes; the existing `fakeStore` only records them and is referenced by six 26-01 tests. A second same-package fake (`memStore`, map + call record + ctx-Err propagation) keeps those tests untouched — the plan itself offered "可提为同包共享 test helper". (`9acd54d`)

**4. [Form] Index const named `idxKnowledgeBasesRole`**
- Followed the plan's literal const name; it differs from the file's `createXxxIdxSQL` convention (`createKPICallsFuncIdxSQL` etc.). The index NAME (`idx_knowledge_bases_role`) and the D14-E1 tail placement match both the plan and the file's precedent. (`1217cdb`)

**5. [Form] `retrieval_enabled` acceptance grep needed a doc pass**
- The criterion wants ≥3 switch references in open.go as evidence the switch semantics live there; the first draft had 2. Strengthened the package-header verdict paragraph to name the column explicitly (genuine documentation of the load-bearing rule), now 4. (`e33f300`)

## Notes for Downstream Plans

- **26-05:** the `Registry` returned by `kb.Open` is store-backed — `Create`/`Deactivate` persist and MUST be accompanied by the caller's insert-only audit rows (`action_kb_created` / `action_kb_deactivated`), which this layer deliberately does not write. `SetRetrievalEnabled` errors on an unknown slug — map it to a 404, not a 500.
- **26-09 boot wiring:** `kb.Open` returns errors; boot decides fatality (`log.Fatalf` on seed/store failure per the LoadViewsDir posture at cmd/server/main.go:490-526). An empty registry with an absent seed is NOT an error here — the existing "zero Skills loaded" boot guard is the backstop.
- **Boot order stays load-bearing:** migrate (store `New`) → `kb.Open` → skill loader, so tier Skills validate against the merged registry (26-01 Consumer row 2).
- **UpsertBase is switch-writing:** the seed-merge path is what preserves a deactivation by passing the persisted value back through. Any future caller that upserts must not hand it a defaulted switch.
- Repo-wide `gofmt -l` noise under Go 1.25's gofmt is PRE-EXISTING (many untouched files flagged, e.g. the `alterDatasets*` alignment block in schema.go); only files created by this plan were formatted. Out of scope per the fix-attempt limit.

## Known Stubs

None. Every exported symbol has a full implementation; no TODO/FIXME/placeholder was introduced (the only grep hit in the touched files is the pre-existing `deep.TODO.ActiveForm` type reference in schema.go's plan_todos comment — the known gate false positive from 26-RESEARCH Pitfall 5). `cmd/server` is intentionally NOT wired to `kb.Open` — that is 26-09's scope, not a stub.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-02-1 seed resurrection of a deactivated base | structure-only upsert passing the persisted switch back through; `TestOpenSeedUpsertStructureOnly` + `TestRestartDeactivateStaysDeactivated` (26-01) |
| T-26-02-2 repudiation of registry changes | layering per plan: mutable table holds current state, caller writes the insert-only audit row (26-05); this file ships no audit path and no delete path |
| T-26-02-3 SQL injection | all three verbs fully `?`-parameterized (12 placeholders, zero string-built SQL); slugs whitelist-validated upstream |
| T-26-02-4 migration failure at boot | CREATE TABLE IF NOT EXISTS + tail index idempotent; `TestMigrateIdempotent` + `TestExistingDBUpgrades` lock both the fresh and legacy paths |
| T-26-SC dependency legitimacy | zero new dependencies (go.mod untouched) |

## Self-Check: PASSED

- Files exist: `internal/store/sqlite/{kb_registry,kb_registry_test}.go`, `internal/kb/{open,open_test}.go`, `internal/store/sqlite/schema.go` (modified) — all FOUND in `git diff --stat fe6ff1b..HEAD` (942 insertions, 0 deletions).
- Commits exist: `530d128`, `1217cdb`, `9acd54d`, `e33f300` — all FOUND in `git log` on `phase-20-p10-6-admin-console`.
- TDD gate sequence: `test(26-02)` RED commit precedes each `feat(26-02)` GREEN commit for both tasks.
- WSL combined suite and Windows hermetic kb suite both PASS after the final commit.
