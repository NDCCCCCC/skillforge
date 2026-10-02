// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 3 — FeedbackMetricsPanel (FBK-03 反馈仪表盘) against the
// frozen 29-05 wire contract:
//
//   GET /api/admin/feedback/metrics?dim=skill|base&days=N
//     → 200 {dim, days,
//            totals: {helpful, unhelpful, outdated, help_rate, outdated_rate,
//                     total_votes, pending_count},
//            by_key: [{key, helpful, unhelpful, outdated, help_rate, outdated_rate}],
//            trend: [{day, verdict, count}]}
//   （help_rate / outdated_rate 为 number|null；null = 0 票）
//
// Pinned here: the verbatim D-29-10 caliber note (FULL-vote denominator —
// the planner must not quietly reintroduce a two-vote denominator), the
// 0-vote 「—」 rendering (never 0% / NaN), the client-side 日/周 re-bucket
// (D-29-11 — no second request), the per-KB double-count footnote, and
// the 待复核 cross-link into the 复核 tab.
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AdminDashboard from '../components/admin/AdminDashboard';
import FeedbackMetricsPanel, { bucketWeekly } from '../components/admin/FeedbackMetricsPanel';
import { useAppStore } from '../store/app-store';

type FetchMock = ReturnType<typeof vi.fn>;

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function stubFetch(routes: Array<{ url: string; respond: () => Response }>): FetchMock {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    const hit = routes.find((r) => r.url === url);
    if (!hit) return Promise.resolve(jsonResponse({ error: 'no route in test' }, 404));
    return Promise.resolve(hit.respond());
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function headerOf(fetchMock: FetchMock, url: string): string | undefined {
  const init = (fetchMock.mock.calls as Array<[RequestInfo | URL, RequestInit | undefined]>).find(
    ([u]) => String(u) === url,
  )?.[1];
  return ((init?.headers ?? {}) as Record<string, string>)['X-Admin-Token'];
}

const SKILLS_URL = '/api/admin/feedback/metrics?dim=skill&days=7';
const BASE_URL = '/api/admin/feedback/metrics?dim=base&days=7';
const DAYS30_URL = '/api/admin/feedback/metrics?dim=skill&days=30';

const TREND = [
  { day: '2026-09-01', verdict: 'helpful', count: 1 },
  { day: '2026-09-02', verdict: 'helpful', count: 2 },
  { day: '2026-09-02', verdict: 'outdated', count: 1 },
  { day: '2026-09-03', verdict: 'unhelpful', count: 1 },
];

const METRICS_BODY = {
  dim: 'skill',
  days: 7,
  totals: {
    helpful: 2,
    unhelpful: 1,
    outdated: 1,
    help_rate: 0.5,
    outdated_rate: 0.25,
    total_votes: 4,
    pending_count: 2,
  },
  by_key: [
    { key: 'compliance', helpful: 2, unhelpful: 1, outdated: 1, help_rate: 0.5, outdated_rate: 0.25 },
    { key: 'simple-qa', helpful: 0, unhelpful: 0, outdated: 0, help_rate: null, outdated_rate: null },
  ],
  trend: TREND,
};

const ZERO_VOTE_BODY = {
  dim: 'skill',
  days: 7,
  totals: {
    helpful: 0,
    unhelpful: 0,
    outdated: 0,
    help_rate: null,
    outdated_rate: null,
    total_votes: 0,
    pending_count: 0,
  },
  by_key: [],
  trend: [],
};

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('FeedbackMetricsPanel — read surface (D-29-10)', () => {
  it('GETs dim=skill&days=7 with the admin token and renders the verbatim caliber note', async () => {
    const fetchMock = stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) }]);
    render(<FeedbackMetricsPanel />);

    const caliber = await screen.findByTestId('metrics-caliber');
    expect(caliber).toHaveTextContent(
      '口径：帮助率 = 有帮助 ÷（有帮助＋没帮助＋报告知识过时）；outdated 率 = 报告知识过时 ÷ 全票。分母不含排除项。',
    );
    expect(headerOf(fetchMock, SKILLS_URL)).toBe('tok-abc');
  });

  it('renders the 5 summary cards and the dimension table from the response', async () => {
    stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) }]);
    render(<FeedbackMetricsPanel />);

    expect(await screen.findByTestId('metrics-card-help-rate')).toHaveTextContent('50.0%');
    expect(screen.getByTestId('metrics-card-outdated-rate')).toHaveTextContent('25.0%');
    expect(screen.getByTestId('metrics-card-helpful')).toHaveTextContent('2');
    expect(screen.getByTestId('metrics-card-unhelpful')).toHaveTextContent('1');
    expect(screen.getByTestId('metrics-card-outdated')).toHaveTextContent('1');

    const row = screen.getByTestId('metrics-row-compliance');
    expect(row).toHaveTextContent('compliance');
    expect(row).toHaveTextContent('4'); // 总票数 = 2 + 1 + 1
    expect(row).toHaveTextContent('50.0%');
    // No per-KB footnote in the per-Skill dimension.
    expect(screen.queryByTestId('metrics-kb-footnote')).toBeNull();
    // The cross-link carries the server pending_count.
    expect(screen.getByTestId('metrics-crosslink')).toHaveTextContent('待复核 2 条');
  });

  it('renders a 0-vote response as 「—」 with no 0% and no NaN anywhere (null ≠ 0%)', async () => {
    stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(ZERO_VOTE_BODY) }]);
    render(<FeedbackMetricsPanel />);

    await screen.findByTestId('metrics-empty');
    expect(screen.getByTestId('metrics-empty')).toHaveTextContent(
      '暂无反馈数据 — 用户在聊天中提交反馈后，此处将展示帮助率与趋势',
    );
    expect(screen.getByTestId('metrics-card-help-rate')).toHaveTextContent('—');
    expect(screen.getByTestId('metrics-card-outdated-rate')).toHaveTextContent('—');
    const text = document.body.textContent ?? '';
    expect(text).not.toContain('0%');
    expect(text).not.toContain('NaN');
  });

  it('renders the verbatim dimension empty state when by_key is empty', async () => {
    const withVotesNoRefs = {
      ...METRICS_BODY,
      by_key: [],
    };
    stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(withVotesNoRefs) }]);
    render(<FeedbackMetricsPanel />);
    expect(await screen.findByTestId('metrics-dim-empty')).toHaveTextContent('该维度暂无数据');
  });
});

