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
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/llmconfig"
)

// CODE-AUDIT 2026-11-05 (A-03): the upstream chat-model assembly previously
// lived in TWO divergent copies — cmd/server/main.go's buildUpstream closure
// (file config → plan fallback → LocalThinkOffModel, with an env-preset
// fallback when no file config exists) and internal/server/llm_admin.go's
// applyLLMConfig resolver (file config only, NO env fallback, NO
// LocalThinkOffModel). The first panel save hot-swapped the startup
// resolver for the impoverished one, silently dropping LOCAL_LLM_THINK=false
// and the env-preset fallback. This factory is now the SINGLE assembly both
// callers share; divergence here is by construction impossible.

// envUpstreamBaseURL mirrors cmd/server's envOr chain for the env-preset
// path. The default constant lives in llmconfig (CODE-AUDIT 2026-11-05
// A-06) so the upstream factory and EffectiveBaseURL's capability
// judgement can never drift apart.
func envUpstreamBaseURL() string {
	if v := os.Getenv("LOCAL_BASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("GLM_BASE_URL"); v != "" {
		return v
	}
	return llmconfig.DefaultGLMBaseURL
}

// envOr returns the first non-empty value (main.go's helper, mirrored).
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// UpstreamEnv carries the operator knobs BuildUpstream needs from the
// process environment. StubActive must be computed by the caller (it also
// drives main.go's startup logging); the URL/key/model triple is re-read
// from env here so both call sites see identical values.
type UpstreamEnv struct {
	// StubActive — no LOCAL_API_KEY / GLM_API_KEY configured at boot. When
	// true and no file config exists, BuildUpstream returns ErrNoUpstream
	// (dynamic slot empty ⇒ stub chat mode) instead of building a keyless
	// env-preset model.
	StubActive bool
}

// BuildUpstream resolves the chat-model upstream with ONE precedence rule:
//
//  1. Panel/file config (~/.skillforge/llm.json) when present — base_url
//     falls back to the catalog plan value when blank (A-04 parity with
//     llmModelHandler), and the result is wrapped by LocalThinkOffModel.
//  2. Env preset (LOCAL_* overrides GLM_*) when no file config — wrapped by
//     LocalThinkOffModel as well.
//  3. ErrNoUpstream when neither is usable (stub mode).
//
// A resolution failure returns an error (Available()=false ⇒ pipeline falls
// back to stub) instead of panicking — runtime reconfiguration must never
// take the process down.
func BuildUpstream(env UpstreamEnv) (model.ToolCallingChatModel, error) {
	cfg, ok, cfgErr := llmconfig.Load()
	if cfgErr != nil || !ok {
		// No usable file config → env preset fallback (was missing from the
		// panel resolver copy — A-03).
		envKey := envOr("LOCAL_API_KEY", os.Getenv("GLM_API_KEY"))
		if env.StubActive || envKey == "" {
			return nil, ErrNoUpstream
		}
		envBase := envUpstreamBaseURL()
		envModel := envOr("LOCAL_MODEL", envOr("GLM_MODEL", "glm-5.3-flash"))
		cmRaw, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
			BaseURL: envBase,
			APIKey:  envKey,
			Model:   envModel,
		})
		if err != nil {
			return nil, fmt.Errorf("env preset: %w", err)
		}
		return LocalThinkOffModel(cmRaw), nil
	}
	key, keyErr := llmconfig.ResolveKey(cfg)
	if keyErr != nil {
		return nil, fmt.Errorf("resolve key: %w", keyErr)
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		// A-04 parity: blank base_url is legal for non-custom vendors —
		// fall back to the catalog plan exactly like llmModelHandler.
		plan, found := llmconfig.FindPlan(cfg.Vendor, cfg.Plan)
		if !found {
			return nil, fmt.Errorf("panel config: unknown vendor/plan %s/%s", cfg.Vendor, cfg.Plan)
		}
		baseURL = plan.BaseURL
	}
	cmRaw, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  key,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("panel config: %w", err)
	}
	// The panel resolver copy dropped this wrapper — LOCAL_LLM_THINK=false
	// silently regressed to minutes-long local thinking after any panel
	// save (A-03).
	return LocalThinkOffModel(cmRaw), nil
}
