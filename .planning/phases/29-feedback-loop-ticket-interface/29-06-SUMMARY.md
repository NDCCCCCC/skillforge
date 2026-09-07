---
phase: 29-feedback-loop-ticket-interface
plan: 06
subsystem: http-api
tags: [hertz, tickets, idempotency, goroutine, panic-recovery, audit-trail, env-assembly, source-grep-lock, tkt-04]

# Dependency graph
requires:
  - phase: 29-feedback-loop-ticket-interface (29-03)
    provides: internal/ticket Provider.Dispatch/Ticket/DispatchResult + Config + ProviderNotWiredError + NewProvider registry ("none" | "webhook" with bounded retries) — the entire outbound seam this plan's goroutine calls
  - phase: 29-feedback-loop-ticket-interface (29-02)
    provides: store PutTicket (UNIQUE(session_id, turn_id), 'none' normalization, RowsAffected replay signal) + TicketByTurn (session-bound read) + GetSessionTurn + TicketRow (id is the "TKT-{id}" number source)
  - phase: 29-feedback-loop-ticket-interface (29-05)
    provides: the write-endpoint shape this plan mirrors (fixed gate order, idempotent replay 200, server-side attribution via the audit-nearby probe, best-effort audit-row posture) — feedbackAttribution is reused verbatim
  - phase: 29-feedback-loop-ticket-interface (29-01)
    provides: AppendTurn assistant row id (D-29-01) — the turn_id the ticket keys on
provides:
  - POST /api/tickets: idempotent public write endpoint — one ticket per message, 201 {ticket_number:"TKT-{row id}", dispatch_status:"none"|"queued"} or 200 {status:"already_exists", ticket_number:first} with NO second dispatch
  - structural non-blocking dispatch (TKT-04): background goroutine per queued row on a context.Background()+WithTimeout budget, defer recover containment, zero shared state with the chat/feedback main link
  - INSERT-only dispatch trail: action_ticket_dispatch_succeeded / action_ticket_dispatch_failed audit rows (ActorType=admin for the /api/admin/audit filter, UserID=system, FrozenArgs JSON snapshot) — the ticket row has no UPDATE path
  - Deps.TicketProvider / TicketProviderName / TicketDispatchBudget (nil-contract godoc) + route h.POST("/api/tickets") bound exactly once
  - cmd/server assembly: SKILLFORGE_TICKET_PROVIDER / _WEBHOOK_URL / _WEBHOOK_TIMEOUT_MS / _WEBHOOK_RETRIES / _DISPATCH_BUDGET_MS (+ optional _WEBHOOK_SECRET) with dual log.Fatalf fail-fast
affects: [29-07 FE FeedbackRow (ticket_number/dispatch_status wire contract frozen here), 29-08 admin FE (dispatch trail visible in /api/admin/audit), 29-09 verify gate (greps the locks this plan ships)]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "29-05 mirror: fixed gate order (shape 400s -> dependency 503s -> turn 404), replay reads the FIRST row back, audit probe degrades honestly, audit row best-effort"
    - "structural async: goroutine only for dispatch_status=queued rows, ctx from context.Background() (never the request), defer recover converts a panicking provider into a failed audit row"
    - "deadline-observation test trick: a bare request ctx carries no deadline, so asserting ctx.Deadline() inside the stub proves the goroutine ctx is budget-derived without timing games"
    - "comment-exempt line scan for INSERT-only locks (29-05 lesson): the grep skips // lines so prose can never trip a statement lock"
key-files:
  created:
    - internal/server/tickets.go
    - internal/server/tickets_test.go
  modified:
    - internal/server/server.go
    - cmd/server/main.go

key-decisions:
  - "TKT-04 is structural, not conventional: the 201 means 'accepted and durable', never 'delivered' — the dispatch runs in its own goroutine so a slow/broken webhook cannot stall the response, and the settled outcome lands in the audit trail instead of rewriting the ticket row"
  - "the dispatch ctx derives from context.Background() + the whole-dispatch budget: the user was told the ticket exists, so an abandoned HTTP request must never abort the outbound half (deadline-observation test locks this)"
  - "dispatch audit rows use ActorType=admin (admin_audit.go:72 filters on it — an invisible trail is an unfollowable trail) with UserID=system (the actor is the platform's dispatcher, not an admin user); both facts must survive in the row itself"
  - "a panicking provider IS a failed dispatch: recover writes action_ticket_dispatch_failed with a 'panic:' prefixed error, so the operator sees the outage in the audit view rather than in a crashed process"
  - "provider-nil 503 runs BEFORE the turn lookup (dependency gates precede existence gates) so an unwired deployment never leaks whether a turn exists; the 404 branch itself stays ONE merged envelope (missing / user-role / foreign session)"
  - "only dispatch_status=='queued' rows start a goroutine — 'none' means 仅落库不外发 with zero goroutines, matching the 29-03 noneProvider zero-side-effect contract"
  - "SKILLFORGE_TICKET_WEBHOOK_SECRET added at the assembly point (Rule 2 deviation): internal/ticket.Config.Secret exists since 29-03 and without an env binding the optional shared-secret header would be permanently unreachable from production"
  - "REQUIREMENTS rows stay Pending per the phase-close convention (29-02/29-04/29-05 precedent) — TKT-02 and TKT-04's substance is fully delivered here"

