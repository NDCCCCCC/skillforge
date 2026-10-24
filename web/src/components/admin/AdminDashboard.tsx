// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Admin console dashboard shell — admin UI redesign 2610.
//
// Layout (was: horizontal tab strip; now: left grouped sidebar):
//   - Left sidebar: grouped nav (数据 / 运营 / 智能层 / 平台) + compact
//     CapacityMonitor pinned at the bottom + Back to Chat.
//   - Top bar: "Admin Console" title + /admin badge + Refresh.
//   - Main: the active view component, full-width.
//
// The shell is structural only — the view components own their data
// fetching. All original data-testids are preserved (admin-dashboard,
// admin-dashboard-topbar, admin-back-to-chat, admin-route-badge,
// admin-refresh, admin-subroute-tabs, admin-tab-*).
//
// History:
//   Plan 20-01 (ADM-01) — original shell.
//   Plan 26-08 (KBM-02) — Knowledge Bases tab.
//   Plan 29-08 (D-29-12) — 反馈 + 复核 tabs; /admin owns its SfToast.
//   Plans 38-05 / 39-08 / 40-07a / 41-04 / Phase 52 — ontology /
//     pipeline / governance / platform / llm tabs.
//   Admin UI redesign 2610 — sidebar shell + grouped navigation.

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
  ShieldCheck,
  SlidersHorizontal,
  Wrench,
  Workflow,
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
import PipelineTab from './pipeline/PipelineTab';
import GovernanceTab from './governance/GovernanceTab';
import PlatformTab from './platform/PlatformTab';
import LLMProviderPanel from './llm/LLMProviderPanel';

export type AdminSubroute =
  | 'batches'
  | 'bases'
  | 'kpis'
  | 'skills'
  | 'schedules'
  | 'feedback'
  | 'review'
  | 'ontology'
  | 'pipeline'
  | 'governance'
  | 'platform'
  | 'llm';

interface NavItem {
  key: AdminSubroute;
  label: string;
  icon: React.ReactNode;
  testId: string;
}

interface NavGroup {
  title: string;
  items: NavItem[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    title: '数据',
    items: [
      { key: 'batches', label: '导入批次', icon: <Database className="h-4 w-4" />, testId: 'admin-tab-batches' },
      { key: 'bases', label: '知识库', icon: <BookOpen className="h-4 w-4" />, testId: 'admin-tab-bases' },
    ],
  },
  {
    title: '运营',
    items: [
      { key: 'kpis', label: 'KPI 定义', icon: <BarChart3 className="h-4 w-4" />, testId: 'admin-tab-kpis' },
      { key: 'skills', label: 'Skill 管理', icon: <Wrench className="h-4 w-4" />, testId: 'admin-tab-skills' },
      { key: 'schedules', label: '定时任务', icon: <Clock className="h-4 w-4" />, testId: 'admin-tab-schedules' },
      { key: 'feedback', label: '反馈', icon: <MessageSquare className="h-4 w-4" />, testId: 'admin-tab-feedback' },
      { key: 'review', label: '复核', icon: <Inbox className="h-4 w-4" />, testId: 'admin-tab-review' },
    ],
  },
  {
    title: '智能层',
    items: [
      { key: 'ontology', label: '本体', icon: <Network className="h-4 w-4" />, testId: 'admin-tab-ontology' },
      { key: 'pipeline', label: 'ECL 管线', icon: <Workflow className="h-4 w-4" />, testId: 'admin-tab-pipeline' },
      { key: 'governance', label: '治理', icon: <ShieldCheck className="h-4 w-4" />, testId: 'admin-tab-governance' },
    ],
  },
  {
    title: '平台',
    items: [
      { key: 'platform', label: '平台工具', icon: <SlidersHorizontal className="h-4 w-4" />, testId: 'admin-tab-platform' },
      { key: 'llm', label: '模型配置', icon: <Wrench className="h-4 w-4" />, testId: 'admin-tab-llm' },
    ],
  },
];

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
    // Refresh is a no-op for now — the views handle their own data fetching.
    // Wired for future: trigger a re-fetch of the active subroute's data.
  }

  function handleSubroute(next: AdminSubroute) {
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
      className="flex h-full bg-bg text-text"
    >
      {/* Left sidebar */}
      <aside className="flex w-52 flex-shrink-0 flex-col border-r border-border bg-surface/60">
        {/* Sidebar top: back + brand */}
        <div className="flex h-11 items-center gap-2 border-b border-border px-3">
          <button
            type="button"
            data-testid="admin-back-to-chat"
            onClick={handleBackToChat}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span>Back to Chat</span>
          </button>
        </div>

        {/* Grouped navigation */}
        <nav
          data-testid="admin-subroute-tabs"
          className="flex-1 overflow-y-auto px-2 py-3"
        >
          {NAV_GROUPS.map((group) => (
            <div key={group.title} className="mb-3">
              <div className="px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-text-dim">
                {group.title}
              </div>
              <div className="flex flex-col gap-0.5">
                {group.items.map((item) => (
                  <SidebarItem
                    key={item.key}
                    active={adminSubroute === item.key}
                    testId={item.testId}
                    icon={item.icon}
                    label={item.label}
                    onClick={() => handleSubroute(item.key)}
                  />
                ))}
              </div>
            </div>
          ))}
        </nav>

        {/* Sidebar bottom: compact capacity strip */}
        <div className="border-t border-border px-3 py-2">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-text-dim">
            容量
          </div>
          <CapacityMonitor compact />
        </div>
      </aside>

      {/* Right column: top bar + body */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar */}
        <header
          data-testid="admin-dashboard-topbar"
          className="flex h-11 items-center justify-between border-b border-border bg-surface px-4"
        >
          <div className="flex items-center gap-3">
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
          {adminSubroute === 'pipeline' && <PipelineTab />}
          {adminSubroute === 'governance' && <GovernanceTab />}
          {adminSubroute === 'platform' && <PlatformTab />}
          {adminSubroute === 'llm' && <LLMProviderPanel />}
        </main>
      </div>

      {/* Plan 29-08 — the /admin toast viewport. App.tsx early-returns for
          /admin, so this branch mounts its own 'sf-toast' listener + slot
          (disposition / metrics toasts stay visible, UI-SPEC §Component 5). */}
      <SfToast />
    </div>
  );
}

// SidebarItem — nav entry in the left sidebar (admin UI redesign 2610).
// Replaces the old inline horizontal SubrouteTab (which itself was copied
// from EvalDashboard.tsx per the Plan 20-01 instruction; the shared-file
// prohibition is lifted by the 2610 redesign).
function SidebarItem({
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
        'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs font-medium transition-colors',
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
