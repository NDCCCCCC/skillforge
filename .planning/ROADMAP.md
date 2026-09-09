# SkillForge Roadmap

## Milestones

| Milestone | Phase Range | Status |
|-----------|-------------|--------|
| v0.7.7 (Vertical Slice) | 1-7 | ✅ SHIPPED 2026-08-11 |
| v0.8 (Memory + HITL) | 8-13 | ✅ SHIPPED 2026-09-06 |
| v1.0 (Business Data) | 14-20 | ✅ SHIPPED 2026-09-06 |
| v1.1 (Eval Trust-Loop) | 21-25 | ✅ SHIPPED 2026-09-06 |
| v1.2 (Knowledge Ops) | 26-30 | ✅ SHIPPED 2026-09-08 |
| **v1.3 (Code Quality)** | **31-37** | **🚧 Planned** |
| v1.4 (OSS Release) | Future | Planned |
| Backlog | 999.x | Backlog |

---

## Phase 31: FE Real Bugs — Hostname Config + evalStatus Ready

**Goal**: Fix two production-impacting frontend bugs — hardcoded hostname whitelist and stuck loading state

**Depends on**: Nothing (first v1.3 phase)

**Requirements**: REALBUG-01, REALBUG-02

**Success Criteria** (what must be TRUE):

1. `sanitize-url.ts` reads hostname whitelist from a runtime-configurable source (env or admin API), not hardcoded list; production deployment domains work without code changes
2. `useEvalReports.ts` calls `setEvalStatus('ready')` on successful fetch; `EvalDashboard` transitions through loading/error/ready states correctly
3. Existing vitest suite for `sanitize-url.ts` passes with the new configurable whitelist
4. `vi.spy` regression test for `evalStatus` state transitions is green

**Plans**: TBD

---

## Phase 32: BE Real Bug + Performance — Typed Sentinel + O(n²) Fix

**Goal**: Fix backend production bug (string-substring error matching) and a performance regression (O(n²) string concat in hot path)

**Depends on**: Phase 31

**Requirements**: REALBUG-03, PERF-01

**Success Criteria** (what must be TRUE):

1. `upload.go:isBackendUnavailable` uses `errors.Is(err, syscall.ECONNREFUSED)`, `errors.Is(err, context.DeadlineExceeded)`, `errors.Is(err, context.Canceled)`, and `net.Error.Timeout()` — no `strings.Contains` substring matching on error messages
2. `chat.go:joinStrings` is replaced with `strings.Join(parts, "")`; each LLM response drain path calls the fixed version
3. Unit tests cover all branches of `isBackendUnavailable` (connection refused, deadline exceeded, canceled, timeout, other error)
4. Go vet/benchmark regression confirms no performance degradation on joinStrings path

**Plans**: TBD

---

## Phase 33: SRP — BE Long Function Split

**Goal**: Split four oversized Go handlers to meet SRP constraints (handler ≤ 200 lines, pipeline ≤ 100 lines)

**Depends on**: Phase 32

**Requirements**: SRP-01, SRP-02, SRP-03, SRP-04

**Success Criteria** (what must be TRUE):

1. `pipeline.go:VerdictPipeline` main function ≤ 100 lines; `handleRefusal()`, `handleVerdict()`, `streamWithRetry()` extracted as named methods; Step 0..12 comments preserved
2. `chat.go:chatHandler` main function ≤ 80 lines; `runDataChannelPath`, `runAgentPath`, `runRegistryRouterPath`, `runVerdictPipelinePath` extracted; 9 DEBUG log blocks merged into named helpers
3. `upload.go:uploadHandler` main function ≤ 80 lines; `onFile` closure split into `verifyMagic`, `dedupCheck`, `writeIndex`, `auditHook` named functions
4. `approval_api.go:approvalPostHandler` main function ≤ 100 lines; `decodeAndValidate`, `lookupPending`, `auditResolution`, `runBuildRunner`, `drainSSE` extracted; self-heal retry block is a named submethod
5. All four files compile with `go vet` zero warnings; no new cyclomatic complexity introduced

**Plans**: TBD

---

## Phase 34: HITL Handler Template Extraction

**Goal**: Eliminate copy-paste duplication across four HITL handlers by extracting shared base class and audit builder

**Depends on**: Phase 33

**Requirements**: HITL-01, HITL-02, HITL-03

**Success Criteria** (what must be TRUE):

1. New `baseHITLTool` struct is embedded in all four HITL handlers (`memory_facts`, `review_report`, `plan_approve`, `report_publish`); `auditOrDefault()` lives in the base class
2. `buildPendingAuditAndInterrupt(ctx, info, args, audit)` is a shared function callable by all four handlers; the 4-step sequence (GetToolCallID → ApprovalInfo → InsertPendingApproval → StatefulInterrupt) is no longer duplicated
3. `noopAuditWriter` is defined in `hitl/audit_writer.go` and imported by `hitl_memory_facts.go`; no duplicate `noopAuditWriter` methods remain
4. All four handlers continue to pass existing unit tests after refactoring

