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
import type { SourceDocumentUIPart } from 'ai';

export function SourceDocumentPart({ part }: { part: SourceDocumentUIPart }) {
  const label = part.title ?? part.sourceId;
  return (
    <span className="citation-chip" data-testid="source-document">
      [{label}]
    </span>
  );
}
