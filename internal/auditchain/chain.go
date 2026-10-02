// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package auditchain is Phase 24 Task 24-02 (decision-chain) and
// 24-03 (time-travel state reconstruction). It reads the
// INSERT-only audit substrate and answers two questions an operator
// or external auditor asks of any session:
//
//   - "Why did this answer happen, step by step?" — ChainForSession
//     walks every audit row for the session and emits a chronological
//     list of {Kind, ts, evidence} links so the UI can render the
//     query → retrieve → verdict → approve/deny → rule_create → revoke
//     flow as one timeline.
//
//   - "What was the system state at time T?" — StateAtSession for a
//     session replays every audit row whose ts <= T and rebuilds the
//     turns-visible-to-the-model + approval-rules-effective-from-T
//     snapshots deterministically. Revoked rules are RESTORED from
//     the frozen_args column on their action_rule_revoked row so a
//     point-in-time query sees the rule that was live at T.
//
// All reconstruction is read-only and zero-LLM (audit is the source
// of truth; the store's InsertPendingApproval / MarkApproved /
// MarkRejected / MarkRuleRevoked / InsertRuleCreated paths are the
// ONLY writers — T-3-AUDIT invariant).
package auditchain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// LinkKind enumerates the chronological link types emitted by
// ChainForSession. The Kind is a classification, not an ordering
// key — ChainForSession always sorts by audit row id ASC (the
// invariant of an INSERT-only ledger).
type LinkKind string

const (
	LinkVerdict          LinkKind = "verdict"
	LinkPendingApproval  LinkKind = "pending_approval"
	LinkApproved         LinkKind = "approved"
	LinkRejected         LinkKind = "rejected"
	LinkRuleCreated      LinkKind = "rule_created"
	LinkRuleRevoked      LinkKind = "rule_revoked"
)

// Link is one node in a session's decision chain. Evidence carries
// enough to render the link in the FE AuditTimelineTab without a
// join back to the audit table.
type Link struct {
	Kind     LinkKind `json:"kind"`
	AuditID  int64    `json:"audit_id"`
	TsUnixMs int64    `json:"ts_unix_ms"`
	// ToolCallID is the shared identity of a HITL lifecycle row
	// (D11-C4). Empty for verdict rows.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ChunkIDs is the CSV from the verdict row's chunk_ids column.
	ChunkIDs string `json:"chunk_ids,omitempty"`
	// FrozenArgsJSON is the byte-identity tool-call payload of an
	// approval row (rule_created / rule_revoked rows also carry the
	// rule PK here — see audit_api.go encodeRuleID).
	FrozenArgsJSON []byte `json:"frozen_args,omitempty"`
	// SkillID and ModelVersion are denormalized so the FE can group
	// links per Skill / per model_version without a join.
	SkillID      string `json:"skill_id"`
	ModelVersion string `json:"model_version"`
	// RefusalReason carries the operator reason (reject / revoke) or
	// the model's refusal code. Empty for normal verdicts / approvals.
	RefusalReason string `json:"refusal_reason,omitempty"`
}

// RuleSnapshot is one row of the approval_rules table reconstructed
// for a given time T. Equal in shape to store.ApprovalRuleDetail;
// duplicated here so auditchain stays a leaf package (no upward
// dependency on a rules-table type that lives elsewhere).
type RuleSnapshot struct {
	Target         string `json:"target"`
	ArgsHash       string `json:"args_hash"`
	SkillID        string `json:"skill_id"`
	ScopeSessionID string `json:"scope_session_id"`
	CreatedAt      int64  `json:"created_at"`
	Revoked        bool   `json:"revoked,omitempty"`
}

// StateAt is the per-session T-point snapshot.
type StateAt struct {
	SessionID    string         `json:"session_id"`
	AtUnixMs     int64          `json:"at_unix_ms"`
	VisibleTurns int            `json:"visible_turns"`
	Rules        []RuleSnapshot `json:"rules"`
	// LastChainAuditID is the audit id of the last link whose ts <= At.
	// Useful for FE to scroll the chain view to that point.
	LastChainAuditID int64 `json:"last_chain_audit_id"`
}

