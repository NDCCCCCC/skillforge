// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-06 Task 2 — ApprovalCard (HITL-01 / HITL-02 / HITL-07).
//
// Store-derived overlay at the chat-column interrupt position — NOT a
// message part (11-UI-SPEC §ApprovalCard: "the central UI/UX
// distinction of Phase 11"). The chat stream keeps flowing underneath;
// this card floats above it and derives 100% from the store's
// approval branch (D14 — the useChat stream is never touched).
//
// Visual states (6): pending + approved / denied / expired /
// auto-passed / failed (11-UI-SPEC §Card visual states). All copy is
// zh-CN per the 11-UI-SPEC copywriting contract. All colors bind to
// the existing theme.css tokens (zero new tokens).
//
// Decision flow (HITL-02):
//   批准一次        → POST {decision:'approve', frozen_args}
//   批准并记住      → scope radio (session|skill, D11-D3 binary) →
//                     POST {decision:'approve', frozen_args, scope}
//   拒绝            → inline prompt, reason >= 3 chars →
//                     POST {decision:'deny', reason}
// Edit-before-approve (HITL-07): 编辑参数 toggles a textarea over the
// FrozenArgs JSON; the edited value is sent as frozen_args on POST.
//
// POST body uses the BE snake_case wire shape (approvalPostRequest in
// internal/server/approval_api.go — the ground truth). The BE streams
// the resume events on the 200 response; the local terminal transition
// is driven by the HTTP status alone (200 → approved, 410 → expired,
// other → failed, D11-C3 fail-closed).

import { useCallback, useState } from 'react';
import type { JSX } from 'react';
import {
  AlertOctagon,
  CircleSlash,
  ScrollText,
  ShieldCheck,
  ShieldX,
  X,
} from 'lucide-react';
import { useAppStore } from '../store/app-store';
import type {
  ApprovalScope,
  JsonObject,
  PendingApproval,
  TerminalApproval,
} from '../store/app-store';

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

/** The "what" line is DERIVED from target + frozenArgs, never
 * free-form (11-UI-SPEC §Copywriting — template locked per target). */
function deriveWhatLine(target: string, frozenArgs: JsonObject): string {
  if (target === 'memory:facts') {
    const fact = String(
      frozenArgs['fact'] ?? frozenArgs['content'] ?? frozenArgs['text'] ?? '',
    );
    return `代理希望写入一条事实：「${fact}」`;
  }
  if (target === 'review_report') {
    const report = String(frozenArgs['report'] ?? frozenArgs['title'] ?? '');
    return `代理希望生成一份审查报告：「${report}」`;
  }
  return `代理希望执行 ${target}`;
}

/** Edited args win when edit mode is on AND the textarea parses;
 * on parse failure fall back to the original snapshot (defensive —
 * the BE re-validates via FrozenArgs byte-identity checks). */
function computeFrozenArgsForPost(
  approval: PendingApproval,
  editedJson: string,
  editMode: boolean,
): JsonObject {
  if (!editMode) return approval.frozenArgs;
  try {
    return JSON.parse(editedJson) as JsonObject;
  } catch {
    return approval.frozenArgs;
  }
}

/** WR-03 (18-REVIEW) — narrowed view of the report_publish frozen args
 * (the BE wire shape hitl.ReportPublishArgs). The structure summary
 * previously read `frozenArgs.sections` / `frozenArgs.templateVersion`
 * which DO NOT exist at the top level — sections live at
 * template.sections, and templateVersion is the server-derived content
 * hash the BE freezes into the args at Pass 1. Replaces the `as any`
 * casts that hid the mismatch from the compiler. */
interface ReportPublishFrozenArgs {
  period?: string;
  template?: {
    title?: string;
    sections?: Array<{ title?: string; kpis?: string[] }>;
  };
  slots?: Record<string, Record<string, string>>;
  skillId?: string;
  sessionId?: string;
  kpiCallIds?: number[];
  templateVersion?: string;
}

function asReportPublishArgs(frozenArgs: JsonObject): ReportPublishFrozenArgs {
  return frozenArgs as ReportPublishFrozenArgs;
}

