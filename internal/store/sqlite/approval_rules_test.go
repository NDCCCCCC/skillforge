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

package sqlite_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRuleTestStore opens a file-backed temp store (mirrors the
// audit_lifecycle_test.go pattern — :memory: with MaxOpenConns>1 would
// give every connection its own private DB, and the RevokeByID
// transaction needs a single connection).
func newRuleTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "rules.db")
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: dsn, MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// createTestRule inserts one rule with 56-01 defaults, applying the
// caller's overrides, and asserts the insert succeeded. Returns the
// rule AS INSERTED (caller-supplied fields, normalized like
// CreateRule's zero-value policy).
func createTestRule(t *testing.T, s *sqlite.SQLiteStore, mod func(*sqlite.ApprovalRule)) sqlite.ApprovalRule {
	t.Helper()
	rule := sqlite.ApprovalRule{
		Target:    "memory:facts",
		ArgsHash:  "abc123",
		SkillID:   "compliance",
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if mod != nil {
		mod(&rule)
	}
	require.NoError(t, s.CreateRule(context.Background(), rule))
	return rule
}

// TestApprovalRulesScopeThreeLayer — D-56-01 three-layer scope + the
// D11-D1 exact key, locked through BOTH the legacy MatchRule
// transitional path and the new ListActiveRulesForTarget candidate
// fetch.
//
// Locks:
//  1. A session-scope rule does NOT match inside a DIFFERENT session
//     (HITL-08 cross-session isolation — T-11-05-03, carried into the
//     three-scope model).
//  2. A session-scope rule MATCHES inside its originating session.
//  3. A skill-scope rule MATCHES across sessions for the same Skill
//     and never for a different skill_id.
//  4. A global rule is visible from ANY session and ANY skill
//     (the tier D11-D3 structurally forbade).
//  5. QueryRules serves all three scope values and rejects unknown
//     ones.
func TestApprovalRulesScopeThreeLayer(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()
	argsHash := "abc123"

	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = argsHash
		r.Scope = string(sqlite.RuleScopeSession)
		r.ScopeSessionID = "sess-A"
	})

	// (1) + (2) — legacy transitional path honors session isolation.
	_, matched, err := s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-B")
	require.NoError(t, err)
	assert.False(t, matched, "session-scope rule MUST NOT match in different session")

	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-A")
	require.NoError(t, err)
	assert.True(t, matched, "session-scope rule MUST match in originating session")

	// (3) — skill-scope across sessions.
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = argsHash
		r.Scope = string(sqlite.RuleScopeSkill)
		r.ScopeSessionID = ""
	})
	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-B")
	require.NoError(t, err)
	assert.True(t, matched, "skill-scope rule MUST match across sessions for the same Skill")

	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "other-skill", "sess-A")
	require.NoError(t, err)
	assert.False(t, matched, "rule MUST NOT match when skill_id differs")

	// (4) — the global tier via the new candidate fetch.
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.Target = "review_report"
		r.ArgsHash = "glob-hash"
		r.SkillID = "" // cross-Skill
		r.Scope = string(sqlite.RuleScopeGlobal)
		r.ScopeSessionID = ""
	})

	// (5) — QueryRules three values + unknown rejection.
	sessionRules, err := s.QueryRules(ctx, "sess-A", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	assert.Len(t, sessionRules, 1, "sess-A sees exactly its own session-scope rule")

	skillRules, err := s.QueryRules(ctx, "", "compliance", sqlite.RuleScopeSkill)
	require.NoError(t, err)
	assert.Len(t, skillRules, 1, "compliance sees exactly its skill-scope rule")

	globalRules, err := s.QueryRules(ctx, "", "", sqlite.RuleScopeGlobal)
	require.NoError(t, err)
	assert.Len(t, globalRules, 1, "the global listing returns the cross-Skill rule")

	_, err = s.QueryRules(ctx, "sess-A", "compliance", sqlite.RuleScope("workspace"))
	assert.Error(t, err, "unknown scope MUST be rejected (closed three-value set)")

	// ListActiveRulesForTarget — the 56-02 engine's candidate fetch:
	// scope-layer visibility per (sessionID, skillID).
	cands, err := s.ListActiveRulesForTarget(ctx, "memory:facts", "compliance", "sess-A")
	require.NoError(t, err)
	assert.Len(t, cands, 2, "sess-A+compliance sees its session rule AND the skill rule")
	if assert.Len(t, cands, 2) {
		// Deterministic priority order: session > skill > global.
		assert.Equal(t, string(sqlite.RuleScopeSession), cands[0].Scope, "session-scope candidate sorts first")
		assert.Equal(t, string(sqlite.RuleScopeSkill), cands[1].Scope, "skill-scope candidate sorts second")
	}

	cands, err = s.ListActiveRulesForTarget(ctx, "memory:facts", "compliance", "sess-B")
	require.NoError(t, err)
	assert.Len(t, cands, 1, "sess-B sees only the skill rule (session isolation)")
	assert.Equal(t, string(sqlite.RuleScopeSkill), cands[0].Scope)

	cands, err = s.ListActiveRulesForTarget(ctx, "review_report", "any-skill", "sess-B")
	require.NoError(t, err)
	assert.Len(t, cands, 1, "the global rule is visible from any session/skill")
	assert.Equal(t, string(sqlite.RuleScopeGlobal), cands[0].Scope)
	assert.Equal(t, "", cands[0].SkillID, "global rules carry no skill binding")

	cands, err = s.ListActiveRulesForTarget(ctx, "plan_approve", "compliance", "sess-A")
	require.NoError(t, err)
	assert.Empty(t, cands, "no rules exist for an unrelated target")
}

