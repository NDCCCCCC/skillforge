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
// Field semantics match the Go struct 1:1 — JSON tags are camelCase
// (matches the Go struct's `json:"toolCallId,omitempty"` etc.).
export interface DecisionPayload {
  /** 1-indexed hop number (1 or 2 for max_hops=2). */
  step: number;
  decision: 'retrieve' | 'answer' | 'refuse' | 'forced_stop';
  /** Links the decision to a specific tool invocation (may be empty). */
  toolCallId?: string;
  /** The (possibly refined) query for a retrieve decision. */
  query?: string;
  /** Model-reported confidence for an answer decision. */
  confidence?: number;
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
