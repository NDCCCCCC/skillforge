# Phase 12 — Plan Outline (chunked mode, MVP vertical slices)

**Phase:** 12 — 规划层（P8.3 — DeepAgent + write_todos + 可回放）
**Mode:** mvp (vertical slices — UI/API/DB end-to-end, NOT horizontal layers)
**Goal:** Skill 声明 `agent: deep` 即获得 DeepAgent 规划能力——实时计划清单流、执行前计划确认门、三层预算门防成本爆炸、规划轨迹仅凭审计行即可完整回放；`ApplyTrustStage` 提升为所有 archetype 统一出口
**Constraints:** 16 inherited D11-* decisions + D8-04 legacy interrupt lint + D-04 transport-swap + D14 Zustand no-messages + D15 wire-type alarm + D8-06 SC-5 stub + D16 data-decision overload + D9-V2 history-no-refusal + Pitfall 8 cost explosion + RF-5 verdict-parse-rate (live spike gated)

---

## Outline Table

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| **12-01** | **BE foundation + ApplyTrustStage hoisting + `internal/runtime` factory + MaxIteration cap + Manifest.Agent dispatch** — new package `internal/runtime/` with `factory.go::dispatchAgent(manifest)` mapping `chat`→Phase 11 ChatModelAgent + `deep`→`deep.New`; `trust_stage.go::ApplyTrustStage` (floor→grounding→critique) hoisted from `internal/verdict` as the unified exit point; `budget.go` callbacks wrap deep.Config.MaxIteration ≤10 + run-level token counter + rolling-3-hash loop detector; `internal/skill/manifest.go` adds `"deep"` to `validAgentTypes` whitelist (no new enforcement); `internal/agent/runner_setup.go` calls `runtime.dispatchAgent()` instead of `adk.NewChatModelAgent` directly | W1 | (none) | **(foundation — no direct requirement IDs; unblocks PLAN-01..05 implementation; D11-A2 trust staircase reuse + D8-04 lint extension + D8-06 SC-5 stub integrity)** |
| **12-02** | **BE write_todos audited Tool + `plan_todos` sqlite table + `data-plan-trace` SSE chunk + audit_writer extension** — `internal/agent/deep_audit.go::writeTodosAuditedTool` mirrors `hitl_memory_facts.go` (Option C — `WithoutWriteTodos=true` + custom Tool, audit + SSE + session-value); `internal/store/sqlite/plan_todos.go` INSERT-only audit table with PK UNIQUE on `(run_id, todo_id)`; `internal/wire/chunk.go` adds `data-plan-trace` discriminator + `PlanTracePayload` struct + `ValidTypes["data-plan-trace"]=true`; `web/src/types/wire-types.ts` mirrors (D15 lockstep); FE side consumed by 12-06; `TestWriteTodosAuditFailClosed` + `TestPlanTodosReplayReconstruction` + `TestDataPlanTraceWireType` | W2 | 12-01 | **PLAN-02** (live todo streaming + cross-refresh persistence — BE half), **PLAN-05** (audit-table replay substrate — BE half) |
| **12-03** | **BE plan approval HITL Tool + `AllowedWriteTargets` registry add + audit_writer extension** — `internal/agent/hitl/hitl_plan_approve.go` mirrors `hitl_review_report.go` (D11-A2 + D11-B2 + D11-C3 + D11-C4 verbatim) with target=`"plan_approve"`; `internal/agent/hitl/registry.go` adds `"plan_approve"` to `AllowedWriteTargets` (D11-B3 single-line) + `hitlHandlers["plan_approve"] = &planApproveTool{}`; audit row `target="plan_approve"` + `action_pending_approval` schema unchanged (reuse frozen_args = full plan JSON); deny semantics = `forced_stop` data-decision (D11-B2 reuse); D8-04 WalkDir extends to `internal/runtime/` + `internal/agent/plan/`; `TestPlanApproveFirstPassInterrupts` + `TestPlanApproveDenyForcesRewrite` + `TestPlanApproveRegistryAllowedTargetsContains` | W2 | 12-01 | **PLAN-03** (plan confirmation gate — BE half: tool interrupt + audit row) |
| **12-04** | **BE plan API endpoints (`GET /api/run/:id/plan`, `POST /api/run/:id/plan/approve`) + RF-5 verdict parse rate spike + structured-output final step** — `internal/server/plan_api.go` GET returns the plan_todos rows for a run (PLAN-05 BE part); POST approve endpoint mirrors `approval_api.go` with target-specific validation; `internal/llm/extract.go` adds `ExtractVerdictJSONWithStructuredOutput` (renders Phase 2 systemPrompt verbatim + `response_format=json_object`); `cmd/spike-deep/main.go` spike binary runs 50-case golden set against DeepAgent to measure verdict-JSON parse rate (RF-5); `TestPlanAPIReplayEndpoint` + `TestExtractVerdictJSONStructuredOutput` + `TestSpikeDeepVerdictParseRateMeasurement` (skipped if no `.env` — operator gate) | W3 | 12-02, 12-03 | **PLAN-05** (BE: replay API + RF-5 spike measurement) |
| **12-05** | **BE budget gate callbacks (MaxIteration + Token + Loop) + `forced_stop` emit + audit `budget_triggered` column + D16 data-decision overload** — `internal/agent/callbacks/budget.go` (Pitfall 8 hardened from day 1): wraps `deep.Config.MaxIteration: 10` (hardcoded, not config); `BeforeModelCallback` enforces token budget via atomic.Int64 accumulator; rolling 3-call fnv64a hash loop detector; on trigger → emit `data-decision{decision:"forced_stop", reason:"PLAN-04 budget exhausted"}` (D16 overload — `data-decision` discriminator stays, `decision` string enum extends); audit row `action_budget_triggered` with column `budget_layer: max_iter|token|loop`; `TestBudgetMaxIterationCap` + `TestBudgetTokenCounterAccumulation` + `TestBudgetLoopDetection3InRow` + `TestBudgetForcedStopEmitsDataDecision` + `TestBudgetAuditRowOnTrigger` | W3 | 12-01 | **PLAN-04** (three budget gates + forced_stop + audit) |
| **12-06** | **FE PlanTracePart streaming component + wire-types D15 mirror + CenterPanel integration** — `web/src/components/PlanTracePart.tsx` new live todo list renderer (mirrors `DataSubprogressPart.tsx` pattern); synthetic `tool-input-*` + `tool-output-available` chunks route through useChat (Pitfall 6 — tool-call lifecycle is AI SDK v5 native live-rendering surface); `web/src/types/wire-types.ts` adds `data-plan-trace` discriminator (D15 lockstep — paired commit with 12-02 BE); `web/src/stores/app-store.ts` adds `plan` slice (NOT in messages — D14); `RightChat.tsx` integrates PlanTracePart between user message + answer text; 8 vitest tests per UI-SPEC §Test Lock Requirements | W4 | 12-02 | **PLAN-02** (UI side: live todo stream renderer + cross-refresh persistence) |
| **12-07** | **FE PlanApprovalCard + PlanReplayView 轨迹回放视图 + LeftSidebar 5th section + CenterPanel tab** — `web/src/components/PlanApprovalCard.tsx` mirrors `ApprovalCard.tsx` (D11-D3 scope radio conditional on 批准并记住; deny requires ≥3 chars reason); `web/src/components/PlanReplayView.tsx` 规划轨迹回放视图 — fetches `GET /api/run/:id/plan` + renders todo timeline with status transitions (PLAN-05 FE half); `web/src/components/LeftSidebar.tsx` adds 6th section `PlanRulesPanel.tsx` (mirrors ApprovalRulesPanel from 11-07); `CenterPanel.tsx` adds new tab `规划轨迹` between `审计时间线` + `引用原文`; `web/src/lib/real-chat-transport.ts` parses `data-plan-trace` + `data-decision{decision:"forced_stop|plan-approved|plan-denied"}` extension; 12 vitest tests | W4 | 12-04, 12-05 | **PLAN-03** (UI side: plan confirmation card), **PLAN-05** (UI side: 轨迹回放视图) |
| **12-08** | **E2E acceptance gate + live spike re-run (RF-5) + frozen eval v2 re-lock + STRIDE-12 threat model** — `scripts/verify-phase-12.sh` 11-stage hermetic gate (BE unit + FE vitest + tsc + D-04 transport-swap + D15 wire-type + D8-04 WalkDir + audit replay golden set); live RF-5 verdict-parse-rate spike re-run on GLM-5.2 + Milvus + BGE-M3 (operator gate — same pattern as Phase 7/9/11); frozen eval baseline v2 re-lock (v0.8 milestone re-baseline including all 12 behaviors); `docs/threat-model/STRIDE-12.md` (8 threats: T-P12-1 sub-agent injection, T-P12-2 plan replay PII, T-P12-3 budget DoS, T-P12-4 plan SSE spoofing, T-P12-5 plan-deny silent abort, T-P12-6 write_todos audit bypass, T-P12-7 verdict parse manipulation, T-P12-8 loop false positive — Critical/High); `eval/golden/deep.jsonl` 50 cases | W5 | 12-06, 12-07 | **PLAN-01..05** (Phase 12 acceptance — all 5 requirements verified end-to-end) |

