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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/knowledge"
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

// KnowledgeBuilder is the graph-endpoint seam (Plan 05-05 / SC-5 /
// KNOW-06). The production implementation wraps a *knowledge.GraphBuilder
// (which in turn calls Retriever.ListMetadata). Tests substitute a
// stub that returns canned Graph fixtures so the handler tests do
// not need a live Milvus / Mongo backend.
//
// Build receives a context.Context so the implementation can honor
// the request's cancellation (the hertz chatHandler forwards the
// request's context into Build). The interface lives in this file
// (rather than internal/knowledge) to keep the knowledge package free
// of hertz/app imports — mirroring the Runner pattern.
//
// GetChunk (Plan 260813-dno / UAT #4) is the on-demand chunk-content
// fetch backing GET /api/knowledge/chunk/:id. The graph endpoint stays
// metadata-only (T-5-22); this returns the verbatim text + metadata
// when the operator clicks a graph chunk. Returns knowledge.ErrChunkNotFound
// (→ 404), knowledge.ErrNoRetriever (→ 500), or propagates other errors (→ 500).
type KnowledgeBuilder interface {
	Build(ctx context.Context) (*knowledge.Graph, error)
	GetChunk(ctx context.Context, id int64) (*knowledge.ChunkPayload, error)
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
	// Wizard is the Plan 05-04 SC-3 wizard (SYSK-06 — NL → SKILL.md).
	// When non-nil, POST /api/skills/wizard + POST /api/skills/wizard/commit
	// are enabled; when nil, the wizard handlers return 503. Production
	// wires it at cmd/server/main.go alongside Agent + Store.
	Wizard *skill.Wizard
	// Knowledge is the Plan 05-05 / SC-5 graph builder (KNOW-06 —
	// tier -> doc -> chunk tree for the FE knowledge browser). When
	// non-nil, GET /api/knowledge/graph is enabled; when nil, the
	// graph handler returns 503. Production wires it at cmd/server/main.go
	// alongside the Retriever.
	Knowledge KnowledgeBuilder
	// EvalReporter is the Plan 06-02 / SC-1 dashboard data source.
	// When non-nil, GET /api/eval/reports, /api/eval/reports/:ts,
	// and /api/eval/frozen are enabled; when nil, the handlers
	// return 503 (mirrors Knowledge / Wizard / Store nil-posture).
	// Production wires a *FilesystemEvalReporter at cmd/server/main.go
	// so the same eval/reports/ directory cmd/eval --archive writes
	// is what /api/eval/reports reads (single source of truth for
	// the trend history).
	EvalReporter EvalReporter
	// ChatModel is the Plan 05-06 / SC-7 redline LLM seam (D5-14). The
	// redline pass calls Generate directly (no tool loop, no ADK) with
	// a structured-output prompt, mirroring internal/agent.MaybeCritique.
	// nil = the redline pass is skipped (production always wires it via
	// the same model.ChatModel instance used to build Agent + prodRunner).
	// Tests substitute a stub ChatModel that counts Generate invocations
	// so TestRedline_Conditional can assert zero calls on a 合规 verdict
	// (D5-14 cost control — the load-bearing regression).
	ChatModel model.ToolCallingChatModel
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
// loaded Skills), POST /api/skills (dry-run Skill validator),
// POST /api/skills/wizard (SC-3 wizard interview → preview),
// POST /api/skills/wizard/commit (atomic write + fsnotify loop close),
// GET /api/knowledge/graph (SC-5 graph browser data source),
// GET /api/knowledge/chunk/:id (SC-5 on-demand chunk-content fetch),
// GET /api/eval/reports (SC-1 eval dashboard data),
// GET /api/eval/reports/:ts (SC-1 single eval report),
// GET /api/eval/frozen (D6-13 frozen-set metadata),
// GET /api/audit (SC-3 audit query + retention projection).
func New(addr string, deps *Deps) *server.Hertz {
	h := server.Default(server.WithHostPorts(addr))
	h.POST("/api/chat", deps.chatHandler)
	h.GET("/api/skills", deps.listSkillsHandler)
	h.POST("/api/skills", deps.validateSkillHandler)
	h.POST("/api/skills/wizard", deps.wizardHandler)
	h.POST("/api/skills/wizard/commit", deps.wizardCommitHandler)
	h.GET("/api/knowledge/graph", deps.graphHandler)
	h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)
	h.GET("/api/knowledge/docs", deps.docsHandler)
	h.GET("/api/knowledge/doc", deps.docContentHandler)
	h.GET("/api/eval/reports", deps.evalIndexHandler)
	h.GET("/api/eval/reports/:ts", deps.evalReportHandler)
	h.GET("/api/eval/frozen", deps.evalFrozenHandler)
	h.GET("/api/audit", deps.auditHandler)
	return h
}
