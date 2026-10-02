// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — vitest cases for HistoryRewriteHint.
//
//   - TestHistoryRewriteHint_Renders — rewrite="差旅住宿费超过500元合规吗";
//     asserts the DOM contains the string and the data-testid.
//   - TestHistoryRewriteHint_EmptyRewrite — rewrite=""; asserts the
//     caption is hidden.

import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import HistoryRewriteHint from './HistoryRewriteHint';

describe('HistoryRewriteHint', () => {
  it('TestHistoryRewriteHint_Renders — renders the rewrite caption with data-testid', () => {
    render(<HistoryRewriteHint rewrite="差旅住宿费超过500元合规吗" />);
    const hint = screen.getByTestId('history-rewrite-hint');
    expect(hint).toBeInTheDocument();
    expect(hint).toHaveAttribute('data-rewrite', '差旅住宿费超过500元合规吗');
    expect(hint).toHaveTextContent('差旅住宿费超过500元合规吗');
  });

  it('TestHistoryRewriteHint_EmptyRewrite — empty rewrite hides the caption', () => {
    const { container } = render(<HistoryRewriteHint rewrite="" />);
    // The caption is hidden entirely — no element with the testid.
    expect(container.querySelector('[data-testid="history-rewrite-hint"]')).toBeNull();
    expect(screen.queryByTestId('history-rewrite-hint')).toBeNull();
  });
});