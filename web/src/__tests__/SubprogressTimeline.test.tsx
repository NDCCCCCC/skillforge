// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SubprogressTimeline tests (2026-11-11).
//
// 用户反馈的原始问题是"没有一步一步来的感觉"：后端连发的步骤会同时堆成
// 一坨、所有点长得一样、看不出当前跑到哪。这里锁住三件事：
//   1. live 时逐条入场（间隔 STEP_STAGGER_MS），而不是一次全出现；
//   2. 当前步是 active（旋转图标 + "进行中"），之前的步是 done（勾）；
//   3. 回复结束后整体落定，"已完成 N 步"，不再有进行中的行。
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import SubprogressTimeline, {
  STEP_STAGGER_MS,
  type SubprogressEntry,
} from '../components/chat/parts/SubprogressTimeline';

const ENTRIES: SubprogressEntry[] = [
  { key: 's1', data: { step: 'retrieve', detail: '检索 [SQL 取数操作手册] 知识层' } },
  { key: 's2', data: { step: 'found_candidates', count: 5 } },
  { key: 's3', data: { step: 'reading_guides', detail: '读取取数手册：025_学平险学校与类别.md' } },
  { key: 's4', data: { step: 'generating', detail: '正在基于这些笔记生成适配 SQL…' } },
  { key: 's5', data: { step: 'generate' } },
];

describe('SubprogressTimeline', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('live → 步骤逐条入场，而不是一次性全出现 (2026-11-11)', () => {
    render(<SubprogressTimeline entries={ENTRIES} live />);

    // 首帧只有第一条（正在进行）。
    expect(screen.getAllByTestId('proc-step')).toHaveLength(1);
    expect(screen.getByTestId('proc-status')).toHaveTextContent('进行中');

    act(() => {
      vi.advanceTimersByTime(STEP_STAGGER_MS);
    });
    expect(screen.getAllByTestId('proc-step')).toHaveLength(2);

    act(() => {
      vi.advanceTimersByTime(STEP_STAGGER_MS);
    });
    expect(screen.getAllByTestId('proc-step')).toHaveLength(3);
  });

  it('当前步是 active（进行中），之前的步是 done（勾）', () => {
    render(<SubprogressTimeline entries={ENTRIES.slice(0, 2)} live />);
    act(() => {
      vi.advanceTimersByTime(STEP_STAGGER_MS);
    });

    const rows = screen.getAllByTestId('proc-step');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveAttribute('data-state', 'done');
    expect(rows[1]).toHaveAttribute('data-state', 'active');
    expect(rows[1]).toHaveTextContent('进行中');
  });

  it('进行中的那一步显示跳动的计时（后端发完步骤后有几十秒静默）', () => {
    render(<SubprogressTimeline entries={ENTRIES.slice(0, 1)} live />);
    expect(screen.getByTestId('proc-step')).toHaveTextContent('进行中');

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByTestId('proc-step')).toHaveTextContent(/进行中 · \d/);
  });

  it('回复结束后整体落定：全部 done + "已完成 N 步"', () => {
    const { rerender } = render(<SubprogressTimeline entries={ENTRIES} live />);
    rerender(<SubprogressTimeline entries={ENTRIES} live={false} />);

    const rows = screen.getAllByTestId('proc-step');
    expect(rows).toHaveLength(ENTRIES.length);
    for (const row of rows) {
      expect(row).toHaveAttribute('data-state', 'done');
    }
    expect(screen.getByTestId('proc-status')).toHaveTextContent(`已完成 ${ENTRIES.length} 步`);
    // 完成态不再出现"进行中"。
    expect(screen.queryByText('进行中')).toBeNull();
  });

  it('历史消息（live=false）首帧就是完整过程，不做入场节流', () => {
    render(<SubprogressTimeline entries={ENTRIES} live={false} />);
    expect(screen.getAllByTestId('proc-step')).toHaveLength(ENTRIES.length);
  });

  it('可以收起 / 展开（收起后步骤行不再渲染）', () => {
    render(<SubprogressTimeline entries={ENTRIES} live={false} />);
    expect(screen.getAllByTestId('proc-step')).toHaveLength(ENTRIES.length);
    act(() => {
      screen.getByRole('button', { name: /收起/ }).click();
    });
    expect(screen.queryAllByTestId('proc-step')).toHaveLength(0);
    act(() => {
      screen.getByRole('button', { name: /展开/ }).click();
    });
    expect(screen.getAllByTestId('proc-step')).toHaveLength(ENTRIES.length);
  });
});
