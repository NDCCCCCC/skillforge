// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// States — shared empty / error / loading states (admin UI redesign 2610).
//
// Replaces the per-component EmptyState / LoadingSkeleton copies. All
// states accept an optional `testId` and keep the original copy styles
// so existing tests keep passing.

import type { ReactNode } from 'react';
import { AlertTriangle, Inbox, Loader2 } from 'lucide-react';
import { cn } from '../../../lib/cn';

export function EmptyState({
  title,
  hint,
  icon,
  testId,
  action,
}: {
  title: string;
  hint?: string;
  icon?: ReactNode;
  testId?: string;
  action?: ReactNode;
}) {
  return (
    <div
      data-testid={testId}
      className="flex flex-col items-center justify-center gap-2 py-14 text-center"
    >
      <div className="flex h-11 w-11 items-center justify-center rounded-full bg-surface-2 text-text-dim">
        {icon ?? <Inbox className="h-5 w-5" />}
      </div>
      <h3 className="text-sm font-semibold text-text">{title}</h3>
      {hint && <p className="text-xs text-text-muted">{hint}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
  testId,
}: {
  message: string;
  onRetry?: () => void;
  testId?: string;
}) {
  return (
    <div
      data-testid={testId}
      className="flex items-center justify-center gap-3 rounded-lg border border-danger/30 bg-danger/5 py-10 text-sm text-danger"
    >
      <AlertTriangle className="h-4 w-4" />
      <span>{message}</span>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="rounded-md border border-danger/40 px-2 py-1 text-xs font-semibold text-danger hover:bg-danger/10"
        >
          重试
        </button>
      )}
    </div>
  );
}

export function LoadingSkeleton({
  rows = 3,
  className,
  testId,
}: {
  rows?: number;
  className?: string;
  testId?: string;
}) {
  return (
    <div
      data-testid={testId}
      aria-busy="true"
      className={cn('flex flex-col gap-3', className)}
    >
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          className="h-10 animate-pulse rounded-md bg-surface-2"
          style={{ opacity: 1 - i * 0.2 }}
        />
      ))}
    </div>
  );
}

export function InlineSpinner({ className }: { className?: string }) {
  return (
    <Loader2
      className={cn('h-3.5 w-3.5 animate-spin text-text-muted', className)}
    />
  );
}
