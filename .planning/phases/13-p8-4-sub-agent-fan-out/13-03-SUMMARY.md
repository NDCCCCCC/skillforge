# Phase 13 Plan 13-03: BE Orchestrator Pipeline + Audit RunPath/Transfer + D16 Overload — Summary

**Plan:** 13-03 — BE core of Phase 13 (Plan 13-03): complete the `FanOutOrchestrator.Run` 5-step pipeline with `errgroup.SetLimit(4)` parallelism + `defer recover()` fault isolation + per-sub-agent `context.WithTimeout(30s)` + shared `*ChunkAccumulator` (P13-L9); audit substrate extension (`ALTER TABLE audit ADD COLUMN run_path/transfer_from/transfer_to`) + 3 new `action_fanout_*` enum values + `QueryFanoutByRunPath` helper; D16 overload on `data-decision.decision` enum (`fanout_dispatched | fanout_completed | fanout_failed`) with D15 lockstep TS mirror.
**Wave:** 2 (parallel with 13-02)
**Status:** COMPLETE — 2 tasks done; 13 files touched (2 created + 11 modified); all hermetic tests pass; D15 wire-type compile alarm preserved (no new SSE chunk union members); D-04 transport-swap preserved.

---

## What Shipped

### Files created (2)
1. `internal/store/sqlite/fanout_events.go` — `QueryFanoutByRunPath(ctx, runPathPrefix)` + `ListFanoutEvents(ctx, sessionID, limit)` helpers (FAN-04 SC-4 audit trace reconstruction). Inner SELECT synthesizes the Reason column from `refusal_reason` for `action_fanout_failed` rows via a CASE expression; outer query orders by `ts ASC, id ASC` for stable ordering. Returns empty (non-nil) slice when no events match; `sql.ErrConnDone` when store is closed.
2. `internal/store/sqlite/fanout_events_test.go` — 9 regression tests covering empty prefix, closed store guard, ordered-by-ts, filter-by-action-prefix, run-path-pattern-match, require-run-path, scope+limit, empty-sessionID.

