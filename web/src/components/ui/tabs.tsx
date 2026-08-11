// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Tabs — lightweight tabs primitive. State-managed (no Radix) — the
// center panel tabs are a simple, low-complexity surface (Wave 3
// expands if needed). Uses data attributes for active styling so the
// parent can style via CSS or inline.
//
// Pill variant (denova-aligned): the active tab is a filled rounded
// chip (bg-primary-dim) instead of an underline. role=tab /
// aria-selected semantics are unchanged so CenterPanel tests still pass.
//
// Supports both uncontrolled (caller passes `defaultValue`, Tabs owns
// active state) and controlled (caller passes `value`, Tabs reflects the
// prop and skips internal setState on click). Controlled mode is what
// lets CenterPanel bind the active tab to `centerView.kind` so the
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
}

export const Tabs = forwardRef<HTMLDivElement, TabsProps>(
  ({ defaultValue, value, items, onValueChange, className, ...props }, ref) => {
    const isControlled = value !== undefined;
    const [internalValue, setInternalValue] = useState(defaultValue);
    const activeValue = isControlled ? value : internalValue;

    return (
      <div
        ref={ref}
        role="tablist"
        className={cn('flex gap-1', className)}
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
              onClick={() => {
                if (!isControlled) setInternalValue(item.value);
                onValueChange?.(item.value);
              }}
              className={cn(
                'px-3 py-1.5 text-xs font-semibold transition-colors rounded-md',
                active
                  ? 'bg-primary-dim text-text'
                  : 'text-text-muted hover:text-text hover:bg-surface-2',
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
