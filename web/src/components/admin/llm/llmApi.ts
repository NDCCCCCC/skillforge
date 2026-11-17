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

// POST /api/admin/llm/models — 面板「拉取模型列表」的正式入口。
//
// 261117-mpr：改成 POST + 请求体，因为候选连接**必须包含页面上刚填的
// 明文 key**，而 GET 不能把 key 放进 URL（会落进 nginx access log、
// 浏览器历史、Referer 头 —— F-02/E-02 的原始约束）。请求体不进 URL，
// 那三个泄漏面都不成立，于是面板填的 key 能真正到达服务端；此前走 GET
// 时服务端只能发 anonymous，真实网关必然 401，面板就报一条让人误以为
// key 错的「令牌已过期」。
export function fetchLLMModels(
  token: string,
  req: Partial<LLMSaveRequest>,
): Promise<{ models: string[] }> {
  return fetch('/api/admin/llm/models', {
    method: 'POST',
    headers: adminHeaders(token),
    body: JSON.stringify(req),
  }).then(parse) as Promise<{ models: string[] }>;
}
