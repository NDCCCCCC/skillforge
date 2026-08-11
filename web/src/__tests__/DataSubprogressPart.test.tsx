// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 1 — DataSubprogressPart tests.
//
// Pins the FEUI-06 5-step copy mapping per 04-UI-SPEC §Copywriting
// Contract:
//   - retrieve + tier=权威 → 检索 [权威] 知识层...
//   - found_candidates + count=12 → 找到 12 条候选
//   - rerank → 重排序中...
//   - self_verify → 自校验中...
//   - generate → 生成结论中...
// Plus the data-testid="data-subprogress" wrapper + data-step attribute.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DataSubprogressPart from '../components/chat/parts/DataSubprogressPart';
import type { SubprogressStep } from '../types/verdict';

afterEach(cleanup);

interface DataSubprogressPartType {
  type: 'data-subprogress';
  id?: string;
  data: SubprogressStep;
}

function makePart(data: SubprogressStep): DataSubprogressPartType {
  return { type: 'data-subprogress', id: 'sub-1', data };
}

describe('DataSubprogressPart', () => {
  it('retrieves [权威] knowledge layer for step="retrieve" + tier="权威"', () => {
    render(<DataSubprogressPart part={makePart({ step: 'retrieve', tier: '权威' })} />);
    const node = screen.getByTestId('data-subprogress');
    expect(node).toHaveAttribute('data-step', 'retrieve');
    expect(node).toHaveTextContent('检索 [权威] 知识层...');
  });

  it('renders "找到 12 条候选" for step="found_candidates" + count=12', () => {
    render(
      <DataSubprogressPart part={makePart({ step: 'found_candidates', count: 12 })} />,
    );
    const node = screen.getByTestId('data-subprogress');
    expect(node).toHaveAttribute('data-step', 'found_candidates');
    expect(node).toHaveTextContent('找到 12 条候选');
  });

  it('renders "重排序中..." for step="rerank"', () => {
    render(<DataSubprogressPart part={makePart({ step: 'rerank' })} />);
    const node = screen.getByTestId('data-subprogress');
    expect(node).toHaveAttribute('data-step', 'rerank');
    expect(node).toHaveTextContent('重排序中...');
  });

  it('renders "自校验中..." for step="self_verify"', () => {
    render(<DataSubprogressPart part={makePart({ step: 'self_verify' })} />);
    const node = screen.getByTestId('data-subprogress');
    expect(node).toHaveAttribute('data-step', 'self_verify');
    expect(node).toHaveTextContent('自校验中...');
  });

  it('renders "生成结论中..." for step="generate"', () => {
    render(<DataSubprogressPart part={makePart({ step: 'generate' })} />);
    const node = screen.getByTestId('data-subprogress');
    expect(node).toHaveAttribute('data-step', 'generate');
    expect(node).toHaveTextContent('生成结论中...');
  });
});
