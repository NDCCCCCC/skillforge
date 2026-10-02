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
import { useAppStore } from '../store/app-store';
import type { PendingApproval } from '../store/app-store';

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

  // Session-management phase — the x-session-id header is what binds a
  // chat turn to the FE's session partition on the BE (chat.go resolves
  // the header OR mints a random id). Without it the roster/history
  // endpoints never see the conversation.
  it('sends x-session-id from options.chatId on the POST', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      body: textBodyStream(sseFrame({ type: 'start', messageId: 'm1' })),
    } as unknown as Response));
    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    await drain(
      await transport.sendMessages({
        trigger: 'submit-message',
        chatId: 'session-42',
        messageId: 'm1',
        messages: [],
        abortSignal: undefined,
      }),
    );

    const init = (fetchMock.mock.calls[0] as unknown as [string, RequestInit])[1];
    const headers = init.headers as Record<string, string>;
    expect(headers['x-session-id']).toBe('session-42');
  });

  it('omits x-session-id when chatId is empty (no partition binding)', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      body: textBodyStream(sseFrame({ type: 'start', messageId: 'm1' })),
    } as unknown as Response));
    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    await drain(
      await transport.sendMessages({
        trigger: 'submit-message',
        chatId: '',
        messageId: 'm1',
        messages: [],
        abortSignal: undefined,
      }),
    );

    const init = (fetchMock.mock.calls[0] as unknown as [string, RequestInit])[1];
    const headers = init.headers as Record<string, string>;
    expect(headers['x-session-id']).toBeUndefined();
  });
});

// ----------------------------------------------------------------------------
// Plan 11-06 Task 5 — HITL approval-gate dispatch + D-04/D15 invariants
// ----------------------------------------------------------------------------

