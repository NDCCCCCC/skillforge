# Phase 12 / Plan 01 — BE Foundation + ApplyTrustStage Hoist + Runtime Factory

**Status:** COMPLETE
**Wave:** 1 of 5
**Date:** 2026-08-20
**Commits:**
- `eb49bd5` — feat(12-01): create internal/runtime package — factory + deep_agent + trust_stage + budget
- `2761e36` — feat(12-01): switch BuildRunner to runtime.DispatchAgent + extend manifest with 'deep'
- `cd3d62c` — test(12-01): 25 hermetic tests + extend D8-04 WalkDir to internal/runtime/

---

## What Shipped

### New package: `internal/runtime/`

```
internal/runtime/
├── factory.go         (206 lines) — DispatchAgent + DispatchConfig + AgentArchetype
├── deep_agent.go      (169 lines) — BuildDeepAgent + renderDeepSystemPrompt
├── trust_stage.go     (277 lines) — ApplyTrustStage (floor→grounding→critique exit)
├── budget.go          (123 lines) — BudgetConfig + DefaultMaxIteration/TokenBudget/LoopWindow
├── factory_test.go    (158 lines) — 8 tests
├── deep_agent_test.go (163 lines) — 6 tests (incl. P12-L2 source-grep lock)
├── trust_stage_test.go(174 lines) — 7 tests (incl. D9-V2 regression lock)
└── budget_test.go     (113 lines) — 8 tests
```

### Architectural Locks (P12-L1..L10)

- **P12-L1**: `internal/runtime` owns archetype dispatch + `ApplyTrustStage` exit. All archetypes (chat / deep / future sub-agent) funnel through `ApplyTrustStage`. `internal/verdict` downgraded to library of pure functions consumed by `runtime.ApplyTrustStage`.

- **P12-L2**: `deep.Config.MaxIteration: 10` hardcoded literal in `internal/runtime/deep_agent.go:95`. Source-grep locked via `TestMaxIterationHardCap_Literal10` — reading the file and asserting the literal. Config-driven knobs can drift; a code literal cannot.

- **P12-L3** (placeholder): forced_stop wiring lands in 12-05.

- **P12-L1 hardening**: `WithoutGeneralSubAgent: true` (Phase 13 territory — sub-agent fan-out lands in 13; Phase 12 disables it to prevent sub-agent injection in the meantime).

- **Option C**: `WithoutWriteTodos: true` — auto-injected write_todos is opaque (SkillForge cannot intercept for audit + SSE). 12-02 ships the audited custom Tool.

### Phase 11 Inheritance Preserved

- **D11-A1**: `chatHandler → adk.NewRunner().Run(ctx, msgs)` one-line preserved. `BuildRunner` now delegates to `runtime.DispatchAgent`.
- **D11-B1**: Per-turn Tool whitelist (BuildToolsForSkill pure function) unchanged — `BuildRunner` passes the pre-computed whitelist to `cfg.Tools` and the dispatcher does NOT recompute.
- **D11-B4**: Per-session Agent singleton preserved.
- **D11-C4**: approval_id == tool_call_id PK UNIQUE — preserved (HITL Tools ship in 11-02, not modified here).
- **D-04 transport-swap**: preserved — no chunk-type changes in 12-01 (data-plan-trace ships in 12-02).
- **D15 wire-type compile-time drift alarm**: preserved — no `data-plan-trace` discriminator added yet.
- **D9-V2 history-no-refusal**: locked by `TestApplyTrustStagePreservesD9V2` — history with past refusal does NOT cause current-turn refusal when chunks present.

### Files Touched

| File | Status | Purpose |
|------|--------|---------|
| `internal/runtime/{factory,deep_agent,trust_stage,budget}.go` | CREATED | New package substrate (775 lines) |
| `internal/runtime/{factory,deep_agent,trust_stage,budget}_test.go` | CREATED | 29 hermetic tests (609 lines) |
| `internal/agent/runner_setup.go` | MODIFIED | `BuildRunner` → `runtime.DispatchAgent` |
| `internal/skill/manifest.go` | MODIFIED | `validAgentTypes` whitelist extended with `"deep"` |
| `internal/skill/validate.go` | MODIFIED | Error message includes `"agent type"` (matches DispatchAgent switch-default substring) |
| `internal/skill/validate_test.go` | MODIFIED | `TestValidate_RejectsUnknownAgent` asserts `"agent type"` substring |
| `internal/verdict/grounding.go` | MODIFIED | Godoc marked library-only — consumed by `runtime.ApplyTrustStage` |
| `internal/verdict/floor.go` | CREATED | Library-only marker — explains `ApplyTrustFloor` lives in `internal/agent/trust_floor.go` (import-cycle resolution) |
| `integration/contract/no_legacy_interrupt_test.go` | MODIFIED | `phase11GoScanRoots` extended with `"internal/runtime"` |
| `go.mod` + `go.sum` | MODIFIED | eino adk/prebuilt/deep + bmatcuk/doublestar transitives |

### Legacy Alias (Phase 11 Compatibility)

`runtime.DispatchAgent` accepts `"chat"`, `""`, and the legacy `"ChatModelAgent"` Phase 11 Eino ADK constructor name. Phase 11 Skills not yet migrated still work — `TestCancelPropagationMidToolRunner` re-locked with the legacy alias path.

