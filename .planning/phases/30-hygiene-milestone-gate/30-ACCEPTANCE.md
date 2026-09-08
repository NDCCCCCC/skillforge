# Phase 30: Acceptance Evidence

**Phase:** 30-hygiene-milestone-gate
**Plan:** 30-08
**Status:** acceptance_pending
**HYG-03 owner:** D-30-05 / D-30-06

---

## HYG-03 Evidence Compilation (phases 26-29)

Evidence compiled from per-phase VERIFICATION.md reports and verify-phase-*.sh outputs.

| Phase | Unit Tests | Hermetic | WSL Build | Verify Gate | FE vitest/tsc |
|-------|------------|----------|-----------|-------------|---------------|
| 26    | `go test ./internal/kb/... ./internal/skill/...` ok; `go test ./internal/{agent,server,knowledge,store,runtime,data,eval}/...` 12 packages ok (WSL) | `TestPlatformFloorGoldenSchema/FrozenSet/LockSelfConsistent` green; `TestCitableRoleIsAuthority` green; `TestKBAdminCheckOrder` green; `TestRestartDeactivateStaysDeactivated` green | `go build ./...` exit 0 (WSL) | `bash scripts/verify-phase-26.sh` 3/3 stages PASS | vitest 67 files / 497 tests PASS; `tsc --noEmit` exit 0 |
| 27    | `go test ./internal/server/...` 17 TestUpload* PASS (12 from 27-03 + 5 degraded from 27-07) | `TestMagicBytes` / `TestUploadParserRegistry` / `TestMilvusRecreateGateWiring` / `TestSeedLoad` green | `go build ./...` exit 0 (WSL) | `bash scripts/verify-phase-27.sh` 3/3 PASS | vitest 12/12 PASS; `tsc --noEmit` exit 0 |
| 28    | `go test ./internal/knowledge/...` graph_edges_test.go + graph_cache_test.go + knowledge_test.go NeighborsAtDepth/DepthLimit/TestGraphLimits green | `TestNeighborsAtDepth` / `TestDepthLimit` / `TestGraphLimits` / `TestFeedbackNoTrustMutation` green | `go build ./...` exit 0 (WSL) | `bash scripts/verify-phase-28.sh` 3/3 PASS | vitest useGraphNeighbors + NeighborExpandButton + InspectorRetrieval 535 tests PASS; `tsc --noEmit` exit 0 |
| 29    | `go test ./internal/store/... ./internal/server/... ./internal/ticket/... ./internal/wire/...` 13+ packages green (WSL) | `TestFeedbackInsertOnly` / `TestTicketsInsertOnly` / `TestPendingReview` / `TestFeedbackMetrics` / `TestFeedbackNoTrustMutation` / `TestTicketDispatchNonBlocking` / `TestTicketDispatchAuditRows` / `DataTurn*` 10/10 PASS | `go build ./...` exit 0 (WSL) | `bash scripts/verify-phase-29.sh` 4/4 PASS (Stage 1 TODO clear, 2 FBK-05 locks, 3 D15 pairing, 4 DDL discipline) | vitest 74 files / 595 tests PASS; `tsc --noEmit` exit 0 |

**HYG-03 status:** Hermetic layer complete. Live layer (`cmd/eval --frozen --gate-check`) is operator-driven — see Trust Semantics Regression section.

---

## CR-01 + WR-01..06 Regression Locks

Compiled from 29-REVIEW.md findings and Phase 30 fix commits.

| ID | Fix | Test | Status | Evidence |
|----|-----|------|--------|----------|
| CR-01 | SSE emit-failure double-write: pipeline returns `*pipelineEmitError{persisted}` flag; chatHandler error branch skips write when `persisted=true` | `TestCR01Fix` (cr01_test.go) | **COMMITTED** (30-01 Task 1, commit e12fd77) | `pipelineEmitError` carries `persisted bool`; error branch gates `writeAuditAndTurn` on `!persisted`; GET/POST routes unchanged; `cr01_test.go` locks interface contract |
| WR-01 | ON CONFLICT DO NOTHING backfill: replay path calls `PutFeedbackChunkRefs` with `ON CONFLICT(feedback_id, chunk_id) DO NOTHING` so failed refs write is idempotently backfilled on retry | `TestFeedbackChunkRefsBackfill` (implicit in `TestFeedbackHandler` replay branch) | **COMMITTED** (30-02 Task 1, commit 588a5ab) | `feedback.go` replay branch backfills refs; `feedback.go` INSERT uses `ON CONFLICT DO NOTHING`; zero UPDATE/DELETE in store layer |
| WR-02 | Startup sweep: server boot scans `tickets` for `dispatch_status='queued'` rows and re-dispatches through provider (best-effort, idempotent via TKT-{id} number) | `TestQueuedTicketSweep` | **COMMITTED** (30-02 Task 2) | Boot-time sweep goroutine; `dispatchForTicket` idempotent-shaped; INSERT-only `action_ticket_dispatch_*` audit rows |
| WR-03 | CheckRedirect: webhook client refuses cross-host redirects (prevents X-SkillForge-Ticket-Secret leakage on redirect) | `TestCheckRedirect` | **COMMITTED** (30-03 Task 1) | `CheckRedirect` func returns error if `len(via) >= 3` or redirect host differs from first request host; test mocks 302 to cross-origin |
| WR-04 | Action filter + tie-break: `feedbackAttribution` filters to `action=''` rows with `ORDER BY ts DESC, id DESC LIMIT 1` (WR-04 fix) | `TestAttribution` | **COMMITTED** (30-04 Task 1) | Attribution query adds `AND action = ''` and `id DESC` tie-break; `QueryAudit` extended with `ActionEmpty` filter |
| WR-05 | Rate limit + ownership: per-session token-bucket rate limit on `POST /api/feedback` + `POST /api/tickets`; session ID prefix `local-session-` rejected server-side | `TestRateLimit` | **COMMITTED** (30-05 Task 1) | `golang.org/x/time/rate` limiter per session; `local-session-` prefix rejected at header read; in-process, single-binary appropriate |
| WR-06 | useEffect for votedVerdict: FeedbackRow uses `useEffect` to adopt late-arriving `votedVerdict` prop updates | `FeedbackRow.test.tsx` (existing suite) | **COMMITTED** (30-06 Task 1) | `useEffect([votedVerdict])` sets local lock; server truth wins on late arrival; optimistic `setLocked` preserved post-submit |

