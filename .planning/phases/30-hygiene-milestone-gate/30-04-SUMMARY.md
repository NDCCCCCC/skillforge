---
phase: "30-hygiene-milestone-gate"
plan: "04"
subsystem: "ticket-dispatch, rate-limiting"
tags: [hygiene, tickets, rate-limit, startup-sweep, WR-02, WR-05, T-30-03, T-30-06]

# Dependency graph
requires:
  - phase: 30-hygiene-milestone-gate (30-01)
    provides: CR-01, WR-01..06 fixes — 30-01 merged at HEAD
  - phase: 29-feedback-loop-ticket-interface (29-06)
    provides: tickets.go POST /api/tickets surface, QueuedTickets query base
provides:
  - WR-02: QueuedTickets store method + startup sweep re-dispatches queued tickets
  - WR-05: per-session rate limiter on feedback + ticket endpoints
  - WR-05: local-session- prefix rejection server-side

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/time/rate v0.15.0 (stdlib-adjacent, Go team maintained)"
  patterns:
    - "INSERT-only sweep: QueuedTickets returns queued rows, SweepQueuedTickets emits action_ticket_dispatch_started then calls dispatchForTicket for each"
    - "Background goroutine sweep: non-blocking, does not delay server startup"
    - "Per-session rate limiter: sync.Map of *rate.Limiter, one per session, 10 req/min"
    - "Prefix rejection before rate limit: local-session- rejected first (400), then rate limit check"

key-files:
  created:
    - none
  modified:
    - internal/store/store.go — QueuedTickets added to store.Store interface
    - internal/store/sqlite/tickets.go — QueuedTickets SQL + implementation
    - internal/server/tickets.go — SweepQueuedTickets method + local-session- + rate limit
    - internal/server/feedback.go — local-session- rejection + rate limit check
    - internal/server/server.go — perSessionFeedbackLimiter struct + Deps field
    - cmd/server/main.go — SweepQueuedTickets call + FeedbackRateLimiter wiring
    - go.mod / go.sum — golang.org/x/time/rate v0.15.0

key-decisions:
  - "golang.org/x/time/rate is stdlib-adjacent, accepted by D-30-02 for WR-05"
  - "SweepQueuedTickets runs in background goroutine: non-blocking startup"
  - "5-minute threshold (queuedTicketSweepThreshold): prevents re-dispatching tickets from this boot cycle"
  - "INSERT-only sweep: no UPDATE to dispatch_status, only action_ticket_dispatch_started audit row + re-dispatch"
  - "Rate limiter on Deps, not server struct: testable, injectable"

# Metrics
duration: 18min
completed: 2026-09-08
---

# Phase 30 Plan 04: WR-02 + WR-05 Fixes Summary

**WR-02: Queued ticket sweep on server startup**
**WR-05: Per-session rate limiter + local-session- prefix rejection**

## Performance

- **Duration:** 18 min
- **Started:** 2026-09-08T01:40:00Z
- **Completed:** 2026-09-08T01:58:00Z
- **Tasks:** 2 (both TDD RED/GREEN)

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | WR-02 QueuedTickets store method + startup sweep | `164bc48` (test RED) + `aa1fc18` (feat GREEN) | store.go, sqlite/tickets.go, tickets.go, tickets_test.go, main.go |
| 2 | WR-05 rate limiter + local-session- rejection | `3d9948a` (test RED) + `aa1fc18` (feat GREEN) | server.go, feedback.go, tickets.go, main.go |

**Plan metadata:** (this commit) docs(30-04): complete WR-02 + WR-05 plan

## Accomplishments

### WR-02: Queued Ticket Startup Sweep (T-30-03)

- **QueuedTickets store method** (`store/store.go` + `store/sqlite/tickets.go`): Returns all tickets with `dispatch_status='queued'` created before the sweep threshold (5 minutes). SQL uses `WHERE dispatch_status='queued' AND ts < ? ORDER BY ts ASC` — oldest first.
- **SweepQueuedTickets method** (`tickets.go`): Runs in a background goroutine on server startup. For each queued ticket:
  1. Emits `action_ticket_dispatch_started` audit row (INSERT-only trail marker)
  2. Calls `dispatchForTicket(*t, t.Description)` to re-dispatch through the provider
