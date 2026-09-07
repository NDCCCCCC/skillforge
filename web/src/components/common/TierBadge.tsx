// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TierBadge — KNOW-05 mono-text tier badge. Renders the REGISTRY display
// label of a knowledge-base slug (Phase 26 / KBM-02): `[权威]` for the
// seed authority base, the projection's display_name for any later base,
// and `[slug]` for anything unknown — a new KB, or a pre-migration CN tier
// value replayed from an audit row. The unknown-slug degradation is the
// point (T-26-08-2): a legacy value must not masquerade as a live tier,
// and a missing slug must never blank the badge or crash the render.
//
// Color is the SAME for all tiers (tier is differentiated by TEXT, not
// color) — tier text color is `--color-primary`. This matches sketch
// `.citation-chip .tier { color: var(--color-primary); font-weight:
// 600; }`.
import { cn } from '../../lib/cn';
import { useTierLabel } from '../../lib/kb-bases';

export interface TierBadgeProps {
  /** Registry slug of the knowledge base. Legacy CN values degrade to a
   * bracketed label; an empty value renders nothing (badge hidden). */
  tier: string;
  className?: string;
  /** Pre-resolved bracketed label — short-circuits the async projection
   * lookup for callers that already hold the display name. */
  displayName?: string;
}

export default function TierBadge({ tier, className, displayName }: TierBadgeProps) {
  const label = useTierLabel(tier, displayName);
  if (!tier) return null;
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border border-border bg-surface-2 px-2 py-0.5 font-mono text-xs font-semibold text-primary',
        className,
      )}
      data-testid="tier-badge"
      data-tier={tier}
    >
      {label}
    </span>
  );
}
