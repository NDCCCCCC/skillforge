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

import type { Tier } from './verdict';

/** Top-level response shape returned by GET /api/knowledge/graph. */
export interface KnowledgeGraph {
  tiers: TierNode[];
}

/** One tier in the graph. All 4 canonical tiers (权威 / 草稿 / 历史 /
 * 个人) are always emitted, even when the corpus is empty. */
export interface TierNode {
  tier: Tier;
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