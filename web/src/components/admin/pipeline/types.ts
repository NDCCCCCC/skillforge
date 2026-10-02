// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 39-08 — ECL pipeline admin surface types. Mirrors the Plan 07
// StatusDTO (internal/server/admin_pipeline.go → ecl Manager) one-to-one:
// the BE marshals this shape VERBATIM, so the FE mirror must not rename
// or re-nest a single field.

/** One dead-letter / retry-track row (ecl_failures wire shape). */
export interface PipelineFailure {
  note_id: string;
  /** extract | kg-load | milvus-load — opaque BE stage key, rendered raw. */
  stage: string;
  attempt_count: number;
  last_error: string;
  /** pending = auto-retry in flight · dead = manual-retry eligible ·
   * resolved = recovered. */
  status: 'pending' | 'dead' | 'resolved';
  updated_at?: string;
}

/** GET /api/admin/pipeline/status 200 body. `stage` is an opaque BE
 * string; the locked sentinels are idle/merge/extract/kg-load/milvus-load/
 * done (39-UI-SPEC §Stage Mapping) — unknown values render the raw key. */
export interface PipelineStatus {
  stage: string;
  current_file: string;
  done: number;
  total: number;
  queue_len: number;
  failures: PipelineFailure[];
}
