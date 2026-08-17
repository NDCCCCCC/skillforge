// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TextPart — renders the assistant's main response text. Wraps in <p>
// with body font + relaxed line height for comfortable reading.
//
// The AI SDK v7 parser upstream (text-start / text-delta / text-end)
// collapses into a single text part. White-space is preserved so any
// newlines from the model survive rendering.
//
// 260817-i36 (①): optional `verdictStatus` prop. When MessageBubble
// finds a data-policy Verdict in the same message, the platform Status
// is rendered as a SMALL corner badge attached to the conclusion text
// (the large in-card StatusBadge is chat-stream noise, removed in ②).
// Streaming note: data-policy arrives AFTER the text part, so the badge
// appears only once the verdict lands — acceptable per locked decision.
// When the prop is absent (no verdict in this message) the output is
// byte-identical to the pre-260817-i36 renderer.
import type { TextUIPart } from 'ai';
import StatusBadge from '../../common/StatusBadge';
import type { VerdictStatus } from '../../../types/verdict';

export interface TextPartProps {
  part: TextUIPart;
  verdictStatus?: VerdictStatus;
}

export function TextPart({ part, verdictStatus }: TextPartProps) {
  const body = (
    <p
      className="message-assistant-text"
      style={{ whiteSpace: 'pre-wrap' }}
    >
      {part.text}
    </p>
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
