// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// 56-04 Task 4 (PANEL-02) -- step-tree aggregation + rendering pins.
//
// The tree is a pure derivation over EXISTING signals (56-04 PLAN
// Task 4: "节点类型 = 现有四类信号的并集，不发明新数据源"):
//   1. retrieval hits  (source-document parts -> Hop.retrievals)
//   2. tool calls      (dynamic-tool / tool-* parts -> Hop.toolCalls)
//   3. planning steps   (plan slice todos / synthetic write_todos)
//   4. decision hops   (data-decision parts)
//
// Asserts:
//   A. buildStepTree() assembles all four groups in execution order
//      (retrieval -> tool -> plan -> decision).
//   B. active-vs-done: while streaming the LAST hop's nodes are active,
//      earlier hops are done; after the stream ends every node is done.
//   C. in_progress todos render as active nodes; the label uses
//      activeForm (present-tense) when present.
//   D. StepTree renders 进行中 / 完成 badges + spinner/check icons.
//   E. Empty tree falls back to the existing empty copy (distinct copy
//      for streaming vs idle).
//   F. extractSyntheticTodos() reads write_todos input/output.
//   G. The tree is mounted INSIDE the existing Agent 状态 tab -- no 8th
//      tab appears in DetailPanel (IA lock per 56-UI-SPEC section 4).
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import StepTree from '../components/inspector/StepTree';
import AgentInspector from '../components/inspector/AgentInspector';
import {
  buildStepTree,
  extractSyntheticTodos,
  type StepTreeNode,
} from '../components/inspector/step-tree';
import type { Hop } from '../components/inspector/hop-types';
import type { PlanTodo } from '../types/wire-types';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  useAppStore.setState({
    centerView: { kind: 'empty' },
    plan: { planByRunId: {}, currentRunId: null },
  } as never);
});

function makeHop(over: Partial<Hop> = {}): Hop {
  return {
    step: 1,
    decision: 'retrieve',
    toolCalls: [],
    retrievals: [],
    reasoning: '',
    ...over,
  };
}

const todos: PlanTodo[] = [
  { id: 't1', content: '读取差旅制度', status: 'completed' },
  { id: 't2', content: '比对标准额度', activeForm: '正在比对标准额度', status: 'in_progress' },
];

describe('buildStepTree (56-04 Task 4)', () => {
  it('aggregates the four existing signal sources in execution order', () => {
    const hops = [
      makeHop({
        step: 1,
        decision: 'retrieve',
        retrievals: [
          { chunkId: 'c-001', title: '员工差旅管理办法' },
          { chunkId: 'c-002' },
        ],
        toolCalls: [
          { toolCallId: 'tc1', toolName: 'search_authority', state: 'output-available', input: {}, output: {} },
          { toolCallId: 'tc2', toolName: 'write_todos', state: 'output-available', input: {}, output: { todos } },
        ],
      }),
      makeHop({ step: 2, decision: 'answer', confidence: 0.9 }),
    ];

    const nodes = buildStepTree(hops, todos, false);
    const kinds = nodes.map((n) => n.kind);

    // Group order: retrieval -> tool -> plan -> decision(s).
    expect(kinds).toEqual(['retrieve', 'tool', 'plan', 'retrieve', 'answer']);
    // Retrieval group carries one child per hit, chunkId on detail.
    const retrieveGroup = nodes[0];
    expect(retrieveGroup.children).toHaveLength(2);
    expect(retrieveGroup.children?.[0].detail).toBe('c-001');
    expect(retrieveGroup.children?.[0].label).toBe('员工差旅管理办法');
    // Tool group carries one child per call.
    const toolGroup = nodes[1];
    expect(toolGroup.children?.map((c) => c.label)).toEqual(['search_authority', 'write_todos']);
    // Plan group detail is completed/total.
    expect(nodes[2].detail).toBe('1/2');
  });

  it('marks the last hop active while streaming and everything done after', () => {
    const hops = [makeHop({ step: 1 }), makeHop({ step: 2, decision: 'answer' })];

    const streamingNodes = buildStepTree(hops, [], true);
    const lastHopNode = streamingNodes.find((n) => n.id === 'decision-hop-2');
    expect(lastHopNode?.status).toBe('active');
    expect(streamingNodes.find((n) => n.id === 'decision-hop-1')?.status).toBe('done');

    const doneNodes = buildStepTree(hops, [], false);
    expect(doneNodes.find((n) => n.id === 'decision-hop-2')?.status).toBe('done');
  });

  it('renders in_progress todos as active nodes using the activeForm label', () => {
    const nodes = buildStepTree([], todos, true);
    const planGroup = nodes.find((n) => n.kind === 'plan');
    expect(planGroup?.status).toBe('active');
    const inProgressChild = planGroup?.children?.find((c) => c.id === 'plan-t2');
    expect(inProgressChild?.status).toBe('active');
    expect(inProgressChild?.label).toBe('正在比对标准额度');
    expect(inProgressChild?.detail).toBe('进行中');
  });

  it('extractSyntheticTodos reads write_todos output/input (Pitfall 6 fallback)', () => {
    const hops = [
      makeHop({
        toolCalls: [
          { toolCallId: 'tc1', toolName: 'search_authority', state: 'output-available', input: {}, output: {} },
          { toolCallId: 'tc2', toolName: 'write_todos', state: 'output-available', input: {}, output: { todos } },
        ],
      }),
    ];
    expect(extractSyntheticTodos(hops)).toEqual(todos);

    const inputOnly = [
      makeHop({
        toolCalls: [
          { toolCallId: 'tc3', toolName: 'write_todos', state: 'input-available', input: { todos }, output: null },
        ],
      }),
    ];
    expect(extractSyntheticTodos(inputOnly)).toEqual(todos);

    expect(extractSyntheticTodos([makeHop()])).toEqual([]);
  });
});

