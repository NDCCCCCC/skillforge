// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceDocumentPart — renders inline-document citations as an inline
// badge with the document title. Distinct from SourceUrlPart (which
// renders external URLs); these are document references attached to the
// message by the agent.
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
import type { SourceDocumentUIPart } from 'ai';
import { extractChunkMetadata, normalizeChunkID } from '../../../lib/chunk-lookup';
import { useAppStore } from '../../../store/app-store';

export function SourceDocumentPart({ part }: { part: SourceDocumentUIPart }) {
  const label = part.title ?? part.sourceId;
  const handleClick = () => {
    const chunkId = normalizeChunkID(part.sourceId);
    const meta = extractChunkMetadata(chunkId);
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: meta.tier,
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
      onClick={handleClick}
    >
      [{label}]
    </button>
  );
}
