// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// useAdminData — shared admin data-fetching hook (admin UI redesign 2610).
//
// Replaces the per-component `useEffect + fetch + X-Admin-Token + three
// loading/error/data states` boilerplate that every admin view previously
// hand-rolled. The admin token is read from localStorage ('sf_admin_token')
// and sent as the X-Admin-Token header — the BE validates it via
// isAdminRequest (constant-time compare).
//
// Usage:
//   const { data, loading, error, reload } = useAdminData<SchedulesResponse>(
//     '/api/admin/schedules',
//   );
//
// `null` url skips fetching (useful for dependent fetches) AND resets the
// loading flag — a dependent caller that renders <LoadingSkeleton> on
// loading would otherwise spin forever while waiting for its dependency
// (CODE-AUDIT 2026-11-05 E-13).
//
// CODE-AUDIT 2026-11-05 (E-04): `error` carries the HTTP status + backend
// copy (`HTTP 403: admin token required`) instead of a bare boolean — the
// 403 "wrong token" case is by far the most common admin failure and was
// indistinguishable from a 500. Callers that only truthiness-check keep
// working (a non-empty string is truthy, the old `false` is falsy).

import { useCallback, useEffect, useState } from 'react';

export function adminFetch<T>(url: string, init?: RequestInit): Promise<T> {
  return adminFetchResponse(url, init).then((resp) => {
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}`);
    }
    return resp.json() as Promise<T>;
  });
}

/**
 * Same token-injecting fetch as `adminFetch`, but resolves the raw
 * `Response` so mutation verbs can branch on status (400/403/409/503)
 * and surface the backend's `error` copy verbatim.
 */
export function adminFetchResponse(url: string, init?: RequestInit): Promise<Response> {
  let token = '';
  try {
    token = localStorage.getItem('sf_admin_token') ?? '';
  } catch {
    /* storage disabled — requests go out unauthenticated, BE answers 403 */
  }
  return fetch(url, {
    ...init,
    headers: {
      'X-Admin-Token': token,
      ...(init?.headers ?? {}),
    },
  });
}

/** Fetches url and rejects with `HTTP <status>: <backend error copy>`. */
export async function adminFetchWithError<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await adminFetchResponse(url, init);
  if (!resp.ok) {
    let detail = '';
    try {
      const body = (await resp.json()) as { error?: string };
      if (body && typeof body.error === 'string') detail = `: ${body.error}`;
    } catch {
      /* non-JSON error body — status alone */
    }
    throw new Error(`HTTP ${resp.status}${detail}`);
  }
  return resp.json() as Promise<T>;
}

/** 15s — admin endpoints answer from local stores; longer means hang. */
const ADMIN_FETCH_TIMEOUT_MS = 15_000;

/**
 * Global admin-refresh event (CODE-AUDIT 2026-11-05 E-05): AdminDashboard's
 * top-bar Refresh button dispatches this; every mounted useAdminData view
 * refetches. Without a listener the button was a visible no-op.
 */
export const ADMIN_REFRESH_EVENT = 'sf-admin-refresh';

export function dispatchAdminRefresh(): void {
  window.dispatchEvent(new CustomEvent(ADMIN_REFRESH_EVENT));
}

export function useAdminData<T>(url: string | null): AdminDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [tick, setTick] = useState(0);

  const reload = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    const onRefresh = () => setTick((t) => t + 1);
    window.addEventListener(ADMIN_REFRESH_EVENT, onRefresh);
    return () => window.removeEventListener(ADMIN_REFRESH_EVENT, onRefresh);
  }, []);

  useEffect(() => {
    if (url === null) {
      // E-13: skip means skip — do not leave a skeleton spinning forever.
      setLoading(false);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), ADMIN_FETCH_TIMEOUT_MS);
    let cancelled = false;
    setLoading(true);
    setError('');
    adminFetchWithError<T>(url, { signal: controller.signal })
      .then((d) => {
        if (!cancelled) {
          setData(d);
          setLoading(false);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : String(e));
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [url, tick]);

  return { data, loading, error, reload };
}

export interface AdminDataState<T> {
  data: T | null;
  loading: boolean;
  /** `HTTP <status>: <backend copy>` — empty string means no error. */
  error: string;
  reload: () => void;
}
