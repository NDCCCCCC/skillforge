// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-05 + D6-07 + SC-3) — AuditTable.
//
// Reads auditRows and auditFilter from the store. Renders a filter bar
// mapping to /api/audit query params (user, skill, refusal_reason,
// since, until, limit, offset) and a table of audit rows.
//
// Redaction UX (D6-07): rows whose query or verdict_json start with
// "[REDACTED:" render as a gray badge "REDACTED (90d+)" with a
// title attribute showing the full reason (timestamp + retention
// boundary). Raw content is never expandable without an admin token.
//
// The filter bar debounces changes by 300ms before dispatching
// setAuditFilter and refetching /api/audit (D6-03 + D6-05).

import { useEffect, useMemo, useRef, useState } from 'react';
import { Search, ShieldAlert } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { AuditFilter, AuditRow } from '../../types/eval';

const DEBOUNCE_MS = 300;

function buildQuery(filter: AuditFilter): string {
  const params = new URLSearchParams();
  if (filter.user) params.set('user', filter.user);
  if (filter.skill) params.set('skill', filter.skill);
  if (filter.refusal_reason) params.set('refusal_reason', filter.refusal_reason);
  if (filter.since) params.set('since', filter.since);
  if (filter.until) params.set('until', filter.until);
  if (filter.limit > 0) params.set('limit', String(filter.limit));
  if (filter.offset > 0) params.set('offset', String(filter.offset));
  return params.toString();
}

function isRedacted(value: string): boolean {
  // UAT 2026-09-06 B1 hardening — a wire-shape drift once fed `undefined`
  // in here (TypeError → whole-page white screen). A non-string can never
  // be redacted; degrade to false instead of crashing the subtree.
  return typeof value === 'string' && value.startsWith('[REDACTED:');
}

function RedactedBadge({ value }: { value: string }) {
  // Extract a human-friendly short label. The server sends
  // "[REDACTED:90d+ (ts=..., boundary=...)]" or similar.
  const short = isRedacted(value)
    ? value.replace(/^\[REDACTED:\s*/, '').replace(/\]$/, '').split(' ')[0]
    : value;
  return (
    <span
      data-testid="audit-row-redacted"
      title={value}
      className="inline-flex items-center gap-1 rounded-md bg-surface-3 px-2 py-0.5 text-xs text-text-muted"
    >
      REDACTED ({short})
    </span>
  );
}

export interface AuditTableProps {
  /** Optional rows override for tests. Defaults to store.auditRows. */
  rows?: AuditRow[];
}

