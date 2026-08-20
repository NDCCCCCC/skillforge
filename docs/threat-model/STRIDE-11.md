# STRIDE-11 — Phase 11 Threat Model Register

**Phase:** 11 — Write Tools + Approval Gate (P8.2 — HITL-01..08)
**Version:** 1.0
**Last updated:** 2026-08-20
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Accepted at Phase 11 acceptance gate (11-08)

---

## 1. Scope & Trust Boundaries

Phase 11 introduces the **human-in-the-loop (HITL) approval gate** for
write-capable Skills. The trust boundary crossings are:

| Boundary | Description |
|----------|-------------|
| End user → ApprovalCard UI | Operator click crosses here (lower-trust; FE is in users' hands) |
| ApprovalCard UI → POST /api/session/:id/approval/:approvalId | Untrusted HTTP body (`ApprovalDecision.Validate()` rejects empty deny reasons + global scope) |
| chatHandler → adk.Runner.Run | Internal; HTTP request ctx → Runner.Run ctx propagation |
| Runner → CheckPointStore.Set | Eino-internal; Runner owns Set per D8-Pitfall-3 |
| Runner → Runner.ResumeWithParams | Cross-page-refresh entry (operator click → checkpoint lookup → Tool resume) |
| Tool audit_writer.InsertPendingApproval → sqlite | Write path is fail-closed (D11-C3); read path is swallow (T-3-AUDIT inverted) |
| Tool StatefulInterrupt → audit row + checkpoint | Synchronous INSERT before interrupt returns (HITL-04 invariant) |
| approval_rules table writes | Decision-maker scope (session_id bind / skill_id bind — D11-D2 PK) |
| End user → Skill switcher UI | Skill picker disabled when `approval.pending !== null` (D11-B4) |
| Tool whitelist (BuildToolsForSkill) → Agent ToolsConfig | Pure function re-computed every turn (D11-B1); prompt-injection cannot reach a tool that is not in the schema |

---

## 2. STRIDE Register

Threats are ordered by risk: **Critical first** (T-HITL-04/05/07/08),
then **High** (T-HITL-01/02/03/06). Each Critical threat maps to a
named test in the acceptance gate (`scripts/verify-phase-11.sh`).

| Threat ID | Category | Component | Disposition | Mitigation | Verification Gate |
|-----------|----------|-----------|-------------|------------|-------------------|
| **T-HITL-04** | Repudiation | `internal/agent/hitl/audit_writer.go::InsertPendingApproval` | mitigate | Sync INSERT before `tool.StatefulInterrupt` returns; distinct code path from read-path swallow layer (no shared retry helper); Tool returns error on INSERT failure (Runner sees forced_stop) | `TestFailClosedAudit` (11-03) + `TestThreatModelCriticalMitigations/T-HITL-04` (11-08) |
| **T-HITL-05** | Elevation of Privilege | `internal/agent/runner_setup.go::buildRunner` — Tool whitelist | mitigate | Per-turn pure function `BuildToolsForSkill(manifest) = AllowedWriteTargets ∩ manifest.Writes`; Agent Construction panics on mismatch; LLM never sees write tool JSON schema for read-only Skills | `TestToolWhitelistMatchesSkillWrites` + `TestPromptInjectionResistance` + `TestThreatModelCriticalMitigations/T-HITL-05` (11-08) |
| **T-HITL-07** | Tampering | `internal/agent/hitl/hitl_memory_facts.go` + `hitl_review_report.go` | mitigate | `sha256(frozen_args)` re-computed at Tool execution; Tool rejects with `frozen_args_drift` error; 3-way byte equality (preview == audit row == Tool args) | `TestApprovalArgsByteEquality` (11-03) + `TestThreatModelCriticalMitigations/T-HITL-07` (11-08) |
| **T-HITL-08** | Information Disclosure | `internal/store/sqlite/approval_rules.go` + GET /approval-rules | mitigate | `approval_rules` PK UNIQUE `(target, args_hash, skill_id, scope_session_id)`; `scope_session_id` empty refused at insert with structured `ValidationError`; left sidebar Rules panel filters by exact scope | `TestApprovalRulesScope` (11-05) + `TestThreatModelCriticalMitigations/T-HITL-08` (11-08) |
| **T-HITL-01** | Spoofing (High) | `internal/server/approval_api.go` — actor_user_id binding | mitigate | `ApprovalDecision.ActorUserID` required at HTTP boundary; audit row `actor_user_id` captured post-11-04 wiring | Manual audit sampling |
| **T-HITL-02** | Denial of Service (High) | audit / audit_pending queue | mitigate | Guardrail #5 in 11-AI-SPEC §6 (`pending_approval_count > 10` → throttle); per-session counter from audit table | Manual review of `cmd/spike-history` metrics |
| **T-HITL-03** | Denial of Service (High) | `review_report` writer | mitigate | UI requires explicit confirmation when `report_id` matches existing audit row's `frozen_args.report_id`; rule auto-pass on `review_report` opt-in per Skill (default always interrupt) | Manual UI walkthrough |
| **T-HITL-06** | Repudiation (High) | POST /api/session/:id/approval/:approvalId — cross-device race | mitigate | `approval_id == tool_call_id` PK UNIQUE in audit table; Runner's `id2ResumeDataUsed[id]=true` idempotency moat; second POST returns 200 + `state="already_resolved"` | `TestConcurrentApproveDenyIdempotency` (11-04) |

---

## 3. Critical Threats — Full Descriptions

### T-HITL-04 (Repudiation) — Audit INSERT Failure Swallowed

**Stakes.** If the audit INSERT for a pending approval fails silently
but the write tool still executes, the operator has NO evidence the
write occurred. This is the single highest-stakes failure mode in
Phase 11: the entire HITL trust model rests on the audit chain being
INSERT-only and fail-closed.

**Component.** `internal/agent/hitl/audit_writer.go::InsertPendingApproval`
(and the `WritePendingApproval` exported sibling).

**Mitigation.** The write path is the EXPLICIT INVERSION of the read
path: a failed audit INSERT returns immediately with no retry layer
and no swallow, so the HITL Tool aborts BEFORE `tool.StatefulInterrupt`
is reached. The interrupt signal is never raised, the Runner-internal
`CheckPointStore.Set` never runs, and the write tool never executes.
The two paths deliberately share no helper — a copy-paste of the
swallow posture into this file surfaces as a `TestFailClosedAudit`
failure (the P0 no-waiver gate).

**Verification.** `TestFailClosedAudit` (11-03) exercises the fail-
closed path with a stubbed failing store.
`TestThreatModelCriticalMitigations/T-HITL-04` (11-08) adds the
sha256-baseline snapshot of the `WritePendingApproval` body so a
silent code rotation (a refactor that moves the INSERT without
updating the test) fails loudly.

### T-HITL-05 (Elevation of Privilege) — Prompt-Injection Bypasses writes: Enforcement

**Stakes.** A Skill with `writes: []` must NEVER be able to reach a
write tool, regardless of how the LLM is prompted or what the user
pastes into the chat. A prompt-injection that convinces the LLM to
emit a `memory_facts_write` tool call against a read-only Skill would
be a privilege escalation.

**Component.** `internal/agent/runner_setup.go::buildRunner` — the
per-turn Tool whitelist computation.

**Mitigation.** `BuildToolsForSkill` is a D11-B1 pure function: the
LLM-visible Tool list is `intersection(AllowedWriteTargets,
manifest.Writes)`, re-computed every Runner construction. Read-only
Skills (`writes: []`) get a zero-element slice — the LLM never sees
the write tool's JSON schema, so it cannot emit a tool call for it.
Agent Construction panics on a whitelist/manifest mismatch (fail-
closed at construction time, not at tool-call time).

**Verification.** `TestToolWhitelistMatchesSkillWrites` (11-02) locks
the whitelist computation. `TestPromptInjectionResistance` (11-02)
drives a scripted injection payload against a read-only Skill and
asserts no write tool call surfaces.
`TestThreatModelCriticalMitigations/T-HITL-05` (11-08) adds the
functional assertion against the hitl-side primitives
(`AllowedWriteTargets` + `LookupHandler` + `Contains`).

### T-HITL-07 (Tampering) — frozen_args Mutated Between Approval and Execution

**Stakes.** The operator approves a specific set of tool-call args
("批的 A"). If the executed args differ ("不会执行成 B"), the approval
is meaningless. A concurrent edit, a race window, or a malicious
actor with access to the checkpoint blob could mutate the args
between the ApprovalCard preview and the Tool's Pass-2 execution.

**Component.** `internal/agent/hitl/hitl_memory_facts.go` +
`hitl_review_report.go` — the HITL Tool's Pass-2 entry.

**Mitigation.** `sha256(frozen_args)` is re-computed at Tool
execution and compared against the audit row's `frozen_args` column
AND the ApprovalCard preview bytes. The 3-way byte-equality assertion
(`assertThreeWayByteEqual`) checks all three pairs (not transitively
assumed) so a tampered middle representation cannot hide behind one
equal pair. On ANY mismatch the Tool returns `ErrFrozenArgsDrift` —
the operator must re-review, the write NEVER executes against
drifted bytes.

**Verification.** `TestApprovalArgsByteEquality` (11-03) locks the
3-way invariant across preview / audit / executed.
`TestThreatModelCriticalMitigations/T-HITL-07` (11-08) adds the
functional assertion (mismatched bytes → `ErrFrozenArgsDrift`) plus
the sha256-baseline snapshot of the `InvokableRun` body.

### T-HITL-08 (Information Disclosure) — Session-Scope Rule Leaks Across Sessions

**Stakes.** An approve-and-remember rule created in session A must
NEVER auto-pass a write in session B. A scope leak would let one
user's approval silently authorize another user's write — a cross-
session privilege escalation via the rule cache.

**Component.** `internal/store/sqlite/approval_rules.go` + the GET
/approval-rules endpoint.

**Mitigation.** The `approval_rules` table's PRIMARY KEY is the
4-tuple `(target, args_hash, skill_id, scope_session_id)` (D11-D2).
The `scope_session_id` column is the scope boundary: session-scope
rules carry the originating session ID; skill-scope rules carry the
empty string. The `matchRuleSQL` predicate `(scope_session_id = ''
OR scope_session_id = ?)` ensures session-scope rules from OTHER
sessions never match. There is NO global scope — the column set
structurally forbids it (an empty `scope_session_id` ALWAYS means
skill-scope, never global).

**Verification.** `TestApprovalRulesScope` (11-05) drives the cross-
session isolation functional test (session A's rule does not match
session B's lookup). `TestThreatModelCriticalMitigations/T-HITL-08`
(11-08) adds the sha256-baseline snapshots of the `CreateRule` body
and the `matchRuleSQL` DDL block.

---

## 4. High Threats — Paragraph Summaries

### T-HITL-01 (Spoofing) — Operator Identity Not Bound to Approval Audit Row

The audit row's `actor_user_id` column is the non-repudiation anchor
for the approval decision. `ApprovalDecision.ActorUserID` is required
at the HTTP boundary (11-04); the audit row captures it post-wiring.
Phase 11 uses a placeholder identity (the chatHandler's session-bound
user); Phase 12 wires real auth. Mitigation is verified by manual
audit sampling (the audit row's `actor_user_id` column is populated
on every resolution row).

### T-HITL-02 (Denial of Service) — Pending Approval Queue Grows Unbounded

A malicious or malfunctioning agent could flood the pending-approval
queue, forcing the operator to triage thousands of interrupts.
Guardrail #5 in 11-AI-SPEC §6 throttles when
`pending_approval_count > 10` per session; the per-session counter is
derived from the audit table. Mitigation is verified by manual review
of `cmd/spike-history` metrics (the frozen gate's report carries the
pending count).

### T-HITL-03 (Denial of Service) — review_report Overwrite Without Warning

A `review_report` write whose `report_id` matches an existing audit
row's `frozen_args.report_id` could silently overwrite a prior report.
The UI requires explicit confirmation on this collision; rule auto-
pass on `review_report` is opt-in per Skill (default = always
interrupt). Mitigation is verified by manual UI walkthrough (the
ApprovalCard's overwrite-confirmation state).

### T-HITL-06 (Repudiation) — Concurrent Approve/Deny Cross-Device Race

Two operators (or the same operator on two devices) could approve and
deny the same `approval_id` concurrently. The `approval_id ==
tool_call_id` PK UNIQUE constraint (D11-C4) turns the race into a
UNIQUE constraint error — exactly one INSERT wins. The Runner's
`id2ResumeDataUsed[id]=true` idempotency moat (eino-internal) ensures
the second `ResumeWithParams` is a no-op. The second POST returns
200 + `state="already_resolved"` (not an error — idempotent success).
Verified by `TestConcurrentApproveDenyIdempotency` (11-04).

---

## 5. Regulatory Cross-References

Phase 11's threat model supports the following compliance frameworks
(see 11-AI-SPEC §1b for the full regulatory context):

- **SOC 2 CC7.2 / CC7.3** — System monitoring + change management.
  The audit chain (T-HITL-04 mitigation) is the change-management
  evidence; the approval gate is the change-control boundary.
- **ISO 27001 A.8.15** — Logging. The INSERT-only audit table +
  the fail-closed write path satisfy the logging-integrity control.
- **GDPR Art 30** — Records of processing activities. The audit
  row's `actor_user_id` + `frozen_args` columns are the processing-
  activity record for each write.
- **PIPL Art 24** — Automated decision-making transparency. The
  ApprovalCard surfaces the agent's intended write to the operator
  BEFORE execution, satisfying the transparency requirement.

---

## 6. Acceptance Criteria

Phase 11 ships ONLY when all 4 Critical mitigations pass the
acceptance gate (`bash scripts/verify-phase-11.sh`):

- [ ] **T-HITL-04** — `TestFailClosedAudit` (Stage 1) +
  `TestThreatModelCriticalMitigations/T-HITL-04` (Stage 5) PASS.
- [ ] **T-HITL-05** — `TestToolWhitelistMatchesSkillWrites` +
  `TestPromptInjectionResistance` (Stage 1) +
  `TestThreatModelCriticalMitigations/T-HITL-05` (Stage 5) PASS.
- [ ] **T-HITL-07** — `TestApprovalArgsByteEquality` (Stage 1) +
  `TestThreatModelCriticalMitigations/T-HITL-07` (Stage 5) PASS.
- [ ] **T-HITL-08** — `TestApprovalRulesScope` (Stage 1) +
  `TestThreatModelCriticalMitigations/T-HITL-08` (Stage 5) PASS.

A single failure on any of the above reopens a Critical threat to
production; the gate exits 1 and the phase is NOT shipped.

---

## 7. Open Risks (Deferred to v0.9 / Phase 12+)

Phase 11 explicitly does NOT cover:

- **Multi-user scenarios** — the current threat model assumes one
  operator per session. Multi-operator approval (quorum, delegation)
  is Phase 12+.
- **Async / webhook approval** — the approval gate is synchronous
  (operator clicks in the UI). Async approval (email, webhook, Slack)
  is Phase 12+.
- **Global-scope rules** — the D11-D3 binary scope (session | skill)
  deliberately excludes a global scope. A global-scope rule (apply
  to all Skills + all sessions) is a v0.9 decision requiring a
  separate threat-model review.
- **Cross-Skill tier leakage audit** — the D11-B1 whitelist prevents
  cross-Skill TOOL leakage, but the audit chain does not currently
  track cross-Skill TIER leakage (a fact written by Skill A surfaced
  in Skill B's retrieval). Phase 12+.
- **Supply-chain threats (T-HITL-SC)** — go.mod pins eino v0.9.14 +
  eino-ext v0.1.13. New package introductions require slopcheck +
  blocking human checkpoint per the T-{phase}-SC pattern. Not a
  Phase 11 acceptance gate item.

---

## 8. Change Log

| Date | Change |
|------|--------|
| 2026-08-20 | Phase 11 acceptance gate (11-08): initial STRIDE register; reviewed quarterly by Internal Audit Specialist. 4 Critical (T-HITL-04/05/07/08) + 4 High (T-HITL-01/02/03/06) threats mapped to named test gates in `scripts/verify-phase-11.sh`. |
