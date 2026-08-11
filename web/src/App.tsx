// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 + Plan 04-02 Task 3 + Plan 04-03 Task 2 — App
// composition root. Wires the three-panel layout via ThreePanel,
// useChat + createTransport(env), the CenterPanel that hosts
// VerdictCard / RefusalCard / ChunkSourceView / SkillMdPreview, and
// the right chat panel with SkillPicker + ChatInput. Citation click
// flows through Zustand (useAppStore.getState().setCenterView(...)) —
// no prop drilling.
import { useEffect, useMemo, useState } from 'react';
import { useChat } from '@ai-sdk/react';
import type { UIMessage } from 'ai';
import CenterPanel from './components/center/CenterPanel';
import RightChat from './components/layout/RightChat';
import ThreePanel from './components/layout/ThreePanel';
import LeftSidebar from './components/layout/LeftSidebar';
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
  const { messages, sendMessage, status } = useChat({ transport });
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

  function handleSend(text: string) {
    void sendMessage({ text });
  }

  return (
    <ThreePanel
      left={<LeftSidebar />}
      center={<CenterPanel currentVerdict={currentVerdict} />}
      right={
        <RightChat
          messages={messages}
          status={status}
          onSend={handleSend}
        />
      }
    />
  );
}
