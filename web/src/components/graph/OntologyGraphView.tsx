// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-g7x — OntologyGraphView, the 本体视图.
//
// Renders the note-centric ABox projection (GET /api/admin/ontology/graph,
// see ontologyApi.ts) with Cytoscape.js — the SAME plain-npm posture as
// governance/ProvenanceGraphView (no React wrapper): mount via container
// ref + useEffect, destroy on unmount, colors read from CSS variables at
// mount. cose force layout carries the 全库 mode (200+ nodes); zoom/pan is
// cytoscape-native.
//
// Node kind palette (UI-SPEC tokens; no purple token exists in theme.css,
// so organization hardcodes violet-400 with the token fallback comment):
//   note               蓝  --color-primary
//   table              橙  --color-status-needs-info
//   insurance_product  绿  --color-status-compliant
//   organization       紫  #a78bfa (hardcoded — no theme token)
//
// Question focus (260929-d5w3 semantics, 择简 variant): when a focus
// derives ≥1 note id the view fetches that focused neighborhood; the
// toggle flips 仅当前问题 ↔ 全库 and persists under the SAME localStorage
// key GraphCanvas uses, so the two views share one operator preference.
//
// Follow-up (same quick) — wire citations carry ONLY chunk_id/span/tier,
// so note ids resolve ASYNC: focus.chunkIds → fetchChunkContent (existing
// GET /api/knowledge/chunk/:id, returns the chunk's source path) →
// basename leading digits → note_XXX (lib/graph-focus
// resolveFocusNoteIds). Resolution failure is per-chunk; all chunks
// failing degrades to 全库. The projection fetch is held while resolving.
import { useEffect, useMemo, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import {
  fetchOntologyGraph,
  OntologyGraphApiError,
  type OntologyGraph,
  type OntologyNode,
} from './ontologyApi';
import {
  noteIdsFromFocus,
  resolveFocusNoteIds,
  type GraphFocus,
} from '../../lib/graph-focus';

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

  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);

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

  // Cytoscape mount — re-init whenever the data changes (the projection
  // itself is small; a full re-layout is cheaper than incremental sync).
  useEffect(() => {
    if (!containerRef.current || !data) return;

    // Read CSS tokens at mount time (ProvenanceGraphView posture).
    const cs = getComputedStyle(document.documentElement);
    const token = (name: string) => cs.getPropertyValue(name).trim();
    const primary = token('--color-primary');
    const primaryHover = token('--color-primary-hover');
    const borderMuted = token('--color-border-muted');
    const text = token('--color-text');
    const textMuted = token('--color-text-muted');
    const amber = token('--color-status-needs-info');
    const green = token('--color-status-compliant');
    const violet = '#a78bfa'; // organization 紫 — no purple token in theme.css

    const cy = cytoscape({
      container: containerRef.current,
      elements: [
        ...data.nodes.map((n) => ({
          data: { id: n.id, label: n.label, kind: n.kind, tier: n.tier ?? '' },
        })),
        ...data.edges.map((e) => ({
          data: {
            id: `${e.from}|${e.to}|${e.type}`,
            source: e.from,
            target: e.to,
            type: e.type,
          },
        })),
      ],
      layout: {
        name: 'cose',
        animate: false,
        padding: 24,
        nodeOverlap: 8,
        idealEdgeLength: 80,
      } as cytoscape.LayoutOptions,
      // 0.2 (cytoscape's conservative default here) felt sluggish on the
      // dense 全库 graph — 0.6 gives a decisive wheel step.
      wheelSensitivity: 0.6,
      userZoomingEnabled: true,
      userPanningEnabled: true,
      // Nodes ARE grabbable — dragging a node out of the way is the
      // primary declutter interaction on the dense 全库 graph.
      autoungrabify: false,
      boxSelectionEnabled: false,
      style: [
        {
          selector: 'node',
          style: {
            label: 'data(label)',
            'font-family': 'var(--font-sans)',
            'font-size': '11px',
            'font-weight': 600,
            shape: 'round-rectangle',
            width: 'label',
            height: 26,
            padding: '10px',
            'background-color': primary,
            'border-width': 2,
            'border-color': primaryHover,
            // Label inside the pill (white on kind color) — reads far
            // better at overview zoom than the old bottom-outside text.
            color: '#ffffff',
            'text-valign': 'center',
            'text-halign': 'center',
            'text-margin-y': 0,
          },
        },
        {
          selector: 'edge',
          style: {
            width: 2,
            'curve-style': 'bezier',
            'control-point-step-size': 40,
            'line-color': borderMuted,
            'target-arrow-shape': 'triangle',
            'target-arrow-color': borderMuted,
          },
        },
        // Edge color by relation type (matches the legend semantics).
        {
          selector: 'edge[type = "references_table"]',
          style: { 'line-color': amber, 'target-arrow-color': amber },
        },
        {
          selector: 'edge[type = "involves_product"]',
          style: { 'line-color': green, 'target-arrow-color': green },
        },
        {
          selector: 'edge[type = "involves_org"]',
          style: { 'line-color': violet, 'target-arrow-color': violet },
        },
        // parent_org dashed (hierarchy is a weaker, derived relation).
        {
          selector: 'edge[type = "parent_org"]',
          style: {
            'line-style': 'dashed',
            'line-color': violet,
            'target-arrow-color': violet,
            'target-arrow-shape': 'triangle',
          },
        },
        {
          selector: 'node[kind = "note"]',
          style: {
            'background-color': primary,
            'border-color': primaryHover,
            // Notes are the hubs — slightly taller so they read first.
            height: 30,
            'font-size': '12px',
          },
        },
        {
          selector: 'node[kind = "table"]',
          style: { 'background-color': amber, 'border-color': amber },
        },
        {
          selector: 'node[kind = "insurance_product"]',
          style: { 'background-color': green, 'border-color': green },
        },
        {
          selector: 'node[kind = "organization"]',
          style: { 'background-color': violet, 'border-color': violet },
        },
        {
          selector: 'node.selected',
          style: {
            'border-width': 4,
            'border-color': text,
            'overlay-color': primary,
            'overlay-opacity': 0.18,
            'overlay-padding': 10,
          },
        },
        {
          selector: '.faded',
          style: {
            opacity: 0.12,
            'text-opacity': 0.25,
          },
        },
        {
          selector: 'edge.faded',
          style: {
            opacity: 0.06,
          },
        },
      ],
    });
    cyRef.current = cy;

    cy.on('tap', 'node', (e) => {
      const id = e.target.id();
      const node = data.nodes.find((n) => n.id === id);
      if (node) {
        // Neighborhood highlight (mirrors the old document graph's
        // selected ring, plus neighbor-vs-rest dimming so relations
        // read at a glance on the 100+ node 全库 graph).
        cy.elements().removeClass('selected').addClass('faded');
        const self = cy.$(`#${CSS.escape(id)}`);
        self.removeClass('faded').addClass('selected');
        self.neighborhood().removeClass('faded');
        setSelected(node);
      }
    });
    cy.on('tap', (e) => {
      if (e.target === cy) {
        cy.elements().removeClass('selected').removeClass('faded');
        setSelected(null);
      }
    });

    // Zero-height guard: the tab body sizes by content, so the container
    // can be 0px tall at init (blank-canvas bug). Re-measure on every
    // size change; fit once the canvas first gains real height.
    let sawHeight = false;
    const ro = new ResizeObserver(() => {
      const h = containerRef.current?.clientHeight ?? 0;
      if (h > 0 && !sawHeight) {
        sawHeight = true;
        cy.resize();
        cy.fit(undefined, 24);
      } else if (h > 0) {
        cy.resize();
      }
    });
    ro.observe(containerRef.current);

    return () => {
      ro.disconnect();
      cyRef.current = null;
      cy.destroy();
    };
  }, [data]);

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
            onClick={() => cyRef.current?.fit(undefined, 24)}
            title="重新适配全部节点"
            className="ml-1 rounded border border-border px-1.5 py-0.5 font-semibold text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          >
            适应
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* Cytoscape container */}
        <div className="min-h-0 flex-1">
          <div
            ref={containerRef}
            data-testid="ontology-graph-canvas"
            className="h-full w-full min-h-[300px]"
            role="img"
            aria-label={`本体图谱：${data.nodes.length} 个实体、${data.edges.length} 条关系`}
          />
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
          滚轮缩放 · 拖拽平移 · 节点可拖拽 · 点节点高亮邻域 · {EDGE_LABEL.references_table}实线 · {EDGE_LABEL.parent_org}虚线
        </span>
      </div>
    </div>
  );
}
