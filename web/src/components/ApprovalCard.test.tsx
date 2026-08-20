// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-06 Task 4 — ApprovalCard vitest suite (5 tests, locked by
// 11-UI-SPEC §Test Lock Requirements):
//
//   TestApprovalCardRendering          — null gate + pending render +
//                                        FrozenArgs collapsed by default
//   TestApprovalCardStateTransitions   — pending + 5 terminal visuals
//   TestApprovalCardEditBeforeApprove  — HITL-07: edited args reach POST
//   TestApprovalCardScopeRadio         — D11-D3: radio only on 批准并记住
//   TestApprovalCardDenyRequiresReason — HITL-02: submit gated >=3 chars
//
// fetch is stubbed globally (no axios in this repo — the POST body
// uses the BE snake_case approvalPostRequest shape; the test parses
// RequestInit.body back to JSON for assertions).

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useAppStore } from '../store/app-store';
import type { PendingApproval, TerminalApproval } from '../store/app-store';
import { ApprovalCard } from './ApprovalCard';

function makePending(overrides: Partial<PendingApproval> = {}): PendingApproval {
  return {
    approvalId: 'apr-test-1',
    toolName: 'memory_facts_write',
    target: 'memory:facts',
    frozenArgs: { fact: '员工离职需提前 30 天通知' },
    reason: '用户上一轮追问后提出',
    createdAt: Date.now(),
    expiresAt: Date.now() + 3600000,
    skillVersion: 'v1',
    promptHash: 'ph-abc',
    sessionId: 'sess-1',
    skillId: 'simple-qa',
    status: 'pending',
    ...overrides,
  };
}

function makeTerminal(overrides: Partial<TerminalApproval> = {}): TerminalApproval {
  return {
    approvalId: 'apr-test-1',
    status: 'approved',
    frozenArgs: { fact: '员工离职需提前 30 天通知' },
    reason: '',
    createdAt: Date.now(),
    resolvedAt: Date.now(),
    ...overrides,
  };
}

/** Parse the JSON body of the first fetch call. */
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

