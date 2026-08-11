// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// T-1-08 mandatory source-URL sanitization gate.
//
// Per CONTEXT.md §Claude's Discretion (T-1-08) and 04-UI-SPEC.md §FEUI-03
// (citation chip rendering), `source-url` chunks contain attacker-
// controllable href values that MUST be allowlisted before being set on
// any <a> element. The spike stub (`web/src/App.tsx` lines 47-51) sets
// href directly and MUST NOT survive into Phase 4.
//
// This module is the HARD GATE. No <a href={...}> in this codebase may
// bypass it. Rejected URLs render a blocked-citation span + console.warn
// so a malicious fixture cannot silently render a phishing link.
//
// Allowlist per 04-UI-SPEC §FEUI-03:
//   - protocol ∈ {https:, http:} (rejects javascript:, data:, vbscript:)
//   - hostname ∈ {localhost, 127.0.0.1, ::1, spike.local, skillforge.local}
//     (Phase 4 only renders local fixtures; production swap to the real
//     Hertz endpoint widens this allowlist in Wave 3 via D-04 swap test.)

const ALLOWED_PROTOCOLS = new Set(['https:', 'http:']);
const ALLOWED_HOSTNAMES = new Set([
  'localhost',
  '127.0.0.1',
  '::1',
  'spike.local',
  'skillforge.local',
]);

export type SanitizeResult =
  | { ok: true; href: string }
  | {
      ok: false;
      reason: 'blocked_protocol' | 'blocked_hostname' | 'invalid_url';
      hostname?: string;
    };

export function sanitizeSourceUrl(raw: string): SanitizeResult {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return { ok: false, reason: 'invalid_url' };
  }

  if (!ALLOWED_PROTOCOLS.has(url.protocol)) {
    return { ok: false, reason: 'blocked_protocol', hostname: url.hostname };
  }
  if (!ALLOWED_HOSTNAMES.has(url.hostname)) {
    return { ok: false, reason: 'blocked_hostname', hostname: url.hostname };
  }
  return { ok: true, href: url.toString() };
}
