// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 Task 1 — KnowledgeGraphBrowser tests.
//
// Pins the SC-5 (KNOW-06) acceptance assertions for the real Accordion
// implementation that replaced the 05-03 placeholder:
//
//   - Empty corpus (all 4 tiers present with 0 docs) renders the
//     知识库为空 hero.                                   [文档 mode]
//   - Loading prop override renders 正在加载知识层级... and suppresses
//     the empty hero.                                    [文档 mode]
//   - Error from the BE renders 无法加载知识图谱 + 重试 button.
//                                                        [文档 mode]
//   - Populated corpus renders the 3-level tree (tier -> doc -> chunk)
//     and the filter narrows visible docs by filename.   [文档 mode]
//   - Clicking a chunk dispatches setCenterView({kind:"chunk-source", ...}).
//
// Quick 260929-d5w2 — fullscreen overlay pins ( SAME body, Esc close).
//
// Quick 260929-g7x — the browser is a two-mode host: 本体 (OntologyGraphView)
// is the DEFAULT mode; the corpus heroes + GraphCanvas tree all live behind
// the 文档 chip. The ontology view must render regardless of the document
// corpus state, and focus flows into the ontology fetch (?notes=).
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';

// The browser mounts OntologyGraphView (default mode) — jsdom has no
// WebGL, so react-force-graph-3d is mocked here (260929-k3m: was the
// cytoscape mock). The browser tests never touch graph internals; a
// null-rendering stub is enough to satisfy the lazy import.
vi.mock('react-force-graph-3d', () => ({
  default: () => null,
}));

import KnowledgeGraphBrowser from '../components/graph/KnowledgeGraphBrowser';
import { useAppStore } from '../store/app-store';
import type { KnowledgeGraph } from '../types/knowledge';
import type { OntologyGraph } from '../components/graph/ontologyApi';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.removeItem('skillforge.graph.focusMode');
  useAppStore.setState({ centerView: { kind: 'knowledge-graph' } });
});

const EMPTY_GRAPH: KnowledgeGraph = {
  tiers: [
    { tier: '权威', docCount: 0, chunkCount: 0, docs: [] },
    { tier: '历史', docCount: 0, chunkCount: 0, docs: [] },
    { tier: '个人', docCount: 0, chunkCount: 0, docs: [] },
    { tier: '草稿', docCount: 0, chunkCount: 0, docs: [] },
  ],
};

const POPULATED_GRAPH: KnowledgeGraph = {
  tiers: [
    {
      tier: '权威',
      docCount: 1,
      chunkCount: 2,
      docs: [
        {
          id: '权威::employee-travel.md',
          filename: '员工差旅管理办法.md',
          chunkCount: 2,
          chunks: [
            { id: 'c-001', effectiveFrom: 1736899200, effectiveTo: null, version: 'v3.2' },
            { id: 'c-002', effectiveFrom: 1736899200, effectiveTo: null, version: 'v3.2' },
          ],
        },
      ],
    },
    { tier: '历史', docCount: 0, chunkCount: 0, docs: [] },
    { tier: '个人', docCount: 0, chunkCount: 0, docs: [] },
    { tier: '草稿', docCount: 0, chunkCount: 0, docs: [] },
  ],
};

const ONTOLOGY_GRAPH: OntologyGraph = {
  nodes: [
    {
      id: 'http://skillforge.local/ontology#note_032',
      kind: 'note',
      label: '032',
      tier: 'sql-guides',
    },
    { id: 'http://skillforge.local/ontology#tbl_pay_way', kind: 'table', label: 'pay_way' },
  ],
  edges: [
    {
      from: 'http://skillforge.local/ontology#note_032',
      to: 'http://skillforge.local/ontology#tbl_pay_way',
      type: 'references_table',
    },
  ],
};

/** Route-style fetch stub: ontology URLs → ontology body, else doc body. */
function mockFetchRouter(docBody: unknown, ontologyBody: unknown = ONTOLOGY_GRAPH): ReturnType<typeof vi.fn> {
  const fn = vi.fn().mockImplementation(async (url: string) => ({
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () =>
      url.includes('/api/admin/ontology/graph') ? ontologyBody : docBody,
  } as unknown as Response));
  vi.stubGlobal('fetch', fn);
  return fn;
}

