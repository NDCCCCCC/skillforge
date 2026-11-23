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

// approval_rules.go is the persistent substrate for approve-and-remember
// rules, rebuilt in Phase 56 / Plan 56-01 (D-56-01) from the Plan 11-05
// four-tuple-PK form into a policy-entry table. The table is
// INDEPENDENT from the audit table (D11-D2): rule rows are created,
// listed, hit-counted, enabled/disabled, and DELETED here; the audit
// trail of those lifecycle events lives in the audit table (T-3-AUDIT
// INSERT-only — the audit evidence for a revoke is the
// action_rule_revoked audit row, not the missing rule row).
//
// What changed in 56-01 (and what deliberately did not):
//
//   - PK: surrogate autoincrement id (the legacy four-tuple PK cannot
//     be rewritten by a PRAGMA-gated ALTER — schema.go's
//     rebuildLegacyApprovalRules migrates legacy rows). The legacy
//     four-tuple survives as the UNIQUE natural key, with match_type
//     and args_pattern folded in.
//   - Match tiers: exact (args_hash byte-equality, the only legacy
//     tier), prefix (args_pattern locks a JSON-object field subset,
//     naming follows the Claude Code Bash(cmd:*) intuition), template
//     (target-level only). The tier SCORING ENGINE is 56-02's; this
//     plan lands the schema + the candidate SQL (ListActiveRulesForTarget).
//   - Scope: three stored values session | skill | global (replaces the
//     D11-D3 binary scope_session_id-”-vs-nonempty encoding;
//     scope_session_id is retained as the value column for
//     scope='session' rows).
//   - Lifecycle: enabled switch, expires_at TTL, hit_count /
//     last_hit_at statistics, origin provenance
//     (approve_remember | manual | seed).
//   - Deny semantics UNCHANGED (D-56-01): deny / request_change stay
//     one-shot operator decisions — there is NO deny rule, rules only
//     ever auto-pass a write.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/platformfloor"
)

// RuleMatchType enumerates the D-56-01 match tiers. Values mirror the
// approval_rules.match_type CHECK constraint ('exact','prefix',
// 'template') — sqlite is the storage-layer backstop (same posture as
// knowledge_bases.role); HTTP-layer boundary validation lives in
// internal/server/approval_rules_api.go.
const (
	// MatchTypeExact: the incoming tool-call args hash equals the
	// rule's args_hash byte-for-byte (the legacy Plan 11-05 semantics).
	MatchTypeExact = "exact"
	// MatchTypePrefix: args_pattern locks a JSON-object field subset;
	// the remaining fields stay free.
	MatchTypePrefix = "prefix"
	// MatchTypeTemplate: matches at the TARGET level only — every call
	// to the target auto-passes.
	MatchTypeTemplate = "template"
)

// RuleScope is the D-56-01 three-layer scope. It replaces the 11-05
// binary `Scope int` enum (ScopeSession/ScopeSkill — deleted in
// 56-01); the D11-D3 encoding is decoded by the legacy rebuild in
// schema.go and no longer exists at the Go layer.
type RuleScope string

const (
	// RuleScopeSession binds the rule to one session (scope_session_id
	// = that session's ID). Matches only inside that session.
	RuleScopeSession RuleScope = "session"
	// RuleScopeSkill binds the rule to one Skill across sessions
	// (skill_id = the Skill identity, scope_session_id = '').
	RuleScopeSkill RuleScope = "skill"
	// RuleScopeGlobal applies across every Skill and session (the new
	// 56-01 tier the D11-D3 model structurally forbade).
	RuleScopeGlobal RuleScope = "global"
)

// RuleOrigin enumerates where a rule came from. Values mirror the
// approval_rules.origin CHECK constraint.
const (
	// OriginApproveRemember: an operator's approve-and-remember click
	// (the only legacy source — migrated rows carry it).
	OriginApproveRemember = "approve_remember"
	// OriginManual: a panel-authored entry (56-03 wiring).
	OriginManual = "manual"
	// OriginSeed: a seeded enterprise policy entry.
	OriginSeed = "seed"
)

