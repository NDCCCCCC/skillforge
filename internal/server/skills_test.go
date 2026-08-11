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
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The 7 SC-1 acceptance tests for the skills HTTP surface:
//
//  1. TestListSkillsHandler_ReturnsCompliance — SYSK-01
//  2. TestListSkillsHandler_EmptyRegistry — defensive `[]` not `null`
//  3. TestValidateSkillHandler_ValidBody — SYSK-02 happy path
//  4. TestValidateSkillHandler_MissingTemperature — SYSK-08
//  5. TestValidateSkillHandler_MalformedJSON — T-2-30 / V5 ASVS
//  6. TestValidateSkillHandler_DoesNotPersist — D2-07 dry-run contract
//  7. TestValidateSkillHandler_RejectsWildcardReads — P-SKILL-3

// newTestSkillsHertz spins a Hertz server on a dynamic port with BOTH
// skills routes registered against deps (mirrors New()).
func newTestSkillsHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/skills", deps.listSkillsHandler)
	h.POST("/api/skills", deps.validateSkillHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()

	cleanup := func() {
		_ = h.Close()
		<-errCh
	}
	addr := "127.0.0.1:" + portStr(port)
	if err := waitForListener(addr, 3*time.Second); err != nil {
		cleanup()
		t.Fatalf("server warmup: %v", err)
	}
	return addr, cleanup
}

// complianceSkillYAML is a valid (compact) stand-in for the canonical
// skills/compliance/SKILL.md fixture.
const complianceSkillYAML = `---
description: 企业合规/制度审查 Skill — 依据权威制度文档给出可核验的合规判定
agent: ChatModelAgent
reads:
  - authority/**
writes: []
steps: |
  1. 检索权威制度条款
  2. 逐条比对并给出判定
guardrails:
  prohibitions:
    - "Never cite clauses not retrieved from authority tier"
    - "Never paraphrase critical clauses (quote verbatim span)"
    - "Never reveal personal/contact information of any individual"
  required_refusal_phrases:
    - "no_authorized_source"
  tier_limits:
    authority: ["read-only"]
temperature: 0.0
---
# 合规审查
`

// newComplianceLoader writes complianceSkillYAML into a temp
// skills/compliance/SKILL.md and returns a scanned Loader.
func newComplianceLoader(t *testing.T) *skill.Loader {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "compliance")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(complianceSkillYAML), 0o644))
	l := skill.NewLoader(root)
	_, err := l.Scan()
	require.NoError(t, err)
	return l
}

// getSkills issues GET /api/skills and returns the decoded array.
func getSkills(t *testing.T, addr string) (int, []map[string]any, string) {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/api/skills")
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(raw, &out), "GET /api/skills body: %s", string(raw))
	return resp.StatusCode, out, string(raw)
}

// postSkill issues POST /api/skills with a raw body.
func postSkill(t *testing.T, addr, body string) (int, validationResponse) {
	t.Helper()
	resp, err := http.Post("http://"+addr+"/api/skills", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var vr validationResponse
	require.NoError(t, json.Unmarshal(raw, &vr), "POST /api/skills body: %s", string(raw))
	return resp.StatusCode, vr
}

// validSkillJSON is the full 7-field manifest as a JSON request body.
const validSkillJSON = `{
  "description": "企业合规/制度审查",
  "agent": "ChatModelAgent",
  "reads": ["authority/**"],
  "writes": [],
  "steps": "1. 检索\n2. 判定",
  "guardrails": {
    "prohibitions": ["p1", "p2", "p3"],
    "required_refusal_phrases": ["no_authorized_source"],
    "tier_limits": {"authority": ["read-only"]}
  },
  "temperature": 0.0
}`

// TestListSkillsHandler_ReturnsCompliance — SYSK-01 HTTP layer.
func TestListSkillsHandler_ReturnsCompliance(t *testing.T) {
	deps := newTestDeps()
	deps.Loader = newComplianceLoader(t)
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	code, list, raw := getSkills(t, addr)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, list, 1, "raw=%s", raw)
	assert.True(t, strings.HasPrefix(list[0]["description"].(string), "企业合规"))
	assert.Equal(t, "ChatModelAgent", list[0]["agent"])
	assert.Equal(t, []any{"authority/**"}, list[0]["reads"])
	assert.Equal(t, float64(0), list[0]["temperature"])
}