describe('ApprovalCard (Plan 11-06)', () => {
  it('TestApprovalCardRendering', () => {
    // 1a) No pending + no terminal → no card.
    const { container: c1 } = render(<ApprovalCard sessionId="sess-1" />);
    expect(c1.querySelector('[data-testid="approval-card"]')).toBeNull();
    expect(c1.querySelector('[data-testid="approval-card-terminal"]')).toBeNull();
    cleanup();

    // 1b) Pending → card renders with pending state.
    useAppStore.getState().approval.setPending(makePending());
    const { container: c2 } = render(<ApprovalCard sessionId="sess-1" />);
    expect(c2.querySelector('[data-testid="approval-card"]')).not.toBeNull();
    expect(c2.querySelector('[data-testid="approval-card"]')?.getAttribute('data-state')).toBe('pending');
    expect(c2.querySelector('[data-testid="status-badge"]')?.textContent).toBe('待审批');
    expect(c2.querySelector('[data-testid="tool-name"]')?.textContent).toBe('memory_facts_write');

    // 1c) FrozenArgs JSON collapsed by default — pre is absent until 展开.
    expect(c2.querySelector('[data-testid="frozen-json"]')).toBeNull();
    expect(c2.querySelector('[data-testid="frozen-toggle"]')?.textContent).toContain('展开');
  });

  it('TestApprovalCardStateTransitions', () => {
    // Pending state.
    useAppStore.getState().approval.setPending(makePending());
    const { rerender, container } = render(<ApprovalCard sessionId="sess-1" />);
    expect(container.querySelector('[data-state="pending"]')).not.toBeNull();

    // Each of the 5 terminal visuals renders with its own data-state.
    const terminals: Array<TerminalApproval['status']> = [
      'approved',
      'denied',
      'expired',
      'auto-passed',
      'failed',
    ];
    for (const status of terminals) {
      useAppStore.setState((s) => ({
        approval: { ...s.approval, pending: null, terminal: makeTerminal({ status }) },
      }));
      rerender(<ApprovalCard sessionId="sess-1" />);
      const card = container.querySelector(`[data-state="${status}"]`);
      expect(card).not.toBeNull();
      expect(card?.getAttribute('data-testid')).toBe('approval-card-terminal');
    }
  });

  it('TestApprovalCardEditBeforeApprove', async () => {
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    useAppStore.getState().approval.setPending(makePending({ frozenArgs: { fact: 'original' } }));
    render(<ApprovalCard sessionId="sess-1" />);

    // Expand the JSON panel.
    fireEvent.click(screen.getByTestId('frozen-toggle'));
    expect(screen.getByTestId('frozen-json')).not.toBeNull();

    // Enter edit mode (HITL-07).
    fireEvent.click(screen.getByTestId('edit-toggle'));
    expect(screen.getByTestId('edit-textarea')).not.toBeNull();
    expect(screen.getByTestId('edit-banner').textContent).toBe('已修改参数，待审批将以新值执行');

    // Edit the args.
    fireEvent.change(screen.getByTestId('edit-textarea'), {
      target: { value: '{"fact":"edited-value"}' },
    });

    // Approve — the EDITED args must reach the POST body as frozen_args.
    fireEvent.click(screen.getByTestId('approve-once'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));

    const [url] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/session/sess-1/approval/apr-test-1');
    const body = firstFetchBody(fetchMock);
    expect(body).toMatchObject({ decision: 'approve', frozen_args: { fact: 'edited-value' } });
  });

  it('TestApprovalCardScopeRadio', async () => {
    useAppStore.getState().approval.setPending(makePending());
    render(<ApprovalCard sessionId="sess-1" />);

    // 4a) 批准一次 path — no scope radio rendered.
    expect(screen.queryByTestId('scope-radio')).toBeNull();

    // 4b) 批准并记住 toggles the scope radio open.
    fireEvent.click(screen.getByTestId('approve-remember'));
    const radio = screen.getByTestId('scope-radio');
    expect(radio).not.toBeNull();

    // 4c) Default selection is session.
    expect((screen.getByTestId('scope-session') as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId('scope-skill') as HTMLInputElement).checked).toBe(false);

    // 4d) Switch to skill.
    fireEvent.click(screen.getByTestId('scope-skill'));
    expect((screen.getByTestId('scope-skill') as HTMLInputElement).checked).toBe(true);

    // 4e) Confirm — POST body carries scope='skill'.
    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    fireEvent.click(screen.getByTestId('approve-remember-confirm'));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(firstFetchBody(fetchMock)).toMatchObject({ decision: 'approve', scope: 'skill' });

    // 4f) EXACTLY 2 radio options — D11-D3 binary scope, no global.
    expect(radio.querySelectorAll('input[type="radio"]').length).toBe(2);
  });

  it('TestApprovalCardDenyRequiresReason', async () => {
    useAppStore.getState().approval.setPending(makePending());
    render(<ApprovalCard sessionId="sess-1" />);

    // Open the deny prompt.
    fireEvent.click(screen.getByTestId('deny-button'));
    expect(screen.getByTestId('deny-prompt')).not.toBeNull();

    // 5a) Empty reason — submit disabled.
    const submitBtn = screen.getByTestId('deny-submit') as HTMLButtonElement;
    expect(submitBtn.disabled).toBe(true);

    // 5b) 2-char reason — still disabled.
    fireEvent.change(screen.getByTestId('deny-reason-input'), { target: { value: 'ab' } });
    expect(submitBtn.disabled).toBe(true);

    // 5c) 3-char reason — enabled; click POSTs decision:'deny' + reason.
    fireEvent.change(screen.getByTestId('deny-reason-input'), { target: { value: 'abc' } });
    expect(submitBtn.disabled).toBe(false);

    const fetchMock = vi.fn(async () => new Response('ok', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    fireEvent.click(submitBtn);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(firstFetchBody(fetchMock)).toMatchObject({ decision: 'deny', reason: 'abc' });
  });
});
