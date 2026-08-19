// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Session-management phase — FE roster API surface.
//
// Mirrors the chat-restore.ts posture: API_BASE hard-coded, optional
// fetchImpl injection for tests, and non-2xx / network errors surface
// as null (the caller degrades to the local in-memory session state —
// the roster is an enhancement, never a blocker).
//
// Wire shapes mirror internal/server/handlers_sessions.go exactly:
//   - GET  /api/sessions      → { sessions: [{id, title, last_active_ms}] }
//   - PATCH /api/session/:id  body {title} (rename upsert)
//   - DELETE /api/session/:id (idempotent 204)

/** One roster row from GET /api/sessions (snake_case wire, DO NOT
 * camelCase — matches the Go handler's JSON tags). */
export interface SessionRosterEntry {
  id: string;
  title: string;
  last_active_ms: number;
}

/** Response shape of GET /api/sessions. The BE guarantees sessions is
 * an array (never null). */
export interface SessionRosterResponse {
  sessions: SessionRosterEntry[];
}

/** Same API-base posture as chat-restore.ts — '/api' is the Vite proxy
 * target (dev) and the same-origin path (prod). */
const API_BASE = '/api';

/** Fetch GET /api/sessions. Returns the roster or null on any error /
 * non-2xx / malformed payload. A null return means "roster unavailable"
 * — the caller keeps the local session state (fresh-session posture,
 * symmetric with the BE's T-10-11 no-existence-leak stance). */
export async function fetchSessions(
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<SessionRosterEntry[] | null> {
  try {
    const resp = await fetchImpl(`${API_BASE}/sessions`, {
      headers: { Accept: 'application/json' },
    });
    if (!resp.ok) {
      return null;
    }
    const body = (await resp.json()) as Partial<SessionRosterResponse>;
    if (!body || !Array.isArray(body.sessions)) {
      return null;
    }
    return body.sessions.filter(
      (s) => typeof s?.id === 'string' && typeof s?.title === 'string',
    );
  } catch {
    return null;
  }
}

/** PATCH /api/session/:id — rename upsert. Returns true on 2xx, false
 * on any error (the caller decides whether to roll back the optimistic
 * FE rename). */
export async function renameSession(
  sessionId: string,
  title: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<boolean> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
      },
    );
    return resp.ok;
  } catch {
    return false;
  }
}

/** DELETE /api/session/:id — turns + meta, audit untouched. Returns
 * true on 2xx/204 (the BE is idempotent — an unknown id still 204s,
 * so false only means "request failed", never "session did not
 * exist"). */
export async function deleteSession(
  sessionId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<boolean> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}`,
      { method: 'DELETE' },
    );
    return resp.ok;
  } catch {
    return false;
  }
}
