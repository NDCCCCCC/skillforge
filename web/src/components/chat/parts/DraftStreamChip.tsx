// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DraftStreamChip — Plan 260815-cos (modernized).
//
// Renders the LLM token stream during the wait between the user's
// question and the grounded verdict.
//
// Status contract:
//   - 'pending'  — token stream still arriving, verdict not yet parsed.
//                  Render cyan pulse + truncated draft text (chip is
//                  informational, not a transcript).
//   - 'verified' — grounding passed; the underlying Verdict is the
//                  trust-bearing text. Render green check + retain
//                  the draft text (the user watched it stream in, so
//                  continuity matters).
//   - 'refused'  — grounding failed; the chip is REMOVED so the
//                  RefusalCard takes its place. No orphan chip.

export interface DraftStreamChipProps {
  status: 'pending' | 'verified' | 'refused';
  draftText: string;
}

export default function DraftStreamChip({ status, draftText }: DraftStreamChipProps) {
  if (status === 'refused') return null;
  const label = status === 'pending' ? '正在生成回答' : '已验证';
  return (
    <div
      data-testid="draft-chip"
      data-status={status}
      className="draft-chip-modern"
    >
      <span className="draft-chip-dot" aria-hidden />
      <span style={{ fontWeight: 500 }}>{label}</span>
      {draftText && (
        <>
          <span style={{ color: 'var(--color-text-dim)' }}>·</span>
          <span className="draft-chip-text" data-testid="draft-text">
            {draftText}
          </span>
        </>
      )}
    </div>
  );
}
