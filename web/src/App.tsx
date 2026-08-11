// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 + Plan 04-02 Task 3 — App composition root (P4 Wave 2).
//
// Wires the three-panel layout (LeftSidebar skeleton / CenterPanel /
// right chat panel), useChat + createTransport(env) factory, and the
// CenterPanel that hosts VerdictCard / RefusalCard / ChunkSourceView
// / SkillMdPreview. Citation click flows through Zustand
// (useAppStore.getState().setCenterView(...)) — no prop drilling.
//
// Wave 2 keeps LeftSidebar as a skeleton (Wave 3 fills it with the
// skill/session list). Right panel hosts the MessageBubble list.
import { useEffect, useMemo, useState } from 'react';
import { useChat } from '@ai-sdk/react';
import type { UIMessage } from 'ai';
import CenterPanel from './components/center/CenterPanel';
import { MessageBubble } from './components/chat/MessageBubble';
import { isVerdict } from './types/verdict';
import type { Verdict } from './types/verdict';
import { createTransport } from './transport';

interface AppProps {
  /** Optional seeded messages for tests (skips the live useChat hook). */
  initialMessages?: UIMessage[];
}

export default function App({ initialMessages }: AppProps) {
  if (initialMessages) {
    return (
      <div data-testid="seeded-view">
        {initialMessages.map((m) => (
          <MessageBubble key={m.id} message={m} role={m.role === 'user' ? 'user' : 'assistant'} />
        ))}
      </div>
    );
  }

  return <LiveApp />;
}

function LiveApp() {
  const env: 'prod' | 'dev' = ((import.meta as unknown) as { env: { PROD?: boolean } }).env.PROD ? 'prod' : 'dev';
  const transport = useMemo(() => createTransport(env), [env]);
  const { messages } = useChat({ transport });
  const [currentVerdict, setCurrentVerdict] = useState<Verdict | null>(null);

  // Walk messages in reverse, find the latest assistant message
  // carrying a data-policy part with a Verdict payload — that is the
  // protagonist verdict (D-02).
  useEffect(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.role !== 'assistant') continue;
      for (const part of m.parts) {
        if (part.type !== 'data-policy') continue;
        const data = (part as { data?: unknown }).data;
        if (isVerdict(data)) {
          setCurrentVerdict(data);
          return;
        }
      }
    }
    setCurrentVerdict(null);
  }, [messages]);

  return (
    <div className="app-shell" data-testid="live-app">
      <div className="app-main">
        <div
          data-testid="panel-left"
          className="border-r border-border bg-surface"
        />
        <CenterPanel currentVerdict={currentVerdict} />
        <div data-testid="panel-right" className="bg-bg p-3">
          {messages.map((m) => (
            <MessageBubble
              key={m.id}
              message={m}
              role={m.role === 'user' ? 'user' : 'assistant'}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
