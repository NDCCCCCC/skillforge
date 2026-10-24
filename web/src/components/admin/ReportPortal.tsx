// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 19-03 Task 2 — Report Portal FE (PORT-01/PORT-03/PORT-04).
//
// Responsibilities:
//   - List all report_runs rows via GET /reports with skill_id/period filter.
//   - Show drift badge (amber) when drift_detected=true.
//   - Show status badge colored by status (success/failed/skipped_duplicate/eval_gate_blocked).
//   - Manual rerun via POST /api/reports/rerender.
//   - No auth required (matches /eval pattern per PORT-01).
//
// Phase 19 report run status enum:
//   success | failed | skipped_duplicate | eval_gate_blocked

import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, ExternalLink } from 'lucide-react';

export interface ReportRun {
  run_id: string;
  skill_id: string;
  period: string;
  cron_expr: string;
  fired_at: string;
  kpi_snapshots: string;
  output_url: string;
  status: 'success' | 'failed' | 'skipped_duplicate' | 'eval_gate_blocked';
  drift_detected: boolean;
  created_at: string;
}

interface ReportPortalProps {
  /** Optional skill_id seed from URL / filter context */
  initialSkillId?: string;
  /** Optional period seed from URL / filter context */
  initialPeriod?: string;
}

export default function ReportPortal({ initialSkillId = '', initialPeriod = '' }: ReportPortalProps = {}) {
  const [runs, setRuns] = useState<ReportRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [skillFilter, setSkillFilter] = useState(initialSkillId);
  const [periodFilter, setPeriodFilter] = useState(initialPeriod);

  const fetchRuns = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (skillFilter) params.set('skill_id', skillFilter);
      if (periodFilter) params.set('period', periodFilter);
      const res = await fetch(`/api/reports?${params}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setRuns(Array.isArray(data) ? data : []);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'fetch failed');
    } finally {
      setLoading(false);
    }
  }, [skillFilter, periodFilter]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  async function handleRerun(skillId: string, period: string) {
    const res = await fetch('/api/reports/rerender', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ skill_id: skillId, period }),
    });
    if (res.ok) {
      void fetchRuns();
    }
  }

  return (
    <div className="flex h-full flex-col bg-bg text-text">
      {/* Top bar */}
      <header className="flex h-11 items-center justify-between border-b border-border bg-surface px-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold text-text">Report Portal</h1>
          <span className="inline-flex items-center gap-1 rounded-md bg-primary-dim px-2 py-0.5 font-mono text-[11px] text-primary">
            /reports
          </span>
        </div>
        <button
          type="button"
          onClick={() => void fetchRuns()}
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
        >
          <RefreshCw className="h-3.5 w-3.5" />
          <span>Refresh</span>
        </button>
      </header>

      {/* Filters */}
      <div className="flex items-center gap-2 border-b border-border bg-surface px-4 py-2">
        <input
          className="h-7 rounded-md border border-border bg-bg px-2 py-1 text-xs placeholder:text-text-muted focus:border-primary focus:outline-none"
          placeholder="Skill ID"
          value={skillFilter}
          onChange={(e) => setSkillFilter(e.target.value)}
        />
        <input
          className="h-7 rounded-md border border-border bg-bg px-2 py-1 text-xs placeholder:text-text-muted focus:border-primary focus:outline-none"
          placeholder="Period (e.g. 2026-08)"
          value={periodFilter}
          onChange={(e) => setPeriodFilter(e.target.value)}
        />
        <button
          type="button"
          onClick={() => void fetchRuns()}
          className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1 text-xs font-semibold text-white hover:bg-primary/90"
        >
          Filter
        </button>
        {(skillFilter || periodFilter) && (
          <button
            type="button"
            onClick={() => { setSkillFilter(''); setPeriodFilter(''); }}
            className="text-xs text-text-muted hover:text-text"
          >
            Clear
          </button>
        )}
      </div>

      {/* Body */}
      <main className="flex-1 overflow-y-auto p-4">
        {loading ? (
          <p className="text-sm text-text-muted">Loading...</p>
        ) : error ? (
          <p className="text-sm text-status-non-compliant">Error: {error}</p>
        ) : runs.length === 0 ? (
          <p className="text-sm text-text-muted">No report runs found</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse text-xs">
              <thead>
                <tr className="bg-surface-2 text-left text-text-muted">
                  <th className="border border-border px-3 py-2 font-medium">Skill</th>
                  <th className="border border-border px-3 py-2 font-medium">Period</th>
                  <th className="border border-border px-3 py-2 font-medium">Status</th>
                  <th className="border border-border px-3 py-2 font-medium">Drift</th>
                  <th className="border border-border px-3 py-2 font-medium">Fired At</th>
                  <th className="border border-border px-3 py-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr key={run.run_id} className="hover:bg-surface-2">
                    <td className="border border-border px-3 py-2 font-mono text-xs">{run.skill_id}</td>
                    <td className="border border-border px-3 py-2">{run.period}</td>
                    <td className="border border-border px-3 py-2">
                      <StatusBadge status={run.status} />
                    </td>
                    <td className="border border-border px-3 py-2">
                      {run.drift_detected ? (
                        <span className="inline-flex items-center gap-0.5 rounded-full bg-status-needs-info/15 px-1.5 py-0.5 text-[10px] font-semibold text-status-needs-info">
                          DRIFT
                        </span>
                      ) : (
                        <span className="text-text-muted">—</span>
                      )}
                    </td>
                    <td className="border border-border px-3 py-2">
                      {new Date(run.fired_at).toLocaleString()}
                    </td>
                    <td className="border border-border px-3 py-2">
                      <div className="flex items-center gap-3">
                        {run.output_url && (
                          <a
                            href={run.output_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-0.5 text-primary hover:underline"
                          >
                            <ExternalLink className="h-3 w-3" />
                            View
                          </a>
                        )}
                        <button
                          type="button"
                          onClick={() => void handleRerun(run.skill_id, run.period)}
                          className="text-primary hover:underline"
                        >
                          Rerun
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  );
}

function StatusBadge({ status }: { status: ReportRun['status'] }) {
  // admin UI redesign 2610 — light-on-dark status pills from the shared
  // token palette (replaces the light-theme green-100/red-100 set).
  const cls =
    status === 'success'
      ? 'bg-status-compliant/15 text-status-compliant'
      : status === 'failed'
      ? 'bg-status-non-compliant/15 text-status-non-compliant'
      : status === 'skipped_duplicate'
      ? 'bg-status-unanswered/15 text-status-unanswered'
      : 'bg-status-needs-info/15 text-status-needs-info';

  return (
    <span className={`inline-block rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${cls}`}>
      {status}
    </span>
  );
}