describe('RealChatTransport — approval-gate dispatch (Plan 11-06)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    useAppStore.setState((s) => ({
      approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null },
    }));
  });

  it('TestTransportSwapWithApprovalCard', async () => {
    // D-04 invariant, Phase 11 form: the approval-gate UI derives from
    // the store — the transport NEVER imports or renders it. Static
    // source check first (defense against accidental imports).
    const fs = await import('node:fs');
    const transportSource = fs.readFileSync('src/transport/RealChatTransport.ts', 'utf-8');
    expect(transportSource).not.toMatch(/ApprovalCard/);

    // Functional: an SSE stream carrying the 4 new decision strings
    // dispatches to the store's approval branch; those chunks are NOT
    // enqueued to useChat. A pre-existing decision string (route)
    // flows through verbatim — the mock↔Default byte-identity for
    // existing decisions is untouched.
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-route',
          data: { step: 1, decision: 'route', selected: 'simple-qa' },
        }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-int',
          data: {
            step: 0,
            decision: 'interrupted',
            approvalId: 'apr-sse-1',
            toolName: 'memory_facts_write',
            target: 'memory:facts',
            frozenArgs: { fact: 'sse interrupt' },
            reason: 'agent wants to write',
            skillVersion: 'v1',
            promptHash: 'ph-sse',
            sessionId: 'sess-sse',
            skillId: 'simple-qa',
          },
        }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-res',
          data: { step: 0, decision: 'tool-resumed', toolCallId: 'apr-sse-1' },
        }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-rule',
          data: {
            step: 0,
            decision: 'rule-matched',
            approvalId: 'apr-sse-2',
            target: 'memory:facts',
            toolName: 'memory_facts_write',
            frozenArgs: { fact: 'auto' },
            skillVersion: 'v1',
            promptHash: 'ph-sse',
          },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'sess-sse',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // (a) interrupted → pending was set… then tool-resumed resolved it.
    const approval = useAppStore.getState().approval;
    expect(approval.pending).toBeNull();
    expect(approval.terminal?.status).toBe('approved');
    expect(approval.terminal?.approvalId).toBe('apr-sse-1');
    // (b) history captured both the resolution and the rule auto-pass.
    expect(approval.history.map((h) => h.status)).toEqual(['approved', 'auto-passed']);
    expect(approval.history[1].reason).toBe('记忆规则命中');

    // (c) The 4 HITL data-decision chunks never reached useChat; the
    // pre-existing route decision DID (D-04 byte-identity preserved).
    const decisionChunks = chunks.filter((c) => c.type === 'data-decision');
    expect(decisionChunks.length).toBe(1);
    expect((decisionChunks[0] as { id?: string }).id).toBe('dec-route');
    expect(chunks.map((c) => c.type)).toEqual(['start', 'data-decision', 'finish']);
  });

  it('TestWireTypeCompilation', async () => {
    // D15 invariant: NO new SSE chunk types — only the data-decision
    // decision STRING union extended. Static source assertions.
    const fs = await import('node:fs');
    const transportSource = fs.readFileSync('src/transport/RealChatTransport.ts', 'utf-8');
    const wireTypesSource = fs.readFileSync('src/types/wire-types.ts', 'utf-8');

    // 2a) No `type: 'data-*'` literals beyond data-decision anywhere in
    // the transport (zero is expected — the dispatch compares via
    // wireChunk.type === 'data-decision', never constructs chunks).
    const dataTypeMatches = transportSource.match(/type:\s*'data-[a-z-]+'/g) || [];
    const uniqueDataTypes = new Set(
      dataTypeMatches.map((m) => m.replace(/^type:\s*'/, '').replace(/'$/, '')),
    );
    expect([...uniqueDataTypes].filter((t) => t !== 'data-decision')).toEqual([]);

    // 2b) The wire-types decision union carries the 4 new values.
    for (const d of ['interrupted', 'tool-resumed', 'tool-denied', 'rule-matched']) {
      expect(wireTypesSource).toContain(`'${d}'`);
    }

    // 2c) The pre-existing decision strings are still present
    // (additive extension — no breaking change).
    for (const d of ['retrieve', 'answer', 'refuse', 'forced_stop', 'route', 'counter_evidence']) {
      expect(wireTypesSource).toContain(`'${d}'`);
    }

    // 2d) The transport source handles exactly the 4 new strings.
    for (const d of ['interrupted', 'tool-resumed', 'tool-denied', 'rule-matched']) {
      expect(transportSource).toContain(`'${d}'`);
    }

    // 2e) tsc --noEmit (run in the Task 7 gate) covers the compile-time
    // half of D15 — the SkillForgeChunk union still satisfies
    // AssertAssignable<SkillForgeChunk, UIMessageChunk>.
  });

  it('resolvePending via SSE is a no-op for a stale approvalId (race safety)', async () => {
    // Seed a pending row; a tool-denied event for a DIFFERENT id must
    // not clear it (the store guard is the defense).
    const pending: PendingApproval = {
      approvalId: 'apr-live',
      toolName: 'memory_facts_write',
      target: 'memory:facts',
      frozenArgs: { fact: 'live' },
      reason: 'live',
      createdAt: Date.now(),
      expiresAt: null,
      skillVersion: 'v1',
      promptHash: 'ph-live',
      sessionId: 'sess-live',
      skillId: 'simple-qa',
      status: 'pending',
    };
    useAppStore.getState().approval.setPending(pending);

    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-stale',
          data: { step: 0, decision: 'tool-denied', toolCallId: 'apr-STALE', reason: 'stale event' },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });
    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'sess-live',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    const approval = useAppStore.getState().approval;
    expect(approval.pending?.approvalId).toBe('apr-live');
    expect(approval.terminal).toBeNull();
  });
});

// ----------------------------------------------------------------------------
// Plan 12-06 Task 4 — data-plan-trace dispatch + synthetic write_todos
// fallback (Pitfall 6). Authoritative state lives in the plan slice
// (D14-extension); useChat never sees the data-plan-trace chunk.
// ----------------------------------------------------------------------------

