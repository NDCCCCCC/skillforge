// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ReasoningPart — modernized reasoning card (Phase 4 visual upgrade).
//
// D-08 invariants:
//   - Renders ABOVE the answer text in DOM order (parent MessageBubble
//     iterates in array order).
//   - ALWAYS visible — the card body is height-capped at 180px with a
//     fade-out mask, so long reasoning chains never push the answer
//     off-screen.
//   - One-click toggle to expand/collapse.
//
// Visual identity:
//   - Warm amber "thinking" surface (not the cyan/blue of tool calls)
//   - Lightbulb icon in a tinted badge
//   - Mono font for the body (the model's chain-of-thought is code-like)
//   - Streaming label suffix:
//     * `part.state === 'streaming'` → `推理中...`
//     * else while answer is streaming → `生成中...`
//     * when complete → empty suffix
import { useEffect, useRef, useState } from 'react';
import type { UIMessage } from 'ai';
import { ChevronDown, Lightbulb } from 'lucide-react';
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
      className={cn('reasoning-card')}
    >
      <div
        className="reasoning-card-header"
        onClick={() => setExpanded((b) => !b)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setExpanded((b) => !b);
          }
        }}
      >
        <span className="reasoning-card-icon" aria-hidden>
          <Lightbulb className="h-3 w-3" />
        </span>
        <span className="reasoning-card-label">
          推理过程
          {suffix && <span className="reasoning-card-suffix"> · {suffix}</span>}
        </span>
        <span
          className="reasoning-card-toggle"
          data-testid="reasoning-toggle"
          aria-expanded={expanded}
        >
          {expanded ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronDown className="h-3 w-3" style={{ transform: 'rotate(-90deg)' }} />
          )}
          <span>{expanded ? '收起' : '展开'}</span>
        </span>
      </div>
      <pre
        ref={bodyRef}
        data-testid="reasoning-body"
        className="reasoning-card-body"
      >
        {part.text}
      </pre>
    </div>
  );
}
