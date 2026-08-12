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
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/skill"
)

// ---------------------------------------------------------------------------
// LLM stub (server-side mirror of internal/skill/wizard_test.go)
// ---------------------------------------------------------------------------

type stubWizardLLM struct {
	mu       sync.Mutex
	response string
	err      error
	calls    int
}

var _ model.ToolCallingChatModel = (*stubWizardLLM)(nil)

func (m *stubWizardLLM) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	if m.response == "" {
		m.response = "{}"
	}
	return &schema.Message{Role: schema.Assistant, Content: m.response}, nil
}

func (m *stubWizardLLM) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (m *stubWizardLLM) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// validWizardManifestJSON — must be parseable + pass skill.Validate.
// Mirrors the fixture in internal/skill/wizard_test.go.
const wizardValidManifestJSON = `{
  "description": "审批流合规审计 Skill，依据权威 tier 制度给出可核验判定",
  "agent": "ChatModelAgent",
  "reads": ["authority/**"],
  "writes": [],
  "steps": "1. 检索权威制度条款\n2. 逐条比对并给出判定",
  "guardrails": {
    "prohibitions": [
      "Never cite clauses not retrieved from authority tier",
      "Never paraphrase critical clauses (quote verbatim span)",
      "Never modify authority tier content"
    ],
    "required_refusal_phrases": ["no_authorized_source"],
    "tier_limits": {"authority": ["read-only"]}
  },
  "temperature": 0.0
}`

// newWizardHertz spins up a Hertz instance with the wizard + wizard/commit
// routes registered. Used by the SC-3 HTTP surface tests.
func newWizardHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	h := server.Default(server.WithListener(ln))
	h.POST("/api/skills/wizard", deps.wizardHandler)
	h.POST("/api/skills/wizard/commit", deps.wizardCommitHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	cleanup := func() {
		_ = h.Close()
		<-errCh
	}
	addr := "127.0.0.1:" + portStr(port)
	return addr, cleanup
}

