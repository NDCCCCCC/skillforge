// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — useWizard state machine.
//
// Drives the 5-step interview (purpose / reads / type / guardrails /
// temperature) and orchestrates POST /api/skills/wizard (preview) +
// POST /api/skills/wizard/commit (write). The state machine is the
// single source of truth for the wizard's current step + answers; the
// view layer (WizardMode / WizardHeader / WizardPreview) only reads.
//
// State transitions:
//   idle → answering (step=0..4) → previewing → committing → done
//                                ↑                  │
//                                └── regenerate ────┘
//
// cancel() from ANY state → idle + wizardMode.active = false.
//
// On successful commit, the hook fires a window-level "wizard-success"
// CustomEvent carrying {skillName, path} so the success toast in
// App.tsx can listen (the toast lives above the ThreePanel — out of
// the wizard component tree).
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAppStore } from '../store/app-store';
import { WIZARD_STEPS } from '../types/wizard';
import type {
  WizardAnswers,
  WizardCommitResponse,
  WizardPreviewResponse,
} from '../types/wizard';

// useWizard has no constructor args — it's a singleton-shaped hook that
// reads wizardMode from the global store. The store is the source of
// truth for active vs idle; the hook owns the in-flight state.

interface UseWizardReturn {
  // step is the index of the CURRENT step (0..totalSteps-1). When step
  // === totalSteps, the user has answered everything and can request a
  // preview. The `phase` discriminator is the explicit source of truth.
  phase: 'idle' | 'answering' | 'previewing' | 'committing';
  step: number;
  totalSteps: number;
  answers: WizardAnswers;
  preview?: WizardPreviewResponse;
  error?: string;
  // True while a preview/commit fetch is in flight (disables CTAs).
  busy: boolean;
  /** Current prompt copy from WIZARD_STEPS[step] (empty in idle / committing). */
  currentPrompt: string;
  /** Reset everything + drop wizardMode.active. Safe to call from any phase. */
  cancel(): void;
  /** Record an answer for the current step + advance. */
  submitAnswer(value: string): void;
  /** POST /api/skills/wizard with the collected answers. */
  requestPreview(): Promise<void>;
  /** Bounce back to answering phase so the user can re-edit. */
  regenerate(): void;
  /** POST /api/skills/wizard/commit. Fires wizard-success on resolve. */
  commit(): Promise<void>;
  /** True if a preview was returned but not yet committed. */
  hasPreview: boolean;
}

const emptyAnswers: WizardAnswers = {
  purpose: '',
  reads: [],
  skillType: '',
  guardrails: '',
  temperature: null,
};

