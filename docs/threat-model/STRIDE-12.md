# STRIDE-12 — Phase 12 Threat Model Register

**Phase:** 12 — Planning Layer (P8.3 — PLAN-01..05)
**Version:** 1.0
**Last updated:** 2026-08-20
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Accepted at Phase 12 acceptance gate (12-08)

---

## 1. Scope & Trust Boundaries

Phase 12 introduces the **agentic planning layer**: a 4-stage
DeepAgent (sub-agent dispatch + ApplyTrustStage hoist + plan_todos
audit + plan approval gate + 3-layer budget cap) that turns the
flat HITL Skills (Phase 11) into a multi-step orchestrator. The trust
boundary crossings are:

| Boundary | Description |
|----------|-------------|
| chatHandler → adk.Runner | Unchanged from Phase 11; HTTP ctx → Runner.Run ctx propagation |
| adk.Runner → `internal/runtime/dispatchAgent` | **NEW (PLAN-01)** — per-skill agent dispatch; factory returns DeepAgent for `agent: deep` SKILL.md |
| `internal/runtime/ApplyTrustStage` → verdict primitives | **NEW (PLAN-01)** — trust-stage exit before verdict render; hoists the apply stage out of the prompt-injection surface |
| `deep.New` → Handlers budget middleware | **NEW (PLAN-04)** — 3-layer budget gate (MaxIteration hard cap + token counter + loop detection) |
| `write_todos` Tool → SQLiteStore.plan_todos + SSE emitter | **NEW (PLAN-02)** — audit INSERT before SSE emit; cross-refresh rehydration |
| `plan_approve` Tool → approval_id == tool_call_id | **NEW (PLAN-03)** — HITL extension; D11-C4 invariant preserved (approval_id PK UNIQUE) |
| GET `/api/run/:id/plan` → plan_todos read-only | **NEW (PLAN-05)** — replay surface; PlanReplayView consumes it |
| POST `/api/run/:id/plan` → plan approval | **NEW (PLAN-03)** — ApprovalCard-equivalent for plan-shaped writes |

Phase 12 inherits the **8 Critical/High threats** from Phase 11 (T-HITL-01..08) and adds **8 new threats** (T-P12-1..T-P12-8) covering the planning layer's expanded attack surface. The Phase 12 register below enumerates only the **new threats**; for inherited HITL threats see `docs/threat-model/STRIDE-11.md`.

---

## 2. STRIDE Register — Phase 12 New Threats

Threats are ordered by risk: **Critical first** (T-P12-3/6/7), then
**High** (T-P12-1/2/4/5/8). Each Critical threat maps to a named test
in the acceptance gate (`scripts/verify-phase-12.sh`).

