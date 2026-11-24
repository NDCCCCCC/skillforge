// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// AgentInspector — Plan 05-05 / SC-4 (AGNT-06) full Accordion tree.
//
// Walks useChat.messages of the latest assistant message, groups parts
// into "hops" using data-decision's DecisionPayload.step as the hop
// boundary, and renders one <InspectorHop> per hop.
//
// Per D14: messages stay owned by useChat. The inspector reads them
// via a `messages` prop passed down from <App> through <CenterPanel>.
// The prop is NOT stored in Zustand — it's a vanilla React prop, which
// keeps the message stream and the UI state in their respective owners.
//
// Per UI-SPEC §SC-4:
//   - Active hop (last hop + status==='streaming') has 2px left-border
//     in --color-primary.
//   - Counter-evidence hop (data-decision.decision === 'counter_evidence')
//     surfaces a teal badge (历史层反证已检索) — same accent as the
//     Phase-6 SC-6 trace, used to anchor the SC-4 ↔ SC-6 cross-ref.
//   - Per-hop: 决策 row + 工具调用 list + 检索结果 row + 推理 row (collapsed).
//
// Empty state: 暂无 Agent 状态 + body.
// Loading state: 等待 Agent 首个事件... (when streaming && no parts yet).
//
// 56-04 Task 4 (PANEL-02): the run's four signal sources are aggregated
// into ONE Langfuse-style step tree (step-tree.ts buildStepTree) and
// rendered ABOVE the per-hop accordion. Data sources are the existing
// ones only -- retrieval hits / tool calls / data-decision hops come
// from the message parts, planning steps come from the plan slice
// (data-plan-trace) with the synthetic write_todos fallback. No 8th tab
// is added: the tree lives inside the existing 「Agent 状态」 tab.

import { useMemo } from 'react';
import { Activity } from 'lucide-react';
import type { UIMessage } from 'ai';
import InspectorHop from './InspectorHop';
import StepTree from './StepTree';
import KGAnchorLine from './KGAnchorLine';
import { buildStepTree, extractSyntheticTodos } from './step-tree';
import { useAppStore } from '../../store/app-store';
import type { Hop, ToolCallRow, RetrievalRow, DecisionValue } from './hop-types';

export interface AgentInspectorProps {
  /** Latest useChat.messages array (read directly via prop, NOT Zustand). */
  messages: UIMessage[];
  /** useChat.status — 'submitted' | 'streaming' | 'ready' | 'error'. */
  status: 'submitted' | 'streaming' | 'ready' | 'error';
  /** Legacy loading prop kept for backward compat with 05-03 callers. */
  streaming?: boolean;
}

function isDecisionValue(value: string): value is DecisionValue {
  return [
    'retrieve',
    'answer',
    'refuse',
    'forced_stop',
    'route',
    'counter_evidence',
  ].includes(value);
}

function deriveHops(messages: UIMessage[]): Hop[] {
  // Walk in reverse to find the latest assistant message, then walk
  // its parts in order. We do not collapse hops across multiple
  // assistant turns — the inspector reflects the current turn only.
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (m.role !== 'assistant') continue;

    const hops: Hop[] = [];
    let current: Hop = {
      step: 0,
      decision: 'answer',
      toolCalls: [],
      retrievals: [],
      reasoning: '',
    };
    let started = false;

    for (const part of m.parts) {
      const type = (part as { type: string }).type;
      if (type === 'data-decision') {
        const data = (part as { data?: { step?: number; decision?: string; query?: string; confidence?: number } }).data;
        const step = typeof data?.step === 'number' ? data.step : current.step + 1;
        const decision = isDecisionValue(data?.decision ?? '') ? (data?.decision as DecisionValue) : 'answer';
        current = {
          step,
          decision,
          query: data?.query,
          confidence: data?.confidence,
          toolCalls: [],
          retrievals: [],
          reasoning: '',
        };
        hops.push(current);
        started = true;
      } else if (!started) {
        // Skip leading non-decision parts (start, etc.) — only build
        // hops once the first data-decision arrives.
        continue;
      } else if (type === 'tool-input-available' || type === 'dynamic-tool' || type.startsWith('tool-')) {
        const p = part as {
          toolCallId?: string;
          toolName?: string;
          state?: string;
          input?: unknown;
          output?: unknown;
          errorText?: unknown;
        };
        const toolCall: ToolCallRow = {
          toolCallId: p.toolCallId ?? `tc-${current.toolCalls.length + 1}`,
          toolName: p.toolName ?? type,
          state: p.state ?? 'unknown',
          input: p.input,
          output: p.output,
          errorText: typeof p.errorText === 'string' ? p.errorText : undefined,
        };
        current.toolCalls.push(toolCall);
      } else if (type === 'source-document') {
        const p = part as {
          sourceId?: string;
          filename?: string;
          title?: string;
        };
        if (p.sourceId) {
          const row: RetrievalRow = {
            chunkId: p.sourceId,
            filename: p.filename,
            title: p.title,
          };
          current.retrievals.push(row);
        }
      } else if (type === 'reasoning') {
        const p = part as { text?: string };
        if (p.text) {
          current.reasoning = current.reasoning
            ? current.reasoning + '\n' + p.text
            : p.text;
        }
      }
    }

    return hops;
  }

  return [];
}

