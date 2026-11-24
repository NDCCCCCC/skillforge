// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- typed-error + header/URL contract pins.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  IngestApiError,
  asApiError,
  fetchJob,
  fetchJobs,
  retryStep,
} from './ingestApi';

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function okResp(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    json: async () => body,
  } as unknown as Response;
}

function errResp(status: number, body: unknown = { error: 'boom' }): Response {
  return {
    ok: false,
    status,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

describe('ingestApi', () => {
  it('sends X-Admin-Token from localStorage on every call', async () => {
    const calls: Array<[unknown, RequestInit | undefined]> = [];
    const mock = vi.fn(async (url: unknown, init?: RequestInit) => {
      calls.push([url, init]);
      return okResp({ jobs: [] });
    });
    vi.stubGlobal('fetch', mock);
    await fetchJobs();
    const init = calls[0][1];
    expect((init?.headers as Record<string, string>)['X-Admin-Token']).toBe('tok-abc');
  });

  it('unwrapsthe {jobs:[...]} envelope and never returns a non-array', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => okResp({ jobs: [] })));
    expect(await fetchJobs()).toEqual([]);

    vi.stubGlobal('fetch', vi.fn(async () => okResp({})));
    expect(await fetchJobs()).toEqual([]);
  });

  it('encodes the job id and step key into the retry URL', async () => {
    const calls: Array<[unknown, RequestInit | undefined]> = [];
    const mock = vi.fn(async (url: unknown, init?: RequestInit) => {
      calls.push([url, init]);
      return okResp({ status: 'queued', job_id: 'ing-abc', step: 'ecl_extract' });
    });
    vi.stubGlobal('fetch', mock);
    await retryStep('ing/abc', 'ecl extract');
    expect(String(calls[0][0]).includes('ing%2Fabc')).toBe(true);
    expect(String(calls[0][0]).includes('ecl%20extract')).toBe(true);
    expect(calls[0][1]?.method).toBe('POST');
  });

  it('keeps the server error text verbatim for non-403/404 statuses', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errResp(409, { error: 'step is not failed' })));
    await expect(async () => {
      await retryStep('ing-abc', 'parse');
    }).rejects.toMatchObject({ status: 409, message: 'step is not failed' });
  });

  it('maps 403 and 404 to their locked operator copy', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errResp(403, { error: 'admin token required' })));
    await expect(fetchJobs()).rejects.toMatchObject({
      status: 403,
      message: '缺少或错误的 X-Admin-Token',
    });

    vi.stubGlobal('fetch', vi.fn(async () => errResp(404, { error: 'unknown ingest job' })));
    await expect(fetchJob('ing-nope')).rejects.toMatchObject({
      status: 404,
      message: '任务不存在或已被清理',
    });
  });

  it('maps a fetch rejection to status 0 with 网络错误 (never a fabricated status)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('failed to fetch'); }));
    let caught: unknown;
    try {
      await fetchJobs();
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(IngestApiError);
    const apiErr = caught as IngestApiError;
    expect(apiErr.status).toBe(0);
    expect(apiErr.message).toBe('网络错误');
  });

  it('asApiError narrows unknown catch values', () => {
    expect(asApiError(new IngestApiError(500, 'x')).status).toBe(500);
    expect(asApiError(new Error('plain')).message).toBe('plain');
    expect(asApiError('nope').status).toBe(0);
  });
});
