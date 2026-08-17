// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataPolicyPart — Wave 2 dispatch into VerdictCard vs RefusalCard.
//
// Per Pitfall 2, the payload lives on `part.data` (NOT `.policy`).
// The discriminated union `Verdict | Refusal` is resolved by the
// `isVerdict(data)` / `isRefusal(data)` type guards (exported from
// types/verdict.ts).
//
// When verdict: render <VerdictCard />. The CitationCluster inside
// VerdictCard dispatches `setCenterView({ kind: 'chunk-source', ... })`
// directly via the store singleton — no callback prop drilling.
//
// When refusal: render <RefusalCard />.
//
// 260817-hck: optional `chatText` prop (concatenated text parts of the
// same assistant message, supplied by MessageBubble) is passed through
// to VerdictCard so the card can suppress a duplicated verdict-answer
// block. Refusal / unknown-payload branches are untouched.
import type { UIMessage } from 'ai';
import RefusalCard from '../../center/RefusalCard';
import VerdictCard from '../../center/VerdictCard';
import { isRefusal, isVerdict } from '../../../types/verdict';

// AI SDK v7's UIMessage generic UIDataTypes defaults do not include
// the custom data-* payloads we author in fixtures. Cast through
// `unknown` to the structural shape the renderer actually reads.
// (Pitfall 2: payload lives on .data, NOT .policy.)
type DataPolicyPartType = {
  type: 'data-policy';
  id?: string;
  data: unknown;
};

export default function DataPolicyPart({
  part,
  chatText,
}: {
  part: DataPolicyPartType | (UIMessage['parts'][number] & Record<string, unknown>);
  // 260817-hck: concatenated text parts of the same message; forwarded to
  // VerdictCard so it can hide a verdict-answer already shown in the body.
  chatText?: string;
}) {
  const data = (part as DataPolicyPartType).data;
  if (isVerdict(data)) {
    return <VerdictCard verdict={data} chatText={chatText} />;
  }
  if (isRefusal(data)) {
    return <RefusalCard refusal={data} />;
  }
  return (
    <div
      className="my-2 rounded-md border border-danger bg-surface-2 p-3 text-xs"
      data-testid="data-policy-unknown"
    >
      Unknown data-policy payload shape
    </div>
  );
}
