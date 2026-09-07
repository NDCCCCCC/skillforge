// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeGraphBrowser — Plan 05-05 / SC-5 (KNOW-06) full Accordion
// tree implementation. Replaces the 05-03 empty-state placeholder.
//
// The browser fetches GET /api/knowledge/graph once on mount (via
// useKnowledgeGraph) and renders the 3-level tree (tier -> doc ->
// chunk). Empty + loading + error states branch out per UI-SPEC §SC-5:
//
//   - 知识库为空    — when the corpus has 0 docs across all tiers
//                     (one tier entry per registered knowledge base, so
//                     emptiness is detected by summing docCount).
//   - 正在加载知识层级…  — fetch in flight.
//   - 无法加载知识图谱：{error}  — fetch failed; refetch button.
//
// Click a chunk → setCenterView({kind:'chunk-source', ...}). The
// ChunkSourceView tab in CenterPanel lights up (bidirectional nav via
// the controlled Tabs indicator). Click 引用原文 from a verdict
// citation follows the same dispatcher (reuses the CitationCluster
// path).
import { Network } from 'lucide-react';
import { useMemo, useState } from 'react';
import GraphCanvas from './GraphCanvas';
import GraphFilterInput from './GraphFilterInput';
import { useKnowledgeGraph } from '../../hooks/useKnowledgeGraph';

export interface KnowledgeGraphBrowserProps {
  /**
   * Optional override for the loading state. When the parent owns
   * the lifecycle (rare; the hook is self-driving), this prop keeps
   * the public API compatible with the 05-03 placeholder.
   */
  loading?: boolean;
}

export default function KnowledgeGraphBrowser({
  loading: loadingOverride,
}: KnowledgeGraphBrowserProps = {}) {
  const { graph, status, error, refetch } = useKnowledgeGraph();
  const [filter, setFilter] = useState('');

  const isLoading = loadingOverride ?? status === 'loading';

  // Empty-state hero: render when the corpus has 0 docs across all
  // tiers. The BE emits one tier entry per REGISTERED knowledge base
  // (Phase 26 registry — the fixed 4-tier shape is gone), so emptiness is
  // detected by summing docCount rather than by counting tiers.
  const totalDocs = useMemo(
    () => (graph ? graph.tiers.reduce((acc, t) => acc + t.docCount, 0) : 0),
    [graph],
  );

  if (status === 'error') {
    return (
      <div
        data-testid="knowledge-graph-error"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-status-non-compliant/20 mb-4">
          <Network className="h-6 w-6 text-status-non-compliant" />
        </div>
        <div className="text-2xl font-semibold text-text">
          无法加载知识图谱
        </div>
        <p className="mt-2 text-base text-text-muted max-w-md">
          {error ?? '请稍后重试，或确认服务端已启用 /api/knowledge/graph。'}
        </p>
        <button
          type="button"
          data-testid="graph-refresh"
          onClick={refetch}
          className="mt-4 rounded-md border border-border bg-surface-1 px-3 py-1.5 text-xs font-semibold text-text hover:bg-surface-2"
        >
          重试
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div
        data-testid="knowledge-graph-loading"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Network className="h-6 w-6 text-primary animate-pulse" />
        </div>
        <div className="text-base text-text-muted">正在加载知识层级...</div>
      </div>
    );
  }

  if (!graph || totalDocs === 0) {
    return (
      <div
        data-testid="knowledge-graph-empty"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Network className="h-6 w-6 text-primary" />
        </div>
        <div className="text-2xl font-semibold text-text">知识库为空</div>
        <p className="mt-2 text-base text-text-muted max-w-md">
          请先通过知识索引导入文档到已注册的知识库，或运行 <code className="font-mono text-xs">scripts/spiked index</code> 示例语料。
        </p>
      </div>
    );
  }

  return (
    <div data-testid="knowledge-graph" className="flex flex-col gap-3">
      <GraphFilterInput value={filter} onChange={setFilter} />
      <GraphCanvas graph={graph} filter={filter} />
    </div>
  );
}