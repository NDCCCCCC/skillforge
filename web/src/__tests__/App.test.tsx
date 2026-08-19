// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-08 Task 1 — App integration test for CR-01.
//
// Mounts <App> at the React tree root, simulates a user flow that
//   1. sends a message in session A
//   2. creates a new session B (store.createSession — the UI entry
//      moved to the TopBar session picker in the B2 chat-first
//      redesign; the LeftSidebar button is gone)
//   3. switches back to session A (store.setActiveSession)
//   4. asserts session A's previously-streamed bubbles are STILL present
//
// This is the test that would have caught the CR-01 bug: the previous
// implementation used `useChat({ id: activeSessionId, transport })`
// which allocated a fresh Chat() on every activeSessionId flip (verified
// at @ai-sdk/react/dist/index.js:326-328), wiping messages. The fix
// (Plan 05-08) keeps a per-session Chat map in a useRef and passes the
// Chat instance via the `chat` prop instead of `id`.
//
// Plan 10-03 Task 2 (MEM-02 / D10-D1 + D10-D2) — three additional
// tests cover the FE restore surface:
//   - TestApp_InitialMessagesFromHistory — the restored session shows
//     the prior conversation bubbles.
//   - TestApp_SkillAutoSwitch — history.skill_id → store.currentSkillId.
//   - TestApp_HistoryRewriteHint — a data-decision event with
//     decision="history_query_rewritten" renders the HistoryRewriteHint
//     caption above the user bubble.
//
// The tests neutralize useSkillReload's networking (via vi.stubGlobal
// fetch) so the only network noise is the MockChatTransport path that
// <App> instantiates via createTransport('dev').
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import App from '../App';
import { useAppStore } from '../store/app-store';
import type { SessionHistoryResponse } from '../lib/chat-restore';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

beforeEach(() => {
  // Neutralize useSkillReload's outbound fetch so the test does not
  // race against real polling. The <App> mockChatTransport path drives
  // the actual chat traffic.
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [],
    } as unknown as Response),
  );
  // Reset the store to a known seed so test order is independent.
  useAppStore.setState({
    skills: [],
    sessions: [{ id: 'session-init', name: '会话 1', createdAt: 0 }],
    activeSessionId: 'session-init',
    currentSkillId: null,
    skillServiceOffline: false,
  });
});

