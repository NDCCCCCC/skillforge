---
phase: 47-debt-cleanup-reproducible-baseline
plan: 02
subsystem: ontology/governance
tags: [shacl, rollback, tbox, cache, atomic-value, rel-02]

# Dependency graph
requires:
  - phase: 46-entity-shacl-shapes-provenance
    provides: "shacl.Report struct with Trigger field, ValidateWithTrigger(kg, paths, trigger), ShapesPathsFromEnv single read point (D-46-02)"
  - phase: 40-data-governance-shacl-provenance-tbox-versioning
    provides: "store.SetShaclReport/GetShaclReport cache API, GET /api/admin/governance/shacl read handler, RollbackTBox step 11 warn-and-return-0 posture (D-PROV-09)"
provides:
  - "RollbackTBox republishes its post-rollback SHACL report into the store cache (m.kg.SetShaclReport) — closes audit WARNING-1 backend half of REL-02"
  - "Both RollbackTBox cache branches pinned by TestRollbackShaclParity: written on success (trigger=rollback), preserved on failure (seeded boot report survives)"
  - "Admin read-path documentation naming exactly two cache writers instead of overpromising"
affects: [47-03 (REL-01 emitter), 47-04 (FE report refetch, depends on the write being in place), 47-06 (cites e7ca19a as WARNING-1 evidence row), phase-48-ci]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Consume the report a validation already produced instead of re-running validation (zero-cost cache refresh)"
    - "Cache write inside the same m.mu critical section as the persistence writes — no TOCTOU between served report and on-disk TBox"
    - "Seed a pre-existing cache value before exercising a failure branch, so 'unchanged' assertions are non-vacuous"
    - "Warn-and-continue on non-critical side effects; never Store(nil) into atomic.Value"

key-files:
  created: []
  modified:
    - internal/ontology/induction/induction.go
    - internal/ontology/induction/induction_test.go
    - internal/server/admin_governance.go
    - internal/server/server.go

key-decisions:
  - "D-02a: domain layer owns the cache write (RollbackTBox), not the HTTP handler — reuses the already-computed report, no wire change, tests/CLI/MCP callers all consistent"
  - "D-02c: failed post-rollback validation keeps the previous report and still returns success with 0 violations — Steps 3-5 already persisted, so failing closed would lie to the client"
  - "D-02d: no store Clear/sentinel API added this phase; the prohibition is enforced by a comment plus a non-vacuous test, not a new API"
  - "D-05: boot executes Quarantine, rollback does not — documented, deliberately NOT fixed in this plan"

patterns-established:
  - "Non-vacuous failure-branch assertion: seed a boot report before the failing operation, then assert pointer identity + trigger, because nil-on-an-empty-store passes either way"
  - "Mutation-verified tests: temporarily inject a SetShaclReport probe on the failure branch, confirm the assertion fails, revert (probe used here to prove D-02c detection power)"

requirements-completed: [REL-02]

# Metrics
duration: 12min
completed: 2026-10-19
---

# Phase 47 Plan 02: Rollback SHACL Cache Refresh Summary

**A successful TBox rollback now republishes the revalidation report it already computed into the governance SHACL cache, so `GET /api/admin/governance/shacl` stops serving the stale boot report — closing the backend half of audit WARNING-1 (REL-02).**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-10-19
- **Completed:** 2026-10-19
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- **Closed the WARNING-1 cache gap at zero extra cost.** `RollbackTBox` already ran `ValidateWithTrigger(..., "rollback")` and threw the report away, keeping only `ErrorCount`. It now writes that exact report via `m.kg.SetShaclReport(report)` — the same object, one line below its production, no second validation pass.
- **Eliminated the TOCTOU window.** The cache write sits inside the same `m.mu` critical section as the `v{N+1}` / pointer-TTL / version-file writes (Steps 3-5), so no reader can ever see a report that disagrees with the on-disk TBox it describes.
- **Made the failure branch provably non-destructive, not accidentally so.** `TestRollbackShaclParity` now seeds a boot report before the shapes-load failure and asserts it survives — same pointer, `Trigger` still `"boot"`. Mutation-verified: injecting a `SetShaclReport` probe on the error branch made both new assertions fail, so the guard has real detection power.
- **Pinned SC-2's agreement claim directly.** The success branch asserts the cached report's `ErrorCount` equals the violation count the rollback response returned, proving both come from the same `ValidateWithTrigger` result.
- **Stopped the documentation from hiding the bug.** The read-path header claimed "cached report from boot or post-rollback" while only boot wrote it — precisely the stale claim that let WARNING-1 sit undetected. Header, handler doc and route comment now name both writers and state the failed-validation fallback honestly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write the post-rollback report into the SHACL cache** - `e7ca19a` (fix)
2. **Task 2: Pin both cache branches in TestRollbackShaclParity** - `d3f86c2` (test)
3. **Task 3: Correct the read-path comments to name both writers** - `c90f91f` (docs)

