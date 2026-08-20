# Phase 11 — Plan Outline (chunked mode, MVP vertical slices)

**Phase:** 11 — 写工具 + 审批门（P8.2 — HITL-01..08）
**Mode:** mvp (vertical slices — UI/API/DB end-to-end, NOT horizontal layers)
**Goal:** agent 调用写工具前经人在环审批——流内审批卡 / 跨刷新审批态 / 3 行 INSERT-only 审计 / "批的 A 不会执行成 B"
**Constraints:** 16 locked decisions (D11-A1..D11-D4) + trust staircase + D8-04 legacy interrupt lint + D-04 transport-swap + D14 Zustand no-messages + D15 wire-type alarm

---

## Outline Table

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| **11-01** | **BE Runner-ization foundation** — rewrite chatHandler → `adk.NewRunner().Run(ctx, []schema.Message{...history, query})`; split `internal/server/pipeline.go` → 5 files (prompts.go / runner_sse.go / tools/retrieve.go / callbacks/grounding.go / callbacks/classifier.go); wire trust staircase (BeforeAgent trust floor / BeforeModel D10-A1 / AfterModel D2-01); shared ctx between Runner.Run + SSE writer preserves `TestCancelPropagationMidTool` <200ms | W1 | (none) | (foundation — no requirement IDs; D11-A1/A2/A3/A4 + D8-04 + D8-06 SC-5 stub upgrade) |
| **11-02** | **BE HITL Tool infrastructure + first write tool** — `ApprovalDecision` typed schema (D11-B2); `internal/agent/hitl/registry.go` single source of truth with `AllowedWriteTargets=["memory:facts","review_report"]` + `hitlHandlers` map (D11-B3); `BuildToolsForSkill()` per-turn whitelist pure function = `AllowedWriteTargets ∩ manifest.Writes` (D11-B1); `schema.RegisterName[*ApprovalDecision]` dual registration; first write tool `hitl_memory_facts.go` (HITL-06 partial); `TestToolWhitelistMatchesSkillWrites` + `TestNoLegacyInterruptAPI` WalkDir extended | W2 | 11-01 | **HITL-05** (writes: frontmatter code-level enforcement), **HITL-06** (first write tool: memory:facts, partial) |
| **11-03** | **BE Audit 3-row lifecycle + Checkpoint fail-closed** — `internal/agent/hitl/audit_writer.go` sync INSERT `action_pending_approval` BEFORE `tool.StatefulInterrupt` returns; distinct code path from read-path swallow (D11-C3); `action_approved` / `action_rejected` / `action_rule_revoked` enum added to audit table DDL; checkpoint table ALTER ADD COLUMN `skill_version` + `prompt_hash` (D11-C2); `SQLiteStore.Delete(id)` (D11-C2 expire path); `approval_id == tool_call_id` PK UNIQUE (D11-C4); 6 hermetic tests (`TestFailClosedAudit` / `TestAuditChainThreeRowLifecycle` / `TestApprovalArgsByteEquality` / `TestConcurrentApproveDenyIdempotency` / `TestCheckpointDeleteExpired` / `TestApprovalIDIdentityInvariant`) | W2 | 11-01 | **HITL-04** (INSERT-only 3-row audit lifecycle + frozen_args snapshot byte-identity + fail-closed + tool_call_id idempotency) |
| **11-04** | **BE Approval Dual endpoint + Runner.ResumeWithParams + review_report write tool** — `GET /api/session/:id/approval` (returns pending row + state active/expired/auto_passed) (D11-C1); `POST /api/session/:id/approval/:approvalId` (validate `ApprovalDecision` → audit MarkApproved/MarkRejected → `runner.ResumeWithParams(ctx, cpID, &ResumeParams{Targets: {approvalID: &decision}})`); second write tool `hitl_review_report.go`; on-resume expire-check compares `checkpoint.skill_version + prompt_hash` against active Skill (D11-C2); `TestExpiredCheckpointUIState` + `TestCancelPropagationMidToolRunner` <200ms re-locked | W3 | 11-02, 11-03 | **HITL-01** (SSE clean close + ApprovalCard what/tool/why — BE emit `data-decision{decision:"interrupted"}`), **HITL-02** (approve/deny once + structured tool-call binding + deny reason required), **HITL-03** (cross-refresh state survival + skill_version/prompt_hash expire UI state), **HITL-06** (review_report write tool complete) |
| **11-05** | **BE Edit-before-approve + approve-and-remember rules** — HITL Tool `InvokableRun` Pass 2 uses `data.FrozenArgs` (possibly edited) instead of storedArgs, sha256(frozenArgs) re-computed at execution (3-way byte equality invariant) (HITL-07); `internal/store/sqlite/approval_rules.go` independent table with PK=(target, args_hash, skill_id, scope_session_id) (D11-D2); `Rule.Match()` sha256-based per-call (D11-D1); session/skill binary scope radio (D11-D3); `GET /api/session/:id/approval-rules?scope=session|skill` + `DELETE /api/session/:id/approval-rules/:ruleId` (revoke writes `action_rule_revoked` audit row in same tx) (D11-D4 partial); `TestApprovalRulesScope` + `TestApprovalRulesRevoke` | W3 | 11-02, 11-03 | **HITL-07** (edit-before-approve, modified args flow to Tool), **HITL-08** (approve-and-remember, session/skill binary scope, viewable + revocable — BE part) |
| **11-06** | **FE ApprovalCard + Edit + Decision flow** — Zustand `approval` slice (NOT in messages — D14 clean); `ApprovalCard.tsx` store-derived overlay at chat-column interrupt position (z-25); 6 visual states (pending / approved / denied / expired / auto-passed / failed); FrozenArgs JSON preview collapsed by default; `批准一次` / `批准并记住` / `拒绝` button trio (D11-D3 scope radio conditional on 批准并记住); edit mode (HITL-07) textarea over FrozenArgs; deny prompt requires ≥3 chars reason; `SkillPicker` aria-disabled + tooltip when `approval.pending !== null` (D11-B4); real-chat-transport parses `data-decision{decision:"interrupted|tool-resumed|tool-denied|rule-matched"}` extension; `TestTransportSwapWithApprovalCard` + `TestWireTypeCompilation` D-04/D15 invariants | W4 | 11-04 | **HITL-01** (UI side: store-derived ApprovalCard with what/tool/frozenArgs/why), **HITL-02** (approve/deny buttons + deny reason prompt), **HITL-07** (FE edit mode UI) |
| **11-07** | **FE Approval Rules panel + AuditTimelineTab + cross-refresh resume banner** — LeftSidebar 5th section `ApprovalRulesPanel.tsx` (D11-D4 — target/scope/last_used_at row + revoke X); `GET /api/session/:id/audit` + `AuditTimelineTab.tsx` as 6th tab in DetailPanel (between 结论 + 引用原文); 4 status colors (pending=indigo / approved=green / rejected=red / revoked=yellow) + filter chips; on App mount + session change: parallel `GET /api/session/:id/approval` + `/approval-rules` + `/history` — pending → store `approval.pending` set; auto-passed → terminal gray; expired → yellow state + 重新发起 button; 3s auto-dismiss resume banner (HITL-03 complete); 16 vitest tests per UI-SPEC §Test Lock Requirements | W4 | 11-03 (audit endpoint), 11-05 (rules endpoint) | **HITL-03** (cross-refresh resume UI complete), **HITL-08** (FE part: viewable rules + revoke X button + audit chain transparency) |
| **11-08** | **E2E integration + hermetic test gates + frozen eval baseline re-lock** — single-binary smoke E2E (cmd/spiked or new spike-runner): interrupt → cross-refresh → approve → resume → audit chain byte-identical; `TestCancelPropagationMidToolRunner` <200ms lock preserved across Runner migration; `TestNoLegacyInterruptAPI` WalkDir extended to scan `internal/agent/{callbacks,tools,hitl}/`; `cmd/spike-history --frozen --archive` Phase 11 acceptance gate (faithfulness 0.972 ±0.05, refusal 95% ±10pp, injection 0/10, lock byte-identity); `eval/golden/hitl.jsonl` 75 cases (HITL-specific golden set); threat-model STRIDE register (HITL-04 audit fail-closed + HITL-05 prompt-injection + HITL-07 frozen_args drift + HITL-08 scope leak as Critical) | W5 | 11-06, 11-07 | **HITL-01..08** (Phase 11 acceptance — all 8 requirements verified end-to-end) |

