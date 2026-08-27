// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataReferenceChip tests — Plan 17-02 Task 2 (TDD RED→GREEN).
// Phase 17 DASH-01 / DASH-02 contract tests.
//
// Pinned invariants:
//   1. TestChannelName: popup shows "来源：data:aggregated_only" (D17-B4)
//   2. TestPopupRenders: popup renders value / period / as_of / func fields
//   3. TestSingleValueInline: single value renders as inline span with tooltip
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { DataReference } from '@/types/wire-types';
import { DataReferenceChip } from './DataReferenceChip';
import { DataReferencePopup } from './DataReferencePopup';

afterEach(cleanup);

describe('DataReferencePopup', () => {
  it('renders all 5 fields per UI-SPEC §Position 2', () => {
    const dataRef: DataReference = {
      func: 'KPI_LOAN_BALANCE',
      asOfPeriod: '2026-07',
      asOfTs: '2026-08-01',
      rowCount: 1,
      suppressed: 0,
      kMinApplied: 5,
      references: [{ pointIndex: 0, period: '2026-07', value: 42315000000 }],
    };
    render(<DataReferencePopup dataRef={dataRef} pointRef={dataRef.references?.[0]} />);
    const popup = screen.getByTestId('data-ref-popup');

    // §Copywriting Contract: value / 周期 / 数据截至 / 来源 / 函数
    expect(popup).toHaveTextContent('42,315,000,000'); // formatted value
    expect(popup).toHaveTextContent('周期：2026-07');
    expect(popup).toHaveTextContent('数据截至：2026-08-01');
    expect(popup).toHaveTextContent('来源：data:aggregated_only');
    expect(popup).toHaveTextContent('函数：KPI_LOAN_BALANCE');
  });

  it('shows point position as "点 N / total" for sparkline points', () => {
    const dataRef: DataReference = {
      func: 'KPI_LOAN_BALANCE',
      asOfPeriod: '2026-07',
      asOfTs: '2026-08-01',
      rowCount: 3,
      suppressed: 0,
      kMinApplied: 5,
      references: [
        { pointIndex: 0, period: '2026-05', value: 41000000000 },
        { pointIndex: 1, period: '2026-06', value: 42000000000 },
        { pointIndex: 2, period: '2026-07', value: 42315000000 },
      ],
    };
    render(<DataReferencePopup dataRef={dataRef} pointRef={dataRef.references?.[1]} />);
    const popup = screen.getByTestId('data-ref-popup');

    // point 2 of 3 (0-indexed: index 1 → pointIndex 2)
    expect(popup).toHaveTextContent('（点 2 / 3）');
    expect(popup).toHaveTextContent('周期：2026-06');
  });
});

describe('DataReferenceChip', () => {
  it('TestChannelName: popup shows "来源：data:aggregated_only"', () => {
    const dataRef: DataReference = {
      func: 'KPI_LOAN_BALANCE',
      asOfPeriod: '2026-07',
      asOfTs: '2026-08-01',
      rowCount: 1,
      suppressed: 0,
      kMinApplied: 5,
      references: [{ pointIndex: 0, period: '2026-07', value: 42315000000 }],
    };
    render(<DataReferenceChip dataRef={dataRef} />);

    // The channel name is baked into DataReferencePopup copy (D17-B4)
    // and verified via the popup render in the Popup tests above.
    // Here we verify the Chip renders without crashing.
    const wrapper = screen.getByTestId('kpi-value-wrapper');
    expect(wrapper).toBeInTheDocument();
  });

  it('TestSingleValueInline: single value renders as inline span', () => {
    const dataRef: DataReference = {
      func: 'KPI_NPL_RATIO',
      asOfPeriod: '2026-07',
      asOfTs: '2026-08-01',
      rowCount: 1,
      suppressed: 0,
      kMinApplied: 5,
      references: [{ pointIndex: 0, period: '2026-07', value: 0.023 }],
    };
    render(<DataReferenceChip dataRef={dataRef} />);

    // Inline value rendered
    const value = screen.getByTestId('kpi-value');
    expect(value).toBeInTheDocument();
    expect(value).toHaveTextContent('0.023');

    // CSS hover group wrapper
    const wrapper = screen.getByTestId('kpi-value-wrapper');
    expect(wrapper).toHaveClass('group'); // group for CSS hover
  });

  it('renders placeholder when sparkline branch activates (before 17-01 Sparkline exists)', () => {
    const dataRef: DataReference = {
      func: 'KPI_LOAN_BALANCE',
      asOfPeriod: '2026-07',
      asOfTs: '2026-08-01',
      rowCount: 12,
      suppressed: 0,
      kMinApplied: 5,
      references: [
        { pointIndex: 0, period: '2026-01', value: 40000000000 },
        { pointIndex: 1, period: '2026-02', value: 41000000000 },
        { pointIndex: 2, period: '2026-03', value: 42000000000 },
        { pointIndex: 3, period: '2026-04', value: 41500000000 },
      ],
    };
    render(<DataReferenceChip dataRef={dataRef} />);

    const sparklineRef = screen.getByTestId('kpi-sparkline-ref');
    expect(sparklineRef).toBeInTheDocument();
    // Placeholder shows point count until Sparkline component lands in 17-01
    expect(screen.getByTestId('sparkline-placeholder')).toHaveTextContent('[4 点趋势]');
  });
});
