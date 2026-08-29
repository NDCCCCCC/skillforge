// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 1 — extend wire-types.ts with PlanTracePayload + PlanTodo
// + `data-plan-trace` discriminator. D15 lockstep paired commit with
// 12-02 BE (internal/wire/chunk.go: PlanTracePayload + ValidTypes
// ["data-plan-trace"]). The constant string "data-plan-trace" is the
// lock — if BE adds it without TS mirror, D15 alarm fails; vice versa.
//
// Plan 04-01 Task 1 — TypeScript mirror of internal/wire/chunk.go.
//
// THIS FILE IS THE COMPILE-TIME DRIFT ALARM for the wire contract.
// It mirrors the Go UIMessageChunk discriminated union WITH the three
// documented corrections baked in:
//
//   Pitfall 1: terminal chunk uses type:"finish" (NOT "finish-message").
//              Verified at ai@7.0.59/dist/index.d.ts line 2407. The Go
//              spike wire.ValidTypes still carries the legacy name;
//              P4 fixtures MUST emit "finish" so the SDK parser
//              doesn't silently drop the terminal chunk (leaving
//              useChat.status in 'streaming' forever).
//
//   Pitfall 2: data-* custom chunks carry payload on .data (NOT .policy).
//              Verified at ai@7.0.59/dist/index.d.ts lines 2267-2273.
//              DataUIMessageChunk shape is `{type:'data-${NAME}',
//              id?, data, transient?}`. Fixtures using .policy pass
//              through the prefix match but the payload lands on
//              part.data === undefined.
//
//   Pitfall 3: Tool parts are `tool-${name}` or `dynamic-tool`
//              (NOT "tool-invocation"). Verified at ai-sdk.dev/docs/
//              ai-sdk-ui/chatbot-tool-usage. The "tool-invocation"
//              discriminator does not exist in v7; code switching on
//              it falls through to the silent-drop default.
//
// The `_assert` type alias is a compile-time check that every variant
// of SkillForgeChunk is assignable to UIMessageChunk. If AI SDK
// upstream changes a discriminator name, this file fails to compile
// and CI catches the drift BEFORE any fixture is shipped.
//
// Plan 04-04 (2026-08-12) — the finishReason field on the `finish`
// variant was narrowed from a free-form `string` to the SDK's
// `FinishReason` enum (imported from 'ai') to align with the
// UIMessageChunk `finish` arm (ai@7.0.59/dist/index.d.ts lines
// 2407-2409, where FinishReason = 'stop' | 'length' |
// 'content-filter' | 'tool-calls' | 'error' | 'other' per line 124).
// This STRENGTHENS D15: the finish variant is now genuinely
// assignable to UIMessageChunk's finish arm, not merely tolerated by
// conditional-type distribution over the SkillForgeChunk union.

import type { FinishReason, UIMessageChunk } from 'ai';
import type { Refusal, SubprogressStep, Verdict } from './verdict';

// ----------------------------------------------------------------------------
// Decision enum constants — Plan 12-05 + 12-07 BE-side string values
// ----------------------------------------------------------------------------

// Plan 12-05 BE (P12-L3) — forced_stop is the budget-gate trigger
// emitted by internal/runtime/trust_stage.go when max_iter / token /
// loop layers trip (D11-C3 fail-closed + audit-first row written
// BEFORE the data-decision chunk). Mirrored on the TS side so the FE
// RealChatTransport dispatches plan.markPlanStatus(runId, 'forced_stop')
// on receipt. The discriminator (kebab-case data-decision) is unchanged
// (D15 invariant); only the decision STRING enum widens.
export const DecisionForcedStop = 'forced_stop' as const;

// Plan 12-07 BE (12-04 plan_api.go planApproveHandler) — plan-approved
// fires on the SSE stream after the plan approval POST succeeds
// (audit row written, Runner.ResumeWithParams drained, decision emitted).
// The TS side dispatches approval.markApproved + plan.markPlanStatus.
export const DecisionPlanApproved = 'plan-approved' as const;

// Plan 12-07 BE (12-04 plan_api.go planApproveHandler deny path) —
// plan-denied fires when the operator denies the plan (reason required,
// HITL-02 contract; >= 3 chars enforced client + server).
export const DecisionPlanDenied = 'plan-denied' as const;

