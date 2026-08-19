// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Session-management phase — vitest cases for sessions.ts.
//
//   - fetchSessions: 200 roster parses + filters malformed rows;
//     non-2xx / network error / malformed payload → null
//   - renameSession: PATCH shape (method/body/URL-encoded id); true on
//     2xx, false on error
//   - deleteSession: DELETE shape; true on 2xx/204, false on error

import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  deleteSession,
  fetchSessions,
  renameSession,
  type SessionRosterResponse,
} from './sessions';

afterEach(() => {
  vi.restoreAllMocks();
});

const ROSTER: SessionRosterResponse = {
  sessions: [
    { id: 'sess-beta-0002', title: '更新的问题', last_active_ms: 2_000 },
    { id: 'sess-alpha-0001', title: '第一句问话', last_active_ms: 1_000 },
  ],
};

describe('sessions / fetchSessions', () => {
  it('parses a 200 roster and hits GET /api/sessions', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ROSTER,
    } as unknown as Response);
    const out = await fetchSessions(fetchImpl);
    expect(out).toEqual(ROSTER.sessions);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    const [url, init] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/sessions');
    expect((init.headers as Record<string, string>).Accept).toBe('application/json');
  });

  it('filters rows with non-string id/title from a malformed payload', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        sessions: [
          { id: 'sess-okay-00001', title: '好的', last_active_ms: 5 },
          { id: 42, title: 'bad id', last_active_ms: 6 },
          { id: 'sess-null-title', last_active_ms: 7 },
        ],
      }),
    } as unknown as Response);
    const out = await fetchSessions(fetchImpl);
    expect(out).toEqual([{ id: 'sess-okay-00001', title: '好的', last_active_ms: 5 }]);
  });

  it('returns null on non-2xx', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: false, status: 500 } as unknown as Response);
    expect(await fetchSessions(fetchImpl)).toBeNull();
  });

  it('returns null on network error', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new Error('offline'));
    expect(await fetchSessions(fetchImpl)).toBeNull();
  });

  it('returns null when the payload is not roster-shaped', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ unexpected: true }),
    } as unknown as Response);
    expect(await fetchSessions(fetchImpl)).toBeNull();
  });
});

describe('sessions / renameSession', () => {
  it('PATCHes {title} to /api/session/:id and returns true on 2xx', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true } as unknown as Response);
    const out = await renameSession('sess-alpha-0001', '自定义名字', fetchImpl);
    expect(out).toBe(true);
    const [url, init] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/session/sess-alpha-0001');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body as string)).toEqual({ title: '自定义名字' });
  });

  it('URL-encodes the session id', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true } as unknown as Response);
    await renameSession('sess needs encode', 'x', fetchImpl);
    const [url] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/session/sess%20needs%20encode');
  });

  it('returns false on 400 (empty title rejected by BE)', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: false, status: 400 } as unknown as Response);
    expect(await renameSession('sess-alpha-0001', '   ', fetchImpl)).toBe(false);
  });

  it('returns false on network error', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new Error('offline'));
    expect(await renameSession('sess-alpha-0001', 'x', fetchImpl)).toBe(false);
  });
});

describe('sessions / deleteSession', () => {
  it('DELETEs /api/session/:id and returns true on 204', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, status: 204 } as unknown as Response);
    const out = await deleteSession('sess-doomed-001', fetchImpl);
    expect(out).toBe(true);
    const [url, init] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/session/sess-doomed-001');
    expect(init.method).toBe('DELETE');
  });

  it('returns false on network error', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new Error('offline'));
    expect(await deleteSession('sess-doomed-001', fetchImpl)).toBe(false);
  });
});
