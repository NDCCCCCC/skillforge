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

	"github.com/skillforge/skillforge/internal/kb"
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

// testKBReg returns the read-only registry the server tests resolve the
// reads contract against: the four baseline bases of the controlled seed
// (knowledge_bases.yaml), all active. Display names are ASCII here —
// they never participate in reads resolution.
func testKBReg() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	})
}

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
	l := skill.NewLoader(root, testKBReg())
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
// Loader-only Deps (no Registry) → exercises the fallback path: the
// wrapped skillItem shape with the manifest nested under "manifest" +
// version:0 / status:"ok".
func TestListSkillsHandler_ReturnsCompliance(t *testing.T) {
	deps := newTestDeps()
	deps.Loader = newComplianceLoader(t)
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	code, list, raw := getSkills(t, addr)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, list, 1, "raw=%s", raw)
	manifest := list[0]["manifest"].(map[string]any)
	assert.True(t, strings.HasPrefix(manifest["description"].(string), "企业合规"))
	assert.Equal(t, "ChatModelAgent", manifest["agent"])
	assert.Equal(t, []any{"authority/**"}, manifest["reads"])
	assert.Equal(t, float64(0), manifest["temperature"])
	// Wrapped fields (fallback path).
	assert.Equal(t, "compliance", list[0]["id"])
	assert.Equal(t, "compliance", list[0]["name"])
	assert.Equal(t, float64(0), list[0]["version"], "fallback (no Registry) → version 0")
	assert.Equal(t, "ok", list[0]["status"])
	_, hasError := list[0]["error"]
	assert.False(t, hasError, "ok status must omit the error field")
}

// brokenSkillYAML is complianceSkillYAML with the `temperature:` line
// dropped, so the 7-field validator rejects it (SYSK-08). Used to assert
// the status:"failed" overlay WITHOUT a version bump (keep-old contract).
const brokenSkillYAML = `---
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
---
# 合规审查（broken — no temperature）
`

// newComplianceRegistry writes one valid compliance SKILL.md into a temp
// skills root and returns the *skill.Registry over it (nil embedder —
// description-embedding / routing is not exercised here) plus the skill's
// SKILL.md path (for deterministic Reload calls).
func newComplianceRegistry(t *testing.T) (*skill.Registry, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "compliance")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(complianceSkillYAML), 0o644))
	reg, err := skill.New(root, nil, testKBReg())
	require.NoError(t, err)
	return reg, filepath.Join(dir, "SKILL.md")
}

// TestListSkillsHandler_RegistryVersionAndFailedStatus — the §2 polling-B
// primary path. A real skill.Registry over a temp skills dir exposes a
// JS-safe version (>=1) + status:"ok"; a broken-YAML Reload surfaces
// status:"failed" + a non-empty error WITHOUT bumping the version
// (keep-old contract), end-to-end through the HTTP layer.
func TestListSkillsHandler_RegistryVersionAndFailedStatus(t *testing.T) {
	reg, skillPath := newComplianceRegistry(t)

	deps := newTestDeps()
	deps.Registry = reg // Loader nil → exercises the Registry (primary) path
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	// Good state.
	code, list, raw := getSkills(t, addr)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, list, 1, "raw=%s", raw)
	assert.Equal(t, "compliance", list[0]["id"])
	assert.Equal(t, "ok", list[0]["status"])
	verGood := list[0]["version"].(float64)
	assert.GreaterOrEqual(t, verGood, float64(1), "Registry version must be a JS-safe counter >=1")
	manifest, ok := list[0]["manifest"].(map[string]any)
	require.True(t, ok, "manifest must be present on a good skill")
	assert.Contains(t, manifest["description"], "企业合规")
	_, hasErr := list[0]["error"]
	assert.False(t, hasErr, "ok status must omit error")

	// Break the YAML + synchronously reload → status flips to failed,
	// error surfaces, version UNCHANGED (keep-old contract).
	require.NoError(t, os.WriteFile(skillPath, []byte(brokenSkillYAML), 0o644))
	reg.Reload(skillPath)

	code2, list2, raw2 := getSkills(t, addr)
	require.Equal(t, http.StatusOK, code2)
	require.Len(t, list2, 1, "raw=%s", raw2)
	assert.Equal(t, "failed", list2[0]["status"], "broken-YAML reload must surface status:failed")
	assert.Equal(t, verGood, list2[0]["version"].(float64), "a failed reload MUST NOT bump the version (keep-old)")
	errMsg, _ := list2[0]["error"].(string)
	assert.Contains(t, errMsg, "temperature", "the failure error must carry the SYSK-08 detail")
	// keep-old: the manifest is the OLD (good) one — the snapshot entry
	// is untouched by the failed reload.
	manifestAfter, _ := list2[0]["manifest"].(map[string]any)
	require.NotNil(t, manifestAfter)
	assert.Contains(t, manifestAfter["description"], "企业合规", "keep-old: manifest unchanged on failure")
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

// --- Phase 26 / Plan 26-04 Task 3: the endpoint carries the registry ---

// TestValidateSkillEndpointWithRegistry — POST /api/skills against a
// wired registry: a valid skill answers 200/valid, and a reads
// declaration naming an unregistered base answers 200/invalid with the
// active slug set in the error (the load-time gate, surfaced over HTTP).
func TestValidateSkillEndpointWithRegistry(t *testing.T) {
	deps := newTestDeps()
	deps.KBRegistry = kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
	})
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	code, vr := postSkill(t, addr, validSkillJSON)
	require.Equal(t, http.StatusOK, code)
	assert.True(t, vr.Valid, "error=%s", vr.Error)

	// ghost/** is not in the one-entry registry → rejected, active set named.
	body := strings.Replace(validSkillJSON, `["authority/**"]`, `["ghost/**"]`, 1)
	code, vr = postSkill(t, addr, body)
	assert.Equal(t, http.StatusOK, code)
	assert.False(t, vr.Valid, "an unregistered slug must fail validation over HTTP (KBM-04)")
	assert.Contains(t, vr.Error, "ghost")
	assert.Contains(t, vr.Error, "authority", "the error must list the active slug set")
}

// TestValidateSkillEndpointNilRegistry503 — a tier-channel request with
// no registry wired is a 503 fail-closed (mirrors the kpiPipelineNotWired
// posture): answering "valid" would assert a trust contract nobody checked.
func TestValidateSkillEndpointNilRegistry503(t *testing.T) {
	deps := newTestDeps()
	deps.KBRegistry = nil
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	code, vr := postSkill(t, addr, validSkillJSON)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.False(t, vr.Valid)
	assert.Contains(t, vr.Error, "KB registry not wired")
}

// TestValidateSkillDataChannelStillWorks — the data channel has no KB
// semantics (D16-C1 XOR), so a data-channel skill validates even with the
// registry unwired: degrading it would break data skills for no trust gain.
func TestValidateSkillDataChannelStillWorks(t *testing.T) {
	deps := newTestDeps()
	deps.KBRegistry = nil
	addr, cleanup := newTestSkillsHertz(t, deps)
	defer cleanup()

	body := strings.Replace(validSkillJSON, `["authority/**"]`, `["data:aggregated_only"]`, 1)
	code, vr := postSkill(t, addr, body)
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, vr.Valid, "error=%s", vr.Error)
}
