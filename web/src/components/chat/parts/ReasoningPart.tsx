// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ReasoningPart — FEUI-02 progressive-disclosure reasoning preview.
//
// D-08 invariants:
//   - Renders ABOVE the answer text in DOM order. The parent
//     MessageBubble iterates parts in array order; the fixture order
//     ensures reasoning parts arrive before text parts, so this
//     component never reorders siblings.
//   - ALWAYS expanded by default with height capped at 160px (the
//     `reasoning-preview` CSS class). Once the cap is reached the
//     user can scroll freely inside the preview; the answer text
//     stays in view because the message bubble scrolls internally.
//   - One-click `展开全部` / `收起` toggle. Chevron rotates 90deg
//     when expanded.
//
// Label suffix per Copywriting Contract:
//   - `part.state === 'streaming'` → `推理中...`
//   - else while no text has begun arriving → `推理中...`
//   - while text chunks are arriving → `生成中...`
//   - when complete → empty suffix
import { useEffect, useRef, useState } from 'react';
import type { UIMessage } from 'ai';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { cn } from '../../../lib/cn';

type ReasoningPartType = Extract<UIMessage['parts'][number], { type: 'reasoning' }>;

export interface ReasoningPartProps {
  part: ReasoningPartType;
  /** True while assistant text chunks are arriving (sets label to `生成中...`). */
  streamingAnswer?: boolean;
}

export default function ReasoningPart({ part, streamingAnswer }: ReasoningPartProps) {
  const [expanded, setExpanded] = useState(false);
  const bodyRef = useRef<HTMLPreElement>(null);

  // Live-streaming auto-stick-to-bottom: when new reasoning text
  // arrives, scroll the preview to its tail. Only when collapsed
  // (height-capped) so the user can still manually scroll when fully
  // expanded.
  useEffect(() => {
    if (expanded) return;
    const el = bodyRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [part.text, expanded]);

  const streaming = part.state === 'streaming';
  let suffix = '';
  if (streaming) suffix = '推理中...';
  else if (streamingAnswer) suffix = '生成中...';

  return (
    <div
      data-testid="reasoning-preview"
      data-expanded={String(expanded)}
      className={cn('my-2', expanded && 'reasoning-preview-uncap')}
    >
      <div className="flex items-center justify-between gap-2 px-1">
        <span className="font-mono text-xs font-semibold text-primary">
          推理{suffix && ` ${suffix}`}
        </span>
        <button
          type="button"
          onClick={() => setExpanded((b) => !b)}
          className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 font-mono text-xs font-semibold text-primary transition-colors hover:bg-surface-2"
          data-testid="reasoning-toggle"
          aria-expanded={expanded}
        >
          {expanded ? (
            <ChevronDown className="h-3.5 w-3.5 transition-transform" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 transition-transform" />
          )}
          <span>{expanded ? '收起' : '展开全部'}</span>
        </button>
      </div>
      <pre
        ref={bodyRef}
        data-testid="reasoning-body"
        className={cn(
          'reasoning-preview mt-1 whitespace-pre-wrap text-xs text-text-muted',
          expanded && 'max-h-none overflow-visible',
        )}
      >
        {part.text}
      </pre>
    </div>
  );
}
