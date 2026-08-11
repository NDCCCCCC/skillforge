// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Badge — small inline label (tier badges, status labels). Tuned to
// SkillForge design tokens (Label size, 12px).
import { forwardRef, type HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'tier' | 'review' | 'status';
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = 'default', ...props }, ref) => (
    <span
      ref={ref}
      className={cn(
        'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-semibold font-mono',
        variant === 'default' && 'bg-surface-2 text-text border border-border',
        variant === 'tier' && 'bg-surface-2 text-primary border border-border',
        variant === 'review' && 'bg-needs-info text-primary-fg',
        variant === 'status' && 'bg-surface-3 text-text',
        className,
      )}
      {...props}
    />
  ),
);
Badge.displayName = 'Badge';
