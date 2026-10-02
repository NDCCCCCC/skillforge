// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — vitest cases for chat-restore.ts.
//
//   - TestFetchHistory_Success — happy path; mock fetch returns 200
//     with 2 messages; the parsed shape matches the BE wire contract.
//   - TestFetchHistory_NetworkError — fetch rejects; returns null.
//   - TestFetchHistory_404 — server returns 404; returns null (the
//     unknown-session posture on the BE is 200+empty, but a literal
//     404 still yields null — the helper is permissive).
//   - TestHistoryToUIMessages_Transforms — 3-message history (1
//     user, 1 assistant+verdict, 1 user) → UIMessage[] with text +
//     data-policy parts.
//   - TestHistoryToUIMessages_Empty — empty history → empty
//     UIMessage[].

import { describe, expect, it, vi, afterEach } from 'vitest';
import {
  fetchHistory,
  historyToUIMessages,
  type SessionHistoryResponse,
} from './chat-restore';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('chat-restore / fetchHistory', () => {
  it('TestFetchHistory_Success — parses a 200 response with 2 messages', async () => {
    const mockBody: SessionHistoryResponse = {
      messages: [
        { role: 'user', content: 'first question', ts: 1700000000000 },
        {
          role: 'assistant',
          content: 'compliant',
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
      skill_id: 'compliance',
    };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => mockBody,
    } as unknown as Response);
    const out = await fetchHistory('session-abc-12345', fetchImpl);
    expect(out).toEqual(mockBody);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    expect(fetchImpl.mock.calls[0][0]).toBe(
      '/api/session/session-abc-12345/history',
    );
  });

  it('TestFetchHistory_NetworkError — rejected promise yields null', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new Error('network down'));
    const out = await fetchHistory('session-xyz-98765', fetchImpl);
    expect(out).toBeNull();
  });

  it('TestFetchHistory_404 — non-2xx status yields null', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({ error: 'unknown' }),
    } as unknown as Response);
    const out = await fetchHistory('session-ghost-1234', fetchImpl);
    expect(out).toBeNull();
  });

  it('TestFetchHistory_NoSessionId — empty sessionId returns null without fetching', async () => {
    const fetchImpl = vi.fn();
    const out = await fetchHistory('', fetchImpl);
    expect(out).toBeNull();
    expect(fetchImpl).not.toHaveBeenCalled();
  });
});

describe('chat-restore / historyToUIMessages', () => {
  it('TestHistoryToUIMessages_Transforms — 3 messages map to text + data-policy parts', () => {
    const history: SessionHistoryResponse = {
      messages: [
        { role: 'user', content: 'first question', ts: 1000 },
        {
          role: 'assistant',
          content: 'compliant',
          ts: 1100,
          verdict: {
            status: 'ok',
            confidence: 0.85,
            citations: [{ chunk_id: '1', span: '条款 X', tier: '权威' }],
            review_required: false,
            compliance_status: 'compliant',
          },
        },
        { role: 'user', content: 'second question', ts: 2000 },
      ],
      skill_id: 'compliance',
    };
    const out = historyToUIMessages(history, 'session-test-1234');
    expect(out).toHaveLength(3);

    // Row 0: text part only.
    expect(out[0].role).toBe('user');
    expect(out[0].id).toBe('session-test-1234-1000-user');
    expect(out[0].parts).toEqual([{ type: 'text', text: 'first question' }]);

    // Row 1: text part + data-policy part carrying the verdict.
    expect(out[1].role).toBe('assistant');
    expect(out[1].id).toBe('session-test-1234-1100-assistant');
    expect(out[1].parts).toHaveLength(2);
    expect(out[1].parts[0]).toEqual({ type: 'text', text: 'compliant' });
    expect(out[1].parts[1]).toMatchObject({
      type: 'data-policy',
      data: expect.objectContaining({ status: 'ok' }),
    });

    // Row 2: text part only.
    expect(out[2].role).toBe('user');
    expect(out[2].id).toBe('session-test-1234-2000-user');
    expect(out[2].parts).toEqual([{ type: 'text', text: 'second question' }]);
  });

  it('TestHistoryToUIMessages_Empty — empty history yields empty UIMessage[]', () => {
    const out = historyToUIMessages({ messages: [], skill_id: '' }, 'sess-x');
    expect(out).toEqual([]);
  });

  it('TestHistoryToUIMessages_Refusal — assistant row without verdict renders as text', () => {
    const history: SessionHistoryResponse = {
      messages: [
        {
          role: 'assistant',
          content: '无法给出结论：未检索到可回答该问题的授权知识源。',
          ts: 3000,
        },
      ],
      skill_id: '',
    };
    const out = historyToUIMessages(history, 'sess-refuse');
    expect(out).toHaveLength(1);
    expect(out[0].role).toBe('assistant');
    expect(out[0].parts).toEqual([
      {
        type: 'text',
        text: '无法给出结论：未检索到可回答该问题的授权知识源。',
      },
    ]);
  });
});

