// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 Task 1 — KnowledgeGraphBrowser tests.
//
// Pins the SC-5 (KNOW-06) acceptance assertions for the real Accordion
// implementation that replaced the 05-03 placeholder:
//
//   - Empty corpus (all 4 tiers present with 0 docs) renders the
//     知识库为空 hero.
//   - Loading prop override renders 正在加载知识层级... and suppresses
//     the empty hero.
//   - Error from the BE renders 无法加载知识图谱 + 重试 button.
//   - Populated corpus renders the 3-level tree (tier -> doc -> chunk)
//     and the filter narrows visible docs by filename.
//   - Clicking a chunk dispatches setCenterView({kind:"chunk-source", ...}).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import KnowledgeGraphBrowser from '../components/graph/KnowledgeGraphBrowser';
import { useAppStore } from '../store/app-store';
import type { KnowledgeGraph } from '../types/knowledge';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
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

function mockFetchOk(body: unknown, status = 200): ReturnType<typeof vi.fn> {
  const fn = vi.fn().mockImplementation(async () => ({
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    json: async () => body,
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

describe('KnowledgeGraphBrowser', () => {
  it('renders the empty-state hero when the BE returns an empty corpus (all 4 tiers, 0 docs)', async () => {
    mockFetchOk(EMPTY_GRAPH);
    render(<KnowledgeGraphBrowser />);
    const empty = await screen.findByTestId('knowledge-graph-empty');
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent('知识库为空');
  });

  it('renders the loading state when loading=true and suppresses the empty hero', () => {
    // Stub fetch to a never-resolving promise so the hook's useEffect
    // doesn't throw on `fetch(...)` being undefined in jsdom.
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(() => new Promise(() => {})),
    );
    render(<KnowledgeGraphBrowser loading />);
    expect(screen.getByTestId('knowledge-graph-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('knowledge-graph-empty')).toBeNull();
  });

  it('renders the error state with a retry button when the BE returns 500', async () => {
    mockFetchError('upstream mongo timeout');
    render(<KnowledgeGraphBrowser />);
    const errorNode = await screen.findByTestId('knowledge-graph-error');
    expect(errorNode).toBeInTheDocument();
    expect(errorNode).toHaveTextContent('upstream mongo timeout');
    expect(screen.getByTestId('graph-refresh')).toBeInTheDocument();
  });

  it('renders the 3-level tree (tier / doc / chunk) for a populated corpus', async () => {
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    const tree = await screen.findByTestId('knowledge-graph');
    expect(tree).toBeInTheDocument();
    expect(tree).toHaveTextContent('权威');
    expect(tree).toHaveTextContent('员工差旅管理办法.md');
    const chunks = screen.getAllByTestId('graph-chunk-node');
    expect(chunks).toHaveLength(2);
    expect(chunks[0]).toHaveAttribute('data-chunk-id', 'c-001');
    expect(chunks[1]).toHaveAttribute('data-chunk-id', 'c-002');
  });

  it('the filter input narrows visible docs by filename (case-insensitive)', async () => {
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('knowledge-graph');

    const input = screen.getByTestId('graph-filter-input').querySelector('input') as HTMLInputElement;

    // Filter by the doc filename substring — the populated doc remains
    // visible. We assert via the data-doc-id because the filename text
    // is concatenated with " · N 片段" inside a single <summary>.
    fireEvent.change(input, { target: { value: '差旅' } });
    await waitFor(() => {
      expect(
        screen.getByTestId('graph-doc'),
      ).toBeInTheDocument();
    });

    // Filter by something unrelated — the doc row disappears.
    fireEvent.change(input, { target: { value: 'unrelated-keyword' } });
    await waitFor(() => {
      expect(screen.queryByTestId('graph-doc')).toBeNull();
    });
  });

  it('clicking a chunk row dispatches setCenterView({kind:"chunk-source", chunkId, tier})', async () => {
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('knowledge-graph');

    const chunks = screen.getAllByTestId('graph-chunk-node');
    fireEvent.click(chunks[0]);

    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      expect(cv.kind).toBe('chunk-source');
      if (cv.kind === 'chunk-source') {
        expect(cv.chunkId).toBe('c-001');
        expect(cv.tier).toBe('权威');
      }
    });
  });
});