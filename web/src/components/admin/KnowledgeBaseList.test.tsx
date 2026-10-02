// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 2 — the /admin Knowledge Base management view (KBM-02).
//
// This is the FIRST admin view with mutation verbs, so the test pins both
// the read surface (mirroring KPIDefinitionList's trio) and the verb
// surface:
//   read   — list with slug/role/description + the 停用 flag on
//            active:false rows, empty state, error state
//   create — POST /api/admin/kb with X-Admin-Token, list refresh on 201,
//            client-side slug-rule rejection (uppercase/underscore/empty)
//            BEFORE any request, exactly the four semantic roles
//   deactivate — confirm → POST …/deactivate → the row flips to 停用
//   errors — 409 / 400 render the BACKEND error copy, 403 names the token
//
// Trust posture under test (T-26-08-1 / T-26-08-3): every request carries
// X-Admin-Token and the FE renders whatever the server decided — the FE
// never gates.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import KnowledgeBaseList from './KnowledgeBaseList';

const TOKEN = 'tok-abc';

interface KBEntry {
  slug: string;
  display_name: string;
  role: string;
  description: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

function entry(overrides: Partial<KBEntry>): KBEntry {
  return {
    slug: 'authority',
    display_name: '权威',
    role: 'authority',
    description: '唯一可引用证据源（citation 白名单仅此角色）',
    active: true,
    created_at: '2026-09-07T00:00:00Z',
    updated_at: '2026-09-07T00:00:00Z',
    ...overrides,
  };
}

const SEED_ROWS: KBEntry[] = [
  entry({}),
  entry({
    slug: 'legacy-kb',
    display_name: '旧库',
    role: 'historical',
    description: '已停用的历史库',
    active: false,
  }),
];

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

type FetchMock = ReturnType<typeof vi.fn>;

/** Method-aware fetch stub — the GET and the POST share the /api/admin/kb URL. */
function stubFetch(routes: Array<{ url: string; method: string; respond: () => Response }>): FetchMock {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? 'GET').toUpperCase();
    const hit = routes.find((r) => r.url === url && r.method.toUpperCase() === method);
    if (!hit) return Promise.resolve(jsonResponse({ error: 'no route in test' }, 404));
    return Promise.resolve(hit.respond());
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

/** All fetch calls as [url, method, parsed JSON body]. */
function callsOf(fetchMock: FetchMock): Array<[string, string, unknown]> {
  return (fetchMock.mock.calls as Array<[RequestInfo | URL, RequestInit | undefined]>).map(
    ([input, init]) => [
      String(input),
      (init?.method ?? 'GET').toUpperCase(),
      init?.body ? JSON.parse(String(init.body)) : null,
    ],
  );
}

function headerOf(fetchMock: FetchMock, url: string, method: string): string {
  const init = (fetchMock.mock.calls as Array<[RequestInfo | URL, RequestInit | undefined]>).find(
    ([u, i]) => String(u) === url && (i?.method ?? 'GET').toUpperCase() === method,
  )?.[1];
  return ((init?.headers ?? {}) as Record<string, string>)['X-Admin-Token'];
}

function getType(url: string) {
  return { url, method: 'GET' };
}
function postType(url: string) {
  return { url, method: 'POST' };
}

function setInput(testId: string, value: string): void {
  fireEvent.change(screen.getByTestId(testId), { target: { value } });
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', TOKEN);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('KnowledgeBaseList — read surface', () => {
  it('renders every base with slug/role/description and flags the deactivated one', async () => {
    const fetchMock = stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: SEED_ROWS }) },
    ]);
    render(<KnowledgeBaseList />);

    const activeRow = await screen.findByTestId('kb-row-authority');
    expect(activeRow).toHaveTextContent('authority');
    expect(activeRow).toHaveTextContent('权威');
    expect(activeRow).toHaveTextContent('唯一可引用证据源');
    expect(activeRow.querySelector('[data-testid="kb-deactivated"]')).toBeNull();

    const retiredRow = screen.getByTestId('kb-row-legacy-kb');
    expect(retiredRow).toHaveTextContent('historical');
    expect(retiredRow.querySelector('[data-testid="kb-deactivated"]')).not.toBeNull();

    // The GET carries the admin token (the server is the trust boundary).
    expect(headerOf(fetchMock, '/api/admin/kb', 'GET')).toBe(TOKEN);
  });

  it('renders the empty state when the registry holds no bases', async () => {
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
    ]);
    render(<KnowledgeBaseList />);
    expect(await screen.findByTestId('kb-empty')).toBeInTheDocument();
  });

  it('renders the error state when the GET is not ok', async () => {
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ error: 'admin token required' }, 403) },
    ]);
    render(<KnowledgeBaseList />);
    expect(await screen.findByTestId('kb-error')).toBeInTheDocument();
  });
});

