# Phase 20: Code Review Report

**Reviewed:** 2026-08-30T00:00:00Z
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

Phase 20 implements an admin console at `/admin` with four read-only views (Import Batches, KPI Definitions, Skill Management, Scheduled Tasks) and a capacity monitor, gated by `X-Admin-Token` header auth. The implementation is well-structured: all Go handlers are retrieval-only with no mutation verbs, SQL injection is prevented by parameterized placeholders and table allowlisting, `crypto/subtle.ConstantTimeCompare` is used for token comparison, and all admin handlers follow the same nil-store 503 pattern. One minor quality issue was found.

## Critical Issues

None found.

## Warnings

### WR-01: loadLatestEval silently returns nil on error, hiding partial eval data

**File:** `internal/server/admin_skills.go:163-172`
**Issue:** When `os.Open(indexPath)` fails (file not found, permissions) or `json.NewDecoder` fails (malformed JSON), the function returns `nil` silently. Combined with the fact that `SKILLFORGE_EVAL_REPORTS_DIR` is read at request time (not startup), a misconfigured path change or corrupted INDEX.json would produce no eval summaries for any skill — with no error logged or surfaced to the operator. The error is only visible in server logs if the handler itself logs it (it does not).
**Fix:** Log the error so operators can diagnose missing eval summaries:
```go
func loadLatestEval(indexPath string) map[string]*evalSummary {
    file, err := os.Open(indexPath)
    if err != nil {
        // Log at debug/info level — index not yet created is not an error.
        return nil
    }
    defer file.Close()

    var index evalIndex
    if err := json.NewDecoder(file).Decode(&index); err != nil {
        // This is a real problem — malformed INDEX.json means eval data is
        // present but unparseable. Log it.
        log.Printf("[WARN] loadLatestEval: decode %s: %v", indexPath, err)
        return nil
    }
    // ... rest of function
}
```

## Info

### IN-01: handleRefresh in AdminDashboard is a no-op

**File:** `web/src/components/admin/AdminDashboard.tsx:38-41`
**Issue:** `handleRefresh` is wired to the Refresh button in the top bar but contains only a comment saying it is a no-op ("wired for future"). Clicking the button does nothing. This is acknowledged in the code but may mislead the operator.
**Fix:** Either implement a refresh mechanism (e.g., a refresh key incremented in the Zustand store that child components subscribe to via `useEffect`) or remove the Refresh button entirely until it is wired.

### IN-02: adminSkillsHandler reads SKILLFORGE_EVAL_REPORTS_DIR at request time

**File:** `internal/server/admin_skills.go:144-147`
**Issue:** The eval reports directory is read from the environment variable on every `GET /api/admin/skills` request, not at server startup. If the operator changes `SKILLFORGE_EVAL_REPORTS_DIR` at runtime, the handler picks up the new value without a restart. This is described in the godoc as intentional (hot-reload), but combined with the silent nil return on error (WR-01), a misconfigured path change would produce no visible indication of why eval summaries are absent.
**Fix:** No fix required — this is documented intentional behavior. Operators should monitor server logs if eval summaries disappear unexpectedly.

---

_Reviewed: 2026-08-30T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
