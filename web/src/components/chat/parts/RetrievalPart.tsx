// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-04 (PROV-03) -- the retrieval channel summary line
// (58-UI-SPEC Component 3).
//
// Every number comes from the server. A payload that fails the guard
// renders NOTHING: a fabricated "0 命中" would be a lie about a retrieval
// that may well have found plenty.
//
// Two badges can coexist on purpose: "the KB has no ontology" (gray) and
// "the ontology was expected but failed to answer" (amber) are DIFFERENT
// facts, and collapsing them would hide an incident.
import StatusBadge from '../../admin/shared/StatusBadge';
import { isRetrievalStats } from '../../../types/verdict';

export interface RetrievalPartProps {
  data: unknown;
}

export default function RetrievalPart({ data }: RetrievalPartProps) {
  const stats = isRetrievalStats(data) ? data : null;
  if (!stats) return null;

  const anchors = Array.isArray(stats.anchor_entities) ? stats.anchor_entities : [];

  return (
    <div
      data-testid="retrieval-summary"
      aria-label="检索通道分布"
      className="flex flex-wrap items-center gap-2 text-xs text-text-muted"
    >
      <span>
        检索命中 语义 {stats.channels.vector} · 本体 {stats.channels.graph} · 双通道{' '}
        {stats.channels.hybrid}
      </span>
      {anchors.length > 0 && (
        <span data-testid="retrieval-anchors" className="font-mono text-text-dim">
          锚定实体：{anchors.join('、')}
        </span>
      )}
      {stats.kg && stats.kg.available === false && (
        <StatusBadge
          status="unknown"
          tone="gray"
          label="本体增强未启用"
          testId="kg-absent-badge"
        />
      )}
      {stats.degraded === true && (
        <StatusBadge
          status="degraded"
          tone="amber"
          label="本体暂不可用，已降级为纯语义检索"
          testId="kg-degraded-badge"
        />
      )}
    </div>
  );
}
