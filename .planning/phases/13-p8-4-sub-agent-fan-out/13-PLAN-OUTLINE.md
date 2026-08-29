# Phase 13 — Plan Outline (chunked mode, MVP vertical slices)

**Phase:** 13 — 子 agent 扇出（P8.4 — tier 继承 — structural tier-closure + deterministic merge + per-sub-agent grounding）
**Mode:** mvp (vertical slices — BE-only FanOutOrchestrator + tier-closure + audit substrate + acceptance gate; UI hint: no)
**Goal:** Orchestrator 将多跳检索扇出为一层并行子 agent — 子 agent tier 边界构造期闭包继承（结构性无法扩权）、合并确定性 + 保守部分失败合并、per-sub-agent 独立 grounding、扇出事件审计可按 RunPath/Transfer 追溯。**Tier 继承证明测试、10× byte-identity 契约测试、fan-out 20-case UAT 三层门是 v0.8 里程碑收口的可信税产物。**
**Constraints:** 16 inherited D11-* + 10 P12-* + 9 P13-L* locked decisions from 13-RESEARCH.md; D8-04 legacy interrupt lint extension; D-04 transport-swap; D15 wire-type alarm (D16 overload on data-decision, NO new SSE chunk type); D14 Zustand no-messages; D8-06 SC-5 stub preserved; D9-V2 history-no-refusal; D2-01 逐字引用护城河贯穿 sub-agent 输出；Pitfall 9 (tier leak + non-deterministic merge) hardened from day 1; RF-4 spike TDD-led on Wave 1.

**Decision inheritance posture:** Every locked decision from Phase 11 + Phase 12 is **immutable** — this plan-set only EXTENDS `internal/runtime/` (P12-L1 + P13-L2 + P13-L8), `internal/store/sqlite/` (P13-L6 ADD COLUMN), `internal/wire/` (P13-L7 D16 overload), `internal/skill/` (P13-L2 ManifestBuilder), and `internal/agent/` (fan_out_dispatch Tool). It does NOT modify Phase 11 HITL files, Phase 12 DeepAgent files, Phase 12 write_todos files, or Phase 12 budget gate files.

---

