// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-03 Task 3 — T-1-08 sanitization acceptance gate re-run.
//
// Pins the 7+ acceptance assertions for the source-URL sanitization
// gate per 04-UI-SPEC §FEUI-03 (T-1-08 hard contract):
//   1. http://skillforge.local/foo is allowed
//   2. http://localhost/foo is allowed
//   3. https://127.0.0.1/foo is allowed
//   4. javascript:alert(1) is rejected as blocked_protocol (no hostname)
//   5. http://malicious.example.com/foo is rejected as blocked_hostname
//   6. 'not a url' is rejected as invalid_url
//   7. file:///etc/passwd is rejected as blocked_protocol
import { describe, expect, it } from 'vitest';
import { sanitizeSourceUrl } from '../lib/sanitize-url';

describe('sanitizeSourceUrl (T-1-08 acceptance gate)', () => {
  it('allows http://skillforge.local/foo', () => {
    const result = sanitizeSourceUrl('http://skillforge.local/foo');
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.href).toBe('http://skillforge.local/foo');
    }
  });

  it('allows http://localhost/foo', () => {
    const result = sanitizeSourceUrl('http://localhost/foo');
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.href).toBe('http://localhost/foo');
    }
  });

  it('allows https://127.0.0.1/foo', () => {
    const result = sanitizeSourceUrl('https://127.0.0.1/foo');
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.href).toBe('https://127.0.0.1/foo');
    }
  });

  it('blocks javascript:alert(1) with blocked_protocol and no hostname', () => {
    const result = sanitizeSourceUrl('javascript:alert(1)');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe('blocked_protocol');
      // javascript: URIs have an empty hostname in the URL constructor.
      expect(result.hostname ?? '').toBe('');
    }
  });

  it('blocks http://malicious.example.com/foo with blocked_hostname', () => {
    const result = sanitizeSourceUrl('http://malicious.example.com/foo');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe('blocked_hostname');
      expect(result.hostname).toBe('malicious.example.com');
    }
  });

  it('rejects "not a url" with invalid_url', () => {
    const result = sanitizeSourceUrl('not a url');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe('invalid_url');
    }
  });

  it('blocks file:///etc/passwd with blocked_protocol', () => {
    const result = sanitizeSourceUrl('file:///etc/passwd');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe('blocked_protocol');
      expect(result.hostname ?? '').toBe('');
    }
  });
});
