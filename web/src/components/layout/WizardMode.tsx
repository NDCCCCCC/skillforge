// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardMode. The right-panel controller mounted
// when store.wizardMode.active === true. Composes:
//
//   <WizardHeader />            — Progress bar + 步骤 {N}/{total} + 取消
//   <messages area>             — the assistant's current prompt + the
//                                 user's transcript (kept in WizardMode
//                                 local state so cancel clears it)
//   <WizardPreview />           — rendered only when phase === 'previewing'
//   <ChatInput onSend={…} />    — re-uses the existing ChatInput; routes
//                                 through useWizard.submitAnswer in
//                                 wizard mode (ChatInput swap is gated
//                                 on wizardMode.active).

import { useEffect, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import { useAppStore } from '../../store/app-store';
import { useWizard } from '../../hooks/useWizard';
import ChatInput from '../chat/ChatInput';
import WizardHeader from './WizardHeader';
import WizardPreview from './WizardPreview';

interface Turn {
  id: number;
  role: 'assistant' | 'user';
  text: string;
}

let turnIdCounter = 0;

export interface WizardModeProps {
  /** Live transcript presented as MessageBubble-shaped rows so the
   * wizard feels like a chat. Cleared on cancel / commit success. */
  onCancel?(): void;
}

export default function WizardMode({ onCancel }: WizardModeProps) {
  const {
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
    hasPreview,
  } = useWizard();

  const [transcript, setTranscript] = useState<Turn[]>([]);

  const setCenterView = useAppStore((s) => s.setCenterView);

  // Push the current prompt into the transcript whenever the wizard
  // advances a step. The prompt is the assistant turn for the step the
  // user is about to answer; we keep them in order so the user sees the
  // interview unfold like a real chat.
  useEffect(() => {
    if (phase === 'answering' && currentPrompt) {
      setTranscript((prev) => {
        const last = prev[prev.length - 1];
        if (last && last.role === 'assistant' && last.text === currentPrompt) {
          return prev;
        }
        return [
          ...prev,
          { id: ++turnIdCounter, role: 'assistant', text: currentPrompt },
        ];
      });
    }
  }, [phase, currentPrompt]);

  // When a preview arrives, route the center panel to the Skill 定义 tab
  // (UI-SPEC §SC-3). The preview still renders in the right panel as
  // confirmation; the Skill 定义 tab shows the generated fields.
  useEffect(() => {
    if (phase === 'previewing' && preview?.name) {
      setCenterView({ kind: 'skill-md', skillId: preview.name });
    }
  }, [phase, preview?.name, setCenterView]);

  // Cancel tears down transcript + drops wizardMode. The hook also
  // resets its internal state on store-driven phase drop.
  function handleCancel() {
    setTranscript([]);
    cancel();
    onCancel?.();
  }

  // ChatInput submit handler — the wizard consumes every keystroke as
  // an answer for the current step.
  function handleSubmit(text: string) {
    if (busy) return;
    if (!text.trim()) return;
    setTranscript((prev) => [
      ...prev,
      { id: ++turnIdCounter, role: 'user', text },
    ]);
    submitAnswer(text);
  }

  // 生成预览 CTA: when the user has answered the last step, the wizard
  // is sitting on step === totalSteps. Surface a button to fire
  // requestPreview().
  function handleGeneratePreview() {
    setTranscript((prev) => [
      ...prev,
      {
        id: ++turnIdCounter,
        role: 'user',
        text: '生成预览',
      },
    ]);
    void requestPreview();
  }

  // Once the hook reports phase=idle (commit succeeded → wizardMode
  // cleared by the hook's commit() handler) the transcript is no
  // longer needed. We don't render anything here; the conditional is
  // a deliberate side-effect.
  if (phase === 'idle' && transcript.length > 0) {
    // schedule clear on next render
    setTimeout(() => setTranscript([]), 0);
  }

  const answeredSteps = countAnswered(answers);

  return (
    <div
      data-testid="wizard-mode"
      data-wizard-phase={phase}
      className="flex flex-col h-full bg-bg"
    >
      <WizardHeader
        currentStep={Math.min(step + (phase === 'previewing' ? 1 : 0), totalSteps)}
        totalSteps={totalSteps}
        answeredSteps={answeredSteps}
      />

      {hasPreview && preview?.preview ? (
        <WizardPreview
          preview={preview}
          answeredSteps={answeredSteps}
          totalSteps={totalSteps}
          onRegenerate={() => {
            setTranscript([]);
            regenerate();
          }}
          onCommit={() => {
            void commit();
          }}
          onCancelWizard={handleCancel}
          busy={busy}
        />
      ) : (
        <>
          <div
            data-testid="wizard-transcript"
            className="flex-1 overflow-y-auto p-3 space-y-2"
          >
            {transcript.length === 0 ? (
              <div
                data-testid="wizard-empty"
                className="flex h-full flex-col items-center justify-center text-center text-text-muted"
              >
                <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-dim mb-3">
                  <ShieldCheck className="h-5 w-5 text-primary" />
                </div>
                <div className="text-sm font-semibold text-text">Skill 向导</div>
                <p className="mt-1 text-xs text-text-muted">
                  回答 5 个问题，自动生成 SKILL.md
                </p>
              </div>
            ) : (
              transcript.map((t) => (
                <div
                  key={t.id}
                  data-testid={t.role === 'user' ? 'wizard-turn-user' : 'wizard-turn-assistant'}
                  className={
                    t.role === 'user'
                      ? 'ml-auto max-w-[80%] rounded-md bg-primary-dim px-3 py-2 text-xs text-text'
                      : 'mr-auto max-w-[80%] rounded-md bg-surface-2 px-3 py-2 text-xs text-text'
                  }
                >
                  {t.text}
                </div>
              ))
            )}

            {error && (
              <div
                data-testid="wizard-error"
                role="alert"
                className="rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-xs text-danger"
              >
                {error}
              </div>
            )}

            {phase === 'answering' && step >= totalSteps && (
              <button
                type="button"
                data-testid="wizard-generate-preview"
                onClick={handleGeneratePreview}
                disabled={busy}
                className="mt-2 w-full rounded-md bg-primary px-3 py-2 text-xs font-semibold text-primary-fg hover:bg-primary-hover disabled:opacity-50"
              >
                {busy ? '生成中…' : '生成预览'}
              </button>
            )}
          </div>
          <ChatInput
            onSend={handleSubmit}
            modeLabel="Skill 向导"
            disabled={phase !== 'answering'}
            busy={busy}
          />
        </>
      )}

      {/* Tear down on commit success — the hook already exits wizard
       * mode; we just keep the transcript cleared so the next entry
       * starts fresh. */}
    </div>
  );
}

// countAnswered — used by both WizardHeader + WizardPreview so the
// cancel-dialog body copy stays accurate ("{N}/{total} 步已回答").
function countAnswered(a: { purpose: string; reads: string[]; skillType: string; guardrails: string; temperature?: number | null }): number {
  let n = 0;
  if (a.purpose) n++;
  if (a.reads.length > 0) n++;
  if (a.skillType) n++;
  if (a.guardrails) n++;
  if (a.temperature !== undefined && a.temperature !== null) n++;
  return n;
}