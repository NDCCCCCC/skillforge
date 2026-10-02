// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 3 — 「反馈」 tab (FBK-03): the feedback metrics
// dashboard, against the frozen 29-05 wire contract:
//
//   GET /api/admin/feedback/metrics?dim=skill|base&days=N
//     → 200 {dim, days,
//            totals: {helpful, unhelpful, outdated, help_rate, outdated_rate,
//                     total_votes, pending_count},
//            by_key: [{key, helpful, unhelpful, outdated, help_rate, outdated_rate}],
//            trend: [{day, verdict, count}]}
//   （help_rate / outdated_rate 为 number|null；null = 0 票）
//
// Caliber discipline (D-29-10): the rates are computed SERVER-side with
// the FULL-vote denominator (helpful + unhelpful + outdated) and null
// means "no votes yet". This panel NEVER derives a rate itself — a null
// renders 「—」, never 0% (which would read as "everyone voted
// unhelpful") and never NaN. The caliber note is always visible so the
// denominator is never a silent assumption.
//
// Per-KB caliber (29-02 Open Q2): one answer may cite several knowledge
// bases, so dim=base counts a vote once PER CITED KB — its rows can sum
// above the vote total by design, and the footnote below the table says
// so verbatim while `totals` stays the strict once-per-vote aggregate.
//
// Trend (D-29-11): the daily series from the server is re-bucketed
// CLIENT-side for the 周 view (bucketWeekly) — switching granularity
// never fires a second request. The three series reuse the existing
// Sparkline (zero new dependency) tinted through --color-sparkline with
// the three existing status tokens.
//
// Cross-link: 待复核 {pending_count} 条 jumps straight to the 复核 tab —
// the metrics panel is where an operator notices the backlog, the review
// panel is where it gets dispositioned.
//
// TRUST BOUNDARY: token'd fetch + the server decides (KnowledgeBaseList
// pattern). Zero new zustand fields — dim/days/granularity are component-
// local useState; zero new tokens/deps; all copy verbatim from
// 29-UI-SPEC §Copywriting Contract.
import { useCallback, useEffect, useState } from 'react';
import { Sparkline, type SparklinePoint } from '../chat/parts/Sparkline';
import { cn } from '../../lib/cn';
import { Button } from '../ui/button';
import { useAppStore } from '../../store/app-store';

interface MetricsTotals {
  helpful: number;
  unhelpful: number;
  outdated: number;
  help_rate: number | null;
  outdated_rate: number | null;
  total_votes: number;
  pending_count: number;
}

interface MetricsKeyRow {
  key: string;
  helpful: number;
  unhelpful: number;
  outdated: number;
  help_rate: number | null;
  outdated_rate: number | null;
}

export interface TrendCell {
  day: string;
  verdict: string;
  count: number;
}

interface MetricsResponse {
  dim: string;
  days: number;
  totals: MetricsTotals;
  by_key: MetricsKeyRow[];
  trend: TrendCell[];
}

type Dim = 'skill' | 'base';
type Range = 7 | 30;
type Granularity = 'day' | 'week';

const CALIBER_NOTE =
  '口径：帮助率 = 有帮助 ÷（有帮助＋没帮助＋报告知识过时）；outdated 率 = 报告知识过时 ÷ 全票。分母不含排除项。';

const KB_FOOTNOTE =
  '一个回答可引用多个知识库：按知识库维度统计时，同一票在每个被引知识库各计一次；总量与按 Skill 维度每票严格计一次。';

/** Trend series colors — the three existing status tokens (UI-SPEC §Color). */
const SERIES: Array<{ verdict: string; label: string; token: string }> = [
  { verdict: 'helpful', label: '有帮助', token: 'var(--color-status-compliant)' },
  { verdict: 'unhelpful', label: '没帮助', token: 'var(--color-status-non-compliant)' },
  { verdict: 'outdated', label: '报告知识过时', token: 'var(--color-status-needs-info)' },
];

const DAY_MS = 86_400_000;

