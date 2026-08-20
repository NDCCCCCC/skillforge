// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Session-management phase — FE roster API surface.
//
// Mirrors the chat-restore.ts posture: API_BASE hard-coded, optional
// fetchImpl injection for tests, and non-2xx / network errors surface
// as null (the caller degrades to the local in-memory session state —
// the roster is an enhancement, never a blocker).
//
// Wire shapes mirror internal/server/handlers_sessions.go exactly:
//   - GET  /api/sessions      → { sessions: [{id, title, last_active_ms}] }
//   - PATCH /api/session/:id  body {title} (rename upsert)
//   - DELETE /api/session/:id (idempotent 204)
import type { PendingApprovalRow } from '../store/app-store';

/** One roster row from GET /api/sessions (snake_case wire, DO NOT
 * camelCase — matches the Go handler's JSON tags). */
export interface SessionRosterEntry {
  id: string;
  title: string;
  last_active_ms: number;
}

/** Response shape of GET /api/sessions. The BE guarantees sessions is
 * an array (never null). */
export interface SessionRosterResponse {
  sessions: SessionRosterEntry[];
}

/** Same API-base posture as chat-restore.ts — '/api' is the Vite proxy
 * target (dev) and the same-origin path (prod). */
const API_BASE = '/api';

/** Fetch GET /api/sessions. Returns the roster or null on any error /
 * non-2xx / malformed payload. A null return means "roster unavailable"
 * — the caller keeps the local session state (fresh-session posture,
 * symmetric with the BE's T-10-11 no-existence-leak stance). */
export async function fetchSessions(
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<SessionRosterEntry[] | null> {
  try {
    const resp = await fetchImpl(`${API_BASE}/sessions`, {
      headers: { Accept: 'application/json' },
    });
    if (!resp.ok) {
      return null;
    }
    const body = (await resp.json()) as Partial<SessionRosterResponse>;
    if (!body || !Array.isArray(body.sessions)) {
      return null;
    }
    return body.sessions.filter(
      (s) => typeof s?.id === 'string' && typeof s?.title === 'string',
    );
  } catch {
    return null;
  }
}

/** PATCH /api/session/:id — rename upsert. Returns true on 2xx, false
 * on any error (the caller decides whether to roll back the optimistic
 * FE rename). */
export async function renameSession(
  sessionId: string,
  title: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<boolean> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
      },
    );
    return resp.ok;
  } catch {
    return false;
  }
}

/** DELETE /api/session/:id — turns + meta, audit untouched. Returns
 * true on 2xx/204 (the BE is idempotent — an unknown id still 204s,
 * so false only means "request failed", never "session did not
 * exist"). */
