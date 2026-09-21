// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 20-01 (ADM-01) — admin console dashboard shell.
//
// Responsibilities:
//   - Top bar: "Admin Console" title + /admin badge + Back to Chat button + Refresh
//   - Tab bar: 7 SubrouteTab buttons (Import Batches / Knowledge Bases / KPI
//     Definitions / Skill Management / Scheduled Tasks / 反馈 / 复核)
//   - CapacityMonitor: capacity bar strip (audit / kpi_calls / report_runs)
//   - ViewBody: renders the active view component based on adminSubroute
//
// The shell is structural only — the view components own their data
// fetching. SubrouteTab is copied inline from EvalDashboard.tsx per the
// plan instruction (do NOT extract to a shared file).
//
// Plan 26-08 (KBM-02) adds the Knowledge Bases tab — the registry's
// mutation surface (create / deactivate, token-gated, audit-backed).
//
// Plan 29-08 (D-29-12) adds tabs 6 + 7 — 「反馈」 (FeedbackMetricsPanel,
// FBK-03 仪表盘) and 「复核」 (ReviewQueuePanel, FBK-02 复核队列) — and
// mounts <SfToast /> in this root div: /admin early-returns from App.tsx
// before App's own toast mounts, so the admin branch owns its viewport.

import { useEffect } from 'react';
import {
  ArrowLeft,
  BarChart3,
  BookOpen,
  Clock,
  Database,
  Inbox,
  MessageSquare,
  Network,
  RefreshCw,
  Wrench,
} from 'lucide-react';
import { cn } from '../../lib/cn';
import { useAppStore } from '../../store/app-store';
import SfToast from '../layout/SfToast';
import CapacityMonitor from './CapacityMonitor';
import ImportBatchList from './ImportBatchList';
import KnowledgeBaseList from './KnowledgeBaseList';
import DocUpload from './DocUpload';
import KPIDefinitionList from './KPIDefinitionList';
import SkillManagementList from './SkillManagementList';
import ScheduledTaskList from './ScheduledTaskList';
import FeedbackMetricsPanel from './FeedbackMetricsPanel';
import ReviewQueuePanel from './ReviewQueuePanel';
import OntologyTab from './ontology/OntologyTab';

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

  function handleSubroute(
    next: 'batches' | 'bases' | 'kpis' | 'skills' | 'schedules' | 'feedback' | 'review' | 'ontology',
  ) {
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
          active={adminSubroute === 'bases'}
          testId="admin-tab-bases"
          icon={<BookOpen className="h-3.5 w-3.5" />}
          label="Knowledge Bases"
          onClick={() => handleSubroute('bases')}
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
        {/* Plan 29-08 (D-29-12) — 反馈 is the 6th tab, 复核 the 7th. */}
        <SubrouteTab
          active={adminSubroute === 'feedback'}
          testId="admin-tab-feedback"
          icon={<MessageSquare className="h-3.5 w-3.5" />}
          label="反馈"
          onClick={() => handleSubroute('feedback')}
        />
        <SubrouteTab
          active={adminSubroute === 'review'}
          testId="admin-tab-review"
          icon={<Inbox className="h-3.5 w-3.5" />}
          label="复核"
          onClick={() => handleSubroute('review')}
        />
        {/* Plan 38-05 — 本体 (the 8th tab): TBox browser + KG query +
            schema-proposal approval Drawer (v1.4 Ontology Layer). */}
        <SubrouteTab
          active={adminSubroute === 'ontology'}
          testId="admin-tab-ontology"
          icon={<Network className="h-3.5 w-3.5" />}
          label="本体"
          onClick={() => handleSubroute('ontology')}
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
        {adminSubroute === 'bases' && (
          <>
            <KnowledgeBaseList />
            <DocUpload />
          </>
        )}
        {adminSubroute === 'kpis' && <KPIDefinitionList />}
        {adminSubroute === 'skills' && <SkillManagementList />}
        {adminSubroute === 'schedules' && <ScheduledTaskList />}
        {adminSubroute === 'feedback' && <FeedbackMetricsPanel />}
        {adminSubroute === 'review' && <ReviewQueuePanel />}
        {adminSubroute === 'ontology' && <OntologyTab />}
      </main>

      {/* Plan 29-08 — the /admin toast viewport. App.tsx early-returns for
          /admin, so this branch mounts its own 'sf-toast' listener + slot
          (disposition / metrics toasts stay visible, UI-SPEC §Component 5). */}
      <SfToast />
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