describe('FeedbackMetricsPanel — controls re-fetch', () => {
  it('switching to 按知识库 refetches with dim=base and renders the verbatim per-KB footnote', async () => {
    const fetchMock = stubFetch([
      { url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) },
      {
        url: BASE_URL,
        respond: () =>
          jsonResponse({
            ...METRICS_BODY,
            dim: 'base',
            by_key: [
              { key: 'authority', helpful: 2, unhelpful: 1, outdated: 1, help_rate: 0.5, outdated_rate: 0.25 },
            ],
          }),
      },
    ]);
    render(<FeedbackMetricsPanel />);
    await screen.findByTestId('metrics-row-compliance');

    fireEvent.click(screen.getByTestId('metrics-dim-base'));

    const row = await screen.findByTestId('metrics-row-authority');
    expect(row).toHaveTextContent('authority');
    expect(screen.getByTestId('metrics-kb-footnote')).toHaveTextContent(
      '一个回答可引用多个知识库：按知识库维度统计时，同一票在每个被引知识库各计一次；总量与按 Skill 维度每票严格计一次。',
    );
    expect(
      (fetchMock.mock.calls as Array<[RequestInfo | URL]>).some(([u]) => String(u) === BASE_URL),
    ).toBe(true);
  });

  it('switching the range to 近 30 天 refetches with days=30', async () => {
    const fetchMock = stubFetch([
      { url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) },
      { url: DAYS30_URL, respond: () => jsonResponse({ ...METRICS_BODY, days: 30 }) },
    ]);
    render(<FeedbackMetricsPanel />);
    await screen.findByTestId('metrics-row-compliance');

    fireEvent.click(screen.getByTestId('metrics-range-30'));
    await waitFor(() =>
      expect((fetchMock.mock.calls as Array<[RequestInfo | URL]>).some(([u]) => String(u) === DAYS30_URL)).toBe(true),
    );
  });
});

