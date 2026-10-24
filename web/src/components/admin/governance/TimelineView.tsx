// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07b — TBox version timeline with rollback. Newest-first.
// Rollback lifecycle: confirm → POST → toast with new-violation count → refetch.
// Per UI-SPEC §Component 6 + D-LOG-02 who badge colors.
//
// 2610 redesign: the mount-time changelog load migrated to the shared
// useAdminData hook; the rollback CTA migrated to the shared Button. The
// post-rollback refresh stays a direct fetchChangelog call (NOT hook
// reload): a failed secondary refresh must stay silent — a hook reload
// that errors would swap the timeline for the error branch and
// repurpose a successful rollback as a page failure (T-47-14).

import { useState } from 'react';
import { cn } from '../../../lib/cn';
import { useAdminData } from '../shared/useAdminData';
import { Button } from '../../ui/button';
import { fetchChangelog, rollbackTBox } from './governanceApi';
import type { ChangelogEntry } from './types';

// who badge colors per UI-SPEC §Color
const WHO_BADGE: Record<string, string> = {
  admin: 'bg-surface-3 text-text-muted',
  'induction:llm': 'bg-primary-dim text-primary',
  'system:rollback': 'bg-status-needs-info/10 text-status-needs-info',
};

const WHO_LABEL: Record<string, string> = {
  admin: '管理员',
  'induction:llm': 'LLM 归纳',
  'system:rollback': '系统回滚',
};

function TimelineEntry({
  entry,
  isCurrent,
  onRollback,
  rollingBack,
}: {
  entry: ChangelogEntry;
  isCurrent: boolean;
  onRollback: (v: number) => void;
  rollingBack: number | null;
}) {
  return (
    <div
      data-testid={`governance-timeline-entry-${entry.version}`}
      className="flex gap-3 items-start"
    >
      {/* Dot */}
      <div className="mt-1.5 h-2 w-2 rounded-full bg-primary shrink-0" />

      <div className="flex-1 space-y-1">
        {/* Row: version + who badge + current badge + timestamp */}
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-mono text-xs font-semibold">v{entry.version}</span>

          <span
            className={cn(
              'rounded px-1.5 py-0.5 text-[11px] font-semibold',
              WHO_BADGE[entry.who] ?? 'bg-surface-3 text-text-muted',
            )}
          >
            {WHO_LABEL[entry.who] ?? entry.who}
          </span>

          {isCurrent && (
            <span className="rounded bg-status-compliant/10 px-1.5 py-0.5 text-[11px] font-semibold text-status-compliant">
              当前
            </span>
          )}

          <span className="font-mono text-xs text-text-muted">
            {new Date(entry.when).toLocaleString()}
          </span>
        </div>

        {/* what line: +/- counts + collapsible detail */}
        <div className="font-mono text-xs text-text-muted">
          <span className="text-status-compliant">+{entry.what.added.length}</span>
          {' / '}
          <span className="text-danger">-{entry.what.removed.length}</span>
          {entry.what.added.length > 0 && (
            <details className="ml-2 inline">
              <summary className="cursor-pointer text-xs">详情</summary>
              <pre className="text-xs mt-1 p-2 bg-surface-2 rounded overflow-auto max-w-xs break-all">
                {entry.what.added.join('\n')}
              </pre>
            </details>
          )}
        </div>

        {/* why line */}
        <div className="text-xs text-text-muted line-clamp-2" title={entry.why}>
          {entry.why}
        </div>

        {/* Rollback button — only on non-current entries */}
        {!isCurrent && (
          <Button
            variant="secondary"
            size="sm"
            data-testid={`governance-rollback-${entry.version}`}
            onClick={() => onRollback(entry.version)}
            disabled={rollingBack !== null}
            className="mt-1"
            aria-label={`回滚到版本 v${entry.version}`}
          >
            {rollingBack === entry.version ? '回滚中...' : '回滚到此版本'}
          </Button>
        )}
      </div>
    </div>
  );
}

export interface TimelineViewProps {
  /**
   * Fired after a successful rollback so GovernanceTab can refetch the cached
   * SHACL report (REL-02 / D-02b). Optional — the rollback path must not depend
   * on a parent being wired.
   */
  onRollbackComplete?: () => void;
}

