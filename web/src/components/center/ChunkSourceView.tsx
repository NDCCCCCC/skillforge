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
// from the chunks.json fixture for citations. Plan 260813-dno / UAT #4
// adds the async-fetch branch: a graph-browsed chunk the fixture does
// NOT carry is fetched on-demand from GET /api/knowledge/chunk/:id
// (the graph endpoint is metadata-only per T-5-22). The citation path
// stays synchronous; the fetch fires ONLY on a fixture miss.
import { useEffect, useMemo, useState } from 'react';
import { fetchChunkContent, lookupChunk, NOT_FOUND_TEXT } from '../../lib/chunk-lookup';
import { formatEffectiveDate } from '../../lib/format-effective';
import { prettySourcePath } from '../../lib/pretty-source';
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

  // Plan 260813-dno / UAT #4 — async chunk-content fetch for graph-browsed
  // chunks the fixture does NOT carry. The fixture path (result.text !==
  // NOT_FOUND_TEXT, i.e. verdict citations) is unchanged; this branch
  // fires ONLY on a fixture miss, which is the graph-browser case (the
  // graph endpoint is metadata-only per T-5-22, so its chunks have no
  // fixture entry).
  const [asyncText, setAsyncText] = useState<string | null>(null);
  // Quick 260929-d5w2 — the fetch payload's raw source path, rendered as
  // the pretty short form in the metadata row (raw path stays on title).
  const [asyncSource, setAsyncSource] = useState<string | null>(null);
  const [asyncStatus, setAsyncStatus] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');

  const isFixtureMiss = result.text === NOT_FOUND_TEXT;
  useEffect(() => {
    // Only the fixture-miss branch fetches. The citation path is synchronous.
    if (!isFixtureMiss) {
      return;
    }
    setAsyncStatus('loading');
    setAsyncText(null);
    setAsyncSource(null);
    let cancelled = false;
    fetchChunkContent(chunkId)
      .then((data) => {
        if (cancelled) return;
        // 404 → null → render the 「片段不存在」 body (soft miss, not an error).
        setAsyncText(data === null ? '（片段不存在）' : data.text);
        setAsyncSource(data?.source ?? null);
        setAsyncStatus('ready');
      })
      .catch(() => {
        if (cancelled) return;
        setAsyncStatus('error');
      });
    return () => {
      cancelled = true;
    };
  }, [chunkId, isFixtureMiss]);

  // Compute the rendered body: fixture hit → fixture text; fixture miss →
  // switch on the async status (loading / error / ready).
  const body =
    !isFixtureMiss
      ? result.text
      : asyncStatus === 'loading'
        ? '加载中…'
        : asyncStatus === 'error'
          ? '加载片段失败'
          : (asyncText ?? '');

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
      data-tier={tier}
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
        {asyncSource && (
          <span
            data-testid="chunk-source-path"
            title={asyncSource}
            className="font-mono"
          >
            来源: {prettySourcePath(asyncSource)}
          </span>
        )}
      </div>
      <pre
        data-testid="chunk-verbatim"
        className="mt-3 whitespace-pre-wrap text-xs text-text font-mono bg-surface-2 p-4 rounded-md"
      >
        {body}
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