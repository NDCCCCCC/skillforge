// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-k3m — OntologyGraphView, the 本体视图, now on
// react-force-graph-3d (true-3D force layout; replaces the 260929-g7x
// cytoscape rendering). The DATA PIPELINE is untouched: focus resolution
// (sources sync path + chunk_id→note async path), the
// GET /api/admin/ontology/graph fetch, the 仅当前问题/全库 toggle with the
// SHARED localStorage key, stats/legend, and the detail side panel all
// behave exactly as before — only the RENDERING layer changed.
//
// Why react-force-graph-3d (vs cytoscape): d3-force-3d propagates drags
// through the live simulation (drag a node, its neighbors follow), the
// wheel orbits/zooms a real 3D camera, and directional particles make
// relation direction visible without arrowheads. three.js (~600KB) is
// kept OUT of the main chunk via React.lazy + Suspense (「正在加载三维
// 引擎…」fallback).
//
// Node kind palette (UI-SPEC tokens; colors are RESOLVED hex — three.js
// cannot read CSS var strings; no purple token exists in theme.css so
// organization keeps the g7x hardcoded violet):
//   note               蓝  --color-primary
//   table              橙  --color-status-needs-info
//   insurance_product  绿  --color-status-compliant
//   organization       紫  #a78bfa (hardcoded — no theme token)
//
// Relation color carries over from g7x; parent_org was dashed in 2D —
// 3D has no dash primitive, so it renders at 0.5 alpha instead.
//
// Question focus (260929-d5w3 semantics, 择简 variant): when a focus
// derives ≥1 note id the view fetches that focused neighborhood; the
// toggle flips 仅当前问题 ↔ 全库 and persists under the SAME localStorage
// key GraphCanvas uses, so the two views share one operator preference.
//
// Citations carry ONLY chunk_id/span/tier, so note ids resolve ASYNC:
// focus.chunkIds → fetchChunkContent (GET /api/knowledge/chunk/:id,
// returns the chunk's source path) → basename leading digits → note_XXX
// (lib/graph-focus resolveFocusNoteIds). Resolution failure is
// per-chunk; all chunks failing degrades to 全库. The projection fetch
// is held while resolving.
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ComponentType, MutableRefObject } from 'react';
import type { Object3D, Sprite } from 'three';
import type {
  ForceGraphMethods,
  ForceGraphProps,
  GraphData,
  LinkObject,
  NodeObject,
} from 'react-force-graph-3d';
import {
  fetchOntologyGraph,
  OntologyGraphApiError,
  type OntologyEdge,
  type OntologyGraph,
  type OntologyNode,
} from './ontologyApi';
import {
  noteIdsFromFocus,
  resolveFocusNoteIds,
  type GraphFocus,
} from '../../lib/graph-focus';

/** three.js is heavy — never let it ride into the main chunk. The cast
 * pins the accessor generics (FCwithRef's inference degrades NodeType to
 * {} and would strip the node payload from every accessor). */
const LazyForceGraph3D = lazy(
  () => import('react-force-graph-3d'),
) as unknown as ComponentType<
  ForceGraphProps<FgNode, FgLink> & { ref?: MutableRefObject<FgMethods | undefined> }
>;

/** Focus-mode preference key — SHARED with GraphCanvas (d5w3). */
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

const KIND_LABEL: Record<OntologyNode['kind'], string> = {
  note: 'SQL 笔记',
  table: '数据表',
  insurance_product: '险种',
  organization: '机构',
};

const EDGE_LABEL: Record<string, string> = {
  references_table: '引用表',
  involves_product: '涉及险种',
  involves_org: '涉及机构',
  parent_org: '上级机构',
};

/** #rrggbb → rgba(...) — three.js material color with alpha. */
function withAlpha(hex: string, alpha: number): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return hex;
  const n = parseInt(m[1], 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
}

/** Resolve a CSS token to a concrete hex (three.js can't read var()). */
function readToken(name: string, fallback: string): string {
  try {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
  } catch {
    return fallback;
  }
}

/** Force-graph node payload — the OntologyNode plus engine fields. */
interface FgNode {
  id: string;
  /** react-force-graph's label accessor field. */
  name: string;
  kind: OntologyNode['kind'];
  label: string;
  tier?: string;
  /** Relative size: notes are the hubs, everything else smaller. */
  val: number;
}

interface FgLink {
  source: string;
  target: string;
  type: OntologyEdge['type'];
}

type FgMethods = ForceGraphMethods<NodeObject<FgNode>, LinkObject<FgNode, FgLink>>;

export interface OntologyGraphViewProps {
  /** Citations of the latest assistant verdict (same GraphFocus the
   * document canvas consumes). null/undefined = 全库. */
  focus?: GraphFocus | null;
}

