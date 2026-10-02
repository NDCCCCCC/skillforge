// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-5 (KNOW-06) — TS mirror of internal/knowledge.Graph.
//
// The wire shape returned by GET /api/knowledge/graph. Three nested
// levels (tier -> doc -> chunk) with effectiveFrom / effectiveTo +
// version fields per chunk. Mirrors Go Graph / TierNode / DocNode /
// ChunkNode JSON tags verbatim.
//
// IMPORTANT: chunk content is NOT in this type — the graph endpoint
// deliberately returns metadata only (threat T-5-22). The chunk
// verbatim opens separately via ChunkSourceView.

/** Top-level response shape returned by GET /api/knowledge/graph. */
export interface KnowledgeGraph {
  tiers: TierNode[];
}

/** One tier (= one registered knowledge base) in the graph. The BE emits
 * one entry per registry tier — the fixed 4-tier assumption is gone
 * (Phase 26 / KBM-02), so the FE lays out N tiers. `tier` is the registry
 * slug; a corpus replayed from a historical audit row may carry a
 * pre-migration CN value, which the FE degrades to a bracketed label. */
export interface TierNode {
  tier: string;
  docCount: number;
  chunkCount: number;
  docs: DocNode[];
}

/** Groups all chunks belonging to the same source path. */
export interface DocNode {
  /** Stable synthetic key — tier + "::" + source. */
  id: string;
  /** Basename of the source path (e.g. "contract-approval.md"). */
  filename: string;
  chunkCount: number;
  chunks: ChunkNode[];
}

/** Metadata for one chunk. No content here. */
export interface ChunkNode {
  /** Numeric Milvus/Mongo PK rendered as a decimal string. */
  id: string;
  /** Epoch seconds. 0 = perpetual / open-ended. */
  effectiveFrom: number | null;
  /** Epoch seconds. 0 = perpetual / open-ended. < now = superseded. */
  effectiveTo: number | null;
  /** Human-readable version label from the BE; empty when no
   * effective_* window is set. */
  version: string | null;
}

/** True when a chunk is currently active (not superseded). The FE
 * uses this to decide whether to render the 当前有效 BadgeCheck or the
 * 已失效 strikethrough (per UI-SPEC §SC-8). */
export function isChunkActive(
  effectiveTo: number | null,
  nowMs: number = Date.now(),
): boolean {
  if (effectiveTo === null || effectiveTo === 0) {
    return true;
  }
  // effectiveTo is epoch seconds; nowMs is epoch ms. Compare in seconds.
  return effectiveTo * 1000 >= nowMs;
}

/** Edge type constants (Phase 28 — Plan 28-05). */
export const EDGE_TYPE_SECTION_ADJACENT = 'section_adjacent';
export const EDGE_TYPE_VERSION_CHAIN = 'version_chain';
export const EDGE_TYPE_KEYWORDCOOCCUR = 'keyword_cooccur';
export const EDGE_TYPE_CITATIONCOOCCUR = 'citation_cooccur';

export type EdgeType =
  | typeof EDGE_TYPE_SECTION_ADJACENT
  | typeof EDGE_TYPE_VERSION_CHAIN
  | typeof EDGE_TYPE_KEYWORDCOOCCUR
  | typeof EDGE_TYPE_CITATIONCOOCCUR;

/** One edge in the graph relation layer. */
export interface GraphEdge {
  from: string;
  to: string;
  edge_type: EdgeType;
  weight: number;
  tier?: string;
}

/** One node returned by the neighbors API. */
export interface EdgeNode {
  chunk_id: string;
  tier: string;
  source: string;
  effectiveFrom: number | null;
  effectiveTo: number | null;
}

/** Response from GET /api/knowledge/graph/neighbors */
export interface NeighborResult {
  chunk_id: string;
  depth: number;
  nodes: EdgeNode[];
  edges: GraphEdge[];
  degraded: boolean;
  degraded_reason: string;
}

/** Response from GET /api/knowledge/graph/path */
export interface PathResult {
  from: string;
  to: string;
  max_hops: number;
  hops: number;
  path: string[];
  edges: GraphEdge[];
  degraded: boolean;
  degraded_reason: string;
}

/** Extended KnowledgeGraph with degraded flag (GRF-05, Phase 28-04). */
export interface KnowledgeGraph {
  tiers: TierNode[];
  degraded?: boolean;
  degradedReason?: string;
}