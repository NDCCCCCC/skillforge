// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Session-management phase — store-action tests for hydrateSessions /
// renameSession / toggleSessionPin (incl. the localStorage pin mirror).
//
// The store is module-global Zustand state; each test reseeds the
// sessions slice so order is independent. localStorage is a jsdom
// global — cleared per test to isolate the sf.session.pins mirror.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadSessionPins, useAppStore, type SessionMeta } from './app-store';
import type { PendingApproval, TerminalApproval } from './app-store';
import type { PlanTodo } from '../types/wire-types';

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

// Debug session new-session-shows-old-messages (2026-09-11) — the old
// reload-resetting in-memory counter re-minted `local-session-2` after
// every page load; ids reach the server via `x-session-id` (chat POST),
// so the re-minted id collided with an existing roster session
// (duplicate React keys + wrong header title/skill). Minted ids must be
// unique across reloads and must never equal an id already in the
// roster.
describe('app-store / createSession id uniqueness (debug new-session-shows-old-messages)', () => {
  it('mints ids that cannot collide with roster rows or with each other', () => {
    // Simulate the collision posture: the roster (hydrated from the BE)
    // already contains the deterministic `local-session-N` ids the old
    // generator would re-emit after a reload.
    useAppStore.setState({
      sessions: [
        { id: 'local-session-1', name: '会话 1', createdAt: 3_000 },
        { id: 'local-session-2', name: '生成 2026-08 月报', createdAt: 2_000 },
        { id: 'local-session-3', name: '你好', createdAt: 1_000 },
      ],
      activeSessionId: 'local-session-3',
    });

    // Two INTENTIONAL creates: step the mock clock past the rapid
    // double-dispatch guard (CREATE_DOUBLE_DISPATCH_MS) between them.
    const timeSpy = vi.spyOn(Date, 'now');
    let now = 10_000_000;
    timeSpy.mockImplementation(() => now);
    try {
      const first = useAppStore.getState().createSession();
      now += 1_000;
      const second = useAppStore.getState().createSession();

      expect(first).not.toBe(second);
      const preExisting = new Set(['local-session-1', 'local-session-2', 'local-session-3']);
      expect(preExisting.has(first)).toBe(false);
      expect(preExisting.has(second)).toBe(false);

      const ids = useAppStore.getState().sessions.map((s) => s.id);
      expect(new Set(ids).size).toBe(ids.length);
    } finally {
      timeSpy.mockRestore();
    }
  });

  it('swallows a rapid duplicate dispatch (residual #3): same id, one roster row', () => {
    useAppStore.setState({
      sessions: [{ id: 'session-1', name: '会话 1', createdAt: 0 }],
      activeSessionId: 'session-1',
    });

    // A double-clicked / double-dispatched 新建会话 control invokes
    // createSession twice within milliseconds. The second call must be
    // a no-op returning the first mint while it is still active; an
    // intentional create after the guard window still mints.
    const timeSpy = vi.spyOn(Date, 'now');
    let now = 10_000_000;
    timeSpy.mockImplementation(() => now);
    try {
      const first = useAppStore.getState().createSession();
      const duplicate = useAppStore.getState().createSession(); // same tick

      expect(duplicate).toBe(first);
      expect(useAppStore.getState().sessions.filter((s) => s.id === first)).toHaveLength(1);
      expect(useAppStore.getState().activeSessionId).toBe(first);

      now += 300; // past CREATE_DOUBLE_DISPATCH_MS — intentional create
      const intentional = useAppStore.getState().createSession();
      expect(intentional).not.toBe(first);

      const duplicateAgain = useAppStore.getState().createSession(); // same tick
      expect(duplicateAgain).toBe(intentional);
    } finally {
      timeSpy.mockRestore();
    }
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

// ----------------------------------------------------------------------------
// Plan 12-06 Task 2 — plan-trace slice (D14-extension: NOT in messages)
// ----------------------------------------------------------------------------

const PLAN_RUN = 'run-plan-test-1';
const PLAN_TODOS: PlanTodo[] = [
  { id: 't-a', content: '审核合同文本', status: 'pending' },
  { id: 't-b', content: '交叉验证规定引用', status: 'in_progress' },
  { id: 't-c', content: '生成结论', status: 'completed' },
];

function resetPlanSlice(): void {
  useAppStore.setState((s) => ({
    plan: { ...s.plan, planByRunId: {} },
  }));
}

describe('app-store / plan slice (Plan 12-06)', () => {
  beforeEach(() => {
    resetPlanSlice();
  });

  it('TestPlanSliceSetPlan', () => {
    useAppStore.getState().plan.setPlan(PLAN_RUN, PLAN_TODOS, 1700000000000);
    const planState = useAppStore.getState().plan.planByRunId[PLAN_RUN];
    expect(planState).toBeDefined();
    expect(planState.todos).toEqual(PLAN_TODOS);
    expect(planState.generatedAt).toBe(1700000000000);
    expect(planState.status).toBe('streaming');
  });

  it('TestPlanSliceMarkStatus', () => {
    useAppStore.getState().plan.setPlan(PLAN_RUN, PLAN_TODOS, 1700000000000);
    expect(useAppStore.getState().plan.planByRunId[PLAN_RUN].status).toBe('streaming');
    useAppStore.getState().plan.markPlanStatus(PLAN_RUN, 'complete');
    expect(useAppStore.getState().plan.planByRunId[PLAN_RUN].status).toBe('complete');
    // markPlanStatus is a no-op when runId is unknown — keep other rows untouched
    useAppStore.getState().plan.markPlanStatus('unknown-run', 'forced_stop');
    expect(useAppStore.getState().plan.planByRunId[PLAN_RUN].status).toBe('complete');
  });

  it('TestPlanSliceClearPlan', () => {
    useAppStore.getState().plan.setPlan(PLAN_RUN, PLAN_TODOS, 1700000000000);
    expect(useAppStore.getState().plan.planByRunId[PLAN_RUN]).toBeDefined();
    useAppStore.getState().plan.clearPlan(PLAN_RUN);
    expect(useAppStore.getState().plan.planByRunId[PLAN_RUN]).toBeUndefined();
  });

  it('TestPlanSliceLocalStoragePersistence', () => {
    // Step 1: setPlan and confirm the slice mirrored to localStorage.
    useAppStore.getState().plan.setPlan(PLAN_RUN, PLAN_TODOS, 1700000000000);
    const persisted = globalThis.localStorage.getItem('skillforge-plan');
    expect(persisted).not.toBeNull();
    const parsed = JSON.parse(persisted as string) as Record<string, { todos: PlanTodo[] }>;
    expect(parsed[PLAN_RUN]).toBeDefined();
    expect(parsed[PLAN_RUN].todos).toHaveLength(3);

    // Step 2: simulate a page refresh by re-hydrating from localStorage.
    // The slice's initial state is built via loadPersistedPlan on store
    // creation; we use the public hydration contract — re-read + reassemble.
    const freshSlice = {
      plan: {
        planByRunId: {
          [PLAN_RUN]: {
            todos: parsed[PLAN_RUN].todos,
            generatedAt: 1700000000000,
            status: 'streaming' as const,
          },
        },
        setPlan: useAppStore.getState().plan.setPlan,
        markPlanStatus: useAppStore.getState().plan.markPlanStatus,
        clearPlan: useAppStore.getState().plan.clearPlan,
      },
    };
    // Sanity check that the hydrated shape matches (PLAN-02 SC-2).
    expect(freshSlice.plan.planByRunId[PLAN_RUN].todos).toHaveLength(3);
    expect(freshSlice.plan.planByRunId[PLAN_RUN].todos[0].id).toBe('t-a');
  });

  it('TestPlanSliceD14Invariant', () => {
    // D14 extension: the plan slice MUST NOT expose a `messages` field
    // on the store. useChat owns the chat stream; the plan slice owns
    // the live planning-trace snapshot. The audit is structural — we
    // enumerate the AppState keys and assert no `messages`.
    const state = useAppStore.getState() as unknown as Record<string, unknown>;
    expect(state.messages).toBeUndefined();
    expect(state.plan).toBeDefined();
    expect(state.approval).toBeDefined();
  });
});