// Plan 13-03 BE (P13-L6 fanout audit substrate) — fanout_dispatched
// fires on the SSE stream immediately BEFORE each sub-agent's Run
// method is invoked (T-13-03-6 audit-first lifecycle). The TS side
// surfaces it for the PlanInspector timeline (FAN-04 SC-4).
export const DecisionFanoutDispatched = 'fanout_dispatched' as const;

// Plan 13-03 BE — fanout_completed fires on the SSE stream AFTER each
// sub-agent's Run returns successfully AND per-sub-agent
// ApplyTrustStage grounded the verdict (FAN-04 SC-4 grounding proof).
export const DecisionFanoutCompleted = 'fanout_completed' as const;

// Plan 13-03 BE — fanout_failed fires when the sub-agent's Run
// returns an error, panics, or times out (Pitfall 4 mitigation). The
// Reason field on the payload carries the sub-agent's error message.
export const DecisionFanoutFailed = 'fanout_failed' as const;

// Phase 16 / Plan 16-04 — P16/D16-B3 kpi_call data-citation decision.
// When kpi_call produces a structured snapshot, the decision callback
// emits a data-decision chunk with Decision=kpi_reference and DataRef
// carrying the aggregation metadata. The data-decision discriminator is
// unchanged — this is a new VALUE in the Decision string enum, NOT a new
// wire chunk type (D16 overload pattern, same as P13-L7 fanout_*).
// The TS mirror adds the matching literal under D15 lockstep.
export const DecisionKPIReference = 'kpi_reference' as const;

// ----------------------------------------------------------------------------
// DecisionPayload — mirror of internal/wire/chunk.go DecisionPayload
// ----------------------------------------------------------------------------

// Plan 03-01 Task 1 — TS mirror of the Go wire.DecisionPayload struct.
// Carried inside a `data-decision` chunk's `.data` field. The agent
// package (Plan 03-03) populates this via Eino callbacks so the React
// UI can render the autonomous per-hop decisions (D3-03 / SC-1
// observability substrate).
//
// Plan 05-01 Task 1 (D5-04): extended with skillScores / selected /
// ambiguous for the skill-router observability surface, plus two new
// decision enum values (`route`, `counter_evidence`) used by the
// router/counter-evidence Skill flows.
//
// Field semantics match the Go struct 1:1 — JSON tags are camelCase
// (matches the Go struct's `json:"toolCallId,omitempty"` etc.). All
// additive fields are optional (omitempty on the Go side, `?:` here)
// so legacy 5-field payloads from Phase 3/4 still parse cleanly.
export interface SkillScore {
  /** Directory name of the Skill (e.g. "compliance"). */
  skillId: string;
  /** Human-readable name from Manifest.Description. */
  skillName: string;
  /** Cosine-similarity score (range [0.0, 1.0]; BGE-M3 default). */
  score: number;
  /** Exactly one entry has selected=true unless DecisionPayload.ambiguous. */
  selected: boolean;
}

// Plan 05-06 / SC-6 / D5-13 — one 历史-tier annotation row carried inside
// a counter_evidence DecisionPayload. The trust moat (D5-13) lives in
// the BE function signature: emitCounterEvidence never receives the
// verdict's chunksByID map; this struct is annotation-only metadata.
export interface ConflictChunk {
  /** Numeric chunk id rendered as decimal string (matches Hit.ID). */
  chunkId: string;
  /** Always "历史" today; kept on the struct for forward-compat. */
  tier: string;
  /** Slash-normalized corpus path (e.g. "history/2023-policy.md"). */
  source: string;
  /** 1-line text snippet for the annotation row (rendered as text, never HTML). */
  snippet: string;
  /** Epoch-seconds effective_from (0 / undefined = perpetual). */
  effectiveFrom?: number;
  /** Epoch-seconds effective_to (0 / undefined = perpetual). */
  effectiveTo?: number;
}

// Phase 17 — D17-B2 sparkline per-point reference
export interface PointReference {
  /** 0-based index in sparkline points array */
  pointIndex: number;
  /** Period string (e.g. "2026-07") */
  period: string;
  /** The numeric value */
  value: number;
  /** Non-zero when this point was k_min suppressed */
  kMinApplied?: number;
}

