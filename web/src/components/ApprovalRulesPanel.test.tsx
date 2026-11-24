// SPDX-License-Identifier: Apache-2.0

// 56-03 PANEL-01/03 -- ApprovalRulesPanel tests (Task 6 assertion set):
//   TestRulesHitColumn            - hit_count visible / never-hit greyed
//   TestRulesToggleEnabled        - PATCH {enabled} + optimistic update
//   TestRulesEmptyTwoPart         - guidance + suggestion card one-click
//   TestRulesSuggestionsNone      - "no suggestion clusters" fallback
//   TestRulesBadges               - global / match_type / origin badges
//   TestRulesHighlightConsumed    - highlightApprovalRuleId -> highlight class
//
// Fetch mocks mirror the BE envelopes (approval_rules_api.go: POST/PATCH/
// GET /approval-rule-suggestions).

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ApprovalRulesPanel from '../components/ApprovalRulesPanel';
import { useAppStore } from '../store/app-store';

function wireRule(overrides: Record<string, unknown> = {}) {
  return {
    rule_id: 'cnVsdS1tZW0',
    target: 'memory:facts',
    args_hash: 'memhash',
    skill_id: 'compliance',
    scope_session_id: 'sess-1',
    scope: 'session',
    match_type: 'exact',
    args_pattern: '',
    enabled: true,
    expires_at: null,
    origin: 'approve_remember',
    hit_count: 0,
    last_hit_at: null,
    created_at: new Date(Date.now() - 60_000).toISOString(),
    last_used_at: new Date(Date.now() - 30_000).toISOString(),
    ...overrides,
  };
}

function mockFetch(rules: unknown[], opts: { patchOk?: boolean; postOk?: boolean; suggestions?: unknown[] } = {}) {
  const mock = vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === 'PATCH') {
      return Promise.resolve({
        ok: opts.patchOk ?? true,
        status: (opts.patchOk ?? true) ? 200 : 500,
        json: async () => ({ ok: opts.patchOk ?? true }),
      } as Response);
    }
    if (init?.method === 'POST') {
      return Promise.resolve({
        ok: opts.postOk ?? true,
        status: (opts.postOk ?? true) ? 201 : 500,
        json: async () => ({ ok: opts.postOk ?? true }),
      } as Response);
    }
    if (url.includes('approval-rule-suggestions')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ suggestions: opts.suggestions ?? [] }),
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
    approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null, highlightApprovalRuleId: null },
  }));
});

describe('ApprovalRulesPanel (56-03)', () => {
  it('TestRulesHitColumn', async () => {
    mockFetch([
      wireRule({ hit_count: 3, last_hit_at: new Date(Date.now() - 30_000).toISOString() }),
      wireRule({ rule_id: 'r2', target: 'review_report', hit_count: 0, last_hit_at: null }),
    ]);
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getAllByTestId('approval-rules-row').length).toBe(2));
    expect(screen.getByText('命中 3 次')).toBeTruthy();
    expect(screen.getAllByText('未命中').length).toBeGreaterThanOrEqual(2);
  });

  it('TestRulesToggleEnabled', async () => {
    const mock = mockFetch([wireRule({})], { patchOk: true });
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getAllByTestId('approval-rules-row').length).toBe(1));
    const toggle = screen.getByTestId('approval-rules-toggle-cnVsdS1tZW0');
    fireEvent.click(toggle);
    await waitFor(() => expect(screen.getByText('规则已停用：memory:facts')).toBeTruthy());
    // PATCH carried {enabled: false}.
    const patchCall = mock.mock.calls.find(([, init]) => init?.method === 'PATCH');
    expect(patchCall).toBeTruthy();
    expect(JSON.parse(String(patchCall![1]?.body))).toEqual({ enabled: false });
  });

  it('TestRulesToggleRollbackOnError', async () => {
    mockFetch([wireRule({})], { patchOk: false });
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getAllByTestId('approval-rules-row').length).toBe(1));
    fireEvent.click(screen.getByTestId('approval-rules-toggle-cnVsdS1tZW0'));
    await waitFor(() => expect(screen.getByText(/更新失败/)).toBeTruthy());
    // Rollback: the toggle button text flips back to enabled state.
    expect(screen.getByText('停用')).toBeTruthy();
  });

  it('TestRulesEmptyTwoPart + suggestion one-click', async () => {
    mockFetch([], { suggestions: [{ target: 'review_report', occurrences: 3, matchType: 'template' }] });
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getByTestId('approval-rules-empty')).toBeTruthy());
    // Part 1: guidance.
    expect(screen.getByText(/还没有任何审批规则/)).toBeTruthy();
    // Part 2: the suggestion card.
    expect(screen.getByText(/review_report」已人工批准 3 次/)).toBeTruthy();
    // One-click create fires POST with template + session scope.
    fireEvent.click(screen.getByTestId('approval-rule-suggest-create-review_report'));
    await waitFor(() => expect(screen.getByText(/规则已创建：review_report/)).toBeTruthy());
  });

  it('TestRulesSuggestionsNone', async () => {
    mockFetch([], { suggestions: [] });
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getByTestId('approval-rules-empty')).toBeTruthy());
    expect(screen.getByText('暂无重复批准模式。')).toBeTruthy();
  });

  it('TestRulesBadges', async () => {
    mockFetch([
      wireRule({
        scope: 'global',
        match_type: 'prefix',
        origin: 'manual',
        rule_id: 'cnVsdS1nbG9i',
      }),
    ]);
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getAllByTestId('approval-rules-row').length).toBe(1));
    expect(screen.getAllByText('全局').length).toBe(2); // scope badge + dedicated badge
    expect(screen.getByTestId('approval-rules-match-prefix').textContent).toBe('同参数模板');
    expect(screen.getByText('手动')).toBeTruthy();
  });

  it('TestRulesHighlightConsumed', async () => {
    mockFetch([wireRule({})]);
    render(<ApprovalRulesPanel />);
    await waitFor(() => expect(screen.getAllByTestId('approval-rules-row').length).toBe(1));
    const row = screen.getByTestId('approval-rules-row');
    // Simulate a rule-matched event pointing at this rule.
    useAppStore.getState().approval.setHighlightApprovalRuleId('cnVsdS1tZW0');
    await waitFor(() => expect(row.className).toContain('border-[var(--color-primary)]'));
  });
});
