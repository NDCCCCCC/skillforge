// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-04 (PROV-03) -- the refusal card must say WHAT was searched.
// Two pins:
//   A. with stats -> the locked统计 line renders (numbers from the server);
//   B. without stats -> NOTHING is rendered (no fabricated "0 命中").
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import RefusalCard from '../components/center/RefusalCard';

afterEach(cleanup);

const refusal = {
  reason: 'no_authorized_source' as const,
  can_user_rephrase: false,
  suggested_alternative_query: '',
};

describe('RefusalCard + retrieval stats (58-04)', () => {
  it('renders the stats line with the server numbers', () => {
    render(
      <RefusalCard
        refusal={refusal}
        retrieval={{
          channels: { vector: 0, graph: 2, hybrid: 0 },
          anchor_entities: ['差旅住宿费标准'],
          kg: { available: true, entities_matched: 1 },
        }}
      />,
    );
    const line = screen.getByTestId('refusal-retrieval-stats');
    expect(line).toHaveTextContent('本次检索：语义 0 命中 · 本体 2 · 双通道 0');
    expect(line).toHaveTextContent('锚定实体：差旅住宿费标准');
  });

  it('renders the degraded suffix when the turn ran without a KG', () => {
    render(
      <RefusalCard
        refusal={refusal}
        retrieval={{ channels: { vector: 0, graph: 0, hybrid: 0 }, degraded: true }}
      />,
    );
    expect(screen.getByTestId('refusal-retrieval-stats')).toHaveTextContent(
      '本体暂不可用，已降级为纯语义检索',
    );
  });

  it('renders NO stats line when retrieval is absent (pre-58 messages)', () => {
    render(<RefusalCard refusal={refusal} />);
    expect(screen.queryByTestId('refusal-retrieval-stats')).toBeNull();
  });
});
