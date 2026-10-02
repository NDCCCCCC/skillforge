// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ComplianceStatusBadge — Compliance-Skill-specific verdict badge.
// 260817-vsm: separated from StatusBadge so the Compliance Skill's
// verdict (compliant / non_compliant / not_applicable) can render
// alongside the platform-neutral StatusBadge without conflating the two
// semantic axes.
//
// Rendered inside VerdictCard only when Verdict.compliance_status is
// non-empty AND Verdict.answer is empty (factual queries lead with
// the answer text; the ComplianceStatus="not_applicable" badge adds
// no signal there).
import { cn } from '../../lib/cn';
import type { ComplianceStatus } from '../../types/verdict';

export const COMPLIANCE_STATUS_LABELS: Record<ComplianceStatus, string> = {
  compliant: '合规',
  non_compliant: '不合规',
  not_applicable: '非合规审查',
};

const COMPLIANCE_STATUS_CLASSES: Record<ComplianceStatus, string> = {
  compliant: 'bg-status-compliant text-primary-fg',
  non_compliant: 'bg-status-non-compliant text-primary-fg',
  not_applicable: 'bg-status-unanswered text-primary-fg',
};

export interface ComplianceStatusBadgeProps {
  status: ComplianceStatus;
  className?: string;
}

export default function ComplianceStatusBadge({ status, className }: ComplianceStatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md px-2 py-0.5 font-mono text-xs font-semibold',
        COMPLIANCE_STATUS_CLASSES[status],
        className,
      )}
      data-testid="compliance-status-badge"
      data-compliance-status={status}
    >
      {COMPLIANCE_STATUS_LABELS[status]}
    </span>
  );
}
