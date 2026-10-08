// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07a — types for the /admin/governance tab shell.
// Wire shapes mirror the BE DTOs (snake_case JSON, console convention).
// These are consumed by GovernanceTab, GovernanceApi, and the view
// components (ShaclReportView / ProvenanceGraphView / TimelineView).

export type ShaclHealth = 'ok' | 'warn' | 'error';
export type ViolationSeverity = 'error' | 'warning';
/** D-46-03: layer stamped by validate.go; locked by 46-UI-SPEC §Color */
export type ViolationLayer = 'tbox' | 'entity' | 'global';

export interface ShaclViolation {
  severity: ViolationSeverity;
  /** entity URI; '(global)' for shape-level violations */
  entity: string;
  /** sh:minCount | sh:class | provenance-missing | … */
  constraint: string;
  /** engine message, rendered verbatim */
  detail: string;
  /** SC-1: source file path */
  source_path?: string;
  /** D-PROV-05: 1-based line number */
  line_start?: number;
  line_end?: number;
  /** D-46-03: validate.go stamping 点盖戳；legacy/未盖戳 violation 缺省 */
  layer?: ViolationLayer;
  /** NodeShape TargetClass（skf:InsuranceProduct / skf:Organization / skf:NoteChunk …）；layer=global 时缺省 */
  target_class?: string;
}

export interface QuarantinedEntity {
  entity: string;
  triple_count: number;
  /** D-46-04: 只读计数——指向该隔离实体的入边数量（对象位可见）；BE 恒下发，未下发时缺省 */
  dangling_edge_count?: number;
  violations: ShaclViolation[];
}

export interface ShaclReport {
  health: ShaclHealth;
  error_count: number;
  warning_count: number;
  warnings: ShaclViolation[];
  quarantined: QuarantinedEntity[];
  triple_count: number;
  /** zero-hardcode: surface the configured shapes path */
  shapes_path: string;
  /** RFC3339 UTC */
  validated_at: string;
  /** boot | rollback */
  trigger: 'boot' | 'rollback';
}

export type ProvenanceKind = 'file' | 'emitter' | 'induction';

export interface ProvenanceInfo {
  source_kind: ProvenanceKind;
  /** file path | emitter:v{N} | proposal id (D-PROV-02) */
  source_path: string;
  /** 1-based line range (D-PROV-05) */
  line_start?: number;
  line_end?: number;
  /** RFC3339 UTC (D-PROV-11) */
  extracted_at: string;
}

export type GraphNodeKind = 'file' | 'note' | 'table' | 'skill' | 'skillmd' | 'quarantined';

export interface GraphNode {
  id: string;
  kind: GraphNodeKind;
  label: string;
  /** absent on file/skillmd nodes (they ARE the source) */
  provenance?: ProvenanceInfo;
  /** belongsToTier projection, when present */
  tier?: string;
}

export interface GraphEdge {
  source: string;
  target: string;
}

export interface ProvenanceGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export type ChangelogWho = 'admin' | 'induction:llm' | 'system:rollback';

export interface ChangelogEntry {
  /** resulting version v{N} */
  version: number;
  who: ChangelogWho;
  /** RFC3339 */
  when: string;
  what: { added: string[]; removed: string[] };
  /** LLM reasoning | admin note | rollback target+reason */
  why: string;
}
