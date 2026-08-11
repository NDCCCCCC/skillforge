// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChunkSourceView — FEUI-03 / D-01 chunk-source preview. Renders the
// metadata row (TierBadge + 版本: + 生效日期:) at the top, then the
// verbatim chunk text inside a <pre>.
//
// Wave 2 shipped the metadata row + a placeholder body. Wave 3 wires
// the `chunk-lookup` helper so the metadata row reflects the chunk's
// real version / effective date and the body shows the verbatim text
// from the chunks.json fixture (or `未知` / `(chunk not found in
// fixture)` for misses).
import { useMemo } from 'react';
import { lookupChunk } from '../../lib/chunk-lookup';
import TierBadge from '../common/TierBadge';
import type { Tier } from '../../types/verdict';

export interface ChunkSourceViewProps {
  chunkId: string;
  tier: Tier;
  version: string | null;
  effectiveDate: string | null;
}

export default function ChunkSourceView({ chunkId, tier, version, effectiveDate }: ChunkSourceViewProps) {
  // The chunk-lookup helper reads the same fields the parent already
  // has, but it's the canonical source of truth for the verbatim text
  // (the parent props carry the version/effective date for the
  // CenterView; the body text comes from the fixture).
  const result = useMemo(() => lookupChunk(chunkId), [chunkId]);

  return (
    <div data-testid="chunk-source-view" data-chunk-id={chunkId} data-tier={result.tier}>
      <div data-testid="chunk-meta" className="flex items-center gap-3 text-xs text-text-muted">
        <TierBadge tier={tier} />
        <span data-testid="chunk-version">版本: {version ?? '未知'}</span>
        <span data-testid="chunk-effective-date">生效日期: {effectiveDate ?? '未知'}</span>
      </div>
      <pre
        data-testid="chunk-verbatim"
        className="mt-3 whitespace-pre-wrap text-xs text-text font-mono bg-surface-2 p-4 rounded-md"
      >
        {result.text}
      </pre>
    </div>
  );
}
