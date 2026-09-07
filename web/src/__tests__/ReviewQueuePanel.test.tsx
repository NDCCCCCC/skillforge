// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 2 — ReviewQueuePanel (FBK-02 复核视图) against the
// frozen 29-05 wire contracts:
//
//   GET  /api/admin/feedback/review?status=pending|resolved[&disposition=…]
//     → 200 {pending:[{chunk_id, kb_slug, report_count, last_reported_at, comments[]}], total}
//     | 200 {dispositions:[{id, ts, chunk_id, disposition, note, actor}]}
//   POST /api/admin/feedback/dispositions {chunk_id, disposition} → 201 {closed_count} | 4xx {error}
//
// Pinned here: the verbatim FBK-05 trust banner, the chunk aggregation
// (报告次数 ≥2 = the amber priority signal, D-29-08), single-click
// terminal dispositions with the closed-count toast (D-29-09), the
// server-truth refetch, per-report note expansion, and the verbatim
// empty/error copy from 29-UI-SPEC §Copywriting Contract.
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ReviewQueuePanel from '../components/admin/ReviewQueuePanel';

type FetchMock = ReturnType<typeof vi.fn>;

interface ToastDetail {
  kind?: string;
  message: string;
}

function captureToasts() {
  const toasts: ToastDetail[] = [];
  const handler = (e: Event) => {
    toasts.push((e as CustomEvent<ToastDetail>).detail);
  };
  window.addEventListener('sf-toast', handler);
  return { toasts, stop: () => window.removeEventListener('sf-toast', handler) };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Method-aware fetch stub — GET/POST share URLs across status tabs. */
function stubFetch(routes: Array<{ url: string; method?: string; respond: () => Response }>): FetchMock {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? 'GET').toUpperCase();
    const hit = routes.find((r) => r.url === url && (r.method ?? 'GET').toUpperCase() === method);
    if (!hit) return Promise.resolve(jsonResponse({ error: 'no route in test' }, 404));
    return Promise.resolve(hit.respond());
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function headerOf(fetchMock: FetchMock, url: string, method: string): string | undefined {
  const init = (fetchMock.mock.calls as Array<[RequestInfo | URL, RequestInit | undefined]>).find(
    ([u, i]) => String(u) === url && (i?.method ?? 'GET').toUpperCase() === method,
  )?.[1];
  return ((init?.headers ?? {}) as Record<string, string>)['X-Admin-Token'];
}

function bodiesFor(fetchMock: FetchMock, url: string): Array<Record<string, unknown>> {
  return (fetchMock.mock.calls as Array<[RequestInfo | URL, RequestInit | undefined]>)
    .filter(([u, i]) => String(u) === url && i?.method === 'POST')
    .map(([, i]) => JSON.parse(String(i?.body)) as Record<string, unknown>);
}

const PENDING_URL = '/api/admin/feedback/review?status=pending';
const RESOLVED_URL = '/api/admin/feedback/review?status=resolved';
const DISPOSE_URL = '/api/admin/feedback/dispositions';

/** Clock-relative fixtures (29-05 lesson: fixed wall-dates drift). */
function isoAgo(ms: number): string {
  return new Date(Date.now() - ms).toISOString();
}

const PENDING_ROWS = [
  {
    chunk_id: 7,
    kb_slug: 'authority',
    report_count: 2,
    last_reported_at: isoAgo(2 * 3_600_000),
    comments: ['差旅补贴标准已调整', '第二条说明'],
  },
  {
    chunk_id: 12,
    kb_slug: 'historical',
    report_count: 1,
    last_reported_at: isoAgo(5 * 60_000),
    comments: [''],
  },
];

const DISPOSITIONS = [
  { id: 1, ts: isoAgo(26 * 3_600_000), chunk_id: 7, disposition: 'updated', note: '已重新上传 v2', actor: 'admin' },
  { id: 2, ts: isoAgo(3 * 3_600_000), chunk_id: 12, disposition: 'false_positive', note: '', actor: 'op-2' },
];

function pendingResponse(rows = PENDING_ROWS) {
  return jsonResponse({ pending: rows, total: rows.length });
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('ReviewQueuePanel — pending queue', () => {
  it('mounts, GETs pending with the admin token, and renders the chunk-aggregated rows', async () => {
    const { stop } = captureToasts();
    const fetchMock = stubFetch([{ url: PENDING_URL, respond: pendingResponse }]);
    render(<ReviewQueuePanel />);

    const row = await screen.findByTestId('review-row-7');
    expect(within(row).getByText('authority')).toBeInTheDocument();
    expect(screen.getByTestId('review-row-12')).toBeInTheDocument();
    expect(headerOf(fetchMock, PENDING_URL, 'GET')).toBe('tok-abc');
    // The verbatim FBK-05 trust banner is always visible above the table.
    const banner = screen.getByTestId('review-trust-banner');
    expect(banner).toHaveTextContent(
      '复核处置仅更新信号状态并写入审计日志，不会自动修改知识库内容；知识更新请通过「Knowledge Bases」上传流程完成。',
    );
    stop();
  });

  it('emphasizes 报告次数 ≥ 2 with the amber bold priority signal (D-29-08)', async () => {
    const { stop } = captureToasts();
    stubFetch([{ url: PENDING_URL, respond: pendingResponse }]);
    render(<ReviewQueuePanel />);

    const hot = await screen.findByTestId('review-count-7');
    expect(hot).toHaveTextContent('2');
    expect(hot).toHaveClass('text-status-needs-info');
    expect(hot).toHaveClass('font-semibold');
    // A single report stays neutral.
    const calm = screen.getByTestId('review-count-12');
    expect(calm).not.toHaveClass('font-semibold');
    expect(calm).not.toHaveClass('text-status-needs-info');
    stop();
  });

  it('renders a relative 最近报告 time from last_reported_at', async () => {
    const { stop } = captureToasts();
    stubFetch([{ url: PENDING_URL, respond: pendingResponse }]);
    render(<ReviewQueuePanel />);

    const row = await screen.findByTestId('review-row-7');
    expect(within(row).getByText(/小时前/)).toBeInTheDocument();
    const fresh = screen.getByTestId('review-row-12');
    expect(within(fresh).getByText(/分钟前/)).toBeInTheDocument();
    stop();
  });

  it('offers the three terminal dispositions with the aria-label contract', async () => {
    const { stop } = captureToasts();
    stubFetch([{ url: PENDING_URL, respond: pendingResponse }]);
    render(<ReviewQueuePanel />);

    await screen.findByTestId('review-row-7');
    for (const [id, label] of [
      ['updated', '已更新'],
      ['false_positive', '误报'],
      ['ignored', '忽略'],
    ] as const) {
      expect(screen.getByTestId(`review-dispose-7-${id}`)).toHaveAttribute(
        'aria-label',
        `将 chunk 7 处置为${label}`,
      );
    }
    stop();
  });

  it('renders the verbatim pending empty state', async () => {
    const { stop } = captureToasts();
    stubFetch([{ url: PENDING_URL, respond: () => jsonResponse({ pending: [], total: 0 }) }]);
    render(<ReviewQueuePanel />);
    expect(await screen.findByTestId('review-empty')).toHaveTextContent(
      '暂无待复核条目 — 没有新的「报告知识过时」反馈时，此列表为空',
    );
    stop();
  });

  it('renders the load-error state with the retry link and the server error verbatim', async () => {
    const { stop } = captureToasts();
    const fetchMock = stubFetch([
      { url: PENDING_URL, respond: () => jsonResponse({ error: 'admin token required' }, 403) },
    ]);
    render(<ReviewQueuePanel />);

    expect(await screen.findByTestId('review-error')).toHaveTextContent('复核队列加载失败（点击重试）');
    expect(screen.getByTestId('review-error-detail')).toHaveTextContent('admin token required');

    // Retry链 — a successful re-GET recovers the view.
    fetchMock.mockImplementation((input: RequestInfo | URL) =>
      Promise.resolve(String(input) === PENDING_URL ? pendingResponse() : jsonResponse({ error: 'x' }, 404)),
    );
    fireEvent.click(screen.getByTestId('review-retry'));
    await waitFor(() => expect(screen.getByTestId('review-row-7')).toBeInTheDocument());
    stop();
  });
});

describe('ReviewQueuePanel — expand per-report notes', () => {
  it('expands the per-report sub-row and toggles aria-expanded', async () => {
    const { stop } = captureToasts();
    stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      {
        url: '/api/knowledge/chunk/7',
        respond: () =>
          jsonResponse({ id: '7', tier: 'authority', source: 'travel.md', content: '差旅补贴上限为 400 元/天。', effectiveFrom: 0, effectiveTo: 0, version: 'v1' }),
      },
    ]);
    render(<ReviewQueuePanel />);

    const expand = await screen.findByTestId('review-expand-7');
    expect(expand).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(expand);

    const notes = await screen.findByTestId('review-comments-7');
    expect(notes).toHaveTextContent('差旅补贴标准已调整');
    expect(notes).toHaveTextContent('第二条说明');
    expect(screen.getByTestId('review-expand-7')).toHaveAttribute('aria-expanded', 'true');
    // The chunk preview rides the EXISTING read surface (zero new reads).
    expect(await screen.findByTestId('review-preview-7')).toHaveTextContent('差旅补贴上限为 400 元/天。');
    stop();
  });

  it('renders (未填写说明) for reports without a note', async () => {
    const { stop } = captureToasts();
    stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      { url: '/api/knowledge/chunk/12', respond: () => jsonResponse({ error: 'missing' }, 404) },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-expand-12'));
    const notes = await screen.findByTestId('review-comments-12');
    expect(notes).toHaveTextContent('（未填写说明）');
    // Preview miss → the verbatim fallback, never a fabricated chunk body.
    expect(await screen.findByTestId('review-preview-12')).toHaveTextContent('无法加载 chunk 内容');
    stop();
  });
});