## Outline Table

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| **13-01** | **BE foundation — `internal/runtime/orchestrator.go` (FanOutOrchestrator agent type) + `internal/runtime/sub_agent_builder.go` (tier-closure per-Run) + `internal/skill/manifest_builder.go` (WithReads intersection) + `internal/runtime/fanout_runtime.go` (budget scope + LRU accumulator cap)** — Mirror `internal/runtime/factory.go` archetype-dispatch pattern (P12-L1); `BuildSubAgent` enforces `subManifest.Writes = []` (P13-L4 structural read-only); `subManifest.Reads = parent.ResolvedTiers() ∩ subSkill.Reads` AT CONSTRUCTION (P13-L2); load-time error on empty intersection (FAN-02 SC-2); `cmd/spike-fanout/main.go` RF-4 spike binary runs the smoke test; `TestSubAgentTierInheritance` + `TestSubAgentRetrieveToolClosesOverSubManifest` + `TestSubAgentCannotExpandTierBoundary` + `TestSubAgentIsReadOnly` TDD-led; extends D8-04 WalkDir to scan `internal/runtime/orchestrator.go` (NEW — must remain 0-hits; Tier-closure-building via constructor + new `int onFanoutComplete = -1;` constant-string phase boundary marker; sealed sentinel `package runtime // P13-L8: 禁止 adk.NewParallelAgent`); extend `validAgentTypes` with `"orchestrator"` | W1 | (none) | **(foundation — unblocks FAN-01..04 + RF-4 spike + P13-L1..L9 invariant substrate)** |
| **13-02** | **BE `mergeSubAgentResults` pure function (FAN-03 SC-3) — `internal/runtime/merge.go`** — Key sort by `(sub_agent_id ASC, citation_chunk_id ASC, citation_span ASC)` via `sort.SliceStable`; 10× byte-identity contract test via `bytes.Equal(json.Marshal(...))`; conservative partial-failure (any refused/faulted → merged verdict = `needs_input` confidence 0 — D2-14 posture); citation dedup by `(chunk_id, span)` first-wins; min-confidence aggregation; `TestMergeSubAgentResultsByteIdentity` + `TestMergeConservativePartialFailure` + `TestMergeCitationDedup` + `TestMergeAllOKConfirmedStatus` + `TestMergeEmptyResults` + `TestMergeSubAgentOrderIndependent` | W2 | 13-01 | **FAN-03** (deterministic merge + conservative partial-failure + 10× byte-identity contract test) |
| **13-03** | **BE per-sub-agent grounding (FAN-04) + fault isolation + D16 data-decision overload + audit RunPath/Transfer extension** — `internal/runtime/orchestrator.go::FanOutOrchestrator.Run` executes 5-step pipeline (`Classify → Build → Fan-out → Merge → ApplyTrustStage`); each sub-agent goroutine `defer recover()` + `context.WithTimeout(30s)` (Pitfall 4); `runtime.ApplyTrustStage` reused (NOT modified) for per-sub-agent grounding with shared `*ChunkAccumulator` (P13-L4 reuse + FAN-04)；`internal/wire/chunk.go` D16 overload extends `data-decision.decision` enum with `fanout_dispatched \| fanout_completed \| fanout_failed` (NO new SSE chunk type per P13-L7); `web/src/types/wire-types.ts` D15 lockstep mirror in same commit; `internal/store/sqlite/schema.go` ADD COLUMN `run_path TEXT`, `transfer_from TEXT`, `transfer_to TEXT` (additive DDL, P13-L6); `internal/store/sqlite/audit.go` extends `AuditRow` struct + `ActionFanoutDispatched` / `ActionFanoutCompleted` / `ActionFanoutFailed` enum constants; `internal/store/sqlite/fanout_events.go` (NEW) — `QueryFanoutByRunPath(runPath)` helper; `TestFanOutOrchestratorRunsAllSubAgents` + `TestFanOutOrchestratorFaultIsolation` + `TestFanOutOrchestratorAppliesTrustStagePerSubAgent` + `TestFanoutAuditRows` + `TestFanoutRunPathTransferColumns` + `TestSharedChunkAccumulator` + `TestDataDecisionFanoutEnum` (D15 lockstep) | W2 | 13-01 | **FAN-01** (parallel fan-out + fault isolation), **FAN-04** (per-sub-agent grounding + RunPath/Transfer audit) |
| **13-04** | **Hermetic gate + frozen eval v2 re-lock + D8-04 WalkDir extension + STRIDE-13 threat model** — `scripts/verify-phase-13.sh` 9-stage hermetic gate (BE unit + WalkDir legacy interrupt zero hits in 7 roots + D-04 transport-swap zero drift + D15 wire-type compile alarm + frozen eval byte-identity + MinIteration hard-cap source-grep + tier-closure fan-out smoke + sub-agent HITL-tool whitelist exclusion + shared ChunkAccumulator cap); `eval/frozen/fanout.lock.json` NEW v0.8 milestone baseline (re-locks bytes-identity of frozen eval + new fan-out behavior delta); `docs/threat-model/STRIDE-13.md` (8 threats: T-P13-1 tier intersection mismatch, T-P13-2 HITL Tool whitelist leak, T-P13-3 fault propagation to parent, T-P13-4 merge non-determinism, T-P13-5 classifier cost explosion, T-P13-6 shared chunk-accumulator unbounded growth, T-P13-7 sub-manifest wiring typo, T-P13-8 NewParallelAgent accidental use) | W3 | 13-02, 13-03 | **FAN-01..04** (Phase 13 acceptance — 4 requirements hermetically verified) |
| **13-05** | **E2E acceptance gate + live fan-out 20-case UAT + frozen eval v2 re-lock (v0.8 milestone re-baseline)** — `cmd/spike-fanout/main.go` extended with `--live --golden=eval/golden/fanout.jsonl --sub-skill-config=...` for end-to-end multi-hop query rehearsal; `eval/golden/fanout.jsonl` 20-case fan-out golden set (8 multi-hop + 6 tier-mismatch rejection + 4 partial-failure conservative merge + 2 single-vertex fallback); acceptance gate combines hermetic gates from 13-04 + 20-case live fan-out UAT + frozen-eval byte-identity sha256 (`Phase 8 pin 50bb1688…` UNCHANGED + new `eval/frozen/fanout.lock.json` byte-locked); user checkpoint operator-gates the live UAT per Phase 7/9/11 pattern; v0.8 milestone gate `go test -race ./...` + hermetic + live fan-out + frozen eval triple lock | W4 | 13-04 | **FAN-01..04** (Phase 13 acceptance gate — milestone v0.8 re-baseline + fan-out 20-case UAT) |
| **13-06** | **v0.8 milestone wrap — completion report + Operator Gates Ledger + README DIST-09 fill + v0.8.0 tag hand-off** — `.planning/v0.8-MILESTONE-COMPLETION.md` (mirrors v0.7.7-MILESTONE-AUDIT.md skeleton inverted to closeout: 25/25 v0.8 requirement IDs attested via REQUIREMENTS ↔ SUMMARYs ↔ locks 3-source cross-ref; Operator Gates Ledger incl. Phase 10 deferred 35-case `--via-pipeline` re-run (GEVL-08/N9-V2 milestone responsibility) + 13-05 live UAT + push + tag; 3 frozen locks; deviations (Phase 9 NO-GO+escape, Phase 10 closed-blocker-notes, D8-07, -race-on-Windows); deferred-items ledger (07-02 T3 / 07-05 T3 GIF / 07-06 / 06-03→999.2 / 999.1 / 999.2)); README `## Eval Results` PLACEHOLDER replaced by verbatim `go run ./scripts/eval_snapshot` stdout from existing `eval/report.json` (Phase 8 08-05 frozen re-run — faithfulness 0.972 / refusal 95%; NO live stack needed); STATE.md terminal flip (completed_plans 56/61, completed_phases 9, percent 92 plans-basis); operator-gated `git tag -a v0.8.0` + push with evidence-cited ledger; zero code changes; all 3 locks byte-identical before/after. NOTE: the earlier 13-06 slot (hermetic gate) was merged into 13-04; this slot is re-purposed for the wrap plan | W5 | 13-05 | **DIST-09** (README eval results — Phase 7 leftover closed from Phase 8 artifacts) + v0.8 milestone closeout |