export default function TimelineView({ onRollbackComplete }: TimelineViewProps = {}) {
  // Mount-time load via the shared hook; `local` carries the post-rollback
  // secondary refresh so its failures stay silent (see file header).
  const {
    data: hookChangelog,
    loading: hookLoading,
    error: hookError,
  } = useAdminData<{ current_version: number; entries: ChangelogEntry[] }>(
    '/api/admin/ontology/changelog',
  );
  const [local, setLocal] = useState<{
    current_version: number;
    entries: ChangelogEntry[];
  } | null>(null);
  const [rollingBack, setRollingBack] = useState<number | null>(null);

  const changelog = local ?? hookChangelog;
  const loading = hookLoading && local === null;

  const handleRollback = async (targetVersion: number) => {
    if (!changelog) return;
    const nextVersion = (changelog.current_version ?? 0) + 1;

    if (
      !window.confirm(
        `回滚到 v${targetVersion}？将写入新版本 v${nextVersion}、重建 KG 并自动重跑 SHACL 校验；失效引用实体将进入隔离区。`,
      )
    ) {
      return;
    }

    setRollingBack(targetVersion);
    try {
      const result = await rollbackTBox(targetVersion);

      // Success toast
      const toast = document.createElement('div');
      toast.className = 'sf-toast-success';
      toast.textContent = `已回滚至 v${targetVersion}（当前 v${result.version}）· 新增 ${result.new_violations} 条 violation`;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 5000);

      // The POST above is the ONLY thing that decides success. Everything
      // below it is a secondary refresh and runs outside the rollback's
      // outcome classification: a changelog refetch failure or a throwing
      // parent handler must never repurpose a rollback that genuinely
      // happened into `回滚失败：…` (T-47-14 — repudiation, MEDIUM-1 in
      // 47-REVIEW.md, where the awaited refetch sat inside this try and its
      // rejection reached the catch).

      // Notify the parent FIRST and unconditionally so the REL-02 contract
      // (the post-rollback SHACL report must reach the banner) does not
      // depend on the changelog refetch's fate. Guarded: this handler may not
      // throw into the caller either.
      try {
        onRollbackComplete?.();
      } catch {
        // swallowed — see comment above
      }

      // Fire-and-forget changelog refresh. A rejection here is swallowed:
      // the changelog view is stale, but the rollback succeeded, and the
      // stale list is honest feedback rather than a false failure toast.
      void fetchChangelog().then(setLocal).catch(() => {
        /* secondary refresh only — never a rollback outcome */
      });
    } catch (e: unknown) {
      const errMsg = e instanceof Error ? e.message : String(e);
      const toast = document.createElement('div');
      toast.className = 'sf-toast-error';
      toast.textContent = `回滚失败：${errMsg}`;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 5000);
    } finally {
      setRollingBack(null);
    }
  };

  if (hookError) {
    return (
      <div className="text-sm text-danger">
        版本日志加载失败
        <button
          onClick={() => window.location.reload()}
          className="ml-2 underline"
        >
          （点击重试）
        </button>
      </div>
    );
  }

  if (loading || !changelog) {
    return (
      <div className="space-y-2">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-16 animate-pulse rounded bg-surface-2" />
        ))}
      </div>
    );
  }

  if (changelog.entries.length === 0) {
    return (
      <div className="text-sm text-text-muted">
        暂无演进记录 — 当前为初始版本 v{changelog.current_version}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Current version header */}
      <div className="text-sm font-semibold">
        当前版本{' '}
        <span className="font-mono text-primary">v{changelog.current_version}</span>
      </div>

      <h2 className="text-sm font-semibold">版本演进</h2>

      {/* Timeline entries — newest first */}
      <div className="space-y-3">
        {changelog.entries.map((entry) => (
          <TimelineEntry
            key={entry.version}
            entry={entry}
            isCurrent={entry.version === changelog.current_version}
            onRollback={handleRollback}
            rollingBack={rollingBack}
          />
        ))}
      </div>
    </div>
  );
}
