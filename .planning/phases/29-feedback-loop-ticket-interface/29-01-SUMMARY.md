---
phase: 29-feedback-loop-ticket-interface
plan: 01
subsystem: database
tags: [sqlite, go, session-turn, idempotency, mattn-go-sqlite3, message-identity]

# Dependency graph
requires:
  - phase: 03-compliance-vertical-slice
    provides: store.Store interface + SQLiteStore + session_turn UNIQUE(session_id, role, ts) idempotency (D18)
  - phase: 14-p10-0-cgo-trust-floor
    provides: mattn/go-sqlite3 CGO driver + DSN pragma discipline (D14-C1)
  - phase: 24-audit-chain-hardening
    provides: session_turn.id column provenance (PROV export already reads it)
provides:
  - "AppendTurn returns the assistant row's session_turn.id as (int64, error)"
  - "SessionMessage.ID read-back field filled by GetSession"
  - "writeAuditAndTurn returns (assistant turn id, error) — the D-29-01 message identity channel"
  - "conflict-replay returns exactly 0 (Pitfall 2 guard), hermetically locked by TestAppendTurnReturnsAssistantID"
affects: [29-02 feedback tables, 29-04 data-turn wire frame, 29-05 vote endpoints, 29-03 history voted_verdict]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RowsAffected()==0 guard before LastInsertId — the only safe idempotent-replay id contract under mattn/go-sqlite3"
    - "2-value-return call sites discard with `_, _ =` (single `_ =` is a Go compile error)"
    - "test fakes implementing AppendTurn return (0, nil); 0 = degrade signal for downstream data-turn emitters"

key-files:
  created:
    - internal/store/sqlite/session_turnid_test.go
  modified:
    - internal/store/store.go
    - internal/store/sqlite/session.go
    - internal/server/chat.go
    - internal/server/pipeline.go
    - internal/server/runner_setup.go
    - internal/server/admin_audit_test.go
    - internal/server/admin_schedules_test.go
    - internal/server/upload_test.go
    - internal/store/store_test.go
    - internal/store/sqlite/session_meta_test.go
    - internal/store/sqlite/mattn_zero_regression_test.go
    - internal/server/handlers_session_history_test.go
    - internal/server/pipeline_history_test.go

key-decisions:
  - "conflict replay returns 0 via a RowsAffected()==0 guard — LastInsertId is never consulted after a swallowed ON CONFLICT (mattn/go-sqlite3 would return the connection's most recent successful INSERT rowid: a silent cross-turn mismatch, i.e. the exact T-29-06 misbinding)"
  - "writeAuditAndTurn keeps its best-effort posture: Store==nil and append-error both return (0, nil) with the existing WARN log — no error surfaces to callers this plan"
  - "all 12 call sites discard with `_, _ =` plus a comment pointing at Plan 29-04's data-turn emission window — this plan opens the return channel only"

patterns-established:
  - "AppendTurn id==0 sentinel: callers must not emit a data-turn frame for id 0 (FE degrades to a disabled vote row)"
  - "SessionMessage.ID is read-back only: GetSession fills it, AppendTurn ignores it on input (AUTOINCREMENT assigns)"

requirements-completed: [FBK-01, FBK-04]

# Metrics
duration: 13min
completed: 2026-09-07
---

# Phase 29 Plan 01: Turn-id Foundation Summary

**AppendTurn now returns the assistant row's session_turn.id with a RowsAffected conflict-replay guard returning exactly 0 — the server-side message identity (D-29-01) that Phase 29's vote/ticket idempotency keys on, rippled through the interface, sqlite impl, 12 call sites and 5 test fakes in one atomic commit**

## Performance

