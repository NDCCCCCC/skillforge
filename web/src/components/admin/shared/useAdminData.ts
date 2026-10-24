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
// `null` url skips fetching (useful for dependent fetches).

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
  const token = localStorage.getItem('sf_admin_token') ?? '';
  return fetch(url, {
    ...init,
    headers: {
      'X-Admin-Token': token,
      ...(init?.headers ?? {}),
    },
  });
}

export interface AdminDataState<T> {
  data: T | null;
  loading: boolean;
  error: boolean;
  reload: () => void;
}

export function useAdminData<T>(url: string | null): AdminDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [tick, setTick] = useState(0);

  const reload = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    if (url === null) return;
    let cancelled = false;
    setLoading(true);
    setError(false);
    adminFetch<T>(url)
      .then((d) => {
        if (!cancelled) {
          setData(d);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError(true);
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [url, tick]);

  return { data, loading, error, reload };
}
