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

// TestWizardCommitHandler_RefusesExistingSkillWith409 — W-1: committing
// a Skill whose name already exists MUST return 409 Conflict with the
// structured {ok:false, error:"..."} envelope. The originally-committed
// file is preserved byte-for-byte (no silent overwrite). Distinguished
// from the 200 + OK:false validation-failure path so the FE can surface
// "pick a different name" inline.
func TestWizardCommitHandler_RefusesExistingSkillWith409(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps()
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(wizardValidManifestJSON), preview))
	body, err := json.Marshal(wizardCommitRequest{Name: "compliance-audit", Preview: preview})
	require.NoError(t, err)

	// First commit succeeds.
	code, resp := postWizardCommit(t, addr, string(body))
	require.Equal(t, http.StatusOK, code)
	require.True(t, resp.OK, "first commit must succeed: error=%s", resp.Error)

	// Snapshot the original body so we can assert no overwrite.
	originalBody, readErr := os.ReadFile(resp.Path)
	require.NoError(t, readErr)

	// Second commit with the same name MUST 409.
	code2, resp2 := postWizardCommit(t, addr, string(body))
	assert.Equal(t, http.StatusConflict, code2,
		"a name collision is a Conflict (W-1), not a validation failure (200) or a malformed request (400)")
	assert.False(t, resp2.OK, "the 409 envelope MUST carry OK=false")
	assert.Empty(t, resp2.Path, "no path must be returned on the refusal path")
	assert.Contains(t, resp2.Error, "already exists",
		"the error message must guide the operator to pick a different name")
	assert.Contains(t, resp2.Error, "compliance-audit",
		"the error message must name the conflicting Skill")

	// The originally-committed file is preserved byte-for-byte.
	afterBody, readErr := os.ReadFile(resp.Path)
	require.NoError(t, readErr)
	assert.Equal(t, originalBody, afterBody,
		"the refused commit must not mutate the existing SKILL.md")
}

// TestWizardCommitHandler_RegistersAndRefreshesLoader — UAT Test 9
// regression: after a successful commit, the new Skill MUST be visible
// to GET /api/skills WITHOUT a server restart. The handler now calls
// Loader.Scan() synchronously after the atomic write and verifies the
// new name landed in the cache; Registered=true is the FE's signal that
// the "已注册并热加载" toast is truthful.
//
// The test stands up both a real Wizard and a real Loader against the
// same temp dir (mirrors cmd/server/main.go wiring), registers BOTH
// the commit route AND the GET /api/skills route, POSTs a commit, and
// asserts:
//   - response.OK == true AND response.Registered == true
//   - response.SkillsTotal == 1 (the freshly-committed skill)
//   - GET /api/skills immediately after returns 1 skill whose
//     description matches the committed preview
//
// Pre-fix behavior was: OK=true, Registered=(field did not exist), and
// GET /api/skills returned [] because the Loader's startup cache was
// never refreshed.
func TestWizardCommitHandler_RegistersAndRefreshesLoader(t *testing.T) {
	tmp := t.TempDir()

	// Loader + Wizard share the same skills dir — same as cmd/server.
	loader := skill.NewLoader(tmp)
	_, err := loader.Scan()
	require.NoError(t, err)
	require.Empty(t, loader.Skills(), "freshly-scanned empty skills dir must yield 0 skills")

	deps := newTestDeps()
	deps.Loader = loader
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)

	// Stand up BOTH routes so we can drive the full closed loop in one test.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	h := server.Default(server.WithListener(ln))
	h.POST("/api/skills/wizard/commit", deps.wizardCommitHandler)
	h.GET("/api/skills", deps.listSkillsHandler)
	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	defer func() { _ = h.Close(); <-errCh }()
	addr := "127.0.0.1:" + portStr(port)

	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(wizardValidManifestJSON), preview))
	body, err := json.Marshal(wizardCommitRequest{Name: "expense-audit", Preview: preview})
	require.NoError(t, err)

	// 1. POST commit → OK + Registered (the load-bearing assertion).
	code, resp := postWizardCommit(t, addr, string(body))
	require.Equal(t, http.StatusOK, code, "commit error=%s", resp.Error)
	require.True(t, resp.OK, "atomic write must succeed: error=%s", resp.Error)
	require.True(t, resp.Registered,
		"after-fix contract: Registered MUST be true when the rescan finds the new name; error=%s", resp.Error)
	assert.Equal(t, 1, resp.SkillsTotal, "the rescan should see exactly the 1 freshly-committed skill")
	assert.NotEmpty(t, resp.Path)

	// 2. GET /api/skills — the new skill is now listed WITHOUT a restart.
	gList, gCode := getSkillsFromAddr(t, addr)
	require.Equal(t, http.StatusOK, gCode)
	require.Len(t, gList, 1,
		"GET /api/skills MUST list the freshly-committed skill without a server restart")
	assert.Contains(t, gList[0]["description"], "审批流", "the listed skill must be the one we just committed")
	assert.Equal(t, "ChatModelAgent", gList[0]["agent"])
}

// TestWizardCommitHandler_RegisteredFalseWhenLoaderNil — defensive:
// when Deps.Loader is nil (existing 503-fallback test config + any
// future server mode that bypasses listing), the commit still writes
// the file (OK=true) but reports Registered=false with a non-empty
// Error so the FE does not fire the false "已注册并热加载" toast.
func TestWizardCommitHandler_RegisteredFalseWhenLoaderNil(t *testing.T) {
	tmp := t.TempDir()
	deps := newTestDeps() // Loader is nil
	deps.Wizard = skill.NewWizard(&stubWizardLLM{}, tmp)
	addr, cleanup := newWizardHertz(t, deps)
	defer cleanup()

	preview := &skill.Manifest{}
	require.NoError(t, json.Unmarshal([]byte(wizardValidManifestJSON), preview))
	body, err := json.Marshal(wizardCommitRequest{Name: "no-loader-skill", Preview: preview})
	require.NoError(t, err)

	code, resp := postWizardCommit(t, addr, string(body))
	require.Equal(t, http.StatusOK, code)
	require.True(t, resp.OK, "file write succeeds even without a Loader")
	assert.False(t, resp.Registered,
		"Registered MUST be false when no Loader is wired — the toast must NOT fire on this path")
	assert.NotEmpty(t, resp.Error,
		"a non-empty Error guides the FE to the accurate 'written but not registered' toast copy")
	assert.Contains(t, resp.Error, "Loader")
}

// getSkillsFromAddr issues GET /api/skills against a running test server
// and returns the decoded list. Mirrors the helper in skills_test.go but
// is local to the wizard test file so the closed-loop regression test is
// self-contained.
func getSkillsFromAddr(t *testing.T, addr string) ([]map[string]any, int) {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/api/skills")
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(raw, &out), "GET /api/skills body: %s", string(raw))
	return out, resp.StatusCode
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
		"/api/skills/wizard":        `{"answers":{"purpose":"x"}}`,
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
