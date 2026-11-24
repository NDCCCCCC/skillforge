// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- the document x step grid (Airflow Grid view
// mode; 55-UI-SPEC Component 6).
//
// SELF-CONTAINED PROPS (wave-1 isolation): cells arrive ALREADY derived
// (label/tone/pulse/dashed) from the caller, so state mapping still lives
// in exactly one place (55-03 computes them through types.ts). This
// component is a pure renderer.
//
// NOT INTERACTIVE (deliberate): the grid has zero buttons and zero
// onClick handlers. Clicking a cell to jump into a doc would be fake
// interaction density -- the per-step drill-down lives on the timeline,
// where an action actually exists (55-UI-SPEC §Honesty Rule 3).
import type { StatusTone } from '../shared/StatusBadge';
import { cn } from '../../../lib/cn';

/** Local mirror of StatusBadge's pill palette for 12px CELLS (the shared
 * map is keyed to pill surfaces). Kept local so this plan touches no
 * shared file inside wave 1. */
const CELL_TONE_CLASS: Record<StatusTone, string> = {
  green: 'bg-status-compliant',
  amber: 'bg-status-needs-info',
  red: 'bg-status-non-compliant',
  gray: 'bg-status-unanswered',
  indigo: 'bg-primary',
};

export interface MatrixCellData {
  /** Derived label (等待/进行中/失败/完成/已跳过/待重跑). */
  label: string;
  tone: StatusTone;
  pulse: boolean;
  /** skipped cells render dashed. */
  dashed?: boolean;
  /** '3.2s' or '—' -- used in the aria-label only. */
  durationLabel?: string;
}

export interface IngestStepMatrixProps {
  /** Eight column headers (full zh names, no abbreviation). */
  stepLabels: string[];
  docs: Array<{ fileName: string; cells: MatrixCellData[] }>;
  /** Documents whose CHECKED steps are all done (a count, not a state
   * inference -- the caller computes it). */
  doneDocs: number;
  totalDocs: number;
}

const LEGEND: Array<{ label: string; tone: StatusTone; pulse?: boolean; dashed?: boolean }> = [
  { label: '完成', tone: 'green' },
  { label: '进行中', tone: 'amber', pulse: true },
  { label: '失败', tone: 'red' },
  { label: '等待', tone: 'gray' },
  { label: '已跳过', tone: 'gray', dashed: true },
  { label: '待重跑', tone: 'amber' },
];

export default function IngestStepMatrix({
  stepLabels,
  docs,
  doneDocs,
  totalDocs,
}: IngestStepMatrixProps) {
  return (
    <section data-testid="ingest-step-matrix-section" className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-text">文档 × 步骤</h2>
        <span
          data-testid="ingest-matrix-counter"
          className="font-mono text-xs text-text-muted"
        >
          {doneDocs}/{totalDocs} 篇完成
        </span>
      </div>
      <div className="overflow-x-auto">
        <table data-testid="ingest-step-matrix" className="w-full">
          <thead>
            <tr className="border-b border-border text-xs text-text-muted">
              {stepLabels.map((label, i) => (
                <th
                  key={`${label}-${i}`}
                  scope="col"
                  className="whitespace-nowrap px-3 py-1.5 text-left font-semibold"
                >
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {docs.map((doc) => (
              <tr key={doc.fileName} data-testid="ingest-matrix-row">
                <td
                  title={doc.fileName}
                  className="max-w-[16rem] truncate px-3 py-1.5 font-mono text-xs text-text"
                >
                  {doc.fileName}
                </td>
                {doc.cells.map((cell, i) => {
                  const duration =
                    cell.durationLabel && cell.durationLabel !== '—'
                      ? ` · ${cell.durationLabel}`
                      : '';
                  return (
                    <td key={`${doc.fileName}-${i}`} className="px-3 py-1.5">
                      <span
                        role="img"
                        aria-label={`文档 ${doc.fileName} · ${stepLabels[i] ?? ''} · ${cell.label}${duration}`}
                        data-tone={cell.tone}
                        data-dashed={String(Boolean(cell.dashed))}
                        className={cn(
                          'inline-block h-3 w-3 rounded-sm',
                          CELL_TONE_CLASS[cell.tone],
                          cell.pulse && 'animate-pulse',
                          cell.dashed && 'border border-dashed border-border bg-surface-2',
                        )}
                      />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div
        data-testid="ingest-matrix-legend"
        aria-hidden="true"
        className="flex flex-wrap items-center gap-2 text-xs text-text-muted"
      >
        {LEGEND.map((item) => (
          <span key={item.label} className="inline-flex items-center gap-1">
            <span
              className={cn(
                'inline-block h-3 w-3 rounded-sm',
                CELL_TONE_CLASS[item.tone],
                item.pulse && 'animate-pulse',
                item.dashed && 'border border-dashed border-border bg-surface-2',
              )}
            />
            {item.label}
          </span>
        ))}
      </div>
    </section>
  );
}
