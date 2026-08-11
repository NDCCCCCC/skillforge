// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// RightChat — right chat panel. Header holds the SkillPicker; the
// messages scroll area renders one MessageBubble per message with
// auto-stick-to-bottom (preserves user scroll position if they
// scrolled up). Empty-state hero shows 3 suggested prompts per
// 04-UI-SPEC §Copywriting Contract. Footer mounts ChatInput.
import { useEffect, useRef } from 'react';
import type { UIMessage } from 'ai';
import { MessageBubble } from '../chat/MessageBubble';
import ChatInput from '../chat/ChatInput';
import SkillPicker from '../chat/parts/SkillPicker';

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

  useEffect(() => {
    const el = messagesRef.current;
    if (!el) return;
    // Auto-stick to bottom on new messages, but only if the user was
    // already at the bottom (preserves their scroll position if they
    // scrolled up to read older content).
    if (lastStuckRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages]);

  function handleScroll() {
    const el = messagesRef.current;
    if (!el) return;
    const threshold = 32;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    lastStuckRef.current = distanceFromBottom <= threshold;
  }

  return (
    <div data-testid="right-chat" className="flex flex-col h-full bg-bg">
      <div className="border-b border-border px-3 py-2">
        <SkillPicker />
      </div>

      {messages.length === 0 ? (
        <div
          data-testid="right-chat-empty"
          className="flex flex-col items-center justify-center h-full text-center p-6"
        >
          <div className="text-2xl font-semibold text-text">开始一次合规审查</div>
          <p className="mt-2 text-base text-text-muted">
            选择左侧 Skill，在此处提问。每个结论都附可点击的引用与置信度。
          </p>
          <div className="mt-4 flex flex-col gap-2">
            {SUGGESTED_PROMPTS.map((prompt, i) => (
              <button
                key={i}
                type="button"
                data-testid={`right-chat-suggested-${i}`}
                onClick={() => onSend(prompt)}
                className="rounded-md px-3 py-1 text-xs bg-surface-2 border border-border hover:bg-surface-3"
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
