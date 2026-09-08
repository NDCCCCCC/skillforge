---
phase: "30-hygiene-milestone-gate"
plan: "03"
subsystem: "ticket-service"
tags: [go, ticket-provider, webhook, http-client, security, httptest, pure-stdlib]

# Dependency graph
requires:
  - phase: "29-feedback-loop-ticket-interface"
    provides: "webhook provider (webhook.go) without CheckRedirect — source of WR-03"
provides:
  - "http.Client with CheckRedirect policy blocking cross-host redirects and redirect loops"
affects: ["30-hygiene-milestone-gate STRIDE threat model (T-30-04)"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CheckRedirect as http.Client security policy — narrow interface preventing secret header leakage and SSRF pivot"
    - "TDD RED/GREEN cycle: failing tests first, then implementation, then full suite regression"

key-files:
  created: []
  modified:
    - "internal/ticket/webhook.go — CheckRedirect field added to http.Client"
    - "internal/ticket/webhook_test.go — TestCheckRedirect with 3 subtests (cross-host / too-many / same-host)"

key-decisions:
  - "CheckRedirect len(via) >= 3 cap: allows 0/1/2 redirects, blocks on the third to match the plan's error message"
  - "Cross-host check uses via[0].URL.Host: first request's host is the authoritative origin"

patterns-established:
  - "http.Client CheckRedirect function: reject on len(via) >= 3 or cross-host, allow same-host"
  - "Redirect test via httptest with handler switching: httptest.NewServer then Config.Handler override for multi-path scenarios"

requirements-completed: [HYG-02]

# Metrics
duration: 8min
completed: 2026-09-08
tasks-completed: 1
files-created: 0
files-modified: 2
tests-added: 3
---

# Phase 30 Plan 03: WR-03 CheckRedirect Policy Summary

**WR-03 fix: webhook http.Client CheckRedirect blocks cross-host redirects and caps redirect depth, preventing X-SkillForge-Ticket-Secret header leakage and SSRF pivot (T-30-04)**

## Performance

- **Duration:** 8 min
- **Started:** 2026-09-08
- **Completed:** 2026-09-08
- **Tasks:** 1 (with 2 commits: RED + GREEN)
- **Files modified:** 2

## Accomplishments
- Added CheckRedirect policy to webhook http.Client in newWebhookProvider
- Policy denies cross-host redirects: `req.URL.Host != via[0].URL.Host` returns error with host in message
- Policy caps redirect chain at 2: `len(via) >= 3` returns error
- Same-host redirects are allowed (Go default behavior preserved)
- TDD cycle: RED commit (failing tests) then GREEN commit (implementation)

## Task Commits

Each task was committed atomically:

1. **Task 1: WR-03 RED — failing TestCheckRedirect** - `6fb983f` (test)
2. **Task 1: WR-03 GREEN — CheckRedirect implementation** - `24fa9a7` (feat)

**Plan metadata:** none (single-plan phase, no separate docs commit)

## Files Created/Modified
- `internal/ticket/webhook.go` — Added CheckRedirect field to http.Client in newWebhookProvider
- `internal/ticket/webhook_test.go` — Added TestCheckRedirect with 3 subtests

## Decisions Made

- **CheckRedirect cap = 2 redirects (len >= 3 blocked):** Plan specified `len(via) >= 3` as the error trigger. On the 3rd redirect `via` has length 3, so `>= 3` correctly blocks. A cap of 2 was chosen to match the plan's error message "too many redirects."
- **Cross-host check via via[0]:** `via[0]` is the original request, so `req.URL.Host != via[0].URL.Host` correctly detects any cross-origin hop from the origin.

## Deviations from Plan

**1. [Rule 3 - Blocking] Test setup: httptest handler reassignment for too_many_redirects**
- **Found during:** GREEN verification
- **Issue:** `httptest.NewServer(handlerA)` then `redirector.Config.Handler = handlerB` did not reliably replace the handler — the old handler continued to serve requests, causing the test to incorrectly pass with nil error.
- **Fix:** Replaced with a single httptest.NewServer using a switch on `r.URL.Path` to chain `/ -> /r1 -> /r2 -> /r3` (3 redirects = 4 requests = len(via)==3 on the 4th, triggering the cap).
- **Files modified:** `internal/ticket/webhook_test.go` (TestCheckRedirect/too_many_redirects_is_rejected subtest)
- **Verification:** All 3 TestCheckRedirect subtests pass; full 17-test suite passes
- **Committed in:** `24fa9a7` (GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 3 blocking — test infrastructure issue)
**Impact on plan:** Fix was a test setup correction, not an implementation change. CheckRedirect logic matched plan exactly.

## Verification

- `go test ./internal/ticket/... -count=1` — 17/17 PASS (6.5s)
- `go build ./internal/ticket/...` — clean
- `go vet ./internal/ticket/...` — clean
- `go test ./internal/ticket/... -run TestCheckRedirect -v` — 3/3 PASS (cross_host_redirect / too_many_redirects / same_host_redirect)

## Threat Model Posture (T-30-04)

| Threat | Disposition | Mitigation |
|--------|-------------|------------|
| T-30-04: webhook redirect secret leak | Mitigated | CheckRedirect denies cross-host redirects; secret header never forwarded to redirect target (verified by httptest asserting zero secret received) |
| T-30-SC: package installs | N/A | No new packages introduced (pure stdlib) |

## Known Stubs

None — this was a pure security fix with no stubs.

## Self-Check: PASSED

- `internal/ticket/webhook.go` exists and contains `CheckRedirect` field
- `internal/ticket/webhook_test.go` contains `TestCheckRedirect` with 3 subtests
- RED commit `6fb983f` and GREEN commit `24fa9a7` both present in git log
- All 17 tests pass (`go test ./internal/ticket/... -count=1`)

---
*Phase: 30-hygiene-milestone-gate / Plan 03*
*Completed: 2026-09-08*
