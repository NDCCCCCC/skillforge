// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 27-05 Task 3 — Unit tests for the DocUpload panel.
//
// Tests the upload state machine:
//   idle → uploading → parsing → indexing → done | error
//
// Contract (27-03 SUMMARY, frozen):
//   GET  /api/knowledge/bases → { bases: [{ slug, display_name, role }] }
//   POST /api/admin/kb/:slug/upload  (multipart/form-data, X-Admin-Token)
//                              → 200 { chunks: number }
//                              | 413 { error: string }
//                              | 400 { error: string }
//                              | 403 { error: string }

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DocUpload from './DocUpload';

const TOKEN = 'tok-abc';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

type FetchMock = ReturnType<typeof vi.fn>;

/** Method-aware fetch stub. */
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

/** Pre-built base lists for reuse across tests. */
const TWO_BASES = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'historical', display_name: '历史', role: 'historical' },
];
const ONE_BASE = [{ slug: 'authority', display_name: '权威', role: 'authority' }];

beforeEach(() => {
  localStorage.setItem('sf_admin_token', TOKEN);
  // Reset the fetchKnowledgeBases module-level promise cache so each test
  // gets a fresh call that hits our stubbed fetch.
  vi.mock('../../lib/kb-bases', async (importOriginal) => {
    const original = await importOriginal<typeof import('../../lib/kb-bases')>();
    return {
      ...original,
      fetchKnowledgeBases: vi.fn(() => Promise.resolve([] as Array<{ slug: string; display_name: string; role: string }>)),
    };
  });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('DocUpload — panel expand/collapse', () => {
  it('renders collapsed expand button by default', () => {
    stubFetch([]);
    render(<DocUpload />);
    expect(screen.getByTestId('doc-upload-expand')).toBeInTheDocument();
    expect(screen.queryByTestId('doc-upload-panel')).toBeNull();
  });

  it('expands the panel when expand button is clicked', () => {
    stubFetch([]);
    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));
    expect(screen.getByTestId('doc-upload-panel')).toBeInTheDocument();
  });

  it('collapses the panel when collapse button is clicked', () => {
    stubFetch([]);
    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));
    expect(screen.getByTestId('doc-upload-panel')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('doc-upload-collapse'));
    expect(screen.queryByTestId('doc-upload-panel')).toBeNull();
  });
});

describe('DocUpload — KB dropdown', () => {
  it('populates dropdown from fetchKnowledgeBases', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(TWO_BASES);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options).toContain('authority');
    expect(options).toContain('historical');
  });
});

describe('DocUpload — TestUploadHappyPath', () => {
  it('shows done state with chunk count on 200', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ chunks: 5 }),
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });

    const file = new File(['# Hello'], 'hello.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('doc-upload-done')).toHaveTextContent('5 个片段已入库');
    });
  });
});

describe('DocUpload — TestUploadInvalidSlug', () => {
  it('submit button disabled when no slug selected', () => {
    stubFetch([]);
    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const file = new File(['x'], 'x.pdf', { type: 'application/pdf' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    expect(screen.getByTestId('doc-upload-submit')).toBeDisabled();
  });

  it('submit button disabled when no file selected', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([]);
    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });

    expect(screen.getByTestId('doc-upload-submit')).toBeDisabled();
  });
});

describe('DocUpload — TestUploadOversize', () => {
  it('shows error state on 413', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ error: 'file too large' }, 413),
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['x'], 'big.pdf', { type: 'application/pdf' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('doc-upload-error')).toHaveTextContent('file too large');
    });
  });
});

describe('DocUpload — TestUploadInvalidFileType', () => {
  it('shows error text verbatim on 400', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ error: 'unknown format' }, 400),
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['corrupt'], 'bad.exe', { type: 'application/x-executable' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('doc-upload-error')).toHaveTextContent('unknown format');
    });
  });
});

describe('DocUpload — TestUploadNetworkError', () => {
  it('shows 网络错误 on fetch throw', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => { throw new Error('network failure'); },
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['x'], 'x.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('doc-upload-error')).toHaveTextContent('网络错误');
    });
  });
});

describe('DocUpload — TestUploadAdminTokenSent', () => {
  it('POST includes X-Admin-Token header from localStorage', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    const fetchMock = stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ chunks: 1 }),
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['x'], 'x.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    await waitFor(() => {
      expect(headerOf(fetchMock, '/api/admin/kb/authority/upload', 'POST')).toBe(TOKEN);
    });
  });

  it('reset button returns to idle state after done', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ chunks: 3 }),
      },
    ]);

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));

    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['x'], 'x.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });

    fireEvent.click(screen.getByTestId('doc-upload-submit'));
    await waitFor(() => expect(screen.getByTestId('doc-upload-done')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '再传一份' }));

    // Panel should still be expanded but reset to idle
    expect(screen.getByTestId('doc-upload-panel')).toBeInTheDocument();
    expect(screen.queryByTestId('doc-upload-done')).toBeNull();
  });
});
