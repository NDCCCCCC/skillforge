// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 3 + Plan 260815-cos Task 3 — REPLACE Wave 1 spike-stub
// citations test with the real chunk-lookup flow test.
//
// Pinned assertions:
//   1. CitationCluster renders a superscript with data-testid=
//      "citation-superscript", data-chunk-id, data-tier matching the
//      citation. Plan 260815-cos switched from chip → superscript.
//   2. Clicking the superscript dispatches setCenterView({ kind:
//      'chunk-source', chunkId, tier, version, effectiveDate }) with
//      the real metadata pulled by extractChunkMetadata (c-001 → v3.2 /
//      2026-01-15).
//   3. Hovering the superscript opens a popover carrying the verbatim
//      chunk text + tier badge + version + effective date (C3).
//   4. The TierBadge inside the popover renders data-tier="权威" +
//      [权威] text.
//   5. T-1-08 regression guard: SourceUrlPart with a javascript: URL
//      renders a blocked-citation span (NOT a clickable <a>).

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import CitationCluster from '../components/center/CitationCluster';
import { SourceUrlPart } from '../components/chat/parts/SourceUrlPart';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);

beforeEach(() => {
  useAppStore.setState({ centerView: { kind: 'empty' } });
  vi.useRealTimers();
});

describe('CitationCluster + chunk-lookup flow', () => {
  it('renders a superscript citation with data-chunk-id and data-tier', () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    const sup = screen.getByTestId('citation-superscript');
    expect(sup).toBeInTheDocument();
    expect(sup).toHaveAttribute('data-chunk-id', 'c-001');
    expect(sup).toHaveAttribute('data-tier', '权威');
    expect(sup).toHaveTextContent('[1]');
  });

  it('clicking the superscript dispatches setCenterView with real chunk metadata', () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    fireEvent.click(screen.getByTestId('citation-superscript'));
    const centerView = useAppStore.getState().centerView;
    expect(centerView.kind).toBe('chunk-source');
    if (centerView.kind === 'chunk-source') {
      expect(centerView.chunkId).toBe('c-001');
      expect(centerView.tier).toBe('权威');
      // Real metadata pulled by extractChunkMetadata:
      expect(centerView.version).toBe('v3.2');
      expect(centerView.effectiveDate).toBe('2026-01-15');
    }
  });

  // 56-04 Task 5 -- the jump payload is taken from the Citation object
  // itself (not from the array index, not primarily from the fixture),
  // so an audited replay's version/effective_from survive the jump.
  it('uses the Citation object\'s own version/effective_from for the jump', () => {
    render(
      <CitationCluster
        citations={[
          {
            chunk_id: 'c-audit-replay-9',
            span: '历史回放条款',
            tier: '历史',
            version: 'v2024.7',
            effective_from: '2024-07-01',
          },
        ]}
      />,
    );
    fireEvent.click(screen.getByTestId('citation-superscript'));
    const cv = useAppStore.getState().centerView;
    expect(cv.kind).toBe('chunk-source');
    if (cv.kind === 'chunk-source') {
      // chunk_id is authoritative (never the index '[1]').
      expect(cv.chunkId).toBe('c-audit-replay-9');
      expect(cv.tier).toBe('历史');
      // D5-15 fields come from the citation, not from a fixture lookup.
      expect(cv.version).toBe('v2024.7');
      expect(cv.effectiveDate).toBe('2024-07-01');
    }
  });

  it('falls back to the fixture metadata for citations that omit the D5-15 fields', () => {
    render(
      <CitationCluster
        citations={[{ chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' }]}
      />,
    );
    fireEvent.click(screen.getByTestId('citation-superscript'));
    const cv = useAppStore.getState().centerView;
    if (cv.kind === 'chunk-source') {
      expect(cv.version).toBe('v3.2');
      expect(cv.effectiveDate).toBe('2026-01-15');
    }
  });

  it('hovering the superscript opens a popover with the chunk text + metadata', async () => {
    // Real timers here: the popover mount is gated on a 150ms setTimeout
    // debounce (HOVER_OPEN_DELAY_MS) — vitest fake-timer advance does
    // not flush React's commit synchronously with findByTestId's retry
    // loop, so we let the real timer fire and use findByTestId to wait.
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    fireEvent.mouseEnter(screen.getByTestId('citation-superscript'));
    const popover = await screen.findByTestId('citation-popover');
    expect(popover).toBeInTheDocument();
    // Popover carries the verbatim fixture text.
    expect(popover).toHaveTextContent('200 元以内的超标部分');
    // Tier badge inside the popover renders data-tier + text.
    const badge = screen.getByTestId('tier-badge');
    expect(badge).toHaveAttribute('data-tier', '权威');
    expect(badge).toHaveTextContent('[权威]');
    // Version + effective date are rendered.
    expect(popover).toHaveTextContent('v3.2');
    expect(popover).toHaveTextContent('2026-01-15');
  });

  it('mouseLeave closes the popover', async () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    const sup = screen.getByTestId('citation-superscript');
    fireEvent.mouseEnter(sup);
    await screen.findByTestId('citation-popover');
    fireEvent.mouseLeave(sup);
    expect(screen.queryByTestId('citation-popover')).toBeNull();
  });

  it('SourceUrlPart blocks javascript: URL via T-1-08 sanitization gate', () => {
    // Regression guard — the citation click flow uses CitationCluster
    // (which dispatches to ChunkSourceView), but T-1-08 also covers
    // the source-url renderer for external link citations.
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [
        {
          type: 'source-url',
          sourceId: 's1',
          url: 'javascript:alert(1)',
          title: 'malicious',
        } as unknown as UIMessage['parts'][number],
      ],
    };

    render(
      <div>
        {message.parts.map((p, i) => {
          if (p.type === 'source-url') {
            return (
              <SourceUrlPart
                key={i}
                part={p as unknown as Parameters<typeof SourceUrlPart>[0]['part']}
              />
            );
          }
          return null;
        })}
      </div>,
    );
    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.getByTestId('citation-blocked')).toBeInTheDocument();
  });
});
