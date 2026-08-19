// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B3 — KnowledgeModal tests. The modal re-homes
// the deleted LeftSidebar FILES tree:
//   1. All 4 tier headers render; the default-open 权威 tier lists its
//      docs from GET /api/knowledge/docs; a collapsed tier shows no
//      docs until toggled.
//   2. Clicking a doc opens the DocViewer preview pane.
//   3. Close paths: × button, Esc key, scrim click.
//   4. Fetch failure is non-fatal — tier headers + dir hints still
//      render (offline-backend posture carried over from LeftSidebar).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import KnowledgeModal from '../components/layout/KnowledgeModal';

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

/** URL-dispatching fetch stub covering both knowledge endpoints. */
function stubKnowledgeFetch(tiers: FakeTier[] | null, docs: Record<string, string> = {}) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.startsWith('/api/knowledge/docs')) {
      if (tiers === null) return { ok: false, status: 500, json: async () => ({}) } as unknown as Response;
      return { ok: true, json: async () => ({ tiers }) } as unknown as Response;
    }
    if (url.startsWith('/api/knowledge/doc')) {
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
    return { ok: false, status: 404, json: async () => ({}) } as unknown as Response;
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

const AUTH_DOCS: FakeDoc[] = [{ filename: 'proc-001.md', bytes: 2048 }];
const DRAFT_DOCS: FakeDoc[] = [{ filename: 'note-a.md', bytes: 128 }];

function renderModal(open = true) {
  return render(<KnowledgeModal open={open} onClose={() => {}} />);
}

describe('KnowledgeModal', () => {
  beforeEach(() => {
    stubKnowledgeFetch([
      { tier: '权威', docs: AUTH_DOCS },
      { tier: '历史', docs: [] },
      { tier: '个人', docs: [] },
      { tier: '草稿', docs: DRAFT_DOCS },
    ]);
  });

  it('renders all 4 tier headers with the default-open 权威 listing docs', async () => {
    renderModal();
    const headers = screen.getAllByTestId('knowledge-modal-tier');
    expect(headers.map((h) => h.getAttribute('data-tier'))).toEqual(['权威', '历史', '个人', '草稿']);
    // 权威 is default-open → its doc row appears after fetch resolves.
    expect(await waitFor(() => screen.getByTestId('knowledge-modal-doc'))).toHaveAttribute(
      'data-tier',
      '权威',
    );
  });

  it('toggles a collapsed tier open to reveal its docs', async () => {
    renderModal();
    await waitFor(() => screen.getByTestId('knowledge-modal-doc'));
    const draftHeader = screen
      .getAllByTestId('knowledge-modal-tier')
      .find((h) => h.getAttribute('data-tier') === '草稿');
    expect(draftHeader).toBeDefined();
    // 草稿 collapsed by default → its doc hidden.
    const draftDocBefore = screen
      .queryAllByTestId('knowledge-modal-doc')
      .find((d) => d.getAttribute('data-tier') === '草稿');
    expect(draftDocBefore).toBeUndefined();
    fireEvent.click(draftHeader!);
    const draftDocAfter = await waitFor(() =>
      screen
        .getAllByTestId('knowledge-modal-doc')
        .find((d) => d.getAttribute('data-tier') === '草稿'),
    );
    expect(draftDocAfter).toHaveAttribute('data-file', 'note-a.md');
  });

  it('clicking a doc opens the DocViewer preview', async () => {
    stubKnowledgeFetch(
      [
        { tier: '权威', docs: AUTH_DOCS },
        { tier: '历史', docs: [] },
        { tier: '个人', docs: [] },
        { tier: '草稿', docs: [] },
      ],
      { '权威/proc-001.md': '---\nversion: 2\n---\n\n正文内容' },
    );
    renderModal();
    fireEvent.click(await screen.findByTestId('knowledge-modal-doc'));
    expect(await screen.findByTestId('doc-viewer')).toBeInTheDocument();
  });

  it('closes via the × button, the Esc key, and the scrim click', async () => {
    let closed = false;
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

  it('renders nothing when open={false}', () => {
    renderModal(false);
    expect(screen.queryByTestId('knowledge-modal')).toBeNull();
  });

  it('portals the scrim directly under document.body (backdrop-filter containing-block fix)', () => {
    // The modal is mounted inside TopBar, whose `.topbar` carries
    // backdrop-filter — a containing block for FIXED descendants. The
    // scrim must therefore live OUTSIDE that subtree (parent =
    // document.body) or inset-0 covers only the 56px bar.
    renderModal();
    expect(screen.getByTestId('knowledge-modal-scrim').parentElement).toBe(document.body);
  });

  it('keeps tier headers when the docs fetch fails (non-fatal)', async () => {
    stubKnowledgeFetch(null);
    renderModal();
    const headers = await waitFor(() => screen.getAllByTestId('knowledge-modal-tier'));
    expect(headers.length).toBe(4);
    // No doc rows without a successful fetch.
    await vi.waitFor(() => {
      expect(screen.queryByTestId('knowledge-modal-doc')).toBeNull();
    });
  });
});
