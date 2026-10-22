// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 + Plan 260815-cos Task 3 — VerdictCard tests.
//
// Pins D-06 invariants:
//   - data-testid="verdict-card", data-verdict-status per status
//   - data-review-required boolean attribute
//   - data-testid="review-required-badge" only when review_required=true
//   - data-testid="citation-cluster" with one superscript per citation
//     (Plan 260815-cos: chip → superscript migration)
//
// Plan 260815-cos / locked CONTEXT.md decision D (confidence bar):
//   - The 置信度 N% text is REMOVED (assert absence).
//   - The data-testid="confidence-bar" DOM node is RETAINED but
//     visually hidden (display:none via Tailwind `hidden`).
//     Tests assert both presence (testid survives) and absence of
//     the visual rendering (no 置信度 text).
//
// 260817-vsm: Verdict.status is now platform-neutral ({ok, needs_input,
// refused, error}); the Compliance Skill's verdict rides in
// Verdict.compliance_status ({compliant, non_compliant, not_applicable}).
// Tests assert the platform Status on data-verdict-status and the
// Compliance-Skill code on data-compliance-status.
//
// 260817-hck: verdict-answer 在 chatText 已含结论时隐藏 — when the caller
// (MessageBubble via DataPolicyPart) passes the concatenated chat text and
// it verbatim contains verdict.answer (backend conclusionText embeds the
// answer verbatim, chat.go:801-809), the in-card verdict-answer block is
// suppressed so the conclusion renders only once.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import VerdictCard from '../components/center/VerdictCard';
import { useAppStore } from '../store/app-store';
import type { Verdict } from '../types/verdict';

afterEach(cleanup);

const compliant: Verdict = {
  status: 'ok',
  confidence: 0.93,
  citations: [{ chunk_id: 'c-001', span: '经济舱机票每日 800 元', tier: '权威' }],
  review_required: false,
  compliance_status: 'compliant',
};

const needsInfo: Verdict = {
  status: 'needs_input',
  confidence: 0.65,
  citations: [{ chunk_id: 'c-002', span: '商务礼品赠送限额', tier: '权威' }],
  review_required: true,
  compliance_status: 'not_applicable',
};

const nonCompliant: Verdict = {
  status: 'ok',
  confidence: 0.88,
  citations: [{ chunk_id: 'c-003', span: '竞业限制不得超过 2 年', tier: '权威' }],
  review_required: true,
  compliance_status: 'non_compliant',
};

const factualQuery: Verdict = {
  status: 'ok',
  confidence: 0.97,
  citations: [
    { chunk_id: 'c-001', span: '采购合同金额 50 万元以下由部门负责人审批', tier: '权威' },
  ],
  review_required: false,
  answer: '由部门负责人审批',
  compliance_status: 'not_applicable',
};

