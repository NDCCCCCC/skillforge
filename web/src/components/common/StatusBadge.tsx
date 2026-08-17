// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StatusBadge — verdict status-colored label. 260817-vsm: now renders the
// platform-neutral Status (ok / needs_input / refused / error) as the
// badge text, with ComplianceStatus context surfacing via a separate
// component (`ComplianceStatusBadge`) when the Compliance Skill's
// verdict is relevant.
//
// Background + text color map per status; the `text-primary-fg` variants
// keep contrast on saturated red/yellow backgrounds (otherwise dark-on-
// yellow is unreadable). Tailwind v4 auto-generates `bg-status-compliant`
// etc. from the `@theme` block in styles/theme.css.
import { cn } from '../../lib/cn';
import type { VerdictStatus } from '../../types/verdict';

// STATUS_LABELS renders the platform Status as readable Chinese text.
// 260817-vsm: was compliance-specific (合规 / 不合规 / 需补充 / unanswered);
// now platform-neutral. The Compliance Skill's specific verdict
// (compliant / non_compliant / not_applicable) is rendered by
// ComplianceStatusBadge when compliance_status is set.
export const STATUS_LABELS: Record<VerdictStatus, string> = {
  ok: '已给出',
  needs_input: '需补充',
  refused: '拒答',
  error: '错误',
};

const STATUS_CLASSES: Record<VerdictStatus, string> = {
  // ok → green (was "合规"); needs_input → yellow (was "需补充");
  // refused → gray (was "unanswered"); error → deeper red (new).
  ok: 'bg-status-compliant text-primary-fg',
  needs_input: 'bg-status-needs-info text-primary-fg',
  refused: 'bg-status-unanswered text-primary-fg',
  error: 'bg-status-error text-primary-fg',
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