// Phase 18 / Plan 18-02 — D18-B3 report_published card payload
// (WR-02 18-REVIEW lockstep fix). Carried inside DecisionPayload.
// reportCard on a data-decision chunk whose decision is
// 'report_published' — NEVER a standalone chunk type (the BE's
// wire.ValidTypes has no 'report_published'; the old
// `{type:'report_published'}` union member was dead wire shape).
// Mirrors internal/wire/chunk.go ReportCard field-for-field.
export interface ReportCard {
  /** Report title (from the template). */
  title: string;
  /** e.g. "2026-08". */
  period: string;
  /** v1, v2, ... */
  version: number;
  /** /reports/<period>/v<N>.html — same-origin path. */
  url: string;
}

// Phase 16 / Plan 16-04 — P16/D16-B3 structured data-citation payload for
// kpi_reference decisions. Mirrors internal/wire/chunk.go DataReference
// field-for-field under D15 lockstep. When kpi_call produces a snapshot,
// the decision callback emits a data-decision chunk with DataRef carrying
// the aggregation metadata. rows body is NOT carried on SSE (volume
// control + agent already has the full snapshot from tool result; FE
// Phase 17 sparkline reads from kpi_calls audit rows — admin read-only).
export interface DataReference {
  /** KPI function name (registry key, e.g. "loan_overdue_rate_by_region"). */
  func: string;
  /** Caller-supplied parameters (enum/pattern-validated at tool boundary — D16-A3/D2). */
  args?: Record<string, unknown>;
  /** Data-set latest period watermark (system-generated — D16-B4; agent cannot forge). */
  asOfPeriod: string;
  /** Invocation timestamp (RFC3339 — result metadata, not query param). */
  asOfTs: string;
  /** Total rows in snapshot (including suppressed rows — snapshot row count invariant per D16-D1). */
  rowCount: number;
  /** Number of rows where k_min suppression was applied (value replaced with "suppressed" marker — D16-D1). */
  suppressed: number;
  /** The k_min threshold value in effect for this execution (file > dataset override > KMinDefault=5). */
  kMinApplied: number;
  // NEW (D17-B2): multi-point sparkline references
  references?: PointReference[];
}

