// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DrillChip — Phase 17 D17-C1 drill-down chip.
//
// Renders a Badge with KPI-chip styling. onClick calls setPendingDraft
// with the pre-fill text so ChatInput can consume it on the next render.
//
// Chip only renders when the KPI supports multi-period calls (per KPI
// definition file). Default label is "看月度趋势" per UI-SPEC §Position 3.
import { Badge } from '@/components/ui/badge';
import { useAppStore } from '@/store/app-store';

interface DrillChipProps {
  /** KPI function name, e.g. "KPI_LOAN_BALANCE" */
  kpiName: string;
  /** Period template for the drill query, e.g. "最近12月" */
  periodsTemplate: string;
  /** Chip label. Defaults to "看月度趋势" per UI-SPEC §Position 3. */
  label?: string;
}

export function DrillChip({
  kpiName,
  periodsTemplate,
  label = '看月度趋势',
}: DrillChipProps) {
  const setPendingDraft = useAppStore((s) => s.setPendingDraft);

  const handleClick = () => {
    setPendingDraft(`请调 ${kpiName} periods=${periodsTemplate}`);
  };

  return (
    <Badge
      variant="default"
      className="kpi-chip cursor-pointer"
      data-testid="drill-chip"
      onClick={handleClick}
      style={{
        background: 'var(--color-kpi-chip-bg, rgba(74, 158, 255, 0.12))',
        border: '1px solid var(--color-kpi-chip-border, rgba(74, 158, 255, 0.3))',
      }}
    >
      {label}
    </Badge>
  );
}
