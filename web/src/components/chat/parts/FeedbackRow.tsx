// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// FeedbackRow — Plan 29-07 Task 2 (FBK-04 + TKT-03, chat side).
//
// Per-assistant-reply feedback strip at the end of the message body.
// The B6-era placeholder (fireToast on every click, no listener in the
// app) is replaced by the REAL API state machine against the frozen
// wire contracts (29-05 / 29-06):
//
//   POST /api/feedback {session_id, turn_id, verdict, comment?}
//     → 201 {status:"recorded", verdict}
//     | 200 {status:"already_recorded", verdict}
//     | 4xx {error}
//   POST /api/tickets {session_id, turn_id, description}
//     → 201 {ticket_number:"TKT-n", dispatch_status:"none"|"queued"}
//     | 200 {status:"already_exists", ticket_number}
//     | 4xx {error}
//
// Trust semantics (CONTEXT D-29-02/04/06/13/14 + UI-SPEC §Honesty):
//   - The lock is SERVER-DRIVEN. FE local state is a cache, not truth —
//     `votedVerdict` (history echo via the data-turn part) can lock the
//     row on session restore. A failed request NEVER locks (D-29-04);
//     there is no auto-retry. Idempotent replays are not errors
//     (already_recorded / already_exists render like successes with an
//     info toast).
//   - outdated expands an inline OPTIONAL note (D-29-06); empty note
//     submits cleanly. The note rides body.comment.
//   - 创建工单 expands an inline form (not a modal, D-29-13) prefilled
//     with the message summary (≤200 chars) and a read-only mono
//     reference strip; the 4 dispatch states render verbatim honest copy
//     (D-29-14 — provider=none says 留档, never 已发送).
//   - turnId == null → everything disabled with the honest title, never
//     hidden and never a fabricated id (T-29-06).
//
// Wire discipline (Pitfall 7): `messageId` exists ONLY as the
// data-message-id DOM attribute (tests/keys). The request bodies carry
// session_id / turn_id / verdict / comment (feedback) and session_id /
// turn_id / description (tickets) — nothing else. chunkIds are rendered
// read-only from the verdict citations and never reported back: the
// server self-attributes from the audit row (D-29-05 / T-29-04).
//
// Rendering safety (T-29-09): server error strings and the user's note
// flow through React JSX text nodes only — the raw-HTML escape hatch is
// never used in this file.
//
// All user-facing copy is verbatim from 29-UI-SPEC §Copywriting Contract.
import { useState } from 'react';
import { FileWarning, ThumbsDown, ThumbsUp, Ticket } from 'lucide-react';
import { Button } from '../../ui/button';
import { cn } from '../../../lib/cn';

export type Vote = 'helpful' | 'unhelpful' | 'outdated';

export function isVote(value: unknown): value is Vote {
  return value === 'helpful' || value === 'unhelpful' || value === 'outdated';
}

const VOTE_META: Array<{ key: Vote; testid: string; label: string; Icon: typeof ThumbsUp }> = [
  { key: 'helpful', testid: 'feedback-helpful', label: '有帮助', Icon: ThumbsUp },
  { key: 'unhelpful', testid: 'feedback-unhelpful', label: '没帮助', Icon: ThumbsDown },
  { key: 'outdated', testid: 'feedback-outdated', label: '报告知识过时', Icon: FileWarning },
];

const VOTE_LABEL: Record<Vote, string> = {
  helpful: '有帮助',
  unhelpful: '没帮助',
  outdated: '报告知识过时',
};

/** Success toast copy — verbatim per UI-SPEC §Copywriting Contract. */
const VOTE_RECORDED_TOAST: Record<Vote, string> = {
  helpful: '已记录反馈：有帮助',
  unhelpful: '已记录反馈：没帮助',
  outdated: '已记录反馈：已标记知识可能过时，将进入人工复核',
};

const DISABLED_TITLE = '该消息缺少服务端轮次标识，暂时无法反馈';
const COMMENT_PLACEHOLDER = '说明（选填）：例如哪条知识过时了、正确内容是什么';
const TICKET_MAX_DESCRIPTION_CHARS = 200;

