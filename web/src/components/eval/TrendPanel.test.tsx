// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 T2 — TrendPanel tests.
//
// Validates the D6-16 surface: recharts ResponsiveContainer + LineChart
// render, frozen baseline ReferenceLine is present, and a >5% drift
// alert appears when the latest faithfulness differs from baseline by
// more than the default threshold.

import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import TrendPanel from './TrendPanel';
import SAMPLE_INDEX from './__fixtures__/index.json';

afterEach(() => {
  cleanup();
});

describe('TrendPanel', () => {
  it('renders a recharts chart with runs', () => {
    render(<TrendPanel runs={SAMPLE_INDEX.runs} />);
    expect(screen.getByTestId('eval-trend-chart')).toBeInTheDocument();
  });

  it('renders a dashed frozen baseline ReferenceLine', () => {
    render(<TrendPanel runs={SAMPLE_INDEX.runs} baseline={0.9} />);
    expect(screen.getByTestId('eval-trend-baseline')).toHaveTextContent('0.90');
  });

  it('shows the drift flag when latest faithfulness is >5% from baseline', () => {
    // Latest faithfulness in fixture is 0.91; baseline 0.85 = 6% delta.
    render(<TrendPanel runs={SAMPLE_INDEX.runs} baseline={0.85} />);
    expect(screen.getByTestId('trend-drift-flag')).toBeInTheDocument();
    expect(screen.getByTestId('trend-drift-flag')).toHaveTextContent('drifted');
  });

  it('does not show the drift flag when within 5%', () => {
    // Latest faithfulness in fixture is 0.91; baseline 0.88 = 3% delta.
    render(<TrendPanel runs={SAMPLE_INDEX.runs} baseline={0.88} />);
    expect(screen.queryByTestId('trend-drift-flag')).toBeNull();
  });

  it('renders the empty state when there are no runs', () => {
    render(<TrendPanel runs={[]} />);
    expect(screen.getByTestId('eval-trend-empty')).toBeInTheDocument();
  });
});
