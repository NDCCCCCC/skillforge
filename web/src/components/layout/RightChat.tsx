// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// RightChat — right chat panel.
//
// Plan 05-07 (D5-IA-01 + D5-IA-02 + D5-IA-04) — the header is now a
// MINIMAL read-only badge showing the active session name + the
// current Skill name. Selection lives elsewhere:
//   - Skill selection: ChatInput selector (D5-IA-01).
//   - Session list + 新建会话: LeftSidebar 会话 block (D5-IA-02/03).
//   - Skill browse + status: LeftSidebar 技能 block (D5-IA-04).
// The header badge is informational only (no controls) and falls back
// to safe placeholder copy when nothing is selected.
//
// SkillPicker + SessionPicker header USAGE is removed (D5-IA-01/02).
// The SkillPicker COMPONENT stays in the codebase — RoutingTracePart
// still imports it for the SC-1 ambiguity picker mode.
//
// The messages scroll area renders one MessageBubble per message with
// auto-stick-to-bottom (preserves user scroll position if they scrolled
// up). Empty-state hero shows 3 suggested prompts per 04-UI-SPEC
// §Copywriting Contract. Footer mounts ChatInput.
import { useEffect, useRef } from 'react';
import { ShieldCheck } from 'lucide-react';
import type { UIMessage } from 'ai';
import { MessageBubble } from '../chat/MessageBubble';
import ChatInput from '../chat/ChatInput';
import WizardMode from './WizardMode';
import { useAppStore } from '../../store/app-store';

const SUGGESTED_PROMPTS = [
  '这家公司的数据跨境是否符合《数据安全法》?',
  '请审查这份员工离职条款的合规性',
  '差旅报销制度有哪些合规风险?',
];

export interface RightChatProps {
  messages: UIMessage[];
  status: string;
  onSend: (text: string) => void;
}

export default function RightChat({ messages, status, onSend }: RightChatProps) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const lastStuckRef = useRef(true);
  // Plan 05-04 — wizard mode swaps the chat header for the wizard
  // header. The chat messages / ChatInput stay, but the user can no
  // longer interact with them in wizard mode (WizardMode reuses the
  // ChatInput component for answer capture, but routes through
  // useWizard.submitAnswer, not onSend).
  const wizardActive = useAppStore((s) => s.wizardMode.active);
  // Plan 05-07 — read-only header badge sources.
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const skills = useAppStore((s) => s.skills);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const activeSession = sessions.find((s) => s.id === activeSessionId);
  const currentSkill = skills.find((s) => s.id === currentSkillId);

  useEffect(() => {
    if (wizardActive) return;
    const el = messagesRef.current;
    if (!el) return;
    // Auto-stick to bottom on new messages, but only if the user was
    // already at the bottom (preserves their scroll position if they
    // scrolled up to read older content).
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
    <div data-testid="right-chat" className="flex flex-col h-full bg-bg">
      <div
        data-testid="right-chat-badge"
        className="border-b border-border px-3 py-2 flex items-center gap-2 text-xs"
      >
        <span data-testid="right-chat-badge-session" className="font-semibold text-text">
          {activeSession?.name ?? '新会话'}
        </span>
        <span aria-hidden className="text-text-dim">·</span>
        <span data-testid="right-chat-badge-skill" className="text-text-muted">
          {currentSkill ? currentSkill.name : '未选择 Skill'}
        </span>
      </div>

      {messages.length === 0 ? (
        <div
          data-testid="right-chat-empty"
          className="flex flex-col items-center justify-center h-full text-center p-6"
        >
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
            <ShieldCheck className="h-6 w-6 text-primary" />
          </div>
          <div className="text-2xl font-semibold text-text">开始一次合规审查</div>
          <p className="mt-2 text-base text-text-muted">
            选择 Skill，在此处提问。每个结论都附可点击的引用与置信度。
          </p>
          <div className="mt-5 flex flex-col gap-2 w-full max-w-sm">
            {SUGGESTED_PROMPTS.map((prompt, i) => (
              <button
                key={i}
                type="button"
                data-testid={`right-chat-suggested-${i}`}
                onClick={() => onSend(prompt)}
                className="rounded-md px-3 py-2 text-xs text-text-muted bg-surface-2 border border-border hover:bg-surface-3 hover:text-text transition-colors"
              >
                {prompt}
              </button>
            ))}
          </div>
        </div>
      ) : (
        <div
          ref={messagesRef}
          onScroll={handleScroll}
          data-testid="chat-messages"
          data-chat-status={status}
          className="flex-1 overflow-y-auto p-3 space-y-3"
        >
          {messages.map((m) => (
            <MessageBubble
              key={m.id}
              message={m}
              role={m.role === 'user' ? 'user' : 'assistant'}
            />
          ))}
        </div>
      )}

      <ChatInput onSend={onSend} />
    </div>
  );
}