patterns-established:
  - "Pattern: async-after-accept write endpoint — persist -> read back -> 201 -> fire-and-forget goroutine; the replay branch must NEVER start a second dispatch"
  - "Pattern: recovered goroutine writes its own failure audit row, so panic containment and the audit trail are the same code path"

requirements-completed: [TKT-02, TKT-04] # substance delivered (backend complete); REQUIREMENTS.md rows flip at phase close per the 29-02/29-04/29-05 convention

# Metrics
duration: 16min
completed: 2026-09-08
---

# Phase 29 Plan 06: Ticket HTTP Surface Summary

**POST /api/tickets — idempotent per-message ticketing with a derived TKT-{id} number, an immediate 201 while the outbound dispatch runs in a recovered budget-capped goroutine, and every settled outcome recorded as an INSERT-only audit row.**

## Performance

- **Duration:** 16 min
- **Started:** 2026-09-07T20:58:48Z
- **Completed:** 2026-09-07T21:15:00Z
- **Tasks:** 2 (both TDD — RED/GREEN each)
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- TKT-02 complete at the HTTP layer: one ticket per (session, turn) enforced by the store's UNIQUE key, acceptance number derived as "TKT-{row id}" from the read-back (no number column exists to write back), dispatch_status ("none" | "queued") recorded at insert time and never rewritten.
- TKT-04 complete as a structural fact: the 201 returns in <<150ms against a 300ms provider; the goroutine's context is background-derived and budget-capped (abandoning the request cannot abort a ticket the user was told exists); a panicking provider is contained by `defer recover` and recorded as a failed dispatch.
- Every dispatch settles into the audit trail — `action_ticket_dispatch_succeeded` / `action_ticket_dispatch_failed` with a FrozenArgs JSON snapshot — visible in GET /api/admin/audit via ActorType=admin, while the ticket row keeps its insert-time shape forever.
- Operator surface fully env-driven: five SKILLFORGE_TICKET_* knobs (+ optional secret) with WARN+default on bad numerics and dual `log.Fatalf` fail-fast for webhook-without-URL and unknown provider names (D16-08-D — no silent degradation).

## Task Commits

Each task was committed atomically (TDD RED -> GREEN per task):

1. **Task 1: Deps 装配 + POST /api/tickets 幂等落库 + 即时响应** — `7148583` (test RED: documented build failure, ticketsHandler/Deps.TicketProvider* undefined) + `638b54e` (feat GREEN)
2. **Task 2: 异步外发 goroutine + 审计行 + main.go env 装配** — `56b772a` (test RED) + `e781bb3` (feat GREEN)

**Plan metadata:** (this commit) docs(29-06): complete ticket HTTP surface plan

## Files Created/Modified

- `internal/server/tickets.go` — ticketsHandler (fixed gate order + idempotent insert + immediate response), dispatchForTicket (queued-only goroutine, background budget ctx, recover, audit rows), putTicketDispatchAuditRow (ActorType=admin/UserID=system, best-effort), parseTicketChunkIDs (audit CSV -> id list, unparseable segments skipped)
- `internal/server/tickets_test.go` — 7-sub-case TestTicketsHandler behavior matrix, TestTicketDispatchAuditRows (success/failure/panic/budgeted-ctx), TestTicketDispatchNonBlocking, TestTicketsDispatchSourceDiscipline, TestTicketAssemblyWiring, TestTicketsRouteRegisteredOnce; ticketsFakeStore (nil-embed) + ticketStubProvider (delay/panic/error/deadline observation)
- `internal/server/server.go` — Deps.TicketProvider / TicketProviderName / TicketDispatchBudget with nil-contract godoc + `h.POST("/api/tickets")` registered exactly once
- `cmd/server/main.go` — ticket assembly block (5 env knobs + optional secret, registry constructor, dual log.Fatalf) + the three Deps fields wired

