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

import type { ChatTransport, UIMessage, UIMessageChunk } from 'ai';
import type { MockChatTransportOptions } from './mock-types';

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

export class MockChatTransport implements ChatTransport<UIMessage> {
  private readonly opts: MockChatTransportOptions;

  constructor(opts: MockChatTransportOptions) {
    this.opts = opts;
  }

  async sendMessages(): Promise<ReadableStream<UIMessageChunk>> {
    const fixture =
      typeof this.opts.fixture === 'function' ? this.opts.fixture() : this.opts.fixture;
    const delay = this.opts.delayMs ?? 30;
    const jitter = this.opts.jitterMs ?? 10;

    return new ReadableStream<UIMessageChunk>({
      async start(controller) {
        try {
          for (const chunk of fixture) {
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
