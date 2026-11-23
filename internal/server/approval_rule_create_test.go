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

// approval_rule_create_test.go is Plan 56-02 Task 3 + Task 4's
// verification gate — the approve-and-remember closed loop:
//
//   - deriveApprovalRule unit table (scope mapping, tier shapes,
//     anti-smuggling validation, 422-able failures).
//   - E2E: approve + remember(session, prefix) → the rule row lands
//     with the documented field values (①), a smuggled pattern key →
//     422 with ZERO audit rows (③), a one-off approval → no rule (④),
//     and natural-key idempotency — the same tuple twice → one row (②).
//   - The FULL loop: remember → next same-args call auto-passes (no
//     interrupt) → hit_count + ledger row land → the rule-matched
//     frame carries ruleId/matchType/scope.
//   - The SSE frame shape via a real HTTP round trip.
//
// All tests are hermetic (in-memory-ish file SQLite + scripted chat
// model; no live GLM / Milvus / embedding calls).
package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/wire"
)

// ---------------------------------------------------------------------------
// deriveApprovalRule unit table
// ---------------------------------------------------------------------------

// TestDeriveApprovalRule — the creation contract table: scope mapping,
// per-tier shapes, and every 422-able validation failure.
func TestDeriveApprovalRule(t *testing.T) {
	row := &store.AuditRow{Query: "memory:facts", SkillID: "compliance"}

	t.Run("one-off scope creates no rule", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-d1", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "", ApprovalID: "tc-1",
		}, []byte(approvalToolArgs), rememberRuleParams{})
		require.NoError(t, err)
		assert.Nil(t, rule, "scope=\"\" = one-off approval — no rule row")
	})

	t.Run("session scope + exact default — hash of the EXECUTED bytes", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-d2", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-2",
		}, []byte(approvalToolArgs), rememberRuleParams{}) // match_type omitted → exact
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "memory:facts", rule.Target)
		assert.Equal(t, "compliance", rule.SkillID)
		assert.Equal(t, "session", rule.Scope)
		assert.Equal(t, "sess-d2", rule.ScopeSessionID)
		assert.Equal(t, sqlite.MatchTypeExact, rule.MatchType)
		assert.Equal(t, hitl.ComputeFrozenArgsHash([]byte(approvalToolArgs)), rule.ArgsHash)
		assert.Empty(t, rule.ArgsPattern)
		// The godoc-flagged zero-value trap: Enabled MUST be explicit true.
		assert.True(t, rule.Enabled, "a remembered rule MUST be born enabled")
		assert.Equal(t, sqlite.OriginApproveRemember, rule.Origin)
		assert.Zero(t, rule.ExpiresAt, "creation never expires; SetRuleExpiry retires later")
	})

	t.Run("skill scope — session id stays empty", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-d3", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "skill", ApprovalID: "tc-3",
		}, []byte(approvalToolArgs), rememberRuleParams{})
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, "skill", rule.Scope)
		assert.Empty(t, rule.ScopeSessionID, "skill-scope rules must not carry a session id")
	})

	t.Run("prefix — canonical pattern stored", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-d4", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-4",
		}, []byte(approvalToolArgs), rememberRuleParams{
			matchType:   "prefix",
			argsPattern: json.RawMessage(`{ "key" : "employee_count" }`), // sloppy FE bytes
		})
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, sqlite.MatchTypePrefix, rule.MatchType)
		assert.Empty(t, rule.ArgsHash, "prefix rules carry no args hash")
		assert.JSONEq(t, `{"key":"employee_count"}`, rule.ArgsPattern)
	})

	t.Run("template — target-level, pattern+hash stay empty", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-d5", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-5",
		}, []byte(approvalToolArgs), rememberRuleParams{matchType: "template"})
		require.NoError(t, err)
		require.NotNil(t, rule)
		assert.Equal(t, sqlite.MatchTypeTemplate, rule.MatchType)
		assert.Empty(t, rule.ArgsHash)
		assert.Empty(t, rule.ArgsPattern)
	})

	t.Run("unknown tier → 422-able validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-d6", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-6",
		}, []byte(approvalToolArgs), rememberRuleParams{matchType: "fuzzy"})
		require.Error(t, err)
		verr, ok := err.(*approvalRuleValidationError)
		require.True(t, ok, "the handler's 422 classifier needs the typed error")
		assert.Equal(t, "match_type", verr.field)
	})

	t.Run("prefix without pattern → validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-d7", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-7",
		}, []byte(approvalToolArgs), rememberRuleParams{matchType: "prefix"})
		require.Error(t, err)
		assert.Equal(t, "args_pattern", err.(*approvalRuleValidationError).field)
	})

	t.Run("prefix pattern not an object → validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-d8", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-8",
		}, []byte(approvalToolArgs), rememberRuleParams{
			matchType:   "prefix",
			argsPattern: json.RawMessage(`["key"]`),
		})
		require.Error(t, err)
		assert.Equal(t, "args_pattern", err.(*approvalRuleValidationError).field)
	})

	t.Run("prefix non-scalar locked value → validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-d9", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-9",
		}, []byte(approvalToolArgs), rememberRuleParams{
			matchType:   "prefix",
			argsPattern: json.RawMessage(`{"key":{"nested":true}}`),
		})
		require.Error(t, err)
		assert.Contains(t, err.(*approvalRuleValidationError).msg, "scalar")
	})

	t.Run("prefix smuggled key (not in frozenArgs) → validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-da", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-a",
		}, []byte(approvalToolArgs), rememberRuleParams{
			matchType:   "prefix",
			argsPattern: json.RawMessage(`{"region":"north"}`),
		})
		require.Error(t, err)
		assert.Contains(t, err.(*approvalRuleValidationError).msg, "anti-smuggling")
	})

	t.Run("prefix against non-object frozenArgs → validation error", func(t *testing.T) {
		_, err := deriveApprovalRule(row, "sess-db", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "session", ApprovalID: "tc-b",
		}, []byte(`[1,2]`), rememberRuleParams{
			matchType:   "prefix",
			argsPattern: json.RawMessage(`{"key":"k"}`),
		})
		require.Error(t, err)
		assert.Contains(t, err.(*approvalRuleValidationError).msg, "not a JSON object")
	})

	t.Run("global scope is not creatable from the approve flow", func(t *testing.T) {
		rule, err := deriveApprovalRule(row, "sess-dc", &hitl.ApprovalDecision{
			Decision: "approve", Scope: "global", ApprovalID: "tc-c",
		}, []byte(approvalToolArgs), rememberRuleParams{})
		require.NoError(t, err)
		assert.Nil(t, rule, "global rules come from the panel/seed, never the approve flow")
	})
}

