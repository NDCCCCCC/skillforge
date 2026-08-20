// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Session-management phase — store-action tests for hydrateSessions /
// renameSession / toggleSessionPin (incl. the localStorage pin mirror).
//
// The store is module-global Zustand state; each test reseeds the
// sessions slice so order is independent. localStorage is a jsdom
// global — cleared per test to isolate the sf.session.pins mirror.

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { loadSessionPins, useAppStore, type SessionMeta } from './app-store';
import type { PendingApproval, TerminalApproval } from './app-store';

beforeEach(() => {
  globalThis.localStorage.clear();
});

afterEach(() => {
  // Restore a sane default roster for any test that runs after this
  // file (the store is module-global).
  useAppStore.setState({
    sessions: [{ id: 'session-1', name: '会话 1', createdAt: 0 }],
    activeSessionId: 'session-1',
  });
});

function seedRoster(): SessionMeta[] {
  return [
    { id: 'sess-beta-0002', name: '新会话', createdAt: 2_000, lastActiveAt: 2_000 },
    { id: 'sess-alpha-0001', name: '旧会话', createdAt: 1_000, lastActiveAt: 1_000 },
  ];
}

describe('app-store / hydrateSessions', () => {
  it('replaces the local roster and activates the newest BE session', () => {
    useAppStore.setState({
      sessions: [{ id: 'session-1', name: '会话 1', createdAt: 0 }],
      activeSessionId: 'session-1',
    });
    useAppStore.getState().hydrateSessions(seedRoster());
    const s = useAppStore.getState();
    expect(s.sessions.map((x) => x.id)).toEqual(['sess-beta-0002', 'sess-alpha-0001']);
    expect(s.activeSessionId).toBe('sess-beta-0002');
  });

  it('keeps the current active when it is itself in the roster', () => {
    useAppStore.getState().hydrateSessions(seedRoster());
    useAppStore.getState().setActiveSession('sess-alpha-0001');
    // A mid-session re-hydrate (same roster) must not yank the user
    // back to the newest session.
    useAppStore.getState().hydrateSessions(seedRoster());
    expect(useAppStore.getState().activeSessionId).toBe('sess-alpha-0001');
  });

  it('is a no-op on an empty roster (fresh-install posture)', () => {
    useAppStore.setState({
      sessions: [{ id: 'session-1', name: '会话 1', createdAt: 0 }],
      activeSessionId: 'session-1',
    });
    useAppStore.getState().hydrateSessions([]);
    expect(useAppStore.getState().sessions).toEqual([
      { id: 'session-1', name: '会话 1', createdAt: 0 },
    ]);
  });

  it('merges persisted pins into hydrated rows', () => {
    globalThis.localStorage.setItem(
      'sf.session.pins',
      JSON.stringify(['sess-alpha-0001']),
    );
    useAppStore.getState().hydrateSessions(seedRoster());
    const s = useAppStore.getState().sessions;
    expect(s.find((x) => x.id === 'sess-alpha-0001')?.pinned).toBe(true);
    expect(s.find((x) => x.id === 'sess-beta-0002')?.pinned).toBeFalsy();
  });
});

describe('app-store / renameSession', () => {
  it('renames only the target row (optimistic FE write)', () => {
    useAppStore.getState().hydrateSessions(seedRoster());
    useAppStore.getState().renameSession('sess-beta-0002', '改过的名字');
    const s = useAppStore.getState().sessions;
    expect(s.find((x) => x.id === 'sess-beta-0002')?.name).toBe('改过的名字');
    expect(s.find((x) => x.id === 'sess-alpha-0001')?.name).toBe('旧会话');
  });
});

