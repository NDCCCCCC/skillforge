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

  it('renders the relation graph (tier nodes + doc nodes) for a populated corpus', async () => {
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
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
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('knowledge-graph');

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
    mockFetchOk(POPULATED_GRAPH);
    render(<KnowledgeGraphBrowser />);
    await screen.findByTestId('knowledge-graph');

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
});