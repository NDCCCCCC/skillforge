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

package llm

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

// CODE-AUDIT 2026-11-05 (A-01) regression locks: ExtraFieldsModel must apply
// its fields at CALLER-PRIORITY, and PipelineExtraFields must build
// self-sufficient call-site sets under LOCAL_LLM_THINK=false.

// capturingModel records the option list it received.
type capturingModel struct {
	gotOpts []model.Option
}

func (c *capturingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	c.gotOpts = opts
	return &schema.Message{}, nil
}

func (c *capturingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	c.gotOpts = opts
	return nil, nil
}

func (c *capturingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return c, nil
}

// TestExtraFieldsModel_CallerOptionsLast asserts the wrapper's
// WithExtraFields option is prepended BEFORE the caller's options — eino
// applies impl-specific options in slice order and eino-ext's
// WithExtraFields ASSIGNS the whole map (no merge), so the LAST one wins.
// The previous (buggy) order appended the wrapper last, letting
// "think":false stomp response_format/thinking under LOCAL_LLM_THINK=false.
func TestExtraFieldsModel_CallerOptionsLast(t *testing.T) {
	cap := &capturingModel{}
	wrapped := WithExtraFields(cap, map[string]any{"think": false})
	cm, ok := wrapped.(*ExtraFieldsModel)
	if !ok {
		t.Fatalf("WithExtraFields must return *ExtraFieldsModel, got %T", wrapped)
	}
	callerOpt := openai.WithExtraFields(map[string]any{
		"response_format": map[string]string{"type": "json_object"},
	})
	if _, err := cm.Generate(context.Background(), []*schema.Message{}, callerOpt); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(cap.gotOpts) != 2 {
		t.Fatalf("expected 2 options (wrapper first, caller last), got %d", len(cap.gotOpts))
	}
	// generateOptions must allocate a fresh slice — appending onto the
	// caller's variadic slice would risk clobbering its backing array.
	callerOpts := make([]model.Option, 1, 8)
	callerOpts[0] = callerOpt
	_ = cm.generateOptions(callerOpts)
	if cap := capOf(callerOpts); cap != 8 {
		t.Fatalf("generateOptions must not mutate the caller's slice capacity bookkeeping: %d", cap)
	}
}

func capOf(s []model.Option) int { return cap(s) }

// TestPipelineExtraFields_LocalThinkFlag locks the A-01 companion fix: with
// LOCAL_LLM_THINK=false the call-site sets carry "think" themselves, because
// a caller-provided WithExtraFields replaces the wrapper's map wholesale.
func TestPipelineExtraFields_LocalThinkFlag(t *testing.T) {
	t.Run("think:false present when LOCAL_LLM_THINK=false", func(t *testing.T) {
		t.Setenv(LocalThinkDisabledEnv, "false")
		fields := PipelineExtraFields()
		v, ok := fields["think"].(bool)
		if !ok || v {
			t.Fatalf("LOCAL_LLM_THINK=false must add think:false, got %v", fields["think"])
		}
		if _, ok := fields["response_format"]; !ok {
			t.Fatalf("response_format must always be present")
		}
	})
	t.Run("no think field otherwise", func(t *testing.T) {
		t.Setenv(LocalThinkDisabledEnv, "true")
		fields := PipelineExtraFields()
		if _, has := fields["think"]; has {
			t.Fatalf("LOCAL_LLM_THINK!=false must not add think:false, got %v", fields["think"])
		}
	})
}
