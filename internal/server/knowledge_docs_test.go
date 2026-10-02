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
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
)

// newKnowledgeTestHertz spins a Hertz server with the /api/knowledge
// docs routes registered (newTestHertz only mounts /api/chat). It also
// chdirs to the repo root for the test's duration — the docs handlers
// read the cwd-relative "knowledge/" tree (same convention as the
// skills/ loader in cmd/server), and `go test` runs with the package
// dir as cwd.
func newKnowledgeTestHertz(t *testing.T, deps *Deps) (string, func()) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err, "get wd")
	require.NoError(t, os.Chdir("../.."), "chdir to repo root")
	t.Cleanup(func() { _ = os.Chdir(wd) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "bind dynamic listener")
	port := ln.Addr().(*net.TCPAddr).Port

	h := server.Default(server.WithListener(ln))
	h.GET("/api/knowledge/docs", deps.docsHandler)
	h.GET("/api/knowledge/doc", deps.docContentHandler)

	errCh := make(chan error, 1)
	go func() { errCh <- h.Run() }()
	cleanup := func() { _ = h.Close(); <-errCh }

	addr := "127.0.0.1:" + portStr(port)
	require.NoError(t, waitForListener(addr, 3*time.Second), "server warmup")
	return addr, cleanup
}

// TestDocsHandler_ListsTiersFromDisk — GET /api/knowledge/docs returns a
// block per ACTIVE registry base (empty tiers included), tier values being
// the slugs, from the on-disk knowledge/<slug>/ tree. The repo's real
// corpus (knowledge/authority/*.md) is the fixture; the other baseline
// bases hold only .gitkeep and must render as empty.
func TestDocsHandler_ListsTiersFromDisk(t *testing.T) {
	deps := &Deps{KBRegistry: testKBReg()}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/docs")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b := drainBody(t, resp)
	// All 4 baseline slugs present; the authority base carries the corpus.
	for _, slug := range []string{"authority", "draft", "historical", "personal"} {
		assert.Contains(t, b, `"tier":"`+slug+`"`, "base %s must be listed", slug)
	}
	assert.Contains(t, b, "annual_leave.md", "authority corpus must list annual_leave.md")
	assert.NotContains(t, b, ".gitkeep", "dotfiles must be filtered out")
}

// TestDocsHandlerRegistryDriven — the block set is the ACTIVE registry
// set, not a compiled-in list: 3 active bases → exactly 3 blocks in
// ascending slug order; a deactivated 4th base never appears.
func TestDocsHandlerRegistryDriven(t *testing.T) {
	deps := &Deps{KBRegistry: kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "ghost", DisplayName: "Ghost", Role: kb.RoleDraft, RetrievalEnabled: false}, // deactivated
	})}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/docs")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Tiers []struct {
			Tier string `json:"tier"`
		} `json:"tiers"`
	}
	require.NoError(t, json.Unmarshal([]byte(drainBody(t, resp)), &body))

	slugs := make([]string, 0, len(body.Tiers))
	for _, tr := range body.Tiers {
		slugs = append(slugs, tr.Tier)
	}
	assert.Equal(t, []string{"authority", "historical", "personal"}, slugs,
		"tiers must be exactly the ACTIVE registry bases, slug ascending")
	assert.NotContains(t, slugs, "ghost", "a deactivated base must not be listed")
}

// TestDocsHandlerEmptyRegistry — an empty registry is a legal state
// (absent seed + empty store): 200 with an empty tiers array, never a 500.
func TestDocsHandlerEmptyRegistry(t *testing.T) {
	deps := &Deps{KBRegistry: kb.New(nil)}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/docs")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b := drainBody(t, resp)
	assert.Contains(t, b, `"tiers":[]`, "empty registry → empty tier list (legal degraded state)")
}

// TestDocContentHandler_PathTraversalRejected — the tier/file query
// params are concatenated into a filesystem path, so traversal attempts
// must 400 BEFORE any disk access: whitelist-violating tier, unknown tier,
// separator-bearing file, backslash-bearing file, and dot-dot prefixes.
// The tier whitelist is the kb slug REGEX (not a lookup table), so the
// open slug set does not widen the traversal surface (T-26-05-3).
func TestDocContentHandler_PathTraversalRejected(t *testing.T) {
	deps := &Deps{KBRegistry: testKBReg()}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	cases := []struct {
		name string
		url  string
	}{
		{"dot-dot tier", "/api/knowledge/doc?tier=../cmd&file=main.go"},
		{"whitespace tier", "/api/knowledge/doc?tier=%20&file=annual_leave.md"},
		{"unknown tier", "/api/knowledge/doc?tier=ghost&file=annual_leave.md"},
		{"separator in file", "/api/knowledge/doc?tier=authority&file=../main.go"},
		{"backslash in file", "/api/knowledge/doc?tier=authority&file=..\\main.go"},
		{"empty file", "/api/knowledge/doc?tier=authority&file="},
		{"missing tier", "/api/knowledge/doc?file=annual_leave.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get("http://" + addr + tc.url)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"traversal-shaped input must 400, not reach the filesystem")
		})
	}
}

// TestDocContentHandlerValidatesSlug — the tier gate is the slug whitelist
// PLUS registry membership: a traversal-shaped value 400s on the regex, an
// unregistered slug 400s naming the active set, and a live base 200s with
// the pre-existing behavior.
func TestDocContentHandlerValidatesSlug(t *testing.T) {
	deps := &Deps{KBRegistry: testKBReg()}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	// tier=../cmd → the whitelist regex refuses before any registry work.
	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=../cmd&file=main.go")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "traversal tier must 400 (T-26-05-3 regression)")

	// tier=ghost → the registry refuses and names the active set.
	resp, err = http.Get("http://" + addr + "/api/knowledge/doc?tier=ghost&file=annual_leave.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, drainBody(t, resp), "authority", "the 400 must list the active slug set")

	// tier=authority → the pre-existing happy path, unchanged.
	resp, err = http.Get("http://" + addr + "/api/knowledge/doc?tier=authority&file=annual_leave.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestDocContentHandlerDeactivated400 — a deactivated base disappears from
// the browsing surface exactly as it does from retrieval and skill
// validation: its docs 400 (naming the active set), they do not 404 or
// silently keep serving.
func TestDocContentHandlerDeactivated400(t *testing.T) {
	deps := &Deps{KBRegistry: kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: false},
	})}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=historical&file=2023-travel.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, drainBody(t, resp), "authority", "the 400 names the still-active set")
}

// TestDocContentHandler_ServesCorpusDoc — the happy path serves the full
// markdown source of a real corpus doc.
func TestDocContentHandler_ServesCorpusDoc(t *testing.T) {
	deps := &Deps{KBRegistry: testKBReg()}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=authority&file=annual_leave.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b := drainBody(t, resp)
	assert.Contains(t, b, `"tier":"authority"`)
	assert.Contains(t, b, `"file":"annual_leave.md"`)
	assert.Contains(t, b, "年假", "doc content must be served verbatim")
}

// TestDocContentHandler_MissingFile404 — a well-formed request for a
// nonexistent doc returns 404 (not a path-bearing 500, W-3 posture).
func TestDocContentHandler_MissingFile404(t *testing.T) {
	deps := &Deps{KBRegistry: testKBReg()}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=draft&file=nope.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