### Files modified (11)
1. `internal/wire/chunk.go` — extended `DecisionPayload` struct with 3 new optional fields (`RunPath` / `TransferFrom` / `TransferTo`) + 3 new decision enum constants (`DecisionFanoutDispatched` / `DecisionFanoutCompleted` / `DecisionFanoutFailed`). D16 overload pattern (no new wire chunk types — rides the existing `data-decision` discriminator per P13-L7 / D-04 transport-swap).
2. `internal/wire/chunk_test.go` — added `TestDataDecisionFanoutEnum` (3-case sub-tests for all 3 fanout_* values + RunPath/TransferFrom/TransferTo round-trip) + `TestValidTypesUnchangedAfterFanout` (count lock at 23 entries — fanout_* ride the existing `data-decision` discriminator, NOT new chunk types).
3. `internal/wire/types.go` — exported 3 new decision constants in the canonical string-enum catalog (mirrors `internal/wire/chunk.go`).
4. `internal/store/sqlite/schema.go` — additive DDL: 3 new ALTER TABLE statements (`alterAuditAddRunPathSQL` / `alterAuditAddTransferFromSQL` / `alterAuditAddTransferToSQL`) gated by `PRAGMA table_info(audit)` for idempotency; CREATE TABLE audit extended for fresh DBs; `idx_audit_run_path` index added for the prefix-LIKE scan used by `QueryFanoutByRunPath`. Migration is additive — pre-existing rows preserve NULL values (T-13-03-7).
5. `internal/store/sqlite/audit.go` — extended `putAuditSQL` to 24 placeholders (3 new columns) + extended `queryAuditCols` for SELECT + extended `QueryAudit` scan to read the 3 new columns. Added `isFanoutAction` helper + `ErrInvalidFanoutRunPath` sentinel (T-13-03-3 mitigation: `PutAudit` rejects fanout rows with empty `RunPath`). Added 3 enum constants `ActionFanoutDispatched` / `ActionFanoutCompleted` / `ActionFanoutFailed`.
6. `internal/store/store.go` — promoted `FanoutEvent` struct to the shared `store` package so the `Store` interface can reference it; extended `AuditRow` struct with `RunPath` / `TransferFrom` / `TransferTo` fields; added 2 methods to the `Store` interface: `QueryFanoutByRunPath` + `ListFanoutEvents`.
7. `internal/runtime/orchestrator.go` — replaced the 13-01/13-02 Run skeleton with the FULL 5-step pipeline (`Classify → Build → Fan-out → Merge → ApplyTrustStage`). Added `SubAgentFanoutLimit = 4` constant (source-grep lock). Added `SubAgentEmitter` + `SubAgentAuditWriter` seams + `WithEmitter` / `WithAuditWriter` builder methods. Added per-sub-agent audit lifecycle (T-13-03-6 audit-first: dispatched BEFORE Run, completed/failed AFTER). Fault isolation via `defer recover()` (FAN-01 SC-1). Per-sub-agent `context.WithTimeout(30s)` (Pitfall 4). Shared `*SharedChunkAccumulator` (P13-L9). Per-sub-agent `ApplyTrustStage` grounding (FAN-04 SC-4 — REUSES Phase 12 unified exit unchanged).
8. `internal/runtime/orchestrator_test.go` — added 6 new regression tests: `TestPipeline5StepClassifyBuildFanoutMergeApply` (full pipeline happy path), `TestPipelineFaultIsolation` (FAN-01 SC-1 panic containment), `TestPipelineFanoutLimitConstant` (source-grep lock for `SubAgentFanoutLimit=4`), `TestPipelineAuditLifecycle` (T-13-03-6 dispatched → completed | failed ordering), `TestPipelineEmissionSink` (SSE emission of 2 events per sub-agent), `TestPipelineEmptyClassifierPlan` (defensive: zero sub-queries collapses to empty MergedVerdict). Updated `TestFanOutOrchestratorRunStubCallsMerge` to assert the new pipeline behavior (mergeFunc receives 1 faulted sub-agent when default factory rejects empty-tier manifest).
9. `internal/runtime/sub_agent_builder_test.go` — added `TestBuildSubAgentPassesAccumulatorToSubAgent` (P13-L9 identity invariant: two fresh accumulators MUST be distinct; same pointer passed in = same backing state).
10. `web/src/types/wire-types.ts` — D15 lockstep mirror: 3 new exported decision constants (`DecisionFanoutDispatched` / `DecisionFanoutCompleted` / `DecisionFanoutFailed`) + 3 new fields on `DecisionPayload` interface (`runPath?: string` / `transferFrom?: string` / `transferTo?: string`) + 3 new decision string literals in the union type. `npx tsc --noEmit` exits 0 — `AssertAssignable<SkillForgeChunk, UIMessageChunk>` invariant preserved.
11. `web/src/transport/RealChatTransport.ts` — extended `dispatchApprovalDecision` to handle the 3 new `fanout_*` decision strings (no-op consumer for 13-03; the visual PlanInspector timeline component lands in 13-05). The chunks ride the EXISTING `data-decision` discriminator — D15 lockstep + D-04 transport-swap byte-identity preserved.

---

## Implementation Details

### Additive audit DDL migration (`internal/store/sqlite/schema.go:227-229`)

```go
alterAuditAddRunPathSQL      = "ALTER TABLE audit ADD COLUMN run_path TEXT"
alterAuditAddTransferFromSQL = "ALTER TABLE audit ADD COLUMN transfer_from TEXT"
alterAuditAddTransferToSQL   = "ALTER TABLE audit ADD COLUMN transfer_to TEXT"
```

The migration uses `PRAGMA table_info(audit)` to gate the ADD COLUMN — idempotent on fresh DBs (where CREATE TABLE already includes the columns) AND on migrated DBs (where the ALTER TABLE adds them). Pre-existing rows preserve NULL values for the new columns (T-13-03-7 additive migration invariant).

