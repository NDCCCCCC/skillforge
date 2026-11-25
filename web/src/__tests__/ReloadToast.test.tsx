// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — ReloadToast tests.
//
// Pins the SC-2 acceptance assertions for the toast surface:
//   1. Renders nothing when skillReloadStatus is empty.
//   2. Renders a success toast for status='ok' entries.
//   3. Renders a failure toast for status='failed' entries (incl. body).
//   4. Clicking the dismiss button clears the entry from the store.
//   5. Strips absolute filesystem paths in validation errors
//      (T-5-14 — defense-in-depth).
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ReloadToast from '../components/layout/ReloadToast';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);
beforeEach(() => {
  useAppStore.setState({ skillReloadStatus: {} });
});

describe('ReloadToast', () => {
  it('renders nothing when skillReloadStatus is empty', () => {
    const { container } = render(<ReloadToast />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders a success toast for status="ok" entries', () => {
    useAppStore.setState({
      skillReloadStatus: {
        'compliance-review': {
          status: 'ok',
          version: 'new',
          ts: Date.now(),
          skillName: '企业合规/制度审查',
        },
      },
    });
    render(<ReloadToast />);
    expect(screen.getByTestId('reload-toast-success')).toBeInTheDocument();
    expect(screen.getByTestId('reload-toast-success')).toHaveTextContent('Skill 已重载：企业合规/制度审查');
  });

  it('renders a failure toast for status="failed" entries with body', () => {
    useAppStore.setState({
      skillReloadStatus: {
        'compliance-review': {
          status: 'failed',
          version: 'old-kept',
          error: '缺字段 temperature',
          ts: Date.now(),
          skillName: '企业合规/制度审查',
        },
      },
    });
    render(<ReloadToast />);
    expect(screen.getByTestId('reload-toast-failed')).toBeInTheDocument();
    expect(screen.getByTestId('reload-toast-failed')).toHaveTextContent('Skill 加载失败，已保留旧版本：企业合规/制度审查');
    expect(screen.getByTestId('reload-toast-failed')).toHaveTextContent('原因：缺字段 temperature');
  });

  it('clicking the dismiss button clears the entry from the store', () => {
    useAppStore.setState({
      skillReloadStatus: {
        'compliance-review': {
          status: 'failed',
          version: 'old-kept',
          error: 'err',
          ts: Date.now(),
          skillName: 'compliance-review',
        },
      },
    });
    render(<ReloadToast />);
    fireEvent.click(screen.getByTestId('reload-toast-dismiss'));
    expect(useAppStore.getState().skillReloadStatus['compliance-review']).toBeUndefined();
  });

  it('strips absolute filesystem paths from validation errors (T-5-14)', () => {
    useAppStore.setState({
      skillReloadStatus: {
        'compliance-review': {
          status: 'failed',
          version: 'old-kept',
          error: '/etc/secrets/skills/compliance/SKILL.md: missing temperature',
          ts: Date.now(),
          skillName: 'compliance-review',
        },
      },
    });
    render(<ReloadToast />);
    expect(screen.getByTestId('reload-toast-failed')).toHaveTextContent('SKILL.md');
    // The full absolute path must NOT leak.
    expect(screen.getByTestId('reload-toast-failed')).not.toHaveTextContent('/etc/secrets/');
  });

  // Quick 261125-mk7 — regression for the stacked-toast hang: the old
  // parent-level timers array got cleared when a second reload entry
  // arrived within the 4s window and the ts watermark skipped re-arming,
  // so success toasts never dismissed. Both toasts MUST be gone at 4s.
  it('auto-dismisses BOTH stacked success toasts after 4s (261125-mk7)', () => {
    vi.useFakeTimers();
    try {
      const t0 = Date.now();
      useAppStore.setState({
        skillReloadStatus: {
          'simple-qa': { status: 'ok', version: 'v1', ts: t0, skillName: 'simple-qa' },
        },
      });
      render(<ReloadToast />);
      expect(screen.getByTestId('reload-toast-success')).toBeInTheDocument();

      // Second skill reloads 1s later — the live stacking scenario.
      vi.advanceTimersByTime(1000);
      act(() => {
        useAppStore.setState({
          skillReloadStatus: {
            'simple-qa': { status: 'ok', version: 'v1', ts: t0, skillName: 'simple-qa' },
            'sql-guides': { status: 'ok', version: 'v2', ts: t0 + 1000, skillName: 'sql-guides' },
          },
        });
      });
      expect(screen.getAllByTestId('reload-toast-success')).toHaveLength(2);

      act(() => {
        vi.advanceTimersByTime(4000);
      });
      expect(screen.queryByTestId('reload-toast-success')).toBeNull();
      expect(useAppStore.getState().skillReloadStatus).toEqual({});
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps failure toasts past 4s; only the X dismisses them', () => {
    vi.useFakeTimers();
    try {
      useAppStore.setState({
        skillReloadStatus: {
          'compliance-review': {
            status: 'failed',
            version: 'old-kept',
            error: 'err',
            ts: Date.now(),
            skillName: 'compliance-review',
          },
        },
      });
      render(<ReloadToast />);
      vi.advanceTimersByTime(4000);
      expect(screen.getByTestId('reload-toast-failed')).toBeInTheDocument();
      fireEvent.click(screen.getByTestId('reload-toast-dismiss'));
      expect(screen.queryByTestId('reload-toast-failed')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
});
