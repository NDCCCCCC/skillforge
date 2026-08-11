// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 01-03 Task 2 — SC-4 citation rendering tests.
//
// These tests pin the three rendering behaviors the spike stub MUST
// guarantee before Phase 4 builds on top of it:
//  1. source-url chunks render as clickable <a> elements with the
//     chunk's URL as href (the core SC-4 contract).
//  2. Unknown part types are silently dropped (AI SDK v5+ spec) so a
//     newer-server / older-client combo doesn't crash the UI.
//  3. Reasoning chunks render inside <details> so the trace is hidden
//     behind a disclosure widget (matches sketch-findings-ai-agent
//     layout decision: reasoning is collapsible, not always-visible).
//
// The stub renders seeded messages directly (App initialMessages prop)
// — no real transport call, no jsdom network. Phase 4 will add a full
// E2E test against a live Hertz endpoint; Phase 1 only validates the
// rendering contract.

import { cleanup, render, screen } from '@testing-library/react';
import type { UIMessage } from 'ai';
import { afterEach, describe, expect, it } from 'vitest';
import App from '../App';

afterEach(cleanup);

describe('SpikeStub citations (SC-4)', () => {
  it('renders source-url as clickable <a>', () => {
    const messages: UIMessage[] = [
      {
        id: 'm1',
        role: 'assistant',
        parts: [
          { type: 'text', text: 'Authority policy citation: ' },
          {
            type: 'source-url',
            sourceId: 'src-1',
            url: 'https://policy.example/auth-001',
            title: 'Authority Policy 1',
          },
        ],
      },
    ];

    render(<App initialMessages={messages} />);

    // getByRole('link', name) matches the <a> by its accessible name,
    // which is the rendered text content "[Authority Policy 1]".
    const link = screen.getByRole('link', { name: /Authority Policy 1/i });
    expect(link.tagName).toBe('A');
    expect(link).toHaveAttribute('href', 'https://policy.example/auth-001');
  });

  it('drops unknown part types silently', () => {
    // Cast through unknown so TS lets us include a hypothetical future
    // type the SDK doesn't know about yet — this is exactly the scenario
    // the AI SDK v5+ "silently drop" behavior protects against.
    const messages = [
      {
        id: 'm1',
        role: 'assistant',
        parts: [
          { type: 'text', text: 'hi' },
          { type: 'unknown-future-type', payload: 'irrelevant' },
        ],
      },
    ] as unknown as UIMessage[];

    render(<App initialMessages={messages} />);

    expect(screen.getByText('hi')).toBeInTheDocument();
    expect(screen.queryByText('irrelevant')).toBeNull();
  });

  it('renders reasoning in <details>', () => {
    const messages: UIMessage[] = [
      {
        id: 'm1',
        role: 'assistant',
        parts: [{ type: 'reasoning', text: 'deep thought' }],
      },
    ];

    render(<App initialMessages={messages} />);

    const details = screen.getByText('deep thought');
    expect(details.closest('details')).not.toBeNull();
  });
});