// TestApprovalRememberIdempotent — the same natural-key tuple approved
// twice lands as ONE rule row (CreateRule's INSERT OR IGNORE).
func TestApprovalRememberIdempotent(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	row := &store.AuditRow{Query: "memory:facts", SkillID: "compliance"}
	decision := &hitl.ApprovalDecision{Decision: "approve", Scope: "session", ApprovalID: "tc-idem"}
	remember := rememberRuleParams{matchType: "exact"}

	rule1, err := deriveApprovalRule(row, "sess-idem-001", decision, []byte(approvalToolArgs), remember)
	require.NoError(t, err)
	require.NotNil(t, rule1)
	require.NoError(t, s.CreateRule(ctx, *rule1))

	// The operator re-approves the same call and remembers again —
	// semantically identical tuple → no duplicate.
	rule2, err := deriveApprovalRule(row, "sess-idem-001", decision, []byte(approvalToolArgs), remember)
	require.NoError(t, err)
	require.NotNil(t, rule2)
	require.NoError(t, s.CreateRule(ctx, *rule2))

	rules, err := s.QueryRules(ctx, "sess-idem-001", "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1, "re-approving the same tuple must not duplicate the rule")
}

// ---------------------------------------------------------------------------
// E2E — the remember path through the real POST handler
// ---------------------------------------------------------------------------

// TestApprovalRememberCreatesRule — ①: approve + scope=session +
// match_type=prefix + a valid pattern → the rule row lands with the
// documented field values (Target from the pending row, canonical
// pattern, Enabled=true, Origin=approve_remember, no expiry).
func TestApprovalRememberCreatesRule(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-remember-001"

	interruptID := interruptSession(t, ctx, s, sessionID, manifest)
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))
	stampCheckpointSnapshot(t, ctx, s, agent.RunnerCheckPointID(sessionID), snap)

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
		KBRegistry:     testKBReg(),
	}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
		"scope":        "session",
		"match_type":   "prefix",
		"args_pattern": map[string]any{"key": "employee_count"},
	})
	require.Equal(t, http.StatusOK, status, "approve+remember must succeed; body: %s", string(body))

	rules, err := s.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1, "exactly one rule row must land")
	rule := rules[0]
	assert.Equal(t, "memory:facts", rule.Target, "Target comes from the pending row")
	assert.Equal(t, "compliance", rule.SkillID)
	assert.Equal(t, "session", rule.Scope)
	assert.Equal(t, sessionID, rule.ScopeSessionID)
	assert.Equal(t, sqlite.MatchTypePrefix, rule.MatchType)
	assert.Empty(t, rule.ArgsHash, "prefix rules carry no args hash")
	assert.JSONEq(t, `{"key":"employee_count"}`, rule.ArgsPattern, "the pattern is stored canonical")
	assert.True(t, rule.Enabled, "a remembered rule is born enabled (zero-value trap)")
	assert.Equal(t, sqlite.OriginApproveRemember, rule.Origin)
	assert.Zero(t, rule.ExpiresAt)
	assert.Zero(t, rule.HitCount, "a fresh rule has never fired")
}

