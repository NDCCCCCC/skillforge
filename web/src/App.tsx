// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 + Plan 04-02 Task 3 + Plan 04-03 Task 2 + Plan
// 05-03 Task 2 + Plan 05-07 Task 1 + Plan 05-08 Task 1 + Plan 06-04
// T1 — App composition root.
// Wires:
//   - ThreePanel layout (with extracted TopBar / StatusBar in 05-03)
//   - useChat + createTransport(env) — chat streaming
//     (Plan 05-08: per-session Chat instance kept in `chatMapRef`; useChat
//     receives the Chat instance via the `chat` prop (NOT `id`) so messages
//     survive activeSessionId flips. Messages are in-memory and lost on
//     refresh (D5-IA-03) — no backend ListSessions.)
//   - useSkillReload — polls /api/skills; updates store.skillReloadStatus
//     on every version bump or status flip (ok↔failed) AND seeds
//     store.skills from item.manifest.description
//   - ReloadToast — surfaces success/failure toasts (auto-dismiss 4s
//     on success; persistent on failure)
//   - CenterPanel — 5-tab strip (Phase 5: Agent 状态 + 知识图谱)
//   - RightChat — SkillPicker + ChatInput + MessageBubble per message
//   - Plan 06-04 (D6-01 + D-04/D12): route branch — when the store's
//     `route === 'eval'`, the chat three-panel is replaced by
//     <EvalDashboard /> (admin mode). The eval route has its OWN
//     transport (plain fetch + 30s interval per D6-03); the chat SSE
//     wire is untouched. A popstate listener mirrors
//     window.location.pathname into the store so back/forward works.
import { useEffect, useMemo, useRef, useState } from 'react';
import { Chat, useChat } from '@ai-sdk/react';
import type { UIMessage } from 'ai';
import CenterPanel from './components/center/CenterPanel';
import ReloadToast from './components/layout/ReloadToast';
import RightChat from './components/layout/RightChat';
import ThreePanel from './components/layout/ThreePanel';
import LeftSidebar from './components/layout/LeftSidebar';
import EvalDashboard from './components/eval/EvalDashboard';
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
  // Plan 06-04 (D6-01 + D-04/D12) — route branch. When the user navigates
// to /eval (via LeftSidebar, popstate, or a programmatic setRoute),
// the chat three-panel is replaced by EvalDashboard (admin mode).
// The eval route has its own (non-SSE) transport; the chat SSE wire
// stays untouched.
  const route = useAppStore((s) => s.route);
  const env: 'prod' | 'dev' = ((import.meta as unknown) as { env: { PROD?: boolean } }).env.PROD ? 'prod' : 'dev';
  const transport = useMemo(() => createTransport(env), [env]);
  // Plan 05-08 Task 1 (CR-01 fix) — per-session Chat<UIMessage> map
  // kept in a useRef so each session retains its own message stream
  // across activeSessionId flips. The `chat` prop is passed to useChat
  // (NOT `id`) — passing both `chat` and `id` would still trigger the
  // shouldRecreateChat branch in @ai-sdk/react/dist/index.js:326-328.
  // Messages are in-memory and lost on refresh (D5-IA-03 / D14: no
  // messages field in the Zustand store — per-session Chat instances
  // hold messages in their internal state).
  const activeSessionId = useAppStore((s) => s.activeSessionId);
  const chatMapRef = useRef<Map<string, Chat<UIMessage>>>(new Map());
  const chat = useMemo(() => {
    const existing = chatMapRef.current.get(activeSessionId);
    if (existing) return existing;
    const created = new Chat({ id: activeSessionId, transport });
    chatMapRef.current.set(activeSessionId, created);
    return created;
  }, [activeSessionId, transport]);
  const { messages, sendMessage, status } = useChat({ chat });
  const [currentVerdict, setCurrentVerdict] = useState<Verdict | null>(null);
  const lastCreatedSkill = useAppStore((s) => s.lastCreatedSkill);
  const setLastCreatedSkill = useAppStore((s) => s.setLastCreatedSkill);
  const clearLastCreatedSkill = useAppStore((s) => s.clearLastCreatedSkill);
  const setRoute = useAppStore((s) => s.setRoute);
  const setEvalSubroute = useAppStore((s) => s.setEvalSubroute);

  // Plan 06-04 — sync window.location.pathname into the store. We
  // boot from the current URL on mount (so a hard reload at /eval
  // lands in admin mode), and listen for popstate so back/forward
  // navigates between /eval and /eval/audit correctly. pushState is
  // driven by the LeftSidebar nav button and the in-app subroute
  // tabs inside EvalDashboard.
  useEffect(() => {
    function syncFromLocation() {
      const path = window.location.pathname;
      if (path === '/eval/audit') {
        setRoute('eval');
        setEvalSubroute('audit');
      } else if (path === '/eval') {
        setRoute('eval');
        setEvalSubroute('dashboard');
      } else {
        setRoute('chat');
      }
    }
    syncFromLocation();
    window.addEventListener('popstate', syncFromLocation);
    return () => window.removeEventListener('popstate', syncFromLocation);
  }, [setRoute, setEvalSubroute]);

  // Plan 05-03 — SC-2 hot-reload consumer. Mounted at the App root so
  // polling outlives route changes. Polls GET /api/skills and diffs the
  // per-skill version/status, emitting on a version bump or status flip.
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

  // Plan 06-04 (D6-01) — admin-mode route renders the EvalDashboard
  // shell instead of the chat three-panel. The eval component handles
  // its own data fetching + subroute switching (D6-04) and has no SSE.
  if (route === 'eval') {
    return (
        <>
          <EvalDashboard />
          <ReloadToast />
        </>
      );
  }

  return (
    <>
      <ThreePanel
        left={<LeftSidebar />}
        center={
          <CenterPanel
            currentVerdict={currentVerdict}
            messages={messages}
            chatStatus={status}
          />
        }
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
