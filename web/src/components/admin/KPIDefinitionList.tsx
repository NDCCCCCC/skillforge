// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-03 (ADM-03) — KPI Definitions View.
//
// 2610 redesign: migrated to the shared admin component library —
// useAdminData (three-state + X-Admin-Token), DataTable, PageHeader.
// Wire contract unchanged: GET /api/admin/kpis.
//
// Renders a read-only table of KPI definitions. Files are the sole
// source of truth — NO edit button, NO kebab menu.
//
// Data shape:
//   GET /api/admin/kpis → { signature: string, kpis: KPIRow[], latest_calls: Record<string, string> }
//   KPIRow: { name, description, params_summary, dataset }
//   latest_calls maps KPI name → RFC3339 timestamp of last invocation

import { useAdminData } from './shared/useAdminData';
import DataTable from './shared/DataTable';
import { PageHeader } from './shared/Primitives';

interface KPIRow {
  name: string;
  description: string;
  params_summary: string;
  dataset: string;
}

interface KPIAdminResponse {
  signature: string;
  kpis: KPIRow[];
  latest_calls: Record<string, string>;
}

/** Format an RFC3339 timestamp as a human-readable relative time string. */
function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return 'just now';
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

export default function KPIDefinitionList() {
  const { data, loading, error, reload } = useAdminData<KPIAdminResponse>('/api/admin/kpis');

  return (
    <div>
      <PageHeader
        title="KPI 定义"
        description="只读视图：KPI 由 kpis/ 目录下的 YAML 文件声明，文件是唯一真源"
      />
      <DataTable<KPIRow>
        columns={[
          {
            key: 'name',
            label: 'Function Name',
            className: 'font-medium text-text',
            render: (kpi) => kpi.name,
          },
          {
            key: 'signature',
            label: 'Signature',
            render: (kpi) => (
              <code className="font-mono text-xs text-text-muted">{kpi.params_summary}</code>
            ),
          },
          {
            key: 'last-called',
            label: 'Last Called',
            className: 'text-sm text-text-muted',
            render: (kpi) => {
              const lastCalled = data?.latest_calls[kpi.name];
              if (!lastCalled) return <span className="text-text-muted">Never</span>;
              return <span title={lastCalled}>{formatRelativeTime(lastCalled)}</span>;
            },
          },
          {
            key: 'call-count',
            label: 'Call Count',
            headClassName: 'text-right',
            className: 'text-right text-sm text-text-muted',
            render: (kpi) => {
              const called = Boolean(data?.latest_calls[kpi.name]);
              return <span className="text-text-muted">{called ? '1+' : '0'}</span>;
            },
          },
        ]}
        rows={data?.kpis ?? []}
        rowKey={(kpi) => kpi.name}
        loading={loading}
        error={error}
        errorRetry={reload}
        empty={{ title: '暂无 KPI 定义', hint: '在 kpis/ 目录添加 YAML 文件注册新 KPI' }}
      />
    </div>
  );
}