- **Duration:** 13 min
- **Started:** 2026-09-07T18:32:31Z
- **Completed:** 2026-09-07T18:44:50Z
- **Tasks:** 2
- **Files modified:** 14 (13 in Task 1's atomic ripple + 1 new test file in Task 2)

## Accomplishments
- `store.Store.AppendTurn` signature is now `(ctx, sessionID, user, assistant) (int64, error)` returning the assistant row's `session_turn.id`; `SessionMessage` gained the read-back `ID int64` field
- `getSessionSQL` selects `id` and `GetSession` scans it into `m.ID` — every history row now carries its server-side message identity
- `appendTurnHalf` guards with `RowsAffected()==0` before `LastInsertId`, returning 0 on the idempotent replay path (Pitfall 2: mattn's `LastInsertId` after a swallowed conflict is a stale connection rowid)
- `writeAuditAndTurn` returns `(int64, error)`; all 12 call sites (chat.go x4, pipeline.go x6, runner_setup.go x2) compile with `_, _ =` discard + 29-04 emit-window comments
- `TestAppendTurnReturnsAssistantID` (4 hermetic subtests) locks fresh-id / conflict-0 / GetSession-id-monotonic / closed-store semantics

## Task Commits

Each task was committed atomically:

1. **Task 1: AppendTurn signature + implementation + full call-site/fake ripple** - `b5c7a4c` (feat)
2. **Task 2: turn-id semantics hermetic tests** - `c47d15d` (test)

## Files Created/Modified
- `internal/store/store.go` — AppendTurn `(int64, error)` signature + SessionMessage.ID field with contract godoc
- `internal/store/sqlite/session.go` — getSessionSQL `SELECT id, role, content, ts`; appendTurnHalf RowsAffected guard + LastInsertId
- `internal/server/chat.go` — writeAuditAndTurn `(int64, error)`; best-effort posture preserved (nil-store / append-error → `0, nil`)
- `internal/server/pipeline.go` — 6 call sites adapted + 29-04 emit-window comment at the verdict path
- `internal/server/runner_setup.go` — 2 call sites adapted
- `internal/server/admin_audit_test.go`, `internal/server/admin_schedules_test.go`, `internal/server/upload_test.go` — 5 explicit AppendTurn fakes resynced to `(int64, error)` returning `(0, nil)`
- `internal/store/store_test.go`, `internal/store/sqlite/session_meta_test.go`, `internal/store/sqlite/mattn_zero_regression_test.go`, `internal/server/handlers_session_history_test.go`, `internal/server/pipeline_history_test.go` — direct AppendTurn call sites adapted
- `internal/store/sqlite/session_turnid_test.go` — NEW: 4-subtest turn-id semantics lock

## Decisions Made
- Conflict replay returns 0 through an explicit `RowsAffected()` check; `LastInsertId` is structurally unreachable on that branch (the T-29-06 mitigation the plan's threat model calls `mitigate`)
- The 5 explicit test fakes return `(0, nil)` instead of `panic("not implemented")` per the plan — godoc notes 0 = no real row id, downstream data-turn emitters degrade
- `kb_admin_test.go`'s `recordingAuditStore` untouched: it embeds `store.Store`, so the new signature is inherited automatically (checker's Pitfall 1 count of 5 explicit fakes confirmed correct)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `_ = s.writeAuditAndTurn(...)` does not compile for a 2-value return**
- **Found during:** Task 1 (first verification run)
- **Issue:** The plan's prescribed discard form `_ =` is a Go compile error ("assignment mismatch: 1 variable but returns 2 values") — Go requires the blank identifier per return value
- **Fix:** All 12 call sites use `_, _ = s.writeAuditAndTurn(...)`; comments and call order unchanged
- **Files modified:** internal/server/chat.go, internal/server/pipeline.go, internal/server/runner_setup.go
- **Verification:** `go build ./...` exit 0 in WSL
- **Committed in:** b5c7a4c (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — Go syntax mechanics of the plan's own prescribed form)
**Impact on plan:** No scope creep; the prescribed `_ =` form was mechanically impossible.

## TDD Gate Compliance

Task 2 is marked `tdd="true"` but its test commit (`c47d15d`) follows Task 1's implementation commit (`b5c7a4c`) by plan design — the plan explicitly orders the interface+implementation+ripple as ONE atomic commit (D16-03: splitting them breaks the `var _ store.Store` compile gate), with the test file as a separate tdd commit. The behavior therefore could not be demonstrated RED before GREEN; the tests lock rather than drive it. Both gate commits exist in order (`feat` then `test`); the missing RED demonstration is documented here rather than faked.

## Issues Encountered
- `mattn_zero_regression_test.go:78` needed `_, err =` (the enclosing scope already declared `err`) — caught by the compiler on the second run, fixed inline
- Pre-existing working-tree modifications (`internal/server/knowledge.go`, 2 FE files) were left untouched and unstaged per orchestrator constraints

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 29-02 (feedback/tickets tables) can key votes on `session_turn.id` — the store contract and its idempotency semantics are locked by hermetic tests
- 29-04 (data-turn wire frame) has its emission window marked in chat.go and pipeline.go; the turn id already flows to the `writeAuditAndTurn` return value
- 29-03 (history voted_verdict) can read row ids from `GetSession` (`m.ID`) with zero further store changes
- No blockers; no new dependencies (T-29-SC supply-chain surface stays empty)

## Self-Check: PASSED

- internal/store/sqlite/session_turnid_test.go — FOUND
- Commits b5c7a4c, c47d15d — FOUND in git log
- WSL `go build ./... && go vet ./internal/store/... ./internal/server/ && go test ./internal/store/... ./internal/server/... -count=1` — all green post-Task-2

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-07*
