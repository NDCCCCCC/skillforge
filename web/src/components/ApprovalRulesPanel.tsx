// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 / HITL-08 / 56-03 PANEL-01/03 -- the Approval Rules panel.
// 56-03 upgrade: rules become first-class CRUD objects --
//   - hit_count / last_hit_at columns (the PANEL-01 hit ledger visible)
//   - enabled toggle (PATCH, optimistic update + rollback toast)
//   - expires_at display (expired rules greyed with a badge)
//   - match_type badge + global badge + origin suffix
//   - EMPTY STATE two-part (PANEL-03): guidance + suggestion cards
//   - highlight consumption (56-03 rule-matched -> highlightApprovalRuleId)
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Trash2 } from 'lucide-react';
import { cn } from '../lib/cn';
import { useAppStore } from '../store/app-store';
import {
  createApprovalRule,
  fetchApprovalRules,
  fetchRuleSuggestions,
  revokeApprovalRule,
  updateApprovalRule,
  type ApprovalRuleRow,
  type RuleSuggestion,
} from '../lib/sessions';

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
  global: '全局',
};

const MATCH_LABEL: Record<ApprovalRuleRow['matchType'], string> = {
  exact: '仅此参数',
  prefix: '同参数模板',
  template: '全部调用',
};

const FADE_OUT_MS = 120;
const TOAST_MS = 3000;
const HIGHLIGHT_MS = 2500;

