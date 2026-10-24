// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07b — SHACL validation report: quarantine (red) + warnings (yellow).
// Three-color mapping per D-SHACL-03: red=error (quarantined), yellow=warn-only,
// green=ok. HealthBanner is imported from GovernanceTab (exported in 07a).
//
// Plan 46-04 — layered rendering per 46-UI-SPEC: layer/target_class badges
// (D-46-03), zone-heading per-layer count chips, dangling-edge column
// (D-46-04). FE renders the BE-stamped fields verbatim — zero frontend
// layer derivation (Honesty red line 9), 0 dangling edges render as 0
// (Honesty red line 8).
//
// 2610 redesign: the hand-rolled skeleton rows migrated to the shared
// LoadingSkeleton. The zone tables stay purpose-built (not DataTable)
// because the regression suites pin row-level data-testid
// (`quarantine-row-${i}`) and cell order.

import { cn } from '../../../lib/cn';
import { LoadingSkeleton } from '../shared/States';
import type { ShaclReport, ShaclViolation, ViolationLayer } from './types';

// Reuse GovernanceTab's HealthBanner for yellow-zone backfill target
export { HealthBanner } from './GovernanceTab';

// Layer badge colors per 46-UI-SPEC §Color (D-46-03): classification family,
// not severity — severity 徽章与 layer 徽章语义正交。global is the cross-entity
// sentinel: dashed border, transparent bg, no target_class chip.
const LAYER_BADGE: Record<string, { classes: string; label: string }> = {
  tbox: { classes: 'bg-primary-dim text-primary', label: 'TBox' },
  entity: { classes: 'bg-surface-3 text-text-muted', label: '实体' },
  global: { classes: 'border border-dashed border-border text-text-muted', label: '跨实体' },
};

function LayerBadge({ layer }: { layer: ViolationLayer }) {
  // Unknown/future layer values render the raw string with entity's neutral
  // color — console honesty: 不吞不猜 (46-UI-SPEC §Color table row 4).
  const badge = LAYER_BADGE[layer] ?? { classes: LAYER_BADGE.entity.classes, label: layer };
  return (
    <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', badge.classes)}>
      {badge.label}
    </span>
  );
}

// Constraint cell = layer badge → target_class chip → constraint mono (+extra)
// (46-UI-SPEC §Component 2). Missing layer/target_class (legacy reports /
// global sentinel) simply don't render their element. All content renders as
// React text nodes (default escaping) — no raw-HTML sink anywhere in this
// file (T-46-02 XSS mitigation, grep-gated).
function ConstraintCell({ v, extra = 0 }: { v: ShaclViolation; extra?: number }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      {v.layer && <LayerBadge layer={v.layer} />}
      {v.target_class && (
        <span className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-text-muted">
          {v.target_class}
        </span>
      )}
      <span className="font-mono">{v.constraint}</span>
      {extra > 0 && <span className="text-text-muted">+{extra}</span>}
    </span>
  );
}

// Per-layer violation counts for zone headings (46-UI-SPEC §Component 3):
// non-zero layers only, fixed order TBox · 实体 · 跨实体. Legacy reports
// without stamps render no chips — nothing is inferred frontend-side
// (Honesty red line 9).
function layerCounts(violations: ShaclViolation[]): string[] {
  const counts: Record<string, number> = {};
  for (const v of violations) {
    if (v.layer) counts[v.layer] = (counts[v.layer] ?? 0) + 1;
  }
  return (
    [
      { layer: 'tbox', label: 'TBox' },
      { layer: 'entity', label: '实体' },
      { layer: 'global', label: '跨实体' },
    ] as const
  )
    .map(({ layer, label }) => ({ label, count: counts[layer] ?? 0 }))
    .filter((p) => p.count > 0)
    .map((p) => `${p.label} ${p.count}`);
}

function LayerCountChips({ violations, testId }: { violations: ShaclViolation[]; testId: string }) {
  const parts = layerCounts(violations);
  if (parts.length === 0) return null;
  return (
    <span className="ml-2 font-mono text-xs font-normal text-text-muted" data-testid={testId}>
      {parts.join(' · ')}
    </span>
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
    return <LoadingSkeleton rows={3} testId="shacl-skeleton" />;
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
            <LayerCountChips
              violations={report.quarantined.flatMap((qe) => qe.violations)}
              testId="governance-quarantine-layer-counts"
            />
          </h2>
          <div className="overflow-hidden rounded-md border border-status-non-compliant/40">
            <table className="w-full text-left text-xs">
              <thead className="bg-surface-2 text-text-muted">
                <tr>
                  <th className="px-3 py-2">实体</th>
                  <th className="px-3 py-2">三元组</th>
                  <th
                    className="px-3 py-2"
                    title="指向该隔离实体的入边数量（在 KG 查询对象位仍可见）"
                  >
                    悬挂边
                  </th>
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
                      {/* 悬挂边 — D-46-04: 0 renders as 0, missing renders as — (Honesty red line 8) */}
                      <td className="px-3 py-1.5 font-mono">
                        {qe.dangling_edge_count != null ? qe.dangling_edge_count : '—'}
                      </td>
                      {/* 约束 */}
                      <td className="px-3 py-1.5">
                        <ConstraintCell v={first} extra={extra} />
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
            隔离实体不进入主索引（查询 subject 位置不可见）；指向它的入边在对象位仍可见（悬挂边列）。可修复数据后通过 Run ECL 重建，或待下次启动重新校验。
          </p>
        </section>
      )}

      {/* ── Yellow zone: warnings table ───────────────────────────── */}
      {report.warnings.length > 0 && (
        <section data-testid="governance-warnings-zone">
          <h2 className="mb-2 text-sm font-semibold text-status-needs-info">
            警告（数据已放行）
            <LayerCountChips
              violations={report.warnings}
              testId="governance-warnings-layer-counts"
            />
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
                        <ConstraintCell v={w} />
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
