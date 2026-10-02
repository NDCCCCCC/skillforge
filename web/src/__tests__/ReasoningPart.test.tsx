// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 2 — ReasoningPart tests.
//
// Pins the D-08 invariants (Phase 4 visual upgrade — modernized to
// amber "thinking" card with `推理过程` label):
//   - renders `data-testid="reasoning-preview"` wrapper
//   - `data-expanded` boolean attribute reflects toggle state
//   - body uses `.reasoning-card-body` CSS class for the 180px cap
//   - toggle button labeled `展开` when collapsed, `收起` when expanded
//   - label text is `推理过程` plus `推理中...` suffix when part.state === 'streaming'
//   - label text is `推理过程` (no suffix) when complete
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import type { UIMessage } from 'ai';
import { afterEach, describe, expect, it } from 'vitest';
import ReasoningPart from '../components/chat/parts/ReasoningPart';

afterEach(cleanup);

type ReasoningPartType = Extract<UIMessage['parts'][number], { type: 'reasoning' }>;

function makePart(overrides: Partial<ReasoningPartType> = {}): ReasoningPartType {
  return {
    type: 'reasoning',
    text: 'thinking through the policy lookup...',
    state: 'done',
    ...overrides,
  };
}

describe('ReasoningPart', () => {
  it('renders the preview wrapper with data-testid="reasoning-preview"', () => {
    render(<ReasoningPart part={makePart()} />);
    expect(screen.getByTestId('reasoning-preview')).toBeInTheDocument();
  });

  it('exposes data-expanded=false by default and data-expanded=true after toggle', () => {
    render(<ReasoningPart part={makePart()} />);
    const preview = screen.getByTestId('reasoning-preview');
    expect(preview).toHaveAttribute('data-expanded', 'false');

    fireEvent.click(screen.getByTestId('reasoning-toggle'));
    expect(preview).toHaveAttribute('data-expanded', 'true');
  });

  it('toggles the button label between 展开 and 收起', () => {
    render(<ReasoningPart part={makePart()} />);
    const toggle = screen.getByTestId('reasoning-toggle');
    expect(toggle).toHaveTextContent('展开');

    fireEvent.click(toggle);
    expect(toggle).toHaveTextContent('收起');
  });

  it('renders the body text inside a <pre> once expanded (B5: collapsed = header only)', () => {
    render(<ReasoningPart part={makePart({ text: 'specific reasoning copy' })} />);
    // Collapsed (B5 default) — the body is not rendered at all.
    expect(screen.queryByTestId('reasoning-body')).toBeNull();
    fireEvent.click(screen.getByTestId('reasoning-toggle'));
    const body = screen.getByTestId('reasoning-body');
    expect(body.tagName).toBe('PRE');
    expect(body).toHaveTextContent('specific reasoning copy');
  });

  it('appends 推理中... suffix when part.state === "streaming"', () => {
    render(<ReasoningPart part={makePart({ state: 'streaming' })} />);
    expect(screen.getByTestId('reasoning-preview')).toHaveTextContent('推理过程');
    expect(screen.getByTestId('reasoning-preview')).toHaveTextContent('推理中...');
  });

  it('appends 生成中... suffix when streamingAnswer=true and part is complete', () => {
    render(<ReasoningPart part={makePart({ state: 'done' })} streamingAnswer />);
    expect(screen.getByTestId('reasoning-preview')).toHaveTextContent('生成中...');
  });

  it('omits the suffix when part.state is complete and no streamingAnswer', () => {
    render(<ReasoningPart part={makePart({ state: 'done' })} />);
    const preview = screen.getByTestId('reasoning-preview');
    expect(preview).toHaveTextContent('推理过程');
    expect(preview.textContent).not.toMatch(/推理中|生成中/);
  });
});