// ApprovalRule is one row of the approval_rules table. The natural key
// (D-56-01) is (Target, MatchType, ArgsHash, ArgsPattern, SkillID,
// Scope, ScopeSessionID) — enforced by idx_approval_rules_natural; the
// stored PK is the surrogate ID.
type ApprovalRule struct {
	// ID is the surrogate primary key. The wire layer packs it into the
	// opaque rule_id handle the DELETE endpoint consumes.
	ID int64

	Target string

	// MatchType is one of MatchTypeExact / MatchTypePrefix /
	// MatchTypeTemplate ('exact' when empty on insert).
	MatchType string

	// ArgsHash is the hex SHA-256 of the frozen tool-call args bytes
	// (D11-D1 rule key — computed by
	// internal/agent/hitl.ComputeFrozenArgsHash, never re-marshaled
	// through encoding/json). The EXACT-tier index column; '' for
	// prefix/template rules.
	ArgsHash string

	// ArgsPattern is the prefix-tier field-lock pattern ('' for
	// exact/template rules).
	ArgsPattern string

	// SkillID is '' for cross-Skill global rules.
	SkillID string

	// Scope is one of the RuleScope values ('session'|'skill'|'global').
	Scope string

	// ScopeSessionID is the originating session ID for scope='session'
	// rules; '' otherwise.
	ScopeSessionID string

	// Enabled gates the rule in every candidate query. A disabled rule
	// stays visible to no listing path (SetRuleEnabled flips it).
	Enabled bool

	// ExpiresAt is the unix-seconds TTL boundary; 0 = never expires.
	ExpiresAt int64

	// Origin is one of the RuleOrigin values ('approve_remember' when
	// empty on insert).
	Origin string

	// HitCount is the cumulative auto-pass count (HitRule increments).
	HitCount int64

	// LastHitAt is the last auto-pass time; the zero value = never hit.
	LastHitAt time.Time

	// CreatedAt is the row creation time.
	CreatedAt time.Time

	// LastUsedAt is the COMPAT column mirrored from LastHitAt by
	// HitRule (one statement writes both) so the pre-56 FE shim keeps
	// rendering until the 56-03 panel switches to last_hit_at.
	LastUsedAt time.Time
}

// RuleHitRow is one row of the approval_rule_hits INSERT-only ledger —
// the per-auto-pass audit line the 56-02 engine appends (InsertRuleHit)
// and the 56-03 panel reads (QueryRuleHits).
type RuleHitRow struct {
	// ID is the ledger row's surrogate key (insertion order == replay
	// order; QueryRuleHits sorts by ts DESC, id DESC).
	ID int64
	// RuleID references approval_rules.id (by value; no FK — the ledger
	// survives revokes so the hit history of a deleted rule stays
	// replayable).
	RuleID int64
	// Timestamp is the auto-pass time (unix seconds in the DB).
	Timestamp time.Time
	// SessionID is the session the auto-pass happened in.
	SessionID string
	// Target is the write target that was auto-passed.
	Target string
	// ToolCallID is the tool-call identity of the auto-passed call
	// ('' when the caller has none).
	ToolCallID string
	// MatchType is the tier that fired (exact|prefix|template).
	MatchType string
	// MatchedArgsHash is the incoming call's args hash (the exact-tier
	// probe value; '' for prefix/template hits).
	MatchedArgsHash string
}

// approvalRuleSelectCols is the canonical SELECT column list shared by
// every read path — scanApprovalRule consumes exactly this order.
const approvalRuleSelectCols = `
    id, target, match_type, args_hash, args_pattern, skill_id,
    scope, scope_session_id, enabled, expires_at, origin,
    hit_count, last_hit_at, created_at, last_used_at
`

