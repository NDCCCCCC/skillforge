// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 Task 2 — VersionBadge tests (SC-8 / CMPL-08).
//
// Pins the badge shape per UI-SPEC §SC-8:
//   - 当前有效 (BadgeCheck green) when effectiveTo is null OR 0 OR
//     in the future.
//   - 已失效 (strikethrough --color-text-dim) when effectiveTo is in
//     the past.
//   - data-status attribute mirrors the rendered state for downstream
//     test queries.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import VersionBadge from '../components/center/VersionBadge';

afterEach(cleanup);

describe('VersionBadge', () => {
  it('renders 当前有效 with data-status="active" when effectiveTo is null (perpetual)', () => {
    render(<VersionBadge effectiveFrom={1736899200} effectiveTo={null} />);
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
    expect(badge).toHaveTextContent('当前有效');
  });

  it('renders 当前有效 when effectiveTo is 0 (sentinel-0 perpetual)', () => {
    render(<VersionBadge effectiveFrom={1736899200} effectiveTo={0} />);
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
    expect(badge).toHaveTextContent('当前有效');
  });

  it('renders 当前有效 when effectiveTo is in the future', () => {
    const futureEpoch = Math.floor(Date.now() / 1000) + 60 * 60 * 24 * 30; // +30d
    render(<VersionBadge effectiveFrom={1736899200} effectiveTo={futureEpoch} />);
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
    expect(badge).toHaveTextContent('当前有效');
  });

  it('renders 已失效 with data-status="superseded" when effectiveTo is in the past', () => {
    const pastEpoch = 1677628800; // 2023-03-01
    render(<VersionBadge effectiveFrom={1672531200} effectiveTo={pastEpoch} />);
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'superseded');
    expect(badge).toHaveTextContent('已失效');
  });

  it('still renders 当前有效 when effectiveFrom is null but effectiveTo is null (perpetual)', () => {
    // The active/superseded decision is based solely on effectiveTo;
    // effectiveFrom=null doesn't disqualify the badge. (The metadata
    // row surfaces a separate 生效日期: 未知 string when this happens.)
    render(<VersionBadge effectiveFrom={null} effectiveTo={null} />);
    const badge = screen.getByTestId('version-badge');
    expect(badge).toHaveAttribute('data-status', 'active');
    expect(badge).toHaveTextContent('当前有效');
  });
});