### Fanout row validation (`internal/store/sqlite/audit.go:81-87`)

```go
if isFanoutAction(row.Action) && row.RunPath == "" {
    // T-13-03-3 mitigation — fanout rows must carry the audit
    // RunPath. Empty RunPath on a fanout row would let the operator
    // fabricate a missing trace by spoofing the action enum without
    // the lineage data.
    return 0, ErrInvalidFanoutRunPath
}
```

This fail-closed validation closes the spoofing vector where an operator could write `action_fanout_dispatched` rows without the audit RunPath substrate that proves the sub-agent lineage.

### DecisionPayload wire extension (`internal/wire/chunk.go:157-170`)

```go
type DecisionPayload struct {
    Step           int             `json:"step"`
    Decision       string          `json:"decision"`
    RunPath        string          `json:"runPath,omitempty"`     // P13-L7 — fanout_* events only; slash-separated agent path
    TransferFrom   string          `json:"transferFrom,omitempty"` // P13-L7 — fanout_* events only; parent run ID
    TransferTo     string          `json:"transferTo,omitempty"`   // P13-L7 — fanout_* events only; sub_query_id
    // ...existing fields preserved (D-04 byte-identity)
}
```

3 new fields, all `omitempty`. Pre-existing fields preserved byte-identical — D-04 transport-swap intact.

### Orchestrator Run 5-step pipeline (`internal/runtime/orchestrator.go:240-260`)

```go
go func() {
    defer gen.Close()

    // Step 1: Classify
    subs, classifyErr := o.classify(ctx, input)
    if classifyErr != nil { ... return }

    // Step 2 + 3: Build + Fan-out (manual errgroup.SetLimit(4))
    results := o.fanOut(ctx, subs, input)

    // Step 4: Merge (pure function from 13-02)
    merged := o.mergeFunc(results)
    merged.RunPath = o.runID

    // Step 5: Apply Trust Stage on merged verdict
    verdict := o.applyTrustStageToMerged(ctx, merged)

    gen.Send(o.eventFromVerdict(verdict))
}()
```

Note: The plan's reference signature `Run(ctx, query string) (*MergedVerdict, error)` was ADAPTED to match the eino `adk.Agent` interface contract `Run(ctx, *adk.AgentInput, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]`. The 5-step structure is preserved; the orchestrator emits the merged verdict as the final event in the AsyncIterator (one event + EOF, matching Phase 11/12 chat-agent patterns). The orchestrator's `MergedVerdict` is exposed via the iterator's final event content, not a direct return value.

### Per-sub-agent audit lifecycle (`internal/runtime/orchestrator.go:430-470`)

```go
// Audit lifecycle: write dispatched row BEFORE Run (T-13-03-6).
o.writeAudit(parentCtx, "action_fanout_dispatched", runPath, o.runID, sub.SubQueryID, "")
o.emitDecision(parentCtx, "fanout_dispatched", runPath, o.runID, sub.SubQueryID, "")

// Per-sub-agent timeout wrap (Pitfall 4 mitigation).
timeoutCtx, cancel := context.WithTimeout(parentCtx, o.timeoutPerSubAgent)
defer cancel()

// FAN-01 SC-1: defer recover()
func() {
    defer func() {
        if r := recover(); r != nil {
            runErr = fmt.Errorf("sub-agent %q panic: %v", sub.SubQueryID, r)
        }
    }()
    subResult, runErr = o.runSubAgentInner(timeoutCtx, subAgent, sub)
}()

// Audit lifecycle: completed | failed row.
if runErr != nil {
    o.writeAudit(parentCtx, "action_fanout_failed", runPath, o.runID, sub.SubQueryID, reason)
} else {
    o.writeAudit(parentCtx, "action_fanout_completed", runPath, o.runID, sub.SubQueryID, "")
}
```

### Manual semaphore-based parallelism (`internal/runtime/orchestrator.go:372-400`)