---

## Plan Count

**8 plans** in **5 waves**

## Wave Structure Summary

| Wave | Plans | Autonomous | Purpose |
|------|-------|------------|---------|
| 1 | 12-01 | yes | BE foundation + ApplyTrustStage hoisting + `internal/runtime` factory + MaxIteration cap |
| 2 | 12-02, 12-03 | yes, yes | BE write_todos audit + plan_todos table + data-plan-trace chunk **⟂** BE plan approval HITL Tool + registry add |
| 3 | 12-04, 12-05 | yes, yes | BE plan API + RF-5 verdict-parse-rate spike **⟂** BE budget gate callbacks + forced_stop + audit budget_triggered |
| 4 | 12-06, 12-07 | yes, yes | FE PlanTracePart streaming **⟂** FE PlanApprovalCard + PlanReplayView + LeftSidebar 6th section |
| 5 | 12-08 | no (live acceptance gate) | E2E + hermetic gates + frozen eval v2 re-lock + STRIDE-12 threat model |

## Requirement Coverage

- **PLAN-01** (SKILL.md `agent: deep` → DeepAgent + `ApplyTrustStage` exit): 12-01
- **PLAN-02** (live todo stream + cross-refresh persistence): 12-02 (BE) + 12-06 (FE)
- **PLAN-03** (plan confirmation gate, HITL reuse): 12-03 (BE tool) + 12-07 (FE card)
- **PLAN-04** (3 budget gates + forced_stop): 12-05
- **PLAN-05** (replay from audit + UI replay view): 12-02 (table) + 12-04 (API) + 12-07 (view)

