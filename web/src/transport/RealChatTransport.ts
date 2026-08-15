// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 260815-j4y Task 2 (GREEN) — RealChatTransport.
//
// Wraps the AI SDK v7 DefaultChatTransport for the prod (env='prod')
// path. The wrapper exists for ONE reason: AI SDK v7's UIMessageStream
// parser aborts on unknown chunk types (throws UIMessageStreamError
// when it sees `text-delta-draft`), and the backend (Phase 2 Hertz SSE)
// emits text-delta-draft / text-delta-draft-end frames for the draft
// channel before grounding verifies the LLM answer. MockChatTransport
// already filters those frames in its fixture loop; this wrapper does
// the same for the real SSE stream so the parser never sees them.
//
// Per-frame dispatch:
//   - text-delta-draft          → accumulate delta, emit onDraftUpdate('pending'), do NOT enqueue
//   - text-delta-draft-end      → emit onDraftUpdate('pending'), do NOT enqueue (status holds)
//   - data-policy with .status  → emit onDraftUpdate('verified'), enqueue (Verdict arrives)
//   - data-policy with .reason  → emit onDraftUpdate('refused', ''), enqueue (Refusal arrives)
//   - other types               → enqueue verbatim
//
// Draft buffer is LOCAL to each sendMessages() stream start() closure —
// matches MockChatTransport semantics, so each turn starts fresh (the
// chip clears on refusal is the test responsibility, not the transport's).
//
// Re-exports MockChatTransport's DraftStatus / DraftUpdate /
// DraftUpdateHandler / MockChatTransportDraftOptions so callers don't
// have to import from two places.

import type { ChatTransport, UIMessage, UIMessageChunk } from 'ai';
import {
  type DraftStatus,
  type DraftUpdate,
  type DraftUpdateHandler,
  type MockChatTransportDraftOptions,
} from './MockChatTransport';

export type { DraftStatus, DraftUpdate, DraftUpdateHandler, MockChatTransportDraftOptions };

/** Options accepted by RealChatTransport. */
export interface RealChatTransportOptions {
  /** Backend endpoint (defaults to /api/chat — set in createTransport). */
  api: string;
  /** Optional fetch override (tests inject a vi.fn() stub here). */
  fetch?: typeof globalThis.fetch;
  /** Extra request headers (merged into content-type: application/json). */
  headers?: Record<string, string>;
}

export class RealChatTransport implements ChatTransport<UIMessage> {
  private readonly opts: RealChatTransportOptions;
  private readonly onDraftUpdate: DraftUpdateHandler;

  constructor(opts: RealChatTransportOptions, draftOpts: MockChatTransportDraftOptions = {}) {
    this.opts = opts;
    this.onDraftUpdate = draftOpts.onDraftUpdate ?? (() => {});
  }

