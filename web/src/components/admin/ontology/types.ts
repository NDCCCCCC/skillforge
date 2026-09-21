// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — Phase 38 ontology admin surface types. Mirrors the Go
// envelopes in internal/server/admin_ontology.go one-to-one.

export type ProposalKind = 'Class' | 'ObjectProperty' | 'DatatypeProperty';
export type ProposalStatus = 'pending' | 'approved' | 'rejected';

export interface Proposal {
  id: string;
  kind: ProposalKind;
  description_zh: string;
  description_en: string;
  local_name: string;
  proposed_iri: string;
  proposed_triples: string[];
  status: ProposalStatus;
  source_text: string;
  target_skill?: string;
  source_chunk_id?: string;
  created_at: string;
  decided_at?: string;
}

export interface TBoxResponse {
  version: number;
  triple_count: number;
  ttl: string;
}

export interface KGStatsResponse {
  triple_count: number;
  byS_size: number;
  byP_size: number;
  byO_size: number;
  memory_kb_estimate: number;
}

export interface KGTriple {
  s: string;
  p: string;
  o: string;
}

export interface KGQueryResponse {
  matches: KGTriple[];
}

export interface ProposalGenerateResponse {
  proposal: Proposal;
}

export interface ProposalApproveResponse {
  proposal: Proposal;
  tbox_version: number;
}

export interface SnapshotResponse {
  path: string;
  triple_count: number;
}
