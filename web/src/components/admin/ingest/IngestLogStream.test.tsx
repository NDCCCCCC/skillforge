// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-02 (ING-05) -- the collapsible log pins (default collapsed,
// verbatim lines, absent-field empty copy).
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import IngestLogStream from './IngestLogStream';

afterEach(cleanup);

describe('IngestLogStream (55-02)', () => {
  it('is collapsed by default and toggles open/closed with the locked labels', () => {
    render(<IngestLogStream lines={['line one']} />);
    expect(screen.queryByTestId('ingest-log-body')).toBeNull();
    const toggle = screen.getByTestId('ingest-log-toggle');
    expect(toggle).toHaveTextContent('展开');

    fireEvent.click(toggle);
    expect(screen.getByTestId('ingest-log-body')).toBeInTheDocument();
    expect(toggle).toHaveTextContent('收起');

    fireEvent.click(toggle);
    expect(screen.queryByTestId('ingest-log-body')).toBeNull();
    expect(toggle).toHaveTextContent('展开');
  });

  it('renders each line verbatim in a mono pre, unparsed and uncolored', () => {
    const lines = ['  step parse started', '[transient] embed retry 1/3', 'done'];
    render(<IngestLogStream lines={lines} />);
    fireEvent.click(screen.getByTestId('ingest-log-toggle'));
    const rendered = screen.getAllByTestId('ingest-log-line');
    expect(rendered).toHaveLength(3);
    expect(rendered[0].textContent).toBe('  step parse started');
    expect(rendered[1].textContent).toBe('[transient] embed retry 1/3');
    expect(rendered[1].className).toContain('font-mono');
  });

  it('renders the locked empty copy for an empty array', () => {
    render(<IngestLogStream lines={[]} />);
    fireEvent.click(screen.getByTestId('ingest-log-toggle'));
    expect(screen.getByTestId('ingest-log-empty')).toHaveTextContent('本任务暂无过程日志');
  });

  it('renders the same empty copy when the field is absent (54 wire reality)', () => {
    render(<IngestLogStream />);
    fireEvent.click(screen.getByTestId('ingest-log-toggle'));
    expect(screen.getByTestId('ingest-log-empty')).toHaveTextContent('本任务暂无过程日志');
  });

  it('constrains the body height with a scroll region', () => {
    render(<IngestLogStream lines={['x']} />);
    fireEvent.click(screen.getByTestId('ingest-log-toggle'));
    const body = screen.getByTestId('ingest-log-body');
    expect(body.className).toContain('max-h-48');
    expect(body.className).toContain('overflow-y-auto');
  });
});
