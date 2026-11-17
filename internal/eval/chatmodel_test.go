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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/llmconfig"
)

// 261113-obm: eval must resolve the SAME endpoint triple the server
// does — llmconfig.Effective() with panel config > env preset. These
// tests pin that contract for ResolveLLM and ActiveModelVersion (the
// pin-comparison source). Every case isolates SKILLFORGE_LLM_CONFIG via
// t.Setenv so no test touches the developer's real ~/.skillforge/llm.json
// or leaks env into a sibling case.

// writePanelConfig saves a panel config through llmconfig.Save — the
// same writer the admin panel uses — with SKILLFORGE_LLM_CONFIG pointed
// at a temp path.
func writePanelConfig(t *testing.T, cfg llmconfig.Config) {
	t.Helper()
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "llm.json"))
	require.NoError(t, llmconfig.Save(cfg))
}

// isolateEnvPresets blanks every env-preset slot ("" = unset semantics
// for the resolution chains) so host-machine .env values cannot leak in.
func isolateEnvPresets(t *testing.T) {
	t.Helper()
	for _, k := range []string{"LOCAL_BASE_URL", "LOCAL_API_KEY", "LOCAL_MODEL", "GLM_BASE_URL", "GLM_API_KEY", "JUDGE_MODEL"} {
		t.Setenv(k, "")
	}
}

// TestResolveLLM_PanelConfigWins: with a panel config present, env is
// SET but must lose on all three slots — what the page saves is what
// eval dials.
func TestResolveLLM_PanelConfigWins(t *testing.T) {
	writePanelConfig(t, llmconfig.Config{
		Vendor:  "zhipu",
		Plan:    "coding-plan",
		BaseURL: "https://panel.example.com/v1",
		Model:   "glm-panel-model",
		APIKey:  "panel-key-12345678",
	})
	t.Setenv("LOCAL_BASE_URL", "http://env.example.com/v1")
	t.Setenv("LOCAL_API_KEY", "env-key")
	t.Setenv("LOCAL_MODEL", "env-model")
	t.Setenv("JUDGE_MODEL", "judge-model")
	t.Setenv("GLM_BASE_URL", "https://glm.example.com/v1")
	t.Setenv("GLM_API_KEY", "glm-key")

	cfg := ResolveLLM()
	assert.Equal(t, "https://panel.example.com/v1", cfg.BaseURL)
	assert.Equal(t, "glm-panel-model", cfg.Model)
	assert.Equal(t, "panel-key-12345678", cfg.APIKey)
}

// TestResolveLLM_PanelPlanFallback: a panel config with a blank base_url
// falls back to the catalog plan (A-04 parity with BuildUpstream) — and
// env must not fill the slot.
func TestResolveLLM_PanelPlanFallback(t *testing.T) {
	writePanelConfig(t, llmconfig.Config{
		Vendor: "zhipu",
		Plan:   "coding-plan",
		Model:  "glm-panel-model",
		APIKey: "panel-key-12345678",
	})
	t.Setenv("LOCAL_BASE_URL", "http://env.example.com/v1")
	t.Setenv("LOCAL_API_KEY", "env-key")
	t.Setenv("LOCAL_MODEL", "env-model")

	cfg := ResolveLLM()
	assert.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4", cfg.BaseURL)
	assert.Equal(t, "glm-panel-model", cfg.Model)
	assert.Equal(t, "panel-key-12345678", cfg.APIKey)
}

