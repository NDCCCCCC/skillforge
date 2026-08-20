// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 3 — PlanRulesPanel tests (D11-D2 single-table reuse
// + target filter).
//
//   TestPlanRulesPanelEmpty       — empty envelope → empty copy
//   TestPlanRulesPanelList        — 2 plan_approve rules → 2 rows
//   TestPlanRulesPanelRevoke      — X click → DELETE + optimistic unmount
//   TestPlanRulesPanelScopeFilter — mixed targets → only plan_approve rows
//
// Fetch mocks mirror the BE envelopes (internal/server/approval_rules_api.go).

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import PlanRulesPanel from '../components/PlanRulesPanel';
import { useAppStore } from '../store/app-store';

function wireRule(overrides: Record<string, unknown> = {}) {
  return {
    rule_id: 'cnVsdS1wbGFu',
    target: 'plan_approve',
    args_hash: 'plan123hash',
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

describe('PlanRulesPanel', () => {
  it('TestPlanRulesPanelEmpty', async () => {
    mockFetch([]);
    render(<PlanRulesPanel />);
    const empty = await screen.findByTestId('plan-rules-empty');
    expect(empty).toHaveTextContent('还没有任何 plan_approve 规则。');
  });

  it('TestPlanRulesPanelList', async () => {
    mockFetch([
      wireRule({ rule_id: 'cnVsdC1h', target: 'plan_approve' }),
      wireRule({
        rule_id: 'cnVsdC1i',
        target: 'plan_approve',
        scope: 'skill',
        created_at: new Date(Date.now() - 90_000).toISOString(),
      }),
    ]);
    render(<PlanRulesPanel />);
    const rows = await screen.findAllByTestId('plan-rules-row');
    expect(rows).toHaveLength(2);
    // Both rows carry target='plan_approve' (text node).
    for (const row of rows) {
      expect(row).toHaveTextContent('plan_approve');
    }
  });

  it('TestPlanRulesPanelRevoke', async () => {
    const mock = mockFetch([wireRule()]);
    render(<PlanRulesPanel />);
    const row = await screen.findByTestId('plan-rules-row');
    expect(row).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('plan-rules-revoke'));

    await waitFor(() => {
      const deletes = mock.mock.calls.filter(([, init]) => (init as RequestInit)?.method === 'DELETE');
      expect(deletes).toHaveLength(1);
      expect(deletes[0][0]).toContain('/api/session/sess-1/approval-rules/cnVsdS1wbGFu');
    });
    expect(await screen.findByTestId('plan-rules-toast')).toHaveTextContent(
      '已撤销：plan_approve · 本会话',
    );
    await waitFor(() => {
      expect(screen.queryByTestId('plan-rules-row')).not.toBeInTheDocument();
    });
  });

  it('TestPlanRulesPanelScopeFilter', async () => {
    // Mix of plan_approve + memory:facts rules; the panel renders ONLY
    // plan_approve rows (D11-D2 single-table filter).
    mockFetch([
      wireRule({ rule_id: 'plan-1', target: 'plan_approve' }),
      wireRule({ rule_id: 'mem-1', target: 'memory:facts' }),
      wireRule({ rule_id: 'report-1', target: 'review_report' }),
    ]);
    render(<PlanRulesPanel />);
    const rows = await screen.findAllByTestId('plan-rules-row');
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveTextContent('plan_approve');
    expect(rows[0].getAttribute('data-rule-id')).toBe('plan-1');
  });
});