---
phase: 29-feedback-loop-ticket-interface
plan: 02
subsystem: database
tags: [sqlite, insert-only, idempotency, feedback, tickets, additive-migration, source-grep-lock]

# Dependency graph
requires:
  - phase: 29-feedback-loop-ticket-interface (29-01)
    provides: AppendTurn → (int64, error) + SessionMessage.ID read-back — session_turn.id is the turn identity every vote/ticket row keys on
  - phase: 29-feedback-loop-ticket-interface (29-03)
    provides: internal/ticket provider registry (none/webhook) — the provider vocabulary the tickets.provider column records
  - phase: 26 (KBM)
    provides: knowledge_bases slug registry — kb_slug semantic (= retriever chunk Tier) written into feedback_chunk_refs
provides:
  - 4 additive tables (feedback / feedback_chunk_refs / feedback_dispositions / tickets) + 6 migrate()-tail read-path indexes, fresh + legacy proven
  - 14 new Store methods (12 feedback-side + PutTicket/TicketByTurn) with 6 new shared types on internal/store.Store
  - UNIQUE-key idempotency substrate: one vote per (session,turn) + one ticket per (session,turn), RowsAffected-guarded
  - Derived pending-review read model (latest-disposition-wins + reopen, Open Q1) and full-denominator metrics SQL (D-29-10) + per-KB once-per-KB accounting (Open Q2)
  - TestFeedbackInsertOnly / TestTicketsInsertOnly source-grep structural locks (zero UPDATE/DELETE verbs)
affects: [29-04 history echo (VotedVerdictsBySession), 29-05 feedback+admin endpoints, 29-06 tickets endpoint, 29-07 FE FeedbackRow, 29-08 admin feedback tab, 29-09 verify gate]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies (RESEARCH legitimacy audit empty table)
  patterns:
    - "kpi_calls.go mirror: SQL consts + fmtError + isClosed guard + `?`-only parameterization + hardcoded clause constants + strings.Join"
    - "session.go ON CONFLICT DO NOTHING + RowsAffected replay guard (LastInsertId never consulted on the swallowed path)"
    - "derived read model: pending view from per-vote NOT EXISTS, no second write path"
    - "aggregate timestamp TEXT parse: sqlite does not propagate DATETIME decltype through MAX"
key-files:
  created:
    - internal/store/sqlite/feedback.go
    - internal/store/sqlite/tickets.go
    - internal/store/sqlite/feedback_test.go
    - internal/store/sqlite/tickets_test.go
  modified:
    - internal/store/sqlite/schema.go
    - internal/store/store.go

key-decisions:
  - "PutTicket/TicketByTurn interface signatures deferred to Task 3's commit — declaring them at Task 2 would break the var _ store.Store compile gate on a commit that lacks the tickets impl (plan-mandated ordering, D16-03 lesson)"
  - "mattn native time.Time binding kept for feedback/dispositions/tickets ts — its optional-fraction format ('…:00+00:00' < '…:00.25+00:00') keeps lexicographic TEXT comparison chronologically correct, avoiding the RFC3339Nano trailing-zero trap recorded in D16-03-B"
  - "Aggregate MAX(f.ts) scanned as TEXT + parseSQLiteTS (4-layout list) because sqlite hands aggregate columns over without DATETIME decltype"
  - "report_count after reopen counts the UNRESOLVED batch (per-vote NOT EXISTS semantics) — the correct priority signal, not the all-time total"
  - "ResolvedDispositions fails closed on an unknown filter value (error, not empty result — empty would be read by the admin UI as 'no history')"
  - "empty provider/dispatch_status normalize to 'none' in Go: the INSERT names every column so the SQL DEFAULT never fires for a Go zero-value field; non-empty invalid values still fail closed at the CHECK"
  - "REQUIREMENTS FBK-01/02/03/TKT-02 stay Pending at phase level — this plan ships the storage half only; endpoints are 29-05/29-06, FE is 29-07/29-08 (29-01 precedent)"

patterns-established:
  - "TestXxxSchema DDL lock trio: PRAGMA table_info column order + sqlite_master index presence + CREATE-statement substring locks for UNIQUE key and CHECK vocabularies"
  - "INSERT-only family third member: feedback.go/tickets.go join kpi_calls.go/import_batches.go under source-grep locks"

requirements-completed: [FBK-01, FBK-02, FBK-03, TKT-02] # storage-layer half shipped here; endpoint/FE halves remain for 29-05/29-06/29-07/29-08 — phase-level rows intentionally left Pending

