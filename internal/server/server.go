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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/data"
	"github.com/skillforge/skillforge/internal/document"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/externalauth"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/knowledge/ecl"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/scheduler"
	"github.com/skillforge/skillforge/internal/server/classifier"
	"github.com/skillforge/skillforge/internal/server/platform/mcp"
	"github.com/skillforge/skillforge/internal/server/rewrite"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/ticket"
)

// WR-05 / T-30-06: per-session rate limiting constants.
const (
	// feedbackRateLimitRequests is the maximum feedback requests per session per minute.
	feedbackRateLimitRequests = 10
	// feedbackRateLimitBurst is the maximum burst size for the per-session rate limiter.
	feedbackRateLimitBurst = 5
)

// perSessionFeedbackLimiter manages a rate limiter per session for feedback/ticket
// endpoints. Thread-safe via sync.Map.
type perSessionFeedbackLimiter struct {
	limiters sync.Map
	limit    rate.Limit
	burst    int
}

func NewPerSessionFeedbackLimiter(limit rate.Limit, burst int) *perSessionFeedbackLimiter {
	return &perSessionFeedbackLimiter{limit: limit, burst: burst}
}

// Allow returns true if the session is within its rate limit.
func (m *perSessionFeedbackLimiter) Allow(sessionID string) bool {
	lim, _ := m.limiters.LoadOrStore(sessionID, rate.NewLimiter(m.limit, m.burst))
	return lim.(*rate.Limiter).Allow()
}

// Default body-size limits. Import endpoints need a much larger envelope than
// chat routes because they carry raw CSV/SQLite-dump payloads; non-import routes
// keep a tight 8 MiB ceiling so the large import limit cannot be repurposed to
// flood /api/chat. Source-grep locked (scripts/verify-phase-15.sh Stage 9).
const (
	// ImportMaxBodyDefault is 2 GiB (2147483648 bytes).
	ImportMaxBodyDefault int64 = 2147483648
	// NonImportMaxBodyDefault is 8 MiB (8388608 bytes).
	NonImportMaxBodyDefault int64 = 8388608
)

// reportPathAllowlist rejects directory-traversal attacks in GET /reports/* paths.
// The regex allows only [period]/v<N>.html — any ../ or %2e%2e is constructively
// rejected because the slash-separated segments are matched one at a time against
// the allowlist pattern (^[A-Za-z0-9_-]+/v[0-9]+\.html$), so there is no
// way to construct a match for a path containing ".." or encoded traversal.
var reportPathAllowlist = regexp.MustCompile(`^[A-Za-z0-9_-]+/v[0-9]+\.html$`)

