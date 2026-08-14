// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// GraphCanvas — the knowledge-graph RELATION view (replaces the GraphTree
// accordion as the default graph rendering). Nodes are tiers + docs; an
// edge means "doc lives in tier". Click a doc node → the DocViewer opens
// the full corpus source in the center panel (a richer target than a
// single chunk — the doc view shows the whole file the chunks came from).
//
// Layout: hierarchical radial, NOT force-directed. Tier nodes sit on an
// inner ring (evenly spaced); each tier's doc nodes fan out on its outer
// arc. Deterministic (no animation jitter), readable at any corpus size,
// and zero dependencies — pure SVG + trigonometry.
//
// Interactions:
//   - hover a node → highlight its edges + tooltip (filename / chunkCount)
//   - click a doc node → setCenterView({kind:'doc-view', tier, file})
//   - click a tier node → collapse / expand its doc fan
import { useMemo, useState } from 'react';
import { useAppStore } from '../../store/app-store';
import type { Tier } from '../../types/verdict';
import type { KnowledgeGraph, TierNode } from '../../types/knowledge';

const W = 900;
const H = 640;
const CX = W / 2;
const CY = H / 2;
const TIER_RING = 110; // inner ring radius (tier nodes)
const DOC_RING_BASE = 260; // outer ring base radius (doc nodes)

const TIER_COLOR: Record<string, string> = {
  权威: 'var(--status-compliant, #16a34a)',
  历史: 'var(--status-needs-info, #d97706)',
  个人: 'var(--primary, #2563eb)',
  草稿: 'var(--text-muted, #6b7280)',
};

interface PositionedDoc {
  key: string;
  tier: string;
  filename: string;
  chunkCount: number;
  x: number;
  y: number;
}
interface PositionedTier {
  tier: string;
  docCount: number;
  chunkCount: number;
  x: number;
  y: number;
  docs: PositionedDoc[];
}

/** Deterministic radial layout: tiers on the inner ring, docs fanned on
 * each tier's outer arc. Pure function — recomputed on filter change. */
function layout(graph: KnowledgeGraph, collapsed: Record<string, boolean>): PositionedTier[] {
  const tiers = graph.tiers.filter((t) => t.docs.length > 0);
  const n = tiers.length;
  return tiers.map((t: TierNode, i) => {
    const tierAngle = (2 * Math.PI * i) / n - Math.PI / 2;
    const tx = CX + TIER_RING * Math.cos(tierAngle);
    const ty = CY + TIER_RING * Math.sin(tierAngle);
    const isCollapsed = collapsed[t.tier];
    const docs: PositionedDoc[] = t.docs.map((d, j) => {
      if (isCollapsed) {
        // Collapsed docs stack just outside the tier node (tiny offsets).
        return {
          key: d.id, tier: t.tier, filename: d.filename, chunkCount: d.chunkCount,
          x: tx + 26 * Math.cos(tierAngle) + (j % 3) * 3,
          y: ty + 26 * Math.sin(tierAngle) + Math.floor(j / 3) * 3,
        };
      }
      // Fan the docs across an arc centered on the tier's angle.
      const spread = Math.min(Math.PI / 2.4, (2 * Math.PI) / Math.max(t.docs.length * 2.2, 8));
      const a = tierAngle + spread * (j - (t.docs.length - 1) / 2);
      const r = DOC_RING_BASE + (i % 2) * 34; // alternate radius slightly to avoid label overlap
      return {
        key: d.id, tier: t.tier, filename: d.filename, chunkCount: d.chunkCount,
        x: CX + r * Math.cos(a),
        y: CY + r * Math.sin(a),
      };
    });
    return { tier: t.tier, docCount: t.docCount, chunkCount: t.chunkCount, x: tx, y: ty, docs };
  });
}

export interface GraphCanvasProps {
  graph: KnowledgeGraph;
  /** Doc-name filter from GraphFilterInput (same behavior as the tree). */
  filter: string;
}

export default function GraphCanvas({ graph, filter }: GraphCanvasProps) {
  const setCenterView = useAppStore((s) => s.setCenterView);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [hovered, setHovered] = useState<string | null>(null);

  const positioned = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return layout(graph, collapsed);
    return layout(
      {
        tiers: graph.tiers.map((t) => ({
          ...t,
          docs: t.docs.filter((d) => d.filename.toLowerCase().includes(needle)),
        })),
      },
      collapsed,
    );
  }, [graph, filter, collapsed]);

  function openDoc(tier: string, filename: string) {
    setCenterView({ kind: 'doc-view', tier: tier as Tier, file: filename });
  }

  return (
    <div data-testid="graph-canvas" className="rounded-md border border-border bg-surface">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        role="img"
        aria-label="知识库关系图：层级与文档节点"
      >
        {/* edges first (under nodes) */}
        {positioned.map((t) =>
          t.docs.map((d) => (
            <line
              key={`e-${d.key}`}
              x1={t.x}
              y1={t.y}
              x2={d.x}
              y2={d.y}
              stroke={hovered === d.key ? 'var(--primary, #2563eb)' : 'var(--border, #d1d5db)'}
              strokeWidth={hovered === d.key ? 2 : 1}
            />
          )),
        )}
        {/* doc nodes */}
        {positioned.map((t) =>
          t.docs.map((d) => {
            const active = hovered === null || hovered === d.key;
            return (
              <g
                key={`n-${d.key}`}
                data-testid="graph-doc-node"
                data-tier={d.tier}
                data-file={d.filename}
                transform={`translate(${d.x}, ${d.y})`}
                opacity={active ? 1 : 0.35}
                onMouseEnter={() => setHovered(d.key)}
                onMouseLeave={() => setHovered(null)}
                onClick={() => openDoc(d.tier, d.filename)}
                className="cursor-pointer"
              >
                <circle r={9} fill="var(--surface-1, #fff)" stroke={TIER_COLOR[d.tier]} strokeWidth={2} />
                <text y={-14} textAnchor="middle" fontSize={11} fill="var(--text, #111827)">
                  {d.filename.replace(/\.md$/, '').slice(0, 18)}
                </text>
                <text y={22} textAnchor="middle" fontSize={9} fill="var(--text-muted, #6b7280)">
                  {d.chunkCount} chunk
                </text>
              </g>
            );
          }),
        )}
        {/* tier nodes */}
        {positioned.map((t) => (
          <g
            key={`t-${t.tier}`}
            data-testid="graph-tier-node"
            data-tier={t.tier}
            transform={`translate(${t.x}, ${t.y})`}
            onClick={() => setCollapsed((prev) => ({ ...prev, [t.tier]: !prev[t.tier] }))}
            className="cursor-pointer"
          >
            <circle r={26} fill={TIER_COLOR[t.tier]} opacity={0.16} />
            <circle r={26} fill="none" stroke={TIER_COLOR[t.tier]} strokeWidth={2} />
            <text textAnchor="middle" dy={-2} fontSize={14} fontWeight={600} fill="var(--text, #111827)">
              {t.tier}
            </text>
            <text textAnchor="middle" dy={14} fontSize={9} fill="var(--text-muted, #6b7280)">
              {t.docCount} 文档
            </text>
          </g>
        ))}
      </svg>
      <div className="border-t border-border px-3 py-1.5 text-xs text-text-muted">
        点击文档节点查看正文 · 点击层级节点折叠/展开
      </div>
    </div>
  );
}
