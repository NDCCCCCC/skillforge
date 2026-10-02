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
//   - hostname ∈ configurable set (default: localhost, 127.0.0.1, ::1,
//     spike.local, skillforge.local).  Override via VITE_ALLOWED_HOSTNAMES
//     env var (comma-separated).  Runtime override via setHostnameAllowlist().

const DEFAULT_HOSTNAMES = ['localhost', '127.0.0.1', '::1', 'spike.local', 'skillforge.local'] as const;

let _hostnameAllowlist: Set<string> = new Set([...DEFAULT_HOSTNAMES]);

export function getHostnameAllowlist(): Set<string> {
  return _hostnameAllowlist;
}

export function setHostnameAllowlist(hosts: string[]): void {
  _hostnameAllowlist = new Set(hosts);
}

export function resetHostnameAllowlist(): void {
  _hostnameAllowlist = new Set([...DEFAULT_HOSTNAMES]);
}

// Vitest-only test hook — not callable in production bundles.
export function __setHostnameAllowlistForTesting(hosts: string[]): void {
  if (typeof import.meta !== 'undefined' && (import.meta as { vitest?: unknown }).vitest) {
    _hostnameAllowlist = new Set(hosts);
  }
}

// Expand default allowlist from env if set.
const _envHostnames = (import.meta as { env?: { VITE_ALLOWED_HOSTNAMES?: string } }).env?.VITE_ALLOWED_HOSTNAMES;
if (typeof _envHostnames === 'string' && _envHostnames.length > 0) {
  _hostnameAllowlist = new Set(_envHostnames.split(',').map((h: string) => h.trim()));
}

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

  if (!new Set(['https:', 'http:']).has(url.protocol)) {
    return { ok: false, reason: 'blocked_protocol', hostname: url.hostname };
  }
  if (!getHostnameAllowlist().has(url.hostname)) {
    return { ok: false, reason: 'blocked_hostname', hostname: url.hostname };
  }
  return { ok: true, href: url.toString() };
}