export default function ApprovalRulesPanel() {
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const terminalStatus = useAppStore((s) => s.approval.terminal?.status ?? null);
  const highlightRuleId = useAppStore((s) => s.approval.highlightApprovalRuleId);
  const setHighlightRuleId = useAppStore((s) => s.approval.setHighlightApprovalRuleId);
  const [rules, setRules] = useState<ApprovalRuleRow[]>([]);
  const [suggestions, setSuggestions] = useState<RuleSuggestion[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [fadingId, setFadingId] = useState<string | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [toast, setToast] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);
  const [creatingTarget, setCreatingTarget] = useState<string | null>(null);

  const showToast = useCallback((kind: 'success' | 'error', text: string) => {
    setToast({ kind, text });
    setTimeout(() => setToast(null), TOAST_MS);
  }, []);

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
    // 56-03 PANEL-03: the suggestion clusters load alongside (null on
    // error degrades to "no suggestions" in the empty state).
    const sugg = await fetchRuleSuggestions(activeSessionId);
    setSuggestions(sugg);
    setLoading(false);
    // terminalStatus is a deliberate invalidation TRIGGER, not a read: an
    // approve-and-remember terminalization must refetch the rule list even
    // though the value itself is unused. Referencing it here keeps the
    // dependency honest instead of suppressing the lint rule (56-01 review).
    void terminalStatus;
  }, [activeSessionId, currentSkillId, terminalStatus]);

  useEffect(() => {
    void loadRules();
  }, [loadRules]);

  // 56-03 highlight consumption: the rule the chat stream just
  // auto-passed gets a 2.5s highlight band, then the transient clears.
  useEffect(() => {
    if (highlightRuleId) {
      const t = setTimeout(() => setHighlightRuleId(null), HIGHLIGHT_MS);
      return () => clearTimeout(t);
    }
  }, [highlightRuleId, setHighlightRuleId]);

  async function handleRevoke(rule: ApprovalRuleRow) {
    setRevokingId(rule.ruleId);
    const result = await revokeApprovalRule(activeSessionId, rule.ruleId);
    setRevokingId(null);
    if (result.ok) {
      showToast('success', `已撤销：${rule.target} · ${SCOPE_LABEL[rule.scope]}`);
      setFadingId(rule.ruleId);
      setTimeout(() => {
        setRules((prev) => prev.filter((r) => r.ruleId !== rule.ruleId));
        setFadingId(null);
      }, FADE_OUT_MS);
    } else {
      showToast('error', `撤销失败：${result.reason}`);
    }
  }

  // 56-03 PANEL-01: the enabled toggle (PATCH + optimistic update with
  // rollback on failure).
  async function handleToggle(rule: ApprovalRuleRow) {
    setTogglingId(rule.ruleId);
    const next = !rule.enabled;
    const prevRules = rules;
    setRules((rs) => rs.map((r) => (r.ruleId === rule.ruleId ? { ...r, enabled: next } : r)));
    const result = await updateApprovalRule(activeSessionId, rule.ruleId, { enabled: next });
    setTogglingId(null);
    if (result.ok) {
      showToast('success', next ? `规则已启用：${rule.target}` : `规则已停用：${rule.target}`);
    } else {
      setRules(prevRules); // rollback
      showToast('error', `更新失败：${result.reason}`);
    }
  }

  // 56-03 PANEL-03: one-click create from a suggestion cluster (v1 lands
  // a TEMPLATE rule per the BE's fixed match_type; scope=session keeps
  // the blast radius small).
  async function handleCreateFromSuggestion(target: string) {
    setCreatingTarget(target);
    const result = await createApprovalRule(activeSessionId, {
      target,
      matchType: 'template',
      scope: 'session',
      skillId: currentSkillId ?? undefined,
    });
    setCreatingTarget(null);
    if (result.ok) {
      showToast('success', `规则已创建：${target}（全部调用 · 本会话）`);
      setSuggestions((s) => (s ?? []).filter((sg) => sg.target !== target));
      await loadRules();
    } else {
      showToast('error', `创建失败：${result.reason}`);
    }
  }

  const sorted = useMemo(() => {
    return [...rules].sort((a, b) => {
      if (a.lastHitAt === null && b.lastHitAt !== null) return 1;
      if (a.lastHitAt !== null && b.lastHitAt === null) return -1;
      if (a.lastHitAt !== b.lastHitAt) return (b.lastHitAt ?? 0) - (a.lastHitAt ?? 0);
      return b.createdAt - a.createdAt;
    });
  }, [rules]);

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
          <div data-testid="approval-rules-empty">
            {/* PANEL-03 two-part empty state */}
            <p className="text-xs text-text-muted">
              还没有任何审批规则。批准一次时选择「记住」，或从下方建议一键创建：
            </p>
            {suggestions && suggestions.length > 0 ? (
              <div className="mt-2 space-y-1.5" data-testid="approval-rule-suggestions">
                {suggestions.map((sg) => (
                  <div
                    key={sg.target}
                    data-testid={`approval-rule-suggestion-${sg.target}`}
                    className="flex items-center justify-between gap-2 rounded-md border border-border bg-surface-2 px-2 py-1.5"
                  >
                    <span className="min-w-0 truncate text-xs text-text">
                      「{sg.target}」已人工批准 {sg.occurrences} 次
                    </span>
                    <button
                      type="button"
                      data-testid={`approval-rule-suggest-create-${sg.target}`}
                      disabled={creatingTarget === sg.target}
                      onClick={() => void handleCreateFromSuggestion(sg.target)}
                      className="shrink-0 rounded px-2 py-1 text-xs font-semibold bg-[var(--color-primary)] text-[var(--color-primary-fg)] disabled:opacity-50"
                    >
                      一键创建规则
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <p className="mt-1.5 text-xs text-text-dim" data-testid="approval-rule-suggestions-none">
                暂无重复批准模式。
              </p>
            )}
          </div>
        ) : (
          sorted.map((rule) => {
            const isRevoking = revokingId === rule.ruleId;
            const isFading = fadingId === rule.ruleId;
            const isHighlighted = highlightRuleId === rule.ruleId;
            const expired = rule.expiresAt > 0 && rule.expiresAt * 1000 < Date.now();
            return (
              <div
                key={rule.ruleId}
                data-testid="approval-rules-row"
                data-rule-id={rule.ruleId}
                className={cn(
                  'group flex items-start justify-between gap-2 rounded-md border bg-surface px-2 py-1.5 transition-all duration-[120ms] hover:bg-surface-2',
                  isFading && 'opacity-0',
                  isHighlighted && 'border-[var(--color-primary)] bg-[var(--color-primary)]/10',
                  !rule.enabled && 'opacity-60',
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="font-mono text-xs text-text">{rule.target}</span>
                    <span className="text-text-dim">·</span>
                    <span className="text-xs font-semibold text-text-muted">{SCOPE_LABEL[rule.scope]}</span>
                    <span
                      className="rounded px-1 py-0.5 text-[10px] font-semibold bg-surface-3 text-text-muted"
                      data-testid={`approval-rules-match-${rule.matchType}`}
                    >
                      {MATCH_LABEL[rule.matchType]}
                    </span>
                    {rule.scope === 'global' && (
                      <span className="rounded px-1 py-0.5 text-[10px] font-semibold bg-[var(--color-primary)]/15 text-[var(--color-primary)]">
                        全局
                      </span>
                    )}
                    {expired && (
                      <span className="rounded px-1 py-0.5 text-[10px] bg-surface-3 text-text-dim">已过期</span>
                    )}
                    {rule.origin === 'manual' && (
                      <span className="text-[10px] text-text-dim">手动</span>
                    )}
                  </div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs">
                    <span className={cn('text-xs', rule.hitCount > 0 ? 'text-text-muted' : 'text-text-dim')}>
                      {rule.hitCount > 0 ? `命中 ${rule.hitCount} 次` : '未命中'}
                    </span>
                    <span className="text-text-dim">·</span>
                    <span className="text-text-muted">
                      {rule.lastHitAt === null ? '未命中' : `上次命中 ${relativeTimeZh(rule.lastHitAt)}`}
                    </span>
                    <span className="text-text-dim">·</span>
                    <span className={cn('text-xs', rule.enabled ? 'text-text-muted' : 'text-status-non-compliant')}>
                      {rule.enabled ? '已启用' : '已停用'}
                    </span>
                    {/* 56-03 PANEL-01 enabled toggle (PATCH + optimistic) */}
                    <button
                      type="button"
                      data-testid={`approval-rules-toggle-${rule.ruleId}`}
                      disabled={togglingId === rule.ruleId}
                      onClick={() => void handleToggle(rule)}
                      className="text-xs text-[var(--color-primary)] underline-offset-2 hover:underline disabled:opacity-50"
                    >
                      {rule.enabled ? '停用' : '启用'}
                    </button>
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <button
                    type="button"
                    data-testid={`approval-rules-revoke-${rule.ruleId}`}
                    aria-label="撤销规则"
                    title="撤销规则 — 撤销后，将恢复每次审批"
                    disabled={isRevoking}
                    onClick={() => void handleRevoke(rule)}
                    className={cn(
                      'flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-3 hover:text-status-non-compliant',
                      isRevoking && 'opacity-50',
                    )}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
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
