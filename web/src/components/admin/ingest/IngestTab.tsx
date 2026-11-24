// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-03 (ING-05) -- the 摄取任务 admin tab: a Jobs roster + a Job
// detail view that assembles the wave-1 components.
//
// Assembly (the ONLY consumer of 55-01/55-02):
//   IngestJobTimeline   rows + retry slot + expanded panel slot
//     renderPanel -> IngestStepInspect (Variable Inspect panel)
//   IngestStepMatrix    only when the forward-compat docs[] exists (>1 doc)
//   IngestLogStream     forward-compat progress_msg (empty copy today)
//   failed_items table  rendered ONLY when items exist (existence = signal)
//
// Honesty obligations from 55-UI-SPEC §Honesty:
//   - server truth only: no optimistic state, no local stopwatch. After a
//     retry the UI re-fetches; downstream `stale` marks arrive by polling.
//   - one retry entry point (the timeline's failed-step row). No bulk
//     retry, no cancel, no window.confirm (a single click is the verb).
//   - a failed roster load renders an ERROR row, never a fake empty state
//     (a fake "暂无摄取任务" while jobs exist would be a lie).
//   - unknown != blank: unknown statuses render their raw key.
//
// Polling cadence (55-UI-SPEC §Component 7, LOCKED): 10s idle / 2s while
// the job is hot (queued|running, or any step running|failed|stale -- the
// failed/stale family is included so a just-retried job's cascade is
// picked up fast). Error responses never stop the loop.
import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useAppStore } from '../../../store/app-store';
import { PageHeader } from '../shared/Primitives';
import DataTable from '../shared/DataTable';
import StatusBadge from '../shared/StatusBadge';
import { cn } from '../../../lib/cn';
import {
  IngestApiError,
  asApiError,
  fetchJob,
  fetchJobs,
  retryStep,
} from './ingestApi';
import {
  STEP_LABEL,
  STEP_SEQUENCE,
  countDoneSteps,
  deriveStepState,
  formatDuration,
  isDeterministic,
  jobBadge,
} from './types';
import type { IngestJob } from './types';
import IngestJobTimeline from './IngestJobTimeline';
import IngestStepInspect from './IngestStepInspect';
import IngestStepMatrix from './IngestStepMatrix';
import type { MatrixCellData } from './IngestStepMatrix';
import IngestLogStream from './IngestLogStream';

const HOT_POLL_MS = 2000;
const IDLE_POLL_MS = 10000;

/** Toast dispatch helper (same shape as PipelineTab's module helper: the
 * /admin shell owns the single 'sf-toast' viewport). */
function dispatchToast(kind: 'success' | 'error', message: string) {
  window.dispatchEvent(
    new CustomEvent('sf-toast', { detail: { kind, message } }),
  );
}

