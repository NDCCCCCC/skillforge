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
//
// Quick 260929-d5w3 Task 1 — QUESTION FOCUS. When the caller passes a
// `focus` (derived from the latest assistant verdict's citations), the
// canvas defaults to "仅当前问题" mode: cited docs render full-colour,
// enlarged and ring-highlighted, their tiers stay lit, and everything
// else fades to opacity 0.12. A toggle (graph-focus-toggle) flips between
// 仅当前问题 / 全库 and the choice persists (localStorage). An answered
// turn WITHOUT citations (e.g. a refusal) shows the honest empty note and
// falls through to the full graph.
import { useMemo, useState } from 'react';
import { useAppStore } from '../../store/app-store';
import { useTierLabel } from '../../lib/kb-bases';
import { computeFocusMatch, type GraphFocus, type GraphFocusMatch } from '../../lib/graph-focus';
import { prettySourcePath } from '../../lib/pretty-source';
import type { GraphEdge, KnowledgeGraph, NeighborResult, TierNode } from '../../types/knowledge';

const W = 900;
const H = 640;
const CX = W / 2;
const CY = H / 2;
const TIER_RING = 110; // inner ring radius (tier nodes)
const DOC_RING_BASE = 260; // outer ring base radius (doc nodes)

// Palette keyed by registry SLUG (26-08: the tier vocabulary is
// registry-driven). A slug outside the seed palette falls back to the
// primary accent; identity stays TEXT-driven (KNOW-05).
const TIER_COLOR: Record<string, string> = {
  authority: 'var(--color-status-compliant, #16a34a)',
  historical: 'var(--color-status-needs-info, #d97706)',
  personal: 'var(--color-primary, #2563eb)',
  draft: 'var(--color-text-muted, #6b7280)',
};

function tierColor(tier: string): string {
  return TIER_COLOR[tier] ?? 'var(--color-primary, #2563eb)';
}

/** Edge colour by edge_type (Phase 28-05 GRF-04). */
const EDGE_COLOR: Record<string, string> = {
  section_adjacent: 'var(--color-border, #d1d5db)',
  version_chain: 'var(--color-status-needs-info, #d97706)',
  keyword_cooccur: 'var(--color-primary, #2563eb)',
  citation_cooccur: 'var(--color-status-compliant, #16a34a)',
};

function edgeColor(edgeType: string): string {
  return EDGE_COLOR[edgeType] ?? 'var(--color-border, #d1d5db)';
}

/** Human-readable Chinese label for each edge_type. */
const EDGE_TYPE_LABEL: Record<string, string> = {
  section_adjacent: '章节相邻',
  version_chain: '版本链',
  keyword_cooccur: '共现',
  citation_cooccur: '引用共现',
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
  /** Optional: the full neighbor result to render as a subgraph (GRF-04).
   * When provided, the canvas switches to neighbor subgraph mode. */
  neighborResult?: NeighborResult | null;
  /** Optional edges to render as overlay lines on the radial graph (GRF-04).
   * These are distinct from the neighbor subgraph mode. */
  overlayEdges?: GraphEdge[];
  /** Called when user clicks a doc node to expand its neighbors. */
  onExpandNeighbors?: (chunkId: string, depth?: number) => void;
  /** Quick 260929-d5w3 — citations of the latest assistant verdict (see
   * lib/graph-focus). null/undefined = no focus → full-graph mode. */
  focus?: GraphFocus | null;
  /** True when ≥1 assistant turn has answered (verdict OR refusal). With
   * focus === null this drives the "当前回答无引用原文" honest empty note. */
  hasAnswer?: boolean;
}

/** Focus-mode preference key. "记住选择" — the toggle persists across
 * remounts (tab switches unmount the browser). */
const FOCUS_MODE_STORAGE_KEY = 'skillforge.graph.focusMode';

type FocusMode = 'focus' | 'all';

function loadFocusMode(): FocusMode {
  try {
    return window.localStorage.getItem(FOCUS_MODE_STORAGE_KEY) === 'all' ? 'all' : 'focus';
  } catch {
    return 'focus';
  }
}

function saveFocusMode(mode: FocusMode): void {
  try {
    window.localStorage.setItem(FOCUS_MODE_STORAGE_KEY, mode);
  } catch {
    // Private-mode / storage-disabled — the preference just doesn't persist.
  }
}

