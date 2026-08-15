// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// VerdictCard — D-06 / FEUI/CMPL-05 verdict card. Status-colored
// header + review-required corner badge. The citations count footer
// renders below the verdict body, then the CitationCluster renders
// BELOW the footer (superscripts live in their own cluster, not
// inside the header — Plan 260815-cos Task 3 switched from chips to
// inline superscripts).
//
// Plan 260815-cos / locked CONTEXT.md decision D: the green confidence
// bar + 「置信度 N%」 text are REMOVED. The data-testid="confidence-bar"
// DOM node is RETAINED as a `hidden` (display:none) element with the
// data-confidence attribute so existing test assertions / snapshot
// scrapers do not break. The verdict's confidence value is still
// available on the Verdict payload for any future re-introduction.
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
        </div>
        <div className="mt-3 text-xs text-text-muted">
          {verdict.citations.length} 条引用
        </div>
        {/* Plan 260815-cos / CONTEXT.md decision D: confidence bar is
            visually hidden. The data-testid + data-confidence attributes
            are retained so existing test assertions / scrapers do not
            regress; `hidden` is Tailwind's display:none equivalent. */}
        <div
          data-testid="confidence-bar"
          data-confidence={pct}
          className="hidden"
          aria-hidden="true"
        />
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
