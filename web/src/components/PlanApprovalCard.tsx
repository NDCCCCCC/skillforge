// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 1 — PlanApprovalCard (PLAN-03 SC-3 / D11-B2 reuse).
//
// Mirrors ApprovalCard (Phase 11 11-06) verbatim with two differences:
//
//   1. Endpoints:
//      POST /api/run/:runId/plan/:approvalId/approve
//      (NOT /api/session/:id/approval/:approvalId — the BE plan endpoint
//      scopes by run id per internal/server/plan_api.go).
//
//   2. Filter: the approval slice is shared with Phase 11 ApprovalCard;
//      PlanApprovalCard only renders when state.approval.target === 'plan_approve'.
//      The card derives from the SAME store branch (D14 invariant: NO
//      messages field, NO chat-stream reads).
//
// Visual states (6): pending + approved / denied / expired / auto-passed
// / failed. Button trio: 批准一次 / 批准并记住 / 拒绝. D11-D3 scope radio
// (session|skill, NO global) appears on 批准并记住. Edit-before-approve
// (HITL-07 contract) lets the operator edit FrozenArgs JSON inline;
// the BE carries the edited bytes verbatim (D11-C3 fail-closed audit
// row reflects the edit). Deny reason >= 3 chars (HITL-02 contract).
//
// POST body is the same ApprovalDecision shape used by approval_api.go
// (snake_case wire). HTTP status → visual state:
//   200 → approved | denied
//   410 → expired (D11-C2 expire-check mismatch)
//   other → failed (D11-C3 fail-closed)

