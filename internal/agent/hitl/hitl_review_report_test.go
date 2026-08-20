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
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewReportToolFirstPassInterrupts — Pass 1 of the
// StatefulInterrupt contract (mirrors
// TestMemoryFactsToolFirstPassInterrupts). A context WITHOUT prior
// interrupt state must:
//  1. call the audit-writer hook exactly once with an ApprovalInfo
//     echoing the verbatim args + the review_report target,
//  2. return an interrupt-anchored error and no string output.
//
// The spy writer is the shared spyAuditWriter declared in
// hitl_memory_facts_test.go (same package) — not redeclared here.
func TestReviewReportToolFirstPassInterrupts(t *testing.T) {
	called := 0
	var capturedInfo *ApprovalInfo

	h := &reviewReportTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, info *ApprovalInfo) error {
				called++
				capturedInfo = info
				return nil
			},
		},
	}

	ctx := context.Background()
	args := `{"report":"Q3 compliance review","citations":["c1","c2"]}`
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
	assert.Equal(t, ReviewReportToolName, capturedInfo.ToolName)
	assert.Equal(t, ReviewReportTarget, capturedInfo.Target)
	assert.Equal(t, args, capturedInfo.ArgumentsInJSON,
		"the ApprovalInfo must echo the verbatim args string for the FE")
	assert.NotEmpty(t, capturedInfo.ToolCallID,
		"ApprovalInfo.ToolCallID must be non-empty (== ApprovalID contract)")
}

// TestReviewReportToolSecondWriteTool — HITL-06 completion lock: the
// registry carries BOTH write tools after 11-04, with distinct
// LLM-visible names.
func TestReviewReportToolSecondWriteTool(t *testing.T) {
	rr := LookupHandler(ReviewReportTarget)
	require.NotNil(t, rr, "review_report handler must be registered after 11-04")
	mf := LookupHandler(MemoryFactsTarget)
	require.NotNil(t, mf, "memory:facts handler must still be registered (11-02 unbroken)")

	rrInfo, err := rr.Info(context.Background())
	require.NoError(t, err)
	mfInfo, err := mf.Info(context.Background())
	require.NoError(t, err)

	assert.NotEmpty(t, rrInfo.Name)
	assert.NotEmpty(t, mfInfo.Name)
	assert.Equal(t, ReviewReportToolName, rrInfo.Name)
	assert.Equal(t, MemoryFactsToolName, mfInfo.Name)
	assert.NotEqual(t, mfInfo.Name, rrInfo.Name,
		"the two write tools must have distinct LLM-visible names")
}

// TestReviewReportToolRejectsDenyWithoutReason — the ApprovalDecision
// validation boundary (HITL-02). Mirrors
// TestMemoryFactsToolRejectsDenyWithoutReason: the Validate gate is
// exercised directly because the framework-level resume-from-
// checkpoint path requires a live Runner (covered by 11-04's
// server-side integration tests).
func TestReviewReportToolRejectsDenyWithoutReason(t *testing.T) {
	bad := &ApprovalDecision{
		Decision:   "deny",
		Reason:     "",
		Target:     ReviewReportTarget,
		ApprovalID: "abc",
	}
	err := bad.Validate()
	require.Error(t, err)
	verr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "reason", verr.Field,
		"deny-without-reason must surface field=\"reason\" (HITL-02 audit-friendly)")
}

// TestReviewReportToolInfoSchema — the LLM-visible schema MUST declare
// report + citations as required. Any drift here changes the Tool
// schema the LLM sees.
func TestReviewReportToolInfoSchema(t *testing.T) {
	h := &reviewReportTool{}
	info, err := h.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, ReviewReportToolName, info.Name)
	assert.NotEmpty(t, info.Desc, "tool desc must include approval-required phrasing")
	require.NotNil(t, info.ParamsOneOf)

	pm, err := info.ParamsOneOf.ToJSONSchema()
	require.NoError(t, err, "ParamsOneOf must flatten to a JSON Schema")
	require.Contains(t, pm.Required, "report",
		"report must be required (an empty report body is meaningless)")
	require.Contains(t, pm.Required, "citations",
		"citations must be required (an unverifiable report cannot be approved)")
}

// TestReviewReportArgsParse — defense-in-depth: json.Unmarshal of the
// canonical args shape succeeds (the Pass-2 approve path depends on
// these exact json tags).
func TestReviewReportArgsParse(t *testing.T) {
	var parsed ReviewReportArgs
	raw := json.RawMessage(`{"report":"Q3 review","citations":["c1","c2"]}`)
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, "Q3 review", parsed.Report)
	assert.Equal(t, []string{"c1", "c2"}, parsed.Citations)
}
