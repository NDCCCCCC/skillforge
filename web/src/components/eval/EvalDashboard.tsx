// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 06-04 (D6-01..04 + D6-03) — admin-mode dashboard shell.
//
// Responsibilities:
//   - Top bar: "Eval Dashboard" title + manual Refresh (D6-03) +
//     back-to-chat button + subroute tabs (Dashboard / Audit).
//   - Compose the three sub-components: SkillsTable + TrendPanel +
//     CrossEvalTab for the dashboard subroute, AuditTable for the
//     /eval/audit subroute.
//   - Own the 30s polling loop (D6-03) — the hook fans out to
//     /api/eval/reports + /api/eval/frozen + /api/audit.
//
// The eval slice in the Zustand store does NOT contain a `messages`
// field (D14 invariant); this component reads from that slice + the
// polling state, and does NOT import the chat SSE transport.

import { useEffect } from 'react';
import { ArrowLeft, BarChart3, ClipboardList, RefreshCw } from 'lucide-react';
import { cn } from '../../lib/cn';
import { useEvalReports, useEvalFrozen } from '../../hooks/useEvalReports';
import { useAppStore } from '../../store/app-store';
import SkillsTable from './SkillsTable';
import TrendPanel from './TrendPanel';
import CrossEvalTab from './CrossEvalTab';
import AuditTable from './AuditTable';

export interface EvalDashboardProps {
  /** Test-only override for the polling cadence. Default 30s per D6-03. */
  pollIntervalMs?: number;
}

export default function EvalDashboard({ pollIntervalMs = 30_000 }: EvalDashboardProps = {}) {
  const evalSubroute = useAppStore((s) => s.evalSubroute);
  const setEvalSubroute = useAppStore((s) => s.setEvalSubroute);
  const setRoute = useAppStore((s) => s.setRoute);

  // Plan 06-04 (D6-03) — 30s polling cadence (configurable via prop
  // for tests). Mounted at the dashboard root; cleared on unmount.
  const { refetch: refetchReports } = useEvalReports(pollIntervalMs);
  useEvalFrozen();

  function handleRefresh() {
    refetchReports();
  }

  function handleBackToChat() {
    if (window.location.pathname !== '/') {
      window.history.pushState({}, '', '/');
    }
    setRoute('chat');
  }

  function handleSubroute(next: 'dashboard' | 'audit') {
    setEvalSubroute(next);
    const targetPath = next === 'audit' ? '/eval/audit' : '/eval';
    if (window.location.pathname !== targetPath) {
      window.history.pushState({}, '', targetPath);
    }
  }

  // Sync URL → subroute on mount so deep-links land correctly.
  useEffect(() => {
    const path = window.location.pathname;
    if (path === '/eval/audit') {
      setEvalSubroute('audit');
    } else {
      setEvalSubroute('dashboard');
    }
  }, [setEvalSubroute]);

  return (
    <div
      data-testid="eval-dashboard"
      className="flex h-full flex-col bg-bg text-text"
    >
      {/* Top bar */}
      <header
        data-testid="eval-dashboard-topbar"
        className="flex h-11 items-center justify-between border-b border-border bg-surface px-4"
      >
        <div className="flex items-center gap-3">
          <button
            type="button"
            data-testid="eval-back-to-chat"
            onClick={handleBackToChat}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span>Back to Chat</span>
          </button>
          <h1 className="flex items-center gap-1.5 text-base font-semibold text-text">
            <BarChart3 className="h-4 w-4 text-primary" />
            Eval Dashboard
          </h1>
          <span
            data-testid="eval-route-badge"
            className="inline-flex items-center gap-1 rounded-md bg-primary-dim px-2 py-0.5 font-mono text-[11px] text-primary"
          >
            /eval{evalSubroute === 'audit' ? '/audit' : ''}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            data-testid="eval-refresh"
            onClick={handleRefresh}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            <span>Refresh</span>
          </button>
        </div>
      </header>

      {/* Subroute tabs */}
      <nav
        data-testid="eval-subroute-tabs"
        className="flex items-center gap-1 border-b border-border bg-surface px-4 py-2"
      >
        <SubrouteTab
          active={evalSubroute === 'dashboard'}
          testId="eval-subroute-dashboard"
          icon={<BarChart3 className="h-3.5 w-3.5" />}
          label="Dashboard"
          onClick={() => handleSubroute('dashboard')}
        />
        <SubrouteTab
          active={evalSubroute === 'audit'}
          testId="eval-subroute-audit"
          icon={<ClipboardList className="h-3.5 w-3.5" />}
          label="Audit Log"
          onClick={() => handleSubroute('audit')}
        />
      </nav>

      {/* Body */}
      <main
        data-testid="eval-dashboard-body"
        className="flex-1 overflow-y-auto p-4"
      >
        {evalSubroute === 'audit' ? <AuditTable /> : (
          <div className="flex flex-col gap-6">
            <SkillsTable />
            <TrendPanel />
            <CrossEvalTab />
          </div>
        )}
      </main>
    </div>
  );
}

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