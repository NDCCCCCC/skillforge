// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 3 — live planning-trace renderer (PLAN-02 SC-2).
//
// Reads the plan slice (the AUTHORITATIVE state) via
// useAppStore(state => state.plan.planByRunId[runId]) and renders an
// ordered todo list with status indicators + animation on transitions.
// D14 invariant: does NOT read from useChat.messages for plan state —
// the message stream is consulted ONLY for the synthetic write_todos
// tool-call fallback path (Pitfall 6 mitigation).
//
// Visual: a compact card that mirrors the chat column's "step" feel
// (subprogress-step pattern from DataSubprogressPart.tsx). Status
// indicators:
//   - pending    → CircleDashed (gray)
//   - in_progress → Loader2 (blue, animate-spin)
//   - completed  → CheckCircle2 (green)
//
// When collapsed=true AND status==='complete', the card collapses to
// "计划已完成：N 步" with a chevron to expand. Hidden (returns null)
// when the run has no plan (the SSE stream hasn't emitted anything).

import { useState } from 'react';
import type { JSX } from 'react';
import {
  CheckCircle2,
  ChevronDown,
  CircleDashed,
  ClipboardList,
  Loader2,
} from 'lucide-react';
import type { PlanTodo } from '../types/wire-types';
import { useAppStore } from '../store/app-store';
import type { PlanState } from '../store/app-store';

export interface PlanTracePartProps {
  /** Run id to scope the trace render (D14: per-run key). */
  runId: string;
  /** Collapse the card to a one-line summary after the run completes. */
  collapsed?: boolean;
  /** Optional list of synthetic write_todos tool-call parts (Pitfall 6
   *  fallback — fed through useChat messages when data-plan-trace is
   *  buffered by the AI SDK v5 parser). */
  syntheticParts?: Array<{
    toolName: string;
    state: string;
    input?: unknown;
    output?: unknown;
  }>;
}

/** Pull todos from a write_todos tool input/output (synthetic fallback).
 *  Returns null if the synthetic parts don't carry a usable payload. */
function extractSyntheticTodos(
  parts: NonNullable<PlanTracePartProps['syntheticParts']>,
): PlanTodo[] | null {
  for (const part of parts) {
    if (part.toolName !== 'write_todos') continue;
    const output = part.output as { todos?: PlanTodo[] } | undefined;
    if (output && Array.isArray(output.todos) && output.todos.length > 0) {
      return output.todos;
    }
    const input = part.input as { todos?: PlanTodo[] } | undefined;
    if (input && Array.isArray(input.todos) && input.todos.length > 0) {
      return input.todos;
    }
  }
  return null;
}

export function PlanTracePart({
  runId,
  collapsed = false,
  syntheticParts,
}: PlanTracePartProps): JSX.Element | null {
  // D14 invariant — authoritative state is the plan slice (NOT messages).
  const planState = useAppStore((s) => s.plan.planByRunId[runId]);
  const [expanded, setExpanded] = useState(false);

  // Pitfall 6 fallback — when data-plan-trace is buffered by AI SDK v5,
  // we may still see write_todos synthetic tool-call parts in messages.
  // Only consult synthetic path when the slice has no live data yet.
  const todos =
    (planState && planState.todos.length > 0
      ? planState.todos
      : (syntheticParts ? extractSyntheticTodos(syntheticParts) : null)) ?? [];

  // Empty render gate: no plan for this run yet AND no synthetic fallback.
  if (todos.length === 0) {
    return null;
  }

  const status = planState?.status ?? 'streaming';
  const completedCount = todos.filter((t) => t.status === 'completed').length;

  // Collapsed-after-complete summary — only when status==='complete' AND
  // collapsed prop is set explicitly.
  if (collapsed && status === 'complete') {
    return (
      <div
        data-testid="plan-trace-part-collapsed"
        data-status={status}
        className="my-2 rounded-md border border-[var(--color-border-muted)] bg-[var(--color-surface-2)] px-3 py-1.5 text-xs"
      >
        <button
          type="button"
          data-testid="plan-trace-toggle"
          onClick={() => setExpanded((b) => !b)}
          aria-expanded={expanded}
          className="flex w-full items-center justify-between gap-2"
        >
          <span className="flex items-center gap-2 text-[var(--color-text-muted)]">
            <ClipboardList className="h-3.5 w-3.5" />
            <span>计划已完成：{completedCount} 步</span>
          </span>
          <ChevronDown
            className="h-3 w-3"
            style={{ transform: expanded ? undefined : 'rotate(-90deg)' }}
          />
        </button>
        {expanded && (
          <ul
            data-testid="plan-trace-list"
            className="mt-2 space-y-1 text-sm"
          >
            {todos.map((t) => renderTodoRow(t))}
          </ul>
        )}
      </div>
    );
  }

  return (
    <div
      data-testid="plan-trace-part"
      data-status={status}
      data-run-id={runId}
      className="my-2 rounded-md border border-[var(--color-border-muted)] bg-[var(--color-surface)] px-3 py-2 shadow-sm transition-all duration-300"
      role="region"
      aria-label="计划进度"
    >
      <div className="mb-1 flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
        <ClipboardList className="h-3.5 w-3.5" />
        <span data-testid="plan-trace-header">执行计划</span>
        <span data-testid="plan-trace-progress">
          ({completedCount}/{todos.length})
        </span>
      </div>
      <ul
        data-testid="plan-trace-list"
        className="space-y-1 text-sm"
      >
        {todos.map((t) => renderTodoRow(t))}
      </ul>
    </div>
  );
}

/** Render one todo row — extracted so collapsed + full views share it. */
function renderTodoRow(todo: PlanTodo) {
  return (
    <li
      key={todo.id}
      data-testid="plan-todo-row"
      data-todo-id={todo.id}
      data-status={todo.status}
      className="flex items-center gap-2 transition-all duration-300"
    >
      {renderStatusIcon(todo.status)}
      <span
        className={
          todo.status === 'completed'
            ? 'text-[var(--color-text-muted)] line-through'
            : 'text-[var(--color-text)]'
        }
      >
        {todo.activeForm && todo.status === 'in_progress'
          ? todo.activeForm
          : todo.content}
      </span>
    </li>
  );
}

/** Pure renderer for the status indicator icon. Exported so tests can
 *  pin the visual state contract. */
export function renderStatusIcon(status: PlanTodo['status']): JSX.Element {
  switch (status) {
    case 'pending':
      return (
        <CircleDashed
          data-testid="status-icon-pending"
          className="h-4 w-4 text-gray-400"
        />
      );
    case 'in_progress':
      return (
        <Loader2
          data-testid="status-icon-in_progress"
          className="h-4 w-4 animate-spin text-blue-500"
        />
      );
    case 'completed':
      return (
        <CheckCircle2
          data-testid="status-icon-completed"
          className="h-4 w-4 text-green-500"
        />
      );
    default:
      return <CircleDashed className="h-4 w-4 text-gray-400" />;
  }
}

export default PlanTracePart;
export type { PlanState };
