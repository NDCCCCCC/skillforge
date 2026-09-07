// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// KnowledgeModal tests (Phase 26 registry-driven rewrite). The modal
// re-homes the deleted LeftSidebar FILES tree:
//   1. Tier headers render in SERVER order (the registry's active slug
//      set — an admin-created base appears without an FE change) and
//      carry the registry display label via useTierLabel; an unknown
//      slug degrades to `[slug]`.
//   2. The default-open tier is the first resolved tier; clicking a doc
//      opens the DocViewer preview pane (its badge shows the registry
//      label too).
//   3. Close paths: × button, Esc key, scrim click.
//   4. Fetch failure is non-fatal — the seed tier headers + dir hints
//      still render. An OK-but-empty tier list renders an empty tree
//      (a real answer, not the seed fallback).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ComponentType } from 'react';
import '@testing-library/jest-dom/vitest';

// KnowledgeModal imports kb-bases, whose projection promise is cached at
// module scope — every test re-imports the fresh module graph so the
// cache never bleeds across cases (same pattern as kb-bases.test.ts).
type KnowledgeModalT = ComponentType<{ open: boolean; onClose: () => void }>;
async function freshModal(): Promise<KnowledgeModalT> {
  vi.resetModules();
  return (await import('../components/layout/KnowledgeModal')).default;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

interface FakeDoc {
  filename: string;
  bytes: number;
}
interface FakeTier {
  tier: string;
  docs: FakeDoc[];
}
interface FakeBase {
  slug: string;
  display_name: string;
  role: string;
}

interface StubOptions {
  tiers: FakeTier[] | null;
  bases?: FakeBase[] | 'fail';
  docs?: Record<string, string>;
}

/** URL-dispatching fetch stub covering the three knowledge endpoints. */
function stubKnowledgeFetch({ tiers, bases = 'fail', docs = {} }: StubOptions) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.startsWith('/api/knowledge/docs')) {
      if (tiers === null) return { ok: false, status: 500, json: async () => ({}) } as unknown as Response;
      return { ok: true, json: async () => ({ tiers }) } as unknown as Response;
    }
    if (url.startsWith('/api/knowledge/doc?')) {
      const u = new URL(url, 'http://localhost');
      const key = `${u.searchParams.get('tier')}/${u.searchParams.get('file')}`;
      const content = docs[key];
      if (content === undefined) {
        return { ok: false, status: 404, json: async () => ({}) } as unknown as Response;
      }
      return {
        ok: true,
        json: async () => ({ tier: u.searchParams.get('tier'), file: u.searchParams.get('file'), content }),
      } as unknown as Response;
    }
    if (url.startsWith('/api/knowledge/bases')) {
      if (bases === 'fail') {
        return { ok: false, status: 503, json: async () => ({}) } as unknown as Response;
      }
      return { ok: true, json: async () => ({ bases }) } as unknown as Response;
    }
    return { ok: false, status: 404, json: async () => ({}) } as unknown as Response;
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

const AUTH_DOCS: FakeDoc[] = [{ filename: 'proc-001.md', bytes: 2048 }];
const DRAFT_DOCS: FakeDoc[] = [{ filename: 'note-a.md', bytes: 128 }];

const SEED_BASES: FakeBase[] = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'historical', display_name: '历史', role: 'historical' },
  { slug: 'personal', display_name: '个人', role: 'personal' },
  { slug: 'draft', display_name: '草稿', role: 'draft' },
];

async function renderModal(open = true) {
  const KnowledgeModal = await freshModal();
  return render(<KnowledgeModal open={open} onClose={() => {}} />);
}

