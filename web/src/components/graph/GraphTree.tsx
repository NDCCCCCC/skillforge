// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-5 (KNOW-06) — GraphTree component.
//
// Renders the 3-level Accordion tree (tier -> doc -> chunk). Reusable
// — the LeftSidebar file tree (Phase 5 deferred) and the
// KnowledgeGraphBrowser both consume it.
//
// Tier text uses --color-primary (NOT annotation teal — teal is
// reserved for the SC-6 counter-evidence badge). Per UI-SPEC §SC-5:
//
//   - Tier label: `{tier}（{docCount} 文档 / {chunkCount} 片段）`
//   - Doc label:  `{filename} · {chunkCount} 片段`
//   - Chunk row:  `chunkId · {version badge}` (handled by GraphChunkNode)
//
// The tree uses native <details>/<summary> for the Accordion — Phase 5
// deliberately avoids a heavy Radix Accordion dependency for a 3-level
// static browse surface (per CONTEXT §Claude's Discretion: "lean
// toward a lightweight tree/list to avoid a heavy d3/react-flow
// dependency").

import GraphChunkNode from './GraphChunkNode';
import { useTierLabel } from '../../lib/kb-bases';
import type { KnowledgeGraph, TierNode } from '../../types/knowledge';

export interface GraphTreeProps {
  graph: KnowledgeGraph;
  /** Optional filter string — chunks whose ID or doc filename contains
   * the substring (case-insensitive) remain visible. Empty / null
   * disables the filter. */
  filter?: string;
}

export default function GraphTree({ graph, filter }: GraphTreeProps) {
  const needle = filter?.trim().toLowerCase() ?? '';

  return (
    <div data-testid="graph-tree" className="flex flex-col gap-3">
      {graph.tiers.map((tier) => (
        <TierSection key={tier.tier} tier={tier} needle={needle} />
      ))}
    </div>
  );
}

function TierSection({
  tier,
  needle,
}: {
  tier: TierNode;
  needle: string;
}) {
  // 26-08 — the section heading shows the REGISTRY display label (the KB
  // projection's display_name), not the raw slug; an unknown slug degrades
  // to `[slug]` exactly like TierBadge.
  const label = useTierLabel(tier.tier);
  const visibleDocs = needle
    ? tier.docs.filter((d) => d.filename.toLowerCase().includes(needle))
    : tier.docs;
  const totalChunks = visibleDocs.reduce((acc, d) => acc + d.chunkCount, 0);

  return (
    <details
      data-testid="graph-tier"
      data-tier={tier.tier}
      data-doc-count={visibleDocs.length}
      data-chunk-count={totalChunks}
      open
      className="rounded-md border border-border bg-surface-1"
    >
      <summary
        className="cursor-pointer px-3 py-2 text-sm font-semibold text-primary"
      >
        {label}（{visibleDocs.length} 文档 / {totalChunks} 片段）
      </summary>
      {visibleDocs.length === 0 ? (
        <div
          data-testid="graph-tier-empty"
          className="border-t border-border px-3 py-2 text-xs text-text-dim"
        >
          {needle ? '无匹配文档' : '暂无文档'}
        </div>
      ) : (
        <div className="flex flex-col gap-2 border-t border-border p-2">
          {visibleDocs.map((doc) => (
            <details
              key={doc.id}
              data-testid="graph-doc"
              data-doc-id={doc.id}
              data-chunk-count={doc.chunkCount}
              open={!!needle}
              className="rounded-md bg-bg"
            >
              <summary className="cursor-pointer px-2 py-1 text-xs font-medium text-text">
                {doc.filename} · {doc.chunkCount} 片段
              </summary>
              <div
                data-testid="graph-doc-chunks"
                className="flex flex-col gap-1 border-t border-border p-2"
              >
                {doc.chunks.map((chunk) => (
                  <GraphChunkNode
                    key={chunk.id}
                    chunkId={chunk.id}
                    effectiveFrom={chunk.effectiveFrom}
                    effectiveTo={chunk.effectiveTo}
                    version={chunk.version}
                    tier={tier.tier}
                  />
                ))}
              </div>
            </details>
          ))}
        </div>
      )}
    </details>
  );
}