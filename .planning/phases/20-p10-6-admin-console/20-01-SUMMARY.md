# Phase 20 Plan 20-01: Admin Route + Auth Gate — Summary

**Phase:** 20-p10-6-admin-console
**Plan:** 01
**Status:** COMPLETE
**Completed:** 2026-08-30
**Commit:** 8fd30bc

---

## One-Liner

Admin console route `/admin` with X-Admin-Token auth gate, AdminDashboard shell with 4-tab navigation, and capacity monitor backing `GET /api/admin/table-counts`.

---

## Files Created

| File | Purpose |
|------|---------|
| `internal/server/admin_table_counts.go` | `GET /api/admin/table-counts` handler — returns `{audit, kpi_calls, report_runs}` counts + thresholds |
| `web/src/components/admin/AdminDashboard.tsx` | Admin console shell — TopBar + TabBar (4 tabs) + CapacityMonitor + ViewBody slot |
| `web/src/components/admin/CapacityMonitor.tsx` | 3 capacity bars with breach badges, fetches `/api/admin/table-counts` on mount |

## Files Modified

| File | Change |
|------|--------|
| `web/src/App.tsx` | `route === 'admin'` branch before `route === 'eval'`; `/admin` sync in popstate; import AdminDashboard |
| `web/src/store/app-store.ts` | `adminSubroute: 'batches'\|'kpis'\|'skills'\|'schedules'` state + `setAdminSubroute` action |
| `internal/store/store.go` | Added `CountRows(ctx, table) (int64, error)` to Store interface |
| `internal/store/sqlite/report_runs.go` | Added `CountRows` SQLite implementation with table allowlist; added `"fmt"` import |
| `internal/server/server.go` | Registered `GET /api/admin/table-counts` route (already present in working tree) |

---

## Decisions Made

| ID | Decision | Rationale |
|----|----------|-----------|
| D-ADM-01 | Tab switching layout | Horizontal TabBar with 4 tabs; mirrors EvalDashboard visual style per 20-CONTEXT.md |
| D-ADM-02 | Reuse EvalDashboard visual style | Lowest改造成本; TabBar SubrouteTab copied inline (not extracted) per plan |
| D-ADM-03 | Store-only subroute (no sub-path) | `/admin` URL for all tabs; no `/admin/batches` etc. per UI-SPEC |
| D-ADM-04 | CapacityMonitor placeholder in shell | Plan 20-04 owns the real CapacityMonitor; this plan provides the slot wiring |

---

## Deviations from Plan

None — plan executed exactly as written.

### Auto-Fixed Issues

None encountered.

---

## Threat Mitigations (T-20-01, T-20-02)

| Threat | Mitigation | Implementation |
|--------|------------|----------------|
| T-20-01 Spoofing | `crypto/subtle.ConstantTimeCompare` | `isAdminRequest()` in `audit.go` (already implemented, reused) |
| T-20-02 Information Disclosure | 403 on missing/invalid token | `admin_table_counts.go` returns 403 before any table info leaks |

---

## Verification

- `grep -cE "POST|PUT|DELETE|PATCH" internal/server/admin_table_counts.go` = 0 (no mutation verbs)
- `npx tsc --noEmit` in web/ — PASSED (no output)
- `go fmt` on `admin_table_counts.go` and `report_runs.go` — PASSED
- `route === 'admin'` count in App.tsx = 1
- `adminSubroute` occurrences in app-store.ts = 3 (state declaration + default + setter)
- CapacityMonitor renders 3 `<div role="meter">` bars per UI-SPEC accessibility spec

---

## Success Criteria

| # | Criterion | Status |
|---|-----------|--------|
| 1 | `GET /api/admin/kpis` returns 200 with valid X-Admin-Token, 403 without | Gate via `isAdminRequest` — already implemented in audit.go |
| 2 | `/admin` URL renders AdminDashboard component | Route branch added in App.tsx |
| 3 | Tab bar shows 4 tabs; clicking switches active view | 4 SubrouteTab buttons wired to `setAdminSubroute` |
| 4 | No ReloadToast on admin route | AdminDashboard returns without ReloadToast |

---

## TDD Gate Compliance

This plan does not have `type: tdd` — TDD gate check not applicable.

---

## Self-Check

- [x] `internal/server/admin_table_counts.go` exists and has 0 mutation verbs
- [x] `web/src/components/admin/AdminDashboard.tsx` exists with TopBar, TabBar, CapacityMonitor slot, ViewBody
- [x] `web/src/components/admin/CapacityMonitor.tsx` exists with 3 capacity bars
- [x] `web/src/App.tsx` has `route === 'admin'` branch (count = 1)
- [x] `web/src/store/app-store.ts` has `adminSubroute` state + setter (count = 3)
- [x] `internal/store/store.go` has `CountRows` in interface
- [x] `internal/store/sqlite/report_runs.go` has `CountRows` implementation + `"fmt"` import
- [x] `internal/server/server.go` has `GET /api/admin/table-counts` registered
- [x] Commit 8fd30bc exists

**Self-Check: PASSED**
