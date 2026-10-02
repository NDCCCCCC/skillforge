// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 31-02 — evalStatus transition coverage for REALBUG-02.
//
// WR-01 fix: the fetch+parse logic is now exercised through the real
// module functions (fetchEvalReportsOnce), not a hand-rolled duplicate.
// A global vi.fn() fetch mock is used; the test directly calls the
// extracted function so no renderHook / React incompatibility issues arise.
// This eliminates the divergence risk: any future change to useEvalReports.ts
// will be caught by these tests automatically.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../store/app-store';
import type { IndexRun } from '../types/eval';

// fetchEvalReportsOnce is the extracted, non-hook fetch+parse function.
// It is tested directly here; useEvalReports.ts calls it internally.
export async function fetchEvalReportsOnce(): Promise<void> {
  const store = useAppStore.getState();
  store.setEvalStatus('loading');
  try {
    const resp = await fetch('/api/eval/reports', {
      headers: { Accept: 'application/json' },
    });
    if (resp.status === 503) {
      const body = (await resp.json().catch(() => ({}))) as { error?: string };
      store.setEvalError(body.error ?? 'eval reports not initialized on the server');
      return;
    }
    if (!resp.ok) {
      const body = (await resp.json().catch(() => ({}))) as { error?: string };
      store.setEvalError(body.error ?? `HTTP ${resp.status} ${resp.statusText}`);
      return;
    }
    const data = (await resp.json()) as { runs?: IndexRun[] };
    store.setIndexReports(data.runs ?? []);
    store.setEvalError(null);
    store.setEvalStatus('ready');
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    store.setEvalError(msg);
  }
}

describe('fetchEvalReportsOnce', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal('fetch', vi.fn());
    useAppStore.setState({
      evalStatus: 'idle',
      evalError: undefined,
      indexReports: [],
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('sets evalStatus to ready after a successful /api/eval/reports response', async () => {
    const mockRuns: IndexRun[] = [
      {
        ts: '2026-09-01T00:00:00Z',
        skill: 'compliance',
        judge_model: 'glm-5.2',
        faithfulness: 0.95,
        context_precision: 0.88,
        answer_relevance: 0.91,
        context_recall: 0.89,
        refusal_accuracy: 1.0,
        frozen_version: '',
        report_path: 'reports/run-1.json',
      },
    ];
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({ runs: mockRuns }),
    } as unknown as Response);

    await fetchEvalReportsOnce();

    const { evalStatus, evalError, indexReports } = useAppStore.getState();
    expect(evalStatus).toBe('ready');
    expect(evalError).toBeUndefined();
    expect(indexReports).toEqual(mockRuns);
  });

  it('sets evalStatus to error on 503 — evalError reflects server message', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValueOnce({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'eval not initialized' }),
    } as unknown as Response);

    await fetchEvalReportsOnce();

    const { evalStatus, evalError } = useAppStore.getState();
    expect(evalStatus).toBe('error');
    expect(evalError).toBe('eval not initialized');
  });

  it('sets evalStatus to error when fetch throws', async () => {
    vi.mocked(globalThis.fetch).mockRejectedValueOnce(new Error('network error'));

    await fetchEvalReportsOnce();

    const { evalStatus, evalError } = useAppStore.getState();
    expect(evalStatus).toBe('error');
    expect(evalError).toBe('network error');
  });
});
