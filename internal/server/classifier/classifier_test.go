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
// WITHOUT WARRANTIES OR CONDITIONS OF CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package classifier

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/verdict"
)

// stubClassifierChatModel is the test double for the LLM seam used by
// RefusalClassifier. It satisfies model.ToolCallingChatModel. Each
// Generate call returns canned verbatim; a counter records the number
// of attempts so the retry tests can assert on the retry budget.
type stubClassifierChatModel struct {
	mu     sync.Mutex
	count  int
	canned string
	err    error
}

var _ model.ToolCallingChatModel = (*stubClassifierChatModel)(nil)

func (m *stubClassifierChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	if m.err != nil {
		return nil, m.err
	}
	if m.canned == "" {
		return &schema.Message{Role: schema.Assistant, Content: "{}"}, nil
	}
	return &schema.Message{Role: schema.Assistant, Content: m.canned}, nil
}

func (m *stubClassifierChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stubClassifierChatModel: Stream not implemented")
}

func (m *stubClassifierChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *stubClassifierChatModel) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// TestClassify_AllowOnCleanQuery — the happy path. The stub returns
// a valid allow envelope with confidence above the floor; the output
// matches and Valid() is true.
func TestClassify_AllowOnCleanQuery(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "allow", "confidence": 0.95}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "年假有几天",
		Chunks: []string{"[ref-1] tier=权威 年假5天"},
	})
	require.NoError(t, err)
	assert.Equal(t, "allow", out.Decision)
	assert.InDelta(t, 0.95, out.Confidence, 0.001)
	assert.True(t, out.Valid(), "Valid() must report allow@0.95 as valid")
	assert.Equal(t, 1, stub.CallCount(), "single-valid-output path: LLM called once")
}

// TestClassify_AllowJustAboveFloor — boundary test. Confidence
// exactly at the floor (0.5) is allowed. Boundary inclusive.
func TestClassify_AllowJustAboveFloor(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "allow", "confidence": 0.5}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "差旅制度第二条",
		Chunks: []string{"[ref-1] tier=权威 第二条 内容"},
	})
	require.NoError(t, err)
	assert.Equal(t, "allow", out.Decision)
	assert.True(t, out.Valid(), "confidence=0.5 is the floor; Valid() returns true (boundary inclusive)")
}

// TestClassify_RefuseOnPIIQuery — the refuse path. The stub returns
// a refuse envelope with a recognized verdict reason.
func TestClassify_RefuseOnPIIQuery(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "refuse", "reason": "out_of_scope", "confidence": 0.92}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "查询员工联系方式",
		Chunks: nil,
	})
	require.NoError(t, err)
	assert.Equal(t, "refuse", out.Decision)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason)
	assert.InDelta(t, 0.92, out.Confidence, 0.001)
}

// TestClassify_DefaultDenyOnParseFailure — RESOLQ 2 mitigation. The
// stub returns invalid JSON on every attempt; after MaxRetries
// retries the wrapper surfaces the DefaultDeny shape with a non-nil
// error so the pipeline treats this as "refuse by default".
func TestClassify_DefaultDenyOnParseFailure(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: "this is not json",
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "x",
		Chunks: []string{"y"},
	})
	require.Error(t, err, "retry-exhaustion must surface as non-nil error")
	assert.Equal(t, "refuse", out.Decision,
		"DefaultDeny: Decision=refuse")
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason,
		"DefaultDeny: Reason=out_of_scope")
	assert.InDelta(t, 0.0, out.Confidence, 0.001,
		"DefaultDeny: Confidence=0.0")
	assert.Equal(t, DefaultMaxRetries, stub.CallCount(),
		"retry exhaustion: LLM called DefaultMaxRetries times")
}

// TestClassify_DefaultDenyOnInvalidDecision — the LLM returns a
// decision value not in the enum ("maybe"); every attempt is invalid
// so the wrapper surfaces DefaultDeny after retry exhaustion.
func TestClassify_DefaultDenyOnInvalidDecision(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "maybe", "confidence": 0.9}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "x",
		Chunks: []string{"y"},
	})
	require.Error(t, err)
	assert.Equal(t, "refuse", out.Decision)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason)
	assert.InDelta(t, 0.0, out.Confidence, 0.001)
}

// TestClassify_DefaultDenyOnLowConfidence — W3 / T-10-04. The stub
// returns "allow" with confidence below the floor; the floor is
// enforced (via Valid() in types.go + an explicit floor check in
// the retry wrapper). After retry exhaustion the wrapper surfaces
// DefaultDeny — a hesitant allow is worse than a refusal.
func TestClassify_DefaultDenyOnLowConfidence(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "allow", "confidence": 0.3}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "x",
		Chunks: []string{"y"},
	})
	require.Error(t, err, "below-floor allow must surface as error so caller logs it")
	assert.Equal(t, "refuse", out.Decision,
		"below-floor allow → DefaultDeny: Decision=refuse")
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason,
		"below-floor allow → DefaultDeny: Reason=out_of_scope")
	assert.InDelta(t, 0.0, out.Confidence, 0.001)
	assert.Equal(t, DefaultMaxRetries, stub.CallCount())
}

// TestClassify_DefaultDenyOnConfidenceOutOfRange — the LLM returns
// a confidence value outside [0.0, 1.0] (here: 1.5). The Valid()
// gate rejects it; retry → DefaultDeny.
func TestClassify_DefaultDenyOnConfidenceOutOfRange(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "allow", "confidence": 1.5}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "x",
		Chunks: []string{"y"},
	})
	require.Error(t, err)
	assert.Equal(t, "refuse", out.Decision)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason)
	assert.InDelta(t, 0.0, out.Confidence, 0.001)
	assert.Equal(t, DefaultMaxRetries, stub.CallCount())
}

// TestClassify_NilClassifier — defensive nil-receiver guard. The
// caller (pipeline) gates on RefusalClassifier != nil, but if a
// future refactor bypasses that, the function returns DefaultDeny
// rather than panicking.
func TestClassify_NilClassifier(t *testing.T) {
	var cl *RefusalClassifier
	out, err := cl.Classify(context.Background(), RefusalClassifierInput{Query: "x"})
	require.Error(t, err)
	assert.Equal(t, "refuse", out.Decision,
		"nil classifier → DefaultDeny (safer than panic)")
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason)
}

// TestClassify_NilChatModelInConstructor — defensive constructor
// guard.
func TestClassify_NilChatModelInConstructor(t *testing.T) {
	_, err := NewRefusalClassifier(nil)
	require.Error(t, err)
}

// TestClassify_RefuseWithUnknownReasonRetries — a refuse envelope
// with a reason outside the verdict.RefusalReason* enum is treated
// as invalid (retry). The wrapper surfaces DefaultDeny after
// exhaustion so a misconfigured / drifting LLM cannot plant
// unrecognized reasons in the audit row.
func TestClassify_RefuseWithUnknownReasonRetries(t *testing.T) {
	stub := &stubClassifierChatModel{
		canned: `{"decision": "refuse", "reason": "policy_violation", "confidence": 0.9}`,
	}
	c, err := NewRefusalClassifier(stub)
	require.NoError(t, err)
	out, err := c.Classify(context.Background(), RefusalClassifierInput{
		Query:  "x",
		Chunks: []string{"y"},
	})
	require.Error(t, err)
	assert.Equal(t, "refuse", out.Decision)
	assert.Equal(t, verdict.RefusalReasonOutOfScope, out.Reason,
		"unknown refuse reason → DefaultDeny (Reason=out_of_scope)")
	assert.Equal(t, DefaultMaxRetries, stub.CallCount())
}