// TestApprovalRulesCreateIdempotent — INSERT OR IGNORE semantics on the
// new natural key + the CreateRule zero-value policy.
func TestApprovalRulesCreateIdempotent(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()

	// Zero-value policy: MatchType '' → exact, Scope '' → derived from
	// ScopeSessionID (non-empty → session), Origin '' →
	// approve_remember.
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ScopeSessionID = "sess-1"
		r.Scope = "" // derive → session
	})
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ScopeSessionID = "sess-1"
		r.Scope = "" // identical natural key → INSERT OR IGNORE no-op
	})

	rules, err := s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1, "identical re-approve must stay a single row")
	assert.Equal(t, "exact", rules[0].MatchType, "zero MatchType defaults to exact")
	assert.Equal(t, "session", rules[0].Scope, "non-empty ScopeSessionID derives session scope")
	assert.Equal(t, "approve_remember", rules[0].Origin, "zero Origin defaults to approve_remember")
	assert.True(t, rules[0].Enabled, "explicitly enabled rule stays enabled")

	// A DIFFERENT args_pattern widens the natural key → a second row
	// (the prefix tier's coexistence with its exact sibling).
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = ""
		r.ArgsPattern = `{"city":"*"}`
		r.SkillID = ""
		r.Scope = string(sqlite.RuleScopeGlobal)
	})
	globals, err := s.QueryRules(ctx, "", "", sqlite.RuleScopeGlobal)
	require.NoError(t, err)
	require.Len(t, globals, 1)
	assert.Equal(t, `{"city":"*"}`, globals[0].ArgsPattern)
	assert.Equal(t, "", globals[0].ArgsHash, "prefix rules carry no args_hash")
}

