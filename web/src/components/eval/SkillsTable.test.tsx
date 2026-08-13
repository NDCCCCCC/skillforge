// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 T2 — SkillsTable tests.
//
// Validates the SC-1 surface: one row per Skill, all five metrics
// visible, and the threshold badge rendered as ✅ or ❌ depending on
// faithfulness vs the default 0.9 threshold.

import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import SkillsTable from './SkillsTable';
import { useAppStore } from '../../store/app-store';
import SAMPLE_INDEX from './__fixtures__/index.json';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  useAppStore.setState({ indexReports: [] });
});

describe('SkillsTable', () => {
  it('renders one row per Skill with all 5 metrics + threshold badge', async () => {
    useAppStore.setState({ indexReports: SAMPLE_INDEX.runs });
    render(<SkillsTable />);

    const rows = await screen.findAllByTestId('eval-skills-row');
    expect(rows).toHaveLength(1);

    const row = rows[0];
    expect(row).toHaveTextContent('compliance');
    // SkillsTable picks the latest run per Skill (deepseek-chat run at 10-15-07Z).
    expect(row).toHaveTextContent('0.91'); // faithfulness
    expect(row).toHaveTextContent('0.84'); // context_precision (citation precision)
    expect(row).toHaveTextContent('0.87'); // answer_relevance
    expect(row).toHaveTextContent('0.78'); // context_recall
    expect(row).toHaveTextContent('0.96'); // refusal_accuracy
  });

  it('shows ✅ when faithfulness >= threshold', async () => {
    useAppStore.setState({ indexReports: SAMPLE_INDEX.runs });
    render(<SkillsTable />);

    const rows = await screen.findAllByTestId('eval-skills-row');
    expect(rows[0]).toHaveAttribute('data-pass', 'true');
    expect(screen.getByTestId('eval-skills-pass')).toBeInTheDocument();
  });

  it('shows ❌ when faithfulness < threshold', async () => {
    useAppStore.setState({
      indexReports: [
        {
          ...SAMPLE_INDEX.runs[0],
          faithfulness: 0.85, // below 0.9
        },
      ],
    });
    render(<SkillsTable />);

    const rows = await screen.findAllByTestId('eval-skills-row');
    expect(rows[0]).toHaveAttribute('data-pass', 'false');
    expect(screen.getByTestId('eval-skills-fail')).toBeInTheDocument();
  });

  it('renders the empty state when there are no runs', () => {
    render(<SkillsTable />);
    expect(screen.getByTestId('eval-skills-empty')).toBeInTheDocument();
  });

  it('picks the latest run per Skill when multiple timestamps exist', async () => {
    useAppStore.setState({ indexReports: SAMPLE_INDEX.runs });
    render(<SkillsTable />);

    const rows = await screen.findAllByTestId('eval-skills-row');
    expect(rows).toHaveLength(1);
    // The latest compliance run in the fixture is the deepseek-chat run at 0.91.
    expect(rows[0]).toHaveTextContent('0.91');
  });
});
