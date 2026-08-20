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
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRuleTestStore opens a file-backed temp store (mirrors the
// audit_lifecycle_test.go pattern — :memory: with MaxOpenConns>1 would
// give every connection its own private DB, and the Revoke transaction
// needs a single connection).
func newRuleTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "rules.db")
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: dsn, MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestApprovalRulesScope — D11-D3 binary scope + D11-D1 match key.
//
// Locks four truths:
//  1. A session-scope rule does NOT match inside a DIFFERENT session
//     (HITL-08 cross-session isolation — T-11-05-03).
//  2. A session-scope rule MATCHES inside its originating session.
//  3. A skill-scope rule (scope_session_id = ”) MATCHES across
//     sessions for the same Skill.
//  4. A rule NEVER matches when skill_id differs (no cross-Skill
//     bleed; there is no global scope — T-11-05-04).
func TestApprovalRulesScope(t *testing.T) {
	s := newRuleTestStore(t)
	argsHash := "abc123"
	ctx := context.Background()

	require.NoError(t, s.CreateRule(ctx, sqlite.ApprovalRule{
		Target: "memory:facts", ArgsHash: argsHash, SkillID: "compliance",
		ScopeSessionID: "sess-A", CreatedAt: time.Now(), LastUsedAt: time.Now(),
	}))

	_, matched, err := s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-B")
	require.NoError(t, err)
	assert.False(t, matched, "session-scope rule MUST NOT match in different session")

	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-A")
	require.NoError(t, err)
	assert.True(t, matched, "session-scope rule MUST match in originating session")

	require.NoError(t, s.CreateRule(ctx, sqlite.ApprovalRule{
		Target: "memory:facts", ArgsHash: argsHash, SkillID: "compliance",
		ScopeSessionID: "", CreatedAt: time.Now(), LastUsedAt: time.Now(),
	}))
	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-B")
	require.NoError(t, err)
	assert.True(t, matched, "skill-scope rule MUST match across sessions for the same Skill")

	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "other-skill", "sess-A")
	require.NoError(t, err)
	assert.False(t, matched, "rule MUST NOT match when skill_id differs")

	// QueryRules honors the same binary scope.
	sessionRules, err := s.QueryRules(ctx, "sess-A", "", sqlite.ScopeSession)
	require.NoError(t, err)
	assert.Len(t, sessionRules, 1, "sess-A sees exactly its own session-scope rule")

	skillRules, err := s.QueryRules(ctx, "", "compliance", sqlite.ScopeSkill)
	require.NoError(t, err)
	assert.Len(t, skillRules, 1, "compliance sees exactly its skill-scope rule")

	_, err = s.QueryRules(ctx, "sess-A", "compliance", sqlite.Scope(99))
	assert.Error(t, err, "unknown scope MUST be rejected (no global listing)")
}

// TestApprovalRulesRevoke — D11-D4: DELETE approval_rules row + INSERT
// audit action_rule_revoked in a SINGLE transaction (T-3-AUDIT: the
// audit evidence for the revoke is the audit row, not the missing
// rule row).
func TestApprovalRulesRevoke(t *testing.T) {
	s := newRuleTestStore(t)
	ctx := context.Background()
	argsHash := "revoke-hash-1"

	require.NoError(t, s.CreateRule(ctx, sqlite.ApprovalRule{
		Target: "memory:facts", ArgsHash: argsHash, SkillID: "compliance",
		ScopeSessionID: "sess-1", CreatedAt: time.Now(), LastUsedAt: time.Now(),
	}))

	_, matched, err := s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-1")
	require.NoError(t, err)
	require.True(t, matched)

	require.NoError(t, s.Revoke(ctx, "sess-1", "user-1",
		"memory:facts", argsHash, "compliance", "sess-1", "v1", "prompt-hash-1", "panel revoke"))

	_, matched, err = s.MatchRule(ctx, "memory:facts", argsHash, "compliance", "sess-1")
	require.NoError(t, err)
	assert.False(t, matched, "rule row MUST be DELETEd after Revoke")

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-1", Limit: 10})
	require.NoError(t, err)
	var foundRuleRevoke bool
	for _, r := range rows {
		if r.Action == "action_rule_revoked" && r.ToolCallID != "" {
			foundRuleRevoke = true
			assert.Contains(t, string(r.FrozenArgs), argsHash)
			assert.Contains(t, string(r.FrozenArgs), "memory:facts")
			assert.Equal(t, "panel revoke", r.RefusalReason,
				"the operator's revoke reason travels on the audit row")
			assert.Equal(t, "user-1", r.UserID)
		}
	}
	assert.True(t, foundRuleRevoke, "action_rule_revoked audit row MUST exist after Revoke")

	err = s.Revoke(ctx, "sess-1", "user-1",
		"memory:facts", "non-existent-hash", "compliance", "sess-1", "v1", "prompt-hash-1", "double revoke")
	assert.Error(t, err, "Revoke on non-existent rule MUST error")
}