// TestApprovalRememberSmuggledKey422NoAuditRow — ③: a prefix pattern
// locking a key the pending call's args never had → 422, NO audit
// resolution row, NO rule row, pending row untouched.
func TestApprovalRememberSmuggledKey422NoAuditRow(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-smuggl-0001"

	interruptID := interruptSession(t, ctx, s, sessionID, manifest)
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))
	stampCheckpointSnapshot(t, ctx, s, agent.RunnerCheckPointID(sessionID), snap)

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
		KBRegistry:     testKBReg(),
	}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
		"scope":        "session",
		"match_type":   "prefix",
		"args_pattern": map[string]any{"region": "north"}, // NOT in the pending args
	})
	require.Equal(t, http.StatusUnprocessableEntity, status, "smuggled key must 422; body: %s", string(body))
	assert.Contains(t, string(body), "args_pattern", "the failing field must be named")

	// No resolution row may exist (validation runs BEFORE MarkApproved).
	for _, action := range []string{"action_approved", "action_rejected"} {
		res, qErr := s.QueryAudit(ctx, store.AuditFilter{
			SessionID:  sessionID,
			Action:     action,
			ToolCallID: approvalToolCallID,
			Limit:      1,
		})
		require.NoError(t, qErr)
		assert.Empty(t, res, "no %s row may be written when the rule shape is invalid", action)
	}

	// No rule row either.
	rules, err := s.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	assert.Empty(t, rules)

	// The pending row survives for a corrected retry.
	pending, err := s.QueryAudit(ctx, store.AuditFilter{
		SessionID:  sessionID,
		Action:     "action_pending_approval",
		ToolCallID: approvalToolCallID,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, pending, 1, "the pending row survives for a corrected retry")
}

