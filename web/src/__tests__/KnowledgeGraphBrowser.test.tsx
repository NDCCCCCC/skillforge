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
// Quick 260929-g7x — the browser is a two-mode host: 本体
// (OntologyGraphView) and 文档 (the retrieval-corpus tree).
//
// 默认模式已改为「文档」：本体视图走 /api/admin/ontology/graph，需要
// 管理员令牌；把它放在默认落地页会让普通用户一进来就撞上"需要管理员
// 令牌"。本体降级为次级 tab，且在没有令牌时只给一句温和说明（不是
// 错误 + 令牌表单）。
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';

// The browser mounts OntologyGraphView (本体 mode) — jsdom has no
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
  localStorage.removeItem('sf_admin_token');
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
  // ── 默认模式：文档（语料树）──────────────────────────────────────
  it('defaults to the 文档 view — the admin-gated ontology tab is NOT the landing screen', async () => {
    const fetchMock = mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    expect(await screen.findByTestId('graph-canvas')).toBeInTheDocument();
    // Mode chips: 文档 active, 本体 available (secondary).
    expect(screen.getByTestId('graph-mode-document')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('graph-mode-ontology')).toHaveAttribute('aria-pressed', 'false');
    // The default landing screen never asks for the admin-gated projection.
    expect(fetchMock).not.toHaveBeenCalledWith('/api/admin/ontology/graph', expect.anything());
  });

  it('keeps the mode chips switchable on the empty-corpus hero', async () => {
    mockFetchRouter(EMPTY_GRAPH);
    render(<KnowledgeGraphBrowser />);
    expect(await screen.findByTestId('knowledge-graph-empty')).toBeInTheDocument();
    // The chips survive the early return — otherwise the user is stuck
    // on the empty hero with no way to reach the ontology view.
    expect(screen.getByTestId('graph-mode-ontology')).toBeInTheDocument();
    expect(screen.getByTestId('graph-mode-document')).toBeInTheDocument();
  });

  it('without an admin token the 本体 tab shows a gentle note (no error page, no token form)', async () => {
    localStorage.removeItem('sf_admin_token');
    mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('graph-canvas');

    fireEvent.click(screen.getByTestId('graph-mode-ontology'));

    const note = await screen.findByTestId('ontology-needs-admin');
    expect(note).toHaveTextContent('本体视图需要管理员权限');
    // NOT an error + admin-token gate — that made the panel feel broken.
    expect(screen.queryByTestId('ontology-token-gate')).toBeNull();
    expect(screen.queryByTestId('ontology-graph-error')).toBeNull();
    // …and the user can still switch back to the document view.
    fireEvent.click(screen.getByTestId('graph-mode-document'));
    expect(await screen.findByTestId('graph-canvas')).toBeInTheDocument();
  });

  // ── 本体 mode（次级 tab，需要管理员令牌）──────────────────────────
  it('with an admin token the 本体 tab renders the ontology projection', async () => {
    localStorage.setItem('sf_admin_token', 'tok-abc');
    const fetchMock = mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('graph-canvas');

    fireEvent.click(screen.getByTestId('graph-mode-ontology'));

    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.anything(),
    );
  });

  it('focus flows into the ontology fetch (?notes= derived from citations)', async () => {
    localStorage.setItem('sf_admin_token', 'tok-abc');
    const fetchMock = mockFetchRouter(POPULATED_GRAPH);
    render(
      <KnowledgeGraphBrowser
        focus={{ chunkIds: new Set(), sources: ['D:/corpus/032_保单.md'] }}
      />,
    );
    await screen.findByTestId('graph-canvas');
    fireEvent.click(screen.getByTestId('graph-mode-ontology'));
    await screen.findByTestId('ontology-graph-view');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032',
      expect.anything(),
    );
  });

  it('mode chips switch between 文档 and 本体 both ways', async () => {
    localStorage.setItem('sf_admin_token', 'tok-abc');
    mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('graph-canvas');

    fireEvent.click(screen.getByTestId('graph-mode-ontology'));
    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(screen.getByTestId('graph-mode-ontology')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByTestId('graph-canvas')).toBeNull();

    fireEvent.click(screen.getByTestId('graph-mode-document'));
    expect(await screen.findByTestId('graph-canvas')).toBeInTheDocument();
    expect(screen.getByTestId('graph-mode-document')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByTestId('ontology-graph-view')).toBeNull();
  });

  it('the fullscreen overlay renders the ACTIVE mode (文档) and Esc exits', async () => {
    mockFetchRouter(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('graph-canvas');

    fireEvent.click(screen.getByTestId('graph-fullscreen-btn'));
    const overlay = await screen.findByTestId('graph-fullscreen-overlay');
    expect(overlay.parentElement).toBe(document.body);
    expect(within(overlay).getByTestId('graph-canvas')).toBeInTheDocument();
    expect(within(overlay).getByTestId('graph-mode-document')).toBeInTheDocument();

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
