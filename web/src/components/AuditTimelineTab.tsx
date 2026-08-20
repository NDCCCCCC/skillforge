// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 11-07 / HITL-04 / D11-D4 + D11-C3 — the audit timeline tab body
// (DetailPanel 6th tab 审计轨迹). Read-only audit chain view:
//
//   - GET /api/session/:id/audit?limit=100 on mount + session change
//   - 5 client-side filter chips (全部 / 待审批 / 已批准 / 已拒绝 /
//     规则撤销); default 全部
//   - One row per audit entry: 8px status bullet (color per action
//     enum, MUTUALLY EXCLUSIVE per row) + zh-CN action label + target
//     (mono) + ts + prompt-hash prefix + status sentence
//
// T-3-AUDIT INVARIANT: rows are READ-ONLY — no click handler, no
// edit/delete affordance (the audit chain is INSERT-only; the UI
// mirrors that). Fail-closed: a non-2xx / malformed response swaps the
// list for the 审计链异常 banner (role="alert").
//
// Color separation (UI-SPEC): pending=--color-primary (indigo) /
// approved=--color-status-compliant (green) / rejected=
// --color-status-non-compliant (red) / revoked=
// --color-status-needs-info (yellow).
import { useEffect, useState } from 'react';
import { cn } from '../lib/cn';
import { useAppStore } from '../store/app-store';
import { fetchAuditRows, type SessionAuditRow } from '../lib/sessions';

type FilterKey = 'all' | 'pending' | 'approved' | 'rejected' | 'revoked';

const ACTION_ZH: Record<SessionAuditRow['action'], string> = {
  action_pending_approval: '待审批',
  action_approved: '已批准',
  action_rejected: '已拒绝',
  action_rule_revoked: '规则撤销',
};

/** [bullet bg, label fg] — one color pair per action, mutually
 * exclusive per row (UI-SPEC §Color separation contract). */
const ACTION_COLOR: Record<SessionAuditRow['action'], [string, string]> = {
  action_pending_approval: ['bg-primary', 'text-primary'],
  action_approved: ['bg-status-compliant', 'text-status-compliant'],
  action_rejected: ['bg-status-non-compliant', 'text-status-non-compliant'],
  action_rule_revoked: ['bg-status-needs-info', 'text-status-needs-info'],
};

const FILTER_LABELS: Record<FilterKey, string> = {
  all: '全部',
  pending: '待审批',
  approved: '已批准',
  rejected: '已拒绝',
  revoked: '规则撤销',
};

const FILTER_TO_ACTION: Record<Exclude<FilterKey, 'all'>, SessionAuditRow['action']> = {
  pending: 'action_pending_approval',
  approved: 'action_approved',
  rejected: 'action_rejected',
  revoked: 'action_rule_revoked',
};

const FILTER_ORDER: FilterKey[] = ['all', 'pending', 'approved', 'rejected', 'revoked'];

function formatTs(epochMs: number): string {
  const d = new Date(epochMs);
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function auditStatusSentence(row: SessionAuditRow): string {
  switch (row.action) {
    case 'action_pending_approval':
      return '等待评审';
    case 'action_approved':
      return `于 ${formatTs(row.ts)} 批`;
    case 'action_rejected':
      return `于 ${formatTs(row.ts)} 拒`;
    case 'action_rule_revoked':
      return `撤销于 ${formatTs(row.ts)}`;
  }
}

export default function AuditTimelineTab() {
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const [rows, setRows] = useState<SessionAuditRow[]>([]);
  const [filter, setFilter] = useState<FilterKey>('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    void (async () => {
      const list = await fetchAuditRows(activeSessionId);
      if (cancelled) return;
      if (list === null) {
        // Fail-closed (D11-C3): the audit chain is the trust anchor —
        // a broken read surfaces as an operator-visible alert, never
        // as an empty list (an empty list would imply "no events").
        setError(true);
        setRows([]);
      } else {
        setRows(list);
      }
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [activeSessionId]);

  const filtered =
    filter === 'all' ? rows : rows.filter((r) => r.action === FILTER_TO_ACTION[filter]);

  if (error) {
    return (
      <div
        data-testid="audit-timeline-error"
        role="alert"
        className="rounded-md border border-status-error/40 bg-status-error/10 px-3 py-2 text-xs text-status-error"
      >
        审计链异常，请联系管理员。
      </div>
    );
  }

  return (
    <div data-testid="audit-timeline" className="flex flex-col gap-3 px-1">
      <div className="flex flex-wrap gap-1.5" role="tablist" aria-label="审计动作筛选">
        {FILTER_ORDER.map((key) => (
          <button
            key={key}
            type="button"
            data-testid="audit-timeline-filter"
            data-filter={key}
            aria-pressed={filter === key}
            onClick={() => setFilter(key)}
            className={cn(
              'rounded-full border px-2 py-0.5 text-xs font-semibold transition-colors',
              filter === key
                ? 'border-primary bg-primary-dim text-primary'
                : 'border-border bg-surface text-text-muted hover:bg-surface-3 hover:text-text',
            )}
          >
            {FILTER_LABELS[key]}
          </button>
        ))}
      </div>
      {loading ? (
        <div data-testid="audit-timeline-skeleton" className="text-xs text-text-muted">
          加载审计轨迹…
        </div>
      ) : filtered.length === 0 ? (
        <div data-testid="audit-timeline-empty" className="text-xs text-text-muted">
          该会话还没有审计记录。
        </div>
      ) : (
        <ol className="space-y-1.5" data-testid="audit-timeline-list">
          {filtered.map((row) => {
            const [bullet, label] = ACTION_COLOR[row.action];
            return (
              // T-3-AUDIT: plain <li> — no onClick, no edit/delete.
              <li
                key={row.id}
                data-testid="audit-timeline-row"
                data-action={row.action}
                className="rounded-md border border-border bg-surface px-2 py-1.5"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span
                      data-testid="audit-bullet"
                      className={cn('inline-block h-2 w-2 shrink-0 rounded-full', bullet)}
                      aria-hidden="true"
                    />
                    <span className={cn('text-xs font-semibold', label)}>
                      {ACTION_ZH[row.action]}
                    </span>
                    <span className="truncate font-mono text-xs text-text-muted">{row.target}</span>
                  </div>
                  <span className="shrink-0 text-xs text-text-muted">{formatTs(row.ts)}</span>
                </div>
                <div className="mt-1 flex items-center justify-between gap-2 text-xs text-text-dim">
                  <span className="font-mono">
                    hash {row.promptHash ? row.promptHash.slice(0, 8) : '—'}
                  </span>
                  <span className="text-text-muted">{auditStatusSentence(row)}</span>
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
