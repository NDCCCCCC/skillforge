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

// hitl_plan_approve_test.go is Phase 12 / Plan 12-03's hermetic
// test suite for the plan_approve HITL Tool. The test set mirrors
// TestReviewReportTool* — Pass 1 (interrupt), input validation,
// schema, and the registry presence — plus the 12-03-specific
// assertions (audit fail-closed, approval_id == tool_call_id
// identity invariant).
//
// NOTE on Pass 2 execution coverage: eino@v0.9.14's interrupt state
// (core.GetInterruptState / core.GetResumeContext) is populated only
// by a REAL StatefulInterrupt cycle through a real Runner — there is
// no public seam to inject (wasInterrupted=true, ApprovalDecision
// payload) without executing the full interrupt/checkpoint path.
// Pass 2 execution is therefore covered by:
//   - TestPlanApproveSecondPassInvalidDecision (validation gate
//     only; the Tool's resume branch is reached via direct Validate
//     call against a Decision with empty Decision field — see
//     hitl_review_report_test.go's parallel pattern).
//   - 12-04's plan_api.go integration tests (server-side Resume
//     against a real Runner; out of scope for 12-03's Tool layer).
//
// This matches the existing convention: hitl_review_report_test.go
// and hitl_memory_facts_test.go both ship zero Pass 2 execution
// tests at the Tool layer for the same reason.
package hitl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlanApproveFirstPassInterrupts — Pass 1 of the
// StatefulInterrupt contract (mirrors
// TestReviewReportToolFirstPassInterrupts). A context WITHOUT prior
// interrupt state must:
//  1. call the audit-writer hook exactly once with an ApprovalInfo
//     echoing the verbatim args + the plan_approve target,
//  2. return an interrupt-anchored error and no string output.
func TestPlanApproveFirstPassInterrupts(t *testing.T) {
	called := 0
	var capturedInfo *ApprovalInfo
	var capturedArgs []byte

	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, info *ApprovalInfo) error {
				called++
				capturedInfo = info
				capturedArgs = []byte(info.ArgumentsInJSON)
				return nil
			},
		},
	}

	ctx := context.Background()
	args := `{"planId":"plan-1","steps":[{"id":"s1","content":"Step 1","status":"pending"},{"id":"s2","content":"Step 2","status":"pending"}],"estimatedIterations":5,"riskAssessment":"medium"}`
	out, err := h.InvokableRun(ctx, args)

	assert.Empty(t, out, "Pass 1 must not produce string output")
	require.Error(t, err, "Pass 1 must return an interrupt error")

	errMsg := err.Error()
	hasInterruptAnchor := strings.Contains(strings.ToLower(errMsg), "interrupt") ||
		strings.Contains(strings.ToLower(errMsg), "approval")
	assert.True(t, hasInterruptAnchor,
		"Pass 1 error must surface an interrupt/approval signal; got %q", errMsg)

	assert.Equal(t, 1, called, "audit writer hook must be called exactly once on Pass 1")
	require.NotNil(t, capturedInfo, "audit writer hook must receive an ApprovalInfo")
	assert.Equal(t, PlanApproveToolName, capturedInfo.ToolName)
	assert.Equal(t, PlanApproveTarget, capturedInfo.Target)
	assert.Equal(t, args, string(capturedArgs),
		"the ApprovalInfo must echo the verbatim args bytes for the audit row")
	assert.Equal(t, args, capturedInfo.ArgumentsInJSON,
		"the ApprovalInfo must echo the verbatim args string for the FE")
	assert.NotEmpty(t, capturedInfo.ToolCallID,
		"ApprovalInfo.ToolCallID must be non-empty (== ApprovalID contract)")
}

// TestPlanApproveEmptyPlanID — input validation: PlanID is REQUIRED.
// Pass 1 must reject an empty planId at parse time, BEFORE calling
// the audit writer (the audit row must not exist for invalid input).
func TestPlanApproveEmptyPlanID(t *testing.T) {
	called := 0
	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, _ *ApprovalInfo) error {
				called++
				return nil
			},
		},
	}
	out, err := h.InvokableRun(context.Background(),
		`{"planId":"","steps":[{"id":"s1","content":"x","status":"pending"}]}`)
	assert.Empty(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "planId is required")
	assert.Equal(t, 0, called, "audit writer must NOT be called when validation fails (D11-C3 + fail-closed)")
}