describe('FeedbackMetricsPanel — 日/周 trend (D-29-11)', () => {
  it('bucketWeekly merges 7 consecutive days into one bucket with per-verdict sums', () => {
    const week = [
      { day: '2026-09-01', verdict: 'helpful', count: 1 },
      { day: '2026-09-02', verdict: 'helpful', count: 2 },
      { day: '2026-09-03', verdict: 'unhelpful', count: 1 },
      { day: '2026-09-05', verdict: 'helpful', count: 4 },
      { day: '2026-09-07', verdict: 'outdated', count: 2 },
    ];
    const merged = bucketWeekly(week);
    expect(merged).toHaveLength(3);
    // One bucket labeled by its earliest day, counts summed per verdict.
    expect(merged.filter((c) => c.verdict === 'helpful')).toEqual([
      { day: '2026-09-01', verdict: 'helpful', count: 7 },
    ]);
    expect(merged.filter((c) => c.verdict === 'unhelpful')).toEqual([
      { day: '2026-09-01', verdict: 'unhelpful', count: 1 },
    ]);
    expect(merged.filter((c) => c.verdict === 'outdated')).toEqual([
      { day: '2026-09-01', verdict: 'outdated', count: 2 },
    ]);
  });

  it('bucketWeekly splits an 8-day span into two 7-day buckets anchored at the newest day', () => {
    const cells = [
      { day: '2026-09-01', verdict: 'helpful', count: 1 }, // oldest — own bucket
      { day: '2026-09-02', verdict: 'helpful', count: 1 },
      { day: '2026-09-08', verdict: 'helpful', count: 3 }, // newest
    ];
    const merged = bucketWeekly(cells);
    const helpful = merged.filter((c) => c.verdict === 'helpful');
    expect(helpful).toHaveLength(2);
    // Bucket 1 (2026-09-02..09-08) and bucket 2 (2026-08-26..09-01), chronological.
    expect(helpful.map((c) => c.day)).toEqual(['2026-09-01', '2026-09-02']);
    expect(helpful.map((c) => c.count)).toEqual([1, 4]);
  });

  it('switching to 周 re-buckets the same series client-side without a second request', async () => {
    const fetchMock = stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) }]);
    render(<FeedbackMetricsPanel />);

    const helpfulRow = await screen.findByTestId('metrics-trend-helpful');
    // Daily series spans the UNION day axis (2026-09-01..03) — the
    // helpful verdict is 0-filled on 09-03.
    expect(within(helpfulRow).getByTestId('sparkline')).toHaveAttribute(
      'aria-label',
      'sparkline with 3 points',
    );
    const callsBefore = fetchMock.mock.calls.length;

    fireEvent.click(screen.getByTestId('metrics-gran-week'));

    // Same series, re-bucketed to one 7-day bucket — and NOT re-fetched.
    // (The Phase-17 Sparkline always pluralizes "points" in its label.)
    expect(within(screen.getByTestId('metrics-trend-helpful')).getByTestId('sparkline')).toHaveAttribute(
      'aria-label',
      'sparkline with 1 points',
    );
    expect(fetchMock.mock.calls.length).toBe(callsBefore);
  });

  it('colors the three series with the compliant / non-compliant / needs-info status tokens', async () => {
    stubFetch([{ url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) }]);
    render(<FeedbackMetricsPanel />);

    const helpful = await screen.findByTestId('metrics-trend-helpful');
    const unhelpful = screen.getByTestId('metrics-trend-unhelpful');
    const outdated = screen.getByTestId('metrics-trend-outdated');
    expect(helpful.style.getPropertyValue('--color-sparkline')).toBe('var(--color-status-compliant)');
    expect(unhelpful.style.getPropertyValue('--color-sparkline')).toBe('var(--color-status-non-compliant)');
    expect(outdated.style.getPropertyValue('--color-sparkline')).toBe('var(--color-status-needs-info)');
  });
});

describe('FeedbackMetricsPanel — 待复核 cross-link', () => {
  it('clicking 待复核 {n} 条 switches the admin subroute to 复核', async () => {
    stubFetch([
      { url: SKILLS_URL, respond: () => jsonResponse(METRICS_BODY) },
      {
        url: '/api/admin/feedback/review?status=pending',
        respond: () => jsonResponse({ pending: [], total: 0 }),
      },
    ]);
    useAppStore.setState({ adminSubroute: 'feedback' });
    render(<AdminDashboard />);

    fireEvent.click(await screen.findByTestId('metrics-crosslink'));

    await waitFor(() => expect(useAppStore.getState().adminSubroute).toBe('review'));
    expect(await screen.findByTestId('review-view')).toBeInTheDocument();
  });
});

describe('FeedbackMetricsPanel — error state', () => {
  it('renders the verbatim load error with the server error and recovers on retry', async () => {
    const fetchMock = stubFetch([
      { url: SKILLS_URL, respond: () => jsonResponse({ error: 'admin token required' }, 403) },
    ]);
    render(<FeedbackMetricsPanel />);

    expect(await screen.findByTestId('metrics-error')).toHaveTextContent('反馈指标加载失败（点击重试）');
    expect(screen.getByTestId('metrics-error-detail')).toHaveTextContent('admin token required');

    fetchMock.mockImplementation((input: RequestInfo | URL) =>
      Promise.resolve(
        String(input) === SKILLS_URL ? jsonResponse(METRICS_BODY) : jsonResponse({ error: 'x' }, 404),
      ),
    );
    fireEvent.click(screen.getByTestId('metrics-error'));
    await waitFor(() => expect(screen.getByTestId('metrics-card-help-rate')).toHaveTextContent('50.0%'));
  });
});

