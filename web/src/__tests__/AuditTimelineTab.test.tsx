// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 — AuditTimelineTab tests (HITL-04 / T-3-AUDIT).
//
//   - TestAuditTimelineTabRendering: rows + 5 filter chips render
//   - TestAuditTimelineTabReadOnly: rows carry NO interactive element
//     (INSERT-only audit chain mirrored at the UI layer)
//   - TestAuditTimelineTabFilterChips: chip click filters client-side
//
// Fetch mock mirrors the BE envelope (internal/server/audit_api.go):
// GET → {rows: [...], total} (snake_case, RFC3339 ts).
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AuditTimelineTab from '../components/AuditTimelineTab';
import { useAppStore } from '../store/app-store';

function wireRow(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    ts: new Date().toISOString(),
    action: 'action_pending_approval',
    target: 'memory:facts',
    reason: '',
    tool_call_id: 'tc-1',
    approval_id: 'a-1',
    scope_session_id: 'sess-1',
    skill_version: 'v1',
    prompt_hash: '7a3f9c2babcdef',
    user_id: 'op-1',
    ...overrides,
  };
}

function mockFetch(rows: unknown[] | null, ok = true) {
  const mock = vi.fn(() =>
    Promise.resolve({
      ok,
      status: ok ? 200 : 500,
      json: async () => (rows === null ? { unexpected: true } : { rows, total: rows.length }),
    } as Response),
  );
  vi.stubGlobal('fetch', mock);
  return mock;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

beforeEach(() => {
  useAppStore.setState({ activeSessionId: 'sess-1' });
});

describe('AuditTimelineTab', () => {
  it('TestAuditTimelineTabRendering: renders rows + 5 filter chips', async () => {
    mockFetch([
      wireRow({ id: 1, action: 'action_pending_approval' }),
      wireRow({ id: 2, action: 'action_approved' }),
    ]);
    render(<AuditTimelineTab />);
    expect(await screen.findAllByTestId('audit-timeline-row')).toHaveLength(2);
    expect(screen.getAllByTestId('audit-timeline-filter')).toHaveLength(5);
    // zh-CN action labels + status bullets per row (labels also appear
    // on the filter chips, so assert within the rows).
    expect(screen.getAllByTestId('audit-bullet')).toHaveLength(2);
    const rows = screen.getAllByTestId('audit-timeline-row');
    expect(rows[0]).toHaveTextContent('待审批');
    expect(rows[1]).toHaveTextContent('已批准');
  });

  it('TestAuditTimelineTabReadOnly: rows carry no button/anchor (T-3-AUDIT)', async () => {
    mockFetch([wireRow({ id: 1, action: 'action_approved' })]);
    render(<AuditTimelineTab />);
    const row = await screen.findByTestId('audit-timeline-row');
    // The row is a non-interactive <li> — no click handler, no edit /
    // delete affordance anywhere inside it.
    expect(row.tagName.toLowerCase()).toBe('li');
    expect(row.querySelector('button, a')).toBeNull();
  });

  it('TestAuditTimelineTabFilterChips: clicking 待审批 shows only pending rows', async () => {
    mockFetch([
      wireRow({ id: 1, action: 'action_pending_approval' }),
      wireRow({ id: 2, action: 'action_approved' }),
      wireRow({ id: 3, action: 'action_rule_revoked' }),
    ]);
    render(<AuditTimelineTab />);
    expect(await screen.findAllByTestId('audit-timeline-row')).toHaveLength(3);

    const chips = screen.getAllByTestId('audit-timeline-filter');
    const pendingChip = chips.find((c) => c.getAttribute('data-filter') === 'pending')!;
    fireEvent.click(pendingChip);

    const rows = screen.getAllByTestId('audit-timeline-row');
    expect(rows).toHaveLength(1);
    expect(rows[0].getAttribute('data-action')).toBe('action_pending_approval');
    expect(pendingChip.getAttribute('aria-pressed')).toBe('true');

    // 规则撤销 chip isolates the revoke row (D11-D4 transparency).
    const revokedChip = chips.find((c) => c.getAttribute('data-filter') === 'revoked')!;
    fireEvent.click(revokedChip);
    const revokedRows = screen.getAllByTestId('audit-timeline-row');
    expect(revokedRows).toHaveLength(1);
    expect(revokedRows[0].getAttribute('data-action')).toBe('action_rule_revoked');
  });

  it('fail-closed: non-2xx response renders the 审计链异常 banner', async () => {
    mockFetch([], false);
    render(<AuditTimelineTab />);
    const alert = await screen.findByTestId('audit-timeline-error');
    expect(alert).toHaveTextContent('审计链异常，请联系管理员。');
    expect(alert.getAttribute('role')).toBe('alert');
  });
});
