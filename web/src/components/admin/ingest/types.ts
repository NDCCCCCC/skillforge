// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- the FE mirror of the Plan 54 ingest wire
// (internal/server/admin_ingest.go ingestJobDTO / ingestStepDTO /
// ingestItemDTO).
//
// SYNC DISCIPLINE (the highest constraint of this phase): the BE marshals
// this shape VERBATIM, so the FE mirror must not rename, re-nest or
// narrow a single field. Enum fields stay `string` -- an unknown status
// renders the RAW key (never blank, never a crash). Optional
// forward-compat fields (summary / skip_reason / progress_msg / docs) are
// not emitted by the 54 wire yet; when the BE starts emitting them the
// components render them with ZERO FE change, and until then each
// component's absent-branch is the tested degradation. The FE NEVER
// fabricates or locally aggregates these values.
//
// Wire deltas from the UI-SPEC draft are recorded here so nobody
// "fixes" the mirror back to the spec:
//   - the step failure list is `failed_items[]` (54-03), not `failures`
//   - one failed item is `{step, item_key, status, error}` (item_key is
//     the chunk content_hash — the exact re-embed target)
//   - the job badge vocabulary is queued|running|partial|done|failed
//     (there is no `completed` key on the wire)
//   - step.status has no independent `dead`; a deterministic failure
//     rides the error string's `[deterministic] ` prefix instead

import type { StatusTone } from '../shared/StatusBadge';

/** The eight pipeline steps in execution order (54-01 StepSequence
 * mirror). The timeline anchors its rows on this sequence; a payload key
 * outside it sorts last and renders verbatim. */
export const STEP_SEQUENCE = [
  'parse',
  'chunk',
  'embed',
  'vector_insert',
  'ecl_merge',
  'ecl_extract',
  'ecl_kg_load',
  'ecl_milvus_load',
] as const;

/** step sentinel -> zh label (55-UI-SPEC Step Mapping, LOCKED).
 * The four ecl_* labels are the 39-UI-SPEC wording, unchanged (D-55-01).
 * An unknown key renders the raw key (unknown != blank). */
export const STEP_LABEL: Record<string, string> = {
  parse: '解析',
  chunk: '分块',
  embed: '向量化',
  vector_insert: '入向量库',
  ecl_merge: '切分合并',
  ecl_extract: '实体抽取',
  ecl_kg_load: 'KG 写入',
  ecl_milvus_load: 'Milvus 索引',
};

export interface IngestStep {
  /** STEP_SEQUENCE sentinel or an unknown key (rendered verbatim). */
  step: string;
  /** pending|running|done|failed|skipped|stale; unknown renders verbatim. */
  status: string;
  attempt: number;
  /** RFC3339Nano, caller-set by the runner (54-01). */
  started_at?: string;
  completed_at?: string;
  /** Presentation-only derivation on the handler side (54-01) -- the FE
   * prefers the timestamps for display; this field is kept so the mirror
   * stays complete. */
  duration_ms?: number;
  /** Failure reason (a first-class field); a skipped step carries its
   * skip reason here. Carries the 54-03 `[transient] `/`[deterministic] `
   * classification prefix. */
  error?: string;
  /** Staging product locator (D-54-02); rendered mono, never interpreted. */
  artifact_ref?: string;
  /** forward-compat (Dify Variable Inspect; the 54 wire does not emit). */
  summary?: Record<string, number | string>;
  /** forward-compat (the 54 wire does not emit; skipped reasons ride
   * `error` today). */
  skip_reason?: string;
}

/** One row of failed_items[] (54-03 SC 7). `item_key` is the chunk
 * content_hash -- the precise repair target. */
export interface IngestFailedItem {
  step: string;
  item_key: string;
  status: string;
  error: string;
}

/** forward-compat: the 54 wire runs one doc per job, so docs[] is not
 * emitted. Declared so the matrix' data premise is stable. */
export interface IngestDoc {
  doc_id: string;
  file_name: string;
  docs?: unknown;
  steps: IngestStep[];
}

export interface IngestJob {
  job_id: string;
  slug: string;
  filename: string;
  sha256: string;
  doc_id: string;
  /** queued|running|partial|done|failed; unknown renders verbatim. */
  status: string;
  error: string;
  created_at: string;
  updated_at: string;
  /** Full step rows in StepSequence order (54-01). */
  steps: IngestStep[];
  failed_items: IngestFailedItem[];
  /** forward-compat (RAGFlow append-only log; the 54 wire does not emit). */
  progress_msg?: string[];
  /** forward-compat (matrix data premise; the 54 wire does not emit). */
  docs?: IngestDoc[];
}

/** The derived presentation state of one step. `derived` is the
 * vocabulary the UI branches on; an unknown status yields `derived` =
 * the raw key (unknown != blank). */
