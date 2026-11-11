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
import { renderStepCopy } from './subprogress-copy';
import type { UIMessage } from 'ai';

type DataSubprogressPartType = {
  type: 'data-subprogress';
  id?: string;
  data: SubprogressStep;
};

// 2026-11-11：文案抽到 subprogress-copy.ts，与 SubprogressTimeline 共用
// （MessageBubble 现在把连续的步骤收进时间线渲染；本组件仍作为单行
// 渲染器保留，供独立使用/测试）。
function renderStep(step: SubprogressStep, tierLabel: string | null): string {
  return renderStepCopy(step, tierLabel);
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
  // CODE-AUDIT 2026-11-05 (D-05): part.data used to be blindly asserted
  // (`as SubprogressStep`) and dereferenced — a chunk missing `data` (or a
  // wrongly-typed part) threw a TypeError in the message-render path, and
  // with NO ErrorBoundary anywhere in the app that unmounts the whole
  // React tree (white page). The sibling data-* branches in MessageBubble
  // guard with `?.` / early-returns; this file now does the same. The
  // hook stays unconditional (rules-of-hooks), so the guard lives in the
  // tier hook's argument, not in an early return before it.
  const data = (part as { data?: SubprogressStep }).data;
  const tier = data?.tier ?? '';
  // Hook must run unconditionally; useTierLabel('') is a no-op that hides
  // the badge when the step carries no tier.
  const resolvedLabel = useTierLabel(tier);
  const tierLabel = tier ? resolvedLabel : null;

  if (!data) return null;

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
