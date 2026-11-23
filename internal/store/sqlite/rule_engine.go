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

// rule_engine.go is Phase 56 / Plan 56-02 Task 1: the tier scoring
// engine over the approval_rules candidates. 56-01 landed the schema +
// the candidate fetch (ListActiveRulesForTarget); this file adds the
// in-Go evaluation layer — exact / prefix / template tier semantics —
// and the deterministic priority pick. SQL stays a pure candidate
// filter (enabled + TTL + three-layer scope visibility); ALL tier
// semantics live here so the rules remain diffable/reviewable policy
// rows rather than query-shape policies.
//
// DENY SEMANTICS (D-56-01, pinned): the approval_rules table only ever
// expresses an AUTO-PASS policy. There is NO deny rule — deny /
// request_change stay one-shot operator decisions recorded in the audit
// chain. A rule match can never suppress a human decision that the
// audit requires; it only skips the interrupt the operator already
// pre-approved via approve-and-remember.
//
// File-placement note: this engine is a separate file (not appended to
// approval_rules.go) so the sha256-line-range baselines pinned in
// internal/agent/hitl/threat_mitigations_test.go (CreateRule body
// 379-401, listActiveRulesForTargetSQL 275-287) stay byte-stable — the
// only approval_rules.go change in 56-02 is one interface-method line
// BELOW both pinned ranges.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"sort"
)

// MatchRequest is one incoming tool-call probe the engine evaluates.
// The caller (internal/agent/hitl.SqlRuleMatcher) builds it per Pass-1
// invocation; the store layer owns this shape — hitl defines its own
// field-identical struct and converts, keeping the hitl→sqlite
// dependency direction one-way (no reverse import).
type MatchRequest struct {
	// Target is the write-target policy string ("memory:facts",
	// "plan_approve", ...). Candidate rules are fetched per target.
	Target string

	// SkillID is the calling session's Skill identity; it selects the
	// scope='skill' candidate layer ('' = only session/global layers).
	SkillID string

	// SessionID is the calling session's identity; it selects the
	// scope='session' candidate layer.
	SessionID string

	// ArgsJSON is the VERBATIM tool-call args byte string (HITL-07
	// convention: never re-marshaled before this point). Only the
	// prefix tier parses it (JSON object field subset matching).
	ArgsJSON string

	// ArgsHash is the hex SHA-256 of the ArgsJSON bytes, precomputed by
	// the caller via hitl.ComputeFrozenArgsHash. The engine NEVER
	// re-hashes (single hash convention, D11-D1).
	ArgsHash string

	// ToolCallID is the incoming call's identity. It does not affect
	// matching; it is carried so the hit-ledger write (InsertRuleHit)
	// can attribute the auto-pass without the caller re-passing it.
	ToolCallID string
}

// MatchStats is the engine's per-invocation accounting — the test and
// log surface for the deterministic pick. It is observation data, never
// part of the wire contract.
type MatchStats struct {
	// Candidates is the enabled+unexpired+visible rule count the SQL
	// fetch returned for (target, skillID, sessionID).
	Candidates int
	// Matched is how many candidates satisfied their tier predicate.
	Matched int
	// MalformedPatterns counts prefix rules whose ArgsPattern failed to
	// parse as a JSON object — such rules are SKIPPED as non-matching
	// (never an error; see the function doc).
	MalformedPatterns int
	// ChosenRuleID is the winning rule's surrogate PK (0 = no match).
	ChosenRuleID int64
	// ChosenScope mirrors the winner's stored scope column.
	ChosenScope string
	// ChosenMatchType mirrors the winner's stored match_type column.
	ChosenMatchType string
}

// scopeRank maps the stored scope column onto the D-56-01 priority
// order: session (0) > skill (1) > global (2). Unknown values rank
// last (defensive — the CHECK constraint already refuses them at
// INSERT time).
func scopeRank(scope string) int {
	switch scope {
	case string(RuleScopeSession):
		return 0
	case string(RuleScopeSkill):
		return 1
	default:
		return 2
	}
}

// specRank maps the match tier onto the same-layer specificity order:
// exact (0) > prefix (1) > template (2). An exact rule is the most
// specific statement ("this exact call is pre-approved"); a template
// rule is the least ("everything on this target is pre-approved").
func specRank(matchType string) int {
	switch matchType {
	case MatchTypeExact:
		return 0
	case MatchTypePrefix:
		return 1
	default:
		return 2
	}
}

// parsePatternObject decodes a prefix rule's ArgsPattern column into a
// JSON object map. ok=false covers parse failure AND non-object
// payloads (arrays/scalars) — both make the rule unmatchable, not an
// error.
func parsePatternObject(pattern string) (map[string]any, bool) {
	if pattern == "" {
		return nil, false
	}
	var decoded any
	if err := json.Unmarshal([]byte(pattern), &decoded); err != nil {
		return nil, false
	}
	obj, ok := decoded.(map[string]any)
	return obj, ok
}

// prefixRuleMatches implements the prefix-tier predicate: every locked
// field in the pattern must exist in the parsed args and be deeply
// equal (reflect.DeepEqual — scalars compare by value+type, nested
// objects compare recursively). Unlocked fields stay free (the
// naming follows the Claude Code Bash(cmd:*) intuition). A pattern
// key absent from the parsed args is a non-match, never an error.
func prefixRuleMatches(parsedArgs map[string]any, pattern map[string]any) bool {
	for key, want := range pattern {
		got, exists := parsedArgs[key]
		if !exists {
			return false
		}
		if !reflect.DeepEqual(got, want) {
			return false
		}
	}
	return true
}

