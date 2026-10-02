// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ReasoningPart — modernized reasoning card (Phase 4 visual upgrade).
//
// Open Design redesign B5: the card defaults to HEADER-ONLY (the amber
// 推理过程 strip); the chain-of-thought body renders only after the
// user expands. The old 180px-cap-with-fade-mask collapsed preview is
// retired — collapsed now hides the body entirely, expanded shows it
// in full (scrollable if very long).
//
// D-08 invariants:
//   - Renders ABOVE the answer text in DOM order (parent MessageBubble
//     iterates in array order).
//   - ALWAYS visible (the header never drops) — one-click toggle.
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
  // arrives while expanded, scroll the body to its tail.
  useEffect(() => {
    if (!expanded) return;
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
      {expanded && (
        <pre
          ref={bodyRef}
          data-testid="reasoning-body"
          className="reasoning-card-body"
        >
          {part.text}
        </pre>
      )}
    </div>
  );
}
