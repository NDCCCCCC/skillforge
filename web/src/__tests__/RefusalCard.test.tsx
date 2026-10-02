// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 — RefusalCard tests.
//
// Pins D-07 invariants:
//   - data-testid="refusal-card" + data-refusal-reason
//   - out-of-scope: 🚫 + 超出本 Skill 范围 heading + blue-gray border
//   - no_authorized_source: ❓ + 范围内无依据 heading + gray border
//   - suggestion chip only when can_user_rephrase=true
//   - NEVER renders verdict-card or confidence-bar (negative assertions)
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import RefusalCard from '../components/center/RefusalCard';
import type { Refusal } from '../types/verdict';

afterEach(cleanup);

const outOfScope: Refusal = {
  reason: 'out_of_scope',
  can_user_rephrase: false,
  suggested_alternative_query: '',
};

const noSource: Refusal = {
  reason: 'no_authorized_source',
  can_user_rephrase: true,
  suggested_alternative_query: '请尝试添加权威制度文件编号，例如《差旅管理办法》第 12 条',
};

const lowScore: Refusal = {
  reason: 'low_retrieval_score',
  can_user_rephrase: false,
};

describe('RefusalCard', () => {
  it('renders data-testid="refusal-card" and data-refusal-reason="out_of_scope"', () => {
    render(<RefusalCard refusal={outOfScope} />);
    const card = screen.getByTestId('refusal-card');
    expect(card).toBeInTheDocument();
    expect(card).toHaveAttribute('data-refusal-reason', 'out_of_scope');
  });

  it('renders data-refusal-reason="no_authorized_source" for I-don\'t-know', () => {
    render(<RefusalCard refusal={noSource} />);
    expect(screen.getByTestId('refusal-card')).toHaveAttribute('data-refusal-reason', 'no_authorized_source');
  });

  it('out-of-scope heading reads � 超出本 Skill 范围', () => {
    render(<RefusalCard refusal={outOfScope} />);
    const heading = screen.getByRole('heading', { level: 3 });
    expect(heading.textContent).toMatch(/超出本 Skill 范围/);
  });

  it('I-don\'t-know heading reads ❓ 范围内无依据', () => {
    render(<RefusalCard refusal={noSource} />);
    const heading = screen.getByRole('heading', { level: 3 });
    expect(heading.textContent).toMatch(/范围内无依据/);
  });

  it('renders suggestion chip when can_user_rephrase=true and suggestion is non-empty', () => {
    render(<RefusalCard refusal={noSource} />);
    const chip = screen.getByTestId('refusal-suggestion-chip');
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveTextContent('使用此提问：请尝试添加权威制度文件编号');
  });

  it('omits suggestion chip when can_user_rephrase=false', () => {
    render(<RefusalCard refusal={outOfScope} />);
    expect(screen.queryByTestId('refusal-suggestion-chip')).toBeNull();
    expect(screen.getByTestId('refusal-card')).toHaveTextContent('本次无法回答，请联系 Skill 维护者');
  });

  it('renders low_retrieval_score as the I-don\'t-know variant with ❓', () => {
    render(<RefusalCard refusal={lowScore} />);
    const heading = screen.getByRole('heading', { level: 3 });
    expect(heading.textContent).toMatch(/范围内检索置信度不足/);
  });

  it('RefusalCard NEVER renders verdict-card or confidence-bar', () => {
    render(<RefusalCard refusal={outOfScope} />);
    expect(screen.queryByTestId('verdict-card')).toBeNull();
    expect(screen.queryByTestId('confidence-bar')).toBeNull();
  });
});