// reportsRoute serves published static HTML reports from the reports/ directory
// on GET /reports/*filepath. Directory-traversal is blocked by the allowlist regex.
//
// WR-10 (18-REVIEW): defense-in-depth headers on the served HTML —
// bluemonday containment is good but a CSP (no default sources, only
// inline styles) + Cache-Control: no-cache (a published vN is mutable
// — a re-publish replaces the file, the FE must NOT cache the body)
// keep operator-facing artifacts from being reused as script-execution
// surfaces (the X-Content-Type-Options: nosniff block is implied by the
// strict text/html content type already).
func reportsRoute(c context.Context, ctx *app.RequestContext) {
	p := string(ctx.Path())
	rel := strings.TrimPrefix(p, "/reports/")
	if !reportPathAllowlist.MatchString(rel) {
		ctx.String(http.StatusNotFound, "not found")
		return
	}
	file := filepath.Join("reports", filepath.FromSlash(rel))
	b, err := os.ReadFile(file)
	if err != nil {
		ctx.String(http.StatusNotFound, "not found")
		return
	}
	ctx.Response.Header.SetBytesV("Content-Security-Policy", []byte("default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'"))
	ctx.Response.Header.SetBytesV("Cache-Control", []byte("no-cache"))
	ctx.Response.Header.SetBytesV("X-Content-Type-Options", []byte("nosniff"))
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

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
	// PipelineChatModel is the Plan 260815-cos streaming seam. When
	// non-nil, VerdictPipeline Step 8 calls Stream on this model and
	// emits text-delta-draft / text-delta-draft-end frames per token
	// BEFORE the parsed verdict lands on the wire (FE renders 🟡 待校验
	// → ✅ 已验证 on the DraftStreamChip).
	//
	// Independent field from ChatModel — same model instance in
	// production (model.ToolCallingChatModel embeds model.BaseChatModel
	// so Stream is reachable through either type), but keeping them
	// separate means the streaming verdict path can be tested in
	// isolation from the redline path (D5-14 cost-control invariant
	// on the redline seam must NOT be affected by this draft channel).
	// nil = Step 8 falls back to Runner.Query + drainFinalContent
	// (the Phase 2 byte-identical path). cmd/server/main.go wires cm
	// to both fields at startup.
	PipelineChatModel model.BaseChatModel

	// Rewriter is the Phase 10 query-rewriter wrapper (Plan 10-02).
	// When non-nil AND history is non-empty, VerdictPipeline calls
	// r.Rewrite(ctx, historyMsgs, query) before retrieve; the rewritten
	// query drives the embedding + classifier input. The rewriter is
	// best-effort — timeout / parse failure falls back to the original
	// query (D10-C1). nil = the rewrite step is SKIPPED (history prefix
	// STILL flows when history is non-empty — the nil-gate disables
	// ONLY the rewriter step, NOT the history projection).
	//
	// cmd/server/main.go wires a *rewrite.QueryRewriter at startup.
	// Tests may leave this nil to exercise the no-rewriter path.
	Rewriter *rewrite.QueryRewriter

	// RefusalClassifier is the Phase 10 query-only refusal-classifier
	// wrapper (Plan 10-02). When non-nil, VerdictPipeline Step 7 calls
	// c.Classify(ctx, RefusalClassifierInput{Query, Chunks}) before the
	// verdict LLM; Decision="refuse" short-circuits to a refusal emit
	// (D10-A1 / D10-A2 escape from the P9 -95pp drift finding).
	// nil = the semantic refusal layer is SKIPPED; the code-level
	// short-circuits in Step 5/6 (0-chunks / score<0.3) still refuse.
	//
	// cmd/server/main.go wires a *classifier.RefusalClassifier at
	// startup. Tests may leave this nil to exercise the classifier-less
	// path (matches the Phase 9 baseline).
	//
	// D10-A1 — the classifier's input struct has no History field. The
	// type-level isolation is enforced by the classifier package's own
	// test (TestRefusalClassifierInput_NoHistoryField) and the GG-A1-1
	// CI grep gate.
	RefusalClassifier *classifier.RefusalClassifier

	// AuditWriter is the Phase 11 / Plan 11-03 fail-closed HITL audit
	// write path (D11-C3). The approval endpoint (Plan 11-04,
	// approval_api.go) consumes it for MarkApproved / MarkRejected.
	// New() initializes it from Store when nil; tests may inject a
	// writer with a deterministic clock via SetNow.
	AuditWriter *hitl.AuditWriter

	// ActiveManifest is the Phase 11 approval-gate Skill context: the
	// manifest the approval endpoint's D11-C2 expire-check compares
	// each pending row's (skill_version, prompt_hash) snapshot against
	// (HITL-03 cross-refresh survival). Production wires it alongside
	// the chat path's Skill selection; nil → the approval endpoints
	// answer 404 (defensive misconfiguration) and the GET renders
	// every pending row as state="expired" (fail-closed: freshness
	// cannot be proven).
	ActiveManifest *skill.Manifest

	// Importer is the Phase 15 three-channel import orchestrator
	// (Preview / Commit / Push / DryRun). nil → import endpoints return
	// 503, mirroring the SKILLFORGE_DATA_SALT unset fail-closed posture.
	Importer *data.Importer
	// ImportMaxBodyBytes is the per-request body limit for /api/import/*
	// routes. Zero falls back to ImportMaxBodyDefault.
	ImportMaxBodyBytes int64
	// NonImportMaxBodyBytes is the per-request declared body limit for
	// non-import routes. Content-Length headers larger than this are
	// rejected with 413 before the handler runs. Zero falls back to
	// NonImportMaxBodyDefault.
	NonImportMaxBodyBytes int64

	// KPIExecutor is the Phase 16 KPI execution seam (D16-C3). It rides
	// into agent.BuildRunner so a data-channel Skill's kpi_call Tool has
	// a call target. nil → data-channel Skills fail closed at
	// BuildRunner (never half-run) and GET /api/admin/kpis answers 503,
	// mirroring the SKILLFORGE_DATA_SALT unset posture.
	KPIExecutor *kpi.Executor
	// KPIRegistry is the immutable KPI catalog compiled at startup
	// (D16-A4). Backs both the kpi_call Tool description and the
	// read-only GET /api/admin/kpis surface. Same nil contract as
	// KPIExecutor above.
	KPIRegistry *kpi.Registry
	// Scheduler is the Phase 19 cron-driven report regeneration scheduler
	// (PORT-02). Used by the rerender endpoint to access EvalGate.
	// nil = scheduler not wired, rerender returns 503.
	Scheduler *scheduler.Scheduler
	// Detacher is the DuckDB AttachLRU for the dataset lifecycle (Phase 19 / DATA-18).
	// Used by the scheduler's TTL cleanup to DETACH before deleting dataset files.
	// nil = TTL cleanup is a no-op (detacher is nil-safe in RunTTLCleanup).
	Detacher interface {
		Detach(ctx context.Context, id string) error
	}

	// ExternalAuthManager is Phase 21 — the External Authorized
	// Channel orchestrator (DATA-04..14). nil = the channel is
	// disabled; the six /api/external-auth/* routes bound below all
	// answer 503. cmd/server main.go (section 6c-5) assigns this field
	// only when EXTERNAL_AUTH_ENABLED=1; when wired, the five
	// sensitive surfaces (pending/approve/reject/execute/reconcile)
	// additionally require X-Admin-Token.
	ExternalAuthManager *externalauth.Manager

	// KBRegistry is Phase 26 — the knowledge-base registry seeded from
	// the controlled file + sqlite runtime state. It is the reads
	// contract every skill validates against (KBM-04), the source of
	// the knowledge graph node set, and (26-05) the admin
	// create/deactivate surface. nil degrades the KB surfaces to 503
	// fail-closed — the same posture as a nil KPIRegistry, because a
	// skill whose reads cannot be checked against the live base set
	// must not pass validation silently.
	KBRegistry kb.Registry

	// --- Phase 27: Document Upload Pipeline ---

	// IndexerBackend is the narrow insert-only interface for the upload pipeline.
	// Only Insert is needed — the query path goes through retriever.Retriever.
	// Wired at boot: a MilvusIndexerBackend or MongoIndexerBackend.
	IndexerBackend retriever.IndexerBackend

	// ParserRegistry is the document format parser registry (DOC-01).
	// Used by the upload handler to verify magic bytes before parsing.
	ParserRegistry *document.ParserRegistry

	// UploadMaxBodyBytes is the per-route body-size limit for POST /api/admin/kb/:slug/upload.
	// Default: 100 MiB. Set via SKILLFORGE_UPLOAD_MAX_BYTES env var at boot.
	// Zero falls back to the default.
	UploadMaxBodyBytes int64

	// --- Phase 28: Knowledge Graph Limits ---
	// GraphLimits holds the config-driven limits for the knowledge graph.
	// Zero values fall back to hardcoded defaults in the handlers
	// (fail-closed: non-positive values are caught at startup in main.go).
	GraphLimits GraphLimits

	// --- Phase 29: Feedback Loop / Ticket Interface ---

	// TicketProvider is the outbound ticket dispatcher (TKT-01 / Plan
	// 29-06). nil = the server started without a ticket provider: POST
	// /api/tickets answers 503 with ticket.ProviderNotWiredError and no
	// ticket row is ever persisted. The "none" provider IS a wired
	// provider — store-only, zero outbound traffic — and never surfaces
	// that 503. cmd/server wires it from SKILLFORGE_TICKET_PROVIDER.
	TicketProvider ticket.Provider
	// TicketProviderName is the configured provider name ("none" |
	// "webhook"). It decides the insert-time dispatch_status the ticket
	// row carries: "webhook" → "queued" (handed to the background
	// dispatcher), anything else → "none" (store-only receipt). The row is
	// never updated afterwards (INSERT-only; the settled outcome lands in
	// the action_ticket_dispatch_* audit trail instead).
	TicketProviderName string
	// TicketDispatchBudget is the WHOLE-dispatch wall-clock budget for one
	// background dispatch goroutine (T-29-07: the provider's bounded
	// retries run inside this bounded budget). Zero falls back to
	// defaultTicketDispatchBudget. cmd/server wires it from
	// SKILLFORGE_TICKET_DISPATCH_BUDGET_MS.
	TicketDispatchBudget time.Duration

	// --- Phase 30 / WR-05 (T-30-06): per-session rate limiting ---
	// FeedbackRateLimiter bounds feedback/ticket requests per session to prevent
	// unbounded webhook relay abuse. nil = rate limiting disabled (not recommended
	// for webhook-enabled deployments).
	FeedbackRateLimiter *perSessionFeedbackLimiter

	// --- Phase 30 / Admin Token (orchestration visibility) ---
	// AdminToken is the configured SKILLFORGE_ADMIN_TOKEN value read at startup.
	// Plumbed through Deps for discoverability and consistency with other
	// SKILLFORGE_* fields. The actual admin-gate check (isAdminRequest in
	// audit.go) still reads via os.Getenv directly for fail-closed semantics:
	// an empty env means isAdminRequest returns false regardless of this field.
	AdminToken string

	// --- Phase 38: Ontology Layer (v1.4 / ONT-01..09) ---
	// Ontology bundles the TBox emitter output (already persisted by boot),
	// the memory-map KG store, and the schema-induction manager. nil = the
	// server started without the ontology layer: every /api/admin/ontology/*
	// route answers 503 (same nil-posture as Knowledge / Wizard). cmd/server
	// wires it at boot: emit TBox → load KG → snapshot → debounce (5s).
	Ontology *OntologyDeps

	// --- Phase 39: ECL Pipeline (v1.4 / ONT-13) ---
	// ECL carries the section-aware pipeline manager constructed at boot.
	// nil = ECL disabled (SKILLFORGE_ECL_ENABLED=off|false|0) or the active
	// retrieval backend cannot support it (mongo/none — ECL is Milvus-
	// schema-bound per D-Milvus-01): every /api/admin/pipeline/* route
	// answers 503 (same nil-posture as Ontology). Boot ARMS the manager but
	// NEVER runs it (D-Run-01) — runs come from the admin run endpoint only.
	ECL *ecl.Manager

	// --- Phase 41: Platform Layer MCP server (PLAT-ONT-18) ---
	// MCPServer is the stdio MCP server exposing KG tools to external agents.
	// It is constructed in cmd/server/main.go at boot and nil if MCP is disabled
	// (SKILLFORGE_MCP_ENABLED=false|0|off). Admin API handlers use it to surface
	// MCP tool status and invocation to the inspector UI panel.
	MCPServer *mcp.Server
}