// createRuleSQL is INSERT OR IGNORE over every rule-authoring column:
// re-approving an identical natural-key tuple refreshes nothing and
// errors nothing — the rule already exists, which is the desired end
// state (idempotent re-approve). hit_count / last_hit_at keep their
// DEFAULTs (a fresh rule has never fired).
const createRuleSQL = `
INSERT OR IGNORE INTO approval_rules (
    target, match_type, args_hash, args_pattern, skill_id,
    scope, scope_session_id, enabled, expires_at, origin,
    created_at, last_used_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// queryRulesBySessionScopeSQL lists the rules visible inside ONE
// session. The expires predicate makes the panel listing an ACTIVE
// rules view by default (expired rules are history, not policy).
const queryRulesBySessionScopeSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE scope = 'session' AND scope_session_id = ?
  AND (expires_at IS NULL OR expires_at > strftime('%s','now'))
ORDER BY created_at DESC
`

// queryRulesBySkillScopeSQL lists the rules bound to ONE Skill across
// sessions.
const queryRulesBySkillScopeSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE scope = 'skill' AND skill_id = ?
  AND (expires_at IS NULL OR expires_at > strftime('%s','now'))
ORDER BY created_at DESC
`

// queryRulesByGlobalScopeSQL lists the cross-Skill rules (the new
// D-56-01 tier — the D11-D3 model structurally forbade it).
const queryRulesByGlobalScopeSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE scope = 'global'
  AND (expires_at IS NULL OR expires_at > strftime('%s','now'))
ORDER BY created_at DESC
`

// matchRuleSQL is the LEGACY exact-tier auto-pass lookup, retained
// (updated to the new shape) for the transitional SqlRuleMatcher path
// (internal/agent/hitl/rule_matcher.go) until the 56-02 engine takes
// over. Semantics preserved from 11-05: exact args_hash equality,
// skill-or-session scope, cross-session isolation — EXTENDED with the
// enabled gate + TTL predicate so a disabled or expired rule can never
// auto-pass (fail-closed direction: fewer auto-passes, more human
// review). DEPRECATED for removal once 56-02 switches the matcher to
// ListActiveRulesForTarget + tier scoring.
const matchRuleSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE target = ? AND args_hash = ? AND skill_id = ?
  AND enabled = 1
  AND (expires_at IS NULL OR expires_at > strftime('%s','now'))
  AND (scope_session_id = '' OR scope_session_id = ?)
ORDER BY length(scope_session_id) DESC
LIMIT 1
`

// listActiveRulesForTargetSQL is the D-56-01 candidate fetch the 56-02
// matching engine consumes (as MatchRuleForArgs — the tier scoring
// lives there; SQL only filters candidates). It returns every ENABLED,
// UNEXPIRED rule visible to (sessionID, skillID) for one target across
// all three scope layers, deterministic-ordered by scope priority
// (session > skill > global — a more specific rule sorts first) with
// id ASC as the stable tie-break. All values are `?` placeholders
// (T-3-SQLI).
const listActiveRulesForTargetSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE target = ?
  AND enabled = 1
  AND (expires_at IS NULL OR expires_at > strftime('%s','now'))
  AND (
        (scope = 'session' AND scope_session_id = ?)
     OR (scope = 'skill'   AND skill_id = ?)
     OR  scope = 'global'
  )
ORDER BY CASE scope WHEN 'session' THEN 0 WHEN 'skill' THEN 1 ELSE 2 END, id ASC
`

// selectRuleByIDSQL fetches one rule row inside the RevokeByID
// transaction — the revoke's audit row snapshots the rule's key and
// new 56-01 fields so the revoke stays reconstructible from the audit
// chain alone (T-3-AUDIT).
const selectRuleByIDSQL = `
SELECT ` + approvalRuleSelectCols + `
FROM approval_rules
WHERE id = ?
`

