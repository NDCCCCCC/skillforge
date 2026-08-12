// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-5 (KNOW-06) — knowledge graph fetch hook.
//
// Fetches GET /api/knowledge/graph once on mount, memoizes the result
// in component-local state. Per D5-09 the reload scope is the Skill
// definition only — the knowledge graph is stable across Skill reloads,
// so this hook does NOT subscribe to skill-reload events. Callers can
// invoke refetch() to force a manual reload (e.g. on a Refresh button
// click — the spec reserves the "explicit Refresh" path).

import { useCallback, useEffect, useState } from 'react';
import type { KnowledgeGraph } from '../types/knowledge';

export type GraphStatus = 'idle' | 'loading' | 'ready' | 'error';

export interface UseKnowledgeGraphResult {
  graph: KnowledgeGraph | null;
  status: GraphStatus;
  error: string | null;
  refetch: () => void;
}

/**
 * Fetch and memoize the knowledge graph from /api/knowledge/graph.
 * Returns the current graph + a refetch() handle.
 *
 * One-shot on mount (no polling, no SWR refresh). The graph endpoint
 * is metadata-only (id / tier / source / effective_*) — see
 * types/knowledge.ts for the wire shape.
 */
export function useKnowledgeGraph(): UseKnowledgeGraphResult {
  const [graph, setGraph] = useState<KnowledgeGraph | null>(null);
  const [status, setStatus] = useState<GraphStatus>('idle');
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus('loading');
    setError(null);

    fetch('/api/knowledge/graph', { headers: { Accept: 'application/json' } })
      .then(async (resp) => {
        if (cancelled) return null;
        if (resp.status === 503) {
          // 503 mirrors the wizard handler posture: the BE never
          // initialized the graph layer. Surface as a friendly error
          // rather than a hard fail so the FE can keep showing the
          // empty-state hero.
          const body = (await resp.json().catch(() => ({}))) as {
            error?: string;
          };
          throw new Error(
            body.error ?? 'knowledge graph not initialized on the server',
          );
        }
        if (!resp.ok) {
          const body = (await resp.json().catch(() => ({}))) as {
            error?: string;
          };
          throw new Error(
            body.error ?? `HTTP ${resp.status} ${resp.statusText}`,
          );
        }
        return (await resp.json()) as KnowledgeGraph;
      })
      .then((data) => {
        if (cancelled || !data) return;
        setGraph(data);
        setStatus('ready');
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : String(err);
        setError(msg);
        setStatus('error');
      });

    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const refetch = useCallback(() => {
    setReloadKey((k) => k + 1);
  }, []);

  return { graph, status, error, refetch };
}