// ---------------------------------------------------------------------------
// Phase 29 / Plan 29-04 Task 3 — the history echo's FE half (D-29-03).
// Assistant rows carrying turn_id synthesize a data-turn part (same
// consumption point as the live stream); rows without one (user rows,
// unattributed legacy rows) synthesize none. voted_verdict rides the
// data-turn payload's optional TS-only field — the Go emit side never
// marshals it (see wire-types.ts TurnPayload).
// ---------------------------------------------------------------------------

describe('chat-restore / historyToUIMessages data-turn synthesis', () => {
  it('TestHistoryToUIMessages_DataTurnSynthesis — assistant rows with turn_id gain a data-turn part', () => {
    const history: SessionHistoryResponse = {
      messages: [
        { role: 'user', content: 'question', ts: 1000, turn_id: 0 },
        {
          role: 'assistant',
          content: 'compliant',
          ts: 1100,
          turn_id: 41,
          voted_verdict: 'helpful',
          verdict: {
            status: 'ok',
            confidence: 0.9,
            citations: [],
            review_required: false,
            compliance_status: 'compliant',
          },
        },
        {
          role: 'assistant',
          content: '无法给出结论：未检索到可回答该问题的授权知识源。',
          ts: 1200,
          turn_id: 43,
        },
      ],
      skill_id: 'compliance',
    };
    const out = historyToUIMessages(history, 'sess-turn-echo');

    // Row 0 — user: no data-turn part even when the key is present (0
    // is the unset value on the wire; user rows never carry it).
    expect(out[0].parts.every((p) => p.type !== 'data-turn')).toBe(true);

    // Row 1 — verdict assistant: data-turn part carrying turnId +
    // votedVerdict alongside the data-policy part.
    const dataTurn1 = out[1].parts.find((p) => p.type === 'data-turn');
    expect(dataTurn1).toMatchObject({
      type: 'data-turn',
      data: { turnId: 41, votedVerdict: 'helpful' },
    });

    // Row 2 — refusal assistant: data-turn part with NO votedVerdict
    // (unvoted → undefined, never null/fabricated).
    const dataTurn2 = out[2].parts.find((p) => p.type === 'data-turn');
    expect(dataTurn2).toMatchObject({
      type: 'data-turn',
      data: { turnId: 43 },
    });
    expect(
      (dataTurn2 as { data: { votedVerdict?: string } }).data.votedVerdict,
    ).toBeUndefined();
  });

  it('TestHistoryToUIMessages_NoTurnID — rows without turn_id gain no data-turn part', () => {
    const history: SessionHistoryResponse = {
      messages: [
        {
          role: 'assistant',
          content: 'compliant',
          ts: 1100,
          verdict: {
            status: 'ok',
            confidence: 0.9,
            citations: [],
            review_required: false,
            compliance_status: 'compliant',
          },
        },
        {
          role: 'assistant',
          content: 'plain prose',
          ts: 1200,
        },
      ],
      skill_id: '',
    };
    const out = historyToUIMessages(history, 'sess-no-turn');
    expect(out[0].parts.every((p) => p.type !== 'data-turn')).toBe(true);
    expect(out[1].parts.every((p) => p.type !== 'data-turn')).toBe(true);
  });

  it('TestHistoryToUIMessages_TurnIDZero — turn_id 0 emits no data-turn part (degradation posture)', () => {
    const history: SessionHistoryResponse = {
      messages: [
        {
          role: 'assistant',
          content: 'plain prose',
          ts: 1300,
          turn_id: 0,
        },
      ],
      skill_id: '',
    };
    const out = historyToUIMessages(history, 'sess-zero-turn');
    expect(out[0].parts.every((p) => p.type !== 'data-turn')).toBe(true);
  });
});