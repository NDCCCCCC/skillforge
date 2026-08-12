// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-01 Task 1 — wire-contract drift detection (Pitfalls 1-3).
//
// Per .planning/phases/04-streaming-trust-ui/04-RESEARCH.md §Common
// Pitfalls, the spike-era wire contract has THREE drift defects against
// the installed AI SDK v7 parser that will silently swallow data if P4
// ships without correction. This test pins all three as hard acceptance
// criteria:
//   1. Terminal chunk uses type:"finish" (NOT "finish-message")
//   2. data-policy chunks carry payload on .data (NOT .policy)
//   3. Tool parts use tool-${name} or dynamic-tool (NOT "tool-invocation")
//
// If the AI SDK upstream changes the wire format, this test fails
// before any UI code runs. Fixtures violating any of these rules cause
// the SDK to silently drop them — no compile error, no runtime warning.

import type { UIMessageChunk } from 'ai';
import { describe, expect, it } from 'vitest';
import type { SkillForgeChunk } from '../types/wire-types';
import { FIXTURE_REGISTRY } from '../__fixtures__';

describe('wire contract drift detection (PITFALLS 1-3)', () => {
  // Pitfall 1 + type-level SDK alignment — SkillForgeChunk must be
  // assignable to UIMessageChunk (the parser's discriminated union).
  // If a fixture / TS type emits an unrecognized discriminator, the
  // SDK will silently drop it.
  it('SkillForgeChunk is assignable to UIMessageChunk (SDK recognizes all types)', () => {
    const chunk: SkillForgeChunk = { type: 'finish', finishReason: 'stop' };
    const _sdk: UIMessageChunk = chunk;
    expect(_sdk).toBeDefined();
  });

  // Pitfall 1 — terminal chunk. SDK parser has case "finish" (line 7369
  // of ai/dist/index.js) and NO "finish-message" arm. A fixture ending
  // in finish-message leaves useChat.status streaming forever.
  it('every fixture ends with type: "finish" (NOT "finish-message")', () => {
    for (const [name, fixture] of Object.entries(FIXTURE_REGISTRY)) {
      expect(fixture.length, `${name}: fixture must not be empty`).toBeGreaterThan(0);
      const last = fixture[fixture.length - 1];
      expect(last.type, `${name}: terminal chunk must be "finish", got "${last.type}"`).toBe('finish');
    }
  });

  // Pitfall 2 — Verdict payload lives on .data (per DataUIMessageChunk
  // shape, index.d.ts lines 2267-2273). Fixtures using .policy pass
  // through the parser but the payload lands on .data === undefined.
  it('every data-policy chunk carries Verdict on .data (NOT .policy)', () => {
    for (const [name, fixture] of Object.entries(FIXTURE_REGISTRY)) {
      for (const chunk of fixture) {
        if (chunk.type === 'data-policy') {
          const dataChunk = chunk as { type: string; data?: unknown; policy?: unknown };
          expect(
            dataChunk.data,
            `${name}: data-policy chunk must have .data defined (Pitfall 2)`,
          ).toBeDefined();
          expect(
            dataChunk.policy,
            `${name}: data-policy chunk must NOT have .policy (Pitfall 2 — must be .data)`,
          ).toBeUndefined();
        }
      }
    }
  });

  // Pitfall 3 — Tool parts in AI SDK v7 are typed `tool-${name}` or
  // `dynamic-tool`. "tool-invocation" was the legacy v5 discriminator
  // and does not exist in v7; code switching on it falls through to
  // the unknown-type silent-drop default.
  //
  // Plan 04-04 (2026-08-12): the `chunk.type === 'finish-message'` /
  // `'tool-invocation'` comparisons are now widened via a string cast
  // (`const t = chunk.type as string;`) to eliminate TS2367 — TS knows
  // the SkillForgeChunk discriminator union has no overlap with these
  // stale literals, but the runtime guard's intent is precisely to
  // catch a stale fixture that would silently break the SDK parser.
  // The cast preserves the semantic guard without weakening it.
  it('no fixture contains type: "finish-message" or type: "tool-invocation" anywhere', () => {
    for (const [name, fixture] of Object.entries(FIXTURE_REGISTRY)) {
      for (const chunk of fixture) {
        const t = chunk.type as string;
        expect(
          t === 'finish-message',
          `${name}: chunk must NOT use "finish-message" (Pitfall 1 — use "finish")`,
        ).toBe(false);
        expect(
          t === 'tool-invocation',
          `${name}: chunk must NOT use "tool-invocation" (Pitfall 3 — use "tool-${name}")`,
        ).toBe(false);
      }
    }
  });

  // Lifecycle — every fixture starts with type: "start". The SDK parser
  // expects `start` (NOT `start-message`) to initialize a new message.
  it('every fixture starts with type: "start"', () => {
    for (const [name, fixture] of Object.entries(FIXTURE_REGISTRY)) {
      expect(
        fixture[0].type,
        `${name}: first chunk must be "start"`,
      ).toBe('start');
    }
  });
});
