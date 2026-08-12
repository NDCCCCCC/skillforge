// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// hop-types — derived hop / tool-call / retrieval row types used by
// the AgentInspector surface. Built from useChat.message.parts by
// AgentInspector's deriveHops() helper.
//
// The shape is intentionally narrow: only the fields the inspector
// renders are populated. Tool args are kept as `unknown` and are
// stringified at render time — never injected as HTML
// (threat T-5-21 mitigation).

export type DecisionValue =
  | 'retrieve'
  | 'answer'
  | 'refuse'
  | 'forced_stop'
  | 'route'
  | 'counter_evidence';

export interface ToolCallRow {
  toolCallId: string;
  toolName: string;
  state: string;
  input: unknown;
  output: unknown;
  errorText?: string;
}

export interface RetrievalRow {
  /** Chunk ID surfaced via source-document parts. */
  chunkId: string;
  /** Optional filename from the source-document metadata. */
  filename?: string;
  /** Optional title from the source-document metadata. */
  title?: string;
}

export interface Hop {
  /** 1-indexed hop number (data-decision DecisionPayload.step). */
  step: number;
  decision: DecisionValue;
  query?: string;
  confidence?: number;
  toolCalls: ToolCallRow[];
  retrievals: RetrievalRow[];
  /** Concatenated reasoning text for this hop (may be empty). */
  reasoning: string;
}