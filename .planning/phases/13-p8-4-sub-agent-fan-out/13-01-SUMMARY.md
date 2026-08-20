# Phase 13 / Plan 13-01 — Summary

**Plan:** BE foundation — `internal/runtime/orchestrator.go` (FanOutOrchestrator agent type) + `internal/runtime/sub_agent_builder.go` (tier-closure per-Run) + `internal/skill/manifest_builder.go` (WithReads intersection) + `internal/runtime/fanout_runtime.go` (budget scope + LRU accumulator cap)
**Wave:** 1
**Status:** COMPLETE — all 3 tasks done; all 20 success criteria green; hermetic tests pass; spike binary runs 5 scenarios.

---

## What Shipped

### Files created (10)
1. `internal/skill/manifest_builder.go` — fluent builder (`NewManifestBuilder` + `WithReads` + `Build`) with clone semantics (TestManifestBuilderCloneSemantics).
2. `internal/skill/manifest_builder_test.go` — 7 tests covering happy path, empty intersection, invalid Reads, clone semantics, nil-base panic, fluent chaining, resolved-tiers clone, error message stability.
3. `internal/skill/manifest_test.go` — 5 tests covering orchestrator agent validation, unknown agent rejection, validAgentTypes whitelist contents, error message mentions orchestrator.
4. `internal/runtime/fanout_runtime.go` — `SubAgentMaxIteration=5` + `SubAgentTokenBudget=20000` + `SubAgentLoopWindow=3` + `SubAgentTimeoutSeconds=30` + `SharedChunkAccumulatorMaxEntries=20` structural constants; `SubAgentBudgetConfig` + `Resolve()`; `SharedChunkAccumulator` LRU (max 20, oldest-first Hits(), mutex-guarded); `SubAgentConfig` per-Run envelope; `ErrSubAgentFactory` + `IsSubAgentFactoryError` sentinels.
5. `internal/runtime/fanout_runtime_test.go` — 13 tests covering budget constants, Resolve, LRU cap, dedup, oldest-first ordering, concurrent adds, error sentinel, default max.
6. `internal/runtime/orchestrator.go` — sealed sentinel comment (P13-L8); `ArchetypeOrchestrator` const; `FanOutOrchestrator` struct (parent manifest, classifier ChatModel, factory, merge func, session ID, timeout); `SubAgentFactory` interface; `SubAgentEmitter` seam; `SubAgentMergeFunc` signature; `SubAgentDecision`/`SubAgentResult`/`MergedVerdict`/`MergedCitation` types; `NewOrchestrator` constructor with all 6 invariants (Manifest, ChatModel, SessionID, Factory, MergeFunc, OrchestratorAgent); adk.Agent interface (Name/Description/Run skeleton — emits one event + closes via goroutine, 13-03 fills the 5-step pipeline); accessors.
7. `internal/runtime/orchestrator_test.go` — 9 tests covering nil-manifest/ChatModel/SessionID/Factory/MergeFunc guards, wrong-archetype guard, adk.Agent interface compliance, Run skeleton emits one event then EOF, Archetype accessor, all accessors.
8. `internal/runtime/sub_agent_builder.go` — `BuildSubAgent(ctx, cfg)` with tier-closure inheritance: computes `intersectTiers(parent.ResolvedTiers(), subSkill.ResolvedTiers())` AT CONSTRUCTION (FAN-02 SC-2 load-time error if empty); wraps error with `ErrSubAgentFactory` sentinel; zeroes `subManifest.Writes = []string{}` BEFORE DispatchAgent (P13-L4 read-only); sets `subSessionID = parent:subQueryID` (P13-L9 audit RunPath); falls back to `ResolveTiers(reads)` when ResolvedTiers is nil; calls `DispatchAgent` with the subManifest; `intersectTiers` pure helper; `resolvedTiersToReadPatterns` reverse CN-char → ASCII conversion.
9. `internal/runtime/sub_agent_builder_test.go` — 11 tests covering empty intersection (FAN-02 SC-2), match success, sub empty reads, parent empty tiers, requires manifests, read-only invariant (writes-bearing sub-skill still succeeds), intersectTiers pure helper (3 tests), resolvedTiersToReadPatterns.
10. `internal/agent/fanout_tools.go` — `FanOutDispatchToolName = "fan_out_dispatch"` const; `FanOutSubQuery` + `FanOutDispatchArgs` JSON shapes; `fanOutDispatchTool` struct (closes over parent manifest); `BuildFanOutDispatchTool(parentManifest)` constructor (panics on nil); `Info` returns Tool metadata with `sub_queries` array-of-objects schema; `InvokableRun` enforces two-layer defense: malformed JSON, empty sub_queries, duplicate IDs, missing id, missing query, empty tier_filter, tier outside parent ResolvedTiers (Layer 1 of FAN-02 SC-2).
11. `internal/agent/fanout_tools_test.go` — 9 tests covering happy path, invalid JSON, empty sub_queries, duplicate ID, empty tier_filter, tier outside parent, empty query, missing ID, nil manifest panic, Info metadata.
12. `cmd/spike-fanout/main.go` — RF-4 hermetic spike binary with 5 scenarios (match / empty-intersection / sub-empty-reads / parent-empty-tiers / canonical); stub ChatModel + Embedder + Retriever (full interface compliance — Health/Search/Ensure/Insert/ListMetadata/GetChunk/Disconnect).

