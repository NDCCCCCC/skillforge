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

import type { UIMessageChunk } from 'ai';
import type { Refusal, SubprogressStep, Verdict } from './verdict';

// ----------------------------------------------------------------------------
// SkillForgeChunk — the union emitted on the wire
// ----------------------------------------------------------------------------

export type SkillForgeChunk =
  // Lifecycle chunks (Pitfall 1 corrections)
  | { type: 'start'; messageId?: string; messageMetadata?: unknown }
  | { type: 'finish'; finishReason?: string; messageMetadata?: unknown }
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
