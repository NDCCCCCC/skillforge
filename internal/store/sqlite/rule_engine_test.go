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

// rule_engine_test.go is Plan 56-02 Task 1's verification gate: the
// three-tier match semantics (exact / prefix / template, hit + miss
// per tier), the deterministic priority pick (scope > tier > locked
// count > created_at > id), and the fail-closed edge behavior
// (malformed pattern = skip, no-match = nil+nil, disabled/expired =
// invisible). All tests are hermetic file-backed SQLite
// (newRuleTestStore from approval_rules_test.go).
package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// argsHashOf mirrors hitl.ComputeFrozenArgsHash (re-implemented here so
// the sqlite_test package does not depend on the hitl package — the
// dependency direction hitl→sqlite must stay one-way). sha256 of the
// verbatim bytes, hex-encoded.
func argsHashOf(args string) string {
	sum := sha256.Sum256([]byte(args))
	return hex.EncodeToString(sum[:])
}

// matchProbe builds a MatchRequest for the common probe shape.
func matchProbe(sessionID, argsJSON string) sqlite.MatchRequest {
	return sqlite.MatchRequest{
		Target:    "memory:facts",
		SkillID:   "compliance",
		SessionID: sessionID,
		ArgsJSON:  argsJSON,
		ArgsHash:  argsHashOf(argsJSON),
	}
}

// TestMatchRuleForArgsTiers — the three-tier semantics table (hit and
// miss per tier, plus the malformed-pattern fail-closed skip). Each
// sub-test gets a fresh store so natural-key collisions between table
// rows cannot leak across cases.
func TestMatchRuleForArgsTiers(t *testing.T) {
	const (
		argsFull    = `{"key":"employee_count","value":"150","tier":"authority"}`
		argsPartial = `{"key":"employee_count","value":"151","tier":"authority"}`
	)

	tests := []struct {
		name        string
		seed        func(t *testing.T, s *sqlite.SQLiteStore)
		req         sqlite.MatchRequest
		wantMatched bool
		wantType    string
		wantScope   string
		wantCands   int
		wantSkipped int
	}{
		{
			name: "exact hit — byte-equal args hash",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypeExact
					r.ArgsHash = argsHashOf(argsFull)
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsFull),
			wantMatched: true,
			wantType:    "exact",
			wantScope:   "session",
			wantCands:   1,
		},
		{
			name: "exact miss — different args bytes hash differently",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypeExact
					r.ArgsHash = argsHashOf(argsFull)
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsPartial),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "exact rule with empty stored hash never matches (degenerate row)",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypeExact
					r.ArgsHash = ""
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         sqlite.MatchRequest{Target: "memory:facts", SkillID: "compliance", SessionID: "sess-1"},
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix hit — locked fields equal, extra free fields allowed",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"key":"employee_count","value":"150"}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsFull),
			wantMatched: true,
			wantType:    "prefix",
			wantScope:   "session",
			wantCands:   1,
		},
		{
			name: "prefix miss — locked field value differs",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"key":"employee_count","value":"150"}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsPartial),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix miss — locked key absent from parsed args",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"region":"north"}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsFull),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix miss — args payload is not a JSON object (array)",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"key":"employee_count"}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", `["employee_count","150"]`),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix miss — args payload is invalid JSON",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"key":"employee_count"}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", `{invalid`),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix hit — nested object locked value compares deeply",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"meta":{"a":1,"b":"x"}}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", `{"meta":{"a":1,"b":"x"},"extra":true}`),
			wantMatched: true,
			wantType:    "prefix",
			wantScope:   "session",
			wantCands:   1,
		},
		{
			name: "prefix miss — nested object locked value differs",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `{"meta":{"a":1,"b":"x"}}`
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", `{"meta":{"a":1,"b":"y"}}`),
			wantMatched: false,
			wantCands:   1,
		},
		{
			name: "prefix rules with malformed patterns are SKIPPED (counted, not an error)",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `[1,2,3]` // valid JSON but not an object
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypePrefix
					r.ArgsPattern = `not-json-at-all` // invalid JSON
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsFull),
			wantMatched: false,
			wantCands:   2,
			wantSkipped: 2,
		},
		{
			name: "template hit — unconditional at the target level",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypeTemplate
					r.Scope = string(sqlite.RuleScopeSession)
					r.ScopeSessionID = "sess-1"
				})
			},
			req:         matchProbe("sess-1", argsPartial),
			wantMatched: true,
			wantType:    "template",
			wantScope:   "session",
			wantCands:   1,
		},
		{
			name: "template hit — even with empty args",
			seed: func(t *testing.T, s *sqlite.SQLiteStore) {
				createTestRule(t, s, func(r *sqlite.ApprovalRule) {
					r.MatchType = sqlite.MatchTypeTemplate
					r.Scope = string(sqlite.RuleScopeSkill)
					r.ScopeSessionID = ""
				})
			},
			req:         matchProbe("sess-any", ""),
			wantMatched: true,
			wantType:    "template",
			wantScope:   "skill",
			wantCands:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newRuleTestStore(t)
			tc.seed(t, store)

			rule, stats, err := store.MatchRuleForArgs(context.Background(), tc.req)
			require.NoError(t, err, "no-match must be a normal return, never an error")
			assert.Equal(t, tc.wantCands, stats.Candidates, "candidate fetch count")
			if !tc.wantMatched {
				assert.Nil(t, rule, "unexpected match")
				assert.Equal(t, tc.wantSkipped, stats.MalformedPatterns, "malformed-pattern skip count")
				assert.Zero(t, stats.ChosenRuleID, "no-match stats must carry a zero chosen id")
				return
			}
			require.NotNil(t, rule)
			assert.Equal(t, tc.wantType, rule.MatchType)
			assert.Equal(t, tc.wantType, stats.ChosenMatchType)
			if tc.wantScope != "" {
				assert.Equal(t, tc.wantScope, rule.Scope)
				assert.Equal(t, tc.wantScope, stats.ChosenScope)
			}
			assert.Equal(t, rule.ID, stats.ChosenRuleID, "stats must name the returned rule")
			assert.Equal(t, 1, stats.Matched, "exactly one candidate should satisfy the tier predicate")
		})
	}
}

