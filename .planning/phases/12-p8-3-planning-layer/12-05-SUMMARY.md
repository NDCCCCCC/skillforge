---
phase: 12-p8-3-planning-layer
plan: 05
subsystem: deep-agent / planning-layer
tags: [eino, deep-agent, budget-gate, fnv64a, sse, audit, plan-04, p12-l3, p12-l9]

# Dependency graph
requires:
  - phase: 12-01
    provides: runtime.BudgetConfig + DispatchConfig substrate + BuildDeepAgent factory with empty NewBudgetCallbacks placeholder
  - phase: 12-02
    provides: write_todos audited Tool + action_plan_todo_* audit cross-reference + audit write_todos fail-closed pattern
  - phase: 11-p8-2-write-tools-approval-gate
    provides: D11-C3 audit-first fail-closed posture + adk.TypedChatModelAgentMiddleware hook vocabulary
  - phase: 03-01
    provides: DecisionPayload + D16 data-decision discriminator + data-decision ValidTypes registry
provides:
  - 3-layer budget gate middleware (max_iter + token + loop) with D11-C3 audit-first forced_stop
  - audit BudgetLayer column + ActionBudgetTriggered enum (T-12-05-4 / T-12-05-6)
  - runtime wiring: NewBudgetCallbacks returns 1 middleware + DispatchConfig.BudgetStore/BudgetEmitter seams
  - wire.DecisionForcedStop constant + Decision string enum extension (P12-L3, no new discriminator)
affects:
  - 12-04 (chatHandler wires the real emitter + plan_api audit-query joins on BudgetLayer)
  - 12-06 (FE real-chat-transport forced_stop case — D15 paired commit)
  - 12-07 (AuditTimelineTab renders budget events; budget-gate SSE emitter closes over Hertz ctx)
  - 12-08 (acceptance gate measures actual token usage on the 50-case golden set to recalibrate DefaultTokenBudgetPerRun)

# Tech tracking
tech-stack:
  added: []  # zero new external dependencies; eino v0.9.14 + hash/fnv stdlib only
  patterns:
    - 3-layer defense-in-depth budget gate with D11-C3 audit-first fail-closed trigger
    - fnv64a rolling-3-hash loop detector with first-200-chars truncation (Pitfall 5 mitigation)
    - audit BudgetLayer column on the audit table (nullable; whitelist-validated by PutAudit)
    - dual enforcement: factory literal `MaxIteration: 10` + middleware-side model-call counter
    - D16 decision-enum overload pattern extended from Phase 3 / Phase 11

key-files:
  created:
    - internal/agent/callbacks/budget.go
    - internal/agent/callbacks/budget_test.go
    - internal/wire/types.go
  modified:
    - internal/store/sqlite/audit.go (Task 1; committed pre-pause as 3aab985)
    - internal/store/sqlite/schema.go (Task 1; budget_layer ADD COLUMN migration)
    - internal/store/sqlite/audit_budget_test.go (Task 1; 4 tests)
    - internal/store/store.go (Task 1; AuditRow.BudgetLayer field)
    - internal/runtime/budget.go (BudgetConfig.Store/Emitter + 1-middleware factory)
    - internal/runtime/budget_test.go (3 new wiring tests)
    - internal/runtime/deep_agent.go (Handlers wires Store+Emitter; P12-L2 literal now in deep.New call)
    - internal/runtime/factory.go (DispatchConfig.BudgetStore + BudgetEmitter fields)
    - internal/wire/chunk.go (DecisionPayload godoc extends to forced_stop without adding discriminator lines)
    - internal/wire/chunk_test.go (TestForcedStopDataDecisionRouting + TestValidTypesUnchangedAfterBudgetGate)

key-decisions:
  - "D11-C3 audit-first fail-closed POSTURE PRESERVED in budget.go: PutAudit fires BEFORE EmitForcedStopDecision; an audit INSERT error suppresses the SSE emit (the user must never see a forced_stop the durable record does not back)"
  - "P12-L3 (no new wire discriminator): forced_stop rides the existing data-decision chunk type as a new VALUE in the DecisionPayload.Decision string enum; chunk.go discriminator-count grep lock stays at 14"
  - "P12-L9 (loop detection): fnv64a(tool_name + first 200 chars of args), rolling LoopWindow-sized window (default 3); 200-char truncation tolerates legitimate multi-hop with refined queries (Pitfall 5)"
  - "P12-L2 strengthened: the MaxIteration: 10 literal is now INLINE in the deep.New call in its own gofmt alignment group, so TestMaxIterationHardCap_Literal10's source-grep binds to CODE, not just the doc comment"
  - "Resolve() LoopWindow floor fix (Rule 1): pre-pause code let LoopWindow=1 pass through unchanged, which would force_stop on the first tool call; clamped to 2 (the documented floor). Zero and explicit defaults untouched"
  - "LoopWindow floor=2 (NOT 3) per the documented intent: 'Window of 1 would flag every call as a loop; floor at 2' — the pre-pause comment was right but the implementation left 1 un-clamped"

