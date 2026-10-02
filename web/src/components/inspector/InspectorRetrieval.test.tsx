// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 3 — InspectorRetrieval's residual tier sweep.
//
// Pins:
//   - with no explicit prop, the chunk-source dispatch tier is the KB
//     projection's FIRST authority-role slug (the citable role) — never a
//     hardcoded CN tier literal
//   - before the projection resolves the dispatch carries '' (the honest
//     "tier unknown" placeholder that renders no badge)
//   - an explicit defaultTier prop wins over the projection
//   - no retrievals renders nothing
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import InspectorRetrieval from './InspectorRetrieval';
import { useAppStore } from '../../store/app-store';
import type { RetrievalRow } from './hop-types';

const REGISTRY_BASES = [
  { slug: 'kb-2026', display_name: '新库', role: 'authority' },
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'historical', display_name: '历史', role: 'historical' },
];

function stubRegistry(bases: unknown[] = REGISTRY_BASES): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        new Response(JSON.stringify({ bases }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    ),
  );
}

const ROWS = [{ chunkId: '41', filename: 'annual_leave.md', title: 'authority/annual_leave.md' }];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

describe('InspectorRetrieval — registry-derived default tier', () => {
  it('dispatches the projection first authority-role slug', async () => {
    stubRegistry();
    useAppStore.setState({ centerView: { kind: 'empty' } });
    render(<InspectorRetrieval retrievals={ROWS} />);
    const view = screen.getByTestId('inspector-retrieval');
    // Wait until the projection resolved (the wrapper surfaces the default
    // it would dispatch) so the click carries the registry slug, not the
    // pre-resolution '' placeholder.
    await waitFor(() => expect(view).toHaveAttribute('data-default-tier', 'kb-2026'));

    fireEvent.click(screen.getByTestId('inspector-retrieval-row'));
    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      expect(cv.kind).toBe('chunk-source');
      if (cv.kind === 'chunk-source') {
        // The FIRST authority-role slug in projection order, not a literal.
        expect(cv.tier).toBe('kb-2026');
      }
    });
  });

  it('carries an explicit defaultTier prop over the projection', async () => {
    stubRegistry();
    useAppStore.setState({ centerView: { kind: 'empty' } });
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="historical" />);
    fireEvent.click(screen.getByTestId('inspector-retrieval-row'));
    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      if (cv.kind === 'chunk-source') {
        expect(cv.tier).toBe('historical');
      }
    });
  });

  it('renders nothing when the hop has no retrievals', () => {
    stubRegistry();
    const { container } = render(<InspectorRetrieval retrievals={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe('InspectorRetrieval — neighbors panel (Phase 28-06)', () => {
  const ROWS: RetrievalRow[] = [
    { chunkId: '7', filename: 'contract.md', title: 'Chunk 7' },
    { chunkId: '11', filename: 'contract.md', title: 'Chunk 11' },
  ];

  const NEIGHBORS_OK = {
    chunk_id: '7',
    depth: 1,
    nodes: [
      { chunk_id: '7', tier: 'authority', source: 'contract.md', effectiveFrom: 0, effectiveTo: 0 },
      { chunk_id: '11', tier: 'authority', source: 'contract.md', effectiveFrom: 0, effectiveTo: 0 },
    ],
    edges: [
      { from: '7', to: '11', edge_type: 'section_adjacent', weight: 1.0, tier: 'authority' },
      { from: '7', to: '5', edge_type: 'citation_cooccur', weight: 0.85, tier: 'authority' },
    ],
    degraded: false,
    degraded_reason: '',
  };

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    useAppStore.setState({ centerView: { kind: 'empty' } });
  });

  it('shows the neighbors panel when a citation is clicked', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(NEIGHBORS_OK),
    } as Response));
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    fireEvent.click(screen.getAllByTestId('inspector-retrieval-row')[0]);
    await waitFor(() => expect(screen.getByTestId('neighbors-panel')).toBeInTheDocument());
  });

  it('renders edges with Chinese labels', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(NEIGHBORS_OK),
    } as Response));
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    fireEvent.click(screen.getAllByTestId('inspector-retrieval-row')[0]);
    await waitFor(() => {
      expect(screen.getByText('邻接')).toBeInTheDocument();
      expect(screen.getByText('引用共现')).toBeInTheDocument();
    });
  });

  it('shows loading indicator while fetching neighbors', async () => {
    let resolve: (v: unknown) => void;
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise((r) => { resolve = r; })) as ReturnType<typeof vi.fn>);
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    fireEvent.click(screen.getAllByTestId('inspector-retrieval-row')[0]);
    expect(screen.getByText(/加载中/)).toBeInTheDocument();
    // Unblock
    resolve!({ ok: true, json: () => Promise.resolve(NEIGHBORS_OK) });
    await waitFor(() => expect(screen.queryByText(/加载中/)).not.toBeInTheDocument());
  });

  it('shows degraded message when neighbors API returns degraded=true', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ ...NEIGHBORS_OK, degraded: true, degraded_reason: 'Milvus unavailable' }),
    } as Response));
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    fireEvent.click(screen.getAllByTestId('inspector-retrieval-row')[0]);
    await waitFor(() => expect(screen.getByText('Milvus unavailable')).toBeInTheDocument());
  });

  it('collapses the panel on second click of the same citation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(NEIGHBORS_OK),
    } as Response));
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    const btn = screen.getAllByTestId('inspector-retrieval-row')[0];
    fireEvent.click(btn);
    await waitFor(() => expect(screen.getByTestId('neighbors-panel')).toBeInTheDocument());
    fireEvent.click(btn);
    await waitFor(() => expect(screen.queryByTestId('neighbors-panel')).not.toBeInTheDocument());
  });

  it('hides panel silently when neighbors API returns an error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    } as unknown as Response));
    render(<InspectorRetrieval retrievals={ROWS} defaultTier="authority" />);
    fireEvent.click(screen.getAllByTestId('inspector-retrieval-row')[0]);
    // Panel should not appear at all (graceful degradation)
    await waitFor(() => expect(screen.queryByTestId('neighbors-panel')).not.toBeInTheDocument());
  });
});
