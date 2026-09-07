// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-07 Task 2 — FeedbackRow tests (FULL REWRITE of the B6-era
// placeholder suite per Pitfall 7: the old file asserted the toast
// PLACEHOLDER; this file asserts the REAL API state machine).
//
// FeedbackRow now POSTs to the frozen wire contracts
//   POST /api/feedback {session_id, turn_id, verdict, comment?}
//     → 201 {status:"recorded", verdict} | 200 {status:"already_recorded", verdict} | 4xx {error}
//   POST /api/tickets  {session_id, turn_id, description}
//     → 201 {ticket_number:"TKT-n", dispatch_status:"none"|"queued"}
//       | 200 {status:"already_exists", ticket_number} | 4xx {error}
// and the lock is SERVER-DRIVEN (D-29-02/04): 2xx locks, failure leaves
// the row retryable. All copy is verbatim from 29-UI-SPEC §Copywriting
// Contract. messageId NEVER enters a request body (Pitfall 7).
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import FeedbackRow, { type FeedbackRowProps } from '../components/chat/parts/FeedbackRow';

type FetchMock = ReturnType<typeof vi.fn>;

interface ToastDetail {
  kind?: string;
  message: string;
}

function captureToasts() {
  const toasts: ToastDetail[] = [];
  const handler = (e: Event) => {
    toasts.push((e as CustomEvent<ToastDetail>).detail);
  };
  window.addEventListener('sf-toast', handler);
  return {
    toasts,
    stop: () => window.removeEventListener('sf-toast', handler),
  };
}

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

function stubFetch(impl: (url: string, init?: RequestInit) => Promise<Response>): FetchMock {
  const mock = vi.fn(impl);
  vi.stubGlobal('fetch', mock);
  return mock;
}

function bodiesFor(mock: FetchMock, path: string): Array<Record<string, unknown>> {
  return mock.mock.calls
    .filter(([url]) => String(url).includes(path))
    .map(([, init]) => JSON.parse((init as RequestInit).body as string) as Record<string, unknown>);
}

const BASE: FeedbackRowProps = {
  messageId: 'm1',
  turnId: 41,
  votedVerdict: null,
  sessionId: 'sess-1234567890abcd',
  chunkIds: ['3', '7'],
  messageText: '结论：由部门负责人审批。',
};

