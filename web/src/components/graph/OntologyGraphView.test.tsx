// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-g7x — OntologyGraphView pins.
//
// Cytoscape is mocked (the governance ProvenanceGraphView has no test
// precedent; jsdom has no canvas, so a real mount would explode). The
// mock records the init config so the assertions run against the REAL
// element/style payloads the component builds:
//   ① 全库 fetch (no notes param) + cytoscape init with all node kinds
//   ② focus → notes=note_032 wire param (sync source path + async
//     chunk_id resolution through fetchChunkContent; all-fail → 全库)
//   ③ node tap → detail panel (kind/label/tier)
//   ④ 仅当前问题 ↔ 全库 toggle refetches
//   ⑤ 403 → readable admin-token error, never a blank canvas
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import OntologyGraphView from './OntologyGraphView';
import type { GraphFocus } from '../../lib/graph-focus';
import type { OntologyGraph } from './ontologyApi';

const { cyInstance, cytoscapeMock } = vi.hoisted(() => ({
  cyInstance: {
    on: vi.fn(),
    destroy: vi.fn(),
    elements: vi.fn(() => ({ removeClass: vi.fn() })),
    $: vi.fn(() => ({ addClass: vi.fn() })),
  },
  cytoscapeMock: vi.fn(() => cyInstance),
}));

vi.mock('cytoscape', () => ({ default: cytoscapeMock }));

const ONTOLOGY_GRAPH: OntologyGraph = {
  nodes: [
    { id: 'http://skillforge.local/ontology#note_032', kind: 'note', label: '032', tier: 'sql-guides' },
    { id: 'http://skillforge.local/ontology#tbl_pay_way', kind: 'table', label: 'pay_way' },
    {
      id: 'http://skillforge.local/ontology#ent_car',
      kind: 'insurance_product',
      label: '车险',
      tier: 'sql-guides',
    },
    {
      id: 'http://skillforge.local/ontology#ent_hubei',
      kind: 'organization',
      label: '湖北分公司',
      tier: 'sql-guides',
    },
    {
      id: 'http://skillforge.local/ontology#ent_hq',
      kind: 'organization',
      label: '总公司',
    },
  ],
  edges: [
    {
      from: 'http://skillforge.local/ontology#note_032',
      to: 'http://skillforge.local/ontology#tbl_pay_way',
      type: 'references_table',
    },
    {
      from: 'http://skillforge.local/ontology#note_032',
      to: 'http://skillforge.local/ontology#ent_car',
      type: 'involves_product',
    },
    {
      from: 'http://skillforge.local/ontology#note_032',
      to: 'http://skillforge.local/ontology#ent_hubei',
      type: 'involves_org',
    },
    {
      from: 'http://skillforge.local/ontology#ent_hubei',
      to: 'http://skillforge.local/ontology#ent_hq',
      type: 'parent_org',
    },
  ],
};

