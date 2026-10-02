// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Mock transport type aliases. SkillForgeChunk[] is the fixture shape
// consumed by MockChatTransport; MockFixtureName is the registry key.
import type { SkillForgeChunk } from '../types/wire-types';
import type { MockFixtureName } from '../__fixtures__';

export type MockFixture = SkillForgeChunk[];

export type { MockFixtureName };

/** Options accepted by MockChatTransport. */
export interface MockChatTransportOptions {
  fixture: MockFixture | (() => MockFixture);
  /** Inter-chunk delay in ms (default 30ms). */
  delayMs?: number;
  /** Random +/- jitter in ms (default 10ms). */
  jitterMs?: number;
}
