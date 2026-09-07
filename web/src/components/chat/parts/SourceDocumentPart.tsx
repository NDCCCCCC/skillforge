// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceDocumentPart — renders inline-document citations as a pill
// chip with a document icon and the document title. Distinct from
// SourceUrlPart (which renders external URLs); these are document
// references attached to the message by the agent.
//
// Per AI SDK v7 index.d.ts line 2382-2387, source-document requires
// mediaType + title; we render the title and optionally the filename.
//
// 260817-i36 (③): the backend Title now carries the chunk ordinal
// ("authority/annual_leave.md · chunk 3", chat.go titleFor), so the
// legacy "doc: " prefix is dropped — the title is self-describing.
//
// 260817-i36 (②): the chip is now a BUTTON. Clicking dispatches
// setCenterView({kind:'chunk-source', ...}) — the same navigation
// contract as the verdict card's citation superscript (normalizeChunkID
// entry normalization + extractChunkMetadata for version/effectiveDate)
// — making the doc chips under the conclusion the single, interactive
// citation UI in the chat stream.
//
// Open Design B5 — tier 药丸徽章: the chip leads with a tier pill. 26-08
// residual sweep: the fixed 4-key CN title→tier table is GONE. The pill's
// value is the REGISTRY SLUG — derived from the backend title's first path
// segment ("authority/annual_leave.md · chunk 3") with the chunk fixture as
// the fallback — and its DISPLAY label resolves through the KB registry
// projection (lib/kb-bases), the same source TierBadge uses. An unknown
// slug renders `[slug]`; no derivable tier hides the pill. A pre-migration
// CN value replayed from an audit row is not a registry slug, so it
// degrades to a bracketed legacy label instead of masquerading as a live
// tier (T-26-08-2).
import type { SourceDocumentUIPart } from 'ai';
import { FileText } from 'lucide-react';
import { useTierLabel } from '../../../lib/kb-bases';
import { extractChunkMetadata, normalizeChunkID } from '../../../lib/chunk-lookup';
import { useAppStore } from '../../../store/app-store';

function resolveTierSlug(sourceId: string, title: string): string | null {
  // Backend truth first: the title carries the tier directory as its first
  // path segment ("authority/annual_leave.md · chunk 3").
  const lower = title.toLowerCase();
  const slash = lower.indexOf('/');
  if (slash > 0) return lower.slice(0, slash);
  // Fallback: the chunk fixture — null on a miss, no tier invented.
  return extractChunkMetadata(normalizeChunkID(sourceId)).tier;
}

export function SourceDocumentPart({ part }: { part: SourceDocumentUIPart }) {
  const label = part.title ?? part.sourceId;
  const tierSlug = resolveTierSlug(part.sourceId, label);
  const tierLabel = useTierLabel(tierSlug ?? '');
  const handleClick = () => {
    const chunkId = normalizeChunkID(part.sourceId);
    const meta = extractChunkMetadata(chunkId);
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: tierSlug ?? meta.tier ?? '',
      version: meta.version,
      effectiveDate: meta.effectiveDate,
      effectiveTo: null,
    });
  };
  return (
    <button
      type="button"
      className="citation-chip"
      data-testid="source-document"
      data-chunk-id={part.sourceId}
      data-tier={tierSlug ?? ''}
      onClick={handleClick}
    >
      {tierSlug && (
        <span className="citation-tier-pill" data-testid="citation-tier-pill" data-tier={tierSlug}>
          {tierLabel}
        </span>
      )}
      <FileText className="h-3 w-3" />
      <span>{label}</span>
    </button>
  );
}
