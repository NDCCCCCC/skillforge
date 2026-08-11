// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// VerdictCard — D-06 / FEUI/CMPL-05 verdict card. Status-colored
// header + confidence bar + review-required corner badge. The
// citations count footer renders below the verdict body, then the
// CitationCluster renders BELOW the footer (chips live in their own
// clickable cluster, not inside the header).
//
// Wave 2 simplifies the signature to `{ verdict }` only — the
// citation click handler invokes useAppStore.getState() directly
// (Zustand is a singleton; no prop drilling required).
import { cn } from '../../lib/cn';
import type { Verdict, VerdictStatus } from '../../types/verdict';
import StatusBadge from '../common/StatusBadge';
import CitationCluster from './CitationCluster';

const STATUS_BORDER: Record<VerdictStatus, string> = {
  合规: 'border-l-status-compliant',
  不合规: 'border-l-status-non-compliant',
  需补充: 'border-l-status-needs-info',
  unanswered: 'border-l-status-unanswered',
};

const STATUS_BAR: Record<VerdictStatus, string> = {
  合规: 'bg-status-compliant',
  不合规: 'bg-status-non-compliant',
  需补充: 'bg-status-needs-info',
  unanswered: 'bg-status-unanswered',
};

export interface VerdictCardProps {
  verdict: Verdict;
}

export default function VerdictCard({ verdict }: VerdictCardProps) {
  const pct = Math.round(verdict.confidence * 100);
  return (
    <div className="relative">
      <div
        data-testid="verdict-card"
        data-verdict-status={verdict.status}
        data-review-required={String(verdict.review_required)}
        className={cn(
          'my-2 rounded-lg border-0 border-l-4 bg-surface pl-4 pr-5 py-3 text-text',
          STATUS_BORDER[verdict.status],
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <StatusBadge status={verdict.status} />
          <span className="font-mono text-xs">置信度 {pct}%</span>
        </div>
        <div className="mt-2 h-0.5 w-full rounded-full bg-border-muted">
          <div
            data-testid="confidence-bar"
            className={cn('h-0.5 rounded-full transition-all', STATUS_BAR[verdict.status])}
            style={{ width: `${pct}%` }}
          />
        </div>
        <div className="mt-3 text-xs text-text-muted">
          {verdict.citations.length} 条引用
        </div>
      </div>
      {verdict.review_required && (
        <span
          data-testid="review-required-badge"
          className="absolute -top-2 right-3 rounded-full bg-status-needs-info px-2 py-0.5 text-xs font-semibold text-primary-fg"
        >
          需人工复核
        </span>
      )}
      <CitationCluster citations={verdict.citations} />
    </div>
  );
}