/** Sub-component: renders the neighbor subgraph when a node is expanded.
 * Shows the clicked node centered, its neighbors in a ring, and the edges
 * between them. When overlayEdges is set GraphCanvas enters "neighbor mode"
 * and shows only this subgraph (per Plan 28-05 research §7 Risk R2). */
function NeighborSubgraph({
  result,
  centerX,
  centerY,
}: {
  result: NeighborResult;
  centerX: number;
  centerY: number;
}) {
  const RING_RADIUS = 120;
  const CENTER_R = 12;

  // Build a lookup from chunk_id to position.
  // result.nodes[0] is the clicked center node; rest are ring nodes.
  const centerNode = result.nodes[0];
  const ringNodes = result.nodes.slice(1);

  const positions = new Map<string, { x: number; y: number }>();
  if (centerNode) {
    positions.set(centerNode.chunk_id, { x: centerX, y: centerY });
  }
  ringNodes.forEach((n, i) => {
    const angle = (2 * Math.PI * i) / Math.max(ringNodes.length, 1) - Math.PI / 2;
    positions.set(n.chunk_id, {
      x: centerX + RING_RADIUS * Math.cos(angle),
      y: centerY + RING_RADIUS * Math.sin(angle),
    });
  });

  // Build a set of edges that connect nodes we have positions for.
  const renderedEdges = result.edges.filter(
    (e) => positions.has(e.from) && positions.has(e.to),
  );

  return (
    <>
      {/* Edges as straight lines between connected nodes. */}
      {renderedEdges.map((edge) => {
        const from = positions.get(edge.from)!;
        const to = positions.get(edge.to)!;
        return (
          <line
            key={`ne-${edge.from}-${edge.to}`}
            x1={from.x}
            y1={from.y}
            x2={to.x}
            y2={to.y}
            stroke={edgeColor(edge.edge_type)}
            strokeWidth={Math.max(1, Math.round(edge.weight * 2))}
            strokeOpacity={0.7}
          />
        );
      })}
      {/* Ring nodes */}
      {ringNodes.map((n) => {
        const pos = positions.get(n.chunk_id)!;
        return (
          <g
            key={`nn-${n.chunk_id}`}
            data-testid="neighbor-node"
            data-chunk-id={n.chunk_id}
            transform={`translate(${pos.x}, ${pos.y})`}
          >
            <circle r={8} fill="var(--color-surface, #fff)" stroke={tierColor(n.tier)} strokeWidth={2} />
            <text y={-12} textAnchor="middle" fontSize={9} fill="var(--color-text, #111827)">
              {prettySourcePath(n.source).slice(0, 12)}
            </text>
            <text y={20} textAnchor="middle" fontSize={8} fill="var(--color-text-muted, #6b7280)">
              {n.chunk_id}
            </text>
          </g>
        );
      })}
      {/* Center (clicked) node */}
      {centerNode && (
        <g key="neighbor-center" data-testid="neighbor-center-node" transform={`translate(${centerX}, ${centerY})`}>
          <circle r={CENTER_R} fill={tierColor(centerNode.tier)} opacity={0.2} />
          <circle r={CENTER_R} fill="var(--color-surface, #fff)" stroke={tierColor(centerNode.tier)} strokeWidth={2.5} />
          <text y={4} textAnchor="middle" fontSize={10} fontWeight={600} fill="var(--color-text, #111827)">
            中心
          </text>
        </g>
      )}
    </>
  );
}

