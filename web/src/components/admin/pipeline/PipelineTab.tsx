// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 39-08 — /admin/pipeline tab (the 9th admin tab, SC-4 + SC-5).
//
// Implements the approved 39-UI-SPEC.md contract VERBATIM:
//   - Control strip: 「运行 ECL」 primary button (confirm-gated) + stage
//     badge + idempotency hint (SchemaProposalsTab generate-box idiom).
//   - Status strip: CapacityBar-geometry progress meter + queue/failure
//     chips + 当前文件 row, aria-live="polite".
//   - Failures table: 6 locked columns, per-row 重试 enabled ONLY on
//     dead rows (single click, no confirm).
//   - Polling: 2s while a run is active, 10s otherwise (D-Prog-01),
//     re-armed on state flip — useEvalReports interval pattern.
//
// HONESTY RULES (39-UI-SPEC §Honesty, enforced): server truth only — no
// optimistic running state, no optimistic counters; NO cancel / batch-
// retry / auto-run controls (fake capabilities); server error strings
// render verbatim after their zh-CN prefix; dead ≠ deleted.

import { useCallback, useEffect, useState } from 'react';
import { Play } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { PipelineApiError, fetchStatus, retryNote, runPipeline } from './pipelineApi';
import type { PipelineFailure, PipelineStatus } from './types';

/** Run ECL confirmation (39-UI-SPEC §Copywriting — verbatim). */
const RUN_CONFIRM_TEXT =
  '运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行）。';

/** D-Prog-01 cadence: 2s while a run is active, 10s otherwise. */
const POLL_RUNNING_MS = 2000;
const POLL_IDLE_MS = 10000;

/** Stage sentinel → zh label (39-UI-SPEC §Stage Mapping). Unknown BE
 * values render the raw key — never blank. */
const STAGE_LABEL: Record<string, string> = {
  idle: '空闲',
  merge: '切分合并',
  extract: '实体抽取',
  'kg-load': 'KG 写入',
  'milvus-load': 'Milvus 索引',
  done: '已完成',
};

/** Failure-row status badge: label + status-token classes
 * (FeedbackMetricsPanel rule — --color-status-* utilities only). */
const FAILURE_BADGE: Record<PipelineFailure['status'], { label: string; cls: string }> = {
  pending: { label: '重试中', cls: 'bg-status-needs-info/10 text-status-needs-info' },
  dead: { label: '死信', cls: 'bg-status-non-compliant/10 text-status-non-compliant' },
  resolved: { label: '已恢复', cls: 'bg-status-compliant/10 text-status-compliant' },
};

/** Dispatch into the SfToast viewport already mounted in AdminDashboard
 * root — never mount a second viewport here. */
function dispatchToast(kind: 'success' | 'error' | 'info', message: string): void {
  window.dispatchEvent(new CustomEvent('sf-toast', { detail: { kind, message } }));
}

function asApiError(err: unknown): PipelineApiError {
  return err instanceof PipelineApiError ? err : new PipelineApiError(0, '网络错误');
}

function LoadingSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="h-10 rounded bg-surface-2 animate-pulse" />
      ))}
    </div>
  );
}