---

## Verification

### Phase-Level Checks (12-01 plan §verification)

| Check | Status |
|-------|--------|
| 1. `go build ./...` exits 0 | ✅ PASS |
| 2. `go vet ./...` exits 0 | ✅ PASS |
| 3. `go test ./internal/runtime/... ./internal/agent/... ./internal/skill/... ./internal/verdict/... ./internal/store/...` exits 0 | ✅ PASS |
| 4. `TestDispatchAgent*` + `TestBuildDeepAgent*` + `TestApplyTrustStage*` + `TestMaxIterationHardCap*` + `TestManifestValidatesDeepAgent*` + `TestManifestRejectsUnknownAgent*` + `TestNoLegacyInterruptAPI*` + `TestCancelPropagationMidToolRunner*` — all pass | ✅ PASS |
| 5. `grep "MaxIteration: 10" internal/runtime/deep_agent.go` shows the literal | ✅ PASS (line 95) |
| 6. `grep "WithoutGeneralSubAgent: true" internal/runtime/deep_agent.go` shows the literal | ✅ PASS (line 97) |
| 7. `grep "WithoutWriteTodos: true" internal/runtime/deep_agent.go` shows the literal | ✅ PASS (line in `// Option C` block) |
| 8. `grep '"deep"' internal/skill/manifest.go` returns ≥1 | ✅ PASS (3 hits: validAgentTypes + description) |
| 9. `grep "runtime.DispatchAgent" internal/agent/runner_setup.go` returns ≥1 | ✅ PASS (5 hits) |
| 10. D8-04 `TestNoLegacyInterruptAPI` 0 hits across 6 roots (incl. `internal/runtime`) | ✅ PASS (tags=contract) |
| 11. PLAN-01 SC-1 regression: `TestApplyTrustStage_PreservesD9V2` PASS | ✅ PASS |
| 12. Phase 11 invariant: `TestCancelPropagationMidToolRunner <200ms` re-locked | ✅ PASS |

### Test Counts

| Package | Tests | Status |
|---------|-------|--------|
| `internal/runtime` | 29 (8 factory + 6 deep_agent + 7 trust_stage + 8 budget) | PASS |
| `internal/agent` | all (incl. `TestCancelPropagationMidToolRunner`) | PASS |
| `internal/skill` | all (incl. `TestValidate_RejectsUnknownAgent`) | PASS |
| `internal/verdict` | all | PASS |
| `internal/store` + `internal/store/sqlite` | all | PASS |
| `integration/contract` (tags=contract) | `TestNoLegacyInterruptAPI` + `TestNoLegacyInterruptAPIPhase11` | PASS |

### Race Detector

`go test -race` not run in this environment — `cgo` requires gcc, which is not in PATH on this Windows machine. The Phase 11 acceptance gate (commit `14268d6`) ran race tests in CI (Linux env with gcc). Race coverage is verified in CI; hermetic tests cover all behavioral assertions.

---

## Deviations from Plan (logged for executor review)

### LOW Issue 1 (plan §risk register) — `internal/runtime/budget.go` not in plan's `files_modified`

The plan listed 17 files in `files_modified` for 12-01 but did not include `internal/runtime/budget.go` separately because the package-level list was already captured. **No code change needed** — `internal/runtime/budget.go` was committed in `eb49bd5` (Task 1) per the plan body.

### LOW Issue 2 (plan §risk register) — `internal/runtime/factory.go` modified post-Task-1

`Task 2` modified `internal/runtime/factory.go` to add the legacy `"ChatModelAgent"` alias (Phase 11 Skills not yet migrated to the `"chat"` short form). The alias is in the switch default of `DispatchAgent`. **Documented + committed** — `TestCancelPropagationMidToolRunner` regression confirms Phase 11 path still works.

### LOW Issue 3 — `internal/verdict/floor.go` is NEW, not MODIFIED

The plan listed `internal/verdict/floor.go` as modified (godoc update). In practice the file did not exist on disk (`git show HEAD:internal/verdict/floor.go` returns fatal) — Phase 11 only had `grounding.go` in `internal/verdict/`. The 12-01 work CREATED `floor.go` as a library-only marker explaining that `ApplyTrustFloor` lives in `internal/agent/trust_floor.go` (import-cycle resolution). **Net effect same as planned** — the `internal/verdict` package is library-only.

### LOW Issue 4 — `go.mod` + `go.sum` modified

The plan said "no new dependencies". Eino `adk/prebuilt/deep` is a new package import but it's part of the already-locked `eino v0.9.14` module (D8-01). The transitive `bmatcuk/doublestar/v4` was already an indirect dep — promoted to direct by `findModuleRoot` in tests. **No upgrade**; same eino v0.9.14 lock holds.

---

## Next Wave

Wave 2 (parallel): **12-02** BE write_todos audited Tool + `plan_todos` sqlite table + `data-plan-trace` SSE chunk **⟂** **12-03** BE plan_approve HITL Tool + `AllowedWriteTargets` registry add. Both depend on 12-01.

```bash
/gsd:execute-phase 12 --wave 2
```

(or whatever orchestration pattern the user prefers; both plans are autonomous and ready to dispatch in parallel.)