---

## Plan Count

**8 plans** in **5 waves**

## Wave Structure Summary

| Wave | Plans | Autonomous | Purpose |
|------|-------|------------|---------|
| 1 | 11-01 | yes | BE Runner-ization foundation (sequential prerequisite for all) |
| 2 | 11-02, 11-03 | yes, yes | BE HITL Tool infra + first write tool **⟂** BE Audit + Checkpoint fail-closed (different file ownership) |
| 3 | 11-04, 11-05 | yes, yes | BE Approval Dual endpoint + Runner.ResumeWithParams **⟂** BE edit-before-approve + approve-and-remember rules (different file ownership) |
| 4 | 11-06, 11-07 | yes, yes | FE ApprovalCard + Decision flow **⟂** FE Rules panel + AuditTimelineTab + cross-refresh resume (different file ownership) |
| 5 | 11-08 | no (live acceptance gate) | E2E integration + hermetic gates + frozen eval baseline re-lock |

## Requirement Coverage

- **HITL-01** (stream approval card): 11-04 (BE emit) + 11-06 (FE render)
- **HITL-02** (approve/deny once + structured binding): 11-04 (BE endpoint) + 11-06 (FE buttons)
- **HITL-03** (cross-refresh + expire): 11-04 (BE expire check) + 11-07 (FE resume banner)
- **HITL-04** (3-row audit + frozen snapshot + fail-closed + idempotency): 11-03
- **HITL-05** (writes: code-level enforcement): 11-02
- **HITL-06** (memory:facts + review_report write tools): 11-02 (memory:facts) + 11-04 (review_report)
- **HITL-07** (edit-before-approve): 11-05 (BE three-way byte equality) + 11-06 (FE edit mode UI)
- **HITL-08** (approve-and-remember): 11-05 (BE rules + revoke) + 11-07 (FE panel + audit)

