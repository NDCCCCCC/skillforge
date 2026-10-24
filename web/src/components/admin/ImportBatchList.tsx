// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-02 (ADM-02) — admin import batch list view.
//
// 2610 redesign: migrated to the shared admin component library —
// useAdminData (three-state + X-Admin-Token), DataTable (columns-driven
// table with built-in loading/error/empty), StatusBadge (token-based
// status pills replacing the inline badge map and raw palette classes),
// PageHeader. Wire contract unchanged: GET /api/admin/batches.
//   - Columns: Dataset | Status | Row Count | Raw Size | Discarded At
//   - Status pills: completed=green, pending=amber, running=amber, failed=red
//   - Raw size formatted as human-readable (KB/MB)
//
// Read-only enforcement: no action buttons, no edit icons.

import { useAdminData } from './shared/useAdminData';
import DataTable from './shared/DataTable';
import StatusBadge from './shared/StatusBadge';
import { PageHeader } from './shared/Primitives';

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

interface BatchesResponse {
  batches: ImportBatch[];
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

/** Token-based status pill; keeps the original per-batch copy verbatim. */
function BatchStatusBadge({ status }: { status: string }) {
  const labels: Record<string, string> = {
    completed: '已完成',
    pending: '待处理',
    running: '进行中',
    failed: '失败',
  };
  return <StatusBadge status={status} label={labels[status.toLowerCase()] ?? status} />;
}

export default function ImportBatchList() {
  const { data, loading, error, reload } = useAdminData<BatchesResponse>('/api/admin/batches');

  return (
    <div>
      <PageHeader
        title="导入批次"
        description="数据集导入批次列表（只读）：状态、脱敏行数与原始体积"
      />
      <DataTable<BatchesResponse['batches'][number]>
        columns={[
          {
            key: 'dataset',
            label: 'Dataset',
            render: (batch) => (
              <div className="flex flex-col">
                <span className="font-medium text-text">{batch.dataset}</span>
                {batch.period && (
                  <span className="text-xs text-text-muted">{batch.period}</span>
                )}
              </div>
            ),
          },
          {
            key: 'status',
            label: 'Status',
            render: (batch) => <BatchStatusBadge status={batch.status} />,
          },
          {
            key: 'rows',
            label: 'Row Count',
            className: 'font-mono text-text',
            render: (batch) => batch.desensitized_row_count.toLocaleString(),
          },
          {
            key: 'raw-size',
            label: 'Raw Size',
            className: 'font-mono text-text',
            render: (batch) => formatBytes(batch.raw_size_bytes),
          },
          {
            key: 'discarded-at',
            label: 'Discarded At',
            className: 'font-mono text-text-muted',
            render: (batch) => formatDate(batch.discard_ts),
          },
        ]}
        rows={data?.batches ?? []}
        rowKey={(batch) => batch.batch_id}
        loading={loading}
        error={error}
        errorRetry={reload}
        empty={{ title: '暂无导入批次', hint: '数据导入后将显示在此处' }}
      />
    </div>
  );
}
