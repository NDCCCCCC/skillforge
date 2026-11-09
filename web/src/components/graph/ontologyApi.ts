// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-g7x — fetch wrapper for the note-centric ontology graph
// projection (GET /api/admin/ontology/graph). Mirrors governanceApi.ts:
// OntologyGraphApiError carries {status, message}; network failures map
// to 网络错误 (never a fabricated status).
//
// ADMIN GATE: the endpoint is X-Admin-Token-gated (same posture as
// kg/query), so the 本体视图 needs an admin token in localStorage
// (sf_admin_token — set by the admin console login). A 403 surfaces as a
// readable error, never a blank canvas.

// NOTE: note-id derivation (sources path + chunk_id resolution) lives in
// lib/graph-focus; the caller passes resolved ids in — this module is a
// pure wire client.


import { ownerTokenHeaders } from '../../lib/owner-token';

/** One node of the projected ontology graph (BE ontologyGraphNode). */
export interface OntologyNode {
  id: string;
  kind: 'note' | 'table' | 'insurance_product' | 'organization';
  label: string;
  tier?: string;
}

/** One edge of the projected ontology graph (BE ontologyGraphEdge). */
export interface OntologyEdge {
  from: string;
  to: string;
  type: 'references_table' | 'involves_product' | 'involves_org' | 'parent_org';
}

/** The 200 envelope of GET /api/admin/ontology/graph. */
export interface OntologyGraph {
  nodes: OntologyNode[];
  edges: OntologyEdge[];
}

const ADMIN_TOKEN_KEY = 'sf_admin_token';

/**
 * 当前浏览器是否持有管理员令牌。
 *
 * 本体视图走 /api/admin/ontology/graph，是 X-Admin-Token 门禁的端点。
 * 没有令牌时 KnowledgeGraphBrowser 用一句温和的说明代替"错误 + 令牌
 * 输入框"作为落地页 —— 普通用户一眼就能看出这是管理员功能，而不是
 * 以为自己撞上了故障。
 */
export function hasAdminToken(): boolean {
  try {
    return (localStorage.getItem(ADMIN_TOKEN_KEY) ?? '').trim().length > 0;
  } catch {
    // localStorage 不可用 → 按"无令牌"处理（温和说明）
    return false;
  }
}

export class OntologyGraphApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'OntologyGraphApiError';
  }
}

/**
 * Fetch the ontology graph projection for the given note local names.
 *
 * The CALLER owns note-id derivation (the view merges the sync
 * source-path path with the async chunk_id resolution — see
 * OntologyGraphView); an empty list fetches 全库 (the whole ABox
 * projection).
 */
export async function fetchOntologyGraph(notes: string[] = []): Promise<OntologyGraph> {
  const qs = notes.length > 0 ? `?notes=${encodeURIComponent(notes.join(','))}` : '';
  let resp: Response;
  try {
    resp = await fetch(`/api/admin/ontology/graph${qs}`, {
      headers: {
        'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '',
        ...ownerTokenHeaders(),
      },
    });
  } catch {
    throw new OntologyGraphApiError(0, '网络错误');
  }
  if (!resp.ok) {
    let message = `HTTP ${resp.status}`;
    try {
      const body = (await resp.json()) as { error?: unknown };
      if (body && typeof body.error === 'string') message = body.error;
    } catch {
      // body unreadable — keep the HTTP-status message
    }
    if (resp.status === 403) {
      message = '需要管理员令牌（本体视图走 /api/admin 端点）';
    }
    throw new OntologyGraphApiError(resp.status, message);
  }
  const data = (await resp.json()) as Partial<OntologyGraph>;
  // Go nil slices serialize to null — normalize at this choke point
  // (quick-260928-jxb BUG-1 posture).
  return { nodes: data.nodes ?? [], edges: data.edges ?? [] };
}
