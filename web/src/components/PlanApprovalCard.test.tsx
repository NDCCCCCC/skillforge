// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 1 — PlanApprovalCard vitest suite (12 tests, mirrors
// Phase 11 ApprovalCard.test.tsx with plan_approve target filter).
//
//   TestPlanApprovalCardPendingRender      — pending state + 3 buttons
//   TestPlanApprovalCardApprove            — 批准一次 path
//   TestPlanApprovalCardDeny                — 拒绝 with >= 3 chars
//   TestPlanApprovalCardDenyRequiresReason — submit gated
//   TestPlanApprovalCardScopeRadio         — D11-D3 binary scope
//   TestPlanApprovalCardEditBeforeApprove  — HITL-07 edit wins
//   TestPlanApprovalCardApprovedState      — green border
//   TestPlanApprovalCardDeniedState        — red border + reason
//   TestPlanApprovalCardExpiredState       — yellow border (D11-C2)
//   TestPlanApprovalCardAutoPassedState    — gray border (rule match)
//   TestPlanApprovalCardFailedState        — red border (D11-C3)
//   TestPlanApprovalCardD14Invariant       — never reads useChat.messages
//
// POST body uses the BE snake_case ApprovalDecision shape (matches
// internal/server/plan_api.go::planApproveRequest).

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useAppStore } from '../store/app-store';
import type { PendingApproval, TerminalApproval } from '../store/app-store';
import { PlanApprovalCard } from './PlanApprovalCard';

function makePending(overrides: Partial<PendingApproval> = {}): PendingApproval {
  return {
    approvalId: 'apr-plan-1',
    toolName: 'plan_approve',
    target: 'plan_approve',
    frozenArgs: { plan: { title: '三步走', steps: 3 } },
    reason: 'agent wants to execute plan',
    createdAt: Date.now(),
    expiresAt: Date.now() + 3600000,
    skillVersion: 'v1',
    promptHash: 'ph-plan',
    sessionId: 'sess-plan-1',
    skillId: 'simple-qa',
    status: 'pending',
    ...overrides,
  };
}

function makeTerminal(overrides: Partial<TerminalApproval> = {}): TerminalApproval {
  return {
    approvalId: 'apr-plan-1',
    status: 'approved',
    frozenArgs: { plan: { title: '三步走', steps: 3 } },
    reason: '',
    createdAt: Date.now(),
    resolvedAt: Date.now(),
    ...overrides,
  };
}

function firstFetchBody(fetchMock: ReturnType<typeof vi.fn>): Record<string, unknown> {
  const init = fetchMock.mock.calls[0][1] as RequestInit;
  return JSON.parse(String(init.body)) as Record<string, unknown>;
}

beforeEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  useAppStore.setState((s) => ({
    approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null },
  }));
});

