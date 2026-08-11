// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataSubprogressPart — FEUI-06 sub-progress step indicator.
//
// Wave 2 ships the minimum: a single-line component that maps each
// `SubprogressStep` to the Copywriting Contract step copy. Wave 3
// expands with collapse-on-finish animation per UI-SPEC §FEUI-06.
import { cn } from '../../../lib/cn';
import type { SubprogressStep } from '../../../types/verdict';
import type { UIMessage } from 'ai';

// AI SDK v7's UIMessage generic UIDataTypes defaults do not include
// the custom data-* payloads we author in fixtures. Cast through
// `unknown` to the structural shape the renderer actually reads.
// (Pitfall 2: payload lives on .data, NOT .policy.)
type DataSubprogressPartType = {
  type: 'data-subprogress';
  id?: string;
  data: SubprogressStep;
};

function renderStep(step: SubprogressStep): string {
  switch (step.step) {
    case 'retrieve':
      return `检索 [${step.tier ?? '权威'}] 知识层...`;
    case 'found_candidates':
      return `找到 ${step.count ?? 0} 条候选`;
    case 'rerank':
      return '重排序中...';
    case 'self_verify':
      return '自校验中...';
    case 'generate':
      return '生成结论中...';
    default:
      return '';
  }
}

export default function DataSubprogressPart({
  part,
}: {
  part: DataSubprogressPartType | (UIMessage['parts'][number] & Record<string, unknown>);
}) {
  const data = (part as DataSubprogressPartType).data as SubprogressStep;
  return (
    <div
      data-testid="data-subprogress"
      data-step={data.step}
      className={cn(
        'my-2 rounded-md border-l-4 border-l-primary bg-surface-2 px-3 py-2 font-mono text-xs text-text-muted',
      )}
    >
      {renderStep(data)}
    </div>
  );
}
