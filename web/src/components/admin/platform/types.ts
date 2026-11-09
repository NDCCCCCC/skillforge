// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — types for the /admin/platform tab.
// Wire shapes mirror the BE DTOs (snake_case JSON, console convention).
// Reuse ProvenanceInfo from governance/types.

import type { ProvenanceInfo } from '../governance/types';

// ── 三元组查询 types（KG triples）────────────────────────────────────────
//
// 后端把 /api/sparql 改名为 /api/kg/triples，并把这个查询面从
// "SPARQL 查询" 纠正为"三元组模式查询"：s / p / o 三段各自留空即
// 通配匹配，再叠加一个可选 filter。前端类型名同步跟过去 —— 继续
// 叫 Sparql* 就是在宣称一个并不存在的 SPARQL 引擎能力。

export interface TriplePattern {
  s?: string;
  p?: string;
  o?: string;
  limit?: number;
  offset?: number;
  filter?: string;
}

export interface KGTripleResult {
  s: string;
  p: string;
  o: string;
  provenance?: ProvenanceInfo;
}

export interface KGTripleResponse {
  results: KGTripleResult[];
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
