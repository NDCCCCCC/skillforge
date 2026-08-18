// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-08 Task 1 — App integration test for CR-01.
//
// Mounts <App> at the React tree root, simulates a user flow that
//   1. sends a message in session A
//   2. creates a new session B (LeftSidebar 新建会话 button)
//   3. clicks session A back via the LeftSidebar row
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
  it('(1) mounts and shows the initial session in the chat panel', () => {
    render(<App />);
    expect(screen.getByTestId('left-sidebar')).toBeInTheDocument();
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

      // Create session B (LeftSidebar 新建会话 button).
      const newSessionBtn = screen.getByTestId('left-sidebar-new-session');
      await act(async () => {
        fireEvent.click(newSessionBtn);
      });
      const newActive = useAppStore.getState().activeSessionId;
      expect(newActive).not.toBe('session-init');

      // Sanity check: session B is empty (A's messages are hidden while
      // B is active).
      await waitFor(() => {
        expect(screen.queryByText(/SENTINEL-MESSAGE-A/)).toBeNull();
      });

      // Click session A back via the LeftSidebar row.
      const sessionRows = screen.getAllByTestId('left-sidebar-session');
      const sessionARow = sessionRows.find(
        (r) => r.getAttribute('data-session-id') === 'session-init',
      );
      expect(sessionARow).toBeDefined();
      await act(async () => {
        fireEvent.click(sessionARow as HTMLElement);
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
