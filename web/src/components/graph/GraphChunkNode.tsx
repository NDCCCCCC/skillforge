// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-5 (KNOW-06) — GraphChunkNode.
//
// One row in the chunk list. Renders the chunk ID + the version badge
// (当前有效 / 已失效). Click → setCenterView({kind:'chunk-source', ...})
// so the operator can browse the chunk verbatim in ChunkSourceView.
//
// Bidirectional nav: clicking 引用原文 tab returns to the same chunk
// (the CenterPanel's tab indicator is bound to centerView.kind).

import VersionBadge from '../center/VersionBadge';
import { useAppStore } from '../../store/app-store';
import type { Tier } from '../../types/verdict';

export interface GraphChunkNodeProps {
  chunkId: string;
  effectiveFrom: number | null;
  effectiveTo: number | null;
  version: string | null;
  tier: Tier;
}

export default function GraphChunkNode({
  chunkId,
  effectiveFrom,
  effectiveTo,
  version,
  tier,
}: GraphChunkNodeProps) {
  const handleClick = () => {
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier,
      version: version ?? null,
      effectiveDate: effectiveFrom
        ? new Date(effectiveFrom * 1000).toISOString().slice(0, 10)
        : null,
      effectiveTo,
    });
  };

  return (
    <button
      type="button"
      data-testid="graph-chunk-node"
      data-chunk-id={chunkId}
      data-tier={tier}
      onClick={handleClick}
      className="flex w-full items-center justify-between gap-2 rounded-md border border-border bg-surface-1 px-2 py-1 text-left text-xs hover:bg-surface-2"
      title={chunkId}
    >
      <span className="font-mono text-text truncate">{chunkId}</span>
      <VersionBadge
        effectiveFrom={effectiveFrom}
        effectiveTo={effectiveTo}
      />
    </button>
  );
}