All 5 PLAN-01..05 requirement IDs are covered by at least one plan.

## File Ownership Map (parallelism safety)

```
W1:
  12-01 → internal/runtime/{factory,deep_agent,trust_stage,budget}.go +
           internal/agent/runner_setup.go (refactor dispatch site) +
           internal/skill/manifest.go (validAgentTypes) +
           internal/verdict/{grounding,floor}.go (library-only downgrade) +
           internal/agent/runner_setup_test.go (TestFactoryDispatch)

W2 (parallel, no file overlap):
  12-02 → internal/agent/deep_audit.go + internal/store/sqlite/plan_todos.go +
           internal/wire/chunk.go (add data-plan-trace) +
           web/src/types/wire-types.ts (D15 mirror)
  12-03 → internal/agent/hitl/hitl_plan_approve.go + internal/agent/hitl/registry.go +
           internal/agent/hitl/audit_writer.go (extend with action_plan_approve rows)

W3 (parallel, no file overlap):
  12-04 → internal/server/plan_api.go + internal/llm/extract.go (ExtendJSON helper) +
           cmd/spike-deep/main.go + eval/golden/deep.jsonl
  12-05 → internal/agent/callbacks/budget.go + internal/store/sqlite/audit.go
           (add action_budget_triggered) + internal/wire/chunk.go (decision enum extension)

W4 (parallel, no file overlap):
  12-06 → web/src/components/PlanTracePart.tsx + web/src/stores/app-store.ts
           (plan slice — NOT messages per D14) + web/src/lib/real-chat-transport.ts
  12-07 → web/src/components/{PlanApprovalCard,PlanReplayView,PlanRulesPanel}.tsx +
           web/src/components/{CenterPanel,LeftSidebar}.tsx +
           web/src/lib/real-chat-transport.ts (data-decision extension)

W5:
  12-08 → scripts/verify-phase-12.sh + docs/threat-model/STRIDE-12.md +
           eval/golden/deep.jsonl + cmd/spike-deep/main_test.go (no production code)
```