// deleteRuleByIDSQL removes exactly one rule row by surrogate PK. Used
// ONLY by RevokeByID, paired with the action_rule_revoked audit INSERT
// in the same transaction (D11-D4 + T-3-AUDIT).
const deleteRuleByIDSQL = `
DELETE FROM approval_rules WHERE id = ?
`

// hitRuleSQL is the one-statement hit accounting: increment the
// counter and stamp BOTH last_hit_at (new canonical) and last_used_at
// (compat shim) from the same unix-second value, so the two columns
// can never disagree (56-03 switches FE to last_hit_at; last_used_at
// retires next milestone).
const hitRuleSQL = `
UPDATE approval_rules
SET hit_count = hit_count + 1, last_hit_at = ?, last_used_at = ?
WHERE id = ?
`

// setRuleEnabledSQL / setRuleExpirySQL are the two panel PATCH
// primitives (56-03 endpoints consume them; the endpoint layer owns
// the audit rows for those mutations).
const setRuleEnabledSQL = `
UPDATE approval_rules SET enabled = ? WHERE id = ?
`

const setRuleExpirySQL = `
UPDATE approval_rules SET expires_at = ? WHERE id = ?
`

// insertRuleHitSQL appends one row to the INSERT-only hit ledger.
const insertRuleHitSQL = `
INSERT INTO approval_rule_hits (
    rule_id, ts, session_id, target, tool_call_id, match_type, matched_args_hash
) VALUES (?, ?, ?, ?, ?, ?, ?)
`

// queryRuleHitsSQL reads one rule's hit history, newest first. LIMIT ?
// is parameterized (SQLite accepts a bound LIMIT).
const queryRuleHitsSQL = `
SELECT id, rule_id, ts, session_id, target, tool_call_id, match_type, matched_args_hash
FROM approval_rule_hits
WHERE rule_id = ?
ORDER BY ts DESC, id DESC
LIMIT ?
`

// normalizeRule derives the D-56-01 defaults for the zero-value struct
// fields: MatchType ” → exact, Scope ” → the D11-D3 legacy encoding
// (scope_session_id non-empty → session, else skill — keeps the
// pre-56 call shape valid), Origin ” → approve_remember. Enum values
// that ARE set but invalid are not second-guessed here — the
// approval_rules CHECK constraints refuse them at INSERT time.
func (r ApprovalRule) normalizeRule() ApprovalRule {
	if r.MatchType == "" {
		r.MatchType = MatchTypeExact
	}
	if r.Scope == "" {
		if r.ScopeSessionID != "" {
			r.Scope = string(RuleScopeSession)
		} else {
			r.Scope = string(RuleScopeSkill)
		}
	}
	if r.Origin == "" {
		r.Origin = OriginApproveRemember
	}
	return r
}

// CreateRule persists one approve-and-remember rule. INSERT OR IGNORE:
// an identical existing natural-key rule is a no-op success (idempotent
// re-approve). All values are `?` placeholders (T-3-SQLI).
//
// Zero-value policy (56-02 creation wiring reads this): MatchType ” →
// exact; Scope ” → derived from ScopeSessionID (non-empty → session,
// empty → skill — the D11-D3 encoding); Origin ” → approve_remember.
// Enabled is an explicit bool — the zero value inserts a DISABLED rule,
// so callers MUST set Enabled: true for a live rule. Invalid enum
// values are refused by the approval_rules CHECK constraints, not
// silently coerced.
func (s *SQLiteStore) CreateRule(ctx context.Context, rule ApprovalRule) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	rule = rule.normalizeRule()
	created := rule.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	lastUsed := rule.LastUsedAt
	if lastUsed.IsZero() {
		lastUsed = created
	}
	_, err := s.db.ExecContext(ctx, createRuleSQL,
		rule.Target, rule.MatchType, rule.ArgsHash, rule.ArgsPattern, rule.SkillID,
		rule.Scope, rule.ScopeSessionID, boolToInt(rule.Enabled), nullableUnix(rule.ExpiresAt), rule.Origin,
		created.Unix(), lastUsed.Unix(),
	)
	if err != nil {
		return fmtError("CreateRule", err)
	}
	return nil
}

