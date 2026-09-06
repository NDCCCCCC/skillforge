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

// Package eval is the Go-native trust evaluator (Plan 03-06, D6 reversed).
// It computes the 4 RAG-trust metrics (faithfulness, answer_relevancy,
// context_precision, context_recall) over the SkillForge agent without
// leaving the Go process — no Python sidecar, no Ragas pip install.
//
// Architecture (single-language, Apache-2.0 clean):
//
//   - chatmodel.go (this file) — NewGLMChatModel: the GLM Coding Plan
//     ChatModel construction shared by the judge. Mirrors cmd/server's
//     inline construction at main.go:152, with the JUDGE_MODEL override
//     (D3-07).
//   - judge.go — Judge interface + glmJudge impl + StubJudge for hermetic
//     tests. Each judge method is one structured-output LLM call at
//     temperature=0.
//   - faithfulness.go — Faithfulness metric. Ports the Ragas v0.1.21
//     claim-extraction + entailment prompts VERBATIM (the "Ragas 口径"
//     headline number). See eval/NOTICE for Apache-2.0 attribution.
//   - metrics.go — answer_relevancy (BGE-M3 cosine), context_precision,
//     context_recall. Go-authored prompts.
//   - dataset.go — Case / Sample / Report types + LoadGolden / WriteReport.
//   - calibration.go — Calibrate: recommends retrievalScoreThreshold from
//     the score distribution (D2-15 retirement).
//
// The package is consumed by cmd/eval, a Go CLI that drives the agent over
// eval/golden/compliance.jsonl and writes report.json + calibration.json.
package eval

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/llm"
)

// DefaultGLMBaseURL is the GLM Coding Plan endpoint (Pitfall 6: NOT
// /api/paas/v4 — the standard endpoint rejects Coding Plan keys with a
// confusing "余额不足" error). Mirrors cmd/server's defaultGLMBaseURL.
const DefaultGLMBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"

// DefaultJudgeModel is the default LLM used as the trust evaluator's
// judge (D3-07). GLM-5.2 matches the production agent's model — this is
// the "self-eval bias" path documented in eval/README.md (Pitfall 8
// counter-measure: deterministic Go grounding floor + human spot-check +
// the JUDGE_MODEL env-var slot for Phase 6 cross-evaluation with
// DeepSeek/another model).
const DefaultJudgeModel = "glm-5.2"

// LLMConfig is the resolved chat-endpoint triple (base URL / key / model).
type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// firstNonEmptyEnv returns the first non-empty env var among keys.
func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// ResolveLLM resolves the chat endpoint with this precedence:
//
//  1. LOCAL_* — a self-hosted OpenAI-compatible endpoint
//     (LOCAL_BASE_URL + LOCAL_API_KEY + LOCAL_MODEL). Rationale
//     (2026-09-06 operator decision): the GLM Coding Plan throttles at
//     eval-scale request rates (429), so the eval CLI and server can be
//     pointed at a local inference server instead.
//  2. GLM_* — the Zhipu Coding Plan production endpoint.
//  3. Built-in defaults (DefaultGLMBaseURL + DefaultJudgeModel).
//
// Shared by NewGLMChatModel (judge) and cmd/server's inline construction —
// both sides must resolve the SAME endpoint so judge and agent agree on
// the model under test.
func ResolveLLM() LLMConfig {
	baseURL := firstNonEmptyEnv("LOCAL_BASE_URL", "GLM_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultGLMBaseURL
	}
	modelName := firstNonEmptyEnv("LOCAL_MODEL", "JUDGE_MODEL")
	if modelName == "" {
		modelName = DefaultJudgeModel
	}
	return LLMConfig{
		BaseURL: baseURL,
		APIKey:  firstNonEmptyEnv("LOCAL_API_KEY", "GLM_API_KEY"),
		Model:   modelName,
	}
}

// NewGLMChatModel constructs a ChatModel pointing at the resolved
// endpoint (see ResolveLLM for the LOCAL_* > GLM_* precedence), for use
// as the Judge's underlying LLM. Historically GLM Coding Plan-only; the
// LOCAL_* slots (2026-09-06) let the same constructor drive a
// self-hosted OpenAI-compatible server. Returns
// model.ToolCallingChatModel so the judge can be upgraded to a
// tool-calling variant if a future Ragas-style structured-output
// optimization requires it. Today the judge uses plain JSON output
// (response_format: json_object) which the openai.NewChatModel emits
// natively.
func NewGLMChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	cfg := ResolveLLM()
	if cfg.APIKey == "" {
		return nil, errors.New("eval: no chat API key resolved (set LOCAL_API_KEY for a self-hosted endpoint, or GLM_API_KEY for the Coding Plan — copy .env.example to .env and populate)")
	}
	modelName := cfg.Model
	cmRaw, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   modelName,
	})
	if err != nil {
		return nil, fmt.Errorf("eval: NewGLMChatModel(%s): %w", modelName, err)
	}
	// openai.NewChatModel returns a concrete *openai.ChatModel. The same
	// value is consumed by cmd/server's agent.NewAgent which requires
	// the model.ToolCallingChatModel interface (agent.go:111). The
	// assignment below is the compile-time contract that the eval judge
	// and the production agent share the same underlying model
	// capability — if a future eino-ext version regresses
	// *openai.ChatModel's WithTools/Generate methods, the build breaks
	// here AND at cmd/server (intentional: surface the regression at
	// the smallest possible scope).
	var cm model.ToolCallingChatModel = cmRaw
	// LOCAL_LLM_THINK=false decorates every call with the ollama
	// think:false field — the eval ReAct agent has no chat-model-option
	// passthrough (flowagent, eino v0.9.14), so construction-time
	// wrapping is the only point that reaches agent AND judge.
	cm = llm.LocalThinkOffModel(cm)
	return cm, nil
}
