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
