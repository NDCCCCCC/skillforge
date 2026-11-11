// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Shared copy renderer for the data-subprogress step stream.
//
// 260819-fix: this used to live inside DataSubprogressPart. The timeline
// (SubprogressTimeline) renders the exact same rows and must not drift
// from the single-step renderer the test suite pins, so the copy lives
// in one place and both import it.
//
// detail 优先：后端把「实际读了哪几篇」「正在生成什么」写在这里，比下面
// 这些固定文案有信息量得多（2026-11-10 用户要求看到真实过程）。
import type { SubprogressStep } from '../../../types/verdict';

export function renderStepCopy(step: SubprogressStep, tierLabel: string | null): string {
  if (step.detail) return step.detail;
  switch (step.step) {
    case 'retrieve':
      return tierLabel ? `检索 ${tierLabel} 知识层` : '检索知识层';
    case 'found_candidates': {
      // quick task 261027-ebw: Count is min(topK, candidate set) — when the
      // retriever reported a larger pre-truncation set, say so, or "找到 5
      // 条候选" reads as "exactly 5 relevant hits" when it is a top-5 cut.
      const { count, total } = step;
      if (total && total > (count ?? 0)) {
        return `找到 ${count ?? 0} 条候选（候选集共 ${total} 条，已按相关度取前 ${count ?? 0} 条）`;
      }
      return `找到 ${count ?? 0} 条候选`;
    }
    case 'rerank':
      return '重排序';
    case 'self_verify':
      return '自校验';
    case 'generate':
      return '生成结论';
    // 2026-11-10 真实过程事件：只有 detail 才有意义（后端总是带 detail）。
    // detail 缺失时给一个中性文案，绝不渲染成空白行。
    case 'reading_guides':
      return '读取取数手册';
    case 'generating':
      return '正在生成结论…';
    default:
      return '';
  }
}
