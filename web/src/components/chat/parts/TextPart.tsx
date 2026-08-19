// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TextPart — renders the assistant's main response text.
//
// Open Design redesign B4: the pre-wrap plain-text <p> is replaced by
// the MarkdownMessage renderer (react-markdown + remark-gfm; fenced
// code blocks become copyable cmd-blocks). Paragraph typography moves
// to .markdown-body in theme.css.
//
// 260817-i36 (①): optional `verdictStatus` prop. When MessageBubble
// finds a data-policy Verdict in the same message, the platform Status
// is rendered as a SMALL corner badge attached to the conclusion text
// (the large in-card StatusBadge is chat-stream noise, removed in ②).
// Streaming note: data-policy arrives AFTER the text part, so the badge
// appears only once the verdict lands — acceptable per locked decision.
// When the prop is absent (no verdict in this message) the output is
// the markdown body alone (byte-identical wrapper to the pre-B4
// renderer's relative-wrapper shape).
import type { TextUIPart } from 'ai';
import MarkdownMessage from '../MarkdownMessage';
import StatusBadge from '../../common/StatusBadge';
import type { VerdictStatus } from '../../../types/verdict';

export interface TextPartProps {
  part: TextUIPart;
  verdictStatus?: VerdictStatus;
}

export function TextPart({ part, verdictStatus }: TextPartProps) {
  const body = (
    <div className="message-assistant-text">
      <MarkdownMessage text={part.text} />
    </div>
  );
  if (verdictStatus === undefined) {
    return body;
  }
  return (
    <div className="relative">
      {body}
      <StatusBadge
        status={verdictStatus}
        className="absolute -top-2 left-0 px-1.5 py-0 text-[10px]"
      />
    </div>
  );
}
