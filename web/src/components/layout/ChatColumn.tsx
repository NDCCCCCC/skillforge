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
import { Briefcase, CalendarDays, Hotel, Sparkles } from 'lucide-react';
import type { UIMessage } from 'ai';
import { MessageBubble } from '../chat/MessageBubble';
import ChatInput from '../chat/ChatInput';
import DraftStreamChip from '../chat/parts/DraftStreamChip';
import WizardMode from './WizardMode';
import { ApprovalCard } from '../ApprovalCard';
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
}

export default function ChatColumn({ messages, status, onSend }: ChatColumnProps) {
  const messagesRef = useRef<HTMLDivElement>(null);
  const lastStuckRef = useRef(true);
  const wizardActive = useAppStore((s) => s.wizardMode.active);
  const draftStatus = useAppStore((s) => s.draftStatus);
  const draftText = useAppStore((s) => s.draftText);
  const activeSessionId = useAppStore((s) => s.activeSessionId);

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

      <ChatInput onSend={onSend} />
    </div>
  );
}