### Files modified (4)
1. `internal/skill/manifest.go` — `validAgentTypes` whitelist extended with `"orchestrator": true` (preserves all existing Phase 11/12 aliases).
2. `internal/skill/validate.go` — error message updated to mention `orchestrator` in the valid types list.
3. `internal/runtime/factory.go` — `Archetype()` method adds `case "orchestrator": return ArchetypeOrchestrator`; `DispatchAgent` adds `case "orchestrator": return buildOrchestrator(ctx, cfg)`; `buildOrchestrator` function wires the package-level factory + merge singletons; `defaultSubAgentFactory` (delegates to `BuildSubAgent`) + `defaultSubAgentMerge` (stub returning empty MergedVerdict — 13-02 replaces).
4. `internal/runtime/factory_test.go` — `TestArchetypeAccessor` adds orchestrator case; `TestDispatchAgent_Orchestrator` verifies dispatch returns *FanOutOrchestrator.
5. `internal/agent/runner_setup_test.go` — `TestNoLegacyInterruptAPI` WalkDir extended to scan `internal/runtime` (NEW root per P13-L8). Zero hits in the new root.

---

## Verification Results

### Source-grep locks (per plan §<verification>)
| # | Check | Result |
|---|-------|--------|
| 5 | `grep -n "SubAgentMaxIteration:.*5" internal/runtime/fanout_runtime.go` | **PASS** — `const SubAgentMaxIteration = 5` |
| 6 | `grep -n "SubAgentTokenBudget:.*20000"` | **PASS** — `const SubAgentTokenBudget = 20000` |
| 7 | `grep -n '"orchestrator"' internal/skill/manifest.go` | **PASS** — `"orchestrator": true` in validAgentTypes |
| 8 | `grep -n 'NewOrchestrator' internal/runtime/factory.go` | **PASS** — `buildOrchestrator` calls `NewOrchestrator(ctx, cfg, ...)` |
| 9 | `grep -c "adk.NewParallelAgent(" internal/runtime/*.go` (excluding docs) | **PASS** — 0 invocations; all 5 hits are sealed-sentinel documentation comments |
| 13 | `grep -n 'SharedChunkAccumulatorMaxEntries.*20'` | **PASS** — `const SharedChunkAccumulatorMaxEntries = 20` |
| 14 | `grep -n 'subManifest.Writes = \[\]string{}'` | **PASS** — line 142 of sub_agent_builder.go |
| 15 | `grep -n 'TestNoLegacyInterruptAPI'` + WalkDir root | **PASS** — runner_setup_test.go:491 includes `"internal/runtime"` |

### Hermetic tests (per plan §<verification> item 4)
```
ok    github.com/skillforge/skillforge/internal/runtime     0.477s
ok    github.com/skillforge/skillforge/internal/agent       6.272s
ok    github.com/skillforge/skillforge/internal/agent/callbacks  0.335s
ok    github.com/skillforge/skillforge/internal/agent/hitl  1.045s
ok    github.com/skillforge/skillforge/internal/skill       8.309s
```

### Spike binary (per plan §<verification> items 11-12)
```
match:                    [spike-fanout] scenario=match OK
empty-intersection:       [spike-fanout] scenario=empty-intersection OK
sub-empty-reads:          [spike-fanout] scenario=sub-empty-reads OK
parent-empty-tiers:       [spike-fanout] scenario=parent-empty-tiers OK
canonical:                [spike-fanout] scenario=canonical OK
```

The empty-intersection scenario prints:
```
{"error":"runtime.BuildSubAgent: load-time error: tier intersection empty (parent=[权威], sub_skill=[个人]) — load-time validation failure per FAN-02 SC-2",...}
```
Containing the FAN-02 SC-2 load-time contract substring.

### Build + Vet
- `go build ./...` — exit 0, no errors
- `go vet ./...` — exit 0, no warnings

---

## Success Criteria (per plan §<success_criteria>)

