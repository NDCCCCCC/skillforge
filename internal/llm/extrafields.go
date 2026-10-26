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
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/llmconfig"
)

// ExtraFieldsModel wraps a ToolCallingChatModel and injects extra
// top-level request-body fields into EVERY Generate/Stream call.
//
// Why a decorator instead of per-call-site options: the injection must
// reach call sites that predate it — the ReAct agent loop (flowagent has
// no chat-model-option passthrough in eino v0.9.14) and the eval judge —
// and missing ONE site would silently leave that layer on the slow
// thinking path.
type ExtraFieldsModel struct {
	inner  model.ToolCallingChatModel
	fields map[string]any
}

// WithExtraFields wraps cm so every request carries fields. Returns cm
// unchanged when fields is empty.
func WithExtraFields(cm model.ToolCallingChatModel, fields map[string]any) model.ToolCallingChatModel {
	if cm == nil || len(fields) == 0 {
		return cm
	}
	return &ExtraFieldsModel{inner: cm, fields: fields}
}

// LocalThinkDisabledEnv is the operator switch: set LOCAL_LLM_THINK=false
// to disable chain-of-thought on the LOCAL_* endpoint (ollama dialect:
// "think": false). Rationale (2026-09-06): qwen3.8 spends minutes
// reasoning before the first content token — 5-7min per verdict, 10min+
// per eval case — while think:false drops it to seconds. Only the
// LOCAL_* path is affected; GLM keeps its own thinking:{type:disabled}
// per-call field where the server layers already send it.
const LocalThinkDisabledEnv = "LOCAL_LLM_THINK"

// LocalThinkOffModel wraps cm with the ollama think:false field when the
// operator set LOCAL_LLM_THINK=false; otherwise returns cm unchanged.
func LocalThinkOffModel(cm model.ToolCallingChatModel) model.ToolCallingChatModel {
	if os.Getenv(LocalThinkDisabledEnv) != "false" {
		return cm
	}
	return WithExtraFields(cm, map[string]any{"think": false})
}

// PipelineExtraFields returns the extra top-level request fields shared by
// the VerdictPipeline / rewriter / refusal-classifier call sites
// (response_format:json_object + thinking:disabled, per spike 003).
//
// MiniMax M3.x exception (observed 2026-10-26): the M3.1 family REQUIRES
// adaptive thinking — a request carrying thinking.type="disabled" is
// rejected with HTTP 400 code 2013 ("requires adaptive thinking"). For
// vendor "minimax" the thinking field is omitted entirely (adaptive stays
// on by default; the 4096-token budget already headrooms for reasoning).
// Any other vendor — including unknown/none (fail-open, file config
// missing) — keeps the historical thinking:disabled field.
func PipelineExtraFields() map[string]any {
	fields := map[string]any{
		"response_format": map[string]string{"type": "json_object"},
	}
	if llmconfig.CurrentVendor() != "minimax" {
		fields["thinking"] = map[string]string{"type": "disabled"}
	}
	return fields
}

// Generate implements model.BaseChatModel with the extra fields merged in.
func (m *ExtraFieldsModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, input, append(opts, openai.WithExtraFields(m.fields))...)
}

// Stream implements model.BaseChatModel with the extra fields merged in.
func (m *ExtraFieldsModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.inner.Stream(ctx, input, append(opts, openai.WithExtraFields(m.fields))...)
}

// WithTools implements model.ToolCallingChatModel, returning a wrapped
// tool-bound model so the decorator survives tool binding (the ReAct
// agent calls WithTools once per run).
func (m *ExtraFieldsModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &ExtraFieldsModel{inner: inner, fields: m.fields}, nil
}
