// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ChunkSourceView — FEUI-03 / D-01 chunk-source preview. Renders the
// metadata row (TierBadge + 版本 + 当前有效/已失效 badge + 生效日期 +
// (conditional) 失效日期) at the top, then the verbatim chunk text
// inside a <pre>.
//
// Plan 05-05 / SC-8 (CMPL-08): the metadata row gains the VersionBadge
// (当前有效 BadgeCheck green / 已失效 strikethrough dim) plus the
// 生效日期 row (always) and 失效日期 row (only when effective_to is
// set). When the operator navigates to a superseded chunk via the
// graph browser (the only path that surfaces a superseded chunk to
// the FE), the FilteredOutNotice banner renders ABOVE the body.
//
// Wave 2 shipped the metadata row + a placeholder body. Wave 3 wires
// the `chunk-lookup` helper so the metadata row reflects the chunk's
// real version / effective date and the body shows the verbatim text
// from the chunks.json fixture (or `未知` / `(chunk not found in
// fixture)` for misses).
import { useMemo } from 'react';
import { lookupChunk } from '../../lib/chunk-lookup';
import { formatEffectiveDate } from '../../lib/format-effective';
import TierBadge from '../common/TierBadge';
import FilteredOutNotice from './FilteredOutNotice';
import VersionBadge from './VersionBadge';
import type { Tier } from '../../types/verdict';

export interface ChunkSourceViewProps {
  chunkId: string;
  tier: Tier;
  version: string | null;
  effectiveDate: string | null;
  /** Epoch seconds (or null) when the chunk becomes superseded. */
  effectiveTo: number | null;
}

export default function ChunkSourceView({
  chunkId,
  tier,
  version,
  effectiveDate,
  effectiveTo,
}: ChunkSourceViewProps) {
  // The chunk-lookup helper reads the same fields the parent already
  // has, but it's the canonical source of truth for the verbatim text
  // (the parent props carry the version/effective date for the
  // CenterView; the body text comes from the fixture).
  const result = useMemo(() => lookupChunk(chunkId), [chunkId]);

  // SC-8 filtered-out notice: a superseded chunk only reaches the FE
  // via the graph browser (the retriever PRE-filter guarantees it
  // never appears in a verdict.citations payload). The condition
  // effectiveTo > 0 AND effectiveTo * 1000 < now flags the
  // "browseable but not searchable" state.
  const isSuperseded = useMemo(() => {
    if (effectiveTo === null || effectiveTo === 0) return false;
    return effectiveTo * 1000 < Date.now();
  }, [effectiveTo]);

  return (
    <div
      data-testid="chunk-source-view"
      data-chunk-id={chunkId}
      data-tier={result.tier}
    >
      {isSuperseded && <FilteredOutNotice effectiveTo={effectiveTo} />}
      <div data-testid="chunk-meta" className="flex flex-wrap items-center gap-3 text-xs text-text-muted">
        <TierBadge tier={tier} />
        <span data-testid="chunk-version">版本: {version ?? '未知'}</span>
        <VersionBadge
          effectiveFrom={effectiveDate ? epochFromEffectiveDate(effectiveDate) : null}
          effectiveTo={effectiveTo}
        />
        <span data-testid="chunk-effective-date">
          生效日期: {effectiveDate ?? '未知'}
        </span>
        {effectiveTo !== null && effectiveTo > 0 && (
          <span data-testid="chunk-effective-to-date">
            失效日期: {formatEffectiveDate(effectiveTo)}
          </span>
        )}
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

/**
 * Convert a YYYY-MM-DD date string into epoch-seconds (UTC midnight).
 * Returns 0 for any unparseable input — matches the BE's sentinel-0
 * design (D5-15: zero = perpetual / open-ended). The conversion is
 * lossy (HH:MM:SS truncated) but acceptable for the badge input —
 * the badge only needs the year/month/day resolution that the BE
 * carries.
 */
function epochFromEffectiveDate(effectiveDate: string): number {
  const t = Date.parse(`${effectiveDate}T00:00:00Z`);
  return Number.isNaN(t) ? 0 : Math.floor(t / 1000);
}