describe('App integration (Plan 05-08 CR-01 + D5-IA-03)', () => {
  it('(1) mounts and shows the chat-first shell with the initial session', () => {
    render(<App />);
    expect(screen.getByTestId('chat-body')).toBeInTheDocument();
    expect(useAppStore.getState().activeSessionId).toBe('session-init');
  });

  it('(2) sends a message in session A and the bubble renders', async () => {
    render(<App />);
    const textarea = screen.getByTestId('chat-input-textarea');
    fireEvent.change(textarea, { target: { value: 'SENTINEL-MESSAGE-A' } });
    fireEvent.click(screen.getByTestId('chat-input-send'));
    await waitFor(
      () => {
        expect(screen.getByText(/SENTINEL-MESSAGE-A/)).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
  });

  it(
    '(3) retains session A messages after switching to session B and back (CR-01)',
    async () => {
      render(<App />);

      // Send the sentinel in session A.
      const textarea = screen.getByTestId('chat-input-textarea');
      fireEvent.change(textarea, { target: { value: 'SENTINEL-MESSAGE-A' } });
      fireEvent.click(screen.getByTestId('chat-input-send'));
      await waitFor(
        () => {
          expect(screen.getByText(/SENTINEL-MESSAGE-A/)).toBeInTheDocument();
        },
        { timeout: 3000 },
      );

      // Create session B (store action — the UI entry point moved to
      // the TopBar session picker in B2; LeftSidebar is gone).
      await act(async () => {
        useAppStore.getState().createSession();
      });
      const newActive = useAppStore.getState().activeSessionId;
      expect(newActive).not.toBe('session-init');

      // Sanity check: session B is empty (A's messages are hidden while
      // B is active).
      await waitFor(() => {
        expect(screen.queryByText(/SENTINEL-MESSAGE-A/)).toBeNull();
      });

      // Switch back to session A via the store action.
      await act(async () => {
        useAppStore.getState().setActiveSession('session-init');
      });
      expect(useAppStore.getState().activeSessionId).toBe('session-init');

      // CR-01 round-trip: session A's messages must still be present.
      await waitFor(
        () => {
          expect(screen.getByText(/SENTINEL-MESSAGE-A/)).toBeInTheDocument();
        },
        { timeout: 3000 },
      );
    },
  );

  it('(4) does not crash when the chat prop is set without an id prop', () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(<App />);
    // No console.error from React about missing/incompatible props.
    const offendingCalls = consoleErrorSpy.mock.calls.filter((call) => {
      const msg = call.map((c) => String(c)).join(' ');
      return /chat prop|id prop|useChat/.test(msg);
    });
    expect(offendingCalls).toHaveLength(0);
    consoleErrorSpy.mockRestore();
  });
});

// Plan 10-03 Task 2 — FE restore surface (MEM-02 / D10-D1 + D10-D2).
//
// These three tests mock the history endpoint so the App component
// takes the restore path on mount. Each test stubs global.fetch to
// return the desired history payload; the MockChatTransport path is
// still the chat surface (no actual backend LLM calls).
describe('App restore (Plan 10-03 MEM-02 / D10-D1 + D10-D2)', () => {
  it('TestApp_InitialMessagesFromHistory — restored bubbles render on mount', async () => {
    const restored: SessionHistoryResponse = {
      messages: [
        { role: 'user', content: 'RESTORED-USER-BUBBLE', ts: 1700000000000 },
        {
          role: 'assistant',
          content: 'RESTORED-ASSISTANT-BUBBLE',
          ts: 1700000001000,
          verdict: {
            status: 'ok',
            confidence: 0.9,
            citations: [],
            review_required: false,
            compliance_status: 'compliant',
          },
        },
      ],
      skill_id: '',
    };
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (url: string) => {
        if (typeof url === 'string' && url.includes('/api/session/') && url.includes('/history')) {
          return {
            ok: true,
            json: async () => restored,
          } as unknown as Response;
        }
        // Other fetches (useSkillReload) → empty array.
        return {
          ok: true,
          json: async () => [],
        } as unknown as Response;
      }),
    );

    render(<App />);
    await waitFor(() => {
      expect(screen.getByText(/RESTORED-USER-BUBBLE/)).toBeInTheDocument();
    });
    expect(screen.getByText(/RESTORED-ASSISTANT-BUBBLE/)).toBeInTheDocument();
  });

  it('TestApp_SkillAutoSwitch — history.skill_id → store.currentSkillId', async () => {
    const restored: SessionHistoryResponse = {
      messages: [{ role: 'user', content: 'any question', ts: 1700000000000 }],
      skill_id: 'compliance',
    };
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (url: string) => {
        if (typeof url === 'string' && url.includes('/api/session/') && url.includes('/history')) {
          return {
            ok: true,
            json: async () => restored,
          } as unknown as Response;
        }
        return {
          ok: true,
          json: async () => [],
        } as unknown as Response;
      }),
    );

    // Pre-seed the skills list so the auto-switch hook recognizes the
    // session-bound skill as a known Skill.
    useAppStore.setState({
      skills: [{ id: 'compliance', name: 'compliance', description: '', status: 'ok' }],
      currentSkillId: null,
    });

    render(<App />);
    await waitFor(() => {
      expect(useAppStore.getState().currentSkillId).toBe('compliance');
    });
  });
});

describe('App session hydration (session-management phase)', () => {
  it('TestApp_HydrateRoster — GET /api/sessions replaces the local roster + activates newest', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (url: string) => {
        if (typeof url === 'string' && url.includes('/api/sessions')) {
          return {
            ok: true,
            json: async () => ({
              sessions: [
                { id: 'sess-beta-0002', title: '最新的会话', last_active_ms: 2_000 },
                { id: 'sess-alpha-0001', title: '较早的会话', last_active_ms: 1_000 },
              ],
            }),
          } as unknown as Response;
        }
        // History fetch for the hydrated active session → empty; the
        // skill-reload poll → empty array.
        return {
          ok: true,
          json: async () => (url.includes('/history') ? { messages: [], skill_id: '' } : []),
        } as unknown as Response;
      }),
    );

    render(<App />);
    await waitFor(() => {
      expect(useAppStore.getState().activeSessionId).toBe('sess-beta-0002');
    });
    expect(useAppStore.getState().sessions.map((s) => s.id)).toEqual([
      'sess-beta-0002',
      'sess-alpha-0001',
    ]);
    expect(useAppStore.getState().sessions[0].lastActiveAt).toBe(2_000);
  });

  it('TestApp_HydrateEmptyKeepsLocal — empty roster keeps the boot session', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async () => ({
        ok: true,
        json: async () => ({ sessions: [] }),
      }) as unknown as Response,
    ));

    render(<App />);
    // Give the hydrate promise a tick to resolve as null-ish/empty —
    // then the local session must be untouched.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });
    const s = useAppStore.getState();
    expect(s.activeSessionId).toBe('session-init');
    expect(s.sessions).toEqual([{ id: 'session-init', name: '会话 1', createdAt: 0 }]);
  });
});
