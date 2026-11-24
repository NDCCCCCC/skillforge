// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 57-02 (ONT-02) -- the ontology anchor line + degraded badge
// (57-UI-SPEC section 2.1).
//
// Three DISTINCT states, never collapsed into one:
//   1. kgAvailable === false -> 本体未接入（本次仅向量检索）+ amber badge
//      (the KG is absent: an operator-visible degradation, not a silent
//       gap and not a "zero entities found").
//   2. kgAvailable === true, entitiesMatched === 0 -> 本体锚定 · 未识别到实体
//      (the dictionary works; the question simply named nothing in it).
//   3. entitiesMatched > 0 -> 本体锚定 · <labels joined by ' / '>
//
// verbatim rendering only: the labels come from the wire, the FE never
// parses or derives them.
import { Network } from 'lucide-react';

export interface KGAnchorLineProps {
  kgAvailable?: boolean;
  entitiesMatched?: number;
  anchorEntities?: string;
}

export default function KGAnchorLine({
  kgAvailable,
  entitiesMatched,
  anchorEntities,
}: KGAnchorLineProps) {
  // No ontology_anchor event yet -> render nothing (the inspector's own
  // loading/empty states own that moment).
  if (kgAvailable === undefined) return null;

  if (!kgAvailable) {
    return (
      <div
        data-testid="kg-anchor-line"
        data-kg-available="false"
        aria-live="polite"
        className="flex flex-wrap items-center gap-2 text-xs text-text-muted"
      >
        <Network className="h-3.5 w-3.5 text-text-dim" />
        <span>本体未接入（本次仅向量检索）</span>
        <span
          data-testid="kg-degraded-badge"
          role="status"
          aria-label="知识图谱不可用"
          className="rounded-full bg-status-needs-info/15 px-2 py-0.5 text-[11px] font-semibold text-status-needs-info"
        >
          知识图谱不可用
        </span>
      </div>
    );
  }

  const matched = typeof entitiesMatched === 'number' ? entitiesMatched : 0;
  if (matched === 0 || !anchorEntities) {
    return (
      <div
        data-testid="kg-anchor-line"
        data-kg-available="true"
        aria-live="polite"
        className="flex flex-wrap items-center gap-2 text-xs text-text-muted"
      >
        <Network className="h-3.5 w-3.5 text-text-dim" />
        <span>本体锚定 · 未识别到实体</span>
      </div>
    );
  }

  return (
    <div
      data-testid="kg-anchor-line"
      data-kg-available="true"
      data-entities-matched={matched}
      aria-live="polite"
      className="flex flex-wrap items-center gap-2 text-xs text-text-muted"
    >
      <Network className="h-3.5 w-3.5 text-primary" />
      <span>本体锚定 · {anchorEntities}</span>
    </div>
  );
}
