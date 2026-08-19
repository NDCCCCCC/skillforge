// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// MessageBubble — modernized chat layout (Phase 4 visual upgrade).
//
// Layout strategy:
//   - User messages: right-aligned card with subtle blue gradient +
//     rounded-2xl (one corner tighter for "tail" feel) + max-78% width
//   - Assistant messages: full-width column with breathing room —
//     text gets no chrome, only child parts (tool/reasoning/citations)
//     get elevated cards
//   - Reasoning parts (DOM order per D-08): positioned ABOVE text
//   - Tool parts: distinct elevated cards with status indicators
//   - Citations: pill-style chips
//
// The MessageBubble.test.tsx assertions still hold:
//   - reasoning renders BEFORE text in DOM order (D-08)
//   - all 7 part types render without crash
//   - tool-${name} and dynamic-tool both render
//   - unknown part types silently drop
//
// 260817-hck: the assistant branch concatenates all text parts into
// `chatText` and passes it to DataPolicyPart, so VerdictCard can hide
// a verdict-answer block that duplicates the chat body conclusion.
// Prop drilling (not the Zustand store) per D14 — useChat owns the
// message stream; three-level optional-prop drilling is the cheap fix.
//
// 260817-i36 (①): the assistant branch also extracts the Verdict status
// from the message's data-policy part (`verdictStatus`) and passes it
// to TextPart, which renders it as a small corner badge on the
// conclusion text — the large in-card StatusBadge is gone from the
// chat stream (260817-i36 ②).

import type { ReactNode } from 'react';
import type { UIMessage } from 'ai';
import { Sparkles } from 'lucide-react';
import DataPolicyPart from './parts/DataPolicyPart';
import DataSubprogressPart from './parts/DataSubprogressPart';
import ReasoningPart from './parts/ReasoningPart';
import RoutingTracePart from './parts/RoutingTracePart';
import { SourceDocumentPart } from './parts/SourceDocumentPart';
import { SourceUrlPart } from './parts/SourceUrlPart';
import { StepStartPart } from './parts/StepStartPart';
import { TextPart } from './parts/TextPart';
import { ToolPart } from './parts/ToolPart';
// Plan 05-06 Task 2 — SC-6 / SC-7 post-verdict annotation cards.
import CounterEvidenceCard from '../verdict/CounterEvidenceCard';
import RedlineCard from '../verdict/RedlineCard';
// Plan 10-03 Task 2 — D10-C3 transparency caption above the user
// bubble when the history-pipeline rewriter fired.
import HistoryRewriteHint from './HistoryRewriteHint';
import type { ConflictChunk, RedlinePayload } from '../../types/wire-types';
import { isVerdict, type VerdictStatus } from '../../types/verdict';

function renderPart(
  part: UIMessage['parts'][number],
  i: number,
  chatText?: string,
  verdictStatus?: VerdictStatus,
): ReactNode {
  switch (part.type) {
    case 'reasoning':
      return (
        <ReasoningPart
          key={i}
          part={part as Extract<UIMessage['parts'][number], { type: 'reasoning' }>}
        />
      );
    case 'text':
      return (
        <TextPart
          key={i}
          part={part as Parameters<typeof TextPart>[0]['part']}
          verdictStatus={verdictStatus}
        />
      );
    case 'source-url':
      return <SourceUrlPart key={i} part={part as Parameters<typeof SourceUrlPart>[0]['part']} />;
    case 'source-document':
      return <SourceDocumentPart key={i} part={part as Parameters<typeof SourceDocumentPart>[0]['part']} />;
    case 'dynamic-tool':
      return <ToolPart key={i} part={part as Parameters<typeof ToolPart>[0]['part']} />;
    case 'data-policy':
      return (
        <DataPolicyPart
          key={i}
          part={part as Parameters<typeof DataPolicyPart>[0]['part']}
          chatText={chatText}
        />
      );
    case 'data-subprogress':
      return (
        <DataSubprogressPart
          key={i}
          part={part as Parameters<typeof DataSubprogressPart>[0]['part']}
        />
      );
    case 'step-start':
      return <StepStartPart key={i} part={part as Parameters<typeof StepStartPart>[0]['part']} />;
    case 'data-decision': {
      const dp = (part as { data?: { decision?: string; conflictChunks?: ConflictChunk[] } }).data;
      const decision = dp?.decision;
      if (decision === 'counter_evidence') {
        return <CounterEvidenceCard key={i} conflictChunks={dp?.conflictChunks} />;
      }
      if (decision !== 'route') return null;
      return (
        <RoutingTracePart
          key={i}
          part={part as Parameters<typeof RoutingTracePart>[0]['part']}
        />
      );
    }
    case 'data-redline': {
      const data = (part as { data?: RedlinePayload }).data;
      if (!data) return null;
      return <RedlineCard key={i} data={data} />;
    }
    default:
      if (part.type.startsWith('tool-')) {
        return <ToolPart key={i} part={part as Parameters<typeof ToolPart>[0]['part']} />;
      }
      return null;
  }
}

