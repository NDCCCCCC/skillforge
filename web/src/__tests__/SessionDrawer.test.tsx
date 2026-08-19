// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SessionDrawer tests (session-management phase):
//   1. Renders through a portal under document.body (the TopBar
//      backdrop-filter containing-block lesson from KnowledgeModal B7).
//   2. Groups render: 置顶 / 今天 / 7 天内 / 更早, newest-first within
//      a group.
//   3. Search filters rows by name.
//   4. Pin toggles the flag + persists to localStorage.
//   5. Rename: inline input → Enter commits the optimistic store write
//      + PATCH; a failed PATCH reverts + dispatches sf-toast.
//   6. Delete removes the row; deleting the active session falls the
//      active id back to the newest remaining.
//   7. Close paths: ×, Esc, scrim click. open=false renders nothing.
//   8. Row click switches the active session + closes.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SessionDrawer from '../components/layout/SessionDrawer';
import { useAppStore, type SessionMeta } from '../store/app-store';

const NOW = Date.now();

function seedStore(overrides?: { sessions?: SessionMeta[]; activeSessionId?: string }) {
  const sessions: SessionMeta[] =
    overrides?.sessions ??
    [
      { id: 'sess-today-00001', name: '今天的会话', createdAt: NOW, lastActiveAt: NOW - 3_600_000 },
      {
        id: 'sess-week-00001',
        name: '本周的会话',
        createdAt: NOW - 2 * 86_400_000,
        lastActiveAt: NOW - 2 * 86_400_000,
      },
      {
        id: 'sess-oldd-00001',
        name: '更早的会话',
        createdAt: NOW - 30 * 86_400_000,
        lastActiveAt: NOW - 30 * 86_400_000,
      },
    ];
  useAppStore.setState({
    sessions,
    activeSessionId: overrides?.activeSessionId ?? sessions[0].id,
  });
}

function renderDrawer(open = true) {
  return render(<SessionDrawer open={open} onClose={() => {}} />);
}

function rowOf(sessionId: string) {
  return screen
    .getAllByTestId('session-drawer-row')
    .find((r) => r.getAttribute('data-session-id') === sessionId);
}

function actionOn(sessionId: string, testid: string) {
  const row = rowOf(sessionId);
  expect(row).toBeDefined();
  const btn = row!.querySelector(`[data-testid="${testid}"]`);
  expect(btn).not.toBeNull();
  return btn as HTMLElement;
}

/** URL-dispatching fetch stub: /api/session/:id PATCH + DELETE. */
function stubSessionFetch(ok = true) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (url.startsWith('/api/session/')) {
      return { ok, status: ok ? 200 : 500 } as unknown as Response;
    }
    return { ok: false, status: 404 } as unknown as Response;
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

