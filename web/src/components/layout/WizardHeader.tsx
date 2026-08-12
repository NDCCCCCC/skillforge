// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardHeader. Replaces the right-panel chat
// header (SessionPicker + SkillPicker) when store.wizardMode.active is
// true. Shows:
//   - A Progress bar (5 steps; currentStep = step 1..5).
//   - A 步骤 {currentStep}/{totalSteps} counter.
//   - A 取消向导 CTA that opens WizardCancelDialog (the single Phase-5
//     destructive action — guarded by alert-dialog).

import { useState } from 'react';
import { X } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import { Button } from '../ui/button';
import { cn } from '../../lib/cn';
import WizardCancelDialog from './WizardCancelDialog';

export interface WizardHeaderProps {
  /** Current 1-indexed step number (1..totalSteps). Drives the Progress bar. */
  currentStep: number;
  /** Total step count (5). */
  totalSteps: number;
  /** Number of answered steps — passed to the cancel dialog so the body
   * copy can quote the exact step count. */
  answeredSteps: number;
}

/** Pure progress fraction — guards against the input being > totalSteps
 * (preview phase passes currentStep=5, totalSteps=5 which would 1.0). */
function clampProgress(step: number, total: number): number {
  if (total <= 0) return 0;
  if (step >= total) return 100;
  return Math.round((step / total) * 100);
}

export default function WizardHeader({
  currentStep,
  totalSteps,
  answeredSteps,
}: WizardHeaderProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  const setWizardMode = useAppStore((s) => s.setWizardMode);

  const progress = clampProgress(currentStep, totalSteps);

  return (
    <>
      <div
        data-testid="wizard-header"
        className="border-b border-border px-3 py-2 flex items-center gap-3"
      >
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 text-xs font-semibold text-text">
            <span data-testid="wizard-header-title">Skill 向导</span>
            <span
              data-testid="wizard-header-step"
              data-step={currentStep}
              data-total={totalSteps}
              className="text-text-muted"
            >
              · 步骤 {currentStep}/{totalSteps}
            </span>
          </div>
          <div
            data-testid="wizard-header-progress"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progress}
            className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-surface-2"
          >
            <div
              data-testid="wizard-header-progress-fill"
              className={cn('h-full bg-primary transition-all duration-200')}
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="wizard-cancel-trigger"
          onClick={() => setCancelOpen(true)}
          aria-label="取消向导"
          className="gap-1"
        >
          <X className="h-3.5 w-3.5" />
          <span>取消向导</span>
        </Button>
      </div>

      <WizardCancelDialog
        open={cancelOpen}
        answeredSteps={answeredSteps}
        totalSteps={totalSteps}
        onConfirm={() => {
          // Per UI-SPEC §SC-3: confirm clears the interview + exits
          // wizard mode. The useWizard hook's effect watches
          // wizardMode.active and tears down local state.
          setCancelOpen(false);
          setWizardMode({ active: false, step: 0, totalSteps });
        }}
        onCancel={() => setCancelOpen(false)}
      />
    </>
  );
}