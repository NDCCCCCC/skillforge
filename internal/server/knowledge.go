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
//	                         500 { error: "..." }                 (transient retriever failure)
//
// The 503 mirrors the wizard handler posture: when Deps.Knowledge is
// nil (a test that bypasses the graph layer, or a misconfigured server)
// the endpoint refuses the request rather than returning a misleading
// empty array (which the FE would mount as the empty-state hero — a
// valid signal only when the corpus is genuinely empty).

package server

import (
	"context"
	"net/http"

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
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, g)
}

// Compile-time check that knowledge.GraphBuilder satisfies the
// KnowledgeBuilder type alias declared in server.go.
var _ KnowledgeBuilder = (*knowledge.GraphBuilder)(nil)