// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 260815-j4y Task 1 (RED) — RealChatTransport unit tests.
//
// These tests pin the must-haves of the real-backend SSE→UIMessageChunk
// transport wrapper that fixes the AI SDK v7 parser abort caused by
// unrecognized `text-delta-draft` chunks:
//
//   1. drain of the returned ReadableStream must NOT contain draft-* chunks
//      (useChat must never see them — they go to onDraftUpdate instead)
//   2. onDraftUpdate must be invoked in the correct order with the correct
//      DraftStatus transitions:
//        - text-delta-draft accumulates delta → 'pending'
//        - text-delta-draft-end keeps status 'pending'
//        - data-policy {status} → 'verified'
//        - data-policy {reason} → 'refused' (clears buffer)
//   3. reconnectToStream must return null (parity with MockChatTransport)
//
// The test does NOT spin up a backend; fetch is stubbed globally via
// vi.stubGlobal to return a Response wrapping a hand-crafted SSE body.
// SSE encoding: `data: <json>\n\n` per frame (the standard AI SDK v5
// DefaultChatTransport shape). Multi-line data uses `\n` join.
//
// Note: chunk literals are cast through `unknown` because the test must
// emit wire frames that are a SUPERSET of UIMessageChunk (text-delta-
// draft is a SkillForge-extension discriminator not in upstream AI SDK
// v7). The SSE parser in RealChatTransport re-narrows via the runtime
// `chunk.type === ...` discriminator.

import { afterEach, describe, expect, it, vi } from 'vitest';
import type { UIMessageChunk } from 'ai';
import { RealChatTransport, type DraftUpdate } from './RealChatTransport';

/** Loose wire-shape accepted by the SSE serializer — drafts included. */
type WireChunk = Record<string, unknown> & { type: string };

/** SSE frame helper — JSON.stringify the chunk and wrap as `data: ...\n\n`. */
function sseFrame(chunk: WireChunk): string {
  return `data: ${JSON.stringify(chunk)}\n\n`;
}

/** Build a ReadableStream<Uint8Array> from a text body (utf-8 encoded). */
function textBodyStream(text: string): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(text));
      controller.close();
    },
  });
}

/** Drain a ReadableStream<UIMessageChunk> to an array. */
async function drain(stream: ReadableStream<UIMessageChunk>): Promise<UIMessageChunk[]> {
  const reader = stream.getReader();
  const out: UIMessageChunk[] = [];
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    out.push(value);
  }
  reader.releaseLock();
  return out;
}

