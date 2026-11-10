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
	"log"
	"os"
	"sync"

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
		logResolution(resolution{
			Source:  "ENV_PRESET",
			Detail:  fmt.Sprintf("no panel config file at %s", llmconfig.ConfigPath()),
			BaseURL: envBase,
			Model:   envModel,
			KeyLen:  len(envKey),
			EnvVars: envPresence(),
		})
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
	logResolution(resolution{
		Source:  "PANEL_FILE",
		Detail:  fmt.Sprintf("%s (vendor=%s plan=%s)", llmconfig.ConfigPath(), cfg.Vendor, cfg.Plan),
		BaseURL: baseURL,
		Model:   cfg.Model,
		KeyLen:  len(key),
		EnvVars: envPresence(),
		// The env vars are STILL set but do not participate — say so out loud.
		OverridesEnv: anyEnvSet(),
	})
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

// resolution is the loggable shape of one BuildUpstream decision. It never
// carries a key VALUE — only len(key) — so the operator can tell "which of the
// two possible keys won" without a secret landing in the log (T-49-14).
type resolution struct {
	Source       string // ENV_PRESET | PANEL_FILE
	Detail       string
	BaseURL      string
	Model        string
	KeyLen       int
	EnvVars      string
	OverridesEnv bool
}

// lastResolution dedupes the per-call log: BuildUpstream runs on EVERY LLM call
// (Phase 52 hot-reload), so an unconditional log would emit one line per request.
// Logging only on CHANGE keeps the startup line + the panel-save line, which are
// exactly the moments an operator needs to see.
var (
	lastResolutionMu sync.Mutex
	lastResolution   string
)

// logResolution prints the effective upstream source once per change.
//
// WHY THIS EXISTS (2026-11-10 root cause): cmd/server printed "[OK] chat model
// preset from env (…)"; .env said GLM_BASE_URL=http://34.21.254.49:8080/v1; and
// yet every request went to api.minimaxi.com and returned 429 — because a stale
// ~/.skillforge/llm.json (saved 2026-10-26 from the admin panel) takes precedence
// over env by design and nothing said so. The investigation burned days on an
// "env is wrong / key is throttled" theory while the actual endpoint was in a
// file nobody looked at. One line naming the source makes that impossible.
func logResolution(r resolution) {
	identity := fmt.Sprintf("%s|%s|%s|%s|%d", r.Source, r.Detail, r.BaseURL, r.Model, r.KeyLen)
	lastResolutionMu.Lock()
	if identity == lastResolution {
		lastResolutionMu.Unlock()
		return
	}
	lastResolution = identity
	lastResolutionMu.Unlock()

	key := "unset"
	if r.KeyLen > 0 {
		key = "set"
	}
	if r.Source == "PANEL_FILE" {
		log.Printf("[INFO] llm: upstream source=PANEL_FILE config=%s base_url=%s model=%s key=%s", r.Detail, r.BaseURL, r.Model, key)
		if r.OverridesEnv {
			log.Printf("[WARN] llm: PANEL FILE TAKES PRECEDENCE — .env is IGNORED for the chat model (%s). Delete %s to fall back to .env.", r.EnvVars, llmconfig.ConfigPath())
		}
		return
	}
	log.Printf("[INFO] llm: upstream source=ENV_PRESET %s base_url=%s model=%s key=%s (%s)", r.Detail, r.BaseURL, r.Model, key, r.EnvVars)
}

// envPresence renders WHICH env knobs are set (names only, never values) so the
// "LOCAL_* shadows GLM_*" question is answerable from one line.
func envPresence() string {
	names := []string{"LOCAL_API_KEY", "LOCAL_BASE_URL", "LOCAL_MODEL", "GLM_API_KEY", "GLM_BASE_URL", "GLM_MODEL", "LOCAL_LLM_THINK"}
	out := ""
	for _, n := range names {
		state := "unset"
		if os.Getenv(n) != "" {
			state = "set"
		}
		if out != "" {
			out += " "
		}
		out += n + "=" + state
	}
	return out
}

// anyEnvSet reports whether any env LLM knob is set at all — used to warn when a
// panel file is silently shadowing them.
func anyEnvSet() bool {
	for _, n := range []string{"LOCAL_API_KEY", "LOCAL_BASE_URL", "LOCAL_MODEL", "GLM_API_KEY", "GLM_BASE_URL", "GLM_MODEL"} {
		if os.Getenv(n) != "" {
			return true
		}
	}
	return false
}