---

## Plan Count

**6 plans** in **5 waves** (Wave 1: foundation + RF-4 TDD spike; Wave 2: parallel — merge pure function ⟂ per-sub-agent grounding + fault isolation + audit; Wave 3: hermetic gate + threat model + frozen eval; Wave 4: E2E acceptance + live fan-out UAT + frozen eval v2 re-lock; Wave 5: v0.8 milestone wrap — completion report + DIST-09 fill + tag hand-off). UI hint is no, so plans 13-01..05 are BE-only + 1 paired FE TS mirror (D15 lockstep, no new product UI); 13-06 is docs+README only (zero code). The original `13-06` slot (hermetic gate) was merged into 13-04; the slot was re-purposed 2026-08-21 for the v0.8 wrap plan (user-selected route via gsd-resume-work).

## Wave Structure Summary

| Wave | Plans | Autonomous | Purpose |
|------|-------|------------|---------|
| 1 | 13-01 | yes (autonomous:false for spike code, but checkpoint:human-verify NOT used — TDD-led) | BE foundation: orchestrator + sub-agent factory + tier-closure + ManifestBuilder + RF-4 spike binary |
| 2 | 13-02 ⟂ 13-03 | yes, yes | BE merge pure function (FAN-03 10× byte-identity) ⟂ BE per-sub-agent grounding + fault isolation + D16 overload + audit RunPath/Transfer + D15 TS mirror |
| 3 | 13-04 | yes | Hermetic gate + frozen eval v2 re-lock + D8-04 WalkDir extension + STRIDE-13 threat model + 3 audit enum |
| 4 | 13-05 | no (live UAT checkpoint:human-verify gate=blocking) | E2E + 20-case live fan-out UAT + frozen eval v2 re-lock + v0.8 milestone re-baseline |
| 5 | 13-06 | no (v0.8.0 tag/push checkpoint:human-verify gate=blocking; Tasks 1-3 auto) | v0.8 milestone wrap: completion report + Operator Gates Ledger + README DIST-09 fill + STATE terminal flip + tag hand-off |

