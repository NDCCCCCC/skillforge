// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-g7x — noteIdsFromFocus pins: citation source basename
// leading digits = ECL source_idx = the ontology note local name.
import { describe, expect, it } from 'vitest';
import { noteIdsFromFocus, type GraphFocus } from './graph-focus';

function focusWith(...sources: string[]): GraphFocus {
  return { chunkIds: new Set(['c1']), sources };
}

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
