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

// approval_rules.go is the Plan 11-05 / HITL-08 / D11-D2 persistent
// substrate for approve-and-remember rules. The table is INDEPENDENT
// from the audit table (D11-D2): rule rows are created, matched,
// last-used-touched, and DELETED here; the audit trail of those
// lifecycle events lives in the audit table (T-3-AUDIT INSERT-only —
// the audit evidence for a revoke is the action_rule_revoked audit row,
// not the missing rule row).
//
// Rule key (D11-D1): (target, sha256(frozenArgs), skill_id) + scope.
// PK (D11-D2): (target, args_hash, skill_id, scope_session_id).
//
// Binary scope (D11-D3):
//
//   - session-scope rule: scope_session_id = the originating session ID.
//     Matches ONLY inside that session.
//   - skill-scope rule:   scope_session_id = ” with skill_id set.
//     Matches across sessions for the SAME Skill.
//
// There is NO global scope — the column set structurally forbids it
// (an empty scope_session_id ALWAYS means skill-scope, never global).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/platformfloor"
)

// ApprovalRule is one row of the approval_rules table. The 4-tuple
// (Target, ArgsHash, SkillID, ScopeSessionID) is the primary key
// (D11-D2). ArgsHash is the hex SHA-256 of the frozen tool-call args
// bytes (D11-D1 rule key — computed by
// internal/agent/hitl.ComputeFrozenArgsHash, never re-marshaled through
// encoding/json).
type ApprovalRule struct {
	Target         string
	ArgsHash       string
	SkillID        string
	ScopeSessionID string
	CreatedAt      time.Time
	LastUsedAt     time.Time
}

// Scope is the D11-D3 binary scope enum. Only two values exist; there
// is deliberately no ScopeGlobal.
type Scope int

const (
	// ScopeSession binds the rule to one session (scope_session_id =
	// that session's ID).
	ScopeSession Scope = 1
	// ScopeSkill binds the rule to one Skill across sessions
	// (scope_session_id = '', skill_id = the Skill identity).
	ScopeSkill Scope = 2
)

// createRuleSQL is INSERT OR IGNORE: re-approving an identical
// (target, args, skill, scope) tuple refreshes nothing and errors
// nothing — the rule already exists, which is the desired end state.
const createRuleSQL = `
INSERT OR IGNORE INTO approval_rules (
    target, args_hash, skill_id, scope_session_id, created_at, last_used_at
) VALUES (?, ?, ?, ?, ?, ?)
`

// queryRulesBySessionScopeSQL lists the rules visible inside ONE
// session: the session-scope rules bound to that session.
const queryRulesBySessionScopeSQL = `
SELECT target, args_hash, skill_id, scope_session_id, created_at, last_used_at
FROM approval_rules
WHERE scope_session_id = ?
ORDER BY created_at DESC
`

// queryRulesBySkillScopeSQL lists the skill-scope rules for ONE Skill
// (scope_session_id = ” by the D11-D3 binary-scope encoding).
const queryRulesBySkillScopeSQL = `
SELECT target, args_hash, skill_id, scope_session_id, created_at, last_used_at
FROM approval_rules
WHERE skill_id = ? AND scope_session_id = ''
ORDER BY created_at DESC
`

// matchRuleSQL is the D11-D1 lookup. A rule matches when the full key
// (target, args_hash, skill_id) matches AND the rule is either
// skill-scope (scope_session_id = ”) or session-scope bound to the
// CURRENT session. Session-scope rules from OTHER sessions never match
// (the HITL-08 cross-session isolation moat). When both a session rule
// and a skill rule match, the session rule wins (ORDER BY ... DESC —
// the non-empty scope_session_id sorts after the empty one in DESC
// order... SQLite sorts ” first in DESC for TEXT, so we order by
// length to make the session-bound row deterministic-first).
const matchRuleSQL = `
SELECT target, args_hash, skill_id, scope_session_id, created_at, last_used_at
FROM approval_rules
WHERE target = ? AND args_hash = ? AND skill_id = ?
  AND (scope_session_id = '' OR scope_session_id = ?)
ORDER BY length(scope_session_id) DESC
LIMIT 1
`

// deleteRuleSQL removes exactly one rule row by full PK. Used ONLY by
// Revoke, paired with the action_rule_revoked audit INSERT in the same
// transaction (D11-D4).
const deleteRuleSQL = `
DELETE FROM approval_rules
WHERE target = ? AND args_hash = ? AND skill_id = ? AND scope_session_id = ?
`

// updateLastUsedAtSQL refreshes the last_used_at column after a rule
// auto-passed a write. Best-effort (fire-and-forget from the rule
// matcher); never on the critical path.
const updateLastUsedAtSQL = `
UPDATE approval_rules SET last_used_at = ?
WHERE target = ? AND args_hash = ? AND skill_id = ? AND scope_session_id = ?
`