## Key Decision Lock References (inherited from Phase 11 — immutable, do NOT re-ask)

- **D11-A2** (trust staircase 3 callbacks + HITL = Tool+StatefulInterrupt) → 12-01 (ApplyTrustStage hoist) + 12-03 (plan_approve HITL Tool)
- **D11-B1** (per-turn Tool whitelist pure function) → 12-01 (factory respects AllowedWriteTargets)
- **D11-B2** (shared ApprovalDecision schema) → 12-03 (plan_approve uses same schema verbatim)
- **D11-B3** (AllowedWriteTargets + hitlHandlers single file) → 12-03 (registry.go add `plan_approve`)
- **D11-B4** (per-session Agent single + pending Skill picker disabled) → 12-01 (factory preserves)
- **D11-C1** (Dual endpoints GET/POST) → 12-04 (plan_api.go GET + POST /plan/approve)
- **D11-C2** (Snapshot dual-table + expire check) → 12-04 (plan_approve inherits — same skill_version + prompt_hash snapshot)
- **D11-C3** (audit_writer sync INSERT before StatefulInterrupt) → 12-03 (plan_approve audit fail-closed)
- **D11-C4** (approval_id == tool_call_id) → 12-03 (plan_approve single identity)
- **D11-D1** (Rule match = target + sha256(frozenArgs) + skill_id) → 12-07 (PlanRulesPanel)
- **D11-D2** (approval_rules independent table) → 12-07 (plan rules reuse same table)
- **D11-D3** (session/skill binary scope) → 12-07 (PlanRulesPanel radio)

## New Phase 12 Locks (in-plan-set)

- **P12-L1** (`internal/runtime` is the new package — archetype dispatch + `ApplyTrustStage` exit). All archetypes (chat / deep / future sub-agent) MUST funnel through `ApplyTrustStage` before verdict returns to chatHandler. PLAN-01 SC-1 regression lock.
- **P12-L2** (`deep.Config.MaxIteration: 10` hardcoded in factory — not config-driven). Pitfall 8 invariant. PLAN-04 lock.
- **P12-L3** (Three budget gates trigger `forced_stop` via `data-decision{decision:"forced_stop", reason}` D16 overload). PLAN-04.
- **P12-L4** (write_todos audit is INSERT-only, fail-closed — mirrors HITL-04). PLAN-02 + PLAN-05.
- **P12-L5** (plan_todos is a separate sqlite table — not extended audit.action). PLAN-05 replay query hot path.
- **P12-L6** (data-plan-trace is a new wire discriminator — NOT overload of data-decision). D15 lockstep with TS mirror.
- **P12-L7** (plan_approve Tool reuses ApprovalDecision schema + audit_writer + dual-table snapshot — zero schema divergence). PLAN-03 reuse D11-B2/B3/C2/C3/C4 verbatim.
- **P12-L8** (Zustand `plan` slice is independent from `messages` — D14 extension). 12-06 + 12-07.
- **P12-L9** (Loop detection: rolling 3-call `fnv64a(tool_name + first 200 chars of args)` — tolerates multi-hop variation). Pitfall 5 mitigation.
- **P12-L10** (RF-5 structured-output final step: if GLM-5.2 verdict parse rate <90% in spike, lock structured-output final step; if <70%, escalate).

## Test Lock Requirements

