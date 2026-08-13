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

// Plan 05-05 / SC-5 (KNOW-06) — server-side tests for the
// GET /api/knowledge/graph endpoint.
//
// The handler test pins 4 acceptance assertions:
//   1. KnowledgeBuilder nil → 503 + {error:"..."} envelope.
//   2. KnowledgeBuilder.Build succeeds → 200 + the JSON tree.
//   3. KnowledgeBuilder.Build fails → 500 + {error:"knowledge graph
//      build failed — see server logs"} envelope. The full err.Error()
//      is logged at [WARN] server-side and MUST NOT leak to the FE (W-3).
//   4. Route registration: GET /api/knowledge/graph never returns 404
//      (mirror of the wizard handler route-registration regression).

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/knowledge"
)

// stubKnowledgeBuilder is the test double for KnowledgeBuilder. The
// real implementation lives at internal/knowledge.GraphBuilder; tests
// here use canned graphs so the handler logic can be exercised without
// a live Milvus / Mongo backend.
type stubKnowledgeBuilder struct {
	graph        *knowledge.Graph
	err          error
	calls        int
	chunkPayload *knowledge.ChunkPayload
	chunkErr     error
	getChunkCalls int
}

func (s *stubKnowledgeBuilder) Build(_ context.Context) (*knowledge.Graph, error) {
	s.calls++
	return s.graph, s.err
}

// GetChunk (Plan 260813-dno) — returns the configured chunkPayload /
// chunkErr. The chunkHandler tests below drive this via invokeChunkHandler.
func (s *stubKnowledgeBuilder) GetChunk(_ context.Context, _ int64) (*knowledge.ChunkPayload, error) {
	s.getChunkCalls++
	return s.chunkPayload, s.chunkErr
}

func TestGraphHandler_NilBuilderReturns503(t *testing.T) {
	deps := &Deps{Knowledge: nil}
	got := invokeGraphHandler(t, deps)
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "knowledge graph not initialized")
}

func TestGraphHandler_Success(t *testing.T) {
	g := &knowledge.Graph{Tiers: []knowledge.TierNode{
		{Tier: "权威", DocCount: 1, ChunkCount: 1, Docs: []knowledge.DocNode{
			{ID: "权威::a.md", Filename: "a.md", ChunkCount: 1, Chunks: []knowledge.ChunkNode{
				{ID: "1", EffectiveFrom: 0, EffectiveTo: 0, Version: ""},
			}},
		}},
		{Tier: "草稿", DocCount: 0, ChunkCount: 0, Docs: []knowledge.DocNode{}},
		{Tier: "历史", DocCount: 0, ChunkCount: 0, Docs: []knowledge.DocNode{}},
		{Tier: "个人", DocCount: 0, ChunkCount: 0, Docs: []knowledge.DocNode{}},
	}}
	stub := &stubKnowledgeBuilder{graph: g}
	deps := &Deps{Knowledge: stub}

	got := invokeGraphHandler(t, deps)
	assert.Equal(t, http.StatusOK, got.Code)

	// Verify the JSON body has the expected shape (4 tiers, 权威 has 1 doc).
	var decoded knowledge.Graph
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &decoded))
	require.Len(t, decoded.Tiers, 4)
	assert.Equal(t, "权威", decoded.Tiers[0].Tier)
	assert.Equal(t, 1, decoded.Tiers[0].DocCount)
	assert.Equal(t, 1, decoded.Tiers[0].Docs[0].ChunkCount)
	assert.Equal(t, "1", decoded.Tiers[0].Docs[0].Chunks[0].ID)
	assert.Equal(t, 1, stub.calls)
}

func TestGraphHandler_BuildErrorReturns500(t *testing.T) {
	stub := &stubKnowledgeBuilder{err: errors.New("milvus unavailable")}
	deps := &Deps{Knowledge: stub}

	got := invokeGraphHandler(t, deps)
	assert.Equal(t, http.StatusInternalServerError, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	// W-3: the 500 envelope MUST NOT echo err.Error() to the client —
	// internal errors (ErrInvalidTier / ErrEmptySource) embed numeric row
	// IDs and source-path taxonomy that an attacker could probe if the
	// endpoint were exposed. The full error is logged at [WARN]
	// server-side; the FE gets a generic message.
	assert.Contains(t, body.Error, "knowledge graph build failed",
		"the 500 envelope must carry the generic build-failed message")
	assert.Contains(t, body.Error, "see server logs",
		"the operator is directed to the server logs for the full error")
	assert.NotContains(t, body.Error, "milvus",
		"the 500 envelope MUST NOT leak internal error detail (W-3)")
}

func TestGraphHandler_RouteRegistered(t *testing.T) {
	// Regression — must NEVER return 404. The route-registration
	// smoke test exercises the path through server.New so any future
	// refactor that drops /api/knowledge/graph from the route table
	// fails here, not at the first FE request.
	deps := &Deps{Knowledge: &stubKnowledgeBuilder{graph: &knowledge.Graph{}}}
	h := New(":0", deps)
	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/knowledge/graph", nil)
	assert.NotEqual(t, http.StatusNotFound, w.Code, "GET /api/knowledge/graph must be registered (got 404)")
	assert.Equal(t, http.StatusOK, w.Code)
}

// invokeGraphHandler runs the GET /api/knowledge/graph handler in an
// isolated hertz instance and returns the recorder. Mirrors the
// pattern in skills_wizard_test.go.
func invokeGraphHandler(t *testing.T, deps *Deps) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/knowledge/graph", deps.graphHandler)
	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/knowledge/graph", nil)
	return w
}

