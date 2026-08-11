// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// CitationCluster — FEUI-03 cluster of clickable citation chips
// rendered BELOW the verdict body (and the citations count footer).
//
// Each chip is a `<button>` (NOT a link) that signals the CenterPanel
// to swap to the chunk-source view (D-01). The store action
// `setCenterView({ kind: 'chunk-source', chunkId, tier, version,
// effectiveDate })` is invoked via `useAppStore.getState()` directly
// — no prop drilling required since Zustand is a singleton.
//
// Wave 3 calls `extractChunkMetadata(chunkId)` from `lib/chunk-lookup`
// so the chip carries the real version + effective date (rather than
// the `null` placeholders Wave 2 shipped).
import { useCallback } from 'react';
import { extractChunkMetadata } from '../../lib/chunk-lookup';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { Citation } from '../../types/verdict';
import TierBadge from '../common/TierBadge';

export interface CitationClusterProps {
  citations: Citation[];
}

export default function CitationCluster({ citations }: CitationClusterProps) {
  const handleClick = useCallback((chunkId: string, tier: Citation['tier']) => {
    const meta = extractChunkMetadata(chunkId);
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier,
      version: meta.version,
      effectiveDate: meta.effectiveDate,
    });
  }, []);

  return (
    <div data-testid="citation-cluster" className="flex flex-wrap gap-2">
      {citations.map((citation) => (
        <button
          key={citation.chunk_id}
          type="button"
          className={cn('citation-chip')}
          data-testid="citation-chip"
          data-tier={citation.tier}
          data-chunk-id={citation.chunk_id}
          onClick={() => handleClick(citation.chunk_id, citation.tier)}
        >
          <TierBadge tier={citation.tier} />
          <span>{citation.span}</span>
          <span className="text-text-dim">{citation.chunk_id}</span>
        </button>
      ))}
    </div>
  );
}
