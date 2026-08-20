// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 2 — PlanReplayView vitest suite (8 tests, PLAN-05
// 规划轨迹回放视图 surface).
//
//   TestPlanReplayViewFetchOnMount        — fetch on mount + render
//   TestPlanReplayViewEmptyState          — empty todos → empty message
//   TestPlanReplayViewStatusTransitions   — status icons per status
//   TestPlanReplayViewSpeedControl        — speed dropdown updates ms
//   TestPlanReplayViewPlayPause           — Play toggles animation
//   TestPlanReplayViewFetchError          — error state + retry
//   TestPlanReplayViewRunIdChange         — prop change refetches
//   TestPlanReplayViewLifecycleFromAudit  — chronological order preserved
//
// Fetch is stubbed globally; the response mirrors the BE shape
// (internal/server/plan_api.go planGetResponse).

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { PlanReplayView } from './PlanReplayView';

function makeWireResponse(opts: { todos?: unknown[]; runId?: string; generatedAt?: number } = {}) {
  const todos = opts.todos ?? [
    { id: 't-1', content: '第一步', status: 'pending' },
    { id: 't-2', content: '第二步', status: 'in_progress' },
    { id: 't-3', content: '第三步', status: 'completed' },
  ];
  return {
    runId: opts.runId ?? 'run-1',
    todos,
    generatedAt: opts.generatedAt ?? 1_700_000_000,
    count: todos.length,
  };
}

