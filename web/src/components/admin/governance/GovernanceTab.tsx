// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07a — /admin/governance tab shell: HealthBanner + three-view
// switcher (校验报告 / 溯源图谱 / 版本演进). Stub view bodies are
// replaced by real implementations in plan 40-07b.
//
// View-switcher idiom: OntologyTab view-switcher (role="tablist" +
// aria-selected buttons, local state, no router). HealthBanner is
// shared across all three views (always visible above the switcher).

import { useEffect, useState } from 'react';
import { cn } from '../../../lib/cn';
import { fetchShaclReport, runPipeline } from './governanceApi';
import type { ShaclReport } from './types';
import ShaclReportView from './ShaclReportView';
import ProvenanceGraphView from './ProvenanceGraphView';
import TimelineView from './TimelineView';

type GovernanceView = 'shacl' | 'provenance' | 'timeline';

const VIEWS: Array<{ id: GovernanceView; label: string }> = [
  { id: 'shacl', label: '校验报告' },
  { id: 'provenance', label: '溯源图谱' },
  { id: 'timeline', label: '版本演进' },
];

// HealthBanner — shared sub-component. Also referenced by ShaclReportView
// for yellow-zone backfill target (provenance-missing rows).
export function HealthBanner({ report }: { report: ShaclReport }) {
  const dot = (
    <span
      className={cn(
        'h-2 w-2 rounded-full',
        report.health === 'error'
          ? 'bg-status-non-compliant'
          : report.health === 'warn'
            ? 'bg-status-needs-info'
            : 'bg-status-compliant',
      )}
    />
  );
  const label =
    report.health === 'error'
      ? `发现 error 级违规 — ${report.quarantined.length} 个实体已隔离`
      : report.health === 'warn'
        ? '存在 warning 级警告 — 数据已放行'
        : '校验通过 — 0 违规';

  const hasBackfill =
    report.health === 'warn' &&
    report.warnings.some((w) => w.constraint === 'provenance-missing');

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex items-center gap-3 rounded-md border border-border bg-surface-2 p-3"
    >
      {dot}
      <span className="text-sm font-semibold">{label}</span>
      <span className="font-mono text-xs text-text-muted">error {report.error_count}</span>
      <span className="font-mono text-xs text-text-muted">warning {report.warning_count}</span>
      {hasBackfill && (
        <button
          data-testid="governance-backfill-btn"
          onClick={() => {
            if (
              window.confirm(
                '运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行），并同时回填 provenance 与 belongsToTier。',
              )
            ) {
              runPipeline().catch(console.error);
            }
          }}
          className="ml-auto bg-primary px-3 py-1 text-xs font-semibold text-primary-fg rounded"
        >
          Run ECL 回填
        </button>
      )}
      <div className="ml-auto text-xs text-text-muted font-mono">
        校验时点 {new Date(report.validated_at).toLocaleString()}
        （{report.trigger === 'boot' ? '启动时' : '回滚后'}）· shapes{' '}
        {/* D-46-02: shapes_path is a join list post-46 — truncate guard keeps the banner one line (46-UI-SPEC §Component 5) */}
        <span
          className="inline-block max-w-[280px] truncate align-bottom"
          title={report.shapes_path}
        >
          {report.shapes_path}
        </span>
      </div>
    </div>
  );
}

export default function GovernanceTab() {
  const [view, setView] = useState<GovernanceView>('shacl');
  const [shaclReport, setShaclReport] = useState<ShaclReport | null>(null);
  const [shaclError, setShaclError] = useState<string | null>(null);

  useEffect(() => {
    fetchShaclReport()
      .then(setShaclReport)
      .catch((e: unknown) => setShaclError(e instanceof Error ? e.message : String(e)));
  }, []);

  // Single refetch implementation shared by the manual retry button and the
  // post-rollback notification (REL-02 / D-02b). It never rethrows: a failed
  // refresh is surfaced through shaclError, because the caller (TimelineView's
  // success path) has no way to handle a rejection — the rollback itself
  // already succeeded at that point.
  const refreshShaclReport = () => {
    // WR-07: a successful refetch must clear the stale error —
    // ShaclReportView renders the failure branch whenever error is
    // truthy, so leaving it set kept the view stuck on
    // 「校验报告加载失败」 even after the refetch succeeded.
    fetchShaclReport()
      .then((r) => {
        setShaclError(null);
        setShaclReport(r);
      })
      .catch((e: unknown) => setShaclError(e instanceof Error ? e.message : String(e)));
  };

  const healthBanner = shaclReport ? (
    <HealthBanner report={shaclReport} />
  ) : shaclError ? (
    <div className="text-sm text-danger">治理状态加载失败（点击重试）</div>
  ) : (
    <div className="h-10 animate-pulse rounded bg-surface-2" />
  );

  return (
    <div className="space-y-4" data-testid="governance-tab">
      {/* Health banner */}
      <div data-testid="governance-health-banner">{healthBanner}</div>

      {/* View switcher */}
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            type="button"
            role="tab"
            aria-selected={view === v.id}
            data-testid={`governance-view-${v.id}`}
            onClick={() => setView(v.id)}
            className={cn(
              'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
              view === v.id ? 'bg-primary-dim text-primary' : 'text-text-muted hover:bg-surface-2 hover:text-text',
            )}
          >
            {v.label}
          </button>
        ))}
      </div>

      {/* View bodies — real implementations from 40-07b */}
      {view === 'shacl' && (
        <ShaclReportView
          report={shaclReport}
          error={shaclError}
          onRetry={refreshShaclReport}
        />
      )}
      {view === 'provenance' && <ProvenanceGraphView />}
      {view === 'timeline' && <TimelineView onRollbackComplete={refreshShaclReport} />}
    </div>
  );
}