// nullableUnix maps the ExpiresAt convention (0 = permanent) onto the
// NULLable INTEGER column.
func nullableUnix(unix int64) any {
	if unix <= 0 {
		return nil
	}
	return unix
}

// QueryRules lists rules filtered by D-56-01 scope:
//
//   - RuleScopeSession: rules bound to sessionID (scope='session' AND
//     scope_session_id = sessionID) — the wire-compatible semantics of
//     the 11-05 GET ?scope=session.
//   - RuleScopeSkill:   rules bound to skillID across sessions — the
//     wire-compatible semantics of the 11-05 GET ?scope=skill.
//   - RuleScopeGlobal:  cross-Skill rules (ignores both id args).
//
// Expired rules are filtered out in ALL three listings — the panel
// listing is an ACTIVE-rules view by default. Any other scope value is
// rejected (boundary validation lives at the HTTP layer; the store
// still refuses unknown values as defense in depth).
func (s *SQLiteStore) QueryRules(ctx context.Context, sessionID, skillID string, scope RuleScope) ([]ApprovalRule, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	var (
		rows *sql.Rows
		err  error
	)
	switch scope {
	case RuleScopeSession:
		rows, err = s.db.QueryContext(ctx, queryRulesBySessionScopeSQL, sessionID)
	case RuleScopeSkill:
		rows, err = s.db.QueryContext(ctx, queryRulesBySkillScopeSQL, skillID)
	case RuleScopeGlobal:
		rows, err = s.db.QueryContext(ctx, queryRulesByGlobalScopeSQL)
	default:
		return nil, fmt.Errorf(
			"sqlite.QueryRules: unknown scope %q (D-56-01 three-layer scope: session | skill | global)",
			string(scope),
		)
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

// MatchRule is the LEGACY exact-tier auto-pass lookup, retained for the
// transitional SqlRuleMatcher path (see matchRuleSQL). Returns
// (rule, true, nil) when an enabled, unexpired rule with key
// (target, argsHash, skillID) exists in either skill scope or the
// current session's scope. A session-scope rule owned by a DIFFERENT
// session never matches — HITL-08 cross-session isolation. DEPRECATED
// alongside matchRuleSQL: 56-02's engine replaces it.
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

// ListActiveRulesForTarget returns every enabled, unexpired rule
// visible to (sessionID, skillID) for one target across all three
// scope layers, ordered session > skill > global (see
// listActiveRulesForTargetSQL). This is the candidate fetch the 56-02
// matching engine wraps as MatchRuleForArgs — the exact/prefix/
// template scoring lives in that engine layer, NOT here; this plan
// lands the SQL + scan only.
func (s *SQLiteStore) ListActiveRulesForTarget(ctx context.Context, target, skillID, sessionID string) ([]ApprovalRule, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, listActiveRulesForTargetSQL, target, sessionID, skillID)
	if err != nil {
		return nil, fmtError("ListActiveRulesForTarget", err)
	}
	defer rows.Close()

	out := []ApprovalRule{}
	for rows.Next() {
		r, scanErr := scanApprovalRule(rows)
		if scanErr != nil {
			return nil, fmtError("ListActiveRulesForTarget.Scan", scanErr)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListActiveRulesForTarget.Rows", err)
	}
	return out, nil
}

// RevokeByID deletes one approval_rules row by surrogate PK AND
// inserts the action_rule_revoked audit row in a SINGLE transaction
// (D11-D4 + T-3-AUDIT: the audit table is INSERT-only, so the audit
// evidence for the revoke is the audit row, not the missing rule row).
//
// Before the DELETE the rule row is SELECTed inside the same
// transaction so the audit row's frozen_args can snapshot the FULL
// rule identity — the legacy PK fields (target/args_hash/skill_id/
// scope_session_id) PLUS the 56-01 fields (rule_id/match_type/scope) —
// keeping the revoke evidence complete under the new shape
// (internal/auditchain decodes the legacy subset and ignores the new
// keys).
//
// A DELETE that affects zero rows is an error — revoking a rule that
// does not exist is a caller bug (or a double-revoke race) and must
// surface loudly rather than write an audit row for nothing. The error
// text carries the literal "no rule found" (the HTTP layer's 404
// classifier matches it).
//
// The skillVersion / promptHash / reason parameters keep the documented
// 11-05 deviation (see the Revoke godoc in git history): internal/skill
// imports hitl which imports THIS package, so the signature takes the
// audit snapshot values as plain strings; callers that hold a manifest
// (internal/server) derive them above this layer.
func (s *SQLiteStore) RevokeByID(
	ctx context.Context,
	sessionID, userID string,
	ruleID int64,
	skillVersion, promptHash string,
	reason string,
) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("RevokeByID.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Snapshot the row first — after the DELETE it is gone, and the
	// audit row must outlive the rule (T-3-AUDIT).
	var revoked ApprovalRule
	row := tx.QueryRowContext(ctx, selectRuleByIDSQL, ruleID)
	revoked, err = scanApprovalRuleRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf(
				"sqlite.approval_rules.RevokeByID: no rule found for rule_id=%d", ruleID,
			)
		}
		return fmtError("RevokeByID.Select", err)
	}

	res, err := tx.ExecContext(ctx, deleteRuleByIDSQL, ruleID)
	if err != nil {
		return fmtError("RevokeByID.Delete", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmtError("RevokeByID.RowsAffected", err)
	}
	if rowsAffected == 0 {
		// Defensive: the SELECT above already proved existence; a 0-row
		// DELETE under the same tx would be a driver anomaly.
		return fmt.Errorf(
			"sqlite.approval_rules.RevokeByID: no rule found for rule_id=%d (delete affected 0 rows)", ruleID,
		)
	}

	// The audit row's frozen_args captures the revoked rule identity as
	// a JSON object — the legacy PK subset (what internal/auditchain's
	// rulePKPayload decodes) plus the 56-01 fields so the revoke is
	// reconstructible from the audit chain alone. %q produces
	// JSON-compatible quoting for the string fields.
	frozenArgs := fmt.Appendf(nil,
		`{"target":%q,"args_hash":%q,"skill_id":%q,"scope_session_id":%q,"rule_id":%d,"match_type":%q,"scope":%q}`,
		revoked.Target, revoked.ArgsHash, revoked.SkillID, revoked.ScopeSessionID,
		ruleID, revoked.MatchType, revoked.Scope,
	)
	toolCallID := fmt.Sprintf("rule-revoke-%s-%d", revoked.Target, time.Now().UnixNano())
	if err := insertRuleRevokedAuditRow(
		ctx, tx,
		sessionID, userID, revoked.SkillID, skillVersion, promptHash,
		toolCallID, revoked.Target, revoked.ScopeSessionID, frozenArgs, reason,
	); err != nil {
		return fmtError("RevokeByID.AuditInsert", err)
	}
	return tx.Commit()
}

// UpdateLastUsedAt refreshes a rule's last_used_at timestamp after an
// auto-pass. Best-effort: a missing row is a silent no-op (the rule may
// have been revoked between match and touch — T-11-05-09 accept
// disposition).
//
// DEPRECATED (56-01): HitRule replaces this — it writes last_used_at
// AND last_hit_at AND hit_count in one statement. Retained only for
// the transitional SqlRuleMatcher hook; delete after 56-02 switches
// the matcher (by 56-05).
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

// updateLastUsedAtSQL is the legacy PK-keyed touch retained for
// UpdateLastUsedAt (DEPRECATED — see above; delete with it in 56-05).
const updateLastUsedAtSQL = `
UPDATE approval_rules SET last_used_at = ?
WHERE target = ? AND args_hash = ? AND skill_id = ? AND scope_session_id = ?
`

// HitRule records one auto-pass against a rule: hit_count + 1 and a
// single unix-second stamp written to BOTH last_hit_at (canonical) and
// last_used_at (compat shim) so the two columns never disagree. The
// write wiring (56-02 engine) is intentionally NOT connected in 56-01 —
// this plan ships the schema + method only.
func (s *SQLiteStore) HitRule(ctx context.Context, ruleID int64) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, hitRuleSQL, now, now, ruleID)
	if err != nil {
		return fmtError("HitRule", err)
	}
	return nil
}