describe('app-store / toggleSessionPin', () => {
  it('toggles the flag and mirrors the pin set to localStorage', () => {
    useAppStore.getState().hydrateSessions(seedRoster());

    useAppStore.getState().toggleSessionPin('sess-beta-0002');
    expect(useAppStore.getState().sessions.find((x) => x.id === 'sess-beta-0002')?.pinned).toBe(true);
    expect([...loadSessionPins()]).toEqual(['sess-beta-0002']);

    useAppStore.getState().toggleSessionPin('sess-beta-0002');
    expect(useAppStore.getState().sessions.find((x) => x.id === 'sess-beta-0002')?.pinned).toBeFalsy();
    expect(loadSessionPins().size).toBe(0);
  });

  it('survives a deleting localStorage guard (privacy mode, best-effort)', () => {
    // loadSessionPins must degrade to an empty set on a corrupt payload.
    globalThis.localStorage.setItem('sf.session.pins', '{not json');
    expect(loadSessionPins().size).toBe(0);
  });
});

// ----------------------------------------------------------------------------
// Plan 11-06 Task 6 — HITL approval-gate slice contract (D11-B4 read-side)
// ----------------------------------------------------------------------------

function makePendingApproval(overrides: Partial<PendingApproval> = {}): PendingApproval {
  return {
    approvalId: 'apr-store-1',
    toolName: 'memory_facts_write',
    target: 'memory:facts',
    frozenArgs: { fact: 'store test' },
    reason: 'store reason',
    createdAt: Date.now(),
    expiresAt: Date.now() + 3600000,
    skillVersion: 'v1',
    promptHash: 'ph-store',
    sessionId: 'sess-store',
    skillId: 'simple-qa',
    status: 'pending',
    ...overrides,
  };
}

function makeTerminalApproval(overrides: Partial<TerminalApproval> = {}): TerminalApproval {
  return {
    approvalId: 'apr-store-1',
    status: 'approved',
    frozenArgs: { fact: 'store test' },
    reason: '',
    createdAt: Date.now(),
    resolvedAt: Date.now(),
    ...overrides,
  };
}

describe('app-store / approval slice (Plan 11-06)', () => {
  beforeEach(() => {
    useAppStore.setState((s) => ({
      approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null },
    }));
  });

  it('TestSkillSwitchGuardDisabled', () => {
    // The D11-B4 read-side contract: 11-07's SkillPicker reads
    // approval.pending !== null to render aria-disabled + tooltip.
    // This test locks the slice shape that read depends on.
    expect(useAppStore.getState().approval.pending).toBeNull();

    useAppStore.getState().approval.setPending(makePendingApproval());
    const pendingState = useAppStore.getState().approval.pending;
    expect(pendingState).not.toBeNull();
    expect(pendingState?.approvalId).toBe('apr-store-1');
    expect(pendingState?.target).toBe('memory:facts');
  });

  it('TestSkillSwitchGuardReenabled', () => {
    // Recovery half of D11-B4: once the pending row resolves, the
    // field returns to null and 11-07's SkillPicker re-enables.
    useAppStore.getState().approval.setPending(makePendingApproval());
    expect(useAppStore.getState().approval.pending).not.toBeNull();

    useAppStore
      .getState()
      .approval.resolvePending('apr-store-1', makeTerminalApproval({ status: 'approved' }));
    expect(useAppStore.getState().approval.pending).toBeNull();
    expect(useAppStore.getState().approval.terminal?.status).toBe('approved');
  });

  it('TestResolvePendingRaceSafety', () => {
    // A stale SSE event (out-of-order delivery for a different
    // approvalId) must be a no-op — the live pending row survives.
    useAppStore.getState().approval.setPending(makePendingApproval({ approvalId: 'apr-1' }));
    expect(useAppStore.getState().approval.pending?.approvalId).toBe('apr-1');

    useAppStore
      .getState()
      .approval.resolvePending('apr-2', makeTerminalApproval({ approvalId: 'apr-2', status: 'denied' }));

    expect(useAppStore.getState().approval.pending?.approvalId).toBe('apr-1');
    expect(useAppStore.getState().approval.terminal).toBeNull();
  });
});
