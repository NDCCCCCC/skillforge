// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-07 Task 2 — PlanReplayView (PLAN-05 SC-5 规划轨迹回放视图).
//
// Renders the historical plan evolution for one run. Fetches
// GET /api/run/:id/plan (BE 12-04 plan_api.go) on mount + runId change;
// renders the result as a vertical timeline with status indicators +
// speed-controlled animation (1x / 2x / 4x). The replay timeline walks
// the chronological todo snapshot list — each row's status icon +
// label animates in at the user-selected speed.
//
// D14 invariant: NEVER reads from useChat.messages. The plan slice is
// the authoritative state for live traces; the BE replay endpoint is
// the authoritative source for historical replays (Pitfall 6 buffering
// doesn't apply — we fetch from server, not from the chat stream).
//
// Visual:
//   - Vertical timeline (border-l-2) with one row per todo
//   - Status icons: CircleDashed (gray pending) / Loader2 (blue
//     in_progress, animate-spin) / CheckCircle2 (green completed)
//   - Speed control: dropdown 1x / 2x / 4x
//   - Play / Pause toggle button
//   - Empty state: "暂无规划轨迹" when the BE returns 0 todos
//   - Error state: alert banner + 重试 button
//
// The component is mounted by DetailPanel when activeTab === 'plan-replay'.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { JSX } from 'react';
import {
  CheckCircle2,
  CircleDashed,
  ClipboardList,
  Loader2,
  Pause,
  Play,
  RotateCw,
} from 'lucide-react';
import type { PlanTodo } from '../types/wire-types';
import { cn } from '../lib/cn';

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

/** Speed multiplier → ms-per-step animation duration. */
const BASE_MS_PER_STEP = 600;
const SPEED_OPTIONS: Array<{ label: string; value: number; ms: number }> = [
  { label: '1x', value: 1, ms: BASE_MS_PER_STEP },
  { label: '2x', value: 2, ms: BASE_MS_PER_STEP / 2 },
  { label: '4x', value: 4, ms: BASE_MS_PER_STEP / 4 },
];

/** The BE GET /api/run/:id/plan response shape — mirrors
 *  internal/server/plan_api.go planGetResponse. */
interface PlanGetResponse {
  runId: string;
  todos: PlanTodo[];
  /** Epoch seconds — translated to ms below. */
  generatedAt: number;
  count: number;
}

/** Pure status renderer — exported so tests can pin the contract. */
export function renderReplayStatusIcon(status: PlanTodo['status']): JSX.Element {
  switch (status) {
    case 'pending':
      return (
        <CircleDashed
          data-testid="replay-status-icon-pending"
          className="h-4 w-4 text-gray-400"
        />
      );
    case 'in_progress':
      return (
        <Loader2
          data-testid="replay-status-icon-in_progress"
          className="h-4 w-4 animate-spin text-blue-500"
        />
      );
    case 'completed':
      return (
        <CheckCircle2
          data-testid="replay-status-icon-completed"
          className="h-4 w-4 text-green-500"
        />
      );
    default:
      return <CircleDashed className="h-4 w-4 text-gray-400" />;
  }
}

