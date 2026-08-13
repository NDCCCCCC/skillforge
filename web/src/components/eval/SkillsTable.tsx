// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-04 + SC-1) — SkillsTable.
//
// Reads indexReports from the store, groups runs by Skill, and renders
// the latest run per Skill. Columns match the SC-1 surface:
//   - Skill
//   - Faithfulness
//   - Citation precision (context_precision)
//   - Answer relevance
//   - Context recall
//   - Refusal accuracy
//   - Latest run timestamp
//   - Status badge ✅/❌ against a faithfulness threshold.
//
// Status-color tokens come from the sketch-findings skill (mirrored in
// theme.css: --color-status-compliant/non-compliant/needs-info/unanswered).
// Data-testid attributes follow the KnowledgeGraphBrowser pattern.

import { Activity } from 'lucide-react';
import { useMemo } from 'react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import { DEFAULT_FAITHFULNESS_THRESHOLD } from '../../types/eval';
import type { IndexRun } from '../../types/eval';

export interface SkillsTableProps {
  /** Optional per-Skill threshold override. Default 0.9 (D6-10). */
  thresholds?: Record<string, number>;
}

/** Pick the latest run per Skill from the index. */
function latestBySkill(runs: IndexRun[]): IndexRun[] {
  const map = new Map<string, IndexRun>();
  for (const run of runs) {
    const existing = map.get(run.skill);
    if (!existing || run.ts > existing.ts) {
      map.set(run.skill, run);
    }
  }
  return Array.from(map.values()).sort((a, b) => a.skill.localeCompare(b.skill));
}

export default function SkillsTable({ thresholds = {} }: SkillsTableProps) {
  const indexReports = useAppStore((s) => s.indexReports);
  const latest = useMemo(() => latestBySkill(indexReports), [indexReports]);

  if (latest.length === 0) {
    return (
      <div
        data-testid="eval-skills-empty"
        className="rounded-md border border-border bg-surface p-6 text-center text-sm text-text-muted"
      >
        <div className="mb-2 flex justify-center">
          <Activity className="h-5 w-5 text-text-dim" />
        </div>
        No eval runs available. Run <code className="font-mono text-xs">cmd/eval --archive</code> and refresh.
      </div>
    );
  }

  return (
    <div className="rounded-md border border-border bg-surface overflow-hidden">
      <div className="flex items-center justify-between border-b border-border bg-surface-2 px-4 py-2">
        <h2 className="text-sm font-semibold text-text">Skills</h2>
        <span className="text-xs text-text-muted" data-testid="eval-skills-count">
          {latest.length} skill{latest.length === 1 ? '' : 's'}
        </span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="bg-surface-2 text-text-muted">
            <tr>
              <th className="px-3 py-2 font-semibold">Skill</th>
              <th className="px-3 py-2 font-semibold">Faithfulness</th>
              <th className="px-3 py-2 font-semibold">Citation precision</th>
              <th className="px-3 py-2 font-semibold">Answer relevance</th>
              <th className="px-3 py-2 font-semibold">Context recall</th>
              <th className="px-3 py-2 font-semibold">Refusal accuracy</th>
              <th className="px-3 py-2 font-semibold">Latest run</th>
              <th className="px-3 py-2 font-semibold">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {latest.map((run) => {
              const threshold = thresholds[run.skill] ?? DEFAULT_FAITHFULNESS_THRESHOLD;
              const pass = run.faithfulness >= threshold;
              return (
                <tr
                  key={run.skill}
                  data-testid="eval-skills-row"
                  data-skill-id={run.skill}
                  data-pass={pass}
                  className="hover:bg-surface-2"
                >
                  <td className="px-3 py-2 font-medium text-text">{run.skill}</td>
                  <td className="px-3 py-2 font-mono text-text">{run.faithfulness.toFixed(2)}</td>
                  <td className="px-3 py-2 font-mono text-text">{run.context_precision.toFixed(2)}</td>
                  <td className="px-3 py-2 font-mono text-text">{run.answer_relevance.toFixed(2)}</td>
                  <td className="px-3 py-2 font-mono text-text">{run.context_recall.toFixed(2)}</td>
                  <td className="px-3 py-2 font-mono text-text">{run.refusal_accuracy.toFixed(2)}</td>
                  <td className="px-3 py-2 text-text-muted">{run.ts}</td>
                  <td className="px-3 py-2">
                    <span
                      data-testid={pass ? 'eval-skills-pass' : 'eval-skills-fail'}
                      className={cn(
                        'inline-flex items-center gap-1 text-xs font-semibold',
                        pass ? 'text-status-compliant' : 'text-status-non-compliant',
                      )}
                    >
                      {pass ? '✅' : '❌'}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
