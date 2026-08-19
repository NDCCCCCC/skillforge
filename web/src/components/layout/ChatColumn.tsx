// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

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
import { Sparkles } from 'lucide-react';
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

export interface ChatColumnProps {
  messages: UIMessage[];
  status: string;
  onSend: (text: string) => void;
}

export default function ChatColumn({ messages, status, onSend }: ChatColumnProps) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const lastStuckRef = useRef(true);
  const wizardActive = useAppStore((s) => s.wizardMode.active);
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
    <div data-testid="chat-column" className="flex h-full flex-col">
      {messages.length === 0 ? (
        <div
          data-testid="chat-empty"
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
                data-testid={`chat-suggested-${i}`}
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
