---
phase: 26-kb-registry-foundation
fixed_at: 2026-09-07T05:20:20Z
review_path: .planning/phases/26-kb-registry-foundation/26-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 26: Code Review Fix Report

**Fixed at:** 2026-09-07T05:20:20Z
**Source review:** .planning/phases/26-kb-registry-foundation/26-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (CR-01..CR-04 + WR-01 + WR-02)
- Fixed: 6
- Skipped: 0

All work was done on an isolated worktree branch (`gsd-reviewfix/26-*`) and
fast-forwarded onto `phase-20-p10-6-admin-console`; each fix is an atomic commit.

## Fixed Issues

### CR-01: Production import surface broken — ImporterConfig.KBRegistry never wired

**Files modified:** `cmd/server/main.go`, `cmd/server/wiring_lock_test.go`
**Commit:** e4211c6
**Applied fix:** `KBRegistry: kbReg` wired into the boot `data.ImporterConfig`
(the only production construction site), so `ValidateRules` resolves rule-file
tiers against the live registry instead of failing closed on nil — every
Preview/Commit/Push/DryRun of `datarules/loans.yaml` works again in a real
boot. Added `TestImporterConfigWiresKBRegistry`, a source-grep lock on the
literal (the failure mode is invisible to unit tests, which pass a registry
explicitly). Verified failing before the fix.

### CR-02: Citation whitelist not enforced on the fan-out path

**Files modified:** `internal/runtime/fanout_runtime.go`, `internal/runtime/trust_stage.go`,
`internal/runtime/orchestrator.go`, `internal/runtime/factory.go`,
`internal/runtime/citable_gate_test.go` (new), `internal/agent/runner_setup.go`
**Commit:** adce584
**Applied fix:** Same gate as the other two insertion points, single-source
predicate `kb.Citable` (nil registry = legacy posture; lookup failure =
not evidence): `SharedChunkAccumulator.SetKBRegistry` + insertion-time filter
in `Add` (a dropped hit frees its chunk ID); `TrustStageInput.KBRegistry` +
runtime-local `citableChunksByID` for the `ApplyTrustStage` grounding map
(defense in depth); `DispatchConfig.KBRegistry` → `FanOutOrchestrator`, bound
onto every shared accumulator in `fanOut` and threaded into both
`ApplyTrustStage` call sites; `BuildRunner` forwards
`RunnerBuildConfig.KBRegistry`. Behavioral tests: a merged verdict whose
union holds only a historical hit now hits the trust floor (refusal) instead
of grounding; the same union with an authority hit still grounds;
`citableChunksByID` excludes historical/unregistered keys; nil-registry
legacy posture locked.

### CR-03: Nil registry is a real production assembly state (three boot wirings)