export default function GraphCanvas({ graph, filter, neighborResult, overlayEdges, onExpandNeighbors, focus, hasAnswer }: GraphCanvasProps) {
  const setCenterView = useAppStore((s) => s.setCenterView);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [hovered, setHovered] = useState<string | null>(null);
  const [hoveredEdge, setHoveredEdge] = useState<string | null>(null);
  // Question-focus mode (260929-d5w3): defaults to 仅当前问题 whenever a
  // focus exists, unless the operator previously chose 全库 (persisted).
  const [focusMode, setFocusMode] = useState<FocusMode>(loadFocusMode);

  // Neighbor subgraph mode: only when neighborResult is explicitly provided.
  // The overlayEdges are rendered on top of the radial graph when in normal mode.
  const isNeighborMode = Boolean(neighborResult);

  const positioned = useMemo(() => {
    if (isNeighborMode) return [];
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
  }, [graph, filter, collapsed, isNeighborMode]);

  // Citations ∩ graph docs. null when the focus misses the current corpus
  // (stale session vs re-index) — the canvas then stays full-graph instead
  // of fading everything to invisible.
  const focusMatch: GraphFocusMatch | null = useMemo(
    () => computeFocusMatch(graph, focus),
    [graph, focus],
  );
  const focusActive = focusMode === 'focus' && focusMatch !== null;
  // Answered turn but zero citations (refusal / no_authorized_source) —
  // the honest empty state from the spec. Note only; mode stays full-graph.
  const showFocusEmptyNote = Boolean(hasAnswer) && focus == null;

  function toggleFocusMode() {
    setFocusMode((prev) => {
      const next: FocusMode = prev === 'focus' ? 'all' : 'focus';
      saveFocusMode(next);
      return next;
    });
  }

  function openDoc(tier: string, filename: string) {
    // tier is a plain registry slug (26-08 widened Tier to string).
    setCenterView({ kind: 'doc-view', tier, file: filename });
  }

  /** Called when user clicks a doc node to expand its neighbors. */
  function expandNeighbors(chunkId: string) {
    if (onExpandNeighbors) onExpandNeighbors(chunkId);
  }

  /** Build a stable edge key for hover tracking. */
  function edgeKey(e: GraphEdge): string {
    return `${e.from}:${e.to}:${e.edge_type}`;
  }

  return (
    <div data-testid="graph-canvas" className="rounded-md border border-border bg-surface">
      {/* Question-focus toolbar (260929-d5w3). Renders when a focus matched
          the graph (toggle) or when an answered turn had no citations (note). */}
      {(focusMatch !== null || showFocusEmptyNote) && (
        <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-1.5 text-xs">
          {focusMatch !== null ? (
            <>
              <span className="text-text-muted">
                {focusActive
                  ? `已聚焦本次回答引用的 ${focusMatch.docIds.size} 篇文档`
                  : '当前显示全库'}
              </span>
              <button
                type="button"
                data-testid="graph-focus-toggle"
                onClick={toggleFocusMode}
                aria-pressed={focusActive}
                title="在「仅当前问题」与「全库」之间切换"
                className="inline-flex shrink-0 items-center gap-1 rounded-md border border-border bg-surface-1 px-2 py-0.5 font-semibold text-text hover:bg-surface-2"
              >
                {focusActive ? '仅当前问题' : '全库'}
              </button>
            </>
          ) : (
            <span data-testid="graph-focus-empty" className="text-text-muted">
              当前回答无引用原文 — 显示全库
            </span>
          )}
        </div>
      )}
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        role="img"
        aria-label="知识库关系图：层级与文档节点"
      >
        {/* ── Neighbor subgraph mode ── */}
        {isNeighborMode ? (
          <NeighborSubgraph result={neighborResult!} centerX={CX} centerY={CY} />
        ) : (
          <>
            {/* tier→doc edges */}
            {positioned.map((t) =>
              t.docs.map((d) => (
                <line
                  key={`e-${d.key}`}
                  x1={t.x}
                  y1={t.y}
                  x2={d.x}
                  y2={d.y}
                  stroke={hovered === d.key ? 'var(--color-primary, #2563eb)' : 'var(--color-border, #d1d5db)'}
                  strokeWidth={hovered === d.key ? 2 : 1}
                  strokeOpacity={
                    focusActive
                      ? focusMatch!.docIds.has(d.key)
                        ? 0.8
                        : 0.08
                      : 0.55
                  }
                />
              )),
            )}
            {/* overlay relation edges (section_adjacent, version_chain, keyword_cooccur, citation_cooccur) */}
            {overlayEdges && overlayEdges.map((edge) => {
              const ek = edgeKey(edge);
              const isHovered = hoveredEdge === ek;
              return (
                <line
                  key={`ov-${ek}`}
                  x1={CX}
                  y1={CY}
                  x2={CX + 80}
                  y2={CY}
                  stroke={edgeColor(edge.edge_type)}
                  strokeWidth={isHovered ? Math.max(2, Math.round(edge.weight * 3)) : Math.max(1, Math.round(edge.weight * 2))}
                  strokeOpacity={isHovered ? 1 : 0.6}
                  onMouseEnter={() => setHoveredEdge(ek)}
                  onMouseLeave={() => setHoveredEdge(null)}
                />
              );
            })}
            {/* doc nodes */}
            {positioned.map((t) =>
              t.docs.map((d) => {
                const active = hovered === null || hovered === d.key;
                // Question focus (260929-d5w3): cited docs stay lit and
                // grow a dashed highlight ring; everything else drops to
                // 0.12 (hover briefly reveals a faded doc at 0.35 so it
                // stays discoverable).
                const isFocusDoc = focusActive ? focusMatch!.docIds.has(d.key) : true;
                const opacity = focusActive
                  ? isFocusDoc
                    ? 1
                    : hovered === d.key
                      ? 0.35
                      : 0.12
                  : active
                    ? 1
                    : 0.35;
                return (
                  <g
                    key={`n-${d.key}`}
                    data-testid="graph-doc-node"
                    data-tier={d.tier}
                    data-file={d.filename}
                    data-focus={focusActive ? (isFocusDoc ? 'true' : 'false') : undefined}
                    transform={`translate(${d.x}, ${d.y})`}
                    opacity={opacity}
                    onMouseEnter={() => setHovered(d.key)}
                    onMouseLeave={() => setHovered(null)}
                    onClick={() => {
                      openDoc(d.tier, d.filename);
                      expandNeighbors(d.key);
                    }}
                    className="cursor-pointer"
                  >
                    {/* Raw source path stays hoverable (SVG <title> tooltip);
                        the label renders the pretty short form. */}
                    <title>{d.filename}</title>
                    {focusActive && isFocusDoc && (
                      // Dashed highlight ring — the "焦点文档高亮环".
                      <circle
                        r={16}
                        fill="none"
                        stroke={tierColor(d.tier)}
                        strokeWidth={1.5}
                        strokeDasharray="3 3"
                      />
                    )}
                    <circle
                      r={focusActive && isFocusDoc ? 11 : 9}
                      fill="var(--color-surface, #fff)"
                      stroke={tierColor(d.tier)}
                      strokeWidth={focusActive && isFocusDoc ? 2.5 : 2}
                    />
                    <text y={-14} textAnchor="middle" fontSize={11} fill="var(--color-text, #111827)">
                      {prettySourcePath(d.filename).slice(0, 18)}
                    </text>
                    <text y={22} textAnchor="middle" fontSize={9} fill="var(--color-text-muted, #6b7280)">
                      {d.chunkCount} chunk
                    </text>
                  </g>
                );
              }),
            )}
            {/* tier nodes */}
            {positioned.map((t) => {
              // Focus tiers (owning ≥1 cited doc) stay lit; the rest 淡显.
              const isFocusTier = focusActive ? focusMatch!.tiers.has(t.tier) : true;
              return (
                <g
                  key={`t-${t.tier}`}
                  data-testid="graph-tier-node"
                  data-tier={t.tier}
                  data-focus={focusActive ? (isFocusTier ? 'true' : 'false') : undefined}
                  transform={`translate(${t.x}, ${t.y})`}
                  opacity={focusActive && !isFocusTier ? 0.15 : 1}
                  onClick={() => setCollapsed((prev) => ({ ...prev, [t.tier]: !prev[t.tier] }))}
                  className="cursor-pointer"
                >
                  <circle r={26} fill={tierColor(t.tier)} opacity={0.16} />
                  <circle r={26} fill="none" stroke={tierColor(t.tier)} strokeWidth={2} />
                  <text textAnchor="middle" dy={-2} fontSize={14} fontWeight={600} fill="var(--color-text, #111827)">
                    <TierLabel tier={t.tier} />
                  </text>
                  <text textAnchor="middle" dy={14} fontSize={9} fill="var(--color-text-muted, #6b7280)">
                    {t.docCount} 文档
                  </text>
                </g>
              );
            })}
          </>
        )}
        {/* Edge type label tooltip when hovering an overlay edge */}
        {hoveredEdge && (
          <text x={CX + 84} y={CY - 6} fontSize={10} fill="var(--color-text-muted, #6b7280)">
            {EDGE_TYPE_LABEL[overlayEdges?.find((e) => edgeKey(e) === hoveredEdge)?.edge_type ?? ''] ?? hoveredEdge}
          </text>
        )}
      </svg>
      <div className="border-t border-border px-3 py-1.5 text-xs text-text-muted">
        {isNeighborMode
          ? '邻居视图 · 点击空白处返回全图'
          : '点击文档节点查看正文 · 点击层级节点折叠/展开 · 悬停边查看类型'}
      </div>
    </div>
  );
}

// TierLabel — registry display label for one tier node (a component because
// the label resolves through the useTierLabel hook; layout is untouched).
function TierLabel({ tier }: { tier: string }) {
  return <>{useTierLabel(tier)}</>;
}
