// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — FE chat-restore surface.
//
// On page refresh, the React app MUST rebuild the chat surface from
// the prior session's history so the user can resume mid-conversation
// (MEM-02 / D10-D1 / D10-D2). This file owns:
//
//   - SessionHistoryMessage / SessionHistoryResponse — the wire shape
//     mirror of internal/server/handlers_session_history.go's
//     historyEntry / historyResponse. The JSON tag casing is
//     snake_case (matching the Go server) — DO NOT camelCase.
//
//   - fetchHistory — issues GET /api/session/:id/history. Returns
//     null on network error or non-2xx status (the caller treats
//     null as empty — symmetric with the BE unknown-session 200+empty
//     posture per T-10-11).
//
//   - historyToUIMessages — transforms the wire response into a
//     UIMessage[] compatible with useChat({ initialMessages }). The
//     transformation is shape-faithful: a verdict-carrying assistant
//     row becomes a message with a data-policy part carrying the
//     Verdict object (matches the live wire shape — DataPolicyPart
//     renders it identically).

import type { UIMessage } from 'ai';
import type { Verdict } from '../types/verdict';

/** One rendered session turn from the BE history endpoint. The
 * `ts` field is the session_turn timestamp converted to ms (epoch ms). */
export interface SessionHistoryMessage {
  role: 'user' | 'assistant';
  content: string;
  /** Present only for assistant rows whose source content parsed as a
   *  verdict JSON envelope. Refusal / plain-prose rows omit it. */
  verdict?: Verdict;
  ts: number;
}

/** Response shape of GET /api/session/:id/history. `skill_id` is the
 *  most-recent audit row's SkillID for this session (empty string
 *  when the session has no audit rows — fresh / restored-from-cold). */
export interface SessionHistoryResponse {
  messages: SessionHistoryMessage[];
  skill_id: string;
}

/** Default API base URL — '/api' is the Vite proxy target (dev) and
 *  the same-origin path (prod behind the vite build / docker-compose
 *  v1 setup). Hard-coded to keep the helper import-graph small; the
 *  codebase does not currently expose VITE_API_BASE as an env knob. */
const API_BASE = '/api';

/** Fetch GET /api/session/:id/history. Returns the parsed response or
 *  null on any error / non-2xx status. The caller treats null as
 *  empty — symmetric with the BE's unknown-session 200+empty posture
 *  (T-10-11 existence-leak), so a missing session does not 500 the
 *  page. The optional `fetchImpl` parameter is the test injection
 *  point (vitest passes a vi.fn() stub). */
export async function fetchHistory(
  sessionId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<SessionHistoryResponse | null> {
  if (!sessionId) {
    // No session → caller treats as empty (mirrors the unknown-session
    // 200+empty posture on the BE).
    return null;
  }
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}/history`,
      {
        headers: { Accept: 'application/json' },
      },
    );
    if (!resp.ok) {
      return null;
    }
    const body = (await resp.json()) as Partial<SessionHistoryResponse>;
    if (!body || !Array.isArray(body.messages)) {
      // Defensive: malformed payload → null rather than throw.
      return null;
    }
    return {
      messages: body.messages as SessionHistoryMessage[],
      skill_id: typeof body.skill_id === 'string' ? body.skill_id : '',
    };
  } catch {
    // Network error / JSON parse error / abort → null (caller treats
    // as empty; chat surface falls back to a fresh session).
    return null;
  }
}

/** Transform the BE history response into a UIMessage[] compatible
 *  with useChat({ initialMessages }). Each message carries:
 *   - `id`: deterministic from sessionId + ts + role so React keys
 *     stay stable across re-renders.
 *   - `parts`: user → text part; assistant+verdict → data-policy
 *     part (Verdict object) + text part carrying the rendered
 *     conclusion; assistant+refusal → text part carrying the refusal
 *     sentence (refusal text is a plain chat string on the FE side).
 *
 *  The verdict-DataPolicyPart shape mirrors the live wire shape so
 *  DataPolicyPart renders the restored card identically to a streamed
 *  verdict (D10-D1 visual parity with live messages).
 */
export function historyToUIMessages(
  historyResponse: SessionHistoryResponse,
  sessionId: string,
): UIMessage[] {
  return historyResponse.messages.map((m) => {
    const id = `${sessionId}-${m.ts}-${m.role}`;
    if (m.role === 'user') {
      return {
        id,
        role: 'user' as const,
        parts: [{ type: 'text', text: m.content }],
      };
    }
    // Assistant row. If a parsed Verdict is present, emit a
    // data-policy part so DataPolicyPart → VerdictCard renders it.
    // The rendered content is the conclusion / refusal sentence
    // (which is the plain chat surface the user sees for both
    // verdict + refusal rows).
    if (m.verdict) {
      return {
        id,
        role: 'assistant' as const,
        parts: [
          { type: 'text', text: m.content },
          { type: 'data-policy', data: m.verdict },
        ],
      };
    }
    // Refusal / plain-prose assistant row — render as plain chat.
    return {
      id,
      role: 'assistant' as const,
      parts: [{ type: 'text', text: m.content }],
    };
  });
}