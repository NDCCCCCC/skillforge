// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-8 (CMPL-08) — FilteredOutNotice banner.
//
// Renders ABOVE the ChunkSourceView body when the operator navigates
// to a superseded chunk via the graph browser. The retriever PRE-
// filter (D5-15) ensures the chunk never reaches a verdict.citations
// payload, so the only path to view a 已失效 chunk is the explicit
// graph-browser navigation.
//
// Copy: 此片段已失效（{effectiveTo} 起），不会出现在检索结果中。仅可浏览。

import { AlertTriangle } from 'lucide-react';
import { formatEffectiveDate } from '../../lib/format-effective';

export interface FilteredOutNoticeProps {
  /** Epoch seconds (or null) when the chunk became superseded. */
  effectiveTo: number | null;
}

export default function FilteredOutNotice({ effectiveTo }: FilteredOutNoticeProps) {
  const date = formatEffectiveDate(effectiveTo);
  return (
    <div
      data-testid="filtered-out-notice"
      data-effective-to={effectiveTo ?? '0'}
      className="flex items-start gap-2 rounded-md border border-status-non-compliant/40 bg-status-non-compliant/10 px-3 py-2 text-xs text-text"
    >
      <AlertTriangle className="h-4 w-4 mt-0.5 shrink-0 text-status-non-compliant" />
      <span>
        此片段已失效（{date} 起），不会出现在检索结果中。仅可浏览。
      </span>
    </div>
  );
}