describe('VerdictCard', () => {
  it('renders data-testid="verdict-card" and data-verdict-status="ok"', () => {
    render(<VerdictCard verdict={compliant} />);
    const card = screen.getByTestId('verdict-card');
    expect(card).toBeInTheDocument();
    expect(card).toHaveAttribute('data-verdict-status', 'ok');
  });

  it('renders data-compliance-status="compliant" for a compliant verdict', () => {
    render(<VerdictCard verdict={compliant} />);
    expect(screen.getByTestId('verdict-card')).toHaveAttribute(
      'data-compliance-status',
      'compliant',
    );
  });

  it('renders data-compliance-status="non_compliant" for a non-compliant verdict', () => {
    render(<VerdictCard verdict={nonCompliant} />);
    expect(screen.getByTestId('verdict-card')).toHaveAttribute(
      'data-compliance-status',
      'non_compliant',
    );
  });

  it('renders data-verdict-status="needs_input" for a needs-info verdict', () => {
    render(<VerdictCard verdict={needsInfo} />);
    expect(screen.getByTestId('verdict-card')).toHaveAttribute(
      'data-verdict-status',
      'needs_input',
    );
  });

  it('renders the ComplianceStatusBadge only when compliance_status is a real review verdict', () => {
    const { rerender } = render(<VerdictCard verdict={compliant} />);
    expect(screen.getByTestId('compliance-status-badge')).toBeInTheDocument();

    // not_applicable → badge is hidden (factual query, no compliance verdict).
    rerender(<VerdictCard verdict={needsInfo} />);
    expect(screen.queryByTestId('compliance-status-badge')).toBeNull();
  });

  it('renders the factual-answer text in verdict-answer when answer is populated', () => {
    render(<VerdictCard verdict={factualQuery} />);
    const answer = screen.getByTestId('verdict-answer');
    expect(answer).toHaveTextContent('由部门负责人审批');
    // The compliance badge is suppressed for factual queries (not_applicable).
    expect(screen.queryByTestId('compliance-status-badge')).toBeNull();
  });

  it('renders data-testid="review-required-badge" only when review_required=true', () => {
    const { rerender } = render(<VerdictCard verdict={compliant} />);
    expect(screen.queryByTestId('review-required-badge')).toBeNull();

    rerender(<VerdictCard verdict={needsInfo} />);
    expect(screen.getByTestId('review-required-badge')).toBeInTheDocument();
  });

  it('does NOT render confidence text (Plan 260815-cos decision D)', () => {
    render(<VerdictCard verdict={compliant} />);
    // The 「置信度 93%」 text is gone — only data-confidence attribute
    // on the hidden DOM node survives (next test).
    expect(screen.getByTestId('verdict-card')).not.toHaveTextContent('置信度');
    expect(screen.queryByText(/置信度 \d+%/)).toBeNull();
  });

  it('retains data-testid="confidence-bar" hidden node with data-confidence', () => {
    render(<VerdictCard verdict={compliant} />);
    const bar = screen.getByTestId('confidence-bar');
    expect(bar).toBeInTheDocument();
    expect(bar).toHaveAttribute('data-confidence', '93');
    // Tailwind's `hidden` utility = display:none. jsdom does not
    // resolve the Tailwind class chain (no PostCSS at test time), so
    // we assert the class itself is present; production browsers +
    // vite build apply display:none through Tailwind's CSS pipeline
    // (verified by `npx vite build` smoke).
    expect(bar).toHaveClass('hidden');
    // Also confirm the bar is NOT in the visible verdict text content
    // (the parent verdict-card does not contain the percentage text).
    const card = screen.getByTestId('verdict-card');
    expect(card).not.toHaveTextContent('93%');
  });

  it('renders data-testid="citation-cluster" with one superscript per citation', () => {
    const multi: Verdict = {
      ...compliant,
      citations: [
        { chunk_id: 'c-001', span: '条款一', tier: '权威' },
        { chunk_id: 'c-002', span: '条款二', tier: '权威' },
        { chunk_id: 'c-003', span: '条款三', tier: '权威' },
      ],
    };
    render(<VerdictCard verdict={multi} />);
    expect(screen.getByTestId('citation-cluster')).toBeInTheDocument();
    expect(screen.getAllByTestId('citation-superscript')).toHaveLength(3);
  });

  // --- 260817-hck: chatText dedupe of verdict-answer ---

  it('hides verdict-answer when chatText already contains the answer verbatim (260817-hck)', () => {
    render(
      <VerdictCard
        verdict={factualQuery}
        chatText="结论：由部门负责人审批。依据 权威 tier 条款：「采购合同金额 50 万元以下由部门负责人审批」。"
      />,
    );
    // The duplicated answer block is gone — the chat body already shows it.
    expect(screen.queryByTestId('verdict-answer')).toBeNull();
    // The card frame survives in chat context…
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    // …but 260817-i36 (②) strips the duplicated citation UI + the large
    // StatusBadge from the chat stream: the doc chips under the
    // conclusion are the single citation UI, and the corner badge on
    // the conclusion text (Task 4) carries the status.
    expect(screen.queryByTestId('citation-cluster')).toBeNull();
    expect(screen.queryByTestId('status-badge')).toBeNull();
    expect(screen.queryByText('1 条引用')).toBeNull();
  });

  // --- 260817-i36 Task 3 (②): slim chat-context card vs full CenterPanel card ---

  it('omits StatusBadge / citation count / CitationCluster when chatText is provided (chat context, 260817-i36)', () => {
    render(<VerdictCard verdict={compliant} chatText="结论：符合规定。" />);
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.queryByTestId('status-badge')).toBeNull();
    expect(screen.queryByTestId('citation-cluster')).toBeNull();
    expect(screen.queryByText(/条引用/)).toBeNull();
    // Open Design B5 — the chat-stream card is the compact inline
    // stripe; the ComplianceStatusBadge lives in the detail panel the
    // stripe opens (one click away), not in the chat stream.
    expect(screen.queryByTestId('compliance-status-badge')).toBeNull();
  });

  it('keeps the review-required badge in chat context (260817-i36)', () => {
    render(<VerdictCard verdict={nonCompliant} chatText="结论：不合规。" />);
    expect(screen.getByTestId('review-required-badge')).toBeInTheDocument();
    expect(screen.queryByTestId('compliance-status-badge')).toBeNull();
    expect(screen.queryByTestId('status-badge')).toBeNull();
    expect(screen.queryByTestId('citation-cluster')).toBeNull();
  });

  it('renders the FULL card when chatText is not provided (CenterPanel path, 260817-i36)', () => {
    // CenterPanel.tsx renders <VerdictCard verdict={verdict} /> with no
    // chat context — StatusBadge + citation count + CitationCluster must
    // all stay (the center 结论 tab is the structured citation surface).
    render(<VerdictCard verdict={compliant} />);
    expect(screen.getByTestId('status-badge')).toBeInTheDocument();
    expect(screen.getByTestId('citation-cluster')).toBeInTheDocument();
    expect(screen.getByText('1 条引用')).toBeInTheDocument();
  });

  it('keeps verdict-answer when chatText does NOT contain the answer (260817-hck)', () => {
    render(<VerdictCard verdict={factualQuery} chatText="一些不包含答案的正文" />);
    const answer = screen.getByTestId('verdict-answer');
    expect(answer).toBeInTheDocument();
    expect(answer).toHaveTextContent('由部门负责人审批');
  });

  it('keeps verdict-answer when chatText is not provided (CenterPanel path, 260817-hck)', () => {
    // CenterPanel.tsx:191 renders <VerdictCard verdict={verdict} /> with no
    // chat context — behavior must be unchanged. Assert explicitly with a
    // compliance-status verdict that the answer block still renders.
    const withCompliance: Verdict = { ...factualQuery, compliance_status: 'compliant' };
    render(<VerdictCard verdict={withCompliance} />);
    const answer = screen.getByTestId('verdict-answer');
    expect(answer).toBeInTheDocument();
    expect(answer).toHaveTextContent('由部门负责人审批');
    expect(screen.getByTestId('compliance-status-badge')).toBeInTheDocument();
  });

  it('keeps verdict-answer when chatText is an empty string (260817-hck)', () => {
    render(<VerdictCard verdict={factualQuery} chatText="" />);
    expect(screen.getByTestId('verdict-answer')).toBeInTheDocument();
  });

  it('chat-stream inline stripe opens the detail panel verdict tab on click (B5)', () => {
    useAppStore.setState({ centerView: { kind: 'empty' }, detailPanelOpen: false });
    render(<VerdictCard verdict={nonCompliant} chatText="结论：不合规。" />);
    fireEvent.click(screen.getByTestId('verdict-card'));
    expect(useAppStore.getState().centerView).toEqual({ kind: 'verdict', verdictId: 'chat-latest' });
    expect(useAppStore.getState().detailPanelOpen).toBe(true);
  });

  // --- 49-02 (D-08③): stub-mode badge ---

  const stubVerdict: Verdict = {
    ...factualQuery,
    confidence: 0.5,
    review_required: true,
    mode: 'stub',
  };

  it('renders the stub-mode badge on both card surfaces when verdict.mode="stub" (49-02)', () => {
    // Full CenterPanel card.
    const { rerender } = render(<VerdictCard verdict={stubVerdict} />);
    expect(screen.getByTestId('stub-mode-badge')).toHaveTextContent('stub');
    // Chat-stream inline stripe.
    rerender(<VerdictCard verdict={stubVerdict} chatText="一些不包含答案的正文" />);
    expect(screen.getByTestId('stub-mode-badge')).toHaveTextContent('stub');
  });

  it('renders NO stub-mode badge for a real-model verdict (mode absent, 49-02)', () => {
    render(<VerdictCard verdict={compliant} />);
    expect(screen.queryByTestId('stub-mode-badge')).toBeNull();
  });
});
