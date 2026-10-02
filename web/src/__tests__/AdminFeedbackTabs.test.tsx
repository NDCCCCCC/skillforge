// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 1 — the 7-tab admin shell (D-29-12).
//
// Pins the tab ORDER (反馈 6th, 复核 7th — verbatim per D-29-12), the
// store union extension with the 'batches' default untouched, the two
// ViewBody branches, and the /admin SfToast mount (App.tsx early-returns
// for /admin, so the admin branch owns its own 'sf-toast' viewport).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AdminDashboard from '../components/admin/AdminDashboard';
import { useAppStore } from '../store/app-store';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Tab labels in DOM order inside the admin nav. */
function tabOrder(): string[] {
  const nav = screen.getByTestId('admin-subroute-tabs');
  return Array.from(nav.querySelectorAll('button')).map((b) => b.textContent ?? '');
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
  useAppStore.setState({ adminSubroute: 'batches' });
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      // Every view the shell mounts degrades honestly on !ok — the shell
      // test only cares about tabs and mounts, not view internals.
      Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404)),
    ),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AdminDashboard — 7 tabs (29-08 D-29-12)', () => {
  it('renders all tabs in the locked order — 反馈 6th, 复核 7th (order pins updated for the Phase 38 本体, Phase 39 ECL 管线, and Phase 40 治理 tabs)', () => {
    render(<AdminDashboard />);
    expect(tabOrder()).toEqual([
      '导入批次',
      'Knowledge Bases',
      'KPI 定义',
      'Skill 管理',
      '定时任务',
      '反馈',
      '复核',
      '本体',
      'ECL 管线',
      '治理',
      '平台',
    ]);
    // The locked testids exist exactly once each.
    expect(screen.getAllByTestId('admin-tab-feedback')).toHaveLength(1);
    expect(screen.getAllByTestId('admin-tab-review')).toHaveLength(1);
  });

  it('has 11 tabs total with 平台 as the 11th entry', () => {
    render(<AdminDashboard />);
    expect(tabOrder()).toHaveLength(11);
    expect(tabOrder()[10]).toBe('平台');
  });

  it('defaults to the 批次 tab (store default untouched by the union extension)', () => {
    render(<AdminDashboard />);
    expect(useAppStore.getState().adminSubroute).toBe('batches');
    expect(screen.getByTestId('admin-tab-batches')).toHaveClass('bg-primary-dim');
  });

  it('clicking 反馈 activates it and mounts the metrics panel', async () => {
    render(<AdminDashboard />);
    fireEvent.click(screen.getByTestId('admin-tab-feedback'));
    await waitFor(() => expect(screen.getByTestId('metrics-view')).toBeInTheDocument());
    expect(useAppStore.getState().adminSubroute).toBe('feedback');
    expect(screen.getByTestId('admin-tab-feedback')).toHaveClass('bg-primary-dim');
    expect(screen.queryByTestId('review-view')).toBeNull();
  });

  it('clicking 复核 activates it and mounts the review queue panel', async () => {
    render(<AdminDashboard />);
    fireEvent.click(screen.getByTestId('admin-tab-review'));
    await waitFor(() => expect(screen.getByTestId('review-view')).toBeInTheDocument());
    expect(useAppStore.getState().adminSubroute).toBe('review');
    expect(screen.getByTestId('admin-tab-review')).toHaveClass('bg-primary-dim');
    expect(screen.queryByTestId('metrics-view')).toBeNull();
  });

  it('switching between the two new tabs unmounts the previous panel', async () => {
    render(<AdminDashboard />);
    fireEvent.click(screen.getByTestId('admin-tab-feedback'));
    await waitFor(() => expect(screen.getByTestId('metrics-view')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('admin-tab-review'));
    await waitFor(() => expect(screen.queryByTestId('metrics-view')).toBeNull());
    expect(screen.getByTestId('review-view')).toBeInTheDocument();
  });

  it('mounts the SfToast viewport inside the admin branch (toasts are visible on /admin)', async () => {
    render(<AdminDashboard />);
    // App.tsx early-returns for /admin, so this branch must own a listener.
    expect(screen.queryByTestId('sf-toast')).toBeNull();
    fireEvent(
      window,
      new CustomEvent('sf-toast', { detail: { kind: 'success', message: '已处置 chunk #7：已更新（2 条待复核条目已关闭）' } }),
    );
    expect(await screen.findByTestId('sf-toast')).toHaveTextContent(
      '已处置 chunk #7：已更新（2 条待复核条目已关闭）',
    );
  });
});
