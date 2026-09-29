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
// bug (the citation should match a chunk we shipped). A miss returns
// `tier: null` — NO hardcoded tier substitution (Phase 26 / 26-08 deleted
// the old hardcoded authority-tier fallback: after the registry migration
// that CN literal matches no slug, so inventing a tier would fabricate
// evidence) — plus `version: null` / `effectiveDate: null` so the
// ChunkSourceView renders `版本: 未知` / `生效日期: 未知` per 04-UI-SPEC
// §FEUI-03 and the tier badge stays hidden. A console.warn surfaces the
// missing chunk so the dev can spot it immediately in the browser console.
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
  /** Registry slug from the fixture; null = no tier data (badge hidden). */
  tier: Tier | null;
  version: string | null;
  effectiveDate: string | null;
  text: string;
}

export interface ChunkMetadata {
  /** Registry slug from the fixture; null = no tier data (badge hidden). */
  tier: Tier | null;
  version: string | null;
  effectiveDate: string | null;
}

// Exported so ChunkSourceView can detect the fixture-miss branch (graph-
// browsed chunks the fixture does not carry) and fire the async content
// fetch against GET /api/knowledge/chunk/:id. The string value is stable
// — it is the body the fixture path renders on a miss, so comparing
// result.text === NOT_FOUND_TEXT is the canonical fixture-miss signal.
export const NOT_FOUND_TEXT = '(chunk not found in fixture)';

/**
 * normalizeChunkID strips common LLM-emitted or human-typed prefixes
 * from a chunk ID so the canonical numeric form ("7") is what reaches
 * the BE URL path. The BE chunkHandler also accepts prefixed forms
 * (Layer 3 of quick 260815-jpu), so this is a UX optimization — the
 * popover fetch fires immediately instead of waiting for the BE WARN
 * log path.
 *
 * Accepted transformations:
 *   - trim outer whitespace
 *   - strip matching brackets/parens/quotes pairs (handles [[7]], etc.)
 *   - strip case-insensitive "CHUNK"/"REF" prefix with optional
 *     separator (space / dash / nothing)
 *
 * Non-numeric input is returned UNCHANGED — the BE chunkHandler will
 * reject it with a 400, which is the correct boundary: the FE has no
 * way to know if an arbitrary string is a valid chunk ID.
 */
export function normalizeChunkID(chunkId: string): string {
  let s = chunkId.trim();
  // Strip matching pairs of brackets/parens/quotes (repeated for nested forms).
  // Wrappers are always safe to strip — they don't change numeric-ness.
  for (;;) {
    let changed = false;
    if (s.length >= 2) {
      const first = s[0];
      const last = s[s.length - 1];
      if ((first === '[' && last === ']') ||
          (first === '(' && last === ')') ||
          (first === '"' && last === '"') ||
          (first === "'" && last === "'")) {
        s = s.slice(1, -1).trim();
        changed = true;
      }
    }
    if (!changed) break;
  }
  // Try CHUNK/REF prefix strip — but ONLY if the stripped remainder is
  // a valid int64. If the prefix syntactically applies but the rest is
  // not numeric (e.g. "CHUNK abc"), return the input UNCHANGED so the
  // BE parseLenientChunkID can still 400 on it — the FE has no way to
  // know if "CHUNK abc" is a typo vs a legit key.
  const lower = s.toLowerCase();
  for (const prefix of ['chunk', 'ref']) {
    if (lower.startsWith(prefix)) {
      const rest = s.slice(prefix.length);
      if (rest.length === 0 || rest[0] === ' ' || rest[0] === '-' || (rest[0] >= '0' && rest[0] <= '9')) {
        const stripped = rest.replace(/^[\s-]+/, '');
        if (/^-?\d+$/.test(stripped)) {
          return stripped;
        }
        // Prefix could be stripped but remainder is non-numeric — bail.
        return chunkId;
      }
    }
  }
  s = s.trim();
  return s || chunkId; // normalize 后为空则原样返回
}

export function lookupChunk(chunkId: string): ChunkLookupResult {
  const normalized = normalizeChunkID(chunkId);
  const entry = CHUNKS[normalized];
  if (entry) {
    return {
      chunkId: normalized, // sync 规范化后的 ID 写到返回，让下游显示一致
      tier: entry.tier,
      version: entry.version,
      effectiveDate: entry.effective_date,
      text: entry.text,
    };
  }
  // eslint-disable-next-line no-console
  console.warn('[chunk-lookup] missing chunk:', chunkId);
  return {
    chunkId, // fixture miss 路径保留原文（让 console.warn 打原文便于诊断）
    tier: null, // no tier data → the renderer hides the badge (no substitute)
    version: null,
    effectiveDate: null,
    text: NOT_FOUND_TEXT,
  };
}

export function extractChunkMetadata(chunkId: string): ChunkMetadata {
  const normalized = normalizeChunkID(chunkId);
  const entry = CHUNKS[normalized];
  if (entry) {
    return {
      tier: entry.tier,
      version: entry.version,
      effectiveDate: entry.effective_date,
    };
  }
  return { tier: null, version: null, effectiveDate: null };
}

// Plan 260813-dno / UAT #4 — fetchChunkContent is the on-demand chunk-
// content fetch against GET /api/knowledge/chunk/:id. It is the FE half
// of the T-5-22 graph-payload contract: the graph endpoint stays
// metadata-only, so ChunkSourceView fires this ONLY when the fixture
// lookup misses (i.e. the operator clicked a graph-browsed chunk the
// fixture does not carry). The verdict-citation path never reaches here.
//
// Return contract:
//   - 200 → { text: verbatimContent, tier: Tier | null, source: raw path }
//   - 404 → null (a soft miss — ChunkSourceView renders 「片段不存在」)
//   - other !ok → throws (let the caller render the error state; do NOT swallow)
//
// Quick 260929-d5w2 — the payload's raw `source` path is surfaced too so
// the citation / chunk-source views can render the pretty short form
// (prettySourcePath) while keeping the raw path for title attributes.
//
// Mirrors the API-base + Accept-header convention in useKnowledgeGraph.ts
// (same /api/knowledge/... origin; Vite dev proxy handles :8888 in dev).
export async function fetchChunkContent(
  chunkId: string,
): Promise<{ text: string; tier: Tier | null; source: string | null } | null> {
  const normalized = normalizeChunkID(chunkId);
  const resp = await fetch(
    `/api/knowledge/chunk/${encodeURIComponent(normalized)}`,
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
  return {
    text: body.content,
    tier: (body.tier as Tier) ?? null,
    source: body.source ?? null,
  };
}