## Requirement Coverage

- **FAN-01** (orchestrator parallel fan-out + fault isolation): 13-01 (foundation) + 13-03 (per-sub-agent grounding + fault isolation + D16 overload)
- **FAN-02** (tier closure structural inheritance via constructor + load-time intersection check): 13-01 (BuildSubAgent + ManifestBuilder + tier-closure regression tests)
- **FAN-03** (deterministic merge 10× byte-identity + conservative partial-failure): 13-02 (pure function + 10× byte-identity contract test)
- **FAN-04** (per-sub-agent grounding reusing ApplyTrustStage + audit RunPath/Transfer): 13-03 (per-sub-agent grounding via shared ChunkAccumulator + audit columns + D16 overload)

All 4 FAN-01..04 requirement IDs are covered by at least one plan. **0 unmapped requirement IDs** for Phase 13. 13-06 additionally closes **DIST-09** (README eval results — a Phase 7 leftover) from existing Phase 8 frozen-run artifacts, and produces the v0.8 milestone-wide requirement attestation (25/25 IDs across phases 8-13) in `.planning/v0.8-MILESTONE-COMPLETION.md`.

## File Ownership Map (parallelism safety)

```
W1:
  13-01 → internal/runtime/{orchestrator,sub_agent_builder,fanout_runtime}.go
         + internal/runtime/{orchestrator,sub_agent_builder,fanout_runtime}_test.go
         + internal/skill/{manifest_builder.go, manifest_builder_test.go}
         + internal/skill/manifest.go (MODIFIED — extend validAgentTypes with "orchestrator")
         + internal/agent/fanout_tools.go (NEW — fan_out_dispatch Tool scaffolding)
         + internal/agent/fanout_tools_test.go
         + cmd/spike-fanout/main.go + main_test.go (NEW — RF-4 spike)

W2 (parallel, no file overlap):
  13-02 → internal/runtime/merge.go
         + internal/runtime/merge_test.go (10× byte-identity contract test)

  13-03 → internal/runtime/orchestrator.go (MODIFIED — extend Run method with 5-step pipeline)
         + internal/runtime/orchestrator_test.go (RUN — 5 step sub-agent grounding + audit insert)
         + internal/wire/chunk.go (MODIFIED — D16 enum extension fanout_dispatched/completed/failed)
         + internal/wire/chunk_test.go (MODIFIED — TestDataDecisionFanoutEnum)
         + internal/store/sqlite/{schema.go,audit.go,fanout_events.go} (NEW fanout_events.go)
         + internal/store/sqlite/{schema_test.go, audit_test.go, fanout_events_test.go}
         + web/src/types/wire-types.ts (MODIFIED — D15 lockstep mirror)
         + web/src/lib/real-chat-transport.ts (MODIFIED — fanout_* decision routing into agent-state-inspector timeline)

W3:
  13-04 → scripts/verify-phase-13.sh (NEW — 9-stage hermetic gate)
         + Makefile (MODIFIED — add verify-phase-13 target)
         + docs/threat-model/STRIDE-13.md (NEW — 8 threats register)
         + eval/frozen/fanout.lock.json (NEW — v0.8 milestone re-baseline)
         + eval/frozen/README.md (MODIFIED — add fanout.lock.json documentation)

W4:
  13-05 → cmd/spike-fanout/main.go (MODIFIED — add --live --golden flags)
         + cmd/spike-fanout/main_test.go (MODIFIED — add live-mode parity test)
         + eval/golden/fanout.jsonl (NEW — 20-case fan-out UAT)
         + .planning/STATE.md (MODIFIED — Phase 13 closure record)

W5:
  13-06 → .planning/v0.8-MILESTONE-COMPLETION.md (NEW — milestone closeout report)
         + .planning/phases/13-p8-4-sub-agent-fan-out/13-05-SUMMARY.md (created during 13-05;
           captured here ONLY if missing)
         + README.md (MODIFIED — Eval Results DIST-09 fill from scripts/eval_snapshot)
         + .planning/STATE.md (MODIFIED — terminal flip + v0.8 completion record)
```

