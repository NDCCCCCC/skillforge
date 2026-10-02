// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 — TypeScript mirror of the Phase-6 eval dashboard +
// audit API response shapes. Mirrors the Go JSON tags from
// internal/server/eval.go (EvalIndex / FrozenSetInfo) and
// internal/store/store.go (AuditRow / AuditFilter).
//
// INVARIANT: NO `messages` field anywhere in these types. The eval
// slice lives in app-store.ts but never holds chat messages — useChat
// owns the chat stream on a different route (D14).

/** One run entry from GET /api/eval/reports (INDEX.json). */
export interface IndexRun {
  /** ISO timestamp matching internal/eval ArchiveTimestampLayout. */
  ts: string;
  /** Skill id this evaluation was run against. */
  skill: string;
  /** Judge model id (e.g. "glm-5.2"). */
  judge_model: string;
  /** 4 Ragas-aligned metrics + the derived refusal accuracy. */
  faithfulness: number;
  context_precision: number;
  answer_relevance: number;
  context_recall: number;
  refusal_accuracy: number;
  /** Frozen-set version pin (D6-13). Empty string when not pinned. */
  frozen_version: string;
  /** Relative path to the full report inside the reports dir. */
  report_path: string;
}

/** Top-level INDEX.json envelope. */
export interface EvalIndex {
  version: number;
  runs: IndexRun[];
}

/** One frozen-set entry from GET /api/eval/frozen. */
export interface FrozenSetInfo {
  /** Skill id (matches LockFile.SkillName). */
  skill: string;
  /** Frozen-set version label (matches LockFile.EvalSetVersion). */
  version: string;
  /** Lock-file path (relative to the eval dir). */
  lock_path: string;
  /** The frozen-set baseline faithfulness (last published run's score). */
  baseline_faithfulness?: number;
}

/** Audit row shape from /api/audit. Mirrors internal/store.AuditRow
 * (with redaction applied server-side per D6-06). */
export interface AuditRow {
  ts: string;
  user_id: string;
  skill_id: string;
  session_id: string;
  refusal_reason: string;
  query: string;
  verdict_json: string;
  /** Null when not set (run without Ragas eval backfill). */
  faithfulness?: number | null;
  context_precision?: number | null;
  answer_relevance?: number | null;
  context_recall?: number | null;
}

/** Query-param shape for /api/audit (D6-05 + D6-06). */
export interface AuditFilter {
  user?: string;
  skill?: string;
  refusal_reason?: string;
  /** ISO timestamp lower bound (inclusive). */
  since?: string;
  /** ISO timestamp upper bound (exclusive). */
  until?: string;
  limit: number;
  offset: number;
}

/** Empty-default AuditFilter factory — keep callers from drifting on
 * the D14 invariant (the audit slice defaults to a clean filter). */
export const EMPTY_AUDIT_FILTER: AuditFilter = {
  limit: 100,
  offset: 0,
};

/** Default faithfulness threshold for the SkillsTable ❌/✅ badge.
 * Phase 6 ships ONE Skill (compliance, eval-gate.json = 0.9). When
 * a frozen baseline is loaded the threshold may be overridden by
 * the baseline info — the component decides. */
export const DEFAULT_FAITHFULNESS_THRESHOLD = 0.9;