// TestApprovalRulesRevokeByID — D11-D4 on the surrogate PK: SELECT
// snapshot + DELETE + action_rule_revoked audit INSERT in ONE
// transaction; the audit row's frozen_args now carries the 56-01
// fields (rule_id / match_type / scope) alongside the legacy PK subset
// the auditchain decoder reads.
func TestApprovalRulesRevokeByID(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()

	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = "revoke-hash-1"
		r.Scope = string(sqlite.RuleScopeSession)
		r.ScopeSessionID = "sess-1"
	})

	rules, err := s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	ruleID := rules[0].ID
	require.Greater(t, ruleID, int64(0), "surrogate PK is assigned on insert")

	require.NoError(t, s.RevokeByID(ctx, "sess-1", "user-1", ruleID, "v1", "prompt-hash-1", "panel revoke"))

	rules, err = s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	assert.Empty(t, rules, "rule row MUST be DELETEd after RevokeByID")

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-1", Limit: 10})
	require.NoError(t, err)
	var foundRuleRevoke bool
	for _, r := range rows {
		if r.Action == "action_rule_revoked" && r.ToolCallID != "" {
			foundRuleRevoke = true
			// Legacy PK subset — what internal/auditchain decodes.
			assert.Contains(t, string(r.FrozenArgs), "revoke-hash-1")
			assert.Contains(t, string(r.FrozenArgs), "memory:facts")
			// 56-01 fields — the revoke evidence is complete under
			// the new shape.
			assert.Contains(t, string(r.FrozenArgs), `"rule_id"`)
			assert.Contains(t, string(r.FrozenArgs), `"match_type"`)
			assert.Contains(t, string(r.FrozenArgs), `"scope"`)
			assert.Equal(t, "panel revoke", r.RefusalReason,
				"the operator's revoke reason travels on the audit row")
			assert.Equal(t, "user-1", r.UserID)
		}
	}
	assert.True(t, foundRuleRevoke, "action_rule_revoked audit row MUST exist after RevokeByID")

	// Double revoke: the row is gone → the zero-row DELETE surfaces
	// loudly (the literal the HTTP layer's 404 classifier matches).
	err = s.RevokeByID(ctx, "sess-1", "user-1", ruleID, "v1", "prompt-hash-1", "double revoke")
	require.Error(t, err, "RevokeByID on non-existent rule MUST error")
	assert.True(t, strings.Contains(err.Error(), "no rule found"),
		"the error must carry the literal 'no rule found' for the wire 404 classifier, got: %v", err)
}

// TestRuleHitAccounting — HitRule one-statement accounting
// (hit_count++ and last_hit_at/last_used_at stamped from the SAME
// unix second) + the approval_rule_hits INSERT-only ledger round-trip.
// The ledger survives a revoke (rule_id is stored by value, no FK) so
// the hit history of a deleted rule stays replayable.
func TestRuleHitAccounting(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()

	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = "hit-hash-1"
		r.Scope = string(sqlite.RuleScopeSession)
		r.ScopeSessionID = "sess-hit"
	})
	rules, err := s.QueryRules(ctx, "sess-hit", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	ruleID := rules[0].ID
	assert.Equal(t, int64(0), rules[0].HitCount, "a fresh rule has never fired")
	assert.True(t, rules[0].LastHitAt.IsZero(), "a fresh rule has never fired")

	require.NoError(t, s.HitRule(ctx, ruleID))
	require.NoError(t, s.HitRule(ctx, ruleID))

	rules, err = s.QueryRules(ctx, "sess-hit", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, int64(2), rules[0].HitCount, "HitRule increments the counter")
	assert.False(t, rules[0].LastHitAt.IsZero(), "HitRule stamps last_hit_at")
	assert.Equal(t, rules[0].LastHitAt.Unix(), rules[0].LastUsedAt.Unix(),
		"last_hit_at and the last_used_at compat shim are written from the SAME value")

	// Ledger round-trip: two rows, newest first.
	require.NoError(t, s.InsertRuleHit(ctx, sqlite.RuleHitRow{
		RuleID: ruleID, Timestamp: time.Unix(1000, 0).UTC(),
		SessionID: "sess-hit", Target: "memory:facts",
		ToolCallID: "tc-1", MatchType: "exact", MatchedArgsHash: "hit-hash-1",
	}))
	require.NoError(t, s.InsertRuleHit(ctx, sqlite.RuleHitRow{
		RuleID: ruleID, Timestamp: time.Unix(2000, 0).UTC(),
		SessionID: "sess-hit", Target: "memory:facts",
		ToolCallID: "tc-2", MatchType: "exact", MatchedArgsHash: "hit-hash-1",
	}))

	hits, err := s.QueryRuleHits(ctx, ruleID, 0) // 0 → default limit
	require.NoError(t, err)
	require.Len(t, hits, 2)
	assert.Equal(t, "tc-2", hits[0].ToolCallID, "newest hit sorts first (ts DESC)")
	assert.Equal(t, "tc-1", hits[1].ToolCallID)
	assert.Equal(t, "hit-hash-1", hits[0].MatchedArgsHash)
	assert.Equal(t, time.Unix(2000, 0).UTC().Unix(), hits[0].Timestamp.Unix())

	hits, err = s.QueryRuleHits(ctx, ruleID, 1)
	require.NoError(t, err)
	assert.Len(t, hits, 1, "the panel's limit parameter is honored")

	// The ledger is independent of the rule row: revoke the rule, the
	// hit history survives (rule_id by value, no FK).
	require.NoError(t, s.RevokeByID(ctx, "sess-hit", "user-1", ruleID, "v1", "ph", "revoke"))
	hits, err = s.QueryRuleHits(ctx, ruleID, 0)
	require.NoError(t, err)
	assert.Len(t, hits, 2, "hit ledger rows MUST survive the rule revoke (INSERT-only ledger)")
}

