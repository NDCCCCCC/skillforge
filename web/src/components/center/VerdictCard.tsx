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
//
// 260817-hck: optional `chatText` prop. When the caller (MessageBubble
// via DataPolicyPart) passes the concatenated text parts of the same
// assistant message and that text verbatim contains verdict.answer
// (backend conclusionText embeds the answer verbatim into the text-delta,
// chat.go:801-809), the in-card verdict-answer block is suppressed so
// the conclusion renders only once. Consumers without chat context
// (CenterPanel) omit the prop and behavior is unchanged.
import { ChevronRight, ShieldCheck } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import type { Verdict, VerdictStatus } from '../../types/verdict';
import { isStubVerdict } from '../../types/verdict';
import { STATUS_LABELS } from '../common/StatusBadge';
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

// Open Design B5 — the chat-stream inline verdict stripe color (same
// platform-Status mapping as STATUS_BORDER).
const STATUS_STRIPE: Record<VerdictStatus, string> = {
  ok: 'var(--color-status-compliant)',
  needs_input: 'var(--color-status-needs-info)',
  refused: 'var(--color-status-unanswered)',
  error: 'var(--color-status-error)',
};

export interface VerdictCardProps {
  verdict: Verdict;
  // 260817-hck: 调用方（MessageBubble 经 DataPolicyPart）传入本条消息全部
  // text parts 的拼接文本；当该文本已逐字包含 verdict.answer 时（后端
  // conclusionText 把 answer verbatim 嵌入「结论：…。」，chat.go:801-809），
  // 卡片隐藏 verdict-answer 区块避免双份渲染。CenterPanel 等无 chat 流上下文
  // 的使用方不传此 prop，行为不变。
  chatText?: string;
}

export default function VerdictCard({ verdict, chatText }: VerdictCardProps) {
  const pct = Math.round(verdict.confidence * 100);
  const isFactualQuery = !!verdict.answer;
  // 260817-hck: verbatim substring match is sufficient — the backend emits
  // the conclusion as a single atomic text-delta, so there is no partial-
  // text race when the card renders. No fuzzy/regex matching by design.
  const answerText = verdict.answer?.trim() ?? '';
  const answerDuplicatedInChat =
    chatText !== undefined && answerText !== '' && chatText.includes(answerText);
  // 260817-i36 (②): chatText defined ⇔ chat-stream context. There the
  // large StatusBadge, the 「N 条引用」 count, and the CitationCluster
  // all DUPLICATE what the chat body already shows (conclusion text +
  // corner status badge on the conclusion + clickable doc chips), so
  // they are omitted. The CenterPanel 结论 tab (no chatText) keeps the
  // full card — it is the structured citation surface.
  const inChatStream = chatText !== undefined;
  // For compliance reviews the ComplianceStatusBadge carries the verdict
  // code; for factual queries the platform StatusBadge ("已给出") plus the
  // answer text together carry the message. We show both when
  // compliance_status is "not_applicable" so the user sees that the
  // answer was derived from a factual lookup, not a compliance review.
  const showComplianceBadge =
    !!verdict.compliance_status && verdict.compliance_status !== 'not_applicable';
  // 49-02 (D-08③): stub-mode badge — the answer came from the deterministic
  // extractor, not an LLM. Rendered on both card surfaces.
  const stubMode = isStubVerdict(verdict);

  // Open Design B5 — chat-stream inline card: a ≤340px compact stripe
  // (shield + status word + confidence + 详情→) that opens the detail
  // panel's verdict tab. Clicking dispatches setCenterView verdict +
  // setDetailPanelOpen(true) — the panel renders the currentVerdict
  // prop from App's message scan; the verdictId is the nav key.
  if (inChatStream) {
    return (
      <div className="relative">
        {/* 260817-hck contract kept: an answer NOT already verbatim in
            the chat body still renders (the stripe never swallows
            information); a duplicated one stays suppressed. */}
        {isFactualQuery && !answerDuplicatedInChat && (
          <div
            data-testid="verdict-answer"
            className="mb-1 text-sm text-text leading-relaxed"
          >
            {verdict.answer}
          </div>
        )}
        <button
          type="button"
          data-testid="verdict-card"
          data-verdict-status={verdict.status}
          data-compliance-status={verdict.compliance_status ?? ''}
          data-review-required={String(verdict.review_required)}
          onClick={() => {
            useAppStore.getState().setCenterView({ kind: 'verdict', verdictId: 'chat-latest' });
            useAppStore.getState().setDetailPanelOpen(true);
          }}
          className="verdict-inline my-2"
          style={{ borderLeftColor: STATUS_STRIPE[verdict.status] }}
        >
          <ShieldCheck className="h-4 w-4 shrink-0" style={{ color: STATUS_STRIPE[verdict.status] }} aria-hidden />
          <span className="verdict-inline-status">{STATUS_LABELS[verdict.status]}</span>
          {stubMode && (
            <span
              data-testid="stub-mode-badge"
              className="rounded-full border border-current px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide opacity-70"
            >
              stub
            </span>
          )}
          <span className="verdict-inline-conf">置信度 {pct}%</span>
          <span className="verdict-inline-cta">
            详情
            <ChevronRight className="h-3 w-3" aria-hidden />
          </span>
        </button>
        {verdict.review_required && (
          <span
            data-testid="review-required-badge"
            className="absolute -top-2 right-3 rounded-full bg-status-needs-info px-2 py-0.5 text-xs font-semibold text-primary-fg"
          >
            需人工复核
          </span>
        )}
        <div
          data-testid="confidence-bar"
          data-confidence={pct}
          className="hidden"
          aria-hidden="true"
        />
      </div>
    );
  }

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
            {stubMode && (
              <span
                data-testid="stub-mode-badge"
                className="rounded-full border border-current px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-text-muted"
              >
                stub
              </span>
            )}
          </div>
        </div>
        {isFactualQuery && !answerDuplicatedInChat && (
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
