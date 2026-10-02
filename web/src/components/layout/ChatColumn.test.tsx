// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 4 — ChatColumn integration tests for PlanTracePart.
//
// Mounts the column with a stub useChat message stream and asserts the
// live planning-trace chip is / is not rendered based on the plan
// slice's currentRunId (D14-extension: the chip is store-derived, NOT
// message-derived).

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import type { UIMessage } from 'ai';
import ChatColumn from './ChatColumn';
import { useAppStore } from '../../store/app-store';
import type { PlanTodo } from '../../types/wire-types';

const SAMPLE_TODOS: PlanTodo[] = [
  { id: 't-1', content: '审核', status: 'pending' },
  { id: 't-2', content: '验证', status: 'in_progress' },
];

function emptyMessages(): UIMessage[] {
  return [];
}

beforeEach(() => {
  globalThis.localStorage.clear();
  useAppStore.setState((s) => ({
    plan: { ...s.plan, planByRunId: {}, currentRunId: null },
  }));
});

afterEach(() => {
  cleanup();
  useAppStore.setState((s) => ({
    plan: { ...s.plan, planByRunId: {}, currentRunId: null },
  }));
});

describe('ChatColumn — PlanTracePart integration (Plan 12-06 Task 4)', () => {
  it('TestRightChatRendersPlanTracePart', () => {
    // Populate the slice + set currentRunId via setPlan (which bumps it).
    useAppStore.getState().plan.setPlan('run-1', SAMPLE_TODOS, 1700000000000);
    expect(useAppStore.getState().plan.currentRunId).toBe('run-1');

    const { container } = render(
      <ChatColumn
        messages={emptyMessages()}
        status="streaming"
        onSend={() => {}}
      />,
    );

    // PlanTracePart is rendered because currentRunId is set.
    const chip = container.querySelector('[data-testid="plan-trace-part"]');
    expect(chip).not.toBeNull();
    expect(chip?.getAttribute('data-run-id')).toBe('run-1');
    expect(chip?.querySelectorAll('[data-testid="plan-todo-row"]')).toHaveLength(2);
  });

  it('TestRightChatNoPlanForActiveRun', () => {
    // No currentRunId → PlanTracePart is null → not rendered.
    const { container } = render(
      <ChatColumn
        messages={emptyMessages()}
        status="ready"
        onSend={() => {}}
      />,
    );

    expect(container.querySelector('[data-testid="plan-trace-part"]')).toBeNull();
    expect(container.querySelector('[data-testid="plan-trace-part-collapsed"]')).toBeNull();
  });
});
