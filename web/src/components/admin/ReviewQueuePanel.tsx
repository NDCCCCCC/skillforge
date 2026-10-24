// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 29-08 Task 2 — 「复核」 tab (FBK-02): the human-disposition view of
// outdated-report feedback, against the frozen 29-05 wire contracts:
//
// Migrated to the shared admin component library (admin UI redesign 2610):
// PageHeader, the shared LoadingSkeleton, the shared SegmentedButton
// (previously duplicated with FeedbackMetricsPanel) and StatusBadge for
// the disposition-history pill (replacing the per-disposition color map).
// Deliberately NOT on useAdminData: this view POSTs dispositions, serves
// two endpoints on one screen and renders the SERVER error verbatim
// (review-error-detail) — useAdminData collapses failures into a boolean.
//
//   GET  /api/admin/feedback/review?status=pending|resolved[&disposition=…]
//     → 200 {pending:[{chunk_id, kb_slug, report_count, last_reported_at, comments[]}], total}
//     | 200 {dispositions:[{id, ts, chunk_id, disposition, note, actor}]}
//   POST /api/admin/feedback/dispositions {chunk_id, disposition} → 201 {closed_count} | 400/403/503 {error}
//
// Trust semantics (FBK-05 / D-29-08 / D-29-09):
//   - A disposition NEVER touches the knowledge base. It INSERTs one
//     state row + one audit row server-side; the trust banner saying so
//     is always visible above the data.
//   - Pending rows are chunk-AGGREGATED: 报告次数 is the priority signal
//     (≥2 → amber + bold), and ONE disposition closes every pending
//     entry of that chunk — the toast names the closed count so the
//     batch effect is never silent.
//   - Single click, no confirm modal: the disposition is a terminal
//     status marker, and the INSERT-only audit row is the record. There
//     is deliberately no delete / undo control (a button that 404s would
//     be a fake capability).
//   - The list is re-fetched from SERVER TRUTH after a 201; a refused
//     mutation keeps the row and renders body.error verbatim.
//   - Chunk preview rides the EXISTING GET /api/knowledge/chunk/:id read
//     surface (fetchChunkContent) — zero new reads.
//
// TRUST BOUNDARY: every request carries X-Admin-Token and the SERVER
// decides. This component never gates; a 403 renders the token hint the
// backend returned (KnowledgeBaseList honesty rule).
//
// Rendering safety (T-29-09): server error strings, report comments and
// the chunk preview flow through JSX text nodes only.
import { Fragment, useCallback, useEffect, useState } from 'react';
import { fetchChunkContent } from '../../lib/chunk-lookup';
import { cn } from '../../lib/cn';
import { Button } from '../ui/button';
import SegmentedButton from './shared/SegmentedButton';
import { LoadingSkeleton } from './shared/States';
import StatusBadge from './shared/StatusBadge';
import { PageHeader } from './shared/Primitives';

interface PendingRow {
  chunk_id: number;
  kb_slug: string;
  report_count: number;
  last_reported_at: string;
  comments: string[];
}

interface DispositionEntry {
  id: number;
  ts: string;
  chunk_id: number;
  disposition: string;
  note: string;
  actor: string;
}

/** The closed three-value disposition vocabulary (DDL CHECK backstop). */
const DISPOSITIONS = ['updated', 'false_positive', 'ignored'] as const;
type Disposition = (typeof DISPOSITIONS)[number];

const DISPOSITION_LABEL: Record<Disposition, string> = {
  updated: '已更新',
  false_positive: '误报',
  ignored: '忽略',
};

/** History-badge tone per 29-UI-SPEC §Color: 已更新 green / 误报 red / 忽略 gray. */
const DISPOSITION_TONE = {
  updated: 'green',
  false_positive: 'red',
  ignored: 'gray',
} as const satisfies Record<Disposition, 'green' | 'red' | 'gray'>;

const TRUST_BANNER =
  '复核处置仅更新信号状态并写入审计日志，不会自动修改知识库内容；知识更新请通过「Knowledge Bases」上传流程完成。';

type ToastKind = 'success' | 'error' | 'info';

function toast(kind: ToastKind, message: string) {
  window.dispatchEvent(new CustomEvent('sf-toast', { detail: { kind, message } }));
}

/** Component-local relative time (admin-family precedent: each panel owns
 *  its tiny formatter — minutes/hours/days three tiers). */