describe('RealChatTransport — plan-trace dispatch (Plan 12-06)', () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
    useAppStore.setState((s) => ({
      plan: { ...s.plan, planByRunId: {}, currentRunId: null },
    }));
  });

  it('TestDataPlanTraceChunkRouting', async () => {
    const todos = [
      { id: 't-1', content: '审核', status: 'pending' },
      { id: 't-2', content: '验证', status: 'in_progress' },
    ];
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-plan-trace',
          id: 'plan-1',
          data: { runId: 'run-abc', todos, ts: 1700000000000 },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // Plan slice received the trace.
    const planSlice = useAppStore.getState().plan;
    expect(planSlice.planByRunId['run-abc']).toBeDefined();
    expect(planSlice.planByRunId['run-abc'].todos).toHaveLength(2);
    expect(planSlice.planByRunId['run-abc'].todos[0].status).toBe('pending');
    expect(planSlice.planByRunId['run-abc'].todos[1].status).toBe('in_progress');
    expect(planSlice.planByRunId['run-abc'].generatedAt).toBe(1700000000000);
    expect(planSlice.planByRunId['run-abc'].status).toBe('streaming');
    expect(planSlice.currentRunId).toBe('run-abc');

    // D14 invariant: data-plan-trace is NOT routed to useChat (the chip
    // surfaces only what arrives via the plan slice, not via messages).
    expect(chunks.filter((c) => c.type === 'data-plan-trace')).toHaveLength(0);
  });

  it('TestSyntheticWriteTodosToolCallRouting', async () => {
    const todos = [
      { id: 't-1', content: '合成', status: 'completed' },
    ];
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'tool-input-available',
          toolCallId: 'tc-1',
          toolName: 'write_todos',
          input: { todos },
        }),
        sseFrame({
          type: 'tool-output-available',
          toolCallId: 'tc-1',
          output: { toolName: 'write_todos', output: { todos } },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-synth-1',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // The synthetic tool-output routes to the plan slice under the
    // "<chatId>-synthetic" runId convention.
    const syntheticRunId = 'chat-synth-1-synthetic';
    const planSlice = useAppStore.getState().plan;
    expect(planSlice.planByRunId[syntheticRunId]).toBeDefined();
    expect(planSlice.planByRunId[syntheticRunId].todos).toHaveLength(1);
    expect(planSlice.planByRunId[syntheticRunId].todos[0].status).toBe('completed');
  });

  it('TestTransportSwapWithPlanTracePart', async () => {
    // D-04 invariant: data-plan-trace routing must be transport-agnostic.
    // The runtime path (RealChatTransport) and the test path
    // (MockChatTransport) must both reach the same plan slice state for
    // identical payloads. We test the runtime side here (MockChatTransport
    // parity is asserted by the transport-swap.test.tsx suite).
    const todos = [{ id: 't-1', content: '验证传输', status: 'in_progress' }];
    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-plan-trace',
          id: 'plan-1',
          data: { runId: 'run-swap', todos, ts: 1700000000001 },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });

    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'chat-swap',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // UseChat sees the byte-identical stream EXCEPT for the dispatched
    // data-plan-trace chunk (consumed by the transport, never enqueued).
    expect(chunks.map((c) => c.type)).toEqual(['start', 'finish']);

    // The plan slice was mutated identically to what a MockChatTransport
    // consumer would observe for the same data-plan-trace payload.
    const slice = useAppStore.getState().plan;
    expect(slice.planByRunId['run-swap']).toBeDefined();
    expect(slice.planByRunId['run-swap'].todos[0].content).toBe('验证传输');
    expect(slice.currentRunId).toBe('run-swap');
  });
});

// ----------------------------------------------------------------------------
// Plan 12-07 Task 3 — data-decision enum extension (D16 overload pattern;
// D15 lockstep with 12-04 BE plan_api.go + 12-05 BE budget gate). The
// decision string widens by 3 values (forced_stop / plan-approved /
// plan-denied); the data-decision discriminator is unchanged.
// ----------------------------------------------------------------------------

