// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 260813-dno / UAT #4 — tests for fetchChunkContent.
//
// Pins the 3 acceptance assertions for the on-demand chunk-content fetch:
//   1. 200 → resolves { text, tier } from the ChunkPayload body.
//   2. 404 → resolves null (soft miss, distinct from the graph hook's 503 handling).
//   3. other !ok → rejects with the body.error (or HTTP status fallback).
//
// fetch is spied on globalThis and restored between tests so the mock
// state never bleeds across cases (and never leaks into ChunkSourceView's
// tests, which set up their own fetch mock).
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { fetchChunkContent } from '../lib/chunk-lookup';

describe('fetchChunkContent', () => {
  let fetchSpy: MockInstance;

  beforeEach(() => {
    fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('resolves { text, tier } on 200', async () => {
    fetchSpy.mockResolvedValue(
      new Response(
        JSON.stringify({
          id: '1',
          tier: '权威',
          source: 'authority/x.md',
          content: '正文内容',
          effectiveFrom: 0,
          effectiveTo: 0,
          version: '',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );

    const got = await fetchChunkContent('1');
    expect(got).not.toBeNull();
    expect(got).toEqual({ text: '正文内容', tier: '权威' });
  });

  it('returns null on 404', async () => {
    fetchSpy.mockResolvedValue(
      new Response(JSON.stringify({ error: 'chunk not found' }), { status: 404 }),
    );

    const got = await fetchChunkContent('999');
    expect(got).toBeNull();
  });

  it('throws on 500 with the body.error message', async () => {
    fetchSpy.mockResolvedValue(
      new Response(JSON.stringify({ error: 'boom' }), { status: 500 }),
    );

    await expect(fetchChunkContent('1')).rejects.toThrow('boom');
  });

  it('encodes the chunkId into the path segment', async () => {
    fetchSpy.mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'c-1',
          tier: '权威',
          source: '',
          content: 'x',
          effectiveFrom: 0,
          effectiveTo: 0,
          version: '',
        }),
        { status: 200 },
      ),
    );

    await fetchChunkContent('c-1');
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url, init] = fetchSpy.mock.calls[0];
    expect(String(url)).toBe('/api/knowledge/chunk/c-1');
    // Accept header mirrors useKnowledgeGraph convention.
    expect((init as RequestInit | undefined)?.headers).toEqual({
      Accept: 'application/json',
    });
  });
});