// Store is the read surface auditchain needs. The internal/store
// Store satisfies it (with a thin adapter); tests pass an in-memory
// shim. Keep this interface tiny — every method is read-only, and
// none should depend on writer state.
type Store interface {
	// AuditBySessionAt returns the audit rows for sessionID with
	// ts <= tUnixMs, ordered by id ASC (monotonic per writer). The
	// "ASC + ts-bound" pair is what makes time-travel deterministic.
	AuditBySessionAt(ctx context.Context, sessionID string, tUnixMs int64) ([]store.AuditRow, error)
}

// ChainForSession reads every audit row for sessionID and returns a
// chronological slice of Links. Empty slice (non-nil) for sessions
// with no rows.
//
// Reconstruction rules (matching the audit substrate exactly):
//   - verdict row (action = "") → LinkVerdict with chunk_ids, refusal_reason, ts.
//   - action_pending_approval → LinkPendingApproval with frozen_args,
//     tool_call_id (== approval_id, D11-C4).
//   - action_approved | action_rejected → LinkApproved | LinkRejected,
//     joined to the pending row by tool_call_id (carries the operator's
//     refusal_reason when rejected).
//   - action_rule_created | action_rule_revoked → LinkRuleCreated |
//     LinkRuleRevoked, frozen_args = the rule PK JSON (audit_api.go
//     encodeRuleID).
func ChainForSession(ctx context.Context, s Store, sessionID string, limit int) ([]Link, error) {
	if limit <= 0 {
		limit = 1000
	}
	// Use a far-future bound so every row is included.
	rows, err := s.AuditBySessionAt(ctx, sessionID, time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	if err != nil {
		return nil, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	links := make([]Link, 0, len(rows))
	for _, r := range rows {
		links = append(links, classifyRow(r))
	}
	sort.SliceStable(links, func(i, j int) bool {
		return links[i].AuditID < links[j].AuditID
	})
	return links, nil
}

// ChainForSessionAt is ChainForSession restricted to rows with
// ts <= t — used by StateAt to reconstruct the chain at time T
// without a second scan.
func ChainForSessionAt(ctx context.Context, s Store, sessionID string, t time.Time) ([]Link, error) {
	rows, err := s.AuditBySessionAt(ctx, sessionID, t.UnixMilli())
	if err != nil {
		return nil, err
	}
	links := make([]Link, 0, len(rows))
	for _, r := range rows {
		links = append(links, classifyRow(r))
	}
	sort.SliceStable(links, func(i, j int) bool {
		return links[i].AuditID < links[j].AuditID
	})
	return links, nil
}

// StateAtSession reconstructs the per-session state at time T: the
// audit links visible at T and the set of approval-rules whose
// create_ts <= T AND (revoked_ts > T or never revoked). Revoked
// rules are rebuilt from their action_rule_revoked row's
// frozen_args so a time-travel query sees the rule that was live at T.
//
// scope_session_id == "" means the rule was created with skill-scope
// (visible to ALL sessions of that Skill); the rule is rebuilt only
// when its create row's session_id == query sessionID, OR when the
// rule has empty scope_session_id (skill-scope, visible to all).
func StateAtSession(ctx context.Context, s Store, sessionID string, t time.Time) (StateAt, error) {
	chain, err := ChainForSessionAt(ctx, s, sessionID, t)
	if err != nil {
		return StateAt{}, err
	}
	state := StateAt{
		SessionID:    sessionID,
		AtUnixMs:     t.UnixMilli(),
		VisibleTurns: countVisibleTurns(chain),
		Rules:        rebuildRules(chain, sessionID, t),
	}
	if n := len(chain); n > 0 {
		state.LastChainAuditID = chain[n-1].AuditID
	}
	return state, nil
}

// countVisibleTurns counts every LinkVerdict + every LinkApproved /
// LinkRejected (a resolution row is a "decision turn" the user
// participated in) — the same posture the AgentInspector already
// uses for the live session view (see internal/server/agent.go).
func countVisibleTurns(chain []Link) int {
	n := 0
	for _, l := range chain {
		if l.Kind == LinkVerdict || l.Kind == LinkApproved || l.Kind == LinkRejected {
			n++
		}
	}
	return n
}

// rebuildRules replays create / revoke pairs and emits the rules
// effective at t for the queried session.
func rebuildRules(chain []Link, sessionID string, t time.Time) []RuleSnapshot {
	// scope_key = (target, args_hash, skill_id, scope_session_id) — same
	// shape as approval_rules.PK.
	type scopeKey struct {
		Target         string
		ArgsHash       string
		SkillID        string
		ScopeSessionID string
	}
	live := map[scopeKey]bool{}     // currently live rules (create w/o later revoke <= t)
	revoked := map[scopeKey]int64{} // ts of last revoke
	type pending struct {
		row RuleSnapshot
		ts  int64
	}
	var pendingCreates []pending
	for _, l := range chain {
		if l.TsUnixMs > t.UnixMilli() {
			continue
		}
		switch l.Kind {
		case LinkRuleCreated:
			var pk rulePKPayload
			if err := decodeRulePK(l.FrozenArgsJSON, &pk); err != nil {
				continue
			}
			pendingCreates = append(pendingCreates, pending{row: RuleSnapshot{
				Target: pk.Target, ArgsHash: pk.ArgsHash, SkillID: pk.SkillID,
				ScopeSessionID: pk.ScopeSessionID, CreatedAt: l.TsUnixMs,
			}, ts: l.TsUnixMs})
		case LinkRuleRevoked:
			var pk rulePKPayload
			if err := decodeRulePK(l.FrozenArgsJSON, &pk); err != nil {
				continue
			}
			k := scopeKey{pk.Target, pk.ArgsHash, pk.SkillID, pk.ScopeSessionID}
			revoked[k] = l.TsUnixMs
		}
	}
	// Now compute "currently live" = pendingCreates minus any later revoke.
	for _, p := range pendingCreates {
		k := scopeKey{p.row.Target, p.row.ArgsHash, p.row.SkillID, p.row.ScopeSessionID}
		if rts, ok := revoked[k]; ok && rts > p.ts {
			continue // revoked before T
		}
		live[k] = true
	}
	// Apply session visibility (skill-scope rules are visible to all
	// sessions of that Skill; session-scope rules are visible only to
	// the session that created them).
	out := make([]RuleSnapshot, 0, len(live))
	for k := range live {
		if k.ScopeSessionID != "" && k.ScopeSessionID != sessionID {
			continue
		}
		out = append(out, RuleSnapshot{
			Target: k.Target, ArgsHash: k.ArgsHash, SkillID: k.SkillID,
			ScopeSessionID: k.ScopeSessionID,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}

// rulePKPayload mirrors internal/server/approval_rules_api.go's
// ruleIDPayload — kept local to auditchain so this package stays a
// leaf with no upward dependency on the HTTP layer.
type rulePKPayload struct {
	Target         string `json:"target"`
	ArgsHash       string `json:"args_hash"`
	SkillID        string `json:"skill_id"`
	ScopeSessionID string `json:"scope_session_id"`
}

// decodeRulePK inverts audit_api.encodeRuleID: the audit row's
// frozen_args column stores base64url(JSON(rulePKPayload)). Returns
// nil on any parse failure (the caller treats that as "skip this
// link" rather than failing the whole chain — the same posture as
// every other link-recovery path in this package).
func decodeRulePK(raw []byte, dst *rulePKPayload) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty frozen_args")
	}
	// Some legacy rows may have stored the raw JSON (no base64 wrap);
	// try that first, then fall back to base64.
	if err := json.Unmarshal(raw, dst); err == nil &&
		dst.Target != "" && dst.ArgsHash != "" && dst.SkillID != "" {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(string(raw))
	if err != nil {
		return fmt.Errorf("base64: %w", err)
	}
	return json.Unmarshal(payload, dst)
}

// classifyRow maps one audit row to one Link. Pure function — the only
// state involved is the row itself.
func classifyRow(r store.AuditRow) Link {
	link := Link{
		AuditID:       r.ID,
		TsUnixMs:      r.Timestamp.UnixMilli(),
		ToolCallID:    r.ToolCallID,
		ChunkIDs:      r.ChunkIDs,
		SkillID:       r.SkillID,
		ModelVersion:  r.ModelVersion,
		RefusalReason: r.RefusalReason,
	}
	if len(r.FrozenArgs) > 0 {
		link.FrozenArgsJSON = append([]byte(nil), r.FrozenArgs...)
	}
	switch r.Action {
	case "":
		link.Kind = LinkVerdict
	case "action_pending_approval":
		link.Kind = LinkPendingApproval
	case "action_approved":
		link.Kind = LinkApproved
	case "action_rejected":
		link.Kind = LinkRejected
	case "action_rule_created":
		link.Kind = LinkRuleCreated
	case "action_rule_revoked":
		link.Kind = LinkRuleRevoked
	}
	return link
}
