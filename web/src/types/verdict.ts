// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 1 — TypeScript mirror of internal/verdict/* structs.
//
// Source of truth (Go):
//   - internal/verdict/verdict.go       lines 57-62 (Verdict struct)
//   - internal/verdict/citation.go      lines 28-44 (Citation struct)
//   - internal/verdict/citation.go      lines 56-73 (Refusal struct)
//   - internal/verdict/citation.go      lines 78-94 (RefusalReason* consts)
//   - internal/verdict/validate.go      lines 31-36 (ValidStatuses)
//
// NOTE on field casing: the Go wire emits snake_case JSON tags
// (`review_required`, `can_user_rephrase`, `suggested_alternative_query`,
// `chunk_id`) — this is the wire contract pinned by
// integration/contract/contract_test.go. Phase 4 fixtures emit the
// same snake_case so the Go-side emitter (Phase 2) can transition
// without re-encoding. Do NOT camelCase these here; UI-SPEC line 311's
// `reviewRequired` reference was an error caught by RESEARCH.md.

// Verdict status enum — matches internal/verdict/validate.go lines 31-36
export const VERDICT_STATUSES = ['合规', '不合规', '需补充', 'unanswered'] as const;
export type VerdictStatus = typeof VERDICT_STATUSES[number];

// Knowledge tier enum — matches internal/skill/manifest.go ResolveTiers
// (4-tier system; SKILL.md may declare a subset via `reads`)
export const TIERS = ['权威', '历史', '个人', '草稿'] as const;
export type Tier = typeof TIERS[number];

// Verdict struct — mirrors internal/verdict/verdict.go lines 57-62
export interface Verdict {
  status: VerdictStatus;
  /** [0.0, 1.0] — Phase 2 threshold: < 0.9 OR grounding downgrade → review_required=true */
  confidence: number;
  citations: Citation[];
  /** snake_case matches Go wire tag (do NOT camelCase) */
  review_required: boolean;
}

// Citation struct — mirrors internal/verdict/citation.go lines 28-44 (D2-06)
export interface Citation {
  /** chunk identifier (Milvus INT64 PK rendered as string) */
  chunk_id: string;
  /** verbatim quote from chunk supporting the claim */
  span: string;
  /** knowledge tier: 权威 / 历史 / 个人 / 草稿 */
  tier: Tier;
}

// Refusal reason enum — matches internal/verdict/citation.go lines 78-94
export const REFUSAL_REASONS = [
  'no_authorized_source',
  'low_retrieval_score',
  'out_of_scope',
] as const;
export type RefusalReason = typeof REFUSAL_REASONS[number];

// Refusal struct — mirrors internal/verdict/citation.go lines 56-73 (ARAG-05)
export interface Refusal {
  reason: RefusalReason;
  /** snake_case matches Go wire tag */
  can_user_rephrase: boolean;
  /** snake_case matches Go wire tag */
  suggested_alternative_query?: string;
}

// Subprogress step event — for FEUI-06 sub-progress indicator. The step
// label carries the lifecycle phase (retrieve → found → rerank →
// self_verify → generate) and optional tier / count metadata.
export type SubprogressStepName =
  | 'retrieve'
  | 'found_candidates'
  | 'rerank'
  | 'self_verify'
  | 'generate';

export interface SubprogressStep {
  step: SubprogressStepName;
  tier?: Tier;
  count?: number;
}

// Type guards — distinguish Verdict vs Refusal payloads inside the
// discriminated union Verdict | Refusal that arrives on data-policy
// chunks. A Verdict has `status` ∈ VERDICT_STATUSES; a Refusal has
// `reason` ∈ REFUSAL_REASONS. The fields are mutually exclusive at
// the wire level (Phase 2 pipeline emits one or the other per turn).
export function isVerdict(value: unknown): value is Verdict {
  return (
    typeof value === 'object' &&
    value !== null &&
    'status' in value &&
    typeof (value as { status: unknown }).status === 'string' &&
    VERDICT_STATUSES.includes((value as { status: string }).status as VerdictStatus)
  );
}

export function isRefusal(value: unknown): value is Refusal {
  return (
    typeof value === 'object' &&
    value !== null &&
    'reason' in value &&
    typeof (value as { reason: unknown }).reason === 'string' &&
    REFUSAL_REASONS.includes((value as { reason: string }).reason as RefusalReason)
  );
}