type ToastKind = 'success' | 'error' | 'info';

function toast(kind: ToastKind, message: string) {
  // Consumed by the SfToast viewport (29-07 Task 1; admin branch in 29-08).
  window.dispatchEvent(new CustomEvent('sf-toast', { detail: { kind, message } }));
}

/** Real reason extraction: body.error wins, HTTP status text is the
 *  fallback (UI-SPEC copy contract: 反馈提交失败：{真实原因}…). */
function extractReason(status: number, data: unknown): string {
  if (data && typeof data === 'object') {
    const error = (data as { error?: unknown }).error;
    if (typeof error === 'string' && error.trim() !== '') return error;
  }
  return `HTTP ${status}`;
}

interface PostResult {
  status: number;
  data: unknown;
  /** '' when 2xx; body.error / HTTP status / network message otherwise. */
  reason: string;
  networkError: boolean;
}

/** Public endpoints — raw fetch, NO X-Admin-Token (unlike the admin
 *  panels' KnowledgeBaseList pattern). */
async function postJson(url: string, body: unknown): Promise<PostResult> {
  let resp: Response;
  try {
    resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  } catch (err) {
    return {
      status: 0,
      data: null,
      reason: err instanceof Error ? err.message : String(err),
      networkError: true,
    };
  }
  let data: unknown = null;
  try {
    data = await resp.json();
  } catch {
    data = null; // non-JSON body → reason falls back to the HTTP status
  }
  return {
    status: resp.status,
    data,
    reason: resp.ok ? '' : extractReason(resp.status, data),
    networkError: false,
  };
}

interface TicketResult {
  kind: 'created' | 'exists';
  number: string;
  /** dispatch_status of the 201 created branch; null for already_exists. */
  status: 'none' | 'queued' | null;
}

export interface FeedbackRowProps {
  /** DOM-only identity (data-message-id). NEVER sent to the API. */
  messageId: string;
  /** session_turn.id from the data-turn part; null → honest disabled row. */
  turnId: number | null;
  /** Server truth echoed by the history endpoint (data-turn part). */
  votedVerdict: Vote | null;
  sessionId: string;
  /** Read-only citation chunk ids for the ticket reference strip. */
  chunkIds: string[];
  /** Aggregated chat text (ticket description prefill source). */
  messageText: string;
}

