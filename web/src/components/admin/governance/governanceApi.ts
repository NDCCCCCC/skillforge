// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 40-07a — fetch wrappers for the governance admin routes.
// Mirrors pipelineApi.ts: GovernanceApiError carries {status, message};
// network failures map to 网络错误 (never a fabricated status).

import type { ChangelogEntry, ProvenanceGraph, ShaclReport } from './types';

const ADMIN_TOKEN_KEY = 'sf_admin_token';

function adminHeaders(): HeadersInit {
  return { 'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '' };
}

export class GovernanceApiError extends Error {
  constructor(public readonly status: number, public readonly message: string) {
    super(message);
    this.name = 'GovernanceApiError';
  }
}

async function toApiError(resp: Response): Promise<GovernanceApiError> {
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
  return new GovernanceApiError(resp.status, message);
}

async function withNetworkGuard<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (err: unknown) {
    if (err instanceof GovernanceApiError) throw err;
    throw new GovernanceApiError(0, '网络错误');
  }
}

export async function fetchShaclReport(): Promise<ShaclReport> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/governance/shacl', { headers: adminHeaders() });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<ShaclReport>;
    })(),
  );
}

export async function fetchProvenanceGraph(): Promise<ProvenanceGraph> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/governance/provenance', { headers: adminHeaders() });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<ProvenanceGraph>;
    })(),
  );
}

export async function fetchChangelog(): Promise<{ current_version: number; entries: ChangelogEntry[] }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/ontology/changelog', { headers: adminHeaders() });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<{ current_version: number; entries: ChangelogEntry[] }>;
    })(),
  );
}

export async function rollbackTBox(
  targetVersion: number,
  reason?: string,
): Promise<{ version: number; new_violations: number }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/ontology/rollback', {
        method: 'POST',
        headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ target_version: targetVersion, reason }),
      });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<{ version: number; new_violations: number }>;
    })(),
  );
}

export async function runPipeline(): Promise<{ status: string }> {
  return withNetworkGuard(
    (async () => {
      const resp = await fetch('/api/admin/pipeline/run', {
        method: 'POST',
        headers: adminHeaders(),
      });
      if (!resp.ok) throw await toApiError(resp);
      return resp.json() as Promise<{ status: string }>;
    })(),
  );
}
