// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SegmentedButton — shared admin toggle pill (admin UI redesign 2610).
//
// One copy of the segmented-control button previously duplicated between
// FeedbackMetricsPanel and ReviewQueuePanel. Active state uses the
// primary-dim/primary token pair; inactive uses text tokens only.

import { cn } from '../../../lib/cn';

export default function SegmentedButton({
  active,
  testId,
  label,
  onClick,
}: {
  active: boolean;
  testId: string;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={onClick}
      className={cn(
        'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
        active
          ? 'bg-primary-dim text-primary'
          : 'text-text-muted hover:bg-surface-2 hover:text-text',
      )}
    >
      {label}
    </button>
  );
}
