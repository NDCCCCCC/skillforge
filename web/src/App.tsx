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
import { useAppStore } from './store/app-store';

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
  const lastCreatedSkill = useAppStore((s) => s.lastCreatedSkill);
  const setLastCreatedSkill = useAppStore((s) => s.setLastCreatedSkill);
  const clearLastCreatedSkill = useAppStore((s) => s.clearLastCreatedSkill);

  // Plan 05-03 — SC-2 hot-reload consumer. Mounted at the App root so
  // the subscription outlives route changes. Polling fallback disabled
  // in jsdom (the env is 'dev' and the SSE channel is mocked).
  useSkillReload();

  // Plan 05-04 — SC-3 wizard success listener. The wizard hook fires
  // 'wizard-success' as a window-level CustomEvent after a successful
  // POST /api/skills/wizard/commit. We mirror it into the store so
  // any consumer (toasts, status bar) can read it synchronously.
  useEffect(() => {
    function onSuccess(e: Event) {
      const detail = (e as CustomEvent<{ skillName: string; path: string }>).detail;
      if (!detail) return;
      setLastCreatedSkill({ name: detail.skillName, path: detail.path, ts: Date.now() });
    }
    window.addEventListener('wizard-success', onSuccess);
    return () => window.removeEventListener('wizard-success', onSuccess);
  }, [setLastCreatedSkill]);

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
      {lastCreatedSkill && (
        <div
          data-testid="wizard-success-toast"
          role="status"
          aria-live="polite"
          className="fixed bottom-4 right-4 z-40 rounded-md border border-status-compliant/40 bg-surface-2 px-4 py-3 shadow-lg"
        >
          <div className="flex items-center gap-2 text-xs font-semibold text-status-compliant">
            <span>Skill 已创建：{lastCreatedSkill.name}。已注册并热加载。</span>
          </div>
          <div className="mt-1 font-mono text-[11px] text-text-muted">
            {lastCreatedSkill.path}
          </div>
          <button
            type="button"
            data-testid="wizard-success-toast-close"
            onClick={clearLastCreatedSkill}
            className="absolute right-1 top-1 inline-flex h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-surface-3"
            aria-label="关闭"
          >
            ×
          </button>
        </div>
      )}
    </>
  );
}