export interface DecisionPayload {
  /** 1-indexed hop number (1 or 2 for max_hops=2). */
  step: number;
  decision:
    | 'retrieve'
    | 'answer'
    | 'refuse'
    | 'forced_stop'
    | 'route'
    | 'counter_evidence'
    // Plan 10-02 + Plan 10-03 — Phase 10 history-pipeline observability
    // events. The 4 history_* constants live in
    // internal/wire/chunk.go (DecisionHistoryProjected /
    // DecisionHistoryBudgetTruncated / DecisionHistoryQueryRewritten /
    // DecisionHistoryClassifierRefused); the DecisionPayload struct
    // stays structurally identical (D15 invariant — zero new wire
    // types). Only the decision enum widens by 4 string literals;
    // HistoryRewriteHint renders the history_query_rewritten variant.
    | 'history_projected'
    | 'history_budget_truncated'
    | 'history_query_rewritten'
    | 'history_classifier_refused'
    // Plan 11-06 — Phase 11 HITL approval-gate events (D15 string-enum
    // extension; zero new wire types, mirrors the Go side which keeps
    // DecisionPayload.Decision a plain string). approval_api.go emits
    // tool-resumed / tool-denied on the POST-approval stream; the
    // runner emits interrupted when a write tool hits the approval
    // gate and rule-matched when a remember rule auto-passes.
    | 'interrupted'
    | 'tool-resumed'
    | 'tool-denied'
    | 'rule-matched'
    // Plan 12-05 + 12-07 — forced_stop + plan-approved + plan-denied
    // string extensions. forced_stop was added in 12-05 BE (P12-L3
    // budget gate trigger); 12-07 adds plan-approved + plan-denied
    // (12-04 plan_api.go planApproveHandler). The discriminator
    // (kebab-case data-decision) is unchanged — only the decision
    // STRING enum widens by 3 literals. D16 overload pattern + D15
    // lockstep with internal/wire/chunk.go DecisionPayload.
    | 'forced_stop'
    | 'plan-approved'
    | 'plan-denied'
    // Plan 13-03 — FAN-04 SC-4 fanout audit substrate string extensions.
    // The 3 fanout_* enum values mirror the audit.action enum
    // (ActionFanoutDispatched | ActionFanoutCompleted | ActionFanoutFailed).
    // The discriminator (kebab-case data-decision) is unchanged; only
    // the decision STRING enum widens by 3 literals. D16 overload
    // pattern + D15 lockstep with internal/wire/chunk.go DecisionPayload.
    // RealChatTransport (web/src/transport/) routes these to the
    // PlanInspector timeline so the operator sees the sub-agent
    // trace alongside the parent decision trace.
    | 'fanout_dispatched'
    | 'fanout_completed'
    | 'fanout_failed'
    // Phase 16 / Plan 16-04 — P16/D16-B3 kpi_call data-citation decision.
    // When kpi_call produces a structured snapshot, the decision callback
    // emits a data-decision chunk with Decision=kpi_reference. The
    // discriminator (kebab-case data-decision) is unchanged; only the
    // decision STRING enum widens. D16 overload pattern + D15 lockstep
    // with internal/wire/types.go DecisionKPIReference.
    | 'kpi_reference'
    // Phase 18 / Plan 18-02 — D18-B3 + D18-D4 report publication events
    // (WR-02 18-REVIEW lockstep fix). 'report_published' rides the
    // approval/resume data-decision stream with payload.reportCard set;
    // 'plan_change_requested' resolves a request_change third state.
    // Both mirror wire constants in internal/wire/chunk.go
    // (DecisionReportPublished / DecisionPlanChangeRequested).
    | 'report_published'
    | 'plan_change_requested';
  /** Links the decision to a specific tool invocation (may be empty). */
  toolCallId?: string;
  /** The (possibly refined) query for a retrieve decision. */
  query?: string;
  /** Model-reported confidence for an answer decision. */
  confidence?: number;
  /** D5-04 — populated only on `route` decisions. */
  skillScores?: SkillScore[];
  /** D5-04 — SkillID chosen on a `route` decision. */
  selected?: string;
  /** D5-04 — true when the top-2 score gap is below the router threshold. */
  ambiguous?: boolean;
  /** Plan 13-03 / P13-L7 — populated only on `fanout_*` decisions. */
  runPath?: string;
  /** Plan 13-03 / P13-L7 — populated only on `fanout_*` decisions. */
  transferFrom?: string;
  /** Plan 13-03 / P13-L7 — populated only on `fanout_*` decisions. */
  transferTo?: string;
  /** Plan 05-06 / D5-13 — populated only on `counter_evidence` decisions.
   *  Carries the 历史-tier annotation rows (NEVER part of the verdict's grounding). */
  conflictChunks?: ConflictChunk[];
  /** P16 — populated only on `kpi_reference` decisions. Structured data-citation
   *  metadata (func/args/asOf/rowCount/suppressed/kMinApplied). Mirrors Go
   *  json:"dataRef,omitempty" — nil when decision is not kpi_reference.
   *  rows body NOT carried on SSE (volume control + agent already has full
   *  snapshot; FE Phase 17 sparkline reads from kpi_calls audit rows). */
  dataRef?: DataReference;
  /** P18 / D18-B3 — populated only on `report_published` decisions.
   *  The published report card (title/period/version/url). Mirrors Go
   *  json:"reportCard,omitempty" (WR-02 18-REVIEW lockstep fix). */
  reportCard?: ReportCard;
}

// ----------------------------------------------------------------------------
// RedlinePayload — mirror of internal/wire/redline.go RedlinePayload
// ----------------------------------------------------------------------------