---

## UAT Clearance (D-30-07)

Three UAT runbooks cover the phases 27-29 live acceptance. All are operator-driven.

| Runbook | Phase | Status | Notes |
|---------|-------|--------|-------|
| 27-UAT.md | Phase 27 | **pending** | 7 steps (upload + audit + 403 + 409 + FE upload + search); operator signs off |
| 28-UAT.md | Phase 28 | **pending** | 18 steps across 4 sections (graph API + joint acceptance + degraded mode + cache invalidation); operator signs off |
| 29-UAT.md | Phase 29 | **pending** | 9 tests (1-8 live/browser/webhook; 9 hermetic gate already PASS via verify-phase-29.sh) |
| 29-HUMAN-UAT.md | Phase 29 | **pending** | 9 items all pending; operator signs off after all 29-UAT items complete |

**Acceptance gate:** All four rows must show `PASS` with operator sign-off before HYG-03 live layer is considered complete.

---

## Trust Semantics Regression

### Hermetic Layer (D-30-06a)

Structural gates verified by existing test suites and verify-phase-*.sh:

| Invariant | Gate | Evidence |
|-----------|------|----------|
| KNOW-02 fail-closed | `TestCitableRoleIsAuthority` + `TestPipelineNilRegistryGraceful` | `kb.Citable` nil-registry returns true; `ResolveTiers` nil → error |
| D-09 refusal construction | `TestRefusalOnEmptyRetrieval` | Empty tier set produces `RefuseNotEnoughContext`; zero fabricated citations |
| Citation whitelist | `TestCitationFromRetrievalOnly` | `citableChunksByID` gated by `kb.Citable`; history/摘要/resume text cannot enter citation path |
| Frozen golden set self-consistency | `TestFrozenSetLockSelfConsistent` | `eval/frozen/*.lock.json` sha256 matches; `eval/golden/*.jsonl` unchanged since lock |
| **HYG-02 config-driven** | `verify-phase-30.sh` Stage 02 | All 14 SKILLFORGE_* fields wired via `envOr`/`parseIntEnvOrDefault`/`parseBytesEnvOrDefault` in `cmd/server/main.go` |
| **HYG-02 diff scan** | `verify-phase-30.sh` Stage 03 | Phases 26-29 diff Go files contain no hardcoded config literals |
| **HYG-02 TODO zero** | `verify-phase-30.sh` Stage 01 | Phase 30 production Go + TS files have zero `TODO(`/TODO:`/`FIXME(`/`FIXME:` |

### Live Layer (D-30-06b)

Operator-driven. See `29-HUMAN-UAT.md` for execution steps.

| Gate | Command | Threshold |
|------|---------|-----------|
| `cmd/eval --frozen --gate-check` | `go run ./cmd/eval --frozen --gate-check` | faithfulness >= 0.9, refusal_accuracy >= 0.95 |
| Pre-flight probe | Probe Milvus, BGE-M3, and GLM endpoints before running | Any unreachable service: STOP and record env issue |
| STOP-on-disconnect discipline | Per `env-intranet-service-flakiness` memory | If service disconnects mid-run: record env issue, do NOT fake scores |

**Discipline note:** Live eval results reflect actual environment conditions. A mid-run BGE failure that degrades faithfulness to, e.g., 0.83 does NOT get re-run or re-signed — the result stands as recorded. If the run was blocked by a pre-flight failure, record `env:STOP` with the specific service that was unreachable.

---

*Document created: 2026-09-08 by Phase 30 Plan 30-08 executor*
*HYG-03 live layer pending operator execution of UAT runbooks and `cmd/eval --frozen --gate-check`*
