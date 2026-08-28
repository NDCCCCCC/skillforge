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

package hitl

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllowedTargetsSubset — D11-B3 shared source: the registry's
// AllowedWriteTargets slice contains exactly the targets the
// Skill validator + BuildToolsForSkill both read. Locked here as a
// 4-way check (length, presence, sorted defense-copy, immutable
// underlying slice) so accidental edit of the var surfaces
// immediately.
func TestAllowedTargetsSubset(t *testing.T) {
	got := KnownTargets()
	require.NotEmpty(t, got, "KnownTargets must return at least one target (memory:facts is the 11-02 baseline)")

	// Sorted lexicographically. Failure here signals that the
	// validator's error messages will drift across calls (operator
	// sees a different order on each reload). Phase 12 / Plan 12-03
	// adds "plan_approve" between "memory:facts" and "review_report"
	// after sorting (the slice order is independent of the
	// defensive-copy sort).
	assert.Equal(t, []string{"memory:facts", "plan_approve", "report:publish", "review_report", "skill_state"}, got,
		"KnownTargets must be sorted lexicographically (registry contract). "+
			"`skill_state` is the D7-10 calculator placeholder and intentionally "+
			"co-lives with the 11-02 + 11-04 + 12-03 write targets. "+
			"D18-A4 adds report:publish between plan_approve and review_report.")

	// Defensive-copy contract: callers MUST NOT mutate the
	// underlying slice. Mutate the returned copy, re-call, assert
	// unchanged.
	got[0] = "tampered"
	again := KnownTargets()
	assert.NotEqual(t, "tampered", again[0],
		"KnownTargets must return a defensive copy — caller mutation must not reach the source")
	assert.Equal(t, "memory:facts", again[0],
		"KnownTargets re-call must yield the original (untampered) head")
}

// TestPlanApproveRegistryAllowedTargetsContains — Plan 12-03
// D11-B3 single-source-of-truth lock. The registry's
// AllowedWriteTargets slice MUST contain "plan_approve" after this
// plan ships. Without this entry, BuildToolsForSkill silently
// drops the plan_approve write from Skills that declare it — the
// PLAN-03 plan confirmation gate would be unreachable from any
// Skill.
func TestPlanApproveRegistryAllowedTargetsContains(t *testing.T) {
	assert.True(t, Contains(AllowedWriteTargets, "plan_approve"),
		"AllowedWriteTargets must contain \"plan_approve\" (PLAN-03 / D11-B3 single source of truth)")
}

// TestPlanApproveRegistryHandlerExists — Plan 12-03 / D11-B3:
// hitlHandlers["plan_approve"] MUST register &planApproveTool{}.
// BuildToolsForSkill consumes this map to construct the LLM-visible
// Tool whitelist per turn.
func TestPlanApproveRegistryHandlerExists(t *testing.T) {
	h := LookupHandler("plan_approve")
	require.NotNil(t, h, "LookupHandler must return non-nil for \"plan_approve\" (D11-B3 wiring)")
	info, err := h.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, PlanApproveToolName, info.Name,
		"the registered handler must be the planApproveTool (Info.Name == \"plan_approve_review\")")
	require.NotNil(t, info.ParamsOneOf,
		"planApproveTool.Info() must include ParamsOneOf (LLM-visible schema)")
}