# Metrics
duration: 23min
completed: 2026-09-08
---

# Phase 29 Plan 02: 反馈/复核/工单存储层 Summary

**INSERT-only storage layer for the feedback loop: 4 additive tables + UNIQUE-key idempotency (one vote / one ticket per message), a derived pending-review view with reopen semantics, and full-denominator metrics SQL — locked by source-grep structure tests.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-07T19:12:14Z
- **Completed:** 2026-09-07T19:34:54Z
- **Tasks:** 3
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments
- Four additive tables (feedback / feedback_chunk_refs / feedback_dispositions / tickets) with UNIQUE(session_id, turn_id) idempotency keys, CHECK closed vocabularies, and 6 migrate()-tail indexes — proven on fresh AND legacy DBs (TestFeedbackSchema / TestTicketsSchema)
- Store interface grown by 14 methods + 6 types; all implemented with zero row-mutation or row-removal verbs (TestFeedbackInsertOnly / TestTicketsInsertOnly source-grep locks)
- Pending review queue is a pure derived read model: per-vote `NOT EXISTS` gives latest-disposition-wins AND reopen (Open Q1), report_count counts the unresolved batch; per-Skill metrics count every vote once (D-29-10 raw counts, ratio computed Go-side) while per-KB counts a multi-KB vote once per KB (Open Q2, godoc + test locked)

## Task Commits

Each task was committed atomically:

1. **Task 1: 四张新表 DDL + migrate() 尾部索引 + PRAGMA/schema 断言测试** - `fcc0383` (feat)
2. **Task 2: Store 接口新方法 + feedback.go 读写实现** - `3210b56` (feat)
3. **Task 3: tickets.go 幂等落库 + InsertOnly 锁** - `e45e72a` (feat)
4. **Cleanup: schema.go index doc-comment reflow to gofmt preference** - `fdb3255` (style, comment-only)

**Plan metadata:** `2ef1864` (docs: complete plan)

_Note: Tasks 2 and 3 are tdd="true" but have no standalone RED commit — the Store interface and its SQLiteStore implementation must land in the same commit or `var _ store.Store` breaks the build (D16-03 lesson; same advisory posture as 29-03 Task 2). RED was observed before each GREEN commit as the documented build failure on the undefined surface (store.FeedbackRow / s.PutFeedback … undefined)._

## Files Created/Modified
- `internal/store/sqlite/schema.go` — 4 additive CREATE TABLE IF NOT EXISTS + 6 index consts executed at the migrate() tail (D14-E1 convention); checkpoint header comment reworded so the file carries no `DO UPDATE` substring
- `internal/store/store.go` — FeedbackRow / ChunkRef / DispositionRow / PendingReviewRow / FeedbackCountRow / FeedbackTrendRow / TicketRow types + 14 interface methods with idempotency/INSERT-only godoc contracts
- `internal/store/sqlite/feedback.go` — 3 write paths (PutFeedback + refs + PutDisposition) + 9 read/aggregation paths, all `?`-parameterized
- `internal/store/sqlite/tickets.go` — putTicketSQL (ON CONFLICT DO NOTHING) + RowsAffected guard + session-bound TicketByTurn
- `internal/store/sqlite/feedback_test.go` — TestFeedbackSchema + 7 behavior tests + TestFeedbackInsertOnly
- `internal/store/sqlite/tickets_test.go` — TestTicketsSchema + 3 behavior tests + TestTicketsInsertOnly

## Decisions Made
- **14 methods, not 13:** the ROADMAP plan-list line said "13 个 Store 方法"; the plan's own `<interfaces>` block and `<must_haves>` both enumerate 14 (12 feedback-side + PutTicket/TicketByTurn). Shipped 14; ROADMAP line corrected in the shipped-marker edit.
- **Kept mattn time binding instead of RFC3339Nano TEXT:** the pending derivation compares ts values as TEXT across two tables; mattn's format is lexicographically chronologically correct on zero-nanosecond stamps, RFC3339Nano is not (D16-03-B).
- **Dispositions carry no UNIQUE and no status flip anywhere:** multiple dispositions are legal rows; the pending view derives state (Open Q1 resolved semantics locked by TestPendingReviewChunks).
- **Trend ordering is (day ASC, verdict ASC):** deterministic ordering so the 29-08 FE trend renders stably.
- **REQUIREMENTS rows left Pending** (see key-decisions) — storage layer alone does not satisfy the endpoint/FE wording.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pre-existing schema.go comment carried the literal `DO UPDATE` substring**
- **Found during:** Task 1 (verification)
- **Issue:** The Phase 8 checkpoint header comment said "(ON CONFLICT(id) DO UPDATE)" — Task 1's acceptance criterion is "schema.go 全文件不含 `DO UPDATE`", which a later phase gate (29-09 "DDL 只用 DO NOTHING" grep) would mechanically fail
- **Fix:** Reworded that one comment fragment to "INSERT ... ON CONFLICT(id), i.e. replace-on-replay" — zero behavior change (comment-only)
- **Files modified:** internal/store/sqlite/schema.go
- **Verification:** `grep -c "DO UPDATE" internal/store/sqlite/schema.go` == 0; full package tests green
- **Committed in:** fcc0383 (Task 1 commit)

