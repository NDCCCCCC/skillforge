// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// TierBadge — KNOW-05 mono-text tier badge. Renders `[权威]` /
// `[历史]` / `[个人]` / `[草稿]` inside an inline span. Color is the
// SAME for all tiers (tier is differentiated by TEXT, not color) —
// tier text color is `--color-primary`. This matches sketch
// `.citation-chip .tier { color: var(--color-primary); font-weight:
// 600; }`.
import { cn } from '../../lib/cn';
import type { Tier } from '../../types/verdict';

export const TIER_LABELS: Record<Tier, string> = {
  权威: '[权威]',
  历史: '[历史]',
  个人: '[个人]',
  草稿: '[草稿]',
};

export interface TierBadgeProps {
  tier: Tier;
  className?: string;
}

export default function TierBadge({ tier, className }: TierBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border border-border bg-surface-2 px-2 py-0.5 font-mono text-xs font-semibold text-primary',
        className,
      )}
      data-testid="tier-badge"
      data-tier={tier}
    >
      {TIER_LABELS[tier]}
    </span>
  );
}
