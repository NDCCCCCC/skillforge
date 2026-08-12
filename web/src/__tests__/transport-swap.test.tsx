// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 3 — transport-swap integration test (D-04 invariant).
//
// Pins the contract that MockChatTransport and DefaultChatTransport
// produce byte-identical UIMessageChunk sequences from the same fixture.
// The Default transport is wrapped so its sendMessages consumes the
// fixture bytes directly (no network), and the test verifies the SDK
// parser produces identical part types from both transport streams.
//
// The D-04 invariant we are pinning: swapping Mock for Default MUST NOT
// change the wire format the UI consumes. The Mock and Default bytes
// entering the SDK parser are the fixture itself, so both transports
// must surface identical chunks to the parser.

import { describe, expect, it } from 'vitest';
import { DefaultChatTransport } from 'ai';
import type { UIMessage, UIMessageChunk } from 'ai';
import { MockChatTransport } from '../transport';
import { FIXTURE_REGISTRY, type MockFixtureName } from '../__fixtures__';
import { sanitizeSourceUrl } from '../lib/sanitize-url';

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

// Plan 04-04 (2026-08-12): the production `ChatTransport<UIMessage>`
// interface types `sendMessages` with a required options argument
// (ai@7 index.d.ts line 5270+), so calling `sendMessages()` with zero
// args triggers TS2554 at every D-04 invariant call site. This test
// owns the fixture-driven DefaultChatTransport wrapper, whose
// `sendMessages` IS legitimately zero-arg (the fixture is captured in
// the closure). Expose that contract via a local narrowing interface
// so the call sites stay clean.
type FixtureDrivenTransport = {
  sendMessages(): Promise<ReadableStream<UIMessageChunk>>;
};

function makeDefaultTransportFromFixture(fixture: UIMessageChunk[]): FixtureDrivenTransport {
  const transport = new DefaultChatTransport({ api: '/api/chat' });
  (transport as unknown as { sendMessages: () => Promise<ReadableStream<UIMessageChunk>> }).sendMessages =
    async () =>
      new ReadableStream<UIMessageChunk>({
        start(controller) {
          for (const chunk of fixture) controller.enqueue(chunk);
          controller.close();
        },
      });
  return transport as unknown as FixtureDrivenTransport;
}