export default function FeedbackRow({
  messageId,
  turnId,
  votedVerdict,
  sessionId,
  chunkIds,
  messageText,
}: FeedbackRowProps) {
  // Server-truth lock (cache only — the DB UNIQUE constraint is the real
  // idempotency guard). Seeded from the history echo when present.
  const [locked, setLocked] = useState<Vote | null>(votedVerdict);
  const [hasComment, setHasComment] = useState(false);
  const [submittingVote, setSubmittingVote] = useState<Vote | null>(null);
  const [commentOpen, setCommentOpen] = useState(false);
  const [comment, setComment] = useState('');
  const [ticketOpen, setTicketOpen] = useState(false);
  const [ticketDraft, setTicketDraft] = useState(() =>
    messageText.slice(0, TICKET_MAX_DESCRIPTION_CHARS),
  );
  const [ticketSubmitting, setTicketSubmitting] = useState(false);
  const [ticketCreated, setTicketCreated] = useState(false);
  const [ticketResult, setTicketResult] = useState<TicketResult | null>(null);
  const [ticketError, setTicketError] = useState<string | null>(null);

  const votable = turnId !== null;
  const canVote = votable && locked === null && submittingVote === null;

  async function submitVote(verdict: Vote, note: string) {
    if (!canVote) return;
    setSubmittingVote(verdict);
    // Body carries ONLY the contract fields — no messageId (Pitfall 7).
    const body: Record<string, unknown> = {
      session_id: sessionId,
      turn_id: turnId,
      verdict,
    };
    if (verdict === 'outdated') body.comment = note;
    const res = await postJson('/api/feedback', body);
    const payload = res.data as { status?: unknown; verdict?: unknown } | null;
    if (!res.networkError && res.status >= 200 && res.status < 300 && payload?.status === 'recorded') {
      setLocked(verdict);
      setHasComment(verdict === 'outdated' && note.trim() !== '');
      setCommentOpen(false);
      toast('success', VOTE_RECORDED_TOAST[verdict]);
    } else if (!res.networkError && res.status === 200 && payload?.status === 'already_recorded') {
      // Idempotent replay: render the SERVER verdict, never an error.
      const served = isVote(payload.verdict) ? payload.verdict : verdict;
      setLocked(served);
      setCommentOpen(false);
      toast('info', `该消息已反馈过：${VOTE_LABEL[served]}`);
    } else {
      // Failure → NO lock (D-29-04), no auto-retry; row stays retryable.
      toast('error', `反馈提交失败：${res.reason}，请稍后重试`);
    }
    setSubmittingVote(null);
  }

  async function submitTicket() {
    if (turnId === null || ticketSubmitting || ticketCreated) return;
    const description = ticketDraft.trim();
    if (description === '') return;
    setTicketSubmitting(true);
    setTicketError(null);
    const res = await postJson('/api/tickets', { session_id: sessionId, turn_id: turnId, description });
    const payload = res.data as
      | { ticket_number?: unknown; dispatch_status?: unknown; status?: unknown }
      | null;
    if (
      !res.networkError &&
      res.status === 201 &&
      typeof payload?.ticket_number === 'string'
    ) {
      const status = payload.dispatch_status === 'queued' ? 'queued' : 'none';
      setTicketResult({ kind: 'created', number: payload.ticket_number, status });
      setTicketCreated(true);
      setTicketOpen(false);
      toast(
        'success',
        status === 'queued'
          ? `工单已受理（${payload.ticket_number}）：正在排队外发`
          : `工单已受理（${payload.ticket_number}）：未配置外发渠道，工单已留档`,
      );
    } else if (
      !res.networkError &&
      res.status === 200 &&
      payload?.status === 'already_exists' &&
      typeof payload.ticket_number === 'string'
    ) {
      setTicketResult({ kind: 'exists', number: payload.ticket_number, status: null });
      setTicketCreated(true);
      setTicketOpen(false);
      toast('info', `该消息已建过工单：${payload.ticket_number}`);
    } else {
      // Inline error + toast; the form stays open, editable, resubmittable.
      setTicketError(res.reason);
      toast('error', `工单提交失败：${res.reason}，可修改后重试`);
    }
    setTicketSubmitting(false);
  }

  const refsId = `ticket-refs-${messageId}`;
  const descriptionId = `ticket-desc-${messageId}`;
  // Session 短 id for the reference strip (display only — the full id
  // rides the request body).
  const shortSessionId = sessionId.slice(-8);

  return (
    <div
      className="feedback-row"
      data-testid="feedback-row"
      data-message-id={messageId}
      data-turn-id={turnId ?? ''}
      data-voted={locked ?? ''}
    >
      <div role="group" aria-label="回答反馈" className="flex flex-wrap items-center gap-1">
        <span className="feedback-row-q">这个回答有用吗？</span>
        {VOTE_META.map(({ key, testid, label, Icon }) => {
          const isActive = locked === key;
          const isSubmittingThis = submittingVote === key;
          return (
            <button
              key={key}
              type="button"
              data-testid={testid}
              onClick={() => {
                // 报告知识过时 expands the optional note box first
                // (D-29-06); 提交说明 posts the vote.
                if (key === 'outdated' && canVote) {
                  setCommentOpen(true);
                  return;
                }
                void submitVote(key, comment);
              }}
              disabled={!canVote && !isActive}
              aria-pressed={isActive}
              aria-expanded={key === 'outdated' ? commentOpen : undefined}
              aria-label={isActive && hasComment ? `${label}（已附说明）` : undefined}
              title={!votable ? DISABLED_TITLE : undefined}
              style={isSubmittingThis ? { opacity: 0.6 } : undefined}
              className={cn('feedback-row-btn', isActive && 'feedback-row-btn-active')}
            >
              <Icon className="h-3 w-3" />
              <span>{label}</span>
            </button>
          );
        })}
      </div>
      <span className="feedback-row-divider" aria-hidden />
      <button
        type="button"
        data-testid="feedback-ticket"
        onClick={() => setTicketOpen((open) => !open)}
        disabled={!votable || ticketCreated}
        title={!votable ? DISABLED_TITLE : undefined}
        className="feedback-row-btn"
      >
        <Ticket className="h-3 w-3" />
        <span>{ticketCreated ? '已建单' : '创建工单'}</span>
      </button>

      {/* D-29-06 — outdated optional note (empty submits cleanly). */}
      {commentOpen && (
        <div
          className="mt-2 flex w-full flex-wrap items-center gap-2"
          data-testid="feedback-comment-box"
        >
          <textarea
            data-testid="feedback-comment-input"
            aria-label="说明（选填）"
            rows={1}
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder={COMMENT_PLACEHOLDER}
            className="min-w-0 flex-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs text-text placeholder:text-text-muted"
          />
          <Button
            type="button"
            variant="secondary"
            size="sm"
            data-testid="feedback-comment-submit"
            disabled={submittingVote !== null}
            onClick={() => void submitVote('outdated', comment)}
          >
            提交说明
          </Button>
        </div>
      )}

      {/* D-29-13 — inline ticket form (never a modal, never wider than
          the message column: w-full inside .feedback-row). */}
      {ticketOpen && (
        <div
          className="mt-2 w-full rounded-md border border-border bg-surface-2 p-3"
          data-testid="feedback-ticket-form"
        >
          <label
            htmlFor={descriptionId}
            className="block text-xs font-semibold text-text"
          >
            问题描述（必填）
          </label>
          <textarea
            id={descriptionId}
            data-testid="feedback-ticket-description"
            aria-describedby={refsId}
            rows={3}
            value={ticketDraft}
            onChange={(e) => setTicketDraft(e.target.value)}
            className="mt-1 w-full rounded-md border border-border bg-surface px-2 py-1 text-xs text-text"
          />
          {/* Read-only mono reference strip (T-29-04: display-only data
              the client already has; the POST carries no chunk ids). */}
          <div
            id={refsId}
            data-testid="feedback-ticket-refs"
            className="mt-2 font-mono text-xs text-text-muted"
          >
            会话 {shortSessionId} · 轮次 #{turnId} · 引用{' '}
            {chunkIds.length === 0 ? (
              <span>无引用</span>
            ) : (
              chunkIds.map((id) => (
                <span key={id} className="mr-1 rounded border border-border px-1">
                  {id}
                </span>
              ))
            )}
          </div>
          {ticketError && (
            <div
              data-testid="feedback-ticket-error"
              className="mt-2 text-xs text-status-non-compliant"
            >
              工单提交失败：{ticketError}，可修改后重试
            </div>
          )}
          <div className="mt-2 flex items-center gap-2">
            <Button
              type="button"
              variant="default"
              size="sm"
              data-testid="feedback-ticket-submit"
              disabled={ticketSubmitting || ticketDraft.trim() === ''}
              onClick={() => void submitTicket()}
            >
              提交工单
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              data-testid="feedback-ticket-cancel"
              onClick={() => setTicketOpen(false)}
            >
              收起表单
            </Button>
          </div>
        </div>
      )}

      {/* D-29-14 — honest terminal result line (4 states; the two
          failure presentations live in feedback-ticket-error + toast). */}
      {ticketResult && (
        <div
          data-testid="feedback-ticket-result"
          aria-live="polite"
          className="mt-2 w-full text-xs"
        >
          {ticketResult.kind === 'created' ? (
            <span className="text-status-compliant">
              ✓ 工单已受理 · {ticketResult.number} —{' '}
              {ticketResult.status === 'queued'
                ? '正在排队外发'
                : '未配置外发渠道，工单已留档'}
            </span>
          ) : (
            <span className="text-text-muted">
              该消息已建过工单 · {ticketResult.number}
            </span>
          )}
        </div>
      )}
    </div>
  );
}
