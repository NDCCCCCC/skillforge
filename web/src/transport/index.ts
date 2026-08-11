// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Transport factory — explicit env-based selection between Mock and
// Default transport. Per CONTEXT D-03 (Mock Transport) and D-04 (single
// swap point for real wiring), this module is the ONLY file that knows
// about both transports; UI components consume only the returned
// ChatTransport<UIMessage> instance.
//
// Production wiring is a one-line swap: pass 'prod' env to get
// DefaultChatTransport against /api/chat (Phase 2 Hertz backend). Dev
// and test always get MockChatTransport against the named fixture set.

import { DefaultChatTransport, type ChatTransport, type UIMessage } from 'ai';
import { DEFAULT_FIXTURE, FIXTURE_REGISTRY, type MockFixtureName } from '../__fixtures__';
import { MockChatTransport } from './MockChatTransport';

export type TransportEnv = 'prod' | 'dev' | 'test';

export function createTransport(
  env: TransportEnv,
  fixtureName: MockFixtureName = DEFAULT_FIXTURE,
): ChatTransport<UIMessage> {
  if (env === 'prod') {
    return new DefaultChatTransport({ api: '/api/chat' });
  }
  // dev | test — both run Mock; tests pass 'test' to ensure Mock is the
  // only path used (per T-04-04 threat model mitigation).
  return new MockChatTransport({
    fixture: FIXTURE_REGISTRY[fixtureName],
    delayMs: 30,
    jitterMs: 10,
  });
}

export { MockChatTransport };
export type { MockFixtureName } from '../__fixtures__';
