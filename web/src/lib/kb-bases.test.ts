// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 1 — the KB registry projection client (26-05 wire).
//
// Pins:
//   - fetchKnowledgeBases: ONE GET /api/knowledge/bases per session (module
//     promise cache), degrading to [] on network failure / non-2xx (a 503 is
//     "registry unwired" per D26-05-B) / malformed body — never throwing.
//   - displayNameFor: registry display name for a known slug, `[slug]` on a
//     miss (the bracket is part of the contract — it is the badge form).
//   - firstSlugByRole: first active slug of a semantic role, '' when absent.
//   - fallbackTierLabel: the seed-derived offline map + `[slug]` degradation.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The module caches its projection promise at module scope; every test
// re-imports after vi.resetModules() so the cache never bleeds across cases.
async function fresh() {
  vi.resetModules();
  return await import('./kb-bases');
}

function basesOk(bases: unknown[]): Response {
  return new Response(JSON.stringify({ bases }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

const SEED_BASES = [
  { slug: 'authority', display_name: '权威', role: 'authority' },
  { slug: 'historical', display_name: '历史', role: 'historical' },
  { slug: 'kb-2026', display_name: '新库', role: 'authority' },
];

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchKnowledgeBases', () => {
  it('fetches the public projection once and caches it module-wide', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => Promise.resolve(basesOk(SEED_BASES)));
    vi.stubGlobal('fetch', fetchMock);
    const mod = await fresh();

    const first = await mod.fetchKnowledgeBases();
    const second = await mod.fetchKnowledgeBases();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String(fetchMock.mock.calls[0][0])).toBe('/api/knowledge/bases');
    expect(first).toEqual(SEED_BASES);
    expect(second).toBe(first); // same array identity → cached promise
  });

  it('degrades a failed projection to [] and never throws', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('network down'))));
    const mod = await fresh();
    await expect(mod.fetchKnowledgeBases()).resolves.toEqual([]);
  });

  it('degrades a 503 (registry unwired, D26-05-B) to []', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          new Response(JSON.stringify({ error: 'kb registry not wired' }), { status: 503 }),
        ),
      ),
    );
    const mod = await fresh();
    await expect(mod.fetchKnowledgeBases()).resolves.toEqual([]);
  });

  it('degrades a malformed body (no bases array) to []', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response(JSON.stringify({ nope: true }), { status: 200 }))),
    );
    const mod = await fresh();
    await expect(mod.fetchKnowledgeBases()).resolves.toEqual([]);
  });
});

describe('displayNameFor', () => {
  it('returns the registry display name (bracketed) for a known slug', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(basesOk(SEED_BASES))));
    const mod = await fresh();
    await expect(mod.displayNameFor('kb-2026')).resolves.toBe('[新库]');
  });

  it('returns the bracketed slug on a registry miss', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(basesOk(SEED_BASES))));
    const mod = await fresh();
    await expect(mod.displayNameFor('ghost-kb')).resolves.toBe('[ghost-kb]');
  });

  it('returns the bracketed slug when the projection is unavailable', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('down'))));
    const mod = await fresh();
    await expect(mod.displayNameFor('authority')).resolves.toBe('[authority]');
  });
});

describe('firstSlugByRole', () => {
  it('returns the first slug carrying the requested semantic role', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(basesOk(SEED_BASES))));
    const mod = await fresh();
    // authority appears before the later kb-2026 in projection order.
    await expect(mod.firstSlugByRole('authority')).resolves.toBe('authority');
    await expect(mod.firstSlugByRole('historical')).resolves.toBe('historical');
  });

  it("returns '' when no base carries the role (no CN literal default)", async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(basesOk([]))));
    const mod = await fresh();
    await expect(mod.firstSlugByRole('authority')).resolves.toBe('');
  });
});

describe('fallbackTierLabel', () => {
  it('serves the seed-derived offline display name for the 4 baseline slugs', async () => {
    const mod = await fresh();
    expect(mod.fallbackTierLabel('authority')).toBe('[权威]');
    expect(mod.fallbackTierLabel('historical')).toBe('[历史]');
    expect(mod.fallbackTierLabel('personal')).toBe('[个人]');
    expect(mod.fallbackTierLabel('draft')).toBe('[草稿]');
  });

  it('degrades any other slug (and legacy CN values) to a bracketed label', async () => {
    const mod = await fresh();
    expect(mod.fallbackTierLabel('kb-2026')).toBe('[kb-2026]');
    expect(mod.fallbackTierLabel('权威')).toBe('[权威]');
  });
});