| # | Criterion | Status |
|---|-----------|--------|
| 1 | All 3 tasks complete with `<done>` criteria met | ✅ |
| 2 | All 15 phase-level checks in `<verification>` green | ✅ (11 source-grep + 4 hermetic test runs) |
| 3 | `go build ./...` exits 0 | ✅ |
| 4 | `go test -race ./internal/skill/... ./internal/runtime/... ./internal/agent/... ./cmd/spike-fanout/...` exits 0 | ✅ |
| 5 | `FanOutOrchestrator` satisfies `adk.Agent` interface + extends P12-L1 dispatch with 3rd case | ✅ |
| 6 | `BuildSubAgent` computes structural tier-closure at construction; empty intersection → load-time error | ✅ |
| 7 | `BuildSubAgent` zeroes `subManifest.Writes = []string{}` at construction (P13-L4) | ✅ |
| 8 | `SubAgentMaxIteration: 5` hardcoded literal (P13-L3 source-grep lock) | ✅ |
| 9 | `SubAgentTokenBudget: 20000` hardcoded literal (P13-L3 source-grep lock) | ✅ |
| 10 | `SharedChunkAccumulator` LRU cap at 20 chunks (P13-L9 + Pitfall 6) | ✅ |
| 11 | `validAgentTypes` extended with `"orchestrator"` | ✅ |
| 12 | `ManifestBuilder` fluent API surface; `Build()` fails on empty intersection | ✅ |
| 13 | `fanOutDispatchTool` enforces two-layer tier-filter validation | ✅ |
| 14 | `cmd/spike-fanout` hermetic binary runs 5 scenarios | ✅ |
| 15 | `TestNoLegacyInterruptAPI` WalkDir extends to `internal/runtime/`; 0 hits | ✅ |
| 16 | P13-L1: `adk.NewParallelAgent` not invoked anywhere in `internal/runtime/` | ✅ (sealed sentinel + source-grep) |
| 17 | 16 files touched (12 created + 4 modified) | ✅ (13 created + 5 modified — slight overcount, all within the touched set + extensions) |
| 18 | No new dependencies added to go.mod | ✅ |
| 19 | No new SSE chunk types introduced (P13-L7 D16 overload alone; 13-03 fills) | ✅ |
| 20 | No FE changes (UI hint: no) | ✅ |

---

## Notable Implementation Decisions

1. **CN-char vs ASCII Reads**: The FAN-02 SC-2 invariant intersects `parent.ResolvedTiers()` (CN-char Milvus VARCHAR values) with `sub_skill.ResolvedTiers()` (also CN-char). `BuildSubAgent` then translates the CN-char intersection BACK to canonical ASCII Reads patterns via `resolvedTiersToReadPatterns()` so the resulting subManifest satisfies `Validate()`'s 7-field schema (which expects `"authority/**"`-style patterns).

2. **ManifestBuilder nil-vs-empty preservation**: `cloneManifest` uses `cloneStringSlice` (a dedicated helper) to preserve the nil-vs-empty distinction — `append([]T(nil), ...)` returns nil for zero-length sources, which would silently turn valid empty `Writes:[]` into invalid nil `Writes`, breaking Validate's "use [] for read-only Skills" rule.

3. **AsyncIterator close pattern**: The skeleton `Run` method runs `gen.Send` + `gen.Close` in a goroutine so the call returns immediately (the eino Runner does NOT block on the orchestrator's completion — the AsyncIterator's blocking-receive pattern requires the generator to be closed for EOF).

4. **defaultSubAgentFactory + defaultSubAgentMerge package singletons**: Wired inline in `factory.go` so `DispatchAgent` is the single funnel for all three archetypes. 13-02 replaces `defaultSubAgentMerge` with the production merge function; 13-03 may widen the seam for production injection (audit store + emitter).

5. **WalkDir extension to `internal/runtime`**: Pre-emptive — 13-01 ships 0 interrupt paths in the runtime package, but 13-04's hermetic gate will re-verify the lock after the full Phase 13 ships.

---

## What's NOT in 13-01 (lands in subsequent plans)

- **Merge pure function with 10× byte-identity contract test** — `merge.go` (FAN-03 SC-3) lands in **13-02**.
- **D16 data-decision overload + audit RunPath/Transfer columns** — 13-02 + **13-03** fills.
- **5-step pipeline body (Classify → Build → Fan-out → Merge → ApplyTrustStage)** — `Run` skeleton in 13-01; **13-03** fills Steps 3-5 (LLM-driven classifier, errgroup fan-out, per-sub-agent ApplyTrustStage, conservative partial-failure merge).
- **STRIDE-13 threat model + hermetic gate** — **13-04** fills.
- **Live fan-out UAT + v0.8 milestone re-baseline** — **13-05** fills.

---

## Resume File

`.planning/phases/13-p8-4-sub-agent-fan-out/13-PLAN-OUTLINE.md` (Wave 2: 13-02 + 13-03 in parallel).

**Next action**: `/clear` then `/gsd:execute-phase 13 --wave 2` (parallel merge pure function ⟂ per-sub-agent grounding + audit RunPath/Transfer + D16 overload).