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
import { forwardRef, type HTMLAttributes, useState } from 'react';
import { cn } from '../../lib/cn';

export interface TabsProps extends HTMLAttributes<HTMLDivElement> {
  defaultValue: string;
  items: { value: string; label: string }[];
  onValueChange?: (value: string) => void;
}

export const Tabs = forwardRef<HTMLDivElement, TabsProps>(
  ({ defaultValue, items, onValueChange, className, ...props }, ref) => {
    const [value, setValue] = useState(defaultValue);

    return (
      <div
        ref={ref}
        role="tablist"
        className={cn('flex gap-1', className)}
        {...props}
      >
        {items.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              role="tab"
              aria-selected={active}
              data-state={active ? 'active' : 'inactive'}
              onClick={() => {
                setValue(item.value);
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
