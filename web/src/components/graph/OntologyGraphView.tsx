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
// derives ≥1 note id (citation source basename leading digits → note_XXX,
// see lib/graph-focus noteIdsFromFocus), the view fetches that focused
// neighborhood; the toggle flips 仅当前问题 ↔ 全库 and persists under the
// SAME localStorage key GraphCanvas uses, so the two views share one
// operator preference.
import { useEffect, useMemo, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import {
  fetchOntologyGraph,
  OntologyGraphApiError,
  type OntologyGraph,
  type OntologyNode,
} from './ontologyApi';
import { noteIdsFromFocus, type GraphFocus } from '../../lib/graph-focus';

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
  const focusNoteIds = useMemo(() => noteIdsFromFocus(focus), [focus]);
  // Wire param string — the fetch effect keys on THIS (array identity
  // would refetch on every render).
  const notesKey = focusMode === 'focus' ? focusNoteIds.join(',') : '';

  const [data, setData] = useState<OntologyGraph | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<OntologyNode | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setSelected(null);
    fetchOntologyGraph(focusMode === 'focus' ? focus : null)
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
  }, [notesKey]);

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
      wheelSensitivity: 0.2,
      userZoomingEnabled: true,
      userPanningEnabled: true,
      autoungrabify: true,
      boxSelectionEnabled: false,
      style: [
        {
          selector: 'node',
          style: {
            label: 'data(label)',
            'font-family': 'var(--font-sans)',
            'font-size': '11px',
            shape: 'round-rectangle',
            width: 'label',
            height: 22,
            padding: '8px',
            'background-color': primary,
            'border-width': 1.5,
            'border-color': primaryHover,
            color: text,
            'text-valign': 'bottom',
            'text-margin-y': 4,
          },
        },
        {
          selector: 'edge',
          style: {
            width: 1.5,
            'line-color': borderMuted,
            'target-arrow-shape': 'triangle',
            'target-arrow-color': borderMuted,
            'curve-style': 'straight',
          },
        },
        // parent_org dashed (hierarchy is a weaker, derived relation).
        {
          selector: 'edge[type = "parent_org"]',
          style: {
            'line-style': 'dashed',
            'target-arrow-shape': 'triangle',
          },
        },
        {
          selector: 'node[kind = "note"]',
          style: { 'background-color': primary, 'border-color': primaryHover },
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
            'border-width': 3,
            'border-color': text,
          },
        },
      ],
    });

    cy.on('tap', 'node', (e) => {
      const id = e.target.id();
      const node = data.nodes.find((n) => n.id === id);
      if (node) {
        cy.elements().removeClass('selected');
        cy.$(`#${CSS.escape(id)}`).addClass('selected');
        setSelected(node);
      }
    });
    cy.on('tap', (e) => {
      if (e.target === cy) {
        cy.elements().removeClass('selected');
        setSelected(null);
      }
    });

    return () => {
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
        <div className="text-sm text-text-muted">正在加载本体关系…</div>
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
      className="flex min-h-0 flex-1 flex-col rounded-md border border-border bg-surface"
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
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        {/* Cytoscape container */}
        <div className="min-h-0 flex-1">
          <div
            ref={containerRef}
            data-testid="ontology-graph-canvas"
            className="h-full w-full"
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
          滚轮缩放 · 拖拽平移 · {EDGE_LABEL.references_table}实线 · {EDGE_LABEL.parent_org}虚线
        </span>
      </div>
    </div>
  );
}
