// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 — ApprovalRulesPanel tests (HITL-08 / D11-D4).
//
//   - TestApprovalRulesPanelEmptyState: empty envelope → empty copy
//   - TestApprovalRulesPanelRevoke: X click → DELETE + optimistic
//     unmount + success toast
//   - TestApprovalRulesPanelReactiveUpdate: approval.terminal.status
//     flip → re-fetch (D14 reactive pattern)
//
// Fetch mocks mirror the BE envelopes (internal/server/
// approval_rules_api.go): GET → {rules: [...]} (snake_case, RFC3339
// timestamps); DELETE → {status:'revoked'}.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ApprovalRulesPanel from '../components/ApprovalRulesPanel';
import { useAppStore } from '../store/app-store';

function wireRule(overrides: Record<string, unknown> = {}) {
  return {
    rule_id: 'cnVsdS0x',
    target: 'memory:facts',
    args_hash: 'abc123def456',
    skill_id: 'compliance',
    scope_session_id: 'sess-1',
    scope: 'session',
    created_at: new Date(Date.now() - 60_000).toISOString(),
    last_used_at: new Date(Date.now() - 30_000).toISOString(),
    ...overrides,
  };
}

function mockFetch(rules: unknown[], opts: { deleteOk?: boolean } = {}) {
  const deleteOk = opts.deleteOk ?? true;
  const mock = vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === 'DELETE') {
      return Promise.resolve({
        ok: deleteOk,
        status: deleteOk ? 200 : 500,
        json: async () => ({ status: 'revoked' }),
      } as Response);
    }
    if (url.includes('/approval-rules')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ rules }),
      } as Response);
    }
    return Promise.resolve({ ok: true, status: 200, json: async () => ({}) } as Response);
  });
  vi.stubGlobal('fetch', mock);
  return mock;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

beforeEach(() => {
  useAppStore.setState((s) => ({
    activeSessionId: 'sess-1',
    currentSkillId: 'compliance',
    approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null },
  }));
});

describe('ApprovalRulesPanel', () => {
  it('TestApprovalRulesPanelEmptyState: renders empty copy when the API returns []', async () => {
    mockFetch([]);
    render(<ApprovalRulesPanel />);
    const empty = await screen.findByTestId('approval-rules-empty');
    expect(empty).toHaveTextContent('还没有任何审批规则。批准一次时勾选「记住」即可创建。');
  });

  it('TestApprovalRulesPanelRevoke: X click fires DELETE + optimistic unmount + success toast', async () => {
    const mock = mockFetch([wireRule()]);
    render(<ApprovalRulesPanel />);
    const row = await screen.findByTestId('approval-rules-row');
    expect(row).toHaveTextContent('memory:facts');
    expect(row).toHaveTextContent('本会话');

    fireEvent.click(screen.getByTestId('approval-rules-revoke'));

    // DELETE fired against the rule URL.
    await waitFor(() => {
      const deletes = mock.mock.calls.filter(([, init]) => (init as RequestInit)?.method === 'DELETE');
      expect(deletes).toHaveLength(1);
      expect(deletes[0][0]).toContain('/api/session/sess-1/approval-rules/cnVsdS0x');
    });
    // Success toast surfaces (visible during the 120ms fade + 3s dismiss).
    expect(await screen.findByTestId('approval-rules-toast')).toHaveTextContent(
      '已撤销：memory:facts · 本会话',
    );
    // Row fades out then unmounts optimistically.
    await waitFor(() => {
      expect(screen.queryByTestId('approval-rules-row')).not.toBeInTheDocument();
    });
  });

  it('TestApprovalRulesPanelRevoke: failed DELETE keeps the row + shows an error toast', async () => {
    mockFetch([wireRule()], { deleteOk: false });
    render(<ApprovalRulesPanel />);
    await screen.findByTestId('approval-rules-row');

    fireEvent.click(screen.getByTestId('approval-rules-revoke'));

    expect(await screen.findByTestId('approval-rules-toast')).toHaveTextContent('撤销失败：');
    expect(screen.getByTestId('approval-rules-row')).toBeInTheDocument();
  });

  it('TestApprovalRulesPanelReactiveUpdate: refetches when approval.terminal.status flips', async () => {
    const mock = mockFetch([]);
    render(<ApprovalRulesPanel />);
    await screen.findByTestId('approval-rules-empty');
    const callsBefore = mock.mock.calls.filter(([url]) =>
      String(url).includes('scope=session'),
    ).length;
    expect(callsBefore).toBeGreaterThanOrEqual(1);

    // Simulate an approve-and-remember terminalization (11-06 slice).
    useAppStore.setState((s) => ({
      approval: {
        ...s.approval,
        terminal: {
          approvalId: 'a-1',
          status: 'approved',
          frozenArgs: {},
          reason: '',
          createdAt: Date.now(),
          resolvedAt: Date.now(),
        },
      },
    }));

    await waitFor(() => {
      const callsAfter = mock.mock.calls.filter(([url]) =>
        String(url).includes('scope=session'),
      ).length;
      expect(callsAfter).toBeGreaterThan(callsBefore);
    });
  });
});