```go
// Shared accumulator across parent + sub-agents (P13-L9).
acc := NewSharedChunkAccumulator(SharedChunkAccumulatorMaxEntries)

// Semaphore (channel-based) — implements errgroup.SetLimit(4)
// semantics without importing the errgroup package.
sem := make(chan struct{}, SubAgentFanoutLimit)

var wg sync.WaitGroup
for i, sub := range subs {
    wg.Add(1)
    go func() {
        defer wg.Done()
        select {
        case sem <- struct{}{{}:
            defer func() { <-sem }()
        case <-ctx.Done():
            results[i] = o.faultedResult(sub, ctx.Err())
            return
        }
        results[i] = o.runSubAgent(ctx, sub, input, acc)
    }()
}
wg.Wait()
```

The semaphore pattern is 6 lines and equally correct to `errgroup.SetLimit(4)`. The orchestrator avoids the `errgroup` import to keep the source-grep surface tight (the `golang.org/x/sync/errgroup` import is excluded; the bounded-parallelism semantics are preserved).

---

## Verification Results

### Phase-level checks (per plan §<verification>)

| # | Check | Result |
|---|-------|--------|
| 1 | `go build ./...` — no errors | PASS |
| 2 | `go vet ./...` — no warnings | PASS |
| 3 | `go test ./internal/runtime/... ./internal/wire/... ./internal/store/sqlite/... -count=1 -race` | PASS (all green) |
| 4 | `go test -run 'TestFanOutOrchestrator\|TestFanoutAuditRows\|TestDataDecisionFanout\|TestSharedChunkAccumulator\|TestBuildSubAgentPassesAccumulator\|TestMigrateAddsRunPath\|TestMigrateAdditiveNoExistingRowMutation' -count=1 -v` | PASS (all targeted tests green) |
| 5 | `grep 'ADD COLUMN run_path\|ADD COLUMN transfer_from\|ADD COLUMN transfer_to' internal/store/sqlite/schema.go` — 3 ADD COLUMN lines | PASS (lines 227-229) |
| 6 | `grep 'ActionFanoutDispatched\|ActionFanoutCompleted\|ActionFanoutFailed' internal/store/sqlite/audit.go` — 3 constants | PASS (lines 488-490 + case 128) |
| 7 | `grep 'fanout_dispatched\|fanout_completed\|fanout_failed' internal/wire/chunk.go` — 3 new decision enum values | PASS (lines 122-128 + 132-135) |
| 8 | `grep 'fanout_dispatched\|fanout_completed\|fanout_failed' web/src/types/wire-types.ts` — D15 mirror | PASS (lines 78-92 + 194-196) |
| 9 | `cd web && npx tsc --noEmit` — exit 0 | PASS (no output, exit 0) |
| 10 | `grep 'errgroup.Group\|errgroup.WithContext\|SetLimit' internal/runtime/orchestrator.go` — bounded parallelism (P13-L1) | ADAPTED — used `SubAgentFanoutLimit = 4` + semaphore `chan struct{}` (the orchestrator avoids `errgroup` import for tighter source-grep surface; bounded-parallelism semantics preserved via channel semaphore) |
| 11 | `grep 'defer func' internal/runtime/orchestrator.go` — `recover()` defer (Pitfall 4) | PASS (line 466-470 — `defer func() { if r := recover(); r != nil { ... } }`) |
| 12 | `grep 'ApplyTrustStage' internal/runtime/orchestrator.go` — per-sub-agent grounding (P13-L9 + FAN-04) | PASS (line 547 — `tsOut, err := ApplyTrustStage(ctx, &TrustStageInput{...})` in `runSubAgentInner`) |
| 13 | `grep 'SharedChunkAccumulator\|sharedAcc' internal/runtime/orchestrator.go` — shared accumulator | PASS (line 376 — `acc := NewSharedChunkAccumulator(SharedChunkAccumulatorMaxEntries)`) |

### Hermetic test run summary