export async function deleteSession(
  sessionId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<boolean> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}`,
      { method: 'DELETE' },
    );
    return resp.ok;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Plan 11-07 — HITL approval-gate fetchers (HITL-03 cross-refresh resume +
// HITL-08 approval-rules panel + audit timeline tab).
//
// Wire ground truth (internal/server/approval_api.go +
// approval_rules_api.go + audit_api.go):
//   - GET  /api/session/:id/approval
//       → 200 {approvals: [pendingApproval...]} (snake_case; created_at is
//         RFC3339; state ∈ active | expired | auto_passed). Empty list =
//         no pending (there is NO 204 path).
//   - GET  /api/session/:id/approval-rules?scope=session|skill[&skill_id=…]
//       → 200 {rules: [ApprovalRuleJSON...]}. scope=skill REQUIRES skill_id.
//   - DELETE /api/session/:id/approval-rules/:ruleId
//       → 200 {status:'revoked', rule:{…}} (DELETE + audit INSERT in one tx)
//   - GET  /api/session/:id/audit?type=…&limit=N
//       → 200 {rows: [auditEntry...], total}
// Same posture as the helpers above: optional fetchImpl injection; errors
// degrade to null / undefined / {ok:false}, never throw.
// ---------------------------------------------------------------------------

/** Parse an RFC3339 timestamp to epoch ms. Go's zero-time
 * ("0001-01-01T00:00:00Z") parses to a large negative value — the caller
 * treats <= 0 as "unset". */
function parseEpochMs(iso: string): number {
  const t = Date.parse(iso);
  return Number.isNaN(t) ? 0 : t;
}

/** One approval-rule row, mapped to the FE camelCase shape the
 * ApprovalRulesPanel renders (11-UI-SPEC §ApprovalRuleRow). */
export interface ApprovalRuleRow {
  ruleId: string;
  target: string;
  argsHash: string;
  skillId: string;
  scope: 'session' | 'skill';
  scopeSessionId: string;
  /** epoch ms. */
  createdAt: number;
  /** epoch ms, or null when the rule has never auto-passed (BE zero-time). */
  lastUsedAt: number | null;
}

/** Wire shape of one rule row (snake_case — mirrors ApprovalRuleJSON). */
interface ApprovalRuleWire {
  rule_id: string;
  target: string;
  args_hash: string;
  skill_id: string;
  scope_session_id: string;
  scope: string;
  created_at: string;
  last_used_at: string;
}

function mapApprovalRule(w: ApprovalRuleWire): ApprovalRuleRow | null {
  if (typeof w?.rule_id !== 'string' || typeof w?.target !== 'string') {
    return null;
  }
  const lastUsed = parseEpochMs(w.last_used_at ?? '');
  return {
    ruleId: w.rule_id,
    target: w.target,
    argsHash: typeof w.args_hash === 'string' ? w.args_hash : '',
    skillId: typeof w.skill_id === 'string' ? w.skill_id : '',
    scope: w.scope === 'skill' ? 'skill' : 'session',
    scopeSessionId: typeof w.scope_session_id === 'string' ? w.scope_session_id : '',
    createdAt: parseEpochMs(w.created_at ?? ''),
    lastUsedAt: lastUsed > 0 ? lastUsed : null,
  };
}

/** Fetch the session's approval rules for BOTH D11-D3 scopes: the
 * session scope always, plus the skill scope when a current Skill is
 * known (the BE requires skill_id for scope=skill). Merged + deduped by
 * rule_id. Returns null on error (the session-scope request failing is
 * the fail signal; a skill-scope failure degrades to session-only). */
export async function fetchApprovalRules(
  sessionId: string,
  skillId: string | null,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<ApprovalRuleRow[] | null> {
  const base = `${API_BASE}/session/${encodeURIComponent(sessionId)}/approval-rules`;
  try {
    const sessionResp = await fetchImpl(`${base}?scope=session`, {
      headers: { Accept: 'application/json' },
    });
    if (!sessionResp.ok) {
      return null;
    }
    const sessionBody = (await sessionResp.json()) as { rules?: ApprovalRuleWire[] };
    const wires: ApprovalRuleWire[] = Array.isArray(sessionBody?.rules)
      ? sessionBody.rules
      : [];
    if (skillId) {
      try {
        const skillResp = await fetchImpl(
          `${base}?scope=skill&skill_id=${encodeURIComponent(skillId)}`,
          { headers: { Accept: 'application/json' } },
        );
        if (skillResp.ok) {
          const skillBody = (await skillResp.json()) as { rules?: ApprovalRuleWire[] };
          if (Array.isArray(skillBody?.rules)) {
            wires.push(...skillBody.rules);
          }
        }
      } catch {
        // Skill-scope fetch degraded — session-scope rows still render.
      }
    }
    const seen = new Set<string>();
    const out: ApprovalRuleRow[] = [];
    for (const w of wires) {
      const row = mapApprovalRule(w);
      if (!row || seen.has(row.ruleId)) continue;
      seen.add(row.ruleId);
      out.push(row);
    }
    return out;
  } catch {
    return null;
  }
}

/** Revoke one rule. The BE runs DELETE approval_rules + INSERT audit
 * action_rule_revoked in a single transaction (D11-D4 / 11-05). */
export async function revokeApprovalRule(
  sessionId: string,
  ruleId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<{ ok: true } | { ok: false; reason: string }> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}/approval-rules/${encodeURIComponent(ruleId)}`,
      { method: 'DELETE', headers: { Accept: 'application/json' } },
    );
    if (resp.ok) return { ok: true };
    const reason = resp.status === 404 ? '规则不存在' : `HTTP ${resp.status}`;
    return { ok: false, reason };
  } catch (e) {
    return { ok: false, reason: e instanceof Error ? e.message : '网络错误' };
  }
}