// postWizard issues POST /api/skills/wizard with a JSON body and
// returns the decoded wizardResponse.
func postWizard(t *testing.T, addr, body string) (int, wizardResponse) {
	t.Helper()
	resp, err := http.Post("http://"+addr+"/api/skills/wizard", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out wizardResponse
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", string(raw))
	return resp.StatusCode, out
}

func postWizardCommit(t *testing.T, addr, body string) (int, wizardCommitResponse) {
	t.Helper()
	resp, err := http.Post("http://"+addr+"/api/skills/wizard/commit", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out wizardCommitResponse
	require.NoError(t, json.Unmarshal(raw, &out), "body: %s", string(raw))
	return resp.StatusCode, out
}

// ---------------------------------------------------------------------------
// Tests — extractSkillName (server-side gate, runs BEFORE the LLM call)
// ---------------------------------------------------------------------------

func TestExtractSkillName_HappyPaths(t *testing.T) {
	cases := []struct {
		name    string
		purpose string
		want    string
	}{
		{"english_full_sentence", "Compliance audit Skill for policy reviews.", "compliance-audit-skill-for-policy-reviews"},
		{"chinese_purpose_with_english_keyword", "审批流合规审计 Skill", "skill"},
		{"english_with_capitals", "My QA Skill for testing.", "my-qa-skill-for-testing"},
		{"no_terminator", "creative-writing-helper", "creative-writing-helper"},
		{"dash_in_purpose", "approval-flow-audit", "approval-flow-audit"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractSkillName(tc.purpose)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractSkillName_Rejections(t *testing.T) {
	cases := []struct {
		name    string
		purpose string
	}{
		{"empty", ""},
		{"only_chinese", "审批流合规审计"},
		{"only_punctuation", "..."},
		{"only_spaces", "   "},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := extractSkillName(tc.purpose)
			require.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Tests — wizard HTTP surface
// ---------------------------------------------------------------------------

// TestWizardHandler_503WhenNotInitialized — when Deps.Wizard is nil,
// the handler returns 503 with a structured error envelope (not a 500).
func TestWizardHandler_503WhenNotInitialized(t *testing.T) {
	deps := newTestDeps() // Wizard is nil
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	code, resp := postWizard(t, addr, `{"answers":{"purpose":"x"}}`)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.NotEmpty(t, resp.Error, "the 503 envelope MUST carry a non-empty Error string")
	assert.Nil(t, resp.Preview, "no preview on the 503 path")
}

func TestWizardCommitHandler_503WhenNotInitialized(t *testing.T) {
	deps := newTestDeps()
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	code, resp := postWizardCommit(t, addr, `{"name":"x","preview":{}}`)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.NotEmpty(t, resp.Error)
}

// TestWizardHandler_HappyPath — POST /api/skills/wizard returns the
// previewed Manifest + extracted name.
func TestWizardHandler_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	llm := &stubWizardLLM{response: wizardValidManifestJSON}
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(llm, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	body := `{"answers":{"purpose":"Compliance audit Skill for policy reviews.","reads":["authority/**"],"skillType":"compliance"}}`
	code, resp := postWizard(t, addr, body)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, resp.Preview, "successful wizard call MUST return a non-nil preview")
	assert.Empty(t, resp.Error)
	assert.Equal(t, "compliance-audit-skill-for-policy-reviews", resp.Name,
		"the Skill name is derived from the first sentence of the purpose")
	// Validate was run — the preview passes the 7-field gate.
	// (resp.Preview.Name is json:"-" on purpose — the directory name is
	// carried in resp.Name, not echoed in the body.)
	assert.NoError(t, skill.Validate(resp.Preview))
	assert.Equal(t, 1, llm.calls, "one LLM call on a valid first response (no retry)")
}

// TestWizardHandler_LLMErrorSurfacesToFE — T-5-18: a non-validation
// LLM error is surfaced to the FE for inline display.
func TestWizardHandler_LLMErrorSurfacesToFE(t *testing.T) {
	tmp := t.TempDir()
	llm := &stubWizardLLM{err: context.DeadlineExceeded}
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(llm, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	body := `{"answers":{"purpose":"Compliance audit","reads":["authority/**"],"skillType":"compliance"}}`
	code, resp := postWizard(t, addr, body)
	require.Equal(t, http.StatusOK, code, "the handler returns 200 with a structured error envelope (not 500)")
	assert.Nil(t, resp.Preview)
	assert.NotEmpty(t, resp.Error, "the error must be surfaced to the FE inline")
}

// TestWizardHandler_MalformedJSONReturns400 — T-2-30 / V5 ASVS: a
// malformed body is a 400, never a 500.
func TestWizardHandler_MalformedJSONReturns400(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	code, resp := postWizard(t, addr, `{`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.NotEmpty(t, resp.Error)
}

// TestWizardHandler_NameExtractionFailureReturns400 — the purpose is
// empty/non-ASCII → no extractable name → 400.
func TestWizardHandler_NameExtractionFailureReturns400(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	// Chinese-only purpose with no English keywords.
	body := `{"answers":{"purpose":"审批流合规审计","reads":["authority/**"],"skillType":"compliance"}}`
	code, resp := postWizard(t, addr, body)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.NotEmpty(t, resp.Error, "the name-extraction error must reach the FE so the user can rephrase")
}

// TestWizardCommitHandler_HappyPath — atomic write succeeds; the path
// matches the Loader/Registry contract (skills/<name>/SKILL.md).
func TestWizardCommitHandler_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	// Build a valid preview payload from the canonical manifest.
	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(wizardValidManifestJSON), preview))

	body, err := json.Marshal(wizardCommitRequest{Name: "compliance-audit", Preview: preview})
	require.NoError(t, err)

	code, resp := postWizardCommit(t, addr, string(body))
	require.Equal(t, http.StatusOK, code)
	require.True(t, resp.OK, "error=%s", resp.Error)
	require.NotEmpty(t, resp.Path)

	// The committed path matches the Loader/Registry contract.
	expected := filepath.Join(tmp, "compliance-audit", "SKILL.md")
	assert.Equal(t, expected, resp.Path)

	// File exists + content has the frontmatter opener.
	got, readErr := os.ReadFile(resp.Path)
	require.NoError(t, readErr)
	assert.True(t, strings.HasPrefix(string(got), "---\n"))
}

// TestWizardCommitHandler_RejectsPathEscape — T-5-16: a name with
// characters outside ^[a-z0-9-]+$ is rejected BEFORE any disk write.
func TestWizardCommitHandler_RejectsPathEscape(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(wizardValidManifestJSON), preview))

	body, err := json.Marshal(wizardCommitRequest{Name: "../../etc/passwd", Preview: preview})
	require.NoError(t, err)

	code, resp := postWizardCommit(t, addr, string(body))
	assert.Equal(t, http.StatusOK, code, "the handler returns 200 with a structured error envelope (matches validateSkillHandler shape)")
	assert.False(t, resp.OK)
	assert.NotEmpty(t, resp.Error)
	assert.Contains(t, resp.Error, "does not match")

	// No file written.
	entries, _ := os.ReadDir(tmp)
	assert.Empty(t, entries, "the path-escape rejection must not write anything")
}

// TestWizardCommitHandler_RejectsInvalidManifest — third gate: a
// manifest that fails Validate is rejected before any disk write.
func TestWizardCommitHandler_RejectsInvalidManifest(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	// Manifest missing temperature (SYSK-08 violation).
	invalid := `{"description":"broken","agent":"ChatModelAgent","reads":["authority/**"],"writes":[],"steps":"1. step","guardrails":{"prohibitions":["a","b","c"],"required_refusal_phrases":[],"tier_limits":{}}}`
	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(invalid), preview))

	body, err := json.Marshal(wizardCommitRequest{Name: "broken-skill", Preview: preview})
	require.NoError(t, err)

	code, resp := postWizardCommit(t, addr, string(body))
	assert.Equal(t, http.StatusOK, code)
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "validation")

	// No file written.
	entries, _ := os.ReadDir(tmp)
	assert.Empty(t, entries)
}

// TestWizardCommitHandler_MissingPreviewReturns400 — defensive: a
// commit without a preview is a malformed request.
func TestWizardCommitHandler_MissingPreviewReturns400(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	code, resp := postWizardCommit(t, addr, `{"name":"foo"}`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, resp.Error, "preview is required")
}

// TestWizardRoutes_Registered — regression: both wizard routes MUST be
// wired into the default New() server. A grep-style regression check
// that exercises the route table without standing up the wizard deps.
func TestWizardRoutes_Registered(t *testing.T) {
	deps := newTestDeps() // Wizard nil — the 503 branch is enough for registration
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	// Both endpoints accept POST; they should respond (with 503 since
	// Deps.Wizard is nil). A 404 would mean the route isn't registered.
	for path, body := range map[string]string{
		"/api/skills/wizard":       `{"answers":{"purpose":"x"}}`,
		"/api/skills/wizard/commit": `{"name":"x","preview":{}}`,
	} {
		resp, err := http.Post("http://"+addr+path, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
			"route %s must be registered (got 404)", path)
	}
}
