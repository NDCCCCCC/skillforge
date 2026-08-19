// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B4 — MarkdownMessage tests.
//   1. Plain text renders as a paragraph (streaming delta shape).
//   2. A fenced code block becomes a cmd-block: language label from
//      the info string, copy CTA wired to navigator.clipboard with a
//      2s 已复制 confirmation state.
//   3. Inline code keeps the chip styling context (renders <code>).
//   4. An UNCLOSED ``` fence (mid-stream truncation) renders as a
//      cmd-block without throwing.
//   5. GFM tables render (remark-gfm active).
//   6. javascript: URLs are sanitized by react-markdown's default url
//      transform — the rendered link never carries the dangerous href.
//   7. MessageBubble integration: the assistant branch renders the
//      Sparkles 方标 + the markdown body.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import MarkdownMessage from '../components/chat/MarkdownMessage';
import { MessageBubble } from '../components/chat/MessageBubble';

// navigator.clipboard is read-only in jsdom; defineProperty like
// RedlineCard.test does.
const writeText = vi.fn(async (_text: string) => undefined);
beforeEach(() => {
  const desc = { value: { writeText }, configurable: true };
  Object.defineProperty(navigator, 'clipboard', desc);
  writeText.mockClear();
});
afterEach(cleanup);

describe('MarkdownMessage', () => {
  it('renders plain text as a paragraph', () => {
    render(<MarkdownMessage text="结论：由部门负责人审批。" />);
    expect(screen.getByText('结论：由部门负责人审批。').tagName).toBe('P');
  });

  it('renders a fenced code block as a cmd-block with lang label + working copy', async () => {
    render(<MarkdownMessage text={'前置步骤：\n\n```bash\nnpm install\nnpm test\n```'} />);
    const block = screen.getByTestId('cmd-block');
    expect(block).toHaveAttribute('data-lang', 'bash');
    expect(block).toHaveTextContent('npm install');
    // No fenced markers leak into the visible text.
    expect(block.textContent).not.toContain('```');

    fireEvent.click(screen.getByTestId('cmd-block-copy'));
    // hast code blocks carry a trailing newline — copy verbatim.
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('npm install\nnpm test\n'));
    expect(screen.getByTestId('cmd-block-copy')).toHaveTextContent('已复制');
  });

  it('renders a fence with no language as data-lang="text"', () => {
    render(<MarkdownMessage text={'```\nplain\n```'} />);
    expect(screen.getByTestId('cmd-block')).toHaveAttribute('data-lang', 'text');
  });

  it('renders inline code as a styled <code> element', () => {
    render(<MarkdownMessage text={'运行 `sfctl reload` 后生效。'} />);
    const code = screen.getByText('sfctl reload');
    expect(code.tagName).toBe('CODE');
  });

  it('does not crash on an unclosed ``` fence (mid-stream truncation)', () => {
    expect(() => render(<MarkdownMessage text={'```bash\nnpm install'} />)).not.toThrow();
    expect(screen.getByTestId('cmd-block')).toHaveTextContent('npm install');
  });

  it('renders GFM tables via remark-gfm', () => {
    render(
      <MarkdownMessage text={'| 层级 | 目录 |\n| --- | --- |\n| 权威 | authority/ |'} />,
    );
    const table = document.querySelector('.markdown-body table');
    expect(table).not.toBeNull();
    expect(screen.getByText('authority/')).toBeInTheDocument();
  });

  it('sanitizes javascript: URLs (default url transform)', () => {
    render(<MarkdownMessage text={'[click](javascript:alert(1))'} />);
    const link = screen.getByText('click');
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('href')).not.toContain('javascript:');
  });

  it('MessageBubble assistant branch renders the Sparkles mark + markdown body', () => {
    const message: UIMessage = {
      id: 'm1',
      role: 'assistant',
      parts: [{ type: 'text', text: '**结论**：合规。' }],
    };
    render(<MessageBubble message={message} />);
    expect(screen.getByTestId('assistant-mark')).toBeInTheDocument();
    expect(screen.getByTestId('markdown-body')).toBeInTheDocument();
    const strong = screen.getByText('结论');
    expect(strong.tagName).toBe('STRONG');
  });
});
