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
import HistoryRewriteHint from './components/chat/HistoryRewriteHint';
import { useSkillReload } from './hooks/useSkillReload';
import { useSkillAutoSwitch } from './hooks/use-skill-auto-switch';
import { isVerdict } from './types/verdict';
import type { Verdict } from './types/verdict';
import { createTransport } from './transport';
import { useAppStore } from './store/app-store';
import {
  fetchHistory,
  historyToUIMessages,
  type SessionHistoryResponse,
} from './lib/chat-restore';

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
  // Transport selection (D-03/D-04 swap point). Production builds always
  // hit the real backend (DefaultChatTransport → /api/chat). Dev defaults
  // to MockChatTransport (recorded fixtures, zero-network) so the UI is
  // fully testable without a backend and the vitest suite is never
  // coupled to one (T-04-04). Set VITE_REAL_BACKEND=1 (shell env var or
  // web/.env.local) to point dev at the live Hertz backend through the
  // vite /api→:8888 proxy — the only way to end-to-end test real RAG with
  // HMR. Mock stays the default so `npm run dev` is always standalone.
  //
  // Access import.meta.env.* DIRECTLY (not via an intermediate const):
  // Vite static-replaces the literal text `import.meta.env.X` in the
  // compiled output. Storing import.meta in a variable breaks that
  // replacement and leaves import.meta.env undefined at runtime — the
  // browser's native import.meta has no env property. Types live in
  // src/vite-env.d.ts (vite/client + VITE_REAL_BACKEND).
  const env: 'prod' | 'dev' =
    import.meta.env.PROD || import.meta.env.VITE_REAL_BACKEND === '1' ? 'prod' : 'dev';
  // Plan 260815-cos — DraftStreamChip wiring. The transport's
  // onDraftUpdate callback writes into useAppStore.draftStatus +
  // draftText; RightChat subscribes to render the chip. Memoized so
  // the chatMapRef doesn't churn on every render.
  const setDraft = useAppStore((s) => s.setDraft);
  const transport = useMemo(
    () =>
      createTransport(env, undefined, {
        onDraftUpdate: ({ status, draftText }) => setDraft(status, draftText),
      }),
    [env, setDraft],
  );
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
  // Plan 10-03 (D10-D1 + MEM-02) — fetch the prior session's history
  // on mount and inject the transformed messages into the per-session
  // Chat instance via Chat.messages setter. The AI SDK v5 React-side
  // Chat class exposes a public `messages` setter
  // (@ai-sdk/react/dist/index.js line 244) — assigning to it replaces
  // the chat's message stream. The chatMapRef-driven CR-01 invariant
  // requires the Chat to be created exactly once per session; we use
  // a separate `messagesWrittenRef` to track which sessions have
  // already had their restored messages written (so a re-render
  // does NOT clobber user-typed messages from the live chat surface).
  //
  // D14 invariant: the message stream lives in the Chat instance
  // (and the per-session chatMapRef), NEVER in the Zustand store. The
  // store only carries session metadata (activeSessionId, sessions[]).
  const [historySnapshot, setHistorySnapshot] = useState<SessionHistoryResponse | null | undefined>(
    undefined,
  );
  const restoredSessionRef = useRef<string | null>(null);
  useEffect(() => {
    if (restoredSessionRef.current === activeSessionId) return;
    restoredSessionRef.current = activeSessionId;
    let cancelled = false;
    void fetchHistory(activeSessionId).then((resp) => {
      if (cancelled) return;
      setHistorySnapshot(resp);
    });
    return () => {
      cancelled = true;
    };
  }, [activeSessionId]);
  // Skill auto-switch (D10-D2) — the BE returns the session-bound
  // skill_id on the history response; the hook writes it into the
  // Zustand store when present in the loaded skills list. Idempotent
  // (no-op when already set or when skillId is empty).
  useSkillAutoSwitch(historySnapshot?.skill_id ?? null);
  // Create the Chat instance once per session. The chatMapRef lookup
  // ensures the same Chat instance survives re-renders; on the first
  // render we build the Chat with the empty initialMessages and let
  // the history-effect below write the restored stream into it.
  const chat = useMemo(() => {
    const existing = chatMapRef.current.get(activeSessionId);
    if (existing) return existing;
    const created = new Chat({
      id: activeSessionId,
      transport,
    });
    chatMapRef.current.set(activeSessionId, created);
    return created;
  }, [activeSessionId, transport]);
  // Inject the restored messages into the Chat instance once the
  // history arrives. Chat.messages setter is the AI SDK v5 React-side
  // public seam (@ai-sdk/react/dist/index.js line 244) — assignment
  // replaces the chat's message stream with the supplied list.
  //
  // CRITICAL: this effect must fire EXACTLY ONCE per session — on the
  // first-ever restore for that session. Subsequent re-renders (e.g.
  // when the user switches back to a previously-restored session)
  // MUST NOT clobber the chat's live message stream (the user may
  // have typed new messages). appliedSessionsRef is the per-session
  // guard: it records every session that has already had its history
  // applied so the effect is a no-op on the second visit.
  //
  // For a brand-new session the history may be null (the unknown-
  // session 200+empty posture) so we write an empty message stream
  // to the Chat — the empty list is the canonical "fresh session"
  // state, and the chatMapRef stores the Chat instance for the
  // session's lifetime.
  const appliedSessionsRef = useRef<Set<string>>(new Set());
  useEffect(() => {
    if (historySnapshot === undefined) return;
    if (appliedSessionsRef.current.has(activeSessionId)) return;
    const c = chatMapRef.current.get(activeSessionId);
    if (!c) return;
    // CRITICAL: only inject the restored message stream into a
    // freshly-created Chat. If the Chat already has messages (the
    // user may have typed OR an in-flight stream may have already
    // pushed frames), DO NOT overwrite — the restored history is
    // strictly less informative than the live stream. This guards
    // against the timing race where the user types a query BEFORE
    // the history fetch resolves (the chat surface has messages
    // before history arrives).
    if (c.messages.length > 0) return;
    const restored = historySnapshot
      ? historyToUIMessages(historySnapshot, activeSessionId)
      : [];
    c.messages = restored;
    appliedSessionsRef.current.add(activeSessionId);
  }, [historySnapshot, activeSessionId]);
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
