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
// Open Design B5 — tier 药丸徽章: the chip leads with a colored tier
// pill (权威=靛蓝 / 历史=琥珀 / 个人=青 / 草稿=灰). The tier is
// resolved from the chunk fixtures via extractChunkMetadata, falling
// back to the title's directory prefix (authority/… etc.).
import type { SourceDocumentUIPart } from 'ai';
import { FileText } from 'lucide-react';
import { extractChunkMetadata, normalizeChunkID } from '../../../lib/chunk-lookup';
import { useAppStore } from '../../../store/app-store';
import type { Tier } from '../../../types/verdict';

const TITLE_DIR_TO_TIER: Array<[string, Tier]> = [
  ['authority/', '权威'],
  ['historical/', '历史'],
  ['personal/', '个人'],
  ['draft/', '草稿'],
];

function resolveTier(sourceId: string, title: string): Tier | null {
  // Backend-truth first: the title carries the tier directory
  // ("authority/annual_leave.md · chunk 3"). The fixture lookup is the
  // fallback (it never throws — a miss degrades to 权威).
  const lower = title.toLowerCase();
  for (const [dir, tier] of TITLE_DIR_TO_TIER) {
    if (lower.startsWith(dir)) return tier;
  }
  return extractChunkMetadata(normalizeChunkID(sourceId)).tier;
}

export function SourceDocumentPart({ part }: { part: SourceDocumentUIPart }) {
  const label = part.title ?? part.sourceId;
  const tier = resolveTier(part.sourceId, label);
  const handleClick = () => {
    const chunkId = normalizeChunkID(part.sourceId);
    const meta = extractChunkMetadata(chunkId);
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: meta.tier ?? '',
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
      data-tier={tier ?? ''}
      onClick={handleClick}
    >
      {tier && (
        <span className="citation-tier-pill" data-tier={tier}>
          {tier}
        </span>
      )}
      <FileText className="h-3 w-3" />
      <span>{label}</span>
    </button>
  );
}