/** Merge day-bucketed trend cells into consecutive 7-day buckets anchored
 *  at the NEWEST day in the series (the most recent bucket may be partial).
 *  A bucket is labeled by its earliest day. Pure function — the 周 view is
 *  a client-side re-bucket of the SAME daily series, never a new request. */
export function bucketWeekly(cells: TrendCell[]): TrendCell[] {
  if (cells.length === 0) return [];
  const days = [...new Set(cells.map((c) => c.day))].sort();
  const end = new Date(`${days[days.length - 1]}T00:00:00Z`).getTime();
  const bucketOfDay = (day: string): number =>
    Math.floor((end - new Date(`${day}T00:00:00Z`).getTime()) / (7 * DAY_MS));

  const bucketLabel = new Map<number, string>();
  const sums = new Map<string, number>();
  for (const cell of cells) {
    const idx = bucketOfDay(cell.day);
    const earliest = bucketLabel.get(idx);
    if (earliest === undefined || cell.day < earliest) bucketLabel.set(idx, cell.day);
    const key = `${idx}|${cell.verdict}`;
    sums.set(key, (sums.get(key) ?? 0) + cell.count);
  }

  return [...bucketLabel.entries()]
    .sort((a, b) => (a[1] < b[1] ? -1 : a[1] > b[1] ? 1 : 0))
    .flatMap(([idx, label]) =>
      [...sums.entries()]
        .filter(([key]) => key.startsWith(`${idx}|`))
        .map(([key, count]) => ({ day: label, verdict: key.slice(key.indexOf('|') + 1), count })),
    );
}

/** One Sparkline series: the union day axis of the (bucketed) trend, 0-
 *  filled for verdicts with no activity on a day. */
function buildSeries(cells: TrendCell[], verdict: string): SparklinePoint[] {
  const days = [...new Set(cells.map((c) => c.day))].sort();
  const countByDay = new Map(
    cells.filter((c) => c.verdict === verdict).map((c) => [c.day, c.count]),
  );
  return days.map((day, index) => ({ index, period: day, value: countByDay.get(day) ?? 0 }));
}

/** Server truth only: null (0 votes) renders 「—」 — the FE never
 *  computes a rate and never renders 0% / NaN for an empty denominator. */
function formatRate(rate: number | null): string {
  if (rate === null || rate === undefined || Number.isNaN(rate)) return '—';
  return `${(rate * 100).toFixed(1)}%`;
}

function LoadingSkeleton() {
  return (
    <div data-testid="metrics-loading" className="flex flex-col gap-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="flex h-10 items-center gap-4 rounded-md bg-surface" />
      ))}
    </div>
  );
}

function SegmentedButton({
  active,
  testId,
  label,
  onClick,
}: {
  active: boolean;
  testId: string;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={onClick}
      className={cn(
        'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
        active ? 'bg-primary-dim text-primary' : 'text-text-muted hover:bg-surface-2 hover:text-text',
      )}
    >
      {label}
    </button>
  );
}

function MetricCard({
  testId,
  label,
  value,
  display,
}: {
  testId: string;
  label: string;
  value: string;
  display?: boolean;
}) {
  return (
    <div data-testid={testId} className="rounded-md border border-border bg-surface p-4">
      <div className="text-xs text-text-muted">{label}</div>
      <div
        className={cn(
          'mt-1 font-mono font-semibold text-text',
          display ? 'text-2xl' : 'text-lg',
        )}
      >
        {value}
      </div>
    </div>
  );
}

