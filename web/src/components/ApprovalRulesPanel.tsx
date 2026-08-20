// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 / HITL-08 / D11-D4 — the Approval Rules panel. Mounted as
// the left session panel's 审批规则 section (SessionDrawer). Read-only
// list view + per-row revoke X:
//
//   - GET  /api/session/:id/approval-rules?scope=session|skill on mount
//     + on session / skill / approval-terminal-status change (the D14
//     reactive re-fetch after an approve-and-remember rule is created)
//   - DELETE /api/session/:id/approval-rules/:ruleId on the revoke X;
//     2xx → 120ms fade-out + optimistic unmount + success toast;
//     non-2xx → row stays + error toast (the BE audit chain is the
//     source of truth — the optimistic unmount is decorative)
//
// NO confirm dialog (UI-SPEC §Destructive Confirmation — the revoke is
// reversible: re-approving re-creates the rule). NO axios (fetch only,
// mirroring lib/sessions.ts).
import { useCallback, useEffect, useState } from 'react';
import { Trash2 } from 'lucide-react';
import { cn } from '../lib/cn';
import { useAppStore } from '../store/app-store';
import {
  fetchApprovalRules,
  revokeApprovalRule,
  type ApprovalRuleRow,
} from '../lib/sessions';

/** Relative-time label (zh-CN) — same rhythm as SessionDrawer's
 * relativeTime: 刚刚 / N 分钟前 / N 小时前 / N 天前. */
function relativeTimeZh(epochMs: number): string {
  const diff = Date.now() - epochMs;
  if (diff < 60_000) return '刚刚';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`;
  return `${Math.floor(diff / 86_400_000)} 天前`;
}

const SCOPE_LABEL: Record<ApprovalRuleRow['scope'], string> = {
  session: '本会话',
  skill: '整个 Skill',
};

/** Row fade-out duration before the optimistic unmount (UI-SPEC). */
const FADE_OUT_MS = 120;
/** Toast auto-dismiss (success + error share the 3s rhythm). */
const TOAST_MS = 3000;

export default function ApprovalRulesPanel() {
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  // 11-06 owns the approval slice shape — this panel READS the terminal
  // status only, as the reactive re-fetch trigger after an
  // approve-and-remember rule terminalizes (D14 reactive pattern).
  const terminalStatus = useAppStore((s) => s.approval.terminal?.status ?? null);
  const [rules, setRules] = useState<ApprovalRuleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [fadingId, setFadingId] = useState<string | null>(null);
  const [toast, setToast] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);

  const loadRules = useCallback(async () => {
    setLoading(true);
    setError(null);
    const list = await fetchApprovalRules(activeSessionId, currentSkillId);
    if (list === null) {
      setError('无法加载审批规则。');
      setRules([]);
    } else {
      setRules(list);
    }
    setLoading(false);
    // terminalStatus is the reactive re-fetch trigger (rule created via
    // approve-and-remember terminalizes the pending approval → re-list).
  }, [activeSessionId, currentSkillId, terminalStatus]);

  useEffect(() => {
    void loadRules();
  }, [loadRules]);

  async function handleRevoke(rule: ApprovalRuleRow) {
    setRevokingId(rule.ruleId);
    const result = await revokeApprovalRule(activeSessionId, rule.ruleId);
    setRevokingId(null);
    if (result.ok) {
      setToast({
        kind: 'success',
        text: `已撤销：${rule.target} · ${SCOPE_LABEL[rule.scope]}`,
      });
      setFadingId(rule.ruleId);
      setTimeout(() => {
        setRules((prev) => prev.filter((r) => r.ruleId !== rule.ruleId));
        setFadingId(null);
      }, FADE_OUT_MS);
      setTimeout(() => setToast(null), TOAST_MS);
    } else {
      setToast({ kind: 'error', text: `撤销失败：${result.reason}` });
      setTimeout(() => setToast(null), TOAST_MS);
    }
  }

  // Sort: last_used_at DESC NULLS LAST, created_at DESC — the
  // most-recently-used rule floats to the top (UI-SPEC §ApprovalRulesPanel).
  const sorted = [...rules].sort((a, b) => {
    if (a.lastUsedAt === null && b.lastUsedAt !== null) return 1;
    if (a.lastUsedAt !== null && b.lastUsedAt === null) return -1;
    if (a.lastUsedAt !== b.lastUsedAt) return (b.lastUsedAt ?? 0) - (a.lastUsedAt ?? 0);
    return b.createdAt - a.createdAt;
  });

  return (
    <section data-testid="approval-rules-panel" className="border-b border-border px-5 py-3">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-text-muted">审批规则</h3>
      <div className="mt-2 space-y-2">
        {loading ? (
          <div data-testid="approval-rules-skeleton" className="text-xs text-text-muted">
            加载审批规则…
          </div>
        ) : error ? (
          <div data-testid="approval-rules-error" className="text-xs text-status-non-compliant">
            {error}
          </div>
        ) : sorted.length === 0 ? (
          <div data-testid="approval-rules-empty" className="text-center text-xs text-text-muted">
            还没有任何审批规则。批准一次时勾选「记住」即可创建。
          </div>
        ) : (
          sorted.map((rule) => {
            const isRevoking = revokingId === rule.ruleId;
            const isFading = fadingId === rule.ruleId;
            return (
              <div
                key={rule.ruleId}
                data-testid="approval-rules-row"
                data-rule-id={rule.ruleId}
                className={cn(
                  'group flex items-start justify-between gap-2 rounded-md border border-border bg-surface px-2 py-1.5 transition-opacity duration-[120ms] hover:bg-surface-2',
                  isFading && 'opacity-0',
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    <span className="font-mono text-xs text-text">{rule.target}</span>
                    <span className="text-text-dim">·</span>
                    <span className="text-xs font-semibold text-text-muted">
                      {SCOPE_LABEL[rule.scope]}
                    </span>
                  </div>
                  <div className="mt-0.5 truncate text-xs text-text-muted">
                    {relativeTimeZh(rule.createdAt)} · 上次使用{' '}
                    {rule.lastUsedAt === null ? '未使用' : relativeTimeZh(rule.lastUsedAt)}
                  </div>
                </div>
                <button
                  type="button"
                  data-testid="approval-rules-revoke"
                  data-rule-id={rule.ruleId}
                  aria-label="撤销规则"
                  title="撤销规则 — 撤销后，将恢复每次审批。"
                  disabled={isRevoking}
                  onClick={() => void handleRevoke(rule)}
                  className={cn(
                    'flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-3 hover:text-status-non-compliant',
                    isRevoking && 'opacity-50',
                  )}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            );
          })
        )}
        {toast && (
          <div
            data-testid="approval-rules-toast"
            data-toast-kind={toast.kind}
            role="status"
            aria-live="polite"
            className={cn(
              'mt-2 rounded-md border px-2 py-1 text-xs',
              toast.kind === 'success'
                ? 'border-status-compliant/40 bg-status-compliant/10 text-status-compliant'
                : 'border-status-non-compliant/40 bg-status-non-compliant/10 text-status-non-compliant',
            )}
          >
            {toast.text}
          </div>
        )}
      </div>
    </section>
  );
}
