// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- Variable Inspect panel pins (present AND
// absent branches; the absent branches are the current wire reality).
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import IngestStepInspect from './IngestStepInspect';

afterEach(cleanup);

const noop = () => {};

describe('IngestStepInspect (55-02)', () => {
  it('renders all eight known summary keys with zh labels and mono values', () => {
    render(
      <IngestStepInspect
        stepLabel="向量化"
        summary={{
          parsed_chars: 1024,
          chunk_count: 8,
          embedded_count: 7,
          embedded_failed: 1,
          inserted_rows: 7,
          entity_count: 12,
          triples_written: 30,
          purged_before_insert: 3,
        }}
        onGoEcl={noop}
      />,
    );
    expect(screen.getByTestId('ingest-step-summary')).toBeInTheDocument();
    for (const label of ['产物字数', '块数', '向量数', '失败块数', '入库行数', '抽取实体数', '三元组写入数', '插入前清理行数']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.getByText('1024')).toBeInTheDocument();
  });

  it('renders an UNKNOWN summary key verbatim (never blank)', () => {
    render(<IngestStepInspect stepLabel="解析" summary={{ weird_metric: 7 }} onGoEcl={noop} />);
    expect(screen.getByText('weird_metric')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
  });

  it('does not render the summary block when summary is absent (54 wire reality)', () => {
    render(<IngestStepInspect stepLabel="解析" onGoEcl={noop} />);
    expect(screen.queryByTestId('ingest-step-summary')).toBeNull();
    expect(screen.queryByText('输入摘要')).toBeNull();
  });

  it('renders artifact locator mono with a truncate + title, — when absent', () => {
    render(<IngestStepInspect stepLabel="解析" artifactRef="authority/abc-doc.md" onGoEcl={noop} />);
    const artifact = screen.getByTestId('ingest-step-artifact');
    expect(artifact).toHaveTextContent('authority/abc-doc.md');
    expect(artifact).toHaveAttribute('title', 'authority/abc-doc.md');
    expect(artifact.className).toContain('truncate');

    cleanup();
    render(<IngestStepInspect stepLabel="解析" onGoEcl={noop} />);
    expect(screen.getByTestId('ingest-step-artifact')).toHaveTextContent('—');
  });

  it('renders the attempt count and the deterministic-error annotation', () => {
    render(<IngestStepInspect stepLabel="实体抽取" attempt={3} deadNote onGoEcl={noop} />);
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByTestId('ingest-step-dead-note')).toHaveTextContent('确定性错误 — 不自动重试');
  });

  it('renders the retry hint verbatim without over-selling', () => {
    render(<IngestStepInspect stepLabel="实体抽取" showRetryHint onGoEcl={noop} />);
    const hint = screen.getByTestId('ingest-step-retry-hint');
    expect(hint).toHaveTextContent('重试从上游最近成功步骤的产物继续，下游步骤将标记待重跑');
    expect(hint.textContent).not.toContain('零风险');
    expect(hint.textContent).not.toContain('即时完成');
  });

  it('renders the failure reason verbatim in a pre-wrap danger block', () => {
    render(
      <IngestStepInspect
        stepLabel="实体抽取"
        error={'[deterministic] bad payload\nsecond line'}
        onGoEcl={noop}
      />,
    );
    const err = screen.getByTestId('ingest-step-error');
    expect(err.textContent).toContain('bad payload');
    expect(err.textContent).toContain('second line');
    expect(err.className).toContain('whitespace-pre-wrap');
    expect(err.className).toContain('text-status-non-compliant');
  });

  it('renders the skip reason muted and verbatim', () => {
    render(
      <IngestStepInspect stepLabel="切分合并" skipReason="ecl auto-trigger not wired" onGoEcl={noop} />,
    );
    const skip = screen.getByTestId('ingest-step-skip-reason');
    expect(skip).toHaveTextContent('ecl auto-trigger not wired');
    expect(skip.className).toContain('text-text-muted');
  });

  it('renders the ECL cross-link only for ecl steps and calls onGoEcl on click', () => {
    const onGoEcl = vi.fn();
    render(<IngestStepInspect stepLabel="切分合并" isEclStep onGoEcl={onGoEcl} />);
    fireEvent.click(screen.getByTestId('ingest-step-ecl-link'));
    expect(onGoEcl).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('ingest-step-ecl-link')).toHaveTextContent('ECL 全局状态见 ECL 管线 tab');

    cleanup();
    render(<IngestStepInspect stepLabel="解析" onGoEcl={onGoEcl} />);
    expect(screen.queryByTestId('ingest-step-ecl-link')).toBeNull();
  });
});