/** Fetch GET /api/session/:id/approval — the HITL-03 cross-refresh
 * resume read. Returns the first pending row (snake_case, as the store's
 * hydrateFromGet consumes it), null when the envelope is empty (no
 * pending), or undefined on non-2xx / network error / malformed payload
 * (the fail-closed branch — no banner, no pending mutation). */
export async function fetchPendingApproval(
  sessionId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<PendingApprovalRow | null | undefined> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}/approval`,
      { headers: { Accept: 'application/json' } },
    );
    if (!resp.ok) return undefined;
    const body = (await resp.json()) as { approvals?: PendingApprovalRow[] };
    if (!body || !Array.isArray(body.approvals)) return undefined;
    if (body.approvals.length === 0) return null;
    const row = body.approvals[0];
    if (typeof row?.approval_id !== 'string') return undefined;
    return row;
  } catch {
    return undefined;
  }
}

/** One audit row for the AuditTimelineTab (camelCase FE shape; ts is
 * epoch ms parsed from the BE's RFC3339). The action enum is the HITL
 * lifecycle set (D11-D2 + D11-D4). */
export interface SessionAuditRow {
  id: number;
  /** epoch ms. */
  ts: number;
  action:
    | 'action_pending_approval'
    | 'action_approved'
    | 'action_rejected'
    | 'action_rule_revoked';
  target: string;
  reason: string;
  toolCallId: string;
  approvalId: string;
  scopeSessionId: string;
  skillVersion: string;
  promptHash: string;
  userId: string;
}

/** Wire shape of one audit row (snake_case — mirrors auditEntry). */
interface SessionAuditWire {
  id: number;
  ts: string;
  action: string;
  target?: string;
  reason?: string;
  tool_call_id?: string;
  approval_id?: string;
  scope_session_id?: string;
  skill_version?: string;
  prompt_hash?: string;
  user_id?: string;
}

const AUDIT_ACTIONS = new Set([
  'action_pending_approval',
  'action_approved',
  'action_rejected',
  'action_rule_revoked',
]);

/** Fetch GET /api/session/:id/audit?limit=100. Returns the mapped rows
 * or null on any error / non-2xx / malformed payload — the
 * AuditTimelineTab renders its fail-closed banner on null. */
export async function fetchAuditRows(
  sessionId: string,
  fetchImpl: typeof globalThis.fetch = globalThis.fetch,
): Promise<SessionAuditRow[] | null> {
  try {
    const resp = await fetchImpl(
      `${API_BASE}/session/${encodeURIComponent(sessionId)}/audit?limit=100`,
      { headers: { Accept: 'application/json' } },
    );
    if (!resp.ok) return null;
    const body = (await resp.json()) as { rows?: SessionAuditWire[] };
    if (!body || !Array.isArray(body.rows)) return null;
    const out: SessionAuditRow[] = [];
    for (const w of body.rows) {
      if (typeof w?.id !== 'number' || !AUDIT_ACTIONS.has(w?.action)) continue;
      out.push({
        id: w.id,
        ts: parseEpochMs(w.ts ?? ''),
        action: w.action as SessionAuditRow['action'],
        target: typeof w.target === 'string' ? w.target : '',
        reason: typeof w.reason === 'string' ? w.reason : '',
        toolCallId: typeof w.tool_call_id === 'string' ? w.tool_call_id : '',
        approvalId: typeof w.approval_id === 'string' ? w.approval_id : '',
        scopeSessionId: typeof w.scope_session_id === 'string' ? w.scope_session_id : '',
        skillVersion: typeof w.skill_version === 'string' ? w.skill_version : '',
        promptHash: typeof w.prompt_hash === 'string' ? w.prompt_hash : '',
        userId: typeof w.user_id === 'string' ? w.user_id : '',
      });
    }
    return out;
  } catch {
    return null;
  }
}