export interface MessageBubbleProps {
  message: UIMessage;
  role?: 'assistant' | 'user';
}

export function MessageBubble({ message, role = 'assistant' }: MessageBubbleProps) {
  if (role === 'user') {
    // User messages: right-aligned card with the full message text.
    // We render the text parts inline (not via TextPart) so the bubble
    // owns its typography — bubble padding/font/line-height is set
    // here, not in the part renderer. The data-testid stays the same
    // for backward compat with the existing test suite.
    //
    // Plan 10-03 / D10-C3 — when the message carries a data-decision
    // part with decision="history_query_rewritten", render the
    // HistoryRewriteHint caption ABOVE the bubble so the user sees
    // what the rewriter understood. The caption is co-located with the
    // bubble (inside the same message container) so it travels with
    // the message through re-renders / virtualization.
    const rewriteDecision = message.parts.find(
      (p) =>
        p.type === 'data-decision' &&
        (p as { data?: { decision?: string; query?: string } }).data?.decision ===
          'history_query_rewritten',
    ) as { data?: { decision: string; query?: string } } | undefined;
    const rewriteText = rewriteDecision?.data?.query;
    return (
      <div
        className="message-user"
        data-testid="message-bubble"
        data-role="user"
      >
        {rewriteText ? <HistoryRewriteHint rewrite={rewriteText} /> : null}
        <div className="message-user-card">
          {message.parts.map((part, i) => {
            if (part.type === 'text') {
              return <span key={i}>{part.text}</span>;
            }
            return null;
          })}
        </div>
      </div>
    );
  }

  // Assistant messages: full-width column with breathing room.
  // 260817-hck: aggregate all text parts into chatText so VerdictCard
  // (via DataPolicyPart) can suppress a verdict-answer block that the
  // chat body already renders verbatim.
  const chatText = message.parts
    .filter((p) => p.type === 'text')
    .map((p) => (p as { text: string }).text)
    .join('');
  // 260817-i36 (①): lift the Verdict status off the data-policy part so
  // TextPart can render it as the corner badge on the conclusion text.
  // Streaming: data-policy arrives after text — the badge appears once
  // the verdict lands (locked decision, acceptable timing).
  const policyData = (
    message.parts.find((p) => p.type === 'data-policy') as
      | { data?: unknown }
      | undefined
  )?.data;
  const verdictStatus =
    policyData !== undefined && isVerdict(policyData)
      ? policyData.status
      : undefined;
  return (
    <div
      className="message-assistant"
      data-testid="message-bubble"
      data-role="assistant"
    >
      {/* Open Design B4 — 30px Sparkles 方标 anchoring the assistant
          column; parts live in the body (D-08 reasoning-before-text
          order is preserved inside .message-assistant-body). */}
      <div className="assistant-mark" data-testid="assistant-mark" aria-hidden="true">
        <Sparkles className="h-4 w-4" />
      </div>
      <div className="message-assistant-body">
        {message.parts.map((part, i) => renderPart(part, i, chatText, verdictStatus))}
      </div>
    </div>
  );
}