describe('RealChatTransport — plan-decision dispatch (Plan 12-07)', () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
    useAppStore.setState((s) => ({
      approval: { ...s.approval, pending: null, terminal: null, history: [], resumeBanner: null },
      plan: { ...s.plan, planByRunId: {}, currentRunId: null },
    }));
  });

  it('TestForcedStopDataDecisionRouting', async () => {
    // Seed a plan so the slice has a row to markPlanStatus against.
    useAppStore.getState().plan.setPlan('run-fs', [
      { id: 't-1', content: 'x', status: 'pending' },
    ], 1700000000000);

    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-fs',
          data: { step: 0, decision: 'forced_stop', runId: 'run-fs' },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });
    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'run-fs',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // Plan slice marked forced_stop; data-decision chunk NOT enqueued.
    const slice = useAppStore.getState().plan;
    expect(slice.planByRunId['run-fs']?.status).toBe('forced_stop');
    expect(chunks.filter((c) => c.type === 'data-decision')).toHaveLength(0);
  });

  it('TestPlanApprovedDataDecisionRouting', async () => {
    // Seed a pending plan approval so resolvePending + appendHistory have a row.
    const pending = {
      approvalId: 'apr-plan-1',
      toolName: 'plan_approve',
      target: 'plan_approve',
      frozenArgs: { plan: { title: 'P' } },
      reason: 'want to execute',
      createdAt: Date.now(),
      expiresAt: null,
      skillVersion: 'v1',
      promptHash: 'ph',
      sessionId: 'sess-1',
      skillId: 'simple-qa',
      status: 'pending' as const,
    };
    useAppStore.getState().approval.setPending(pending);
    useAppStore.getState().plan.setPlan('run-pa', [
      { id: 't-1', content: 'x', status: 'pending' },
    ], 1700000000001);

    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-pa',
          data: {
            step: 0,
            decision: 'plan-approved',
            approvalId: 'apr-plan-1',
            toolCallId: 'apr-plan-1',
            runId: 'run-pa',
          },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });
    const chunks = await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'run-pa',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    // Approval slice: pending cleared, terminal='approved', history has one row.
    const ap = useAppStore.getState().approval;
    expect(ap.pending).toBeNull();
    expect(ap.terminal?.status).toBe('approved');
    expect(ap.terminal?.approvalId).toBe('apr-plan-1');
    expect(ap.history.map((h) => h.status)).toEqual(['approved']);

    // Plan slice: marked 'complete'.
    expect(useAppStore.getState().plan.planByRunId['run-pa']?.status).toBe('complete');

    // data-decision chunk NOT enqueued (D-04 + D15 — only pre-existing
    // decisions still flow through to useChat).
    expect(chunks.filter((c) => c.type === 'data-decision')).toHaveLength(0);
  });

  it('TestPlanDeniedDataDecisionRouting', async () => {
    useAppStore.getState().approval.setPending({
      approvalId: 'apr-plan-2',
      toolName: 'plan_approve',
      target: 'plan_approve',
      frozenArgs: { plan: { title: 'Q' } },
      reason: 'want to execute',
      createdAt: Date.now(),
      expiresAt: null,
      skillVersion: 'v1',
      promptHash: 'ph',
      sessionId: 'sess-2',
      skillId: 'simple-qa',
      status: 'pending',
    });
    useAppStore.getState().plan.setPlan('run-pd', [
      { id: 't-1', content: 'y', status: 'pending' },
    ], 1700000000002);

    const fetchMock = vi.fn(async () => {
      const body = [
        sseFrame({ type: 'start', messageId: 'msg-1' }),
        sseFrame({
          type: 'data-decision',
          id: 'dec-pd',
          data: {
            step: 0,
            decision: 'plan-denied',
            approvalId: 'apr-plan-2',
            toolCallId: 'apr-plan-2',
            runId: 'run-pd',
            reason: 'too risky',
          },
        }),
        sseFrame({ type: 'finish', finishReason: 'stop' }),
      ].join('');
      return new Response(textBodyStream(body), {
        status: 200,
        headers: { 'content-type': 'text/event-stream' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const transport = new RealChatTransport({
      api: '/api/chat',
      fetch: fetchMock as unknown as typeof globalThis.fetch,
    });
    await drain(await transport.sendMessages({
      trigger: 'submit-message',
      chatId: 'run-pd',
      messageId: 'msg-1',
      messages: [],
      abortSignal: undefined,
    }));

    const ap = useAppStore.getState().approval;
    expect(ap.pending).toBeNull();
    expect(ap.terminal?.status).toBe('denied');
    expect(ap.terminal?.reason).toBe('too risky');
    expect(useAppStore.getState().plan.planByRunId['run-pd']?.status).toBe('forced_stop');
  });
});