export function formatRelativeTime(iso: string, now: number = Date.now()): string {
  const ts = new Date(iso).getTime();
  if (Number.isNaN(ts)) return '—';
  const mins = Math.floor(Math.max(0, now - ts) / 60_000);
  if (mins < 1) return '刚刚';
  if (mins < 60) return `${mins} 分钟前`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} 小时前`;
  return `${Math.floor(hours / 24)} 天前`;
}

type PreviewState =
  | { state: 'loading' }
  | { state: 'ready'; text: string }
  | { state: 'error' };

export default function ReviewQueuePanel() {
  const [statusTab, setStatusTab] = useState<'pending' | 'resolved'>('pending');
  const [historyFilter, setHistoryFilter] = useState<'all' | Disposition>('all');
  const [pending, setPending] = useState<PendingRow[] | null>(null);
  const [total, setTotal] = useState(0);
  const [history, setHistory] = useState<DispositionEntry[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [serverError, setServerError] = useState('');
  const [expanded, setExpanded] = useState<number[]>([]);
  const [previews, setPreviews] = useState<Record<string, PreviewState>>({});
  const [busyChunk, setBusyChunk] = useState<number | null>(null);
  const [actionError, setActionError] = useState<{ chunkId: number; message: string } | null>(null);

  const loadQueue = useCallback((status: 'pending' | 'resolved', disp: 'all' | Disposition) => {
    setLoading(true);
    const token = localStorage.getItem('sf_admin_token') ?? '';
    let url = `/api/admin/feedback/review?status=${status}`;
    if (status === 'resolved' && disp !== 'all') url += `&disposition=${disp}`;
    return fetch(url, { headers: { 'X-Admin-Token': token } })
      .then(async (resp) => {
        if (!resp.ok) {
          const body = (await resp.json().catch(() => ({}))) as { error?: unknown };
          // Server error verbatim (KnowledgeBaseList honesty rule) — the
          // FE never gates, it reports what the gate said.
          setServerError(typeof body.error === 'string' && body.error !== '' ? body.error : `HTTP ${resp.status}`);
          setError(true);
          setLoading(false);
          return;
        }
        const body = (await resp.json()) as {
          pending?: PendingRow[];
          total?: number;
          dispositions?: DispositionEntry[];
        };
        if (status === 'pending') {
          setPending(body.pending ?? []);
          setTotal(typeof body.total === 'number' ? body.total : (body.pending?.length ?? 0));
        } else {
          setHistory(body.dispositions ?? []);
        }
        setError(false);
        setServerError('');
        setLoading(false);
      })
      .catch(() => {
        setServerError('');
        setError(true);
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    void loadQueue('pending', 'all');
  }, [loadQueue]);

  function switchStatusTab(next: 'pending' | 'resolved') {
    setStatusTab(next);
    setActionError(null);
    void loadQueue(next, next === 'resolved' ? historyFilter : 'all');
  }

  function switchHistoryFilter(next: 'all' | Disposition) {
    setHistoryFilter(next);
    void loadQueue('resolved', next);
  }

  function isExpanded(chunkId: number): boolean {
    return expanded.includes(chunkId);
  }

  function toggleExpand(chunkId: number) {
    setExpanded((prev) => (prev.includes(chunkId) ? prev.filter((id) => id !== chunkId) : [...prev, chunkId]));
    // Chunk preview: fetch once per chunk on first expand (existing read
    // surface), keep loading / fallback states per chunk.
    const key = String(chunkId);
    if (previews[key]) return;
    setPreviews((prev) => ({ ...prev, [key]: { state: 'loading' } }));
    fetchChunkContent(key)
      .then((res) => {
        setPreviews((prev) => ({
          ...prev,
          [key]: res ? { state: 'ready', text: res.text } : { state: 'error' },
        }));
      })
      .catch(() => {
        setPreviews((prev) => ({ ...prev, [key]: { state: 'error' } }));
      });
  }

  async function dispose(chunkId: number, disposition: Disposition) {
    if (busyChunk !== null) return;
    setBusyChunk(chunkId);
    setActionError(null);
    const token = localStorage.getItem('sf_admin_token') ?? '';
    try {
      const resp = await fetch('/api/admin/feedback/dispositions', {
        method: 'POST',
        headers: { 'X-Admin-Token': token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ chunk_id: chunkId, disposition }),
      });
      if (resp.ok) {
        const body = (await resp.json().catch(() => ({}))) as { closed_count?: unknown };
        const closed = typeof body.closed_count === 'number' ? body.closed_count : 0;
        // D-29-08/09: one decision closes the chunk's WHOLE pending batch —
        // the toast names the count so the batch effect is never silent.
        toast(
          'success',
          `已处置 chunk #${chunkId}：${DISPOSITION_LABEL[disposition]}（${closed} 条待复核条目已关闭）`,
        );
        await loadQueue('pending', 'all'); // server truth, not local splicing
      } else {
        const body = (await resp.json().catch(() => ({}))) as { error?: unknown };
        const reason =
          typeof body.error === 'string' && body.error !== '' ? body.error : `HTTP ${resp.status}`;
        setActionError({ chunkId, message: reason });
        toast('error', `处置失败：${reason}`);
      }
    } catch (err) {
      const reason = err instanceof Error ? err.message : String(err);
      setActionError({ chunkId, message: reason });
      toast('error', `处置失败：${reason}`);
    } finally {
      setBusyChunk(null);
    }
  }

  const pendingRows = statusTab === 'pending' ? (pending ?? []) : [];
  const historyRows = statusTab === 'resolved' ? (history ?? []) : [];

  return (
    <div data-testid="review-view" className="flex flex-col gap-4">
      <PageHeader
        title="过时反馈复核"
        description="对「报告知识过时」反馈做终态处置并留痕审计（FBK-02）"
      />

      {/* FBK-05 — the structural honesty contract of this view, always visible. */}
      <div
        data-testid="review-trust-banner"
        className="rounded-md border border-border bg-surface px-3 py-2 text-xs text-text-muted"
      >
        {TRUST_BANNER}
      </div>

      {/* Filter row: 状态 segmented (default 待复核); in 处置历史 a second
          disposition filter appears. */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-1" role="group" aria-label="复核状态">
          <SegmentedButton
            active={statusTab === 'pending'}
            testId="review-status-pending"
            label="待复核"
            onClick={() => switchStatusTab('pending')}
          />
          <SegmentedButton
            active={statusTab === 'resolved'}
            testId="review-status-history"
            label="处置历史"
            onClick={() => switchStatusTab('resolved')}
          />
        </div>
        {statusTab === 'resolved' && (
          <div className="flex items-center gap-1" role="group" aria-label="处置过滤">
            <SegmentedButton
              active={historyFilter === 'all'}
              testId="review-history-filter-all"
              label="全部"
              onClick={() => switchHistoryFilter('all')}
            />
            {DISPOSITIONS.map((d) => (
              <SegmentedButton
                key={d}
                active={historyFilter === d}
                testId={`review-history-filter-${d}`}
                label={DISPOSITION_LABEL[d]}
                onClick={() => switchHistoryFilter(d)}
              />
            ))}
          </div>
        )}
        {statusTab === 'pending' && !loading && !error && (
          <span data-testid="review-total" className="text-xs text-text-muted">
            共 {total} 条
          </span>
        )}
      </div>

      {error && (
        <div className="flex flex-col gap-1">
          <button
            type="button"
            data-testid="review-error"
            onClick={() => void loadQueue(statusTab, statusTab === 'resolved' ? historyFilter : 'all')}
            className="self-start text-sm text-danger hover:underline"
          >
            复核队列加载失败（点击重试）
          </button>
          {serverError !== '' && (
            <div data-testid="review-error-detail" className="text-xs text-text-muted">
              {serverError}
            </div>
          )}
        </div>
      )}

      {loading && !error && <LoadingSkeleton testId="review-loading" />}

      {/* Pending queue — chunk-aggregated rows. */}
      {!loading && !error && statusTab === 'pending' && pendingRows.length === 0 && (
        <div data-testid="review-empty" className="flex flex-col items-center justify-center py-12 text-center">
          <p className="text-sm text-text">暂无待复核条目 — 没有新的「报告知识过时」反馈时，此列表为空</p>
        </div>
      )}

      {!loading && !error && statusTab === 'pending' && pendingRows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border text-xs text-text-muted">
                <th scope="col" className="pb-2 text-left font-semibold">Chunk ID</th>
                <th scope="col" className="pb-2 text-left font-semibold">知识库</th>
                <th scope="col" className="pb-2 text-left font-semibold">报告次数</th>
                <th scope="col" className="pb-2 text-left font-semibold">最近报告</th>
                <th scope="col" className="pb-2 text-left font-semibold">说明</th>
                <th scope="col" className="pb-2 text-right font-semibold">处置</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {pendingRows.map((row) => {
                const expandedRow = isExpanded(row.chunk_id);
                return (
                  <Fragment key={row.chunk_id}>
                    <tr key={row.chunk_id} data-testid={`review-row-${row.chunk_id}`} className="hover:bg-surface transition-colors">
                      <td className="py-3">
                        <code className="font-mono text-xs text-text">{row.chunk_id}</code>
                      </td>
                      <td className="py-3">
                        <code className="font-mono text-xs text-text-muted">{row.kb_slug}</code>
                      </td>
                      {/* D-29-08: the count IS the priority signal. */}
                      <td className="py-3 text-sm">
                        <span
                          data-testid={`review-count-${row.chunk_id}`}
                          className={cn(
                            'font-mono text-xs',
                            row.report_count >= 2 && 'text-status-needs-info font-semibold',
                          )}
                        >
                          {row.report_count}
                        </span>
                      </td>
                      <td className="py-3 text-xs text-text-muted" title={row.last_reported_at}>
                        {formatRelativeTime(row.last_reported_at)}
                      </td>
                      <td className="py-3">
                        <button
                          type="button"
                          data-testid={`review-expand-${row.chunk_id}`}
                          aria-expanded={expandedRow}
                          onClick={() => toggleExpand(row.chunk_id)}
                          className="rounded-md border border-border bg-surface-2 px-2 py-1 text-xs font-semibold text-text hover:bg-surface-3"
                        >
                          说明（{row.comments.length}）
                        </button>
                      </td>
                      <td className="py-3">
                        <div className="flex flex-wrap items-center justify-end gap-1">
                          {DISPOSITIONS.map((d) => (
                            <Button
                              key={d}
                              type="button"
                              variant="secondary"
                              size="sm"
                              data-testid={`review-dispose-${row.chunk_id}-${d}`}
                              aria-label={`将 chunk ${row.chunk_id} 处置为${DISPOSITION_LABEL[d]}`}
                              disabled={busyChunk !== null}
                              onClick={() => void dispose(row.chunk_id, d)}
                            >
                              {DISPOSITION_LABEL[d]}
                            </Button>
                          ))}
                        </div>
                        {actionError && actionError.chunkId === row.chunk_id && (
                          <div
                            data-testid={`review-action-error-${row.chunk_id}`}
                            role="alert"
                            className="mt-1 text-right text-xs text-status-non-compliant"
                          >
                            处置失败：{actionError.message}
                          </div>
                        )}
                      </td>
                    </tr>
                    {expandedRow && (
                      <tr key={`${row.chunk_id}-expanded`} className="bg-surface">
                        <td colSpan={6} className="px-3 py-3">
                          <div
                            data-testid={`review-comments-${row.chunk_id}`}
                            className="flex flex-col gap-1 text-xs text-text"
                          >
                            {row.comments.map((comment, idx) => (
                              <div key={idx} className="flex flex-col">
                                <span>{comment === '' ? '（未填写说明）' : comment}</span>
                              </div>
                            ))}
                            <span className="text-text-muted">最近报告：{formatRelativeTime(row.last_reported_at)}</span>
                            {previews[String(row.chunk_id)]?.state === 'loading' && (
                              <span
                                data-testid={`review-preview-loading-${row.chunk_id}`}
                                className="text-text-muted"
                              >
                                加载 chunk 内容…
                              </span>
                            )}
                            {previews[String(row.chunk_id)]?.state === 'ready' && (
                              <pre
                                data-testid={`review-preview-${row.chunk_id}`}
                                className="max-w-full whitespace-pre-wrap rounded-md border border-border bg-surface-2 px-3 py-2 font-mono text-xs text-text"
                              >
                                {(previews[String(row.chunk_id)] as { state: 'ready'; text: string }).text}
                              </pre>
                            )}
                            {previews[String(row.chunk_id)]?.state === 'error' && (
                              <span
                                data-testid={`review-preview-${row.chunk_id}`}
                                className="text-status-non-compliant"
                              >
                                无法加载 chunk 内容
                              </span>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Disposition history — INSERT-only audit mirror of past decisions. */}
      {!loading && !error && statusTab === 'resolved' && historyRows.length === 0 && (
        <div data-testid="review-history-empty" className="py-12 text-center text-sm text-text-muted">
          暂无处置记录
        </div>
      )}

      {!loading && !error && statusTab === 'resolved' && historyRows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border text-xs text-text-muted">
                <th scope="col" className="pb-2 text-left font-semibold">Chunk ID</th>
                <th scope="col" className="pb-2 text-left font-semibold">处置</th>
                <th scope="col" className="pb-2 text-left font-semibold">备注</th>
                <th scope="col" className="pb-2 text-left font-semibold">操作人</th>
                <th scope="col" className="pb-2 text-left font-semibold">时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {historyRows.map((row) => {
                const label = (DISPOSITION_LABEL as Record<string, string>)[row.disposition] ?? row.disposition;
                const tone =
                  (DISPOSITION_TONE as Record<string, 'green' | 'red' | 'gray'>)[row.disposition] ?? 'gray';
                return (
                  <tr key={row.id} data-testid={`review-history-row-${row.id}`} className="hover:bg-surface transition-colors">
                    <td className="py-3">
                      <code className="font-mono text-xs text-text">{row.chunk_id}</code>
                    </td>
                    <td className="py-3">
                      <StatusBadge
                        testId={`review-badge-${row.id}`}
                        tone={tone}
                        label={label}
                        status={row.disposition}
                      />
                    </td>
                    <td className="py-3 text-xs text-text-muted">{row.note === '' ? '—' : row.note}</td>
                    <td className="py-3">
                      <code className="font-mono text-xs text-text-muted">{row.actor}</code>
                    </td>
                    <td className="py-3 text-xs text-text-muted" title={row.ts}>
                      {formatRelativeTime(row.ts)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
