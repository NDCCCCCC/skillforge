// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Open Design redesign B6 — FeedbackRow tests.
//   1. Renders the question + 3 votes + the independent 创建工单.
//   2. A vote locks the single-select group (later clicks ignored,
//      losers disabled, winner highlighted + aria-pressed).
//   3. Each action dispatches the 'sf-toast' CustomEvent placeholder
//      with the message id + human text.
//   4. 创建工单 stays available AFTER a vote (independent trigger).
//   5. MessageBubble mounts the row at the end of every assistant body.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import type { UIMessage } from 'ai';
import FeedbackRow from '../components/chat/parts/FeedbackRow';
import { MessageBubble } from '../components/chat/MessageBubble';

afterEach(cleanup);

function lastToastDetail(events: Event[]): { kind: string; message: string } | undefined {
  const last = events[events.length - 1] as CustomEvent<{ kind: string; message: string }>;
  return last?.detail;
}

describe('FeedbackRow', () => {
  it('renders the question, 3 votes, and 创建工单', () => {
    render(<FeedbackRow messageId="m1" />);
    expect(screen.getByTestId('feedback-row')).toHaveTextContent('这个回答有用吗？');
    expect(screen.getByTestId('feedback-helpful')).toHaveTextContent('有帮助');
    expect(screen.getByTestId('feedback-unhelpful')).toHaveTextContent('没帮助');
    expect(screen.getByTestId('feedback-outdated')).toHaveTextContent('报告知识过时');
    expect(screen.getByTestId('feedback-ticket')).toHaveTextContent('创建工单');
  });

  it('locks the vote after the first click (single-select)', () => {
    render(<FeedbackRow messageId="m1" />);
    fireEvent.click(screen.getByTestId('feedback-helpful'));
    const helpful = screen.getByTestId('feedback-helpful');
    expect(helpful).toHaveAttribute('aria-pressed', 'true');
    // Losers are disabled and a later click changes nothing.
    expect(screen.getByTestId('feedback-unhelpful')).toBeDisabled();
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));
    expect(screen.getByTestId('feedback-helpful')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('feedback-unhelpful')).not.toHaveAttribute('aria-pressed', 'true');
  });

  it('dispatches an sf-toast CustomEvent per action', () => {
    const events: Event[] = [];
    const onToast = (e: Event) => events.push(e);
    window.addEventListener('sf-toast', onToast);
    render(<FeedbackRow messageId="m1" />);
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    window.removeEventListener('sf-toast', onToast);
    expect(events).toHaveLength(2);
    expect((events[0] as CustomEvent<{ message: string }>).detail.message).toContain('没帮助');
    // ticket is independent and still fires after the locked vote.
    expect(lastToastDetail(events)?.message).toContain('创建工单');
  });

  it('创建工单 stays enabled after a vote', () => {
    render(<FeedbackRow messageId="m1" />);
    fireEvent.click(screen.getByTestId('feedback-outdated'));
    expect(screen.getByTestId('feedback-ticket')).toBeEnabled();
  });

  it('MessageBubble renders the row at the end of an assistant body', () => {
    const message: UIMessage = {
      id: 'm9',
      role: 'assistant',
      parts: [{ type: 'text', text: '结论：由部门负责人审批。' }],
    };
    render(<MessageBubble message={message} />);
    const row = screen.getByTestId('feedback-row');
    expect(row).toHaveAttribute('data-message-id', 'm9');
    const body = row.closest('.message-assistant-body');
    const text = screen.getByText('结论：由部门负责人审批。');
    expect(body).toContainElement(text);
  });
});