// GraphLimits holds the config-driven limits for the knowledge graph.
// Zero values fall back to hardcoded defaults in the handlers.
// Non-positive values cause a fatal log at startup (fail-fast).
type GraphLimits struct {
	MaxNodes           int // SKILLFORGE_GRAPH_MAX_NODES (default 50)
	MaxEdges           int // SKILLFORGE_GRAPH_MAX_EDGES (default 200)
	NeighborDepthLimit int // SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT (default 2)
	PathHopsLimit      int // SKILLFORGE_GRAPH_PATH_HOPS_LIMIT (default 4)
}

// Phase 10 nil-gate table — the load-bearing behavior contract for the
// Rewriter + RefusalClassifier fields above. The test
// TestVerdictPipeline_NilRewriterStillFlowsHistory (pipeline_default_test.go)
// is the regression lock for the "Both nil → history prefix STILL flows"
// row. A future refactor that inadvertently couples history projection
// to rewriter / classifier nil-gates will trip this test.
//
//	| Field              | nil behavior                                                |
//	|--------------------|-------------------------------------------------------------|
//	| Rewriter nil       | rewrite step skipped, original query used for retrieval      |
//	|                    | + classification                                            |
//	| RefusalClassifier  | classify step skipped, only the code-level SC-2 short-       |
//	| nil                | circuits (0-chunks / score<0.3) refuse                      |
//	| Both nil           | history prefix STILL flows when history exists (B2 fix)     |

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
// GET /api/audit (SC-3 audit query + retention projection),
// GET /api/run/:id/plan (Phase 12 PLAN-05 plan replay),
// POST /api/run/:id/plan/:approvalId/approve (Phase 12 PLAN-03 plan
// confirm endpoint),
// POST /api/import/preview, /api/import/commit, /api/import/push,
// DELETE /api/import/dataset/:name (Phase 15 data-import surface).
func New(addr string, deps *Deps) *server.Hertz {
	importMax := deps.ImportMaxBodyBytes
	if importMax <= 0 {
		importMax = ImportMaxBodyDefault
	}
	nonImportMax := deps.NonImportMaxBodyBytes
	if nonImportMax <= 0 {
		nonImportMax = NonImportMaxBodyDefault
	}

	h := server.Default(
		server.WithHostPorts(addr),
		server.WithStreamBody(true),
		server.WithDisablePreParseMultipartForm(true),
		// WithMaxRequestBodySize takes an int; ImportMaxBodyDefault fits on
		// 64-bit platforms and is guarded by the non-import guard below.
		server.WithMaxRequestBodySize(int(importMax)),
	)

	// Phase 15: per-route declared body-size guard. The import endpoints need
	// a 2 GiB envelope, but that ceiling must not be usable to flood /api/chat.
	// Reject non-import routes that declare Content-Length > nonImportMax with
	// 413 before any handler runs. Import routes are handled by the global
	// WithMaxRequestBodySize limit above and never hit this guard.
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		if strings.HasPrefix(path, "/api/import/") {
			c.Next(ctx)
			return
		}
		cl := c.Request.Header.ContentLength()
		if cl > 0 && int64(cl) > nonImportMax {
			c.JSON(http.StatusRequestEntityTooLarge, graphResponseError{
				Error: "request body too large",
			})
			c.Abort()
			return
		}
		c.Next(ctx)
	})

	h.POST("/api/chat", deps.chatHandler)
	h.GET("/api/skills", deps.listSkillsHandler)
	h.POST("/api/skills", deps.validateSkillHandler)
	h.POST("/api/skills/wizard", deps.wizardHandler)
	h.POST("/api/skills/wizard/commit", deps.wizardCommitHandler)
	h.GET("/api/knowledge/graph", deps.graphHandler)
	h.GET("/api/knowledge/graph/neighbors", deps.neighborsHandler)
	h.GET("/api/knowledge/graph/path", deps.pathHandler)
	h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)
	h.GET("/api/knowledge/docs", deps.docsHandler)
	h.GET("/api/knowledge/doc", deps.docContentHandler)
	h.GET("/api/eval/reports", deps.evalIndexHandler)
	h.GET("/api/eval/reports/:ts", deps.evalReportHandler)
	h.GET("/api/eval/frozen", deps.evalFrozenHandler)
	h.GET("/api/audit", deps.auditHandler)
	// Phase 14 Plan 02 (PLAT-10 / D14-B2) — admin preview of the effective
	// system prompt for a Skill. Returns floor_version, skill_id,
	// composed_prompt, and prompt_hash (same hash that audit rows carry).
	h.GET("/api/admin/platform-prompt", deps.platformPromptHandler)
	// Phase 16 Plan 08 (KPI-01) — admin read-only view of the KPI
	// aggregation channel: registry catalog + content signature +
	// last-invocation timestamps from the kpi_calls audit table. The KPI
	// definitions are controlled files loaded once at startup, so this
	// surface is retrieval-only by design (a mutation route here would be
	// a registration backdoor — see kpi_admin.go godoc).
	h.GET("/api/admin/kpis", deps.kpiAdminHandler)
	// Phase 26 Plan 26-05 (KBM-02 / KBM-03) — knowledge-base registry
	// admin surface + public projection. Unlike the KPI view above, this
	// surface INTENTIONALLY carries POST verbs: KBs are operational
	// content, not controlled SQL definitions. Both mutations are
	// admin-token-gated (CHECK ORDER: 403 before the nil-registry 503) and
	// each writes one INSERT-only audit row (action_kb_created /
	// action_kb_deactivated); there is no delete endpoint (deactivate ≠
	// delete). Trust model: kb_admin.go.
	h.GET("/api/admin/kb", deps.kbAdminHandler)
	h.POST("/api/admin/kb", deps.kbCreateHandler)
	h.POST("/api/admin/kb/:slug/deactivate", deps.kbDeactivateHandler)
	// Phase 27 — document upload surface. POST /api/admin/kb/:slug/upload
	// accepts a multipart/form-data file, verifies magic bytes, parses it
	// into chunks, embeds each chunk, and inserts into the vector backend.
	h.POST("/api/admin/kb/:slug/upload", deps.uploadHandler)
	// Public read-only projection for the chat FE (slug → display_name
	// badge mapping). No token by design: the payload is minimized to
	// slug/display_name/role of ACTIVE entries only.
	h.GET("/api/knowledge/bases", deps.knowledgeBasesHandler)
	// Phase 29 Plan 29-05 (FBK-01/02/03/05) — the feedback loop HTTP
	// surface. POST /api/feedback is a PUBLIC write endpoint (same trust
	// shape as /api/chat: local deployment, no account system): one vote
	// per (session, turn) enforced by the store's UNIQUE key (D-29-02);
	// the cited chunks + skill are copied SERVER-side from the turn's
	// audit row (D-29-05), never from the request body. The three admin
	// routes are token-gated (CHECK ORDER: 403 before the nil-Store 503)
	// and carry the review queue, the disposition verb (INSERT-only state
	// row + audit row — the ONLY path feedback influences anything), and
	// the full-denominator metrics view. FBK-05 red line: no feedback path
	// mutates knowledge state (TestFeedbackNoTrustMutation locks that at
	// source level). Trust model: feedback.go.
	h.POST("/api/feedback", deps.feedbackHandler)
	h.GET("/api/admin/feedback/review", deps.feedbackReviewHandler)
	h.POST("/api/admin/feedback/dispositions", deps.feedbackDispositionHandler)
	h.GET("/api/admin/feedback/metrics", deps.feedbackMetricsHandler)
	// Phase 29 Plan 29-06 (TKT-02/04) — the ticket HTTP surface. POST
	// /api/tickets is a PUBLIC write endpoint (same trust shape as
	// /api/chat and /api/feedback): one ticket per message via the store's
	// UNIQUE(session_id, turn_id) key (D-29-14), an acceptance number
	// derived as "TKT-{row id}", and an IMMEDIATE 201 — the outbound
	// dispatch runs in its own goroutine on a background-derived budget
	// (TKT-04 structural non-blocking) and its settled outcome lands as an
	// INSERT-only action_ticket_dispatch_* audit row, never as an update to
	// the ticket row. Trust model: tickets.go.
	h.POST("/api/tickets", deps.ticketsHandler)
	// Phase 20 Plan 20-01 (ADM-01) — admin console capacity monitor.
	// Returns per-table row counts + thresholds for the capacity bars.
	h.GET("/api/admin/table-counts", deps.tableCountsHandler)
	// Phase 20 Plan 20-02 (ADM-02) — admin import batch list view.
	// Returns every import batch audit row across all datasets, newest first.
	// Retrieval-only by construction: the import_batches table is INSERT-only
	// and this handler only reads (see admin_batches.go godoc).
	h.GET("/api/admin/batches", deps.batchesAdminHandler)
	// Phase 20 Plan 20-06 (ADM-06) — admin audit sub-view endpoint.
	// Returns audit rows with actor_type='admin', with Phase 6 redact()
	// applied (isAdmin=true → admin sees raw rows). Filtered at the store
	// layer via AuditFilter.ActorType="admin".
	h.GET("/api/admin/audit", deps.adminAuditHandler)
	// Phase 20 Plan 20-04 (ADM-04) — admin skill registry view.
	// Overlays hot-reload status + last eval result from eval/reports/INDEX.json.
	h.GET("/api/admin/skills", deps.adminSkillsHandler)
	// Phase 20 Plan 20-05 (ADM-05) — admin scheduled task list view.
	// Returns report run list with cron_expr, status, drift_status via ListReportRuns.
	h.GET("/api/admin/schedules", deps.adminSchedulesHandler)
	// Phase 38 Plan 38-04 (v1.4 / D-Route-01) — ontology admin surface.
	// Eight token-gated routes (403 before the nil-layer 503): the TBox
	// browser feed, KG stats + pattern query, the schema-induction
	// proposal queue (generate / list / approve / reject — approve is the
	// ONLY TBox mutation path, ONT-04 human gate), and a manual snapshot
	// trigger. Trust model: admin_ontology.go godoc.
	h.GET("/api/admin/ontology/tbox", deps.adminOntologyTBoxHandler)
	h.GET("/api/admin/ontology/kg/stats", deps.adminOntologyKGStatsHandler)
	h.GET("/api/admin/ontology/kg/query", deps.adminOntologyKGQueryHandler)
	// Quick 260929-g7x — note-centric ABox projection for the FE 本体视图.
	h.GET("/api/admin/ontology/graph", deps.adminOntologyGraphHandler)
	h.POST("/api/admin/ontology/proposals/generate", deps.adminOntologyProposalsGenerateHandler)
	h.GET("/api/admin/ontology/proposals", deps.adminOntologyProposalsListHandler)
	h.POST("/api/admin/ontology/proposals/:id/approve", deps.adminOntologyProposalsApproveHandler)
	h.POST("/api/admin/ontology/proposals/:id/reject", deps.adminOntologyProposalsRejectHandler)
	h.POST("/api/admin/ontology/snapshot", deps.adminOntologySnapshotHandler)
	// Phase 40 Plan 05 (D-LOG-01 / D-TBox-03) — TBox version evolution log
	// + rollback. Timeline entries read newest-first; rollback writes v{N+1}
	// = copy of target, rebuilds KG, re-runs SHACL returning violation count.
	h.GET("/api/admin/ontology/changelog", deps.adminOntologyChangelogHandler)
	h.POST("/api/admin/ontology/rollback", deps.adminOntologyRollbackHandler)
	// Phase 40 Plan 06 (D-SHACL-02 / D-SHACL-03) — governance admin surface:
	// SHACL validation report (cached from boot or rollback) + provenance graph
	// including quarantined entities. Both routes are token-gated (403 before 503).
	h.GET("/api/admin/governance/shacl", deps.adminGovernanceShaclHandler)
	h.GET("/api/admin/governance/provenance", deps.adminGovernanceProvenanceHandler)
	// Phase 41 Plan 01 (PLAT-ONT-18) — MCP server inspector surface for the
	// admin platform tab. All routes are token-gated (403 before nil 503).
	// MCPServer is nil when SKILLFORGE_MCP_ENABLED=false|0|off.
	h.GET("/api/admin/platform/mcp/status", deps.adminPlatformMcpStatusHandler)
	h.GET("/api/admin/platform/mcp/tools", deps.adminPlatformMcpToolsHandler)
	h.POST("/api/admin/platform/mcp/invoke", deps.adminPlatformMcpInvokeHandler)
	// Phase 41 Plan 02 (PLAT-ONT-19) — SPARQL-like endpoint for KG queries.
	// X-Skill-Name header carries caller identity for tier isolation (D-SPARQL-03).
	// NOT under /api/admin/ — the token is the X-Skill-Name header, not X-Admin-Token.
	h.POST("/api/sparql", deps.adminPlatformSparqlHandler)
	// Phase 41 Plan 03 (PLAT-ONT-20) — Hybrid Retrieval endpoint.
	// X-Skill-Name header carries caller identity for tier isolation (D-HYB-05).
	// POST body: {query, max_hops, rerank}; response: {kg_nodes, kg_edges, milvus_hits}.
	h.POST("/api/admin/platform/hybrid/search", deps.adminPlatformHybridSearchHandler)
	// Phase 39 Plan 39-07 (v1.4 / D-Route-01) — ECL pipeline admin surface.
	// Three token-gated routes (403 before the nil-manager 503, LOCKED
	// order): run (fire-and-forget trigger → 200 started / 409 already
	// running), status (the D-Prog-01 polling feed: stage / current_file /
	// done / total / queue_len / failures), and per-note dead-letter retry
	// (400 malformed id / 404 unknown-or-non-dead / 409 busy / 200 queued).
	// run + retry trigger LLM spend and KG/Milvus writes. Trust model:
	// admin_pipeline.go godoc.
	h.POST("/api/admin/pipeline/run", deps.adminPipelineRunHandler)
	h.GET("/api/admin/pipeline/status", deps.adminPipelineStatusHandler)
	h.POST("/api/admin/pipeline/retry/:note_id", deps.adminPipelineRetryHandler)
	// Phase 10 Plan 03 (MEM-02 / D10-D1 + D10-D2) — cross-refresh
	// history restore. The handler returns the session's rendered
	// turns + the session-bound skill_id so the FE can rebuild the
	// chat surface via useChat({ initialMessages }) on page load.
	h.GET("/api/session/:id/history", deps.sessionHistoryHandler)
	// Session-management phase — the roster surface for the FE
	// SessionDrawer: list (newest-first, rename override merged),
	// rename upsert (no existence leak), idempotent delete (audit
	// trail untouched per T-3-AUDIT).
	h.GET("/api/sessions", deps.sessionsListHandler)
	h.PATCH("/api/session/:id", deps.sessionRenameHandler)
	h.DELETE("/api/session/:id", deps.sessionDeleteHandler)
	// Phase 11 Plan 04 (HITL-01/02/03 + D11-C1 dual endpoint + D11-C2
	// expire-check): the operator approval gate. GET lists the
	// session's pending approvals with derived state; POST resolves
	// one via AuditWriter + Runner.ResumeWithParams; GET audit is the
	// AuditTimelineTab (FE 11-07) data source.
	h.GET("/api/session/:id/approval", deps.approvalGetHandler)
	h.POST("/api/session/:id/approval/:approvalId", deps.approvalPostHandler)
	h.GET("/api/session/:id/audit", deps.auditListHandler)
	// Phase 24 / Task 24-01 — session-scoped W3C PROV-JSON export of the
	// audit chain (internal/auditprov; zero-LLM, read-only, SC-9 isolation
	// identical to the audit route above).
	h.GET("/api/session/:id/audit/prov", deps.sessionProvHandler)
	// Phase 24 Tasks 24-02 + 24-03 (decision-chain + time-travel state).
	// decision-chain returns every audit row for the session classified
	// into a chronological Link list; state?at=RFC3339 reconstructs the
	// session state at time T (visible turns + effective rules). Both
	// are zero-LLM read-only reconstructions from the INSERT-only
	// audit substrate (T-3-AUDIT).
	h.GET("/api/session/:id/decision-chain", deps.SessionChainHandler())
	h.GET("/api/session/:id/state", deps.SessionStateHandler())

	// Phase 21 Task 21-04 — External Authorized Channel endpoints.
	// 6 routes: create + list pending + approve + reject + execute
	// + reconcile. All return 503 when ExternalAuthManager is nil
	// (degraded-mode posture; cmd/server main.go wires the manager
	// only when EXTERNAL_AUTH_ENABLED=1).
	mgr := deps.ExternalAuthManager
	h.POST("/api/external-auth/request", postExternalAuthRequest(deps, mgr))
	h.GET("/api/external-auth/pending", getExternalAuthPending(deps, mgr))
	h.POST("/api/external-auth/approve", postExternalAuthApprove(deps, mgr))
	h.POST("/api/external-auth/reject", postExternalAuthReject(deps, mgr))
	h.POST("/api/external-auth/execute", postExternalAuthExecute(deps, mgr))
	h.GET("/api/external-auth/audit/reconcile", getExternalAuthAuditReconcile(deps, mgr))
	// Phase 11 / Plan 11-05 (HITL-08) — approval_rules CRUD surface.
	// ListApprovalRulesHandler supports ?scope=session|skill[&skill_id=X][&target=X];
	// RevokeApprovalRuleHandler clears a remembered rule (D11-D2 single-table reuse).
	h.GET("/api/session/:id/approval-rules", deps.ListApprovalRulesHandler)
	h.DELETE("/api/session/:id/approval-rules/:ruleId", deps.RevokeApprovalRuleHandler)
	// Phase 12 Plan 04 (PLAN-05 BE replay + PLAN-03 BE confirm
	// endpoint) — the run-keyed planning surface. GET returns the
	// run's plan_todos rows in ts-ASC latest-snapshot order (the FE
	// PlanReplayView 12-07 data source); POST resolves a pending
	// plan_approve approval (mirrors the Phase 11 approval POST with
	// target=plan_approve, D11-C2 expire-check included).
	h.GET("/api/run/:id/plan", deps.planGetHandler)
	h.POST("/api/run/:id/plan/:approvalId/approve", deps.planApproveHandler)
	// Phase 18 Plan 18-02 — static report portal (RPT-04).
	// GET /reports/<period>/v<N>.html serves published HTML reports.
	// Directory traversal is blocked by reportPathAllowlist regex.
	h.GET("/reports/*filepath", reportsRoute)
	// Phase 19 Plan 19-01 — scheduler-triggered report run portal (PORT-01).
	// GET /reports lists report_runs rows filtered by skill_id/period.
	h.GET("/reports", deps.reportsListHandler)
	// Phase 19 Plan 19-02 — manual rerender endpoint (PORT-05).
	// POST /api/reports/rerender triggers immediate report rerun with IsManual=true.
	h.POST("/api/reports/rerender", deps.rerenderHandler)
	// Phase 15 — data-import surface. Handlers live in import_api.go.
	h.POST("/api/import/preview", deps.importPreviewHandler)
	h.POST("/api/import/commit", deps.importCommitHandler)
	h.POST("/api/import/push", deps.importPushHandler)
	h.DELETE("/api/import/dataset/:name", deps.importDeleteHandler)
	// Initialize the fail-closed HITL audit writer from the store when
	// the caller did not inject one (D11-C3 single write path).
	if deps.AuditWriter == nil && deps.Store != nil {
		deps.AuditWriter = hitl.NewAuditWriter(deps.Store, agent.PlatformFloorVersion)
	}
	log.Printf("[OK] server: import max body = %s, non-import max body = %s",
		byteSizeLabel(importMax), byteSizeLabel(nonImportMax))
	return h
}

// byteSizeLabel renders a byte count as a human-readable string (e.g. "2 GiB").
func byteSizeLabel(n int64) string {
	switch {
	case n >= 1<<30:
		return strconv.FormatInt(n/(1<<30), 10) + " GiB"
	case n >= 1<<20:
		return strconv.FormatInt(n/(1<<20), 10) + " MiB"
	case n >= 1<<10:
		return strconv.FormatInt(n/(1<<10), 10) + " KiB"
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}
