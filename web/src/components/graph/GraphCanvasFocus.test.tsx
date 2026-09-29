// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-d5w3 Task 1 — question-focus rendering pins.
//
//  ① focus mode: cited docs render lit (opacity 1 + data-focus="true" +
//    highlight ring), non-focus docs fade to 0.12, non-focus tiers 淡显.
//  ② toggle: 仅当前问题 ↔ 全库 flips opacities and persists the choice.
//  ③ answered turn WITHOUT citations (refusal) → honest empty note.
//  ③b source-path basename match (tolerated future wire field).
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import GraphCanvas from './GraphCanvas';
import {
  extractGraphFocus,
  hasAnsweredTurn,
  type GraphFocus,
} from '../../lib/graph-focus';
import type { KnowledgeGraph } from '../../types/knowledge';
import type { UIMessage } from 'ai';

const TWO_TIER_GRAPH: KnowledgeGraph = {
  tiers: [
    {
      tier: 'authority',
      docCount: 1,
      chunkCount: 2,
      docs: [
        {
          id: 'authority::authority-doc.md',
          filename: 'authority-doc.md',
          chunkCount: 2,
          chunks: [
            { id: '101', effectiveFrom: null, effectiveTo: null, version: null },
            { id: '102', effectiveFrom: null, effectiveTo: null, version: null },
          ],
        },
      ],
    },
    {
      tier: 'historical',
      docCount: 1,
      chunkCount: 1,
      docs: [
        {
          id: 'historical::historical-doc.md',
          filename: 'historical-doc.md',
          chunkCount: 1,
          chunks: [{ id: '202', effectiveFrom: null, effectiveTo: null, version: null }],
        },
      ],
    },
  ],
};

/** No registry fetch needed — labels degrade to [slug] without stubbing. */
function renderCanvas(focus: GraphFocus | null, hasAnswer = false) {
  return render(<GraphCanvas graph={TWO_TIER_GRAPH} filter="" focus={focus} hasAnswer={hasAnswer} />);
}

function docNode(file: string): HTMLElement {
  return screen
    .getAllByTestId('graph-doc-node')
    .find((el) => el.getAttribute('data-file') === file)!;
}

beforeEach(() => {
  // The toggle persists via localStorage — isolate every test from the
  // previous test's choice (default must be 仅当前问题).
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
});