// TestListSkillsHandler_EmptyRegistry — `[]`, not `null`, not 404.
func TestListSkillsHandler_EmptyRegistry(t *testing.T) {
	deps := newTestDeps() // Loader nil
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	code, list, raw := getSkills(t, addr)
	assert.Equal(t, http.StatusOK, code)
	assert.Empty(t, list)
	assert.Contains(t, raw, "[]", "empty registry must serialize as [] not null")
}

// TestValidateSkillHandler_ValidBody — SYSK-02 happy path.
func TestValidateSkillHandler_ValidBody(t *testing.T) {
	addr, cleanup := newTestSkillsHertz(t, newTestDeps())
	defer cleanup()

	code, vr := postSkill(t, addr, validSkillJSON)
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, vr.Valid, "error=%s", vr.Error)
	assert.Empty(t, vr.Error)
}

// TestValidateSkillHandler_MissingTemperature — SYSK-08 HTTP surface.
func TestValidateSkillHandler_MissingTemperature(t *testing.T) {
	addr, cleanup := newTestSkillsHertz(t, newTestDeps())
	defer cleanup()

	body := removeJSONLine(validSkillJSON, `"temperature"`)

	code, vr := postSkill(t, addr, body)
	assert.Equal(t, http.StatusOK, code)
	assert.False(t, vr.Valid)
	assert.Contains(t, vr.Error, "temperature")
	assert.Contains(t, vr.Error, "SYSK-08")
}

// TestValidateSkillHandler_MalformedJSON — T-2-30 / V5 ASVS.
func TestValidateSkillHandler_MalformedJSON(t *testing.T) {
	addr, cleanup := newTestSkillsHertz(t, newTestDeps())
	defer cleanup()

	code, vr := postSkill(t, addr, `{`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.False(t, vr.Valid)
	assert.Contains(t, vr.Error, "malformed json")
}

// TestValidateSkillHandler_RejectsWildcardReads — P-SKILL-3 HTTP surface.
func TestValidateSkillHandler_RejectsWildcardReads(t *testing.T) {
	addr, cleanup := newTestSkillsHertz(t, newTestDeps())
	defer cleanup()

	body := strings.Replace(validSkillJSON, `["authority/**"]`, `["*"]`, 1)
	code, vr := postSkill(t, addr, body)
	assert.Equal(t, http.StatusOK, code)
	assert.False(t, vr.Valid)
	assert.Contains(t, vr.Error, "wildcard")
}

// TestValidateSkillHandler_DoesNotPersist — the D2-07 dry-run contract.
// POSTing a valid Skill must NOT register it: GET before == GET after.
func TestValidateSkillHandler_DoesNotPersist(t *testing.T) {
	deps := newTestDeps()
	deps.Loader = newComplianceLoader(t)
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	_, before, _ := getSkills(t, addr)
	require.Len(t, before, 1)

	code, vr := postSkill(t, addr, validSkillJSON)
	require.Equal(t, http.StatusOK, code)
	require.True(t, vr.Valid, "error=%s", vr.Error)

	_, after, raw := getSkills(t, addr)
	assert.Len(t, after, 1, "dry-run POST must not register a Skill; raw=%s", raw)
	assert.Equal(t, before, after, "registry mutated by a dry-run POST (D2-07 violation)")
}

// removeJSONLine drops the line containing needle and fixes the trailing
// comma on the preceding line, producing still-valid JSON.
func removeJSONLine(src, needle string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.Contains(ln, needle) {
			// drop the trailing comma of the previous kept line
			if n := len(out); n > 0 {
				out[n-1] = strings.TrimSuffix(strings.TrimRight(out[n-1], " "), ",")
			}
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}