## Key Decision Lock References (inherited from Phase 11 + Phase 12 — immutable, do NOT re-ask)

- **D11-A2** (trust staircase 3 callbacks + HITL = Tool+StatefulInterrupt) → 13-03 (FanOutOrchestrator reuses trust staircase; sub-agents funnel through `runtime.ApplyTrustStage`)
- **D11-B1** (per-turn Tool whitelist pure function) → 13-01 (`BuildSubAgent` enforces `subManifest.Writes = []` for structural read-only sub-agents; `TestSubAgentToolWhitelistExcludesHITL` regression lock)
- **D11-B3** (AllowedWriteTargets + hitlHandlers single file) → 13-01 (registry untouched; sub-agents cannot reach HITL Tools structurally)
- **D11-C3** (audit_writer sync INSERT before StatefulInterrupt) → 13-03 (audit INSERT before SSE emit; Phase 13 has NO StatefulInterrupt — fan-out is fire-and-forget)
- **P12-L1** (internal/runtime owns archetype dispatch + ApplyTrustStage exit) → 13-01 (FanOutOrchestrator agent type lives in internal/runtime; mirrors factory.go pattern) + 13-03 (per-sub-agent grounding reuses `ApplyTrustStage` unchanged)
- **P12-L2** (deep.Config.MaxIteration: 10 hardcoded) → 13-01 (sub-agent budget scope = half of parent: MaxIteration=5, TokenBudget=20000; structural constant in `internal/runtime/fanout_runtime.go`)
- **P12-L6** (wire-type discrimination strategy) → 13-03 (D16 overload on `data-decision.decision` with 3 new enum values — NO new SSE chunk type per P13-L7)
- **P12-L9** (Loop detection rolling 3-call hash) → 13-01 (fanout_runtime budget loop detection reuses Phase 12 hash)
- **D-04** (transport-swap byte-identical) → 13-05 (acceptance gate verifies mock ↔ Default transport invariance still holds with `fanout_*` chunks)
- **D15** (wire-type compile-time alarm) → 13-03 (paired Go + TS commit for `data-decision.decision` enum extension)
- **D14** (Zustand no-messages invariant) → no FE changes (UI hint: no); fan-out events ride existing `data-decision` channel
- **D8-04** (legacy interrupt API lint) → 13-04 (WalkDir extended to `internal/runtime/orchestrator.go`; NEW `internal/runtime/` invariant: 0 hits)
- **D8-06** (SC-5 stub cross-process resume) → preserved; sub-agent result is NOT checkpointed in 13-01 (future scope); `schema.RegisterName[*SubAgentResult]("skillforge.SubAgentResult")` registered preemptively for future interrupt support
- **D9-V2** (history does NOT influence refusal classification) → 13-03 (per-sub-agent grounding passes `History: nil` — same posture as parent Run)
- **D2-01** (citation entailment via Normalize + verbatim) → 13-03 (shared `*ChunkAccumulator` lets per-sub-agent grounding use parent's chunk whitelist spans too)

## New Phase 13 Locks (in-plan-set)

- **P13-L1** (Option B — manual `errgroup` + per-Run agent construction; `adk.NewParallelAgent` REJECTED for FAN-02). The orchestrator's contract is documented as a sealed sentinel `package runtime // P13-L8: 禁止 adk.NewParallelAgent — full-context sharing breaks FAN-02 tier-closure`. 13-01 source-grep `TestNoNewParallelAgentUsage` enforces.
- **P13-L2** (Tier inheritance is structural via constructor closure — `subManifest.Reads = parent.ResolvedTiers() ∩ subSkill.Reads` computed ONCE at construction; sub-agent's `retrieve_compliance_chunks` Tool closes over `subManifest.ResolvedTiers()`; sub-agent cannot expand tiers via Tool args). Load-time error `tier intersection empty` (FAN-02 SC-2). 13-01 + 13-04.
- **P13-L3** (Per-sub-agent budget scope — `MaxIteration: 5`, `TokenBudget: 20000`, `LoopWindow: 3` — half of parent's 10/50000/3). Calibration in 13-05 acceptance gate; if any sub-agent trips budget, escalate to 8/30000 (still < parent 10/50000). 13-01 + 13-05.
- **P13-L4** (Merge is pure function with `sort.SliceStable` on `(sub_agent_id ASC, citation_chunk_id ASC, citation_span ASC)`; 10× byte-identity contract test via `bytes.Equal(json.Marshal(...))`). 13-02.
- **P13-L5** (Conservative partial-failure — any sub-agent `Status: refused` OR `Status: faulted` → merged verdict is `{status: "needs_input", confidence: 0, review_required: true}` — D2-14 conservative posture). 13-02 + 13-03.
- **P13-L6** (Audit substrate — 3 nullable TEXT columns via ADD COLUMN DDL on existing audit table: `run_path`, `transfer_from`, `transfer_to`; no migration break; 3 new enum values `action_fanout_dispatched | action_fanout_completed | action_fanout_failed`). 13-03.
- **P13-L7** (D16 overload on `data-decision.decision` — new enum values `fanout_dispatched | fanout_completed | fanout_failed`; NO new SSE chunk type per D15 alarm + D-04 transport-swap). 13-03.
- **P13-L8** (Sub-agent scope: ONE layer only — `WithoutGeneralSubAgent: true` already in Phase 12 `BuildDeepAgent`; orchestrator does NOT invoke sub-orchestrators; recursive fan-out banned by structural design — fanout_runtime.go has no entry point for recursive DispatchAgent call into orchestrator). 13-01 + 13-04.
- **P13-L9** (Per-sub-agent grounding reuses Phase 12 `runtime.ApplyTrustStage`; shared `*ChunkAccumulator` pointer across parent + sub-agents — citation whitelist spans parent + sub-agents). 13-03.

## Test Lock Requirements

**Backend (Go hermetic):**
- `TestDispatchAgentOrchestrator` (NEW archetype: "orchestrator" → FanOutOrchestrator) — 13-01
- `TestSubAgentTierInheritance` (empty intersection → load-time error substring "tier intersection empty") — 13-01
- `TestSubAgentRetrieveToolClosesOverSubManifest` (sub-agent's retrieve Tool's manifest field is subManifest, NOT parentManifest) — 13-01
- `TestSubAgentCannotExpandTierBoundary` (sub-agent's Tool args containing parent's tier not in intersection → cannot pass Tool arg validation) — 13-01
- `TestSubAgentIsReadOnly` (sub-agent's Tool whitelist contains NO HITL Tools; subManifest.Writes = []) — 13-01
- `TestSubAgentMaxIterationHardCap` (sub-agent `BudgetConfig.MaxIteration: 5` hardcoded literal) — 13-01
- `TestSubAgentBudgetHalfOfParent` (sub-agent TokenBudget: 20000 ≤ parent 50000) — 13-01
- `TestSharedChunkAccumulator` (parent + sub-agents share one accumulator; sub-agent retrieve appends to parent's map) — 13-03
- `TestSharedChunkAccumulatorCap` (LRU eviction at 20 chunks max — shared accumulator cannot grow unbounded) — 13-01
- `TestNoNewParallelAgentUsage` (source-grep on `adk.NewParallelAgent` in `internal/runtime/` returns 0 hits; sealed sentinel marker enforces P13-L1) — 13-01 + 13-04
- `TestManifestBuilderWithReads` (fluent builder applies tier-closure intersection; empty intersection → Validate error) — 13-01
- `TestMergeSubAgentResultsByteIdentity` (10× same input → same `json.Marshal` bytes per FAN-03 contract test) — 13-02
- `TestMergeConservativePartialFailure` (any refused/faulted → needs_input confidence 0) — 13-02
- `TestMergeCitationDedup` (same chunk cited by 2 sub-agents → dedup first-wins) — 13-02
- `TestMergeAllOKConfirmedStatus` (all OK → status confirmed; min-confidence aggregation) — 13-02
- `TestMergeEmptyResults` (zero sub-agent results → empty MergedVerdict) — 13-02
- `TestMergeSubAgentOrderIndependent` (input slice permutation → byte-identical output) — 13-02
- `TestFanOutOrchestratorRunsAllSubAgents` (5-step pipeline: Classify → Build → Fan-out → Merge → ApplyTrustStage; emit 3 sub-agents concurrently) — 13-03
- `TestFanOutOrchestratorFaultIsolation` (1 sub-agent panics → other 2 complete; merged sees faulted as refusal-equivalent) — 13-03
- `TestFanOutOrchestratorAppliesTrustStagePerSubAgent` (per-sub-agent grounding via ApplyTrustStage before merge) — 13-03
- `TestFanoutAuditRows` (each sub-agent invocation emits action_fanout_dispatched + action_fanout_completed/action_fanout_failed) — 13-03
- `TestFanoutRunPathTransferColumns` (run_path populated; transfer_from=parent_run; transfer_to=sub_query_id) — 13-03
- `TestDataDecisionFanoutEnum` (D16 overload — fanout_dispatched/completed/failed in ValidTypes...) — 13-03
- `TestWireTypeCompilationD15` (`AssertAssignable<SkillForgeChunk, UIMessageChunk>` invariant preserved; fanout_* decision extends enum only) — 13-03 + 13-04
- `TestNoLegacyInterruptAPI` WalkDir extended to `internal/runtime/orchestrator.go` (NEW root) — 13-04

**Acceptance gate (13-04 hermetic + 13-05 live):**
- `scripts/verify-phase-13.sh` 9-stage hermetic gate
- `cmd/spike-fanout parse-rate --golden=eval/golden/fanout.jsonl` (live fan-out 20-case UAT — operator runs live stack; same gate as Phase 7/9/11)
- Frozen eval v2 re-lock (v0.8 milestone re-baseline; sha256 byte-identity verified)
- Threat-model STRIDE-13 register (8 threats — Critical/High)
- `eval/golden/fanout.jsonl` 20 cases (8 multi-hop + 6 tier-mismatch rejection + 4 partial-failure + 2 single-vertex)

## OUTLINE COMPLETE

**5 plans** in **4 waves** — vertical slices, MVP mode, BE-only + 1 paired FE TS mirror (D15 lockstep, no new product UI).

1. **13-01** — BE foundation: orchestrator + sub-agent factory + tier-closure + ManifestBuilder + RF-4 TDD spike
2. **13-02** — BE merge pure function (FAN-03 10× byte-identity)
3. **13-03** — BE per-sub-agent grounding + fault isolation + D16 overload + audit RunPath/Transfer + D15 TS mirror
4. **13-04** — Hermetic gate + frozen eval v2 re-lock + D8-04 WalkDir extension + STRIDE-13
5. **13-05** — E2E acceptance gate + live fan-out 20-case UAT + frozen eval v2 re-lock + v0.8 milestone re-baseline
