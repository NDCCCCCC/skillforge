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

import "testing"

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