export default function PipelineTab() {
  const [status, setStatus] = useState<PipelineStatus | null>(null);
  const [loadError, setLoadError] = useState<PipelineApiError | null>(null);
  const [firstLoad, setFirstLoad] = useState(true);
  const [runInFlight, setRunInFlight] = useState(false);
  /** note_id of the row whose retry POST is in flight (per-row disable). */
  const [retryInFlight, setRetryInFlight] = useState<string | null>(null);

  const fetchOnce = useCallback(async () => {
    try {
      const data = await fetchStatus();
      setStatus(data);
      setLoadError(null);
    } catch (err: unknown) {
      setLoadError(asApiError(err));
    } finally {
      setFirstLoad(false);
    }
  }, []);

  // Server truth only: stage derives exclusively from the last poll.
  const stage = status?.stage ?? 'idle';
  const isRunning = stage !== 'idle' && stage !== 'done';
  const cadence = isRunning ? POLL_RUNNING_MS : POLL_IDLE_MS;

  // Poll loop (useEvalReports shape): fetchOnce in useCallback +
  // window.setInterval re-armed when the cadence flips so the new
  // cadence applies immediately. Errors never stop the loop.
  useEffect(() => {
    let cancelled = false;
    void fetchOnce();
    const id = window.setInterval(() => {
      if (cancelled) return;
      void fetchOnce();
    }, cadence);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [cadence, fetchOnce]);

  async function handleRun() {
    // Mutating POST + cost-bearing (LLM calls + Milvus re-embed) — ask
    // once (KnowledgeBaseList deactivate precedent). No optimistic
    // running state: 「运行中…」 appears only when a poll confirms it.
    if (!window.confirm(RUN_CONFIRM_TEXT)) return;
    setRunInFlight(true);
    try {
      await runPipeline();
      dispatchToast('info', 'ECL 管线已启动');
      await fetchOnce(); // re-sync server truth immediately
    } catch (err: unknown) {
      const e = asApiError(err);
      if (e.status === 409) {
        dispatchToast('error', '管线正在运行中，请等待完成');
      } else if (e.status === 0) {
        dispatchToast('error', '网络错误');
      } else {
        dispatchToast('error', `启动失败：${e.message}，请稍后重试`);
      }
    } finally {
      setRunInFlight(false);
    }
  }

  async function handleRetry(noteId: string) {
    // Bounded single-note action — single click, NO confirm (Phase 29
    // disposition precedent). Transitions arrive via polling only.
    if (retryInFlight !== null) return;
    setRetryInFlight(noteId);
    try {
      await retryNote(noteId);
      dispatchToast('success', `已重新入队 note ${noteId}`);
      await fetchOnce();
    } catch (err: unknown) {
      const e = asApiError(err);
      dispatchToast('error', `重试失败：${e.message}`);
    } finally {
      setRetryInFlight(null);
    }
  }

  // First load: 3 skeleton rows (KnowledgeBaseList trio). A failed first
  // fetch falls through to the panel + inline error lines.
  if (firstLoad && loadError === null) {
    return (
      <section data-testid="pipeline-tab" className="space-y-4">
        <LoadingSkeleton />
      </section>
    );
  }

  const eclOff = loadError?.status === 503;
  const tokenRejected = loadError?.status === 403;

  const done = status?.done ?? 0;
  const total = status?.total ?? 0;
  const queueLen = status?.queue_len;
  const failures = status?.failures ?? [];
  const currentFile = status?.current_file ?? '';
  const deadPresent = failures.some((f) => f.status === 'dead');

  const pct = total > 0 ? Math.min((done / total) * 100, 100) : 0;
  const neverRun = status !== null && stage === 'idle' && total === 0;

  const runLabel = eclOff ? 'ECL 未启用' : isRunning ? '运行中…' : '运行 ECL';
  const runDisabled = eclOff || isRunning || runInFlight;

  // isRunning drives the pulse badge (amber); done green; everything
  // else (idle + unknown before first truth) neutral.
  const badgeCls = isRunning
    ? 'bg-status-needs-info/10 text-status-needs-info animate-pulse'
    : stage === 'done'
      ? 'bg-status-compliant/10 text-status-compliant'
      : 'bg-surface-3 text-text-muted';
  const badgeLabel = eclOff ? 'ECL 未启用' : (STAGE_LABEL[stage] ?? stage);
  const stageRowLabel = STAGE_LABEL[stage] ?? stage;

  return (
    <section data-testid="pipeline-tab" className="space-y-4">
      {/* Inline error / notice lines. 403 and 503 render their dedicated
          locked copy; any other failure renders the retry-button line. */}
      {loadError !== null && !eclOff && !tokenRejected && (
        <div data-testid="pipeline-load-error" className="text-sm text-danger">
          <button
            type="button"
            onClick={() => void fetchOnce()}
            className="text-danger hover:underline"
          >
            管线状态加载失败
          </button>
          （点击重试）
        </div>
      )}
      {tokenRejected && (
        <div data-testid="pipeline-403" className="text-sm text-danger">
          缺少或错误的 X-Admin-Token
        </div>
      )}
      {eclOff && (
        <div data-testid="pipeline-503" className="text-sm text-text-muted">
          ECL 管线未启用（SKILLFORGE_ECL_ENABLED=off）
        </div>
      )}

      {/* Control strip (SchemaProposalsTab generate-box idiom). */}
      <div className="rounded-md border border-border bg-surface-2 p-3">
        <div className="flex items-center gap-2">
          <button
            type="button"
            data-testid="pipeline-run-btn"
            disabled={runDisabled}
            onClick={() => void handleRun()}
            className="inline-flex h-fit items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg disabled:opacity-50"
          >
            <Play className="h-3.5 w-3.5" />
            {runLabel}
          </button>
          <span
            data-testid="pipeline-stage-badge"
            className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', badgeCls)}
          >
            {badgeLabel}
          </span>
        </div>
        <p className="mt-2 text-xs text-text-muted">
          手动触发；boot 不自动运行。重复运行按 note 级替换，结果幂等。
        </p>
      </div>

      {/* Status strip — poll updates announce politely, never steal focus. */}
      <div aria-live="polite" className="space-y-2">
        {neverRun && (
          <p data-testid="pipeline-idle-hero" className="text-sm text-text-muted">
            尚未运行 — 点击「运行 ECL」启动管线
          </p>
        )}
        <div className="flex items-center gap-2">
          <span className="w-20 flex-shrink-0 text-xs text-text-muted">{stageRowLabel}</span>
          <div
            role="meter"
            aria-valuenow={done}
            aria-valuemin={0}
            aria-valuemax={total}
            aria-label={`ECL 管线进度: ${done} of ${total}`}
            className="flex flex-1 items-center gap-2"
          >
            <div className="flex-1 h-2 bg-surface-2 rounded">
              <div
                className="h-full rounded bg-primary transition-all"
                style={{ width: `${pct}%` }}
              />
            </div>
            <span data-testid="pipeline-progress-counter" className="text-xs font-mono text-text-muted">
              {total === 0 ? '—' : `${done}/${total}`}
            </span>
          </div>
          <span className="flex-shrink-0 text-xs text-text-muted">
            待加载队列 <span className="font-mono">{queueLen ?? '—'}</span>
          </span>
          <span
            className={cn(
              'flex-shrink-0 text-xs',
              failures.length > 0 && deadPresent ? 'text-status-needs-info' : 'text-text-muted',
            )}
          >
            失败 <span className="font-mono">{status !== null ? failures.length : '—'}</span>
          </span>
        </div>
        <div className="flex items-center gap-2">
          <span className="w-20 flex-shrink-0 text-xs text-text-muted">当前文件</span>
          <span
            data-testid="pipeline-current-file"
            className="max-w-full truncate font-mono text-xs text-text"
            title={currentFile || undefined}
          >
            {currentFile || '—'}
          </span>
        </div>
      </div>

      {/* Failures + dead-letter table. The section renders even when the
          pipeline never ran (empty row below) — 39-UI-SPEC §Component 3. */}
      <div className="space-y-2">
        <h2 className="text-sm font-semibold">失败与死信</h2>
        <div className="overflow-hidden rounded-md border border-border">
          <table data-testid="pipeline-failures-table" className="w-full text-left text-xs">
            <thead className="bg-surface-2 text-text-muted">
              <tr>
                <th scope="col" className="px-3 py-2">Note</th>
                <th scope="col" className="px-3 py-2">阶段</th>
                <th scope="col" className="px-3 py-2">尝试</th>
                <th scope="col" className="px-3 py-2">最近错误</th>
                <th scope="col" className="px-3 py-2">状态</th>
                <th scope="col" className="px-3 py-2">操作</th>
              </tr>
            </thead>
            <tbody>
              {failures.map((f) => {
                const badge = FAILURE_BADGE[f.status] ?? {
                  label: f.status,
                  cls: 'bg-surface-3 text-text-muted',
                };
                return (
                  <tr
                    key={f.note_id}
                    data-testid={`pipeline-failure-row-${f.note_id}`}
                    className="border-t border-border hover:bg-surface-2"
                  >
                    <td className="px-3 py-1.5 font-mono">{f.note_id}</td>
                    <td className="px-3 py-1.5 font-mono">{f.stage}</td>
                    <td className="px-3 py-1.5 font-mono">{f.attempt_count}/3</td>
                    <td
                      className={cn(
                        'max-w-xs truncate px-3 py-1.5',
                        f.status === 'dead' && 'text-status-non-compliant',
                      )}
                      title={f.last_error}
                    >
                      {f.last_error}
                    </td>
                    <td className="px-3 py-1.5">
                      <span className={cn('rounded px-1.5 py-0.5 text-[11px] font-semibold', badge.cls)}>
                        {badge.label}
                      </span>
                    </td>
                    <td className="px-3 py-1.5">
                      <button
                        type="button"
                        data-testid={`pipeline-retry-${f.note_id}`}
                        disabled={f.status !== 'dead' || retryInFlight === f.note_id}
                        onClick={() => void handleRetry(f.note_id)}
                        aria-label={`重试 note ${f.note_id}`}
                        className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold hover:bg-surface-3 disabled:opacity-50"
                      >
                        重试
                      </button>
                    </td>
                  </tr>
                );
              })}
              {failures.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-3 text-center text-text-muted">
                    暂无失败记录 — 管线运行中的失败自动重试 3 次后进入死信，可在此手动重试
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
