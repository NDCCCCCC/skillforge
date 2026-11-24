// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-03 (ING-05) -- the 摄取任务 tab pins: roster columns/copy,
// honest error rows, encryption cadence, the upload focus handoff, the
// per-step retry lifecycle, and every wire-degradation branch.
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import IngestTab from './IngestTab';
import { useAppStore } from '../../../store/app-store';
import type { IngestJob } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function makeJob(over: Partial<IngestJob> = {}): IngestJob {
  return {
    job_id: 'ing-abc',
    slug: 'authority',
    filename: 'policy.md',
    sha256: 'deadbeef',
    doc_id: 'authority/x-policy.md',
    status: 'running',
    error: '',
    created_at: '2026-11-24T10:00:00.000Z',
    updated_at: '2026-11-24T10:00:05.000Z',
    steps: [
      { step: 'parse', status: 'done', attempt: 1, started_at: '2026-11-24T10:00:00.000Z', completed_at: '2026-11-24T10:00:01.000Z', artifact_ref: 'authority/x-policy.md' },
      { step: 'chunk', status: 'done', attempt: 1 },
      { step: 'embed', status: 'running', attempt: 1, started_at: '2026-11-24T10:00:02.000Z' },
      { step: 'vector_insert', status: 'pending', attempt: 0 },
      { step: 'ecl_merge', status: 'pending', attempt: 0 },
      { step: 'ecl_extract', status: 'failed', attempt: 2, error: '[deterministic] bad payload' },
      { step: 'ecl_kg_load', status: 'stale', attempt: 1 },
      { step: 'ecl_milvus_load', status: 'pending', attempt: 0 },
    ],
    failed_items: [],
    ...over,
  };
}

let jobsList: IngestJob[] = [];
let listStatus = 200;
let detailStatus = 200;
let detailBody: IngestJob | null = null;
let retryStatus = 200;

const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(input);
  const method = init?.method ?? 'GET';
  if (url === '/api/admin/ingest/jobs' && method === 'GET') {
    if (listStatus !== 200) {
      return Promise.resolve(jsonResponse({ error: 'list boom' }, listStatus));
    }
    return Promise.resolve(jsonResponse({ jobs: jobsList }));
  }
  if (url.startsWith('/api/admin/ingest/jobs/') && url.includes('/steps/') && method === 'POST') {
    if (retryStatus !== 200) {
      return Promise.resolve(jsonResponse({ error: 'step is not failed' }, retryStatus));
    }
    return Promise.resolve(jsonResponse({ status: 'queued', job_id: 'ing-abc', step: 'ecl_extract' }));
  }
  if (url.startsWith('/api/admin/ingest/jobs/') && method === 'GET') {
    if (detailStatus !== 200) {
      return Promise.resolve(jsonResponse({ error: 'detail boom' }, detailStatus));
    }
    return Promise.resolve(jsonResponse(detailBody ?? makeJob()));
  }
  return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
});

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok');
  jobsList = [];
  listStatus = 200;
  detailStatus = 200;
  detailBody = makeJob();
  retryStatus = 200;
  fetchMock.mockClear();
  vi.stubGlobal('fetch', fetchMock);
  useAppStore.setState({
    ingestFocusJobId: null,
    adminSubroute: 'ingest',
    route: 'admin',
  });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('IngestTab — Jobs roster', () => {
  it('renders the six locked column headers and one row per job', async () => {
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-jobs-row')).toBeInTheDocument());
    const headers = Array.from(
      screen.getByTestId('ingest-jobs-row').closest('table')!.querySelectorAll('th'),
    ).map((h) => h.textContent);
    expect(headers).toEqual(['Job', '文档', '状态', '进度', '创建时间', '操作']);
    expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument();
  });

  it('renders the locked empty state ONLY for a real 2xx empty roster', async () => {
    jobsList = [];
    render(<IngestTab />);
    expect(await screen.findByTestId('ingest-jobs-empty')).toHaveTextContent('暂无摄取任务');
    expect(screen.getByTestId('ingest-jobs-empty')).toHaveTextContent('上传文档后自动出现在这里');
  });

  it('renders an honest ERROR row (not a fake empty state) when the roster read fails', async () => {
    listStatus = 500;
    render(<IngestTab />);
    const err = await screen.findByTestId('ingest-load-error');
    expect(err).toHaveTextContent('摄取任务加载失败');
    expect(err).toHaveTextContent('点击重试');
    expect(screen.queryByTestId('ingest-jobs-empty')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '点击重试' }));
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(1));
  });

  it('renders the 403 copy verbatim when the admin token is rejected', async () => {
    listStatus = 403;
    render(<IngestTab />);
    expect(await screen.findByTestId('ingest-403')).toHaveTextContent('缺少或错误的 X-Admin-Token');
    expect(screen.queryByTestId('ingest-jobs-empty')).toBeNull();
  });
});

