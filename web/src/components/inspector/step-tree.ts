// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// step-tree — 56-04 Task 4 (PANEL-02) Langfuse-style trace aggregation.
//
// One answer produces ONE tree. The nodes are the UNION of the four
// signal sources the FE already receives -- no new wire field, no new
// fetch:
//
//   1. retrieval hits   -- source-document parts grouped per hop
//                          (InspectorRetrieval's data, reused)
//   2. tool calls       -- dynamic-tool / tool-* parts
//                          (InspectorToolCall's data, reused)
//   3. planning steps   -- plan slice todos (data-plan-trace) OR the
//                          synthetic write_todos tool-call fallback
//   4. decision hops    -- data-decision parts (the hop boundary)
//
// Execution state: a node is `active` when the chat is still streaming
// and it belongs to the current (last) hop / the plan's in_progress
// todo; everything else is `done`. The tree is a pure derivation --
// buildStepTree() takes plain data and returns plain data, so the
// assembly is unit-testable without a DOM.
//
// PHASE 58 EXTENSION SLOT: `StepTreeNode.channels` is declared but
// never populated or rendered today. Phase 58 PROV-01/02 fills it with
// per-channel retrieval scores + Citation origin/via once the
// data-retrieval SSE exists (56-04 PLAN 显式 out of scope).

import type { PlanTodo } from '../../types/wire-types';
import type { Hop } from './hop-types';

/** What kind of work this node represents. */
export type StepNodeKind = 'retrieve' | 'tool' | 'plan' | 'answer';

/** active = 执行中 (spinner) / done = 已完成 (check). */
export type StepNodeStatus = 'active' | 'done';

export interface StepTreeNode {
  /** Stable key within one tree (kind + source identity). */
  id: string;
  kind: StepNodeKind;
  /** Display label already localized (zh-CN literal). */
  label: string;
  status: StepNodeStatus;
  /** Short right-aligned detail (chunk id, tool state, todo status). */
  detail?: string;
  /**
   * Phase 58 PROV-01/02 extension slot — per-channel retrieval scores /
   * citation origin+via. Declared here so the node type is stable when
   * Phase 58 lands; NEVER rendered by 56-04 (explicit out of scope).
   */
  channels?: unknown;
  children?: StepTreeNode[];
}

/** Plan todo status -> tree node status. in_progress is the only
 * running state; pending/completed map to done-ish display but keep
 * their own detail string so the user can tell them apart. */
function todoStatus(todo: PlanTodo): StepNodeStatus {
  return todo.status === 'in_progress' ? 'active' : 'done';
}

const TODO_STATUS_ZH: Record<PlanTodo['status'], string> = {
  pending: '待开始',
  in_progress: '进行中',
  completed: '已完成',
};

const DECISION_ZH: Record<string, string> = {
  retrieve: '检索',
  answer: '回答',
  refuse: '拒绝',
  forced_stop: '强制停止',
  route: '路由',
  counter_evidence: '反证检索',
};

/**
 * Aggregate the four signal sources into one ordered tree.
 *
 * Order mirrors the natural execution order: 检索 -> 工具调用 ->
 * 规划步骤 -> 回答. Within each group the source order is preserved.
 *
 * @param hops       derived from data-decision parts (AgentInspector)
 * @param todos      plan slice todos (may be empty — chat archetype runs
 *                   never write todos, which is an EXPECTED empty, not a
 *                   hole; see 56-04 AUDIT-WRITE-MATRIX.md)
 * @param isStreaming useChat.status in {submitted, streaming}
 */
