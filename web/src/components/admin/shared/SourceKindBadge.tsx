// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceKindBadge — shared provenance source-kind pill (admin UI redesign 2610).
//
// One mapping for the `file / emitter / induction` source-kind badge that
// ProvenanceGraphView, SparqlPlaygroundView and HybridDebugView each used
// to hand-roll (three copies of KIND_BADGE + label ternary). Token-only
// colors; unknown kinds fall back to the neutral surface chip.

import { cn } from '../../../lib/cn';

const KIND_CLASS: Record<string, string> = {
  file: 'bg-status-compliant/10 text-status-compliant',
  emitter: 'bg-surface-3 text-text-muted',
  induction: 'bg-primary-dim text-primary',
};

const KIND_LABEL: Record<string, string> = {
  file: '文件',
  emitter: '生成',
  induction: '归纳',
};

export default function SourceKindBadge({
  kind,
  className,
}: {
  kind: string;
  className?: string;
}) {
  return (
    <span
      className={cn(
        'rounded px-1.5 py-0.5 text-[11px] font-semibold',
        KIND_CLASS[kind] ?? 'bg-surface-3 text-text-muted',
        className,
      )}
    >
      {KIND_LABEL[kind] ?? kind}
    </span>
  );
}

// Named alias — some call sites import { SourceKindBadge } (2610 redesign).
export { SourceKindBadge };