export default function FeedbackMetricsPanel() {
  const setAdminSubroute = useAppStore((s) => s.setAdminSubroute);

  const [dim, setDim] = useState<Dim>('skill');
  const [range, setRange] = useState<Range>(7);
  const [granularity, setGranularity] = useState<Granularity>('day');
  const [data, setData] = useState<MetricsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [serverError, setServerError] = useState('');

  const loadMetrics = useCallback((nextDim: Dim, nextDays: Range) => {
    setLoading(true);
    const token = localStorage.getItem('sf_admin_token') ?? '';
    return fetch(`/api/admin/feedback/metrics?dim=${nextDim}&days=${nextDays}`, {
      headers: { 'X-Admin-Token': token },
    })
      .then(async (resp) => {
        if (!resp.ok) {
          const body = (await resp.json().catch(() => ({}))) as { error?: unknown };
          setServerError(typeof body.error === 'string' && body.error !== '' ? body.error : `HTTP ${resp.status}`);
          setError(true);
          setLoading(false);
          return;
        }
        const body = (await resp.json()) as MetricsResponse;
        setData(body);
        setError(false);
        setServerError('');
        setLoading(false);
      })
      .catch(() => {
        setServerError('');
        setError(true);
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    void loadMetrics(dim, range);
  }, [loadMetrics, dim, range]);

  const totals = data?.totals;
  const totalVotes = totals?.total_votes ?? 0;
  // 日/周 is a client-side re-bucket of the same series — granularity is
  // deliberately NOT a fetch dependency (no second request shape).
  const trendCells =
    data && granularity === 'week' ? bucketWeekly(data.trend) : (data?.trend ?? []);

  return (
    <div data-testid="metrics-view" className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold text-text">反馈指标</h2>

      {/* D-29-10 — the denominator is never a silent assumption. */}
      <div
        data-testid="metrics-caliber"
        className="rounded-md border border-border bg-surface px-3 py-2 text-xs text-text-muted"
      >
        {CALIBER_NOTE}
      </div>

      {/* Controls: 维度 / 时间范围 re-fetch; 趋势粒度 is client-side only. */}
      <div className="flex flex-wrap items-center gap-4">
        <div className="flex items-center gap-1" role="group" aria-label="统计维度">
          <SegmentedButton
            active={dim === 'skill'}
            testId="metrics-dim-skill"
            label="按 Skill"
            onClick={() => setDim('skill')}
          />
          <SegmentedButton
            active={dim === 'base'}
            testId="metrics-dim-base"
            label="按知识库"
            onClick={() => setDim('base')}
          />
        </div>
        <div className="flex items-center gap-1" role="group" aria-label="时间范围">
          <SegmentedButton
            active={range === 7}
            testId="metrics-range-7"
            label="近 7 天"
            onClick={() => setRange(7)}
          />
          <SegmentedButton
            active={range === 30}
            testId="metrics-range-30"
            label="近 30 天"
            onClick={() => setRange(30)}
          />
        </div>
        <div className="flex items-center gap-1" role="group" aria-label="趋势粒度">
          <SegmentedButton
            active={granularity === 'day'}
            testId="metrics-gran-day"
            label="日"
            onClick={() => setGranularity('day')}
          />
          <SegmentedButton
            active={granularity === 'week'}
            testId="metrics-gran-week"
            label="周"
            onClick={() => setGranularity('week')}
          />
        </div>
      </div>

      {error && (
        <div className="flex flex-col gap-1">
          <button
            type="button"
            data-testid="metrics-error"
            onClick={() => void loadMetrics(dim, range)}
            className="self-start text-sm text-danger hover:underline"
          >
            反馈指标加载失败（点击重试）
          </button>
          {serverError !== '' && (
            <div data-testid="metrics-error-detail" className="text-xs text-text-muted">
              {serverError}
            </div>
          )}
        </div>
      )}

      {loading && !error && <LoadingSkeleton />}

      {!loading && !error && totals && totalVotes === 0 && (
        <div data-testid="metrics-empty" className="rounded-md border border-border bg-surface px-3 py-3 text-sm text-text-muted">
          暂无反馈数据 — 用户在聊天中提交反馈后，此处将展示帮助率与趋势
        </div>
      )}

      {/* Summary cards — rates render 「—」 on a 0-vote denominator. */}
      {!loading && !error && totals && (
        <div className="grid grid-cols-2 gap-4 md:grid-cols-5">
          <MetricCard testId="metrics-card-help-rate" label="帮助率" value={formatRate(totals.help_rate)} display />
          <MetricCard testId="metrics-card-helpful" label="有帮助" value={String(totals.helpful)} />
          <MetricCard testId="metrics-card-unhelpful" label="没帮助" value={String(totals.unhelpful)} />
          <MetricCard testId="metrics-card-outdated" label="报告知识过时" value={String(totals.outdated)} />
          <MetricCard
            testId="metrics-card-outdated-rate"
            label="outdated 率"
            value={formatRate(totals.outdated_rate)}
            display
          />
        </div>
      )}

      {/* Trend — three Sparkline rows, one per verdict, colored with the
          existing status tokens through --color-sparkline. */}
      {!loading && !error && totals && totalVotes > 0 && (
        <div className="flex flex-col gap-3" data-testid="metrics-trend-block">
          {SERIES.map(({ verdict, label, token }) => (
            <div
              key={verdict}
              data-testid={`metrics-trend-${verdict}`}
              className="flex items-center gap-3"
              style={{ '--color-sparkline': token } as React.CSSProperties}
            >
              <span
                aria-hidden
                className="inline-block h-2 w-2 shrink-0 rounded-full"
                style={{ backgroundColor: token }}
              />
              <span className="w-24 shrink-0 text-xs text-text-muted">{label}</span>
              <Sparkline points={buildSeries(trendCells, verdict)} onPointHover={() => {}} />
            </div>
          ))}
        </div>
      )}

      {/* Dimension table — by_key serves the SELECTED dimension; totals
          always stay the strict once-per-vote aggregate (29-05). */}
      {!loading && !error && data && data.by_key.length === 0 && (
        <div data-testid="metrics-dim-empty" className="py-8 text-center text-sm text-text-muted">
          该维度暂无数据
        </div>
      )}

      {!loading && !error && data && data.by_key.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full" data-testid="metrics-table">
            <thead>
              <tr className="border-b border-border text-xs text-text-muted">
                <th scope="col" className="pb-2 text-left font-semibold">
                  {dim === 'skill' ? 'Skill' : '知识库'}
                </th>
                <th scope="col" className="pb-2 text-right font-semibold">总票数</th>
                <th scope="col" className="pb-2 text-right font-semibold">有帮助</th>
                <th scope="col" className="pb-2 text-right font-semibold">没帮助</th>
                <th scope="col" className="pb-2 text-right font-semibold">报告知识过时</th>
                <th scope="col" className="pb-2 text-right font-semibold">帮助率</th>
                <th scope="col" className="pb-2 text-right font-semibold">outdated 率</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {data.by_key.map((row) => (
                <tr key={row.key} data-testid={`metrics-row-${row.key}`} className="hover:bg-surface transition-colors">
                  <td className="py-3">
                    <code className="font-mono text-xs text-text">{row.key}</code>
                  </td>
                  <td className="py-3 text-right font-mono text-xs text-text">
                    {row.helpful + row.unhelpful + row.outdated}
                  </td>
                  <td className="py-3 text-right font-mono text-xs text-text">{row.helpful}</td>
                  <td className="py-3 text-right font-mono text-xs text-text">{row.unhelpful}</td>
                  <td className="py-3 text-right font-mono text-xs text-text">{row.outdated}</td>
                  <td className="py-3 text-right font-mono text-xs text-text">{formatRate(row.help_rate)}</td>
                  <td className="py-3 text-right font-mono text-xs text-text">{formatRate(row.outdated_rate)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Per-KB caliber footnote — the double-count is by design and is
          declared where the numbers are read. */}
      {!loading && !error && dim === 'base' && data && data.by_key.length > 0 && (
        <p data-testid="metrics-kb-footnote" className="text-xs text-text-muted">
          {KB_FOOTNOTE}
        </p>
      )}

      {/* Cross-link: the metrics view notices the backlog, the review
          view dispositions it. */}
      {!loading && !error && totals && (
        <div className="flex items-center gap-2 border-t border-border pt-3">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            data-testid="metrics-crosslink"
            onClick={() => setAdminSubroute('review')}
          >
            待复核 {totals.pending_count} 条
          </Button>
        </div>
      )}
    </div>
  );
}
