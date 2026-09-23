// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — fetch wrappers for the platform admin routes.
// Mirrors governanceApi.ts: PlatformApiError carries {status, message};
// network failures map to 网络错误 (never a fabricated status).

import type {
  HybridQuery,
  HybridResponse,
  McpInvokeRequest,
  McpInvokeResponse,
  McpServerStatus,
  McpTool,
  SparqlPattern,
  SparqlResponse,
} from './types';

const ADMIN_TOKEN_KEY = 'sf_admin_token';

function adminHeaders(): HeadersInit {
  return { 'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '' };
}

function skillHeaders(skill: string): HeadersInit {
  return { 'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '', 'X-Skill-Name': skill };
}

export class PlatformApiError extends Error {
  constructor(public readonly status: number, public readonly message: string) {
    super(message);
    this.name = 'PlatformApiError';
  }
}

async function toApiError(resp: Response): Promise<PlatformApiError> {
  let raw = '';
  try {
    raw = await resp.text();
  } catch {
    // body unreadable
  }
  let message = raw;
  try {
    const parsed = JSON.parse(raw) as { error?: unknown; message?: unknown };
    if (parsed && typeof parsed.message === 'string') {
      message = parsed.message;
    }
  } catch {
    // not JSON — keep raw body
  }
  return new PlatformApiError(resp.status, message);
}

async function withNetworkGuard<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (err: unknown) {
    if (err instanceof PlatformApiError) throw err;
    throw new PlatformApiError(0, '网络错误');
  }
}

// ── SPARQL ────────────────────────────────────────────────────────────────

export async function sparqlQuery(pattern: SparqlPattern, skill: string): Promise<SparqlResponse> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/sparql', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...skillHeaders(skill),
        },
        body: JSON.stringify(pattern),
      });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<SparqlResponse>;
    })(),
  );
}

// ── MCP ──────────────────────────────────────────────────────────────────

export async function fetchMcpStatus(): Promise<McpServerStatus> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/platform/mcp/status', { headers: adminHeaders() });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<McpServerStatus>;
    })(),
  );
}

export async function fetchMcpTools(): Promise<McpTool[]> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/platform/mcp/tools', { headers: adminHeaders() });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<McpTool[]>;
    })(),
  );
}

export async function invokeMcpTool(req: McpInvokeRequest): Promise<McpInvokeResponse> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/platform/mcp/invoke', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...adminHeaders(),
        },
        body: JSON.stringify(req),
      });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<McpInvokeResponse>;
    })(),
  );
}

// ── Hybrid ────────────────────────────────────────────────────────────────

export async function runHybridSearch(query: HybridQuery, skill: string): Promise<HybridResponse> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/platform/hybrid/search', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...skillHeaders(skill),
        },
        body: JSON.stringify(query),
      });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<HybridResponse>;
    })(),
  );
}
