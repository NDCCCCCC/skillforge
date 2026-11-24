// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-05 (ROI-01) -- the audit row's channel segment is SHAPE-DRIVEN:
// it renders only when the server attached channels, and a pre-58 row
// (no channels) must show no placeholder at all.
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AuditTimelineTab from '../components/AuditTimelineTab';
import { useAppStore } from '../store/app-store';

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function row(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    ts: '2026-11-24T10:00:00.000Z',
    action: 'action_approved',
    target: 'memory:facts',
    skill_version: '',
    prompt_hash: 'abcdef1234567890',
    ...over,
  };
}

beforeEach(() => {
  cleanup();
  useAppStore.setState({ activeSessionId: 'sess-ch' });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('AuditTimelineTab channels (58-05)', () => {
  it('renders the channel segment when the row carries channels', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('/audit')) {
        return json({ rows: [row({ channels: { vector: 3, graph: 1, hybrid: 2 } })] });
      }
      return json({ links: [] });
    }));
    render(<AuditTimelineTab />);
    await waitFor(() => expect(screen.getByTestId('audit-timeline-row')).toBeInTheDocument());
    expect(screen.getByTestId('audit-row-channels')).toHaveTextContent('语义 3 · 本体 1 · 双 2');
  });

  it('renders NO channel segment for a pre-58 row (shape-driven, no placeholder)', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('/audit')) return json({ rows: [row()] });
      return json({ links: [] });
    }));
    render(<AuditTimelineTab />);
    await waitFor(() => expect(screen.getByTestId('audit-timeline-row')).toBeInTheDocument());
    expect(screen.queryByTestId('audit-row-channels')).toBeNull();
  });

  it('drops a partial/garbage channel payload instead of coercing it to zeros', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('/audit')) {
        return json({ rows: [row({ channels: { vector: 'x', graph: 1, hybrid: 2 } })] });
      }
      return json({ links: [] });
    }));
    render(<AuditTimelineTab />);
    await waitFor(() => expect(screen.getByTestId('audit-timeline-row')).toBeInTheDocument());
    expect(screen.queryByTestId('audit-row-channels')).toBeNull();
  });
});
