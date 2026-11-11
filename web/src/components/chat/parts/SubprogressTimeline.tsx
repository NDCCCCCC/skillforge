// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SubprogressTimeline — the agent pipeline's step stream, rendered as ONE
// continuous, stateful timeline instead of a pile of detached rows.
//
// 2026-11-11 用户反馈："没有一步一步来的感觉"。
//
// 旧实现（DataSubprogressPart 逐 part 平铺）的问题：
//   1. 后端 retrieve / found_candidates 之间只隔几十毫秒，两条 dot
//      同时冒出来，"先后"读不出来；
//   2. 所有 dot 同色同尺寸，已经跑完的步骤和正在跑的步骤长得一模一样；
//   3. 每条 step 各自 absolute 定位连接线，跨 part 对不齐，中间还夹着
//      别的 part —— 视觉上根本不成一条线。
//
// 这一版的做法：
//   - 一个卡片容器（"执行过程"），一条连续竖线，所有行共享；
//   - 逐条入场：两条之间的显示间隔不小于 STEP_STAGGER_MS，同时到达的
//     步骤也会一条一条落下来；新行有淡入+上移，旧行原地不动；
//   - 三态可见：已完成（勾）/ 进行中（转圈 + 脉冲）；当前这一步高亮，
//     其余降权 —— 眼睛始终知道"现在跑到第几枪"；
//   - 每一步结束时留下真实耗时（下一步入场时刻 - 本步入场时刻），
//     用户看得见"这一步花了 32 秒"，而不是一坨静止的列表；
//   - 「正在生成结论」那一步额外显示实时输出字数（订阅 store 里的
//     draftText —— 后端 streamOnce 逐 token 发 text-delta-draft，所以
//     这个数字是真进度，不是假动画）。后端实测：5 条步骤在 0.3 秒内
//     全发完，之后 66 秒只有模型在生成；没有这行跳动的"已输出 N 字 /
//     Ns"，界面在那一分钟里就是死的；
//   - 回复结束后自动整体落定（全部打勾、标题变"已完成 N 步"），仍可
//     手动收起/展开。
import { useEffect, useRef, useState } from 'react';
import { Check, ChevronDown, ChevronUp, Loader2 } from 'lucide-react';
import type { SubprogressStep } from '../../../types/verdict';
import { useTierLabel } from '../../../lib/kb-bases';
import { useAppStore } from '../../../store/app-store';
import { renderStepCopy } from './subprogress-copy';

/** 两条步骤之间最小的显示间隔（毫秒）。后端可能一口气连发多条，
 *  照原样追加就失去了"一步一步"的推进感。 */
export const STEP_STAGGER_MS = 420;

export interface SubprogressEntry {
  /** 稳定 key —— part.id 优先，其次索引。 */
  key: string;
  data: SubprogressStep;
}

type StepState = 'done' | 'active';

function formatElapsed(ms: number): string {
  if (!Number.isFinite(ms) || ms < 1000) return '';
  const seconds = ms / 1000;
  return seconds < 10 ? `${seconds.toFixed(1)}s` : `${Math.round(seconds)}s`;
}

function StepRow({
  entry,
  state,
  elapsedMs,
  streamChars,
}: {
  entry: SubprogressEntry;
  state: StepState;
  elapsedMs: number;
  /** 当前这轮已到达的草稿字数（只有生成类步骤才展示）。 */
  streamChars: number;
}) {
  const tier = entry.data.tier ?? '';
  // Hook 必须无条件调用；空 tier 时 useTierLabel('') 是 no-op。
  const resolvedLabel = useTierLabel(tier);
  const copy = renderStepCopy(entry.data, tier ? resolvedLabel : null);
  if (!copy) return null;
  const elapsed = formatElapsed(elapsedMs);
  // 进行中的那一步显示"进行中 · 32s" —— 后端在 0.3 秒内把 5 条步骤全发完，
  // 之后是几十秒的模型生成期；没有这行跳动的秒数，界面就是死的。
  const meta = state === 'active' ? (elapsed ? `进行中 · ${elapsed}` : '进行中') : elapsed;
  const generating =
    entry.data.step === 'generate' || entry.data.step === 'generating';
  const showChars = state === 'active' && generating && streamChars > 0;
  return (
    <li
      className="proc-step"
      data-state={state}
      data-step={entry.data.step}
      data-testid="proc-step"
    >
      <span className="proc-step-marker" aria-hidden="true">
        {state === 'active' ? (
          <Loader2 className="proc-step-spinner" />
        ) : (
          <Check className="proc-step-check" />
        )}
      </span>
      <span className="proc-step-text">
        {copy}
        {showChars && (
          <span className="proc-step-stream" data-testid="proc-stream-chars">
            {' '}
            · 已输出 {streamChars} 字
          </span>
        )}
      </span>
      <span className="proc-step-meta">{meta}</span>
    </li>
  );
}

