// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// InspectorHop — Plan 05-05 / SC-4 (AGNT-06) per-hop Accordion item.
//
// One Accordion row per AgentDecisionPayload.step. Renders:
//   - 决策: {decision} row (with counter-evidence badge when applicable)
//   - 工具调用 list (InspectorToolCall expandables)
//   - 检索结果 row listing chunk IDs (InspectorRetrieval)
//   - 推理 row (collapsed, expand to read full reasoning text)
//
// Active hop styling: 2px left-border in --color-primary when this is
// the latest hop AND useChat.status === 'streaming'. Past hops render
// in --color-text-muted (no border).
//
// Counter-evidence hop badge (teal 历史层反证已检索) appears on hops
// where data-decision.decision === 'counter_evidence'. Per UI-SPEC §SC-4
// + §SC-6.

import { useState } from 'react';
import { BadgeCheck, ChevronDown, ChevronRight, Sparkles } from 'lucide-react';
import InspectorToolCall from './InspectorToolCall';
import InspectorRetrieval from './InspectorRetrieval';
import type { ToolCallRow, RetrievalRow, Hop } from './hop-types';

export interface InspectorHopProps {
  hop: Hop;
  isActive: boolean;
}

function formatDecisionLabel(decision: string): string {
  switch (decision) {
    case 'retrieve':
      return '检索';
    case 'answer':
      return '回答';
    case 'refuse':
      return '拒绝';
    case 'forced_stop':
      return '强制停止';
    case 'route':
      return '路由';
    case 'counter_evidence':
      return '反证检索';
    default:
      return decision;
  }
}

export default function InspectorHop({ hop, isActive }: InspectorHopProps) {
  const [expanded, setExpanded] = useState(isActive);
  const isCounterEvidence = hop.decision === 'counter_evidence';

  return (
    <details
      data-testid="inspector-hop"
      data-hop-step={hop.step}
      data-decision={hop.decision}
      data-active={String(isActive)}
      data-counter-evidence={String(isCounterEvidence)}
      open={isActive || expanded}
      className={
        'rounded-md border border-border bg-surface-1 ' +
        (isActive
          ? 'border-l-4 border-l-primary'
          : isCounterEvidence
            ? 'border-l-4 border-l-secondary'
            : 'border-l-4 border-l-transparent')
      }
    >
      <summary
        className="flex cursor-pointer items-center gap-2 px-3 py-2 text-sm font-semibold text-text"
        onClick={(e) => {
          // <details> toggles itself on summary click; we don't want to
          // double-toggle, so just expand via React state when click
          // bubbles up from a non-native detail handler.
          e.preventDefault();
          setExpanded((b) => !b);
        }}
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5 text-text-muted" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 text-text-muted" />
        )}
        <span className="font-mono text-primary">Hop {hop.step}</span>
        <span className="text-text-muted">·</span>
        <span>决策: {formatDecisionLabel(hop.decision)}</span>
        {hop.query && (
          <span
            data-testid="inspector-hop-query"
            className="ml-2 truncate font-mono text-xs text-text-muted"
            title={hop.query}
          >
            ({hop.query})
          </span>
        )}
        {isCounterEvidence && (
          <span
            data-testid="inspector-hop-counter-evidence-badge"
            className="ml-auto inline-flex items-center gap-1 rounded-md border border-secondary/40 bg-secondary/10 px-2 py-0.5 text-[11px] font-semibold text-secondary"
          >
            <Sparkles className="h-3 w-3" />
            历史层反证已检索
          </span>
        )}
        {isActive && !isCounterEvidence && (
          <span
            data-testid="inspector-hop-active-badge"
            className="ml-auto inline-flex items-center gap-1 rounded-md border border-primary/40 bg-primary/10 px-2 py-0.5 text-[11px] font-semibold text-primary"
          >
            <BadgeCheck className="h-3 w-3" />
            进行中
          </span>
        )}
      </summary>
      <div className="flex flex-col gap-3 border-t border-border p-3">
        {/* Reasoning row — collapsed by default. */}
        {hop.reasoning && (
          <details
            data-testid="inspector-hop-reasoning"
            className="rounded-md bg-bg"
          >
            <summary className="cursor-pointer px-2 py-1 text-xs font-semibold text-text-muted">
              推理（点击展开）
            </summary>
            <pre
              data-testid="inspector-hop-reasoning-body"
              className="mt-1 whitespace-pre-wrap px-2 pb-2 font-mono text-xs text-text"
            >
              {hop.reasoning}
            </pre>
          </details>
        )}
        {/* Tool calls list. */}
        {hop.toolCalls.length > 0 && (
          <div data-testid="inspector-hop-tools" className="flex flex-col gap-2">
            <div className="text-xs font-semibold text-text-muted">
              工具调用 ({hop.toolCalls.length})
            </div>
            {hop.toolCalls.map((tc) => (
              <InspectorToolCall key={tc.toolCallId} toolCall={tc} />
            ))}
          </div>
        )}
        {/* Retrieval results. */}
        {hop.retrievals.length > 0 && (
          <InspectorRetrieval retrievals={hop.retrievals} />
        )}
      </div>
    </details>
  );
}

export type { ToolCallRow, RetrievalRow, Hop };