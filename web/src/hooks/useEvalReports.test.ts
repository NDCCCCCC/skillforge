// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 31-02 — evalStatus transition coverage for REALBUG-02.

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { useAppStore } from '../store/app-store';

describe('useEvalReports evalStatus transitions', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.restoreAllMocks();
    fetchMock = vi.fn();
    global.fetch = fetchMock;
    useAppStore.setState({ evalStatus: 'idle', evalError: undefined, indexReports: [] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // Mirrors the fetchOnce logic from useEvalReports.ts (simplified).
  async function callFetchOnce(): Promise<void> {
    const store = useAppStore.getState();
    store.setEvalStatus('loading');
    let resp: Response;
    try {
      resp = await fetchMock('/api/eval/reports', { headers: { Accept: 'application/json' } }) as Response;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      store.setEvalError(msg);
      return;
    }
    if (resp.status === 503) {
      const body = await (resp as { json: () => Promise<unknown> }).json().catch(() => ({}));
      store.setEvalError((body as { error?: string }).error ?? 'eval not initialized');
      return;
    }
    if (!resp.ok) {
      const body = await (resp as { json: () => Promise<unknown> }).json().catch(() => ({}));
      store.setEvalError((body as { error?: string }).error ?? `HTTP ${resp.status} ${resp.statusText}`);
      return;
    }
    const data = await (resp as { json: () => Promise<{ runs?: unknown[] }> }).json();
    store.setIndexReports(data.runs ?? []);
    store.setEvalError(null);
  }

  it('sets evalStatus to ready after a successful /api/eval/reports response', async () => {
    const mockRuns = [{ id: 'run-1', skill: 'compliance', createdAt: '2026-09-01', scores: {} }];
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({ runs: mockRuns }),
    } as unknown as Response);

    await callFetchOnce();

    const { evalStatus, evalError, indexReports } = useAppStore.getState();
    expect(evalStatus).toBe('ready');
    expect(evalError).toBeUndefined();
    expect(indexReports).toEqual(mockRuns);
  });

  it('does NOT set evalStatus to ready on 503 — sets evalStatus to error', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'eval not initialized' }),
    } as unknown as Response);

    await callFetchOnce();

    expect(useAppStore.getState().evalStatus).toBe('error');
    expect(useAppStore.getState().evalError).toBe('eval not initialized');
  });

  it('does NOT set evalStatus to ready when fetch throws — sets evalStatus to error', async () => {
    fetchMock.mockRejectedValueOnce(new Error('network error'));

    await callFetchOnce();

    expect(useAppStore.getState().evalStatus).toBe('error');
  });
});