// InsertRuleHit appends one row to the approval_rule_hits INSERT-only
// ledger. A zero Timestamp defaults to now. Write wiring is 56-02's
// (this plan ships schema + method only).
func (s *SQLiteStore) InsertRuleHit(ctx context.Context, hit RuleHitRow) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	ts := hit.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, insertRuleHitSQL,
		hit.RuleID, ts.Unix(), hit.SessionID, hit.Target,
		hit.ToolCallID, hit.MatchType, hit.MatchedArgsHash,
	)
	if err != nil {
		return fmtError("InsertRuleHit", err)
	}
	return nil
}

// QueryRuleHits reads one rule's hit history, newest first (the 56-03
// panel's hit-history view). A limit <= 0 defaults to 50.
func (s *SQLiteStore) QueryRuleHits(ctx context.Context, ruleID int64, limit int) ([]RuleHitRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, queryRuleHitsSQL, ruleID, limit)
	if err != nil {
		return nil, fmtError("QueryRuleHits", err)
	}
	defer rows.Close()

	out := []RuleHitRow{}
	for rows.Next() {
		var (
			h  RuleHitRow
			ts int64
		)
		if err := rows.Scan(
			&h.ID, &h.RuleID, &ts, &h.SessionID, &h.Target,
			&h.ToolCallID, &h.MatchType, &h.MatchedArgsHash,
		); err != nil {
			return nil, fmtError("QueryRuleHits.Scan", err)
		}
		h.Timestamp = time.Unix(ts, 0).UTC()
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("QueryRuleHits.Rows", err)
	}
	return out, nil
}

