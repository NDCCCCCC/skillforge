// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 260813-dno / UAT #4 — tests for fetchChunkContent.
//
// Pins the 3 acceptance assertions for the on-demand chunk-content fetch:
//   1. 200 → resolves { text, tier } from the ChunkPayload body.
//   2. 404 → resolves null (soft miss, distinct from the graph hook's 503 handling).
//   3. other !ok → rejects with the body.error (or HTTP status fallback).
//
// Quick 260815-jpu (Layer 4) extends this file with the normalizeChunkID
// pure-fn block (14 cases), lookupChunk-with-normalize (CHUNK 7 / ref-7
// hit the "7" fixture key), and fetchChunkContent-with-normalize (the
// "CHUNK 7" URL is `/api/knowledge/chunk/7` after normalize).
//
// fetch is spied on globalThis and restored between tests so the mock
// state never bleeds across cases (and never leaks into ChunkSourceView's
// tests, which set up their own fetch mock).
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { fetchChunkContent, lookupChunk, normalizeChunkID, NOT_FOUND_TEXT } from '../lib/chunk-lookup';

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

// Quick 260815-jpu (Layer 4) — normalizeChunkID is a pure function that
// strips common LLM-emitted prefixes from a chunk ID. Non-numeric input
// is returned UNCHANGED (so the BE parseLenientChunkID can still 400).
describe('normalizeChunkID', () => {
  it.each([
    ['7', '7'],
    ['CHUNK 7', '7'],
    ['chunk 7', '7'],
    ['CHUNK7', '7'],
    ['ref-7', '7'],
    ['REF-7', '7'],
    ['[ref-7]', '7'],
    ['[CHUNK 7]', '7'],
    ['  7  ', '7'],
    ['"7"', '7'],
    ["'7'", '7'],
    ['abc', 'abc'],
    ['', ''],
    ['CHUNK abc', 'CHUNK abc'],
    ['c-1', 'c-1'],
  ])('normalizeChunkID(%j) === %j', (input, expected) => {
    expect(normalizeChunkID(input)).toBe(expected);
  });
});

// Quick 260815-jpu (Layer 4) — verify normalizeChunkID is wired into
// lookupChunk: a prefixed input that resolves to a fixture key returns
// the real entry, not NOT_FOUND_TEXT.
describe('lookupChunk with normalize', () => {
  it('CHUNK 7 in fixture → hits key "7"', () => {
    const got = lookupChunk('CHUNK 7');
    expect(got.text).not.toBe(NOT_FOUND_TEXT);
    expect(got.chunkId).toBe('7');
  });

  it('ref-7 in fixture → hits key "7"', () => {
    const got = lookupChunk('ref-7');
    expect(got.text).not.toBe(NOT_FOUND_TEXT);
  });
});

// Quick 260815-jpu (Layer 4) — verify normalizeChunkID is wired into
// fetchChunkContent: a prefixed input is normalized before the URL is
// built, so the BE only ever sees the bare numeric ID.
describe('fetchChunkContent with normalize', () => {
  let fetchSpy: MockInstance;
  beforeEach(() => {
    fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes CHUNK 7 → path /api/knowledge/chunk/7', async () => {
    fetchSpy.mockResolvedValue(new Response(JSON.stringify({
      id: '7', tier: '权威', source: '', content: 'x', effectiveFrom: 0, effectiveTo: 0, version: '',
    }), { status: 200 }));
    await fetchChunkContent('CHUNK 7');
    const [url] = fetchSpy.mock.calls[0];
    expect(String(url)).toBe('/api/knowledge/chunk/7');
  });
});
