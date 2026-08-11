// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 3 — REPLACE Wave 1 spike-stub citations test with
// the real chunk-lookup flow test.
//
// Pinned assertions:
//   1. CitationCluster renders a chip with data-testid="citation-chip",
//      data-chunk-id, data-tier matching the citation.
//   2. Clicking the chip dispatches setCenterView({ kind: 'chunk-source',
//      chunkId, tier, version, effectiveDate }) with the real metadata
//      pulled by extractChunkMetadata (c-001 → v3.2 / 2026-01-15).
//   3. The TierBadge inside the chip renders data-tier="权威" + [权威] text.
//   4. T-1-08 regression guard: SourceUrlPart with a javascript: URL
//      renders a blocked-citation span (NOT a clickable <a>).

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import CitationCluster from '../components/center/CitationCluster';
import { SourceUrlPart } from '../components/chat/parts/SourceUrlPart';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);

beforeEach(() => {
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

describe('CitationCluster + chunk-lookup flow', () => {
  it('renders a citation chip with data-chunk-id and data-tier', () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    const chip = screen.getByTestId('citation-chip');
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAttribute('data-chunk-id', 'c-001');
    expect(chip).toHaveAttribute('data-tier', '权威');
  });

  it('clicking the chip dispatches setCenterView with real chunk metadata', () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '权威 tier 条款', tier: '权威' },
        ]}
      />,
    );
    fireEvent.click(screen.getByTestId('citation-chip'));
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

  it('renders TierBadge inside the chip with [权威] text', () => {
    render(
      <CitationCluster
        citations={[
          { chunk_id: 'c-001', span: '条款', tier: '权威' },
        ]}
      />,
    );
    const badge = screen.getByTestId('tier-badge');
    expect(badge).toHaveAttribute('data-tier', '权威');
    expect(badge).toHaveTextContent('[权威]');
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