/** Fetch GET /api/run/:id/plan. Returns null on error / non-2xx. */
async function fetchPlanReplay(
  runId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<PlanGetResponse | null> {
  try {
    const resp = await fetchImpl(
      `/api/run/${encodeURIComponent(runId)}/plan`,
      { headers: { Accept: 'application/json' } },
    );
    if (!resp.ok) return null;
    const body = (await resp.json()) as Partial<PlanGetResponse>;
    if (!body || !Array.isArray(body.todos)) return null;
    return {
      runId: typeof body.runId === 'string' ? body.runId : runId,
      todos: body.todos as PlanTodo[],
      generatedAt: typeof body.generatedAt === 'number' ? body.generatedAt : Date.now() / 1000,
      count: typeof body.count === 'number' ? body.count : body.todos.length,
    };
  } catch {
    return null;
  }
}

// ----------------------------------------------------------------------------
// PlanReplayView
// ----------------------------------------------------------------------------

export interface PlanReplayViewProps {
  runId: string;
  /** Test-injected fetch override (defaults to globalThis.fetch). */
  fetchImpl?: typeof globalThis.fetch;
}

export function PlanReplayView({
  runId,
  fetchImpl,
}: PlanReplayViewProps): JSX.Element {
  const [todos, setTodos] = useState<PlanTodo[]>([]);
  const [generatedAt, setGeneratedAt] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [speed, setSpeed] = useState<number>(1);
  const [playing, setPlaying] = useState(false);

  // Refs let the timeout-callback read the latest values without
  // re-creating the effect (which would restart the animation).
  const indexRef = useRef(0);
  const playingRef = useRef(false);
  playingRef.current = playing;
  const todosRef = useRef<PlanTodo[]>([]);
  todosRef.current = todos;

  // Fetch on mount + runId change (re-keyed by reloadKey for retry).
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    setPlaying(false);
    indexRef.current = 0;
    void (async () => {
      const result = await fetchPlanReplay(runId, fetchImpl);
      if (cancelled) return;
      if (result === null) {
        setError(true);
        setTodos([]);
      } else {
        setTodos(result.todos);
        setGeneratedAt(result.generatedAt);
      }
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [runId, reloadKey, fetchImpl]);

  const currentSpeed = useMemo(
    () => SPEED_OPTIONS.find((s) => s.value === speed) ?? SPEED_OPTIONS[0],
    [speed],
  );

  // Animation tick — every speed.ms, advance the visible-step count by
  // one until we reach the end. Cleared when paused or on unmount.
  useEffect(() => {
    if (!playing) return;
    const handle = setTimeout(() => {
      const total = todosRef.current.length;
      if (total === 0) {
        setPlaying(false);
        return;
      }
      if (indexRef.current >= total - 1) {
        setPlaying(false);
        return;
      }
      indexRef.current += 1;
      // Force a re-render so the visible step counter updates. We don't
      // mutate todos (the BE snapshot is canonical); we tick a counter.
      setTick((n) => n + 1);
    }, currentSpeed.ms);
    return () => clearTimeout(handle);
  }, [playing, currentSpeed.ms]);

  // Independent tick state — bumped from the animation timer above so
  // the visible steps re-render. (We avoid mutating todos to preserve
  // the BE canonical snapshot.)
  const [tick, setTick] = useState(0);

  // Visible window: show todos[0..visibleCount] — the rest are rendered
  // faded out via opacity-30 so the user can still see the full chain.
  const visibleCount = Math.min(todos.length, indexRef.current + 1);

  const handleRetry = useCallback(() => {
    setReloadKey((n) => n + 1);
  }, []);

  // Empty / loading / error states.
  if (loading) {
    return (
      <div
        data-testid="plan-replay-loading"
        className="flex items-center gap-2 px-4 py-3 text-xs text-[var(--color-text-muted)]"
      >
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        加载规划轨迹…
      </div>
    );
  }

  if (error) {
    return (
      <div
        data-testid="plan-replay-error"
        role="alert"
        className="mx-4 my-3 rounded-md border border-status-error/40 bg-status-error/10 px-3 py-2 text-xs text-status-error"
      >
        <p>规划轨迹加载失败，请检查网络后重试。</p>
        <button
          type="button"
          data-testid="plan-replay-retry"
          onClick={handleRetry}
          className="mt-2 inline-flex items-center gap-1 rounded bg-status-error/20 px-2 py-1 text-xs font-semibold hover:bg-status-error/30"
        >
          <RotateCw className="h-3 w-3" />
          重试
        </button>
      </div>
    );
  }

  if (todos.length === 0) {
    return (
      <div
        data-testid="plan-replay-empty"
        className="px-4 py-6 text-center text-xs text-[var(--color-text-muted)]"
      >
        暂无规划轨迹
      </div>
    );
  }

  return (
    <div
      data-testid="plan-replay"
      data-run-id={runId}
      data-tick={tick}
      className="flex flex-col gap-3 px-4 py-3"
      role="region"
      aria-label="规划轨迹回放"
    >
      {/* Header strip — generatedAt + speed + play/pause */}
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--color-text-muted)]">
        <div className="flex items-center gap-2">
          <ClipboardList className="h-3.5 w-3.5" />
          <span data-testid="plan-replay-header">规划轨迹</span>
          <span className="font-mono text-[10px]">
            生成于 {formatTs(generatedAt)}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            data-testid="plan-replay-play-pause"
            data-playing={playing}
            aria-label={playing ? '暂停' : '播放'}
            onClick={() => setPlaying((p) => !p)}
            disabled={indexRef.current >= todos.length - 1 && !playing}
            className={cn(
              'inline-flex h-7 w-7 items-center justify-center rounded-md border border-border bg-surface text-text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:opacity-50',
            )}
          >
            {playing ? (
              <Pause className="h-3.5 w-3.5" />
            ) : (
              <Play className="h-3.5 w-3.5" />
            )}
          </button>
          <label className="flex items-center gap-1 text-xs">
            <span>速度</span>
            <select
              data-testid="plan-replay-speed"
              value={speed}
              onChange={(e) => setSpeed(Number(e.target.value))}
              className="rounded border border-border bg-surface px-1 py-0.5 text-xs text-text focus:outline-none"
            >
              {SPEED_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      {/* Timeline */}
      <ol
        data-testid="plan-replay-list"
        className="space-y-2 border-l-2 border-border pl-4"
      >
        {todos.map((todo, i) => {
          const visible = i < visibleCount;
          return (
            <li
              key={todo.id}
              data-testid="plan-replay-row"
              data-todo-id={todo.id}
              data-status={todo.status}
              data-step-index={i}
              className={cn(
                'flex items-start gap-3 py-1.5 transition-opacity duration-300',
                visible ? 'opacity-100' : 'opacity-30',
              )}
            >
              <span className="mt-0.5 flex-none">{renderReplayStatusIcon(todo.status)}</span>
              <div className="min-w-0 flex-1">
                <div className="font-medium text-sm text-text">
                  {todo.activeForm && todo.status === 'in_progress'
                    ? todo.activeForm
                    : todo.content}
                </div>
                <div className="mt-0.5 text-xs text-text-muted">
                  Step {i + 1} · Status: {todo.status}
                </div>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function formatTs(epochSeconds: number): string {
  if (!epochSeconds || epochSeconds <= 0) return '未知';
  const d = new Date(epochSeconds * 1000);
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export default PlanReplayView;