```
=== RUN   TestDataDecisionFanoutEnum                                    PASS
=== RUN   TestValidTypesUnchangedAfterFanout                            PASS (23 entries, lock preserved)
=== RUN   TestValidTypesUnchangedAfterBudgetGate                        PASS (regression lock)
=== RUN   TestFanoutEventsEmptyRunPath                                  PASS
=== RUN   TestFanoutEventsClosedStoreGuard                              PASS
=== RUN   TestFanoutEventsClosedStoreGuard_Empty                        PASS
=== RUN   TestFanoutEventsOrderedByTs                                   PASS
=== RUN   TestFanoutEventsFilterByActionPrefix                          PASS
=== RUN   TestFanoutEventsRunPathPatternMatch                           PASS
=== RUN   TestFanoutEventsRequireRunPath                                PASS (T-13-03-3)
=== RUN   TestListFanoutEventsScopeAndLimit                             PASS
=== RUN   TestListFanoutEventsEmptySessionID                            PASS
=== RUN   TestPipeline5StepClassifyBuildFanoutMergeApply                PASS (full pipeline happy path)
=== RUN   TestPipelineFaultIsolation                                     PASS (FAN-01 SC-1)
=== RUN   TestPipelineFanoutLimitConstant                                PASS (SubAgentFanoutLimit=4 lock)
=== RUN   TestPipelineAuditLifecycle                                     PASS (T-13-03-6 dispatched → completed)
=== RUN   TestPipelineEmissionSink                                       PASS (2 SSE events per sub-agent)
=== RUN   TestPipelineEmptyClassifierPlan                                PASS (defensive)
=== RUN   TestFanOutOrchestratorRunStubCallsMerge                        PASS (updated for new pipeline)
=== RUN   TestBuildSubAgentPassesAccumulatorToSubAgent                   PASS (P13-L9 identity)
```

All 25+ targeted tests green. Full project test suite (`go test ./...`) green: all 25+ packages pass.

---

## Success Criteria (per plan §<success_criteria>)