// TestPlanApproveEmptySteps — input validation: Steps is REQUIRED and
// must be non-empty. A plan with zero steps is meaningless — the
// operator reviewing the ApprovalCard would see an empty body.
func TestPlanApproveEmptySteps(t *testing.T) {
	called := 0
	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, _ *ApprovalInfo) error {
				called++
				return nil
			},
		},
	}
	out, err := h.InvokableRun(context.Background(),
		`{"planId":"plan-1","steps":[]}`)
	assert.Empty(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one step is required")
	assert.Equal(t, 0, called, "audit writer must NOT be called when validation fails")
}

// TestPlanApproveInvalidJSON — input validation: malformed JSON is
// rejected at Unmarshal with the canonical "invalid args" wrapper.
func TestPlanApproveInvalidJSON(t *testing.T) {
	called := 0
	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, _ *ApprovalInfo) error {
				called++
				return nil
			},
		},
	}
	out, err := h.InvokableRun(context.Background(), `not-json`)
	assert.Empty(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid args")
	assert.Equal(t, 0, called, "audit writer must NOT be called when JSON parse fails")
}

// TestPlanApproveApprovalDecisionValidation — ApprovalDecision
// validation boundary (mirrors TestMemoryFactsToolRejectsDenyWithoutReason
// and TestReviewReportToolRejectsDenyWithoutReason). The Tool's
// Pass 2 branch calls ApprovalDecision.Validate FIRST, so we
// exercise the gate directly here: the framework-level
// resume-from-checkpoint path requires a live Runner (covered by
// 12-04's plan_api.go integration tests).
func TestPlanApproveApprovalDecisionValidation(t *testing.T) {
	t.Run("deny_without_reason_rejected", func(t *testing.T) {
		bad := &ApprovalDecision{
			Decision:   "deny",
			Reason:     "",
			Target:     PlanApproveTarget,
			ApprovalID: "abc",
		}
		err := bad.Validate()
		require.Error(t, err)
		verr, ok := err.(*ValidationError)
		require.True(t, ok)
		assert.Equal(t, "reason", verr.Field,
			"deny-without-reason must surface field=\"reason\" (HITL-02 audit-friendly)")
	})
	t.Run("unknown_decision_rejected", func(t *testing.T) {
		bad := &ApprovalDecision{
			Decision:   "maybe",
			Target:     PlanApproveTarget,
			ApprovalID: "abc",
		}
		err := bad.Validate()
		require.Error(t, err)
		verr, ok := err.(*ValidationError)
		require.True(t, ok)
		assert.Equal(t, "decision", verr.Field,
			"unknown Decision value must surface field=\"decision\"")
	})
	t.Run("empty_approval_id_rejected", func(t *testing.T) {
		bad := &ApprovalDecision{
			Decision: "approve",
			Reason:   "ok",
			Target:   PlanApproveTarget,
		}
		err := bad.Validate()
		require.Error(t, err)
		verr, ok := err.(*ValidationError)
		require.True(t, ok)
		assert.Equal(t, "approval_id", verr.Field,
			"empty approval_id must surface field=\"approval_id\" (D11-C4 single identity)")
	})
	t.Run("approve_with_target_match_passes", func(t *testing.T) {
		good := &ApprovalDecision{
			Decision:   "approve",
			Reason:     "looks fine",
			Target:     PlanApproveTarget,
			ApprovalID: "plan-1",
		}
		assert.NoError(t, good.Validate(),
			"approve with reason + matching target + non-empty approval_id must pass")
	})
	t.Run("deny_with_reason_for_plan_approve_passes", func(t *testing.T) {
		good := &ApprovalDecision{
			Decision:   "deny",
			Reason:     "too many steps",
			Target:     PlanApproveTarget,
			ApprovalID: "plan-1",
		}
		assert.NoError(t, good.Validate(),
			"deny with reason + matching target + non-empty approval_id must pass")
	})
}

