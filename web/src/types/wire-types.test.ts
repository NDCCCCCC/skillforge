// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 1 (D15 lockstep paired commit with 12-02 BE) — wire-type
// tests for `data-plan-trace` variant + PlanTracePayload / PlanTodo shapes.
//
// The D15 compile-time drift alarm (AssertAssignable) at the bottom of
// wire-types.ts asserts every SkillForgeChunk variant is assignable to
// UIMessageChunk. This file adds two runtime/narrow tests that PIN the
// Plan 12-06 extension so a future refactor cannot silently remove the
// variant without breaking a test.
//
// The vitest suite already has the FILE-LEVEL D15 alarm test at
// src/__tests__/wire-contract.test.ts (Pitfalls 1-3) — this file lives
// next to wire-types.ts as the round-1 addition-side narrowing.

import type { UIMessageChunk } from 'ai';
import { describe, expect, it } from 'vitest';
import type {
  PlanTodo,
  PlanTracePayload,
  SkillForgeChunk,
} from './wire-types';

describe('wire-types (Plan 12-06 Task 1)', () => {
  // D15 lockstep guard — SkillForgeChunk union now includes
  // data-plan-trace. AssertAssignable is enforced at compile time; this
  // test nails the runtime narrowing by constructing a chunk literal
  // and asserting it's assignable to UIMessageChunk.
  it('TestWireTypeDataPlanTraceAdded', () => {
    const chunk: SkillForgeChunk = {
      type: 'data-plan-trace',
      data: {
        runId: 'run-1',
        todos: [
          { id: 't-1', content: 'first', status: 'pending' },
        ],
        ts: Date.now(),
      },
    };
    // The compile-time drift alarm — this assignment must typecheck.
    const sdkChunk: UIMessageChunk = chunk;
    expect(sdkChunk.type).toBe('data-plan-trace');
  });

  // PlanTracePayload shape test — runId + todos + ts are present;
  // PlanTodo has id + content + status; status enum is the three-value
  // union mirrored from the BE jsonschema tag enum.
  it('TestPlanTracePayloadTypeShape', () => {
    const todo: PlanTodo = {
      id: 'todo-1',
      content: 'plan something',
      activeForm: 'planning something',
      status: 'in_progress',
    };
    const payload: PlanTracePayload = {
      runId: 'run-12',
      todos: [todo],
      ts: 1700000000000,
    };
    expect(payload.runId).toBe('run-12');
    expect(payload.todos).toHaveLength(1);
    expect(payload.ts).toBe(1700000000000);
    expect(payload.todos[0].id).toBe('todo-1');
    expect(payload.todos[0].status).toBe('in_progress');
  });
});
