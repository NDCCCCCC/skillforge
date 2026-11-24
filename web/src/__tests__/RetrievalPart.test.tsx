// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-04 (PROV-03) -- the retrieval summary pins: server numbers
// only, guard-fail renders nothing, and the two KG badges stay distinct.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import RetrievalPart from '../components/chat/parts/RetrievalPart';
import { isRetrievalStats } from '../types/verdict';

afterEach(cleanup);

const stats = {
  channels: { vector: 3, graph: 1, hybrid: 2 },
  anchor_entities: ['差旅住宿费标准', '年假制度'],
  kg: { available: true, entities_matched: 2 },
};

describe('isRetrievalStats', () => {
  it('accepts a well-formed payload', () => {
    expect(isRetrievalStats(stats)).toBe(true);
  });

  it('rejects malformed shapes instead of coercing them to zeros', () => {
    expect(isRetrievalStats(undefined)).toBe(false);
    expect(isRetrievalStats(null)).toBe(false);
    expect(isRetrievalStats({})).toBe(false);
    expect(isRetrievalStats({ channels: {} })).toBe(false);
    expect(isRetrievalStats({ channels: { vector: 'x', graph: 1, hybrid: 2 } })).toBe(false);
    expect(isRetrievalStats({ channels: { vector: -1, graph: 0, hybrid: 0 } })).toBe(false);
  });
});

describe('RetrievalPart', () => {
  it('renders the channel distribution with the server numbers', () => {
    render(<RetrievalPart data={stats} />);
    const summary = screen.getByTestId('retrieval-summary');
    expect(summary).toHaveAttribute('aria-label', '检索通道分布');
    expect(summary).toHaveTextContent('检索命中 语义 3 · 本体 1 · 双通道 2');
  });

  it('renders the anchored entities joined with 、', () => {
    render(<RetrievalPart data={stats} />);
    expect(screen.getByTestId('retrieval-anchors')).toHaveTextContent('锚定实体：差旅住宿费标准、年假制度');
  });

  it('renders NOTHING for a malformed payload (no fabricated zeros)', () => {
    const { container } = render(<RetrievalPart data={{ channels: {} }} />);
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByTestId('retrieval-summary')).toBeNull();
  });

  it('shows the gray "ontology not enabled" badge when the KG is absent', () => {
    render(<RetrievalPart data={{ ...stats, kg: { available: false, entities_matched: 0 } }} />);
    expect(screen.getByTestId('kg-absent-badge')).toHaveTextContent('本体增强未启用');
  });

  it('shows the amber degraded badge when the KG failed, and BOTH can coexist', () => {
    render(
      <RetrievalPart
        data={{ ...stats, kg: { available: false, entities_matched: 0 }, degraded: true }}
      />,
    );
    expect(screen.getByTestId('kg-absent-badge')).toBeInTheDocument();
    expect(screen.getByTestId('kg-degraded-badge')).toHaveTextContent('本体暂不可用，已降级为纯语义检索');
  });
});
