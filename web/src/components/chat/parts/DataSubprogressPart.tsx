// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataSubprogressPart — FEUI-06 sub-progress step indicator (modernized).
//
// Renders each agent pipeline step as a row in a vertical timeline:
//   - left: animated dot (pulse on the currently running step)
//   - middle: connector line to the next step
//   - right: human-readable step copy
//
// The component is intentionally append-only — the timeline grows
// top-to-bottom as the agent emits more steps, with no animation on
// the prior rows (they're done; the user already saw them stream in).
//
// 26-08 residual sweep: the retrieve step's tier label comes from the KB
// registry projection (lib/kb-bases) and a step with NO tier renders NO
// badge — the old hardcoded tier default was deleted because, after the
// KB registry migration, that CN literal matches no slug and would
// describe a knowledge layer that cannot exist.
import type { SubprogressStep } from '../../../types/verdict';
import { useTierLabel } from '../../../lib/kb-bases';
import type { UIMessage } from 'ai';

type DataSubprogressPartType = {
  type: 'data-subprogress';
  id?: string;
  data: SubprogressStep;
};

function renderStep(step: SubprogressStep, tierLabel: string | null): string {
  switch (step.step) {
    case 'retrieve':
      return tierLabel ? `检索 ${tierLabel} 知识层` : '检索知识层';
    case 'found_candidates':
      return `找到 ${step.count ?? 0} 条候选`;
    case 'rerank':
      return '重排序';
    case 'self_verify':
      return '自校验';
    case 'generate':
      return '生成结论';
    default:
      return '';
  }
}

// Pre-fix note (2026-08-19 quick task 260819-fix-datasubprogress-state-ref):
// The previous implementation read `data.state === 'pending'` to flip a CSS
// class and a `data-pending` attribute. `SubprogressStep` (verdict.ts:118)
// has no `state` field — only `step` / `tier?` / `count?`. Steps are also
// append-only per the file godoc: a step is rendered only after the agent
// has emitted it, so there is no live "pending" state to model on the wire
// or in this component. Removed the dead read and the unused `cn` import.
export default function DataSubprogressPart({
  part,
}: {
  part: DataSubprogressPartType | (UIMessage['parts'][number] & Record<string, unknown>);
}) {
  const data = (part as DataSubprogressPartType).data as SubprogressStep;
  // Hook must run unconditionally; useTierLabel('') is a no-op that hides
  // the badge when the step carries no tier.
  const resolvedLabel = useTierLabel(data.tier ?? '');
  const tierLabel = data.tier ? resolvedLabel : null;

  return (
    <div
      data-testid="data-subprogress"
      data-step={data.step}
      className="subprogress-step"
    >
      <span className="subprogress-step-dot" aria-hidden />
      <span>{renderStep(data, tierLabel)}</span>
      <span className="subprogress-step-line" aria-hidden />
    </div>
  );
}
