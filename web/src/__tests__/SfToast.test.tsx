// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-07 Task 1 — SfToast global toast viewport tests.
//
// Before this phase the 'sf-toast' CustomEvent had DISPATCHERS but NO
// LISTENER anywhere in the app (FeedbackRow fired toasts into the void —
// the placeholder this plan replaces; SessionDrawer still fires real
// error toasts the user never saw). SfToast owns BOTH the window
// listener and the viewport:
//
//   1. A dispatched CustomEvent('sf-toast') renders its message with
//      role="status" aria-live="polite" (UI-SPEC §Surface 4).
//   2. Lenient kind mapping — 'error' → error, 'success'/'compliant' →
//      success, anything else (missing / unknown legacy kinds) → info —
//      so SessionDrawer's existing {kind:'error'} dispatch works verbatim.
//   3. Auto-dismiss 4000ms (success/info) / 6000ms (error).
//   4. Single slot, latest-wins: a new message REPLACES the old one and
//      RESETS the dismiss timer.
//   5. data-testid="sf-toast-close" (aria-label 关闭) closes immediately.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SfToast from '../components/layout/SfToast';

function dispatchToast(detail: { kind?: string; message: string }) {
  act(() => {
    window.dispatchEvent(new CustomEvent('sf-toast', { detail }));
  });
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('SfToast', () => {
  it('renders nothing until a sf-toast event arrives', () => {
    render(<SfToast />);
    expect(screen.queryByTestId('sf-toast')).toBeNull();
  });

  it('renders the dispatched message with role=status aria-live=polite', () => {
    render(<SfToast />);
    dispatchToast({ kind: 'success', message: '已记录反馈：有帮助' });
    const toast = screen.getByTestId('sf-toast');
    expect(toast).toHaveTextContent('已记录反馈：有帮助');
    expect(toast).toHaveAttribute('role', 'status');
    expect(toast).toHaveAttribute('aria-live', 'polite');
  });

  it('maps kinds to variant classes (success / error / info)', () => {
    render(<SfToast />);

    dispatchToast({ kind: 'success', message: 's' });
    expect(screen.getByTestId('sf-toast').className).toContain('border-status-compliant/40');
    expect(screen.getByTestId('sf-toast').className).toContain('text-status-compliant');

    dispatchToast({ kind: 'error', message: 'e' });
    expect(screen.getByTestId('sf-toast').className).toContain('border-status-non-compliant/40');
    expect(screen.getByTestId('sf-toast').className).toContain('text-status-non-compliant');

    dispatchToast({ kind: 'info', message: 'i' });
    expect(screen.getByTestId('sf-toast').className).toContain('border-border');
    expect(screen.getByTestId('sf-toast').className).toContain('text-text');
  });

  it('maps legacy kinds leniently: compliant → success, unknown/missing → info', () => {
    render(<SfToast />);

    // Legacy FeedbackRow placeholder dispatched raw Vote values as kind.
    dispatchToast({ kind: 'compliant', message: 'legacy-compliant' });
    expect(screen.getByTestId('sf-toast').className).toContain('text-status-compliant');

    dispatchToast({ kind: 'some-unknown-kind', message: 'unknown' });
    expect(screen.getByTestId('sf-toast').className).toContain('border-border');

    dispatchToast({ message: 'no-kind-at-all' });
    expect(screen.getByTestId('sf-toast').className).toContain('border-border');
  });

  it('auto-dismisses success/info after 4000ms', () => {
    render(<SfToast />);
    dispatchToast({ kind: 'success', message: 'X' });
    act(() => {
      vi.advanceTimersByTime(3999);
    });
    expect(screen.getByTestId('sf-toast')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByTestId('sf-toast')).toBeNull();
  });

  it('keeps error toasts for 6000ms', () => {
    render(<SfToast />);
    dispatchToast({ kind: 'error', message: '反馈提交失败：HTTP 500，请稍后重试' });
    act(() => {
      vi.advanceTimersByTime(4000);
    });
    // A success toast would be gone here — the error variant persists.
    expect(screen.getByTestId('sf-toast')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(1999);
    });
    expect(screen.getByTestId('sf-toast')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByTestId('sf-toast')).toBeNull();
  });

  it('is a single latest-wins slot: a new message replaces the old and resets the timer', () => {
    render(<SfToast />);
    dispatchToast({ kind: 'success', message: '第一条' });
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    dispatchToast({ kind: 'success', message: '第二条' });
    // Old message is replaced, not stacked.
    expect(screen.getByTestId('sf-toast')).toHaveTextContent('第二条');
    expect(screen.queryByText('第一条')).toBeNull();
    // 3s after the SECOND message (6s after the first): the timer was
    // reset by the replacement, so the slot is still visible.
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByTestId('sf-toast')).toHaveTextContent('第二条');
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.queryByTestId('sf-toast')).toBeNull();
  });

  it('closes immediately via the close button (aria-label 关闭)', () => {
    render(<SfToast />);
    dispatchToast({ kind: 'error', message: '重命名失败，已恢复原名称' });
    const close = screen.getByTestId('sf-toast-close');
    expect(close).toHaveAttribute('aria-label', '关闭');
    fireEvent.click(close);
    expect(screen.queryByTestId('sf-toast')).toBeNull();
  });
});