export function buildStepTree(
  hops: Hop[],
  todos: PlanTodo[],
  isStreaming: boolean,
): StepTreeNode[] {
  const nodes: StepTreeNode[] = [];

  // 1. Retrieval group -- one child per hit chunk id. The group node is
  // active while its hop is the last hop and the stream is live.
  hops.forEach((hop, hopIndex) => {
    if (hop.retrievals.length === 0) return;
    const isLastHop = hopIndex === hops.length - 1;
    const status: StepNodeStatus = isStreaming && isLastHop ? 'active' : 'done';
    nodes.push({
      id: `retrieve-hop-${hop.step}`,
      kind: 'retrieve',
      label: `检索 · Hop ${hop.step}`,
      status,
      detail: `${hop.retrievals.length} 命中`,
      children: hop.retrievals.map((r) => ({
        id: `retrieve-hop-${hop.step}-${r.chunkId}`,
        kind: 'retrieve' as StepNodeKind,
        label: r.title ?? r.filename ?? r.chunkId,
        status: 'done' as StepNodeStatus,
        detail: r.chunkId,
      })),
    });
  });

  // 2. Tool-call group -- one child per call. A call whose state is
  // still input-available is shown as running while streaming.
  const allToolCalls = hops.flatMap((h) => h.toolCalls);
  if (allToolCalls.length > 0) {
    const anyRunning = allToolCalls.some(
      (tc) => tc.state === 'input-available' || tc.state === 'call' || tc.state === 'unknown',
    );
    nodes.push({
      id: 'tool-group',
      kind: 'tool',
      label: '工具调用',
      status: isStreaming && anyRunning ? 'active' : 'done',
      detail: `${allToolCalls.length} 次`,
      children: allToolCalls.map((tc) => ({
        id: `tool-${tc.toolCallId}`,
        kind: 'tool' as StepNodeKind,
        label: tc.toolName,
        status:
          isStreaming && (tc.state === 'input-available' || tc.state === 'unknown')
            ? ('active' as StepNodeStatus)
            : ('done' as StepNodeStatus),
        detail: tc.state,
      })),
    });
  }

  // 3. Planning group -- the plan slice todos (data-plan-trace) or the
  // synthetic write_todos fallback both land here as plain PlanTodo[]
  // (the caller resolves the source; this builder stays source-agnostic).
  if (todos.length > 0) {
    const completed = todos.filter((t) => t.status === 'completed').length;
    nodes.push({
      id: 'plan-group',
      kind: 'plan',
      label: '规划步骤',
      status:
        isStreaming && todos.some((t) => t.status === 'in_progress') ? 'active' : 'done',
      detail: `${completed}/${todos.length}`,
      children: todos.map((t) => ({
        id: `plan-${t.id}`,
        kind: 'plan' as StepNodeKind,
        label: t.status === 'in_progress' && t.activeForm ? t.activeForm : t.content,
        status: todoStatus(t),
        detail: TODO_STATUS_ZH[t.status],
      })),
    });
  }

  // 4. Decision nodes -- the hop itself (决策 + 置信度). Last hop while
  // streaming is the active one; earlier hops are completed.
  hops.forEach((hop, hopIndex) => {
    const isLastHop = hopIndex === hops.length - 1;
    nodes.push({
      id: `decision-hop-${hop.step}`,
      kind: hop.decision === 'retrieve' ? 'retrieve' : 'answer',
      label: `决策 · ${DECISION_ZH[hop.decision] ?? hop.decision}`,
      status: isStreaming && isLastHop ? 'active' : 'done',
      detail:
        typeof hop.confidence === 'number' ? `置信度 ${hop.confidence.toFixed(2)}` : undefined,
    });
  });

  return nodes;
}


/** The synthetic write_todos fallback (Pitfall 6): when the plan slice has
 * no snapshot for the run, a write_todos tool call still carries the todo
 * list in its input/output. Reuses PlanTracePart's extraction rule so the
 * two surfaces never disagree. */
export function extractSyntheticTodos(hops: Hop[]): PlanTodo[] {
  for (const hop of hops) {
    for (const tc of hop.toolCalls) {
      if (tc.toolName !== 'write_todos') continue;
      const output = tc.output as { todos?: PlanTodo[] } | undefined;
      if (output && Array.isArray(output.todos) && output.todos.length > 0) {
        return output.todos;
      }
      const input = tc.input as { todos?: PlanTodo[] } | undefined;
      if (input && Array.isArray(input.todos) && input.todos.length > 0) {
        return input.todos;
      }
    }
  }
  return [];
}
