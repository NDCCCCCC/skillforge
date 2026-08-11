// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 1 spike stub — proves the Hertz SSE → Vercel AI SDK v5 wire
// contract renders `source-url` chunks as clickable <a> elements (SC-4).
// Phase 4 will replace this with the full three-panel streaming UI per
// sketch-findings-ai-agent (layout-three-panel + chat-input-attach-skill).
//
// The stub uses the real AI SDK v7 (`ai`) + `@ai-sdk/react@v4` hooks so
// the wire contract validated here is the same one Phase 4 will build on.
// `DefaultChatTransport` reads `data: <json>` SSE frames and reassembles
// them into typed UIMessage parts (text / reasoning / source-url / etc.),
// matching what `internal/spike.EmitChunk` emits on the Go side.

import { useChat } from '@ai-sdk/react';
import { DefaultChatTransport, type UIMessage } from 'ai';
import type { ReactNode } from 'react';

interface AppProps {
  /**
   * Optional seeded messages for tests. When provided, the stub renders
   * these directly and skips the live useChat hook (no transport call,
   * no network). In production (no prop) the stub wires useChat against
   * the Hertz SSE endpoint via DefaultChatTransport.
   */
  initialMessages?: UIMessage[];
}

// One DefaultChatTransport instance is shared across LiveChat mounts.
// In dev, Vite proxies /api → http://localhost:8888 (Hertz); in prod,
// SkillForge serves the frontend and API from a single origin.
const spikeTransport = new DefaultChatTransport({
  api: '/api/spike/chat',
});

function renderPart(p: UIMessage['parts'][number], i: number): ReactNode {
  switch (p.type) {
    case 'text':
      return <p key={i}>{p.text}</p>;
    case 'reasoning':
      return (
        <details key={i}>
          <summary>reasoning</summary>
          {p.text}
        </details>
      );
    case 'source-url':
      // PHASE 4 SAFETY (T-1-08 threat): the spike sets href directly from
      // the chunk URL with NO sanitization. Phase 4 MUST add an allowlist
      // + rel="noopener noreferrer" before production. The spike test
      // never executes user URLs — it only checks the <a> renders.
      return (
        <a key={i} href={p.url}>
          [{p.title ?? p.url}]
        </a>
      );
    case 'source-document':
      return <span key={i}>[doc: {p.title ?? p.sourceId}]</span>;
    default:
      // AI SDK v5+ spec: unknown part types are silently dropped so a
      // newer-server / older-client combo doesn't crash the UI. The
      // `drops unknown part types silently` test pins this behavior.
      return null;
  }
}

function renderMessage(m: UIMessage): ReactNode {
  return <div key={m.id}>{m.parts.map((p, i) => renderPart(p, i))}</div>;
}

function LiveChat() {
  // useChat is a hook — must be called unconditionally inside LiveChat.
  // LiveChat is only mounted when no initialMessages are provided, so
  // tests that seed messages never trigger a transport call.
  const { messages } = useChat({ transport: spikeTransport });
  return <div className="spike-stub">{messages.map(renderMessage)}</div>;
}

function SeededView({ messages }: { messages: UIMessage[] }) {
  return <div className="spike-stub">{messages.map(renderMessage)}</div>;
}

export default function App({ initialMessages }: AppProps) {
  if (initialMessages) {
    return <SeededView messages={initialMessages} />;
  }
  return <LiveChat />;
}