// TestResolveLLM_EnvPresetWhenNoPanelConfig: no panel file ⇒ the env
// preset chain answers, LOCAL_* over GLM_* / JUDGE_MODEL.
func TestResolveLLM_EnvPresetWhenNoPanelConfig(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "absent.json"))
	isolateEnvPresets(t)
	t.Setenv("LOCAL_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("LOCAL_API_KEY", "local-key")
	t.Setenv("LOCAL_MODEL", "local-model")
	t.Setenv("GLM_BASE_URL", "https://glm.example.com/v1")
	t.Setenv("GLM_API_KEY", "glm-key")
	t.Setenv("JUDGE_MODEL", "judge-model")

	cfg := ResolveLLM()
	assert.Equal(t, "http://localhost:11434/v1", cfg.BaseURL)
	assert.Equal(t, "local-key", cfg.APIKey)
	assert.Equal(t, "local-model", cfg.Model)

	// GLM_* / JUDGE_MODEL fill the slots LOCAL_* leaves empty.
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("LOCAL_API_KEY", "")
	t.Setenv("LOCAL_MODEL", "")
	cfg = ResolveLLM()
	assert.Equal(t, "https://glm.example.com/v1", cfg.BaseURL)
	assert.Equal(t, "glm-key", cfg.APIKey)
	assert.Equal(t, "judge-model", cfg.Model)
}

// TestResolveLLM_DefaultsWithNothingSet: neither panel nor env ⇒ the
// built-in floors.
func TestResolveLLM_DefaultsWithNothingSet(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "absent.json"))
	isolateEnvPresets(t)

	cfg := ResolveLLM()
	assert.Equal(t, DefaultGLMBaseURL, cfg.BaseURL)
	assert.Equal(t, DefaultJudgeModel, cfg.Model)
	assert.Empty(t, cfg.APIKey, "no key source ⇒ empty key so NewGLMChatModel fails loudly")
}

// TestResolveLLM_UnresolvablePanelKeyDoesNotDegradeToEnv: a panel
// config whose api_key_env names an unset variable must NOT silently
// fall back to env — the key comes back empty so NewGLMChatModel fails
// with the actionable message (明确失败，不静默降级).
func TestResolveLLM_UnresolvablePanelKeyDoesNotDegradeToEnv(t *testing.T) {
	writePanelConfig(t, llmconfig.Config{
		Vendor:    "zhipu",
		Plan:      "coding-plan",
		Model:     "glm-panel-model",
		APIKeyEnv: "SKILLFORGE_TEST_MISSING_KEY_261113",
	})
	t.Setenv("LOCAL_API_KEY", "env-key")

	cfg := ResolveLLM()
	assert.Empty(t, cfg.APIKey, "unresolvable panel key must blank the key, not degrade to LOCAL_API_KEY")
}

// TestActiveModelVersion_PanelConfig: the pin comparison must read the
// SAME source the judge dials — the page model wins even when env names
// a different one. This is the regression that motivated 261113-obm:
// before, a panel model change updated the dial target but
// ActiveModelVersion kept reporting the env value.
func TestActiveModelVersion_PanelConfig(t *testing.T) {
	writePanelConfig(t, llmconfig.Config{
		Vendor:  "zhipu",
		Plan:    "coding-plan",
		BaseURL: "https://panel.example.com/v1",
		Model:   "glm-panel-model",
		APIKey:  "panel-key-12345678",
	})
	t.Setenv("LOCAL_MODEL", "env-model")
	t.Setenv("JUDGE_MODEL", "judge-model")

	assert.Equal(t, "glm-panel-model", ActiveModelVersion())
}

// TestActiveModelVersion_UnresolvablePanelKeyFallsToFloor: with a panel
// config whose key cannot be resolved, the model id is unknown — fall
// back to DefaultJudgeModel rather than reporting a model the process
// cannot actually dial.
func TestActiveModelVersion_UnresolvablePanelKeyFallsToFloor(t *testing.T) {
	writePanelConfig(t, llmconfig.Config{
		Vendor:    "zhipu",
		Plan:      "coding-plan",
		Model:     "glm-panel-model",
		APIKeyEnv: "SKILLFORGE_TEST_MISSING_KEY_261113",
	})
	isolateEnvPresets(t)

	assert.Equal(t, DefaultJudgeModel, ActiveModelVersion())
}
