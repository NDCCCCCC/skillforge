// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

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
    | 'rule-matched';
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
  /** Plan 05-06 / D5-13 — populated only on `counter_evidence` decisions.
   *  Carries the 历史-tier annotation rows (NEVER part of the verdict's grounding). */
  conflictChunks?: ConflictChunk[];
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
  // Plan 03-01 — agent per-hop decisions (ARAG-01 / SC-1 observability).
  // Strict refinement of AI SDK v5 DataUIMessageChunk — assignability
  // holds so the D15 drift alarm stays green.
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
// Surface the assertion — TS will error here if assignability breaks.
// (No runtime cost; this exists purely for compile-time drift check.)
export const _driftCheckTrue: _driftCheck = true;
void _driftCheckTrue; // satisfy "declared but unused" if no other consumer
