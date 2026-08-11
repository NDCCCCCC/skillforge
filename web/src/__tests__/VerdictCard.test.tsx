// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 — VerdictCard tests.
//
// Pins D-06 invariants:
//   - data-testid="verdict-card", data-verdict-status per status
//   - data-review-required boolean attribute
//   - data-testid="review-required-badge" only when review_required=true
//   - confidence text reads `置信度 {Math.round(conf*100)}%`
//   - data-testid="citation-cluster" with one chip per citation
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

  it('renders confidence text 置信度 93% when confidence is 0.93', () => {
    render(<VerdictCard verdict={compliant} />);
    expect(screen.getByTestId('verdict-card')).toHaveTextContent('置信度 93%');
  });

  it('renders data-testid="citation-cluster" with one chip per citation', () => {
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
    expect(screen.getAllByTestId('citation-chip')).toHaveLength(3);
  });
});
