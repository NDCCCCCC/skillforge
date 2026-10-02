// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 10-03 Task 2 — HistoryRewriteHint component.
//
// Plan 10-02 (Phase 10 history pipeline) emits a data-decision SSE
// chunk with Decision="history_query_rewritten" carrying the rewritten
// query in the payload's `query` field (D10-C3: 改写结果 = 透明提示).
// The FE renders a small caption above the user bubble so the user
// can see what the rewriter understood and correct it if it was
// wrong.
//
// The component is PRESENTATION-ONLY. It does not dispatch any event;
// MessageBubble (or its dispatcher) reads the data-decision part and
// renders this component when decision === 'history_query_rewritten'.

import { Sparkles } from 'lucide-react';

export interface HistoryRewriteHintProps {
  /** The rewritten query string from the data-decision payload. */
  rewrite: string;
}

/**
 * Small caption rendered above a user bubble when the history
 * pipeline's query rewriter fired (D10-C3 transparency loop).
 *
 * Empty `rewrite` props render nothing (the caption is hidden when
 * the data-decision event carries an empty / missing rewritten text —
 * the chip would be visual noise otherwise).
 */
export function HistoryRewriteHint({ rewrite }: HistoryRewriteHintProps) {
  if (!rewrite) return null;
  return (
    <div
      data-testid="history-rewrite-hint"
      data-rewrite={rewrite}
      className="mb-1 flex items-center gap-1.5 text-xs text-text-muted"
      aria-label="已理解为改写后的查询"
    >
      <Sparkles className="h-3 w-3" aria-hidden />
      <span>已理解为「{rewrite}」</span>
    </div>
  );
}

export default HistoryRewriteHint;