// TestApprovalDecisionRoundTrip — D11-B2 dual registration. The
// struct that crosses the gob checkpoint MUST round-trip byte-
// equality through encoding/gob; the JSON wire shape (FE ->
// backend) must round-trip via encoding/json. FrozenArgs is the
// load-bearing field (HITL-07 byte-equality invariant with the
// audit row's frozen_args column).
//
// We exercise BOTH halves: gob round-trip via bytes.Buffer +
// gob.Encoder/Decoder, JSON round-trip via json.Marshal +
// json.Unmarshal. Both must produce the same ApprovalDecision
// values (Decision, Reason, Target, FrozenArgs, Scope, ApprovalID
// all preserved; FrozenArgs is byte-equal in both edges).
func TestApprovalDecisionRoundTrip(t *testing.T) {
	src := &ApprovalDecision{
		Decision:   "approve",
		Reason:     "verified by operator 2026-08-20",
		Target:     "memory:facts",
		FrozenArgs: json.RawMessage(`{"key":"employee_count","value":"150","tier":"权威"}`),
		Scope:      "session",
		ApprovalID: "abc-123-def-456",
	}

	// --- gob half ----------------------------------------------------
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(src))
	var viaGob ApprovalDecision
	require.NoError(t, gob.NewDecoder(&buf).Decode(&viaGob),
		"ApprovalDecision must gob-round-trip (D11-B2 on-disk checkpoint)")

	assert.Equal(t, src.Decision, viaGob.Decision)
	assert.Equal(t, src.Reason, viaGob.Reason)
	assert.Equal(t, src.Target, viaGob.Target)
	assert.Equal(t, src.Scope, viaGob.Scope)
	assert.Equal(t, src.ApprovalID, viaGob.ApprovalID)
	assert.Equal(t, src.FrozenArgs, viaGob.FrozenArgs,
		"FrozenArgs must byte-equal across gob round-trip (HITL-07 byte-equality invariant)")

	// --- JSON half --------------------------------------------------
	raw, err := json.Marshal(src)
	require.NoError(t, err)
	var viaJSON ApprovalDecision
	require.NoError(t, json.Unmarshal(raw, &viaJSON),
		"ApprovalDecision must JSON-round-trip (FE wire shape)")
	assert.Equal(t, src.Decision, viaJSON.Decision)
	assert.Equal(t, src.Reason, viaJSON.Reason)
	assert.Equal(t, src.Target, viaJSON.Target)
	assert.Equal(t, src.FrozenArgs, viaJSON.FrozenArgs,
		"FrozenArgs must byte-equal across JSON round-trip")
}

// TestApprovalDecisionValidate — table-driven: 4 failure modes
// (unknown decision, deny-without-reason, empty approval_id,
// unknown scope) plus the valid pass. The returned error must be
// *ValidationError with the expected Field name so the FE can
// highlight the right form input.
func TestApprovalDecisionValidate(t *testing.T) {
	cases := []struct {
		name      string
		d         *ApprovalDecision
		expectErr bool
		field     string
	}{
		{
			name: "valid_approve_with_reason_and_id",
			d: &ApprovalDecision{
				Decision:   "approve",
				Reason:     "looks fine",
				Target:     "memory:facts",
				ApprovalID: "abc-123",
				Scope:      "session",
			},
			expectErr: false,
		},
		{
			name: "deny_without_reason_rejected",
			d: &ApprovalDecision{
				Decision:   "deny",
				Reason:     "",
				Target:     "memory:facts",
				ApprovalID: "abc-123",
			},
			expectErr: true,
			field:     "reason",
		},
		{
			name: "unknown_decision_rejected",
			d: &ApprovalDecision{
				Decision:   "maybe",
				ApprovalID: "abc-123",
			},
			expectErr: true,
			field:     "decision",
		},
		{
			name: "empty_approval_id_rejected",
			d: &ApprovalDecision{
				Decision: "approve",
				Reason:   "ok",
			},
			expectErr: true,
			field:     "approval_id",
		},
		{
			name: "unknown_scope_rejected",
			d: &ApprovalDecision{
				Decision:   "approve",
				Reason:     "ok",
				Target:     "memory:facts",
				ApprovalID: "abc-123",
				Scope:      "tenant",
			},
			expectErr: true,
			field:     "scope",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.d.Validate()
			if !tc.expectErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			verr, ok := err.(*ValidationError)
			require.True(t, ok,
				"expected *ValidationError, got %T", err)
			assert.Equal(t, tc.field, verr.Field,
				"the failing field name must be reported so the FE can highlight it")
		})
	}
}

// TestApprovalDecisionTargetMatches — TargetMatches echoes the
// hit-handling carve-out: empty target is recoverable (audit_writer
// backfills), non-empty mismatch is a spoof signal.
func TestApprovalDecisionTargetMatches(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		expected string
		want     bool
	}{
		{"empty_target_backfill_allowed", "", "memory:facts", true},
		{"matching_target_passes", "memory:facts", "memory:facts", true},
		{"mismatch_rejected", "review_report", "memory:facts", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &ApprovalDecision{Target: tc.target}
			assert.Equal(t, tc.want, d.TargetMatches(tc.expected))
		})
	}
}
