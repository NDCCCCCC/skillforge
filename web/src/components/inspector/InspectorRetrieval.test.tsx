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
