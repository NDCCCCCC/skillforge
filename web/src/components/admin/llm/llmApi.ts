// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 52 typed client for the admin LLM provider panel.
// Every call is X-Admin-Token gated server-side; the token is supplied by
// the caller (same pattern as governanceApi.ts).

export interface LLMPlan {
  id: string;
  label: string;
  base_url: string;
  models_url: string;
  note: string;
}

export interface LLMVendor {
  id: string;
  label: string;
  plans: LLMPlan[];
}

// 261113-mkc: mirrors the backend wire contract (internal/server/
// llm_admin.go llmConfigPublic). base_url / masked_key / api_key_env /
// updated_at carry Go omitempty, so they can be absent at runtime (e.g.
// env-derived config) — hence optional. The backend never emitted
// api_key_masked / api_key_set (the previous optimistic shape).
export interface LLMConfigView {
  vendor: string;
  plan: string;
  base_url?: string;
  model: string;
  masked_key?: string;
  api_key_env?: string;
  updated_at?: string;
}

export interface LLMConfigState {
  source: 'file' | 'env' | 'none';
  config: LLMConfigView | null;
  mode: string;
  available: boolean;
}

export interface LLMSaveRequest {
  vendor: string;
  plan: string;
  base_url?: string;
  model: string;
  api_key?: string;
  api_key_env?: string;
}

export interface LLMTestResult {
  ok: boolean;
  latency_ms: number;
  status: string;
  error?: string;
}

function adminHeaders(token: string): HeadersInit {
  return { 'Content-Type': 'application/json', 'X-Admin-Token': token };
}

async function parse(res: Response): Promise<unknown> {
  const text = await res.text();
  let body: { error?: string };
  try {
    body = text ? (JSON.parse(text) as { error?: string }) : {};
  } catch {
    throw new Error(`HTTP ${res.status}: ${text.slice(0, 200)}`);
  }
  if (!res.ok) throw new Error(body.error ?? `HTTP ${res.status}`);
  return body;
}

export function fetchLLMProviders(token: string, signal?: AbortSignal): Promise<{ vendors: LLMVendor[] }> {
  return fetch('/api/admin/llm/providers', { headers: adminHeaders(token), signal }).then(parse) as Promise<{
    vendors: LLMVendor[];
  }>;
}

export function fetchLLMConfig(token: string, signal?: AbortSignal): Promise<LLMConfigState> {
  return fetch('/api/admin/llm/config', { headers: adminHeaders(token), signal }).then(parse) as Promise<LLMConfigState>;
}

export function saveLLMConfig(token: string, req: LLMSaveRequest): Promise<{ saved: boolean; mode: string; available: boolean }> {
  return fetch('/api/admin/llm/config', {
    method: 'POST',
    headers: adminHeaders(token),
    body: JSON.stringify(req),
  }).then(parse) as Promise<{ saved: boolean; mode: string; available: boolean }>;
}

export function testLLMKey(token: string, req: LLMSaveRequest): Promise<LLMTestResult> {
  return fetch('/api/admin/llm/test', {
    method: 'POST',
    headers: adminHeaders(token),
    body: JSON.stringify(req),
  }).then(parse) as Promise<LLMTestResult>;
}

// GET /api/admin/llm/models takes the candidate connection as QUERY
// PARAMETERS (the handler reads c.Query, not a body).
//
// CODE-AUDIT 2026-11-05 (F-02/E-02): `api_key` is explicitly EXCLUDED from
// the query — GET URLs land in nginx access logs, browser history and the
// Referer header. The backend resolves the credential from api_key_env
// (SKILLFORGE_* whitelist) or the server-stored config instead, so the
// panel flow works without ever putting a plaintext key in a URL.
export function fetchLLMModels(
  token: string,
  req: Partial<LLMSaveRequest>,
): Promise<{ models: string[] }> {
  const q = new URLSearchParams(
    Object.entries(req).filter(
      ([k, v]) => k !== 'api_key' && v != null && v !== '',
    ) as [string, string][],
  );
  return fetch(`/api/admin/llm/models?${q.toString()}`, {
    headers: adminHeaders(token),
  }).then(parse) as Promise<{ models: string[] }>;
}