patterns-established:
  - "Budget gate pattern: BudgetConfig (runtime layer) + agentcallbacks.BudgetConfig (implementation layer) — runtime stays decoupled from the middleware internals; both expose Resolve() for defaults"
  - "D11-C3 trigger sequence: PutAudit (fail-closed) → EmitForcedStopDecision → return error. Mirrors Phase 11 hitl/audit_writer.go (the plan's risk text was off — see Deviations)"
  - "audit.fail-closed test seam: shared eventLog records audit + emit events in order; TestBudgetTriggerOrderAuditBeforeEmit asserts the lock"
  - "Closed-store test: sqlite.New(:memory:) + Close() → PutAudit returns sql.ErrConnDone; the middleware MUST NOT emit SSE (TestBudgetFailClosedClosedStoreSuppressesSSE)"
  - "wire constant + value-catalog: DecisionForcedStop lives in internal/wire/types.go with the full enum doc; internal/agent.DecisionForcedStop (existing) carries the same string — locked by shared literal, NOT a Go-level import (which would cycle test packages)"
  - "Source-grep locks as a structural pattern: P12-L2 (MaxIteration: 10 call-site literal), P12-L9 (fnv.New64a hash), P12-L3 (chunk.go discriminator count = 14) — all bound to CODE, not comments"

requirements-completed: [PLAN-04]

# Metrics
duration: ~25min
completed: 2026-08-20
---

# Phase 12 Plan 05: PLAN-04 Budget Gate Summary

**3-layer DeepAgent budget gate (max_iter + token + loop) with D11-C3 audit-first forced_stop, P12-L3 no-new-discriminator wire extension, and P12-L2 source-grep call-site lock.**

## Scope

PLAN-04 SC-4 requires three budget gates that fire `forced_stop` conservatively (never run-to-exhaustion). 12-05 ships:

1. **Layer 1 — max_iter**: dual enforcement. The factory carries the inline literal `MaxIteration: 10` in its own gofmt alignment group (`internal/runtime/deep_agent.go:95`); the middleware's `BeforeModelRewriteState` independently counts model invocations and fires forced_stop on call 11. Either side drifting leaves the other intact — structural cap, not a config knob.
2. **Layer 2 — token**: `atomic.Int64` accumulator on `msg.ResponseMeta.Usage.TotalTokens` (OpenAI-compatible standard field; GLM-5.2 reports it). `BeforeModelRewriteState` refuses the next call once `used > budget` (strictly-greater: the run gets its full 50000-token budget). Default 50000; configurable per-Run.
3. **Layer 3 — loop**: `fnv64a(tool_name + first 200 chars of args)` into a rolling window of size `LoopWindow` (default 3). 200-char truncation tolerates legitimate multi-hop refinement (Pitfall 5). Triggers when the last N hashes are all identical.

On **any** trigger the middleware executes the D11-C3 sequence: (1) `Store.PutAudit(action="action_budget_triggered", budget_layer=<max_iter|token|loop>)`; failure suppresses the SSE emit and returns the error (fail-closed). (2) `Emitter.EmitForcedStopDecision(reason)` where reason is `"PLAN-04 budget exhausted: <layer> (<detail>)"`. (3) return a non-nil error — the adk runner fails fast and the model call / tool call that tripped the gate never executes.

The SSE chunk rides the **existing** `data-decision` discriminator as a new VALUE in the `Decision` string enum (P12-L3 lock; D16 overload pattern extended from Phase 3). `internal/wire/chunk.go` discriminator count stays at 14. The TS mirror ships in 12-07 under the D15 lockstep invariant.

The audit substrate extends `AuditRow` with `BudgetLayer string` (nullable; populated only for `action_budget_triggered` rows) plus a `BudgetLayerNames` whitelist (max_iter | token | loop) — `PutAudit` rejects unknown values with `ErrInvalidBudgetLayer` (T-12-05-6 spoofing guard). The additive DDL is `ALTER TABLE audit ADD COLUMN budget_layer TEXT NOT NULL DEFAULT ''` (idempotent via the migration helper).

