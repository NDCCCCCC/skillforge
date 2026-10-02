// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardCancelDialog. The SINGLE destructive
// action in Phase 5 (UI-SPEC §SC-3). Hand-rolled alert-dialog primitive
// (shadcn init runs as a Wave-1 task; until then we mirror the
// shadcn alert-dialog API so the call site doesn't change when we swap
// in the real primitive).
//
// Copy is verbatim from UI-SPEC:
//   title:   放弃当前向导？
//   body:    放弃当前进度？当前进度（{N}/{total} 步已回答）将丢失，无法恢复。
//   confirm: 放弃            (rendered in --color-danger per the variant)
//   cancel:  继续向导

import { useEffect, useRef } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '../ui/button';
import { cn } from '../../lib/cn';

export interface WizardCancelDialogProps {
  open: boolean;
  /** Number of interview steps the user has already answered (1..total). */
  answeredSteps: number;
  totalSteps: number;
  /** Called when the user clicks 放弃 — the parent decides how to tear
   * down the wizard (typically setWizardMode({active:false})). */
  onConfirm(): void;
  /** Called when the user clicks 继续向导 OR dismisses via backdrop/Esc. */
  onCancel(): void;
}

export default function WizardCancelDialog({
  open,
  answeredSteps,
  totalSteps,
  onConfirm,
  onCancel,
}: WizardCancelDialogProps) {
  // Esc closes the dialog (standard alert-dialog behavior).
  useEffect(() => {
    if (!open) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') onCancel();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [open, onCancel]);

  // Focus the cancel button on open so Enter dismisses by default
  // (avoid accidental confirmation by Enter).
  const cancelRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (open) cancelRef.current?.focus();
  }, [open]);

  if (!open) return null;

  return (
    <div
      data-testid="wizard-cancel-dialog"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="wizard-cancel-dialog-title"
      aria-describedby="wizard-cancel-dialog-body"
      className="fixed inset-0 z-50 flex items-center justify-center"
    >
      {/* Backdrop */}
      <button
        type="button"
        aria-label="关闭"
        data-testid="wizard-cancel-backdrop"
        onClick={onCancel}
        className="absolute inset-0 bg-black/50"
      />
      {/* Panel */}
      <div
        className={cn(
          'relative z-10 w-[min(420px,calc(100vw-32px))] rounded-lg border border-border bg-surface-2 p-5 shadow-xl',
        )}
      >
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-danger/10">
            <AlertTriangle className="h-5 w-5 text-danger" />
          </div>
          <div className="min-w-0 flex-1">
            <h2
              id="wizard-cancel-dialog-title"
              data-testid="wizard-cancel-dialog-title"
              className="text-sm font-semibold text-text"
            >
              放弃当前向导？
            </h2>
            <p
              id="wizard-cancel-dialog-body"
              data-testid="wizard-cancel-dialog-body"
              className="mt-2 text-xs leading-relaxed text-text-muted"
            >
              放弃当前进度？当前进度（{answeredSteps}/{totalSteps} 步已回答）将丢失，无法恢复。
            </p>
          </div>
        </div>
        <div className="mt-5 flex items-center justify-end gap-2">
          <Button
            ref={cancelRef}
            type="button"
            variant="secondary"
            size="sm"
            data-testid="wizard-cancel-dialog-continue"
            onClick={onCancel}
          >
            继续向导
          </Button>
          <Button
            type="button"
            variant="destructive"
            size="sm"
            data-testid="wizard-cancel-dialog-confirm"
            onClick={onConfirm}
          >
            放弃
          </Button>
        </div>
      </div>
    </div>
  );
}