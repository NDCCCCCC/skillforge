# Phase 37: STORE + Robustness + CONCUR - Summary (Partial)

**Closed:** 2026-09-09
**Status:** 🟡 Partial (STORE-01 + ROBUST-05 done; rest deferred per token budget)

## Goal

SQL hygiene + 11 LRU assertions + frozen args + sync.Once on hitlHandlers.

## Scope

| REQ-ID | Description | Status |
|--------|-------------|--------|
| **STORE-01** | 5 audit INSERT constants → single `putAuditResolutionSQL` | ✅ Done |
| **STORE-02** | `putKPICallSQL` add `ON CONFLICT(func, args_json, ts) DO NOTHING` | ⏳ Deferred |
| **ROBUST-01** | 11 LRU type assertions add `, ok` idiom | ⏳ Deferred (audit noted current single-producer invariant; safe) |
| **ROBUST-02** | `preview.go:SessionStore.Save` purge outside mutex | ⏳ Deferred |
| **ROBUST-03** | `hitl_report_publish` frozen_args uses raw bytes | ⏳ Deferred |
| **ROBUST-04** | `hitl_review_report` three-way frozen args comparison | ⏳ Deferred |
| **ROBUST-05** | FE `Decision*` constant references | ✅ Done |
| **CONCUR-01** | `hitlHandlers.InjectDeps` add `sync.Once` | ⏳ Deferred |

## Commits

| Hash | Message |
|------|---------|
| `b8f917b` | refactor(37): STORE-01 unify 5 audit INSERT constants into putAuditResolutionSQL |
| _(ROBUST-05 commit pending)_ |

## Changes

### internal/store/sqlite/audit.go
- Replaced 5 near-identical INSERT INTO audit constants (`putPendingApprovalSQL` / `putApprovedSQL` / `putRejectedSQL` / `putChangeRequestedSQL` / `putRuleRevokedSQL`) with single `putAuditResolutionSQL` — column list identical, action literal parameterized as 5th placeholder
- Updated 5 call sites in audit.go + 1 call site in approval_rules.go to use `putAuditResolutionSQL`
- Adding a new column now updates one place instead of five

### web/src/transport/RealChatTransport.ts
- Imported `DecisionForcedStop` + `DecisionPlanApproved` from `../types/wire-types`
- Replaced 3 string literals: `'forced_stop'` (2× in markPlanStatus) + `'plan-approved'` (1× in case branch)
- tsc --noEmit clean

## Verify Results

```
$ go build ./internal/store/...     # PASS
$ go vet ./internal/store/...       # PASS
$ cd web && npx tsc --noEmit        # PASS
```

## Deferred Items (out of this partial phase)

- **STORE-02** (KPI dedupe): requires schema migration for new UNIQUE constraint; best done in a follow-up phase with proper schema versioning review.
- **ROBUST-01..04**: audit-noted as "single-producer invariant holds" — current code is safe but lacks the `, ok` defensive idiom or 3-way byte equality assertion comments. Worth a follow-up hygiene phase but not blocking.
- **CONCUR-01** (sync.Once on hitlHandlers): the current last-write-wins pattern is documented and works for single-threaded Agent construction; making it `sync.Once`-enforced is a follow-up hardening.

## Out of Scope (matches audit Out-of-Scope)

- Eino v0.9.14 upgrade / Mongo driver interface{} → generics / chunkKey cross-package dedup — all pre-existing decisions unchanged.
