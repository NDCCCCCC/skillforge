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
// the graduated internal/{verdict,wire,retriever,embedder,skill,knowledge}
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
package server

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
)

// Embedder aliases the embedder.Embedder interface (the production
// BGE-M3 interface the pipeline consumes). The implementation lives in
// cmd/server/main.go (a thin wrapper around the BGE-M3 HTTP client
// via internal/embedder/bge). Tests substitute a stub that returns
// a constant vector regardless of input text.
type Embedder = embedder.Embedder

// Retriever aliases the retriever.Retriever interface. The
// implementation in cmd/server/main.go wraps a *milvus.MilvusClient.
// Tests substitute a stub that returns canned Hit slices — without
// ever touching Milvus.
//
// Search takes the resolved tier VALUES (not a pre-built filter string)
// so the implementation owns the filter-building (a Milvus-specific
// concern, kept inside the milvus subpackage).
type Retriever = retriever.Retriever

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
//
// Phase 3 wiring contract for Agent + Store (Plan 03-05):
//
//   - **Agent** supersedes Runner for the /api/chat path in Phase 3; the
//     chatHandler branches on `s.Agent != nil`. Runner is retained for
//     the eval golden-set runner (Plan 03-06) and as a fallback so the
//     Phase 2 acceptance tests (which construct a Deps with only Runner
//     set) stay green during the migration. cmd/server constructs BOTH.
//
//   - **Store** is the audit + session-memory backend (D3-09). The
//     chatHandler writes one audit row + one (user, assistant) turn
//     pair per /api/chat response before returning (GEVL-03 / AGNT-05).
//     Tests may leave Store nil to exercise a degraded mode that streams
//     a verdict/refusal without persisting it.
//
//   - **Decision-emit wiring (closure capture, NOT an adapter type):**
//     internal/agent owns `type DecisionEmitFunc func(step int, decision,
//     toolCallID, query string, confidence float64)` (Plan 03-03,
//     hertz-free). chat.go owns `func emitDecision(c *app.RequestContext,
//     p wire.DecisionPayload) error` (writes the data-decision SSE chunk).
//     Per request, the chatHandler wraps emitDecision into the agent's
//     DecisionEmitFunc via closure capture so the per-request `c` is
//     available inside the callback goroutine. We do NOT introduce an
//     EmitDecisionFunc adapter type taking *app.RequestContext in this
//     file (would couple Deps to Hertz), and we do NOT bind Emit at
//     main.go construction time (the per-request `c` is unavailable
//     there). This keeps internal/agent free of hertz/app (D-04/D15
//     wire boundary).
type Deps struct {
	Loader    *skill.Loader
	Embedder  Embedder
	Retriever Retriever
	Runner    Runner
	// Agent is the Phase 3 ReAct agent (Plan 03-03). nil = Phase 2
	// fallback to Runner via VerdictPipeline (preserves existing tests).
	Agent *agent.Agent
	// Store persists audit rows + session turns (Plan 03-02). nil =
	// degraded mode (responses still stream, nothing persisted).
	Store store.Store
	// Router is the Plan 05-02 Skill Router (D5-01). When non-nil and
	// Registry is non-nil, chatHandler routes via the registry snapshot
	// BEFORE VerdictPipeline. When nil (existing tests), chatHandler
	// falls back to the Phase 2 Loader-based findSkill path.
	Router *skill.Router
	// Registry is the Plan 05-01 Skill registry (atomic.Pointer snapshot
	// of validated Skills + their cached description embeddings).
	// Required by Router; nil + Router non-nil = chatHandler errors out
	// at startup (the Router needs a snapshot to route from).
	Registry *skill.Registry
}

// New constructs a Hertz server bound to addr with the chat route
// registered. Returns the *server.Hertz so cmd/server/main.go can call
// Spin() (Pattern 1 from 02-RESEARCH.md — verified against
// hertz@v0.10.6/pkg/app/server/option.go:158 WithHostPorts).
//
// The chat handler closure captures deps so tests can construct a
// Server with mock deps without touching real Milvus/BGE-M3/GLM.
//
// Routes: POST /api/chat (verdict pipeline), GET /api/skills (list
// loaded Skills), POST /api/skills (dry-run Skill validator).
func New(addr string, deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(addr))
	h.POST("/api/chat", deps.chatHandler)
	h.GET("/api/skills", deps.listSkillsHandler)
	h.POST("/api/skills", deps.validateSkillHandler)
	return h
}