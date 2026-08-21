# STRIDE-13 — Phase 13 (P8.4 — Sub-agent Fan-out) Threat Model

**Phase:** 13 (P8.4 — 子 agent 扇出 — tier 继承)
**Version:** 1.0
**Last updated:** 2026-08-21
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Accepted at Phase 13 hermetic acceptance gate (13-04). Live
UAT operator sign-off (13-05) closes the v0.8 milestone.

---

## 1. Scope & Trust Boundaries

Phase 13 introduces **structural sub-agent fan-out** (P8.4 — tier 继承):
a `FanOutOrchestrator` agent type in `internal/runtime/orchestrator.go`
that dispatches one layer of parallel sub-agents, each with a tier-
closure-inherited manifest, per-sub-agent grounding via the existing
`ApplyTrustStage` trust staircase, conservative partial-failure merge,
and RunPath/Transfer audit columns. The trust boundary crossings are:

| Boundary | Description |
|----------|-------------|
| `internal/runtime/orchestrator.go::FanOutOrchestrator.Run` 5-step pipeline | Orchestrator owns the fan-out lifecycle; sub-agents constructed per-Run via `runtime.BuildSubAgent` (tier-closure baked into Tool closure at construction time) |
| `runtime.BuildSubAgent` constructor (tier-closure) | The ONLY site where `subManifest.Reads = intersectTiers(parent.ResolvedTiers(), subSkill.Reads)` is computed. Empty intersection = load-time error. `subManifest.Writes = []string{}` AT CONSTRUCTION (structural read-only by P13-L4) |
| `runSubAgent` per-goroutine (errgroup bounded) | `defer recover()` catches panics → `Status: faulted`; `context.WithTimeout(30s)` per sub-agent; per-sub-agent `runtime.ApplyTrustStage` grounds citations BEFORE merge |
| Audit `data-decision` discriminator (D16 overload) | 3 new values `fanout_dispatched \| fanout_completed \| fanout_failed` extended onto existing chunk; NO new SSE chunk type (D15 alarm + D-04 transport-swap preserved) |
| `errgroup.SetLimit(4)` parallelism cap | One sub-agent's slow retriever/LLM cannot starve the others |
| `*ChunkAccumulator` shared across parent + sub-agents | LRU cap at 20 chunks (P13-L9 + Pitfall 6); sub-agents append via shared pointer; eviction at capacity |

Phase 13 inherits the **8 Critical/High threats** from Phase 11
(T-HITL-01..08) and the **8 Critical/High threats** from Phase 12
(T-P12-1..T-P12-8), and adds **8 new threats** (T-P13-1..T-P13-8)
covering the sub-agent fan-out surface. The Phase 13 register below
enumerates only the **new threats**; for inherited threats see
`docs/threat-model/STRIDE-11.md` § and `docs/threat-model/STRIDE-12.md`.

---

## 2. STRIDE Register — Phase 13 New Threats

Threats are ordered by risk: **Critical first** (T-P13-1/2/3/4/5/6/7/8).
Each threat maps to a named test in the acceptance gate
(`scripts/verify-phase-13.sh`).

