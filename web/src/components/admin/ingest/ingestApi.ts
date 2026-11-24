// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- fetch wrappers for the ingest admin routes.
// Mirrors pipelineApi.ts exactly: admin token from localStorage
// 'sf_admin_token', a typed IngestApiError carrying the numeric status +
// the server's RAW error string so the UI can branch 403 / 404 / 409 /
// 503 and render the reason verbatim. Fetch rejections map to 网络错误
// (status 0) -- never a fabricated status.
//
// Endpoints (54-01 read + 55-03 roster + 54-03 retry):
//   GET  /api/admin/ingest/jobs
//   GET  /api/admin/ingest/jobs/:id
//   POST /api/admin/ingest/jobs/:id/steps/:step/retry

import type { IngestJob } from './types';

/** Typed error so the UI can branch on .status. `message` is the server's
 * raw error string, or 网络错误 for fetch rejections (status 0). */
export class IngestApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'IngestApiError';
    this.status = status;
  }
}

function adminToken(): string {
  return localStorage.getItem('sf_admin_token') ?? '';
}

/** Map a non-OK response to an IngestApiError. The BE error envelope is
 * { "error": string } (graphResponseError) -- unwrapped verbatim; a
 * non-JSON body is kept as-is. 403/404 get the locked copy because those
 * two branches are operator-actionable (wrong token / job cleaned up). */
async function toApiError(resp: Response): Promise<IngestApiError> {
  let raw = '';
  try {
    raw = await resp.text();
  } catch {
    // body unreadable -- fall through with an empty raw string
  }
  let message = raw;
  try {
    const parsed = JSON.parse(raw) as { error?: unknown };
    if (parsed && typeof parsed.error === 'string') {
      message = parsed.error;
    }
  } catch {
    // not JSON -- keep the raw body text
  }
  if (resp.status === 403) {
    message = '缺少或错误的 X-Admin-Token';
  } else if (resp.status === 404) {
    message = '任务不存在或已被清理';
  }
  return new IngestApiError(resp.status, message);
}

/** Wrap a fetch promise so network rejections become typed 网络错误. */
async function withNetworkGuard<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (err: unknown) {
    if (err instanceof IngestApiError) throw err;
    throw new IngestApiError(0, '网络错误');
  }
}

/** GET /api/admin/ingest/jobs -- the roster (55-03 Jobs view). The BE
 * answers {jobs: [...]}; an empty roster is a real 200 with an empty
 * array (the FE may then say 暂无摄取任务 honestly). */
export function fetchJobs(): Promise<IngestJob[]> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/ingest/jobs', {
        headers: { 'X-Admin-Token': adminToken() },
      });
      if (!resp.ok) throw await toApiError(resp);
      const body = (await resp.json()) as { jobs?: IngestJob[] };
      return Array.isArray(body.jobs) ? body.jobs : [];
    })(),
  );
}

/** GET /api/admin/ingest/jobs/:id -- one job with its step rows. */
export function fetchJob(id: string): Promise<IngestJob> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch(`/api/admin/ingest/jobs/${encodeURIComponent(id)}`, {
        headers: { 'X-Admin-Token': adminToken() },
      });
      if (!resp.ok) throw await toApiError(resp);
      return (await resp.json()) as IngestJob;
    })(),
  );
}

/** POST /api/admin/ingest/jobs/:id/steps/:step/retry -- re-enter the
 * pipeline at the failed step. The response carries the step's CURRENT
 * error text (RAGFlow lesson: the first real error, not a generic
 * failure). */
export function retryStep(
  id: string,
  step: string,
): Promise<{ status: string; job_id: string; step: string; error?: string }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch(
        `/api/admin/ingest/jobs/${encodeURIComponent(id)}/steps/${encodeURIComponent(step)}/retry`,
        { method: 'POST', headers: { 'X-Admin-Token': adminToken() } },
      );
      if (!resp.ok) throw await toApiError(resp);
      return (await resp.json()) as { status: string; job_id: string; step: string; error?: string };
    })(),
  );
}

/** Narrow an unknown catch value to an IngestApiError for UI rendering. */
export function asApiError(err: unknown): IngestApiError {
  if (err instanceof IngestApiError) return err;
  return new IngestApiError(0, err instanceof Error ? err.message : '网络错误');
}
