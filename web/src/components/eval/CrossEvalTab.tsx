// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-14 + Pitfall 8) — CrossEvalTab.
//
// Reads indexReports from the store, groups by Skill + judge_model, and
// renders a table comparing the latest faithfulness per judge. Flags
// any Skill whose faithfulness delta across judges exceeds 5%.
//
// This is the dashboard landing point for the JUDGE_MODEL cross-eval
// slot documented in Phase 3 and locked in D6-14. The computation is
// intentionally a thin view over the already-archived per-judge reports.

import { useMemo } from 'react';
import { GitCompare } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { IndexRun } from '../../types/eval';

const DEFAULT_DELTA_PCT = 5;

interface CrossEvalRow {
  skill: string;
  /** judge_model → latest faithfulness for that skill */
  byJudge: Record<string, number>;
  judges: string[];
  deltaPct: number;
  flagged: boolean;
}

/** For each Skill, collect the latest run per judge model. */
function buildCrossEvalRows(runs: IndexRun[]): CrossEvalRow[] {
  const bySkill = new Map<string, Map<string, IndexRun>>();
  for (const run of runs) {
    const judges = bySkill.get(run.skill) ?? new Map<string, IndexRun>();
    const existing = judges.get(run.judge_model);
    if (!existing || run.ts > existing.ts) {
      judges.set(run.judge_model, run);
    }
    bySkill.set(run.skill, judges);
  }

  const rows: CrossEvalRow[] = [];
  for (const [skill, judges] of bySkill) {
    const sorted = Array.from(judges.entries()).sort((a, b) => a[0].localeCompare(b[0]));
    const byJudge: Record<string, number> = {};
    const values: number[] = [];
    for (const [judge, run] of sorted) {
      byJudge[judge] = run.faithfulness;
      values.push(run.faithfulness);
    }
    const max = Math.max(...values);
    const min = Math.min(...values);
    const deltaPct = (max - min) * 100;
    rows.push({
      skill,
      byJudge,
      judges: sorted.map(([j]) => j),
      deltaPct,
      flagged: values.length > 1 && deltaPct > DEFAULT_DELTA_PCT,
    });
  }
  return rows.sort((a, b) => a.skill.localeCompare(b.skill));
}

export default function CrossEvalTab() {
  const indexReports = useAppStore((s) => s.indexReports);
  const rows = useMemo(() => buildCrossEvalRows(indexReports), [indexReports]);

  if (rows.length === 0) {
    return (
      <div
        data-testid="eval-cross-eval-empty"
        className="rounded-md border border-border bg-surface p-6 text-center text-sm text-text-muted"
      >
        No cross-eval data. Run eval with two judge models via
        <code className="font-mono text-xs">--judge-models a,b</code>.
      </div>
    );
  }

  const singleJudge = rows.some((r) => r.judges.length === 1);

  return (
    <div className="rounded-md border border-border bg-surface overflow-hidden">
      <div className="flex items-center justify-between border-b border-border bg-surface-2 px-4 py-2">
        <div className="flex items-center gap-2">
          <GitCompare className="h-4 w-4 text-primary" />
          <h2 className="text-sm font-semibold text-text">Cross-Eval Bias Check</h2>
        </div>
        <span className="text-xs text-text-muted">&gt;{DEFAULT_DELTA_PCT}% delta flagged</span>
      </div>

      {singleJudge && (
        <div className="border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-muted">
          Only one judge model is represented in the archive. Cross-eval needs
          runs with at least two different judge models to surface bias.
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="bg-surface-2 text-text-muted">
            <tr>
              <th className="px-3 py-2 font-semibold">Skill</th>
              {rows[0]?.judges.map((judge) => (
                <th key={judge} className="px-3 py-2 font-semibold">
                  {judge}
                </th>
              ))}
              <th className="px-3 py-2 font-semibold">Δ %</th>
              <th className="px-3 py-2 font-semibold">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rows.map((row) => (
              <tr
                key={row.skill}
                data-testid="eval-cross-eval-row"
                data-skill-id={row.skill}
                data-flagged={row.flagged}
                className="hover:bg-surface-2"
              >
                <td className="px-3 py-2 font-medium text-text">{row.skill}</td>
                {row.judges.map((judge) => (
                  <td key={judge} className="px-3 py-2 font-mono text-text">
                    {row.byJudge[judge]?.toFixed(2) ?? '—'}
                  </td>
                ))}
                <td className="px-3 py-2 font-mono text-text">
                  {row.judges.length > 1 ? `${row.deltaPct.toFixed(1)}%` : '—'}
                </td>
                <td className="px-3 py-2">
                  {row.flagged ? (
                    <span
                      data-testid="eval-cross-eval-flag"
                      className="inline-flex items-center gap-1 text-xs font-semibold text-status-non-compliant"
                    >
                      ❌ &gt;{DEFAULT_DELTA_PCT}% drift
                    </span>
                  ) : row.judges.length > 1 ? (
                    <span className="inline-flex items-center gap-1 text-xs font-semibold text-status-compliant">
                      ✅ within {DEFAULT_DELTA_PCT}%
                    </span>
                  ) : (
                    <span className="text-xs text-text-muted">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