function mockFetchPlan(runId: string, body: unknown) {
  const mock = vi.fn(async (url: string) => {
    if (typeof url === 'string' && url.includes(`/api/run/${runId}/plan`)) {
      return {
        ok: true,
        status: 200,
        json: async () => body,
      } as Response;
    }
    return { ok: false, status: 404, json: async () => ({}) } as Response;
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

beforeEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('PlanReplayView (Plan 12-07)', () => {
  it('TestPlanReplayViewFetchOnMount', async () => {
    const mock = mockFetchPlan('run-1', makeWireResponse());
    render(<PlanReplayView runId="run-1" />);

    // Wait for the fetch + render.
    const list = await screen.findByTestId('plan-replay-list');
    expect(list).toBeInTheDocument();

    // 3 rows.
    const rows = screen.getAllByTestId('plan-replay-row');
    expect(rows).toHaveLength(3);

    // Fetch URL matches the contract.
    const called = mock.mock.calls.some(
      ([url]) => typeof url === 'string' && url === '/api/run/run-1/plan',
    );
    expect(called).toBe(true);
  });

  it('TestPlanReplayViewEmptyState', async () => {
    mockFetchPlan('run-empty', makeWireResponse({ todos: [], runId: 'run-empty' }));
    render(<PlanReplayView runId="run-empty" />);
    expect(await screen.findByTestId('plan-replay-empty')).toHaveTextContent('暂无规划轨迹');
  });

  it('TestPlanReplayViewStatusTransitions', async () => {
    mockFetchPlan(
      'run-status',
      makeWireResponse({
        runId: 'run-status',
        todos: [
          { id: 't-1', content: '审核', status: 'pending' },
          { id: 't-2', content: '验证', status: 'in_progress' },
          { id: 't-3', content: '生成', status: 'completed' },
        ],
      }),
    );
    render(<PlanReplayView runId="run-status" />);
    const list = await screen.findByTestId('plan-replay-list');
    expect(list).toBeInTheDocument();

    // Each row carries its own data-status + matching status icon.
    expect(screen.getAllByTestId('plan-replay-row')[0]).toHaveAttribute('data-status', 'pending');
    expect(screen.getAllByTestId('plan-replay-row')[1]).toHaveAttribute('data-status', 'in_progress');
    expect(screen.getAllByTestId('plan-replay-row')[2]).toHaveAttribute('data-status', 'completed');

    // All three status icons are rendered (one per row).
    expect(screen.getAllByTestId('replay-status-icon-pending').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByTestId('replay-status-icon-in_progress').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByTestId('replay-status-icon-completed').length).toBeGreaterThanOrEqual(1);
  });

  it('TestPlanReplayViewSpeedControl', async () => {
    mockFetchPlan('run-speed', makeWireResponse({ runId: 'run-speed' }));
    render(<PlanReplayView runId="run-speed" />);
    await screen.findByTestId('plan-replay-list');

    const select = screen.getByTestId('plan-replay-speed') as HTMLSelectElement;
    expect(select.value).toBe('1');
    fireEvent.change(select, { target: { value: '4' } });
    expect(select.value).toBe('4');
    fireEvent.change(select, { target: { value: '2' } });
    expect(select.value).toBe('2');
  });

  it('TestPlanReplayViewPlayPause', async () => {
    mockFetchPlan('run-play', makeWireResponse({ runId: 'run-play' }));
    render(<PlanReplayView runId="run-play" />);
    await screen.findByTestId('plan-replay-list');

    const btn = screen.getByTestId('plan-replay-play-pause');
    expect(btn.getAttribute('data-playing')).toBe('false');
    fireEvent.click(btn);
    expect(btn.getAttribute('data-playing')).toBe('true');
    fireEvent.click(btn);
    expect(btn.getAttribute('data-playing')).toBe('false');
  });

  it('TestPlanReplayViewFetchError', async () => {
    const mock = vi.fn(async () => ({
      ok: false,
      status: 500,
      json: async () => ({}),
    } as Response));
    vi.stubGlobal('fetch', mock);

    render(<PlanReplayView runId="run-err" />);
    const err = await screen.findByTestId('plan-replay-error');
    expect(err).toHaveTextContent('规划轨迹加载失败');

    // Retry refetches.
    mockFetchPlan('run-err', makeWireResponse({ runId: 'run-err' }));
    fireEvent.click(screen.getByTestId('plan-replay-retry'));
    await waitFor(() => {
      expect(screen.getByTestId('plan-replay-list')).toBeInTheDocument();
    });
  });

  it('TestPlanReplayViewRunIdChange', async () => {
    const mock = vi.fn(async (url: string) => {
      if (typeof url === 'string' && url.includes('/api/run/run-A/plan')) {
        return { ok: true, status: 200, json: async () => makeWireResponse({ runId: 'run-A' }) } as Response;
      }
      if (typeof url === 'string' && url.includes('/api/run/run-B/plan')) {
        return {
          ok: true,
          status: 200,
          json: async () =>
            makeWireResponse({
              runId: 'run-B',
              todos: [{ id: 'b-1', content: 'B 的计划', status: 'pending' }],
            }),
        } as Response;
      }
      return { ok: false, status: 404, json: async () => ({}) } as Response;
    });
    vi.stubGlobal('fetch', mock);

    const { rerender } = render(<PlanReplayView runId="run-A" />);
    await screen.findByTestId('plan-replay-list');
    expect(mock.mock.calls.some(([url]) => typeof url === 'string' && url.includes('/run-A/plan'))).toBe(true);

    // Switch runId.
    await act(async () => {
      rerender(<PlanReplayView runId="run-B" />);
    });
    await waitFor(() => {
      const calls = mock.mock.calls.filter(
        ([url]) => typeof url === 'string' && url.includes('/run-B/plan'),
      );
      expect(calls.length).toBeGreaterThanOrEqual(1);
    });

    // The new run's todos are rendered.
    await screen.findByText('B 的计划');
  });

  it('TestPlanReplayViewLifecycleFromAudit', async () => {
    // 4 todos in chronological order — the BE returns ts-ASC, and the
    // FE preserves the array order. Step indexes ascend 0..3.
    mockFetchPlan(
      'run-chain',
      makeWireResponse({
        runId: 'run-chain',
        todos: [
          { id: 't-1', content: '计划', status: 'completed' },
          { id: 't-2', content: '检索', status: 'completed' },
          { id: 't-3', content: '验证', status: 'completed' },
          { id: 't-4', content: '生成', status: 'completed' },
        ],
      }),
    );
    render(<PlanReplayView runId="run-chain" />);

    const rows = await screen.findAllByTestId('plan-replay-row');
    expect(rows.map((r) => r.getAttribute('data-step-index'))).toEqual(['0', '1', '2', '3']);
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining('计划'),
      expect.stringContaining('检索'),
      expect.stringContaining('验证'),
      expect.stringContaining('生成'),
    ]);
  });
});