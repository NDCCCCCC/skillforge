// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StatusBadge — unified admin status pill (admin UI redesign 2610).
//
// One mapping table for every admin view. Replaces the per-component
// inline `map: Record<string, {label, className}>` copies and the raw
// hex classNames with the theme status tokens:
//   green  — terminal success (completed / success / active / approved)
//   amber  — in-flight / attention (running / pending / drifted / review)
//   red    — failure (failed / error / rejected)
//   gray   — neutral / idle / unknown
//
// `tone` overrides the derived color; `label` overrides the derived text.

import { cn } from '../../../lib/cn';

export type StatusTone = 'green' | 'amber' | 'red' | 'gray' | 'indigo';

const TONE_CLASS: Record<StatusTone, string> = {
  green: 'bg-status-compliant/15 text-status-compliant',
  amber: 'bg-status-needs-info/15 text-status-needs-info',
  red: 'bg-status-non-compliant/15 text-status-non-compliant',
  gray: 'bg-status-unanswered/15 text-status-unanswered',
  indigo: 'bg-primary-dim text-primary',
};

const DEFAULT_MAP: Record<string, { label: string; tone: StatusTone }> = {
  // terminal success
  completed: { label: '已完成', tone: 'green' },
  success: { label: '成功', tone: 'green' },
  succeeded: { label: '成功', tone: 'green' },
  active: { label: '启用', tone: 'green' },
  approved: { label: '已通过', tone: 'green' },
  compliant: { label: '合规', tone: 'green' },
  done: { label: '完成', tone: 'green' },
  ok: { label: '正常', tone: 'green' },
  healthy: { label: '健康', tone: 'green' },
  merged: { label: '已合并', tone: 'green' },
  imported: { label: '已导入', tone: 'green' },
  // in-flight / attention
  running: { label: '运行中', tone: 'amber' },
  pending: { label: '等待中', tone: 'amber' },
  processing: { label: '处理中', tone: 'amber' },
  in_progress: { label: '进行中', tone: 'amber' },
  needs_input: { label: '待补充', tone: 'amber' },
  review: { label: '待复核', tone: 'amber' },
  drifted: { label: 'drift', tone: 'amber' },
  degraded: { label: '降级', tone: 'amber' },
  skipped: { label: '已跳过', tone: 'amber' },
  partial: { label: '部分成功', tone: 'amber' },
  // failure
  failed: { label: '失败', tone: 'red' },
  error: { label: '错误', tone: 'red' },
  rejected: { label: '已驳回', tone: 'red' },
  breach: { label: '超限', tone: 'red' },
  dead_letter: { label: '死信', tone: 'red' },
  // neutral
  idle: { label: '空闲', tone: 'gray' },
  inactive: { label: '停用', tone: 'gray' },
  disabled: { label: '禁用', tone: 'gray' },
  unknown: { label: '未知', tone: 'gray' },
  draft: { label: '草稿', tone: 'gray' },
};

export function statusToTone(status: string): StatusTone {
  const s = status.toLowerCase();
  return DEFAULT_MAP[s]?.tone ?? 'gray';
}

export function statusToLabel(status: string): string {
  return DEFAULT_MAP[status.toLowerCase()]?.label ?? status;
}

export default function StatusBadge({
  status,
  tone,
  label,
  className,
  testId,
}: {
  status: string;
  tone?: StatusTone;
  label?: string;
  className?: string;
  testId?: string;
}) {
  const resolvedTone = tone ?? statusToTone(status);
  const resolvedLabel = label ?? statusToLabel(status);
  return (
    <span
      data-testid={testId}
      aria-label={`状态: ${resolvedLabel}`}
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium leading-4',
        TONE_CLASS[resolvedTone],
        className,
      )}
    >
      <span className="h-1 w-1 rounded-full bg-current" />
      {resolvedLabel}
    </span>
  );
}
