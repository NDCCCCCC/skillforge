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

import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import type { UIMessage } from 'ai';
import { afterEach, describe, expect, it } from 'vitest';
import { MessageBubble } from '../components/chat/MessageBubble';
import { useAppStore } from '../store/app-store';

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

  it('omits the VerdictCard entirely for an ok factual verdict in chat — doc chips are the single citation UI (260817-i36 ②)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：由部门负责人审批。' },
        {
          type: 'source-document',
          sourceId: 'c-001',
          mediaType: 'text/markdown',
          title: 'authority/contract_approval.md · chunk c-001',
        },
        { type: 'data-policy', id: 'p1', data: factualVerdictData },
      ],
    };

    render(<MessageBubble message={message} />);
    // The chat body keeps the conclusion text (it is the useChat anchor).
    expect(screen.getByText('结论：由部门负责人审批。')).toBeInTheDocument();
    // 260817-i36 (②): the ok factual card is fully redundant in chat —
    // no VerdictCard frame, no duplicated CitationCluster / citation
    // count; the source-document chip under the conclusion is the one
    // citation UI (260817-hck's verdict-answer dedupe is subsumed).
    expect(screen.queryByTestId('verdict-card')).toBeNull();
    expect(screen.queryByTestId('citation-cluster')).toBeNull();
    expect(screen.queryByText(/条引用/)).toBeNull();
    expect(screen.getByTestId('source-document')).toBeInTheDocument();
  });

  it('renders the SLIM card for non-ok verdicts in chat (needs_input keeps frame, loses duplicated citation UI) (260817-i36)', () => {
    const needsInputVerdict = {
      status: 'needs_input',
      confidence: 0.65,
      citations: [
        { chunk_id: 'c-002', span: '商务礼品赠送限额', tier: '权威' },
      ],
      review_required: true,
      compliance_status: 'not_applicable',
    };
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：needs_input（置信度 65%）。' },
        { type: 'data-policy', id: 'p1', data: needsInputVerdict },
      ],
    };

    render(<MessageBubble message={message} />);
    // needs_input is NOT a clean factual answer — the card frame and
    // the review-required badge stay visible…
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.getByTestId('review-required-badge')).toBeInTheDocument();
    // …but the large StatusBadge + CitationCluster + citation count are
    // chat-stream noise and are stripped from the CARD (260817-i36 ②).
    const card = screen.getByTestId('verdict-card');
    expect(within(card).queryByTestId('status-badge')).toBeNull();
    expect(screen.queryByTestId('citation-cluster')).toBeNull();
    expect(screen.queryByText(/条引用/)).toBeNull();
    // The status survives as the small corner badge on the conclusion
    // text (260817-i36 ①) — the bubble's single status-badge node.
    const cornerBadge = screen.getByTestId('status-badge');
    expect(cornerBadge).toHaveAttribute('data-status', 'needs_input');
  });

  // --- 260817-i36 Task 3 (②): clickable source-document chips ---

  it('clicking a source-document chip dispatches setCenterView({kind:"chunk-source", ...}) (260817-i36)', () => {
    useAppStore.setState({ centerView: { kind: 'empty' } });
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-document',
          sourceId: 'c-001',
          mediaType: 'text/markdown',
          title: 'authority/travel_expense.md · chunk c-001',
        },
      ],
    };

    render(<MessageBubble message={message} />);
    const chip = screen.getByTestId('source-document');
    expect(chip).toHaveAttribute('data-chunk-id', 'c-001');
    fireEvent.click(chip);
    const centerView = useAppStore.getState().centerView;
    expect(centerView.kind).toBe('chunk-source');
    if (centerView.kind === 'chunk-source') {
      expect(centerView.chunkId).toBe('c-001');
      // 26-08: the dispatch tier is the REGISTRY SLUG derived from the
      // title's directory prefix; the display label is the projection's job.
      expect(centerView.tier).toBe('authority');
      expect(centerView.version).toBe('v3.2');
      expect(centerView.effectiveDate).toBe('2026-01-15');
    }
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

  // --- 260817-i36 Task 4 (①): verdict status as a corner badge on the conclusion text ---

  it('renders the verdict status as a small corner badge attached to the conclusion text (260817-i36 ①)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：由部门负责人审批。' },
        { type: 'data-policy', id: 'p1', data: factualVerdictData },
      ],
    };

    render(<MessageBubble message={message} />);
    const badge = screen.getByTestId('status-badge');
    expect(badge).toHaveAttribute('data-status', 'ok');
    expect(badge).toHaveTextContent('已给出');
    // The badge is attached to the conclusion text's relative wrapper,
    // not floating elsewhere in the bubble.
    const textEl = screen.getByText('结论：由部门负责人审批。');
    const wrapper = textEl.closest('.relative');
    expect(wrapper).not.toBeNull();
    expect(wrapper!.contains(badge)).toBe(true);
    // The ok factual card is omitted (Task 3 ②), so this corner badge
    // is the bubble's ONLY status-badge node.
    expect(screen.getAllByTestId('status-badge')).toHaveLength(1);
  });

  it('renders NO corner badge when the message carries no data-policy verdict (260817-i36 ①)', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [{ type: 'text', text: '流式中途的草稿文本。' }],
    };

    render(<MessageBubble message={message} />);
    expect(screen.getByText('流式中途的草稿文本。')).toBeInTheDocument();
    expect(screen.queryByTestId('status-badge')).toBeNull();
  });

  // --- Plan 29-07 Task 2: FeedbackRow 接真 (data-turn part + new props) ---

  it('derives turnId/votedVerdict from the data-turn part — history echo locks the row (29-07)', () => {
    const message: UIMessage = {
      id: 'm9',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：由部门负责人审批。' },
        { type: 'data-turn', data: { turnId: 41, votedVerdict: 'helpful' } },
      ],
    };

    render(<MessageBubble message={message} />);
    const row = screen.getByTestId('feedback-row');
    expect(row).toHaveAttribute('data-turn-id', '41');
    expect(row).toHaveAttribute('data-message-id', 'm9');
    expect(row).toHaveAttribute('data-voted', 'helpful');
    // votedVerdict (server truth echoed by the history endpoint) renders
    // the locked state without any request.
    expect(screen.getByTestId('feedback-helpful')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('feedback-unhelpful')).toBeDisabled();
  });

  it('renders the honest disabled row when no data-turn part exists yet (mid-stream) (29-07)', () => {
    const message: UIMessage = {
      id: 'm9',
      role: 'assistant',
      parts: [{ type: 'text', text: '流式草稿。' }],
    };

    render(<MessageBubble message={message} />);
    const row = screen.getByTestId('feedback-row');
    expect(row).toHaveAttribute('data-turn-id', '');
    const helpful = screen.getByTestId('feedback-helpful');
    expect(helpful).toBeDisabled();
    expect(helpful).toHaveAttribute('title', '该消息缺少服务端轮次标识，暂时无法反馈');
  });

  it('feeds FeedbackRow from the store session + verdict citations for the ticket form (29-07)', () => {
    useAppStore.setState({ activeSessionId: 'sess-abc12345678' });
    const message: UIMessage = {
      id: 'm9',
      role: 'assistant',
      parts: [
        { type: 'text', text: '结论：由部门负责人审批。' },
        { type: 'data-policy', id: 'p1', data: factualVerdictData },
        { type: 'data-turn', data: { turnId: 41 } },
      ],
    };

    render(<MessageBubble message={message} />);
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    // messageText = the aggregated chat text parts (260817-hck chatText).
    const textarea = screen.getByTestId('feedback-ticket-description') as HTMLTextAreaElement;
    expect(textarea.value).toBe('结论：由部门负责人审批。');
    const refs = screen.getByTestId('feedback-ticket-refs');
    expect(refs).toHaveTextContent('会话 12345678');
    expect(refs).toHaveTextContent('轮次 #41');
    // chunkIds derive from verdict.citations[].chunk_id (Go snake_case).
    expect(refs).toHaveTextContent('c-001');
  });
});
