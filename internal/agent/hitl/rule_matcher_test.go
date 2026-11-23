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

// rule_matcher_test.go is Plan 56-02 Task 2's verification gate:
// SqlRuleMatcher over the REAL store-backed tier engine (exact +
// prefix paths), the fail-closed posture (store error / nil store =
// no-match → interrupt), and the default OnMatch hit accounting
// (HitRule counter + InsertRuleHit ledger row, eventual — the hook is
// fire-and-forget by the T-11-05-09 accept disposition).
package hitl

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMatcherTestStore opens a file-backed temp store for the matcher
// tests (the RevokeByID transaction needs a single connection; the
// same posture as approval_rules_test.go's newRuleTestStore).
func newMatcherTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "matcher-rules.db")
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: dsn, MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedSessionRule inserts one enabled session-scope rule of the given
// tier and returns nothing (the engine finds it by scope).
func seedSessionRule(t *testing.T, s *sqlite.SQLiteStore, mod func(*sqlite.ApprovalRule)) {
	t.Helper()
	rule := sqlite.ApprovalRule{
		Target:         "memory:facts",
		SkillID:        "compliance",
		Scope:          "session",
		ScopeSessionID: "sess-1",
		Enabled:        true,
	}
	if mod != nil {
		mod(&rule)
	}
	require.NoError(t, s.CreateRule(context.Background(), rule))
}

// matcherProbe builds the MatchRequest the Tool Pass-1 would build.
func matcherProbe(args string) MatchRequest {
	return MatchRequest{
		Target:     "memory:facts",
		SkillID:    "compliance",
		SessionID:  "sess-1",
		ArgsJSON:   args,
		ArgsHash:   ComputeFrozenArgsHash([]byte(args)),
		ToolCallID: "tc-match-01",
	}
}

// TestSqlRuleMatcherEngineExact — the exact tier through the REAL
// engine: byte-identical args match, drifted args do not.
func TestSqlRuleMatcherEngineExact(t *testing.T) {
	s := newMatcherTestStore(t)
	ctx := context.Background()
	args := `{"key":"employee_count","value":"150"}`
	seedSessionRule(t, s, func(r *sqlite.ApprovalRule) {
		r.MatchType = sqlite.MatchTypeExact
		r.ArgsHash = ComputeFrozenArgsHash([]byte(args))
	})

	m := NewSqlRuleMatcher(s)
	rule, matched := m.Match(ctx, matcherProbe(args))
	require.True(t, matched, "byte-identical args must match the exact rule")
	require.NotNil(t, rule)
	assert.Equal(t, sqlite.MatchTypeExact, rule.MatchType)
	assert.Equal(t, "session", rule.Scope)

	// Same session, drifted args → no match (the interrupt path).
	_, matched = m.Match(ctx, matcherProbe(`{"key":"employee_count","value":"151"}`))
	assert.False(t, matched, "drifted args must NOT match an exact rule")
}

// TestSqlRuleMatcherEnginePrefix — the prefix tier through the REAL
// engine: a locked-field subset matches with free extra fields; a
// drifted locked value does not. This is the tier the legacy 11-05
// lookup structurally could not express.
func TestSqlRuleMatcherEnginePrefix(t *testing.T) {
	s := newMatcherTestStore(t)
	ctx := context.Background()
	seedSessionRule(t, s, func(r *sqlite.ApprovalRule) {
		r.MatchType = sqlite.MatchTypePrefix
		r.ArgsPattern = `{"key":"employee_count"}`
		r.ArgsHash = ""
	})

	m := NewSqlRuleMatcher(s)

	rule, matched := m.Match(ctx, matcherProbe(`{"key":"employee_count","value":"150"}`))
	require.True(t, matched, "locked-field subset with free extras must match")
	require.NotNil(t, rule)
	assert.Equal(t, sqlite.MatchTypePrefix, rule.MatchType)

	_, matched = m.Match(ctx, matcherProbe(`{"key":"headcount","value":"150"}`))
	assert.False(t, matched, "a drifted LOCKED field must not match")
}

