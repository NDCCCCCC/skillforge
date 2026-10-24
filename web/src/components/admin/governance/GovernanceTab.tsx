// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07a — /admin/governance tab shell: HealthBanner + three-view
// switcher (校验报告 / 溯源图谱 / 版本演进). Real view bodies landed in
// plan 40-07b.
//
// View-switcher idiom: OntologyTab view-switcher (role="tablist" +
// aria-selected buttons, local state, no router). HealthBanner is
// shared across all three views (always visible above the switcher).
//
// 2610 redesign: the SHACL report one-shot load migrated to the shared
// useAdminData hook (admin token handled centrally); the switcher
// buttons migrated to the shared Button primitive; header migrated to
// PageHeader. The BE nil-slice normalization (quick-260928-jxb BUG-1)
// is re-applied client-side on the hook result.

import { useMemo, useState } from 'react';
import { cn } from '../../../lib/cn';
import { useAdminData } from '../shared/useAdminData';
import { PageHeader } from '../shared/Primitives';
import { Button } from '../../ui/button';
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

// quick-260928-jxb BUG-1: Go nil slices serialize to JSON null while the
// FE types claim arrays — normalize so view components never see null
// (warnings:null white-screened 治理 tab). useAdminData hands back the raw
// payload, so the choke point moved here from fetchShaclReport.
function normalizeReport(r: ShaclReport): ShaclReport {
  return { ...r, warnings: r.warnings ?? [], quarantined: r.quarantined ?? [] };
}

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
        <Button
          size="sm"
          data-testid="governance-backfill-btn"
          onClick={() => {
            if (
              window.confirm(
                '运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行），并同时回填 provenance 与 belongsToTier。',
              )
            ) {
              void runPipeline().catch(console.error);
            }
          }}
          className="ml-auto"
        >
          Run ECL 回填
        </Button>
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
  const {
    data: rawData,
    loading: shaclLoading,
    error: shaclHookError,
    reload: reloadShacl,
  } = useAdminData<ShaclReport>('/api/admin/governance/shacl');

  const shaclReport = useMemo(
    () => (rawData ? normalizeReport(rawData) : null),
    [rawData],
  );
  // Hook error is a boolean flag; the view layer wants a message string.
  const shaclError = shaclHookError ? '治理状态加载失败' : null;

  // Single refetch implementation shared by the manual retry button and the
  // post-rollback notification (REL-02 / D-02b). Delegates to the hook's
  // reload: a failed refresh surfaces through the hook error flag, because
  // the caller (TimelineView's success path) has no way to handle a
  // rejection — the rollback itself already succeeded at that point. A
  // successful refetch clears the stale error (WR-07: ShaclReportView
  // renders the failure branch whenever error is truthy).
  const refreshShaclReport = reloadShacl;

  const healthBanner = shaclReport ? (
    <HealthBanner report={shaclReport} />
  ) : shaclError ? (
    <div className="text-sm text-danger">
      治理状态加载失败（
      <button onClick={reloadShacl} className="underline">
        点击重试
      </button>
      ）
    </div>
  ) : (
    <div className="h-10 animate-pulse rounded bg-surface-2" />
  );

  return (
    <div className="space-y-4" data-testid="governance-tab">
      <PageHeader
        title="治理"
        description="SHACL 校验、溯源图谱与 TBox 版本演进的统一治理入口"
      />

      {/* Health banner */}
      <div data-testid="governance-health-banner">{healthBanner}</div>

      {/* View switcher */}
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map((v) => (
          <Button
            key={v.id}
            type="button"
            role="tab"
            aria-selected={view === v.id}
            data-testid={`governance-view-${v.id}`}
            variant={view === v.id ? 'secondary' : 'ghost'}
            size="sm"
            onClick={() => setView(v.id)}
            className={cn(
              view === v.id
                ? 'bg-primary-dim text-primary hover:bg-primary-dim'
                : 'text-text-muted hover:text-text',
            )}
          >
            {v.label}
          </Button>
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