// Ensure stub satisfies the interface at compile time (defense-in-depth).
var _ KnowledgeBuilder = (*stubKnowledgeBuilder)(nil)

// Plan 260813-dno / UAT #4 — chunkHandler tests (GET /api/knowledge/chunk/:id).
//
// Mirrors the graphHandler test posture: nil builder → 503, bad id → 400,
// found → 200 + JSON round-trip, not-found → 404, other-err → 500 generic
// (W-3: no err.Error() leak), route registered → never 404.

func TestChunkHandler_NilBuilderReturns503(t *testing.T) {
	deps := &Deps{Knowledge: nil}
	got := invokeChunkHandler(t, deps, "1")
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "knowledge graph not initialized")
}

func TestChunkHandler_BadIDReturns400(t *testing.T) {
	stub := &stubKnowledgeBuilder{chunkPayload: &knowledge.ChunkPayload{}}
	deps := &Deps{Knowledge: stub}
	got := invokeChunkHandler(t, deps, "abc")
	assert.Equal(t, http.StatusBadRequest, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Equal(t, "invalid chunk id", body.Error)
	assert.Equal(t, 0, stub.getChunkCalls, "GetChunk must NOT be invoked on a bad id")
}

func TestChunkHandler_Found(t *testing.T) {
	payload := &knowledge.ChunkPayload{
		ID:            "1",
		Tier:          "权威",
		Source:        "authority/a.md",
		Content:       "正文片段",
		EffectiveFrom: 0,
		EffectiveTo:   0,
		Version:       "",
	}
	stub := &stubKnowledgeBuilder{chunkPayload: payload}
	deps := &Deps{Knowledge: stub}

	got := invokeChunkHandler(t, deps, "1")
	assert.Equal(t, http.StatusOK, got.Code)

	// JSON round-trip — every field survives the wire.
	var decoded knowledge.ChunkPayload
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &decoded))
	assert.Equal(t, "1", decoded.ID)
	assert.Equal(t, "权威", decoded.Tier)
	assert.Equal(t, "authority/a.md", decoded.Source)
	assert.Equal(t, "正文片段", decoded.Content)
	assert.Equal(t, int64(0), decoded.EffectiveFrom)
	assert.Equal(t, int64(0), decoded.EffectiveTo)
	assert.Equal(t, "", decoded.Version)
	assert.Equal(t, 1, stub.getChunkCalls, "GetChunk invoked exactly once")
}

func TestChunkHandler_NotFoundReturns404(t *testing.T) {
	stub := &stubKnowledgeBuilder{chunkErr: knowledge.ErrChunkNotFound}
	deps := &Deps{Knowledge: stub}

	got := invokeChunkHandler(t, deps, "999")
	assert.Equal(t, http.StatusNotFound, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Equal(t, "chunk not found", body.Error)
	assert.Equal(t, 1, stub.getChunkCalls)
}

func TestChunkHandler_BuildErrorReturns500(t *testing.T) {
	stub := &stubKnowledgeBuilder{chunkErr: errors.New("boom")}
	deps := &Deps{Knowledge: stub}

	got := invokeChunkHandler(t, deps, "1")
	assert.Equal(t, http.StatusInternalServerError, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	// W-3: generic message — the 500 envelope MUST NOT echo err.Error().
	assert.Contains(t, body.Error, "knowledge chunk fetch failed",
		"the 500 envelope must carry the generic chunk-failed message")
	assert.Contains(t, body.Error, "see server logs")
	assert.NotContains(t, body.Error, "boom",
		"the 500 envelope MUST NOT leak internal error detail (W-3)")
}

func TestChunkHandler_RouteRegistered(t *testing.T) {
	// Regression — the route must NEVER 404. Mirrors
	// TestGraphHandler_RouteRegistered so a future refactor that drops
	// /api/knowledge/chunk/:id from the route table fails here, not at
	// the first FE request.
	deps := &Deps{Knowledge: &stubKnowledgeBuilder{
		chunkPayload: &knowledge.ChunkPayload{ID: "1"},
	}}
	h := New(":0", deps)
	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/knowledge/chunk/1", nil)
	assert.NotEqual(t, http.StatusNotFound, w.Code, "GET /api/knowledge/chunk/:id must be registered (got 404)")
	assert.Equal(t, http.StatusOK, w.Code)
}

// invokeChunkHandler runs the GET /api/knowledge/chunk/:id handler in
// an isolated hertz instance and returns the recorder. idPath is the
// raw path segment (e.g. "1" or "abc"). Mirrors invokeGraphHandler.
func invokeChunkHandler(t *testing.T, deps *Deps, idPath string) *ut.ResponseRecorder {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)
	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/knowledge/chunk/"+idPath, nil)
	return w
}

// Sanity-check the wire envelope (defensive — guards against a future
// refactor that drops the Error field).
func TestGraphResponseError_ErrorField(t *testing.T) {
	body, err := json.Marshal(graphResponseError{Error: "boom"})
	require.NoError(t, err)
	assert.Contains(t, string(body), `"error":"boom"`)
	// Sanity: the error envelope shape MUST match the wizard handler.
	// (Visual cross-check — both error structs are single-field.)
	if !strings.Contains(string(body), "error") {
		t.Fatalf("missing error key in envelope: %s", string(body))
	}
}

// Force the bytes import to remain in use across future refactors.
var _ = bytes.NewBufferString
