// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// RightChat — right chat panel (modernized Phase 4 visual upgrade).
//
// Plan 05-07 (D5-IA-01 + D5-IA-02 + D5-IA-04) — the header is now a
// MINIMAL read-only badge showing the active session name + the
// current Skill name (with a status dot when a Skill is active).
//
// Modernized visuals:
//   - Header: clean two-line layout with title + skill pill status
//   - Empty state: hero with gradient icon, helpful copy, and 3
//     suggested prompts (each rendered as a hover-lift card)
//   - Messages scroll area: subtle gradient backdrop
//   - Auto-stick-to-bottom preserves user's scroll position
import { useEffect, useRef } from 'react';
import { Sparkles, MessageCircle } from 'lucide-react';
import type { UIMessage } from 'ai';
import { MessageBubble } from '../chat/MessageBubble';
import ChatInput from '../chat/ChatInput';
import DraftStreamChip from '../chat/parts/DraftStreamChip';
import WizardMode from './WizardMode';
import { useAppStore } from '../../store/app-store';

const SUGGESTED_PROMPTS = [
  { icon: '📅', text: '年假未休完的部分如何折算或结转？' },
  { icon: '💼', text: '采购合同 50 万元以下由谁审批？' },
  { icon: '🏨', text: '出差住宿费超过标准额度时如何审批？' },
];

export interface RightChatProps {
  messages: UIMessage[];
  status: string;
  onSend: (text: string) => void;
}

export default function RightChat({ messages, status, onSend }: RightChatProps) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const lastStuckRef = useRef(true);
  const wizardActive = useAppStore((s) => s.wizardMode.active);
  const sessions = useAppStore((s) => s.sessions);
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const skills = useAppStore((s) => s.skills);
  const currentSkillId = useAppStore((s) => s.currentSkillId);
  const activeSession = sessions.find((s) => s.id === activeSessionId);
  const currentSkill = skills.find((s) => s.id === currentSkillId);
  const draftStatus = useAppStore((s) => s.draftStatus);
  const draftText = useAppStore((s) => s.draftText);

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
    <div data-testid="right-chat" className="flex flex-col h-full">
      <div
        data-testid="right-chat-badge"
        className="chat-header-modern"
      >
        <MessageCircle
          className="h-4 w-4"
          style={{ color: 'var(--color-primary)' }}
        />
        <span
          data-testid="right-chat-badge-session"
          className="chat-header-title"
        >
          {activeSession?.name ?? '新会话'}
        </span>
        <span aria-hidden className="chat-header-divider">·</span>
        <span
          data-testid="right-chat-badge-skill"
          className="chat-header-skill"
        >
          {currentSkill ? (
            <>
              <span className="chat-header-skill-dot" aria-hidden />
              <span>{currentSkill.name}</span>
            </>
          ) : (
            <span className="chat-header-skill-missing">未选择 Skill</span>
          )}
        </span>
      </div>

      {messages.length === 0 ? (
        <div
          data-testid="right-chat-empty"
          className="chat-empty-modern"
        >
          <div className="chat-empty-icon">
            <Sparkles className="h-6 w-6" />
          </div>
          <div className="chat-empty-title">开始一次文档问答</div>
          <p className="chat-empty-subtitle">
            选择 Skill，在此处提问。每个回答都附可点击的引用与置信度。
          </p>
          <div className="chat-empty-prompts">
            {SUGGESTED_PROMPTS.map((prompt, i) => (
              <button
                key={i}
                type="button"
                data-testid={`right-chat-suggested-${i}`}
                onClick={() => onSend(prompt.text)}
                className="chat-empty-prompt"
              >
                <span className="chat-empty-prompt-icon" aria-hidden>
                  {prompt.icon}
                </span>
                <span style={{ flex: 1 }}>{prompt.text}</span>
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

      <ChatInput onSend={onSend} />
    </div>
  );
}