// TestApprovalOneOffCreatesNoRule — ④: scope omitted (one-off approve)
// resolves the approval but lands ZERO rule rows in every scope layer.
func TestApprovalOneOffCreatesNoRule(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-oneoff-0001"

	interruptID := interruptSession(t, ctx, s, sessionID, manifest)
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))
	stampCheckpointSnapshot(t, ctx, s, agent.RunnerCheckPointID(sessionID), snap)

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
		KBRegistry:     testKBReg(),
	}
	addr, cleanup := approvalTestHertz(t, deps)
	defer cleanup()

	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
		// scope omitted — one-off
	})
	require.Equal(t, http.StatusOK, status, "one-off approve must succeed; body: %s", string(body))

	for _, scope := range []sqlite.RuleScope{sqlite.RuleScopeSession, sqlite.RuleScopeSkill, sqlite.RuleScopeGlobal} {
		rules, err := s.QueryRules(ctx, sessionID, "compliance", scope)
		require.NoError(t, err)
		assert.Empty(t, rules, "scope=%s must have zero rule rows after a one-off approve", scope)
	}
}

// ---------------------------------------------------------------------------
// The closed loop — remember → auto-pass → accounting → SSE frame
// ---------------------------------------------------------------------------

// TestApprovalRememberThenAutoPass — the D-56-02 loop end to end:
// remember an exact rule (phase 1), then run the SAME tool call again
// in the same session with the production matcher wired — the write
// auto-passes (NO interrupt), the hit accounting lands (hit_count+1 +
// one ledger row), and the ruleHitEmit hook fires with the winning
// rule's identity.
func TestApprovalRememberThenAutoPass(t *testing.T) {
	ctx := context.Background()
	s := historyTestStore(t)
	manifest := approvalManifest()
	sessionID := "sess-loop-000001"

	// Phase 1: interrupt → approve-and-remember (exact, session scope).
	interruptID := interruptSession(t, ctx, s, sessionID, manifest)
	snap := approvalSnapshot(manifest)
	seedPendingApproval(t, ctx, s, sessionID, snap, approvalToolCallID, "memory:facts", []byte(approvalToolArgs))
	stampCheckpointSnapshot(t, ctx, s, agent.RunnerCheckPointID(sessionID), snap)

	deps := &Deps{
		Store:          s,
		ChatModel:      &scriptedApprovalModel{},
		ActiveManifest: manifest,
		Embedder:       newStubEmbedder(),
		Retriever:      &stubRetriever{},
		KBRegistry:     testKBReg(),
	}
	addr, cleanup := approvalTestHertz(t, deps)
	status, body := postApproval(t, addr, sessionID, approvalToolCallID, map[string]any{
		"decision":     "approve",
		"target":       "memory:facts",
		"approval_id":  approvalToolCallID,
		"interrupt_id": interruptID,
		"scope":        "session",
		// match_type omitted → exact (the default tier)
	})
	require.Equal(t, http.StatusOK, status, "phase-1 remember approve; body: %s", string(body))
	cleanup()

	rules, err := s.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSession)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	ruleID := rules[0].ID
	assert.Equal(t, sqlite.MatchTypeExact, rules[0].MatchType)

	// Phase 2: the SAME call again, with the production matcher + a
	// captured emit hook. The Tool's Pass-1 must auto-pass.
	var mu sync.Mutex
	var emitRuleID int64
	var emitType, emitScope, emitTarget, emitCallID string
	emitCalls := 0
	emitter := func(ruleID int64, matchType, scope, target, toolCallID string) {
		mu.Lock()
		defer mu.Unlock()
		emitCalls++
		emitRuleID, emitType, emitScope, emitTarget, emitCallID =
			ruleID, matchType, scope, target, toolCallID
	}

	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        newStubEmbedder(),
		Retriever:       &stubRetriever{},
		ChatModel:       &scriptedApprovalModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
		KBRegistry:      testKBReg(),
		RuleMatcher:     hitl.NewSqlRuleMatcher(s),
		RuleHitEmit:     emitter,
	})
	require.NoError(t, err, "phase-2 BuildRunner")

	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("persist the employee count again")},
		adk.WithCheckPointID(agent.RunnerCheckPointID(sessionID)))
	sawInterrupt := false
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		require.NoError(t, ev.Err, "phase-2 events must be error-free")
		if ev.Action != nil && ev.Action.Interrupted != nil {
			sawInterrupt = true
		}
	}
	assert.False(t, sawInterrupt, "a remembered call must auto-pass — no interrupt")

	mu.Lock()
	require.Equal(t, 1, emitCalls, "the hit branch must emit exactly one rule-matched event")
	assert.Equal(t, ruleID, emitRuleID)
	assert.Equal(t, "exact", emitType)
	assert.Equal(t, "session", emitScope)
	assert.Equal(t, "memory:facts", emitTarget)
	assert.NotEmpty(t, emitCallID)
	mu.Unlock()

	// Hit accounting is fire-and-forget — poll until it lands.
	require.Eventually(t, func() bool {
		rules, err := s.QueryRules(ctx, sessionID, "", sqlite.RuleScopeSession)
		return err == nil && len(rules) == 1 && rules[0].HitCount == 1
	}, 2*time.Second, 10*time.Millisecond, "hit_count must increment after the auto-pass")

	require.Eventually(t, func() bool {
		hits, err := s.QueryRuleHits(ctx, ruleID, 10)
		return err == nil && len(hits) == 1
	}, 2*time.Second, 10*time.Millisecond, "one hit-ledger row must land after the auto-pass")

	hits, err := s.QueryRuleHits(ctx, ruleID, 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, "sess-loop-000001", hits[0].SessionID)
	assert.Equal(t, "memory:facts", hits[0].Target)
	assert.Equal(t, sqlite.MatchTypeExact, hits[0].MatchType)
	assert.Equal(t, hitl.ComputeFrozenArgsHash([]byte(approvalToolArgs)), hits[0].MatchedArgsHash)
}

