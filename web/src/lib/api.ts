// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 36 (REPEATFE-01) — shared API fetch helper.
//
// Previously each FE consumer implemented its own fetch+JSON+error
// pipeline (FeedbackRow.postJson, useEvalReports.fetchOnce,
// useEvalFrozen.refetch). The 4 implementations diverged subtly:
// - inconsistent body.error vs HTTP status text fallback
// - inconsistent Accept header / content-type handling
// - inconsistent test injection (vi.spyOn(global.fetch) vs fetchImpl param)
//
// apiFetch<T>(url, opts) provides a single source of truth.
// Callers can migrate incrementally — the helper is additive.

export interface ApiFetchOptions extends Omit<RequestInit, 'body'> {
  /** Request body — will be JSON.stringify'd if not a string. */
  body?: unknown;
  /** Override fetch for unit-test injection. Defaults to global fetch. */
  fetchImpl?: typeof globalThis.fetch;
}

export interface ApiError extends Error {
  status: number;
  statusText: string;
  /** Server-provided body.error message, if present. */
  body?: unknown;
}

/** GET / POST etc. with JSON encoding + structured error envelope. */
export async function apiFetch<T>(url: string, opts: ApiFetchOptions = {}): Promise<T> {
  const fetchFn = opts.fetchImpl ?? globalThis.fetch;
  const headers = new Headers(opts.headers);
  if (opts.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }

  const resp = await fetchFn(url, {
    ...opts,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });

  if (!resp.ok) {
    const body = (await resp.json().catch(() => ({}))) as { error?: string };
    const err = new Error(body.error ?? `HTTP ${resp.status} ${resp.statusText}`) as ApiError;
    err.status = resp.status;
    err.statusText = resp.statusText;
    err.body = body;
    throw err;
  }

  return (await resp.json()) as T;
}

/** Extract error reason string for UI display (REPEATFE-02). */
export function extractReason(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  return String(err);
}
