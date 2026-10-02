// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 3 — SourceDocumentPart's residual tier sweep.
//
// Pins the post-sweep contract (the fixed CN title→tier table is gone):
//   - the pill's value is the REGISTRY SLUG from the title's directory
//     prefix, displayed through the KB registry projection
//   - an unknown slug renders `[slug]` (bracketed degradation)
//   - no derivable tier → the pill is hidden (never a fabricated tier)
//   - the chunk-source dispatch carries the slug (or '' when unknown)
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { SourceDocumentPart } from './SourceDocumentPart';
import { useAppStore } from '../../../store/app-store';

const REGISTRY_BASES = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'kb-2026', display_name: '新库', role: 'historical' },
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

function renderPart(props: { sourceId: string; title?: string }) {
  return render(
    <SourceDocumentPart
      part={{
        type: 'source-document',
        sourceId: props.sourceId,
        mediaType: 'text/markdown',
        title: props.title ?? props.sourceId,
      }}
    />,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

describe('SourceDocumentPart — registry-driven tier pill', () => {
  it('renders the registry display_name for the title directory slug', async () => {
    stubRegistry();
    renderPart({ sourceId: 'c-001', title: 'authority/annual_leave.md · chunk 3' });
    const pill = await screen.findByTestId('citation-tier-pill');
    await waitFor(() => expect(pill).toHaveTextContent('权威'));
    expect(pill.getAttribute('data-tier')).toBe('authority');
  });

  it('renders `[slug]` for a slug the registry does not know', async () => {
    // A slug no test in this file has registered — kb-bases caches the
    // projection per module, so the unknown-slug case needs a fresh slug.
    stubRegistry();
    renderPart({ sourceId: 'c-001', title: 'ghost-kb/notes.md · chunk 1' });
    const pill = await screen.findByTestId('citation-tier-pill');
    await waitFor(() => expect(pill).toHaveTextContent('[ghost-kb]'));
    expect(pill.getAttribute('data-tier')).toBe('ghost-kb');
  });

  it('hides the pill when no tier can be derived (no prefix, no fixture hit)', () => {
    stubRegistry();
    renderPart({ sourceId: 'unknown-chunk', title: 'A plain document title' });
    expect(screen.queryByTestId('citation-tier-pill')).toBeNull();
    // The chip still renders and reports an empty tier.
    expect(screen.getByTestId('source-document').getAttribute('data-tier')).toBe('');
  });

  it('falls back to the chunk fixture tier when the title carries no directory', () => {
    stubRegistry();
    // c-002 is the 历史 fixture entry.
    renderPart({ sourceId: 'c-002', title: 'A plain document title' });
    expect(screen.getByTestId('citation-tier-pill').getAttribute('data-tier')).toBe('历史');
  });

  it('dispatches chunk-source with the derived slug', async () => {
    stubRegistry();
    useAppStore.setState({ centerView: { kind: 'empty' } });
    renderPart({ sourceId: 'c-001', title: 'authority/annual_leave.md · chunk 3' });
    fireEvent.click(screen.getByTestId('source-document'));
    await waitFor(() => {
      const cv = useAppStore.getState().centerView;
      expect(cv.kind).toBe('chunk-source');
      if (cv.kind === 'chunk-source') {
        expect(cv.tier).toBe('authority');
        expect(cv.chunkId).toBe('c-001');
      }
    });
  });
});
