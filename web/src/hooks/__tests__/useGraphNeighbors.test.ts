// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 28-05 Task 2 & 4 — useGraphNeighbors hook tests.
//
// Pins:
//   - fetch fires on mount when chunkId is provided
//   - loading transitions: false → true → false
//   - data is populated on success
//   - error is set on failure
//   - cache: second call with same (chunkId, depth) does NOT refetch
//   - refetch evicts cache and re-fetches
//   - enabled=false suppresses the initial fetch
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { NeighborResult } from '../../types/knowledge';

const FIXTURE_RESULT: NeighborResult = {
  chunk_id: '7',
  depth: 1,
  nodes: [
    { chunk_id: '7', tier: 'authority', source: 'contract.md', effectiveFrom: null, effectiveTo: null },
    { chunk_id: '8', tier: 'authority', source: 'contract.md', effectiveFrom: null, effectiveTo: null },
  ],
  edges: [
    { from: '7', to: '8', edge_type: 'section_adjacent', weight: 1.0, tier: 'authority' },
  ],
  degraded: false,
  degraded_reason: '',
};

// fresh() resets the module cache and re-imports useGraphNeighbors so the
// module-level Map cache is cleared. Each test starts with a clean cache.
async function fresh() {
  vi.resetModules();
  const mod = await import('../useGraphNeighbors');
  mod._resetCache();
  return mod;
}

// Build a fetch mock that returns the given json with the given ok flag.
function okResponse(json: unknown, ok = true) {
  return Promise.resolve({
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(json),
  }) as unknown as Response;
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('useGraphNeighbors', () => {
  it('fetches on mount when chunkId is provided', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(okResponse(FIXTURE_RESULT));
    const { useGraphNeighbors } = await fresh();
    const { result } = renderHook(() => useGraphNeighbors('7', 1));
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual(FIXTURE_RESULT);
    expect(result.current.error).toBeNull();
  });

  it('sets error on non-2xx response', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(okResponse({ error: 'server error' }, false));
    const { useGraphNeighbors } = await fresh();
    const { result } = renderHook(() => useGraphNeighbors('7', 1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('fetch failed');
  });

  it('sets error on network failure', async () => {
    vi.mocked(globalThis.fetch).mockRejectedValue(new Error('network down'));
    const { useGraphNeighbors } = await fresh();
    const { result } = renderHook(() => useGraphNeighbors('7', 1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('fetch failed');
  });

  it('does not re-fetch the same (chunkId, depth) within a session', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(okResponse(FIXTURE_RESULT));
    const { useGraphNeighbors } = await fresh();
    const { result: r1 } = renderHook(() => useGraphNeighbors('7', 1));
    await waitFor(() => expect(r1.current.loading).toBe(false));
    expect(r1.current.data?.chunk_id).toBe('7');

    // Fetch should not be called again for the same (7,1).
    const callCount = vi.mocked(globalThis.fetch).mock.calls.length;
    const { result: r2 } = renderHook(() => useGraphNeighbors('7', 1));
    expect(r2.current.data?.chunk_id).toBe('7');
    expect(vi.mocked(globalThis.fetch).mock.calls.length).toBe(callCount);
  });

  it('refetch evicts cache and re-fetches a new chunkId', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(okResponse(FIXTURE_RESULT));
    const { useGraphNeighbors } = await fresh();
    const { result } = renderHook(() => useGraphNeighbors('7', 1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data?.chunk_id).toBe('7');

    const NEW_RESULT: NeighborResult = {
      ...FIXTURE_RESULT,
      chunk_id: '9',
      nodes: [{ chunk_id: '9', tier: 'personal', source: 'memo.md', effectiveFrom: null, effectiveTo: null }],
      edges: [],
    };
    vi.mocked(globalThis.fetch).mockResolvedValue(okResponse(NEW_RESULT));

    await act(async () => {
      result.current.refetch('9', 1);
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data?.chunk_id).toBe('9');
  });

  it('enabled=false suppresses automatic fetch', async () => {
    const { useGraphNeighbors } = await fresh();
    const { result } = renderHook(() => useGraphNeighbors('7', 1, { enabled: false }));
    expect(result.current.loading).toBe(false);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();
  });
});
