// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 12-06 Task 4 — extends ChatColumn with PlanTracePart (PLAN-02
// SC-2). The component reads the plan slice's currentRunId (set by the
// RealChatTransport on every data-plan-trace chunk) and renders the
// live todo list above the messages area. D14 invariant: PlanTracePart
// is the AUTHORITATIVE surface for plan state — it never reads from
// useChat.messages for plan (the synthetic write_todos fallback in
// PlanTracePart is a defensive layer only).
//
// ChatColumn — the chat-first layout's main column (Open Design
// redesign B2; formerly RightChat, the 420px right panel of the
// three-panel layout). The panel header (session + skill badge) moved
// to the TopBar in B3; this file now owns only the chat surface:
//   - Empty state: hero with gradient icon, helpful copy, and 3
//     suggested prompts (each rendered as a hover-lift card)
//   - Messages scroll area: subtle gradient backdrop
//   - Auto-stick-to-bottom preserves user's scroll position
//   - WizardMode replaces the whole column when wizardMode.active
import { useEffect, useRef } from 'react';
import { Briefcase, CalendarDays, Hotel, Info, Sparkles } from 'lucide-react';
import type { UIMessage } from 'ai';
import { MessageBubble } from '../chat/MessageBubble';
import ChatInput from '../chat/ChatInput';
import DraftStreamChip from '../chat/parts/DraftStreamChip';
import WizardMode from './WizardMode';
import { ApprovalCard } from '../ApprovalCard';
import { PlanTracePart } from '../PlanTracePart';
import { useAppStore } from '../../store/app-store';

// Open Design B5 — emoji sug-card icons are replaced by lucide icons;
// the suggested-question copy (business examples) is unchanged.
const SUGGESTED_PROMPTS = [
  { icon: CalendarDays, text: '年假未休完的部分如何折算或结转？' },
  { icon: Briefcase, text: '采购合同 50 万元以下由谁审批？' },
  { icon: Hotel, text: '出差住宿费超过标准额度时如何审批？' },
];

export interface ChatColumnProps {
  messages: UIMessage[];
  status: string;
  onSend: (text: string) => void;
  /**
   * 中止正在跑的 LLM 流（useChat 的 stop）。流式进行中时 composer
   * 的发送按钮切换成「停止」—— 没有它，用户只能等这一轮跑完，无法
   * 打断一条已经跑偏的回答。
   *
   * D14 不变量：stop 只是 useChat 的产物，作为 prop 逐层传递，
   * **不进 Zustand store**（store 里不能出现第二份消息/流状态）。
   */
  onStop?: () => void;
  /** Boot-hydration send gate (debug new-session-shows-old-messages
   * residual #2): while the mount roster fetch is in flight the active
   * session may still be the throwaway boot session and the skill is
   * unresolved — a send now would create a "ghost" session. App passes
   * false until the fetch settles (fail-open bounded). Gates BOTH the
   * composer and the empty-state suggested prompts (a prompt click is
   * a send). */
  inputDisabled?: boolean;
  /**
   * 语义路由的兜底结果（2026-11-09）。后端在 ambiguous=true 但仍有
   * 可用 top1 时会直接执行它并返回答案 —— 这个 prop 让 UI 说明
   * 「已自动选了谁、可能不是你要的知识域」，避免兜底被当成确定结论。
   * 两者都为空/undefined 时不渲染。
   */
  routeNotice?: { skill: string; top1: number } | null;
}