**Backend (Go hermetic):**
- `TestFactoryDispatch` (`agent: chat` → ChatModelAgent; `agent: deep` → DeepAgent) — 12-01
- `TestApplyTrustStageHoist` (all archetypes funnel through trust floor→grounding→critique) — 12-01
- `TestMaxIterationHardCap` (deep.Config.MaxIteration: 10, not config-driven) — 12-01
- `TestWriteTodosAuditedTool` (audit + SSE + session-value triple) — 12-02
- `TestPlanTodosReplay` (write_todos audit row + replay query SELECT) — 12-02
- `TestDataPlanTraceWireType` (data-plan-trace in ValidTypes + PlanTracePayload struct) — 12-02
- `TestPlanApproveFirstPassInterrupts` (target="plan_approve", audit fail-closed) — 12-03
- `TestPlanApproveDenyForcesRewrite` (deny → data-decision{decision:"forced_stop", reason:"plan denied"}) — 12-03
- `TestPlanApproveRegistryAllowedTargetsContains` (AllowedWriteTargets contains plan_approve) — 12-03
- `TestPlanAPIReplayEndpoint` (GET /api/run/:id/plan returns ordered plan_todos) — 12-04
- `TestExtractVerdictJSONStructuredOutput` (verdict-JSON suffix + structured output parse) — 12-04
- `TestBudgetMaxIterationCap` (deep.Config.MaxIteration: 10 → forced_stop after 10) — 12-05
- `TestBudgetTokenCounterAccumulation` (model Usage.TotalTokens sum → forced_stop on >50K) — 12-05
- `TestBudgetLoopDetection3InRow` (3 same hash calls → forced_stop) — 12-05
- `TestBudgetForcedStopEmitsDataDecision` (data-decision{decision:"forced_stop"}) — 12-05
- `TestBudgetAuditRowOnTrigger` (audit action_budget_triggered with budget_layer) — 12-05
- `TestNoLegacyInterruptAPI` WalkDir extended to `internal/runtime/` + `internal/agent/plan/` + `internal/agent/deep_audit.go` — 12-01 + 12-08

**Frontend (vitest):**
- `TestPlanTracePartStreaming` (live todo updates as data-plan-trace arrives) — 12-06
- `TestPlanTracePartCrossRefresh` (plan slice persists across page refresh) — 12-06
- `TestPlanTracePartSyntheticToolCall` (synthetic tool-input-* + tool-output-available for AI SDK v5 live render) — 12-06
- `TestWireTypeDataPlanTraceAdded` (D15 mirror updated) — 12-06
- `TestPlanApprovalCardRendering` + `TestPlanApprovalCardApproveDeny` + `TestPlanApprovalCardScopeRadio` — 12-07
- `TestPlanReplayViewReconstruction` (fetches GET /api/run/:id/plan + renders timeline) — 12-07
- `TestPlanReplayViewStatusTransitions` (pending → in_progress → completed animation) — 12-07
- `TestPlanRulesPanelRevoke` (LeftSidebar 6th section mirrors ApprovalRulesPanel) — 12-07
- `TestPlanApprovalCardDenyRequiresReason` (≥3 chars reason for plan deny) — 12-07
- `TestForcedStopDataDecisionRouting` (data-decision{decision:"forced_stop"} → PlanApprovalCard expired state) — 12-07
- `TestTransportSwapWithPlanTracePart` (D-04 byte-identical preserved with new chunks) — 12-06 + 12-08
- `TestWireTypeCompilation` (D15 invariant preserved with data-plan-trace added) — 12-06 + 12-08

**Acceptance gate (12-08):**
- `scripts/verify-phase-12.sh` 11-stage hermetic gate
- `cmd/spike-deep parse-rate --golden=eval/golden/deep.jsonl` (RF-5 verdict parse rate — operator runs live stack)
- Frozen eval v2 re-lock (v0.8 milestone re-baseline; sha256 byte-identity verified)
- Threat-model STRIDE-12 register (8 threats — T-P12-1 sub-agent injection, T-P12-2 plan replay PII, T-P12-3 budget DoS, T-P12-4 plan SSE spoofing, T-P12-5 plan-deny silent abort, T-P12-6 write_todos audit bypass, T-P12-7 verdict parse manipulation, T-P12-8 loop false positive)
- `eval/golden/deep.jsonl` 50 cases (PLAN-05 replay golden set)

---

## OUTLINE COMPLETE

**8 plans** in **5 waves** — vertical slices, MVP mode, FE+BE Planning same phase.

1. **12-01** — BE foundation + ApplyTrustStage hoisting + `internal/runtime` factory + MaxIteration cap
2. **12-02** — BE write_todos audited Tool + plan_todos table + data-plan-trace SSE chunk
3. **12-03** — BE plan approval HITL Tool + AllowedWriteTargets registry add
4. **12-04** — BE plan API endpoints + RF-5 verdict parse rate spike + structured-output final step
5. **12-05** — BE budget gate callbacks (MaxIteration + Token + Loop) + forced_stop + audit
6. **12-06** — FE PlanTracePart streaming component + wire-types D15 mirror
7. **12-07** — FE PlanApprovalCard + PlanReplayView + LeftSidebar 6th section
8. **12-08** — E2E acceptance gate + RF-5 spike re-run + frozen eval v2 re-lock + STRIDE-12
