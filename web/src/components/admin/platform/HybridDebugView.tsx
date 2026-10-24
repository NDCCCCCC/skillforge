// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — Hybrid 调试 view.
// Query input + max_hops select + rerank toggle,
// KG Expansion section (node-list + detail panel),
// Milvus Hits table with top-5 highlighted.
//
// 2610 redesign: source-kind badges migrated to the shared
// SourceKindBadge, the KG node table to the shared DataTable, the
// detail button to the shared Button, loading/empty states to
// LoadingSkeleton/EmptyState. The Milvus hits table stays purpose-built
// (row-level testid `hybrid-milvus-hits` plus per-row top-5 highlight
// styling that DataTable's column-driven API does not express). Search
// interaction (POST runHybridSearch with X-Skill-Name) is unchanged.

import { useState } from 'react';
import { Search } from 'lucide-react';
import DataTable from '../shared/DataTable';
import { EmptyState, LoadingSkeleton } from '../shared/States';
import { SourceKindBadge } from '../shared/SourceKindBadge';
import { Button } from '../../ui/button';
import { runHybridSearch } from './platformApi';
import type { HybridKGNode, HybridResponse } from './types';

interface Props {
  skillName: string;
}

function NodeDetailPanel({ node }: { node: HybridKGNode | null }) {
  if (!node) return <div className="w-72 text-text-muted text-xs">点击节点查看详情</div>;
  const prov = node.provenance;
  return (
    <div className="w-72 space-y-2 text-xs">
      <div>
        <span className="text-text-muted">IRI</span>
        <br />
        <span className="font-mono break-all">{node.id}</span>
      </div>
      {prov ? (
        <>
          <div>
            <span className="text-text-muted">来源</span>
            <br />
            <SourceKindBadge kind={prov.source_kind} />
          </div>
          {prov.source_path && (
            <div>
              <span className="text-text-muted">路径</span>
              <br />
              <span className="font-mono break-all">{prov.source_path}</span>
            </div>
          )}
          {prov.line_start != null && (
            <div>
              <span className="text-text-muted">行号</span>
              <br />
              <span className="font-mono">
                L{prov.line_start}–L{prov.line_end ?? prov.line_start}
              </span>
            </div>
          )}
        </>
      ) : null}
      {node.tier && (
        <div>
          <span className="text-text-muted">tier</span>
          <br />
          <span className="font-mono">{node.tier}</span>
        </div>
      )}
    </div>
  );
}

export default function HybridDebugView({ skillName }: Props) {
  const [query, setQuery] = useState('');
  const [maxHops, setMaxHops] = useState<1 | 2>(1);
  const [rerank, setRerank] = useState(true);
  const [response, setResponse] = useState<HybridResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<HybridKGNode | null>(null);

  async function doSearch() {
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    setSelectedNode(null);
    try {
      const resp = await runHybridSearch({ query: query.trim(), max_hops: maxHops, rerank }, skillName);
      setResponse(resp);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
      setResponse(null);
    } finally {
      setLoading(false);
    }
  }

  const milvusHits = response?.milvus_hits ?? [];
  const top5 = milvusHits.slice(0, 5);
  const rest = milvusHits.slice(5);

  return (
    <div className="space-y-4" data-testid="platform-view-hybrid">
      {/* Query input + controls */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-muted" />
          <input
            data-testid="hybrid-search"
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') doSearch(); }}
            placeholder="输入查询词开始混合检索"
            className="w-full rounded-md border border-border bg-surface-2 pl-8 pr-2 py-1.5 text-xs"
          />
        </div>
        <select
          value={maxHops}
          onChange={(e) => setMaxHops(Number(e.target.value) as 1 | 2)}
          className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs"
        >
          <option value={1}>1</option>
          <option value={2}>2</option>
        </select>
        <span className="text-xs text-text-muted">max_hops</span>
        <select
          value={rerank ? 'on' : 'off'}
          onChange={(e) => setRerank(e.target.value === 'on')}
          className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs"
        >
          <option value="on">On</option>
          <option value="off">Off</option>
        </select>
        <span className="text-xs text-text-muted">rerank</span>
      </div>

      {/* Error */}
      {error && (
        <div className="text-sm text-danger">
          检索失败：{error}
          <button onClick={doSearch} className="ml-2 underline">（点击重试）</button>
        </div>
      )}

      {/* Empty state */}
      {!response && !loading && !error && (
        <EmptyState title="混合检索" hint="输入查询词开始混合检索" />
      )}

      {/* Loading */}
      {loading && <LoadingSkeleton rows={3} />}

      {/* KG Expansion + Milvus Hits */}
      {response && !loading && (
        <div className="space-y-6">
          {/* KG Expansion */}
          <section>
            <h3 className="mb-2 text-sm font-semibold">
              KG Expansion ({maxHops} hop) — {response.kg_nodes.length} nodes · {response.kg_edges.length} edges
            </h3>
            <div className="grid gap-4 lg:grid-cols-[1fr_288px]">
              <DataTable
                columns={[
                  {
                    key: 'node',
                    label: 'node',
                    render: (n) => (
                      <span className="block max-w-xs truncate font-mono" title={n.id}>
                        {n.label || n.id}
                      </span>
                    ),
                  },
                  {
                    key: 'type',
                    label: 'type',
                    render: (n) => <SourceKindBadge kind={n.kind} />,
                  },
                  {
                    key: 'tier',
                    label: 'tier',
                    render: (n) => (
                      <span className="font-mono text-text-muted">{n.tier ?? '—'}</span>
                    ),
                  },
                  {
                    key: 'actions',
                    label: 'actions',
                    render: (n) => (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => setSelectedNode(n)}
                      >
                        detail
                      </Button>
                    ),
                  },
                ]}
                rows={response.kg_nodes}
                rowKey={(n) => n.id}
                empty={{ title: '无节点' }}
              />
              <div className="border border-border rounded p-3">
                <NodeDetailPanel node={selectedNode} />
              </div>
            </div>
          </section>

          {/* Milvus Hits */}
          <section>
            <h3 className="mb-2 text-sm font-semibold">Milvus Hits (top-50, reranked top-5 highlighted)</h3>
            <div className="overflow-hidden rounded-md border border-border">
              <table className="w-full text-left text-xs" data-testid="hybrid-milvus-hits">
                <thead className="bg-surface-2 text-text-muted">
                  <tr>
                    <th className="px-3 py-2 text-right">score</th>
                    <th className="px-3 py-2">text snippet</th>
                  </tr>
                </thead>
                <tbody>
                  {milvusHits.length === 0 ? (
                    <tr>
                      <td colSpan={2} className="px-3 py-4 text-center text-text-muted">无命中</td>
                    </tr>
                  ) : (
                    <>
                      {top5.map((hit, i) => (
                        <tr key={`top-${i}`} className="border-t border-border bg-primary-dim/10">
                          <td className="px-3 py-1.5 font-mono text-right text-primary">
                            ★ {hit.score.toFixed(2)}
                          </td>
                          <td className="px-3 py-1.5 max-w-xs truncate" title={hit.text}>
                            {hit.text}
                          </td>
                        </tr>
                      ))}
                      {rest.map((hit, i) => (
                        <tr key={`rest-${i}`} className="border-t border-border">
                          <td className="px-3 py-1.5 font-mono text-right text-text-muted">
                            {hit.score.toFixed(2)}
                          </td>
                          <td className="px-3 py-1.5 max-w-xs truncate" title={hit.text}>
                            {hit.text}
                          </td>
                        </tr>
                      ))}
                    </>
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
