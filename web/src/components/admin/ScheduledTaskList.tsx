// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-05 (ADM-05) — Scheduled Task List view.
// Migrated to the shared admin component library (admin UI redesign 2610):
// useAdminData (token'd fetch + three states), DataTable, and the shared
// StatusBadge — the per-component inline status-map copy (with its raw
// hex colors) is deleted. PageHeader added.
//
// Renders a read-only table of scheduled report runs loaded from
// GET /api/admin/schedules. Files are the sole source of truth —
// NO edit button, NO action buttons.
//
// Data shape (unchanged by the migration):
//   GET /api/admin/schedules → { schedules: ScheduleRow[] }
//   ScheduleRow: { run_id, skill_id, cron_expr, status,
//                  last_fired_at, next_fire_at, drift_status }
//
// Drift badge: shown only when drift_status === 'drifted' (amber StatusBadge).
// Cron expressions rendered in font-mono per UI-SPEC.

import DataTable from './shared/DataTable';
import { PageHeader } from './shared/Primitives';
import StatusBadge from './shared/StatusBadge';
import { useAdminData } from './shared/useAdminData';

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

export default function ScheduledTaskList() {
  const { data, loading, error, reload } = useAdminData<SchedulesResponse>(
    '/api/admin/schedules',
  );

  return (
    <div>
      <PageHeader
        title="定时任务"
        description="Skill 定时报告调度的运行记录与下次执行时间（只读）"
      />
      <DataTable<ScheduleRow>
        columns={[
          {
            key: 'skill_id',
            label: 'Skill',
            render: (schedule) => (
              <span className="font-medium text-text">{schedule.skill_id}</span>
            ),
          },
          {
            key: 'cron_expr',
            label: 'Cron 表达式',
            render: (schedule) => (
              <code className="font-mono text-xs text-text-muted">
                {schedule.cron_expr}
              </code>
            ),
          },
          {
            key: 'last_fired_at',
            label: '上次运行',
            render: (schedule) => (
              <span className="text-sm text-text-muted">
                {formatAbsoluteTime(schedule.last_fired_at)}
              </span>
            ),
          },
          {
            key: 'status',
            label: '状态',
            render: (schedule) => <StatusBadge status={schedule.status} />,
          },
          {
            key: 'drift_status',
            label: 'Drift',
            render: (schedule) =>
              schedule.drift_status === 'drifted' && (
                <StatusBadge status="drifted" />
              ),
          },
          {
            key: 'next_fire_at',
            label: '下次执行',
            render: (schedule) => (
              <span className="text-sm text-text-muted">
                {schedule.next_fire_at
                  ? formatAbsoluteTime(schedule.next_fire_at)
                  : '—'}
              </span>
            ),
          },
        ]}
        rows={data?.schedules ?? []}
        rowKey={(schedule) => schedule.run_id}
        loading={loading}
        error={error}
        errorRetry={reload}
        empty={{
          title: '暂无定时任务',
          hint: '在 Skill 的 report_schedule 字段配置调度',
        }}
      />
    </div>
  );
}
