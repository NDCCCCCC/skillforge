// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 1 — TypeScript mirror of internal/verdict/* structs.
//
// Source of truth (Go):
//   - internal/verdict/verdict.go       (Verdict struct)
//   - internal/verdict/citation.go      (Citation + Refusal structs)
//   - internal/verdict/validate.go      (ValidStatuses enum)
//   - internal/verdict/grounding.go     (DowngradeStatus / NonCompliantStatus /
//                                         CompliantStatus / NotApplicableStatus /
//                                         ComplianceStatuses)
//
// NOTE on field casing: the Go wire emits snake_case JSON tags
// (`review_required`, `can_user_rephrase`, `suggested_alternative_query`,
// `chunk_id`, `compliance_status`) — this is the wire contract pinned by
// integration/contract/contract_test.go. Phase 4 fixtures emit the
// same snake_case so the Go-side emitter (Phase 2) can transition
// without re-encoding. Do NOT camelCase these here; UI-SPEC line 311's
// `reviewRequired` reference was an error caught by RESEARCH.md.

// Platform-neutral Status enum — matches internal/verdict/validate.go
// ValidStatuses. 260817-vsm: was ['合规', '不合规', '需补充', 'unanswered']
// (compliance-Skill-specific Chinese); migrated to platform-neutral English
// so the shared envelope is skill-agnostic (multi-Skill platform fix).
//
// Semantics:
//   - 'ok'          — produced a grounded answer (factual or compliance verdict)
//   - 'needs_input' — info insufficient (replaces 需补充)
//   - 'refused'     — graceful refusal (replaces unanswered)
//   - 'error'       — internal failure (new; audit distinguishes from refusal)
export const VERDICT_STATUSES = ['ok', 'needs_input', 'refused', 'error'] as const;
export type VerdictStatus = typeof VERDICT_STATUSES[number];

// Compliance-Skill-specific verdict enum — matches
// internal/verdict/grounding.go ComplianceStatuses. Rides in
// Verdict.compliance_status (NOT Verdict.status). 260817-vsm.
//
// Semantics:
//   - 'compliant'      — subject matches cited policy (was 合规)
//   - 'non_compliant'  — subject fails cited policy (was 不合规)
//   - 'not_applicable' — query is factual lookup, see Verdict.answer
export const COMPLIANCE_STATUSES = ['compliant', 'non_compliant', 'not_applicable'] as const;
export type ComplianceStatus = typeof COMPLIANCE_STATUSES[number];

// Knowledge-base tier slugs — Phase 26 (KBM-02) made the tier vocabulary
// REGISTRY-DRIVEN: any slug matching internal/kb.ValidateSlug is legal, so
// `Tier` is a plain string and the FE never assumes a fixed tier count
// (N knowledge bases are first-class). TIERS below is the SEED FIXTURE list
// (the four knowledge_bases.yaml baseline slugs), NOT a closed enum — do not
// switch on it. Display names come from the registry projection
// (lib/kb-bases.ts); the pre-migration Chinese display names are display
// strings only and are no longer identifiers anywhere in the FE.
export const TIERS = ['authority', 'historical', 'personal', 'draft'] as const;
export type Tier = string;

// Verdict struct — mirrors internal/verdict/verdict.go
//
// 260817-vsm: Verdict is now the platform-agnostic response envelope.
// Compliance-Skill verdict codes live in `compliance_status`; factual-query
// direct answers live in `answer`. Renderers should lead with `answer` when
// populated (factual query), otherwise use `compliance_status` (Compliance
// Skill) or `status` (other Skills).
export interface Verdict {
  /** platform-neutral: one of VERDICT_STATUSES */
  status: VerdictStatus;
  /** [0.0, 1.0] — Phase 2 threshold: < 0.9 OR grounding downgrade → review_required=true */
  confidence: number;
  citations: Citation[];
  /** snake_case matches Go wire tag (do NOT camelCase) */
  review_required: boolean;
  /** 260817-vsm: factual-query direct answer (e.g. "由部门负责人审批").
   * Empty for compliance-review queries. Leads the conclusion render when
   * populated. */
  answer?: string;
  /** 260817-vsm: Compliance-Skill-specific verdict. One of COMPLIANCE_STATUSES.
   * Empty for non-compliance Skills (simple-qa / calculator). */
  compliance_status?: ComplianceStatus;
}

// Citation struct — mirrors internal/verdict/citation.go (D2-06)
export interface Citation {
  /** chunk identifier (Milvus INT64 PK rendered as string) */
  chunk_id: string;
  /** verbatim quote from chunk supporting the claim */
  span: string;
  /** Registry slug of the source knowledge base (e.g. "authority"). A
   * historical audit replay may carry a pre-migration CN display name —
   * render it through TierBadge, never compare against it. */
  tier: Tier;
  /** D5-15 (SC-8): null = perpetually valid (backward-compat with versionless chunks). */
  effective_from?: string | null;
  /** D5-15 (SC-8): null = perpetually valid; < today = superseded (filtered by retriever). */
  effective_to?: string | null;
  /** D5-15 (SC-8): human-readable version label (e.g. "v2", "2026-03修订版"). */
  version?: string | null;
}

// Refusal reason enum — matches internal/verdict/citation.go
export const REFUSAL_REASONS = [
  'no_authorized_source',
  'low_retrieval_score',
  'out_of_scope',
] as const;
export type RefusalReason = typeof REFUSAL_REASONS[number];

// Refusal struct — mirrors internal/verdict/citation.go (ARAG-05)
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
  /** Registry slug of the tier being searched; absent = no tier badge. */
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