export function useWizard(): UseWizardReturn {
  const wizardMode = useAppStore((s) => s.wizardMode);
  const setWizardMode = useAppStore((s) => s.setWizardMode);

  const [step, setStep] = useState(0);
  const [answers, setAnswers] = useState<WizardAnswers>(emptyAnswers);
  const [preview, setPreview] = useState<WizardPreviewResponse | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [phase, setPhase] = useState<UseWizardReturn['phase']>('idle');
  const [busy, setBusy] = useState(false);

  const totalSteps = WIZARD_STEPS.length;

  // Sync the store flag — when the hook starts (active=true), reset local
  // state to step 0; when the hook ends (active=false), drop local state.
  useEffect(() => {
    if (wizardMode.active && phase === 'idle') {
      setStep(0);
      setAnswers(emptyAnswers);
      setPreview(undefined);
      setError(undefined);
      setPhase('answering');
    }
    if (!wizardMode.active && phase !== 'idle') {
      setPhase('idle');
      setStep(0);
      setAnswers(emptyAnswers);
      setPreview(undefined);
      setError(undefined);
      setBusy(false);
    }
  }, [wizardMode.active, phase]);

  const currentPrompt = useMemo(() => {
    if (phase === 'answering' && step < totalSteps) {
      return WIZARD_STEPS[step].prompt;
    }
    return '';
  }, [phase, step, totalSteps]);

  const cancel = useCallback(() => {
    setWizardMode({ active: false, step: 0, totalSteps });
  }, [setWizardMode, totalSteps]);

  const submitAnswer = useCallback(
    (value: string) => {
      if (phase !== 'answering') return;
      const trimmed = value.trim();
      const currentStep = WIZARD_STEPS[step];
      if (!currentStep) return;
      // Empty answers are accepted only as "skip" for the optional
      // temperature step; for required steps (purpose, reads, type,
      // guardrails) we ignore the submit so the user can re-type.
      if (!trimmed && !currentStep.isOptional) return;
      if (trimmed) {
        setAnswers((prev) => {
          switch (currentStep.id) {
            case 'purpose':
              return { ...prev, purpose: trimmed };
            case 'reads': {
              // Accept comma- or newline-separated tier names.
              const parts = trimmed
                .split(/[,\n]/)
                .map((s) => s.trim())
                .filter(Boolean);
              return { ...prev, reads: parts };
            }
            case 'type':
              return { ...prev, skillType: trimmed.toLowerCase() };
            case 'guardrails':
              return { ...prev, guardrails: trimmed };
            case 'temperature': {
              const num = Number(trimmed);
              const next: WizardAnswers = {
                ...prev,
                temperature: Number.isFinite(num) ? num : null,
              };
              return next;
            }
            default:
              return prev;
          }
        });
      }
      setError(undefined);
      // Advance to the next step (or signal preview-readiness).
      if (step + 1 < totalSteps) {
        setStep(step + 1);
      } else {
        setStep(totalSteps); // sentinel — user can now click 生成预览
      }
    },
    [phase, step, totalSteps],
  );

  const requestPreview = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      const resp = await fetch('/api/skills/wizard', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({
          answers,
          currentStep: step,
        }),
      });
      const body = (await resp.json()) as WizardPreviewResponse;
      if (body.error) {
        setError(body.error);
        setPreview(undefined);
        setPhase('answering');
      } else if (body.preview) {
        setPreview(body);
        setPhase('previewing');
      } else {
        setError('向导生成失败：未返回 preview，请稍后重试。');
      }
    } catch (e) {
      setError(`向导生成失败：${(e as Error).message}`);
    } finally {
      setBusy(false);
    }
  }, [answers, busy, step]);

  const regenerate = useCallback(() => {
    setPreview(undefined);
    setError(undefined);
    setStep(0);
    setPhase('answering');
  }, []);

  const commit = useCallback(async () => {
    if (busy) return;
    if (!preview?.preview || !preview.name) {
      setError('预览尚未生成，无法保存。');
      return;
    }
    setBusy(true);
    setPhase('committing');
    setError(undefined);
    try {
      const resp = await fetch('/api/skills/wizard/commit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({
          name: preview.name,
          preview: preview.preview,
        }),
      });
      const body = (await resp.json()) as WizardCommitResponse;
      if (!body.ok) {
        setError(body.error || '保存失败，请稍后重试。');
        setPhase('previewing');
        return;
      }
      // Dispatch a CustomEvent so the success toast (mounted above the
      // ThreePanel) can render Skill 已创建：{skillName}。已注册并热加载。
      const detail = { skillName: preview.name, path: body.path };
      window.dispatchEvent(new CustomEvent('wizard-success', { detail }));

      // Exit wizard mode — the chat panel resets to its normal layout.
      setWizardMode({ active: false, step: 0, totalSteps });
      setPhase('idle');
    } catch (e) {
      setError(`保存失败：${(e as Error).message}`);
      setPhase('previewing');
    } finally {
      setBusy(false);
    }
  }, [busy, preview, setWizardMode, totalSteps]);

  return {
    phase,
    step,
    totalSteps,
    answers,
    preview,
    error,
    busy,
    currentPrompt,
    cancel,
    submitAnswer,
    requestPreview,
    regenerate,
    commit,
    hasPreview: Boolean(preview?.preview),
  };
}

// ----------------------------------------------------------------------------
// wizard-success CustomEvent payload — shared with App.tsx toast listener.
// ----------------------------------------------------------------------------

export interface WizardSuccessDetail {
  skillName: string;
  path: string;
}

declare global {
  interface WindowEventMap {
    'wizard-success': CustomEvent<WizardSuccessDetail>;
  }
}