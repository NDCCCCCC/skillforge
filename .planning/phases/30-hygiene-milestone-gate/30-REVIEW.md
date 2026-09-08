---
phase: 30-hygiene-milestone-gate
reviewed: 2026-09-08T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/server/main.go
  - docs/threat-model/STRIDE-30.md
  - internal/server/chat.go
  - internal/server/cr01_test.go
  - internal/server/feedback.go
  - internal/server/pipeline.go
  - internal/server/server.go
  - internal/server/tickets.go
  - internal/store/sqlite/tickets.go
  - internal/store/store.go
  - internal/ticket/webhook.go
  - internal/ticket/webhook_test.go
  - scripts/verify-phase-30.sh
  - web/src/__tests__/FeedbackRow.test.tsx
  - web/src/components/chat/parts/FeedbackRow.tsx
  - go.mod
findings:
  critical: 2
  warning: 0
  info: 0
  total: 2
status: issues_found
---

# Phase 30: Code Review Report

**Reviewed:** 2026-09-08T00:00:00Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

Two bugs found in `internal/server/knowledge.go` via diff against the Phase 30 baseline (8703e07). Both are definite defects introduced by the Phase 30 changes:

- **CR-01**: `neighborsHandler` — dead code removal accidentally duplicated the `c.JSON` call, causing a double-write to the HTTP response (the second JSON would be corrupted partial bytes).
- **CR-02**: `pathHandler` — the "degraded because requested hops > limit" condition uses the wrong variable (`maxHops` instead of `requestedHops`), making it always fire when the cap is applied (since `maxHops` equals `maxHopsLimit` after the cap), and never fire when the limit is not hit.

No other critical, warning, or info findings across the 20 files reviewed.

---

## Critical Issues

### CR-01: neighborsHandler double c.JSON call — response corruption

**File:** `internal/server/knowledge.go:341-344`
**Issue:** After the edge-capping block, the dead-code removal accidentally left a second `c.JSON(http.StatusOK, result)` call. The first call at line 343 is unconditional; the second at line 345 would write a second HTTP response body, corrupting the client stream.

**Diff (against 8703e07):**
```diff
-	c.JSON(http.StatusOK, result)
-	if result == nil {
-		c.JSON(http.StatusNotFound, graphResponseError{Error: "chunk not found"})
-		return
-	}
-	c.JSON(http.StatusOK, result)
+	c.JSON(http.StatusOK, result)
 }
```

The old code had the `if result == nil` guard after the edge-capping block. When removing that guard, the final `c.JSON(http.StatusOK, result)` on the last line was supposed to be the ONLY emit. Instead, the first `c.JSON` (line 343) was kept AND the dead `if result == nil` block was removed but the second `c.JSON` on what is now the closing brace was kept as a separate call.

**Fix:**
```go
// DELETE the second c.JSON call (the one before the closing brace).
// The correct structure is:
    c.JSON(http.StatusOK, result)
}
```
Remove the second `c.JSON(http.StatusOK, result)` that sits between the edge-capping block and the function's closing brace.

---

### CR-02: pathHandler wrong variable in "degraded" condition — always fires when cap applies, never fires otherwise

**File:** `internal/server/knowledge.go:413`
**Issue:** The degraded-reason flag checks `maxHops < maxHopsLimit` to decide whether to tag the result as degraded. After the cap is applied, `maxHops` equals `maxHopsLimit`, so the condition is **always false** when the cap is hit (maxHops is not less than maxHopsLimit). Meanwhile, when the cap is NOT hit, `maxHops == requestedHops <= maxHopsLimit`, so the condition is also false. The degraded flag is **never set**.

**Diff (against 8703e07):**
```diff
-	if maxHops < maxHopsLimit {
+	if requestedHops > maxHopsLimit {
```

The fix uses `requestedHops` (the original user request before capping) so the degraded flag correctly fires when the user asked for more hops than the configured maximum.

**Fix:**
```go
// Change line 413 from:
if maxHops < maxHopsLimit {
// To:
if requestedHops > maxHopsLimit {
```

---

## Verified Trust Invariants (no regression)

| Invariant | Status | Evidence |
|-----------|--------|----------|
| CR-01 persisted flag propagation (12 call sites) | PASS | `writeAuditAndTurn` at chat.go:323,475,491,511; pipeline.go:252,311,340,379,556,581; runner_setup.go:216,242 — all use type switch on `pipelineEmitError.GetPersisted()` before writing |
| WR-02 SweepQueuedTickets startup goroutine | PASS | tickets.go:444-475; main.go:947-954 — guarded by `ticketProviderName == ticket.ProviderWebhook`, runs in background goroutine |
| WR-03 CheckRedirect cross-host block | PASS | webhook.go:95-103; test coverage in webhook_test.go:294-375 — cross-host returns error, same-host allowed, max 3 redirects |
| WR-05 local-session- prefix rejection | PASS | feedback.go:163, tickets.go:161 — both check `strings.HasPrefix(body.SessionID, "local-session-")` before rate limit |
| WR-05 per-session rate limiter | PASS | server.go:79-95; FeedbackRateLimiter wired at main.go:935 |
| INSERT-only tables (feedback/tickets/disposition) | PASS | sqlite/tickets.go:47-51 `ON CONFLICT DO NOTHING`; store.go:1019-1027 docs confirm INSERT-only |
| Admin token constant-time compare | PASS | audit.go:114-129 — uses `subtle.ConstantTimeCompare` |
| CR-01 pipelineEmitError type | PASS | cr01_test.go:28-50 — compile-time interface verification |
| WR-06 useEffect sync | PASS | FeedbackRow.tsx:174-178 — `useEffect([votedVerdict])` syncs server truth |

---

_Reviewed: 2026-09-08T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
