// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// InspectorRetrieval — chunk-ID list rendered under each hop in the
// AgentInspector. Each ID is a button that dispatches
// setCenterView({kind:'chunk-source', chunkId, ...}) so the operator
// can browse the chunk verbatim from the inspector.
//
// The dispatch needs a tier for the ChunkSourceView metadata row, but the
// wire chunk carries none (the agent doesn't tag it). 26-08 residual sweep:
// the default is the REGISTRY's first authority-role slug (the citable
// role) resolved from the public projection — the hardcoded CN default was
// deleted because, after the KB migration, that literal is not a slug and
// the dispatch would target a knowledge base that cannot exist. Until the
// projection resolves, the placeholder is the EMPTY STRING and TierBadge
// renders nothing for it (an honest "tier unknown", never a fabricated
// tier).
import { useEffect, useState } from 'react';
import { firstSlugByRole } from '../../lib/kb-bases';
import { useAppStore } from '../../store/app-store';
import type { NeighborResult } from '../../types/knowledge';
import type { RetrievalRow } from './hop-types';

/** Human-readable Chinese label for each edge_type (Phase 28-06). */
const EDGE_TYPE_LABELS: Record<string, string> = {
  section_adjacent: '邻接',
  version_chain: '版本链',
  keyword_cooccur: '共现',
  citation_cooccur: '引用共现',
};

export interface InspectorRetrievalProps {
  retrievals: RetrievalRow[];
  /** Optional tier override for the chunk-source dispatch. When omitted,
   * the first authority-role slug from the KB projection is used ('' until
   * the projection resolves — no tier literal substitute). */
  defaultTier?: string;
}

export default function InspectorRetrieval({
  retrievals,
  defaultTier,
}: InspectorRetrievalProps) {
  const [registryDefault, setRegistryDefault] = useState('');
  const [expandedChunkId, setExpandedChunkId] = useState<string | null>(null);
  const [neighborsResult, setNeighborsResult] = useState<NeighborResult | null>(null);
  const [neighborsLoading, setNeighborsLoading] = useState(false);

  useEffect(() => {
    if (defaultTier !== undefined) return;
    let cancelled = false;
    void firstSlugByRole('authority').then((slugValue) => {
      if (!cancelled) setRegistryDefault(slugValue);
    });
    return () => {
      cancelled = true;
    };
  }, [defaultTier]);

  if (retrievals.length === 0) return null;

  async function handleClick(chunkId: string) {
    if (expandedChunkId === chunkId) {
      setExpandedChunkId(null);
      setNeighborsResult(null);
      return;
    }
    setExpandedChunkId(chunkId);
    setNeighborsLoading(true);
    try {
      const url = `/api/knowledge/graph/neighbors?chunk_id=${encodeURIComponent(chunkId)}&depth=1`;
      const resp = await fetch(url, { headers: { Accept: 'application/json' } });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const json: NeighborResult = await resp.json();
      setNeighborsResult(json);
    } catch {
      setNeighborsResult(null);
    } finally {
      setNeighborsLoading(false);
    }
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: defaultTier ?? registryDefault,
      version: null,
      effectiveDate: null,
      effectiveTo: null,
    });
  }

  return (
    <div data-testid="inspector-retrieval" data-default-tier={defaultTier ?? registryDefault}>
      <div className="text-xs font-semibold text-text-muted">
        检索结果（{retrievals.length} 条）
      </div>
      <ul className="mt-1 flex flex-col gap-1">
        {retrievals.map((r) => (
          <li key={r.chunkId} className="flex items-center gap-2 text-xs">
            <button
              type="button"
              data-testid="inspector-retrieval-row"
              data-chunk-id={r.chunkId}
              onClick={() => handleClick(r.chunkId)}
              className="rounded-md border border-border bg-surface-2 px-2 py-1 font-mono text-text hover:bg-surface-3"
              title={r.title ?? r.chunkId}
            >
              {r.chunkId}
              {r.filename && (
                <span className="ml-2 text-text-muted">{r.filename}</span>
              )}
            </button>
          </li>
        ))}
      </ul>
      {expandedChunkId && (
        <div data-testid="neighbors-panel" className="mt-2 rounded-md border border-border bg-surface-2 p-2">
          <div className="text-xs font-semibold text-text-muted">
            常被一起引用 {neighborsLoading && <span className="ml-1 text-text-muted">加载中…</span>}
          </div>
          {neighborsResult && !neighborsLoading && (
            <>
              {(neighborsResult.edges ?? []).length === 0 ? (
                <div className="mt-1 text-xs text-text-muted">暂无关联片段</div>
              ) : (
                <ul className="mt-1 flex flex-col gap-1">
                  {(neighborsResult.edges ?? []).map((edge) => (
                    <li key={`${edge.from}-${edge.to}`} className="flex items-center gap-2 text-xs">
                      <span className="rounded bg-surface-3 px-1 py-0.5 font-mono">{edge.from}</span>
                      <span className="text-text-muted">→</span>
                      <span className="rounded bg-surface-3 px-1 py-0.5 font-mono">{edge.to}</span>
                      <span className="rounded border border-border px-1 py-0.5 text-text-muted">
                        {EDGE_TYPE_LABELS[edge.edge_type] ?? edge.edge_type}
                      </span>
                      {edge.weight < 1.0 && (
                        <span className="text-text-muted">({edge.weight.toFixed(2)})</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
              {neighborsResult.degraded && (
                <div className="mt-1 text-xs text-status-needs-info">{neighborsResult.degraded_reason}</div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
