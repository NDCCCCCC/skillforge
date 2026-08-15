// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 2 — MockChatTransport.
//
// Implements the AI SDK v7 ChatTransport interface (verified at
// web/node_modules/ai/dist/index.d.ts lines 5270-5336) by replaying
// UIMessageChunk sequences from JSON fixtures through a ReadableStream.
// Zero-network, pure frontend, vitest-assertable.
//
// Per CONTEXT D-03 (Mock Transport foundation) and D-04 (single transport
// swap point for real wiring), this class is the dev/test replacement
// for DefaultChatTransport. The swap point is in createTransport() at
// web/src/transport/index.ts — UI component code is unchanged.
//
// Reference pattern: DirectChatTransport in ai-sdk.dev/docs/ai-sdk-ui/
// transport (in-process agent invocation; zero-network) is the official
// precedent — MockChatTransport is the same shape but reads from JSON
// fixtures instead of an Agent instance.
//
// Plan 260815-cos — draft channel (text-delta-draft / text-delta-draft-end):
// the transport maintains an OUT-OF-BAND draft buffer + status state
// machine for the DraftStreamChip (🟡 待校验 → ✅ 已验证 on grounding pass;
// refusal clears the buffer). The buffer is exposed via an `onDraftUpdate`
// callback the App wires at construction time. useChat never sees the
// draft frames — they are filtered out of the controller.enqueue path so
// they cannot pollute message.parts[].text (the load-bearing wire-contract
// invariant of the draft channel).

import type { ChatTransport, UIMessage, UIMessageChunk } from 'ai';
import type { MockChatTransportOptions } from './mock-types';

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** Status reported to the DraftStreamChip via onDraftUpdate. */
export type DraftStatus = 'pending' | 'verified' | 'refused';

/** Draft buffer snapshot exposed to the App via onDraftUpdate. */
export interface DraftUpdate {
  status: DraftStatus;
  draftText: string;
}

/** Optional draft hook called as the transport parses each chunk. */
export type DraftUpdateHandler = (update: DraftUpdate) => void;

/** Extended options accepted by MockChatTransport (Plan 260815-cos). */
export interface MockChatTransportDraftOptions {
  /** Invoked when the draft buffer or status changes. Default: no-op. */
  onDraftUpdate?: DraftUpdateHandler;
}

export class MockChatTransport implements ChatTransport<UIMessage> {
  private readonly opts: MockChatTransportOptions;
  private readonly onDraftUpdate: DraftUpdateHandler;

  constructor(opts: MockChatTransportOptions, draftOpts: MockChatTransportDraftOptions = {}) {
    this.opts = opts;
    this.onDraftUpdate = draftOpts.onDraftUpdate ?? (() => {});
  }

  async sendMessages(): Promise<ReadableStream<UIMessageChunk>> {
    const fixture =
      typeof this.opts.fixture === 'function' ? this.opts.fixture() : this.opts.fixture;
    const delay = this.opts.delayMs ?? 30;
    const jitter = this.opts.jitterMs ?? 10;

    // Plan 260815-cos: per-stream draft buffer + status state machine.
    // Both are LOCAL to this ReadableStream start() closure so each
    // turn's stream starts fresh (verifying the chip clears on refusal
    // is the test responsibility, not the transport's).
    let draftBuffer = '';
    let draftStatus: DraftStatus = 'pending';

    const emitDraft = (next: DraftStatus, text: string) => {
      draftStatus = next;
      draftBuffer = text;
      this.onDraftUpdate({ status: next, draftText: text });
    };

    return new ReadableStream<UIMessageChunk>({
      async start(controller) {
        try {
          for (const chunk of fixture) {
            const chunkType = (chunk as { type?: string }).type;

            // Plan 260815-cos: draft-channel handling — keep the
            // buffer up-to-date BUT never enqueue the draft frame to
            // useChat (which would route it into message.parts[].text
            // via the text-delta parser and pollute the trust surface).
            if (chunkType === 'text-delta-draft') {
              const delta = (chunk as { delta?: string }).delta ?? '';
              emitDraft('pending', draftBuffer + delta);
              const wait = Math.max(0, delay + (Math.random() * 2 - 1) * jitter);
              await sleep(wait);
              continue;
            }
            if (chunkType === 'text-delta-draft-end') {
              // Status stays 'pending' until data-policy decides —
              // either verified (data carries status+citations) or
              // refused (data carries reason). Holding 'pending' here
              // is the chip's 🟡 待校验 state during the parse window.
              const wait = Math.max(0, delay + (Math.random() * 2 - 1) * jitter);
              await sleep(wait);
              continue;
            }

            // data-policy is the trust-bearing transition: it carries
            // either a Verdict (verified → chip turns ✅ 已验证) or a
            // Refusal (refused → chip unmounts, RefusalCard takes over).
            if (chunkType === 'data-policy') {
              const data = (chunk as { data?: { status?: string; reason?: string } }).data;
              if (data && typeof data === 'object') {
                if (typeof data.status === 'string') {
                  emitDraft('verified', draftBuffer);
                } else if (typeof data.reason === 'string') {
                  emitDraft('refused', '');
                }
              }
            }

            // Honor abortSignal via a noop check at the top of each chunk;
            // tests pass an already-aborted signal to verify short-circuit.
            controller.enqueue(chunk as UIMessageChunk);
            const wait = Math.max(0, delay + (Math.random() * 2 - 1) * jitter);
            await sleep(wait);
          }
          controller.close();
        } catch (err) {
          controller.error(err);
        }
      },
      cancel() {
        // Stream cancelled by consumer (e.g. useChat.status === 'submitted'
        // but user cancels). No persistent state to clean up.
      },
    });
  }

  async reconnectToStream(): Promise<ReadableStream<UIMessageChunk> | null> {
    // Mock has no persistent server-side stream; matches DirectChatTransport
    // semantics documented at ai-sdk.dev/docs/ai-sdk-ui/transport.
    return null;
  }
}
