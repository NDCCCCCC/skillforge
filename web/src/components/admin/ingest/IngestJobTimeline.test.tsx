// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- the locked eight-row timeline pins.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import IngestJobTimeline from './IngestJobTimeline';
import type { IngestStep } from './types';

afterEach(cleanup);

function makeStep(over: Partial<IngestStep>): IngestStep {
  return { step: 'parse', status: 'pending', attempt: 0, ...over };
}

const EIGHT: IngestStep[] = [
  { step: 'parse', status: 'done', attempt: 1, started_at: '2026-11-24T10:00:00.000Z', completed_at: '2026-11-24T10:00:03.200Z', artifact_ref: 'authority/x.md' },
  { step: 'chunk', status: 'done', attempt: 1, started_at: '2026-11-24T10:00:03.200Z', completed_at: '2026-11-24T10:00:04.000Z' },
  { step: 'embed', status: 'done', attempt: 1 },
  { step: 'vector_insert', status: 'done', attempt: 1 },
  { step: 'ecl_merge', status: 'skipped', attempt: 0, error: 'ecl auto-trigger not wired (54-04)' },
  { step: 'ecl_extract', status: 'failed', attempt: 2, error: '[deterministic] bad payload' },
  { step: 'ecl_kg_load', status: 'stale', attempt: 1, error: 'superseded by upstream retry' },
  { step: 'ecl_milvus_load', status: 'running', attempt: 1, started_at: '2026-11-24T10:00:05.000Z' },
];

describe('IngestJobTimeline (55-01)', () => {
  it('renders all eight rows anchored on STEP_SEQUENCE even for a shuffled payload', () => {
    const shuffled = [EIGHT[7], EIGHT[0], EIGHT[3], EIGHT[1], EIGHT[6], EIGHT[2], EIGHT[5], EIGHT[4]];
    render(<IngestJobTimeline steps={shuffled} />);
    const container = screen.getByTestId('ingest-job-timeline');
    expect(container).toHaveAttribute('aria-live', 'polite');
    const ids = Array.from(container.querySelectorAll('[data-testid^="ingest-step-"]'))
      .map((el) => el.getAttribute('data-testid'))
      .filter((id) => (id as string).split('-').length === 3 || !(id as string).includes('badge'));
    expect(ids.filter((id) => /^ingest-step-(parse|chunk|embed|vector_insert|ecl_merge|ecl_extract|ecl_kg_load|ecl_milvus_load)$/.test(id as string)))
      .toEqual([
        'ingest-step-parse',
        'ingest-step-chunk',
        'ingest-step-embed',
        'ingest-step-vector_insert',
        'ingest-step-ecl_merge',
        'ingest-step-ecl_extract',
        'ingest-step-ecl_kg_load',
        'ingest-step-ecl_milvus_load',
      ]);
  });

  it('renders the six derived badges with the locked labels, and pulses only on running', () => {
    render(<IngestJobTimeline steps={EIGHT} />);
    expect(screen.getByTestId('ingest-step-badge-parse')).toHaveTextContent('完成');
    expect(screen.getByTestId('ingest-step-badge-ecl_merge')).toHaveTextContent('已跳过');
    expect(screen.getByTestId('ingest-step-badge-ecl_extract')).toHaveTextContent('失败');
    expect(screen.getByTestId('ingest-step-badge-ecl_kg_load')).toHaveTextContent('待重跑');
    expect(screen.getByTestId('ingest-step-badge-ecl_milvus_load')).toHaveTextContent('进行中');

    // Pulse is running-only: stale is amber but must NOT pulse.
    expect(screen.getByTestId('ingest-step-badge-ecl_milvus_load').className).toContain('animate-pulse');
    expect(screen.getByTestId('ingest-step-badge-ecl_kg_load').className).not.toContain('animate-pulse');
    // skipped uses the neutral tone (gray), never DEFAULT_MAP's amber.
    expect(screen.getByTestId('ingest-step-badge-ecl_merge').className).toContain('status-unanswered');
  });

  it('renders the wall-clock duration for completed steps and — for running/missing stamps', () => {
    render(<IngestJobTimeline steps={EIGHT} />);
    expect(screen.getByTestId('ingest-step-duration-parse')).toHaveTextContent('3.2s');
    expect(screen.getByTestId('ingest-step-duration-ecl_milvus_load')).toHaveTextContent('—');
    expect(screen.getByTestId('ingest-step-duration-embed')).toHaveTextContent('—');
  });

  it('puts the failure reason on the row header title, and omits title when there is none', () => {
    render(<IngestJobTimeline steps={EIGHT} />);
    const failedRow = screen.getByTestId('ingest-step-ecl_extract');
    const header = failedRow.querySelector('button[aria-controls]') as HTMLElement;
    expect(header.getAttribute('title')).toBe('[deterministic] bad payload');

    const doneRow = screen.getByTestId('ingest-step-chunk');
    const doneHeader = doneRow.querySelector('button[aria-controls]') as HTMLElement;
    expect(doneHeader.getAttribute('title')).toBeNull();
  });

  it('renders the retry button ONLY for the failed step (no disabled props elsewhere)', () => {
    render(<IngestJobTimeline steps={EIGHT} onRetry={() => {}} />);
    const retry = screen.getByTestId('ingest-step-retry-ecl_extract');
    expect(retry).toHaveTextContent('重试此步骤');
    expect(retry).toHaveAttribute('aria-label', '重试步骤 实体抽取');

    for (const key of ['parse', 'chunk', 'embed', 'vector_insert', 'ecl_merge', 'ecl_kg_load', 'ecl_milvus_load']) {
      expect(screen.queryByTestId(`ingest-step-retry-${key}`)).toBeNull();
    }
  });

  it('calls onRetry with the step key and disables the button while the POST is in flight', () => {
    const onRetry = vi.fn();
    render(<IngestJobTimeline steps={EIGHT} onRetry={onRetry} retryInFlight="ecl_extract" />);
    const retry = screen.getByTestId('ingest-step-retry-ecl_extract');
    expect(retry).toBeDisabled();
    fireEvent.click(retry);
    expect(onRetry).not.toHaveBeenCalled();

    cleanup();
    render(<IngestJobTimeline steps={EIGHT} onRetry={onRetry} retryInFlight={null} />);
    fireEvent.click(screen.getByTestId('ingest-step-retry-ecl_extract'));
    expect(onRetry).toHaveBeenCalledWith('ecl_extract');
  });

  it('toggles the expanded panel and hands the step to renderPanel', () => {
    const renderPanel = vi.fn((s: IngestStep) => <div data-testid="panel-content">{s.step}</div>);
    render(<IngestJobTimeline steps={EIGHT} renderPanel={renderPanel} />);
    const header = screen.getByTestId('ingest-step-parse').querySelector('button[aria-controls]') as HTMLElement;
    expect(header).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('ingest-step-expand-parse')).toBeNull();

    fireEvent.click(header);
    expect(header).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByTestId('ingest-step-expand-parse')).toBeInTheDocument();
    expect(screen.getByTestId('panel-content')).toHaveTextContent('parse');
    expect(renderPanel).toHaveBeenCalled();
  });

  it('renders an unknown step key verbatim (unknown != blank)', () => {
    render(<IngestJobTimeline steps={[makeStep({ step: 'mystery_step', status: 'mystery_status' })]} />);
    const row = screen.getByTestId('ingest-step-mystery_step');
    expect(row).toHaveTextContent('mystery_step');
    expect(screen.getByTestId('ingest-step-badge-mystery_step')).toHaveTextContent('mystery_status');
  });
});
