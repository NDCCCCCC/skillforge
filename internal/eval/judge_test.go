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

package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChatModel returns a canned content string for every Generate call,
// enabling hermetic tests of glmJudge's structured-output parsing path
// (generateJSON) without a live provider. Stream and WithTools are unused
// by the judge but implemented to satisfy model.ToolCallingChatModel.
type fakeChatModel struct {
	content string
}

func (f *fakeChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: f.content}, nil
}

func (f *fakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("fakeChatModel: Stream not implemented")
}

func (f *fakeChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}

// TestGLMJudgeParsesProseWrappedResponse is the OOS-1 follow-up regression
// lock for the eval package. generateJSON must parse a JSON object that the
// judge model wrapped in prose + fences ("Here is my judgment:\n```json
// {\"attributable\": true}\n```\n..."). The private stripJSONDelim this path
// used to carry could NOT detect a fence that was not anchored at the
// absolute start of the trimmed string — prose-wrapped output defeated it,
// silently degrading eval metrics. The path now delegates to the shared,
// robust llm.ExtractJSON (first-'{'-to-last-'}' brace scan); this test
// proves the consolidation holds end-to-end through a real Judge method
// (IsAttributable — the simplest single-field schema).
func TestGLMJudgeParsesProseWrappedResponse(t *testing.T) {
	ctx := context.Background()
	fake := &fakeChatModel{
		content: "Here is my judgment:\n```json\n" + `{"attributable": true}` + "\n```\nLet me know if you need more detail.",
	}
	j := NewJudge(fake)

	got, err := j.IsAttributable(ctx, "statement", "context")
	require.NoError(t, err)
	assert.True(t, got)
}

// TestGLMJudgeErrorsOnEmptyResponse pins the ErrEmptyJudgeResponse contract
// that generateJSON enforces BEFORE attempting to parse, so an empty model
// reply is reported as the distinct sentinel rather than a confusing JSON
// parse error from llm.ExtractJSON("").
func TestGLMJudgeErrorsOnEmptyResponse(t *testing.T) {
	ctx := context.Background()
	fake := &fakeChatModel{content: "   "}
	j := NewJudge(fake)

	_, err := j.IsAttributable(ctx, "statement", "context")
	require.ErrorIs(t, err, ErrEmptyJudgeResponse)
}