## Tasks Executed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | Audit BudgetLayer column + ActionBudgetTriggered enum | `3aab985` (pre-pause) | internal/store/sqlite/{audit.go, schema.go, audit_budget_test.go}, internal/store/store.go |
| 2 | 3-layer budget middleware + 13 tests | `12d4a2f` | internal/agent/callbacks/budget.go (465 lines), budget_test.go (580 lines) |
| 3 | runtime wiring + P12-L2 call-site literal | `2ac743c` | internal/runtime/{budget.go, budget_test.go, deep_agent.go, factory.go} |
| 4 | wire.DecisionForcedStop + P12-L3 invariant tests | `ff581bd` | internal/wire/{types.go (new), chunk.go, chunk_test.go} |

Total: **3 commits this run** + Task 1 from pre-pause + final docs commit. **14 files touched** (3 created + 11 modified).

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| Build (12-05 territory) | `go build ./internal/{store,agent/callbacks,runtime,wire}/...` | OK |
| Build (full repo) | `go build ./...` | OK |
| Vet (full repo) | `go vet ./...` | OK |
| Tests (territory) | `go test ./internal/{store/sqlite,agent/callbacks,runtime,wire}/... -count=1` | 4 packages OK |
| Integration contract | `go test -tags=contract -run 'TestNoLegacyInterruptAPI|TestNoLegacyInterruptAPIPhase11' ./integration/contract/...` | 2/2 PASS |
| P12-L2 grep | `grep -n 'MaxIteration: 10' internal/runtime/deep_agent.go` | line 95 = call-site code |
| P12-L9 grep | `grep -n 'fnv' internal/agent/callbacks/budget.go` | `fnv.New64a` line 391 |
| Audit grep | `grep -n 'action_budget_triggered' internal/store/sqlite/audit.go` | enum const + 5 references |
| P12-L3 grep | `grep -c 'data-plan-trace\|data-decision' internal/wire/chunk.go` | **14** (unchanged) |
| forced_stop const | `grep -n 'forced_stop' internal/wire/types.go` | line 76 `DecisionForcedStop = "forced_stop"` |
| Migration | `grep -n 'ADD COLUMN budget_layer' internal/store/sqlite/schema.go` | line 206 |

`-race` is unavailable on this Windows host (cgo gcc missing) — the plain runs are green; the middleware is mutex+atomic-guarded by construction (see budget.go `BudgetState{...}`).

## Deviations from Plan

### Rule 1 (auto-fix bugs)

**1. LoopWindow=1 self-inflicted DoS — `BudgetConfig.Resolve()` floor**
- **Found during:** Task 2 review of pre-pause budget.go
- **Issue:** The pre-pause `Resolve()` had a nested branch that filled `<=0` with the default but let `<=1` pass through unchanged. A 1-element window trivially equals itself (`len(recentHashes) == 1 == window → allSame=true`), so the FIRST tool call on any Run would fire forced_stop — the gate DoS'ing itself.
- **Fix:** Rewrote the Resolve block to clamp `<=0 → default(3)` and `==1 → 2` (the documented floor).
- **Files modified:** internal/agent/callbacks/budget.go
- **Test added:** TestBudgetLoopWindowFloor (`BudgetConfig{LoopWindow:1}.Resolve().LoopWindow == 2`)

### Rule 3 (auto-fix blocking — task boundary)

**2. `internal/runtime/budget.go` BudgetConfig gained a field not in plan sketch**
- **Found during:** Task 3 wiring
- **Issue:** The plan Task 3 sketch maps runtime.BudgetConfig → agentcallbacks.BudgetConfig via `{TokenBudget, LoopWindow, RunID: SessionID, Store, Emitter}`. It did NOT include `MaxIteration`. Without it, the middleware's own max_iter counter falls back to the agentcallbacks default (10) — accidentally identical to the factory literal — but a future bump in either side would silently desync.
- **Fix:** Added `MaxIteration` to the mapping so the dual-enforcement stays visibly consistent. The middleware's Resolve fills 10 by default; the runtime now forwards whatever the caller (or `BudgetConfig{}.Resolve()`) computed.
- **Files modified:** internal/runtime/budget.go
- **No test impact** (the existing TestBudgetMaxIterationCap exercises the default path; a future test could observe explicit pass-through).

**3. P12-L2 source-grep locked to COMMENT before Task 3**
- **Found during:** Task 2 TestBudgetMaxIterationCap
- **Issue:** The pre-pause `deep_agent.go` used `const maxIter = 10` and `MaxIteration: maxIter,` in the deep.New call. `strings.Contains(src, "MaxIteration: 10")` only matched the doc comment on line 16 — the lock was comment-deep. A refactor that renamed the const or removed the comment line would silently break the test.
- **Fix:** Task 3 inlined the literal `MaxIteration: 10,` in its own gofmt alignment group (preceded by a full-line comment + blank line). The grep now binds to the actual call-site code (line 95).
- **Files modified:** internal/runtime/deep_agent.go
- **Test verified:** `grep -n 'MaxIteration: 10' internal/runtime/deep_agent.go` returns lines 16, 56 (doc), AND **95 (call-site code)** — the lock is now structural.

