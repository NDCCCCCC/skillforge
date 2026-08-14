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
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestDocsHandler_ListsTiersFromDisk — GET /api/knowledge/docs returns
// all 4 canonical tiers (empty tiers included) from the on-disk
// knowledge/ tree. The repo's real corpus (knowledge/authority/*.md) is
// the fixture; other tiers hold only .gitkeep and must render as empty.
func TestDocsHandler_ListsTiersFromDisk(t *testing.T) {
	deps := &Deps{}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/docs")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b := drainBody(t, resp)
	// All 4 tiers present; the authority tier carries the real corpus.
	for _, tier := range []string{"权威", "历史", "个人", "草稿"} {
		assert.Contains(t, b, `"tier":"`+tier+`"`, "tier %s must be listed", tier)
	}
	assert.Contains(t, b, "annual_leave.md", "authority corpus must list annual_leave.md")
	assert.NotContains(t, b, ".gitkeep", "dotfiles must be filtered out")
}

// TestDocContentHandler_PathTraversalRejected — the tier/file query
// params are concatenated into a filesystem path, so traversal attempts
// must 400 BEFORE any disk access: unknown tier, separator-bearing file,
// backslash-bearing file, and dot-dot prefixes.
func TestDocContentHandler_PathTraversalRejected(t *testing.T) {
	deps := &Deps{}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	cases := []struct {
		name string
		url  string
	}{
		{"unknown tier", "/api/knowledge/doc?tier=../cmd&file=main.go"},
		{"separator in file", "/api/knowledge/doc?tier=权威&file=../main.go"},
		{"backslash in file", "/api/knowledge/doc?tier=权威&file=..\\main.go"},
		{"empty file", "/api/knowledge/doc?tier=权威&file="},
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

// TestDocContentHandler_ServesCorpusDoc — the happy path serves the full
// markdown source of a real corpus doc.
func TestDocContentHandler_ServesCorpusDoc(t *testing.T) {
	deps := &Deps{}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=权威&file=annual_leave.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	b := drainBody(t, resp)
	assert.Contains(t, b, `"tier":"权威"`)
	assert.Contains(t, b, `"file":"annual_leave.md"`)
	assert.Contains(t, b, "年假", "doc content must be served verbatim")
}

// TestDocContentHandler_MissingFile404 — a well-formed request for a
// nonexistent doc returns 404 (not a path-bearing 500, W-3 posture).
func TestDocContentHandler_MissingFile404(t *testing.T) {
	deps := &Deps{}
	addr, cleanup := newKnowledgeTestHertz(t, deps)
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/api/knowledge/doc?tier=草稿&file=nope.md")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
