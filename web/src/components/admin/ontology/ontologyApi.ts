// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — fetch wrappers for the 8 ontology admin routes
// (D-Route-01). Admin token comes from localStorage 'sf_admin_token'
// (same contract as CapacityMonitor / DocUpload).

import type {
  KGQueryResponse,
  KGStatsResponse,
  Proposal,
  ProposalApproveResponse,
  ProposalGenerateResponse,
  SnapshotResponse,
  TBoxResponse,
} from './types';

function adminHeaders(): Record<string, string> {
  const token = localStorage.getItem('sf_admin_token') ?? '';
  return { 'X-Admin-Token': token };
}

function jsonHeaders(): Record<string, string> {
  return { ...adminHeaders(), 'Content-Type': 'application/json' };
}

async function getJSON<T>(path: string): Promise<T> {
  const resp = await fetch(path, { headers: adminHeaders() });
  if (!resp.ok) {
    throw new Error(`${resp.status} ${await resp.text()}`);
  }
  return resp.json() as Promise<T>;
}

async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const resp = await fetch(path, {
    method: 'POST',
    headers: jsonHeaders(),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!resp.ok) {
    throw new Error(`${resp.status} ${await resp.text()}`);
  }
  return resp.json() as Promise<T>;
}

export function fetchTBox(): Promise<TBoxResponse> {
  return getJSON<TBoxResponse>('/api/admin/ontology/tbox');
}

export function fetchKGStats(): Promise<KGStatsResponse> {
  return getJSON<KGStatsResponse>('/api/admin/ontology/kg/stats');
}

export function fetchKGQuery(s: string, p: string, o: string): Promise<KGQueryResponse> {
  const params = new URLSearchParams();
  if (s) params.set('s', s);
  if (p) params.set('p', p);
  if (o) params.set('o', o);
  const qs = params.toString();
  return getJSON<KGQueryResponse>(`/api/admin/ontology/kg/query${qs ? `?${qs}` : ''}`);
}

export function generateProposal(text: string): Promise<ProposalGenerateResponse> {
  return postJSON<ProposalGenerateResponse>('/api/admin/ontology/proposals/generate', { text });
}

export function fetchProposals(): Promise<{ proposals: Proposal[] }> {
  return getJSON<{ proposals: Proposal[] }>('/api/admin/ontology/proposals');
}

export function approveProposal(id: string, customLocalName?: string): Promise<ProposalApproveResponse> {
  return postJSON<ProposalApproveResponse>(`/api/admin/ontology/proposals/${id}/approve`, {
    custom_local_name: customLocalName ?? '',
  });
}

export function rejectProposal(id: string): Promise<{ proposal: Proposal }> {
  return postJSON<{ proposal: Proposal }>(`/api/admin/ontology/proposals/${id}/reject`);
}

export function triggerSnapshot(): Promise<SnapshotResponse> {
  return postJSON<SnapshotResponse>('/api/admin/ontology/snapshot');
}
