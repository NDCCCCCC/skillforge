// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 — useResumeBanner tests (HITL-03 cross-refresh resume).
//
//   - TestApprovalCardResumeBanner: GET /approval returning an active
//     pending row → store.approval.pending set + 恢复了一次中断的请求
//     banner
//   - TestApprovalCardExpiredState: state=expired → terminal.expired +
//     已过期 banner
//   - TestApprovalCardAuditFailClosed: 5xx → pending cleared, NO
//     banner (fail-closed; the error surfaces inside ApprovalCard)
//
// Fetch mock mirrors the BE envelopes: GET /approval → {approvals:
// [...]} (snake_case, RFC3339 created_at); /approval-rules → {rules:
// []}; /history → {messages: [], skill_id: ''}.
import { cleanup, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RESUME_BANNER_TEXT, useResumeBanner } from '../hooks/use-resume-banner';
import { useAppStore } from '../store/app-store';

function HookHarness({ sessionId }: { sessionId: string }) {
  useResumeBanner(sessionId);
  return null;
}

function wirePending(overrides: Record<string, unknown> = {}) {
  return {
    approval_id: 'a-1',
    tool_name: 'memory_facts_write',
    target: 'memory:facts',
    frozen_args: { fact: '员工离职需提前 30 天通知' },
    reason: 'r',
    created_at: new Date().toISOString(),
    state: 'active',
    skill_version: 'v1',
    prompt_hash: 'ph',
    ...overrides,
  };
}

function mockFetch(approvalResp: { ok: boolean; status: number; body: unknown }) {
  const mock = vi.fn((url: string) => {
    if (url.includes('/approval-rules')) {
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ rules: [] }) } as Response);
    }
    if (url.includes('/history')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ messages: [], skill_id: '' }),
      } as Response);
    }
    if (url.includes('/approval')) {
      return Promise.resolve({
        ok: approvalResp.ok,
        status: approvalResp.status,
        json: async () => approvalResp.body,
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

describe('useResumeBanner', () => {
  it('TestApprovalCardResumeBanner: active pending row → store.approval.pending + banner', async () => {
    mockFetch({ ok: true, status: 200, body: { approvals: [wirePending()] } });
    render(<HookHarness sessionId="sess-1" />);

    await waitFor(() => {
      expect(useAppStore.getState().approval.pending?.approvalId).toBe('a-1');
    });
    const banner = useAppStore.getState().approval.resumeBanner;
    expect(banner?.status).toBe('pending');
    expect(banner ? RESUME_BANNER_TEXT[banner.status] : null).toBe('恢复了一次中断的请求');
    // The pending row hydrated with the snake→camel mapping.
    const pending = useAppStore.getState().approval.pending;
    expect(pending?.toolName).toBe('memory_facts_write');
    expect(pending?.sessionId).toBe('sess-1');
    expect(pending?.skillId).toBe('compliance');
  });

  it('TestApprovalCardExpiredState: state=expired → terminal.expired + 已过期 banner', async () => {
    mockFetch({
      ok: true,
      status: 200,
      body: { approvals: [wirePending({ state: 'expired', skill_version: 'v2', prompt_hash: 'ph2' })] },
    });
    render(<HookHarness sessionId="sess-1" />);

    await waitFor(() => {
      expect(useAppStore.getState().approval.terminal?.status).toBe('expired');
    });
    // An expired row never becomes a pending card.
    expect(useAppStore.getState().approval.pending).toBeNull();
    const banner = useAppStore.getState().approval.resumeBanner;
    expect(banner?.status).toBe('expired');
    expect(banner ? RESUME_BANNER_TEXT[banner.status] : null).toContain('已过期');
  });

  it('TestApprovalCardResumeBanner: state=auto_passed → terminal auto-passed + 记忆规则 banner', async () => {
    mockFetch({
      ok: true,
      status: 200,
      body: { approvals: [wirePending({ state: 'auto_passed' })] },
    });
    render(<HookHarness sessionId="sess-1" />);

    await waitFor(() => {
      expect(useAppStore.getState().approval.terminal?.status).toBe('auto-passed');
    });
    expect(useAppStore.getState().approval.pending).toBeNull();
    const banner = useAppStore.getState().approval.resumeBanner;
    expect(banner?.status).toBe('auto-passed');
    expect(banner ? RESUME_BANNER_TEXT[banner.status] : null).toContain('记忆规则已通过');
  });

  it('TestApprovalCardAuditFailClosed: 5xx → pending cleared + NO banner', async () => {
    mockFetch({ ok: false, status: 500, body: {} });
    render(<HookHarness sessionId="sess-1" />);

    await waitFor(() => {
      // The fetch completed (approval-rules call resolved) and the
      // fail-closed branch ran: pending stays null, no banner.
      expect(useAppStore.getState().approval.pending).toBeNull();
    });
    // Give the effect a tick to (not) set a banner.
    await new Promise((r) => setTimeout(r, 20));
    expect(useAppStore.getState().approval.resumeBanner).toBeNull();
    expect(useAppStore.getState().approval.terminal).toBeNull();
  });
});
