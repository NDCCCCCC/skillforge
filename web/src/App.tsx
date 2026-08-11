// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 3 — App composition root (P4 Wave 1).
//
// Replaces the spike stub's `spikeTransport` with `createTransport(env)`.
// In dev/test the transport is MockChatTransport replaying a fixture set
// (D-03). In prod the same factory returns DefaultChatTransport against
// `/api/chat` (D-04 swap point — Phase 2 backend).
//
// Wave 1 ships a minimal App that wires useChat + MessageBubble so we
// can verify the parts renderer end-to-end. Wave 2 adds the three-panel
// layout; Wave 3 adds citation-click → center panel chunk view.

import { useChat } from '@ai-sdk/react';
import type { UIMessage } from 'ai';
import { MessageBubble } from './components/chat/MessageBubble';
import { useAppStore } from './store/app-store';
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
  const centerView = useAppStore((s) => s.centerView);
  const env = import.meta.env.PROD ? 'prod' : 'dev';
  const { messages } = useChat({ transport: createTransport(env) });

  return (
    <div className="app-shell" data-testid="live-app">
      <div className="app-main">
        <div data-testid="panel-left" className="bg-surface border-r border-border" />
        <div data-testid="panel-center" className="bg-bg border-r border-border p-3 text-xs font-mono">
          {JSON.stringify(centerView, null, 2)}
        </div>
        <div data-testid="panel-right" className="bg-bg p-3">
          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} role={m.role === 'user' ? 'user' : 'assistant'} />
          ))}
        </div>
      </div>
    </div>
  );
}
