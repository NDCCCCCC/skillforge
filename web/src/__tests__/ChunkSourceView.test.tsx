// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 1 — ChunkSourceView tests.
//
// Pins the 5 acceptance assertions for the chunk-lookup wire:
//   1. data-testid="chunk-verbatim" contains the verbatim text from
//      chunks.json entry `c-001`.
//   2. data-testid="chunk-version" reads `版本: v3.2` for `c-001`.
//   3. data-testid="chunk-effective-date" reads `生效日期: 2026-01-15`
//      for `c-001`.
//   4. TierBadge inside the chunk view carries the chunk's tier
//      (权威 for c-001, 草稿 for c-004).
//   5. For a chunkId NOT in the fixture, renders `(chunk not found
//      in fixture)` and `版本: 未知` / `生效日期: 未知`.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ChunkSourceView from '../components/center/ChunkSourceView';

afterEach(cleanup);

// Plan 260813-dno — graph-browsed chunks now fetch their content async.
// Mock globalThis.fetch for the whole describe block so the fixture-miss
// branch has a deterministic backend. The fixture-hit cases (c-001,
// c-002, c-004) never reach the fetch — their result.text !== NOT_FOUND_TEXT
// so the effect returns early. Restored between tests to keep the mock
// state isolated.
let fetchSpy: MockInstance;

beforeEach(() => {
  fetchSpy = vi.fn();
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ChunkSourceView', () => {
  it('renders chunk-verbatim with the literal text from chunks.json for c-001', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    const verbatim = screen.getByTestId('chunk-verbatim');
    expect(verbatim).toBeInTheDocument();
    // The exact text from chunks.json entry c-001 (truncated substring
    // assertion to avoid strict equality drift if punctuation tweaks):
    expect(verbatim.textContent).toContain('员工出差住宿费超过标准额度');
    expect(verbatim.textContent).toContain('分管副总审批');
  });

  it('renders chunk-version as "版本: v3.2" for c-001', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    expect(screen.getByTestId('chunk-version')).toHaveTextContent('版本: v3.2');
  });

  it('renders chunk-effective-date as "生效日期: 2026-01-15" for c-001', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    expect(screen.getByTestId('chunk-effective-date')).toHaveTextContent('生效日期: 2026-01-15');
  });

  it('renders TierBadge with data-tier="权威" for c-001 and data-tier="草稿" for c-004', () => {
    const { rerender } = render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    expect(screen.getByTestId('tier-badge')).toHaveAttribute('data-tier', '权威');

    rerender(
      <ChunkSourceView
        chunkId="c-004"
        tier="草稿"
        version="draft-0.3"
        effectiveDate="2026-07-30"
        effectiveTo={null}
      />,
    );
    expect(screen.getByTestId('tier-badge')).toHaveAttribute('data-tier', '草稿');
  });

  it('fetches + renders the verbatim text for a graph-browsed chunk not in the fixture (260813-dno)', async () => {
    // Graph-browsed chunks now fetch their content from
    // GET /api/knowledge/chunk/:id when the fixture lookup misses.
    fetchSpy.mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'c-graph-1',
          tier: '历史',
          source: 'historical/old.md',
          content: '来自后端的原文',
          effectiveFrom: 0,
          effectiveTo: 0,
          version: '',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );

    render(
      <ChunkSourceView
        chunkId="c-graph-1"
        tier="历史"
        version={null}
        effectiveDate={null}
        effectiveTo={null}
      />,
    );

    // findByTestId awaits the async resolve — the body flips from
    // 「加载中…」 to the fetched verbatim text.
    const verbatim = await screen.findByTestId('chunk-verbatim');
    expect(verbatim.textContent).toContain('来自后端的原文');
    expect(verbatim.textContent).not.toContain('(chunk not found in fixture)');
    // Metadata row is still prop-driven and correct (历史 from the prop).
    expect(screen.getByTestId('chunk-version')).toHaveTextContent('版本: 未知');
    expect(screen.getByTestId('chunk-effective-date')).toHaveTextContent('生效日期: 未知');
    expect(screen.getByTestId('chunk-source-view')).toHaveAttribute('data-tier', '历史');
  });

  it('renders 加载中… before the fetch resolves for a fixture-miss chunk', () => {
    // A never-resolving promise keeps the component in the loading state
    // so the synchronous assertion can pin the 「加载中…」 body.
    fetchSpy.mockReturnValue(new Promise(() => {}));

    render(
      <ChunkSourceView
        chunkId="c-graph-2"
        tier="历史"
        version={null}
        effectiveDate={null}
        effectiveTo={null}
      />,
    );
    expect(screen.getByTestId('chunk-verbatim').textContent).toContain('加载中…');
  });

  it('renders 加载片段失败 when the chunk fetch rejects (260813-dno)', async () => {
    fetchSpy.mockResolvedValue(
      new Response(JSON.stringify({ error: 'boom' }), { status: 500 }),
    );

    render(
      <ChunkSourceView
        chunkId="c-graph-err"
        tier="权威"
        version={null}
        effectiveDate={null}
        effectiveTo={null}
      />,
    );

    const verbatim = await screen.findByTestId('chunk-verbatim');
    expect(verbatim.textContent).toContain('加载片段失败');
    expect(verbatim.textContent).not.toContain('(chunk not found in fixture)');
  });

  // Plan 05-05 / SC-8 (CMPL-08) — VersionBadge + effective_to row tests.
  it('shows 当前有效 badge when effectiveTo is null (perpetual)', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
    expect(badge).toHaveTextContent('当前有效');
  });

  it('shows 当前有效 badge when effectiveTo is 0 (sentinel-0 perpetual)', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={0}
      />,
    );
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
  });

  it('shows 已失效 badge when effectiveTo is in the past', () => {
    render(
      <ChunkSourceView
        chunkId="c-002"
        tier="历史"
        version="v2023.1"
        effectiveDate="2023-03-01"
        effectiveTo={1677628800} // 2023-03-01
      />,
    );
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'superseded');
    expect(badge).toHaveTextContent('已失效');
  });

  it('renders 失效日期 row when effectiveTo is set', () => {
    render(
      <ChunkSourceView
        chunkId="c-002"
        tier="历史"
        version="v2023.1"
        effectiveDate="2023-03-01"
        effectiveTo={1677628800}
      />,
    );
    expect(screen.getByTestId('chunk-effective-to-date')).toHaveTextContent('失效日期: 2023-03-01');
  });

  it('omits 失效日期 row when effectiveTo is null', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    expect(screen.queryByTestId('chunk-effective-to-date')).toBeNull();
  });

  it('renders FilteredOutNotice when navigating to a superseded chunk (graph-browser path)', () => {
    render(
      <ChunkSourceView
        chunkId="c-002"
        tier="历史"
        version="v2023.1"
        effectiveDate="2023-03-01"
        effectiveTo={1677628800} // 2023-03-01 UTC — well in the past
      />,
    );
    const notice = screen.getByTestId('filtered-out-notice');
    expect(notice).toBeInTheDocument();
    expect(notice.textContent).toContain('此片段已失效');
    expect(notice.textContent).toContain('2023-03-01');
  });

  it('does NOT render FilteredOutNotice for currently-active chunks', () => {
    render(
      <ChunkSourceView
        chunkId="c-001"
        tier="权威"
        version="v3.2"
        effectiveDate="2026-01-15"
        effectiveTo={null}
      />,
    );
    expect(screen.queryByTestId('filtered-out-notice')).toBeNull();
  });
});