  async sendMessages(options: {
    trigger: 'submit-message' | 'regenerate-message';
    chatId: string;
    messageId: string | undefined;
    messages: UIMessage[];
    abortSignal: AbortSignal | undefined;
  } & Record<string, unknown>): Promise<ReadableStream<UIMessageChunk>> {
    const fetchFn = this.opts.fetch ?? globalThis.fetch;
    const api = this.opts.api;
    const extraHeaders = this.opts.headers;
    // Capture the handler in a local — inside the ReadableStream
    // `start()` callback, `this` is bound to the underlying source
    // object (NOT the RealChatTransport instance), so we cannot
    // reference `this.onDraftUpdate` from inside the stream loop.
    const onDraftUpdate = this.onDraftUpdate;

    // Issue the POST to the backend. The backend (Phase 2 Hertz SSE)
    // responds with `text/event-stream` frames encoded as JSON
    // UIMessageChunk payloads per the AI SDK v5 wire protocol.
    const response = await fetchFn(api, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        ...extraHeaders,
      },
      body: JSON.stringify({
        messages: options.messages,
        id: options.chatId,
        trigger: options.trigger,
      }),
      signal: options.abortSignal,
    });

    if (!response.ok || !response.body) {
      return new ReadableStream<UIMessageChunk>({
        start(controller) {
          controller.error(
            new Error(
              `RealChatTransport: ${api} returned ${response.status} (body=${response.body ? 'present' : 'missing'})`,
            ),
          );
        },
      });
    }

    // The stream is owned by this closure — draftBuffer / draftStatus
    // are LOCAL so each sendMessages() invocation starts fresh (matches
    // MockChatTransport semantics; the chip clears on refusal is the
    // UI's responsibility, not the transport's).
    return new ReadableStream<UIMessageChunk>({
      async start(controller) {
        let draftBuffer = '';
        let draftStatus: DraftStatus = 'pending';

        const emitDraft = (next: DraftStatus, text: string) => {
          draftStatus = next;
          draftBuffer = text;
          onDraftUpdate({ status: next, draftText: text });
        };

        try {
          const reader = response.body!.getReader();
          const decoder = new TextDecoder();
          let pendingText = '';

          // SSE framing: `data: <json>\n\n` per event. Each ReadableStream
          // chunk may carry a partial frame (the fetch body is NOT
          // pre-splitted), so we accumulate into pendingText and split
          // on `\n\n` boundaries. Multi-line `data:` (lines starting
          // with `data:`) are joined with `\n` per the SSE spec.
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            pendingText += decoder.decode(value, { stream: true });

            // Split on the blank-line frame terminator. Each frame may
            // have multiple `data:` lines; join them with `\n` per spec.
            let frameEnd: number;
            while ((frameEnd = pendingText.indexOf('\n\n')) !== -1) {
              const frame = pendingText.slice(0, frameEnd);
              pendingText = pendingText.slice(frameEnd + 2);

              const dataLines: string[] = [];
              for (const line of frame.split('\n')) {
                if (line.startsWith('data:')) {
                  dataLines.push(line.slice(5).trimStart());
                }
                // Ignore `event:`, `id:`, `retry:` etc — the AI SDK
                // wire format only uses `data:` for chunk payloads.
              }
              if (dataLines.length === 0) continue;
              const payload = dataLines.join('\n');

              let chunk: UIMessageChunk;
              try {
                chunk = JSON.parse(payload) as UIMessageChunk;
              } catch {
                // Malformed frame — skip silently (matches
                // DirectChatTransport behavior: don't crash the stream
                // on a single bad frame; the next frame will likely
                // resync).
                continue;
              }

              // SkillForge wire extensions (text-delta-draft /
              // text-delta-draft-end) are NOT part of the upstream
              // AI SDK v7 UIMessageChunk union — so the structural
              // comparison must happen against the un-narrowed wire
              // shape. Cast through `unknown` to a structural view
              // for the discriminator check.
              const wireChunk = chunk as unknown as { type: string; delta?: string };

              // Draft channel — keep buffer up to date but never enqueue
              // the draft frame to useChat (which would route it into
              // message.parts[].text via the text-delta parser).
              if (wireChunk.type === 'text-delta-draft') {
                const delta = wireChunk.delta ?? '';
                emitDraft('pending', draftBuffer + delta);
                continue;
              }
              if (wireChunk.type === 'text-delta-draft-end') {
                // Status holds 'pending' until data-policy decides.
                emitDraft(draftStatus, draftBuffer);
                continue;
              }

              // data-policy is the trust-bearing transition: it carries
              // either a Verdict (verified → chip turns ✅ 已验证) or a
              // Refusal (refused → chip unmounts, RefusalCard takes over).
              if (wireChunk.type === 'data-policy') {
                const data = (chunk as { data?: { status?: string; reason?: string } }).data;
                if (data && typeof data === 'object') {
                  if (typeof data.status === 'string') {
                    emitDraft('verified', draftBuffer);
                  } else if (typeof data.reason === 'string') {
                    emitDraft('refused', '');
                  }
                }
              }

              // Plan 260815-k8d — text-end is the terminal signal for the
              // conclusion text stream. The draft channel's own
              // text-delta-draft-end was intercepted above (continue),
              // so this branch only fires for the FINAL answer text-end.
              // Emit ('idle', '') so RightChat unmounts the
              // DraftStreamChip — without this the chip stays pinned at
              // "✅ 已验证" forever after every successful verdict.
              if (wireChunk.type === 'text-end') {
                emitDraft('idle', '');
              }

              controller.enqueue(chunk);
            }
          }

          controller.close();
        } catch (err) {
          controller.error(err);
        }
      },
      cancel() {
        // Stream cancelled by consumer (e.g. useChat.status === 'submitted'
        // but user cancels). The fetch body will close automatically
        // when the reader is released.
      },
    });
  }

  async reconnectToStream(_options: {
    chatId: string;
    abortSignal?: AbortSignal;
  } & Record<string, unknown>): Promise<ReadableStream<UIMessageChunk> | null> {
    // No persistent server-side stream handle in v1; matches
    // MockChatTransport and DirectChatTransport semantics.
    return null;
  }
}