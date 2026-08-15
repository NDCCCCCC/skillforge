// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DraftStreamChip tests — Plan 260815-cos Task 2 (TDD RED→GREEN).
//
// Pinned invariants:
//  1. status='pending' + draftText → renders 🟡 待校验 chip + the
//     accumulated text (text-delta-draft frames have been buffered).
//  2. status='verified' + draftText → renders ✅ 已验证 chip + the
//     text is retained (the user watched it stream in, continuity
//     matters for the trust transition 🟡 → ✅).
//  3. status='refused' → return null (RefusalCard takes its place).
//  4. Streaming append (draftText grows over re-renders) → the chip
//     always renders the latest accumulated text (progressive UX).
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DraftStreamChip from './DraftStreamChip';

afterEach(cleanup);

describe('DraftStreamChip', () => {
  it('renders 🟡 待校验 chip with the draft text when status=pending', () => {
    render(<DraftStreamChip status="pending" draftText="依据制度..." />);
    const chip = screen.getByTestId('draft-chip');
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAttribute('data-status', 'pending');
    expect(chip).toHaveTextContent('待校验');
    expect(screen.getByTestId('draft-text')).toHaveTextContent('依据制度...');
  });

  it('renders ✅ 已验证 chip and retains the draft text on status=verified', () => {
    render(<DraftStreamChip status="verified" draftText="依据制度第3条" />);
    const chip = screen.getByTestId('draft-chip');
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAttribute('data-status', 'verified');
    expect(chip).toHaveTextContent('已验证');
    // The draft text persists — continuity for the trust transition.
    expect(screen.getByTestId('draft-text')).toHaveTextContent('依据制度第3条');
  });

  it('returns null when status=refused (RefusalCard replaces the chip)', () => {
    const { container } = render(
      <DraftStreamChip status="refused" draftText="依据制度..." />,
    );
    expect(container.firstChild).toBeNull();
    // queryByTestId (NOT getByTestId) — refuses to throw on miss.
    expect(screen.queryByTestId('draft-chip')).toBeNull();
  });

  it('re-renders with the latest accumulated text as draftText grows', () => {
    const { rerender } = render(<DraftStreamChip status="pending" draftText="依" />);
    expect(screen.getByTestId('draft-text')).toHaveTextContent('依');

    rerender(<DraftStreamChip status="pending" draftText="依据" />);
    expect(screen.getByTestId('draft-text')).toHaveTextContent('依据');

    rerender(<DraftStreamChip status="pending" draftText="依据制度" />);
    expect(screen.getByTestId('draft-text')).toHaveTextContent('依据制度');
  });
});