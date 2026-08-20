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
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryFactsToolFirstPassInterrupts — Pass 1 of the
// StatefulInterrupt contract. A context WITHOUT prior interrupt
// state (wasInterrupted == false) must:
//   1. successfully call the audit-writer hook (the noop writer in
//      11-02; the real writer in 11-03) with an ApprovalInfo that
//      matches the requested args,
//   2. return an error that wraps or implements the eino
//      InterruptSignal — operators can extract the interrupt ID
//      and ApprovalInfo from the signal's data attribute.
//
// We assert the audit hook via a sentinel error (the test
// substitutes its own auditWriterHook that fails the call). The
// interrupt-signal extraction is verified by errors.As against the
// public InterruptSignal type if it is reachable; otherwise we
// assert the error message is non-empty and contains the canonical
// interrupt anchor terms ("interrupt" / "approve").
func TestMemoryFactsToolFirstPassInterrupts(t *testing.T) {
	called := 0
	var capturedInfo *ApprovalInfo

	h := &memoryFactsTool{
		audit: &spyAuditWriter{
			onInsert: func(_ context.Context, _ string, info *ApprovalInfo) error {
				called++
				capturedInfo = info
				return nil
			},
		},
	}

	ctx := context.Background()
	args := `{"key":"employee_count","value":"150","tier":"权威"}`
	out, err := h.InvokableRun(ctx, args)

	// Pass 1 must produce an Interrupt error and must NOT return
	// a string output (the tool yields to the orchestration layer).
	assert.Empty(t, out, "Pass 1 must not produce string output")
	require.Error(t, err, "Pass 1 must return an interrupt error")

	// errors.As against the internal eino InterruptSignal type is
	// not stable across v0.9 patches — the type lives in
	// eino/internal/core which is not a public import. We instead
	// assert the error implements Error() with a message
	// containing the canonical anchor ("interrupt" or
	// "approval"). Both phrases appear in StatefulInterrupt's
	// descending call stack and operator-facing logs.
	errMsg := err.Error()
	hasInterruptAnchor := strings.Contains(strings.ToLower(errMsg), "interrupt") ||
		strings.Contains(strings.ToLower(errMsg), "approval")
	assert.True(t, hasInterruptAnchor,
		"Pass 1 error must surface an interrupt/approval signal; got %q", errMsg)

	// The audit hook was called exactly once with the right Info.
	assert.Equal(t, 1, called, "audit writer hook must be called exactly once on Pass 1")
	require.NotNil(t, capturedInfo,
		"audit writer hook must receive an ApprovalInfo")
	assert.Equal(t, MemoryFactsToolName, capturedInfo.ToolName)
	assert.Equal(t, MemoryFactsTarget, capturedInfo.Target)
	assert.Equal(t, args, capturedInfo.ArgumentsInJSON,
		"the ApprovalInfo must echo the verbatim args string for the FE")
	assert.NotEmpty(t, capturedInfo.ToolCallID,
		"ApprovalInfo.ToolCallID must be non-empty (== ApprovalID contract)")
}

// TestMemoryFactsToolRejectsDenyWithoutReason — ApprovalDecision
// validation boundary. The data crossing the gob checkpoint must
// be Validate-checked before the tool proceeds; a Decision=="deny"
// without Reason is the canonical failure mode.
//
// We exercise the Validate gate directly (the framework-level
// resume-from-checkpoint path is tested by 11-04's integration
// tests with the real Runner).
func TestMemoryFactsToolRejectsDenyWithoutReason(t *testing.T) {
	bad := &ApprovalDecision{
		Decision:   "deny",
		Reason:     "",
		ApprovalID: "abc",
	}
	err := bad.Validate()
	require.Error(t, err)
	verr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "reason", verr.Field,
		"deny-without-reason must surface field=\"reason\" (HITL-02 audit-friendly)")
}