/** POST the operator decision. Returns the HTTP status so the caller
 * can map 200 → approved / 410 → expired / other → failed. */
async function postDecision(
  sessionId: string,
  approval: PendingApproval,
  body: Record<string, unknown>,
): Promise<number> {
  const res = await fetch(
    `/api/session/${sessionId}/approval/${approval.approvalId}`,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(10000),
    },
  );
  return res.status;
}

// ----------------------------------------------------------------------------
// ApprovalCard — render gate + pending/terminal dispatch
// ----------------------------------------------------------------------------

export function ApprovalCard({ sessionId }: { sessionId: string }): JSX.Element | null {
  const pending = useAppStore((s) => s.approval.pending);
  const terminal = useAppStore((s) => s.approval.terminal);

  // Render gate: card visible when pending OR terminal is non-null.
  if (pending === null && terminal === null) {
    return null;
  }
  if (pending !== null) {
    return <PendingCardView approval={pending} sessionId={sessionId} />;
  }
  return <TerminalCardView approval={terminal!} />;
}

// ----------------------------------------------------------------------------
// PendingCardView — the active interrupt card
// ----------------------------------------------------------------------------

function PendingCardView({
  approval,
  sessionId,
}: {
  approval: PendingApproval;
  sessionId: string;
}): JSX.Element {
  const resolvePending = useAppStore((s) => s.approval.resolvePending);
  const appendHistory = useAppStore((s) => s.approval.appendHistory);

  const [jsonExpanded, setJsonExpanded] = useState(false);
  const [editMode, setEditMode] = useState(false);
  const [editedJson, setEditedJson] = useState('');
  const [showRememberScope, setShowRememberScope] = useState(false);
  const [scope, setScope] = useState<ApprovalScope>('session');
  const [denyPromptOpen, setDenyPromptOpen] = useState(false);
  const [denyReason, setDenyReason] = useState('');
  const [requestChangeOpen, setRequestChangeOpen] = useState(false);
  const [requestChangeReason, setRequestChangeReason] = useState('');
  const [inFlight, setInFlight] = useState<
    null | 'approve' | 'approve-remember' | 'deny' | 'request_change'
  >(null);

  const settle = useCallback(
    (status: TerminalApproval['status'], reason: string, finalArgs: JsonObject) => {
      const terminal: TerminalApproval = {
        approvalId: approval.approvalId,
        status,
        frozenArgs: finalArgs,
        reason,
        createdAt: approval.createdAt,
        resolvedAt: Date.now(),
      };
      resolvePending(approval.approvalId, terminal);
      appendHistory({
        approvalId: approval.approvalId,
        status,
        target: approval.target,
        toolName: approval.toolName,
        frozenArgs: finalArgs,
        createdAt: approval.createdAt,
        resolvedAt: terminal.resolvedAt,
        reason,
        skillVersion: approval.skillVersion,
        promptHash: approval.promptHash,
      });
    },
    [approval, resolvePending, appendHistory],
  );

  const handleApprove = useCallback(
    async (remember: boolean) => {
      setInFlight(remember ? 'approve-remember' : 'approve');
      const finalArgs = computeFrozenArgsForPost(approval, editedJson, editMode);
      try {
        const status = await postDecision(sessionId, approval, {
          decision: 'approve',
          target: approval.target,
          frozen_args: finalArgs,
          approval_id: approval.approvalId,
          ...(approval.interruptId ? { interrupt_id: approval.interruptId } : {}),
          ...(remember ? { scope } : {}),
        });
        if (status === 200) {
          settle('approved', '', finalArgs);
        } else if (status === 410) {
          settle('expired', 'Skill 或 prompt 已变更，无法恢复', approval.frozenArgs);
        } else {
          settle('failed', `操作失败：HTTP ${status}. 请重试。`, approval.frozenArgs);
        }
      } catch (err) {
        settle('failed', String(err), approval.frozenArgs);
      } finally {
        setInFlight(null);
      }
    },
    [approval, editedJson, editMode, scope, sessionId, settle],
  );

  const handleDeny = useCallback(async () => {
    const reason = denyReason.trim();
    // Belt-and-suspenders — the submit button is already disabled
    // under 3 chars; the BE ApprovalDecision.Validate 422s too.
    if (reason.length < 3) return;
    setInFlight('deny');
    try {
      const status = await postDecision(sessionId, approval, {
        decision: 'deny',
        reason,
        target: approval.target,
        approval_id: approval.approvalId,
        ...(approval.interruptId ? { interrupt_id: approval.interruptId } : {}),
      });
      if (status === 200) {
        settle('denied', reason, approval.frozenArgs);
      } else if (status === 410) {
        settle('expired', 'Skill 或 prompt 已变更，无法恢复', approval.frozenArgs);
      } else {
        settle('failed', `操作失败：HTTP ${status}. 请重试。`, approval.frozenArgs);
      }
    } catch (err) {
      settle('failed', String(err), approval.frozenArgs);
    } finally {
      setInFlight(null);
    }
  }, [approval, denyReason, sessionId, settle]);

  const handleRequestChange = useCallback(async () => {
    const reason = requestChangeReason.trim();
    if (reason.length < 3) return;
    setInFlight('request_change');
    try {
      const status = await postDecision(sessionId, approval, {
        decision: 'request_change',
        reason,
        target: approval.target,
        approval_id: approval.approvalId,
        ...(approval.interruptId ? { interrupt_id: approval.interruptId } : {}),
      });
      if (status === 200) {
        settle('change_requested', reason, approval.frozenArgs);
      } else if (status === 410) {
        settle('expired', 'Skill 或 prompt 已变更，无法恢复', approval.frozenArgs);
      } else {
        settle('failed', `操作失败：HTTP ${status}. 请重试。`, approval.frozenArgs);
      }
    } catch (err) {
      settle('failed', String(err), approval.frozenArgs);
    } finally {
      setInFlight(null);
      setRequestChangeOpen(false);
    }
  }, [requestChangeReason, sessionId, approval, settle]);

  return (
    <div
      className="absolute z-25 left-4 right-4 bottom-4 max-w-[840px] my-4 rounded-md border-l-[3px] border-l-[var(--color-primary)] bg-[var(--color-surface)] shadow-md"
      data-testid="approval-card"
      data-state="pending"
      role="region"
      aria-label="审批申请"
    >
      {/* Header strip */}
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--color-surface-2)] rounded-t-md">
        <div className="flex items-center gap-2">
          <ScrollText size={16} className="text-[var(--color-primary)]" />
          <span
            className="font-mono text-xs text-[var(--color-text)]"
            data-testid="tool-name"
          >
            {approval.toolName}
          </span>
        </div>
        <span
          className="text-xs font-semibold uppercase tracking-wider text-[var(--color-primary)] px-2 py-0.5 rounded"
          style={{ letterSpacing: '0.05em' }}
          data-testid="status-badge"
        >
          待审批
        </span>
      </div>

      {/* Body */}
      <div className="px-3 py-3 space-y-2">
        {/* "what" line — derived from target, never free-form */}
        <p className="text-sm text-[var(--color-text)]" data-testid="what-line">
          {deriveWhatLine(approval.target, approval.frozenArgs)}
        </p>

        {/* "tool" + "why" line */}
        <p className="text-xs text-[var(--color-text-muted)]" data-testid="tool-line">
          目标：<span className="font-mono">{approval.target}</span>
          <span className="mx-1">·</span>
          原因：{approval.reason || '未提供具体说明'}
        </p>

        {/* D18-D4: report:publish structure summary — section list + KPI names.
            WR-03: reads frozenArgs.template.sections (the real location) with
            array guards; templateVersion is the server-derived content hash. */}
        {approval.target === 'report:publish' &&
          (() => {
            const reportArgs = asReportPublishArgs(approval.frozenArgs);
            const sections = Array.isArray(reportArgs.template?.sections)
              ? reportArgs.template!.sections!
              : [];
            return (
              <div
                className="mt-2 p-2 rounded"
                style={{ backgroundColor: 'var(--color-surface-2)' }}
              >
                <div className="text-xs font-semibold mb-1" style={{ color: 'var(--color-text)' }}>
                  报告结构
                </div>
                {sections.length === 0 && (
                  <div className="text-xs mb-0.5" style={{ color: 'var(--color-text-muted)' }}>
                    （模板未声明章节）
                  </div>
                )}
                {sections.map((sec, idx) => (
                  <div key={idx} className="text-xs mb-0.5" style={{ color: 'var(--color-text-muted)' }}>
                    · {sec.title ?? `章节 ${idx + 1}`}: {Array.isArray(sec.kpis) ? sec.kpis.join(', ') : ''}
                  </div>
                ))}
                {reportArgs.templateVersion && (
                  <div className="text-xs mt-1" style={{ color: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}>
                    模板版本：sha256:{reportArgs.templateVersion.slice(0, 16)}...
                  </div>
                )}
              </div>
            );
          })()}

        {/* FrozenArgs JSON panel — collapsed by default */}
        <div className="rounded border border-[var(--color-border-muted)] bg-[var(--color-surface-2)]">
          <button
            type="button"
            onClick={() => setJsonExpanded(!jsonExpanded)}
            className="w-full flex items-center justify-between px-2 py-1 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            data-testid="frozen-toggle"
          >
            <span className="font-mono">参数快照</span>
            <span>{jsonExpanded ? '收起' : '展开'}</span>
          </button>
          {jsonExpanded && (
            <div className="px-2 py-2">
              {editMode ? (
                <div data-testid="edit-mode">
                  <textarea
                    className="w-full h-32 px-2 py-1 font-mono text-xs bg-[var(--color-surface-3)] border border-[var(--color-primary)] rounded text-[var(--color-text)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]"
                    value={editedJson}
                    onChange={(e) => setEditedJson(e.target.value)}
                    aria-label="编辑参数"
                    data-testid="edit-textarea"
                  />
                  <p
                    className="text-xs text-[var(--color-primary)] mt-1"
                    data-testid="edit-banner"
                  >
                    已修改参数，待审批将以新值执行
                  </p>
                </div>
              ) : (
                <pre
                  className="font-mono text-xs text-[var(--color-text)] overflow-x-auto"
                  data-testid="frozen-json"
                >
                  {JSON.stringify(approval.frozenArgs, null, 2)}
                </pre>
              )}
            </div>
          )}
        </div>

        {/* Edit toggle (HITL-07) — dwells in the expanded args panel */}
        {jsonExpanded && (
          <button
            type="button"
            onClick={() => {
              if (!editMode) {
                setEditedJson(JSON.stringify(approval.frozenArgs, null, 2));
              }
              setEditMode(!editMode);
            }}
            className="text-xs text-[var(--color-primary)] hover:text-[var(--color-primary-hover)]"
            data-testid="edit-toggle"
          >
            {editMode ? '取消编辑' : '编辑参数'}
          </button>
        )}

        {/* Button trio */}
        <div className="flex gap-2 pt-2">
          <button
            type="button"
            onClick={() => handleApprove(false)}
            disabled={inFlight !== null}
            className="px-3 py-1.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)] text-sm font-semibold hover:bg-[var(--color-primary-hover)] disabled:opacity-50"
            data-testid="approve-once"
          >
            {inFlight === 'approve' ? '批准一次...' : '批准一次'}
          </button>
          <button
            type="button"
            onClick={() => setShowRememberScope(!showRememberScope)}
            disabled={inFlight !== null}
            className="px-3 py-1.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)] text-sm font-semibold hover:bg-[var(--color-primary-hover)] disabled:opacity-50"
            data-testid="approve-remember"
          >
            {showRememberScope ? '批准并记住 ▲' : '批准并记住 ▼'}
          </button>
          <button
            type="button"
            onClick={() => setDenyPromptOpen(!denyPromptOpen)}
            disabled={inFlight !== null}
            className="px-3 py-1.5 rounded bg-[var(--color-danger)] text-white text-sm font-semibold hover:opacity-90 disabled:opacity-50"
            data-testid="deny-button"
          >
            拒绝
          </button>
        </div>

        {/* 要求修改第三态 — D18-D4: 仅 report:publish 可见 */}
        {approval.target === 'report:publish' && (
          <>
            <button
              type="button"
              onClick={() => setRequestChangeOpen(!requestChangeOpen)}
              disabled={inFlight !== null}
              className="mt-2 px-3 py-1.5 rounded text-sm font-semibold"
              style={{
                backgroundColor: 'var(--color-status-needs-info)',
                color: '#14171e',
              }}
              data-testid="request-change-button"
            >
              要求修改
            </button>

            {requestChangeOpen && (
              <div
                className="mt-2 p-2 rounded border"
                style={{
                  borderColor: 'var(--color-border)',
                  backgroundColor: 'var(--color-surface-2)',
                }}
                data-testid="request-change-prompt"
              >
                <p className="text-xs mb-1" style={{ color: 'var(--color-text-muted)' }}>
                  请填写修改意见（至少 3 个字符）
                </p>
                <textarea
                  className="w-full h-16 px-2 py-1 text-sm rounded"
                  style={{
                    backgroundColor: 'var(--color-surface-3)',
                    borderColor: 'var(--color-border)',
                    color: 'var(--color-text)',
                  }}
                  value={requestChangeReason}
                  onChange={(e) => setRequestChangeReason(e.target.value)}
                  placeholder="例如：更突出 Q2 风险..."
                  data-testid="request-change-reason-input"
                />
                <div className="flex gap-2 mt-1">
                  <button
                    type="button"
                    onClick={handleRequestChange}
                    disabled={requestChangeReason.trim().length < 3 || inFlight !== null}
                    className="px-3 py-1.5 rounded text-sm font-semibold"
                    style={{
                      backgroundColor: 'var(--color-status-needs-info)',
                      color: '#14171e',
                    }}
                    data-testid="request-change-submit"
                  >
                    {inFlight === 'request_change' ? '确认修改...' : '确认修改'}
                  </button>
                </div>
              </div>
            )}
          </>
        )}

        {/* Scope radio — ONLY when 批准并记住 is toggled open (D11-D3) */}
        {showRememberScope && (
          <div
            className="px-2 py-2 rounded border border-[var(--color-border-muted)] bg-[var(--color-surface-2)]"
            data-testid="scope-radio"
          >
            <p className="text-xs text-[var(--color-text-muted)] mb-1">作用域：</p>
            <div className="flex gap-3">
              <label className="flex items-center gap-1.5 text-sm text-[var(--color-text)]">
                <input
                  type="radio"
                  name="scope"
                  value="session"
                  checked={scope === 'session'}
                  onChange={() => setScope('session')}
                  data-testid="scope-session"
                  className="accent-[var(--color-primary)]"
                />
                <span>仅本次会话</span>
              </label>
              <label className="flex items-center gap-1.5 text-sm text-[var(--color-text)]">
                <input
                  type="radio"
                  name="scope"
                  value="skill"
                  checked={scope === 'skill'}
                  onChange={() => setScope('skill')}
                  data-testid="scope-skill"
                  className="accent-[var(--color-primary)]"
                />
                <span>整个 Skill 共享</span>
              </label>
            </div>
            <button
              type="button"
              onClick={() => handleApprove(true)}
              disabled={inFlight !== null}
              className="mt-2 px-3 py-1.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)] text-sm font-semibold disabled:opacity-50"
              data-testid="approve-remember-confirm"
            >
              {inFlight === 'approve-remember'
                ? '批准并记住...'
                : `确认：批准并记住（${scope === 'session' ? '本会话' : '整个 Skill'}）`}
            </button>
          </div>
        )}

        {/* Deny prompt — reason required (>= 3 chars, HITL-02) */}
        {denyPromptOpen && (
          <div
            className="px-2 py-2 rounded border border-[var(--color-border-muted)] bg-[var(--color-surface-2)]"
            data-testid="deny-prompt"
          >
            <p className="text-xs text-[var(--color-text-muted)] mb-1">
              请填写拒绝理由（至少 3 个字符）
            </p>
            <textarea
              className="w-full h-20 px-2 py-1 text-sm bg-[var(--color-surface-3)] border border-[var(--color-border)] rounded text-[var(--color-text)]"
              value={denyReason}
              onChange={(e) => setDenyReason(e.target.value)}
              placeholder="例如：这条事实当前不准确，请改为..."
              data-testid="deny-reason-input"
            />
            <div className="flex gap-2 mt-1">
              <button
                type="button"
                onClick={() => handleDeny()}
                disabled={denyReason.trim().length < 3 || inFlight !== null}
                className="px-3 py-1.5 rounded bg-[var(--color-danger)] text-white text-sm font-semibold disabled:opacity-50"
                data-testid="deny-submit"
                title={denyReason.trim().length < 3 ? '拒绝时必须填写理由' : undefined}
              >
                {inFlight === 'deny' ? '确认拒绝...' : '确认拒绝'}
              </button>
              <button
                type="button"
                onClick={() => {
                  setDenyPromptOpen(false);
                  setDenyReason('');
                }}
                className="px-3 py-1.5 rounded text-sm text-[var(--color-text-muted)]"
                data-testid="deny-cancel"
              >
                取消
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ----------------------------------------------------------------------------
// TerminalCardView — the 5 terminal visual states
// ----------------------------------------------------------------------------

function TerminalCardView({ approval }: { approval: TerminalApproval }): JSX.Element {
  const clearTerminal = useAppStore((s) => s.approval.clearTerminal);

  const statusConfig: Record<
    TerminalApproval['status'],
    { border: string; badge: string; text: string; icon: JSX.Element }
  > = {
    approved: {
      border: 'border-l-[var(--color-status-compliant)]',
      badge: 'bg-[var(--color-status-compliant)] text-black',
      text: '已批准',
      icon: <ShieldCheck size={16} />,
    },
    denied: {
      border: 'border-l-[var(--color-status-non-compliant)]',
      badge: 'bg-[var(--color-status-non-compliant)] text-black',
      text: '已拒绝',
      icon: <ShieldX size={16} />,
    },
    expired: {
      border: 'border-l-[var(--color-status-needs-info)]',
      badge: 'bg-[var(--color-status-needs-info)] text-black',
      text: '已过期',
      icon: <AlertOctagon size={16} />,
    },
    'auto-passed': {
      border: 'border-l-[var(--color-status-unanswered)]',
      badge: 'bg-[var(--color-status-unanswered)] text-black',
      text: '已通过',
      icon: <CircleSlash size={16} />,
    },
    failed: {
      border: 'border-l-[var(--color-status-error)]',
      badge: 'bg-[var(--color-status-error)] text-white',
      text: '失败',
      icon: <AlertOctagon size={16} />,
    },
    change_requested: {
      border: 'border-l-[var(--color-status-needs-info)]',
      badge: 'bg-[var(--color-status-needs-info)] text-black',
      text: '已要求修改',
      icon: <AlertOctagon size={16} />,
    },
  };
  const cfg = statusConfig[approval.status];

  const line =
    approval.status === 'auto-passed'
      ? '已自动通过（记忆规则）'
      : approval.status === 'expired'
        ? '该审批已过期'
        : approval.status === 'failed'
          ? `审批操作失败：${approval.reason}`
          : approval.status === 'denied'
            ? `已拒绝：${approval.reason}`
            : approval.status === 'change_requested'
              ? `已要求修改：${approval.reason}`
              : '已批准';

  return (
    <div
      className={`absolute z-25 left-4 right-4 bottom-4 max-w-[840px] my-3 rounded-md border-l-[3px] ${cfg.border} bg-[var(--color-surface)]`}
      data-testid="approval-card-terminal"
      data-state={approval.status}
      role="region"
      aria-label="审批结果"
    >
      <div className="flex items-center justify-between px-3 py-2">
        <div className="flex items-center gap-2 text-[var(--color-text)]">
          {cfg.icon}
          <span className="font-mono text-xs">{line}</span>
        </div>
        <div className="flex items-center gap-2">
          <span
            className={`px-2 py-0.5 text-xs font-semibold uppercase tracking-wider rounded ${cfg.badge}`}
            style={{ letterSpacing: '0.05em' }}
          >
            {cfg.text}
          </span>
          <button
            type="button"
            onClick={() => clearTerminal()}
            aria-label="关闭"
            className="text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            data-testid="terminal-close"
          >
            <X size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