// TestSqlRuleMatcherStoreErrorIsNoMatch — fail-closed: a store failure
// must degrade to the interrupt path, never to silent execution. A
// closed store makes every engine call error (isClosed → sql.ErrConnDone).
func TestSqlRuleMatcherStoreErrorIsNoMatch(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "closed-store.db")
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: dsn, MaxOpenConns: 1})
	require.NoError(t, err)
	require.NoError(t, s.Close())

	m := NewSqlRuleMatcher(s)
	rule, matched := m.Match(context.Background(), matcherProbe(`{"key":"k"}`))
	assert.False(t, matched, "store error must degrade to no-match (fail-closed)")
	assert.Nil(t, rule)
}

// TestSqlRuleMatcherNilStoreIsNoMatch — the zero-value matcher (no
// store) never matches.
func TestSqlRuleMatcherNilStoreIsNoMatch(t *testing.T) {
	m := &SqlRuleMatcher{}
	rule, matched := m.Match(context.Background(), matcherProbe(`{"key":"k"}`))
	assert.False(t, matched)
	assert.Nil(t, rule)
}

// TestSqlRuleMatcherHitAccounting — the DEFAULT OnMatch hook records
// the auto-pass: hit_count increments AND one approval_rule_hits ledger
// row lands with the right attribution. The hook is fire-and-forget
// (T-11-05-09), so the assertions poll via require.Eventually.
func TestSqlRuleMatcherHitAccounting(t *testing.T) {
	s := newMatcherTestStore(t)
	ctx := context.Background()
	args := `{"key":"employee_count","value":"150"}`
	seedSessionRule(t, s, func(r *sqlite.ApprovalRule) {
		r.MatchType = sqlite.MatchTypeExact
		r.ArgsHash = ComputeFrozenArgsHash([]byte(args))
	})

	m := NewSqlRuleMatcher(s)
	rule, matched := m.Match(ctx, matcherProbe(args))
	require.True(t, matched)
	require.NotNil(t, rule)

	// Exact-tier hit: the ledger row carries the incoming args hash.
	require.Eventually(t, func() bool {
		rules, err := s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
		if err != nil || len(rules) != 1 {
			return false
		}
		return rules[0].HitCount == 1
	}, 2*time.Second, 10*time.Millisecond, "hit_count must increment (eventual — fire-and-forget hook)")

	require.Eventually(t, func() bool {
		hits, err := s.QueryRuleHits(ctx, rule.ID, 10)
		return err == nil && len(hits) == 1
	}, 2*time.Second, 10*time.Millisecond, "one ledger row must land (eventual — fire-and-forget hook)")

	hits, err := s.QueryRuleHits(ctx, rule.ID, 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	hit := hits[0]
	assert.Equal(t, rule.ID, hit.RuleID)
	assert.Equal(t, "sess-1", hit.SessionID)
	assert.Equal(t, "memory:facts", hit.Target)
	assert.Equal(t, "tc-match-01", hit.ToolCallID, "the ledger row must attribute the tool call")
	assert.Equal(t, sqlite.MatchTypeExact, hit.MatchType)
	assert.Equal(t, ComputeFrozenArgsHash([]byte(args)), hit.MatchedArgsHash,
		"exact-tier hits carry the incoming args hash")

	// A prefix-tier hit stores '' in matched_args_hash (the hash is
	// meaningless there — the rule never compares it).
	seedSessionRule(t, s, func(r *sqlite.ApprovalRule) {
		r.MatchType = sqlite.MatchTypePrefix
		r.ArgsPattern = `{"key":"headcount"}`
		r.ArgsHash = ""
	})
	rule2, matched2 := m.Match(ctx, matcherProbe(`{"key":"headcount","value":"9"}`))
	require.True(t, matched2)
	require.NotNil(t, rule2)
	require.Eventually(t, func() bool {
		hits, err := s.QueryRuleHits(ctx, rule2.ID, 10)
		return err == nil && len(hits) == 1
	}, 2*time.Second, 10*time.Millisecond)
	hits2, err := s.QueryRuleHits(ctx, rule2.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, hits2[0].MatchedArgsHash, "prefix-tier hits carry '' as matched_args_hash")
}

// TestNoopRuleMatcherNeverMatches — the fail-closed default: no store,
// no match, interrupt path.
func TestNoopRuleMatcherNeverMatches(t *testing.T) {
	rule, matched := NoopRuleMatcher{}.Match(context.Background(), matcherProbe(`{"key":"k"}`))
	assert.False(t, matched)
	assert.Nil(t, rule)
}
