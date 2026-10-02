// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardPreview. Renders the generated SKILL.md
// preview returned by POST /api/skills/wizard. The preview is a 7-field
// Manifest that has already passed skill.Validate BE-side; the FE
// renders the 7 fields verbatim (no transformation) plus the three
// confirmation CTAs (重新生成 / 确认并保存到 skills/{name}/SKILL.md /
// 取消向导).
//
// This component is rendered INSIDE the right chat panel — it replaces
// the messages area when the wizard reaches the preview phase. The
// center panel is routed to the Skill 定义 tab by the same useWizard
// effect that flips to phase=previewing.

import { Check, RefreshCcw, X } from 'lucide-react';
import type { WizardAnswers, WizardPreviewResponse } from '../../types/wizard';
import { Button } from '../ui/button';
import WizardCancelDialog from './WizardCancelDialog';
import { useState } from 'react';

export interface WizardPreviewProps {
  /** The full server response (carries the embedded preview + derived name). */
  preview: WizardPreviewResponse;
  /** Total answered steps (for the cancel-dialog body). */
  answeredSteps: number;
  totalSteps: number;
  /** Hook callbacks — wired by WizardMode via useWizard(). */
  onRegenerate(): void;
  onCommit(): void;
  onCancelWizard(): void;
  busy: boolean;
}

export default function WizardPreview({
  preview,
  answeredSteps,
  totalSteps,
  onRegenerate,
  onCommit,
  onCancelWizard,
  busy,
}: WizardPreviewProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  const m = preview.preview!;
  const prohibitionList = m.guardrails.prohibitions.join(', ');
  const refusalPhrases = m.guardrails.required_refusal_phrases.join(', ');
  const tierLimits = m.guardrails.tier_limits ?? {};
  const tierLimitsEntries = Object.entries(tierLimits);
  const writes = m.writes?.length ? m.writes.join(', ') : '（空）';

  return (
    <div data-testid="wizard-preview" className="flex h-full flex-col gap-3 p-3 text-text">
      <div className="rounded-md border border-border bg-surface-2 p-3">
        <div className="flex items-center gap-2 text-xs font-semibold text-text">
          <span data-testid="wizard-preview-name">{preview.name}</span>
          <span className="text-text-muted">/</span>
          <span className="font-mono text-text-muted">SKILL.md</span>
        </div>
        <p className="mt-1 text-xs text-text-muted">{m.description}</p>
      </div>

      <div
        data-testid="wizard-preview-fields"
        className="flex-1 overflow-y-auto rounded-md border border-border bg-bg p-3 font-mono text-xs leading-relaxed"
      >
        <Field k="agent" v={m.agent} />
        <Field k="reads" v={m.reads.join(', ')} />
        <Field k="writes" v={writes} />
        <Field k="temperature" v={String(m.temperature ?? '0.0')} />

        <div className="mt-2">
          <span className="text-text-dim">steps:</span>
          <ol className="mt-1 list-decimal list-inside space-y-0.5 text-text">
            {m.steps
              .split('\n')
              .map((s) => s.trim())
              .filter(Boolean)
              .map((s, i) => (
                <li key={i}>{s}</li>
              ))}
          </ol>
        </div>

        <div className="mt-2">
          <span className="text-text-dim">prohibitions:</span>{' '}
          <span data-testid="wizard-preview-prohibitions">
            [{prohibitionList}]
          </span>
        </div>
        <div className="mt-1">
          <span className="text-text-dim">required_refusal_phrases:</span>{' '}
          [{refusalPhrases}]
        </div>
        {tierLimitsEntries.length > 0 && (
          <div className="mt-1">
            <span className="text-text-dim">tier_limits:</span>
            <ul className="mt-1 list-disc list-inside space-y-0.5 text-text">
              {tierLimitsEntries.map(([tier, ops]) => (
                <li key={tier}>
                  <span className="font-mono">{tier}</span>: {ops.join(', ')}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      <div
        data-testid="wizard-preview-actions"
        className="flex flex-wrap items-center gap-2"
      >
        <Button
          type="button"
          variant="secondary"
          size="sm"
          data-testid="wizard-preview-regenerate"
          onClick={onRegenerate}
          disabled={busy}
        >
          <RefreshCcw className="h-3.5 w-3.5" />
          <span>重新生成</span>
        </Button>
        <Button
          type="button"
          variant="default"
          size="sm"
          data-testid="wizard-preview-confirm"
          onClick={onCommit}
          disabled={busy}
        >
          <Check className="h-3.5 w-3.5" />
          <span>确认并保存到 skills/{preview.name}/SKILL.md</span>
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          data-testid="wizard-preview-cancel"
          onClick={() => setCancelOpen(true)}
          disabled={busy}
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
          setCancelOpen(false);
          onCancelWizard();
        }}
        onCancel={() => setCancelOpen(false)}
      />
    </div>
  );
}

function Field({ k, v }: { k: string; v: string }) {
  return (
    <div className="mt-2">
      <span className="text-text-dim">{k}:</span>{' '}
      <span data-testid={`wizard-preview-${k}`} className="text-text">
        {v}
      </span>
    </div>
  );
}

// Re-exported so consumers don't reach into types/wizard.ts redundantly.
export type { WizardAnswers };