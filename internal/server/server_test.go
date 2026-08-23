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

// Phase 15 Plan 06 — server-level tests for the Hertz body-size options,
// the per-route Content-Length guard, and import endpoint wiring.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/skill"
)

// invokeServer performs one request against a server.New instance
// constructed with the supplied deps. It uses Hertz's in-memory test
// harness so no TCP port is required.
func invokeServer(t *testing.T, deps *Deps, method, path string, body *ut.Body, headers ...ut.Header) *ut.ResponseRecorder {
	t.Helper()
	h := New(":0", deps)
	return ut.PerformRequest(h.Engine, method, path, body, headers...)
}

// minimalChatDeps returns a Deps whose /api/chat handler can run to
// completion without real Milvus/BGE/GLM. The canned agent always refuses,
// which is fine — the tests only care about the guard status codes.
func minimalChatDeps(t *testing.T) *Deps {
	t.Helper()
	m := &skill.Manifest{}
	m.SetResolvedTiersForTest([]string{"权威"})
	ag, err := agent.NewAgent(nil, agent.AgentConfig{
		Manifest:  m,
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
		ChatModel: &cannedChatModel{},
		MaxHops:   1,
	})
	require.NoError(t, err)
	return &Deps{
		Agent:     ag,
		Store:     newTestStore(t),
		Loader:    skill.NewLoader("skills/"),
		Embedder:  newStubEmbedder(),
		Retriever: &stubRetriever{},
	}
}

// TestServerNew verifies New() wires the import routes and the global
// Hertz options (WithStreamBody + WithDisablePreParseMultipartForm +
// WithMaxRequestBodySize) without panicking. Import endpoints return 503
// when Deps.Importer is nil (fail-closed).
func TestServerNew(t *testing.T) {
	deps := minimalChatDeps(t)

	// Import routes exist and are fail-closed when no Importer is wired.
	t.Run("import preview 503 when importer nil", func(t *testing.T) {
		resp := invokeServer(t, deps, http.MethodPost, "/api/import/preview", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
		var body graphResponseError
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Contains(t, body.Error, "SKILLFORGE_DATA_SALT")
	})
	t.Run("import commit 503 when importer nil", func(t *testing.T) {
		resp := invokeServer(t, deps, http.MethodPost, "/api/import/commit", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})
	t.Run("import push 503 when importer nil", func(t *testing.T) {
		resp := invokeServer(t, deps, http.MethodPost, "/api/import/push", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})
	t.Run("import delete 503 when importer nil", func(t *testing.T) {
		resp := invokeServer(t, deps, http.MethodDelete, "/api/import/dataset/loans__2026-07", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})

	// A small non-import request passes the guard (it may be handled or
	// refused by the chat path, but it must not be 413).
	smallJSON := []byte(`{"query":"hello","skill":"compliance"}`)
	resp := invokeServer(t, deps, http.MethodPost, "/api/chat", &ut.Body{Body: bytes.NewReader(smallJSON), Len: len(smallJSON)})
	assert.NotEqual(t, http.StatusRequestEntityTooLarge, resp.Code, "/api/chat small body must not be rejected by size guard")
}

// TestNonImportBodyLimit verifies the per-route Content-Length guard:
// non-import routes with a declared body larger than NonImportMaxBodyBytes
// are rejected with 413, while import routes are allowed through (they are
// governed by the global WithMaxRequestBodySize limit instead).
func TestNonImportBodyLimit(t *testing.T) {
	deps := minimalChatDeps(t)

	smallJSON := []byte(`{"query":"hello","skill":"compliance"}`)
	largeLen := int(NonImportMaxBodyDefault) + 1

	t.Run("non-import over limit → 413", func(t *testing.T) {
		// Declare a body larger than the non-import limit but send no bytes.
		// The guard looks at Content-Length, not the actual body size.
		resp := invokeServer(t, deps, http.MethodPost, "/api/chat", &ut.Body{Body: bytes.NewReader(nil), Len: largeLen})
		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
		var body graphResponseError
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Contains(t, body.Error, "request body too large")
	})

	t.Run("non-import under limit → not 413", func(t *testing.T) {
		resp := invokeServer(t, deps, http.MethodPost, "/api/chat", &ut.Body{Body: bytes.NewReader(smallJSON), Len: len(smallJSON)})
		assert.NotEqual(t, http.StatusRequestEntityTooLarge, resp.Code)
	})

	t.Run("import route large declaration bypasses guard → 503", func(t *testing.T) {
		// Import routes must not be blocked by the non-import guard even when
		// the declared Content-Length is huge. The handler (importer nil)
		// returns 503 before any body processing.
		resp := invokeServer(t, deps, http.MethodPost, "/api/import/push", &ut.Body{Body: bytes.NewReader(nil), Len: largeLen})
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	})
}

// newStubEmbedder and stubRetriever are defined in testhelpers_test.go; they
// are reused here to keep the test file focused on the server wiring.