**Evidence commit for plan 47-06:** `e7ca19a` — the WARNING-1 evidence row should cite this SHA.

## Files Created/Modified

- `internal/ontology/induction/induction.go` — RollbackTBox Step 11 now calls `m.kg.SetShaclReport(report)` on the success path only; the error branch is unchanged apart from a prohibition comment on `SetShaclReport(nil)`. No new import, no new error return, no fail-closed behavior.
- `internal/ontology/induction/induction_test.go` — both `TestRollbackShaclParity` subtests extended with cache-state assertions (success: written with `trigger == "rollback"`; failure: seeded boot report survives). Sole new import is `internal/ontology/shacl`, which `induction.go` already imports, so no new dependency edge.
- `internal/server/admin_governance.go` — wire-contract header and handler doc comment corrected (comment-only).
- `internal/server/server.go` — route comment above `GET /api/admin/governance/shacl` corrected (comment-only).

`internal/ontology/store/store.go` was **not** touched (`git diff --stat` empty), per the plan's read-only constraint.

## Verification

| Gate | Result |
|---|---|
| `CGO_ENABLED=1 go build ./...` | exit 0 |
| `CGO_ENABLED=1 go vet ./internal/ontology/induction/` | exit 0 |
| `CGO_ENABLED=1 go test ./internal/ontology/induction/ -count=1` | ok |
| `CGO_ENABLED=1 go test ./internal/ontology/induction/ -run '^TestRollbackShaclParity$' -count=1 -v` | both subtests `--- PASS` |
| `CGO_ENABLED=1 go test ./internal/server/ -count=1` | ok (17.3s) |
| `git diff --stat internal/ontology/store/store.go` | empty |
| Non-comment `SetShaclReport(` call sites in `induction.go` | exactly 1 |
| Non-comment `SetShaclReport(nil)` / `Store(nil)` in `internal/` + `cmd/` | none |
| Task 3 diff is comment-only | confirmed (`grep '^[+-]' \| grep -v '^[+-][+-]' \| grep -v '^[+-][[:space:]]*//'` empty) |

### Known pre-existing failure (not this plan's scope)

`CGO_ENABLED=1 go test ./internal/ontology/... -count=1` reports one failure:

```
--- FAIL: TestEmitSubCategoryInstanceCount (0.03s)
    emitter_test.go:83: SubCategory instances = 6, want 7
```

This is **D-45-DEF-01**, explicitly enumerated in `47-CONTEXT.md` D-04b/D-04c and owned by plan 47-03 (emitter hermeticize). `internal/ontology/emitter/` is not in this plan's `files_modified` and was not touched. All four packages this plan does own — `induction`, `shacl`, `store`, `server` — are green.

## Deviations from Plan

None. All three tasks executed as written; no scope was added or dropped.

One addition within Task 2's stated intent: a **mutation probe** was run to demonstrate the new failure-branch assertions are not vacuous (the temporary `SetShaclReport` probe on the error branch made both assertions fail; it was reverted with `git checkout` before the task commit, and the committed tree contains no probe code). This is the "provably non-destructive, not merely unchanged by accident" success criterion being satisfied by evidence rather than by assertion.

## Notes for Downstream Plans

- **47-04 (FE report refetch)** can rely on the backend write being in place: after a rollback, the next `GET /api/admin/governance/shacl` returns `trigger: "rollback"`, which makes `GovernanceTab`'s previously dead `report.trigger === 'boot' ? '启动时' : '回滚后'` branch live with no render change.
- **47-06** should cite `e7ca19a` as the WARNING-1 evidence row.
- The FE half of REL-02 (D-02b) is **not** done here — `TimelineView` still refetches only the changelog, so the live branch is not yet reachable from the UI.
- **D-05 known limitation stands:** boot executes `Quarantine`, rollback does not, so the banner's "N 个实体已隔离" still overstates on the rollback path. Documented in the code comment at the new write site; disposition belongs in `47-DEBT-DISPOSITION.md`.

---
*Plan: 47-02 | Phase 47: 清债与可复现基线 | REL-02 backend half*