function renderRow(overrides: Partial<FeedbackRowProps> = {}) {
  render(<FeedbackRow {...BASE} {...overrides} />);
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('FeedbackRow', () => {
  it('renders the question, 3 votes, and 创建工单 when the turn is votable', () => {
    const { stop } = captureToasts();
    renderRow();
    const row = screen.getByTestId('feedback-row');
    expect(row).toHaveTextContent('这个回答有用吗？');
    expect(row).toHaveAttribute('data-turn-id', '41');
    expect(row).toHaveAttribute('data-message-id', 'm1');
    expect(row).toHaveAttribute('data-voted', '');
    for (const testid of ['feedback-helpful', 'feedback-unhelpful', 'feedback-outdated', 'feedback-ticket']) {
      expect(screen.getByTestId(testid)).toBeEnabled();
    }
    expect(screen.getByTestId('feedback-helpful')).toHaveAttribute('aria-pressed', 'false');
    stop();
  });

  it('turnId null → honest disabled degradation (not hidden) with the verbatim title', () => {
    renderRow({ turnId: null });
    const row = screen.getByTestId('feedback-row');
    expect(row).toHaveAttribute('data-turn-id', '');
    for (const testid of ['feedback-helpful', 'feedback-unhelpful', 'feedback-outdated', 'feedback-ticket']) {
      const btn = screen.getByTestId(testid);
      expect(btn).toBeDisabled();
      expect(btn).toHaveAttribute('title', '该消息缺少服务端轮次标识，暂时无法反馈');
    }
  });

  it('helpful vote POSTs the exact body (no comment key, NO messageId) and locks on 201 recorded', async () => {
    const { toasts, stop } = captureToasts();
    const mock = stubFetch((url) => {
      expect(String(url)).toBe('/api/feedback');
      return Promise.resolve(jsonResponse(201, { status: 'recorded', verdict: 'helpful' }));
    });
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-helpful'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    // Body shape is the frozen contract — session_id / turn_id / verdict
    // ONLY. messageId never rides the request (Pitfall 7).
    const bodies = bodiesFor(mock, '/api/feedback');
    expect(bodies).toHaveLength(1);
    expect(Object.keys(bodies[0]).sort()).toEqual(['session_id', 'turn_id', 'verdict']);
    expect(bodies[0]).toEqual({ session_id: 'sess-1234567890abcd', turn_id: 41, verdict: 'helpful' });

    // Server truth drives the lock: active pill, losers disabled.
    const helpful = screen.getByTestId('feedback-helpful');
    expect(helpful).toHaveAttribute('aria-pressed', 'true');
    expect(helpful).toHaveClass('feedback-row-btn-active');
    expect(screen.getByTestId('feedback-unhelpful')).toBeDisabled();
    expect(screen.getByTestId('feedback-outdated')).toBeDisabled();
    expect(screen.getByTestId('feedback-row')).toHaveAttribute('data-voted', 'helpful');
    // 创建工单 is independent of the vote group.
    expect(screen.getByTestId('feedback-ticket')).toBeEnabled();
    expect(toasts[0]).toEqual({ kind: 'success', message: '已记录反馈：有帮助' });
    stop();
  });

  it('unhelpful success toast uses the verbatim copy', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() => Promise.resolve(jsonResponse(201, { status: 'recorded', verdict: 'unhelpful' })));
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));
    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({ kind: 'success', message: '已记录反馈：没帮助' });
    stop();
  });

  it('outdated expands the comment box; a note rides body.comment and the box collapses with 已附说明', async () => {
    const { toasts, stop } = captureToasts();
    const mock = stubFetch(() =>
      Promise.resolve(jsonResponse(201, { status: 'recorded', verdict: 'outdated' })),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-outdated'));

    // Selection expands the inline box (not a modal).
    const box = await screen.findByTestId('feedback-comment-box');
    expect(box).toBeInTheDocument();
    const input = screen.getByTestId('feedback-comment-input') as HTMLTextAreaElement;
    expect(input).toHaveAttribute(
      'placeholder',
      '说明（选填）：例如哪条知识过时了、正确内容是什么',
    );
    fireEvent.change(input, { target: { value: '差旅补贴政策已于 2026-06 调整' } });
    fireEvent.click(screen.getByTestId('feedback-comment-submit'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({
      kind: 'success',
      message: '已记录反馈：已标记知识可能过时，将进入人工复核',
    });
    const bodies = bodiesFor(mock, '/api/feedback');
    expect(bodies[0]).toEqual({
      session_id: 'sess-1234567890abcd',
      turn_id: 41,
      verdict: 'outdated',
      comment: '差旅补贴政策已于 2026-06 调整',
    });
    // Box collapses after the lock; the locked pill carries 已附说明.
    expect(screen.queryByTestId('feedback-comment-box')).toBeNull();
    expect(screen.getByTestId('feedback-outdated').getAttribute('aria-label')).toContain('已附说明');
    stop();
  });

  it('outdated submits cleanly with an EMPTY comment (no 已附说明)', async () => {
    const { toasts, stop } = captureToasts();
    const mock = stubFetch(() =>
      Promise.resolve(jsonResponse(201, { status: 'recorded', verdict: 'outdated' })),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-outdated'));
    await screen.findByTestId('feedback-comment-box');
    fireEvent.click(screen.getByTestId('feedback-comment-submit'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    const bodies = bodiesFor(mock, '/api/feedback');
    expect(bodies[0].comment).toBe('');
    // Empty note = nothing attached → no 已附说明 aria-label at all.
    expect(screen.getByTestId('feedback-outdated')).not.toHaveAttribute('aria-label');
    stop();
  });

  it('200 already_recorded renders the SERVER verdict as the active pill + info toast', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() =>
      Promise.resolve(jsonResponse(200, { status: 'already_recorded', verdict: 'unhelpful' })),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-helpful'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    // Idempotent replay is not an error — identical visual to recorded.
    const unhelpful = screen.getByTestId('feedback-unhelpful');
    expect(unhelpful).toHaveAttribute('aria-pressed', 'true');
    expect(unhelpful).toHaveClass('feedback-row-btn-active');
    expect(screen.getByTestId('feedback-helpful')).not.toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('feedback-helpful')).toBeDisabled();
    expect(screen.getByTestId('feedback-row')).toHaveAttribute('data-voted', 'unhelpful');
    expect(toasts[0]).toEqual({ kind: 'info', message: '该消息已反馈过：没帮助' });
    stop();
  });

  it('5xx failure does NOT lock (row stays retryable) and the error toast carries the real reason', async () => {
    const { toasts, stop } = captureToasts();
    let attempt = 0;
    const mock = stubFetch(() => {
      attempt += 1;
      if (attempt === 1) {
        return Promise.resolve(jsonResponse(500, { error: 'store 不可用' }));
      }
      return Promise.resolve(jsonResponse(201, { status: 'recorded', verdict: 'unhelpful' }));
    });
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({ kind: 'error', message: '反馈提交失败：store 不可用，请稍后重试' });
    // No lock: every pill enabled, aria-pressed all false.
    expect(screen.getByTestId('feedback-unhelpful')).toBeEnabled();
    expect(screen.getByTestId('feedback-helpful')).toBeEnabled();
    expect(screen.getByTestId('feedback-helpful')).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByTestId('feedback-unhelpful')).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByTestId('feedback-outdated')).toHaveAttribute('aria-pressed', 'false');

    // Retry succeeds — no auto-retry happened (exactly 2 calls: 1 fail + 1 manual).
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));
    await waitFor(() => expect(toasts).toHaveLength(2));
    expect(toasts[1]).toEqual({ kind: 'success', message: '已记录反馈：没帮助' });
    expect(mock.mock.calls).toHaveLength(2);
    stop();
  });

  it('network failure surfaces the real reason and keeps the row unlocked', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() => Promise.reject(new Error('网络中断')));
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-helpful'));
    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({ kind: 'error', message: '反馈提交失败：网络中断，请稍后重试' });
    expect(screen.getByTestId('feedback-helpful')).toBeEnabled();
    expect(screen.getByTestId('feedback-row')).toHaveAttribute('data-voted', '');
    stop();
  });

  it('falls back to the HTTP status text when the error body is not JSON', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() =>
      Promise.resolve({
        ok: false,
        status: 503,
        json: () => Promise.reject(new Error('not json')),
      } as Response),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-outdated'));
    await screen.findByTestId('feedback-comment-box');
    fireEvent.click(screen.getByTestId('feedback-comment-submit'));
    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0].message).toBe('反馈提交失败：HTTP 503，请稍后重试');
    stop();
  });

  it('votedVerdict prop (history echo) renders the locked state with zero fetches', () => {
    const { stop } = captureToasts();
    const mock = stubFetch(() => Promise.resolve(jsonResponse(201, { status: 'recorded' })));
    renderRow({ votedVerdict: 'helpful' });
    const helpful = screen.getByTestId('feedback-helpful');
    expect(helpful).toHaveAttribute('aria-pressed', 'true');
    expect(helpful).toHaveClass('feedback-row-btn-active');
    expect(screen.getByTestId('feedback-unhelpful')).toBeDisabled();
    expect(screen.getByTestId('feedback-outdated')).toBeDisabled();
    expect(screen.getByTestId('feedback-row')).toHaveAttribute('data-voted', 'helpful');
    fireEvent.click(screen.getByTestId('feedback-unhelpful'));
    expect(mock.mock.calls).toHaveLength(0);
    stop();
  });

  // --- Ticket form (D-29-13 / D-29-14) ---

  it('创建工单 expands the inline form with label, prefill, and the mono reference strip', () => {
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    const form = screen.getByTestId('feedback-ticket-form');
    expect(form).toBeInTheDocument();

    const label = screen.getByText('问题描述（必填）');
    const textarea = screen.getByTestId('feedback-ticket-description') as HTMLTextAreaElement;
    expect(label.getAttribute('for')).toBe(textarea.id);
    expect(textarea.value).toBe('结论：由部门负责人审批。');

    const refs = screen.getByTestId('feedback-ticket-refs');
    expect(refs).toHaveTextContent('会话 7890abcd');
    expect(refs).toHaveTextContent('轮次 #41');
    expect(refs).toHaveTextContent('3');
    expect(refs).toHaveTextContent('7');

    expect(screen.getByTestId('feedback-ticket-submit')).toHaveTextContent('提交工单');
    expect(screen.getByTestId('feedback-ticket-cancel')).toHaveTextContent('收起表单');
  });

  it('ticket prefill truncates the message summary to 200 chars', () => {
    renderRow({ messageText: '字'.repeat(300) });
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    const textarea = screen.getByTestId('feedback-ticket-description') as HTMLTextAreaElement;
    expect(textarea.value).toHaveLength(200);
  });

  it('shows 无引用 when the message carries no citations', () => {
    renderRow({ chunkIds: [] });
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    expect(screen.getByTestId('feedback-ticket-refs')).toHaveTextContent('无引用');
  });

  it('ticket 201 dispatch_status=none renders the honest 留档 result + toast; trigger becomes 已建单', async () => {
    const { toasts, stop } = captureToasts();
    const mock = stubFetch((url) => {
      expect(String(url)).toBe('/api/tickets');
      return Promise.resolve(jsonResponse(201, { ticket_number: 'TKT-17', dispatch_status: 'none' }));
    });
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    fireEvent.click(screen.getByTestId('feedback-ticket-submit'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    const bodies = bodiesFor(mock, '/api/tickets');
    expect(bodies).toHaveLength(1);
    expect(Object.keys(bodies[0]).sort()).toEqual(['description', 'session_id', 'turn_id']);
    expect(bodies[0]).toEqual({
      session_id: 'sess-1234567890abcd',
      turn_id: 41,
      description: '结论：由部门负责人审批。',
    });
    expect(screen.getByTestId('feedback-ticket-result')).toHaveTextContent(
      '✓ 工单已受理 · TKT-17 — 未配置外发渠道，工单已留档',
    );
    expect(toasts[0]).toEqual({
      kind: 'success',
      message: '工单已受理（TKT-17）：未配置外发渠道，工单已留档',
    });
    // Form collapsed to the result line; the trigger is honestly spent.
    expect(screen.queryByTestId('feedback-ticket-form')).toBeNull();
    const trigger = screen.getByTestId('feedback-ticket');
    expect(trigger).toBeDisabled();
    expect(trigger).toHaveTextContent('已建单');
    stop();
  });

  it('ticket 201 dispatch_status=queued renders the 排队外发 result + toast', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() =>
      Promise.resolve(jsonResponse(201, { ticket_number: 'TKT-18', dispatch_status: 'queued' })),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    fireEvent.click(screen.getByTestId('feedback-ticket-submit'));
    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(screen.getByTestId('feedback-ticket-result')).toHaveTextContent(
      '✓ 工单已受理 · TKT-18 — 正在排队外发',
    );
    expect(toasts[0]).toEqual({
      kind: 'success',
      message: '工单已受理（TKT-18）：正在排队外发',
    });
    stop();
  });

  it('ticket 200 already_exists renders the existing number + info toast (一消息一单)', async () => {
    const { toasts, stop } = captureToasts();
    stubFetch(() =>
      Promise.resolve(jsonResponse(200, { status: 'already_exists', ticket_number: 'TKT-9' })),
    );
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    fireEvent.click(screen.getByTestId('feedback-ticket-submit'));
    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(screen.getByTestId('feedback-ticket-result')).toHaveTextContent('该消息已建过工单 · TKT-9');
    expect(toasts[0]).toEqual({ kind: 'info', message: '该消息已建过工单：TKT-9' });
    expect(screen.getByTestId('feedback-ticket')).toBeDisabled();
    expect(screen.getByTestId('feedback-ticket')).toHaveTextContent('已建单');
    stop();
  });

  it('ticket failure keeps the form open, editable, resubmittable + error toast with the real reason', async () => {
    const { toasts, stop } = captureToasts();
    let attempt = 0;
    stubFetch(() => {
      attempt += 1;
      if (attempt === 1) {
        return Promise.resolve(jsonResponse(500, { error: 'webhook 未配置' }));
      }
      return Promise.resolve(jsonResponse(201, { ticket_number: 'TKT-19', dispatch_status: 'none' }));
    });
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    const textarea = screen.getByTestId('feedback-ticket-description') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: '修订后的描述' } });
    fireEvent.click(screen.getByTestId('feedback-ticket-submit'));

    await waitFor(() => expect(toasts).toHaveLength(1));
    expect(toasts[0]).toEqual({
      kind: 'error',
      message: '工单提交失败：webhook 未配置，可修改后重试',
    });
    // Inline error inside the still-open form.
    expect(screen.getByTestId('feedback-ticket-error')).toHaveTextContent(
      '工单提交失败：webhook 未配置，可修改后重试',
    );
    expect(screen.getByTestId('feedback-ticket-form')).toBeInTheDocument();
    // Editable + resubmittable.
    fireEvent.change(screen.getByTestId('feedback-ticket-description'), {
      target: { value: '修订后的描述 v2' },
    });
    fireEvent.click(screen.getByTestId('feedback-ticket-submit'));
    await waitFor(() => expect(toasts).toHaveLength(2));
    expect(screen.getByTestId('feedback-ticket-result')).toHaveTextContent(
      '✓ 工单已受理 · TKT-19 — 未配置外发渠道，工单已留档',
    );
    stop();
  });

  it('提交工单 is disabled while the description is empty; 收起表单 collapses without submitting', async () => {
    const { stop } = captureToasts();
    const mock = stubFetch(() => Promise.resolve(jsonResponse(201, { ticket_number: 'TKT-1' })));
    renderRow();
    fireEvent.click(screen.getByTestId('feedback-ticket'));
    const textarea = screen.getByTestId('feedback-ticket-description') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: '' } });
    expect(screen.getByTestId('feedback-ticket-submit')).toBeDisabled();
    fireEvent.click(screen.getByTestId('feedback-ticket-cancel'));
    expect(screen.queryByTestId('feedback-ticket-form')).toBeNull();
    expect(mock.mock.calls).toHaveLength(0);
    stop();
  });
});
