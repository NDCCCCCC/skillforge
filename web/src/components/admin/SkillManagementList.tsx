// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-04 (ADM-04) — Skill Management View.
// Migrated to the shared admin component library (admin UI redesign 2610):
// useAdminData (token'd fetch + three states), DataTable (column-driven
// table with built-in loading/error/empty), StatusBadge (token-tinted
// status pill replacing the inline hex StatusDot), PageHeader.
//
// Renders a read-only table of registered Skills loaded from GET /api/admin/skills.
// The endpoint overlays: hot-reload status from the skill registry + last eval summary
// from eval/reports/INDEX.json.
//
// Data shape (unchanged by the migration):
//   GET /api/admin/skills → { skills: SkillRow[] }
//   SkillRow: { name, version, status, last_eval_summary, last_change_at }
//
// Interaction (UI-SPEC §Interaction Behaviors / SkillManagementList):
//   - Hover delay: 200ms before tooltip shows (avoids flash on mouse pass-through)
//   - Eval results shown as hover tooltip ONLY — NOT inline badge
//   - Tooltip dismisses on mouse leave

import { useRef, useState } from 'react';
import DataTable from './shared/DataTable';
import { PageHeader } from './shared/Primitives';
import StatusBadge from './shared/StatusBadge';
import { useAdminData } from './shared/useAdminData';

interface EvalSummary {
  faithfulness: number;
  context_precision: number;
  eval_ts: string;
}

interface SkillRow {
  name: string;
  version: number;
  status: string;
  last_eval_summary: EvalSummary | null;
  last_change_at: string;
}

interface SkillsAdminResponse {
  skills: SkillRow[];
}

/** Format an RFC3339 timestamp as a human-readable relative time string. */
function formatRelativeTime(iso: string): string {
  if (!iso) return '—';
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

/** Format an RFC3339 timestamp as a local date/time string. */
function formatDateTime(iso: string): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleString();
}

/** Tooltip showing eval summary scores. */
function EvalTooltip({ summary }: { summary: EvalSummary }) {
  return (
    <div
      className="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-52 rounded-md border border-border bg-surface p-3 shadow-lg"
      role="tooltip"
    >
      <div className="mb-2 text-xs font-semibold text-text">Eval Results</div>
      <div className="space-y-1 text-xs">
        <div className="flex justify-between">
          <span className="text-text-muted">faithfulness</span>
          <span className="font-mono text-text">{summary.faithfulness.toFixed(3)}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-text-muted">context_precision</span>
          <span className="font-mono text-text">{summary.context_precision.toFixed(3)}</span>
        </div>
        <div className="mt-2 border-t border-border pt-2 text-text-muted">
          {formatDateTime(summary.eval_ts)}
        </div>
      </div>
    </div>
  );
}

export default function SkillManagementList() {
  const { data, loading, error, reload } = useAdminData<SkillsAdminResponse>(
    '/api/admin/skills',
  );
  const skills = data?.skills ?? [];
  const [hoveredSkill, setHoveredSkill] = useState<string | null>(null);
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  function handleMouseEnter(name: string) {
    if (hoverTimerRef.current) {
      clearTimeout(hoverTimerRef.current);
    }
    hoverTimerRef.current = setTimeout(() => {
      setHoveredSkill(name);
    }, 200);
  }

  function handleMouseLeave() {
    if (hoverTimerRef.current) {
      clearTimeout(hoverTimerRef.current);
    }
    setHoveredSkill(null);
  }

  return (
    <div>
      <PageHeader
        title="Skill 管理"
        description="已注册 Skill 的版本、热加载状态与最近评测摘要（只读）"
      />
      <DataTable<SkillRow>
        columns={[
          {
            key: 'name',
            label: 'Name',
            render: (skill) => (
              <span className="font-medium text-text">{skill.name}</span>
            ),
          },
          {
            key: 'version',
            label: 'Version',
            render: (skill) => (
              <code className="font-mono text-xs text-text-muted">
                {skill.version > 0 ? `v${skill.version}` : '—'}
              </code>
            ),
          },
          {
            key: 'status',
            label: 'Status',
            render: (skill) => <StatusBadge status={skill.status} />,
          },
          {
            key: 'last_eval',
            label: 'Last Eval',
            className: 'relative',
            render: (skill) => (
              <span
                onMouseEnter={() => handleMouseEnter(skill.name)}
                onMouseLeave={handleMouseLeave}
              >
                <span className="cursor-pointer text-xs text-text-muted hover:text-text">
                  {skill.last_eval_summary ? 'View details' : 'No eval'}
                </span>
                {hoveredSkill === skill.name && skill.last_eval_summary && (
                  <EvalTooltip summary={skill.last_eval_summary} />
                )}
              </span>
            ),
          },
          {
            key: 'last_change',
            label: 'Last Change',
            render: (skill) => (
              <span className="text-sm text-text-muted">
                {skill.last_change_at ? (
                  <span title={skill.last_change_at}>
                    {formatRelativeTime(skill.last_change_at)}
                  </span>
                ) : (
                  '—'
                )}
              </span>
            ),
          },
        ]}
        rows={skills}
        rowKey={(skill) => skill.name}
        loading={loading}
        error={error}
        errorRetry={reload}
        empty={{ title: '暂无注册 Skill', hint: '创建 skills/*.d/SKILL.md 注册新 Skill' }}
      />
    </div>
  );
}
