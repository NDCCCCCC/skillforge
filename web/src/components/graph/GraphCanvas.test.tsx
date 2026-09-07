// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 3 — GraphCanvas is N-tier safe (Phase 28 owns the layout
// upgrade; 26-08 only guarantees no-break plus registry-driven labels).
//
// Pins:
//   - a 3-KB registry fixture renders 3 tier nodes + their doc nodes
//   - a 6-KB registry fixture renders 6 tier nodes (the N-dynamic evidence)
//   - every doc node stays inside the SVG viewBox (no layout overflow)
//   - tier labels resolve through the KB registry projection (display_name
//     for a known slug, `[slug]` for an unknown one)
//   - clicking a doc node dispatches doc-view with the raw slug as tier
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import GraphCanvas from './GraphCanvas';
import { useAppStore } from '../../store/app-store';
import type { KnowledgeGraph } from '../../types/knowledge';

const REGISTRY_BASES = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'historical', display_name: '历史', role: 'historical' },
  { slug: 'personal', display_name: '个人', role: 'personal' },
  { slug: 'draft', display_name: '草稿', role: 'draft' },
  { slug: 'kb-2026', display_name: '新库', role: 'authority' },
  { slug: 'ghost-kb', display_name: '幽灵库', role: 'draft' },
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

/** Build a graph with `n` tiers, one doc (2 chunks) per tier. */
function graphWithTiers(slugs: string[]): KnowledgeGraph {
  return {
    tiers: slugs.map((slug) => ({
      tier: slug,
      docCount: 1,
      chunkCount: 2,
      docs: [
        {
          id: `${slug}::${slug}-doc.md`,
          filename: `${slug}-doc.md`,
          chunkCount: 2,
          chunks: [
            { id: '1', effectiveFrom: null, effectiveTo: null, version: null },
            { id: '2', effectiveFrom: null, effectiveTo: null, version: null },
          ],
        },
      ],
    })),
  };
}

const THREE_KB = graphWithTiers(['authority', 'historical', 'personal']);
const SIX_KB = graphWithTiers([
  'authority',
  'historical',
  'personal',
  'draft',
  'kb-2026',
  'ghost-kb',
]);

/** Parse `translate(x, y)` out of a node's transform attribute. */
function positionOf(el: Element): { x: number; y: number } {
  const m = /translate\((-?[\d.]+),\s*(-?[\d.]+)\)/.exec(el.getAttribute('transform') ?? '');
  expect(m).not.toBeNull();
  return { x: Number(m![1]), y: Number(m![2]) };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  useAppStore.setState({ centerView: { kind: 'knowledge-graph' } });
});

describe('GraphCanvas — N-tier layout', () => {
  it('renders 3 tier nodes + 3 doc nodes for a 3-KB registry fixture', () => {
    stubRegistry();
    render(<GraphCanvas graph={THREE_KB} filter="" />);
    expect(screen.getAllByTestId('graph-tier-node')).toHaveLength(3);
    expect(screen.getAllByTestId('graph-doc-node')).toHaveLength(3);
  });

  it('renders 6 tier nodes + 6 doc nodes for a 6-KB registry fixture', () => {
    stubRegistry();
    render(<GraphCanvas graph={SIX_KB} filter="" />);
    expect(screen.getAllByTestId('graph-tier-node')).toHaveLength(6);
    expect(screen.getAllByTestId('graph-doc-node')).toHaveLength(6);
  });

  it('keeps every doc node inside the viewBox (no overflow) on the 6-KB fixture', () => {
    stubRegistry();
    const { container } = render(<GraphCanvas graph={SIX_KB} filter="" />);
    const nodes = screen.getAllByTestId('graph-doc-node');
    expect(nodes).toHaveLength(6);
    for (const node of nodes) {
      const { x, y } = positionOf(node);
      expect(x).toBeGreaterThanOrEqual(0);
      expect(x).toBeLessThanOrEqual(900);
      expect(y).toBeGreaterThanOrEqual(0);
      expect(y).toBeLessThanOrEqual(640);
    }
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('lays the 6 tier nodes out on distinct ring positions (no overlap)', () => {
    stubRegistry();
    render(<GraphCanvas graph={SIX_KB} filter="" />);
    const seen = new Set<string>();
    for (const node of screen.getAllByTestId('graph-tier-node')) {
      const { x, y } = positionOf(node);
      seen.add(`${Math.round(x)}:${Math.round(y)}`);
    }
    expect(seen.size).toBe(6);
  });
});

describe('GraphCanvas — registry-driven tier labels', () => {
  it('labels a known tier with the registry display_name', async () => {
    stubRegistry();
    render(<GraphCanvas graph={graphWithTiers(['authority'])} filter="" />);
    await waitFor(() => {
      expect(screen.getByTestId('graph-tier-node')).toHaveTextContent('权威');
    });
  });

  it('labels an unknown tier with the `[slug]` degradation', async () => {
    stubRegistry(REGISTRY_BASES.filter((b) => b.slug !== 'ghost-kb'));
    render(<GraphCanvas graph={graphWithTiers(['ghost-kb'])} filter="" />);
    await waitFor(() => {
      expect(screen.getByTestId('graph-tier-node')).toHaveTextContent('[ghost-kb]');
    });
  });
});

describe('GraphCanvas — dispatch', () => {
  it('clicking a doc node dispatches doc-view with the raw registry slug', async () => {
    stubRegistry();
    useAppStore.setState({ centerView: { kind: 'knowledge-graph' } });
    render(<GraphCanvas graph={THREE_KB} filter="" />);
    fireEvent.click(screen.getAllByTestId('graph-doc-node')[0]);
    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      expect(cv.kind).toBe('doc-view');
      if (cv.kind === 'doc-view') {
        expect(cv.tier).toBe('authority');
        expect(cv.file).toBe('authority-doc.md');
      }
    });
  });
});
