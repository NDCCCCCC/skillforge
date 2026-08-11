// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceDocumentPart — renders inline-document citations as an inline
// badge with the document title. Distinct from SourceUrlPart (which
// renders external URLs); these are document references attached to the
// message by the agent.
//
// Per AI SDK v7 index.d.ts line 2382-2387, source-document requires
// mediaType + title; we render the title and optionally the filename.
import type { SourceDocumentUIPart } from 'ai';

export function SourceDocumentPart({ part }: { part: SourceDocumentUIPart }) {
  const label = part.title ?? part.sourceId;
  return (
    <span className="citation-chip" data-testid="source-document">
      [doc: {label}]
    </span>
  );
}