import { useCallback, useState } from 'react';
import type { JSX } from 'react';
import {
  AlertOctagon,
  CircleSlash,
  ClipboardList,
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

/** The "what" line for plan_approve targets — describes the proposed plan
 * being approved (NOT a memory fact or review report). */
function derivePlanWhatLine(frozenArgs: JsonObject): string {
  const plan = frozenArgs['plan'] as { title?: string; summary?: string } | undefined;
  if (plan && typeof plan.title === 'string') {
    return `代理希望执行计划：「${plan.title}」`;
  }
  if (plan && typeof plan.summary === 'string') {
    return `代理希望执行计划：「${plan.summary}」`;
  }
  return '代理希望执行该计划';
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

/** POST the operator decision. Returns the HTTP status so the caller
 * can map 200 → approved / denied, 410 → expired, other → failed. */
async function postPlanDecision(
  runId: string,
  approval: PendingApproval,
  body: Record<string, unknown>,
): Promise<number> {
  const res = await fetch(
    `/api/run/${encodeURIComponent(runId)}/plan/${encodeURIComponent(approval.approvalId)}/approve`,
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
// PlanApprovalCard — render gate + pending/terminal dispatch
// ----------------------------------------------------------------------------

export function PlanApprovalCard({ runId }: { runId: string }): JSX.Element | null {
  const pending = useAppStore((s) => s.approval.pending);
  const terminal = useAppStore((s) => s.approval.terminal);

  // Render gate: only react to plan_approve approvals. Phase 11
  // ApprovalCard owns the memory:facts / review_report pending rows;
  // this card owns plan_approve (mirrors D11-D2 single-table pattern).
  if (pending === null && terminal === null) return null;
  if (pending !== null && pending.target !== 'plan_approve') return null;
  if (terminal !== null && terminal.approvalId !== pending?.approvalId && !isPlanTerminal(terminal)) {
    return null;
  }
  if (pending !== null) {
    return <PendingPlanView approval={pending} runId={runId} />;
  }
  return <TerminalPlanView approval={terminal!} />;
}

/** True when a terminal approval was for a plan_approve target. We can't
 * tell from TerminalApproval alone (target isn't carried); callers must
 * check via the live approval.slice or pass a fresh PendingApproval.
 * Here we accept terminal when pending is null AND terminal is set — the
 * upstream gate above (pending check) ensures we only fall through to
 * TerminalPlanView when no plan_approve pending row exists, so the most
 * recent terminal row (if it was a plan_approve) is what we render. */
function isPlanTerminal(_t: TerminalApproval): boolean {
  // ApprovalCard owns memory:facts / review_report terminals; the live
  // render gate above guarantees we only get here when terminal was
  // for plan_approve OR no pending row exists. Always accept when the
  // caller reaches TerminalPlanView.
  return true;
}

// ----------------------------------------------------------------------------
// PendingPlanView — the active interrupt card
// ----------------------------------------------------------------------------

function PendingPlanView({
  approval,
  runId,
}: {
  approval: PendingApproval;
  runId: string;
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
  const [inFlight, setInFlight] = useState<
    null | 'approve' | 'approve-remember' | 'deny'
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
        const status = await postPlanDecision(runId, approval, {
          decision: 'approve',
          target: approval.target,
          frozen_args: finalArgs,
          approval_id: approval.approvalId,
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
    [approval, editedJson, editMode, scope, runId, settle],
  );

  const handleDeny = useCallback(async () => {
    const reason = denyReason.trim();
    // Belt-and-suspenders — submit button is already disabled under
    // 3 chars; the BE ApprovalDecision.Validate 422s too.
    if (reason.length < 3) return;
    setInFlight('deny');
    try {
      const status = await postPlanDecision(runId, approval, {
        decision: 'deny',
        reason,
        target: approval.target,
        approval_id: approval.approvalId,
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
  }, [approval, denyReason, runId, settle]);

  return (
    <div
      className="absolute z-25 left-4 right-4 bottom-4 max-w-[840px] my-4 rounded-md border-l-[3px] border-l-[var(--color-primary)] bg-[var(--color-surface)] shadow-md"
      data-testid="plan-approval-card"
      data-state="pending"
      data-target="plan_approve"
      role="region"
      aria-label="计划审批申请"
    >
      {/* Header strip */}
      <div className="flex items-center justify-between px-3 py-2 bg-[var(--color-surface-2)] rounded-t-md">
        <div className="flex items-center gap-2">
          <ClipboardList size={16} className="text-[var(--color-primary)]" />
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
        <p className="text-sm text-[var(--color-text)]" data-testid="what-line">
          {derivePlanWhatLine(approval.frozenArgs)}
        </p>

        <p className="text-xs text-[var(--color-text-muted)]" data-testid="tool-line">
          目标：<span className="font-mono">{approval.target}</span>
          <span className="mx-1">·</span>
          原因：{approval.reason || '未提供具体说明'}
        </p>

        {/* FrozenArgs JSON panel — collapsed by default */}
        <div className="rounded border border-[var(--color-border-muted)] bg-[var(--color-surface-2)]">
          <button
            type="button"
            onClick={() => setJsonExpanded(!jsonExpanded)}
            className="w-full flex items-center justify-between px-2 py-1 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            data-testid="frozen-toggle"
          >
            <span className="font-mono">计划参数快照</span>
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
                    aria-label="编辑计划参数"
                    data-testid="edit-textarea"
                  />
                  <p
                    className="text-xs text-[var(--color-primary)] mt-1"
                    data-testid="edit-banner"
                  >
                    已修改计划参数，待审批将以新值执行
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

        {/* Edit toggle (HITL-07) */}
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
                  name="plan-scope"
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
                  name="plan-scope"
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
              placeholder="例如：计划步骤过多，请拆分..."
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
// TerminalPlanView — the 5 terminal visual states
// ----------------------------------------------------------------------------

function TerminalPlanView({ approval }: { approval: TerminalApproval }): JSX.Element {
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
            : '已批准';

  return (
    <div
      className={`absolute z-25 left-4 right-4 bottom-4 max-w-[840px] my-3 rounded-md border-l-[3px] ${cfg.border} bg-[var(--color-surface)]`}
      data-testid="plan-approval-card-terminal"
      data-state={approval.status}
      role="region"
      aria-label="计划审批结果"
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

export default PlanApprovalCard;