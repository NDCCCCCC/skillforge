// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// RefusalCard — D-07 / FEUI-05 refusal card. Visually distinct from
// the VerdictCard:
//   - Out-of-scope → --color-status-out-of-scope (blue-gray) border +
//     🚫 glyph + heading `超出本 Skill 范围`.
//   - I-don't-know (no_authorized_source / low_retrieval_score) →
//     --color-status-unanswered (gray) border + ❓ glyph + heading
//     `范围内无依据` or `范围内检索置信度不足`.
//
// NEVER renders confidence bar, status-colored header (green/yellow/
// red), or citations count — those are verdict-only.
import { cn } from '../../lib/cn';
import type { Refusal, RefusalReason } from '../../types/verdict';

const REASON_META: Record<RefusalReason, { icon: string; label: string; border: string }> = {
  out_of_scope: {
    icon: '🚫',
    label: '超出本 Skill 范围',
    border: 'border-status-out-of-scope text-status-out-of-scope',
  },
  no_authorized_source: {
    icon: '❓',
    label: '范围内无依据',
    border: 'border-status-unanswered text-status-unanswered',
  },
  low_retrieval_score: {
    icon: '❓',
    label: '范围内检索置信度不足',
    border: 'border-status-unanswered text-status-unanswered',
  },
};

const REASON_TEXT: Record<RefusalReason, string> = {
  out_of_scope: '该请求不在本 Skill 的处理范围内。',
  no_authorized_source: '当前知识库内未找到权威依据可回答此问题。',
  low_retrieval_score: '检索到的候选文档匹配度过低，未达到引用阈值。',
};

export interface RefusalCardProps {
  refusal: Refusal;
}

export default function RefusalCard({ refusal }: RefusalCardProps) {
  const meta = REASON_META[refusal.reason];
  const canRephrase = refusal.can_user_rephrase;
  const suggestion = refusal.suggested_alternative_query ?? '';
  return (
    <div
      data-testid="refusal-card"
      data-refusal-reason={refusal.reason}
      className={cn(
        'my-2 rounded-lg border-[1.5px] bg-surface p-4 text-text',
        meta.border,
      )}
    >
      <h3 className={cn('text-lg font-semibold leading-tight', meta.border)}>
        {meta.icon} {meta.label}
      </h3>
      <p className="mt-2 text-base text-text">{REASON_TEXT[refusal.reason]}</p>
      {canRephrase && suggestion ? (
        <>
          <p className="mt-2 text-xs text-text-muted">您可以尝试换一种提问方式：</p>
          <button
            type="button"
            data-testid="refusal-suggestion-chip"
            className="mt-1 inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-0.5 text-xs font-semibold text-primary"
          >
            使用此提问：{suggestion}
          </button>
        </>
      ) : (
        <p className="mt-2 text-xs text-text-muted">本次无法回答，请联系 Skill 维护者。</p>
      )}
    </div>
  );
}