| Threat ID | Category | Component | Disposition | Mitigation | Verification Gate |
|-----------|----------|-----------|-------------|------------|-------------------|
| **T-P13-1** | Elevation of Privilege | `BuildSubAgent` tier intersection mismatch allows sub-agent to query parent's unauthorized tier | mitigate | P13-L2 — BuildSubAgent returns error on empty intersection AT CONSTRUCTION (FAN-02 SC-2 load-time, NOT runtime); classifier prompt restricts `tier_filter ⊆ parent.ResolvedTiers()` (two-layer defense); `TestSubAgentTierInheritance` + `TestSubAgentCannotExpandTierBoundary` regression locks (13-01); Stage 7 hermetic gate (13-04) | `scripts/verify-phase-13.sh` Stage 2 (hermetic Go suite incl. `TestSubAgentTierInheritance` + `TestSubAgentCannotExpandTierBoundary`) + Stage 3 (WalkDir 8 roots covers the build site) |
| **T-P13-2** | Elevation of Privilege | Sub-agent Tool whitelist inherits HITL Tools from parent's manifest (write-capability leak) | mitigate | P13-L4 — BuildSubAgent zeroes `subManifest.Writes = []string{}` AT CONSTRUCTION; `BuildToolsForSkill` returns empty HITL whitelist for sub-agents; `TestSubAgentIsReadOnly` + `TestSubAgentToolWhitelistExcludesHITL` regression locks (13-01) | `scripts/verify-phase-13.sh` Stage 2 (hermetic Go suite) |
| **T-P13-3** | Denial of Service | Sub-agent panic (LLM timeout, retrieval backend stall, OOM, malformed verdict JSON) propagates to parent Run | mitigate | P13-L8 + errgroup nil-return + `defer recover()` per goroutine (Pitfall 4); `context.WithTimeout(30s)` per sub-agent; `TestFanOutOrchestratorFaultIsolation` + `TestFanOutOrchestratorSubAgentTimeout` regression locks (13-03) | `scripts/verify-phase-13.sh` Stage 2 (hermetic Go suite) |
| **T-P13-4** | Repudiation | Merge function non-determinism (Go map iteration randomness) breaks audit replay (Pitfall 3) | mitigate | P13-L4 — merge is PURE FUNCTION with `sort.SliceStable` on `(sub_agent_id ASC, citation_chunk_id ASC, citation_span ASC)`; 10× byte-identity contract test `TestMergeSubAgentResultsByteIdentity` (FAN-03 SC-3 regression lock) (13-02) | `scripts/verify-phase-13.sh` Stage 2 (hermetic Go suite) |
| **T-P13-5** | Denial of Service | Classifier LLM cost explosion (always fan-out, even for trivial queries) | mitigate | P13-L3 — per-sub-agent budget cap `MaxIteration=5` + `TokenBudget=20000` (structural, NOT config); pre-fanout budget check sums projected sub-agent budgets + parent remaining — rejects if total exceeds Run cap; `TestSubAgentBudgetHalfOfParent` regression lock; 13-05 acceptance gate measures actual cost | `scripts/verify-phase-13.sh` Stage 8 (sub-agent budget source-grep — MaxIteration=5 + TokenBudget=20000 literals) + Stage 2 (`TestSubAgentBudgetHalfOfParent`) |
| **T-P13-6** | Denial of Service | Shared ChunkAccumulator memory + token budget explosion (sub-agent N's context includes all previous retrievals; Pitfall 6) | mitigate | P13-L9 — SharedChunkAccumulator LRU cap at 20 chunks; sub-agents append to parent's accumulator via shared pointer but LRU evicts oldest when capacity reached; `TestSharedChunkAccumulatorLRUCap` + `TestSharedChunkAccumulatorDedup` + `TestSharedChunkAccumulatorHitsOrder` regression locks (13-01 + 13-03) | `scripts/verify-phase-13.sh` Stage 9 (SharedChunkAccumulator LRU cap) + Stage 2 (hermetic Go suite) |
| **T-P13-7** | Tampering | Sub-agent's retrieve Tool closure accidentally binds parent manifest (typo in wiring) | mitigate | SubAgentConfig type-safe shape (ParentManifest vs SubSkillManifest fields distinct); BuildSubAgent uses ONLY subManifest for Tool closure; `TestSubAgentRetrieveToolClosesOverSubManifest` regression lock (13-01) | `scripts/verify-phase-13.sh` Stage 2 (hermetic Go suite) |
| **T-P13-8** | Tampering | `adk.NewParallelAgent` accidentally used (full-context sharing breaks FAN-02 per forkRunCtx at `adk/runctx.go:478-508`) | mitigate | P13-L1 — sealed sentinel `// 禁止 adk.NewParallelAgent — full-context sharing breaks FAN-02 tier-closure` at top of orchestrator.go; source-grep `TestNoNewParallelAgentUsage` (13-01 + 13-04) | `scripts/verify-phase-13.sh` Stage 4 (source-grep + sealed sentinel check) |

---

## 3. Critical Threats — Full Descriptions

### T-P13-1 (Elevation of Privilege) — Sub-Agent Tier Boundary Leak

**Stakes.** A sub-agent that successfully constructs with
`subManifest.Reads` NOT a subset of `parent.ResolvedTiers()` could
query tiers the parent's Skill author did not authorize. A
multi-tenant deployment would let one tenant's Skills read another
tenant's data via a fan-out sub-skill. This is the structural
defense of the entire sub-agent design (P13-L2).

**Component.** `internal/runtime/sub_agent_builder.go::BuildSubAgent`
(the ONLY site where tier intersection is computed) +
`internal/agent/fanout_tools.go::fanOutDispatchTool.InvokableRun` (the
classifier-side validation of `tier_filter` argument).

**Mitigation.** Two-layer defense:

1. **Load-time intersection (FAN-02 SC-2)** — `BuildSubAgent` computes
   `subManifest.Reads = intersectTiers(parent.ResolvedTiers(),
   subSkill.ResolvedTiers())` AT CONSTRUCTION. If the intersection
   is empty, `BuildSubAgent` returns an error wrapping the
   `ErrSubAgentFactory` sentinel. The error message contains the
   substring `"tier intersection empty"` so future regression tests
   can grep for it.
2. **Classifier-side argument validation** — `fanOutDispatchTool`
   rejects any `sub_query.tier_filter` element that is NOT in
   `parentManifest.ResolvedTiers()` BEFORE the sub-agent is built.
   This is a runtime check that complements the load-time check; an
   attacker that bypasses the manifest schema cannot bypass the
   Tool validator.

**Verification.** `TestSubAgentTierInheritance` (13-01) drives the
empty-intersection case → asserts the load-time error contains
`"tier intersection empty"`. `TestSubAgentCannotExpandTierBoundary`
(13-01) drives a sub-agent attempt to query a parent's tier NOT in
the intersection → asserts the Tool validator rejects the call.
`TestSubAgentRetrieveToolClosesOverSubManifest` (13-01) verifies the
sub-agent's retrieve Tool's `manifest` field is `subManifest`, NOT
`parentManifest` (Pitfall 5 — typo mitigation). All Stage 2 in
`scripts/verify-phase-13.sh`.

### T-P13-2 (Elevation of Privilege) — HITL Tool Whitelist Leak

**Stakes.** A sub-agent that inherits the parent's Tool whitelist
would have access to the parent's HITL Tools (`hitl_memory_facts`,
`hitl_review_report`, `hitl_plan_approve`). The HITL Tools are
write-capable: `hitl_memory_facts` writes to `memory:facts`,
`hitl_review_report` writes to `memory:sessions`, `hitl_plan_approve`
advances the plan approval gate. A sub-agent with HITL Tools would
allow adversarial Skills to bypass the per-turn Tool whitelist
(D11-B1) by hiding writes inside sub-agents.