describe('RealChatTransport (Plan 260815-j4y)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('filters draft-* chunks from the ReadableStream surface (useChat never sees them)', async () => {
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({ type: 'text-start', id: 'msg-1' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'Hello ' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'world' }),
        sseFrame({ type: 'text-delta-draft-end', id: 'draft-1' }),
        sseFrame({ type: 'text-delta', id: 'msg-1', delta: 'final answer' }),
        sseFrame({ type: 'text-end', id: 'msg-1' }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const updates: DraftUpdate[] = [];
    const transport = new RealChatTransport(
      { api: '/api/chat', fetch: fetchMock as unknown as typeof globalThis.fetch },
      {
        onDraftUpdate: (u) => updates.push(u),
      },
    );

    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // No draft-* chunks reach the consumer (useChat).
    const types = chunks.map((c) => c.type);
    expect(types).not.toContain('text-delta-draft');
    expect(types).not.toContain('text-delta-draft-end');
    // But the non-draft chunks DO reach the consumer (start/text/text-end/finish).
    expect(types).toEqual([
      'start',
      'text-start',
      'text-delta',
      'text-end',
      'finish',
    ]);
  });

  it('routes onDraftUpdate in correct order: pending → verified (data-policy with status)', async () => {
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'Some ' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'answer ' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'text' }),
        sseFrame({ type: 'text-delta-draft-end', id: 'draft-1' }),
        sseFrame({
          type: 'data-policy',
          id: 'pol-1',
          data: { status: 'verified', confidence: 0.91, citations: [] },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const updates: DraftUpdate[] = [];
    const transport = new RealChatTransport(
      { api: '/api/chat', fetch: fetchMock as unknown as typeof globalThis.fetch },
      { onDraftUpdate: (u) => updates.push(u) },
    );

    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // 3 draft deltas → 3 'pending' emits, then draft-end (still pending),
    // then data-policy status → 1 'verified' emit.
    expect(updates).toEqual([
      { status: 'pending', draftText: 'Some ' },
      { status: 'pending', draftText: 'Some answer ' },
      { status: 'pending', draftText: 'Some answer text' },
      { status: 'pending', draftText: 'Some answer text' }, // draft-end holds pending
      { status: 'verified', draftText: 'Some answer text' },
    ]);
  });

  it('routes onDraftUpdate: pending → refused (data-policy with reason)', async () => {
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'leak' }),
        sseFrame({ type: 'text-delta-draft-end', id: 'draft-1' }),
        sseFrame({
          type: 'data-policy',
          id: 'pol-1',
          data: { reason: 'insufficient_evidence' },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const updates: DraftUpdate[] = [];
    const transport = new RealChatTransport(
      { api: '/api/chat', fetch: fetchMock as unknown as typeof globalThis.fetch },
      { onDraftUpdate: (u) => updates.push(u) },
    );

    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    expect(updates).toEqual([
      { status: 'pending', draftText: 'leak' },
      { status: 'pending', draftText: 'leak' }, // draft-end holds pending
      { status: 'refused', draftText: '' },     // refusal clears buffer
    ]);
  });

  it('routes onDraftUpdate terminal: verified → idle on text-end (unmounts DraftStreamChip)', async () => {
    // Plan 260815-k8d — full SSE sequence through the verified terminal
    // state must finish with an emitDraft('idle', '') so RightChat
    // unmounts the DraftStreamChip. Without this final idle emit the
    // chip stays pinned at "✅ 已验证" forever (the chip is only
    // rendered when draftStatus !== 'idle').
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({ type: 'text-start', id: 'msg-1' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'hi ' }),
        sseFrame({ type: 'text-delta-draft', id: 'draft-1', delta: 'there' }),
        sseFrame({ type: 'text-delta-draft-end', id: 'draft-1' }),
        sseFrame({
          type: 'data-policy',
          id: 'pol-1',
          data: { status: 'verified', confidence: 0.91, citations: [] },
        }),
        sseFrame({ type: 'text-delta', id: 'msg-1', delta: 'final answer' }),
        sseFrame({ type: 'text-end', id: 'msg-1' }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const updates: DraftUpdate[] = [];
    const transport = new RealChatTransport(
      { api: '/api/chat', fetch: fetchMock as unknown as typeof globalThis.fetch },
      { onDraftUpdate: (u) => updates.push(u) },
    );

    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // 3 pending emits (2 draft deltas + 1 draft-end) + 1 verified + 1 idle = 5.
    expect(updates.length).toBeGreaterThanOrEqual(5);
    // The LAST emit must be the idle reset — that's what unmounts the
    // DraftStreamChip. Without the fix (no idle on text-end), this is
    // {status: 'verified', draftText: 'hi there'} and the chip hangs.
    expect(updates[updates.length - 1]).toEqual({
      status: 'idle',
      draftText: '',
    });
    // The penultimate emit must be the verified state — order matters.
    expect(updates[updates.length - 2]).toEqual({
      status: 'verified',
      draftText: 'hi there',
    });
  });

  it('reconnectToStream returns null (parity with MockChatTransport)', async () => {
    const transport = new RealChatTransport({ api: '/api/chat' });
    const result = await transport.reconnectToStream({
      chatId: 'chat-1',
    });
    expect(result).toBeNull();
  });
});