| Threat ID | Category | Component | Disposition | Mitigation | Verification Gate |
|-----------|----------|-----------|-------------|------------|-------------------|
| **T-P12-3** | Denial of Service | `internal/runtime/deep_agent.go` + `internal/agent/callbacks/budget.go` — 3-layer budget gate | mitigate | Hard-coded `MaxIteration: 10` literal at `deep_agent.go:95` (P12-L2 source-grep locked by `TestMaxIterationHardCap`); token counter accumulator (50,000 cap) via Handlers middleware; 3-call fnv64a loop detection with 200-char first-arg truncation (Pitfall 5) | `TestBudgetMaxIterationCap` + `TestBudgetTokenCounterAccumulation` + `TestBudgetLoopDetection3InRow` + `TestBudgetForcedStopEmitsDataDecision` + `TestBudgetAuditRowOnTrigger` |
| **T-P12-6** | Tampering / Repudiation | `internal/agent/hitl/plan_todos.go` + SQLiteStore.plan_todos — write_todos audit | mitigate | T-3-AUDIT INSERT-only pattern (no UPDATE/DELETE methods on plan_todos table); audit INSERT executes BEFORE the SSE data-plan-trace emit (P12-L4 ordering); INSERT failure → Tool returns error, SSE never fires, no silent bypass | `TestWriteTodosAuditedTool` + `TestPlanTracePartStreaming` + `TestPlanTracePartCrossRefresh` |
| **T-P12-7** | Tampering | `internal/verdict/extract.go` — verdict JSON parse manipulation | mitigate | RF-5 structured-output final step (`ExtractVerdictJSONWithStructuredOutput`) on direct-path miss; P12-L10 lock: rate < 90% → structured-output mandatory; rate < 70% → escalate to verdict-suffix + forced_stop on parse failure | `TestSpikeDeepVerdictParseRateMeasurement` (operator-live, 70% floor) + `TestSpikeDeepVerdictParseRateReportsStructuredOutputImprovement` (operator-live, 90% floor) + `TestSpikeDeepParseRateOutputJSONLSchema` |
| **T-P12-1** | Tampering / Elevation of Privilege | `internal/runtime/factory.go` — DeepAgent sub-agent injection | mitigate | `WithoutGeneralSubAgent: true` in deep_agent.go (P12-L1 hardening) — DeepAgent cannot dispatch a sub-agent at runtime via a Skill manifest's `sub_agents:` field; sub-agent fan-out deferred to Phase 13 with explicit tier-inheritance + per-sub-agent grounding + budget-accounting | `TestFactoryDispatch` + `TestApplyTrustStageHoist` + `TestManifestValidatesDeepAgent` (manifest schema rejects sub_agents for `agent: deep`) |
| **T-P12-2** | Information Disclosure | `internal/store/sqlite/plan_todos.go` + GET /api/run/:id/plan — plan replay PII | mitigate | plan_todos content may include sensitive skill context (tool args, retrieval excerpts); retention redaction (Phase 6 SC-5) applies to plan_todos rows older than retention window; PlanReplayView renders text-only via React JSX escaping (no innerHTML, no markdown raw HTML) | `TestPlanTodosReplay` + `TestPlanAPIReplayEndpoint` + `TestPlanReplayViewReconstruction` + `TestPlanReplayViewLifecycleFromAudit` |
| **T-P12-4** | Spoofing | `internal/wire/chunk.go` — data-plan-trace SSE chunk fabrication | mitigate | SSE channel is server-emit-only (Hertz invariant; same as data-decision); FE `useChat` only parses server-originated UIMessageChunk; no client-side chunk construction; transport-swap test (`RealChatTransport.test.ts`) verifies byte-identical serialization | `TestSyntheticWriteTodosToolCallRouting` + `TestPlanTracePartStreaming` + D-04 transport-swap byte-identity gate (Stage 8 of `scripts/verify-phase-12.sh`) |
| **T-P12-5** | Tampering / Repudiation | `internal/agent/hitl/hitl_plan_approve.go` — plan-deny silent abort | mitigate | Deny semantics = `forced_stop` data-decision with `decision:"plan-denied"` (D16 overload of forced_stop enum) + `reason:"..."` field; user sees the denied state in PlanApprovalCard (NOT silent abort); agent rewrites plan and re-submits via plan_approve (P12-L7 loop) | `TestPlanApproveFirstPassInterrupts` + `TestPlanAPIApproveValidation` + `TestPlanApprovalCardRendering` |
| **T-P12-8** | Denial of Service | `internal/agent/callbacks/budget.go` — loop detection false positives | mitigate | fnv64a hash of the FIRST 200 CHARS of the tool call args (Pitfall 5 mitigation — full-arg hash would falsely flag legitimate multi-hop calls with shifting context); threshold tunable via env (`LOOP_DETECTION_THRESHOLD`, default 3); legitimate multi-hop uses DIFFERENT first-200-char prefixes (verified by `TestSyntheticWriteTodosToolCallRouting` — distinct prefixes do not collide) | `TestBudgetLoopDetection3InRow` + `TestSyntheticWriteTodosToolCallRouting` |

---

## 3. Critical Threats — Full Descriptions

### T-P12-3 (Denial of Service) — DeepAgent Cost Explosion

**Stakes.** A DeepAgent that runs without a budget cap can consume
unbounded LLM tokens — a single misconfigured Skill could cost
hundreds of USD before a human notices. The 12-04 spike measured
GLM-5.2 at ~$0.01/call; an unbounded DeepAgent with 50 multi-hop
cases × 20 inner-loop calls/case = 1000 calls × $0.01 = $10.00 per
single Skill invocation. Without the 3-layer gate, this is a
budget-blow-up vector that Phase 11's flat-Skill design avoided.

