// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 27-05 Task 3 — Unit tests for the DocUpload panel.
//
// Phase 55-03 (ING-05) RETIRED the fake state machine. The panel now has
//
//   idle → uploading → done | error
//
// and 2xx means "a job was accepted", not "chunks landed". Pinned here:
//   - the parsing/indexing states are GONE (no testid exists at any point)
//   - a 200 hands {job_id} to the store (ingestFocusJobId) and switches the
//     admin subroute to 'ingest'
//   - the done copy is the handoff sentence, not a fabricated chunk count
//
// Contract (54-01 async upload):
//   GET  /api/knowledge/bases → { bases: [{ slug, display_name, role }] }
//   POST /api/admin/kb/:slug/upload  (multipart/form-data, X-Admin-Token)
//                              → 200 { job_id, status: 'queued' }
//                              | 413 { error: string }
//                              | 400 { error: string }
//                              | 403 { error: string }

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DocUpload from './DocUpload';
import { useAppStore } from '../../store/app-store';

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

describe('DocUpload — TestUploadHappyPath (55-03 handoff)', () => {
  it('hands the job_id to the store and switches to the ingest tab on 200', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    useAppStore.setState({ ingestFocusJobId: null, adminSubroute: 'batches' });

    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => jsonResponse({ job_id: 'ing-test-1', status: 'queued' }),
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
      expect(screen.getByTestId('doc-upload-done')).toHaveTextContent(
        '已创建摄取任务，正在打开流水线视图…',
      );
    });
    // Handoff: the job id is staged for IngestTab, and the subroute flipped.
    expect(useAppStore.getState().ingestFocusJobId).toBe('ing-test-1');
    expect(useAppStore.getState().adminSubroute).toBe('ingest');
    // No fabricated chunk count is claimed anywhere on the panel.
    expect(screen.queryByText(/个片段已入库/)).toBeNull();
  });

  it('never renders the retired parsing / indexing states, and shows uploading while in flight', async () => {
    const { fetchKnowledgeBases } = await import('../../lib/kb-bases');
    (fetchKnowledgeBases as ReturnType<typeof vi.fn>).mockResolvedValueOnce(ONE_BASE);

    // Gate the POST so the in-flight state is observable.
    let release: (() => void) | undefined;
    const gate = new Promise<void>((res) => { release = res; });
    stubFetch([
      {
        ...postType('/api/admin/kb/authority/upload'),
        respond: () => {
          void release;
          return jsonResponse({ job_id: 'ing-slow', status: 'queued' });
        },
      },
    ]);
    // Replace the global fetch with one that awaits the gate first.
    const baseFetch = globalThis.fetch;
    vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).includes('/upload')) await gate;
      return baseFetch(input, init);
    });

    render(<DocUpload />);
    fireEvent.click(screen.getByTestId('doc-upload-expand'));
    const select = await screen.findByTestId('doc-upload-slug');
    fireEvent.change(select, { target: { value: 'authority' } });
    const file = new File(['x'], 'x.md', { type: 'text/markdown' });
    fireEvent.change(screen.getByTestId('doc-upload-file'), { target: { files: [file] } });
    fireEvent.click(screen.getByTestId('doc-upload-submit'));

    expect(screen.getByTestId('doc-upload-uploading')).toHaveTextContent('正在上传…');
    expect(screen.queryByTestId('doc-upload-parsing')).toBeNull();
    expect(screen.queryByTestId('doc-upload-indexing')).toBeNull();

    release?.();
    await waitFor(() => expect(screen.getByTestId('doc-upload-done')).toBeInTheDocument());
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
        respond: () => jsonResponse({ job_id: 'ing-token-1', status: 'queued' }),
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
        respond: () => jsonResponse({ job_id: 'ing-reset-1', status: 'queued' }),
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
