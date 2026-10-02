// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — Hybrid 调试 view.
// Query input + max_hops select + rerank toggle,
// KG Expansion section (node-list + detail panel),
// Milvus Hits table with top-5 highlighted.

import { useState } from 'react';
import { Search } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { runHybridSearch } from './platformApi';
import type { HybridKGNode, HybridResponse } from './types';

interface Props {
  skillName: string;
}

const KIND_BADGE: Record<string, string> = {
  file: 'bg-status-compliant/10 text-status-compliant',
  emitter: 'bg-surface-3 text-text-muted',
  induction: 'bg-primary-dim text-primary',
};

function NodeDetailPanel({ node }: { node: HybridKGNode | null }) {
  if (!node) return <div className="w-72 text-text-muted text-xs">点击节点查看详情</div>;
  const prov = node.provenance;
  const isSource = node.kind === 'file';
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
            <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', KIND_BADGE[prov.source_kind] ?? 'bg-surface-3 text-text-muted')}>
              {prov.source_kind === 'file' ? '文件' : prov.source_kind === 'emitter' ? '生成' : '归纳'}
            </span>
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
        <div className="py-12 text-center text-sm text-text-muted">输入查询词开始混合检索</div>
      )}

      {/* Loading */}
      {loading && (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => <div key={i} className="h-10 animate-pulse rounded bg-surface-2" />)}
        </div>
      )}

      {/* KG Expansion + Milvus Hits */}
      {response && !loading && (
        <div className="space-y-6">
          {/* KG Expansion */}
          <section>
            <h3 className="mb-2 text-sm font-semibold">
              KG Expansion ({maxHops} hop) — {response.kg_nodes.length} nodes · {response.kg_edges.length} edges
            </h3>
            <div className="grid gap-4 lg:grid-cols-[1fr_288px]">
              <div className="overflow-hidden rounded-md border border-border">
                <table className="w-full text-left text-xs">
                  <thead className="bg-surface-2 text-text-muted">
                    <tr>
                      <th className="px-3 py-2">node</th>
                      <th className="px-3 py-2">type</th>
                      <th className="px-3 py-2">tier</th>
                      <th className="px-3 py-2">actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {response.kg_nodes.length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-3 py-4 text-center text-text-muted">无节点</td>
                      </tr>
                    ) : (
                      response.kg_nodes.map((n) => (
                        <tr
                          key={n.id}
                          className="border-t border-border hover:bg-surface-2"
                        >
                          <td className="px-3 py-1.5 font-mono truncate max-w-xs" title={n.id}>{n.label || n.id}</td>
                          <td className="px-3 py-1.5">
                            <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', KIND_BADGE[n.kind] ?? 'bg-surface-3 text-text-muted')}>
                              {n.kind}
                            </span>
                          </td>
                          <td className="px-3 py-1.5 font-mono text-text-muted">{n.tier ?? '—'}</td>
                          <td className="px-3 py-1.5">
                            <button
                              type="button"
                              onClick={() => setSelectedNode(n)}
                              className="rounded border border-border bg-surface-2 px-2 py-0.5 text-xs hover:bg-surface-3"
                            >
                              detail
                            </button>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
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