// TestMatchRuleForArgsPriority — the deterministic pick: scope
// (session > skill > global), then tier (exact > prefix > template),
// then prefix locked-field count DESC, then created_at ASC, then id
// ASC. Multi-match cases assert the FULL matched count too, proving the
// pick is a sort over all matches rather than a first-hit short
// circuit.
func TestMatchRuleForArgsPriority(t *testing.T) {
	ctx := context.Background()

	t.Run("scope priority: session beats skill beats global", func(t *testing.T) {
		s := newRuleTestStore(t)
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeGlobal)
			r.SkillID = ""
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeSkill)
			r.ScopeSessionID = ""
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})

		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", `{}`))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "session", rule.Scope, "session-scope rule must win")
		assert.Equal(t, "session", stats.ChosenScope)
		assert.Equal(t, 3, stats.Matched)

		// Same store, caller from another session: the session rule is
		// invisible → skill layer wins.
		rule, _, err = s.MatchRuleForArgs(ctx, matchProbe("sess-2", `{}`))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "skill", rule.Scope, "skill-scope rule wins when no session rule is visible")

		// Another session AND another skill: only the global layer is
		// visible.
		req := matchProbe("sess-3", `{}`)
		req.SkillID = "hr-policy"
		rule, _, err = s.MatchRuleForArgs(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "global", rule.Scope, "global-scope rule wins when no session/skill rule is visible")
	})

	t.Run("tier specificity: exact beats prefix beats template in one scope", func(t *testing.T) {
		s := newRuleTestStore(t)
		const args = `{"key":"employee_count","value":"150"}`
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"key":"employee_count"}`
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeExact
			r.ArgsHash = argsHashOf(args)
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})

		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", args))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "exact", rule.MatchType)
		assert.Equal(t, 3, stats.Matched, "all three tiers matched; exact won on specificity")
	})

	t.Run("prefix locked-field count DESC breaks same-scope same-tier ties", func(t *testing.T) {
		s := newRuleTestStore(t)
		const args = `{"key":"employee_count","value":"150","tier":"authority"}`
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"key":"employee_count"}` // 1 locked field
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"key":"employee_count","value":"150"}` // 2 locked fields
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})

		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", args))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, 2, stats.Matched)
		assert.Contains(t, rule.ArgsPattern, `"value":"150"`,
			"the prefix rule locking MORE fields must win")
	})

	t.Run("created_at ASC breaks locked-count ties", func(t *testing.T) {
		s := newRuleTestStore(t)
		older := time.Unix(1700000000, 0).UTC()
		newer := time.Unix(1700000100, 0).UTC()

		// Same locked count (1 field each), different created_at → the
		// older rule wins. Distinct patterns keep the natural keys apart.
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"tier":"authority"}`
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
			r.CreatedAt = newer
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"key":"employee_count"}`
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
			r.CreatedAt = older
		})

		const args = `{"key":"employee_count","tier":"authority"}`
		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", args))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, 2, stats.Matched)
		assert.Contains(t, rule.ArgsPattern, `"key":"employee_count"`,
			"the OLDER same-specificity rule must win")
	})

	t.Run("id ASC breaks created_at ties", func(t *testing.T) {
		s := newRuleTestStore(t)
		// Identical created_at (same unix second — the column's
		// granularity) → lower surrogate id wins. The FIRST insert gets
		// the lower id.
		same := time.Unix(1700000200, 0).UTC()
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"value":"150"}`
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
			r.CreatedAt = same
		})
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypePrefix
			r.ArgsPattern = `{"extra":"z"}`
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
			r.CreatedAt = same
		})

		const args = `{"value":"150","extra":"z"}`
		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", args))
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, 2, stats.Matched)
		assert.Equal(t, int64(1), rule.ID, "the LOWER id wins the final tie-break (first insert)")
	})
}

