// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// CitationCluster — Plan 260815-cos Task 3 (per locked CONTEXT.md
// decision C3 + RESEARCH "raw React 19 hover, no Radix"):
//
// Citations render as inline superscripts `[1]` `[2]` ... attached to
// the verdict body. Hovering a superscript opens a popover with the
// verbatim chunk text + metadata (tier / version / effective date /
// source). The popover uses raw React 19 (onMouseEnter / onMouseLeave
// + useState + fetchChunkContent); no Radix / Headless UI surface
// added — keep the surface area minimal (D15-style compositional
// discipline) and the deps small.
//
// fetchChunkContent mirrors the ChunkSourceView pattern (lookupChunk
// synchronous fallback + fetchChunkContent async for graph-browsed
// chunks the fixture does not carry — verbatim text wins on hit,
// metadata row + 「片段不存在」 state on miss).
//
// onClick still navigates to the chunk-source view via setCenterView
// (the superscript accepts onClick); the popover is informational, the
// click is the deeper drill-down. The TierBadge moves from the chip
// surface into the popover header so the superscript stays compact.

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  extractChunkMetadata,
  fetchChunkContent,
  lookupChunk,
  NOT_FOUND_TEXT,
} from '../../lib/chunk-lookup';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { Citation } from '../../types/verdict';
import TierBadge from '../common/TierBadge';

const HOVER_OPEN_DELAY_MS = 150;

export interface CitationClusterProps {
  citations: Citation[];
}

/** Popover payload — the verbatim text + metadata row for one citation. */
interface HoverState {
  chunkId: string;
  text: string;
  tier: Citation['tier'];
  version: string | null;
  effectiveDate: string | null;
  notFound: boolean;
}

export default function CitationCluster({ citations }: CitationClusterProps) {
  const [openChunkId, setOpenChunkId] = useState<string | null>(null);
  const [hover, setHover] = useState<HoverState | null>(null);
  const hoverTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Cancel any pending hover-open when the component unmounts or the
  // open chunk changes — defensive against the "rapid mouse across
  // multiple superscripts" race that would otherwise queue stale fetches.
  useEffect(() => {
    return () => {
      if (hoverTimer.current) clearTimeout(hoverTimer.current);
    };
  }, []);

  const handleClick = useCallback((chunkId: string, tier: Citation['tier']) => {
    const meta = extractChunkMetadata(chunkId);
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier,
      version: meta.version,
      effectiveDate: meta.effectiveDate,
      effectiveTo: null,
    });
  }, []);

  const openHover = useCallback((chunkId: string, tier: Citation['tier']) => {
    // Synchronous fixture lookup first — the typical path (citation
    // chunk is in chunks.json). The async fetch is the fallback for
    // graph-browsed chunks the fixture does not carry.
    const sync = lookupChunk(chunkId);
    setHover({
      chunkId,
      text: sync.text === NOT_FOUND_TEXT ? '' : sync.text,
      tier,
      version: sync.version,
      effectiveDate: sync.effectiveDate,
      notFound: sync.text === NOT_FOUND_TEXT,
    });
    setOpenChunkId(chunkId);
    if (sync.text === NOT_FOUND_TEXT) {
      // Async fetch for the on-demand /api/knowledge/chunk/:id path.
      fetchChunkContent(chunkId)
        .then((data) => {
          if (data) {
            setHover((prev) =>
              prev && prev.chunkId === chunkId
                ? { ...prev, text: data.text, notFound: false }
                : prev,
            );
          }
        })
        .catch(() => {
          /* keep notFound=true so the popover renders 「片段不存在」 */
        });
    }
  }, []);

  const scheduleOpen = useCallback(
    (chunkId: string, tier: Citation['tier']) => {
      if (hoverTimer.current) clearTimeout(hoverTimer.current);
      hoverTimer.current = setTimeout(() => openHover(chunkId, tier), HOVER_OPEN_DELAY_MS);
    },
    [openHover],
  );

  const cancelHover = useCallback(() => {
    if (hoverTimer.current) {
      clearTimeout(hoverTimer.current);
      hoverTimer.current = null;
    }
    setOpenChunkId(null);
    setHover(null);
  }, []);

  return (
    <div data-testid="citation-cluster" className="flex flex-wrap gap-2">
      {citations.map((citation, index) => {
        const isOpen = openChunkId === citation.chunk_id;
        return (
          <span
            key={`${index}-${citation.chunk_id}`}
            className="relative inline"
            onMouseEnter={() => scheduleOpen(citation.chunk_id, citation.tier)}
            onMouseLeave={cancelHover}
          >
            <sup
              data-testid="citation-superscript"
              data-chunk-id={citation.chunk_id}
              data-tier={citation.tier}
              onClick={() => handleClick(citation.chunk_id, citation.tier)}
              className="ml-0.5 cursor-pointer text-xs text-accent"
            >
              [{index + 1}]
            </sup>
            {isOpen && hover && hover.chunkId === citation.chunk_id && (
              <span
                data-testid="citation-popover"
                role="tooltip"
                className={cn(
                  'absolute left-0 top-full z-20 mt-1 w-80 max-w-xs rounded-md border border-border bg-surface p-3 text-left text-xs shadow-lg',
                )}
              >
                <span className="mb-2 flex items-center gap-2">
                  <TierBadge tier={citation.tier} />
                  <span className="font-mono text-[11px] text-text-muted">
                    {citation.chunk_id}
                  </span>
                  {hover.version && (
                    <span className="font-mono text-[11px] text-text-muted">
                      v{hover.version}
                    </span>
                  )}
                  {hover.effectiveDate && (
                    <span className="text-[11px] text-text-muted">
                      生效 {hover.effectiveDate}
                    </span>
                  )}
                </span>
                <span className="block max-h-40 overflow-y-auto whitespace-pre-wrap text-text">
                  {hover.notFound ? '（片段不存在）' : hover.text}
                </span>
              </span>
            )}
          </span>
        );
      })}
    </div>
  );
}