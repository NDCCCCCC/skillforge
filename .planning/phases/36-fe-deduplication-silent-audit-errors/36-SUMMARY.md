# Phase 36: FE Deduplication + Silent Audit Errors - Summary

**Closed:** 2026-09-09
**Status:** ✅ Complete

## Goal

Extract shared frontend fetch/error helpers and fix silent audit write failures in backend.

## Scope

| REQ-ID | Description | Status |
|--------|-------------|--------|
| **SILENTAUDIT-01** | 3 audit sites `_ = audit.PutAudit(...)` → WARN log on err | ✅ Done |
| **REPEATFE-01** | `apiFetch<T>(url, opts)` shared helper | ✅ Done (additive) |
| **REPEATFE-02** | `extractReason(err)` shared helper | ✅ Done |
| **REPEATFE-03** | `useCallback` + `fetchImpl` injection in useEvalReports | ✅ Phase 31 fix already covered |

## Commits

| Hash | Message |
|------|---------|
| `a645484` | fix(36): SILENTAUDIT-01 — surface audit write failures at WARN log |
| `ebc97b9` | feat(36): REPEATFE-01..02 apiFetch helper + extractReason — shared FE HTTP layer |

## Changes

### internal/agent/fanout_tools.go
- Imports: +`log`
- Line 363: `_ = t.audit.PutAudit(...)` → `if err := ...; log.Printf("[WARN] agent: fan-out audit write failed (action=%s skillID=%s): %v", ...)`

### internal/runtime/orchestrator.go
- Imports: +`log`
- `emitFailure` (line 816): same pattern with `[WARN] runtime: orchestrator audit write failed (stage=%s runID=%s)`
- `writeAudit` (line 826): same pattern with `[WARN] runtime: orchestrator audit write failed (action=%s runPath=%s)`

### web/src/lib/api.ts (new)
- `apiFetch<T>(url, opts)` — unified GET/POST helper with JSON encoding, Accept/Content-Type defaults, structured `ApiError`, `fetchImpl` injection seam
- `extractReason(err)` — UI error message extraction helper

### Phase 31 fix already covered REPEATFE-03
- `useEvalReports.ts` `fetchEvalReportsOnce` exported and uses `vi.fn()` injection in tests (no `vi.spyOn(global.fetch)` reliance)

## Verify Results

```
[PASS] no silent audit write patterns
[PASS] 3 audit write failure WARN log sites
[PASS] apiFetch helper exported from web/src/lib/api.ts
[PASS] extractReason helper exported from web/src/lib/api.ts
[PASS] go build clean
[PASS] tsc --noEmit clean
=== ALL CHECKS PASSED ===
```

## Trust Semantics Preserved

- SILENTAUDIT: fail-open preserved (audit hiccup must not abort Run); WARN log added for production observability
- REPEATFE: helper is additive — no breaking changes to existing callers (FeedbackRow / useEvalReports / useEvalFrozen can migrate incrementally)

## Out of Scope (additive pattern)

- Migrating existing 4 fetch call sites to apiFetch — preserves Phase 31 fixed test contracts; subsequent phases can migrate incrementally without breaking existing tests