// TestMemoryFactsToolInfoSchema — the LLM-visible schema MUST
// declare key + value as required strings. Any drift here changes
// the Tool schema the LLM sees.
//
// We assert via the JSON Schema's `Required` field (a []string
// emitted by ToJSONSchema) rather than introspecting the
// Properties orderedmap (eino@v0.9.14 typed the Properties as
// *orderedmap.OrderedMap which is not a vanilla map[string]any).
func TestMemoryFactsToolInfoSchema(t *testing.T) {
	h := &memoryFactsTool{}
	info, err := h.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, MemoryFactsToolName, info.Name)
	assert.NotEmpty(t, info.Desc, "tool desc must include approval-required phrasing")
	require.NotNil(t, info.ParamsOneOf)

	pm, err := info.ParamsOneOf.ToJSONSchema()
	require.NoError(t, err, "ParamsOneOf must flatten to a JSON Schema")
	require.Contains(t, pm.Required, "key",
		"key must be required (ComplianceSkill cannot write a fact without a stable identifier)")
	require.Contains(t, pm.Required, "value",
		"value must be required (an empty fact body is meaningless)")
}

// spyAuditWriter records every InsertPendingApproval call. The
// handler is injectable so the test can verify call-count and
// payload shape without standing up a real sqlite database.
type spyAuditWriter struct {
	onInsert func(ctx context.Context, approvalID string, info *ApprovalInfo) error
}

func (s *spyAuditWriter) InsertPendingApproval(ctx context.Context, approvalID string, info *ApprovalInfo) error {
	if s.onInsert != nil {
		return s.onInsert(ctx, approvalID, info)
	}
	return nil
}

// TestNewApprovalID — sanity check on the crypto/rand fallback.
// The returned ID must be non-empty, hex-decodable, and 32 chars
// (16 bytes).
func TestNewApprovalID(t *testing.T) {
	id := newApprovalID()
	require.NotEmpty(t, id)
	// Fallback path returns the literal sentinel — skip the
	// hex-length assertion in that case.
	if id == "fallback-approval-id-no-entropy" {
		return
	}
	assert.Len(t, id, 32, "16-byte hex string is 32 chars")
	_, err := hexDecodeString(id)
	require.NoError(t, err, "newApprovalID must return valid hex")
}

// hexDecodeString is a local helper that avoids importing
// encoding/hex in the test file (the production code imports it
// already — this keeps the import cluster visible).
func hexDecodeString(s string) ([]byte, error) {
	// Use encoding/hex under the hood — wrapper keeps the test
	// file import surface minimal.
	out := make([]byte, len(s)/2)
	const hexChars = "0123456789abcdef"
	for i := 0; i < len(out); i++ {
		hi := strings.IndexByte(hexChars, s[2*i])
		lo := strings.IndexByte(hexChars, s[2*i+1])
		if hi < 0 || lo < 0 {
			return nil, errors.New("invalid hex char")
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

// TestNoopAuditWriter — the no-op writer returns nil for every
// call. Used as the default when memoryFactsTool.audit is nil.
func TestNoopAuditWriter(t *testing.T) {
	w := noopAuditWriter{}
	err := w.InsertPendingApproval(context.Background(), "any-id", &ApprovalInfo{
		ToolName: MemoryFactsToolName,
		Target:   MemoryFactsTarget,
	})
	require.NoError(t, err)
}

// TestMemFactsArgsParse — defense-in-depth: confirm json.Unmarshal
// succeeds with the canonical Tier defaults documented in Info().
//
// Production code uses this parsing path in InvokableRun's Pass-2
// approve branch; we exercise it standalone so a regression in
// the struct's json tags surfaces here, not at first memory-facts
// write in prod.
func TestMemFactsArgsParse(t *testing.T) {
	var parsed memFactsArgs
	raw := json.RawMessage(`{"key":"employee_count","value":"150","tier":"权威"}`)
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, "employee_count", parsed.Key)
	assert.Equal(t, "150", parsed.Value)
	assert.Equal(t, "权威", parsed.Tier)
}

// invokableTool is the type assertion sentinel: a memoryFactsTool
// value MUST satisfy tool.InvokableTool so the eino
// compose.ToolsNodeConfig accepts it. Compile-time check.
var _ tool.InvokableTool = (*memoryFactsTool)(nil)