export default function OntologyGraphView({ focus = null }: OntologyGraphViewProps = {}) {
  // 仅当前问题 / 全库 — same preference key as the document canvas.
  const [focusMode, setFocusMode] = useState<FocusMode>(loadFocusMode);
  const [data, setData] = useState<OntologyGraph | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<OntologyNode | null>(null);
  /** Hovered node id — drives the 1.8× magnify. */
  const [hoverId, setHoverId] = useState<string | null>(null);
  /** Measured host size — passed explicitly to ForceGraph3D (without
   * explicit width/height it renders at a degenerate default). */
  const [size, setSize] = useState({ w: 0, h: 0 });
  /** Lazy three.js label factory (kept in the lazy chunk — see
   * ontology3d-labels.ts). null until the dynamic import settles; the
   * nodeThreeObject accessor degrades to default-only meanwhile, and the
   * state flip re-renders so the decorator picks the sprites up. */
  const [label3d, setLabel3d] = useState<{
    makeLabelSprite(text: string, sphereRadius: number): Sprite;
  } | null>(null);

  useEffect(() => {
    let alive = true;
    import('./ontology3d-labels').then((m) => {
      if (alive) setLabel3d(m);
    });
    return () => {
      alive = false;
    };
  }, []);

  const canvasHostRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<FgMethods | undefined>(undefined);
  /** Dataset the auto-fit already ran for (one fit per fetch, not per
   * engine stop after every drag reheat). */
  const fittedGraphRef = useRef<OntologyGraph | null>(null);

  // Focus → note ids, two complementary paths:
  //   - sources (tolerated future citation field) — sync derivation.
  //   - chunk_ids (what the wire actually carries) — ASYNC resolution:
  //     each chunk_id goes through fetchChunkContent to learn its source
  //     path, then basename leading digits → note_XXX. null = resolving
  //     (the projection fetch is HELD until this settles; every chunk
  //     failing yields [] = degrade to 全库, per resolveFocusNoteIds).
  const sourceNoteIds = useMemo(() => noteIdsFromFocus(focus), [focus]);
  // Lazy init: a focus that carries chunk ids starts in the resolving
  // state; anything else starts already settled (nothing to resolve).
  const [chunkNoteIds, setChunkNoteIds] = useState<string[] | null>(() =>
    focus && focus.chunkIds.size > 0 ? null : [],
  );

  useEffect(() => {
    let cancelled = false;
    const chunkIds = focus ? Array.from(focus.chunkIds) : [];
    if (chunkIds.length === 0) {
      setChunkNoteIds([]);
      return;
    }
    setChunkNoteIds(null);
    resolveFocusNoteIds(chunkIds).then((ids) => {
      if (!cancelled) setChunkNoteIds(ids);
    });
    return () => {
      cancelled = true;
    };
  }, [focus]);

  const focusNoteIds = useMemo(() => {
    const merged = [...sourceNoteIds];
    for (const id of chunkNoteIds ?? []) {
      if (!merged.includes(id)) merged.push(id);
    }
    return merged;
  }, [sourceNoteIds, chunkNoteIds]);
  const resolving = chunkNoteIds === null;

  // Wire param string — the fetch effect keys on THIS (array identity
  // would refetch on every render).
  const notesKey = focusMode === 'focus' ? focusNoteIds.join(',') : '';

  useEffect(() => {
    // Hold the projection fetch until chunk→note resolution settles —
    // fetching 全库 in the interim would flash the wrong neighborhood.
    if (resolving) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    setSelected(null);
    fetchOntologyGraph(focusMode === 'focus' ? focusNoteIds : [])
      .then((graph) => {
        if (!cancelled) {
          setData(graph);
          setLoading(false);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setError(
            e instanceof OntologyGraphApiError ? e.message : String(e),
          );
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
    // notesKey is the stable wire-shape of the (focusMode, focus) pair.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [notesKey, resolving]);

  // Node/link engine payloads. Fresh objects per fetch — the engine
  // re-seeds positions (a full re-layout, same posture as the old
  // cytoscape re-init).
  const graphData = useMemo<GraphData<FgNode, FgLink>>(() => {
    if (!data) return { nodes: [], links: [] };
    return {
      nodes: data.nodes.map((n) => ({
        ...n,
        name: n.label,
        val: n.kind === 'note' ? 6 : 3,
      })),
      links: data.edges.map((e) => ({
        source: e.from,
        target: e.to,
        type: e.type,
      })),
    };
  }, [data]);

  // Colors are RESOLVED once per mount — three.js needs concrete rgba
  // strings, not var() references.
  const palette = useMemo(() => ({
    note: readToken('--color-primary', '#2563eb'),
    table: readToken('--color-status-needs-info', '#f59e0b'),
    product: readToken('--color-status-compliant', '#16a34a'),
    // organization 紫 — no purple token in theme.css (g7x posture kept).
    org: '#a78bfa',
    muted: readToken('--color-border-muted', '#64748b'),
  }), []);

  const linkColorFor = useCallback(
    (type: OntologyEdge['type']): string => {
      switch (type) {
        case 'references_table':
          return palette.table;
        case 'involves_product':
          return palette.product;
        case 'involves_org':
          return palette.org;
        case 'parent_org':
          // Hierarchy is a weaker derived relation — 2D dashed it; 3D has
          // no dash primitive, so half-alpha plays the same role.
          return withAlpha(palette.org, 0.5);
        default:
          return palette.muted;
      }
    },
    [palette],
  );

  const nodeColorFor = useCallback(
    (kind: OntologyNode['kind']): string => {
      switch (kind) {
        case 'note':
          return palette.note;
        case 'table':
          return palette.table;
        case 'insurance_product':
          return palette.product;
        case 'organization':
          return palette.org;
        default:
          return palette.muted;
      }
    },
    [palette],
  );

  /** zoomToFit(800ms transition, 40px padding) — once per dataset. */
  const fitOnce = useCallback((graph: OntologyGraph) => {
    if (fittedGraphRef.current === graph) return;
    fittedGraphRef.current = graph;
    fgRef.current?.zoomToFit(800, 40);
  }, []);

  // Zero-height guard (same posture as the cytoscape version): the tab
  // body sizes by content, so the host can be 0px tall when data first
  // lands. Measure via ResizeObserver → explicit width/height props
  // (without them ForceGraph3D renders at a degenerate default size);
  // fit once the canvas first gains real height.
  useEffect(() => {
    const el = canvasHostRef.current;
    if (!data || !el) return;
    let sawReal = false;
    const ro = new ResizeObserver(() => {
      const w = el.clientWidth;
      const h = el.clientHeight;
      if (w <= 0 || h <= 0) return;
      setSize((prev) => (prev.w === w && prev.h === h ? prev : { w, h }));
      if (!sawReal) {
        sawReal = true;
        fitOnce(data);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [data, fitOnce]);

  // Layout density for the ~160-node 全库 graph: tighter repulsion +
  // 60px link distance keeps labels readable without exploding the
  // cluster. Re-applied per dataset (covers engine re-init on refetch).
  useEffect(() => {
    const fg = fgRef.current;
    if (!fg) return;
    fg.d3Force('charge')?.strength(-120);
    fg.d3Force('link')?.distance(60);
  }, [graphData]);

  function toggleFocusMode() {
    setFocusMode((prev) => {
      const next: FocusMode = prev === 'focus' ? 'all' : 'focus';
      saveFocusMode(next);
      return next;
    });
  }

  if (error) {
    return (
      <div
        data-testid="ontology-graph-error"
        className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 text-center"
      >
        <div className="text-sm font-semibold text-text">无法加载本体视图</div>
        <p className="max-w-md text-xs text-text-muted">{error}</p>
      </div>
    );
  }

  if (loading || !data) {
    return (
      <div
        data-testid="ontology-graph-loading"
        className="flex min-h-0 flex-1 items-center justify-center"
      >
        <div className="text-sm text-text-muted">
          {resolving ? '正在解析引用来源…' : '正在加载本体关系…'}
        </div>
      </div>
    );
  }

  if (data.nodes.length === 0) {
    return (
      <div
        data-testid="ontology-graph-empty"
        className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 text-center"
      >
        <div className="text-sm font-semibold text-text">本体暂无数据</div>
        <p className="max-w-md text-xs text-text-muted">
          知识抽取（ECL）尚未写入笔记/实体三元组 — 先运行一次抽取流水线。
        </p>
      </div>
    );
  }

  const counts = data.nodes.reduce<Record<string, number>>((acc, n) => {
    acc[n.kind] = (acc[n.kind] ?? 0) + 1;
    return acc;
  }, {});

  return (
    <div
      data-testid="ontology-graph-view"
      className="flex h-full min-h-[360px] flex-1 flex-col rounded-md border border-border bg-surface"
    >
      {/* Focus toolbar + legend */}
      <div className="flex flex-none flex-wrap items-center justify-between gap-2 border-b border-border px-3 py-1.5 text-xs">
        <div className="flex min-w-0 items-center gap-2">
          {focusNoteIds.length > 0 ? (
            <button
              type="button"
              data-testid="ontology-focus-toggle"
              onClick={toggleFocusMode}
              aria-pressed={focusMode === 'focus'}
              title="在「仅当前问题」与「全库」之间切换"
              className="inline-flex shrink-0 items-center gap-1 rounded-md border border-border bg-surface-1 px-2 py-0.5 font-semibold text-text hover:bg-surface-2"
            >
              {focusMode === 'focus' ? '仅当前问题' : '全库'}
            </button>
          ) : null}
          <span className="min-w-0 truncate text-text-muted">
            {focusMode === 'focus' && focusNoteIds.length > 0
              ? `聚焦 ${focusNoteIds.length} 篇被引笔记的本体邻域`
              : '全库实体关系网'}
          </span>
        </div>
        <div data-testid="ontology-legend" className="flex flex-none items-center gap-2 text-[11px] text-text-muted">
          <span>笔记 {counts.note ?? 0}</span>
          <span>表 {counts.table ?? 0}</span>
          <span>险种 {counts.insurance_product ?? 0}</span>
          <span>机构 {counts.organization ?? 0}</span>
          <button
            type="button"
            data-testid="ontology-fit-btn"
            onClick={() => fgRef.current?.zoomToFit(800, 40)}
            title="重新适配全部节点"
            className="ml-1 rounded border border-border px-1.5 py-0.5 font-semibold text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          >
            适应
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* 3D force-graph canvas (lazy three.js chunk) */}
        <div className="min-h-0 flex-1">
          <div
            ref={canvasHostRef}
            data-testid="ontology-graph-canvas"
            className="h-full w-full min-h-[300px]"
            role="img"
            aria-label={`本体图谱：${data.nodes.length} 个实体、${data.edges.length} 条关系`}
          >
            <Suspense
              fallback={
                <div className="flex h-full min-h-[300px] items-center justify-center text-sm text-text-muted">
                  正在加载三维引擎…
                </div>
              }
            >
              <LazyForceGraph3D
                ref={fgRef}
                graphData={graphData}
                width={size.w || undefined}
                height={size.h || undefined}
                backgroundColor="rgba(0,0,0,0)"
                showNavInfo={false}
                nodeColor={(n) => nodeColorFor(n.kind)}
                nodeLabel={(n) => `${n.label} · ${KIND_LABEL[n.kind]}`}
                nodeVal={(n) => (hoverId === n.id ? n.val * 1.8 : n.val)}
                nodeThreeObjectExtend
                nodeThreeObject={((n: FgNode) =>
                  label3d ? label3d.makeLabelSprite(n.label, n.val * 1.2) : null) as unknown as (
                    obj: NodeObject<FgNode>,
                  ) => Object3D}
                linkColor={(l) => linkColorFor(l.type)}
                linkWidth={1.5}
                linkDirectionalParticles={3}
                linkDirectionalParticleSpeed={0.006}
                onNodeClick={(n) => {
                  const found = data.nodes.find((x) => x.id === n.id);
                  if (found) setSelected(found);
                }}
                onBackgroundClick={() => setSelected(null)}
                onNodeHover={(n) => setHoverId(n ? n.id : null)}
                onEngineStop={() => fitOnce(data)}
              />
            </Suspense>
          </div>
        </div>
        {/* Detail side panel (governance posture) */}
        <div
          data-testid="ontology-node-detail"
          className="w-56 flex-none overflow-auto border-l border-border p-3 text-xs"
        >
          {selected ? (
            <div className="space-y-2">
              <div>
                <span className="text-text-muted">类型</span>
                <br />
                <span className="font-semibold text-text">{KIND_LABEL[selected.kind]}</span>
              </div>
              <div>
                <span className="text-text-muted">名称</span>
                <br />
                <span className="text-text">{selected.label}</span>
              </div>
              {selected.tier && (
                <div>
                  <span className="text-text-muted">tier</span>
                  <br />
                  <span className="font-mono">{selected.tier}</span>
                </div>
              )}
              <div>
                <span className="text-text-muted">IRI</span>
                <br />
                <span className="break-all font-mono text-[10px] text-text-muted">{selected.id}</span>
              </div>
            </div>
          ) : (
            <span className="text-text-muted">点击节点查看本体明细</span>
          )}
        </div>
      </div>

      <div className="flex flex-none items-center justify-between gap-2 border-t border-border px-3 py-1.5 text-xs text-text-muted">
        <span className="min-w-0 truncate">
          左键旋转 · 右键平移 · 滚轮缩放 · 节点可拖拽（力学联动） · 粒子流向=关系方向 · {EDGE_LABEL.parent_org}为半透明
        </span>
      </div>
    </div>
  );
}