**Component.** `internal/runtime/sub_agent_builder.go::BuildSubAgent`
(the constructor that wraps `subManifest.Writes`) +
`internal/skill/manifest_builder.go::WithReads` (the fluent API that
builds the sub-manifest).

**Mitigation.** `BuildSubAgent` zeroes `subManifest.Writes =
[]string{}` BEFORE calling `DispatchAgent`. This is structural
read-only — even if a sub-skill's SKILL.md frontmatter declares
`writes: ["memory:facts"]`, the sub-agent's manifest has no writes;
the Tool whitelist computation (`BuildToolsForSkill`) returns an
empty HITL whitelist.

**Verification.** `TestSubAgentIsReadOnly` (13-01) verifies
`subManifest.Writes` is `[]string{}` after construction, regardless
of the sub-skill's source manifest's `Writes` field.
`TestSubAgentToolWhitelistExcludesHITL` (13-01) verifies the
sub-agent's Tool whitelist contains NO HITL Tools. Both Stage 2 in
`scripts/verify-phase-13.sh`.

### T-P13-3 (Denial of Service) — Sub-Agent Fault Propagation

**Stakes.** A misbehaving sub-agent (LLM timeout, retrieval backend
stall, OOM, malformed verdict JSON) that propagates its panic to the
parent Run would corrupt the entire fan-out — the user would see a
crash, not the successful sub-agents' verdicts. Worse, a malicious
Skill could intentionally trigger sub-agent panics to mask their
activity from audit. The fault isolation is the second line of
defense after the conservative partial-failure merge (T-P13-4
posture is conservative; T-P13-3 is the structural isolation).