// Plan 05-01 Task 1 — TS mirror of the 18th wire type's payload
// (D5-14). Carried inside a `data-redline` chunk's `.data` field.
// The compliance Skill emits one of these per clause-level rewrite.
//
// V8 mitigation (05-UI-SPEC §Registry Safety): the three core fields
// are REQUIRED — a truncated payload fails JSON parse on the Go side
// via RedlinePayload.UnmarshalJSON (which uses pointer-fields to
// distinguish absent from empty). The TS mirror reflects this by
// typing them as required (`originalClause` / `proposedRewrite` /
// `rationale` carry no `?:` modifier).
export interface RedlinePayload {
  /** Optional stable id of the clause (e.g. "C-7.3.2") — links redline to citation. */
  clauseId?: string;
  /** REQUIRED — exact text of the original clause in the source document. */
  originalClause: string;
  /** REQUIRED — agent's proposed replacement text. */
  proposedRewrite: string;
  /** REQUIRED — human-readable explanation of why the rewrite is proposed. */
  rationale: string;
}

// ----------------------------------------------------------------------------
// PlanTodo + PlanTracePayload — mirror of internal/wire/chunk.go
// (Plan 12-02 BACKEND half; FE mirror lives here for D15 lockstep)
// ----------------------------------------------------------------------------

// Plan 12-02 / 12-06 — TS mirror of the Go wire.PlanTodo struct.
// Mirrors eino/adk/prebuilt/deep.TODO field-for-field but is
// SkillForge-owned (P12-L7). Status enum matches eino upstream
// jsonschema tag enum: "pending" | "in_progress" | "completed".
export interface PlanTodo {
  /** Stable todo id (UUID-shaped; matches audit row PK). */
  id: string;
  /** Human-readable description of the step the agent plans to do. */
  content: string;
  /** Optional present-tense verb form rendered when status==='in_progress'. */
  activeForm?: string;
  status: 'pending' | 'in_progress' | 'completed';
}

// Plan 12-02 / 12-06 — TS mirror of the Go wire.PlanTracePayload struct.
// Carried inside a `data-plan-trace` chunk's `.data` field. The
// internal/agent/deep_audit.go writeTodosAuditedTool emits one of these
// per Tool call so the React UI can render the live planning trace
// (PLAN-02 BE substrate). Field semantics 1:1 with the Go struct.
//
//   - RunID: scopes the trace to one conversation (FE mirrors it as
//     the plan slice's planByRunId key).
//   - Todos: the FULL post-write snapshot, not a delta. The trace is
//     idempotent — each chunk replaces the prior snapshot. AI SDK v5
//     buffers data-* chunks until message ends; the FE mirrors the
//     post-write snapshot on each chunk (PLAN-02 SC-2).
//   - Ts: epoch millis when write_todos was invoked.
export interface PlanTracePayload {
  runId: string;
  todos: PlanTodo[];
  ts: number;
}

// ----------------------------------------------------------------------------
// SkillForgeChunk — the union emitted on the wire
// ----------------------------------------------------------------------------