**2. [Rule 2 - Missing Critical] Empty provider/dispatch_status normalization in PutTicket**
- **Found during:** Task 3 (GREEN run)
- **Issue:** The INSERT names every column, so the DDL `DEFAULT 'none'` never fires for a Go zero-value field — a half-populated TicketRow would land `""` and die on the CHECK constraint instead of taking the documented none-provider state
- **Fix:** Go-side fallback to "none" for empty provider/dispatch_status (the appendTurnHalf role-fallback pattern); any non-empty invalid value still fails closed at the CHECK
- **Files modified:** internal/store/sqlite/tickets.go, internal/store/sqlite/tickets_test.go
- **Verification:** TestTicketRoundTrip minimal-row sub-case green; CHECK failure no longer reachable for zero-value rows
- **Committed in:** e45e72a (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical)
**Impact on plan:** Both fixes preserve the plan's stated acceptance criteria and INSERT-only discipline. No scope creep.

## Issues Encountered
- **Aggregate timestamp scan:** `MAX(f.ts)` arrives as driver.Value string (sqlite does not propagate DATETIME decltype through MAX), so scanning into time.Time failed at runtime. Resolved with a small parseSQLiteTS helper over the writer layouts; plain column scans are unaffected (TestResolvedDispositions proved the driver converts those).
- **Two test-expectation bugs of mine caught by GREEN:** (a) trend rows sort verdict ASC within a day so "outdated" precedes "unhelpful"; (b) report_count after reopen is 1 (unresolved batch), not the all-time 3 — both were wrong expectations, not wrong implementation; the Pattern 5 SQL semantics stand.
- **Windows gofmt is untrustworthy on this host** (it rewrites `''` to a smart quote in comments); formatting verified via WSL gofmt instead. Pre-existing gofmt deltas in store.go (ReportRow alignment, FanoutEvent comment reflow) and schema.go (alterDatasets alignment) are NOT from this plan and were left untouched per scope boundary.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None — every method is wired end-to-end against the real tables; no placeholder returns. (The write-path consumers arrive in 29-05/29-06 by plan design; the store surface itself is complete and tested.)

## Next Phase Readiness
- 29-04 (history echo) can consume `VotedVerdictsBySession` immediately for the `voted_verdict` inline field
- 29-05 (feedback + admin endpoints) has the full storage surface: idempotent PutFeedback, refs write, PutDisposition, PendingReviewChunks/Count, ResolvedDispositions, metrics ×2 + trend, GetSessionTurn for the server-side D-29-05 attribution validation
- 29-06 (tickets endpoint) has PutTicket/TicketByTurn plus the documented "TKT-{row id}" number derivation convention
- 29-09's "DDL 只用 DO NOTHING" grep gate is already satisfiable: all three touched files grep zero for `DO UPDATE` and the two new files zero for `UPDATE `/`DELETE `
- Watch item for 29-05: comment text rides GROUP_CONCAT(char(10)) into PendingReviewChunks.Comments — a multi-line comment will split into multiple list entries (plan-prescribed trade-off; FE renders the list, so impact is cosmetic)

## Self-Check: PASSED

- Files verified on disk: internal/store/sqlite/{feedback.go, tickets.go, feedback_test.go, tickets_test.go, schema.go}, internal/store/store.go, 29-02-SUMMARY.md — all FOUND
- Commits verified in git log: fcc0383 (Task 1), 3210b56 (Task 2), e45e72a (Task 3), fdb3255 (style cleanup), 2ef1864 (plan docs) — all FOUND
- Plan-level verification re-run at HEAD: WSL `go build ./...` exit 0 + `go test ./internal/store/... -count=1` green + `grep -c "DO UPDATE"` == 0 across schema.go / feedback.go / tickets.go

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