export interface DerivedStepState {
  derived: string;
  label: string;
  tone: StatusTone;
  pulse: boolean;
}

/**
 * deriveStepState -- the TWO-LAYER derivation (55-UI-SPEC §Derived State
 * Mapping). This is the phase's single source of state mapping: every
 * surface (timeline row, matrix cell, inspect panel) consumes it.
 *
 *   pending  -> waiting      (gray,  no pulse)
 *   running  -> in progress  (amber, PULSE -- the only pulsing state)
 *   done     -> complete     (green, no pulse)
 *   failed   -> failed       (red,   no pulse)
 *   skipped  -> skipped      (GRAY -- overrides StatusBadge's amber
 *                             DEFAULT_MAP entry for `skipped`)
 *   stale    -> to re-run    (amber, NO pulse: it is not running)
 *
 * attempt>=2 is deliberately NOT a separate state (jitter suppression);
 * the attempt count lives in the expanded inspect panel.
 */
export function deriveStepState(step: IngestStep): DerivedStepState {
  switch (step.status) {
    case 'pending':
      return { derived: 'queued', label: '等待', tone: 'gray', pulse: false };
    case 'running':
      return { derived: 'running', label: '进行中', tone: 'amber', pulse: true };
    case 'done':
      return { derived: 'done', label: '完成', tone: 'green', pulse: false };
    case 'failed':
      return { derived: 'error', label: '失败', tone: 'red', pulse: false };
    case 'skipped':
      return { derived: 'skipped', label: '已跳过', tone: 'gray', pulse: false };
    case 'stale':
      return { derived: 'stale', label: '待重跑', tone: 'amber', pulse: false };
    default:
      return { derived: step.status, label: step.status, tone: 'gray', pulse: false };
  }
}

/** The 54-03 classification prefix that marks a failure as NOT
 * auto-retryable. Byte-for-byte the BE's retry.go ErrClass.Prefix
 * literal (`"[" + c.String() + "] "`) -- the FE only reads it, it never
 * writes it. */
export const DETERMINISTIC_PREFIX = '[deterministic]';

export function isDeterministic(step: IngestStep): boolean {
  return (step.error ?? '').startsWith(DETERMINISTIC_PREFIX);
}

/** Job badge mapping. Keys are the WIRE vocabulary (done, not the
 * UI-SPEC draft's `completed`); an unknown key renders verbatim gray. */
export const JOB_BADGE: Record<string, { label: string; tone: StatusTone; pulse: boolean }> = {
  queued: { label: '等待', tone: 'gray', pulse: false },
  running: { label: '运行中', tone: 'amber', pulse: true },
  partial: { label: '部分成功', tone: 'amber', pulse: false },
  done: { label: '已完成', tone: 'green', pulse: false },
  failed: { label: '失败', tone: 'red', pulse: false },
};

export function jobBadge(status: string): { label: string; tone: StatusTone; pulse: boolean } {
  return JOB_BADGE[status] ?? { label: status, tone: 'gray', pulse: false };
}

/**
 * formatDuration -- wall-clock duration from the two stored stamps.
 * Both stamps present and parseable -> `3.2s`; otherwise `—` (a running
 * step, a missing stamp, or a corrupt stamp). ZERO local stopwatch: the
 * value refreshes with each poll, never ticks on its own.
 */
export function formatDuration(startedAt?: string, completedAt?: string): string {
  if (!startedAt || !completedAt) return '—';
  const start = Date.parse(startedAt);
  const end = Date.parse(completedAt);
  if (Number.isNaN(start) || Number.isNaN(end)) return '—';
  const ms = end - start;
  if (ms < 0) return '—';
  return `${(ms / 1000).toFixed(1)}s`;
}

/** Order the payload's step rows on STEP_SEQUENCE. Unknown keys keep
 * their payload order at the tail. The FE reorders only -- it never
 * inserts a missing step (zero derivation). */
export function orderSteps(steps: IngestStep[]): IngestStep[] {
  const known: IngestStep[] = [];
  const unknown: IngestStep[] = [];
  for (const step of steps) {
    if ((STEP_SEQUENCE as readonly string[]).includes(step.step)) known.push(step);
    else unknown.push(step);
  }
  known.sort(
    (a, b) =>
      (STEP_SEQUENCE as readonly string[]).indexOf(a.step) -
      (STEP_SEQUENCE as readonly string[]).indexOf(b.step),
  );
  return [...known, ...unknown];
}

/** Count the steps whose derived state is `done` (the progress meter and
 * the jobs-table progress column both read this). */
export function countDoneSteps(steps: IngestStep[]): number {
  return steps.filter((s) => deriveStepState(s).derived === 'done').length;
}
