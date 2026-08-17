// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// MessageBubble — the parts switch dispatcher. Iterates message.parts
// and routes each by `part.type` to the appropriate renderer.
//
// Per 04-RESEARCH.md Pattern 3 + D-08 (reasoning MUST render BEFORE
// text in DOM order so reasoning expansion never pushes the answer off-
// screen), the switch keeps `case 'reasoning'` BEFORE `case 'text'` —
// but since we iterate in array order and the AI SDK parser emits
// parts in emission order, the fixture ordering matters. Reasoning
// chunks arrive before text chunks when the agent streams a
// reasoning-first pattern (GLM-5.2 default), so iteration preserves
// the contract. The MessageBubble.test.tsx assertion enforces this.
//
// Unknown part types silently drop (per AI SDK v7 spec — a newer-
// server / older-client combo must not crash the UI).
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
// Both render AFTER the verdict; RedlineCard is the LAST part in the
// bubble (the operator's final affordance: copy / download the diff).
import CounterEvidenceCard from '../verdict/CounterEvidenceCard';
import RedlineCard from '../verdict/RedlineCard';
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
      // Plan 05-03 / SC-1 — `route` decisions render via RoutingTracePart.
      // Plan 05-06 / SC-6 — `counter_evidence` decisions render via the
      // CounterEvidenceCard annotation surface (D5-13 trust moat: this
      // is NOT routing info, it's the visible "history tier conflicts"
      // marker that lives OUTSIDE the verdict's grounding).
      // Other decision values (`retrieve`, `answer`, `refuse`,
      // `forced_stop`) drop silently — they're consumed by the SC-4
      // AgentInspector panel (mounted in the center), not here.
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
      // Plan 05-06 / SC-7 — D5-14. The BE emits a data-redline chunk
      // ONLY on 不合规 / 需补充 verdicts (合规 emits ZERO chunks + makes
      // ZERO LLM calls). The card is therefore ABSENT on 合规 verdicts;
      // this dispatcher simply never runs for them. If a future BE bug
      // emits an empty/partial redline, RedlinePayload's required-field
      // typing catches it at the cast below (TS refuses to narrow).
      const data = (part as { data?: RedlinePayload }).data;
      if (!data) return null;
      return <RedlineCard key={i} data={data} />;
    }
    default:
      // For tool-${name} types we didn't pre-list (Pitfall 3), fall
      // through to ToolPart. Anything else silently drops per spec.
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
  const bg = role === 'user' ? 'bg-user-bubble' : 'bg-assistant-bubble';
  // 260817-hck: aggregate all text parts into chatText so VerdictCard
  // (via DataPolicyPart) can suppress a verdict-answer block that the
  // chat body already renders verbatim. Assistant messages only — user
  // messages never carry data-policy parts.
  const chatText =
    role === 'assistant'
      ? message.parts
          .filter((p) => p.type === 'text')
          .map((p) => (p as { text: string }).text)
          .join('')
      : undefined;
  // 260817-i36 (①): lift the Verdict status off the data-policy part so
  // TextPart can render it as the corner badge on the conclusion text.
  // Assistant messages only. Streaming: data-policy arrives after text —
  // the badge appears once the verdict lands (locked decision,
  // acceptable timing).
  const policyData =
    role === 'assistant'
      ? (message.parts.find((p) => p.type === 'data-policy') as
          | { data?: unknown }
          | undefined)?.data
      : undefined;
  const verdictStatus =
    policyData !== undefined && isVerdict(policyData)
      ? policyData.status
      : undefined;
  return (
    <div
      className={`my-3 rounded-lg ${bg} p-3 text-text`}
      data-testid="message-bubble"
      data-role={role}
    >
      {message.parts.map((part, i) => renderPart(part, i, chatText, verdictStatus))}
    </div>
  );
}