export default function AgentInspector({
  messages,
  status,
  streaming,
}: AgentInspectorProps) {
  const isStreaming = streaming ?? (status === 'streaming' || status === 'submitted');
  const hops = useMemo(() => deriveHops(messages), [messages]);

  // 56-04 Task 4 -- planning steps come from the plan slice (the
  // authoritative live state per D14), with PlanTracePart's synthetic
  // write_todos extraction as the fallback when the slice has no
  // snapshot for this run (Pitfall 6).
  const currentRunId = useAppStore((s) => s.plan.currentRunId);
  const planState = useAppStore((s) =>
    currentRunId ? s.plan.planByRunId[currentRunId] : undefined,
  );
  // Phase 57-02 (ONT-02) -- the anchor line reads the ontology_anchor
  // data-decision event emitted by the BE (kgAvailable / entitiesMatched /
  // anchorEntities). Absent event -> undefined -> the line renders
  // nothing (the inspector's own states own that moment).
  const kgAnchor = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.role !== 'assistant') continue;
      for (const part of m.parts) {
        const p = part as {
          type?: string;
          data?: {
            decision?: string;
            kgAvailable?: boolean;
            entitiesMatched?: number;
            anchorEntities?: string;
          };
        };
        if (p.type === 'data-decision' && p.data?.decision === 'ontology_anchor') {
          return p.data;
        }
      }
    }
    return undefined;
  }, [messages]);

  const stepNodes = useMemo(() => {
    const sliceTodos = planState && planState.todos.length > 0 ? planState.todos : [];
    const todos = sliceTodos.length > 0 ? sliceTodos : extractSyntheticTodos(hops);
    return buildStepTree(hops, todos, isStreaming);
  }, [hops, planState, isStreaming]);

  if (hops.length === 0) {
    if (isStreaming) {
      return (
        <div
          data-testid="agent-inspector-loading"
          className="flex flex-col items-center justify-center text-center min-h-[60vh]"
        >
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
            <Activity className="h-6 w-6 text-primary animate-pulse" />
          </div>
          <div className="text-base text-text-muted">等待 Agent 首个事件...</div>
        </div>
      );
    }
    return (
      <div
        data-testid="agent-inspector-empty"
        className="flex flex-col items-center justify-center text-center min-h-[60vh]"
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
          <Activity className="h-6 w-6 text-primary" />
        </div>
        <div className="text-2xl font-semibold text-text">暂无 Agent 状态</div>
        <p className="mt-2 text-base text-text-muted max-w-sm">
          提出问题后，此处实时显示当前回合的工具调用、检索（含 chunk ID）与中间推理。
        </p>
      </div>
    );
  }

  const activeHopStep = isStreaming ? hops[hops.length - 1].step : null;

  return (
    <div data-testid="agent-inspector" className="flex flex-col gap-3">
      <div className="text-xs text-text-muted">
        当前回合 {hops.length} 跳{isStreaming ? ' · 进行中' : ''}
      </div>
      <KGAnchorLine
        kgAvailable={kgAnchor?.kgAvailable}
        entitiesMatched={kgAnchor?.entitiesMatched}
        anchorEntities={kgAnchor?.anchorEntities}
      />
      <StepTree nodes={stepNodes} isStreaming={isStreaming} />
      {hops.map((hop) => (
        <InspectorHop
          key={hop.step}
          hop={hop}
          isActive={hop.step === activeHopStep}
        />
      ))}
    </div>
  );
}