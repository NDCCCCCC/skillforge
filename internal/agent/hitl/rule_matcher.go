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

// rule_matcher.go is the Plan 11-05 / HITL-08 / D11-D1 seam between the
// HITL Tool and the approval_rules store. The Tool's InvokableRun
// queries Match BEFORE reaching tool.StatefulInterrupt; a matched rule
// auto-passes the write (skip the interrupt, execute directly, audit
// the auto-pass via AuditWriter.MarkAutoPassedRule).
//
// The dependency edge is deliberately narrow: this file imports
// internal/store/sqlite for the ApprovalRule value type and the
// ApprovalRulesStore narrow interface — never internal/store/store.go's
// full Store surface.
package hitl

import (
	"context"

	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// RuleMatcher is the 1-method interface the HITL Tool consumes.
// D11-D1: the match key is (target, argsHash, skill_id) plus the
// D11-D3 binary scope lookup (session-scope rules match only their
// originating session; skill-scope rules match across sessions for the
// same Skill).
//
// argsHash is ComputeFrozenArgsHash(argumentsInJSON) — the hex SHA-256
// of the verbatim tool-call args bytes, never re-marshaled through
// encoding/json (HITL-07 byte-identity anchor shared with the rule
// key).
type RuleMatcher interface {
	Match(ctx context.Context, target, argsHash, skillID, sessionID string) (*sqlite.ApprovalRule, bool)
}

// SqlRuleMatcher is the production RuleMatcher backed by the
// approval_rules table. On a successful match it fires OnMatch in a
// goroutine (default: UpdateLastUsedAt — best-effort, T-11-05-09
// accept disposition: last_used_at is observability, never on the
// critical path).
type SqlRuleMatcher struct {
	Store sqlite.ApprovalRulesStore

	// OnMatch is invoked fire-and-forget after a successful Match. Nil
	// disables the hook (tests inject spies here).
	OnMatch func(target, argsHash, skillID, scopeSessionID string)
}

// NewSqlRuleMatcher constructs the matcher with the default OnMatch
// hook (last_used_at refresh on a background goroutine).
func NewSqlRuleMatcher(store sqlite.ApprovalRulesStore) *SqlRuleMatcher {
	return &SqlRuleMatcher{
		Store: store,
		OnMatch: func(target, argsHash, skillID, scopeSessionID string) {
			// context.Background(): the caller's ctx may be cancelled
			// by the time the goroutine runs; the touch is independent
			// of the request lifecycle.
			_ = store.UpdateLastUsedAt(context.Background(), target, argsHash, skillID, scopeSessionID)
		},
	}
}

// Match implements RuleMatcher. Store errors are treated as no-match:
// a rule-lookup failure must NEVER block the approval flow — the Tool
// falls back to the normal interrupt path, which is the safe direction
// (fail-closed toward human review, not toward silent execution).
func (m *SqlRuleMatcher) Match(ctx context.Context, target, argsHash, skillID, sessionID string) (*sqlite.ApprovalRule, bool) {
	if m == nil || m.Store == nil {
		return nil, false
	}
	rule, matched, err := m.Store.MatchRule(ctx, target, argsHash, skillID, sessionID)
	if err != nil || !matched {
		return nil, false
	}
	if m.OnMatch != nil && rule != nil {
		go m.OnMatch(rule.Target, rule.ArgsHash, rule.SkillID, rule.ScopeSessionID)
	}
	return rule, true
}

// NoopRuleMatcher never matches. Used by tests and by read-only Skills
// (writes: []) where no rule lookup should ever occur.
type NoopRuleMatcher struct{}

// Match implements RuleMatcher, always returning (nil, false).
func (NoopRuleMatcher) Match(_ context.Context, _, _, _, _ string) (*sqlite.ApprovalRule, bool) {
	return nil, false
}
