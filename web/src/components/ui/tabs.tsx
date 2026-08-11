// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Tabs — lightweight tabs primitive. State-managed (no Radix) — the
// center panel tabs are simple, low-complexity surface (Wave 3 expands
// if needed). Uses data attributes for active styling so the parent can
// style via CSS or inline.
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
        className={cn('flex gap-1 border-b border-border', className)}
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
                'px-3 py-2 text-xs font-semibold transition-colors',
                'border-b-2 -mb-px',
                active
                  ? 'border-primary text-primary'
                  : 'border-transparent text-text-muted hover:text-text',
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
