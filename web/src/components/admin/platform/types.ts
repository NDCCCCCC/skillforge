// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — types for the /admin/platform tab.
// Wire shapes mirror the BE DTOs (snake_case JSON, console convention).
// Reuse ProvenanceInfo from governance/types.

import type { ProvenanceInfo } from '../governance/types';

// ── SPARQL types ──────────────────────────────────────────────────────────

export interface SparqlPattern {
  s?: string;
  p?: string;
  o?: string;
  limit?: number;
  offset?: number;
  filter?: string;
}

export interface SparqlResult {
  s: string;
  p: string;
  o: string;
  provenance?: ProvenanceInfo;
}

export interface SparqlResponse {
  results: SparqlResult[];
  count: number;
  truncated: boolean;
}

// ── MCP types ─────────────────────────────────────────────────────────────

export type McpToolStatus = 'online' | 'offline';

export interface McpTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}

export interface McpServerStatus {
  status: McpToolStatus;
  service_skill: string;
}

export interface McpInvokeRequest {
  tool: string;
  params: Record<string, unknown>;
}

export interface McpInvokeResponse {
  result?: unknown;
  error?: string;
}

// ── Hybrid types ──────────────────────────────────────────────────────────

export interface HybridQuery {
  query: string;
  max_hops?: 1 | 2;
  rerank?: boolean;
}

export interface HybridKGNode {
  id: string;
  label: string;
  kind: string;
  tier?: string;
  provenance?: ProvenanceInfo;
}

export interface HybridKGEdge {
  source: string;
  target: string;
}

export interface HybridMilvusHit {
  score: number;
  text: string;
  source_path?: string;
  section?: string;
}

export interface HybridResponse {
  kg_nodes: HybridKGNode[];
  kg_edges: HybridKGEdge[];
  milvus_hits: HybridMilvusHit[];
}
