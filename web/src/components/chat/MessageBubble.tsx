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

import type { ReactNode } from 'react';
import type { UIMessage } from 'ai';
import { DataPolicyPart } from './parts/DataPolicyPart';
import ReasoningPart from './parts/ReasoningPart';
import { SourceDocumentPart } from './parts/SourceDocumentPart';
import { SourceUrlPart } from './parts/SourceUrlPart';
import { StepStartPart } from './parts/StepStartPart';
import { TextPart } from './parts/TextPart';
import { ToolPart } from './parts/ToolPart';

function renderPart(part: UIMessage['parts'][number], i: number): ReactNode {
  switch (part.type) {
    case 'reasoning':
      return (
        <ReasoningPart
          key={i}
          part={part as Extract<UIMessage['parts'][number], { type: 'reasoning' }>}
        />
      );
    case 'text':
      return <TextPart key={i} part={part as Parameters<typeof TextPart>[0]['part']} />;
    case 'source-url':
      return <SourceUrlPart key={i} part={part as Parameters<typeof SourceUrlPart>[0]['part']} />;
    case 'source-document':
      return <SourceDocumentPart key={i} part={part as Parameters<typeof SourceDocumentPart>[0]['part']} />;
    case 'dynamic-tool':
      return <ToolPart key={i} part={part as Parameters<typeof ToolPart>[0]['part']} />;
    case 'data-policy':
      return <DataPolicyPart key={i} part={part as Parameters<typeof DataPolicyPart>[0]['part']} />;
    case 'data-subprogress':
      // Wave 3 adds DataSubprogressPart renderer (FEUI-06).
      return null;
    case 'step-start':
      return <StepStartPart key={i} part={part as Parameters<typeof StepStartPart>[0]['part']} />;
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
  return (
    <div
      className={`my-3 rounded-lg ${bg} p-3 text-text`}
      data-testid="message-bubble"
      data-role={role}
    >
      {message.parts.map((part, i) => renderPart(part, i))}
    </div>
  );
}