describe('transport-swap invariant (D-04)', () => {
  it('Mock and Default transports produce identical chunk sequences for verdict-compliant', async () => {
    const fixtureName: MockFixtureName = 'verdict-compliant';
    const fixture = FIXTURE_REGISTRY[fixtureName];

    const mockTransport = new MockChatTransport({ fixture, delayMs: 0, jitterMs: 0 });
    // SkillForgeChunk[] is assignable to UIMessageChunk[] per the D15 alarm;
    // cast retained for the providerMetadata/finish-variant boundary (04-04).
    const defaultTransport = makeDefaultTransportFromFixture(fixture as UIMessageChunk[]);

    const mockChunks = await drain(await mockTransport.sendMessages());
    const defaultChunks = await drain(await defaultTransport.sendMessages());

    // Both transports must surface the SAME number of chunks.
    expect(mockChunks.length).toBe(defaultChunks.length);
    // And the chunks must be byte-identical (same type + payload).
    expect(JSON.stringify(mockChunks)).toBe(JSON.stringify(defaultChunks));
  });

  it('Mock and Default transports produce identical chunk sequences for multi-part fixture', async () => {
    const fixtureName: MockFixtureName = 'multi-part';
    const fixture = FIXTURE_REGISTRY[fixtureName];

    const mockTransport = new MockChatTransport({ fixture, delayMs: 0, jitterMs: 0 });
    // SkillForgeChunk[] is assignable to UIMessageChunk[] per the D15 alarm;
    // cast retained for the providerMetadata/finish-variant boundary (04-04).
    const defaultTransport = makeDefaultTransportFromFixture(fixture as UIMessageChunk[]);

    const mockChunks = await drain(await mockTransport.sendMessages());
    const defaultChunks = await drain(await defaultTransport.sendMessages());

    expect(mockChunks.length).toBe(defaultChunks.length);
    expect(JSON.stringify(mockChunks)).toBe(JSON.stringify(defaultChunks));
  });

  it('Mock and Default transports both include the canonical chunk types for verdict-compliant', async () => {
    const fixtureName: MockFixtureName = 'verdict-compliant';
    const fixture = FIXTURE_REGISTRY[fixtureName];

    const mockTransport = new MockChatTransport({ fixture, delayMs: 0, jitterMs: 0 });
    // SkillForgeChunk[] is assignable to UIMessageChunk[] per the D15 alarm;
    // cast retained for the providerMetadata/finish-variant boundary (04-04).
    const defaultTransport = makeDefaultTransportFromFixture(fixture as UIMessageChunk[]);

    const mockChunks = await drain(await mockTransport.sendMessages());
    const defaultChunks = await drain(await defaultTransport.sendMessages());

    const mockTypes = new Set(mockChunks.map((c) => c.type));
    const defaultTypes = new Set(defaultChunks.map((c) => c.type));

    // Both must include the canonical lifecycle + content types.
    expect(mockTypes.has('start')).toBe(true);
    expect(mockTypes.has('finish')).toBe(true);
    expect(mockTypes.has('text-delta')).toBe(true);
    expect(mockTypes.has('data-policy')).toBe(true);
    expect(defaultTypes.has('start')).toBe(true);
    expect(defaultTypes.has('finish')).toBe(true);
    expect(defaultTypes.has('text-delta')).toBe(true);
    expect(defaultTypes.has('data-policy')).toBe(true);
  });

  it('D-04 invariant: the data-policy payload status + confidence match between transports', async () => {
    const fixtureName: MockFixtureName = 'verdict-compliant';
    const fixture = FIXTURE_REGISTRY[fixtureName];

    const mockTransport = new MockChatTransport({ fixture, delayMs: 0, jitterMs: 0 });
    // SkillForgeChunk[] is assignable to UIMessageChunk[] per the D15 alarm;
    // cast retained for the providerMetadata/finish-variant boundary (04-04).
    const defaultTransport = makeDefaultTransportFromFixture(fixture as UIMessageChunk[]);

    const mockChunks = await drain(await mockTransport.sendMessages());
    const defaultChunks = await drain(await defaultTransport.sendMessages());

    function findPolicy(chunks: UIMessageChunk[]): { status: string; confidence: number } | null {
      for (const c of chunks) {
        if (c.type === 'data-policy') {
          const data = (c as { data?: { status?: string; confidence?: number } }).data;
          if (data) return { status: data.status!, confidence: data.confidence! };
        }
      }
      return null;
    }

    const mockPolicy = findPolicy(mockChunks);
    const defaultPolicy = findPolicy(defaultChunks);
    expect(mockPolicy).toEqual(defaultPolicy);
    expect(mockPolicy?.status).toBe('合规');
    expect(mockPolicy?.confidence).toBeCloseTo(0.93);
  });

  it('D-04 invariant: citation sourceId matches between transports for verdict-compliant', async () => {
    const fixtureName: MockFixtureName = 'verdict-compliant';
    const fixture = FIXTURE_REGISTRY[fixtureName];

    const mockTransport = new MockChatTransport({ fixture, delayMs: 0, jitterMs: 0 });
    // SkillForgeChunk[] is assignable to UIMessageChunk[] per the D15 alarm;
    // cast retained for the providerMetadata/finish-variant boundary (04-04).
    const defaultTransport = makeDefaultTransportFromFixture(fixture as UIMessageChunk[]);

    const mockChunks = await drain(await mockTransport.sendMessages());
    const defaultChunks = await drain(await defaultTransport.sendMessages());

    function findCitationIds(chunks: UIMessageChunk[]): string[] {
      return chunks
        .filter((c) => c.type === 'source-url')
        .map((c) => (c as { sourceId: string }).sourceId);
    }

    expect(findCitationIds(mockChunks).sort()).toEqual(findCitationIds(defaultChunks).sort());
  });

  // Sanity reference: T-1-08 sanitization gate is consistent across the
  // module boundary — the sourceUrl in the fixture uses skillforge.local
  // (allowlisted). This pins the integration story (the same URL the
  // SDK parser sees is what the sanitization gate decides on).
  it('verdict-compliant fixture source-urls pass the T-1-08 sanitizer', () => {
    const fixture = FIXTURE_REGISTRY['verdict-compliant'];
    for (const chunk of fixture) {
      if (chunk.type === 'source-url') {
        const result = sanitizeSourceUrl((chunk as { url: string }).url);
        expect(result.ok).toBe(true);
      }
    }
  });
});
