// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DraftStreamChip 的新形态（流式预览区）测试 —— 与 DraftStreamChip.test.tsx
// 的既有契约测试互补：这里钉的是「边生成边显示」的视觉证据，不改旧契约。
//
// 钉死的不变量：
//   1. pending 时末尾有闪烁光标 + aria-busy="true"（用户看得出还在写）。
//   2. verified 时光标消失（写完了，不再假装在打字）。
//   3. 正文超过 6000 字时只渲染尾部并给出折叠提示（预览不进整段 DOM）。
//   4. 正文容器是块级滚动区（多行显示），不再是单行胶囊。
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DraftStreamChip from './DraftStreamChip';

afterEach(cleanup);

describe('DraftStreamChip — live preview shape', () => {
  it('shows a blinking caret and aria-busy while status=pending', () => {
    render(<DraftStreamChip status="pending" draftText="依据制度" />);
    expect(screen.getByTestId('draft-caret')).toBeInTheDocument();
    expect(screen.getByTestId('draft-chip')).toHaveAttribute('aria-busy', 'true');
  });

  it('drops the caret once status=verified', () => {
    render(<DraftStreamChip status="verified" draftText="依据制度第3条" />);
    expect(screen.queryByTestId('draft-caret')).toBeNull();
    expect(screen.getByTestId('draft-chip')).toHaveAttribute('aria-busy', 'false');
  });

  it('renders the tail only (with a fold hint) when the draft is very long', () => {
    const head = 'HEAD-MARKER';
    const longText = head + 'x'.repeat(7000);
    render(<DraftStreamChip status="pending" draftText={longText} />);
    const body = screen.getByTestId('draft-text');
    // 只保留尾部 6000 字，开头的旧内容不进 DOM。
    expect(body.textContent ?? '').toHaveLength(6000);
    expect(body.textContent ?? '').not.toContain(head);
    expect(screen.getByTestId('draft-scroll').textContent).toContain('仅显示最近 6000 字');
  });

  it('exposes the streamed body as a scroll region', () => {
    render(<DraftStreamChip status="pending" draftText={'第一行\n第二行'} />);
    const region = screen.getByTestId('draft-scroll');
    expect(region.className).toContain('draft-chip-body');
  });
});
