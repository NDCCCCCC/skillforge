// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// 最小身份层（owner token）的行为契约 —— 后端按这个 token 过滤
// 会话列表，所以「同一个浏览器始终用同一个 token」是硬要求。

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  getOwnerToken,
  ownerTokenHeaders,
  OWNER_TOKEN_STORAGE_KEY,
} from './owner-token';

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  localStorage.clear();
});

describe('owner-token', () => {
  it('generates a token on first use and persists it under sf_owner_token', () => {
    const token = getOwnerToken();
    expect(token.length).toBeGreaterThan(0);
    expect(localStorage.getItem(OWNER_TOKEN_STORAGE_KEY)).toBe(token);
  });

  it('returns the SAME token on subsequent calls (stable identity)', () => {
    const first = getOwnerToken();
    const second = getOwnerToken();
    expect(second).toBe(first);
  });

  it('reuses the token persisted by a previous page load', () => {
    localStorage.setItem(OWNER_TOKEN_STORAGE_KEY, 'persisted-token');
    expect(getOwnerToken()).toBe('persisted-token');
  });

  it('exposes the token as the X-Owner-Token request header', () => {
    expect(ownerTokenHeaders()['X-Owner-Token']).toBe(getOwnerToken());
  });
});
