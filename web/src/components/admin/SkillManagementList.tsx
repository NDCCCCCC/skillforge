// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-04 (ADM-04) — Skill Management View.
//
// Renders a read-only table of registered Skills loaded from GET /api/admin/skills.
// The endpoint overlays: hot-reload status from the skill registry + last eval summary
// from eval/reports/INDEX.json.
//
// Data shape:
//   GET /api/admin/skills → { skills: SkillRow[] }
//   SkillRow: { name, version, status, last_eval_summary, last_change_at }
//
// Interaction (UI-SPEC §Interaction Behaviors / SkillManagementList):
//   - Hover delay: 200ms before tooltip shows (avoids flash on mouse pass-through)
//   - Eval results shown as hover tooltip ONLY — NOT inline badge
//   - Tooltip dismisses on mouse leave

import { useEffect, useState, useRef } from 'react';

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

/** Colored dot for hot-reload status. */
function StatusDot({ status }: { status: string }) {
  const colors: Record<string, string> = {
    ok: 'bg-[#34d399]',       // green — completed / ok
    failed: 'bg-[#f87171]',   // red — failed / error
    pending: 'bg-[#fbbf24]',  // amber — pending / running
  };
  const colorClass = colors[status] ?? 'bg-[#8b95a3]'; // unknown / never-run
  return (
    <span
      className={`inline-block h-2 w-2 rounded-full ${colorClass}`}
      aria-label={`Status: ${status}`}
    />
  );
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

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <h3 className="text-lg font-semibold text-text">暂无注册 Skill</h3>
      <p className="mt-1 text-sm text-text-muted">创建 skills/*.d/SKILL.md 注册新 Skill</p>
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

export default function SkillManagementList() {
  const [skills, setSkills] = useState<SkillRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [hoveredSkill, setHoveredSkill] = useState<string | null>(null);
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const token = localStorage.getItem('sf_admin_token') ?? '';
    fetch('/api/admin/skills', {
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
        return resp.json() as Promise<SkillsAdminResponse>;
      })
      .then((d) => {
        if (d) {
          setSkills(d.skills);
        }
        setLoading(false);
      })
      .catch(() => {
        setError(true);
        setLoading(false);
      });
  }, []);

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

  if (loading) {
    return <LoadingSkeleton />;
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 text-sm text-danger">
        <span>Skill 数据加载失败</span>
      </div>
    );
  }

  if (skills.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-border text-xs text-text-muted">
            <th scope="col" className="pb-2 text-left font-semibold">Name</th>
            <th scope="col" className="pb-2 text-left font-semibold">Version</th>
            <th scope="col" className="pb-2 text-left font-semibold">Status</th>
            <th scope="col" className="pb-2 text-left font-semibold">Last Eval</th>
            <th scope="col" className="pb-2 text-left font-semibold">Last Change</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {skills.map((skill) => (
            <tr
              key={skill.name}
              className="hover:bg-surface transition-colors"
            >
              <td className="py-3 font-medium text-text">{skill.name}</td>
              <td className="py-3">
                <code className="font-mono text-xs text-text-muted">{skill.version > 0 ? `v${skill.version}` : '—'}</code>
              </td>
              <td className="py-3">
                <StatusDot status={skill.status} />
              </td>
              <td
                className="relative py-3"
                onMouseEnter={() => handleMouseEnter(skill.name)}
                onMouseLeave={handleMouseLeave}
              >
                <span className="cursor-pointer text-xs text-text-muted hover:text-text">
                  {skill.last_eval_summary ? 'View details' : 'No eval'}
                </span>
                {hoveredSkill === skill.name && skill.last_eval_summary && (
                  <EvalTooltip summary={skill.last_eval_summary} />
                )}
              </td>
              <td className="py-3 text-sm text-text-muted">
                {skill.last_change_at ? (
                  <span title={skill.last_change_at}>{formatRelativeTime(skill.last_change_at)}</span>
                ) : (
                  '—'
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
