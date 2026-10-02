// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataReferenceChip — inline number hover wrapper for KPI data-reference.
//
// DASH-01 / DASH-02 per UI-SPEC:
//   - N>1 points (sparkline mode): renders Sparkline inline (pure SVG,
//     zero chart deps — 17-01)
//   - 0 or 1 point (single-value mode): renders inline span with CSS hover popup
//
// Phase 17 D17-B4: single-value and sparkline-point hover share the same
// DataReferencePopup component — same fields, same copy, same CSS tokens.
// Tooltip implementation: CSS-only hover (no new shadcn blocks per UI-SPEC
// §Registry Safety: "zero new shadcn blocks — reuse existing tooltip badge").
import type { DataReference } from '@/types/wire-types';
import { DataReferencePopup } from './DataReferencePopup';
import { Sparkline } from './Sparkline';

interface DataReferenceChipProps {
  dataRef: DataReference;
}

export function DataReferenceChip({ dataRef }: DataReferenceChipProps) {
  const refs = dataRef.references ?? [];
  const hasSparkline = refs.length > 1;

  if (hasSparkline) {
    // Sparkline mode (N>1 points): pure SVG multi-period trend, one
    // hoverable circle per references[] entry (17-01 component, DASH-03).
    return (
      <span
        className="kpi-sparkline-wrapper inline-flex flex-col gap-1"
        data-testid="kpi-sparkline-ref"
      >
        <Sparkline
          points={refs.map((r, i) => ({
            index: i,
            period: r.period,
            value: Number(r.value),
          }))}
          onPointHover={(i) => {
            // Point-level tooltip shares the same DataReferencePopup
            // copy as the single-value mode (D17-B4); the hover popup
            // is driven by the chat-message-level pointRef wiring.
          }}
        />
      </span>
    );
  }

  // Single-value: inline span with CSS hover tooltip
  const ref = refs[0];
  const displayValue = ref ? Number(ref.value).toLocaleString() : '—';

  return (
    <span
      className="kpi-value-inline-group group relative inline cursor-help"
      data-testid="kpi-value-wrapper"
    >
      <span
        className="underline decoration-dotted"
        data-testid="kpi-value"
      >
        {displayValue}
      </span>
      {/* CSS hover tooltip — appears above the value on hover */}
      <span
        className="kpi-popup absolute bottom-full left-1/2 z-50 mb-1 hidden -translate-x-1/2 group-hover:block"
        data-testid="kpi-tooltip"
      >
        <DataReferencePopup dataRef={dataRef} pointRef={ref} />
      </span>
    </span>
  );
}