All 8 HITL requirement IDs are covered by at least one plan.

## File Ownership Map (parallelism safety)

```
W2:
  11-02 → internal/agent/hitl/{registry,approval_decision,hitl_memory_facts}.go + internal/agent/runner_setup.go
  11-03 → internal/agent/hitl/audit_writer.go + internal/store/sqlite/{checkpoint,audit,schema}.go
  ZERO overlap

W3:
  11-04 → internal/server/{approval_api,audit_api}.go + internal/agent/hitl/hitl_review_report.go
  11-05 → internal/store/sqlite/approval_rules.go + internal/server/approval_rules_api.go
  ZERO overlap

W4:
  11-06 → web/src/components/ApprovalCard.tsx + web/src/stores/app-store.ts (approval slice) + web/src/lib/real-chat-transport.ts
  11-07 → web/src/components/{ApprovalRulesPanel,AuditTimelineTab,SkillPicker,LeftSidebar}.tsx
  ZERO overlap (SkillPicker.tsx touched by both — explicit ordering: 11-06 reads s.approval.pending; 11-07 modifies SkillPicker disabled render → 11-07 depends on 11-06's slice shape)

  Actually: 11-07 also modifies SkillPicker.tsx for disabled state (D11-B4). To avoid conflict, 11-06 ships the slice shape only; 11-07 owns SkillPicker.tsx entirely. REVISED: 11-06 only touches stores/app-store.ts + ApprovalCard.tsx + real-chat-transport.ts; 11-07 owns all SkillPicker.tsx edits including the approval.pending read.

W5:
  11-08 → integration tests + spike runner + threat model + eval/golden/hitl.jsonl (no production code)
```

## Key Decision Lock References (immutable — do NOT re-ask in PLANs)

- **D11-A1** (Runner-ization one-line) → 11-01
- **D11-A2** (trust staircase 3 callbacks + HITL = Tool+StatefulInterrupt) → 11-01 + 11-02
- **D11-A3** (pipeline.go → 5 files) → 11-01
- **D11-A4** (shared ctx + SSE adapter; <200ms cancel preserved) → 11-01 + 11-04
- **D11-B1** (per-turn Tool whitelist pure function) → 11-02
- **D11-B2** (shared ApprovalDecision schema) → 11-02
- **D11-B3** (AllowedWriteTargets + hitlHandlers single file) → 11-02
- **D11-B4** (per-session Agent single + pending Skill picker disabled) → 11-02 + 11-06
- **D11-C1** (Dual endpoints GET/POST) → 11-04
- **D11-C2** (Snapshot dual-table + expire check + Delete) → 11-03 + 11-04
- **D11-C3** (audit_writer sync INSERT before StatefulInterrupt) → 11-03
- **D11-C4** (approval_id == tool_call_id) → 11-03
- **D11-D1** (Rule match = target + sha256(frozenArgs) + skill_id) → 11-05
- **D11-D2** (approval_rules independent table) → 11-05
- **D11-D3** (session/skill binary scope) → 11-05 + 11-06
- **D11-D4** (Left sidebar Rule panel + audit rule_revoked) → 11-05 (BE) + 11-07 (FE)