describe('PlanApprovalCard (Plan 12-07)', () => {
  it('TestPlanApprovalCardPendingRender', () => {
    // No pending + no terminal → no card.
    const { container: c1 } = render(<PlanApprovalCard runId="run-1" />);
    expect(c1.querySelector('[data-testid="plan-approval-card"]')).toBeNull();
    cleanup();

    // Pending → card renders.
    useAppStore.getState().approval.setPending(makePending());
    const { container: c2 } = render(<PlanApprovalCard runId="run-1" />);
    expect(c2.querySelector('[data-testid="plan-approval-card"]')).not.toBeNull();
    expect(c2.querySelector('[data-testid="plan-approval-card"]')?.getAttribute('data-state')).toBe('pending');
    expect(c2.querySelector('[data-testid="status-badge"]')?.textContent).toBe('待审批');
    expect(c2.querySelector('[data-testid="tool-name"]')?.textContent).toBe('plan_approve');

    // 3 buttons visible (批准一次 / 批准并记住 / 拒绝).
    expect(screen.getByTestId('approve-once')).toBeInTheDocument();
    expect(screen.getByTestId('approve-remember')).toBeInTheDocument();
    expect(screen.getByTestId('deny-button')).toBeInTheDocument();

    // FrozenArgs JSON collapsed by default.
    expect(c2.querySelector('[data-testid="frozen-json"]')).toBeNull();
    expect(c2.querySelector('[data-testid="frozen-toggle"]')?.textContent).toContain('展开');
  });

  it('TestPlanApprovalCardNonPlanApprovalIgnored', () => {
    // Pending target='memory:facts' is owned by Phase 11 ApprovalCard,
    // NOT this one. PlanApprovalCard must NOT render.
    useAppStore.getState().approval.setPending(
      makePending({ approvalId: 'apr-mem', target: 'memory:facts', toolName: 'memory_facts_write' }),
    );
    const { container } = render(<PlanApprovalCard runId="run-1" />);
    expect(container.querySelector('[data-testid="plan-approval-card"]')).toBeNull();
  });

  it('TestPlanApprovalCardApprove', async () => {
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    useAppStore.getState().approval.setPending(makePending());
    render(<PlanApprovalCard runId="run-1" />);

    fireEvent.click(screen.getByTestId('approve-once'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));

    const [url] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('/api/run/run-1/plan/apr-plan-1/approve');
    const body = firstFetchBody(fetchMock);
    expect(body).toMatchObject({
      decision: 'approve',
      target: 'plan_approve',
      approval_id: 'apr-plan-1',
      frozen_args: { plan: { title: '三步走', steps: 3 } },
    });
    expect(body).not.toHaveProperty('scope');
  });

  it('TestPlanApprovalCardDeny', async () => {
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    useAppStore.getState().approval.setPending(makePending());
    render(<PlanApprovalCard runId="run-1" />);

    fireEvent.click(screen.getByTestId('deny-button'));
    fireEvent.change(screen.getByTestId('deny-reason-input'), {
      target: { value: 'too long plan' },
    });

    fireEvent.click(screen.getByTestId('deny-submit'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));

    const body = firstFetchBody(fetchMock);
    expect(body).toMatchObject({
      decision: 'deny',
      reason: 'too long plan',
      target: 'plan_approve',
    });
  });

  it('TestPlanApprovalCardDenyRequiresReason', async () => {
    useAppStore.getState().approval.setPending(makePending());
    render(<PlanApprovalCard runId="run-1" />);

    fireEvent.click(screen.getByTestId('deny-button'));
    const submitBtn = screen.getByTestId('deny-submit') as HTMLButtonElement;
    // Empty reason — disabled.
    expect(submitBtn.disabled).toBe(true);
    // 2 chars — still disabled.
    fireEvent.change(screen.getByTestId('deny-reason-input'), { target: { value: 'ab' } });
    expect(submitBtn.disabled).toBe(true);
    // 3 chars — enabled.
    fireEvent.change(screen.getByTestId('deny-reason-input'), { target: { value: 'abc' } });
    expect(submitBtn.disabled).toBe(false);
  });

  it('TestPlanApprovalCardScopeRadio', async () => {
    useAppStore.getState().approval.setPending(makePending());
    render(<PlanApprovalCard runId="run-1" />);

    // 4a) No scope radio initially.
    expect(screen.queryByTestId('scope-radio')).toBeNull();

    // 4b) Click 批准并记住 → scope radio opens.
    fireEvent.click(screen.getByTestId('approve-remember'));
    const radio = screen.getByTestId('scope-radio');
    expect(radio).not.toBeNull();

    // 4c) Default = session.
    expect((screen.getByTestId('scope-session') as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId('scope-skill') as HTMLInputElement).checked).toBe(false);

    // 4d) Switch to skill.
    fireEvent.click(screen.getByTestId('scope-skill'));
    expect((screen.getByTestId('scope-skill') as HTMLInputElement).checked).toBe(true);

    // 4e) Confirm POSTs scope='skill'.
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    fireEvent.click(screen.getByTestId('approve-remember-confirm'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(firstFetchBody(fetchMock)).toMatchObject({
      decision: 'approve',
      scope: 'skill',
    });

    // 4f) Exactly 2 radios — D11-D3 binary scope.
    expect(radio.querySelectorAll('input[type="radio"]').length).toBe(2);
  });

  it('TestPlanApprovalCardEditBeforeApprove', async () => {
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    useAppStore
      .getState()
      .approval.setPending(makePending({ frozenArgs: { plan: { title: 'original' } } }));
    render(<PlanApprovalCard runId="run-1" />);

    fireEvent.click(screen.getByTestId('frozen-toggle'));
    expect(screen.getByTestId('frozen-json')).not.toBeNull();
    fireEvent.click(screen.getByTestId('edit-toggle'));
    expect(screen.getByTestId('edit-textarea')).not.toBeNull();
    expect(screen.getByTestId('edit-banner').textContent).toBe('已修改计划参数，待审批将以新值执行');

    fireEvent.change(screen.getByTestId('edit-textarea'), {
      target: { value: '{"plan":{"title":"edited-title"}}' },
    });

    fireEvent.click(screen.getByTestId('approve-once'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(firstFetchBody(fetchMock)).toMatchObject({
      decision: 'approve',
      frozen_args: { plan: { title: 'edited-title' } },
    });
  });

  it('TestPlanApprovalCardApprovedState', () => {
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<PlanApprovalCard runId="run-1" />);
    expect(container.querySelector('[data-state="pending"]')).not.toBeNull();

    useAppStore.setState((s) => ({
      approval: { ...s.approval, pending: null, terminal: makeTerminal({ status: 'approved' }) },
    }));
    rerender(<PlanApprovalCard runId="run-1" />);
    const card = container.querySelector('[data-state="approved"]');
    expect(card).not.toBeNull();
    expect(card?.getAttribute('data-testid')).toBe('plan-approval-card-terminal');
  });

  it('TestPlanApprovalCardDeniedState', () => {
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<PlanApprovalCard runId="run-1" />);

    useAppStore.setState((s) => ({
      approval: {
        ...s.approval,
        pending: null,
        terminal: makeTerminal({ status: 'denied', reason: 'no thanks' }),
      },
    }));
    rerender(<PlanApprovalCard runId="run-1" />);
    const card = container.querySelector('[data-state="denied"]');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('已拒绝：no thanks');
  });

  it('TestPlanApprovalCardExpiredState', () => {
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<PlanApprovalCard runId="run-1" />);

    useAppStore.setState((s) => ({
      approval: {
        ...s.approval,
        pending: null,
        terminal: makeTerminal({ status: 'expired' }),
      },
    }));
    rerender(<PlanApprovalCard runId="run-1" />);
    const card = container.querySelector('[data-state="expired"]');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('该审批已过期');
  });

  it('TestPlanApprovalCardAutoPassedState', () => {
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<PlanApprovalCard runId="run-1" />);

    useAppStore.setState((s) => ({
      approval: {
        ...s.approval,
        pending: null,
        terminal: makeTerminal({ status: 'auto-passed' }),
      },
    }));
    rerender(<PlanApprovalCard runId="run-1" />);
    const card = container.querySelector('[data-state="auto-passed"]');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('已自动通过（记忆规则）');
  });

  it('TestPlanApprovalCardFailedState', () => {
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<PlanApprovalCard runId="run-1" />);

    useAppStore.setState((s) => ({
      approval: {
        ...s.approval,
        pending: null,
        terminal: makeTerminal({ status: 'failed', reason: 'HTTP 500' }),
      },
    }));
    rerender(<PlanApprovalCard runId="run-1" />);
    const card = container.querySelector('[data-state="failed"]');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('审批操作失败：HTTP 500');
  });

  it('TestPlanApprovalCardD14Invariant', () => {
    // Static source check — PlanApprovalCard source MUST NOT reference
    // useChat.messages / messages: / messagesSetter — D14 invariant.
    // (The approval slice is the authoritative source, NOT the chat stream.)
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const fs = require('node:fs') as typeof import('node:fs');
    const src = fs.readFileSync(
      'src/components/PlanApprovalCard.tsx',
      'utf-8',
    );
    expect(src).not.toMatch(/useChat\.messages/);
    expect(src).not.toMatch(/useChat\(\)/);
    // No 'messages' field read on the store.
    expect(src).not.toMatch(/state\.messages/);
    expect(src).not.toMatch(/s\.messages/);
  });
});