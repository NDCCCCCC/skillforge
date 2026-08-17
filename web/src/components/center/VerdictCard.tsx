// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// VerdictCard — D-06 / FEUI/CMPL-05 verdict card. 260817-vsm: leads with
// the direct answer for factual queries (Verdict.answer); falls back to
// the Compliance-Skill verdict badge (compliance_status) for compliance
// reviews. The platform StatusBadge rides along as a secondary chip.
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
import ComplianceStatusBadge from '../common/ComplianceStatusBadge';
import CitationCluster from './CitationCluster';

// Border color maps the platform Status (not ComplianceStatus) so the
// card frame reflects "did we produce a grounded answer?" — the
// Compliance-Skill-specific verdict code shows up on the badge inside.
// 260817-vsm: was keyed by compliance-Skill-specific Status values
// (合规 / 不合规 / 需补充 / unanswered); now platform-neutral.
const STATUS_BORDER: Record<VerdictStatus, string> = {
  ok: 'border-l-status-compliant',         // grounded answer (factual or compliant verdict)
  needs_input: 'border-l-status-needs-info',
  refused: 'border-l-status-unanswered',
  error: 'border-l-status-error',
};

export interface VerdictCardProps {
  verdict: Verdict;
}

export default function VerdictCard({ verdict }: VerdictCardProps) {
  const pct = Math.round(verdict.confidence * 100);
  const isFactualQuery = !!verdict.answer;
  // For compliance reviews the ComplianceStatusBadge carries the verdict
  // code; for factual queries the platform StatusBadge ("已给出") plus the
  // answer text together carry the message. We show both when
  // compliance_status is "not_applicable" so the user sees that the
  // answer was derived from a factual lookup, not a compliance review.
  const showComplianceBadge =
    !!verdict.compliance_status && verdict.compliance_status !== 'not_applicable';
  return (
    <div className="relative">
      <div
        data-testid="verdict-card"
        data-verdict-status={verdict.status}
        data-compliance-status={verdict.compliance_status ?? ''}
        data-review-required={String(verdict.review_required)}
        className={cn(
          'my-2 rounded-lg border-0 border-l-4 bg-surface pl-4 pr-5 py-3 text-text',
          STATUS_BORDER[verdict.status],
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <StatusBadge status={verdict.status} />
            {showComplianceBadge && (
              <ComplianceStatusBadge status={verdict.compliance_status!} />
            )}
          </div>
        </div>
        {isFactualQuery && (
          <div
            data-testid="verdict-answer"
            className="mt-2 text-sm text-text leading-relaxed"
          >
            {verdict.answer}
          </div>
        )}
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
