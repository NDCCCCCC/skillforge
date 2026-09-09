// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-03) — eval reports + frozen-set fetch hooks.
//
// Polls GET /api/eval/reports and GET /api/eval/frozen every 30s
// when mounted. The fetch loop lives in EvalDashboard.tsx (the
// shell owns the interval); this hook is the per-endpoint fetch
// helper that mirrors useKnowledgeGraph's one-shot fetch + refetch
// pattern, with the indexReports/frozenBaseline written into the
// Zustand store (so the table + trend + cross-eval panels share
// one source of truth).
//
// The hooks do NOT import the chat SSE transport — D-04/D12
// preserved by isolation (the chat wire is not at risk from
// anything in this file).

import { useCallback, useEffect, useRef } from 'react';
import { useAppStore } from '../store/app-store';
import type { EvalIndex, FrozenSetInfo } from '../types/eval';

export type EvalStatus = 'idle' | 'loading' | 'ready' | 'error';

export interface UseEvalReportResult {
  status: EvalStatus;
  error: string | null;
  refetch: () => void;
}

/** Fetch /api/eval/reports and write the parsed EvalIndex into the
 * store. Errors land in the store's `evalError` key for the dashboard
 * shell to render. */
export function useEvalReports(pollIntervalMs = 30_000): UseEvalReportResult {
  const setIndexReports = useAppStore((s) => s.setIndexReports);
  const setEvalError = useAppStore((s) => s.setEvalError);
  const setEvalStatus = useAppStore((s) => s.setEvalStatus);
  const evalStatus = useAppStore((s) => s.evalStatus);
  const evalError = useAppStore((s) => s.evalError);
  const reloadKey = useRef(0);

  const fetchOnce = useCallback(async () => {
    setEvalStatus('loading');
    try {
      const resp = await fetch('/api/eval/reports', {
        headers: { Accept: 'application/json' },
      });
      if (resp.status === 503) {
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        setEvalError(body.error ?? 'eval reports not initialized on the server');
        return;
      }
      if (!resp.ok) {
        const body = (await resp.json().catch(() => ({}))) as { error?: string };
        setEvalError(body.error ?? `HTTP ${resp.status} ${resp.statusText}`);
        return;
      }
      const data = (await resp.json()) as EvalIndex;
      setIndexReports(data.runs ?? []);
      setEvalError(null);
      setEvalStatus('ready');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setEvalError(msg);
    }
  }, [setIndexReports, setEvalError, setEvalStatus]);

  useEffect(() => {
    let cancelled = false;
    // Initial fetch on mount + on reloadKey bump.
    void fetchOnce();
    const id = window.setInterval(() => {
      if (cancelled) return;
      void fetchOnce();
    }, pollIntervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [pollIntervalMs, reloadKey.current, fetchOnce]);

  // Refetch trigger: bump the ref so the effect re-runs.
  // (Caller mutates reloadKey.current — keeps the hook read-only.)
  const refetch = useCallback(() => {
    reloadKey.current += 1;
    void fetchOnce();
  }, [fetchOnce]);

  return { status: evalStatus, error: evalError ?? null, refetch };
}

/** Fetch /api/eval/frozen and write the first matching FrozenSetInfo
 * into the store. The TrendPanel reads `frozenBaseline` to overlay
 * the dashed gray reference line (D6-16). */
export function useEvalFrozen(): { refetch: () => void } {
  const setFrozenBaseline = useAppStore((s) => s.setFrozenBaseline);
  const refetch = useCallback(async () => {
    try {
      const resp = await fetch('/api/eval/frozen', {
        headers: { Accept: 'application/json' },
      });
      if (!resp.ok) return;
      const list = (await resp.json()) as FrozenSetInfo[];
      // The dashboard's per-Skill view picks the first baseline for the
      // currently-shown Skill. With one Skill (compliance) this is the
      // only entry. Multiple Skills will need per-Skill selection; out
      // of scope for 06-04's single-Skill dashboard.
      if (list.length > 0) setFrozenBaseline(list[0]);
    } catch {
      // Frozen baseline is best-effort (D6-16 overlay). No-op on error.
    }
  }, [setFrozenBaseline]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  return { refetch };
}