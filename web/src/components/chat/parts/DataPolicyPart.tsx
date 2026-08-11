// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataPolicyPart — Wave 1 placeholder for the VerdictCard/RefusalCard
// target. The full UI ships in Wave 2 (VerdictCard with status-colored
// header + confidence bar + review-required badge per D-06; RefusalCard
// with out-of-scope vs I-don't-know visual distinction per D-07).
//
// Per Pitfall 2, the payload lives on `part.data` (NOT `.policy`).
// This component reads `part.data` and JSON-serializes for the placeholder.
// The discriminated union `Verdict | Refusal` distinguishes the two
// shapes by checking for `status` (Verdict) vs `reason` (Refusal).

import type { UIMessage } from 'ai';
import { isRefusal, isVerdict } from '../../../types/verdict';

type DataPolicyPart = Extract<UIMessage['parts'][number], { type: 'data-policy' }>;

export function DataPolicyPart({ part }: { part: DataPolicyPart }) {
  const data = part.data;
  // Wave 2 expands this to <VerdictCard verdict={verdict} /> or
  // <RefusalCard refusal={refusal} /> with full D-06/D-07 visual
  // semantics. For Wave 1 we emit a placeholder that still proves
  // the data shape arrives intact on part.data.
  if (isVerdict(data)) {
    return (
      <div
        className="my-2 rounded-md border border-border bg-surface-2 p-3 text-xs font-mono"
        data-testid="data-policy-placeholder"
        data-policy-status={data.status}
        data-policy-review-required={String(data.review_required)}
      >
        <div className="text-text-muted">[Wave 2 placeholder — VerdictCard]</div>
        <pre className="mt-1 text-text">{JSON.stringify(data, null, 2)}</pre>
      </div>
    );
  }
  if (isRefusal(data)) {
    return (
      <div
        className="my-2 rounded-md border border-border bg-surface-2 p-3 text-xs font-mono"
        data-testid="data-policy-placeholder"
        data-refusal-reason={data.reason}
      >
        <div className="text-text-muted">[Wave 2 placeholder — RefusalCard]</div>
        <pre className="mt-1 text-text">{JSON.stringify(data, null, 2)}</pre>
      </div>
    );
  }
  return (
    <div className="my-2 rounded-md border border-danger bg-surface-2 p-3 text-xs" data-testid="data-policy-unknown">
      Unknown data-policy payload shape
    </div>
  );
}
