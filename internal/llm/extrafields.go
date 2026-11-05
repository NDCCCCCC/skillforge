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
// rejected with HTTP 400 code 2013 ("requires adaptive thinking").
//
// CODE-AUDIT 2026-11-05 (A-05/A-06/A-10): the policy is now an ENDPOINT
// CAPABILITY WHITELIST (llmconfig.SupportsThinkingDisabled), not a
// "!= minimax" blacklist on the declared vendor. That fixes three cases
// at once: a vendor whose base_url points at another provider, an env
// preset deployment (which has no vendor concept at all), and every
// OpenAI-compatible endpoint that never had a thinking parameter.
//
// LOCAL_LLM_THINK=false (CODE-AUDIT 2026-11-05 A-01): when the operator
// disables local chain-of-thought, "think":false is included HERE so the
// sets built by the pipeline/classifier/rewriter call sites are COMPLETE.
// Rationale: ExtraFieldsModel injects its own fields with caller-priority
// semantics (see Generate below) — a caller that passes its own
// WithExtraFields replaces the wrapper's map wholesale (eino-ext
// WithExtraFields is assignment, not merge), so any field the wrapper
// carries but the caller's set lacks would be dropped. Keeping the local
// think flag in this shared builder guarantees the call-site sets stay
// self-sufficient under both orderings.
func PipelineExtraFields() map[string]any {
	fields := map[string]any{
		"response_format": map[string]string{"type": "json_object"},
	}
	if llmconfig.SupportsThinkingDisabled() {
		fields["thinking"] = map[string]string{"type": "disabled"}
	}
	if os.Getenv(LocalThinkDisabledEnv) == "false" {
		fields["think"] = false
	}
	return fields
}

// extraFieldsOption returns the option carrying m.fields. It MUST be placed
// BEFORE any caller-provided options: eino applies impl-specific options in
// slice order and eino-ext's WithExtraFields ASSIGNS the whole map (no
// merge), so the LAST WithExtraFields wins. Positioning ours first gives
// caller-priority semantics — a call site that passes its own extra fields
// (the pipeline/classifier/rewriter sets, complete per PipelineExtraFields)
// keeps them; option-less call sites (ReAct agent loop, eval judge) get the
// wrapper defaults. (CODE-AUDIT 2026-11-05 A-01: the previous order — ours
// appended last — made the wrapper stomp response_format/thinking whenever
// LOCAL_LLM_THINK=false, fail-closing the classifier to out_of_scope.)
func (m *ExtraFieldsModel) extraFieldsOption() model.Option {
	return openai.WithExtraFields(m.fields)
}

// generateOptions builds a fresh option slice with the wrapper's extra
// fields FIRST so caller options keep priority. A new backing array is
// allocated on every call — appending onto the caller's slice would risk
// clobbering its backing array when len(opts) < cap(opts).
func (m *ExtraFieldsModel) generateOptions(opts []model.Option) []model.Option {
	out := make([]model.Option, 0, len(opts)+1)
	out = append(out, m.extraFieldsOption())
	return append(out, opts...)
}

// Generate implements model.BaseChatModel with the wrapper's extra fields
// injected at caller-priority (see extraFieldsOption).
func (m *ExtraFieldsModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, input, m.generateOptions(opts)...)
}

// Stream implements model.BaseChatModel with the wrapper's extra fields
// injected at caller-priority (see extraFieldsOption).
func (m *ExtraFieldsModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.inner.Stream(ctx, input, m.generateOptions(opts)...)
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