export type SkillForgeChunk =
  // Lifecycle chunks (Pitfall 1 corrections)
  | { type: 'start'; messageId?: string; messageMetadata?: unknown }
  | { type: 'finish'; finishReason?: FinishReason; messageMetadata?: unknown }
  // Text streaming
  | { type: 'text-start'; id: string; providerMetadata?: unknown }
  | { type: 'text-delta'; id: string; delta: string; providerMetadata?: unknown }
  | { type: 'text-end'; id: string; providerMetadata?: unknown }
  // Plan 260815-cos: draft streaming channel (LLM tokens before grounding).
  // NOT routed through AI SDK v5 DefaultChatTransport parser — the
  // MockChatTransport keeps draft-* frames in a separate buffer so they
  // cannot pollute message.parts[].text. ID namespace "draft-1" isolates
  // from "msg-1" text parts so useChat sees zero draft frames.
  //
  // D15 drift alarm: text-delta-draft is structurally narrower than
  // AI SDK v5's text-delta variant ({ type, id, delta } vs
  // { type, id, delta, providerMetadata? }). The narrower field set
  // stays assignable because every required field on SkillForgeChunk
  // exists on the SDK shape; the SDK's optional providerMetadata is
  // simply not asserted here.
  | { type: 'text-delta-draft'; id: string; delta: string }
  | { type: 'text-delta-draft-end'; id: string }
  // Reasoning streaming
  | { type: 'reasoning-start'; id: string; providerMetadata?: unknown }
  | { type: 'reasoning-delta'; id: string; delta: string; providerMetadata?: unknown }
  | { type: 'reasoning-end'; id: string; providerMetadata?: unknown }
  // Tool parts (Pitfall 3 corrections — tool-${name} + dynamic-tool)
  | { type: 'tool-input-start'; toolCallId: string; toolName: string; providerMetadata?: unknown }
  | { type: 'tool-input-delta'; toolCallId: string; inputTextDelta: string }
  | { type: 'tool-input-available'; toolCallId: string; toolName: string; input: unknown }
  | { type: 'tool-output-available'; toolCallId: string; output: unknown }
  | { type: 'tool-output-error'; toolCallId: string; errorText: string }
  // Source parts (citations)
  | { type: 'source-url'; sourceId: string; url: string; title?: string; providerMetadata?: unknown }
  | { type: 'source-document'; sourceId: string; mediaType?: string; title: string; filename?: string; providerMetadata?: unknown }
  // Custom data parts (Pitfall 2 — payload on .data)
  | { type: 'data-policy'; id?: string; data: Verdict | Refusal }
  | { type: 'data-subprogress'; id?: string; data: SubprogressStep }
  // Plan 12-02 / 12-06 — live planning trace (PLAN-02 SC-2). Paired
  // commit with BE internal/wire/chunk.go ValidTypes["data-plan-trace"]
  // (D15 lockstep invariant). The FE plan slice stores the post-write
  // snapshot keyed by runId; PlanTracePart renders it. Idempotent —
  // each chunk carries the FULL current todo list.
  | { type: 'data-plan-trace'; id?: string; data: PlanTracePayload }
  // Plan 03-01 — agent per-hop decisions (ARAG-01 / SC-1 observability).
  // Strict refinement of AI SDK v5 DataUIMessageChunk — assignability
  // holds so the D15 drift alarm stays green.
  //
  // Plan 18-02 / WR-02 (18-REVIEW): the report card is NOT a chunk
  // type — it rides data-decision as
  // DecisionPayload{decision:'report_published', reportCard}. The old
  // `{type:'report_published'}` union member was a wire shape the BE
  // can never emit (wire.ValidTypes rejects it pre-write, T-1-11) and
  // has been removed.
  | { type: 'data-decision'; id?: string; data: DecisionPayload }
  // Plan 05-01 — agent's clause-level redline proposals (D5-14). The
  // 18th wire type. Required-field enforcement happens on the Go side
  // (RedlinePayload.UnmarshalJSON — V8 mitigation). The TS mirror
  // declares them as required types so a truncated JSON object
  // matches the `(p: RedlinePayload) => void` consumer signature
  // and the FE never tries to render a blank-diff redline card.
  | { type: 'data-redline'; id?: string; data: RedlinePayload }
  // Step boundaries
  | { type: 'start-step' }
  | { type: 'finish-step' }
  // Error
  | { type: 'error'; errorText: string };

// ----------------------------------------------------------------------------
// Compile-time assignability assertion
// ----------------------------------------------------------------------------

// If AI SDK upstream adds a required field to any chunk variant, or
// renames a discriminator, this assertion fails to compile. The TS
// compiler is the first line of defense against wire drift.
type AssertAssignable<A, B> = A extends B ? (B extends A ? true : never) : never;
type _driftCheck = AssertAssignable<SkillForgeChunk, UIMessageChunk>;
// Phase 19 / Plan 19-03 — Report Run type (PORT-01/PORT-03).
// Mirrors internal/store.ReportRunRow field-for-field.
export interface ReportRun {
  run_id: string;
  skill_id: string;
  period: string;
  cron_expr: string;
  fired_at: string;
  kpi_snapshots: string;
  output_url: string;
  status: 'success' | 'failed' | 'skipped_duplicate' | 'eval_gate_blocked';
  drift_detected: boolean;
  created_at: string;
}

// Surface the assertion — TS will error here if assignability breaks.
// (No runtime cost; this exists purely for compile-time drift check.)
export const _driftCheckTrue: _driftCheck = true;
void _driftCheckTrue; // satisfy "declared but unused" if no other consumer
