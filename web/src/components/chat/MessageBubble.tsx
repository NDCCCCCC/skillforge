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
import FeedbackRow from './parts/FeedbackRow';
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
import { DataReferenceChip } from './parts/DataReferenceChip';
import { isVerdict, type VerdictStatus } from '../../types/verdict';

function renderPart(
  part: UIMessage['parts'][number] | { type: 'report_published'; data?: import('../../types/wire-types').ReportCard },
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
      const dp = (part as { data?: { decision?: string; conflictChunks?: ConflictChunk[]; dataRef?: import('../../types/wire-types').DataReference } }).data;
      const decision = dp?.decision;
      if (decision === 'counter_evidence') {
        return <CounterEvidenceCard key={i} conflictChunks={dp?.conflictChunks} />;
      }
      if (decision === 'kpi_reference' && dp?.dataRef) {
        return <DataReferenceChip key={i} dataRef={dp.dataRef} />;
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
    case 'report_published': {
      const card = (part as { data?: import('../../types/wire-types').ReportCard }).data;
      if (!card) return null;
      const period = card.period;
      const versionBadge = `v${card.version}`;
      const viewReportUrl = card.url;
      return (
        <div
          className="mt-3 p-3 rounded-md border"
          style={{
            backgroundColor: 'var(--color-report-bg, #14171e)',
            borderColor: 'var(--color-report-border, #242833)',
          }}
        >
          {/* Header */}
          <div className="flex justify-between items-center mb-2">
            <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--color-text)' }}>
              {period} 业务月报
            </span>
            <span
              style={{
                backgroundColor: 'var(--color-report-badge-bg)',
                borderColor: 'var(--color-report-badge-border)',
                color: 'var(--color-text-muted)',
                fontFamily: 'var(--font-mono)',
                fontSize: '0.75rem',
                padding: '0.125rem 0.5rem',
                borderRadius: '0.25rem',
                border: '1px solid',
              }}
            >
              {versionBadge}
            </span>
          </div>

          {/* Structure summary */}
          {card.sections && card.sections.length > 0 && (
            <div className="text-xs mb-2" style={{ color: 'var(--color-text-muted)' }}>
              {card.sections.map((s, idx) => (
                <div key={idx}>· {s.title}: {s.kpis?.join(', ')}</div>
              ))}
            </div>
          )}

          {/* Action row */}
          <div className="flex justify-between items-center">
            <a
              href={viewReportUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-sm"
              style={{ color: 'var(--color-primary)' }}
            >
              查看报告
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
                <polyline points="15 3 21 3 21 9"/>
                <line x1="10" y1="14" x2="21" y2="3"/>
              </svg>
            </a>
            <button
              type="button"
              onClick={() => {
                const prefillText = `请重新生成报告（基于 ${period}），修改意见：`;
                document.dispatchEvent(new CustomEvent('chat-input-prefill', { detail: prefillText }));
              }}
              style={{
                backgroundColor: 'var(--color-regen-chip-bg)',
                borderColor: 'var(--color-regen-chip-border)',
                color: 'var(--color-regen-chip-text)',
                cursor: 'pointer',
                padding: '0.125rem 0.5rem',
                borderRadius: '0.25rem',
                border: '1px solid',
                fontSize: '0.75rem',
              }}
            >
              带修改重生成
            </button>
          </div>
        </div>
      );
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
        {/* Open Design B6 — per-reply feedback strip (vote locks after
            the first choice; 创建工单 stays independent). */}
        <FeedbackRow messageId={message.id} />
      </div>
    </div>
  );
}
