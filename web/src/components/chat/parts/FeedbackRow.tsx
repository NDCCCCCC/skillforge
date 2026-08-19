// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// FeedbackRow — Open Design redesign B6. Per-assistant-reply feedback
// strip at the end of the message body:
//   - "这个回答有用吗？" + THREE single-select votes (有帮助 /
//     没帮助 / 报告知识过时). The first click LOCKS the choice
//     (re-clicking another option is ignored — one verdict per reply).
//   - A divider + an INDEPENDENT 创建工单 trigger (fires alongside or
//     after a vote; it is not part of the single-select group).
//
// Actions are toast PLACEHOLDERS: every click dispatches a window-level
// 'sf-toast' CustomEvent carrying a human message (the same
// window-CustomEvent pattern as 'wizard-success'). No listener ships
// in B6 — the backend ticketing/vote protocol is out of scope here;
// wiring the listener + API call lands with the backend contract.
import { useState } from 'react';
import { FileWarning, ThumbsDown, ThumbsUp, Ticket } from 'lucide-react';
import { cn } from '../../../lib/cn';

type Vote = 'helpful' | 'unhelpful' | 'outdated';

const VOTE_META: Array<{ key: Vote; testid: string; label: string; Icon: typeof ThumbsUp }> = [
  { key: 'helpful', testid: 'feedback-helpful', label: '有帮助', Icon: ThumbsUp },
  { key: 'unhelpful', testid: 'feedback-unhelpful', label: '没帮助', Icon: ThumbsDown },
  { key: 'outdated', testid: 'feedback-outdated', label: '报告知识过时', Icon: FileWarning },
];

const TOAST_TEXT: Record<Vote | 'ticket', string> = {
  helpful: '感谢反馈：已记录「有帮助」',
  unhelpful: '感谢反馈：已记录「没帮助」',
  outdated: '感谢反馈：已标记知识可能过时',
  ticket: '创建工单：功能即将上线（占位）',
};

function fireToast(kind: Vote | 'ticket') {
  window.dispatchEvent(new CustomEvent('sf-toast', { detail: { kind, message: TOAST_TEXT[kind] } }));
}

export interface FeedbackRowProps {
  /** The message this row is attached to (carried in the toast detail). */
  messageId: string;
}

export default function FeedbackRow({ messageId }: FeedbackRowProps) {
  // null = not yet voted; once set, the vote is locked (single-select).
  const [vote, setVote] = useState<Vote | null>(null);

  function cast(candidate: Vote) {
    if (vote !== null) return; // locked after the first choice
    setVote(candidate);
    fireToast(candidate);
  }

  return (
    <div className="feedback-row" data-testid="feedback-row" data-message-id={messageId} data-voted={vote ?? ''}>
      <span className="feedback-row-q">这个回答有用吗？</span>
      {VOTE_META.map(({ key, testid, label, Icon }) => (
        <button
          key={key}
          type="button"
          data-testid={testid}
          onClick={() => cast(key)}
          disabled={vote !== null && vote !== key}
          aria-pressed={vote === key}
          className={cn('feedback-row-btn', vote === key && 'feedback-row-btn-active')}
        >
          <Icon className="h-3 w-3" />
          <span>{label}</span>
        </button>
      ))}
      <span className="feedback-row-divider" aria-hidden />
      <button
        type="button"
        data-testid="feedback-ticket"
        onClick={() => fireToast('ticket')}
        className="feedback-row-btn"
      >
        <Ticket className="h-3 w-3" />
        <span>创建工单</span>
      </button>
    </div>
  );
}
