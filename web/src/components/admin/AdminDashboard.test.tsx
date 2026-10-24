// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 26-08 Task 2 — the 'bases' subroute wiring in the admin shell.
//
// Pins that the KB management tab is reachable, activates, and mounts the
// KnowledgeBaseList view (the shell stays structural — the view owns its
// own data fetch).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AdminDashboard from './AdminDashboard';
import { useAppStore } from '../../store/app-store';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
  useAppStore.setState({ adminSubroute: 'batches' });
  vi.stubGlobal(
    'fetch',
    vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/admin/kb') {
        return Promise.resolve(
          jsonResponse({
            signature: 'sig-1',
            bases: [
              {
                slug: 'authority',
                display_name: '权威',
                role: 'authority',
                description: '唯一可引用证据源',
                active: true,
                created_at: '2026-09-07T00:00:00Z',
                updated_at: '2026-09-07T00:00:00Z',
              },
            ],
          }),
        );
      }
      // CapacityMonitor / ImportBatchList / other views degrade on !ok.
      return Promise.resolve(jsonResponse({ error: 'not stubbed' }, 404));
    }),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AdminDashboard — bases subroute', () => {
  it('activates the Knowledge Bases tab and mounts the KB management view', async () => {
    render(<AdminDashboard />);
    expect(screen.queryByTestId('kb-view')).toBeNull();

    fireEvent.click(screen.getByTestId('admin-tab-bases'));

    await waitFor(() => expect(screen.getByTestId('kb-view')).toBeInTheDocument());
    // The mounted view really is the KB registry surface.
    expect(await screen.findByTestId('kb-row-authority')).toBeInTheDocument();
  });

  it('labels the tab "知识库"', () => {
    render(<AdminDashboard />);
    expect(screen.getByTestId('admin-tab-bases')).toHaveTextContent('知识库');
  });

  it('switching away unmounts the KB view', async () => {
    render(<AdminDashboard />);
    fireEvent.click(screen.getByTestId('admin-tab-bases'));
    await waitFor(() => expect(screen.getByTestId('kb-view')).toBeInTheDocument());

    fireEvent.click(screen.getByTestId('admin-tab-kpis'));
    await waitFor(() => expect(screen.queryByTestId('kb-view')).toBeNull());
  });
});