// TestRuleMatchedFrameShape — the SSE frame the server emitter builds:
// a real HTTP round trip asserting the data-decision JSON carries
// decision:"rule-matched" + ruleId / ruleMatchType / ruleScope, and
// that the frame type is the EXISTING data-decision discriminator
// (D-04 / D15 — no new chunk type).
func TestRuleMatchedFrameShape(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	h := server.Default(server.WithListener(ln))
	deps := &Deps{}
	h.GET("/emit-rule-match", func(ctx context.Context, c *hertzapp.RequestContext) {
		deps.ruleMatchEmitter(c)(42, "prefix", "session", "memory:facts", "tc-frame-01")
	})
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	defer func() {
		_ = h.Close()
		<-errCh
	}()
	addr := "127.0.0.1:" + portStr(port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "frame-test server warmup")

	resp, err := http.Get("http://" + addr + "/emit-rule-match")
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	text := string(raw)

	assert.Contains(t, text, `"type":"data-decision"`, "the frame rides the EXISTING data-decision type (D15)")
	assert.Contains(t, text, `"decision":"rule-matched"`)
	assert.Contains(t, text, `"ruleId":42`)
	assert.Contains(t, text, `"ruleMatchType":"prefix"`)
	assert.Contains(t, text, `"ruleScope":"session"`)
	assert.Contains(t, text, `"toolCallId":"tc-frame-01"`)
	assert.Contains(t, text, `"query":"memory:facts"`)

	// Structural backstop: the payload decodes into wire.DecisionPayload
	// with the additive fields set.
	var frame wire.UIMessageChunk
	dataStart := strings.Index(text, "{")
	require.GreaterOrEqual(t, dataStart, 0, "an SSE data line must carry a JSON object")
	// Extract the JSON payload after the "data: " prefix.
	dataLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "data:") {
			dataLine = strings.TrimPrefix(line, "data:")
			break
		}
	}
	require.NotEmpty(t, dataLine, "one data: line expected")
	require.NoError(t, json.Unmarshal([]byte(dataLine), &frame))
	require.Equal(t, "data-decision", frame.Type)
	var payload wire.DecisionPayload
	require.NoError(t, json.Unmarshal(frame.Data, &payload))
	assert.Equal(t, wire.DecisionRuleMatched, payload.Decision)
	assert.Equal(t, int64(42), payload.RuleID)
	assert.Equal(t, "prefix", payload.RuleMatchType)
	assert.Equal(t, "session", payload.RuleScope)
}