**Component.** `internal/runtime/orchestrator.go::runSubAgent` (the
per-goroutine execution path) + `errgroup.WithContext` (the bounded
parallelism).

**Mitigation.** Three independent layers:

1. **`defer recover()` per goroutine** — `runSubAgent` catches any
   panic, records `Status: faulted`, and returns `nil` to the
   errgroup. The parent sees a `Status: faulted` result and the
   conservative merge downgrades.
2. **`context.WithTimeout(30s)` per sub-agent** (P13-L3 derived) —
   prevents an indefinite LLM call from blocking the parent.
3. **`errgroup.SetLimit(4)` parallelism cap** — one slow sub-agent
   cannot starve the others (the 4-worker pool is full while one
   stalls; the other 3 still complete).

**Verification.** `TestFanOutOrchestratorFaultIsolation` (13-03)
drives a sub-agent that panics → asserts the other 2 sub-agents
complete successfully + the merged verdict reflects the fault.
`TestFanOutOrchestratorSubAgentTimeout` (13-03) drives a sub-agent
with a 1ms timeout → asserts the sub-agent times out gracefully +
the merged verdict uses the conservative posture. Both Stage 2 in
`scripts/verify-phase-13.sh`.

### T-P13-4 (Repudiation) — Merge Function Non-Determinism

**Stakes.** A merge function that iterates a Go map (random order)
produces different `MergedVerdict` bytes for the same input. Audit
replay (the audit timeline + `QueryFanoutByRunPath`) would diverge
across runs, breaking non-repudiation — a malicious actor could
argue "the merge was different because of non-determinism, not my
fault." The 10× byte-identity contract test is the structural
defense: same input MUST produce same bytes 10× in a row.

**Component.** `internal/runtime/merge.go::mergeSubAgentResults`
(the PURE function) — sorts via `sort.SliceStable` before serializing.

**Mitigation.** The merge function is PURE with no global state.
Citation keys are sorted by `(sub_agent_id ASC, citation_chunk_id ASC,
citation_span ASC)` via `sort.SliceStable`. The function takes a
slice of `SubAgentResult` and returns a `MergedVerdict`; no maps, no
channels, no goroutines. The 10× byte-identity contract test
asserts `bytes.Equal(json.Marshal(...))` for 10 identical inputs.

**Verification.** `TestMergeSubAgentResultsByteIdentity` (13-02)
drives the function 10× with the same input → asserts all 10
`json.Marshal(...)` outputs are `bytes.Equal`.
`TestMergeSubAgentOrderIndependent` (13-02) drives the function
with permuted input slices → asserts the output bytes are identical
(regardless of sub-agent input order). Both Stage 2 in
`scripts/verify-phase-13.sh`.

---

## 4. High Threats — Paragraph Summaries

### T-P13-5 (Denial of Service) — Classifier Cost Explosion

