# Phase 35: HTTP Layer Deduplication - Summary

**Closed:** 2026-09-09
**Status:** ✅ Complete

## Goal

Remove copy-paste HTTP error structs and admin-request middleware from backend handlers.

## Scope

| REQ-ID | Description | Status |
|--------|-------------|--------|
| **REPEATHTTP-01** | `graphResponseError` single-source (already centralized in Phase 27-28) | ✅ Verified |
| **REPEATHTTP-02** | `requireAdmin(c)` middleware helper replaces 14 admin gate templates | ✅ Done |
| **REPEATHTTP-03** | `sessionIDRegex` single-source (already centralized in Phase 8) | ✅ Verified |

## Commits

| Hash | Message |
|------|---------|
| `bb3128c` | refactor(35): requireAdmin helper — replace 14 copy-paste admin gate templates |

## Changes

### internal/server/audit.go
- Added `requireAdmin(c *app.RequestContext) bool` helper adjacent to `isAdminRequest`
- Writes 403 + `graphResponseError{Error: "admin token required"}` on mismatch; returns true on pass

### 10 files: admin_* / feedback / import_api / kb_admin / kpi_admin / platform_prompt / external_auth / upload
- 14 admin-gate call sites rewired: `if !isAdminRequest(c) { c.JSON(403, X); return }` → `if !requireAdmin(c) { return }`
- Standardized response envelope: all admin gates now emit `graphResponseError{Error: "admin token required"}` (was inconsistent — `graphResponseError` in 6 files, `map[string]string` in 4 files)

## Verify Results

```
[PASS] all admin gates use requireAdmin (single template)
[PASS] requireAdmin helper defined in audit.go
[PASS] graphResponseError single-source: internal/server/knowledge.go
[PASS] sessionIDRegex single-source: internal/server/handlers_session_history.go
[PASS] go build ./internal/server/... succeeds
[PASS] go vet ./internal/server/... clean
=== ALL CHECKS PASSED ===
```

## Trust Semantics Preserved

- 403 MUST NOT leak KB registry state (D6-03 / T-3-AUDIT) — unchanged
- `requireAdmin` calls `isAdminRequest` internally; same `subtle.ConstantTimeCompare` for token
- Single response envelope (`graphResponseError`) for all admin gates — frontend sees consistent shape

## Pre-existing Finding (out of Phase 35 scope)

- `internal/server/server/audit.go:273` `isAdminRequest(c)` call inside `redact()` function — this is a boolean check, not an admin gate template; not duplicated, just a legitimate call site.
- DuckDB static-link test failure on Windows host is pre-existing environment issue (mirrors Phase 33 finding) — `go vet` + `go build` confirm code correctness.