### D11-C3 ordering (orchestrator-flagged from plan verification)

**4. Plan risk text + plan Task 2 sketch were internally inconsistent on the audit-vs-emit order**
- **Plan risk text said:** "If SSE emit succeeds but audit INSERT fails, the user sees forced_stop but no audit row. **Mitigation:** The order is audit-INSERT-then-emit (mirrors D11-C3)."
- **Plan Task 2 code sketch suggested:** emitter first, audit second (the if-block ordering in the sketch).
- **Orchestrator-flagged catch (resume instructions §⚠️):** "implement audit-first, emit-second".
- **What shipped:** The pre-pause budget.go `triggerForcedStop` already had the CORRECT order (audit INSERT first, SSE emit second). My implementation preserved it (no change needed). Two tests lock it:
  - `TestBudgetTriggerOrderAuditBeforeEmit` — shared event log asserts `["audit:token", "emit:..."]` order.
  - `TestBudgetFailClosedClosedStoreSuppressesSSE` — closed `*sqlite.SQLiteStore` returns `sql.ErrConnDone` from PutAudit; middleware returns error containing "audit INSERT fail-closed" AND `emitter.reasons == 0` (NO SSE chunk emitted).

## Stub / Empty State Scan

No intentional stubs. The middleware behavior with `nil` Store + nil Emitter (the no-op path) is documented in `BudgetConfig` godoc: the gate still fires (budget protection must not depend on audit availability); only the durable row + live-stream surface is suppressed. This is by design — 12-07 ships the real Hertz-backed emitter closure; the production wiring at the chatHandler boundary is out of scope for 12-05 (the plan confines 12-05 to the middleware substrate).

## Known Stubs

None. All `?` fields in this plan's territory are filled:
- `NewBudgetCallbacks` returns the 3-layer middleware (was empty placeholder in 12-01).
- `DispatchConfig.BudgetStore` + `BudgetEmitter` are typed (agentcallbacks interfaces); nil values are the documented no-SSE/no-audit fallback path.
- The audit `BudgetLayer` column has an additive migration; `BudgetLayerNames` whitelist + `ErrInvalidBudgetLayer` sentinel cover the T-12-05-6 spoofing guard.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: T-12-05-4 / T-12-05-6 (mitigated) | internal/agent/callbacks/budget.go + internal/store/sqlite/audit.go | Budget event visibility (audit row) + layer-value spoofing guard (whitelist) — both mitigated within this plan |

No new threat surface beyond what the plan's threat model already enumerated. The middleware does NOT introduce new network endpoints, auth paths, or file access patterns; it reads `ResponseMeta.Usage.TotalTokens` (existing model API) and writes a new audit row + a new SSE chunk value. The audit row's BudgetLayer column is enumerable (the AuditFilter extension for layer filtering is 12-07 territory).

## Phase 12 / Plan 12-05 Sign-Off

- [x] All 4 tasks executed with `<done>` criteria met (Task 1 pre-pause `3aab985`; Tasks 2-4 committed this run as `12d4a2f`, `2ac743c`, `ff581bd`)
- [x] Per-task atomic commits with `feat(12-05):` prefix
- [x] All 10 phase-level checks in `<verification>` green
- [x] `go build ./...` + `go vet ./...` exit 0 (12-04's in-flight `internal/llm/extract.go` resolved by their commits `929b1d7`/`bc29621`/`1ffb305`)
- [x] P12-L2 call-site literal locked (`MaxIteration: 10` at line 95)
- [x] P12-L3 discriminator count unchanged (14) — forced_stop rides existing `data-decision` chunk type
- [x] D11-C3 audit-first fail-closed posture verified by 2 tests (event log + closed-store SSE suppression)
- [x] 20+ new tests across budget middleware + audit extension + runtime factory wiring + wire extension (sum well exceeds the plan's 20-test target)
- [x] 14 files touched (3 created + 11 modified); no files outside the plan's territory modified
- [x] Zero new external dependencies added to go.mod (eino v0.9.14 unchanged)
- [x] Plan requirement PLAN-04 completed

## Self-Check: PASSED

- All created files exist (budget.go, budget_test.go, types.go).
- All 4 commits reachable in `git log --oneline` (`3aab985`, `12d4a2f`, `2ac743c`, `ff581bd`).
- `grep` locks verified by the verification script output above.
