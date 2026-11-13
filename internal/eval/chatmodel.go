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
//   - chatmodel.go (this file) — NewGLMChatModel + ResolveLLM: the
//     judge's ChatModel construction, sharing llmconfig.Effective()
//     with the server's BuildUpstream (panel config > env preset), so
//     what the admin panel saves is exactly what eval runs
//     (261113-obm; formerly an env-only mirror of cmd/server's inline
//     construction at main.go:152, with the JUDGE_MODEL override
//     D3-07).
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

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/llm"
	"github.com/skillforge/skillforge/internal/llmconfig"
)

// DefaultGLMBaseURL is the GLM Coding Plan endpoint (Pitfall 6: NOT
// /api/paas/v4 — the standard endpoint rejects Coding Plan keys with a
// confusing "余额不足" error). 261113-obm: aliased to
// llmconfig.DefaultGLMBaseURL instead of repeating the literal — one
// endpoint string, shared with the server's env-preset path.
const DefaultGLMBaseURL = llmconfig.DefaultGLMBaseURL

// DefaultJudgeModel is the FINAL fallback for the trust evaluator's
// judge model (D3-07): used only when there is NEITHER a panel config
// (~/.skillforge/llm.json) NOR any env model slot. The panel config
// wins over it — llmconfig.Effective() (shared with the server's
// BuildUpstream) resolves the actual model, and this constant is only
// the env-preset floor's floor. It still matches the production agent's
// default model — the "self-eval bias" path documented in eval/README.md
// (Pitfall 8 counter-measure: deterministic Go grounding floor + human
// spot-check + the JUDGE_MODEL env-var slot for Phase 6 cross-evaluation
// with DeepSeek/another model).
//
// 261113-obm: aliased to llmconfig.DefaultGLMModel to kill the second
// literal (the two must never drift — one names the server's default
// model, the other the judge's). History: was "glm-5.2"; the
// deployment's account group does not serve that id — every eval case
// came back 404 "Model glm-5.2 is not supported by any configured
// account in this group" and scored as agent_error, which the report
// only summarised as a count (75/75) without the cause. The model id is
// ALSO an input to the frozen prompt_hash, so the pin has to be
// re-authored whenever it changes. scripts/eval-models.ps1 lists what
// the endpoint actually serves before a run.
const DefaultJudgeModel = llmconfig.DefaultGLMModel

// LLMConfig is the resolved chat-endpoint triple (base URL / key / model).
type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// ResolveLLM resolves the chat endpoint the SAME way the server does —
// through llmconfig.Effective(), whose precedence is:
//
//  1. Panel config (~/.skillforge/llm.json, saved from the admin panel's
//     「LLM Provider 配置」) — what the user changes on the page is what
//     eval runs. A blank base_url falls back to the catalog plan.
//  2. Env preset — LOCAL_* overrides GLM_* / JUDGE_MODEL, then the
//     built-in defaults (DefaultGLMBaseURL + DefaultJudgeModel).
//     Rationale for the LOCAL_* slots (2026-09-06 operator decision):
//     the GLM Coding Plan throttles at eval-scale request rates (429),
//     so the eval CLI and server can be pointed at a local inference
//     server instead.
//
// 261113-obm: this used to be an env-only lookup (LOCAL_* > GLM_*),
// which meant the panel and the eval baseline ran two DIFFERENT
// resolutions — the user changed the model on the page and eval kept
// dialling whatever .env said. Both sides now share one chain, so the
// judge and the agent under test agree on the model by construction.
//
// A panel config whose key cannot be resolved (e.g. api_key_env names
// an unset variable) surfaces here as an empty APIKey — NewGLMChatModel
// then fails with the actionable no-key message. It must NOT silently
// degrade to env: dialling an endpoint the user didn't configure is
// harder to debug than failing.
func ResolveLLM() LLMConfig {
	baseURL, apiKey, modelName, err := llmconfig.Effective()
	if err != nil {
		// Effective only errors when a panel config exists but its key
		// is unresolvable. Blank the key so NewGLMChatModel reports the
		// actionable message instead of half-configured silence.
		apiKey = ""
	}
	return LLMConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   modelName,
	}
}

// NewGLMChatModel constructs a ChatModel pointing at the resolved
// endpoint (see ResolveLLM for the panel-config > env-preset precedence
// shared with the server's BuildUpstream), for use as the Judge's
// underlying LLM. Historically GLM Coding Plan-only; the LOCAL_* slots
// (2026-09-06) let the same constructor drive a self-hosted
// OpenAI-compatible server. Returns model.ToolCallingChatModel so the
// judge can be upgraded to a tool-calling variant if a future
// Ragas-style structured-output optimization requires it. Today the
// judge uses plain JSON output (response_format: json_object) which the
// openai.NewChatModel emits natively.
func NewGLMChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	cfg := ResolveLLM()
	if cfg.APIKey == "" {
		return nil, errors.New("eval: no chat API key resolved — either save a provider config once in the admin panel's「LLM Provider 配置」(writes ~/.skillforge/llm.json; eval shares the server's resolution chain, so the panel config wins), or set GLM_API_KEY / LOCAL_API_KEY in .env as the env-preset fallback")
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
