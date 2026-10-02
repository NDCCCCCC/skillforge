// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 Task 2 — AgentInspector full Accordion tests.
//
// Pins the SC-4 (AGNT-06) acceptance assertions for the derived
// useChat.messages view:
//
//   1. Empty messages + status=ready → empty-state hero (暂无 Agent 状态).
//   2. status=streaming + no hops yet → loading state (等待 Agent 首个事件...).
//   3. Multi-hop message → one inspector-hop per data-decision step;
//      the LAST hop (when streaming) carries data-active="true".
//   4. Active hop has 2px left-border styling (border-l-4 + border-l-primary).
//   5. Counter-evidence hop surfaces the teal 历史层反证已检索 badge.
//   6. Tool call expansion reveals args + output (data-testid="inspector-tool-call-args").
//   7. Reasoning row is collapsed by default (data-testid="inspector-hop-reasoning"
//      element has summary with 推理（点击展开） label; body hidden until expanded).
//   8. Retrieval chunk IDs render as buttons that dispatch setCenterView.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import AgentInspector from '../components/inspector/AgentInspector';
import { useAppStore } from '../store/app-store';

afterEach(() => {
  cleanup();
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

function makeMessage(parts: UIMessage['parts'], role: 'user' | 'assistant' = 'assistant', id = 'm1'): UIMessage {
  return {
    id,
    role,
    parts,
  } as unknown as UIMessage;
}

describe('AgentInspector', () => {
  it('renders the empty-state hero when messages is empty and status=ready', () => {
    render(<AgentInspector messages={[]} status="ready" />);
    const empty = screen.getByTestId('agent-inspector-empty');
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent('暂无 Agent 状态');
  });

  it('renders the loading state when status=streaming and no hops are present', () => {
    render(<AgentInspector messages={[]} status="streaming" />);
    expect(screen.getByTestId('agent-inspector-loading')).toBeInTheDocument();
    expect(screen.getByTestId('agent-inspector-loading')).toHaveTextContent('等待 Agent 首个事件...');
    expect(screen.queryByTestId('agent-inspector-empty')).toBeNull();
  });

  it('groups parts into one hop per data-decision step and marks the active hop (streaming)', () => {
    const messages = [
      makeMessage([
        {
          type: 'data-decision',
          id: 'd1',
          data: { step: 1, decision: 'retrieve', query: '差旅住宿' },
        },
        {
          type: 'dynamic-tool',
          toolCallId: 'tc1',
          toolName: 'search_authority',
          state: 'output-available',
          input: { q: '差旅住宿' },
          output: { hits: ['c-001', 'c-002'] },
        },
        {
          type: 'source-document',
          sourceId: 'c-001',
          mediaType: 'text/markdown',
          filename: 'employee-travel.md',
          title: '员工差旅管理办法',
        },
        { type: 'reasoning', state: 'done', text: '需要先检索权威层。' },
        {
          type: 'data-decision',
          id: 'd2',
          data: { step: 2, decision: 'answer', confidence: 0.9 },
        },
      ]),
    ];
    render(<AgentInspector messages={messages} status="streaming" />);
    const inspector = screen.getByTestId('agent-inspector');
    expect(inspector).toBeInTheDocument();

    const hops = screen.getAllByTestId('inspector-hop');
    expect(hops).toHaveLength(2);
    expect(hops[0]).toHaveAttribute('data-hop-step', '1');
    expect(hops[0]).toHaveAttribute('data-decision', 'retrieve');
    expect(hops[0]).toHaveAttribute('data-active', 'false');
    expect(hops[1]).toHaveAttribute('data-hop-step', '2');
    expect(hops[1]).toHaveAttribute('data-decision', 'answer');
    // Last hop with status=streaming is the active hop.
    expect(hops[1]).toHaveAttribute('data-active', 'true');
  });

  it('active hop has 2px left-border in --color-primary (border-l-4 border-l-primary)', () => {
    const messages = [
      makeMessage([
        { type: 'data-decision', id: 'd1', data: { step: 1, decision: 'retrieve' } },
        { type: 'data-decision', id: 'd2', data: { step: 2, decision: 'answer' } },
      ]),
    ];
    const { container } = render(<AgentInspector messages={messages} status="streaming" />);
    const hops = screen.getAllByTestId('inspector-hop');
    // The active hop is the LAST hop when streaming; it must carry
    // border-l-4 + border-l-primary classes for the 2px accent left-border.
    expect(hops[1].className).toContain('border-l-4');
    expect(hops[1].className).toContain('border-l-primary');
    // Past hops use border-l-transparent (no visible border).
    expect(hops[0].className).toContain('border-l-transparent');
    // Sanity: at least one element on the page has the primary accent border.
    const allBordered = container.querySelectorAll('.border-l-primary');
    expect(allBordered.length).toBeGreaterThanOrEqual(1);
  });

  it('surfaces the counter-evidence badge (teal 历史层反证已检索) on the SC-6 hop', () => {
    const messages = [
      makeMessage([
        { type: 'data-decision', id: 'd1', data: { step: 1, decision: 'retrieve' } },
        {
          type: 'data-decision',
          id: 'd2',
          data: { step: 2, decision: 'counter_evidence', query: '历史层' },
        },
      ]),
    ];
    render(<AgentInspector messages={messages} status="ready" />);
    const hops = screen.getAllByTestId('inspector-hop');
    expect(hops[1]).toHaveAttribute('data-decision', 'counter_evidence');
    expect(hops[1]).toHaveAttribute('data-counter-evidence', 'true');
    const badge = screen.getByTestId('inspector-hop-counter-evidence-badge');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveTextContent('历史层反证已检索');
  });

  it('expands a tool call to reveal args + output (no dangerouslySetInnerHTML)', () => {
    const messages = [
      makeMessage([
        {
          type: 'data-decision',
          id: 'd1',
          data: { step: 1, decision: 'retrieve' },
        },
        {
          type: 'dynamic-tool',
          toolCallId: 'tc1',
          toolName: 'search_authority',
          state: 'output-available',
          input: { q: '差旅' },
          output: { hits: ['c-001'] },
        },
      ]),
    ];
    render(<AgentInspector messages={messages} status="ready" />);
    const toolCall = screen.getByTestId('inspector-tool-call');
    expect(toolCall).toHaveAttribute('data-tool-name', 'search_authority');
    // Initial state: collapsed (no `open` attribute on <details>).
    expect(toolCall).not.toHaveAttribute('open');

    // Click summary to expand.
    fireEvent.click(toolCall.querySelector('summary') as HTMLElement);
    expect(screen.getByTestId('inspector-tool-call-args')).toBeInTheDocument();
    expect(screen.getByTestId('inspector-tool-call-output')).toBeInTheDocument();
    expect(screen.getByTestId('inspector-tool-call-args').textContent).toContain('差旅');
    expect(screen.getByTestId('inspector-tool-call-output').textContent).toContain('c-001');
  });

  it('reasoning row is collapsed by default with the 推理（点击展开） label', () => {
    const messages = [
      makeMessage([
        { type: 'data-decision', id: 'd1', data: { step: 1, decision: 'retrieve' } },
        { type: 'reasoning', state: 'done', text: '需要先检索权威层。' },
      ]),
    ];
    render(<AgentInspector messages={messages} status="ready" />);
    const reasoning = screen.getByTestId('inspector-hop-reasoning');
    expect(reasoning).toBeInTheDocument();
    // Native <details> renders children but hides them when closed. The
    // collapsed state is observable via the absence of the `open`
    // attribute on the <details> element.
    expect(reasoning).not.toHaveAttribute('open');
    expect(reasoning.querySelector('summary')).toHaveTextContent('推理（点击展开）');
  });

  it('retrieval chunk IDs render as buttons that dispatch setCenterView', () => {
    const messages = [
      makeMessage([
        { type: 'data-decision', id: 'd1', data: { step: 1, decision: 'retrieve' } },
        {
          type: 'source-document',
          sourceId: 'c-001',
          mediaType: 'text/markdown',
          title: '员工差旅管理办法',
        },
      ]),
    ];
    render(<AgentInspector messages={messages} status="ready" />);
    const row = screen.getByTestId('inspector-retrieval-row');
    expect(row).toHaveAttribute('data-chunk-id', 'c-001');
    fireEvent.click(row);
    const cv = useAppStore.getState().centerView;
    expect(cv.kind).toBe('chunk-source');
    if (cv.kind === 'chunk-source') {
      expect(cv.chunkId).toBe('c-001');
    }
  });
});