describe('ReviewQueuePanel — disposition verb', () => {
  it('single-click POSTs {chunk_id, disposition}, refetches pending, and toasts the closed count', async () => {
    const { toasts, stop } = captureToasts();
    const fetchMock = stubFetch([
      {
        url: PENDING_URL,
        respond: (() => {
          let calls = 0;
          return () => {
            calls += 1;
            return calls > 1 ? jsonResponse({ pending: [], total: 0 }) : pendingResponse();
          };
        })(),
      },
      { url: DISPOSE_URL, method: 'POST', respond: () => jsonResponse({ closed_count: 2 }, 201) },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-dispose-7-updated'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({
      kind: 'success',
      message: '已处置 chunk #7：已更新（2 条待复核条目已关闭）',
    });
    // Body is the frozen contract — chunk_id + disposition only.
    expect(bodiesFor(fetchMock, DISPOSE_URL)).toEqual([{ chunk_id: 7, disposition: 'updated' }]);
    expect(headerOf(fetchMock, DISPOSE_URL, 'POST')).toBe('tok-abc');
    // Server-truth refetch: the second GET lands and the row leaves.
    await waitFor(() => expect(screen.getByTestId('review-empty')).toBeInTheDocument());
    stop();
  });

  it('on failure keeps the row, renders the server error verbatim, and toasts the real reason', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      { url: DISPOSE_URL, method: 'POST', respond: () => jsonResponse({ error: 'admin token required' }, 403) },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-dispose-7-ignored'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({ kind: 'error', message: '处置失败：admin token required' });
    expect(await screen.findByTestId('review-action-error-7')).toHaveTextContent('admin token required');
    // The row is NOT removed on a refused mutation.
    expect(screen.getByTestId('review-row-7')).toBeInTheDocument();
    stop();
  });

  it('disables the row disposition buttons while the POST is in flight', async () => {
    const { stop } = captureToasts();
    let resolvePost: (r: Response) => void = () => {};
    stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      {
        url: DISPOSE_URL,
        method: 'POST',
        respond: () => new Promise<Response>((resolve) => { resolvePost = resolve; }),
      },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-dispose-7-updated'));
    await waitFor(() => expect(screen.getByTestId('review-dispose-7-updated')).toBeDisabled());
    expect(screen.getByTestId('review-dispose-7-false_positive')).toBeDisabled();
    expect(screen.getByTestId('review-dispose-7-ignored')).toBeDisabled();

    resolvePost(jsonResponse({ closed_count: 1 }, 201));
    await waitFor(() => expect(screen.getByTestId('review-empty')).toBeInTheDocument());
    stop();
  });
});