// MatchRuleForArgs evaluates the active rule candidates for one
// incoming tool call and returns the deterministic winner, or (nil,
// stats, nil) when nothing matches.
//
// NO-MATCH IS A NORMAL RETURN, NOT AN ERROR (D-56-02): the caller must
// fall through to the human-interrupt path. The error channel is
// reserved for store I/O failure — and the CALLER (SqlRuleMatcher)
// treats even that as no-match (fail-closed toward human review, never
// toward silent execution).
//
// Tier semantics (locked, mirrors the 56-02 plan text the UI copy and
// tests are written against):
//
//   - exact:   rule.ArgsHash == req.ArgsHash (byte-equality via the
//     precomputed hash; args_hash is an index, the comparison is the
//     contract — D-56-01). An exact rule with an EMPTY stored hash can
//     never match (defensive: a degenerate row must not match a
//     degenerate probe).
//   - prefix:  req.ArgsJSON parses to a JSON object AND every locked
//     field in the rule's ArgsPattern exists in it with a deeply equal
//     value; unlocked fields are free. A rule whose pattern is not a
//     parseable JSON object is SKIPPED (counted in stats.
//     MalformedPatterns) — a malformed policy row must not fail the
//     whole lookup, and must never match more than its author intended
//     (fail-closed: skip means interrupt, not auto-pass).
//   - template: unconditional match at the TARGET level — every call
//     to the target auto-passes within the rule's scope visibility.
//
// Deterministic pick — the FIRST rule in this order wins:
//
//  1. scope ASC   (session > skill > global — more specific scope first)
//  2. tier ASC    (exact > prefix > template — more specific tier first)
//  3. locked-field count DESC (a prefix rule locking more fields wins)
//  4. created_at ASC (the older policy statement wins)
//  5. id ASC      (stable surrogate-PK tie-break)
//
// The candidate fetch already sorts by (scope, id); the in-Go sort
// re-orders within equal scope by tier/locked/created_at and relies on
// sort.SliceStable for the id tie-break the SQL order established.
func (s *SQLiteStore) MatchRuleForArgs(ctx context.Context, req MatchRequest) (*ApprovalRule, MatchStats, error) {
	if s.isClosed() {
		return nil, MatchStats{}, sql.ErrConnDone
	}
	candidates, err := s.ListActiveRulesForTarget(ctx, req.Target, req.SkillID, req.SessionID)
	if err != nil {
		return nil, MatchStats{}, fmtError("MatchRuleForArgs", err)
	}

	stats := MatchStats{Candidates: len(candidates)}
	if len(candidates) == 0 {
		return nil, stats, nil
	}

	// Parse the probe args ONCE. A non-JSON / non-object payload leaves
	// parsedArgs nil — exact rules still evaluate (hash equality does
	// not need JSON), prefix rules simply never match.
	var parsedArgs map[string]any
	if req.ArgsJSON != "" {
		var decoded any
		if uErr := json.Unmarshal([]byte(req.ArgsJSON), &decoded); uErr == nil {
			if obj, ok := decoded.(map[string]any); ok {
				parsedArgs = obj
			}
		}
	}

	type scoredRule struct {
		rule      ApprovalRule
		scopeRank int
		specRank  int
		locked    int
	}
	matched := make([]scoredRule, 0, len(candidates))
	for _, rule := range candidates {
		switch rule.MatchType {
		case MatchTypeExact:
			if rule.ArgsHash == "" || rule.ArgsHash != req.ArgsHash {
				continue
			}
			matched = append(matched, scoredRule{
				rule:      rule,
				scopeRank: scopeRank(rule.Scope),
				specRank:  specRank(rule.MatchType),
			})
		case MatchTypePrefix:
			pattern, ok := parsePatternObject(rule.ArgsPattern)
			if !ok {
				stats.MalformedPatterns++
				continue
			}
			if parsedArgs == nil || !prefixRuleMatches(parsedArgs, pattern) {
				continue
			}
			matched = append(matched, scoredRule{
				rule:      rule,
				scopeRank: scopeRank(rule.Scope),
				specRank:  specRank(rule.MatchType),
				locked:    len(pattern),
			})
		case MatchTypeTemplate:
			matched = append(matched, scoredRule{
				rule:      rule,
				scopeRank: scopeRank(rule.Scope),
				specRank:  specRank(rule.MatchType),
			})
		default:
			// Unknown tier (future match_type added by a newer binary
			// reading an older... inverse: an older binary reading a
			// newer row). Fail-closed: an unknown tier can never
			// auto-pass — it is skipped like a non-match.
			continue
		}
	}
	stats.Matched = len(matched)
	if len(matched) == 0 {
		return nil, stats, nil
	}

	// Deterministic priority pick (see the function doc for the order).
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].scopeRank != matched[j].scopeRank {
			return matched[i].scopeRank < matched[j].scopeRank
		}
		if matched[i].specRank != matched[j].specRank {
			return matched[i].specRank < matched[j].specRank
		}
		if matched[i].locked != matched[j].locked {
			return matched[i].locked > matched[j].locked
		}
		if !matched[i].rule.CreatedAt.Equal(matched[j].rule.CreatedAt) {
			return matched[i].rule.CreatedAt.Before(matched[j].rule.CreatedAt)
		}
		return matched[i].rule.ID < matched[j].rule.ID
	})

	winner := matched[0].rule
	stats.ChosenRuleID = winner.ID
	stats.ChosenScope = winner.Scope
	stats.ChosenMatchType = winner.MatchType
	return &winner, stats, nil
}
