// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Transport factory — explicit env-based selection between Mock and
// Default transport. Per CONTEXT D-03 (Mock Transport foundation) and
// D-04 (single transport swap point for real wiring), this module is
// the ONLY file that knows about both transports; UI components
// consume only the returned ChatTransport<UIMessage> instance.
//
// Production wiring is a one-line swap: pass 'prod' env to get
// DefaultChatTransport against /api/chat (Phase 2 Hertz backend). Dev
// and test always get MockChatTransport against the named fixture set.
//
// Plan 260815-cos — DraftStreamChip wiring: when env !== 'prod', the
// factory forwards an `onDraftUpdate` hook to MockChatTransport so the
// chip mirrors the draft buffer + status in real time. For prod
// (DefaultChatTransport) the same hook can be attached by wrapping
// the transport's `sendMessages` return; the factory keeps the dev/
// test path simple and the prod path pristine (no draft buffering
// needed when the AI SDK parser owns the stream shape).

import { type ChatTransport, type UIMessage } from 'ai';
import { DEFAULT_FIXTURE, FIXTURE_REGISTRY, type MockFixtureName } from '../__fixtures__';
import { MockChatTransport, type DraftUpdateHandler } from './MockChatTransport';
import { RealChatTransport } from './RealChatTransport';

export type TransportEnv = 'prod' | 'dev' | 'test';

export interface CreateTransportOptions {
  /** Plan 260815-cos + 260815-j4y — draft channel state mirror. */
  onDraftUpdate?: DraftUpdateHandler;
}

export function createTransport(
  env: TransportEnv,
  fixtureName: MockFixtureName = DEFAULT_FIXTURE,
  opts: CreateTransportOptions = {},
): ChatTransport<UIMessage> {
  if (env === 'prod') {
    // Plan 260815-j4y: RealChatTransport wraps DefaultChatTransport and
    // filters `text-delta-draft` / `text-delta-draft-end` SSE frames at
    // the SSE→UIMessageChunk layer (defaulting to onDraftUpdate) so the
    // AI SDK v7 parser never sees them and aborts the stream with
    // UIMessageStreamError. See RealChatTransport.ts for the dispatcher.
    return new RealChatTransport(
      { api: '/api/chat' },
      { onDraftUpdate: opts.onDraftUpdate },
    );
  }
  // dev | test — both run Mock; tests pass 'test' to ensure Mock is the
  // only path used (per T-04-04 threat model mitigation).
  return new MockChatTransport(
    {
      fixture: FIXTURE_REGISTRY[fixtureName],
      delayMs: 30,
      jitterMs: 10,
    },
    { onDraftUpdate: opts.onDraftUpdate },
  );
}

export { MockChatTransport, RealChatTransport };
export type { MockFixtureName } from '../__fixtures__';