// CreateRule persists one approve-and-remember rule. INSERT OR IGNORE:
// an identical existing rule is a no-op success (idempotent re-approve).
// All values are `?` placeholders (T-3-SQLI).
func (s *SQLiteStore) CreateRule(ctx context.Context, rule ApprovalRule) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	created := rule.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	lastUsed := rule.LastUsedAt
	if lastUsed.IsZero() {
		lastUsed = created
	}
	_, err := s.db.ExecContext(ctx, createRuleSQL,
		rule.Target, rule.ArgsHash, rule.SkillID, rule.ScopeSessionID,
		created.Unix(), lastUsed.Unix(),
	)
	if err != nil {
		return fmtError("CreateRule", err)
	}
	return nil
}

// QueryRules lists rules filtered by D11-D3 binary scope:
//
//   - ScopeSession: rules bound to sessionID (scope_session_id = sessionID).
//   - ScopeSkill:   rules bound to skillID across sessions
//     (skill_id = skillID AND scope_session_id = ”).
//
// Any other scope value is rejected — there is no global listing.
func (s *SQLiteStore) QueryRules(ctx context.Context, sessionID, skillID string, scope Scope) ([]ApprovalRule, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	var (
		rows *sql.Rows
		err  error
	)
	switch scope {
	case ScopeSession:
		rows, err = s.db.QueryContext(ctx, queryRulesBySessionScopeSQL, sessionID)
	case ScopeSkill:
		rows, err = s.db.QueryContext(ctx, queryRulesBySkillScopeSQL, skillID)
	default:
		return nil, fmt.Errorf("sqlite.QueryRules: unknown scope %d (D11-D3 binary scope: ScopeSession | ScopeSkill)", scope)
	}
	if err != nil {
		return nil, fmtError("QueryRules", err)
	}
	defer rows.Close()

	out := []ApprovalRule{}
	for rows.Next() {
		r, scanErr := scanApprovalRule(rows)
		if scanErr != nil {
			return nil, fmtError("QueryRules.Scan", scanErr)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("QueryRules.Rows", err)
	}
	return out, nil
}

// MatchRule is the D11-D1 auto-pass lookup. Returns (rule, true, nil)
// when a rule with key (target, argsHash, skillID) exists in either
// skill scope (scope_session_id = ”) or the current session's scope
// (scope_session_id = sessionID). A session-scope rule owned by a
// DIFFERENT session never matches — HITL-08 cross-session isolation.
func (s *SQLiteStore) MatchRule(ctx context.Context, target, argsHash, skillID, sessionID string) (*ApprovalRule, bool, error) {
	if s.isClosed() {
		return nil, false, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, matchRuleSQL, target, argsHash, skillID, sessionID)
	if err != nil {
		return nil, false, fmtError("MatchRule", err)
	}
	defer rows.Close()
	if rows.Next() {
		r, scanErr := scanApprovalRule(rows)
		if scanErr != nil {
			return nil, false, fmtError("MatchRule.Scan", scanErr)
		}
		return &r, true, nil
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmtError("MatchRule.Rows", err)
	}
	return nil, false, nil
}

// Revoke deletes one approval_rules row AND inserts the
// action_rule_revoked audit row in a SINGLE transaction (D11-D4 +
// T-3-AUDIT: the audit table is INSERT-only, so the audit evidence for
// the revoke is the audit row, not the missing rule row).
//
// A DELETE that affects zero rows is an error — revoking a rule that
// does not exist is a caller bug (or a double-revoke race) and must
// surface loudly rather than write an audit row for nothing.
//
// DEVIATION (documented, load-bearing): the 11-05 plan specified a
// `manifest *skill.Manifest` parameter. internal/skill imports
// internal/agent/hitl (validate.go) and internal/agent/hitl imports
// THIS package (rule_matcher.go), so sqlite importing skill would close
// an import cycle (sqlite → skill → hitl → sqlite). The signature
// therefore takes the two snapshot values the audit row needs —
// skillVersion + promptHash — as plain strings; callers that hold a
// manifest (internal/server) derive them above this layer.
func (s *SQLiteStore) Revoke(
	ctx context.Context,
	sessionID, userID string,
	target, argsHash, skillID, scopeSessionID string,
	skillVersion, promptHash string,
	reason string,
) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("Revoke.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, deleteRuleSQL, target, argsHash, skillID, scopeSessionID)
	if err != nil {
		return fmtError("Revoke.Delete", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmtError("Revoke.RowsAffected", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf(
			"sqlite.approval_rules.Revoke: no rule found for PK (target=%q args_hash=%q skill_id=%q scope_session_id=%q)",
			target, argsHash, skillID, scopeSessionID,
		)
	}

	// The audit row's frozen_args captures the revoked rule KEY as a
	// JSON object so the revoke is reconstructible from the audit chain
	// alone (T-3-AUDIT: the rule row is gone; the audit row is the
	// evidence). %q produces JSON-compatible quoting for these fields.
	frozenArgs := []byte(fmt.Sprintf(
		`{"target":%q,"args_hash":%q,"skill_id":%q,"scope_session_id":%q}`,
		target, argsHash, skillID, scopeSessionID,
	))
	toolCallID := fmt.Sprintf("rule-revoke-%s-%d", target, time.Now().UnixNano())
	if err := insertRuleRevokedAuditRow(
		ctx, tx,
		sessionID, userID, skillID, skillVersion, promptHash,
		toolCallID, target, scopeSessionID, frozenArgs, reason,
	); err != nil {
		return fmtError("Revoke.AuditInsert", err)
	}
	return tx.Commit()
}

// UpdateLastUsedAt refreshes a rule's last_used_at timestamp after an
// auto-pass. Best-effort: a missing row is a silent no-op (the rule may
// have been revoked between match and touch — T-11-05-09 accept
// disposition).
func (s *SQLiteStore) UpdateLastUsedAt(ctx context.Context, target, argsHash, skillID, scopeSessionID string) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	_, err := s.db.ExecContext(ctx, updateLastUsedAtSQL,
		time.Now().Unix(), target, argsHash, skillID, scopeSessionID,
	)
	if err != nil {
		return fmtError("UpdateLastUsedAt", err)
	}
	return nil
}

// insertRuleRevokedAuditRow writes the action_rule_revoked audit row
// INSIDE the supplied transaction (D11-D4 single-tx invariant). It
// reuses putRuleRevokedSQL from audit.go (11-03) so the INSERT shape is
// shared with SQLiteStore.MarkRuleRevoked — one SQL constant, two
// call-postures (standalone vs tx-bound).
func insertRuleRevokedAuditRow(
	ctx context.Context,
	tx *sql.Tx,
	sessionID, userID, skillID, skillVersion, promptHash string,
	toolCallID, target, scopeSessionID string,
	frozenArgs []byte,
	reason string,
) error {
	_, err := tx.ExecContext(ctx, putRuleRevokedSQL,
		time.Now().UTC(), sessionID, userID, skillID,
		target, promptHash, skillVersion,
		"",                                 // model_version — n/a for a rule revoke
		"",                                 // verdict_json — n/a
		reason,                             // refusal_reason carries the operator's revoke reason
		"",                                 // chunk_ids — n/a
		frozenArgs, toolCallID, toolCallID, // D11-C4: approval_id == tool_call_id
		scopeSessionID,
		platformfloor.PlatformFloorVersion, // PLAT-10: every audit row carries the floor version
	)
	return err
}

// scanApprovalRule scans one approval_rules row. created_at /
// last_used_at are INTEGER unix seconds in the DB; the struct carries
// time.Time (UTC).
func scanApprovalRule(rows *sql.Rows) (ApprovalRule, error) {
	var (
		r                 ApprovalRule
		created, lastUsed int64
	)
	if err := rows.Scan(
		&r.Target, &r.ArgsHash, &r.SkillID, &r.ScopeSessionID,
		&created, &lastUsed,
	); err != nil {
		return ApprovalRule{}, err
	}
	r.CreatedAt = time.Unix(created, 0).UTC()
	r.LastUsedAt = time.Unix(lastUsed, 0).UTC()
	return r, nil
}

// ApprovalRulesStore is the narrow interface the HITL layer
// (internal/agent/hitl/rule_matcher.go) and the HTTP layer
// (internal/server/approval_rules_api.go) consume. It is satisfied by
// *SQLiteStore; the narrow shape keeps the consumers free of the full
// store.Store surface.
type ApprovalRulesStore interface {
	CreateRule(ctx context.Context, rule ApprovalRule) error
	QueryRules(ctx context.Context, sessionID, skillID string, scope Scope) ([]ApprovalRule, error)
	MatchRule(ctx context.Context, target, argsHash, skillID, sessionID string) (*ApprovalRule, bool, error)
	Revoke(ctx context.Context, sessionID, userID string, target, argsHash, skillID, scopeSessionID string, skillVersion, promptHash string, reason string) error
	UpdateLastUsedAt(ctx context.Context, target, argsHash, skillID, scopeSessionID string) error
}

// Compile-time check that *SQLiteStore satisfies ApprovalRulesStore.
var _ ApprovalRulesStore = (*SQLiteStore)(nil)
