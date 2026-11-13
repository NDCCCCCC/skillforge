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

package llmconfig

import (
	"path/filepath"
	"testing"
)

// CODE-AUDIT 2026-11-05 (A-05/A-06/A-10): request-field policy MUST be
// decided by the ACTUAL dialled endpoint, not the declared vendor ID and
// not by a "!= minimax" blacklist.

func TestEffectiveBaseURL_EnvPreset(t *testing.T) {
	// A-06: the env-preset path has no llm.json, but it still HAS an
	// endpoint — it must no longer be treated as "unknown vendor".
	t.Setenv("SKILLFORGE_LLM_CONFIG", t.TempDir()+"/absent.json")
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("GLM_BASE_URL", "")
	if got := EffectiveBaseURL(); got != DefaultGLMBaseURL {
		t.Fatalf("no env preset ⇒ default GLM base, got %q", got)
	}
	t.Setenv("GLM_BASE_URL", "https://api.minimaxi.com/v1")
	if got := EffectiveBaseURL(); got != "https://api.minimaxi.com/v1" {
		t.Fatalf("GLM_BASE_URL must win over default, got %q", got)
	}
	t.Setenv("LOCAL_BASE_URL", "http://localhost:11434/v1")
	if got := EffectiveBaseURL(); got != "http://localhost:11434/v1" {
		t.Fatalf("LOCAL_BASE_URL must override GLM_BASE_URL, got %q", got)
	}
}

