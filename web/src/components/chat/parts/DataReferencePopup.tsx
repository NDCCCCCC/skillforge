// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataReferencePopup — shared hover popup for single-value and sparkline-point
// hover. Renders value + period + as_of + channel name + KPI function name.
//
// DASH-02 per UI-SPEC §Position 2 copy layout:
//   4,231.5 亿元          ← value (--color-text)
//   周期：2026-07         ← period (--color-text-muted)
//   数据截至：2026-08-01  ← as_of (--color-text-muted)
//   来源：data:aggregated_only  ← channel (--color-text-dim)
//   函数：KPI_LOAN_BALANCE ← KPI name (mono)
//
// Phase 17 D17-B4: the SAME component renders for both single-value and
// sparkline-point hover — same fields, same layout, same CSS tokens.

interface PointReference {
  pointIndex: number;
  period: string;
  value: number;
  kMinApplied?: number;
}

interface DataReference {
  asOfPeriod?: string;
  asOfTs?: string;
  func?: string;
  references?: PointReference[];
}

interface DataReferencePopupProps {
  dataRef: DataReference;
  pointRef?: PointReference;
}

export function DataReferencePopup({ dataRef, pointRef }: DataReferencePopupProps) {
  const displayValue = pointRef
    ? Number(pointRef.value).toLocaleString()
    : Number(dataRef.asOfPeriod ?? 0).toLocaleString();
  // Phase 17 live-UAT fix: single-value snapshots carry an empty period
  // string (single-period KPI SQL has no period output column), so fall
  // through on empty — ?? alone treats '' as present.
  const period = pointRef?.period || dataRef.asOfPeriod || '—';
  const pointInfo =
    pointRef && dataRef.references
      ? `（点 ${pointRef.pointIndex + 1} / ${dataRef.references.length}）`
      : '';

  return (
    <div
      className="flex flex-col gap-1 p-2 min-w-0 max-w-[280px]"
      data-testid="data-ref-popup"
    >
      <span className="text-sm font-medium text-[var(--color-text)]">
        {displayValue}
        {pointInfo}
      </span>
      <span className="text-xs text-[var(--color-text-muted)]">
        周期：{period}
      </span>
      <span className="text-xs text-[var(--color-text-muted)]">
        数据截至：{dataRef.asOfTs ?? '—'}
      </span>
      <span className="text-xs text-[var(--color-text-dim)]">
        来源：data:aggregated_only
      </span>
      <span className="text-xs font-mono text-[var(--color-text-muted)] text-[10px]">
        函数：{dataRef.func ?? '—'}
      </span>
    </div>
  );
}