describe('KnowledgeModal', () => {
  beforeEach(() => {
    stubKnowledgeFetch({
      tiers: [
        { tier: 'authority', docs: AUTH_DOCS },
        { tier: 'historical', docs: [] },
        { tier: 'personal', docs: [] },
        { tier: 'draft', docs: DRAFT_DOCS },
      ],
      bases: SEED_BASES,
    });
  });

  it('renders tier headers in server order with registry display labels; first tier defaults open', async () => {
    await renderModal();
    const headers = await waitFor(() => screen.getAllByTestId('knowledge-modal-tier'));
    // Server order verbatim — the registry is the single source of the
    // KB set (KBM-01); the FE keeps no ordering list of its own.
    expect(headers.map((h) => h.getAttribute('data-tier'))).toEqual([
      'authority',
      'historical',
      'personal',
      'draft',
    ]);
    // Labels resolve to the registry display names (bracketed badge form).
    await waitFor(() => {
      expect(headers[0]).toHaveTextContent('[权威]');
      expect(headers[3]).toHaveTextContent('[草稿]');
    });
    // First resolved tier is default-open → its doc row appears.
    expect(await screen.findByTestId('knowledge-modal-doc')).toHaveAttribute(
      'data-tier',
      'authority',
    );
  });

  it('renders a server-provided slug absent from the projection as the bracketed slug', async () => {
    stubKnowledgeFetch({
      tiers: [{ tier: 'it-helpdesk', docs: [] }],
      bases: SEED_BASES, // it-helpdesk not registered here
    });
    await renderModal();
    const header = await waitFor(() => screen.getByTestId('knowledge-modal-tier'));
    expect(header).toHaveTextContent('[it-helpdesk]');
  });

  it('renders the empty-registry answer as an empty tree, not the seed fallback', async () => {
    stubKnowledgeFetch({ tiers: [], bases: [] });
    await renderModal();
    expect(await screen.findByTestId('knowledge-modal-empty')).toBeInTheDocument();
    expect(screen.queryByTestId('knowledge-modal-tier')).toBeNull();
    expect(screen.queryByTestId('knowledge-modal-doc')).toBeNull();
  });

  it('toggles a collapsed tier open to reveal its docs', async () => {
    await renderModal();
    await screen.findByTestId('knowledge-modal-doc');
    const draftHeader = screen
      .getAllByTestId('knowledge-modal-tier')
      .find((h) => h.getAttribute('data-tier') === 'draft');
    expect(draftHeader).toBeDefined();
    // draft collapsed by default → its doc hidden.
    const draftDocBefore = screen
      .queryAllByTestId('knowledge-modal-doc')
      .find((d) => d.getAttribute('data-tier') === 'draft');
    expect(draftDocBefore).toBeUndefined();
    fireEvent.click(draftHeader!);
    const draftDocAfter = await waitFor(() =>
      screen
        .getAllByTestId('knowledge-modal-doc')
        .find((d) => d.getAttribute('data-tier') === 'draft'),
    );
    expect(draftDocAfter).toHaveAttribute('data-file', 'note-a.md');
  });

  it('clicking a doc opens the DocViewer preview with the registry tier badge', async () => {
    stubKnowledgeFetch({
      tiers: [
        { tier: 'authority', docs: AUTH_DOCS },
        { tier: 'historical', docs: [] },
        { tier: 'personal', docs: [] },
        { tier: 'draft', docs: [] },
      ],
      bases: SEED_BASES,
      docs: { 'authority/proc-001.md': '---\nversion: 2\n---\n\n正文内容' },
    });
    await renderModal();
    fireEvent.click(await screen.findByTestId('knowledge-modal-doc'));
    expect(await screen.findByTestId('doc-viewer')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId('doc-viewer-tier')).toHaveTextContent('[权威]');
    });
  });

  it('closes via the × button, the Esc key, and the scrim click', async () => {
    let closed = false;
    const KnowledgeModal = await freshModal();
    render(<KnowledgeModal open onClose={() => { closed = true; }} />);
    await screen.findByTestId('knowledge-modal');
    fireEvent.click(screen.getByTestId('knowledge-modal-close'));
    expect(closed).toBe(true);

    closed = false;
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(closed).toBe(true);

    closed = false;
    fireEvent.click(screen.getByTestId('knowledge-modal-scrim'));
    expect(closed).toBe(true);
  });

  it('renders nothing when open={false}', async () => {
    await renderModal(false);
    expect(screen.queryByTestId('knowledge-modal')).toBeNull();
  });

  it('portals the scrim directly under document.body (backdrop-filter containing-block fix)', async () => {
    // The modal is mounted inside TopBar, whose `.topbar` carries
    // backdrop-filter — a containing block for FIXED descendants. The
    // scrim must therefore live OUTSIDE that subtree (parent =
    // document.body) or inset-0 covers only the 56px bar.
    await renderModal();
    expect(screen.getByTestId('knowledge-modal-scrim').parentElement).toBe(document.body);
  });

  it('keeps seed tier headers when the docs fetch fails (non-fatal offline posture)', async () => {
    // Docs endpoint down, registry projection UP → seed headers still
    // render WITH the registry display labels.
    stubKnowledgeFetch({ tiers: null, bases: SEED_BASES });
    await renderModal();
    const headers = await waitFor(() => screen.getAllByTestId('knowledge-modal-tier'));
    // The offline posture derives from kb-bases' whitelisted seed home —
    // 4 headers, first one default-open with its computed directory hint.
    expect(headers.map((h) => h.getAttribute('data-tier'))).toEqual([
      'authority',
      'historical',
      'personal',
      'draft',
    ]);
    await waitFor(() => expect(headers[0]).toHaveTextContent('[权威]'));
    await vi.waitFor(() => {
      expect(screen.queryByTestId('knowledge-modal-doc')).toBeNull();
    });
    expect(screen.getByText('knowledge/authority/')).toBeInTheDocument();
  });

  it('degrades labels to bracketed slugs when BOTH endpoints are down (full outage)', async () => {
    stubKnowledgeFetch({ tiers: null, bases: 'fail' });
    await renderModal();
    const headers = await waitFor(() => screen.getAllByTestId('knowledge-modal-tier'));
    expect(headers.map((h) => h.getAttribute('data-tier'))).toEqual([
      'authority',
      'historical',
      'personal',
      'draft',
    ]);
    // No projection data anywhere → the honest `[slug]` form, never a
    // guessed display name.
    await waitFor(() => expect(headers[0]).toHaveTextContent('[authority]'));
    expect(headers[0]).not.toHaveTextContent('权威');
  });
});
