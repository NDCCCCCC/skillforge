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

// Exported so ChunkSourceView can detect the fixture-miss branch (graph-
// browsed chunks the fixture does not carry) and fire the async content
// fetch against GET /api/knowledge/chunk/:id. The string value is stable
// — it is the body the fixture path renders on a miss, so comparing
// result.text === NOT_FOUND_TEXT is the canonical fixture-miss signal.
export const NOT_FOUND_TEXT = '(chunk not found in fixture)';

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

// Plan 260813-dno / UAT #4 — fetchChunkContent is the on-demand chunk-
// content fetch against GET /api/knowledge/chunk/:id. It is the FE half
// of the T-5-22 graph-payload contract: the graph endpoint stays
// metadata-only, so ChunkSourceView fires this ONLY when the fixture
// lookup misses (i.e. the operator clicked a graph-browsed chunk the
// fixture does not carry). The verdict-citation path never reaches here.
//
// Return contract:
//   - 200 → { text: verbatimContent, tier: Tier | null }
//   - 404 → null (a soft miss — ChunkSourceView renders 「片段不存在」)
//   - other !ok → throws (let the caller render the error state; do NOT swallow)
//
// Mirrors the API-base + Accept-header convention in useKnowledgeGraph.ts
// (same /api/knowledge/... origin; Vite dev proxy handles :8888 in dev).
export async function fetchChunkContent(
  chunkId: string,
): Promise<{ text: string; tier: Tier | null } | null> {
  const resp = await fetch(
    `/api/knowledge/chunk/${encodeURIComponent(chunkId)}`,
    { headers: { Accept: 'application/json' } },
  );
  // A missing chunk is a soft miss, NOT an initialization failure
  // (distinct from the graph hook's 503 handling). Return null so the
  // caller can render a 「片段不存在」 state without surfacing an error.
  if (resp.status === 404) {
    return null;
  }
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    throw new Error(
      (body as { error?: string }).error ??
        `HTTP ${resp.status} ${resp.statusText}`,
    );
  }
  const body = (await resp.json()) as {
    id: string;
    tier: string;
    source: string;
    content: string;
    effectiveFrom: number;
    effectiveTo: number;
    version: string;
  };
  return { text: body.content, tier: (body.tier as Tier) ?? null };
}
