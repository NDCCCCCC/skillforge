// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 3 — PlanTracePart vitest suite (D14-authoritative
// state from the plan slice; synthetic write_todos fallback is a
// Pitfall 6 defensive layer).

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { PlanTracePart, renderStatusIcon } from './PlanTracePart';
import { useAppStore } from '../store/app-store';
import type { PlanTodo } from '../types/wire-types';

const RUN = 'run-test-1';

const SAMPLE_TODOS: PlanTodo[] = [
  { id: 't-1', content: '审核合同条款', activeForm: '正在审核合同条款', status: 'completed' },
  { id: 't-2', content: '交叉验证引用', activeForm: '正在验证引用', status: 'in_progress' },
  { id: 't-3', content: '生成结论', status: 'pending' },
];

beforeEach(() => {
  globalThis.localStorage.clear();
  useAppStore.setState((s) => ({
    plan: { ...s.plan, planByRunId: {} },
  }));
});

afterEach(() => {
  cleanup();
  useAppStore.setState((s) => ({
    plan: { ...s.plan, planByRunId: {} },
  }));
});

describe('PlanTracePart (Plan 12-06 Task 3)', () => {
  it('TestPlanTracePartStreamingRender', () => {
    useAppStore.getState().plan.setPlan(RUN, SAMPLE_TODOS, 1700000000000);
    const { container } = render(<PlanTracePart runId={RUN} />);

    expect(container.querySelector('[data-testid="plan-trace-part"]')).not.toBeNull();
    expect(container.querySelectorAll('[data-testid="plan-todo-row"]')).toHaveLength(3);

    const row1 = container.querySelector('[data-todo-id="t-1"]');
    expect(row1?.getAttribute('data-status')).toBe('completed');
    expect(row1?.textContent).toContain('审核合同条款');

    const row2 = container.querySelector('[data-todo-id="t-2"]');
    expect(row2?.getAttribute('data-status')).toBe('in_progress');
    // activeForm is shown when in_progress
    expect(row2?.textContent).toContain('正在验证引用');
  });

  it('TestPlanTracePartStatusTransitionAnimation', () => {
    // Step 1: pending todos only
    useAppStore
      .getState()
      .plan.setPlan(RUN, [{ id: 't-1', content: '初始', status: 'pending' }], 1700000000000);
    const { container, rerender } = render(<PlanTracePart runId={RUN} />);
    expect(container.querySelector('[data-testid="status-icon-pending"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="status-icon-in_progress"]')).toBeNull();

    // Step 2: flip the same todo to in_progress via setPlan replacement
    useAppStore
      .getState()
      .plan.setPlan(RUN, [{ id: 't-1', content: '进行中', activeForm: '执行中', status: 'in_progress' }], 1700000000001);
    rerender(<PlanTracePart runId={RUN} />);

    expect(container.querySelector('[data-testid="status-icon-pending"]')).toBeNull();
    expect(container.querySelector('[data-testid="status-icon-in_progress"]')).not.toBeNull();
  });

  it('TestPlanTracePartSyntheticToolCallFallback', () => {
    // No plan slice state — only synthetic tool-call parts fed through.
    const synthetic = [
      {
        toolName: 'write_todos',
        state: 'output-available',
        output: { todos: SAMPLE_TODOS },
      },
    ];
    const { container } = render(<PlanTracePart runId={RUN} syntheticParts={synthetic} />);

    // The synthetic fallback path must render the todos even when the
    // plan slice has no entry for this run (Pitfall 6 mitigation).
    expect(container.querySelectorAll('[data-testid="plan-todo-row"]')).toHaveLength(3);
    expect(container.querySelector('[data-testid="status-icon-completed"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="status-icon-in_progress"]')).not.toBeNull();
  });

  it('TestPlanTracePartEmptyRun', () => {
    // No plan slice + no synthetic parts → returns null.
    const { container } = render(<PlanTracePart runId="unknown-run" />);
    expect(container.querySelector('[data-testid="plan-trace-part"]')).toBeNull();
  });

  it('TestPlanTracePartCollapsedAfterComplete', () => {
    useAppStore.getState().plan.setPlan(RUN, SAMPLE_TODOS, 1700000000000);
    useAppStore.getState().plan.markPlanStatus(RUN, 'complete');

    const { container } = render(<PlanTracePart runId={RUN} collapsed />);
    const collapsed = container.querySelector('[data-testid="plan-trace-part-collapsed"]');
    expect(collapsed).not.toBeNull();
    expect(collapsed?.textContent).toContain('计划已完成');
    expect(collapsed?.textContent).toContain('1 步'); // exactly t-1 is completed in SAMPLE_TODOS
  });

  it('TestPlanTracePartCrossRefresh', () => {
    // Step 1: populate + persist.
    useAppStore.getState().plan.setPlan(RUN, SAMPLE_TODOS, 1700000000000);

    // Step 2: confirm the localStorage payload carries the todo snapshot.
    const persisted = globalThis.localStorage.getItem('skillforge-plan');
    expect(persisted).not.toBeNull();
    const parsed = JSON.parse(persisted as string) as Record<string, { todos: PlanTodo[] }>;
    expect(parsed[RUN]).toBeDefined();
    expect(parsed[RUN].todos).toHaveLength(3);

    // Step 3: re-hydrate manually (mirrors the loadPersistedPlan contract).
    // We re-populate the slice from the persisted payload to simulate a
    // page refresh; the slice's INITIAL state already does this on
    // store creation, but here we rebuild the slice entries to make the
    // path explicit.
    useAppStore.setState((s) => ({
      plan: {
        ...s.plan,
        planByRunId: {
          [RUN]: {
            todos: parsed[RUN].todos,
            generatedAt: 1700000000000,
            status: 'streaming',
          },
        },
      },
    }));

    const { container } = render(<PlanTracePart runId={RUN} />);
    expect(container.querySelectorAll('[data-testid="plan-todo-row"]')).toHaveLength(3);
  });

  it('TestPlanTracePartD14Invariant', () => {
    // The state selector MUST read from the plan slice, NOT messages.
    // We assert structurally: when the plan slice has an entry, the
    // component renders without needing any messages prop. (Pitfall 7
    // D14 invariant — the slice is the source of truth; the synthetic
    // fallback is ONLY a defensive layer.)
    useAppStore.getState().plan.setPlan(RUN, SAMPLE_TODOS, 1700000000000);
    const { container } = render(<PlanTracePart runId={RUN} />);

    expect(container.querySelectorAll('[data-testid="plan-todo-row"]')).toHaveLength(3);

    // Sanity: even with no synthetic parts, the slice's snapshot renders.
    const sliced = useAppStore.getState().plan.planByRunId[RUN];
    expect(sliced).toBeDefined();
    expect(sliced.todos).toHaveLength(3);
  });

  it('renderStatusIcon contract', () => {
    // Pins the visual status indicators — Phase 11 / Phase 12 lock.
    const pending = render(renderStatusIcon('pending')).container;
    expect(pending.querySelector('[data-testid="status-icon-pending"]')).not.toBeNull();

    const inProgress = render(renderStatusIcon('in_progress')).container;
    expect(inProgress.querySelector('[data-testid="status-icon-in_progress"]')).not.toBeNull();

    const completed = render(renderStatusIcon('completed')).container;
    expect(completed.querySelector('[data-testid="status-icon-completed"]')).not.toBeNull();
  });
});
