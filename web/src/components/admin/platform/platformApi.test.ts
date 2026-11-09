// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// 后端改名后的端点契约回归 —— /api/sparql → /api/kg/triples。
//
// 这条契约很容易在改名时被漏掉（前端还打老端点 → 404 → 管理台
// 里的三元组查询面板全线报错），所以把它钉死成一个测试。

import { afterEach, describe, expect, it, vi } from 'vitest';
import { kgTripleQuery, PlatformApiError } from './platformApi';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function stubFetch(body: unknown, ok = true, status = 200) {
  const fn = vi.fn().mockResolvedValue({
    ok,
    status,
    statusText: ok ? 'OK' : 'Error',
    json: async () => body,
  } as unknown as Response);
  vi.stubGlobal('fetch', fn);
  return fn;
}

describe('platformApi / kgTripleQuery', () => {
  it('POSTs to /api/kg/triples (NOT the retired /api/sparql)', async () => {
    const fetchMock = stubFetch({ results: [], count: 0, truncated: false });

    await kgTripleQuery({ s: '', p: 'rdf:type', o: 'skf:Skill', limit: 10 }, 'compliance');

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/kg/triples');
    expect(url).not.toContain('/api/sparql');
    expect(init.method).toBe('POST');
  });

  it('expands prefixed names into full IRIs and keeps the skill header', async () => {
    const fetchMock = stubFetch({ results: [], count: 0, truncated: false });

    await kgTripleQuery({ s: '', p: 'rdf:type', o: 'skf:Skill' }, 'compliance');

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(init.body as string) as { p: string; o: string };
    expect(body.p).toBe('http://www.w3.org/1999/02/22-rdf-syntax-ns#type');
    expect(body.o).toBe('http://skillforge.local/ontology#Skill');
    const headers = init.headers as Record<string, string>;
    expect(headers['X-Skill-Name']).toBe('compliance');
  });

  it('carries the owner token (backend minimal identity layer)', async () => {
    const fetchMock = stubFetch({ results: [], count: 0, truncated: false });

    await kgTripleQuery({ s: '', p: '', o: '' }, 'compliance');

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers['X-Owner-Token']).toBeTruthy();
  });

  it('surfaces a non-2xx as PlatformApiError', async () => {
    stubFetch({ error: 'boom' }, false, 500);
    await expect(kgTripleQuery({ s: '', p: '', o: '' }, 'compliance')).rejects.toBeInstanceOf(
      PlatformApiError,
    );
  });
});