export default function AuditTable({ rows }: AuditTableProps = {}) {
  const storeRows = useAppStore((s) => s.auditRows);
  const filter = useAppStore((s) => s.auditFilter);
  const setAuditFilter = useAppStore((s) => s.setAuditFilter);
  const setAuditRows = useAppStore((s) => s.setAuditRows);
  const displayedRows = rows ?? storeRows;

  // Local form state mirrors the store filter; debounced sync to store.
  const [local, setLocal] = useState<AuditFilter>(filter);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Keep local state in sync if store filter changes externally.
  useEffect(() => {
    setLocal(filter);
  }, [filter]);

  function updateField(field: keyof AuditFilter, value: string | number) {
    const next = { ...local, [field]: value };
    setLocal(next);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setAuditFilter(next);
      void fetchAudit(next);
    }, DEBOUNCE_MS);
  }

  async function fetchAudit(next: AuditFilter) {
    try {
      const qs = buildQuery(next);
      const resp = await fetch(`/api/audit?${qs}`, {
        headers: { Accept: 'application/json' },
      });
      if (!resp.ok) return;
      const data = (await resp.json()) as AuditRow[];
      setAuditRows(data);
    } catch {
      // Best-effort; the EvalDashboard polling loop also refreshes.
    }
  }

  // On mount, fetch with the current filter so the table is not empty
  // if the user navigated directly to /eval/audit.
  useEffect(() => {
    void fetchAudit(filter);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const refusalColor = (reason: string): string => {
    if (!reason) return 'text-text-muted';
    if (reason === 'resolved') return 'text-status-compliant';
    if (reason === 'needs_info') return 'text-status-needs-info';
    if (reason === 'non_compliant') return 'text-status-non-compliant';
    return 'text-status-unanswered';
  };

  return (
    <div className="flex h-full flex-col gap-3">
      {/* Filter bar */}
      <div
        data-testid="audit-filter-bar"
        className="grid grid-cols-1 gap-2 rounded-md border border-border bg-surface p-3 sm:grid-cols-2 lg:grid-cols-4"
      >
        <FilterInput
          label="User"
          value={local.user ?? ''}
          onChange={(v) => updateField('user', v)}
          testId="audit-filter-user"
        />
        <FilterInput
          label="Skill"
          value={local.skill ?? ''}
          onChange={(v) => updateField('skill', v)}
          testId="audit-filter-skill"
        />
        <FilterInput
          label="Refusal reason"
          value={local.refusal_reason ?? ''}
          onChange={(v) => updateField('refusal_reason', v)}
          testId="audit-filter-refusal-reason"
        />
        <FilterInput
          label="Since"
          value={local.since ?? ''}
          onChange={(v) => updateField('since', v)}
          testId="audit-filter-since"
        />
        <FilterInput
          label="Until"
          value={local.until ?? ''}
          onChange={(v) => updateField('until', v)}
          testId="audit-filter-until"
        />
        <FilterInput
          label="Limit"
          value={String(local.limit ?? 100)}
          onChange={(v) => updateField('limit', parseInt(v, 10) || 0)}
          testId="audit-filter-limit"
          type="number"
        />
        <FilterInput
          label="Offset"
          value={String(local.offset ?? 0)}
          onChange={(v) => updateField('offset', parseInt(v, 10) || 0)}
          testId="audit-filter-offset"
          type="number"
        />
      </div>

      {/* Table */}
      {displayedRows.length === 0 ? (
        <div
          data-testid="audit-table-empty"
          className="flex flex-1 flex-col items-center justify-center rounded-md border border-border bg-surface p-6 text-center text-sm text-text-muted"
        >
          <ShieldAlert className="mb-2 h-5 w-5 text-text-dim" />
          No audit rows match the current filter.
        </div>
      ) : (
        <div className="flex-1 overflow-auto rounded-md border border-border bg-surface">
          <table className="w-full text-left text-xs">
            <thead className="bg-surface-2 text-text-muted sticky top-0">
              <tr>
                <th className="px-3 py-2 font-semibold">Timestamp</th>
                <th className="px-3 py-2 font-semibold">User</th>
                <th className="px-3 py-2 font-semibold">Skill</th>
                <th className="px-3 py-2 font-semibold">Refusal reason</th>
                <th className="px-3 py-2 font-semibold">Query</th>
                <th className="px-3 py-2 font-semibold">Answer / Verdict</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {displayedRows.map((row, idx) => (
                <tr
                  key={`${row.ts}-${row.user_id}-${idx}`}
                  data-testid="audit-table-row"
                  data-user={row.user_id}
                  data-skill={row.skill_id}
                  className="hover:bg-surface-2"
                >
                  <td className="px-3 py-2 whitespace-nowrap text-text-muted">{row.ts}</td>
                  <td className="px-3 py-2 font-mono text-text">{row.user_id}</td>
                  <td className="px-3 py-2 text-text">{row.skill_id}</td>
                  <td className="px-3 py-2">
                    <span className={cn('text-xs font-semibold', refusalColor(row.refusal_reason))}>
                      {row.refusal_reason || '—'}
                    </span>
                  </td>
                  <td className="px-3 py-2 max-w-xs truncate">
                    {isRedacted(row.query) ? (
                      <RedactedBadge value={row.query} />
                    ) : (
                      <span className="text-xs text-text">{row.query}</span>
                    )}
                  </td>
                  <td className="px-3 py-2 max-w-xs truncate">
                    {isRedacted(row.verdict_json) ? (
                      <RedactedBadge value={row.verdict_json} />
                    ) : (
                      <span className="text-xs text-text">{row.verdict_json}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function FilterInput({
  label,
  value,
  onChange,
  testId,
  type = 'text',
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  testId: string;
  type?: string;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">
        {label}
      </label>
      <div className="relative flex items-center">
        <Search className="pointer-events-none absolute left-2 h-3 w-3 text-text-dim" aria-hidden />
        <input
          type={type}
          data-testid={testId}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-md border border-border bg-surface-1 py-1 pl-6 pr-2 text-xs text-text placeholder:text-text-dim"
        />
      </div>
    </div>
  );
}
