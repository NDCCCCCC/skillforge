// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- the eight-row locked pipeline timeline
// (55-UI-SPEC Component 5).
//
// Visual: one row per STEP_SEQUENCE step (the BE always ships all eight,
// so the row COUNT is stable). Each row shows the zh label, a derived
// StatusBadge, the wall-clock duration, and -- ONLY for a failed step --
// a retry button. The expanded panel is a render-prop slot so this
// component stays self-contained in wave 1 (55-03 injects
// IngestStepInspect).
//
// Honesty rules baked in here:
//   - server truth only: no local stopwatch (duration is a timestamp
//     difference that refreshes with the poll)
//   - a non-error step renders NO button (not a disabled prop)
//   - the error text is a hover title, never a silent tooltip-only fact
//   - unknown step keys render their raw label/status (unknown != blank)
import { useState } from 'react';
import type { ReactNode } from 'react';
import StatusBadge from '../shared/StatusBadge';
import { cn } from '../../../lib/cn';
import { orderSteps, deriveStepState, formatDuration, STEP_LABEL } from './types';
import type { IngestStep } from './types';

export interface IngestJobTimelineProps {
  /** The server's steps array, untouched (ordering happens here on
   * STEP_SEQUENCE -- reorder only, never insert/delete rows). */
  steps: IngestStep[];
  /** Retry handler; when absent the button still renders for failed
   * steps (the caller decides whether the verb is wired). */
  onRetry?: (step: string) => void;
  /** The step whose retry POST is in flight -> its button is disabled. */
  retryInFlight?: string | null;
  /** Expanded-panel content slot (55-03 injects IngestStepInspect). */
  renderPanel?: (step: IngestStep) => ReactNode;
}

export default function IngestJobTimeline({
  steps,
  onRetry,
  retryInFlight,
  renderPanel,
}: IngestJobTimelineProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const ordered = orderSteps(steps);

  function toggle(stepKey: string) {
    setExpanded((prev) => ({ ...prev, [stepKey]: !prev[stepKey] }));
  }

  return (
    <div
      data-testid="ingest-job-timeline"
      aria-live="polite"
      className="overflow-hidden rounded-md border border-border"
    >
      {ordered.map((step) => {
        const state = deriveStepState(step);
        const isOpen = Boolean(expanded[step.step]);
        const label = STEP_LABEL[step.step] ?? step.step;
        const isError = state.derived === 'error';
        const panelId = `ingest-step-expand-${step.step}`;
        return (
          <div
            key={step.step}
            data-testid={`ingest-step-${step.step}`}
            className="border-t border-border first:border-t-0 transition-colors hover:bg-surface-2"
          >
            <div className="flex items-center gap-2 px-3 py-1.5">
              <button
                type="button"
                aria-expanded={isOpen}
                aria-controls={panelId}
                title={step.error || undefined}
                onClick={() => toggle(step.step)}
                className="flex min-w-0 flex-1 items-center gap-2 text-left"
              >
                <span className="text-xs font-semibold text-text">{label}</span>
                <StatusBadge
                  status={step.status}
                  tone={state.tone}
                  label={state.label}
                  className={cn(state.pulse && 'animate-pulse')}
                  testId={`ingest-step-badge-${step.step}`}
                />
                <span
                  data-testid={`ingest-step-duration-${step.step}`}
                  className="ml-auto shrink-0 font-mono text-xs text-text-muted"
                >
                  {formatDuration(step.started_at, step.completed_at)}
                </span>
              </button>
              {isError && (
                <button
                  type="button"
                  data-testid={`ingest-step-retry-${step.step}`}
                  aria-label={`重试步骤 ${label}`}
                  disabled={retryInFlight === step.step}
                  onClick={() => onRetry?.(step.step)}
                  className="shrink-0 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold hover:bg-surface-3 disabled:opacity-50"
                >
                  重试此步骤
                </button>
              )}
            </div>
            {isOpen && (
              <div
                id={panelId}
                data-testid={panelId}
                className="m-3 rounded-md border border-border bg-surface-2 p-3"
              >
                {renderPanel ? renderPanel(step) : null}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