A naive fan-out classifier that emits sub-agents for every query —
even trivial single-vertex queries — would multiply LLM cost by N
sub-agents per query. Phase 13's P13-L3 budget cap is structural:
`SubAgentMaxIteration=5` + `SubAgentTokenBudget=20000` (half of the
parent's 10/50000). The constants are hardcoded literals in
`internal/runtime/fanout_runtime.go` and source-grep-locked by
Stage 8 of `scripts/verify-phase-13.sh`. The pre-fanout budget
check sums the projected sub-agent budgets against the parent's
remaining budget and rejects the fan-out if total exceeds the cap.
The 13-05 live UAT measures actual cost (cost-limit $2.00/20 cases)
and records it in `eval/frozen/fanout.lock.json` for regression
comparison.

### T-P13-6 (Denial of Service) — Shared Chunk Accumulator Growth

A `*ChunkAccumulator` shared across N sub-agents could grow without
bound — sub-agent N's retrieval context includes all previous
sub-agents' chunks. The P13-L9 LRU cap is structural:
`SharedChunkAccumulatorMaxEntries=20`. When capacity is reached,
the LRU evicts the OLDEST chunk (sub-agent 1's first chunk is
forgotten by sub-agent 5). The cap is a hardcoded constant and
source-grep-locked. `TestSharedChunkAccumulatorLRUCap` +
`TestSharedChunkAccumulatorDedup` + `TestSharedChunkAccumulatorHitsOrder`
(13-01 + 13-03) verify the cap + dedup + oldest-first ordering
behaviors. Stage 9 of `scripts/verify-phase-13.sh` runs these tests.

### T-P13-7 (Tampering) — Sub-Agent Manifest Wiring Typo

A wiring typo in `BuildSubAgent` that accidentally closes over the
parent's manifest instead of the sub-manifest would let sub-agents
inherit the parent's tool whitelist + tier set — the entire
sub-agent design is broken by a single-line typo. The defense is
the type-safe `SubAgentConfig` struct with explicit `ParentManifest`
vs `SubSkillManifest` fields; the constructor uses ONLY the
`SubSkillManifest` for Tool closure. `TestSubAgentRetrieveToolClosesOverSubManifest`
(13-01) verifies the sub-agent's retrieve Tool's `manifest` field is
the sub-manifest, NOT the parent. Pitfall 5 mitigation.

### T-P13-8 (Tampering) — adk.NewParallelAgent Accidental Use

`adk.NewParallelAgent` provides a convenient parallel-execution
primitive, but its `forkRunCtx` (eino's `adk/runctx.go:478-508`)
shares the parent's full context across all sub-agents — this
breaks FAN-02 tier-closure because sub-agents would inherit the
parent's tier set via context. Phase 13 explicitly REJECTS this API
(P13-L1) and uses a manual `errgroup` + per-Run agent construction
instead. The defense is a sealed sentinel comment at the top of
`internal/runtime/orchestrator.go` + a source-grep check (Stage 4
of `scripts/verify-phase-13.sh`) that fails if any production code
inside `internal/runtime/` invokes `adk.NewParallelAgent(`. The
check is fast (~1s for the directory) and catches the violation
before any test runs.

---

## 5. Trust Boundaries — Detail

### 5.1 New Boundaries (Phase 13)

| Boundary | Description | Trust Anchor |
|----------|-------------|--------------|
| `dispatchAgent` factory → FanOutOrchestrator + BuildSubAgent | Per-skill agent dispatch; orchestrator archetype dispatches sub-agents; sub-manifest tier-closure computed at construction | `internal/runtime/factory.go::DispatchAgent` + `internal/runtime/orchestrator.go::NewOrchestrator` + `internal/runtime/sub_agent_builder.go::BuildSubAgent` |
| `ApplyTrustStage` → verdict primitives (per-sub-agent grounding) | Per-sub-agent citation grounding before merge; shared `*ChunkAccumulator` with LRU cap | `internal/runtime/trust_stage.go::ApplyTrustStage` + `internal/runtime/fanout_runtime.go::SharedChunkAccumulator` |
| `runSubAgent` per-goroutine (errgroup bounded) | `defer recover()` + `context.WithTimeout(30s)`; conservative partial-failure merge | `internal/runtime/orchestrator.go::runSubAgent` |
| `mergeSubAgentResults` pure function | Deterministic merge with `sort.SliceStable`; 10× byte-identity contract | `internal/runtime/merge.go::mergeSubAgentResults` |
| Audit `data-decision` D16 overload | 3 new enum values `fanout_dispatched \| fanout_completed \| fanout_failed`; NO new SSE chunk type | `internal/wire/chunk.go` + `web/src/types/wire-types.ts` |
| `fanout_events` audit table columns | `run_path` + `transfer_from` + `transfer_to` ADD COLUMN DDL (P13-L6); `ActionFanoutDispatched` + `ActionFanoutCompleted` + `ActionFanoutFailed` enum constants | `internal/store/sqlite/schema.go` + `internal/store/sqlite/audit.go` + `internal/store/sqlite/fanout_events.go` |
| `QueryFanoutByRunPath(runPath)` helper | Audit traceback via RunPath/Transfer columns | `internal/store/sqlite/fanout_events.go::QueryFanoutByRunPath` |

### 5.2 Inherited Boundaries (Phase 11 + 12)

The Phase 11 trust boundaries (chatHandler → adk.Runner, Runner →
CheckPointStore, approval_rules PK UNIQUE, etc.) and the Phase 12
boundaries (deep.New → Handlers budget middleware, write_todos Tool
→ plan_todos + SSE, plan_approve Tool → approval_id) are inherited
unchanged. See `docs/threat-model/STRIDE-11.md` §1 and
`docs/threat-model/STRIDE-12.md` §5.

---

## 6. Cross-References

### 6.1 Phase 11 Decisions (preserved)

- **D11-A2** — HITL = Tool+StatefulInterrupt. Phase 13
  orchestrator + sub-agents NEVER call StatefulInterrupt (FAN-01
  SC-1; sub-agents are fire-and-forget, return compressed results).
- **D11-B1** — per-turn Tool whitelist pure function
  `BuildToolsForSkill`. Phase 13 sub-agents use the same function
  with a sub-manifest that has zero Writes → empty HITL whitelist
  (T-P13-2 mitigation).
- **D11-B3** — AllowedWriteTargets + hitlHandlers single file.
  Phase 13 sub-agents cannot reach HITL Tools structurally
  (T-P13-2 mitigation).
- **D11-C3** — audit_writer sync INSERT before StatefulInterrupt.
  Phase 13 13-03 inherits the sync INSERT posture for fanout_events
  (audit INSERT before SSE emit); fan-out has NO StatefulInterrupt,
  but the audit-first invariant is preserved.

### 6.2 Phase 12 Decisions (preserved)

- **P12-L1** — `WithoutGeneralSubAgent: true` (sub-agent injection
  mitigation). Phase 13 honors this — sub-agent fan-out is via
  Phase 13's OWN `BuildSubAgent` (NOT via the Phase 12
  `dispatchAgent`'s `WithoutGeneralSubAgent` escape hatch).
- **P12-L2** — `MaxIteration: 10` hardcoded literal in
  `internal/runtime/deep_agent.go:95`. Phase 13 adds a SECOND
  literal `SubAgentMaxIteration = 5` in `fanout_runtime.go`
  (T-P13-5 mitigation).
- **P12-L6** — wire-type discrimination strategy. Phase 13 D16
  overload extends the `data-decision.decision` enum with 3 new
  values; NO new SSE chunk type (D15 alarm + D-04 transport-swap
  preserved).
- **P12-L9** — Loop detection rolling 3-call hash. Phase 13
  fanout_runtime reuses the Phase 12 hash for sub-agent loop
  detection.

### 6.3 New Phase 13 Locks

- **P13-L1** — `adk.NewParallelAgent` BANNED. Sealed sentinel
  + source-grep (T-P13-8).
- **P13-L2** — Tier-closure structural inheritance via constructor
  closure (T-P13-1).
- **P13-L3** — Sub-agent budget scope: `MaxIteration=5` +
  `TokenBudget=20000` (T-P13-5).
- **P13-L4** — `subManifest.Writes = []string{}` AT CONSTRUCTION
  (T-P13-2). Merge pure function with `sort.SliceStable` (T-P13-4).
- **P13-L5** — Conservative partial-failure (any refused/faulted →
  needs_input confidence 0).
- **P13-L6** — Audit substrate (3 nullable TEXT columns via ADD
  COLUMN DDL; no migration break).
- **P13-L7** — D16 overload on `data-decision.decision` (3 new
  enum values; NO new SSE chunk type).
- **P13-L8** — Sub-agent scope: ONE layer only; `WithoutGeneralSubAgent: true`
  already in Phase 12 `BuildDeepAgent`; orchestrator does NOT
  invoke sub-orchestrators (T-P13-3 fault isolation).
- **P13-L9** — Per-sub-agent grounding reuses Phase 12
  `runtime.ApplyTrustStage`; shared `*ChunkAccumulator` pointer
  across parent + sub-agents — citation whitelist spans parent
  + sub-agents (T-P13-6 LRU cap).

---

## 7. Security Domain Controls (ASVS V2..V8)

Phase 13 inherits the ASVS V2..V8 controls documented in 13-RESEARCH.md
§Security Domain + STRIDE-11 + STRIDE-12:

| ASVS Category | Phase 13 Control |
|---------------|------------------|
| V2 — Authentication | Unchanged from Phase 11/12 (single-operator session model) |
| V3 — Session Management | Sub-agent `subSessionID = parent:subQueryID` (audit RunPath correlation; P13-L9) |
| V4 — Access Control | Sub-manifest tier-closure (P13-L2; T-P13-1) + empty Writes (P13-L4; T-P13-2) |
| V5 — Validation/Sanitization/Coding | Two-layer tier-filter validation (T-P13-1); conservative merge posture (T-P13-4 / P13-L5) |
| V6 — Cryptography | N/A (no new crypto in Phase 13) |
| V7 — Error Handling and Logging | `defer recover()` per goroutine (T-P13-3); conservative merge downgrades to needs_input (T-P13-4) |
| V8 — Data Protection | Tier-closure inheritance prevents cross-tier data leak (T-P13-1) |
| V9 — Communications | SSE channel server-emit-only (D16 overload + D-04 transport-swap preserved) |
| V10 — Malicious Code | Sub-agent structural read-only (T-P13-2); sealed sentinel blocks `adk.NewParallelAgent` accidental use (T-P13-8); loop detection + budget cap (T-P13-5) |

---

## 8. Acceptance Criteria

Phase 13 ships ONLY when all 8 threats pass the acceptance gate
(`bash scripts/verify-phase-13.sh`):

- [ ] **T-P13-1** — `TestSubAgentTierInheritance` +
  `TestSubAgentCannotExpandTierBoundary` +
  `TestSubAgentRetrieveToolClosesOverSubManifest` (Stage 2) PASS.
- [ ] **T-P13-2** — `TestSubAgentIsReadOnly` +
  `TestSubAgentToolWhitelistExcludesHITL` (Stage 2) PASS.
- [ ] **T-P13-3** — `TestFanOutOrchestratorFaultIsolation` +
  `TestFanOutOrchestratorSubAgentTimeout` (Stage 2) PASS.
- [ ] **T-P13-4** — `TestMergeSubAgentResultsByteIdentity` (10×
  contract) + `TestMergeSubAgentOrderIndependent` (Stage 2) PASS.
- [ ] **T-P13-5** — `TestSubAgentBudgetHalfOfParent` (Stage 2) +
  source-grep on `SubAgentMaxIteration = 5` + `SubAgentTokenBudget = 20000` (Stage 8) PASS.
- [ ] **T-P13-6** — `TestSharedChunkAccumulatorLRUCap` +
  `TestSharedChunkAccumulatorDedup` +
  `TestSharedChunkAccumulatorHitsOrder` (Stage 9) PASS.
- [ ] **T-P13-7** — `TestSubAgentRetrieveToolClosesOverSubManifest`
  (Stage 2) PASS.
- [ ] **T-P13-8** — Stage 4 source-grep + sealed sentinel check
  PASS (0 invocations of `adk.NewParallelAgent(` in
  `internal/runtime/`; sealed sentinel comment present in
  `orchestrator.go`).

A single failure on any of the above reopens a Critical threat to
production; the gate exits 1 and the phase is NOT shipped.

---

## 9. Open Risks (Deferred to Phase 14 / v1.1)

- **Cross-Skill plan replay with fan-out** — Phase 12's
  GET `/api/run/:id/plan` is scoped to one run; cross-Skill plan
  correlation across fan-out sub-agents is Phase 14+.
- **Plan-shape approval rules for sub-agent writes** — Phase 13
  sub-agents are structural read-only (T-P13-2); if v1.1 allows
  sub-agent writes, a new `plan_approval_rules` table for sub-agent
  writes is required.
- **Async sub-agent approval** — synchronous sub-agent dispatch
  (no HITL inside sub-agents); async approval (email/webhook/Slack)
  is Phase 14+.
- **Quorum / delegation** — Single-operator model inherited from
  Phase 11. Multi-operator quorum for fan-out is Phase 14+.
- **Cross-tenant fan-out** — Phase 13's tier-closure is per-Skill,
  not per-tenant. Multi-tenant fan-out isolation is Phase 14+.

---

## 10. Change Log

| Date | Change |
|------|--------|
| 2026-08-21 | Phase 13 acceptance gate (13-04 hermetic): initial STRIDE register; 8 threats (T-P13-1..T-P13-8) mapped to named test gates in `scripts/verify-phase-13.sh`. All threats Critical or High. 13-05 live UAT operator sign-off closes the v0.8 milestone. Reviewed quarterly by Internal Audit Specialist. |