describe('GraphCanvas — question focus (260929-d5w3)', () => {
  it('① focuses cited docs: lit + ring + data-focus, non-focus fades to 0.12', () => {
    renderCanvas({ chunkIds: new Set(['101']), sources: [] });
    const cited = docNode('authority-doc.md');
    const other = docNode('historical-doc.md');
    // Focus doc: full opacity + marked + enlarged (r=11) with ring.
    expect(cited.getAttribute('opacity')).toBe('1');
    expect(cited.getAttribute('data-focus')).toBe('true');
    expect(cited.querySelector('circle[stroke-dasharray="3 3"]')).not.toBeNull();
    // Non-focus doc: faded, marked false.
    expect(other.getAttribute('opacity')).toBe('0.12');
    expect(other.getAttribute('data-focus')).toBe('false');
    // Focus tier stays lit; the other tier 淡显.
    const tiers = screen.getAllByTestId('graph-tier-node');
    const authorityTier = tiers.find((el) => el.getAttribute('data-tier') === 'authority')!;
    const historicalTier = tiers.find((el) => el.getAttribute('data-tier') === 'historical')!;
    expect(authorityTier.getAttribute('data-focus')).toBe('true');
    expect(authorityTier.getAttribute('opacity')).toBe('1');
    expect(historicalTier.getAttribute('opacity')).toBe('0.15');
    // Toggle defaults to 仅当前问题.
    const toggle = screen.getByTestId('graph-focus-toggle');
    expect(toggle).toHaveTextContent('仅当前问题');
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    // No empty note while citations exist.
    expect(screen.queryByTestId('graph-focus-empty')).toBeNull();
  });

  it('② toggling to 全库 restores every doc and persists the choice', () => {
    renderCanvas({ chunkIds: new Set(['101']), sources: [] });
    fireEvent.click(screen.getByTestId('graph-focus-toggle'));
    const toggle = screen.getByTestId('graph-focus-toggle');
    expect(toggle).toHaveTextContent('全库');
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    for (const el of screen.getAllByTestId('graph-doc-node')) {
      expect(el.getAttribute('opacity')).toBe('1');
      expect(el.getAttribute('data-focus')).toBeNull();
    }
    expect(window.localStorage.getItem('skillforge.graph.focusMode')).toBe('all');
  });

  it('② remembers a persisted 全库 choice across remounts', () => {
    window.localStorage.setItem('skillforge.graph.focusMode', 'all');
    cleanup();
    renderCanvas({ chunkIds: new Set(['101']), sources: [] });
    expect(screen.getByTestId('graph-focus-toggle')).toHaveTextContent('全库');
    // Non-focus docs stay lit — the persisted choice wins over the default.
    expect(docNode('historical-doc.md').getAttribute('opacity')).toBe('1');
  });

  it('③ shows the honest empty note when an answered turn has no citations', () => {
    renderCanvas(null, true);
    expect(screen.getByTestId('graph-focus-empty')).toHaveTextContent(
      '当前回答无引用原文 — 显示全库',
    );
    // Full-graph mode: no toggle, no fading.
    expect(screen.queryByTestId('graph-focus-toggle')).toBeNull();
    expect(docNode('historical-doc.md').getAttribute('opacity')).toBe('1');
  });

  it('③ shows no note for a fresh session without answers', () => {
    renderCanvas(null, false);
    expect(screen.queryByTestId('graph-focus-empty')).toBeNull();
  });

  it('③b matches citation.source basename against doc.filename', () => {
    renderCanvas({ chunkIds: new Set(), sources: ['corpus/authority/authority-doc.md'] });
    expect(docNode('authority-doc.md').getAttribute('data-focus')).toBe('true');
    expect(docNode('authority-doc.md').getAttribute('opacity')).toBe('1');
    expect(docNode('historical-doc.md').getAttribute('opacity')).toBe('0.12');
  });
});

describe('lib/graph-focus — message scanning', () => {
  function assistantMsg(parts: unknown[]): UIMessage {
    return { id: `m-${Math.random()}`, role: 'assistant', parts } as unknown as UIMessage;
  }
  function policyPart(data: unknown) {
    return { type: 'data-policy', data };
  }

  it('extracts citations from the LATEST verdict and ignores older ones', () => {
    const messages: UIMessage[] = [
      assistantMsg([policyPart({ status: 'ok', citations: [{ chunk_id: 'old-1' }] })]),
      { id: 'u2', role: 'user', parts: [] } as unknown as UIMessage,
      assistantMsg([policyPart({ status: 'ok', citations: [{ chunk_id: 'new-1' }, { chunk_id: 'new-2' }] })]),
    ];
    const focus = extractGraphFocus(messages);
    expect(focus).not.toBeNull();
    expect([...focus!.chunkIds]).toEqual(['new-1', 'new-2']);
  });

  it('returns null for a refusal verdict (citations: []) — no fallback to older turns', () => {
    const messages: UIMessage[] = [
      assistantMsg([policyPart({ status: 'ok', citations: [{ chunk_id: 'old-1' }] })]),
      assistantMsg([policyPart({ status: 'refused', citations: [] })]),
    ];
    expect(extractGraphFocus(messages)).toBeNull();
  });

  it('returns null when no assistant verdict exists yet', () => {
    expect(extractGraphFocus([{ id: 'u1', role: 'user', parts: [] } as unknown as UIMessage])).toBeNull();
  });

  it('hasAnsweredTurn is true for a refusal (data-policy present) and false for a fresh session', () => {
    expect(hasAnsweredTurn([assistantMsg([policyPart({ reason: 'out_of_scope' })])])).toBe(true);
    expect(hasAnsweredTurn([{ id: 'u1', role: 'user', parts: [] } as unknown as UIMessage])).toBe(false);
  });
});