// TestPlanApproveApprovalIDIdentityInvariant — D11-C4: the audit
// row's approval_id field equals the tool_call_id. On the Pass 1
// path, the Tool falls back to args.PlanID when
// compose.GetToolCallID returns "" — a stable identity across the
// agent's rewrite-and-re-submit iterations.
func TestPlanApproveApprovalIDIdentityInvariant(t *testing.T) {
	captured := &ApprovalInfo{}
	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, info *ApprovalInfo) error {
				*captured = *info
				return nil
			},
		},
	}

	args := `{"planId":"plan-stability-id","steps":[{"id":"s1","content":"x","status":"pending"}],"estimatedIterations":1,"riskAssessment":"low"}`
	_, _ = h.InvokableRun(context.Background(), args)

	require.NotEmpty(t, captured.ToolCallID,
		"ToolCallID must be populated (either compose.GetToolCallID or PlanID fallback)")
	assert.Equal(t, "plan-stability-id", captured.ToolCallID,
		"PlanID fallback must produce a stable identity when compose.GetToolCallID returns ''")
}

// TestPlanApproveAuditFailClosed — D11-C3 fail-closed posture. When
// the audit INSERT fails, the Tool must return the canonical
// "audit INSERT failed (fail-closed)" error BEFORE the interrupt
// fires — the operator (not the LLM) decides how to retry.
func TestPlanApproveAuditFailClosed(t *testing.T) {
	h := &planApproveTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, _ *ApprovalInfo) error {
				return errors.New("simulated sqlite busy")
			},
		},
	}

	args := `{"planId":"plan-fail","steps":[{"id":"s1","content":"x","status":"pending"}],"estimatedIterations":1,"riskAssessment":"low"}`
	out, err := h.InvokableRun(context.Background(), args)

	assert.Empty(t, out, "audit failure must NOT produce string output")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audit INSERT failed (fail-closed)",
		"audit failure must surface the fail-closed wrapper")
	assert.Contains(t, err.Error(), "simulated sqlite busy",
		"the underlying error must be wrapped (not swallowed)")
}

// TestPlanApproveRegistryAllowedTargetsContains — Task 2 contract:
// the registry's AllowedWriteTargets slice MUST contain "plan_approve"
// after this plan ships. The plan_approve Tool cannot reach an LLM
// without this entry. (This test mirrors the registry_test.go version
// for hermetic-test coverage at this layer; the canonical assertion
// lives in registry_test.go alongside the D11-B3 source-of-truth lock.)
//
// TestPlanApproveRegistryHandlerExists — same posture as above; the
// canonical lock lives in registry_test.go.
//
// (The two tests below were moved to registry_test.go to keep the
// D11-B3 source-of-truth assertions co-located. They are removed
// from this file to avoid the duplicate-name build error.)

// TestPlanApproveInfoSchema — the LLM-visible schema MUST declare
// planId + steps as required. Any drift here changes the Tool
// schema the LLM sees.
func TestPlanApproveInfoSchema(t *testing.T) {
	h := &planApproveTool{}
	info, err := h.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, PlanApproveToolName, info.Name)
	assert.NotEmpty(t, info.Desc, "tool desc must include approval-required phrasing")
	require.NotNil(t, info.ParamsOneOf)

	pm, err := info.ParamsOneOf.ToJSONSchema()
	require.NoError(t, err, "ParamsOneOf must flatten to a JSON Schema")
	require.Contains(t, pm.Required, "planId",
		"planId must be required (every plan needs a stable identifier for audit linkage)")
	require.Contains(t, pm.Required, "steps",
		"steps must be required (an empty plan is meaningless)")
}

// TestPlanApproveArgsParse — defense-in-depth: json.Unmarshal of the
// canonical args shape succeeds. The Pass-2 approve path depends on
// these exact json tags.
func TestPlanApproveArgsParse(t *testing.T) {
	var parsed PlanApproveArgs
	raw := json.RawMessage(`{"planId":"p1","steps":[{"id":"s1","content":"Step 1","activeForm":"Doing step 1","status":"pending"}],"estimatedIterations":3,"riskAssessment":"medium"}`)
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, "p1", parsed.PlanID)
	require.Len(t, parsed.Steps, 1)
	assert.Equal(t, "s1", parsed.Steps[0].ID)
	assert.Equal(t, "Step 1", parsed.Steps[0].Content)
	assert.Equal(t, "Doing step 1", parsed.Steps[0].ActiveForm)
	assert.Equal(t, "pending", parsed.Steps[0].Status)
	assert.Equal(t, 3, parsed.EstimatedIterations)
	assert.Equal(t, "medium", parsed.RiskAssessment)
}

// invokableToolPlanApprove is the type assertion sentinel: a
// planApproveTool value MUST satisfy tool.InvokableTool so the eino
// compose.ToolsNodeConfig accepts it. Compile-time check.
var _ tool.InvokableTool = (*planApproveTool)(nil)