// TestMatchRuleForArgsFailClosed — invisible rules never match:
// disabled (SetRuleEnabled), expired (SetRuleExpiry in the past), and
// the empty-table case are all a clean no-match.
func TestMatchRuleForArgsFailClosed(t *testing.T) {
	ctx := context.Background()

	t.Run("empty table — no candidates", func(t *testing.T) {
		s := newRuleTestStore(t)
		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", `{}`))
		require.NoError(t, err)
		assert.Nil(t, rule)
		assert.Equal(t, 0, stats.Candidates)
	})

	t.Run("disabled rule is invisible to the engine", func(t *testing.T) {
		s := newRuleTestStore(t)
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})
		rules, err := s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		require.NoError(t, s.SetRuleEnabled(ctx, rules[0].ID, false))

		rule, stats, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", `{}`))
		require.NoError(t, err)
		assert.Nil(t, rule, "a disabled rule must never auto-pass")
		assert.Equal(t, 0, stats.Candidates, "disabled rules are filtered at the candidate fetch")
	})

	t.Run("expired rule is invisible to the engine", func(t *testing.T) {
		s := newRuleTestStore(t)
		createTestRule(t, s, func(r *sqlite.ApprovalRule) {
			r.MatchType = sqlite.MatchTypeTemplate
			r.Scope = string(sqlite.RuleScopeSession)
			r.ScopeSessionID = "sess-1"
		})
		rules, err := s.QueryRules(ctx, "sess-1", "", sqlite.RuleScopeSession)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		require.NoError(t, s.SetRuleExpiry(ctx, rules[0].ID, time.Now().Add(-time.Hour).Unix()))

		rule, _, err := s.MatchRuleForArgs(ctx, matchProbe("sess-1", `{}`))
		require.NoError(t, err)
		assert.Nil(t, rule, "an expired rule must never auto-pass")
	})
}
