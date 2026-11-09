// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// apiFetch 的身份头契约 —— 后端最小身份层要求每个请求都带
// `X-Owner-Token`。apiFetch 是全站统一的 fetch helper，所以身份头
// 必须在这里注入（而不是逐个改几十个调用点）。

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch } from './api';
import { OWNER_TOKEN_STORAGE_KEY } from './owner-token';

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

function stubFetch(body: unknown = { ok: true }) {
  const fn = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => body,
  } as unknown as Response);
  vi.stubGlobal('fetch', fn);
  return fn;
}

describe('apiFetch / owner token', () => {
  it('injects X-Owner-Token on every request', async () => {
    const fetchMock = stubFetch({ hello: 'world' });

    await apiFetch<{ hello: string }>('/api/anything');

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Headers;
    expect(headers.get('X-Owner-Token')).toBeTruthy();
  });

  it('persists the token so every request of this browser shares one identity', async () => {
    const fetchMock = stubFetch({ hello: 'world' });

    await apiFetch('/api/one');
    await apiFetch('/api/two');

    const stored = localStorage.getItem(OWNER_TOKEN_STORAGE_KEY);
    expect(stored).toBeTruthy();
    const tokens = fetchMock.mock.calls.map(
      (call) => (call[1] as RequestInit).headers as Headers,
    );
    expect(tokens[0].get('X-Owner-Token')).toBe(stored);
    expect(tokens[1].get('X-Owner-Token')).toBe(stored);
  });

  it('lets an explicit caller-supplied header win (test injection)', async () => {
    const fetchMock = stubFetch({ hello: 'world' });

    await apiFetch('/api/anything', { headers: { 'X-Owner-Token': 'explicit-token' } });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect((init.headers as Headers).get('X-Owner-Token')).toBe('explicit-token');
  });
});
