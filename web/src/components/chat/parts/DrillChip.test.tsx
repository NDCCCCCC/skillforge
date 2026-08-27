// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

import { render, screen, fireEvent } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { DrillChip } from './DrillChip';
import { useAppStore } from '../../../store/app-store';

afterEach(() => {
  // Reset pendingDraft between tests
  useAppStore.setState({ pendingDraft: '' });
});

describe('DrillChip', () => {
  it('TestBadgeRenders', () => {
    useAppStore.setState({ pendingDraft: '' });
    render(
      <DrillChip kpiName="KPI_LOAN_BALANCE" periodsTemplate="最近12月" />,
    );
    expect(screen.getByTestId('drill-chip')).toBeInTheDocument();
    expect(screen.getByText('看月度趋势')).toBeInTheDocument();
  });

  it('TestPrefill', () => {
    const setPendingDraft = vi.fn();
    // Seed the store's setPendingDraft with our mock
    useAppStore.setState({ pendingDraft: '', setPendingDraft });
    render(
      <DrillChip kpiName="KPI_LOAN_BALANCE" periodsTemplate="最近12月" />,
    );
    fireEvent.click(screen.getByTestId('drill-chip'));
    expect(setPendingDraft).toHaveBeenCalledOnce();
    expect(setPendingDraft).toHaveBeenCalledWith(
      '请调 KPI_LOAN_BALANCE periods=最近12月',
    );
  });

  it('TestClickCallsSetPendingDraft', () => {
    const setPendingDraft = vi.fn();
    useAppStore.setState({ pendingDraft: '', setPendingDraft });
    render(
      <DrillChip
        kpiName="KPI_NPL_RATIO"
        periodsTemplate="2024-01,2024-02,2024-03"
        label="看同比"
      />,
    );
    fireEvent.click(screen.getByText('看同比'));
    expect(setPendingDraft).toHaveBeenCalledOnce();
    expect(setPendingDraft).toHaveBeenCalledWith(
      '请调 KPI_NPL_RATIO periods=2024-01,2024-02,2024-03',
    );
  });
});
