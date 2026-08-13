// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-14 + D6-16) — TrendPanel.
//
// Renders a recharts line chart of faithfulness over runs, with a gray
// dashed ReferenceLine for the frozen baseline. If the latest live run
// drifts >5% from the baseline, a role="alert" flag is surfaced.
//
// Uses the project's status-color tokens:
//   - line stroke = --color-status-compliant (green)
//   - baseline ReferenceLine = --color-text-muted (gray, dashed)
//   - drift flag = --color-status-non-compliant (red)

import { useMemo } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts';
import { AlertTriangle } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { IndexRun } from '../../types/eval';

export interface TrendPanelProps {
  /** Runs to plot. Defaults to indexReports from the store. */
  runs?: IndexRun[];
  /** Frozen baseline faithfulness. Defaults to store.frozenBaseline.baseline_faithfulness. */
  baseline?: number;
  /** Drift alert threshold in percent (default 5, per D6-14 / Pitfall 8). */
  deltaThresholdPct?: number;
}

export default function TrendPanel({
  runs,
  baseline,
  deltaThresholdPct = 5,
}: TrendPanelProps = {}) {
  const storeRuns = useAppStore((s) => s.indexReports);
  const storeBaseline = useAppStore((s) => s.frozenBaseline?.baseline_faithfulness);

  const activeRuns = runs ?? storeRuns;
  const activeBaseline = baseline ?? storeBaseline;

  const data = useMemo(
    () =>
      activeRuns.map((r) => ({
        ts: r.ts,
        faithfulness: r.faithfulness,
        context_precision: r.context_precision,
        answer_relevance: r.answer_relevance,
      })),
    [activeRuns],
  );

  const latest = activeRuns.at(-1)?.faithfulness ?? 0;
  const driftPct = activeBaseline ? Math.abs(latest - activeBaseline) * 100 : 0;
  const drifted = activeBaseline != null && driftPct > deltaThresholdPct;

  if (data.length === 0) {
    return (
      <div
        data-testid="eval-trend-empty"
        className="rounded-md border border-border bg-surface p-6 text-center text-sm text-text-muted"
      >
        No trend data available.
      </div>
    );
  }

  return (
    <div className="rounded-md border border-border bg-surface p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Faithfulness Trend</h2>
        {activeBaseline != null && (
          <span className="text-xs text-text-muted" data-testid="eval-trend-baseline">
            Frozen baseline: {activeBaseline.toFixed(2)}
          </span>
        )}
      </div>

      {drifted && (
        <div
          role="alert"
          data-testid="trend-drift-flag"
          className="mb-3 flex items-center gap-2 rounded-md border border-status-non-compliant/40 bg-surface-2 px-3 py-2 text-xs text-status-non-compliant"
        >
          <AlertTriangle className="h-4 w-4 shrink-0" />
          Live faithfulness drifted {driftPct.toFixed(1)}% from frozen baseline.
        </div>
      )}

      <div data-testid="eval-trend-chart" className="h-[300px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data}>
            <XAxis
              dataKey="ts"
              tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
              axisLine={{ stroke: 'var(--color-border)' }}
              tickLine={{ stroke: 'var(--color-border)' }}
            />
            <YAxis
              domain={[0, 1]}
              tick={{ fontSize: 10, fill: 'var(--color-text-muted)' }}
              axisLine={{ stroke: 'var(--color-border)' }}
              tickLine={{ stroke: 'var(--color-border)' }}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'var(--color-surface-2)',
                borderColor: 'var(--color-border)',
                color: 'var(--color-text)',
              }}
              itemStyle={{ color: 'var(--color-text)' }}
              labelStyle={{ color: 'var(--color-text-muted)' }}
            />
            <Line
              type="monotone"
              dataKey="faithfulness"
              stroke="var(--color-status-compliant)"
              strokeWidth={2}
              dot={{ r: 3, fill: 'var(--color-status-compliant)' }}
              activeDot={{ r: 5 }}
            />
            {activeBaseline != null && (
              <ReferenceLine
                y={activeBaseline}
                stroke="var(--color-text-muted)"
                strokeDasharray="4 4"
                label={{
                  value: 'frozen baseline',
                  fill: 'var(--color-text-muted)',
                  fontSize: 10,
                  position: 'insideTopRight',
                }}
              />
            )}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
