// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07b — SHACL validation report: quarantine (red) + warnings (yellow).
// Three-color mapping per D-SHACL-03: red=error (quarantined), yellow=warn-only,
// green=ok. HealthBanner is imported from GovernanceTab (exported in 07a).

import { cn } from '../../../lib/cn';
import type { ShaclReport, ShaclViolation } from './types';

// Reuse GovernanceTab's HealthBanner for yellow-zone backfill target
export { HealthBanner } from './GovernanceTab';

// Severity badge colors per UI-SPEC §Color
const SEVERITY_BADGE: Record<string, string> = {
  error: 'bg-status-non-compliant/10 text-status-non-compliant',
  warning: 'bg-status-needs-info/10 text-status-needs-info',
};

function ViolationBadge({ severity }: { severity: ShaclViolation['severity'] }) {
  return (
    <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', SEVERITY_BADGE[severity])}>
      {severity === 'error' ? '错误' : '警告'}
    </span>
  );
}

// Skeleton loader
function SkeletonRows() {
  return (
    <>
      {[1, 2, 3].map((i) => (
        <tr key={i}>
          <td colSpan={6} className="px-3 py-2">
            <div className="h-10 animate-pulse rounded bg-surface-2" />
          </td>
        </tr>
      ))}
    </>
  );
}

interface ShaclReportViewProps {
  report: ShaclReport | null;
  error?: string | null;
  onRetry?: () => void;
}

export default function ShaclReportView({ report, error, onRetry }: ShaclReportViewProps) {
  if (error) {
    return (
      <div className="text-sm text-danger">
        校验报告加载失败
        {onRetry && (
          <button
            onClick={onRetry}
            className="ml-2 underline"
            data-testid="shacl-retry-btn"
          >
            （点击重试）
          </button>
        )}
      </div>
    );
  }

  if (!report) {
    return (
      <table className="w-full text-left text-xs">
        <tbody data-testid="shacl-skeleton">
          <SkeletonRows />
        </tbody>
      </table>
    );
  }

  // ok state — no tables, single muted line
  if (report.health === 'ok') {
    return (
      <div className="text-sm text-text-muted">
        全部 ABox triple 通过 SHACL 校验（{report.triple_count} 条）
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* ── Red zone: quarantine table ─────────────────────────────── */}
      {report.quarantined.length > 0 && (
        <section data-testid="governance-quarantine-zone">
          <h2 className="mb-2 text-sm font-semibold text-status-non-compliant">
            隔离区（error 级违规实体）
          </h2>
          <div className="overflow-hidden rounded-md border border-status-non-compliant/40">
            <table className="w-full text-left text-xs">
              <thead className="bg-surface-2 text-text-muted">
                <tr>
                  <th className="px-3 py-2">实体</th>
                  <th className="px-3 py-2">三元组</th>
                  <th className="px-3 py-2">约束</th>
                  <th className="px-3 py-2">违规详情</th>
                  <th className="px-3 py-2">来源</th>
                  <th className="px-3 py-2">行号</th>
                </tr>
              </thead>
              <tbody>
                {report.quarantined.map((qe, qi) => {
                  const first = qe.violations[0];
                  const extra = qe.violations.length - 1;
                  return (
                    <tr
                      key={qe.entity}
                      data-testid={`quarantine-row-${qi}`}
                      className="border-t border-border hover:bg-surface-2"
                    >
                      {/* 实体 */}
                      <td className="px-3 py-1.5 font-mono break-all text-danger">
                        {qe.entity}
                      </td>
                      {/* 三元组 */}
                      <td className="px-3 py-1.5 font-mono">{qe.triple_count}</td>
                      {/* 约束 */}
                      <td className="px-3 py-1.5 font-mono">
                        {first.constraint}
                        {extra > 0 && (
                          <span className="ml-1 text-text-muted">+{extra}</span>
                        )}
                      </td>
                      {/* 违规详情 */}
                      <td className="px-3 py-1.5 max-w-xs truncate" title={first.detail}>
                        {first.detail}
                      </td>
                      {/* 来源 */}
                      <td className="px-3 py-1.5 font-mono truncate max-w-xs" title={first.source_path ?? ''}>
                        {first.source_path ?? '—'}
                      </td>
                      {/* 行号 */}
                      <td className="px-3 py-1.5 font-mono">
                        {first.line_start != null
                          ? `L${first.line_start}–L${first.line_end ?? first.line_start}`
                          : '—'}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="mt-2 text-xs text-text-muted">
            隔离实体不进入主索引；可修复数据后通过 Run ECL 重建，或待下次启动重新校验。
          </p>
        </section>
      )}

      {/* ── Yellow zone: warnings table ───────────────────────────── */}
      {report.warnings.length > 0 && (
        <section data-testid="governance-warnings-zone">
          <h2 className="mb-2 text-sm font-semibold text-status-needs-info">
            警告（数据已放行）
          </h2>
          <div className="overflow-hidden rounded-md border border-status-needs-info/40">
            <table className="w-full text-left text-xs">
              <thead className="bg-surface-2 text-text-muted">
                <tr>
                  <th className="px-3 py-2">实体</th>
                  <th className="px-3 py-2">约束</th>
                  <th className="px-3 py-2">违规详情</th>
                  <th className="px-3 py-2">来源</th>
                  <th className="px-3 py-2">行号</th>
                </tr>
              </thead>
              <tbody>
                {report.warnings.map((w, wi) => (
                  <tr
                    key={`${w.entity}-${w.constraint}-${wi}`}
                    data-testid={`warning-row-${wi}`}
                    className="border-t border-border hover:bg-surface-2"
                  >
                    {/* 实体 */}
                    <td className="px-3 py-1.5 font-mono break-all">{w.entity}</td>
                    {/* 约束 */}
                    <td className="px-3 py-1.5">
                      {w.constraint === 'provenance-missing' ? (
                        <span className="rounded bg-status-needs-info/10 px-1.5 py-0.5 text-[11px] font-semibold text-status-needs-info">
                          provenance 缺失（存量数据）
                        </span>
                      ) : (
                        <span className="font-mono">{w.constraint}</span>
                      )}
                    </td>
                    {/* 违规详情 */}
                    <td className="px-3 py-1.5 max-w-xs truncate" title={w.detail}>
                      {w.detail}
                    </td>
                    {/* 来源 */}
                    <td className="px-3 py-1.5 font-mono truncate max-w-xs" title={w.source_path ?? ''}>
                      {w.source_path ?? '—'}
                    </td>
                    {/* 行号 */}
                    <td className="px-3 py-1.5 font-mono">
                      {w.line_start != null
                        ? `L${w.line_start}–L${w.line_end ?? w.line_start}`
                        : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
}
