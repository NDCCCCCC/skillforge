// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// chunk-lookup — FEUI-03 / KNOW-05 chunk metadata + verbatim lookup.
//
// Reads the chunks.json fixture via Vite's JSON import and returns the
// metadata (tier / version / effective date) and the verbatim text
// for a given chunk_id. Used by:
//   - ChunkSourceView (renders metadata row + verbatim text in the
//     center panel after a citation click — D-01)
//   - CitationCluster (calls extractChunkMetadata on click to populate
//     the CenterView with version + effectiveDate without pulling the
//     full text)
//
// MISS policy: a chunk_id not present in the fixture is a development
// bug (the citation should match a chunk we shipped). Returns
// `tier: '权威'` as a safe fallback (matches the demo Skill's reads)
// and `version: null` / `effectiveDate: null` so the ChunkSourceView
// renders `版本: 未知` / `生效日期: 未知` per 04-UI-SPEC §FEUI-03. A
// console.warn surfaces the missing chunk so the dev can spot it
// immediately in the browser console.
import type { Tier } from '../types/verdict';
import chunksData from '../__fixtures__/chunks.json';

interface ChunkFixtureEntry {
  tier: Tier;
  version: string;
  effective_date: string;
  text: string;
}

const CHUNKS: Record<string, ChunkFixtureEntry> = chunksData as Record<string, ChunkFixtureEntry>;

export interface ChunkLookupResult {
  chunkId: string;
  tier: Tier;
  version: string | null;
  effectiveDate: string | null;
  text: string;
}

export interface ChunkMetadata {
  tier: Tier;
  version: string | null;
  effectiveDate: string | null;
}

const NOT_FOUND_TEXT = '(chunk not found in fixture)';

export function lookupChunk(chunkId: string): ChunkLookupResult {
  const entry = CHUNKS[chunkId];
  if (entry) {
    return {
      chunkId,
      tier: entry.tier,
      version: entry.version,
      effectiveDate: entry.effective_date,
      text: entry.text,
    };
  }
  // eslint-disable-next-line no-console
  console.warn('[chunk-lookup] missing chunk:', chunkId);
  return {
    chunkId,
    tier: '权威',
    version: null,
    effectiveDate: null,
    text: NOT_FOUND_TEXT,
  };
}

export function extractChunkMetadata(chunkId: string): ChunkMetadata {
  const entry = CHUNKS[chunkId];
  if (entry) {
    return {
      tier: entry.tier,
      version: entry.version,
      effectiveDate: entry.effective_date,
    };
  }
  return { tier: '权威', version: null, effectiveDate: null };
}
