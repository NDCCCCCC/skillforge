// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 + Plan 04-02 Task 2 + Task 3 — MessageBubble tests.
//
// Pins the seven acceptance assertions from the plan:
//  1. All 7 part types render without crash (text, reasoning, source-url,
//     source-document, tool parts, data-policy, step-start).
//  2. Reasoning renders BEFORE text in DOM order (D-08 invariant).
//     [Wave 2: the placeholder was replaced with the real
//     ReasoningPart; testid moved from 'reasoning' to
//     'reasoning-preview'.]
//  3. SourceUrlPart with javascript: URL renders citation-blocked span,
//     NOT a link (T-1-08).
//  4. SourceUrlPart with allowlisted URL renders <a rel="noopener
//     noreferrer" target="_blank">.
//  5. DataPolicyPart reads part.data (not part.policy) — the .data key
//     wins (Pitfall 2). [Wave 2: dispatches to VerdictCard via
//     isVerdict(), not the JSON-serializing placeholder.]
//  6. Tool parts tool-${name} and dynamic-tool both render (Pitfall 3).
//  7. Unknown part types render nothing (silently dropped per spec).
//  8. data-decision chunks (Plan 03-01 wire variant, no renderer yet)
//     drop gracefully — forward-compat: older UI tolerates newer wire.
//  9. 260817-hck: when the text parts verbatim contain verdict.answer,
//     the VerdictCard's verdict-answer block is suppressed (dedupe);
//     when they do not, verdict-answer still renders inside the card.

import { cleanup, render, screen } from '@testing-library/react';
import type { UIMessage } from 'ai';
import { afterEach, describe, expect, it } from 'vitest';
import { MessageBubble } from '../components/chat/MessageBubble';

afterEach(cleanup);

