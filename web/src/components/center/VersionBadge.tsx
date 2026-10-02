// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 / SC-8 (CMPL-08) — VersionBadge component.
//
// Renders the 当前有效 (BadgeCheck green) or 已失效 (strikethrough dim)
// badge next to a chunk's metadata. Active when effective_to is 0
// (perpetual) or >= now. Otherwise superseded.
//
// Per UI-SPEC §SC-8: the BADGE itself is the version signal — the
// metadata row still carries the explicit 生效日期 / 失效日期 strings
// for fine-grained audit; this badge is the at-a-glance indicator.

import { BadgeCheck } from 'lucide-react';
import { cn } from '../../lib/cn';
import { isChunkActive } from '../../types/knowledge';

export interface VersionBadgeProps {
  effectiveFrom: number | null;
  effectiveTo: number | null;
  /** Override the active/inactive calculation (test seam). Defaults
   * to isChunkActive(effectiveTo). */
  nowMs?: number;
  className?: string;
}

export default function VersionBadge({
  effectiveFrom,
  effectiveTo,
  nowMs,
  className,
}: VersionBadgeProps) {
  const active = isChunkActive(effectiveTo, nowMs);
  const dataAttrs = {
    'data-testid': 'version-badge',
    'data-status': active ? 'active' : 'superseded',
    'data-effective-from': effectiveFrom ?? '0',
    'data-effective-to': effectiveTo ?? '0',
  };

  if (active) {
    return (
      <span
        {...dataAttrs}
        className={cn(
          'inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-0.5 text-xs font-semibold text-success',
          className,
        )}
      >
        <BadgeCheck className="h-3 w-3" />
        当前有效
      </span>
    );
  }

  return (
    <span
      {...dataAttrs}
      className={cn(
        'inline-flex items-center rounded-md border border-border bg-surface-2 px-2 py-0.5 text-xs font-semibold text-text-dim line-through',
        className,
      )}
    >
      已失效
    </span>
  );
}