beforeEach(() => {
  globalThis.localStorage.clear();
  seedStore();
  stubSessionFetch();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('SessionDrawer', () => {
  it('portals directly under document.body (backdrop-filter containing-block fix)', () => {
    renderDrawer();
    expect(screen.getByTestId('session-drawer').parentElement).toBe(document.body);
    expect(screen.getByTestId('session-drawer-scrim').parentElement).toBe(document.body);
  });

  it('renders time groups with the seeded rows', () => {
    renderDrawer();
    const groups = screen.getAllByTestId('session-drawer-group');
    expect(groups.map((g) => g.getAttribute('data-group'))).toEqual(['today', 'week', 'older']);
    expect(screen.getByText('今天的会话')).toBeInTheDocument();
    expect(screen.getByText('本周的会话')).toBeInTheDocument();
    expect(screen.getByText('更早的会话')).toBeInTheDocument();
  });

  it('sorts newest-first within a group and pins surface in 置顶', () => {
    seedStore({
      sessions: [
        {
          id: 'sess-pin1-000001',
          name: '置顶的旧会话',
          createdAt: NOW - 30 * 86_400_000,
          lastActiveAt: NOW - 30 * 86_400_000,
          pinned: true,
        },
        { id: 'sess-new1-000001', name: '新', createdAt: NOW, lastActiveAt: NOW },
        { id: 'sess-new2-000002', name: '旧一点', createdAt: NOW, lastActiveAt: NOW - 5_000 },
      ],
      activeSessionId: 'sess-new1-000001',
    });
    renderDrawer();
    const groups = screen.getAllByTestId('session-drawer-group');
    expect(groups[0].getAttribute('data-group')).toBe('pinned');
    const todayRows = groups[1].querySelectorAll('[data-testid="session-drawer-row"]');
    expect(todayRows[0].getAttribute('data-session-id')).toBe('sess-new1-000001');
    expect(todayRows[1].getAttribute('data-session-id')).toBe('sess-new2-000002');
  });

  it('search filters rows by name', () => {
    renderDrawer();
    fireEvent.change(screen.getByTestId('session-drawer-search'), {
      target: { value: '本周' },
    });
    expect(screen.getByText('本周的会话')).toBeInTheDocument();
    expect(screen.queryByText('今天的会话')).toBeNull();
    expect(screen.queryByText('更早的会话')).toBeNull();
  });

  it('pin toggle flips the flag and persists to localStorage', () => {
    renderDrawer();
    fireEvent.click(actionOn('sess-today-00001', 'session-drawer-pin'));
    expect(
      useAppStore.getState().sessions.find((s) => s.id === 'sess-today-00001')?.pinned,
    ).toBe(true);
    expect(JSON.parse(globalThis.localStorage.getItem('sf.session.pins') ?? '[]')).toEqual([
      'sess-today-00001',
    ]);
    // The row moved to the 置顶 group.
    const groups = screen.getAllByTestId('session-drawer-group');
    expect(groups[0].getAttribute('data-group')).toBe('pinned');
  });

  it('rename: Enter commits optimistic store write + PATCH', async () => {
    const fetchMock = stubSessionFetch(true);
    renderDrawer();
    fireEvent.click(actionOn('sess-today-00001', 'session-drawer-rename'));
    const input = screen.getByTestId('session-drawer-rename-input') as HTMLInputElement;
    expect(input.value).toBe('今天的会话');
    fireEvent.change(input, { target: { value: '改过的名字' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(
      useAppStore.getState().sessions.find((s) => s.id === 'sess-today-00001')?.name,
    ).toBe('改过的名字');
    await waitFor(() => {
      const patch = fetchMock.mock.calls.find(
        ([url, init]) => String(url).startsWith('/api/session/') && init?.method === 'PATCH',
      );
      expect(patch).toBeDefined();
      expect(JSON.parse((patch![1] as RequestInit).body as string)).toEqual({
        title: '改过的名字',
      });
    });
  });

  it('rename: failed PATCH reverts the name and dispatches sf-toast', async () => {
    stubSessionFetch(false); // every request fails
    const toasts: string[] = [];
    window.addEventListener('sf-toast', (e) => {
      toasts.push((e as CustomEvent<{ message: string }>).detail.message);
    });
    renderDrawer();
    fireEvent.click(actionOn('sess-today-00001', 'session-drawer-rename'));
    const input = screen.getByTestId('session-drawer-rename-input');
    fireEvent.change(input, { target: { value: '会失败的名字' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    // Optimistic write first…
    expect(
      useAppStore.getState().sessions.find((s) => s.id === 'sess-today-00001')?.name,
    ).toBe('会失败的名字');
    // …then the revert once the PATCH resolves.
    await waitFor(() => {
      expect(
        useAppStore.getState().sessions.find((s) => s.id === 'sess-today-00001')?.name,
      ).toBe('今天的会话');
    });
    expect(toasts).toContain('重命名失败，已恢复原名称');
  });

  it('delete removes the row, fires DELETE, and falls the active id back', async () => {
    const fetchMock = stubSessionFetch(true);
    seedStore({ activeSessionId: 'sess-today-00001' });
    renderDrawer();
    fireEvent.click(actionOn('sess-today-00001', 'session-drawer-delete'));

    expect(useAppStore.getState().sessions.map((s) => s.id)).not.toContain('sess-today-00001');
    // Store fallback: newest remaining session becomes active.
    expect(useAppStore.getState().activeSessionId).toBe('sess-week-00001');
    await waitFor(() => {
      const del = fetchMock.mock.calls.find(
        ([, init]) => init?.method === 'DELETE',
      );
      expect(del).toBeDefined();
    });
  });

  it('row click switches the active session and closes the drawer', () => {
    let closed = false;
    render(<SessionDrawer open onClose={() => { closed = true; }} />);
    fireEvent.click(screen.getByText('本周的会话'));
    expect(useAppStore.getState().activeSessionId).toBe('sess-week-00001');
    expect(closed).toBe(true);
  });

  it('新建 creates a session and closes', () => {
    let closed = false;
    render(<SessionDrawer open onClose={() => { closed = true; }} />);
    const before = useAppStore.getState().sessions.length;
    fireEvent.click(screen.getByTestId('session-drawer-create'));
    expect(useAppStore.getState().sessions.length).toBe(before + 1);
    expect(closed).toBe(true);
  });

  it('closes via the × button, the Esc key, and the scrim click', () => {
    let closed = false;
    render(<SessionDrawer open onClose={() => { closed = true; }} />);
    fireEvent.click(screen.getByTestId('session-drawer-close'));
    expect(closed).toBe(true);

    closed = false;
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(closed).toBe(true);

    closed = false;
    fireEvent.click(screen.getByTestId('session-drawer-scrim'));
    expect(closed).toBe(true);
  });

  it('renders nothing when open={false}', () => {
    renderDrawer(false);
    expect(screen.queryByTestId('session-drawer')).toBeNull();
    expect(screen.queryByTestId('session-drawer-scrim')).toBeNull();
  });
});