// SetRuleEnabled flips the rule's enabled gate (56-03 PATCH endpoint
// consumes; audit rows for the mutation belong to the endpoint layer).
func (s *SQLiteStore) SetRuleEnabled(ctx context.Context, ruleID int64, enabled bool) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	_, err := s.db.ExecContext(ctx, setRuleEnabledSQL, boolToInt(enabled), ruleID)
	if err != nil {
		return fmtError("SetRuleEnabled", err)
	}
	return nil
}

// SetRuleExpiry sets the rule's TTL boundary in unix seconds
// (expiresAtUnix <= 0 clears the expiry = never expires). 56-03 PATCH
// endpoint consumes; audit rows belong to the endpoint layer.
func (s *SQLiteStore) SetRuleExpiry(ctx context.Context, ruleID int64, expiresAtUnix int64) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	_, err := s.db.ExecContext(ctx, setRuleExpirySQL, nullableUnix(expiresAtUnix), ruleID)
	if err != nil {
		return fmtError("SetRuleExpiry", err)
	}
	return nil
}

// insertRuleRevokedAuditRow writes the action_rule_revoked audit row
// INSIDE the supplied transaction (D11-D4 single-tx invariant). It
// reuses putAuditResolutionSQL from audit.go (11-03) so the INSERT
// shape is shared with SQLiteStore.MarkRuleRevoked — one SQL constant,
// two call-postures (standalone vs tx-bound).
func insertRuleRevokedAuditRow(
	ctx context.Context,
	tx *sql.Tx,
	sessionID, userID, skillID, skillVersion, promptHash string,
	toolCallID, target, scopeSessionID string,
	frozenArgs []byte,
	reason string,
) error {
	_, err := tx.ExecContext(ctx, putAuditResolutionSQL,
		time.Now().UTC(), sessionID, userID, skillID,
		"action_rule_revoked",
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

// sqlScanner is the minimal interface shared by *sql.Rows and
// *sql.Row, letting one scan implementation serve both the multi-row
// read paths and RevokeByID's single-row snapshot.
type sqlScanner interface {
	Scan(dest ...any) error
}

// scanApprovalRule scans one approval_rules row from a *sql.Rows.
// enabled is INTEGER 0/1; expires_at / last_hit_at are NULLable unix
// seconds (NULL → ExpiresAt 0 / zero LastHitAt).
func scanApprovalRule(rows *sql.Rows) (ApprovalRule, error) {
	return scanApprovalRuleRow(rows)
}

// scanApprovalRuleRow scans one approval_rules row from any
// sqlScanner. Integer columns are scanned into explicit int holders
// (never straight into *bool — driver-dependent convertAssign for
// INTEGER→bool is exactly the kind of subtlety this store does not
// gamble on); unix seconds convert to UTC time.Time at the boundary.
func scanApprovalRuleRow(sc sqlScanner) (ApprovalRule, error) {
	var (
		r                 ApprovalRule
		enabled           int
		expiresAt         sql.NullInt64
		lastHitAt         sql.NullInt64
		created, lastUsed int64
	)
	if err := sc.Scan(
		&r.ID, &r.Target, &r.MatchType, &r.ArgsHash, &r.ArgsPattern, &r.SkillID,
		&r.Scope, &r.ScopeSessionID, &enabled, &expiresAt, &r.Origin,
		&r.HitCount, &lastHitAt, &created, &lastUsed,
	); err != nil {
		return ApprovalRule{}, err
	}
	r.Enabled = enabled == 1
	if expiresAt.Valid {
		r.ExpiresAt = expiresAt.Int64
	}
	if lastHitAt.Valid {
		r.LastHitAt = time.Unix(lastHitAt.Int64, 0).UTC()
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
//
// 56-01 surface changes:
//   - QueryRules takes RuleScope (three-layer string) — the binary
//     `Scope int` enum is gone.
//   - Revoke replaces Revoke (full-PK keyed) with RevokeByID
//     (surrogate-PK keyed).
//   - New: ListActiveRulesForTarget (56-02 engine candidate fetch),
//     HitRule / InsertRuleHit / QueryRuleHits (hit accounting; write
//     wiring in 56-02), SetRuleEnabled / SetRuleExpiry (56-03 PATCH).
//   - MatchRule + UpdateLastUsedAt are the retained transitional pair
//     (DEPRECATED; delete after 56-02 switches the matcher, by 56-05).
type ApprovalRulesStore interface {
	CreateRule(ctx context.Context, rule ApprovalRule) error
	QueryRules(ctx context.Context, sessionID, skillID string, scope RuleScope) ([]ApprovalRule, error)
	MatchRule(ctx context.Context, target, argsHash, skillID, sessionID string) (*ApprovalRule, bool, error)
	ListActiveRulesForTarget(ctx context.Context, target, skillID, sessionID string) ([]ApprovalRule, error)
	RevokeByID(ctx context.Context, sessionID, userID string, ruleID int64, skillVersion, promptHash string, reason string) error
	UpdateLastUsedAt(ctx context.Context, target, argsHash, skillID, scopeSessionID string) error
	HitRule(ctx context.Context, ruleID int64) error
	InsertRuleHit(ctx context.Context, hit RuleHitRow) error
	QueryRuleHits(ctx context.Context, ruleID int64, limit int) ([]RuleHitRow, error)
	SetRuleEnabled(ctx context.Context, ruleID int64, enabled bool) error
	SetRuleExpiry(ctx context.Context, ruleID int64, expiresAtUnix int64) error
}

// Compile-time check that *SQLiteStore satisfies ApprovalRulesStore.
var _ ApprovalRulesStore = (*SQLiteStore)(nil)