describe('MessageBubble', () => {
  it('renders all 7 part types without crash', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: 'Hello.' },
        { type: 'reasoning', text: 'thinking...', state: 'done' },
        {
          type: 'source-url',
          sourceId: 'c1',
          url: 'http://skillforge.local/chunk',
          title: 'Chunk 1',
        },
        {
          type: 'source-document',
          sourceId: 'c2',
          mediaType: 'text/markdown',
          title: 'Doc 2',
        },
        {
          type: 'dynamic-tool',
          toolName: 'dynamic-tool',
          toolCallId: 'tc1',
          state: 'output-available',
          input: { query: 'foo' },
          output: { result: 'bar' },
        },
        {
          type: 'data-policy',
          id: 'p1',
          data: {
            status: 'ok',
            confidence: 0.9,
            citations: [],
            review_required: false,
            compliance_status: 'compliant',
          },
        },
        { type: 'step-start' },
      ],
    };

    render(<MessageBubble message={message} />);
    expect(screen.getByText('Hello.')).toBeInTheDocument();
    expect(screen.getByTestId('reasoning-preview')).toBeInTheDocument();
    expect(screen.getByTestId('citation-link')).toBeInTheDocument();
    expect(screen.getByTestId('source-document')).toBeInTheDocument();
    expect(screen.getByTestId('tool-part')).toBeInTheDocument();
    // Wave 2: DataPolicyPart dispatches to VerdictCard via isVerdict().
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.getByTestId('step-start')).toBeInTheDocument();
  });

  it('renders reasoning BEFORE text in DOM order (D-08)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'reasoning', text: 'reasoning content', state: 'done' },
        { type: 'text', text: 'answer content' },
      ],
    };

    const { container } = render(<MessageBubble message={message} />);
    const reasoningEl = screen.getByTestId('reasoning-preview');
    const textEl = screen.getByText('answer content');
    // Reasoning must come BEFORE text in document order.
    const pos = reasoningEl.compareDocumentPosition(textEl);
    expect(pos & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Sanity: not the same node.
    expect(container.contains(reasoningEl)).toBe(true);
    expect(container.contains(textEl)).toBe(true);
  });

  it('SourceUrlPart blocks javascript: URL (T-1-08)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-url',
          sourceId: 'c1',
          url: 'javascript:alert(1)',
          title: 'malicious',
        },
      ],
    };

    render(<MessageBubble message={message} />);
    expect(screen.queryByTestId('citation-link')).toBeNull();
    expect(screen.getByTestId('citation-blocked')).toBeInTheDocument();
  });

  it('SourceUrlPart with allowlisted URL renders safe <a>', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-url',
          sourceId: 'c1',
          url: 'http://skillforge.local/chunk',
          title: 'Chunk 1',
        },
      ],
    };

    render(<MessageBubble message={message} />);
    const link = screen.getByTestId('citation-link');
    expect(link.tagName).toBe('A');
    expect(link).toHaveAttribute('href', 'http://skillforge.local/chunk');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('DataPolicyPart reads part.data (NOT part.policy)', () => {
    // Fixture helper shape: emit BOTH keys; test asserts .data wins.
    const message = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'data-policy',
          id: 'p1',
          data: {
            status: 'ok',
            confidence: 0.91,
            citations: [],
            review_required: false,
            compliance_status: 'compliant',
          },
          // Cast: include the legacy .policy key as a stale-fixture
          // simulation; the component should ignore it.
          policy: { status: 'WRONG_FROM_POLICY' },
        },
      ],
    } as unknown as UIMessage;

    render(<MessageBubble message={message} />);
    // Wave 2: DataPolicyPart dispatches to VerdictCard; the legacy
    // .policy key is ignored and .data drives the verdict status.
    const card = screen.getByTestId('verdict-card');
    expect(card).toHaveAttribute('data-verdict-status', 'ok');
    expect(card).toHaveAttribute('data-compliance-status', 'compliant');
    expect(card).toHaveAttribute('data-review-required', 'false');
  });

  it('Tool parts tool-${name} and dynamic-tool both render', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'dynamic-tool',
          toolName: 'dynamic-tool',
          toolCallId: 'tc1',
          state: 'output-available',
          input: { q: 'x' },
          output: { r: 'y' },
        },
        // Fall-through arm: a tool- prefixed type we didn't enumerate.
        // We use a string cast since TS would reject unknown tools.
        {
          type: 'tool-search_authority',
          toolCallId: 'tc2',
          state: 'output-available',
          input: { query: 'foo' },
          output: { results: [] },
        } as unknown as UIMessage['parts'][number],
      ],
    };

    render(<MessageBubble message={message} />);
    const toolParts = screen.getAllByTestId('tool-part');
    expect(toolParts).toHaveLength(2);
  });

  it('drops unknown part types silently', () => {
    const message = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: 'visible text' },
        { type: 'some-future-type', payload: 'hidden' },
      ],
    } as unknown as UIMessage;

    render(<MessageBubble message={message} />);
    expect(screen.getByText('visible text')).toBeInTheDocument();
    expect(screen.queryByText('hidden')).toBeNull();
  });

  it('drops data-decision chunks gracefully (no renderer yet)', () => {
    // data-decision was added to the SkillForgeChunk union in Plan 03-01
    // (commit 0b43156) as the ARAG-01 / SC-1 per-hop decision substrate.
    // The agent package (Plan 03-03) will emit it; no DataDecisionPart
    // renderer exists yet. The dispatcher must silently drop it
    // (default → return null) so an older UI tolerates the newer wire
    // variant without crashing (AI SDK v7 older-client/newer-server
    // forward-compat contract). Distinct from the generic "unknown drop"
    // test above: data-decision IS a declared union member, just unhandled.
    const message = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: 'visible answer' },
        {
          type: 'data-decision',
          id: 'd1',
          data: {
            step: 1,
            decision: 'retrieve',
            toolCallId: 'tc1',
            query: 'should-not-leak',
            confidence: 0.42,
          },
        },
      ],
    } as unknown as UIMessage;

    render(<MessageBubble message={message} />);
    expect(screen.getByText('visible answer')).toBeInTheDocument();
    // Decision payload must NOT leak into the rendered DOM.
    expect(screen.queryByText('retrieve')).toBeNull();
    expect(screen.queryByText('should-not-leak')).toBeNull();
    expect(screen.queryByText('0.42')).toBeNull();
    expect(screen.queryByTestId('data-decision')).toBeNull();
  });

  // --- 260817-hck: verdict-answer dedupe against the chat text body ---

  const factualVerdictData = {
    status: 'ok',
    confidence: 0.97,
    citations: [
      { chunk_id: 'c-001', span: '采购合同金额 50 万元以下由部门负责人审批', tier: '权威' },
    ],
    review_required: false,
    answer: '由部门负责人审批',
    compliance_status: 'not_applicable',
  };

  it('hides in-card verdict-answer when the text part already contains the conclusion (260817-hck)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：由部门负责人审批。' },
        { type: 'data-policy', id: 'p1', data: factualVerdictData },
      ],
    };

    render(<MessageBubble message={message} />);
    // The chat body keeps the conclusion text (it is the useChat anchor).
    expect(screen.getByText('结论：由部门负责人审批。')).toBeInTheDocument();
    // The card renders but WITHOUT the duplicated verdict-answer block.
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.queryByTestId('verdict-answer')).toBeNull();
    expect(screen.getByTestId('citation-cluster')).toBeInTheDocument();
  });

  it('keeps in-card verdict-answer when the text part does NOT contain the answer (260817-hck)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: '已为你查询相关条款。' },
        { type: 'data-policy', id: 'p1', data: factualVerdictData },
      ],
    };

    render(<MessageBubble message={message} />);
    expect(screen.getByText('已为你查询相关条款。')).toBeInTheDocument();
    const answer = screen.getByTestId('verdict-answer');
    expect(answer).toBeInTheDocument();
    expect(answer).toHaveTextContent('由部门负责人审批');
  });

  // --- 260817-i36 Task 2 (③): chunk-disambiguated source-document chip titles ---

  it('renders the chunk-disambiguated source-document title without the legacy "doc:" prefix (260817-i36)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-document',
          sourceId: '3',
          mediaType: 'text/markdown',
          title: 'authority/annual_leave.md · chunk 3',
        },
      ],
    };

    render(<MessageBubble message={message} />);
    const chip = screen.getByTestId('source-document');
    expect(chip).toHaveTextContent('annual_leave.md · chunk 3');
    // The Title is self-describing now (backend appends "· chunk N"),
    // so the legacy "doc:" prefix is dropped.
    expect(chip).not.toHaveTextContent('doc:');
  });

  it('renders distinguishable chips for two chunks of the same document (260817-i36)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-document',
          sourceId: '3',
          mediaType: 'text/markdown',
          title: 'authority/annual_leave.md · chunk 3',
        },
        {
          type: 'source-document',
          sourceId: '7',
          mediaType: 'text/markdown',
          title: 'authority/annual_leave.md · chunk 7',
        },
      ],
    };

    render(<MessageBubble message={message} />);
    const chips = screen.getAllByTestId('source-document');
    expect(chips).toHaveLength(2);
    expect(chips[0]).toHaveTextContent('annual_leave.md · chunk 3');
    expect(chips[1]).toHaveTextContent('annual_leave.md · chunk 7');
  });
});