function mockFetchError(message: string): ReturnType<typeof vi.fn> {
  const fn = vi.fn().mockImplementation(async () => ({
    ok: false,
    status: 500,
    statusText: 'Internal Server Error',
    json: async () => ({ error: message }),
  } as unknown as Response));
  vi.stubGlobal('fetch', fn);
  return fn;
}

/** Render, then flip to the 文档 mode (corpus tree + heroes live there). */
function renderAndSwitchToDocument(props: Parameters<typeof KnowledgeGraphBrowser>[0] = {}) {
  mockFetchRouter(POPULATED_GRAPH);
  render(<KnowledgeGraphBrowser {...props} />);
  fireEvent.click(screen.getByTestId('graph-mode-document'));
}

describe('KnowledgeGraphBrowser', () => {
  // ── Quick 260929-g7x: 本体 default mode ──────────────────────────────
  it('defaults to the 本体 view and fetches the ontology projection', async () => {
    const fetchMock = mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.anything(),
    );
    // The corpus tree is NOT rendered in ontology mode.
    expect(screen.queryByTestId('graph-canvas')).toBeNull();
    // Mode chips: 本体 active, 文档 available.
    expect(screen.getByTestId('graph-mode-ontology')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('graph-mode-document')).toHaveAttribute('aria-pressed', 'false');
  });

  it('the ontology view renders even when the document corpus is empty', async () => {
    mockFetchRouter(EMPTY_GRAPH);
    render(<KnowledgeGraphBrowser />);
    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(screen.queryByTestId('knowledge-graph-empty')).toBeNull();
  });

  it('focus flows into the ontology fetch (?notes= derived from citations)', async () => {
    const fetchMock = mockFetchRouter(POPULATED_GRAPH);
    render(
      <KnowledgeGraphBrowser
        focus={{ chunkIds: new Set(), sources: ['D:/corpus/032_保单.md'] }}
      />,
    );
    await screen.findByTestId('ontology-graph-view');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032',
      expect.anything(),
    );
  });

  it('mode chips switch between 本体 and 文档 both ways', async () => {
    mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('ontology-graph-view');

    fireEvent.click(screen.getByTestId('graph-mode-document'));
    expect(await screen.findByTestId('graph-canvas')).toBeInTheDocument();
    expect(screen.getByTestId('graph-mode-document')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByTestId('ontology-graph-view')).toBeNull();

    fireEvent.click(screen.getByTestId('graph-mode-ontology'));
    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(screen.queryByTestId('graph-canvas')).toBeNull();
  });

  it('the fullscreen overlay renders the ACTIVE mode (ontology) and Esc exits', async () => {
    mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('ontology-graph-view');

    fireEvent.click(screen.getByTestId('graph-fullscreen-btn'));
    const overlay = await screen.findByTestId('graph-fullscreen-overlay');
    expect(overlay.parentElement).toBe(document.body);
    expect(within(overlay).getByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(within(overlay).getByTestId('graph-mode-ontology')).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByTestId('graph-fullscreen-overlay')).toBeNull();
    expect(screen.getByTestId('knowledge-graph')).toBeInTheDocument();
  });

  // ── 文档 mode: corpus tree (backward-compatible second mode) ────────
  it('renders the empty-state hero in 文档 mode when the corpus has 0 docs', async () => {
    mockFetchRouter(EMPTY_GRAPH);
    render(<KnowledgeGraphBrowser />);
    fireEvent.click(screen.getByTestId('graph-mode-document'));
    const empty = await screen.findByTestId('knowledge-graph-empty');
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent('知识库为空');
  });

  it('renders the loading state in 文档 mode when loading=true', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => new Promise(() => {})),
    );
    render(<KnowledgeGraphBrowser loading />);
    fireEvent.click(screen.getByTestId('graph-mode-document'));
    expect(screen.getByTestId('knowledge-graph-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('knowledge-graph-empty')).toBeNull();
  });

  it('renders the error state with a retry button in 文档 mode when the BE returns 500', async () => {
    mockFetchError('upstream mongo timeout');
    render(<KnowledgeGraphBrowser />);
    fireEvent.click(screen.getByTestId('graph-mode-document'));
    const errorNode = await screen.findByTestId('knowledge-graph-error');
    expect(errorNode).toBeInTheDocument();
    expect(errorNode).toHaveTextContent('upstream mongo timeout');
    expect(screen.getByTestId('graph-refresh')).toBeInTheDocument();
  });

  it('renders the relation graph (tier nodes + doc nodes) in 文档 mode', async () => {
    renderAndSwitchToDocument();
    const graphRoot = await screen.findByTestId('knowledge-graph');
    expect(graphRoot).toBeInTheDocument();
    expect(screen.getByTestId('graph-canvas')).toBeInTheDocument();
    // Tier node renders the tier name + doc count.
    const tierNode = screen.getByTestId('graph-tier-node');
    expect(tierNode).toHaveAttribute('data-tier', '权威');
    expect(tierNode).toHaveTextContent('1 文档');
    // Doc node renders the filename (basename, .md stripped) + chunk
    // count badge, carrying tier/file for the doc-view dispatch.
    const docNode = screen.getByTestId('graph-doc-node');
    expect(docNode).toHaveAttribute('data-tier', '权威');
    expect(docNode).toHaveAttribute('data-file', '员工差旅管理办法.md');
    expect(docNode).toHaveTextContent('员工差旅管理办法');
    expect(docNode).toHaveTextContent('2 chunk');
  });

  it('the filter input narrows visible doc nodes by filename (case-insensitive)', async () => {
    renderAndSwitchToDocument();
    await screen.findByTestId('graph-canvas');

    const input = screen.getByTestId('graph-filter-input').querySelector('input') as HTMLInputElement;

    // Filter by the doc filename substring — the doc node remains.
    fireEvent.change(input, { target: { value: '差旅' } });
    await waitFor(() => {
      expect(screen.getByTestId('graph-doc-node')).toBeInTheDocument();
    });

    // Filter by something unrelated — the doc node disappears.
    fireEvent.change(input, { target: { value: 'unrelated-keyword' } });
    await waitFor(() => {
      expect(screen.queryByTestId('graph-doc-node')).toBeNull();
    });
  });

  it('clicking a doc node dispatches setCenterView({kind:"doc-view", tier, file})', async () => {
    renderAndSwitchToDocument();
    await screen.findByTestId('graph-canvas');

    const docNode = screen.getByTestId('graph-doc-node');
    fireEvent.click(docNode);

    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      expect(cv.kind).toBe('doc-view');
      if (cv.kind === 'doc-view') {
        expect(cv.tier).toBe('权威');
        expect(cv.file).toBe('员工差旅管理办法.md');
      }
    });
  });

  // Quick 260929-d5w2 — 全屏 opens a fixed inset-0 overlay (portal to
  // document.body) rendering the SAME document-mode body; close button exits.
  it('fullscreen in 文档 mode shows the same tree and the close button exits', async () => {
    renderAndSwitchToDocument();
    await screen.findByTestId('graph-canvas');

    fireEvent.click(screen.getByTestId('graph-fullscreen-btn'));

    const overlay = screen.getByTestId('graph-fullscreen-overlay');
    expect(overlay).toBeInTheDocument();
    // Portaled to document.body so no panel ancestor clips the fixed box.
    expect(overlay.parentElement).toBe(document.body);
    // Same graph content renders inside the overlay (canvas + filter).
    expect(within(overlay).getByTestId('graph-canvas')).toBeInTheDocument();
    expect(within(overlay).getByTestId('graph-filter-input')).toBeInTheDocument();
    // The narrow body is replaced — no duplicate canvas outside the overlay.
    expect(document.querySelectorAll('[data-testid="graph-canvas"]').length).toBe(1);

    fireEvent.click(within(overlay).getByTestId('graph-fullscreen-close'));

    expect(screen.queryByTestId('graph-fullscreen-overlay')).toBeNull();
    expect(screen.getByTestId('knowledge-graph')).toBeInTheDocument();
  });
});
