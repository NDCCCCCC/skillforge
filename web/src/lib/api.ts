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
//
// 后端最小身份层：每个浏览器持有一个 owner token，所有请求都要带上
// `X-Owner-Token` 头（后端据此过滤会话列表）。token 在这里注入，
// 于是所有走 apiFetch 的调用点自动获得身份 —— 不必逐个改几十处
// 调用点。调用方显式带了同名头时以调用方为准（测试注入优先）。

import { getOwnerToken } from './owner-token';

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
  // 身份头（后端最小身份层）：缺失时补上 owner token。
  if (!headers.has('X-Owner-Token')) {
    headers.set('X-Owner-Token', getOwnerToken());
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