## Test Lock Requirements (UI-SPEC + AI-SPEC §Specifics)

All Phase 11 plans must ship these regression tests:

**Backend (Go hermetic):**
- `TestApprovalArgsByteEquality` (3-way equality preview == audit == Tool args) — 11-03
- `TestFailClosedAudit` (audit INSERT failure → Tool error + no checkpoint Set) — 11-03 (P0, no waiver)
- `TestAuditChainThreeRowLifecycle` (pending → approved/rejected chain) — 11-03
- `TestConcurrentApproveDenyIdempotency` (10 concurrent POSTs → 1 Tool execution) — 11-03
- `TestExpiredCheckpointUIState` (skill_version + prompt_hash mismatch → expire UI) — 11-04
- `TestApprovalRulesScope` (session vs skill binary) — 11-05
- `TestApprovalRulesRevoke` (DELETE + INSERT action_rule_revoked same tx) — 11-05
- `TestCheckpointDeleteExpired` (SQLiteStore.Delete + skill_version filter) — 11-03
- `TestApprovalIDIdentityInvariant` (approval_id == tool_call_id across audit/checkpoint) — 11-03
- `TestToolWhitelistMatchesSkillWrites` (per-turn pure function) — 11-02
- `TestPromptInjectionResistance` (writes:[] Skill → 0 write tool reach) — 11-02
- `TestCancelPropagationMidToolRunner` <200ms — 11-01 + 11-04 (re-locked)
- `TestNoLegacyInterruptAPI` WalkDir extended — 11-01 + 11-08 (final gate)

**Frontend (vitest):**
- `TestApprovalCardRendering` + `TestApprovalCardStateTransitions` (7 states) — 11-06
- `TestApprovalCardEditBeforeApprove` — 11-06
- `TestApprovalCardScopeRadio` (radio on 批准并记住 only) — 11-06
- `TestApprovalCardDenyRequiresReason` — 11-06
- `TestApprovalCardResumeBanner` — 11-07
- `TestApprovalCardExpiredState` + `TestApprovalCardAuditFailClosed` — 11-07
- `TestApprovalRulesPanelEmptyState` + `TestApprovalRulesPanelRevoke` + `TestApprovalRulesPanelReactiveUpdate` — 11-07
- `TestAuditTimelineTabRendering` + `TestAuditTimelineTabReadOnly` — 11-07
- `TestSkillSwitchGuardDisabled` + `TestSkillSwitchGuardReenabled` — 11-06
- `TestTransportSwapWithApprovalCard` (D-04 invariant preserved) — 11-06 + 11-08
- `TestWireTypeCompilation` (D15 invariant preserved) — 11-06

**Acceptance gate (11-08):**
- `cmd/spike-history --frozen --archive` (faithfulness 0.972 ±0.05, refusal 95% ±10pp, injection 0/10, lock byte-identity)
- `eval/golden/hitl.jsonl` 75 cases (interleave with Phase 10 frozen compliance.jsonl for 150 total)
- Threat-model STRIDE register (T-11-04 audit fail-closed + T-11-05 prompt-injection + T-11-07 frozen_args drift + T-11-08 scope leak as Critical)

---

## OUTLINE COMPLETE

**8 plans** in **5 waves** — vertical slices, MVP mode, FE+BE HITL same phase.

1. **11-01** — BE Runner-ization foundation (chatHandler → adk.Runner; pipeline.go → 5 files; trust staircase; <200ms cancel)
2. **11-02** — BE HITL Tool infrastructure (ApprovalDecision + registry + per-turn whitelist) + first write tool memory:facts
3. **11-03** — BE Audit 3-row lifecycle + Checkpoint fail-closed (sync audit_writer; skill_version+prompt_hash; Delete; idempotency)
4. **11-04** — BE Approval Dual endpoint + Runner.ResumeWithParams + review_report write tool + expire-check
5. **11-05** — BE Edit-before-approve + approve-and-remember rules (3-way byte equality; approval_rules table; session/skill scope)
6. **11-06** — FE ApprovalCard + Edit + Decision flow (Zustand approval slice; 6 visual states; scope radio; SkillPicker disabled)
7. **11-07** — FE Approval Rules panel + AuditTimelineTab + cross-refresh resume banner (LeftSidebar 5th section; DetailPanel 6th tab)
8. **11-08** — E2E integration + hermetic test gates + frozen eval baseline re-lock (acceptance gate; 75 HITL cases; STRIDE threat model)