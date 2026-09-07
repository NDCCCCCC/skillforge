// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 28-05 Task 2 — useGraphNeighbors hook.
//
// Mirrors the useKnowledgeBases pattern from kb-bases.ts:
//   - module-level promise cache keyed by (chunkId, depth) tuple
//   - null-safe degradation (returns null data on network/parse failure)
//   - hook returns { data, loading, error, refetch }
//
// The neighbors API is public-read (no auth token required) per D-28-02-AUTH.
import { useState, useEffect, useCallback } from 'react';
import type { NeighborResult } from '../types/knowledge';

// Module-level result cache keyed by `${chunkId}:${depth}`.
// Caches null (degraded fetch) as well as successful results to avoid
// re-fetching the same (chunk, depth) within a session.
let resultCache = new Map<string, NeighborResult | null>();
let pendingPromises = new Map<string, Promise<NeighborResult | null>>();

/**
 * Test-only: clear all module-level caches (resultCache + pendingPromises).
 * Call this in beforeEach to prevent cross-test cache pollution.
 * @internal
 */
export function _resetCache(): void {
  resultCache.clear();
  pendingPromises.clear();
}

function cacheKey(chunkId: string, depth: number): string {
  return `${chunkId}:${depth}`;
}

async function fetchNeighborResult(chunkId: string, depth: number): Promise<NeighborResult | null> {
  const key = cacheKey(chunkId, depth);
  const cached = resultCache.get(key);
  if (cached !== undefined) return cached;

  // Deduplicate concurrent fetches for the same (chunkId, depth).
  const pending = pendingPromises.get(key);
  if (pending !== undefined) return pending;

  const fetchPromise = (async () => {
    try {
      const url = `/api/knowledge/graph/neighbors?chunk_id=${encodeURIComponent(chunkId)}&depth=${encodeURIComponent(String(depth))}`;
      const resp = await fetch(url, { headers: { Accept: 'application/json' } });
      if (!resp.ok) {
        // Network-level failure — cache null, do not throw.
        resultCache.set(key, null);
        return null;
      }
      const json = (await resp.json()) as NeighborResult;
      resultCache.set(key, json);
      return json;
    } catch {
      // Parse error or network failure — cache null, do not throw.
      resultCache.set(key, null);
      return null;
    } finally {
      pendingPromises.delete(key);
    }
  })();

  pendingPromises.set(key, fetchPromise);
  return fetchPromise;
}

export interface UseGraphNeighborsOptions {
  /** Set false to disable automatic fetch on mount. Default: true. */
  enabled?: boolean;
}

export interface UseGraphNeighborsResult {
  /** NeighborResult when the fetch succeeded; null when degraded or not yet fetched. */
  data: NeighborResult | null;
  /** True while a fetch is in flight for the current (chunkId, depth). */
  loading: boolean;
  /** Error message string when the fetch failed; null otherwise. */
  error: string | null;
  /** Re-fetch for a new (chunkId, depth), or re-trigger the same pair. */
  refetch: (chunkId: string, depth?: number) => void;
}

/**
 * Hook: fetch and cache BFS neighbor results for a given chunk.
 *
 * The first fetch for a given (chunkId, depth) pair is stored in the
 * module-level cache. Subsequent calls with the same pair within the same
 * session return the cached result synchronously (no additional network
 * request). Call `refetch` with a new chunkId (and optionally depth) to
 * evict the cache entry and trigger a fresh fetch.
 *
 * Degrades to data=null on any failure (network error, non-2xx, parse
 * error) — the UI should handle null data gracefully.
 */
export function useGraphNeighbors(
  initialChunkId?: string,
  initialDepth?: number,
  options: UseGraphNeighborsOptions = {},
): UseGraphNeighborsResult {
  const [chunkId, setChunkId] = useState<string | undefined>(initialChunkId);
  const [depth, setDepth] = useState<number>(initialDepth ?? 2);
  const [data, setData] = useState<NeighborResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const doFetch = useCallback(async (cid: string, d: number) => {
    const key = cacheKey(cid, d);
    const cached = resultCache.get(key);
    if (cached !== undefined) {
      // Synchronous cache hit — no loading state needed.
      setData(cached);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);
    const result = await fetchNeighborResult(cid, d);
    setData(result);
    setError(result === null ? 'fetch failed' : null);
    setLoading(false);
  }, []);

  useEffect(() => {
    if (options.enabled === false) return;
    if (!chunkId) return;
    void doFetch(chunkId, depth);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chunkId, depth, doFetch, options.enabled]);

  const refetch = useCallback((cid: string, d?: number) => {
    // Evict any cached entry so the next fetch goes to the network.
    const key = cacheKey(cid, d ?? depth);
    resultCache.delete(key);
    setChunkId(cid);
    if (d !== undefined) setDepth(d);
    void doFetch(cid, d ?? depth);
  }, [depth, doFetch]);

  return { data, loading, error, refetch };
}
