// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 3 — PlanRulesPanel (D11-D2 single-table reuse).
//
// Lifted from ApprovalRulesPanel.tsx (Phase 11 11-07 / HITL-08) verbatim
// with one filter change: the panel renders ONLY rules whose target is
// `plan_approve`. The BE endpoint stays the same (DELETE
// /api/session/:id/approval-rules/:ruleId per D11-D2 — approval_rules
// is shared across all HITL targets; the FE filter here is the
// responsibility split between Phase 11 ApprovalRulesPanel and the
// new PlanRulesPanel).
//
// Mounted as the 6th section of the SessionDrawer (formerly
// LeftSidebar.tsx in pre-Open-Design, the surviving left-side surface
// after Open Design B2). Sits directly below the Phase 11
// ApprovalRulesPanel — same structural placement, narrower rule scope.
//
//   - GET  /api/session/:id/approval-rules?scope=session|skill on mount
//     + on session / skill / approval-terminal-status change (D14 reactive)
//   - Filter by target='plan_approve' (this panel's only deviation from
//     ApprovalRulesPanel)
//   - DELETE on revoke X: 2xx → optimistic fade + success toast;
//     non-2xx → row stays + error toast
//
// NO confirm dialog (the revoke is reversible via re-approving).
import { useCallback, useEffect, useState } from 'react';
import { Trash2 } from 'lucide-react';
import { cn } from '../lib/cn';
import { useAppStore } from '../store/app-store';
import {
  fetchApprovalRules,
  revokeApprovalRule,
  type ApprovalRuleRow,
} from '../lib/sessions';

/** Relative-time label (zh-CN) — mirrors ApprovalRulesPanel. */
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

/** Row fade-out duration before the optimistic unmount. */
const FADE_OUT_MS = 120;
/** Toast auto-dismiss. */
const TOAST_MS = 3000;

/** The HITL target this panel owns — Phase 11 ApprovalRulesPanel owns
 * memory:facts / review_report; this panel owns plan_approve. */
export const PLAN_RULES_TARGET = 'plan_approve';

export default function PlanRulesPanel() {
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  // D14 reactive re-fetch: when a plan-approval terminalizes, refetch
  // the rule list (an approve-and-remember rule may have just been
  // created).
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
      setError('无法加载 plan_approve 规则。');
      setRules([]);
    } else {
      // Single-table filter (D11-D2): only plan_approve rows are
      // rendered here. Phase 11 ApprovalRulesPanel owns the rest.
      setRules(list.filter((r) => r.target === PLAN_RULES_TARGET));
    }
    setLoading(false);
    // terminalStatus is the reactive re-fetch trigger.
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

  // Sort: last_used_at DESC NULLS LAST, created_at DESC.
  const sorted = [...rules].sort((a, b) => {
    if (a.lastUsedAt === null && b.lastUsedAt !== null) return 1;
    if (a.lastUsedAt !== null && b.lastUsedAt === null) return -1;
    if (a.lastUsedAt !== b.lastUsedAt) return (b.lastUsedAt ?? 0) - (a.lastUsedAt ?? 0);
    return b.createdAt - a.createdAt;
  });

  return (
    <section data-testid="plan-rules-panel" className="border-b border-border px-5 py-3">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-text-muted">计划审批规则</h3>
      <div className="mt-2 space-y-2">
        {loading ? (
          <div data-testid="plan-rules-skeleton" className="text-xs text-text-muted">
            加载 plan_approve 规则…
          </div>
        ) : error ? (
          <div data-testid="plan-rules-error" className="text-xs text-status-non-compliant">
            {error}
          </div>
        ) : sorted.length === 0 ? (
          <div data-testid="plan-rules-empty" className="text-center text-xs text-text-muted">
            还没有任何 plan_approve 规则。批准计划时勾选「记住」即可创建。
          </div>
        ) : (
          sorted.map((rule) => {
            const isRevoking = revokingId === rule.ruleId;
            const isFading = fadingId === rule.ruleId;
            return (
              <div
                key={rule.ruleId}
                data-testid="plan-rules-row"
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
                  data-testid="plan-rules-revoke"
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
            data-testid="plan-rules-toast"
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