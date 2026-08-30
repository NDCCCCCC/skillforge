// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-03 (ADM-03) — KPI Definitions View.
//
// Renders a read-only table of KPI definitions loaded from GET /api/admin/kpis.
// Files are the sole source of truth — NO edit button, NO kebab menu.
//
// Data shape:
//   GET /api/admin/kpis → { signature: string, kpis: KPIRow[], latest_calls: Record<string, string> }
//   KPIRow: { name, description, params_summary, dataset }
//   latest_calls maps KPI name → RFC3339 timestamp of last invocation

import { useEffect, useState } from 'react';

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

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <h3 className="text-lg font-semibold text-text">暂无 KPI 定义</h3>
      <p className="mt-1 text-sm text-text-muted">在 kpis/ 目录添加 YAML 文件注册新 KPI</p>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="flex h-10 items-center gap-4 rounded-md bg-surface" />
      ))}
    </div>
  );
}

export default function KPIDefinitionList() {
  const [data, setData] = useState<KPIAdminResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/kpis', {
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
        return resp.json() as Promise<KPIAdminResponse>;
      })
      .then((d) => {
        if (d) {
          setData(d);
        }
        setLoading(false);
      })
      .catch(() => {
        setError(true);
        setLoading(false);
      });
  }, []);

  if (loading) {
    return <LoadingSkeleton />;
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 text-sm text-danger">
        <span>KPI 数据加载失败</span>
      </div>
    );
  }

  if (!data || data.kpis.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-border text-xs text-text-muted">
            <th scope="col" className="pb-2 text-left font-semibold">Function Name</th>
            <th scope="col" className="pb-2 text-left font-semibold">Signature</th>
            <th scope="col" className="pb-2 text-left font-semibold">Last Called</th>
            <th scope="col" className="pb-2 text-right font-semibold">Call Count</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {data.kpis.map((kpi) => {
            const lastCalled = data.latest_calls[kpi.name];
            const called = Boolean(lastCalled);
            return (
              <tr
                key={kpi.name}
                className="hover:bg-surface transition-colors"
              >
                <td className="py-3 font-medium text-text">{kpi.name}</td>
                <td className="py-3">
                  <code className="font-mono text-xs text-text-muted">{kpi.params_summary}</code>
                </td>
                <td className="py-3 text-sm text-text-muted">
                  {called ? (
                    <span title={lastCalled}>{formatRelativeTime(lastCalled)}</span>
                  ) : (
                    <span className="text-text-muted">Never</span>
                  )}
                </td>
                <td className="py-3 text-right text-sm">
                  {called ? (
                    <span className="text-text-muted">1+</span>
                  ) : (
                    <span className="text-text-muted">0</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
