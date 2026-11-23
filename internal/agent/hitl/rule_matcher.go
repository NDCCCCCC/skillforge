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
// HITL Tool and the approval_rules store, upgraded by Phase 56 / Plan
// 56-02 (D-56-02): the matcher now drives the FULL three-tier engine
// (exact / prefix / template, deterministic session > skill > global +
// specificity scoring — sqlite.MatchRuleForArgs) instead of the legacy
// exact-only lookup, and the OnMatch hook performs the hit accounting
// (HitRule counter + InsertRuleHit ledger row) instead of the retired
// last_used_at touch.
//
// A matched rule auto-passes the write (skip the interrupt, execute
// directly, audit the auto-pass via AuditWriter.MarkAutoPassedRule —
// the Tool's hit branch owns that call and the rule-matched SSE emit
// through RuleHitEmitFunc, audit row FIRST, event after, the 12-06
// P12-L6 ordering).
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

// MatchRequest is one incoming tool-call probe. Field-identical to
// sqlite.MatchRequest — kept as a SEPARATE hitl-owned struct so the
// RuleMatcher interface (which every test doubles) stays independent of
// the store package's engine surface; SqlRuleMatcher converts at the
// boundary. The hitl→sqlite import direction stays one-way.
type MatchRequest struct {
	// Target is the write-target policy string ("memory:facts", ...).
	Target string
	// SkillID is the calling session's Skill identity.
	SkillID string
	// SessionID is the calling session's identity.
	SessionID string
	// ArgsJSON is the VERBATIM tool-call args byte string (HITL-07:
	// never re-marshaled before hashing).
	ArgsJSON string
	// ArgsHash is ComputeFrozenArgsHash(ArgsJSON bytes), precomputed by
	// the Tool (the engine never re-hashes).
	ArgsHash string
	// ToolCallID is the incoming call's identity (== approvalID per
	// D11-C4). It does not affect matching; it rides along so the hit
	// ledger row and the rule-matched SSE frame can attribute the
	// auto-pass.
	ToolCallID string
}

// RuleMatcher is the 1-method interface the HITL Tools consume on
// Pass 1. A match means "an approve-and-remember rule pre-approved
// THIS call" — the Tool skips tool.StatefulInterrupt and executes
// directly.
//
// 56-02 semantics: the implementation consults the FULL tier engine —
// exact (args-hash byte equality), prefix (locked-field JSON-object
// subset), template (target-level), scored session > skill > global
// then exact > prefix > template then locked-field-count DESC then
// created_at ASC then id ASC. A no-match or a store error means the
// Tool takes the normal interrupt path.
type RuleMatcher interface {
	Match(ctx context.Context, req MatchRequest) (*sqlite.ApprovalRule, bool)
}

// SqlRuleMatcher is the production RuleMatcher backed by the
// approval_rules table through the 56-02 engine. On a successful match
// it fires OnMatch in a goroutine.
type SqlRuleMatcher struct {
	Store sqlite.ApprovalRulesStore

	// OnMatch is invoked fire-and-forget after a successful Match. The
	// DEFAULT implementation (NewSqlRuleMatcher) is the 56-02 hit
	// accounting: HitRule(rule.ID) (hit_count+1 + the dual
	// last_hit_at/last_used_at stamp) + InsertRuleHit (the INSERT-only
	// approval_rule_hits ledger row). Accounting is OBSERVABILITY, not
	// the critical path (T-11-05-09 accept disposition carried into
	// 56-02): store errors are swallowed — the auto-pass itself already
	// wrote its durable audit row via MarkAutoPassedRule before this
	// hook can even run. Nil disables the hook (tests inject spies).
	//
	// 56-02 RETIRED the 11-05 default hook (UpdateLastUsedAt — the
	// store method survives deprecated until 56-05 deletes it).
	OnMatch func(rule *sqlite.ApprovalRule, req MatchRequest)
}

// NewSqlRuleMatcher constructs the matcher with the default OnMatch
// hook (hit accounting on a background goroutine). The rule-matched
// SSE emission does NOT travel through this matcher — the Tool's hit
// branch invokes its own RuleHitEmitFunc synchronously after
// MarkAutoPassedRule so the audit-row-before-event ordering is
// deterministic (12-06 pattern), never racing this goroutine.
func NewSqlRuleMatcher(store sqlite.ApprovalRulesStore) *SqlRuleMatcher {
	return &SqlRuleMatcher{
		Store: store,
		OnMatch: func(rule *sqlite.ApprovalRule, req MatchRequest) {
			// context.Background(): the caller's ctx may be cancelled
			// by the time the goroutine runs; the accounting is
			// independent of the request lifecycle.
			ctx := context.Background()
			// Best-effort, T-11-05-09 accept disposition: hit_count and
			// the ledger are observability surfaces (the 56-03 panel
			// renders them); a failed increment must never fail an
			// auto-pass that already executed.
			_ = store.HitRule(ctx, rule.ID)
			// The ledger's matched_args_hash is the exact-tier probe
			// value; prefix/template hits store '' (the hash is
			// meaningless for them — the rule never compares it).
			matchedHash := req.ArgsHash
			if rule.MatchType != sqlite.MatchTypeExact {
				matchedHash = ""
			}
			_ = store.InsertRuleHit(ctx, sqlite.RuleHitRow{
				RuleID:          rule.ID,
				SessionID:       req.SessionID,
				Target:          rule.Target,
				ToolCallID:      req.ToolCallID,
				MatchType:       rule.MatchType,
				MatchedArgsHash: matchedHash,
			})
		},
	}
}

// Match implements RuleMatcher over the 56-02 engine. Store errors are
// treated as no-match: a rule-lookup failure must NEVER block the
// approval flow — the Tool falls back to the normal interrupt path,
// which is the safe direction (fail-closed toward human review, never
// toward silent execution).
func (m *SqlRuleMatcher) Match(ctx context.Context, req MatchRequest) (*sqlite.ApprovalRule, bool) {
	if m == nil || m.Store == nil {
		return nil, false
	}
	rule, _, err := m.Store.MatchRuleForArgs(ctx, sqlite.MatchRequest{
		Target:     req.Target,
		SkillID:    req.SkillID,
		SessionID:  req.SessionID,
		ArgsJSON:   req.ArgsJSON,
		ArgsHash:   req.ArgsHash,
		ToolCallID: req.ToolCallID,
	})
	if err != nil || rule == nil {
		return nil, false
	}
	if m.OnMatch != nil {
		go m.OnMatch(rule, req)
	}
	return rule, true
}

// RuleHitEmitFunc is the 56-02 SSE emission hook the Tool hit branch
// invokes AFTER MarkAutoPassedRule succeeded (audit row first, event
// after — the P12-L6 / 12-06 write_todos ordering). The signature is
// deliberately NARROW: the hitl package never touches the
// wire.DecisionPayload shape; the server-side closure
// (server.ruleMatchEmitter) assembles the data-decision chunk. nil
// hook = the event is not sent; the audit row is the durable record
// either way.
type RuleHitEmitFunc func(ruleID int64, matchType, scope, target, toolCallID string)

// NoopRuleMatcher never matches. Used by tests and by read-only Skills
// (writes: []) where no rule lookup should ever occur.
type NoopRuleMatcher struct{}

// Match implements RuleMatcher, always returning (nil, false).
func (NoopRuleMatcher) Match(_ context.Context, _ MatchRequest) (*sqlite.ApprovalRule, bool) {
	return nil, false
}