**Files modified:** `cmd/server/main.go`, `cmd/server/wiring_lock_test.go`,
`cmd/eval/main.go`, `cmd/eval/wiring_lock_test.go` (new),
`internal/server/runner_setup.go`, `internal/server/runner_wiring_test.go` (new)
**Commit:** dc05ca7
**Applied fix:** `KBRegistry: kbReg` in `cmd/server`'s `agent.AgentConfig`
(agent path accumulator gate live); `buildAgent` takes `kbReg kb.Registry`
and both `cmd/eval` call sites pass it (eval measures the same gated contract
production serves); `KBRegistry: s.KBRegistry` in the ChatHandler
`BuildRunner` (memory:facts Skills no longer fail hard with "kb registry
required" on that path). All three construction sites are locked by
source-grep tests in the same discipline as 26-09's `TestBootAssemblyOrder`.
All three locks verified failing before the fix.

### CR-04: Shared HITL singletons mutated per request (data race + audit mis-attribution)

**Files modified:** `internal/agent/hitl/registry.go`,
`internal/agent/hitl/hitl_memory_facts.go`, `internal/agent/hitl_tools.go`,
`internal/agent/hitl/handler_session_test.go` (new),
`internal/agent/hitl/threat_mitigations_test.go`
**Commit:** 629590a
**Applied fix:** Request-scoped handlers instead of shared-instance mutation.
New `hitl.HandlerForSession(target, reg, sessionID, skillID, ruleMatcher)`
returns a fresh `MemoryFactsTool` clone (`cloneForSession`: value copy +
registry rebind + per-request identity — the audit hook rides along) for
handlers with bindable state, and the shared read-only singleton otherwise.
`WithContext` deleted (nothing else called it; keeping a shared-instance
mutator would invite the race back); `BindKBRegistry` kept for explicit
boot/test construction and re-documented as off the per-request path;
`BuildToolsForSkill` no longer touches globals, so the whitelist recompute is
pure again. Nil-registry fail-closed contract (T-26-06-2) moves to the request
path intact. The T-HITL-07 sha256 body lock was re-ranged 320-422 → 339-441;
`InvokableRun`'s body is byte-identical (hash unchanged, verified).
Tests: request-scoping + untouched-singleton assertions, nil/unbound
fail-closed, and a 16-goroutine × 50-iteration hammer asserting zero
cross-session mis-attribution — clean under `-race`.
**Status note:** logic changed (per-request scoping) — flagged for human
verification per the fixer's logic-bug policy; concurrency and behavioral
tests cover it under `-race`.

### WR-01: Deactivating a KB permanently 500s GET /api/knowledge/graph

**Files modified:** `internal/knowledge/graph.go`, `internal/knowledge/graph_test.go`
**Commit:** d9cb8b4
**Applied fix:** `GraphBuilder.Build` adopts the walker's three-state
semantics: active slug renders; a non-active but non-empty tier is skipped
and summed into one `[WARN]` line (KBM-02 keeps deactivated data on disk with
no delete verb, so stale rows are expected and could never be nodes anyway);
an EMPTY tier stays a hard `ErrInvalidTier` (corrupt data, not a deactivation
consequence). Two tests rewritten to the new contract, one added for the
empty-tier reject.

### WR-02: Registry signature and entry set swapped as two independent atomics

**Files modified:** `internal/kb/registry.go`, `internal/kb/registry_test.go`
**Commit:** d67d8a8
**Applied fix:** Entries + signature packed into one immutable
`snapshotState` behind a single `atomic.Pointer[snapshotState]`; writers build
the state and store it once, readers take one load and get a self-consistent
pair. `TestSnapshotSignatureAlwaysMatchesEntrySet` hammers `swap` from 4
goroutines while readers recompute the signature of the observed entry set;
it reproduced the torn read against the pre-fix implementation.

## Skipped Issues

None — all six in-scope findings were fixed.

## Open (out of scope for this round)

- WR-03 (seed-merge audit rows), WR-04 (verify-script quote forms), WR-05
  (refusal-suggestion drift), WR-06 (personal/draft content endpoints),
  WR-07 (kb.Store context), IN-01..IN-10.
- Residual race seam of the same class as CR-04: `hitl.InjectDeps` still
  binds the per-request AuditWriter / report sink / emit hooks on the shared
  `plan_approve` + `report:publish` singletons. Documented in REVIEW.md's
  Fix Round for the next round.
- Pre-existing `-race` failure in `TestDataChannelFanOut*` from the test
  fixture `stubKPICaller.Call` (`fanout_tools_test.go:185`) — reproduced on
  base commit 9054cc4, unrelated to this round.

## Gates (after the fix round)

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./... -count=1` (WSL) — PASS: 40 packages ok, 0 FAIL
- `-race` on `./internal/agent/...` (excluding the pre-existing fixture race)
  and 3× `TestHandlerForSession*` — PASS
- `scripts/verify-phase-26.sh` — 3/3 structural gates PASS
- Platform floor — untouched: no commit in this round touches any
  `platform_floor*.go` (both remain byte-frozen at `174ee18`). Note: there is
  no zero-byte `platform_floor.go` in the tree — `internal/platformfloor/platform_floor.go`
  is 9331 bytes and `internal/agent/platform_floor.go` is 2768 bytes; the
  frozen invariant is the one the review verified (byte-unchanged).

---

_Fixed: 2026-09-07T05:20:20Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