// TestRuleEnabledAndExpiry — the lifecycle gates:
//
//   - a disabled rule stays LISTABLE (the 56-03 panel must see it to
//     re-enable it) but never enters the active candidate fetch;
//   - an expired rule leaves BOTH the listing and the candidate fetch
//     (the listing is an ACTIVE-rules view — expires_at filters by
//     default);
//   - SetRuleExpiry(0) clears the TTL (back to permanent).
func TestRuleEnabledAndExpiry(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()

	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = "gate-hash-1"
		r.Scope = string(sqlite.RuleScopeSkill)
		r.Enabled = false // disabled on insert
	})
	createTestRule(t, s, func(r *sqlite.ApprovalRule) {
		r.ArgsHash = "gate-hash-2"
		r.Scope = string(sqlite.RuleScopeSkill)
		r.ExpiresAt = time.Now().Add(-time.Hour).Unix() // already expired
	})

	// Disabled: listable, not a candidate.
	rules, err := s.QueryRules(ctx, "", "compliance", sqlite.RuleScopeSkill)
	require.NoError(t, err)
	assert.Len(t, rules, 1, "the disabled rule stays visible to the panel listing")
	assert.False(t, rules[0].Enabled)

	cands, err := s.ListActiveRulesForTarget(ctx, "memory:facts", "compliance", "sess-x")
	require.NoError(t, err)
	assert.Empty(t, cands, "a disabled rule MUST NOT enter the candidate fetch")

	// Expired: invisible to BOTH (listing filters expired by default).
	// Re-enable the first rule so only the expiry gate can hide it.
	require.NoError(t, s.SetRuleEnabled(ctx, rules[0].ID, true))
	rules, err = s.QueryRules(ctx, "", "compliance", sqlite.RuleScopeSkill)
	require.NoError(t, err)
	assert.Len(t, rules, 1, "the expired rule MUST NOT appear in the active listing")

	// A is now enabled+permanent so it enters the candidate fetch; the
	// expiry gate is proven by B (expired) staying OUT.
	cands, err = s.ListActiveRulesForTarget(ctx, "memory:facts", "compliance", "sess-x")
	require.NoError(t, err)
	require.Len(t, cands, 1, "exactly the re-enabled permanent rule enters the candidate fetch")
	assert.Equal(t, "gate-hash-1", cands[0].ArgsHash, "the expired rule B must stay out of the candidate fetch")

	// Clear the expiry → back to permanent, visible everywhere again.
	require.NoError(t, s.SetRuleExpiry(ctx, rules[0].ID, 0))
	rules, err = s.QueryRules(ctx, "", "compliance", sqlite.RuleScopeSkill)
	require.NoError(t, err)
	assert.Len(t, rules, 1, "clearing the expiry restores the rule to the listing")
	assert.True(t, rules[0].Enabled)
	assert.Equal(t, int64(0), rules[0].ExpiresAt, "ExpiresAt 0 = permanent")

	// A future expiry keeps the rule active until the boundary.
	require.NoError(t, s.SetRuleExpiry(ctx, rules[0].ID, time.Now().Add(time.Hour).Unix()))
	rules, err = s.QueryRules(ctx, "", "compliance", sqlite.RuleScopeSkill)
	require.NoError(t, err)
	assert.Len(t, rules, 1, "a not-yet-expired rule stays listed")
	assert.Greater(t, rules[0].ExpiresAt, int64(0))
}
