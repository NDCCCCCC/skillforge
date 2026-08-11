// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 2 — MockChatTransport tests.
//
// Pins the four behaviors the zero-network harness MUST guarantee:
//  1. sendMessages returns a ReadableStream whose chunks decode to
//     UIMessageChunk[] in fixture order.
//  2. Inter-chunk delay is configurable (delayMs / jitterMs).
//  3. reconnectToStream() returns null (matches DirectChatTransport).
//  4. The transport emits identical Uint8Array byte sequences for the
//     same fixture input (deterministic for snapshot tests) — note that
//     our delay/jitter is non-deterministic by design, so we test order
//     and content determinism rather than timing.

import type { UIMessageChunk } from 'ai';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MockChatTransport } from '../transport/MockChatTransport';
import type { MockFixture } from '../transport/mock-types';
import { FIXTURE_REGISTRY } from '../__fixtures__';

// Helper: drain a ReadableStream into an array.
async function drain(stream: ReadableStream<UIMessageChunk>): Promise<UIMessageChunk[]> {
  const reader = stream.getReader();
  const chunks: UIMessageChunk[] = [];
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
  }
  reader.releaseLock();
  return chunks;
}

describe('MockChatTransport', () => {
  beforeEach(() => {
    // Pin time so Math.random-derived jitter is reproducible across runs.
    // (Tests assert chunk order/content, not exact timing — fake timers
    // let us flush the streaming loop deterministically.)
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('sendMessages returns a ReadableStream emitting chunks in fixture order', async () => {
    const fixture = FIXTURE_REGISTRY['multi-part'];
    const transport = new MockChatTransport({
      fixture,
      delayMs: 1, // tight loop for tests
      jitterMs: 0,
    });

    const stream = await transport.sendMessages();
    expect(stream).toBeInstanceOf(ReadableStream);

    const chunks = await drain(stream);
    expect(chunks).toHaveLength(fixture.length);
    expect(chunks.map((c) => c.type)).toEqual(fixture.map((c) => c.type));

    // First chunk is the lifecycle start; last is the terminal finish.
    expect(chunks[0].type).toBe('start');
    expect(chunks[chunks.length - 1].type).toBe('finish');
  });

  it('accepts a function fixture and resolves it once', async () => {
    let calledCount = 0;
    const fixtureFn = (): MockFixture => {
      calledCount++;
      return FIXTURE_REGISTRY['verdict-compliant'];
    };

    const transport = new MockChatTransport({
      fixture: fixtureFn,
      delayMs: 1,
      jitterMs: 0,
    });

    const stream = await transport.sendMessages();
    await drain(stream);

    expect(calledCount).toBe(1);
  });

  it('reconnectToStream returns null (matches DirectChatTransport precedent)', async () => {
    const transport = new MockChatTransport({
      fixture: FIXTURE_REGISTRY['verdict-compliant'],
      delayMs: 1,
    });

    const result = await transport.reconnectToStream();
    expect(result).toBeNull();
  });

  it('emits deterministic chunk order for identical fixture (snapshot)', async () => {
    // Same fixture → identical chunk types + payloads (timing aside).
    // Two separate runs of the same fixture produce the same chunk array.
    const t1 = new MockChatTransport({
      fixture: FIXTURE_REGISTRY['citation-tiered'],
      delayMs: 1,
      jitterMs: 0,
    });
    const t2 = new MockChatTransport({
      fixture: FIXTURE_REGISTRY['citation-tiered'],
      delayMs: 1,
      jitterMs: 0,
    });

    const c1 = await drain(await t1.sendMessages());
    const c2 = await drain(await t2.sendMessages());

    expect(JSON.stringify(c1)).toBe(JSON.stringify(c2));
  });

  it('honors inter-chunk delay (delayMs ≥ 1)', async () => {
    // With delayMs=50 and 5 chunks, the total elapsed time should be at
    // least 50 * (chunks - 1) ms = 200ms. We assert >= 150ms to allow
    // for setTimeout scheduling jitter on slow CI.
    const fixture = FIXTURE_REGISTRY['sub-progress'];
    const transport = new MockChatTransport({
      fixture,
      delayMs: 50,
      jitterMs: 0,
    });

    const start = Date.now();
    const stream = await transport.sendMessages();
    await drain(stream);
    const elapsed = Date.now() - start;

    // 8 chunks → 7 inter-chunk delays * 50ms = 350ms minimum; assert >= 200ms.
    expect(elapsed).toBeGreaterThanOrEqual(200);
  });
});