export interface SubprogressTimelineProps {
  entries: SubprogressEntry[];
  /** 本段之后回复是否仍在生成（决定最后一行是"进行中"还是"已完成"）。 */
  live: boolean;
}

export default function SubprogressTimeline({ entries, live }: SubprogressTimelineProps) {
  const total = entries.length;
  // 历史消息（live=false）首帧就要看到完整过程，不做入场节流；
  // 正在生成的回复从第一条开始一条一条落。
  const [shown, setShown] = useState(() => (live ? Math.min(1, total) : total));
  const [collapsed, setCollapsed] = useState(false);
  // 后端逐 token 发 text-delta-draft → store.draftText 就是这一轮模型已经
  // 吐出来的真实内容。拿它的字数做「已输出 N 字」，是真实进度而非假动画。
  const draftText = useAppStore((s) => s.draftText);
  // 每秒一跳：让"进行中 · Ns"真的在走（后端发完步骤后到模型出结果之间
  // 常有几十秒静默，静止的"进行中"看起来像卡死）。
  const [, setTick] = useState(0);
  // 每条实际显示的时刻，用来算上一条的耗时。
  const shownAt = useRef<number[]>([]);
  const lastShownAt = useRef(0);

  useEffect(() => {
    if (!live) return;
    // 首条（初始 shown=1）的入场时刻在这里补记，否则它在直到被第二条取代
    // 之前都拿不到计时起点，"进行中"后面永远是空的。
    if (shownAt.current[0] === undefined) shownAt.current[0] = Date.now();
    const id = window.setInterval(() => setTick((n) => n + 1), 1000);
    return () => window.clearInterval(id);
  }, [live]);

  // 流式结束 / 会话恢复：补齐全部（历史消息没有入场计时，耗时列自然为空；
  // 从 live 过渡到结束的这一次保留已经测到的耗时，别让用户刚看到的
  // "32s" 在回答落定的瞬间消失）。
  useEffect(() => {
    if (live) return;
    setShown((n) => (n === total ? n : total));
  }, [live, total]);

  // 逐条入场：距上一条显示不足 STEP_STAGGER_MS 时排队，否则立即显示。
  useEffect(() => {
    if (!live || shown >= total) return;
    const elapsed = Date.now() - lastShownAt.current;
    const wait = shown === 0 || elapsed >= STEP_STAGGER_MS ? 0 : STEP_STAGGER_MS - elapsed;
    const timer = window.setTimeout(() => {
      lastShownAt.current = Date.now();
      shownAt.current[shown] = lastShownAt.current;
      setShown((n) => Math.min(n + 1, total));
    }, wait);
    return () => window.clearTimeout(timer);
  }, [live, shown, total]);

  if (total === 0) return null;

  const visible = entries.slice(0, Math.max(shown, 0));

  return (
    <div className="proc-card" data-live={live ? 'true' : 'false'} data-testid="proc-card">
      <div className="proc-card-head">
        <span className="proc-card-title">执行过程</span>
        {live ? (
          <span className="proc-card-status" data-testid="proc-status">
            <Loader2 className="proc-card-status-spinner" aria-hidden="true" />
            进行中
          </span>
        ) : (
          <span className="proc-card-status proc-card-status-done" data-testid="proc-status">
            已完成 {total} 步
          </span>
        )}
        <button
          type="button"
          className="proc-card-toggle"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed((value) => !value)}
        >
          {collapsed ? '展开' : '收起'}
          {collapsed ? (
            <ChevronDown className="h-3 w-3" aria-hidden="true" />
          ) : (
            <ChevronUp className="h-3 w-3" aria-hidden="true" />
          )}
        </button>
      </div>
      {!collapsed && (
        <ol className="proc-list" aria-live={live ? 'polite' : 'off'}>
          {visible.map((entry, index) => {
            const active = live && index === shown - 1;
            // 耗时 = 下一步入场时刻 - 本步入场时刻；最后一步/历史步骤无值。
            const start = shownAt.current[index];
            const next = shownAt.current[index + 1];
            const elapsedMs = active
              ? typeof start === 'number'
                ? Date.now() - start
                : 0
              : typeof start === 'number' && typeof next === 'number'
                ? next - start
                : 0;
            return (
              <StepRow
                key={entry.key}
                entry={entry}
                state={active ? 'active' : 'done'}
                elapsedMs={elapsedMs}
                streamChars={draftText.length}
              />
            );
          })}
        </ol>
      )}
    </div>
  );
}
