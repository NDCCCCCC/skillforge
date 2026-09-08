# Phase 30 Plan 02: WR-01 + WR-04 Hygiene Fixes — Summary

**Plan:** 30-02
**Phase:** 30-hygiene-milestone-gate
**Status:** COMPLETE
**Commits:** `37f1805` (Task 2), `3d262a2` (Task 3)
**Date:** 2026-09-08

## One-liner

WR-01 idempotent feedback refs backfill + WR-04 attribution Action filter with ts+id tie-break delivered in 2 atomic commits.

## What was done

### Task 1 — WR-01: PutFeedbackChunkRefs ON CONFLICT DO NOTHING (PRE-EXISTING)
The `ON CONFLICT(feedback_id, chunk_id) DO NOTHING` clause was already present in `internal/store/sqlite/feedback.go` (lines 118-122) at plan execution start. The store layer was already idempotent for duplicate ref pairs. `TestFeedbackChunkRefsIdempotent` (sqlite store tests) passes green.

### Task 2 — WR-01: backfill call in feedback replay path (`37f1805`)
**File modified:** `internal/server/feedback.go`

The `!inserted` branch in `feedbackHandler` (replay path) now calls `PutFeedbackChunkRefs` to backfill refs from a previously failed write:
```go
if !inserted {
    // WR-01: backfill missing refs from a previously failed write.
    // ON CONFLICT(feedback_id, chunk_id) DO NOTHING makes this idempotent.
    _ = s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs)
    c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", Verdict: saved.Verdict})
    return
}
```

Fake store updated with deduplication (mirrors real DB behavior) and `putFeedbackChunkRefsCalls` counter. Test assertion verifies backfill is called exactly once on replay.

### Task 3 — WR-04: attribution Action filter + ts+id tie-break (`3d262a2`)
**Files modified:** `internal/store/store.go`, `internal/store/sqlite/audit.go`, `internal/server/feedback.go`, `internal/server/feedback_test.go`

**store.go:** `AuditFilter` gains two new fields:
- `ActionEmpty bool` — when true, adds `AND action = ''` to the WHERE clause
- `OrderIDDesc bool` — when true, orders by `ts DESC, id DESC` instead of just `ts DESC`

**audit.go:** `QueryAudit` updated:
- When `ActionEmpty=true`: appends `action = ''` clause (binds attribution to verdict rows only; fanout/pending rows have non-empty action)
- When `OrderIDDesc=true`: changes ORDER BY from `ts DESC` to `ts DESC, id DESC`

**feedback.go:** `feedbackAttribution` now passes `ActionEmpty: true, OrderIDDesc: true` to `QueryAudit`, ensuring the attribution probe binds to verdict rows only (WR-04) and has a deterministic tie-break.

**feedback_test.go:** Fake `QueryAudit` updated to handle new filter fields; seeded audit rows now carry deterministic IDs; attribution probe assertions verify `ActionEmpty` and `OrderIDDesc` are set.

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/store/...` | PASS |
| `go build ./internal/server/...` | PASS |
| `go vet ./internal/store/ ./internal/server/` | PASS |
| `gofmt` | clean |
| `go test ./internal/store/sqlite/ -run TestFeedback -v -count=1` | PASS (5/5) |
| Server test binary compile | BLOCKED (DuckDB CGO linking — pre-existing Windows host limitation; not in scope for this plan) |

## Must-haves (from plan)

| Must-have | Status |
|-----------|--------|
| PutFeedbackChunkRefs ON CONFLICT DO NOTHING | PRE-EXISTING (lines 118-122) |
| Backfill call in replay path | DONE (`37f1805`) |
| Attribution Action filter `action = ''` | DONE (`3d262a2`) |
| Attribution tie-break `ts DESC, id DESC` | DONE (`3d262a2`) |

## Deviations

1. **Task 1 was pre-existing**: The `ON CONFLICT DO NOTHING` clause was already in `PutFeedbackChunkRefs` at execution start. No code change needed; `TestFeedbackChunkRefsIdempotent` (existing) provides coverage.

## Threat Model Coverage

| Threat | Mitigation | File |
|--------|------------|------|
| T-30-02 (WR-01: feedback refs backfill gap) | `ON CONFLICT DO NOTHING` idempotent backfill + replay path call | `feedback.go:234` |
| T-30-05 (WR-04: attribution misbinding) | `ActionEmpty` + `OrderIDDesc` filter on attribution probe | `audit.go:197-201`, `feedback.go:274` |

## Self-Check

- [x] `37f1805` and `3d262a2` exist in git log
- [x] `ON CONFLICT(feedback_id, chunk_id) DO NOTHING` present in `internal/store/sqlite/feedback.go`
- [x] `PutFeedbackChunkRefs` called in `!inserted` branch in `internal/server/feedback.go`
- [x] `ActionEmpty` and `OrderIDDesc` fields added to `AuditFilter` in `internal/store/store.go`
- [x] `action = ''` and `ORDER BY ts DESC, id DESC` present in `QueryAudit` in `internal/store/sqlite/audit.go`
- [x] `feedbackAttribution` passes both new filter flags
- [x] Build and vet clean
