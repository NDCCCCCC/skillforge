// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 3 — GraphTree is N-tier safe with registry-driven labels.
//
// Pins:
//   - a 3-KB fixture renders 3 tier sections; a 6-KB fixture renders 6
//   - the section heading shows the registry display_name (not the slug)
//   - an unknown slug degrades to `[slug]` — same contract as TierBadge
//   - the raw slug stays on data-tier (the machine-readable identity)
//   - the filter still narrows docs per tier (existing behavior preserved)
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import GraphTree from './GraphTree';
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

function graphWithTiers(slugs: string[]): KnowledgeGraph {
  return {
    tiers: slugs.map((slug) => ({
      tier: slug,
      docCount: 1,
      chunkCount: 3,
      docs: [
        {
          id: `${slug}::${slug}-doc.md`,
          filename: `${slug}-doc.md`,
          chunkCount: 3,
          chunks: [
            { id: '1', effectiveFrom: null, effectiveTo: null, version: null },
            { id: '2', effectiveFrom: null, effectiveTo: null, version: 'v2' },
            { id: '3', effectiveFrom: null, effectiveTo: null, version: null },
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

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('GraphTree — N-tier sections', () => {
  it('renders 3 tier sections for a 3-KB fixture with per-tier counts', () => {
    stubRegistry();
    render(<GraphTree graph={THREE_KB} />);
    const tiers = screen.getAllByTestId('graph-tier');
    expect(tiers).toHaveLength(3);
    expect(tiers.map((t) => t.getAttribute('data-tier'))).toEqual([
      'authority',
      'historical',
      'personal',
    ]);
    expect(screen.getAllByTestId('graph-doc')).toHaveLength(3);
  });

  it('renders 6 tier sections for a 6-KB fixture (N-dynamic evidence)', () => {
    stubRegistry();
    render(<GraphTree graph={SIX_KB} />);
    expect(screen.getAllByTestId('graph-tier')).toHaveLength(6);
    expect(screen.getAllByTestId('graph-doc')).toHaveLength(6);
  });

  it('heads each section with the registry display_name and keeps data-tier as the slug', async () => {
    stubRegistry();
    render(<GraphTree graph={THREE_KB} />);
    const sections = screen.getAllByTestId('graph-tier');
    expect(sections[0].getAttribute('data-tier')).toBe('authority');
    await waitFor(() => {
      // The heading label is the KNOW-05 bracketed badge form resolved from
      // the registry projection.
      expect(screen.getAllByTestId('graph-tier')[0]).toHaveTextContent('[权威]（1 文档 / 3 片段）');
    });
  });

  it('degrades an unknown slug to `[slug]` in the section heading', async () => {
    stubRegistry(REGISTRY_BASES.filter((b) => b.slug !== 'ghost-kb'));
    render(<GraphTree graph={graphWithTiers(['ghost-kb'])} />);
    await waitFor(() => {
      expect(screen.getByTestId('graph-tier')).toHaveTextContent('[ghost-kb]（1 文档 / 3 片段）');
    });
  });

  it('still narrows docs per tier when a filter is applied', () => {
    stubRegistry();
    render(<GraphTree graph={THREE_KB} filter="historical-doc" />);
    // Sections stay (the BE still emits every registry tier)…
    const sections = screen.getAllByTestId('graph-tier');
    expect(sections).toHaveLength(3);
    const byTier = (slug: string) =>
      sections.find((s) => s.getAttribute('data-tier') === slug)!;
    // …but only the matching tier keeps its docs.
    expect(byTier('historical').getAttribute('data-doc-count')).toBe('1');
    expect(byTier('authority').getAttribute('data-doc-count')).toBe('0');
    expect(byTier('personal').getAttribute('data-doc-count')).toBe('0');
    expect(screen.getAllByTestId('graph-tier-empty')).toHaveLength(2);
  });
});