func TestSupportsThinkingDisabled(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    bool
	}{
		// A-10: only GLM endpoints are能力白名单成员。
		{"zhipu coding plan", "https://open.bigmodel.cn/api/coding/paas/v4", true},
		{"zhipu standard", "https://open.bigmodel.cn/api/paas/v4", true},
		// A-06: MiniMax via GLM_* env preset — the case that used to
		// receive thinking:disabled and 400/2013 on every request.
		{"minimax env preset", "https://api.minimaxi.com/v1", false},
		// A-05: declared vendor may say zhipu while base_url points
		// elsewhere — the endpoint, not the label, decides.
		{"zhipu label but minimax url", "https://api.minimax.io/v1", false},
		{"openai", "https://api.openai.com/v1", false},
		{"deepseek", "https://api.deepseek.com/v1", false},
		{"siliconflow", "https://api.siliconflow.cn/v1", false},
		{"ollama", "http://localhost:11434/v1", false},
		// 空 base 回落默认 GLM（coding plan）端点 ⇒ 支持，属正确行为。
		{"empty falls back to default GLM", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SKILLFORGE_LLM_CONFIG", t.TempDir()+"/absent.json")
			t.Setenv("LOCAL_BASE_URL", "")
			t.Setenv("GLM_BASE_URL", "")
			if tc.baseURL != "" && tc.baseURL != DefaultGLMBaseURL {
				t.Setenv("GLM_BASE_URL", tc.baseURL)
			}
			if got := SupportsThinkingDisabled(); got != tc.want {
				t.Fatalf("base=%q SupportsThinkingDisabled()=%v, want %v", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestEffectiveBaseURL_FileConfigPlanFallback(t *testing.T) {
	path := newTestStoreEnv(t)
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("GLM_BASE_URL", "")
	// A-05: a file config with blank base_url falls back to the catalog
	// plan value, and the policy follows THAT actual endpoint.
	cfg := Config{Vendor: "minimax", Plan: "standard", Model: "MiniMax-M3.1", APIKey: "k12345678"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_ = path
	if got := EffectiveBaseURL(); got != "https://api.minimaxi.com/v1" {
		t.Fatalf("blank base_url must fall back to catalog plan, got %q", got)
	}
	if SupportsThinkingDisabled() {
		t.Fatal("minimax (even with blank base_url) must NOT get thinking:disabled")
	}
}

// ---- Effective (261113-obm) ----
//
// Effective 是「页面配置 > env preset」的单一事实来源（与服务端
// BuildUpstream 同一条解析链）。这里钉住它的全部回落路径。

// isolateEnvPresets 把 env preset 的全部槽位设为空串（= unset 语义），
// 保证用例之间不互相污染、也不读到宿主机 .env 注入的值。
func isolateEnvPresets(t *testing.T) {
	t.Helper()
	for _, k := range []string{"LOCAL_BASE_URL", "LOCAL_API_KEY", "LOCAL_MODEL", "GLM_BASE_URL", "GLM_API_KEY", "JUDGE_MODEL"} {
		t.Setenv(k, "")
	}
}

// TestEffective_FileConfigWinsOverEnv 覆盖两个场景：文件配置存在、以及
// 文件 + env 同时存在 —— 文件必须**整体**胜出，env 一个槽位都不许漏进来。
func TestEffective_FileConfigWinsOverEnv(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "llm.json"))
	cfg := Config{
		Vendor:  "zhipu",
		Plan:    "coding-plan",
		BaseURL: "https://panel.example.com/v1",
		Model:   "glm-panel-model",
		APIKey:  "panel-key-12345678",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// env 全部设上 —— 必须全部输给文件。
	t.Setenv("LOCAL_BASE_URL", "http://env.example.com/v1")
	t.Setenv("LOCAL_API_KEY", "env-key")
	t.Setenv("LOCAL_MODEL", "env-model")
	t.Setenv("GLM_BASE_URL", "https://glm.example.com/v1")
	t.Setenv("GLM_API_KEY", "glm-key")
	t.Setenv("JUDGE_MODEL", "judge-model")

	base, key, model, err := Effective()
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if base != "https://panel.example.com/v1" {
		t.Fatalf("base_url 必须取文件值, got %q", base)
	}
	if key != "panel-key-12345678" {
		t.Fatalf("api_key 必须取文件值, got %q", key)
	}
	if model != "glm-panel-model" {
		t.Fatalf("model 必须取文件值, got %q", model)
	}
}

// TestEffective_EnvPresetWhenNoFile：无文件配置时回落 env preset，且
// LOCAL_* 覆盖 GLM_* / JUDGE_MODEL。
func TestEffective_EnvPresetWhenNoFile(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "absent.json"))
	isolateEnvPresets(t)
	t.Setenv("LOCAL_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("LOCAL_API_KEY", "local-key")
	t.Setenv("LOCAL_MODEL", "local-model")
	t.Setenv("GLM_BASE_URL", "https://glm.example.com/v1")
	t.Setenv("GLM_API_KEY", "glm-key")
	t.Setenv("JUDGE_MODEL", "judge-model")

	base, key, model, err := Effective()
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if base != "http://localhost:11434/v1" || key != "local-key" || model != "local-model" {
		t.Fatalf("LOCAL_* 必须覆盖 GLM_*/JUDGE_MODEL: base=%q key=%q model=%q", base, key, model)
	}

	// GLM_* / JUDGE_MODEL 填补 LOCAL_* 留空的槽位。
	t.Setenv("LOCAL_BASE_URL", "")
	t.Setenv("LOCAL_API_KEY", "")
	t.Setenv("LOCAL_MODEL", "")
	base, key, model, err = Effective()
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if base != "https://glm.example.com/v1" || key != "glm-key" || model != "judge-model" {
		t.Fatalf("GLM_*/JUDGE_MODEL 必须填补空槽: base=%q key=%q model=%q", base, key, model)
	}
}

// TestEffective_DefaultsWithNothingSet：无文件、env 全空时回落包内默认值。
func TestEffective_DefaultsWithNothingSet(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "absent.json"))
	isolateEnvPresets(t)

	base, key, model, err := Effective()
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if base != DefaultGLMBaseURL {
		t.Fatalf("base_url 应回落 DefaultGLMBaseURL, got %q", base)
	}
	if model != DefaultGLMModel {
		t.Fatalf("model 应回落 DefaultGLMModel, got %q", model)
	}
	if key != "" {
		t.Fatalf("无任何 key 来源时 apiKey 应为空串, got %q", key)
	}
}

// TestEffective_BlankBaseURLFallsBackToPlan：文件配置 base_url 为空时回落
// catalog plan（A-04 parity），且 env 的 base 不得漏进来。
func TestEffective_BlankBaseURLFallsBackToPlan(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "llm.json"))
	cfg := Config{Vendor: "zhipu", Plan: "coding-plan", Model: "glm-panel-model", APIKey: "panel-key-12345678"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	isolateEnvPresets(t)
	t.Setenv("LOCAL_BASE_URL", "http://env.example.com/v1")

	base, _, _, err := Effective()
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if base != "https://open.bigmodel.cn/api/coding/paas/v4" {
		t.Fatalf("空 base_url 必须回落 catalog plan, got %q", base)
	}
}

// TestEffective_KeyResolutionFailureIsLoud：文件配置存在但 key 解析失败
// （api_key_env 指向未设置变量）必须报错 —— 明确失败，不静默降级到 env。
func TestEffective_KeyResolutionFailureIsLoud(t *testing.T) {
	t.Setenv("SKILLFORGE_LLM_CONFIG", filepath.Join(t.TempDir(), "llm.json"))
	cfg := Config{
		Vendor:    "zhipu",
		Plan:      "coding-plan",
		Model:     "glm-panel-model",
		APIKeyEnv: "SKILLFORGE_TEST_MISSING_KEY_261113",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	isolateEnvPresets(t)
	t.Setenv("LOCAL_API_KEY", "env-key")

	if _, _, _, err := Effective(); err == nil {
		t.Fatal("文件配置 key 解析失败必须报错，不得静默降级到 env preset")
	}
}
