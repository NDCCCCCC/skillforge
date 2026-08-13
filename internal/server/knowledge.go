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

// Plan 05-05 / SC-5 (KNOW-06) — GET /api/knowledge/graph endpoint.
//
// The graph endpoint exposes the metadata-only tier -> doc -> chunk tree
// to the FE knowledge browser. Implementation lives behind a dedicated
// /api/knowledge route group (separate from /api/skills) so future
// knowledge-related endpoints (e.g. /api/knowledge/chunks/{id} for the
// future on-demand chunk-content fetch) can register alongside without
// crowding the skills surface.
//
// Wire contract:
//
//	GET /api/knowledge/graph → 200 { tiers: [{ tier, docCount, chunkCount, docs: [...] }] }
//	                         503 { error: "knowledge graph not initialized" }
//	                         500 { error: "knowledge graph build failed — see server logs" }
//
// The 503 mirrors the wizard handler posture: when Deps.Knowledge is
// nil (a test that bypasses the graph layer, or a misconfigured server)
// the endpoint refuses the request rather than returning a misleading
// empty array (which the FE would mount as the empty-state hero — a
// valid signal only when the corpus is genuinely empty).
//
// The 500 envelope deliberately carries a generic message rather than
// err.Error(): ErrInvalidTier / ErrEmptySource embed internal metadata
// (numeric row IDs, source-path taxonomy) that an attacker could probe
// via hand-edited retriever fixtures if the endpoint is exposed. The
// full error is logged at [WARN] server-side (W-3).

package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/knowledge"
)

// graphResponseError is the {error: "..."} envelope returned with 4xx /
// 5xx responses on /api/knowledge/*. Mirrors the wizard + commit
// envelope so the FE error-handling path is uniform.
type graphResponseError struct {
	Error string `json:"error"`
}

// graphHandler implements GET /api/knowledge/graph. The Deps.Knowledge
// field holds the GraphBuilder. When nil (test mode or unconfigured
// production), the handler returns 503 with the standard envelope.
// On success it returns 200 + the Graph JSON the FE renders as a 3-level
// Accordion tree.
//
// Plan 05-05 / T-5-22 mitigation: the Graph JSON carries metadata only
// (id / tier / source / effective_*). Verbatim chunk text opens
// separately via ChunkSourceView — the graph endpoint deliberately
// does NOT carry content into the browser.
func (s *Deps) graphHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized (set Deps.Knowledge at server construction)",
		})
		return
	}
	g, err := s.Knowledge.Build(ctx)
	if err != nil {
		// W-3 (Phase 5 review): log the full err.Error() server-side at
		// WARN — ErrInvalidTier / ErrEmptySource embed internal metadata
		// (numeric row IDs, source-path taxonomy) that an attacker could
		// probe via hand-edited retriever fixtures if /api/knowledge/graph
		// is exposed. Return a generic message to the FE; the operator
		// finds the full error in the server logs. Mirrors the [WARN]
		// convention used in pipeline.go for non-fatal build failures.
		log.Printf("[WARN] server: knowledge graph build failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "knowledge graph build failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, g)
}

// chunkHandler implements GET /api/knowledge/chunk/:id. Plan 260813-dno
// / UAT #4 — the on-demand chunk-content fetch. The graph endpoint stays
// metadata-only (T-5-22); this handler returns the verbatim chunk text
// + metadata when the operator clicks a graph chunk (the FE fires it
// only on a fixture-miss in chunk-lookup, so the verdict-citation path
// is untouched).
//
// Posture mirrors graphHandler exactly so the FE error handling is
// uniform across /api/knowledge/*:
//
//	GET /api/knowledge/chunk/:id → 200 { id, tier, source, content,
//	                                  effectiveFrom, effectiveTo, version }
//	                             → 503 { error: "knowledge graph not initialized..." }  (nil Knowledge)
//	                             → 400 { error: "invalid chunk id" }                    (non-int64 path)
//	                             → 404 { error: "chunk not found" }                     (ErrChunkNotFound)
//	                             → 500 { error: "knowledge chunk fetch failed — see server logs" }  (other err, W-3)
//
// W-3 (Phase 5 review, mirrored from graphHandler): the 500 envelope
// carries a generic message — NEVER err.Error(). The full error is
// logged at [WARN] server-side so the operator can diagnose without the
// FE leaking internal detail to a potential attacker.
func (s *Deps) chunkHandler(ctx context.Context, c *app.RequestContext) {
	if s.Knowledge == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "knowledge graph not initialized (set Deps.Knowledge at server construction)",
		})
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "invalid chunk id",
		})
		return
	}
	payload, err := s.Knowledge.GetChunk(ctx, id)
	if errors.Is(err, knowledge.ErrChunkNotFound) {
		c.JSON(http.StatusNotFound, graphResponseError{
			Error: "chunk not found",
		})
		return
	}
	if err != nil {
		// W-3: log the full err.Error() server-side at WARN; return a
		// generic message to the FE. Mirrors graphHandler's [WARN]
		// convention so a future refactor stays consistent.
		log.Printf("[WARN] server: knowledge chunk fetch failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "knowledge chunk fetch failed — see server logs",
		})
		return
	}
	c.JSON(http.StatusOK, payload)
}

// Compile-time check that knowledge.GraphBuilder satisfies the
// KnowledgeBuilder type alias declared in server.go.
var _ KnowledgeBuilder = (*knowledge.GraphBuilder)(nil)
