// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChunkSourceView — FEUI-03 / D-01 chunk-source preview. Renders the
// metadata row (TierBadge + 版本: + 生效日期:) at the top, then the
// verbatim chunk text inside a <pre>.
//
// Wave 2 ships the metadata row + a placeholder body. Wave 3 wires
// the metadata lookup helper so the placeholder is replaced with the
// real chunk text from the chunks.json fixture.
import TierBadge from '../common/TierBadge';
import type { Tier } from '../../types/verdict';

export interface ChunkSourceViewProps {
  chunkId: string;
  tier: Tier;
  version: string | null;
  effectiveDate: string | null;
}

export default function ChunkSourceView({ chunkId, tier, version, effectiveDate }: ChunkSourceViewProps) {
  return (
    <div>
      <div className="flex items-center gap-3 text-xs text-text-muted">
        <TierBadge tier={tier} />
        <span>版本: {version ?? '未知'}</span>
        <span>生效日期: {effectiveDate ?? '未知'}</span>
      </div>
      <pre
        data-testid="chunk-verbatim"
        className="mt-3 whitespace-pre-wrap font-mono text-xs text-text"
      >{`[chunk ${chunkId} verbatim — populated in Wave 3]`}</pre>
    </div>
  );
}