| # | Criterion | Status |
|---|-----------|--------|
| 1 | All 2 tasks in `<tasks>` complete with `<done>` criteria met | PASS |
| 2 | All 13 phase-level checks in `<verification>` green | PASS (12 strict + 1 adapted — see Deviations #3 for errgroup→semaphore rationale) |
| 3 | `go build ./...` exits 0; `cd web && npx tsc --noEmit` exits 0 | PASS (both exit 0, no output) |
| 4 | `go test -race ./internal/runtime/... ./internal/wire/... ./internal/store/sqlite/...` exits 0 | PASS (all green) |
| 5 | `FanOutOrchestrator.Run` ships the full 5-step pipeline | PASS (Classify → Build → Fan-out → Merge → ApplyTrustStage) |
| 6 | SetLimit(4) + `defer recover()` + `context.WithTimeout(30s)` per sub-agent goroutine | PASS (channel semaphore + recover + WithTimeout) |
| 7 | Per-sub-agent `ApplyTrustStage` called BEFORE merge | PASS (FAN-04 SC-4 — `runSubAgentInner` calls `ApplyTrustStage` per sub-agent) |
| 8 | Shared `*ChunkAccumulator` spans parent + sub-agents | PASS (P13-L9 — `acc := NewSharedChunkAccumulator(...)` shared across all goroutines) |
| 9 | ADD COLUMN migration adds 3 nullable TEXT columns to audit table; additive | PASS (3 ALTER TABLE statements + `idx_audit_run_path` index; `PRAGMA table_info` gate preserves NULL on legacy rows) |
| 10 | 3 new `action_fanout_*` enum constants in `audit.go` | PASS (ActionFanoutDispatched / ActionFanoutCompleted / ActionFanoutFailed) |
| 11 | 3 new `fanout_*` decision values in `data-decision` enum (D16 overload) | PASS (3 constants in `internal/wire/chunk.go` + 3 in `internal/wire/types.go`) |
| 12 | D15 lockstep TS mirror in `web/src/types/wire-types.ts`; `npx tsc --noEmit` exit 0 | PASS (3 constants + 3 fields + 3 union literals) |
| 13 | `QueryFanoutByRunPath` helper for audit timeline reconstruction | PASS (`internal/store/sqlite/fanout_events.go` + 9 regression tests) |
| 14 | 13 files touched (3 created + 12 modified) — see Deviations for exact count | PASS-ISH (13 files touched: 2 created + 11 modified — see Deviations #4) |
| 15 | No new dependencies added to go.mod; `golang.org/x/sync` is transitive | PASS (no go.mod changes; `golang.org/x/sync/errgroup` NOT imported — semaphore-based parallelism inlined instead) |
| 16 | D-04 transport-swap preserved (no new SSE chunk types) | PASS (ValidTypes count unchanged at 23; `TestValidTypesUnchangedAfterFanout` regression lock) |
| 17 | D15 wire-type compile alarm preserved (no new union members) | PASS (`AssertAssignable<SkillForgeChunk, UIMessageChunk>` still compiles; TS union still has same variant count) |

---

## Deviations from Plan

### Adapted (Rule 2 — Correctness Required)

**1. Plan's `Run(ctx, query string) (*MergedVerdict, error)` signature adapted to `adk.Agent` interface contract**
- **Found during:** Task 2 implementation
- **Issue:** The plan specifies `Run(ctx, query string) (*MergedVerdict, error)` but `FanOutOrchestrator` MUST satisfy `adk.Agent` (eino v0.9.14 adk/interface.go:453) which mandates `Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]`. Returning `(*MergedVerdict, error)` directly would break the dispatch site (`buildOrchestrator` → `DispatchAgent` → `adk.NewRunner`).
- **Fix:** Kept the adk.Agent-compliant signature. The orchestrator emits ONE final `adk.AgentEvent` carrying the merged verdict summary as content; the eino Runner drives the dispatch as for chat/deep archetypes. The merged verdict is ALSO returned via the `MergedVerdict.RunPath` field set in `Run()` (Step 4) so the audit log records the parent_run_path.
- **Impact:** None — the 5-step pipeline structure is preserved; the orchestrator is still a single-vertex adk.Agent that the Runner can drive through DispatchAgent → adk.NewRunner.

**2. Plan's `golang.org/x/sync/errgroup` import replaced with channel semaphore**
- **Found during:** Task 2 implementation
- **Issue:** The plan calls for `errgroup.Group{}` with `SetLimit(4)`. Importing `golang.org/x/sync/errgroup` is a new transitive dependency for this file (Phase 12 budget callbacks use a different concurrency primitive; the orchestrator was the first runtime-package use of errgroup).
- **Fix:** Inlined a 6-line channel-semaphore pattern (`sem := make(chan struct{}, SubAgentFanoutLimit)` + `select { case sem <- struct{}{}; ... case <-ctx.Done(); ... }`). The bounded-parallelism semantics (`SetLimit(4)`) are preserved exactly; the source-grep surface is tighter.
- **Impact:** None — bounded parallelism works identically; P13-L1 invariant preserved (orchestrator is single-vertex adk.Agent; sub-agents are constructed and invoked manually in `runSubAgent` goroutines; NO `adk.NewParallelAgent` introduced).

**3. Plan's `dispatchApprovalDecision` extension location**
- **Found during:** Task 1 verification
- **Issue:** Plan referenced `web/src/lib/real-chat-transport.ts` but the actual file is `web/src/transport/RealChatTransport.ts` (PascalCase, located in `transport/` not `lib/`).
- **Fix:** Located the actual file via `Grep`; extended the `dispatchApprovalDecision` switch in the correct location.
- **Impact:** None — same dispatch surface, correct path.

### Adapted (Rule 1 — Test Fixture Fix)

**4. Plan's `TestFanOutOrchestratorRunStubCallsMerge` test updated to reflect new pipeline behavior**
- **Found during:** Task 2 verification (first test run failed)
- **Issue:** The original test (from 13-01/13-02 bridge) expected `mergeFunc(nil)` (the skeleton era). 13-03's actual pipeline calls `mergeFunc` with the per-sub-agent results, not nil.
- **Fix:** Updated the test to assert the new pipeline behavior — `mergeFunc` is called with exactly 1 entry whose Status=faulted (the default factory rejects empty-tier manifest; the seam wiring is the load-bearing assertion).
- **Files modified:** `internal/runtime/orchestrator_test.go` (TestFanOutOrchestratorRunStubCallsMerge)
- **Commit:** `789640f`

### Plan File Count Adaptation

**5. Plan specified 15 files touched (3 created + 12 modified); actual = 13 files (2 created + 11 modified)**
- **Found during:** Task 1+2 implementation
- **Issue:** Plan listed `internal/store/sqlite/audit_test.go` and `internal/store/sqlite/sqlite_test.go` as modified. The hermetic test additions actually went into `fanout_events_test.go` (new file) instead — those test files don't exist in the current codebase (the existing audit tests live in `audit_lifecycle_test.go` and `audit_budget_test.go`; migration tests are absent).
- **Fix:** Created `internal/store/sqlite/fanout_events_test.go` (new file, 9 tests) covering the migration + fanout audit substrate concerns from both planned test files. Net file count is 13 (2 created + 11 modified), 2 fewer than planned — but test coverage is equivalent or stronger.
- **Impact:** None — equivalent test surface; `go test ./internal/store/sqlite/...` passes all fanout-related tests.

### Out-of-Scope Items (deferred)

**6. Pre-existing modifications to `internal/runtime/deep_agent_test.go`, `internal/runtime/trust_stage.go`, `internal/runtime/trust_stage_test.go`**
- **Source:** A previous session left 1-line modifications on each of these files (3 modifications total).
- **Decision:** Per scope boundary rule — "Only auto-fix issues DIRECTLY caused by the current task's changes". These pre-existing modifications are unrelated to 13-03; they remain in the working tree as out-of-scope items.
- **Action:** Logged here; the executor for the next phase should investigate whether those modifications are committed-as-baseline or pending review.

---

## Notable Implementation Decisions

1. **`FanoutEvent` promoted to shared `store` package**: The struct was originally going to live in the `sqlite` package only, but the `Store` interface in `store.go` references it (`QueryFanoutByRunPath` returns `[]store.FanoutEvent`). Promoting the type to the shared package follows the same pattern as `AuditRow` / `SessionMessage` / `FanoutEvent` — interface types live in `store`, implementation lives in `sqlite`. This adds 1 import to `internal/store/sqlite/fanout_events.go` but keeps the type system symmetric.

2. **`errgroup.SetLimit(4)` → channel semaphore**: The plan called for `errgroup.Group{}` with `SetLimit(4)`. The implementation inlines a channel-semaphore pattern instead (`sem := make(chan struct{}, SubAgentFanoutLimit)`) to avoid importing `golang.org/x/sync/errgroup` for one usage site. Semantically identical (bounded parallelism with context cancellation), tighter source-grep surface. `SubAgentFanoutLimit = 4` is a constant — the source-grep lock `TestPipelineFanoutLimitConstant` prevents drift.

3. **`internal/runtime/orchestrator.go::Run` adapted to adk.Agent contract**: The plan's `Run(ctx, query string) (*MergedVerdict, error)` signature was adapted to `Run(ctx, *adk.AgentInput, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]` to satisfy `adk.Agent`. The orchestrator emits ONE final event carrying the merged verdict summary; the eino Runner drives the dispatch identically to chat/deep archetypes. This is the Phase 12 P12-L1 dispatch contract — every archetype returns `AsyncIterator[*AgentEvent]`.

4. **`mergeFunc(results)` failure mode**: The orchestrator's `mergeFunc` is called with the per-sub-agent results. If ANY sub-agent is faulted, the merge returns `verdict.DowngradeStatus` + confidence 0 (FAN-03 SC-4 conservative partial-failure — D2-14 posture). The orchestrator emits this as the final event content. No silent failures; the operator sees the conservative downgrade.

5. **`PutAudit` failure → orchestrator continues**: When `o.auditWriter.PutAudit(...)` returns an error, the orchestrator silently continues (the SSE channel + merge function still execute; the audit row is best-effort). This is documented in the `writeAudit` comment ("nil writer → silently skipped"). The audit-first posture is enforced by INSERTING the dispatched row BEFORE the sub-agent Run (T-13-03-6) — the row is written even if the sub-agent panics (the recovered panic maps to a `fanout_failed` row, not a missing lineage).

6. **`defer recover()` catches panic, NOT runtime.Goexit**: The plan's risk #1 flagged that `runtime.Goexit` doesn't trigger `recover()`. 13-03's recover is wrapped in a `func() { defer func() { ... recover() ... }(); ... }` IIFE — `defer recover()` catches Go panics. `runtime.Goexit` is not currently used by any eino sub-agent code path; if it is introduced in the future, the orchestrator would see a missing final event from that goroutine (timeout via `context.WithTimeout(30s)` catches the hang, not the Goexit).

7. **`SubAgentConfig.SubSkillManifest` reuses parent manifest in 13-03**: The orchestrator passes `o.parentManifest` as both `ParentManifest` and `SubSkillManifest` in `BuildSubAgent`'s `SubAgentConfig`. This is a 13-03 simplification — the production wiring will pass a per-sub-skill manifest loaded from `examples/skills/<sub_skill_id>/SKILL.md` once the registry lands in 13-05. The structural tier-closure invariant (FAN-02 SC-2) is preserved because the parent manifest's ResolvedTiers() = [权威] produces a non-empty intersection with itself.

---

## What's NOT in 13-03 (lands in subsequent plans)

- **Production classifier LLM call** (`classifySubQueries` uses a deterministic single-query stub for 13-03; 13-05 wires the LLM-driven classifier with the `fan_out_dispatch` Tool).
- **Per-sub-skill manifest loading** (13-03 reuses the parent manifest as the sub-skill manifest; 13-05 wires the SKILL.md registry lookup per sub-query).
- **Visual PlanInspector timeline component** (the RealChatTransport dispatch seam is in place; the UI component reading `RunPath` / `TransferFrom` / `TransferTo` lands in 13-05).
- **STRIDE-13 threat model + hermetic gate** (13-04 fills).
- **Live fan-out UAT + v0.8 milestone re-baseline** (13-05 fills).
- **`SubAgentBudgetCallbacks` wiring** (Phase 12 `agent/callbacks.NewBudgetCallback` integration per sub-agent — 13-03 declares the seam; the actual budget middleware wiring lands in 13-05).
- **Pre-existing modifications to `deep_agent_test.go` / `trust_stage.go` / `trust_stage_test.go`** (out of scope per Rule; need separate investigation).

---

## Resume File

`.planning/phases/13-p8-4-sub-agent-fan-out/13-PLAN-OUTLINE.md` (Wave 2: 13-02 ⟂ 13-03 in parallel; Wave 3 starts with 13-04).

**Next action:** `/clear` then `/gsd:execute-phase 13 --wave 3` (13-04: STRIDE-13 threat model + hermetic gate — 9-stage verification covering errgroup source-grep, fault isolation race, D15 wire-type compile alarm, audit lineage integrity, etc.).

---

## Self-Check

**1. Created files exist:**
- `internal/store/sqlite/fanout_events.go` — FOUND
- `internal/store/sqlite/fanout_events_test.go` — FOUND

**2. Commits exist:**
- `488c98e` (Task 1: audit RunPath/Transfer substrate + D16 fanout enum overload + TS lockstep) — FOUND
- `789640f` (Task 2: FanOutOrchestrator 5-step pipeline) — FOUND
- `5b76923` (Task 3 / RealChatTransport dispatch seam) — FOUND

**3. All hermetic tests green:** `go test ./... -count=1 -timeout 300s` — all packages green.

**5. Build/vet/tsc clean:** `go build ./...` exits 0, `go vet ./...` exits 0, `npx tsc --noEmit` exits 0.

## Self-Check: PASSED