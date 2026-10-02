// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-g7x — noteIdsFromFocus pins: citation source basename
// leading digits = ECL source_idx = the ontology note local name.
// Follow-up: resolveFocusNoteIds pins the chunk_id → source → note async
// path (wire citations carry ONLY chunk_id/span/tier).
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  noteIdsFromFocus,
  resolveFocusNoteIds,
  type GraphFocus,
} from './graph-focus';

function focusWith(...sources: string[]): GraphFocus {
  return { chunkIds: new Set(['c1']), sources };
}

/** BE wire shape of GET /api/knowledge/chunk/:id. */
function chunkBody(source: string | null) {
  return {
    id: 'x',
    tier: 'sql-guides',
    source: source ?? '',
    content: 'SELECT 1',
    effectiveFrom: 0,
    effectiveTo: 0,
    version: 'v1',
  };
}

/** Fetch stub keyed by chunk URL → source (null = 404 soft-miss). */
function stubChunkSources(byId: Record<string, string | null | 'throw'>) {
  const fn = vi.fn().mockImplementation(async (url: string) => {
    const id = decodeURIComponent(url.split('/').pop() ?? '');
    const source = byId[id];
    if (source === 'throw') {
      throw new Error('network down');
    }
    if (source === undefined || source === null) {
      return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({}) };
    }
    return { ok: true, status: 200, json: async () => chunkBody(source) };
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('noteIdsFromFocus', () => {
  it('derives note_XXX from the leading digits of each source basename', () => {
    expect(noteIdsFromFocus(focusWith('D:/corpus/032_保单号查支付方式与开户行.md'))).toEqual([
      'note_032',
    ]);
  });

  it('dedupes and preserves order across multiple sources', () => {
    expect(
      noteIdsFromFocus(
        focusWith('a/032_x.md', 'b\\032_y.md', '007_note.md', 'notes/12.md'),
      ),
    ).toEqual(['note_032', 'note_007', 'note_12']);
  });

  it('ignores sources without a leading digit', () => {
    expect(noteIdsFromFocus(focusWith('README.md', 'x/abc_032.md'))).toEqual([]);
  });

  it('returns empty for null/empty focus (the common case while the BE '
    + 'does not yet enrich citations with source paths)', () => {
    expect(noteIdsFromFocus(null)).toEqual([]);
    expect(noteIdsFromFocus(undefined)).toEqual([]);
    expect(noteIdsFromFocus({ chunkIds: new Set(['pk1']), sources: [] })).toEqual([]);
  });
});

describe('resolveFocusNoteIds', () => {
  it('resolves chunk_ids through fetchChunkContent and derives notes from the source basename', async () => {
    stubChunkSources({
      '1032': 'D:/corpus/032_保单号查支付方式与开户行.md',
      '1007': '/kb/sql-guides/007_机构部门.md',
    });
    await expect(resolveFocusNoteIds(['1032', '1007'])).resolves.toEqual([
      'note_032',
      'note_007',
    ]);
  });

  it('skips 404 soft-misses, thrown fetches, and digit-less sources per chunk', async () => {
    stubChunkSources({
      '1': null, // 404 → fetchChunkContent resolves null
      '2': 'throw', // network failure
      '3': 'README.md', // no leading digits
      '4': 'D:/corpus/032_保单.md',
    });
    await expect(resolveFocusNoteIds(['1', '2', '3', '4'])).resolves.toEqual(['note_032']);
  });

  it('dedupes multiple chunks of the same note in first-seen order', async () => {
    stubChunkSources({
      '9': 'x/032_a.md',
      '8': 'y/032_b.md',
      '7': 'z/007_c.md',
    });
    await expect(resolveFocusNoteIds(['9', '8', '7'])).resolves.toEqual(['note_032', 'note_007']);
  });

  it('degrades to [] when EVERY chunk fails — caller falls back to 全库', async () => {
    stubChunkSources({ a: null, b: 'throw' });
    await expect(resolveFocusNoteIds(['a', 'b'])).resolves.toEqual([]);
  });

  it('returns [] without fetching for an empty chunk list', async () => {
    const fetchMock = stubChunkSources({});
    await expect(resolveFocusNoteIds([])).resolves.toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