describe('ReviewQueuePanel — disposition history', () => {
  it('switches to 处置历史, GETs resolved, and renders the badge/note/actor columns', async () => {
    const { stop } = captureToasts();
    const fetchMock = stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      { url: RESOLVED_URL, respond: () => jsonResponse({ dispositions: DISPOSITIONS }) },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-status-history'));

    const row = await screen.findByTestId('review-history-row-1');
    expect(within(row).getByText('已更新')).toBeInTheDocument();
    expect(within(row).getByText('已重新上传 v2')).toBeInTheDocument();
    expect(within(row).getByText('admin')).toBeInTheDocument();
    const badge = screen.getByTestId('review-badge-1');
    expect(badge).toHaveClass('text-status-compliant');
    // 误报 = red text tint; the row's own classes stay neutral.
    expect(screen.getByTestId('review-badge-2')).toHaveTextContent('误报');
    expect(screen.getByTestId('review-badge-2')).toHaveClass('text-status-non-compliant');
    expect(within(screen.getByTestId('review-history-row-2')).getByText('op-2')).toBeInTheDocument();
    expect(within(screen.getByTestId('review-history-row-2')).getByText('—')).toBeInTheDocument();
    expect(headerOf(fetchMock, RESOLVED_URL, 'GET')).toBe('tok-abc');
    stop();
  });

  it('history disposition filter refetches with the disposition query param', async () => {
    const { stop } = captureToasts();
    const fetchMock = stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      { url: RESOLVED_URL, respond: () => jsonResponse({ dispositions: DISPOSITIONS }) },
      {
        url: `${RESOLVED_URL}&disposition=ignored`,
        respond: () => jsonResponse({ dispositions: [] }),
      },
    ]);
    render(<ReviewQueuePanel />);

    fireEvent.click(await screen.findByTestId('review-status-history'));
    await screen.findByTestId('review-history-row-1');
    fireEvent.click(screen.getByTestId('review-history-filter-ignored'));

    expect(await screen.findByTestId('review-history-empty')).toBeInTheDocument();
    expect(
      (fetchMock.mock.calls as Array<[RequestInfo | URL]>).some(([u]) => String(u) === `${RESOLVED_URL}&disposition=ignored`),
    ).toBe(true);
    stop();
  });

  it('renders the verbatim history empty state', async () => {
    const { stop } = captureToasts();
    stubFetch([
      { url: PENDING_URL, respond: pendingResponse },
      { url: RESOLVED_URL, respond: () => jsonResponse({ dispositions: [] }) },
    ]);
    render(<ReviewQueuePanel />);
    fireEvent.click(await screen.findByTestId('review-status-history'));
    expect(await screen.findByTestId('review-history-empty')).toHaveTextContent('暂无处置记录');
    stop();
  });
});
