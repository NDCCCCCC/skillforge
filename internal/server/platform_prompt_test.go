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

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// platformPromptTestHertz builds a Hertz instance with the admin preview
// route registered against deps.
func platformPromptTestHertz(t *testing.T, deps *Deps) *server.Hertz {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/admin/platform-prompt", deps.platformPromptHandler)
	return h
}

// invokePlatformPrompt issues GET /api/admin/platform-prompt?skill= with the
// optional X-Admin-Token header and returns the response recorder.
func invokePlatformPrompt(t *testing.T, deps *Deps, skillID, adminToken string) *ut.ResponseRecorder {
	t.Helper()
	h := platformPromptTestHertz(t, deps)
	var headers []ut.Header
	if adminToken != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: adminToken})
	}
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/admin/platform-prompt?skill="+skillID, nil, headers...)
}

// writeTestSkillDir creates a minimal valid skills/compliance/SKILL.md under
// a temp directory and returns the skills root.
func writeTestSkillDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "compliance")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := `---
description: admin preview test skill
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. Retrieve authority chunks.
  2. Answer grounded in retrieved chunks.
guardrails:
  prohibitions:
    - "Never answer without citation"
    - "Never fabricate sources or chunk ids"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_relevant_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.3
---

# Admin Preview Test Skill
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
	return root
}

// TestPlatformPromptPreview — PLAT-10 admin preview endpoint contract:
// token gate (unset/wrong/correct), registered skill 200 with composed
// prompt + hash, unknown skill 404.
func TestPlatformPromptPreview(t *testing.T) {
	skillDir := writeTestSkillDir(t)
	loader := skill.NewLoader(skillDir)
	_, err := loader.Scan()
	require.NoError(t, err)

	// (a) env unset → 403.
	t.Run("unset env", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "")
		deps := &Deps{Loader: loader}
		resp := invokePlatformPrompt(t, deps, "compliance", "")
		assert.Equal(t, http.StatusForbidden, resp.Code)
	})

	// (b) wrong token → 403.
	t.Run("wrong token", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{Loader: loader}
		resp := invokePlatformPrompt(t, deps, "compliance", "wrong-token")
		assert.Equal(t, http.StatusForbidden, resp.Code)
	})

	// (c) correct token + registered skill → 200 with floor + prompt + hash.
	t.Run("registered skill", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{Loader: loader}
		resp := invokePlatformPrompt(t, deps, "compliance", "the-real-token")
		require.Equal(t, http.StatusOK, resp.Code, "body: %s", string(resp.Body.Bytes()))

		var body map[string]string
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))

		assert.Equal(t, agent.PlatformFloorVersion, body["floor_version"])
		assert.Equal(t, "compliance", body["skill_id"])

		manifest := deps.findSkill("compliance")
		require.NotNil(t, manifest)
		composed := agent.RenderSystemPrompt(manifest)
		assert.Equal(t, composed, body["composed_prompt"],
			"composed_prompt must match the rendered system prompt")
		assert.Equal(t, computePromptHash("compliance", "glm-5.2", composed), body["prompt_hash"],
			"prompt_hash must be computed from the same inputs as the audit row")
	})

	// (d) correct token + unknown skill → 404.
	t.Run("unknown skill", func(t *testing.T) {
		t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
		deps := &Deps{Loader: loader}
		resp := invokePlatformPrompt(t, deps, "nonexistent", "the-real-token")
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}