## Decisions Made

See `key-decisions` in the frontmatter. The load-bearing ones: TKT-04 non-blocking is goroutine-structural with a background-derived budget; the dispatch trail is ActorType=admin + UserID=system; a provider panic is a failed-dispatch audit event; the "none" provider starts zero goroutines; the webhook secret got its env binding.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing wiring] Added SKILLFORGE_TICKET_WEBHOOK_SECRET to the assembly point**
- **Found during:** Task 2 (main.go assembly)
- **Issue:** internal/ticket.Config.Secret (29-03) supports an optional X-SkillForge-Ticket-Secret header, but the plan's env list froze five knobs — without an env binding the capability is unreachable from production forever.
- **Fix:** `ticketSecret := envOr("SKILLFORGE_TICKET_WEBHOOK_SECRET", "")` wired into ticket.Config (empty = header not sent, byte-identical default behavior).
- **Files modified:** cmd/server/main.go
- **Verification:** go build ./... exit 0; TestTicketAssemblyWiring still green (asserts the five required envs, does not forbid the sixth)
- **Committed in:** e781bb3

**2. [Rule 1 - Test bugs] Two defects in my own Task 1 test matrix (fixed before GREEN landed)**
- **Found during:** Task 1 GREEN run
- **Issue:** (a) a sanity assertion compared `len(rows)` to 0 instead of 1; (b) the uniform-404 sub-test built Deps without a provider, so the handler's provider-nil 503 (correctly placed before the turn gate) answered 503 instead of reaching the 404 branch.
- **Fix:** (a) assert the snapshot length via the accessor; (b) wire the none provider in that sub-test with a comment explaining WHY (dependency 503s precede the turn gate by design — an unwired deployment must not leak turn existence).
- **Files modified:** internal/server/tickets_test.go (committed as part of 638b54e)
- **Verification:** TestTicketsHandler 7/7 sub-cases green
- **Committed in:** 638b54e

### Advisory (not a deviation — documented for the TDD gate)

Task 2's RED commit (`56b772a`) has two tests that pass by design: TestTicketDispatchNonBlocking and TestTicketsDispatchSourceDiscipline. The goroutine isolation they assert (background ctx, recover) shipped with Task 1's GREEN, because Task 1's own behavior ("handler 响应不等待外发完成") required a safe goroutine and shipping a recover-less goroutine — even for one commit — would be a latent server-killer. The honest RED in that commit is the audit-trail matrix (3 sub-tests fail on zero audit rows) and the assembly-wiring lock (fails on the five missing envs). Mirrors the 29-03 advisory precedent.

---

**Total deviations:** 2 auto-fixed (1 missing-critical wiring, 1 test bug) + 1 advisory note
**Impact on plan:** Both auto-fixes are additive/corrective with no scope creep; the frozen wire contract and the five env knobs are exactly as planned.

## Issues Encountered

- `gofmt` (Go 1.25 in WSL) flags broad pre-existing drift across the repo (e.g. server.go's Detacher/GraphLimits alignment from Phases 19/28) — out of scope, left untouched. Only the three files this plan touched were normalized; joining the Deps composite literal required realigning six pre-existing field lines in cmd/server/main.go (whitespace only).
- `go build ./...` in WSL now exits 0 for cmd/server (the cgo link held), so the assembly is build-verified rather than assumed.

## User Setup Required

None - no external service configuration required. Operators who want outbound dispatch set `SKILLFORGE_TICKET_PROVIDER=webhook` + `SKILLFORGE_TICKET_WEBHOOK_URL` (and optionally `_WEBHOOK_SECRET` / `_WEBHOOK_TIMEOUT_MS` / `_WEBHOOK_RETRIES` / `_DISPATCH_BUDGET_MS`); the default (`none`) is the legal store-only posture.

## Next Phase Readiness

- 29-07 (FE FeedbackRow 接真) can start: the ticket wire contract is frozen — 201 `{ticket_number, dispatch_status:"none"|"queued"}`, 200 `{status:"already_exists", ticket_number}`, 503 `{error: ticket.ProviderNotWiredError}` for the 未配置 provider third state, plus 400/404 envelopes.
- The dispatch trail already surfaces in the existing GET /api/admin/audit (ActorType=admin filter) — no admin endpoint work needed for 29-08.
- Residual: none known for this plan; REQUIREMENTS rows for TKT-02/TKT-04 flip at phase close per convention.

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
