// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 55-01 (ING-05) -- pins the derivation table. Every surface of
// this phase (timeline row, matrix cell, inspect panel, jobs table)
// reads these functions, so this file is the phase's contract.
import { describe, expect, it } from 'vitest';
import {
  STEP_LABEL,
  STEP_SEQUENCE,
  DETERMINISTIC_PREFIX,
  JOB_BADGE,
  countDoneSteps,
  deriveStepState,
  formatDuration,
  isDeterministic,
  jobBadge,
  orderSteps,
  type IngestStep,
} from './types';

function step(over: Partial<IngestStep>): IngestStep {
  return { step: 'parse', status: 'pending', attempt: 0, ...over };
}

describe('STEP_LABEL / STEP_SEQUENCE', () => {
  it('pins the eight zh labels verbatim (the four ecl_* labels are 39-UI-SPEC wording, D-55-01)', () => {
    expect(STEP_SEQUENCE).toHaveLength(8);
    expect(STEP_LABEL.parse).toBe('解析');
    expect(STEP_LABEL.chunk).toBe('分块');
    expect(STEP_LABEL.embed).toBe('向量化');
    expect(STEP_LABEL.vector_insert).toBe('入向量库');
    expect(STEP_LABEL.ecl_merge).toBe('切分合并');
    expect(STEP_LABEL.ecl_extract).toBe('实体抽取');
    expect(STEP_LABEL.ecl_kg_load).toBe('KG 写入');
    expect(STEP_LABEL.ecl_milvus_load).toBe('Milvus 索引');
    // Every sequence key must have a label -- a gap would render a raw key.
    for (const key of STEP_SEQUENCE) {
      expect(STEP_LABEL[key]).toBeTruthy();
    }
  });
});

describe('deriveStepState (two-layer mapping)', () => {
  it('maps all six wire statuses to the derived vocabulary', () => {
    expect(deriveStepState(step({ status: 'pending' }))).toEqual({
      derived: 'queued', label: '等待', tone: 'gray', pulse: false,
    });
    expect(deriveStepState(step({ status: 'running' }))).toEqual({
      derived: 'running', label: '进行中', tone: 'amber', pulse: true,
    });
    expect(deriveStepState(step({ status: 'done' }))).toEqual({
      derived: 'done', label: '完成', tone: 'green', pulse: false,
    });
    expect(deriveStepState(step({ status: 'failed' }))).toEqual({
      derived: 'error', label: '失败', tone: 'red', pulse: false,
    });
    expect(deriveStepState(step({ status: 'skipped' }))).toEqual({
      derived: 'skipped', label: '已跳过', tone: 'gray', pulse: false,
    });
    expect(deriveStepState(step({ status: 'stale' }))).toEqual({
      derived: 'stale', label: '待重跑', tone: 'amber', pulse: false,
    });
  });

  it('overrides StatusBadge DEFAULT_MAP amber on skipped (skipped is neutral, not attention)', () => {
    expect(deriveStepState(step({ status: 'skipped' })).tone).toBe('gray');
  });

  it('pulses ONLY on running -- stale is amber but never pulses', () => {
    const pulsing = ['pending', 'running', 'done', 'failed', 'skipped', 'stale']
      .map((s) => deriveStepState(step({ status: s })))
      .filter((d) => d.pulse);
    expect(pulsing).toHaveLength(1);
    expect(pulsing[0].derived).toBe('running');
    expect(deriveStepState(step({ status: 'stale' })).pulse).toBe(false);
  });

  it('attempt>=2 is not a separate state (jitter suppression)', () => {
    expect(deriveStepState(step({ status: 'running', attempt: 5 })).derived).toBe('running');
    expect(deriveStepState(step({ status: 'done', attempt: 3 })).derived).toBe('done');
  });

  it('renders an UNKNOWN status verbatim instead of blanking it', () => {
    const d = deriveStepState(step({ status: 'mystery_state' }));
    expect(d.derived).toBe('mystery_state');
    expect(d.label).toBe('mystery_state');
    expect(d.tone).toBe('gray');
    expect(d.pulse).toBe(false);
  });
});

