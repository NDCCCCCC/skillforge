// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DataTable — shared admin table (admin UI redesign 2610).
//
// Column-driven table that owns the three standard states (loading /
// error / empty) so view components only describe columns + rows.
// Keeps the original markup contract: thead `border-b border-border
// text-xs text-text-muted`, tbody `divide-y divide-border`, row hover.
//
// Usage:
//   <DataTable
//     columns={[
//       { key: 'skill', label: 'Skill', render: (r) => r.skill_id },
//       { key: 'status', label: '状态', render: (r) => <StatusBadge status={r.status} /> },
//     ]}
//     rows={data?.schedules ?? []}
//     rowKey={(r) => r.run_id}
//     loading={loading}
//     error={error}
//     errorRetry={reload}
//     empty={{ title: '暂无定时任务', hint: '在 Skill 的 report_schedule 字段配置调度' }}
//     errorTestid="schedules-error"
//     emptyTestid="schedules-empty"
//   />

import type { ReactNode } from 'react';
import { cn } from '../../../lib/cn';
import { EmptyState, ErrorState, LoadingSkeleton } from './States';

export interface DataTableColumn<T> {
  key: string;
  label: string;
  render: (row: T, index: number) => ReactNode;
  className?: string;
  headClassName?: string;
}

export default function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  error = false,
  errorRetry,
  errorTestid,
  empty,
  emptyTestid,
  loadingTestid,
  loadingRows = 4,
  rowTestId,
  className,
}: {
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T, index: number) => string;
  loading?: boolean;
  error?: boolean;
  errorRetry?: () => void;
  errorTestid?: string;
  empty: { title: string; hint?: string };
  emptyTestid?: string;
  loadingTestid?: string;
  loadingRows?: number;
  /** Optional per-row testid accessor — lets tests pin individual rows. */
  rowTestId?: (row: T, index: number) => string | undefined;
  className?: string;
}) {
  if (loading) {
    return <LoadingSkeleton rows={loadingRows} testId={loadingTestid} />;
  }
  if (error) {
    return (
      <ErrorState
        message="数据加载失败"
        onRetry={errorRetry}
        testId={errorTestid}
      />
    );
  }
  if (rows.length === 0) {
    return (
      <EmptyState
        title={empty.title}
        hint={empty.hint}
        testId={emptyTestid}
      />
    );
  }
  return (
    <div className={cn('overflow-x-auto', className)}>
      <table className="w-full">
        <thead>
          <tr className="border-b border-border text-xs text-text-muted">
            {columns.map((col) => (
              <th
                key={col.key}
                scope="col"
                className={cn(
                  'whitespace-nowrap pb-2 text-left font-semibold',
                  col.headClassName,
                )}
              >
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {rows.map((row, i) => (
            <tr
              key={rowKey(row, i)}
              data-testid={rowTestId?.(row, i)}
              className="transition-colors hover:bg-surface-2/60"
            >
              {columns.map((col) => (
                <td key={col.key} className={cn('py-3 pr-4 align-middle text-sm', col.className)}>
                  {col.render(row, i)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