**Component.** `internal/runtime/deep_agent.go::deep.New` (the
DeepAgent construction) + `internal/agent/callbacks/budget.go`
(Handlers middleware wrapping each tool call).

**Mitigation.** Three independent layers, any of which triggers a
`forced_stop` data-decision emit:

1. **MaxIteration hard cap (P12-L2)** — `MaxIteration: 10` is a
   **literal in source** at `internal/runtime/deep_agent.go:95`,
   source-grep locked by `TestMaxIterationHardCap`. Any future
   change that re-parameterizes this (e.g. via env var) surfaces as
   a test failure.
2. **Token counter accumulator** — `internal/agent/callbacks/budget.go`
   sums the OpenAI-compatible `usage.total_tokens` field on every
   model response; caps at 50,000 tokens total per run. Tunable via
   `DEEP_TOKEN_CAP` env (default 50,000).
3. **Loop detection (3-call rule)** — fnv64a hash of the first 200
   chars of tool-call args; if 3 consecutive calls produce identical
   hashes, the agent is in a loop → forced_stop. Tunable via
   `LOOP_DETECTION_THRESHOLD` env (default 3).

On ANY trigger, the budget middleware emits `data-decision` chunk
with `decision:"forced_stop"` + the triggering layer (recorded in
the audit row's `budget_layer` column). The UI surfaces the stop
verbatim in the chat; the audit row anchors the non-repudiation.

**Verification.** `TestBudgetMaxIterationCap` (12-05) drives 11
inner iterations → asserts forced_stop at iteration 11.
`TestBudgetTokenCounterAccumulation` (12-05) drives 50,001 tokens
via scripted ChatModel → asserts forced_stop on the 50,001st.
`TestBudgetLoopDetection3InRow` (12-05) drives 3 identical tool
calls → asserts forced_stop on the 3rd. `TestBudgetForcedStopEmitsDataDecision`
verifies the data-decision chunk shape. `TestBudgetAuditRowOnTrigger`
verifies the audit row INSERT (P12-L4 ordering). All Stage 4 in
`scripts/verify-phase-12.sh`.

### T-P12-6 (Tampering / Repudiation) — write_todos Audit Bypass

**Stakes.** The `write_todos` Tool is the planning layer's
non-repudiation anchor — every plan_todo the agent emits must have
a corresponding audit row. If the audit INSERT happens AFTER the
SSE emit (P12-L4 inverted), a crash between them leaves a
plan_todo visible to the user with NO audit trail. Phase 11's
T-HITL-04 mitigation applies the same fail-closed posture to
write_todos.

**Component.** `internal/agent/hitl/plan_todos.go::planTodosTool.InvokableRun`
+ `internal/store/sqlite/plan_todos.go::InsertPlanTodo`.

**Mitigation.** INSERT BEFORE SSE emit (P12-L4). The Tool's
`InvokableRun` body is:

```
1. INSERT into plan_todos (run_id, todo_id, content, status, ts)  ← fail-closed
2. Emit data-plan-trace SSE chunk  ← only on INSERT success
3. Return nil to the runner
```

If step 1 fails (DB locked, disk full, schema mismatch), the Tool
returns an error; step 2 NEVER runs; the SSE chunk is NEVER
emitted; the user sees NO plan_todo. This is the EXACT inversion
of the read path (T-3-AUDIT inverted; the audit read path swallows
failures so a transient DB blip does not block retrieval).

**Verification.** `TestWriteTodosAuditedTool` (12-02) drives the
fail-closed path with a stubbed failing store → asserts no SSE emit.
`TestPlanTracePartStreaming` (12-02/12-06) drives the success path
→ asserts SSE emit + audit row INSERT. `TestPlanTracePartCrossRefresh`
(12-06) verifies the audit row is queryable after a page refresh
(PlanReplayView rehydration).

### T-P12-7 (Tampering) — Verdict JSON Parse Manipulation

**Stakes.** The verdict is the DeepAgent's structured output — a
JSON object describing the answer's ground-truth mapping. If the
LLM emits free-form text instead of JSON (a common failure mode
for mid-size models like GLM-5.2 on adversarial prompts), the
verdict renderer crashes and the user sees an unstructured blob.
A determined adversary can craft prompts that reliably break
verdict parsing.

**Component.** `internal/verdict/extract.go::ExtractJSON` (direct
path) + `ExtractVerdictJSONWithStructuredOutput` (12-04 Task 3
fallback) + `cmd/spike-deep/main.go::measureParseRate` (RF-5
measurement harness).

**Mitigation.** The RF-5 spike measures GLM-5.2's parse rate on
the 50-case `eval/golden/deep.jsonl` set. The P12-L10 lock
evaluation:

- direct_rate ≥ 90%: structured-output final step is OPTIONAL
  (recommend enabling for safety, but not blocking).
- direct_rate 70-89%: structured-output final step is MANDATORY
  (P12-L10 lock).
- direct_rate < 70%: ESCALATE — fallback to verdict-JSON suffix +
  `forced_stop` on parse failure.

The structured-output fallback propagates the recorded
OpenAI-compatible `response_format=json_object` signal to the
model; on a direct-path miss, the fallback call gets the JSON
schema and the parse rate jumps (verified by the operator-runs-live
`TestSpikeDeepVerdictParseRateReportsStructuredOutputImprovement`).

**Verification.** `TestSpikeDeepVerdictParseRateMeasurement` (12-08
Task 2; operator-runs-live) asserts `direct_rate >= 0.70`. Below
70% the test FAILS with "RF-5 lock: structured-output final step
is mandatory + escalation needed".
`TestSpikeDeepVerdictParseRateReportsStructuredOutputImprovement`
(12-08 Task 2; operator-runs-live) asserts `total_rate >= 0.90`
with structured-output enabled. Below 90% the fallback wiring is
broken → fail loudly. `TestSpikeDeepParseRateOutputJSONLSchema`
(12-08 Task 2; hermetic) locks the JSONL schema against refactor
drift.

---

## 4. High Threats — Paragraph Summaries

### T-P12-1 (Tampering / Elevation of Privilege) — Sub-Agent Injection via SKILL.md

A malicious Skill author could embed `sub_agents: [...]` in a
SKILL.md manifest to instruct the DeepAgent to dispatch a sub-agent
at runtime, bypassing the per-Skill tool whitelist (D11-B1). Phase
12's mitigation is **structural**: `WithoutGeneralSubAgent: true`
in `deep_agent.go` (P12-L1 hardening) means the DeepAgent cannot
honor a `sub_agents:` field — the manifest schema rejects it at
load time (`TestManifestValidatesDeepAgent`). Sub-agent fan-out is
deferred to Phase 13 with explicit tier-inheritance + per-sub-agent
grounding + budget-accounting (the sub-agent's tool calls count
against the parent's MaxIteration cap).

### T-P12-2 (Information Disclosure) — plan_todos PII via Replay Surface

plan_todos rows may include sensitive skill context (the LLM's
intermediate reasoning, retrieval excerpts, tool args). The
GET `/api/run/:id/plan` endpoint exposes these to the
PlanReplayView. Mitigation is layered:

1. **Retention redaction** (Phase 6 SC-5) applies — rows older than
   the retention window are redacted at the SQL level (the
   `content` column is replaced with a hash-only stub).
2. **Text-only rendering** — PlanReplayView uses React JSX escaping
   (no `dangerouslySetInnerHTML`, no markdown raw HTML), so an
   adversarial tool-arg payload cannot execute as HTML.
3. **Read-only surface** — GET is the only verb on the plan API;
   POST `/api/run/:id/plan/approve` is the only state-mutating
   verb and is scoped to the approval gate (T-HITL-04 fail-closed
   posture inherited).

### T-P12-4 (Spoofing) — data-plan-trace SSE Chunk Fabrication

A malicious client could attempt to construct a `data-plan-trace`
chunk in the browser and post it to the chat handler, claiming
the agent emitted a plan todo that did not exist. Mitigation is
identical to T-HITL-04's pattern: the SSE channel is server-emit-
only (Hertz invariant). The FE `useChat` only parses server-
originated UIMessageChunk; no client-side chunk construction is
possible (the RealChatTransport's `sendMessages` method rejects
non-`text-delta` outbound chunks at the protocol layer).

### T-P12-5 (Tampering / Repudiation) — Plan-Deny Silent Abort

A naive plan-deny implementation would silently abort the agent's
execution, leaving the user with no signal that their denial was
recorded. Phase 12's deny semantics emit a `forced_stop` data-
decision with `decision:"plan-denied"` + a `reason:"..."` field;
PlanApprovalCard surfaces the denied state to the user, and the
audit row records the denial + the agent's next action (re-submit
with a rewritten plan, per P12-L7).

### T-P12-8 (Denial of Service) — Loop Detection False Positives

Loop detection is necessary (T-P12-3 layer 3) but a naive
implementation would falsely flag legitimate multi-hop calls
(where the same tool is called multiple times with DIFFERENT
args). The fnv64a hash on the FIRST 200 CHARS of args (Pitfall 5)
trades false-positive resistance for some sensitivity to short-
prefix collisions; legitimate multi-hop uses DISTINCT prefixes
(verified by `TestSyntheticWriteTodosToolCallRouting`).

---

## 5. Trust Boundaries — Detail

### 5.1 New Boundaries (Phase 12)

| Boundary | Description | Trust Anchor |
|----------|-------------|--------------|
| `dispatchAgent` factory → DeepAgent constructor | Per-skill agent dispatch; rejects `agent: deep` Skills without `WithoutGeneralSubAgent: true` | `internal/runtime/factory.go::BuildAgentForSkill` + `TestFactoryDispatch` |
| `ApplyTrustStage` → verdict primitives | Trust stage hoists apply out of prompt-injection surface | `internal/runtime/trust_stage.go::ApplyTrustStage` + `TestApplyTrustStageHoist` |
| Budget middleware → forced_stop emitter | Token + iteration + loop counts → data-decision chunk on threshold | `internal/agent/callbacks/budget.go::Handlers` + 3 budget tests |
| `write_todos` Tool → plan_todos + SSE | Audit INSERT before SSE emit (P12-L4) | `internal/agent/hitl/plan_todos.go::planTodosTool` + `TestWriteTodosAuditedTool` |
| `plan_approve` Tool → approval_id | approval_id == tool_call_id (D11-C4 invariant preserved) | `internal/agent/hitl/hitl_plan_approve.go` + `TestPlanApproveFirstPassInterrupts` |
| GET `/api/run/:id/plan` → SQLiteStore.plan_todos read | Read-only; PlanReplayView consumes it | `internal/server/plan_api.go` + `TestPlanAPIReplayEndpoint` |

### 5.2 Inherited Boundaries (Phase 11)

The Phase 11 trust boundaries (chatHandler → adk.Runner, Runner →
CheckPointStore, approval_rules PK UNIQUE, etc.) are inherited
unchanged. See `docs/threat-model/STRIDE-11.md` §1.

---

## 6. Cross-References

### 6.1 Phase 11 Decisions (preserved)

- **D11-B1** — per-turn pure function `BuildToolsForSkill`. Phase 12
  inherits; the deep archetype's tool whitelist is computed the same
  way.
- **D11-B4** — Skill picker disabled when `approval.pending !== null`.
  Phase 12 extends to `plan_approval.pending !== null`.
- **D11-C4** — `approval_id == tool_call_id` PK UNIQUE. Phase 12
  preserves for `plan_approve`.
- **D11-D2** — `approval_rules` PK `(target, args_hash, skill_id,
  scope_session_id)`. Phase 12 does NOT add plan-shape rules
  (delegated to Phase 13+).

### 6.2 New Phase 12 Locks

- **P12-L1** — `WithoutGeneralSubAgent: true` (sub-agent injection
  mitigation).
- **P12-L2** — `MaxIteration: 10` literal in
  `internal/runtime/deep_agent.go:95`. Source-grep locked.
- **P12-L4** — plan_todos INSERT before SSE emit (fail-closed audit).
- **P12-L7** — plan-deny forces re-submission loop (D16 overloaded
  forced_stop semantics).
- **P12-L10** — RF-5 parse rate ≥ 70% escalation floor; ≥ 90%
  structured-output rescue floor.

---

## 7. Security Domain Controls (ASVS V2..V8)

Phase 12 inherits the ASVS V2..V8 controls documented in 12-RESEARCH.md §Security Domain:

| ASVS Category | Phase 12 Control |
|---------------|------------------|
| V2 — Authentication | Unchanged from Phase 11 (single-operator session model) |
| V3 — Session Management | Plan approval tokens follow the D11-C4 approval_id pattern |
| V4 — Access Control | Per-Skill tool whitelist (D11-B1) inherited; DeepAgent cannot exceed it |
| V5 — Validation/Sanitization/Coding | Verdict JSON schema validation (T-P12-7 mitigation); React JSX escaping (T-P12-2) |
| V6 — Cryptography | N/A (no new crypto in Phase 12) |
| V7 — Error Handling and Logging | Budget middleware logs forced_stop with budget_layer (T-P12-3) |
| V8 — Data Protection | Retention redaction on plan_todos (T-P12-2) |
| V9 — Communications | SSE channel server-emit-only (T-P12-4) |
| V10 — Malicious Code | Sub-agent injection blocked (T-P12-1); loop detection blocks cost-explosion (T-P12-3) |

---

## 8. Acceptance Criteria

Phase 12 ships ONLY when all 3 Critical mitigations pass the
acceptance gate (`bash scripts/verify-phase-12.sh`):

- [ ] **T-P12-3** — `TestBudgetMaxIterationCap` +
  `TestBudgetTokenCounterAccumulation` +
  `TestBudgetLoopDetection3InRow` +
  `TestBudgetForcedStopEmitsDataDecision` +
  `TestBudgetAuditRowOnTrigger` (Stage 1) PASS.
- [ ] **T-P12-6** — `TestWriteTodosAuditedTool` +
  `TestPlanTracePartStreaming` + `TestPlanTracePartCrossRefresh`
  (Stage 1) PASS.
- [ ] **T-P12-7** — `TestSpikeDeepVerdictParseRateMeasurement`
  (operator-live; Stage 10 deferred) PASS at ≥ 70% +
  `TestSpikeDeepVerdictParseRateReportsStructuredOutputImprovement`
  PASS at ≥ 90% with structured-output.

A single failure on any of the above reopens a Critical threat to
production; the gate exits 1 and the phase is NOT shipped.

---

## 9. Open Risks (Deferred to Phase 13 / v1.1)

- **Sub-agent fan-out** — `WithoutGeneralSubAgent: true` is a
  structural block; Phase 13 introduces tier-inheritance + per-
  sub-agent grounding + budget-accounting.
- **Cross-Skill plan replay** — Phase 12's GET `/api/run/:id/plan`
  is scoped to one run; cross-Skill plan correlation is Phase 13+.
- **Plan-shape approval rules** — Phase 12 does NOT add plan-shape
  rules to the `approval_rules` table (the data-decision chunk
  carries the verdict + budget_layer inline). A separate
  `plan_approval_rules` table is Phase 13+.
- **Async plan approval** — Phase 12's `plan_approve` is
  synchronous (operator clicks in the UI). Async approval
  (email/webhook/Slack) is Phase 13+.
- **Quorum / delegation** — Single-operator model inherited from
  Phase 11. Multi-operator quorum is Phase 13+.

---

## 10. Change Log

| Date | Change |
|------|--------|
| 2026-08-20 | Phase 12 acceptance gate (12-08): initial STRIDE register; 8 threats (T-P12-1..T-P12-8) mapped to named test gates in `scripts/verify-phase-12.sh`. 3 Critical (T-P12-3/6/7) + 5 High (T-P12-1/2/4/5/8). Reviewed quarterly by Internal Audit Specialist. |