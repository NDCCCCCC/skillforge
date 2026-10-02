// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Tabs — lightweight tabs primitive. State-managed (no Radix) — the
// detail panel tabs are a simple, low-complexity surface (Wave 3
// expands if needed). Uses data attributes for active styling so the
// parent can style via CSS or inline.
//
// Variants (Open Design redesign B2):
//   - 'pill' (default): active tab is a filled rounded chip
//     (bg-primary-dim). Unchanged from the previous style — existing
//     consumers keep their look.
//   - 'underline': active tab gets a 2px bottom primary indicator;
//     hover tint is text-only. Used by the chat-first DetailPanel.
//
// role=tab / aria-selected semantics are identical across variants so
// tests pass either way.
//
// Supports both uncontrolled (caller passes `defaultValue`, Tabs owns
// active state) and controlled (caller passes `value`, Tabs reflects the
// prop and skips internal setState on click). Controlled mode is what
// lets the DetailPanel bind the active tab to `centerView.kind` so the
// indicator follows programmatic store changes (04-UAT test 8).
import { forwardRef, type HTMLAttributes, useState } from 'react';
import { cn } from '../../lib/cn';

export interface TabsProps extends HTMLAttributes<HTMLDivElement> {
  /** Initial active value for uncontrolled usage. Ignored when `value` is set. */
  defaultValue?: string;
  /** Controlled active value. When set, Tabs reflects this and does not keep internal state. */
  value?: string;
  items: { value: string; label: string }[];
  onValueChange?: (value: string) => void;
  /** Visual variant — 'pill' (default) or 'underline' (redesign B2). */
  variant?: 'pill' | 'underline';
}

export const Tabs = forwardRef<HTMLDivElement, TabsProps>(
  ({ defaultValue, value, items, onValueChange, variant = 'pill', className, ...props }, ref) => {
    const isControlled = value !== undefined;
    const [internalValue, setInternalValue] = useState(defaultValue);
    const activeValue = isControlled ? value : internalValue;

    return (
      <div
        ref={ref}
        role="tablist"
        className={cn('flex gap-1', variant === 'underline' && 'gap-2', className)}
        {...props}
      >
        {items.map((item) => {
          const active = item.value === activeValue;
          return (
            <button
              key={item.value}
              role="tab"
              aria-selected={active}
              data-state={active ? 'active' : 'inactive'}
              data-variant={variant}
              onClick={() => {
                if (!isControlled) setInternalValue(item.value);
                onValueChange?.(item.value);
              }}
              className={cn(
                'px-3 py-1.5 text-xs font-semibold transition-colors',
                variant === 'underline'
                  ? cn(
                      'relative rounded-none border-b-2 px-2 py-2',
                      active
                        ? 'border-primary text-text'
                        : 'border-transparent text-text-muted hover:text-text',
                    )
                  : cn(
                      'rounded-md',
                      active
                        ? 'bg-primary-dim text-text'
                        : 'text-text-muted hover:text-text hover:bg-surface-2',
                    ),
              )}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    );
  },
);
Tabs.displayName = 'Tabs';