function formatDate(value: string): string {
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** A job is "hot" while anything about it can still change on its own. */
function isJobHot(job: IngestJob | null): boolean {
  if (!job) return false;
  if (job.status === 'queued' || job.status === 'running') return true;
  return job.steps.some((s) =>
    s.status === 'running' || s.status === 'failed' || s.status === 'stale',
  );
}

/** Shape the forward-compat docs[] into matrix rows. Derivation goes
 * through the SAME mapping table every other surface uses. */
function shapeMatrixDocs(
  docs: NonNullable<IngestJob['docs']>,
): Array<{ fileName: string; cells: MatrixCellData[] }> {
  return docs.map((doc) => ({
    fileName: doc.file_name,
    cells: doc.steps.map((step) => {
      const state = deriveStepState(step);
      return {
        label: state.label,
        tone: state.tone,
        pulse: state.pulse,
        dashed: state.derived === 'skipped',
        durationLabel: formatDuration(step.started_at, step.completed_at),
      };
    }),
  }));
}

export default function IngestTab() {
  const setIngestFocusJobId = useAppStore((s) => s.setIngestFocusJobId);

  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [job, setJob] = useState<IngestJob | null>(null);
  const [jobs, setJobs] = useState<IngestJob[] | null>(null);
  const [loadError, setLoadError] = useState<IngestApiError | null>(null);
  const [detailError, setDetailError] = useState<IngestApiError | null>(null);
  const [retryInFlight, setRetryInFlight] = useState<string | null>(null);
  const [loadingJobs, setLoadingJobs] = useState(true);

  const selectedRef = useRef<string | null>(null);
  selectedRef.current = selectedJobId;

  // Declared before the poll effect: arm() dereferences jobRef on its
  // first synchronous call whenever a job detail is already open.
  const jobRef = useRef<IngestJob | null>(null);
  jobRef.current = job;

  // 55-03 upload handoff: read ONCE on mount, then clear (read-then-clear).
  useEffect(() => {
    const focus = useAppStore.getState().ingestFocusJobId;
    if (focus) {
      setIngestFocusJobId(null);
      setSelectedJobId(focus);
      void fetchJob(focus)
        .then((j) => setJob(j))
        .catch((err: unknown) => {
          const apiErr = asApiError(err);
          if (apiErr.status === 404) {
            dispatchToast('error', '任务不存在或已被清理');
            setSelectedJobId(null);
          } else {
            setDetailError(apiErr);
          }
        });
    }
  }, [setIngestFocusJobId]);

  const loadJobs = useCallback(async () => {
    try {
      const list = await fetchJobs();
      setJobs(list);
      setLoadError(null);
    } catch (err: unknown) {
      // An error roster is NOT an empty roster (honesty rule).
      setLoadError(asApiError(err));
      setJobs(null);
    } finally {
      setLoadingJobs(false);
    }
  }, []);

  const loadJob = useCallback(async (id: string) => {
    try {
      const detail = await fetchJob(id);
      setJob(detail);
      setDetailError(null);
    } catch (err: unknown) {
      const apiErr = asApiError(err);
      if (apiErr.status === 404) {
        dispatchToast('error', '任务不存在或已被清理');
        setJob(null);
        setSelectedJobId(null);
        return;
      }
      setDetailError(apiErr);
    }
  }, []);

  // Poll loop: ONE self-arming timeout whose period flips with the hot
  // state (10s idle / 2s hot). The period is recomputed AFTER each fetch,
  // so a status transition shortens the NEXT wait instead of the one after
  // it. Error responses never break the loop.
  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    async function tick() {
      if (cancelled) return;
      if (selectedRef.current) {
        await loadJob(selectedRef.current);
      } else {
        await loadJobs();
      }
    }

    function arm() {
      if (cancelled) return;
      const hot = selectedRef.current !== null && isJobHot(jobRef.current);
      const period = hot ? HOT_POLL_MS : IDLE_POLL_MS;
      timer = window.setTimeout(async () => {
        await tick();
        arm();
      }, period);
    }

    function onRefresh() {
      void tick();
    }

    void tick();
    arm();
    window.addEventListener('admin-refresh', onRefresh);
    return () => {
      cancelled = true;
      if (timer) window.clearTimeout(timer);
      window.removeEventListener('admin-refresh', onRefresh);
    };
  }, [loadJobs, loadJob]);


  async function handleRetry(stepKey: string) {
    if (!selectedJobId) return;
    setRetryInFlight(stepKey);
    try {
      await retryStep(selectedJobId, stepKey);
      dispatchToast('success', `已重新入队步骤 ${STEP_LABEL[stepKey] ?? stepKey}`);
      // Server truth: re-sync immediately; the downstream stale cascade
      // arrives via polling (never pre-marked locally).
      await loadJob(selectedJobId);
    } catch (err: unknown) {
      dispatchToast('error', `重试失败：${asApiError(err).message}`);
    } finally {
      setRetryInFlight(null);
    }
  }

  function openJob(id: string) {
    setSelectedJobId(id);
    setJob(null);
    setDetailError(null);
    void loadJob(id);
  }

  function backToJobs() {
    setSelectedJobId(null);
    setJob(null);
    setDetailError(null);
    void loadJobs();
  }

  // ---------- Job detail ----------
  if (selectedJobId) {
    const done = job ? countDoneSteps(job.steps) : 0;
    const total = job ? job.steps.length : 0;
    const badge = job ? jobBadge(job.status) : null;

    return (
      <section data-testid="ingest-tab" className="space-y-4">
        <button
          type="button"
          data-testid="ingest-back-to-jobs"
          onClick={backToJobs}
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          返回任务列表
        </button>

        {detailError ? (
          <div
            data-testid="ingest-detail-error"
            role="alert"
            className="rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-xs text-danger"
          >
            任务加载失败 — {detailError.message}
            <button
              type="button"
              onClick={() => void loadJob(selectedJobId)}
              className="ml-2 underline"
            >
              重试
            </button>
          </div>
        ) : !job ? (
          <div data-testid="ingest-detail-loading" className="text-xs text-text-muted">
            加载摄取任务…
          </div>
        ) : (
          <>
            <div className="rounded-md border border-border bg-surface-2 p-3">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  title={job.job_id}
                  className="max-w-[20rem] truncate font-mono text-xs text-text"
                >
                  {job.job_id}
                </span>
                {badge && (
                  <StatusBadge
                    status={job.status}
                    tone={badge.tone}
                    label={badge.label}
                    className={cn(badge.pulse && 'animate-pulse')}
                    testId="ingest-job-badge"
                  />
                )}
                <span className="font-mono text-xs text-text-muted">{job.filename}</span>
              </div>
              <div
                role="meter"
                aria-label={`摄取进度: ${done} of ${total}`}
                aria-valuenow={done}
                aria-valuemin={0}
                aria-valuemax={total}
                className="mt-2 flex items-center gap-2"
              >
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-3">
                  <div
                    className="h-full rounded-full bg-primary"
                    style={{ width: total === 0 ? '0%' : `${(done / total) * 100}%` }}
                  />
                </div>
                <span
                  data-testid="ingest-progress-counter"
                  className="font-mono text-xs text-text-muted"
                >
                  {total === 0 ? '—' : `${done}/${total}`}
                </span>
              </div>
            </div>

            <IngestJobTimeline
              steps={job.steps}
              onRetry={handleRetry}
              retryInFlight={retryInFlight}
              renderPanel={(step) => (
                <IngestStepInspect
                  stepLabel={STEP_LABEL[step.step] ?? step.step}
                  summary={step.summary}
                  artifactRef={step.artifact_ref}
                  attempt={step.attempt}
                  error={step.status === 'skipped' ? undefined : step.error}
                  skipReason={step.skip_reason ?? (step.status === 'skipped' ? step.error : undefined)}
                  deadNote={isDeterministic(step)}
                  showRetryHint={deriveStepState(step).derived === 'error'}
                  isEclStep={step.step.startsWith('ecl_')}
                  onGoEcl={() => useAppStore.getState().setAdminSubroute('pipeline')}
                />
              )}
            />

            {(job.docs?.length ?? 0) > 1 && (
              <IngestStepMatrix
                stepLabels={STEP_SEQUENCE.map((k) => STEP_LABEL[k] ?? k)}
                docs={shapeMatrixDocs(job.docs!)}
                doneDocs={
                  job.docs!.filter((doc) =>
                    doc.steps.every((s) => deriveStepState(s).derived === 'done'),
                  ).length
                }
                totalDocs={job.docs!.length}
              />
            )}

            <IngestLogStream lines={job.progress_msg} />

            {job.failed_items.length > 0 && (
              <section data-testid="ingest-failed-items" className="space-y-2">
                <h2 className="text-sm font-semibold text-text">失败明细</h2>
                <div className="overflow-x-auto rounded-md border border-border">
                  <table data-testid="ingest-failed-items-table" className="w-full">
                    <thead>
                      <tr className="border-b border-border text-xs text-text-muted">
                        <th scope="col" className="px-3 py-1.5 text-left font-semibold">步骤</th>
                        <th scope="col" className="px-3 py-1.5 text-left font-semibold">条目</th>
                        <th scope="col" className="px-3 py-1.5 text-left font-semibold">错误</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {job.failed_items.map((item, i) => (
                        <tr key={`${item.item_key}-${i}`} data-testid="ingest-failed-item-row">
                          <td className="px-3 py-1.5 text-xs">{STEP_LABEL[item.step] ?? item.step}</td>
                          <td
                            title={item.item_key}
                            className="max-w-[16rem] truncate px-3 py-1.5 font-mono text-xs"
                          >
                            {item.item_key}
                          </td>
                          <td
                            title={item.error}
                            className="max-w-xs truncate px-3 py-1.5 text-xs text-status-non-compliant"
                          >
                            {item.error}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            )}

            <p className="text-xs text-text-muted">
              每次重试记入审计（操作者、时间、步骤前后状态）。审计可在治理 → 时间线查看
            </p>
          </>
        )}
      </section>
    );
  }

  // ---------- Jobs roster ----------
  return (
    <section data-testid="ingest-tab" className="space-y-4">
      <PageHeader
        title="摄取任务"
        description="上传与目录扫描的异步摄取流水线（状态、耗时、失败原因、步骤级重试）"
      />
      {loadError ? (
        <div
          data-testid={loadError.status === 403 ? 'ingest-403' : 'ingest-load-error'}
          role="alert"
          className="rounded-md border border-danger/30 bg-danger/5 px-3 py-2 text-xs text-danger"
        >
          {loadError.status === 403 ? (
            '缺少或错误的 X-Admin-Token'
          ) : (
            <>
              摄取任务加载失败
              <button
                type="button"
                onClick={() => void loadJobs()}
                className="ml-2 underline"
              >
                点击重试
              </button>
            </>
          )}
        </div>
      ) : (
        <DataTable<IngestJob>
          columns={[
            {
              key: 'job',
              label: 'Job',
              render: (row) => (
                <span
                  title={row.job_id}
                  className="block max-w-[14rem] truncate font-mono text-xs"
                >
                  {row.job_id}
                </span>
              ),
            },
            {
              key: 'doc',
              label: '文档',
              render: (row) => (
                <span
                  title={row.filename}
                  className="block max-w-[18rem] truncate font-mono text-xs"
                >
                  {row.filename}
                </span>
              ),
            },
            {
              key: 'status',
              label: '状态',
              render: (row) => {
                const badge = jobBadge(row.status);
                return (
                  <StatusBadge
                    status={row.status}
                    tone={badge.tone}
                    label={badge.label}
                    className={cn(badge.pulse && 'animate-pulse')}
                  />
                );
              },
            },
            {
              key: 'progress',
              label: '进度',
              render: (row) => (
                <span className="font-mono text-xs">
                  {row.steps.length === 0
                    ? '—'
                    : `${countDoneSteps(row.steps)}/${row.steps.length}`}
                </span>
              ),
            },
            {
              key: 'created',
              label: '创建时间',
              render: (row) => <span className="text-xs">{formatDate(row.created_at)}</span>,
            },
            {
              key: 'actions',
              label: '操作',
              render: (row) => (
                <button
                  type="button"
                  data-testid={`ingest-view-${row.job_id}`}
                  onClick={() => openJob(row.job_id)}
                  className="rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold hover:bg-surface-3"
                >
                  查看
                </button>
              ),
            },
          ]}
          rows={jobs ?? []}
          rowKey={(row) => row.job_id}
          rowTestId={() => 'ingest-jobs-row'}
          loading={loadingJobs}
          loadingTestid="ingest-jobs-loading"
          empty={{ title: '暂无摄取任务', hint: '上传文档后自动出现在这里' }}
          emptyTestid="ingest-jobs-empty"
          className="rounded-md border border-border"
        />
      )}
    </section>
  );
}