function mockFetchRouter(handlers: Array<{ match: string; status?: number; body: unknown }>) {
  const fn = vi.fn().mockImplementation(async (url: string) => {
    const handler = handlers.find((h) => url.includes(h.match));
    if (!handler) throw new Error('unexpected fetch: ' + url);
    const status = handler.status ?? 200;
    return {
      ok: status >= 200 && status < 300,
      status,
      json: async () => handler.body,
    } as unknown as Response;
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

/** BE wire shape of GET /api/knowledge/chunk/:id (fetchChunkContent). */
function chunkWireBody(source: string) {
  return {
    id: 'x',
    tier: 'sql-guides',
    source,
    content: 'SELECT 1',
    effectiveFrom: 0,
    effectiveTo: 0,
    version: 'v1',
  };
}

/** The cytoscape 'tap node' handler the component registered. */
function registeredTapNodeHandler(): (e: unknown) => void {
  const call = cyInstance.on.mock.calls.find(
    (c) => c[0] === 'tap' && c[1] === 'node',
  );
  if (!call) throw new Error('component never registered cy.on("tap", "node")');
  return call[2] as (e: unknown) => void;
}

beforeEach(() => {
  localStorage.setItem('skillforge.graph.focusMode', 'focus');
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.removeItem('skillforge.graph.focusMode');
  cytoscapeMock.mockClear();
  cyInstance.on.mockClear();
  cyInstance.destroy.mockClear();
});

describe('OntologyGraphView', () => {
  it('fetches 全库 (no notes param) on mount and mounts cytoscape with all node kinds', async () => {
    const fetchMock = mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView />);
    const view = await screen.findByTestId('ontology-graph-view');
    expect(view).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.objectContaining({ headers: expect.objectContaining({ 'X-Admin-Token': '' }) }),
    );

    expect(cytoscapeMock).toHaveBeenCalledTimes(1);
    const initArg = (cytoscapeMock.mock.calls as unknown as Array<[unknown]>)[0]?.[0] ?? {};
    const config = initArg as {
      elements: Array<{ data: Record<string, unknown> }>;
      layout: { name: string };
    };
    const kinds = config.elements
      .filter((el) => 'kind' in el.data)
      .map((el) => el.data.kind);
    expect(kinds).toEqual(
      expect.arrayContaining(['note', 'table', 'insurance_product', 'organization']),
    );
    const product = config.elements.find((el) => el.data.kind === 'insurance_product');
    expect(product?.data.label).toBe('车险');
    const parentEdge = config.elements.find(
      (el) => el.data.type === 'parent_org',
    );
    expect(parentEdge).toBeDefined();
    expect(config.layout.name).toBe('cose');

    // Legend counts the four kinds.
    expect(screen.getByTestId('ontology-legend')).toHaveTextContent('笔记 1');
    expect(screen.getByTestId('ontology-legend')).toHaveTextContent('机构 2');
  });

  it('focus narrows the fetch via the tolerated source field (sync path)', async () => {
    const focus: GraphFocus = {
      chunkIds: new Set(),
      sources: ['D:/corpus/032_保单号查支付方式与开户行.md'],
    };
    const fetchMock = mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView focus={focus} />);
    await screen.findByTestId('ontology-graph-view');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032',
      expect.anything(),
    );
    expect(screen.getByTestId('ontology-focus-toggle')).toHaveTextContent('仅当前问题');
  });

  it('resolves chunk_ids through fetchChunkContent to notes (the wire path)', async () => {
    // The wire verdict carries ONLY chunk_id/span/tier — the note id comes
    // from the chunk's source path, resolved per-chunk before the fetch.
    const focus: GraphFocus = {
      chunkIds: new Set(['1032', '1007']),
      sources: [],
    };
    const fetchMock = mockFetchRouter([
      { match: '/api/knowledge/chunk/1032', body: chunkWireBody('D:/corpus/032_保单.md') },
      { match: '/api/knowledge/chunk/1007', body: chunkWireBody('/kb/007_机构.md') },
      { match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH },
    ]);
    render(<OntologyGraphView focus={focus} />);
    // Resolution loading is visible before the projection fetch lands.
    expect(screen.getByTestId('ontology-graph-loading')).toBeInTheDocument();
    await screen.findByTestId('ontology-graph-view');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/knowledge/chunk/1032',
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032%2Cnote_007',
      expect.anything(),
    );
    expect(screen.getByTestId('ontology-focus-toggle')).toHaveTextContent('仅当前问题');
  });

  it('every chunk failing resolution degrades to the full projection', async () => {
    const focus: GraphFocus = {
      chunkIds: new Set(['404', 'boom']),
      sources: [],
    };
    const fetchMock = mockFetchRouter([
      { match: '/api/knowledge/chunk/404', status: 404, body: { error: 'nope' } },
      { match: '/api/knowledge/chunk/boom', status: 500, body: { error: 'kaboom' } },
      { match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH },
    ]);
    render(<OntologyGraphView focus={focus} />);
    await screen.findByTestId('ontology-graph-view');
    // No notes resolved → 全库 fetch (no notes param), no focus toggle.
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.anything(),
    );
    expect(screen.queryByTestId('ontology-focus-toggle')).toBeNull();
    expect(screen.getByTestId('ontology-legend')).toBeInTheDocument();
  });

  it('tapping a node opens the detail panel with kind/label/tier', async () => {
    mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView />);
    await screen.findByTestId('ontology-graph-view');
    expect(screen.getByTestId('ontology-node-detail')).toHaveTextContent(
      '点击节点查看本体明细',
    );

    const handler = registeredTapNodeHandler();
    await act(async () => {
      handler({ target: { id: () => 'http://skillforge.local/ontology#ent_car' } });
    });

    const detail = screen.getByTestId('ontology-node-detail');
    expect(detail).toHaveTextContent('险种');
    expect(detail).toHaveTextContent('车险');
    expect(detail).toHaveTextContent('sql-guides');
  });

  it('the 仅当前问题/全库 toggle refetches the full projection', async () => {
    const focus: GraphFocus = {
      chunkIds: new Set(),
      sources: ['D:/corpus/032_保单.md'],
    };
    const fetchMock = mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView focus={focus} />);
    await screen.findByTestId('ontology-graph-view');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032',
      expect.anything(),
    );

    fireEvent.click(screen.getByTestId('ontology-focus-toggle'));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/admin/ontology/graph',
        expect.anything(),
      );
    });
    // Preference persists under the shared GraphCanvas key.
    expect(localStorage.getItem('skillforge.graph.focusMode')).toBe('all');
    expect(screen.getByTestId('ontology-focus-toggle')).toHaveTextContent('全库');
  });

  it('renders the honest empty state when the projection has no nodes', async () => {
    mockFetchRouter([
      { match: '/api/admin/ontology/graph', body: { nodes: [], edges: [] } },
    ]);
    render(<OntologyGraphView />);
    expect(await screen.findByTestId('ontology-graph-empty')).toBeInTheDocument();
  });

  it('403 surfaces the admin-token hint instead of a blank canvas', async () => {
    mockFetchRouter([
      {
        match: '/api/admin/ontology/graph',
        status: 403,
        body: { error: 'admin token required' },
      },
    ]);
    render(<OntologyGraphView />);
    const err = await screen.findByTestId('ontology-graph-error');
    expect(err).toBeInTheDocument();
    expect(err).toHaveTextContent('需要管理员令牌');
  });
});
