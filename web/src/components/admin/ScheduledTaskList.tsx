// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-05 (ADM-05) — Scheduled Task List view.
//
// Renders a read-only table of scheduled report runs loaded from
// GET /api/admin/schedules. Files are the sole source of truth —
// NO edit button, NO action buttons.
//
// Data shape:
//   GET /api/admin/schedules → { schedules: ScheduleRow[] }
//   ScheduleRow: { run_id, skill_id, cron_expr, status,
//                  last_fired_at, next_fire_at, drift_status }
//
// Drift badge: shown only when drift_status === 'drifted' (amber badge).
// Cron expressions rendered in font-mono per UI-SPEC.

import { useEffect, useState } from 'react';

interface ScheduleRow {
  run_id: string;
  skill_id: string;
  cron_expr: string;
  status: string;
  last_fired_at: string;
  next_fire_at: string;
  drift_status: string;
}

interface SchedulesResponse {
  schedules: ScheduleRow[];
}

/** Format an RFC3339 timestamp as a human-readable absolute datetime. */
function formatAbsoluteTime(iso: string): string {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  } catch {
    return iso;
  }
}

/** Status badge mapping per UI-SPEC color semantics. */
function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { label: string; className: string }> = {
    completed: {
      label: '已完成',
      className: 'bg-[#34d399]/20 text-[#34d399]',
    },
    success: {
      label: '成功',
      className: 'bg-[#34d399]/20 text-[#34d399]',
    },
    running: {
      label: '运行中',
      className: 'bg-[#fbbf24]/20 text-[#fbbf24]',
    },
    pending: {
      label: '等待中',
      className: 'bg-[#fbbf24]/20 text-[#fbbf24]',
    },
    failed: {
      label: '失败',
      className: 'bg-[#f87171]/20 text-[#f87171]',
    },
    error: {
      label: '错误',
      className: 'bg-[#f87171]/20 text-[#f87171]',
    },
    idle: {
      label: '空闲',
      className: 'bg-[#8b95a3]/20 text-[#8b95a3]',
    },
  };
  const cfg = map[status] ?? {
    label: status,
    className: 'bg-[#8b95a3]/20 text-[#8b95a3]',
  };
  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${cfg.className}`}
      aria-label={`状态: ${cfg.label}`}
    >
      {cfg.label}
    </span>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <h3 className="text-lg font-semibold text-text">暂无定时任务</h3>
      <p className="mt-1 text-sm text-text-muted">
        在 Skill 的 report_schedule 字段配置调度
      </p>
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

export default function ScheduledTaskList() {
  const [data, setData] = useState<SchedulesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/schedules', {
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
        return resp.json() as Promise<SchedulesResponse>;
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
        <span>定时任务数据加载失败</span>
      </div>
    );
  }

  if (!data || data.schedules.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-border text-xs text-text-muted">
            <th scope="col" className="pb-2 text-left font-semibold">
              Skill
            </th>
            <th scope="col" className="pb-2 text-left font-semibold">
              Cron 表达式
            </th>
            <th scope="col" className="pb-2 text-left font-semibold">
              上次运行
            </th>
            <th scope="col" className="pb-2 text-left font-semibold">
              状态
            </th>
            <th scope="col" className="pb-2 text-left font-semibold">
              Drift
            </th>
            <th scope="col" className="pb-2 text-left font-semibold">
              下次执行
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {data.schedules.map((schedule) => (
            <tr
              key={schedule.run_id}
              className="hover:bg-surface transition-colors"
            >
              <td className="py-3 font-medium text-text">{schedule.skill_id}</td>
              <td className="py-3">
                <code className="font-mono text-xs text-text-muted">
                  {schedule.cron_expr}
                </code>
              </td>
              <td className="py-3 text-sm text-text-muted">
                {formatAbsoluteTime(schedule.last_fired_at)}
              </td>
              <td className="py-3">
                <StatusBadge status={schedule.status} />
              </td>
              <td className="py-3">
                {schedule.drift_status === 'drifted' && (
                  <span className="inline-flex items-center rounded bg-[#fbbf24]/20 px-2 py-0.5 text-xs font-medium text-[#fbbf24]">
                    drift
                  </span>
                )}
              </td>
              <td className="py-3 text-sm text-text-muted">
                {schedule.next_fire_at
                  ? formatAbsoluteTime(schedule.next_fire_at)
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
