// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 + Plan 04-02 Task 3 + Plan 04-03 Task 2 + Plan
// 05-03 Task 2 — App composition root. Wires:
//   - ThreePanel layout (with extracted TopBar / StatusBar in 05-03)
//   - useChat + createTransport(env) — chat streaming
//   - useSkillReload — subscribes to /api/skills/events + polling
//     fallback; updates store.skillReloadStatus on every reload event
//   - ReloadToast — surfaces success/failure toasts (auto-dismiss 4s
//     on success; persistent on failure)
//   - CenterPanel — 5-tab strip (Phase 5: Agent 状态 + 知识图谱)
//   - RightChat — SkillPicker + ChatInput + MessageBubble per message
import { useEffect, useMemo, useState } from 'react';
import { useChat } from '@ai-sdk/react';
import type { UIMessage } from 'ai';
import CenterPanel from './components/center/CenterPanel';
import ReloadToast from './components/layout/ReloadToast';
import RightChat from './components/layout/RightChat';
import ThreePanel from './components/layout/ThreePanel';
import LeftSidebar from './components/layout/LeftSidebar';
import { MessageBubble } from './components/chat/MessageBubble';
import { useSkillReload } from './hooks/useSkillReload';
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

  // Plan 05-03 — SC-2 hot-reload consumer. Mounted at the App root so
  // the subscription outlives route changes. Polling fallback disabled
  // in jsdom (the env is 'dev' and the SSE channel is mocked).
  useSkillReload();

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
    <>
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
      <ReloadToast />
    </>
  );
}