export default function ChatColumn({
  messages,
  status,
  onSend,
  onStop,
  inputDisabled = false,
  routeNotice = null,
}: ChatColumnProps) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const lastStuckRef = useRef(true);
  const wizardActive = useAppStore((s) => s.wizardMode.active);
  const draftStatus = useAppStore((s) => s.draftStatus);
  const draftText = useAppStore((s) => s.draftText);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  // Plan 12-06 Task 4 — current runId for the plan slice. The
  // RealChatTransport writes this on every data-plan-trace chunk.
  // PlanTracePart uses it as the lookup key into planByRunId.
  const planCurrentRunId = useAppStore((s) => s.plan.currentRunId);

  useEffect(() => {
    if (wizardActive) return;
    const el = messagesRef.current;
    if (!el) return;
    if (lastStuckRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, wizardActive]);

  function handleScroll() {
    const el = messagesRef.current;
    if (!el) return;
    const threshold = 32;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    lastStuckRef.current = distanceFromBottom <= threshold;
  }

  if (wizardActive) {
    return <WizardMode />;
  }

  return (
    <div data-testid="chat-column" className="relative flex h-full flex-col">
      {/* Plan 11-06 — HITL approval-gate overlay. Store-derived (D14):
          renders null until store.approval.pending/terminal is set;
          floats above the streaming chat at the interrupt position. */}
      <ApprovalCard sessionId={activeSessionId} />
      {/* Plan 12-06 — live planning-trace chip. Inline above the
          messages scroll when the SSE stream has emitted a
          data-plan-trace for the current run (PLAN-02 SC-2). Collapses
          to a one-line summary after the run completes. Reads
          exclusively from the plan slice (D14-extension); the synthetic
          write_todos fallback in PlanTracePart is defensive only. */}
      {planCurrentRunId && <PlanTracePart runId={planCurrentRunId} />}
      {/* 2026-11-09 — 路由兜底提示。后端在 ambiguous=true 且给出
          selected 时会直接执行 top1 并返回答案；这条提示把「我不太
          确定」这件事讲明白，否则用户会把兜底答案当成确定结论。 */}
      {routeNotice && (
        <div
          data-testid="route-fallback-notice"
          role="status"
          aria-live="polite"
          className="mx-4 mt-2 flex items-start gap-2 rounded-md border border-text-muted/25 bg-surface-2 px-3 py-2 text-[11px] leading-relaxed text-text-muted"
        >
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
          <span>
            已自动选择 Skill{' '}
            <span className="font-semibold">{routeNotice.skill}</span>
            {routeNotice.top1 > 0
              ? '（最高匹配度 ' + (routeNotice.top1 * 100).toFixed(0) + '%）'
              : ''}
            ，可能不是你要的知识域。可以切换 Skill 后重问。
          </span>
        </div>
      )}
      {messages.length === 0 ? (
        <div
          data-testid="chat-empty"
          className="chat-empty-modern"
        >
          <div className="chat-empty-icon chat-empty-icon-gradient">
            <Sparkles className="h-6 w-6" />
          </div>
          <div className="chat-empty-title">SkillForge</div>
          <p className="chat-empty-subtitle">
            用 Skill 定义你的 Agent，提问即获可信回答。
          </p>
          <div className="chat-empty-prompts">
            {SUGGESTED_PROMPTS.map((prompt, i) => {
              const Icon = prompt.icon;
              return (
                <button
                  key={i}
                  type="button"
                  data-testid={`chat-suggested-${i}`}
                  onClick={() => onSend(prompt.text)}
                  disabled={inputDisabled}
                  className="chat-empty-prompt"
                >
                  <span className="chat-empty-prompt-icon" aria-hidden>
                    <Icon className="h-3.5 w-3.5" />
                  </span>
                  <span style={{ flex: 1 }}>{prompt.text}</span>
                </button>
              );
            })}
          </div>
        </div>
      ) : (
        <div
          ref={messagesRef}
          onScroll={handleScroll}
          data-testid="chat-messages"
          data-chat-status={status}
          className="chat-messages-modern flex-1 overflow-y-auto px-4 py-2"
        >
          {messages.map((m) => (
            <MessageBubble
              key={m.id}
              message={m}
              role={m.role === 'user' ? 'user' : 'assistant'}
            />
          ))}
          {draftStatus !== 'idle' && (
            <DraftStreamChip
              status={draftStatus as 'pending' | 'verified' | 'refused'}
              draftText={draftText}
            />
          )}
        </div>
      )}

      <ChatInput
        onSend={onSend}
        disabled={inputDisabled}
        streaming={status === 'streaming'}
        onStop={onStop}
      />
    </div>
  );
}