describe('KnowledgeBaseList — create verb', () => {
  it('POSTs the new base with the admin token and refreshes the list on 201', async () => {
    const created = entry({ slug: 'kb-2026', display_name: '新库', role: 'authority', description: '' });
    let getCalls = 0;
    const fetchMock = stubFetch([
      {
        ...getType('/api/admin/kb'),
        respond: () => {
          getCalls += 1;
          return jsonResponse({ signature: 'sig-1', bases: getCalls > 1 ? [created] : [] });
        },
      },
      { ...postType('/api/admin/kb'), respond: () => jsonResponse(created, 201) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-create-form');

    setInput('kb-slug-input', 'kb-2026');
    setInput('kb-display-name-input', '新库');
    fireEvent.change(screen.getByTestId('kb-role-select'), { target: { value: 'authority' } });
    fireEvent.click(screen.getByTestId('kb-submit'));

    await waitFor(() => expect(screen.getByTestId('kb-row-kb-2026')).toBeInTheDocument());

    const post = callsOf(fetchMock).find(([url, method, body]) => url === '/api/admin/kb' && method === 'POST' && body !== null);
    expect(post).toBeDefined();
    expect(post![2]).toEqual({
      slug: 'kb-2026',
      display_name: '新库',
      role: 'authority',
      description: '',
    });
    expect(headerOf(fetchMock, '/api/admin/kb', 'POST')).toBe(TOKEN);
  });

  it.each([
    ['KB_2026', 'uppercase'],
    ['kb_2026', 'underscore'],
    ['', 'empty'],
    ['-lead', 'leading hyphen'],
  ])('rejects the %j slug client-side (%s) without POSTing', async (slug) => {
    const fetchMock = stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
    ]);
    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-create-form');

    if (slug !== '') {
      setInput('kb-slug-input', slug);
    }
    setInput('kb-display-name-input', '新库');
    fireEvent.click(screen.getByTestId('kb-submit'));

    expect(await screen.findByTestId('kb-slug-error')).toHaveTextContent('slug');
    // Exactly ONE call happened — the initial GET. No mutation left the FE.
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('offers exactly the four semantic roles', async () => {
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
    ]);
    render(<KnowledgeBaseList />);
    const select = await screen.findByTestId('kb-role-select');
    const values = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(values).toEqual(['authority', 'historical', 'personal', 'draft']);
  });

  it('renders the backend 409 copy for a duplicate slug', async () => {
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
      { ...postType('/api/admin/kb'), respond: () => jsonResponse({ error: 'slug already exists' }, 409) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-create-form');
    setInput('kb-slug-input', 'authority');
    setInput('kb-display-name-input', '重复');
    fireEvent.click(screen.getByTestId('kb-submit'));

    expect(await screen.findByTestId('kb-action-error')).toHaveTextContent('slug 已存在');
  });

  it('renders the backend 400 rule copy verbatim on a rule rejection', async () => {
    const backend = 'slug "x" is reserved: names a semantic role';
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
      { ...postType('/api/admin/kb'), respond: () => jsonResponse({ error: backend }, 400) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-create-form');
    // The FE regex passes "historical2"; only the SERVER refuses it — the FE
    // must surface the backend verdict rather than inventing its own.
    setInput('kb-slug-input', 'historical2');
    setInput('kb-display-name-input', '保留字边界');
    fireEvent.click(screen.getByTestId('kb-submit'));

    expect(await screen.findByTestId('kb-action-error')).toHaveTextContent(backend);
  });

  it('names the admin token on a 403 mutation', async () => {
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: [] }) },
      { ...postType('/api/admin/kb'), respond: () => jsonResponse({ error: 'admin token required' }, 403) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-create-form');
    setInput('kb-slug-input', 'kb-2026');
    setInput('kb-display-name-input', '新库');
    fireEvent.click(screen.getByTestId('kb-submit'));

    expect(await screen.findByTestId('kb-action-error')).toHaveTextContent('X-Admin-Token');
  });
});

describe('KnowledgeBaseList — deactivate verb', () => {
  it('confirms, POSTs deactivate with the token, and marks the row 停用', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const fetchMock = stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: SEED_ROWS }) },
      { ...postType('/api/admin/kb/authority/deactivate'), respond: () => jsonResponse({ slug: 'authority', active: false }) },
    ]);

    render(<KnowledgeBaseList />);
    const row = await screen.findByTestId('kb-row-authority');
    expect(row.querySelector('[data-testid="kb-deactivated"]')).toBeNull();

    fireEvent.click(screen.getByTestId('kb-deactivate-authority'));

    await waitFor(() => {
      expect(screen.getByTestId('kb-row-authority').querySelector('[data-testid="kb-deactivated"]')).not.toBeNull();
    });
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(headerOf(fetchMock, '/api/admin/kb/authority/deactivate', 'POST')).toBe(TOKEN);
  });

  it('shows the backend error copy when deactivate is refused', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: SEED_ROWS }) },
      { ...postType('/api/admin/kb/authority/deactivate'), respond: () => jsonResponse({ error: 'unknown knowledge base' }, 404) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-row-authority');
    fireEvent.click(screen.getByTestId('kb-deactivate-authority'));

    expect(await screen.findByTestId('kb-action-error')).toHaveTextContent('unknown knowledge base');
    // A refused mutation flips nothing locally.
    expect(screen.getByTestId('kb-row-authority').querySelector('[data-testid="kb-deactivated"]')).toBeNull();
  });

  it('does nothing when the operator dismisses the confirm', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    const fetchMock = stubFetch([
      { ...getType('/api/admin/kb'), respond: () => jsonResponse({ signature: 'sig-1', bases: SEED_ROWS }) },
      { ...postType('/api/admin/kb/authority/deactivate'), respond: () => jsonResponse({ error: 'should not be called' }, 500) },
    ]);

    render(<KnowledgeBaseList />);
    await screen.findByTestId('kb-row-authority');
    fireEvent.click(screen.getByTestId('kb-deactivate-authority'));

    expect(screen.getByTestId('kb-row-authority').querySelector('[data-testid="kb-deactivated"]')).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
