// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StatusBadge — D-06 verdict status-colored label (合规 / 需补充 /
// 不合规 / unanswered). Background + text color map per status; the
// `text-primary-fg` variants keep contrast on the saturated red/yellow
// backgrounds (otherwise dark-on-yellow is unreadable). Tailwind v4
// auto-generates `bg-status-compliant` etc. from the `@theme` block
// in styles/theme.css.
import { cn } from '../../lib/cn';
import type { VerdictStatus } from '../../types/verdict';

export const STATUS_LABELS: Record<VerdictStatus, string> = {
  合规: '合规',
  不合规: '不合规',
  需补充: '需补充',
  unanswered: 'unanswered',
};

const STATUS_CLASSES: Record<VerdictStatus, string> = {
  合规: 'bg-status-compliant text-status-compliant',
  不合规: 'bg-status-non-compliant text-primary-fg',
  需补充: 'bg-status-needs-info text-primary-fg',
  unanswered: 'bg-status-unanswered text-primary-fg',
};

export interface StatusBadgeProps {
  status: VerdictStatus;
  className?: string;
}

export default function StatusBadge({ status, className }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md px-2 py-0.5 font-mono text-xs font-semibold',
        STATUS_CLASSES[status],
        className,
      )}
      data-testid="status-badge"
      data-status={status}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}