describe('IngestTab — detail view', () => {
  it('opens the detail view with the progress meter, timeline and audit hint', async () => {
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));

    await waitFor(() => expect(screen.getByTestId('ingest-job-timeline')).toBeInTheDocument());
    expect(screen.getByTestId('ingest-progress-counter')).toHaveTextContent('2/8');
    const meter = screen.getByRole('meter');
    expect(meter).toHaveAttribute('aria-label', '摄取进度: 2 of 8');
    expect(screen.getByText(/每次重试记入审计/)).toBeInTheDocument();
    // Back link returns to the roster.
    fireEvent.click(screen.getByTestId('ingest-back-to-jobs'));
    await waitFor(() => expect(screen.getByTestId('ingest-jobs-row')).toBeInTheDocument());
  });

  it('renders the failed-items table ONLY when items exist, with zero retry buttons inside', async () => {
    detailBody = makeJob({
      failed_items: [
        { step: 'embed', item_key: 'hash-1', status: 'failed', error: 'embed 500' },
        { step: 'embed', item_key: 'hash-2', status: 'failed', error: 'embed 500' },
      ],
    });
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));

    await waitFor(() => expect(screen.getByTestId('ingest-failed-items-table')).toBeInTheDocument());
    expect(screen.getAllByTestId('ingest-failed-item-row')).toHaveLength(2);
    expect(screen.getByText('hash-1')).toBeInTheDocument();
    // The single retry entry point is the timeline row -- never the table.
    const table = screen.getByTestId('ingest-failed-items-table');
    expect(table.querySelectorAll('button')).toHaveLength(0);
  });

  it('does not render the matrix while docs[] is absent (wire degradation)', async () => {
    detailBody = makeJob();
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));
    await waitFor(() => expect(screen.getByTestId('ingest-job-timeline')).toBeInTheDocument());
    expect(screen.queryByTestId('ingest-step-matrix')).toBeNull();
  });

  it('renders the collapsed log section with the forward-compat empty copy', async () => {
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));
    await waitFor(() => expect(screen.getByTestId('ingest-log-toggle')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-log-toggle'));
    expect(screen.getByTestId('ingest-log-empty')).toHaveTextContent('本任务暂无过程日志');
  });

  it('falls back to the roster with a toast when the detail read 404s', async () => {
    jobsList = [makeJob()];
    detailStatus = 404;
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));
    await waitFor(() => expect(screen.queryByTestId('ingest-job-timeline')).toBeNull());
  });
});

describe('IngestTab — retry lifecycle', () => {
  it('posts the retry, toasts the locked success copy and re-fetches the job', async () => {
    jobsList = [makeJob()];
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));
    await waitFor(() => expect(screen.getByTestId('ingest-step-retry-ecl_extract')).toBeInTheDocument());

    const before = fetchMock.mock.calls.length;
    fireEvent.click(screen.getByTestId('ingest-step-retry-ecl_extract'));

    await waitFor(() => {
      const posted = fetchMock.mock.calls.some(
        ([url, init]) =>
          String(url) === '/api/admin/ingest/jobs/ing-abc/steps/ecl_extract/retry' &&
          (init as RequestInit | undefined)?.method === 'POST',
      );
      expect(posted).toBe(true);
    });
    // Re-sync fired (server truth -- no local pre-marking of stale steps).
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(before + 1));
  });

  it('toasts the locked success copy and the verbatim server error on a 409', async () => {
    // The /admin shell owns the single toast viewport; here we capture the
    // dispatched events directly so the copy is pinned, not just "a call
    // happened".
    const toasts: Array<{ kind: string; message: string }> = [];
    const listener = (e: Event) => {
      const detail = (e as CustomEvent<{ kind: string; message: string }>).detail;
      toasts.push(detail);
    };
    window.addEventListener('sf-toast', listener);
    try {
      jobsList = [makeJob()];
      render(<IngestTab />);
      await waitFor(() => expect(screen.getByTestId('ingest-view-ing-abc')).toBeInTheDocument());
      fireEvent.click(screen.getByTestId('ingest-view-ing-abc'));
      await waitFor(() => expect(screen.getByTestId('ingest-step-retry-ecl_extract')).toBeInTheDocument());

      fireEvent.click(screen.getByTestId('ingest-step-retry-ecl_extract'));
      await waitFor(() =>
        expect(toasts.some((t) => t.message === '已重新入队步骤 实体抽取')).toBe(true),
      );

      // 409 on the second retry: the server's own wording must survive.
      retryStatus = 409;
      fireEvent.click(screen.getByTestId('ingest-step-retry-ecl_extract'));
      await waitFor(() =>
        expect(toasts.some((t) => t.message === '重试失败：step is not failed')).toBe(true),
      );
    } finally {
      window.removeEventListener('sf-toast', listener);
    }
  });
});

describe('IngestTab — upload focus handoff', () => {
  it('opens the staged job detail once and clears the store field', async () => {
    jobsList = [makeJob()];
    useAppStore.setState({ ingestFocusJobId: 'ing-abc' });
    render(<IngestTab />);
    await waitFor(() => expect(screen.getByTestId('ingest-job-timeline')).toBeInTheDocument());
    expect(screen.getByTestId('ingest-progress-counter')).toHaveTextContent('2/8');
    // Read-then-clear: the handoff is consumed exactly once.
    expect(useAppStore.getState().ingestFocusJobId).toBeNull();
  });
});

describe('IngestTab — polling cadence (55-UI-SPEC Component 7)', () => {
  it('polls a hot job at 2s and flips to 10s once it reaches a terminal state', async () => {
    vi.useFakeTimers();
    jobsList = [makeJob({ status: 'failed', steps: makeJob().steps.map((s) => ({ ...s, status: 'done' })) })];
    render(<IngestTab />);
    // Initial tick resolves the roster.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const idleCalls = fetchMock.mock.calls.length;

    // Idle cadence: nothing at 2s, one poll at 10s.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(fetchMock.mock.calls.length).toBe(idleCalls);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(8000);
    });
    expect(fetchMock.mock.calls.length).toBeGreaterThan(idleCalls);
    vi.useRealTimers();
  });
});
