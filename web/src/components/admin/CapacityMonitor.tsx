// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-01 (ADM-01) — admin console capacity monitor.
//
// Renders 3 capacity bars (audit / kpi_calls / report_runs) with:
//   - Per-row progress bar (normal = bg-primary, breach = bg-danger)
//   - count/threshold counter (normal = text-text-muted, breach = text-danger + bold)
//   - Red "!" breach badge when count >= threshold
//
// Thresholds are fetched from GET /api/admin/table-counts (env-configurable
// on the BE). Read-only — no user interaction on the bars themselves.

import { useEffect, useState } from 'react';

interface TableCapacity {
  count: number;
  threshold: number;
}

interface TableCounts {
  audit: TableCapacity;
  kpi_calls: TableCapacity;
  report_runs: TableCapacity;
}

function CapacityBar({
  label,
  current,
  threshold,
  compact = false,
}: {
  label: string;
  current: number;
  threshold: number;
  compact?: boolean;
}) {
  const pct = threshold > 0 ? Math.min((current / threshold) * 100, 100) : 0;
  const breach = current >= threshold;
  const ariaLabel = breach
    ? `${label} capacity: ${current.toLocaleString()} of ${threshold.toLocaleString()} — BREACH`
    : `${label} capacity: ${current.toLocaleString()} of ${threshold.toLocaleString()}`;

  return (
    <div className="flex items-center gap-2">
      <span className={compact ? 'w-16 flex-shrink-0 text-[10px] text-text-muted' : 'w-20 flex-shrink-0 text-xs text-text-muted'}>{label}</span>
      <div
        role="meter"
        aria-valuenow={current}
        aria-valuemin={0}
        aria-valuemax={threshold}
        aria-label={ariaLabel}
        className="flex flex-1 items-center gap-2"
      >
        <div className="flex-1 h-1.5 bg-surface-2 rounded">
          <div
            className={`h-full rounded transition-all ${breach ? 'bg-danger' : 'bg-primary'}`}
            style={{ width: `${pct}%` }}
          />
        </div>
        <span className={`text-[10px] font-mono ${breach ? 'text-danger font-bold' : 'text-text-muted'}`}>
          {compact ? `${current.toLocaleString()}` : `${current.toLocaleString()}/${threshold.toLocaleString()}`}
        </span>
        {breach && (
          <span className="bg-danger text-white text-[10px] px-1 rounded font-bold" aria-label="breach indicator">
            !
          </span>
        )}
      </div>
    </div>
  );
}

export default function CapacityMonitor({ compact = false }: { compact?: boolean }) {
  const [counts, setCounts] = useState<TableCounts | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    // The admin token is read from localStorage on the FE side — the BE
    // validates X-Admin-Token header via isAdminRequest (constant-time compare).
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/table-counts', {
      headers: {
        'X-Admin-Token': token,
      },
    })
      .then((resp) => {
        if (!resp.ok) {
          setError(true);
          return null;
        }
        return resp.json() as Promise<TableCounts>;
      })
      .then((data) => {
        if (data) setCounts(data);
      })
      .catch(() => setError(true));
  }, []);

  if (error) {
    return (
      <div className="flex items-center gap-2 text-xs text-danger">
        <span>容量数据加载失败</span>
      </div>
    );
  }

  if (!counts) {
    return (
      <div className="flex flex-col gap-1.5">
        <CapacityBar label="audit" current={0} threshold={10000} />
        <CapacityBar label="kpi_calls" current={0} threshold={50000} />
        <CapacityBar label="report_runs" current={0} threshold={5000} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      <CapacityBar compact={compact} label="audit" current={counts.audit.count} threshold={counts.audit.threshold} />
      <CapacityBar compact={compact} label="kpi_calls" current={counts.kpi_calls.count} threshold={counts.kpi_calls.threshold} />
      <CapacityBar compact={compact} label="report_runs" current={counts.report_runs.count} threshold={counts.report_runs.threshold} />
    </div>
  );
}
