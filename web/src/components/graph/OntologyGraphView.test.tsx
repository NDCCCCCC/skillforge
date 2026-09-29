// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-k3m — OntologyGraphView pins (react-force-graph-3d era).
//
// react-force-graph-3d is mocked (jsdom has no WebGL; three.js would
// explode on mount). The stub renders a probe div carrying the node/link
// counts and records every props object it receives, so assertions run
// against the REAL graphData/accessor payloads the component builds:
//   ① 全库 fetch (no notes param) + graphData with all node kinds,
//     note-hub sizing, and the directional-particle wiring
//   ② focus → notes=note_032 wire param (sync source path)
//   ③ chunk_ids → fetchChunkContent async resolution → notes param
//     (projection fetch held during resolution — loading state visible)
//   ④ all chunks failing resolution degrades to the full projection
//   ⑤ node click → detail panel (kind/label/tier); background click clears
//   ⑥ 仅当前问题 ↔ 全库 toggle refetches + persists the shared pref key
//   ⑦ honest empty state
//   ⑧ 403 → readable admin-token error, never a blank canvas
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import OntologyGraphView from './OntologyGraphView';
import type { GraphFocus } from '../../lib/graph-focus';
import type { OntologyGraph } from './ontologyApi';

const { fgProps } = vi.hoisted(() => ({
  fgProps: [] as Array<Record<string, unknown>>,
}));

vi.mock('react-force-graph-3d', () => ({
  default: function ForceGraph3DStub(props: Record<string, unknown>) {
    fgProps.push(props);
    const gd = props.graphData as
      | { nodes: unknown[]; links: unknown[] }
      | undefined;
    return (
      <div
        data-testid="force-graph-3d"
        data-graph={JSON.stringify({
          nodes: gd?.nodes.length ?? 0,
          links: gd?.links.length ?? 0,
        })}
      />
    );
  },
}));

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

/** Props the stub received on its most recent render. */
function lastFgProps(): Record<string, unknown> {
  const props = fgProps.at(-1);
  if (!props) throw new Error('ForceGraph3D stub never rendered');
  return props;
}

function fgGraphData(): { nodes: Array<Record<string, unknown>>; links: Array<Record<string, unknown>> } {
  return lastFgProps().graphData as {
    nodes: Array<Record<string, unknown>>;
    links: Array<Record<string, unknown>>;
  };
}

beforeEach(() => {
  localStorage.setItem('skillforge.graph.focusMode', 'focus');
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  localStorage.removeItem('skillforge.graph.focusMode');
  fgProps.length = 0;
});

describe('OntologyGraphView', () => {
  it('fetches 全库 (no notes param) on mount and feeds force-graph-3d all node kinds', async () => {
    const fetchMock = mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView />);
    expect(await screen.findByTestId('ontology-graph-view')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.objectContaining({ headers: expect.objectContaining({ 'X-Admin-Token': '' }) }),
    );

    // The 3D engine stub mounts with the full projection.
    const stub = await screen.findByTestId('force-graph-3d');
    expect(stub).toHaveAttribute('data-graph', JSON.stringify({ nodes: 5, links: 4 }));

    const { nodes, links } = fgGraphData();
    expect(nodes.map((n) => n.kind)).toEqual(
      expect.arrayContaining(['note', 'table', 'insurance_product', 'organization']),
    );
    const product = nodes.find((n) => n.kind === 'insurance_product');
    expect(product?.label).toBe('车险');
    // Notes are the hubs (bigger), everything else smaller.
    expect(nodes.find((n) => n.kind === 'note')?.val).toBe(6);
    expect(nodes.find((n) => n.kind === 'table')?.val).toBe(3);
    expect(product?.name).toBe('车险');
    // Direction flows: parent_org edge survives the mapping.
    expect(links.find((l) => l.type === 'parent_org')).toMatchObject({
      source: 'http://skillforge.local/ontology#ent_hubei',
      target: 'http://skillforge.local/ontology#ent_hq',
    });
    // Flowing particles are the dynamic-signature wiring.
    expect(lastFgProps().linkDirectionalParticles).toBe(3);
    expect(lastFgProps().linkDirectionalParticleSpeed).toBe(0.006);

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
    await screen.findByTestId('force-graph-3d');
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
    await screen.findByTestId('force-graph-3d');
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
    await screen.findByTestId('force-graph-3d');
    // No notes resolved → 全库 fetch (no notes param), no focus toggle.
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.anything(),
    );
    expect(screen.queryByTestId('ontology-focus-toggle')).toBeNull();
    expect(screen.getByTestId('ontology-legend')).toBeInTheDocument();
  });

  it('clicking a node opens the detail panel; background click clears it', async () => {
    mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView />);
    await screen.findByTestId('force-graph-3d');
    expect(screen.getByTestId('ontology-node-detail')).toHaveTextContent(
      '点击节点查看本体明细',
    );

    // The stub receives the mapped node payload — click the product node.
    const node = fgGraphData().nodes.find((n) => n.kind === 'insurance_product');
    expect(node).toBeDefined();
    await act(async () => {
      (lastFgProps().onNodeClick as (n: unknown) => void)(node);
    });

    const detail = screen.getByTestId('ontology-node-detail');
    expect(detail).toHaveTextContent('险种');
    expect(detail).toHaveTextContent('车险');
    expect(detail).toHaveTextContent('sql-guides');

    // Background click deselects (detail back to the hint).
    await act(async () => {
      (lastFgProps().onBackgroundClick as () => void)();
    });
    expect(screen.getByTestId('ontology-node-detail')).toHaveTextContent(
      '点击节点查看本体明细',
    );
  });

  it('the 仅当前问题/全库 toggle refetches the full projection', async () => {
    const focus: GraphFocus = {
      chunkIds: new Set(),
      sources: ['D:/corpus/032_保单.md'],
    };
    const fetchMock = mockFetchRouter([{ match: '/api/admin/ontology/graph', body: ONTOLOGY_GRAPH }]);
    render(<OntologyGraphView focus={focus} />);
    await screen.findByTestId('force-graph-3d');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph?notes=note_032',
      expect.anything(),
    );

    fireEvent.click(screen.getByTestId('ontology-focus-toggle'));

    // The toggle flips to 全库 and the full projection refetch lands.
    await screen.findByTestId('force-graph-3d');
    // Preference persists under the shared GraphCanvas key.
    expect(localStorage.getItem('skillforge.graph.focusMode')).toBe('all');
    expect(screen.getByTestId('ontology-focus-toggle')).toHaveTextContent('全库');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/ontology/graph',
      expect.anything(),
    );
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