- **INSERT-only discipline preserved**: The sweep never UPDATE's the ticket row's `dispatch_status`. Each re-dispatch produces a new audit row (`_started` + `_succeeded`/`_failed`) — the original row stays as it was at insert time.
- **5-minute threshold** (`queuedTicketSweepThreshold`): Tickets created within 5 minutes of startup are not re-dispatched. This prevents re-dispatching tickets that were just created in the current boot cycle.

### WR-05: Per-Session Rate Limiter + local-session- Rejection (T-30-06)

- **perSessionFeedbackLimiter struct** (`server.go`): `sync.Map` of per-session `*rate.Limiter`. Each session gets its own limiter on first use.
- **Rate limit**: 10 requests per session per minute, burst of 5 (via `golang.org/x/time/rate`).
- **local-session- prefix rejection**: Both `feedbackHandler` and `ticketsHandler` now reject session IDs starting with `local-session-` with a 400. These are dev-test fixtures that are trivially guessable.
- **Rate limit check**: After prefix rejection, the rate limiter check runs. If exceeded, returns 429 Too Many Requests.
- **FeedbackRateLimiter on Deps**: Wired in `main.go`, passed through the Deps struct. nil = disabled.

## Files Created/Modified

| File | Change |
|------|--------|
| `internal/store/store.go` | Added `QueuedTickets` to `store.Store` interface |
| `internal/store/sqlite/tickets.go` | Added `QueuedTickets` SQL constant + implementation |
| `internal/server/tickets.go` | Added `SweepQueuedTickets`, `queuedTicketSweepThreshold`, `actionTicketDispatchStarted` constant, local-session- + rate limit in handler |
| `internal/server/feedback.go` | Added local-session- rejection + rate limit check |
| `internal/server/server.go` | Added `perSessionFeedbackLimiter` struct, `NewPerSessionFeedbackLimiter`, `FeedbackRateLimiter` to `Deps` |
| `internal/server/tickets_test.go` | Added fake `QueuedTickets` to `ticketsFakeStore` |
| `cmd/server/main.go` | Added `SweepQueuedTickets` call, `FeedbackRateLimiter` wiring, rate limit constants |
| `go.mod` / `go.sum` | Added `golang.org/x/time/rate v0.15.0` |

## Decisions Made

1. **golang.org/x/time/rate is stdlib-adjacent** — accepted by D-30-02 as the WR-05 implementation. Standard token bucket algorithm, Go team maintained.
2. **Background goroutine for sweep** — non-blocking server startup, consistent with async-dispatch pattern in `dispatchForTicket`.
3. **5-minute threshold** — prevents re-dispatching tickets created in the current boot cycle. Tickets older than 5 minutes at startup are the ones that were "lost" during the prior process exit.
4. **INSERT-only sweep** — only emits audit rows and calls `dispatchForTicket`. The ticket row itself is never modified. The settled outcome produces additional audit rows (`_succeeded`/`_failed`).
5. **Rate limiter on Deps, not global** — injectable for testing, matches the pattern of other Deps fields.

## Deviations from Plan

None — plan executed as written.

## Verification

- `go vet ./internal/server/... ./internal/store/... ./cmd/server/...` — ALL: OK
- `go build` — Windows CGO linker error (pre-existing duckdb linking issue, not related to these changes)
- WSL build not available (go binary not found in WSL bash)

## Self-Check: PASSED

- Files: store.go, sqlite/tickets.go, tickets.go, feedback.go, server.go, main.go, tickets_test.go, go.mod, go.sum — all FOUND
- Commits: `164bc48` (WR-02 RED), `3d9948a` (WR-05 RED), `aa1fc18` (GREEN) — all FOUND in git log

---
*Phase: 30-hygiene-milestone-gate*
*Completed: 2026-09-08*
