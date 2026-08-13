// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 T3 — AuditTable tests.
//
// Validates SC-3 surface: rows render, filter-bar changes dispatch
// setAuditFilter and refetch with updated query params, and redacted
// cells show the gray badge with hover detail.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AuditTable from './AuditTable';
import { useAppStore } from '../../store/app-store';
import type { AuditRow } from '../../types/eval';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  useAppStore.setState({ auditRows: [], auditFilter: { limit: 100, offset: 0 } });
});

const RAW_ROW: AuditRow = {
  ts: '2026-08-13T10-05-21Z',
  user_id: 'alice',
  skill_id: 'compliance',
  session_id: 'sess-1',
  refusal_reason: 'resolved',
  query: 'What is the travel policy?',
  verdict_json: '{"answer":"see section 3"}',
  faithfulness: 0.92,
  context_precision: 0.85,
  answer_relevance: 0.88,
  context_recall: 0.79,
};

const REDACTED_ROW: AuditRow = {
  ts: '2026-01-01T00-00-00Z',
  user_id: 'bob',
  skill_id: 'compliance',
  session_id: 'sess-2',
  refusal_reason: 'non_compliant',
  query: '[REDACTED:90d+ (ts=2026-01-01T00-00-00Z, boundary=2026-04-01T00-00-00Z)]',
  verdict_json: '[REDACTED:90d+ (ts=2026-01-01T00-00-00Z, boundary=2026-04-01T00-00-00Z)]',
};

function mockFetchOk(rows: AuditRow[]): ReturnType<typeof vi.fn> {
  const fn = vi.fn().mockImplementation(async (url: string) => {
    // Capture the query string so tests can assert on it.
    return {
      ok: true,
      status: 200,
      json: async () => rows,
      url,
    } as unknown as Response;
  });
  vi.stubGlobal('fetch', fn);
  return fn;
}

describe('AuditTable', () => {
  it('renders rows from /api/audit', async () => {
    mockFetchOk([RAW_ROW, REDACTED_ROW]);
    render(<AuditTable />);

    const rows = await screen.findAllByTestId('audit-table-row');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveAttribute('data-user', 'alice');
    expect(rows[0]).toHaveTextContent('What is the travel policy?');
  });

  it('renders [REDACTED] badges on rows past retention', async () => {
    mockFetchOk([REDACTED_ROW]);
    render(<AuditTable />);

    const badges = await screen.findAllByTestId('audit-row-redacted');
    expect(badges.length).toBeGreaterThanOrEqual(1);
    expect(badges[0]).toHaveAttribute(
      'title',
      '[REDACTED:90d+ (ts=2026-01-01T00-00-00Z, boundary=2026-04-01T00-00-00Z)]',
    );
  });

  it('filter bar change updates the fetch query params', async () => {
    const fetchMock = mockFetchOk([RAW_ROW]);
    render(<AuditTable />);

    // Wait for the mount fetch to settle.
    await screen.findAllByTestId('audit-table-row');

    const skillInput = screen.getByTestId('audit-filter-skill') as HTMLInputElement;
    fireEvent.change(skillInput, { target: { value: 'compliance' } });

    await waitFor(() => {
      const calls = fetchMock.mock.calls as [string][];
      const auditCalls = calls.filter(([url]) => url.startsWith('/api/audit'));
      expect(auditCalls.some(([url]) => url.includes('skill=compliance'))).toBe(true);
    });
  });

  it('renders the empty state when there are no rows', () => {
    render(<AuditTable rows={[]} />);
    expect(screen.getByTestId('audit-table-empty')).toBeInTheDocument();
  });
});