describe('StepTree render (56-04 Task 4)', () => {
  const nodes: StepTreeNode[] = [
    {
      id: 'retrieve-hop-1',
      kind: 'retrieve',
      label: '检索 · Hop 1',
      status: 'done',
      detail: '1 命中',
      children: [
        { id: 'retrieve-hop-1-c-001', kind: 'retrieve', label: '员工差旅管理办法', status: 'done', detail: 'c-001' },
      ],
    },
    { id: 'decision-hop-2', kind: 'answer', label: '决策 · 回答', status: 'active', detail: '置信度 0.90' },
  ];

  it('renders nested nodes with 完成 / 进行中 badges and status icons', () => {
    render(<StepTree nodes={nodes} isStreaming />);
    expect(screen.getByTestId('step-tree')).toHaveAttribute('data-node-count', '2');
    expect(screen.getByTestId('step-tree-summary')).toHaveTextContent('共 2 步 · 1 进行中');

    const rows = screen.getAllByTestId('step-tree-node');
    // Group row + its child + the decision row.
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveAttribute('data-status', 'done');
    expect(rows[2]).toHaveAttribute('data-status', 'active');

    const badges = screen.getAllByTestId('step-tree-status-badge');
    expect(badges[0]).toHaveAttribute('data-status', 'done');
    expect(badges[0]).toHaveTextContent('完成');
    expect(badges[2]).toHaveAttribute('data-status', 'active');
    expect(badges[2]).toHaveTextContent('进行中');

    const icons = screen.getAllByTestId('step-tree-status-icon');
    expect(icons[0]).toHaveAttribute('data-status', 'done');
    expect(icons[2]).toHaveAttribute('data-status', 'active');

    expect(screen.getByText('员工差旅管理办法')).toBeInTheDocument();
    expect(screen.getByText('置信度 0.90')).toBeInTheDocument();
  });

  it('falls back to the existing empty copy, distinguishing streaming from idle', () => {
    const { rerender } = render(<StepTree nodes={[]} isStreaming />);
    const empty = screen.getByTestId('step-tree-empty');
    expect(empty).toHaveTextContent('等待第一个执行步骤…');
    expect(screen.queryByTestId('step-tree')).toBeNull();

    rerender(<StepTree nodes={[]} isStreaming={false} />);
    expect(screen.getByTestId('step-tree-empty')).toHaveTextContent(
      '本回合没有可聚合的步骤（普通问答不产生检索/工具/规划节点）。',
    );
  });
});

describe('AgentInspector + step tree integration (56-04 Task 4)', () => {
  function makeMessage(parts: UIMessage['parts']): UIMessage {
    return { id: 'm1', role: 'assistant', parts } as unknown as UIMessage;
  }

  it('mounts the step tree above the per-hop accordion, reading the plan slice', () => {
    useAppStore.setState({
      plan: {
        planByRunId: { 'run-1': { todos, generatedAt: 0, status: 'streaming' } },
        currentRunId: 'run-1',
      },
    } as never);

    const messages = [
      makeMessage([
        { type: 'data-decision', id: 'd1', data: { step: 1, decision: 'retrieve' } } as never,
        {
          type: 'source-document',
          sourceId: 'c-001',
          mediaType: 'text/markdown',
          title: '员工差旅管理办法',
        } as never,
        { type: 'data-decision', id: 'd2', data: { step: 2, decision: 'answer' } } as never,
      ]),
    ];

    render(<AgentInspector messages={messages} status="streaming" />);
    expect(screen.getByTestId('step-tree')).toBeInTheDocument();
    // Plan group came from the plan slice (not from a write_todos part).
    expect(screen.getAllByTestId('step-tree-node').some((n) => n.getAttribute('data-kind') === 'plan')).toBe(
      true,
    );
    // The existing accordion still renders both hops.
    expect(screen.getAllByTestId('inspector-hop')).toHaveLength(2);
  });
});
