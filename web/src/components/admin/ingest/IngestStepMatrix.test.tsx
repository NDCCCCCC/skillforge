// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- the grid pins: six tones, role=img labels,
// legend verbatim, counter, and the NON-interactive rule.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import IngestStepMatrix from './IngestStepMatrix';
import { STEP_LABEL, STEP_SEQUENCE } from './types';
import type { MatrixCellData } from './IngestStepMatrix';

afterEach(cleanup);

const labels = STEP_SEQUENCE.map((k) => STEP_LABEL[k] ?? k);

const SIX_CELLS: MatrixCellData[] = [
  { label: '完成', tone: 'green', pulse: false, durationLabel: '3.2s' },
  { label: '进行中', tone: 'amber', pulse: true, durationLabel: '—' },
  { label: '失败', tone: 'red', pulse: false, durationLabel: '1.0s' },
  { label: '等待', tone: 'gray', pulse: false, durationLabel: '—' },
  { label: '已跳过', tone: 'gray', pulse: false, dashed: true },
  { label: '待重跑', tone: 'amber', pulse: false },
];

const DOCS = [{ fileName: 'authority/policy.md', cells: SIX_CELLS }];

describe('IngestStepMatrix (55-02)', () => {
  it('renders the eight full zh column headers (no abbreviation)', () => {
    render(<IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={0} totalDocs={1} />);
    const table = screen.getByTestId('ingest-step-matrix');
    const headers = Array.from(table.querySelectorAll('th'));
    // 8 step columns + the lead doc-name column.
    expect(headers.map((h) => h.textContent)).toEqual([...labels]);
  });

  it('renders the six cell tones with pulse on running and dashed on skipped only', () => {
    render(<IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={0} totalDocs={1} />);
    const cells = screen.getAllByRole('img');
    expect(cells[0].className).toContain('bg-status-compliant');
    expect(cells[1].className).toContain('bg-status-needs-info');
    expect(cells[1].className).toContain('animate-pulse');
    expect(cells[2].className).toContain('bg-status-non-compliant');
    expect(cells[3].className).toContain('bg-status-unanswered');
    expect(cells[4].className).toContain('border-dashed');
    expect(cells[5].className).toContain('bg-status-needs-info');
    expect(cells[5].className).not.toContain('animate-pulse');
  });

  it('labels each cell with doc · step · state · duration, omitting — durations', () => {
    render(<IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={0} totalDocs={1} />);
    const cells = screen.getAllByRole('img');
    expect(cells[0]).toHaveAttribute(
      'aria-label',
      '文档 authority/policy.md · 解析 · 完成 · 3.2s',
    );
    expect(cells[1].getAttribute('aria-label')).toBe('文档 authority/policy.md · 分块 · 进行中');
  });

  it('renders the six-item legend verbatim and hides it from assistive tech', () => {
    render(<IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={0} totalDocs={1} />);
    const legend = screen.getByTestId('ingest-matrix-legend');
    expect(legend).toHaveAttribute('aria-hidden', 'true');
    for (const label of ['完成', '进行中', '失败', '等待', '已跳过', '待重跑']) {
      expect(legend.textContent).toContain(label);
    }
  });

  it('renders the completion counter in mono', () => {
    render(<IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={1} totalDocs={3} />);
    const counter = screen.getByTestId('ingest-matrix-counter');
    expect(counter).toHaveTextContent('1/3 篇完成');
    expect(counter.className).toContain('font-mono');
  });

  it('has ZERO buttons and ZERO click handlers (deliberate non-interactive grid)', () => {
    const { container } = render(
      <IngestStepMatrix stepLabels={labels} docs={DOCS} doneDocs={0} totalDocs={1} />,
    );
    expect(container.querySelectorAll('button')).toHaveLength(0);
    expect(container.querySelectorAll('[onclick]')).toHaveLength(0);
  });
});
