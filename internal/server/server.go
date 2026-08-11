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

// Package server is the SkillForge Phase 2 Hertz HTTP surface: one
// long-running binary (cmd/server) that exposes POST /api/chat against
// the graduated internal/{verdict,wire,retriever,skill,knowledge}
// packages.
//
// This file holds the Deps dependency-injection container + the New()
// constructor (Pattern 1 from 02-RESEARCH.md). The chat handler lives
// in chat.go; the verdict pipeline orchestration lives in pipeline.go.
//
// The Deps struct is the single integration point for the whole server.
// Each pipeline step (embed → retrieve → prompt → LLM → ground → emit)
// is reachable through one of the fields. Tests construct their own
// Deps with mock implementations of every interface to exercise the
// refusal + grounding + tier-filter behavior without standing up
// Milvus, BGE-M3, or GLM.
//
// Plan 02-04 will add the GET/POST /api/skills routes to this same
// Deps + New(). For now the Server exposes only POST /api/chat.
package server

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// Embedder is the production BGE-M3 interface the pipeline consumes.
// The implementation lives in cmd/server/main.go (a thin wrapper around
// retriever.EmbedQuery). Tests substitute a stub that returns a
// constant vector regardless of input text.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// Searcher is the retriever seam the pipeline consumes. The
// implementation in cmd/server/main.go wraps retriever.SearchWithTierFilter
// with a PRE-built filter expression derived from the Skill's
// resolved tiers. Tests substitute a stub that returns canned
// TierSearchResult slices — without ever touching Milvus.
//
// SearchWithFilter takes the EXACT filter string the pipeline built
// (not the tier values), so tests can assert on the precise filter
// expression — the trust boundary is auditable end-to-end.
type Searcher interface {
	SearchWithFilter(ctx context.Context, queryVec []float32, filter string, topK int) ([]retriever.TierSearchResult, error)
}

// Runner is the LLM-call seam. The production implementation wraps
// adk.NewRunner(ChatModelAgent) and emits a temperature=skill.Temperature
// + json_object + thinking:disabled call per spike 003 (proven in
// .claude/skills/spike-findings-ai-agent). Tests substitute a mock
// that captures the options slice (TestTemperaturePassthrough) and
// returns a canned AgentEvent iterator with a known JSON verdict.
//
// CallCount supports the refusal-by-construction assertion
// (TestNoChunksRefusal, TestLowScoreRefusal — the runner must NOT
// have been invoked when the pipeline short-circuits).
type Runner interface {
	Query(ctx context.Context, query string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]
	CallCount() int
}

// Deps bundles every dependency the chat pipeline needs. cmd/server's
// main() builds a Deps with production clients; tests build a Deps
// with mocks. The Server does not import the production clients
// directly — every external dependency is reached via Deps.
//
// Loader may be nil in unit tests that bypass the skill-scope check
// (TestChatHandlerSSE constructs a Deps with Loader=nil + a pre-built
// Manifest passed directly to VerdictPipeline). In production cmd/server
// Loader is always non-nil — the startup hook Scan()s skills/compliance/.
type Deps struct {
	Loader    *skill.Loader
	Milvus    client.Client
	MilvusCfg retriever.MilvusConfig
	BGEURL    string
	Embedder  Embedder
	Searcher  Searcher
	Runner    Runner
}

// New constructs a Hertz server bound to addr with the chat route
// registered. Returns the *server.Hertz so cmd/server/main.go can call
// Spin() (Pattern 1 from 02-RESEARCH.md — verified against
// hertz@v0.10.6/pkg/app/server/option.go:158 WithHostPorts).
//
// The chat handler closure captures deps so tests can construct a
// Server with mock deps without touching real Milvus/BGE-M3/GLM.
//
// Plan 02-04 will extend this constructor with GET /api/skills +
// POST /api/skills (the dry-run validator). For now the route table
// is single-row.
func New(addr string, deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(addr))
	h.POST("/api/chat", deps.chatHandler)
	return h
}