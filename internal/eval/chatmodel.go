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

// NewGLMChatModel constructs a ChatModel pointing at the GLM Coding Plan
// endpoint, for use as the Judge's underlying LLM. It reads:
//
//   - GLM_BASE_URL (optional, default DefaultGLMBaseURL) — Coding Plan
//     endpoint base URL.
//   - GLM_API_KEY (REQUIRED) — Coding Plan subscription key.
//   - JUDGE_MODEL (optional, default "glm-5.2") — the D3-07 slot for
//     cross-evaluation with another model (DeepSeek, etc.) without
//     touching the production agent.
//
// Returns model.ToolCallingChatModel so the judge can be upgraded to a
// tool-calling variant if a future Ragas-style structured-output
// optimization requires it. Today the judge uses plain JSON output
// (response_format: json_object) which the openai.NewChatModel emits
// natively.
//
// NOTE: this construction is byte-identical to cmd/server/main.go:152
// and cmd/spiked/e2e.go:179. The 4 existing sites COULD be refactored
// to share this helper, but that is out of scope for Plan 03-06 — the
// eval package deliberately stands alone so it can be lifted into a
// separate binary (cmd/eval) without dragging cmd/server's wiring.
func NewGLMChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	if err := validateEnv(); err != nil {
		return nil, err
	}
	baseURL := os.Getenv("GLM_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultGLMBaseURL
	}
	modelName := os.Getenv("JUDGE_MODEL")
	if modelName == "" {
		modelName = DefaultJudgeModel
	}
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  os.Getenv("GLM_API_KEY"),
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
	var tcm model.ToolCallingChatModel = cm
	return tcm, nil
}

// validateEnv raises a clear, actionable error if GLM_API_KEY is unset
// (T-3-ENV mitigation). The key is read via os.Getenv and NEVER
// serialized to report/calibration files (the report writer has no
// access to env vars by design). .env is in .gitignore (verified during
// Plan 03-02 — the same env-var handling pattern).
func validateEnv() error {
	if os.Getenv("GLM_API_KEY") == "" {
		return errors.New("eval: GLM_API_KEY env var not set (the judge needs a GLM Coding Plan key — copy .env.example to .env and populate, or set JUDGE_MODEL to point at a different OpenAI-compatible endpoint and adjust GLM_BASE_URL)")
	}
	return nil
}
