// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 39-08 — fetch wrappers for the 3 ECL pipeline admin routes
// (Plan 07, D-Route-01). Admin token comes from localStorage
// 'sf_admin_token' (same contract as CapacityMonitor / ontologyApi).
//
// Error honesty rule (39-UI-SPEC): non-OK responses become a typed
// PipelineApiError carrying the numeric status + the server's RAW error
// string, so PipelineTab can branch 403 / 409 / 503 and render the
// reason verbatim. Fetch rejections (network) map to 网络错误 — never a
// fabricated status.

import type { PipelineStatus } from './types';

/** Typed error so the UI can branch on .status (403/409/503). `message`
 * is the server's raw error string, or 网络错误 for fetch rejections
 * (status 0). */
export class PipelineApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'PipelineApiError';
    this.status = status;
  }
}

function adminToken(): string {
  return localStorage.getItem('sf_admin_token') ?? '';
}

/** Map a non-OK response to a PipelineApiError. The BE error envelope is
 * { "error": string } (graphResponseError) — unwrap it verbatim; a
 * non-JSON body is kept as-is. */
async function toApiError(resp: Response): Promise<PipelineApiError> {
  let raw = '';
  try {
    raw = await resp.text();
  } catch {
    // body unreadable — fall through with an empty raw string
  }
  let message = raw;
  try {
    const parsed = JSON.parse(raw) as { error?: unknown };
    if (parsed && typeof parsed.error === 'string') {
      message = parsed.error;
    }
  } catch {
    // not JSON — keep the raw body text
  }
  return new PipelineApiError(resp.status, message);
}

/** Wrap a fetch promise so network rejections become typed 网络错误. */
async function withNetworkGuard<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (err: unknown) {
    if (err instanceof PipelineApiError) throw err;
    throw new PipelineApiError(0, '网络错误');
  }
}

export function fetchStatus(): Promise<PipelineStatus> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/pipeline/status', {
        headers: { 'X-Admin-Token': adminToken() },
      });
      if (!resp.ok) throw await toApiError(resp);
      return (await resp.json()) as PipelineStatus;
    })(),
  );
}

export function runPipeline(): Promise<{ status: string }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/pipeline/run', {
        method: 'POST',
        headers: { 'X-Admin-Token': adminToken() },
      });
      if (!resp.ok) throw await toApiError(resp);
      return (await resp.json()) as { status: string };
    })(),
  );
}

export function retryNote(noteId: string): Promise<{ status: string; note_id: string }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch(`/api/admin/pipeline/retry/${encodeURIComponent(noteId)}`, {
        method: 'POST',
        headers: { 'X-Admin-Token': adminToken() },
      });
      if (!resp.ok) throw await toApiError(resp);
      return (await resp.json()) as { status: string; note_id: string };
    })(),
  );
}
