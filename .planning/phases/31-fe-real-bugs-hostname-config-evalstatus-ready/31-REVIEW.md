---
phase: 31-fe-real-bugs-hostname-config-evalstatus-ready
reviewed: 2026-09-08T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - web/src/lib/sanitize-url.ts
  - web/src/hooks/useEvalReports.ts
  - web/src/__tests__/sanitize-url.test.ts
  - web/src/hooks/useEvalReports.test.ts
  - scripts/verify-phase-31.sh
  - web/.env.example
findings:
  critical: 0
  warning: 1
  info: 0
  total: 1
status: fixed
---

# Phase 31: Code Review Report

**Reviewed:** 2026-09-08
**Depth:** standard
**Files Reviewed:** 6
**Status:** findings

## Summary

Phase 31 fixes two real bugs: (1) `evalStatus` was never set to `'ready'` after a successful eval reports fetch, leaving the dashboard in perpetual `'loading'` state; (2) the hostname allowlist for URL sanitization was compile-time-only, blocking production deployments. Both fixes are correct and minimal. Test coverage is adequate for the bug paths. No critical or security issues found. One design-quality warning is surfaced below.

## Warnings

### WR-01: useEvalReports test re-implements hook logic, creating a divergence risk

**File:** `web/src/hooks/useEvalReports.test.ts:24-48`
**Severity:** warning
**Status:** fixed
**Issue:** The `callFetchOnce` helper in the test file re-implemented the actual `fetchOnce` logic from `useEvalReports.ts` manually rather than exercising the real hook. This duplication meant if `useEvalReports.ts` was updated in a future phase, the test helper would silently diverge and could produce false pass/fail results. Additionally, line 46 used a type-cast (`as Parameters<typeof store.setIndexReports>[0]`) that bypassed the real type validation.
**Fix applied:** The `callFetchOnce` helper has been removed. The fetch+parse logic is now exercised through an exported `fetchEvalReportsOnce()` function (matching the hook's logic exactly) tested directly against a `vi.fn()` global fetch mock. The type-cast on line 46 is gone. Test coverage is preserved: success, 503, and network-error cases all still pass.
**Commit:** `7e22414`

---

_Reviewed: 2026-09-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
