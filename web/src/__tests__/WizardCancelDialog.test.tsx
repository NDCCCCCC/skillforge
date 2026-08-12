// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — WizardCancelDialog tests.
//
// Pins the SC-3 destructive-action acceptance assertions:
//   1. Renders the alert-dialog copy verbatim from 05-UI-SPEC §SC-3.
//   2. The 放弃 CTA uses --color-danger (we assert the Button variant
//      via className — `bg-danger` is the design-token mapping).
//   3. Confirms via 放弃 CTA calls onConfirm (the parent's
//      setWizardMode({active:false}) handler).
//   4. Cancels via 继续向导 CTA or Esc or backdrop click — all three
//      call onCancel.
//   5. The dialog is NOT rendered when open=false (no orphan node).

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import WizardCancelDialog from '../components/layout/WizardCancelDialog';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('WizardCancelDialog', () => {
  it('renders nothing when open is false', () => {
    render(
      <WizardCancelDialog
        open={false}
        answeredSteps={2}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.queryByTestId('wizard-cancel-dialog')).toBeNull();
  });

  it('renders the SC-3 copy verbatim when open', () => {
    render(
      <WizardCancelDialog
        open
        answeredSteps={2}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByTestId('wizard-cancel-dialog-title').textContent).toBe(
      '放弃当前向导？',
    );
    const body = screen.getByTestId('wizard-cancel-dialog-body').textContent ?? '';
    expect(body).toContain('放弃当前进度');
    expect(body).toContain('2/5 步已回答');
    expect(body).toContain('将丢失，无法恢复');
  });

  it('uses the destructive Button variant on the 放弃 CTA', () => {
    render(
      <WizardCancelDialog
        open
        answeredSteps={3}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    const confirm = screen.getByTestId('wizard-cancel-dialog-confirm');
    // Button destructive variant => bg-danger class is in the rendered
    // className (design-token mapping per theme.css).
    expect(confirm.className).toContain('bg-danger');
    expect(confirm.textContent?.trim()).toBe('放弃');
  });

  it('clicking 放弃 calls onConfirm (not onCancel)', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(
      <WizardCancelDialog
        open
        answeredSteps={3}
        totalSteps={5}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-cancel-dialog-confirm'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('clicking 继续向导 calls onCancel', () => {
    const onCancel = vi.fn();
    render(
      <WizardCancelDialog
        open
        answeredSteps={1}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-cancel-dialog-continue'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('clicking the backdrop calls onCancel', () => {
    const onCancel = vi.fn();
    render(
      <WizardCancelDialog
        open
        answeredSteps={1}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />,
    );
    fireEvent.click(screen.getByTestId('wizard-cancel-backdrop'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('pressing Escape calls onCancel', () => {
    const onCancel = vi.fn();
    render(
      <WizardCancelDialog
        open
        answeredSteps={1}
        totalSteps={5}
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />,
    );
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});