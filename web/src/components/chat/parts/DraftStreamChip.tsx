// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DraftStreamChip — Plan 260815-cos.
//
// Renders the LLM token stream (🟡 待校验) during the wait between the
// user's question and the grounded verdict (✅ 已验证). When grounding
// fails, the chip is unmounted (RefusalCard replaces the bubble).
//
// Per locked CONTEXT.md decision A2: the chip's status state machine
// transitions pending → verified (or → refused) based on the data-policy
// verdict emitted AFTER the draft stream. The MockChatTransport (and
// the real DefaultChatTransport in production) keeps the draft buffer
// in a separate ref so the text-delta channel cannot pollute
// message.parts[].text — this is the load-bearing wire-contract
// invariant of the draft channel.
//
// Status contract:
//   - 'pending'  — token stream still arriving, verdict not yet parsed.
//                  Render 🟡 待校验 + the text-delta buffer (truncated
//                  surface area; the chip is informational, not a
//                  transcript).
//   - 'verified' — grounding passed, the underlying Verdict is the
//                  trust-bearing text. Render ✅ 已验证 + retain the
//                  draft text as the user-facing conclusion (the user
//                  watched it stream in, so continuity matters; the
//                  verdict text part itself can supplement this).
//   - 'refused'  — grounding failed; the chip is REMOVED (return null)
//                  so the RefusalCard takes its place. No orphan chip.

export interface DraftStreamChipProps {
  status: 'pending' | 'verified' | 'refused';
  draftText: string;
}

export default function DraftStreamChip({ status, draftText }: DraftStreamChipProps) {
  if (status === 'refused') return null;
  const label = status === 'pending' ? '🟡 待校验' : '✅ 已验证';
  return (
    <div
      data-testid="draft-chip"
      data-status={status}
      className="my-2 rounded-md border border-border-muted bg-surface-2 px-3 py-2 text-sm"
    >
      <span className="mr-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs">
        {label}
      </span>
      <span data-testid="draft-text">{draftText}</span>
    </div>
  );
}