describe('isDeterministic', () => {
  it('matches only the 54-03 classification prefix', () => {
    expect(DETERMINISTIC_PREFIX).toBe('[deterministic]');
    expect(isDeterministic(step({ status: 'failed', error: '[deterministic] bad payload' }))).toBe(true);
    expect(isDeterministic(step({ status: 'failed', error: '[transient] 429 too many requests' }))).toBe(false);
    expect(isDeterministic(step({ status: 'failed', error: 'plain failure' }))).toBe(false);
    expect(isDeterministic(step({ status: 'pending' }))).toBe(false);
  });
});

describe('JOB_BADGE', () => {
  it('uses the WIRE vocabulary (done, not the spec draft completed)', () => {
    expect(JOB_BADGE.done.label).toBe('已完成');
    expect(JOB_BADGE.queued.label).toBe('等待');
    expect(JOB_BADGE.running.label).toBe('运行中');
    expect(JOB_BADGE.partial.label).toBe('部分成功');
    expect(JOB_BADGE.failed.label).toBe('失败');
    // The retired draft key must not exist.
    expect(Object.prototype.hasOwnProperty.call(JOB_BADGE, 'completed')).toBe(false);
  });

  it('pulses only for running and falls back to the raw key for unknown job statuses', () => {
    expect(jobBadge('running').pulse).toBe(true);
    expect(jobBadge('partial').pulse).toBe(false);
    const unknown = jobBadge('weird_job_state');
    expect(unknown.label).toBe('weird_job_state');
    expect(unknown.tone).toBe('gray');
  });
});

describe('formatDuration', () => {
  it('renders a one-decimal second string from two parseable stamps', () => {
    expect(
      formatDuration('2026-11-24T10:00:00.000Z', '2026-11-24T10:00:03.200Z'),
    ).toBe('3.2s');
  });

  it('renders — for a running step (missing completed_at), missing or corrupt stamps', () => {
    expect(formatDuration('2026-11-24T10:00:00.000Z', undefined)).toBe('—');
    expect(formatDuration(undefined, '2026-11-24T10:00:03.200Z')).toBe('—');
    expect(formatDuration()).toBe('—');
    expect(formatDuration('not-a-date', '2026-11-24T10:00:03.200Z')).toBe('—');
    // Negative (corrupt ordering) is also — rather than a nonsense '-1.0s'.
    expect(formatDuration('2026-11-24T10:00:05.000Z', '2026-11-24T10:00:00.000Z')).toBe('—');
  });
});

describe('orderSteps / countDoneSteps', () => {
  it('anchors the payload on STEP_SEQUENCE and puts unknown keys last in payload order', () => {
    const shuffled: IngestStep[] = [
      { step: 'ecl_milvus_load', status: 'pending', attempt: 0 },
      { step: 'parse', status: 'done', attempt: 1 },
      { step: 'mystery_step', status: 'done', attempt: 1 },
      { step: 'chunk', status: 'done', attempt: 1 },
      { step: 'another_mystery', status: 'pending', attempt: 0 },
    ];
    const ordered = orderSteps(shuffled).map((s) => s.step);
    expect(ordered).toEqual([
      'parse', 'chunk', 'ecl_milvus_load', 'mystery_step', 'another_mystery',
    ]);
  });

  it('reorders only -- it never inserts a missing step', () => {
    expect(orderSteps([{ step: 'parse', status: 'done', attempt: 1 }])).toHaveLength(1);
  });

  it('counts derived-done steps only', () => {
    expect(
      countDoneSteps([
        { step: 'parse', status: 'done', attempt: 1 },
        { step: 'chunk', status: 'done', attempt: 1 },
        { step: 'embed', status: 'skipped', attempt: 0 },
        { step: 'vector_insert', status: 'stale', attempt: 1 },
      ]),
    ).toBe(2);
  });
});