**Plans**: TBD

---

## Phase 35: HTTP Layer Deduplication

**Goal**: Remove copy-paste HTTP error structs and admin-request middleware from backend handlers

**Depends on**: Phase 34

**Requirements**: REPEATHTTP-01, REPEATHTTP-02, REPEATHTTP-03

**Success Criteria** (what must be TRUE):

1. `graphResponseError` struct is defined once in `internal/server/server_response.go`; all six+ duplicated definitions (`knowledge.go`, `feedback.go`, `upload.go`, `tickets.go`, `approval_api.go`, `plan_api.go`) are replaced with imports
2. Admin-gated handlers use a shared `requireAdmin(c) error` middleware function; the 403 response + audit-write logic appears in exactly one place
3. `sessionIDRegex` is exported from `server.go` and imported by all eight+ files that currently define it locally; `handlers_session_history.go` no longer re-exports a private copy
4. All affected routes continue to return identical JSON error shapes after refactoring

**Plans**: TBD

---

## Phase 36: FE Deduplication + Silent Audit Errors

**Goal**: Extract shared frontend fetch/error helpers and fix silent audit write failures in backend

**Depends on**: Phase 35

**Requirements**: REPEATFE-01, REPEATFE-02, REPEATFE-03, SILENTAUDIT-01

**Success Criteria** (what must be TRUE):

1. `lib/api.ts` exports `apiFetch<T>(url, opts)` used by `FeedbackRow.postJson`, `useEvalReports.fetchOnce`, `useEvalFrozen.refetch`, and other fetch sites — no independent fetch+JSON+error implementations remain
2. `lib/errors.ts` exports `extractReason` used by both `FeedbackRow` and `useEvalReports` — error text extraction logic appears in exactly one file
3. `useEvalReports.fetchOnce` is wrapped in `useCallback` and accepts `fetchImpl` as an injected parameter; tests use `vi.fn()` injection instead of global fetch spy
4. Four backend audit write sites that silently swallow errors with `_ = ...` now log at WARN level with session/audit_id/context; the "audit hiccup must not fail Run" invariant is preserved (log does not throw)

**Plans**: TBD

---

## Phase 37: Store SQL Hygiene + Robustness + Concurrency

**Goal**: Fix SQL parameterization, add missing constraints, strengthen type-safety invariants, and add concurrency guarantees

**Depends on**: Phase 36

**Requirements**: STORE-01, STORE-02, ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-05, CONCUR-01

**Success Criteria** (what must be TRUE):

1. All five `INSERT INTO audit` statements share a single parameterized SQL statement with `action=?` placeholder; adding a new audit action column requires changes in exactly one place
2. `putKPICallSQL` includes `ON CONFLICT(func, args_json, ts) DO NOTHING`; re-calling the same KPI with identical args is idempotent
3. All 11 LRU type assertions (`data/preview.go:121,141,208`, `duckdb/attach_lru.go:124`, `knowledge/graph_cache.go:122,150`, `runtime/fanout_runtime.go:266`) use the `, ok` idiom or have explicit comments documenting why the invariant holds
4. `SessionStore.Save` purge logic runs outside the mutex critical section (async or TTL-heap)
5. `hitl_report_publish.go` frozen_args uses raw `argsBytes` string, not `json.Marshal(parsed)` — sha256 hash of frozen_args matches LLM's original input byte-for-byte
6. `hitl_review_report.go` performs three-way frozen args comparison (preview vs. audit row vs. executed) or documents why two-way is sufficient
7. `RealChatTransport.ts` uses `DecisionForcedStop` and other `Decision*` constants from `wire-types.ts` instead of string literals
8. `hitlHandlers` registry uses `sync.Once` during `InjectDeps` to guarantee single initialization; documentation states the process-wide initialization invariant

**Plans**: TBD

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 31. FE Real Bugs | 0/2 | Not started | — |
| 32. BE Real Bug + PERF | 0/2 | Not started | — |
| 33. SRP Split | 0/4 | Not started | — |
| 34. HITL Template | 0/3 | Not started | — |
| 35. HTTP Deduplication | 0/3 | Not started | — |
| 36. FE Deduplication + SILENTAUDIT | 0/4 | Not started | — |
| 37. STORE + Robustness + CONCUR | 0/8 | Not started | — |

---

*Last updated: 2026-09-09 — v1.3 roadmap created (7 phases 31-37, 26 requirements mapped)*
