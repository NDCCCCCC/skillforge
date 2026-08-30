// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-01 (ADM-01) — admin console dashboard shell.
//
// Responsibilities:
//   - Top bar: "Admin Console" title + /admin badge + Back to Chat button + Refresh
//   - Tab bar: 4 SubrouteTab buttons (Import Batches / KPI Definitions / Skill Management / Scheduled Tasks)
//   - CapacityMonitor: capacity bar strip (audit / kpi_calls / report_runs)
//   - ViewBody: renders the active view component based on adminSubroute
//
// The shell is structural only — the four view components are implemented
// in later plans. SubrouteTab is copied inline from EvalDashboard.tsx per
// the plan instruction (do NOT extract to a shared file).

import { useEffect } from 'react';
import { ArrowLeft, BarChart3, Clock, Database, RefreshCw, Wrench } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import CapacityMonitor from './CapacityMonitor';
import ImportBatchList from './ImportBatchList';

export default function AdminDashboard() {
  const adminSubroute = useAppStore((s) => s.adminSubroute);
  const setAdminSubroute = useAppStore((s) => s.setAdminSubroute);
  const setRoute = useAppStore((s) => s.setRoute);

  function handleBackToChat() {
    if (window.location.pathname !== '/') {
      window.history.pushState({}, '', '/');
    }
    setRoute('chat');
  }

  function handleRefresh() {
    // Refresh is a no-op for now — the four views handle their own data fetching.
    // Wired for future: trigger a re-fetch of the active subroute's data.
  }

  function handleSubroute(next: 'batches' | 'kpis' | 'skills' | 'schedules') {
    setAdminSubroute(next);
    // URL is /admin for all subroutes (store-only subroute, no sub-path segment).
    if (window.location.pathname !== '/admin') {
      window.history.pushState({}, '', '/admin');
    }
  }

  // Sync URL → subroute on mount so deep-links land correctly.
  useEffect(() => {
    const path = window.location.pathname;
    if (path === '/admin') {
      setAdminSubroute('batches');
    }
  }, [setAdminSubroute]);

  return (
    <div
      data-testid="admin-dashboard"
      className="flex h-full flex-col bg-bg text-text"
    >
      {/* Top bar */}
      <header
        data-testid="admin-dashboard-topbar"
        className="flex h-11 items-center justify-between border-b border-border bg-surface px-4"
      >
        <div className="flex items-center gap-3">
          <button
            type="button"
            data-testid="admin-back-to-chat"
            onClick={handleBackToChat}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span>Back to Chat</span>
          </button>
          <h1 className="flex items-center gap-1.5 text-base font-semibold text-text">
            Admin Console
          </h1>
          <span
            data-testid="admin-route-badge"
            className="inline-flex items-center gap-1 rounded-md bg-primary-dim px-2 py-0.5 font-mono text-[11px] text-primary"
          >
            /admin
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            data-testid="admin-refresh"
            onClick={handleRefresh}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            <span>Refresh</span>
          </button>
        </div>
      </header>

      {/* Tab bar */}
      <nav
        data-testid="admin-subroute-tabs"
        className="flex items-center gap-1 border-b border-border bg-surface px-4 py-2"
      >
        <SubrouteTab
          active={adminSubroute === 'batches'}
          testId="admin-tab-batches"
          icon={<Database className="h-3.5 w-3.5" />}
          label="导入批次"
          onClick={() => handleSubroute('batches')}
        />
        <SubrouteTab
          active={adminSubroute === 'kpis'}
          testId="admin-tab-kpis"
          icon={<BarChart3 className="h-3.5 w-3.5" />}
          label="KPI 定义"
          onClick={() => handleSubroute('kpis')}
        />
        <SubrouteTab
          active={adminSubroute === 'skills'}
          testId="admin-tab-skills"
          icon={<Wrench className="h-3.5 w-3.5" />}
          label="Skill 管理"
          onClick={() => handleSubroute('skills')}
        />
        <SubrouteTab
          active={adminSubroute === 'schedules'}
          testId="admin-tab-schedules"
          icon={<Clock className="h-3.5 w-3.5" />}
          label="定时任务"
          onClick={() => handleSubroute('schedules')}
        />
      </nav>

      {/* CapacityMonitor */}
      <div className="border-b border-border px-4 py-3">
        <CapacityMonitor />
      </div>

      {/* ViewBody */}
      <main
        data-testid="admin-dashboard-body"
        className="flex-1 overflow-y-auto p-4"
      >
        {adminSubroute === 'batches' && <ImportBatchList />}
        {adminSubroute === 'kpis' && (
          <div className="text-xs text-text-muted">KPI 定义视图（后续计划实现）</div>
        )}
        {adminSubroute === 'skills' && (
          <div className="text-xs text-text-muted">Skill 管理视图（后续计划实现）</div>
        )}
        {adminSubroute === 'schedules' && (
          <div className="text-xs text-text-muted">定时任务视图（后续计划实现）</div>
        )}
      </main>
    </div>
  );
}

// SubrouteTab — copied inline from EvalDashboard.tsx per plan instruction.
// Do NOT extract to a shared file.
function SubrouteTab({
  active,
  testId,
  icon,
  label,
  onClick,
}: {
  active: boolean;
  testId: string;
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={onClick}
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
        active
          ? 'bg-primary-dim text-primary'
          : 'text-text-muted hover:bg-surface-2 hover:text-text',
      )}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}
