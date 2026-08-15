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
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import VerdictCard from '../components/center/VerdictCard';
import type { Verdict } from '../types/verdict';

afterEach(cleanup);

const compliant: Verdict = {
  status: '合规',
  confidence: 0.93,
  citations: [{ chunk_id: 'c-001', span: '经济舱机票每日 800 元', tier: '权威' }],
  review_required: false,
};

const needsInfo: Verdict = {
  status: '需补充',
  confidence: 0.65,
  citations: [{ chunk_id: 'c-002', span: '商务礼品赠送限额', tier: '权威' }],
  review_required: true,
};

const nonCompliant: Verdict = {
  status: '不合规',
  confidence: 0.88,
  citations: [{ chunk_id: 'c-003', span: '竞业限制不得超过 2 年', tier: '权威' }],
  review_required: true,
};

describe('VerdictCard', () => {
  it('renders data-testid="verdict-card" and data-verdict-status="合规"', () => {
    render(<VerdictCard verdict={compliant} />);
    const card = screen.getByTestId('verdict-card');
    expect(card).toBeInTheDocument();
    expect(card).toHaveAttribute('data-verdict-status', '合规');
  });

  it('renders data-verdict-status="不合规" for a non-compliant verdict', () => {
    render(<VerdictCard verdict={nonCompliant} />);
    expect(screen.getByTestId('verdict-card')).toHaveAttribute('data-verdict-status', '不合规');
  });

  it('renders data-verdict-status="需补充" for a needs-info verdict', () => {
    render(<VerdictCard verdict={needsInfo} />);
    expect(screen.getByTestId('verdict-card')).toHaveAttribute('data-verdict-status', '需补充');
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
});
