// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// PageHeader / StatCard / Toolbar / SectionCard — shared admin layout
// primitives (admin UI redesign 2610).
//
// PageHeader  — view title + description + right-aligned action slot.
// StatCard    — KPI tile with optional tone accent + hint.
// Toolbar     — search input + right-side action slot (filters, CTAs).
// SectionCard — Card with a titled header row (title/description/actions).

import type { ReactNode } from 'react';
import { Search } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { Card } from '../../ui/card';

export function PageHeader({
  title,
  description,
  actions,
  testId,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  testId?: string;
}) {
  return (
    <div
      data-testid={testId}
      className="mb-4 flex flex-wrap items-end justify-between gap-3"
    >
      <div>
        <h2 className="text-lg font-semibold text-text">{title}</h2>
        {description && (
          <p className="mt-0.5 text-xs text-text-muted">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}

const STAT_TONE: Record<string, string> = {
  default: 'text-text',
  green: 'text-status-compliant',
  amber: 'text-status-needs-info',
  red: 'text-status-non-compliant',
  indigo: 'text-primary',
};

export function StatCard({
  label,
  value,
  hint,
  tone = 'default',
  testId,
}: {
  label: string;
  value: ReactNode;
  hint?: string;
  tone?: keyof typeof STAT_TONE;
  testId?: string;
}) {
  return (
    <div
      data-testid={testId}
      className="rounded-lg border border-border bg-surface px-4 py-3"
    >
      <div className="text-xs text-text-muted">{label}</div>
      <div
        className={cn(
          'mt-1 font-mono text-xl font-semibold tabular-nums',
          STAT_TONE[tone],
        )}
      >
        {value}
      </div>
      {hint && <div className="mt-0.5 text-[11px] text-text-dim">{hint}</div>}
    </div>
  );
}

export function Toolbar({
  search,
  onSearchChange,
  searchPlaceholder = '搜索…',
  children,
  testId,
}: {
  search?: string;
  onSearchChange?: (next: string) => void;
  searchPlaceholder?: string;
  children?: ReactNode;
  testId?: string;
}) {
  return (
    <div
      data-testid={testId}
      className="mb-3 flex flex-wrap items-center justify-between gap-2"
    >
      {onSearchChange !== undefined && (
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-dim" />
          <input
            type="text"
            value={search ?? ''}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={searchPlaceholder}
            className="h-8 w-56 rounded-md border border-border bg-surface pl-8 pr-3 text-xs text-text placeholder:text-text-dim focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

export function SectionCard({
  title,
  description,
  actions,
  children,
  className,
  testId,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  testId?: string;
}) {
  return (
    <Card className={cn('p-4', className)} data-testid={testId}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-text">{title}</h3>
          {description && (
            <p className="mt-0.5 text-xs text-text-muted">{description}</p>
          )}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      {children}
    </Card>
  );
}
