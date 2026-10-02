// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-02 (ADM-02) — admin import batch list view.
//
// Renders a read-only table of import batches from GET /api/admin/batches:
//   - Columns: Dataset | Status | Row Count | Raw Size | Discarded At
//   - Status badges: completed=green, pending=amber, running=amber+pulse, failed=red
//   - Raw size formatted as human-readable (KB/MB)
//   - Empty state when API returns zero rows
//
// Read-only enforcement: no action buttons, no edit icons.

import { useEffect, useState } from 'react';

interface ImportBatch {
  batch_id: string;
  dataset: string;
  period: string;
  status: string;
  desensitized_row_count: number;
  raw_size_bytes: number;
  discard_ts: string;
  created_at: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso: string): string {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return iso;
  }
}

type BadgeStatus = 'completed' | 'pending' | 'running' | 'failed';

function StatusBadge({ status }: { status: string }) {
  const s = status as BadgeStatus;
  const labels: Record<BadgeStatus, string> = {
    completed: '已完成',
    pending: '待处理',
    running: '进行中',
    failed: '失败',
  };

  const classes: Record<BadgeStatus, string> = {
    completed: 'bg-green-500/20 text-green-400',
    pending: 'bg-amber-500/20 text-amber-400',
    running: 'bg-amber-500/20 text-amber-400 animate-pulse',
    failed: 'bg-red-500/20 text-red-400',
  };

  const cls = classes[s] ?? 'bg-surface-2 text-text-muted';
  const label = labels[s] ?? status;

  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${cls}`}
      aria-label={`状态: ${label}`}
    >
      {label}
    </span>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-12 text-center">
      <p className="text-base font-semibold text-text">暂无导入批次</p>
      <p className="text-sm text-text-muted">数据导入后将显示在此处</p>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="flex h-10 items-center gap-4 rounded bg-surface-2 animate-pulse" />
      ))}
    </div>
  );
}

export default function ImportBatchList() {
  const [batches, setBatches] = useState<ImportBatch[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/batches', {
      headers: {
        'X-Admin-Token': token,
      },
    })
      .then((resp) => {
        if (!resp.ok) {
          setError(true);
          setLoading(false);
          return null;
        }
        return resp.json() as Promise<{ batches: ImportBatch[] }>;
      })
      .then((data) => {
        if (data) {
          setBatches(data.batches);
        }
        setLoading(false);
      })
      .catch(() => {
        setError(true);
        setLoading(false);
      });
  }, []);

  if (error) {
    return (
      <div className="flex items-center gap-2 text-sm text-danger">
        <span>加载失败，请检查网络或刷新页面</span>
      </div>
    );
  }

  if (loading || batches === null) {
    return <LoadingSkeleton />;
  }

  if (batches.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border text-left">
            <th scope="col" className="pb-2 pr-4 font-medium text-text-muted">Dataset</th>
            <th scope="col" className="pb-2 pr-4 font-medium text-text-muted">Status</th>
            <th scope="col" className="pb-2 pr-4 font-medium text-text-muted">Row Count</th>
            <th scope="col" className="pb-2 pr-4 font-medium text-text-muted">Raw Size</th>
            <th scope="col" className="pb-2 font-medium text-text-muted">Discarded At</th>
          </tr>
        </thead>
        <tbody>
          {batches.map((batch) => (
            <tr
              key={batch.batch_id}
              className="border-b border-border/50 hover:bg-surface-2 transition-colors"
            >
              <td className="py-3 pr-4">
                <div className="flex flex-col">
                  <span className="font-medium text-text">{batch.dataset}</span>
                  {batch.period && (
                    <span className="text-xs text-text-muted">{batch.period}</span>
                  )}
                </div>
              </td>
              <td className="py-3 pr-4">
                <StatusBadge status={batch.status} />
              </td>
              <td className="py-3 pr-4 font-mono text-text">
                {batch.desensitized_row_count.toLocaleString()}
              </td>
              <td className="py-3 pr-4 font-mono text-text">
                {formatBytes(batch.raw_size_bytes)}
              </td>
              <td className="py-3 font-mono text-text-muted">
                {formatDate(batch.discard_ts)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
