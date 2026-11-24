// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-02 (PROV-01) -- the citation origin pill.
//
// ONE decision point for the wire default: `resolveCitationOrigin` is
// the only place that maps "field absent" to "vector". Everything else
// renders what it is told -- the FE never infers a channel from tier,
// score or text (58-UI-SPEC Channel Encoding lock).
//
// Unknown origin values render their RAW string with no glyph: an
// unknown provenance is still provenance, and blanking it would hide a
// fact (39 §Stage Mapping unknown-render-raw rule).
import { Network, Search } from 'lucide-react';
import { cn } from '../../../lib/cn';
import type { Citation } from '../../../types/verdict';

export type CitationOrigin = 'vector' | 'graph' | 'hybrid';

/** The single wire-default decision point (D-58-02). */
export function resolveCitationOrigin(citation: Citation): CitationOrigin {
  return citation.origin ?? 'vector';
}

const LABEL: Record<CitationOrigin, string> = {
  vector: '语义',
  graph: '本体',
  hybrid: '双通道',
};

export interface OriginBadgeProps {
  origin: string;
  className?: string;
}

export default function OriginBadge({ origin, className }: OriginBadgeProps) {
  if (origin === 'vector' || origin === 'graph' || origin === 'hybrid') {
    const isVector = origin === 'vector' || origin === 'hybrid';
    const isGraph = origin === 'graph' || origin === 'hybrid';
    return (
      <span
        data-testid="origin-badge"
        data-origin={origin}
        className={cn(
          'inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-semibold',
          origin === 'vector' && 'bg-primary-dim text-primary',
          origin === 'graph' && 'bg-secondary/10 text-secondary',
          origin === 'hybrid' && 'bg-surface-2 text-text',
          className,
        )}
      >
        {isVector && <Search className="h-3 w-3" data-glyph="vector" />}
        {isGraph && <Network className="h-3 w-3" data-glyph="graph" />}
        {LABEL[origin]}
      </span>
    );
  }
  // Unknown origin: raw key, no glyph, muted (never blank).
  return (
    <span
      data-testid="origin-badge"
      data-origin={origin}
      className={cn('inline-flex items-center rounded-full bg-surface-2 px-1.5 py-0.5 text-[11px] text-text-muted', className)}
    >
      {origin}
    </span>
  );
}
