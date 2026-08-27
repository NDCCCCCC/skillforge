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

// Command server is the long-running Hertz HTTP surface for SkillForge
// Phase 2 (Pattern 1 in 02-RESEARCH.md §lines 263-292). It exposes
// POST /api/chat and wires the graduated internal/{verdict, wire,
// retriever, embedder, skill, knowledge} packages into a single binary.
//
// Startup sequence (fail fast on any error — the operator must see the
// problem at startup, never at request time):
//
//  1. Load skills via skill.NewLoader("skills/").Scan(). An invalid
//     Skill is fatal — the operator should see the SYSK-02 violation
//     in the startup log, not as a confusing 500 later.
//  2. Construct the configured retriever backend (RETRIEVER_BACKEND):
//     "milvus" (default) — milvus.New (bounded retries) + Ensure + Index.
//     "mongo"            — mongo.New (Connect+Ping) + Ensure +
//     SeedFromKnowledgeDir. Used when the internal
//     Milvus is unavailable; MongoDB 4.4+ has no
//     native vector search, so cosine is computed
//     in Go against the tier-filtered rows.
//  3. BGE-M3 outage is NOT fatal at startup (Pitfall 8 — production
//     refuses on BGE-M3 outage at request time, not startup time, so
//     the server can boot into a known-broken state and operators can
//     see the failure mode in the first chat request).
//  4. Build the GLM-5.2 ChatModel via openai.NewChatModel pointed at
//     the Coding Plan endpoint (Pitfall 6 — NOT /api/paas/v4, the
//     standard endpoint rejects Coding Plan keys with a confusing
//     "余额不足" error).
//  5. Wrap the ChatModel in adk.NewChatModelAgent + adk.NewRunner.
//  6. Build *server.Deps, h := server.New(addr, deps), h.Spin().
//
// Environment variables (the deployment surface):
//
//   - RETRIEVER_BACKEND (optional) — "milvus" (default) | "mongo"
//   - MILVUS_ADDRESS    (milvus only, required) — gRPC host:port
//   - MONGO_URI         (mongo only, required)  — e.g. mongodb://10.62.10.34:27017
//   - BGE_M3_URL        (required) — internal BGE-M3 HTTP endpoint
//   - BGE_M3_TLS_SKIP_VERIFY (optional) — "true" disables TLS cert verification
//     for BGE_M3_URL. Internal-network self-signed HTTPS ONLY; logs [WARN]
//     at startup. Default unset means verify-on (production safe).
//   - GLM_API_KEY       (required) — Zhipu Coding Plan subscription key
//   - GLM_BASE_URL      (optional) — default https://open.bigmodel.cn/api/coding/paas/v4
//   - SERVER_ADDR       (optional) — default ":8888" (Pitfall 7)
//   - USE_AGENT_PATH    (optional) — "true" opts into the Phase 3 ReAct
//     agent path (runAgentPath). Default unset → VerdictPipeline is the
//     /api/chat handler. See .planning/debug/agent-llm-query-rewrite.md.
//
// All are read at startup; missing required vars are fatal with
// an actionable error message. The Skill author / operator should be
// able to fix the issue from the log alone.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/data"
	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/skillforge/skillforge/internal/retriever/mongo"
	"github.com/skillforge/skillforge/internal/server"
	"github.com/skillforge/skillforge/internal/server/classifier"
	"github.com/skillforge/skillforge/internal/server/rewrite"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// env-or-default helpers. Each required var aborts startup on empty.
const (
	defaultGLMBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	defaultServerAddr = ":8888"
	defaultCollection = "skillforge_knowledge"
	// Plan 06-02 / SC-1: the eval dashboard reads the same files
	// cmd/eval --archive writes. Keeping these paths in sync with
	// cmd/eval's defaults is the load-bearing invariant — a
	// dashboard that reads a different directory than the CLI
	// writes would be silently empty.
	defaultEvalReportsDir = "eval/reports"
	defaultEvalFrozenDir  = "eval/frozen"
)

// envRequired reads a required env var; returns the value or aborts
// with a clear "set X in .env" message. Mirrors the spike-era pattern.
func envRequired(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("[FAIL] cmd/server: %s env var not set (copy .env.example to .env and populate)", name)
	}
	return v
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// parseBytesEnvOrDefault parses an int64 byte-count env var. Non-numeric or
// non-positive values log a WARN and fall back to def. This keeps a typo in
// SKILLFORGE_IMPORT_MAX_BODY_BYTES from making the server unusable.
func parseBytesEnvOrDefault(name string, def int64) int64 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		log.Printf("[WARN] cmd/server: %s=%q invalid (expected positive integer bytes); using default %d", name, v, def)
		return def
	}
	return n
}

func main() {
	ctx := context.Background()

	// 1. Env var parsing. MILVUS_ADDRESS / MONGO_URI are validated inside
	// the backend switch (only the configured backend's URI is required).
	bgeURL := envRequired("BGE_M3_URL")
	bgeModel := envOr("BGE_M3_MODEL", "bge-m3")
	bgeTimeoutSec := envOr("BGE_M3_TIMEOUT", "60")
	// BGE_M3_TLS_SKIP_VERIFY (opt-in) disables TLS certificate verification
	// for the BGE-M3 endpoint. ONLY for internal-network self-signed HTTPS
	// (BGE-M3 behind a corporate reverse proxy); production must leave
	// this unset so verify-on is the default. Case-insensitive "true" maps
	// to true; any other value (including empty / unset) keeps verify-on.
	bgeTLSSkipVerify := strings.EqualFold(os.Getenv("BGE_M3_TLS_SKIP_VERIFY"), "true")
	glmKey := envRequired("GLM_API_KEY")
	glmBase := envOr("GLM_BASE_URL", defaultGLMBaseURL)
	serverAddr := envOr("SERVER_ADDR", defaultServerAddr)
	evalReportsDir := envOr("EVAL_REPORTS_DIR", defaultEvalReportsDir)
	evalFrozenDir := envOr("EVAL_FROZEN_DIR", defaultEvalFrozenDir)
	// USE_AGENT_PATH (opt-in) selects the Phase 3 ReAct agent path
	// (internal/server.runAgentPath) for /api/chat. The PRODUCTION
	// DEFAULT is the Phase 2 VerdictPipeline (code-forced original-query
	// retrieval → LLM emits verdict JSON only). The agent path delegates
	// query rewrite / chunk selection / refusal judgement to the LLM,
	// which live-verified drifts on GLM-5.2 (年假 → 出差住宿费 rewrite
	// → unanswered on a query the retriever answers correctly with the
	// original phrasing). The agent + its pre-retrieve code stay in-tree
	// and unit-tested; this flag only gates whether cmd/server
	// CONSTRUCTS that agent. See .planning/debug/agent-llm-query-rewrite.md.
	useAgentPath := strings.EqualFold(os.Getenv("USE_AGENT_PATH"), "true")

	// DISABLE_HISTORY_PIPELINE (opt-in) disables the Phase 10 history-
	// aware pipeline wrappers (rewriter + refusal classifier). When
	// true, Rewriter and RefusalClassifier in Deps are nil — the
	// history prefix STILL flows when history is non-empty
	// (TestVerdictPipeline_NilRewriterStillFlowsHistory is the B2
	// regression lock). Use this flag to isolate behavior changes to
	// the history-projection path without disabling the projection
	// itself. The default (unset) keeps both wrappers active so
	// production runs the SC-1/SC-2 escape mechanisms.
	disableHistoryPipeline := strings.EqualFold(os.Getenv("DISABLE_HISTORY_PIPELINE"), "true")

	// Phase 15 — data-import pipeline configuration.
	// SKILLFORGE_DATA_SALT is the L1 HMAC site salt. Unset → import endpoints
	// are disabled (503 fail-closed), but the rest of the server starts.
	// SKILLFORGE_IMPORT_MAX_BODY_BYTES / SKILLFORGE_MAX_BODY_BYTES tune the
	// import / non-import body limits; parse failures fall back to defaults
	// with a WARN rather than aborting startup.
	dataSalt := envOr("SKILLFORGE_DATA_SALT", "")
	importMaxBody := parseBytesEnvOrDefault("SKILLFORGE_IMPORT_MAX_BODY_BYTES", server.ImportMaxBodyDefault)
	nonImportMaxBody := parseBytesEnvOrDefault("SKILLFORGE_MAX_BODY_BYTES", server.NonImportMaxBodyDefault)
	datarulesDir := envOr("SKILLFORGE_DATARULES_DIR", "datarules")
	// Phase 16 — KPI aggregation channel (D16-A1). SKILLFORGE_KPIS_DIR is the
	// directory of controlled KPI definition files (*.yaml) that is the ONLY
	// source of callable aggregations. The directory lives INSIDE git (.gitignore
	// only excludes data/ — the KPI definitions are reviewable, diffable,
	// versioned artifacts, unlike the datasets they read).
	kpisDir := envOr("SKILLFORGE_KPIS_DIR", "kpis")

	// 2. Load Skills (SYSK-02 validation at startup).
	loader := skill.NewLoader("skills/")
	manifests, err := loader.Scan()
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: skill scan: %v (fix the SKILL.md or remove it from skills/)", err)
	}
	log.Printf("[OK] loaded %d Skill(s)", len(manifests))

	// 3. BGE-M3 HTTP embedder (production; refuses on outage).
	// BGE_M3_MODEL defaults to "bge-m3" (required by Ollama's
	// OpenAI-compatible /v1/embeddings; harmless on vLLM/TEI).
	// BGE_M3_TIMEOUT defaults to 60s (Ollama cold-loads the 1.2GB model on
	// first call; internal Volcano/TEI returns <1s so the ceiling is harmless).
	bgeTimeout, err := time.ParseDuration(bgeTimeoutSec + "s")
	if err != nil || bgeTimeout <= 0 {
		bgeTimeout = 60 * time.Second
	}
	emb := bge.New(bge.Config{URL: bgeURL, Model: bgeModel, Timeout: bgeTimeout, InsecureSkipVerify: bgeTLSSkipVerify})
	if bgeTLSSkipVerify {
		log.Printf("[WARN] cmd/server: BGE_M3_TLS_SKIP_VERIFY=true — TLS certificate verification is DISABLED for %s (internal-network self-signed HTTPS only; NEVER use in production)", bgeURL)
	}

	// 4. Construct the configured retriever backend + seed the knowledge
	// corpus. The default is "milvus" (unchanged historical behavior);
	// "mongo" switches to the MongoDB-backed retriever (used when the
	// internal Milvus is unavailable — see 02-04-MONGO-SUMMARY.md).
	backend := strings.ToLower(os.Getenv("RETRIEVER_BACKEND"))
	if backend == "" {
		backend = "milvus"
	}
	r, backendLabel := buildRetriever(ctx, backend, emb)

	// 5. GLM-5.2 ChatModel (Coding Plan endpoint per Pitfall 6).
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: glmBase,
		APIKey:  glmKey,
		Model:   "glm-5.2",
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: GLM chat model init: %v (check GLM_BASE_URL=%s, GLM_API_KEY set)", err, glmBase)
	}

	// 6. ChatModelAgent + Runner.
	adkAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "skillforge-compliance",
		Description: "SkillForge compliance-review ChatModelAgent",
		Instruction: "You are a strict compliance auditor. Output ONLY the JSON object specified by the runtime system prompt.",
		Model:       cm,
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: adk agent init: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: adkAgent})

	// 6b. Phase 3 ReAct Agent (Plan 03-03). Constructed ONLY when
	// USE_AGENT_PATH=true (opt-in). The PRODUCTION DEFAULT is the Phase 2
	// VerdictPipeline (chatHandler falls through to it when s.Agent ==
	// nil), which live-verified 6/6 SC green and is robust against LLM
	// query-rewrite drift. The agent path (runAgentPath) delegates query
	// rewrite / chunk selection / relevance+refusal judgement to the LLM;
	// on GLM-5.2 every layer drifts (年假 → 出差住宿费 rewrite → wrong doc
	// → unanswered; a tweaked "MUST answer from chunks" prompt then broke
	// the refusal moat on out-of-scope queries). That direction is
	// live-verified FAILED — see
	// .planning/debug/agent-llm-query-rewrite.md. The agent code, the
	// pre-retrieve guard (commits 0171680 + d3178ed), and its unit tests
	// stay in-tree as the opt-in experimental path; this flag only
	// determines whether cmd/server constructs it.
	//
	// MaxHops=2 is the D3-02 hard cap (ComputeMaxStep(2)=5 LLM turns max).
	// Emit is NOT bound here — internal/agent's DecisionEmitFunc requires
	// the per-request *app.RequestContext, which does not exist at server
	// startup. The chatHandler wraps emitDecision(c, ...) into a closure
	// per request (see internal/server/chat.go runAgentPath).
	ag := (*agent.Agent)(nil)
	if useAgentPath {
		ag, err = agent.NewAgent(ctx, agent.AgentConfig{
			Manifest:  firstManifest(loader),
			Embedder:  emb,
			Retriever: r,
			ChatModel: cm,
			MaxHops:   2, // D3-02 hard cap — see agent.ComputeMaxStep
		})
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: agent init: %v", err)
		}
	}
	if useAgentPath {
		log.Printf("[OK] cmd/server: /api/chat path = runAgentPath (USE_AGENT_PATH=true; experimental)")
	} else {
		log.Printf("[OK] cmd/server: /api/chat path = VerdictPipeline (default; USE_AGENT_PATH unset)")
	}

	// 6c. Store (audit + session-memory backend, Plan 03-02). Zero-ops
	// default: pure-Go modernc.org/sqlite at SQLITE_DSN (default
	// "skillforge.db" so a plain `go run cmd/server` produces a
	// persistent audit trail next to the binary). Health-check fails
	// fast on a misconfigured DSN — the operator sees the problem at
	// startup, not at the first /api/chat request.
	st, err := buildStore(ctx)
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: store init: %v", err)
	}

	// 6c-2. Phase 15 — data-import orchestrator. The importer needs a
	// dedicated DuckDB connection for lazy ATTACH/DETACH of dataset files.
	// A nil Importer is fine (import endpoints return 503), but a non-empty
	// SKILLFORGE_DATA_SALT wires the full pipeline.
	var importer *data.Importer
	if dataSalt != "" {
		ddb, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: duckdb connector for import: %v", err)
		}
		lru, err := duckdb.NewAttachLRU(ddb, 0)
		if err != nil {
			_ = ddb.Close()
			log.Fatalf("[FAIL] cmd/server: attach LRU for import: %v", err)
		}
		importer = data.NewImporter(data.ImporterConfig{
			SiteSalt: dataSalt,
			RulesDir: datarulesDir,
		}, st, lru)
		log.Printf("[OK] cmd/server: import pipeline wired (salt present, rules=%s)", datarulesDir)
	} else {
		log.Printf("[WARN] cmd/server: SKILLFORGE_DATA_SALT unset — import endpoints disabled (fail-closed)")
	}

	// 6c-3. Phase 16 — KPI aggregation pipeline (D16-A1 / D16-C3 / KPI-01).
	// Mirrors the importer assembly above (dedicated in-memory DuckDB
	// connector + AttachLRU + fail-closed posture) with ONE deliberate
	// difference: the KPI pipeline is wired UNCONDITIONALLY, regardless of
	// SKILLFORGE_DATA_SALT. Rationale — the salt is the L1 HMAC site secret
	// for the WRITE side (import desensitization); KPI calls only READ
	// already-desensitized datasets and never re-derive a hash, so gating
	// them on the salt would disable analytics on a perfectly valid
	// read-only deployment.
	//
	// Fail-closed posture (same as Skill Scan above): a malformed KPI file
	// is fatal. The KPI definitions are controlled files and the ONLY source
	// of callable aggregations (D16-A1) — booting with a half-loaded
	// registry would mean the agent silently loses indicators. An EMPTY (or
	// absent) kpis/ directory is the single legal degraded state: the
	// registry loads empty, the log says so, and data-channel Skills simply
	// have nothing to call.
	//
	// Windows/WSL note: cmd/server has been a CGO build since the Phase 15
	// importer assembly (go-duckdb). MSYS2 gcc on Windows can link sqlite
	// but NOT duckdb (D15-01-1), so running the server locally on Windows
	// is a WSL/Docker activity. This is pre-existing Phase 15 status quo,
	// NOT a constraint introduced by the KPI pipeline.
	kpiConn, err := duckdb.New(ctx, duckdb.Config{DSN: ":memory:", MaxOpenConns: 1})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: duckdb connector for KPI pipeline: %v", err)
	}
	kpiLRU, err := duckdb.NewAttachLRU(kpiConn, 0)
	if err != nil {
		_ = kpiConn.Close()
		log.Fatalf("[FAIL] cmd/server: attach LRU for KPI pipeline: %v", err)
	}
	// Layer 2 validation (json_serialize_sql AST walk) runs over this same
	// :memory: connection at load time — the serializer never touches a
	// dataset file, it only asks DuckDB to parse the statement.
	kpiRegistry, kpiSignature, err := kpi.LoadKPIs(ctx, kpisDir,
		duckdb.NewDuckDBSQLSerializer(kpiConn), storeDatasetLister{st: st})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: KPI load from %s: %v (fix the .yaml or remove it from the directory — controlled files are load-bearing, D16-A1)", kpisDir, err)
	}
	kpiExecutor := kpi.NewExecutor(kpiRegistry, storeDatasetMeta{st: st},
		duckdb.NewEngineAdapter(kpiLRU), st, time.Now)
	log.Printf("[OK] cmd/server: KPI pipeline wired (kpis=%s, registry=%d entries, signature=%s)",
		kpisDir, len(kpiRegistry.Names()), shortSignature(kpiSignature))

	// 6d. Skill Registry (Plan 05-01 / §2 hot-reload). The fsnotify-driven
	// registry is the production source of truth for the GET /api/skills
	// listing's version + status fields — touching skills/<name>/SKILL.md
	// on disk bumps that skill's Entry.Version (which the FE polls) and, on
	// a broken-YAML reload, records status:"failed" WITHOUT bumping the
	// version (keep-old). A nil embedder disables description-embedding
	// (routing is not wired here — Router stays nil and chatHandler falls
	// back to the Loader findSkill path); this also avoids a BGE round-trip
	// at startup. The watcher goroutine runs until ctx (background) is
	// cancelled — i.e. process exit — matching the existing Store.Close
	// posture (Phase 6 adds graceful shutdown).
	reg, err := skill.New("skills/", nil)
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: skill registry: %v (fix the SKILL.md or remove it from skills/)", err)
	}
	if err := reg.Watch(ctx); err != nil {
		log.Fatalf("[FAIL] cmd/server: registry watch: %v", err)
	}

	// 6e. Phase 10 history-aware pipeline assembly (Plan 10-02 Task 4 /
	// checker B2 key fix). Without this step the running product would
	// have nil Rewriter / RefusalClassifier and the SC-1 / SC-2 escape
	// mechanisms would never fire. Both constructors fail-fast at
	// startup (mirrors the existing `cm, err := openai.NewChatModel(…)`
	// posture); the operator must see any wiring failure at startup,
	// not at the first /api/chat request.
	//
	// The DISABLE_HISTORY_PIPELINE flag (opt-in) leaves both fields nil
	// for incident-response scenarios where the operator wants to isolate
	// a regression to the wrappers vs the projection. The history prefix
	// STILL flows in that mode (the B2 nil-gate fix) — the regression
	// surface is the rewriter + classifier layers, NOT the projection.
	var (
		rewriter          *rewrite.QueryRewriter
		refusalClassifier *classifier.RefusalClassifier
	)
	if disableHistoryPipeline {
		log.Printf("[WARN] cmd/server: DISABLE_HISTORY_PIPELINE=true — semantic rewrite + classifier layers DISABLED (history prefix still flows; non-fatal)")
	} else {
		rewriter, err = rewrite.NewQueryRewriter(cm)
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: query rewriter init: %v (check GLM_BASE_URL=%s, GLM_API_KEY set)", err, glmBase)
		}
		refusalClassifier, err = classifier.NewRefusalClassifier(cm)
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: refusal classifier init: %v (check GLM_BASE_URL=%s, GLM_API_KEY set)", err, glmBase)
		}
		log.Printf("[OK] cmd/server: history-aware pipeline ACTIVE (rewrite + classifier; budget window + neutral placeholder + multi-turn messages)")
	}

	// 7. Build *server.Deps. Embedder + Retriever are concrete impls
	// behind the interfaces; Runner wraps the Eino Runner. Phase 3 adds
	// Agent + Store so chatHandler can run the ReAct loop + persist audit
	// rows + session turns (Plan 03-05).
	//
	// Wizard (Plan 05-04) is wired here so /api/skills/wizard + /commit
	// are live in production. The wizard writes into the SAME skillsDir
	// the Loader scans; the fsnotify-driven Registry (Plan 05-01) picks
	// up the new file automatically and the chat path sees it on the
	// next request — D5-11 closed loop. SkillsDir is intentionally the
	// same string passed to skill.NewLoader("skills/") above so the
	// "wizard wrote here, Loader reads there" contract holds.
	//
	// EvalReporter (Plan 06-02 / SC-1) reads the same eval/reports/
	// directory cmd/eval --archive writes; this is the single source of
	// truth for the trend history. Empty paths default to
	// "eval/reports" / "eval/frozen" inside FilesystemEvalReporter so
	// a plain `go run cmd/server` finds the archive the CLI produced.
	//
	// Agent is nil unless USE_AGENT_PATH=true (see 6b). A nil Agent
	// makes chatHandler take the VerdictPipeline branch (the production
	// default); the agent path remains constructable + unit-tested for
	// future use.
	deps := &server.Deps{
		Loader:       loader,
		Registry:     reg,
		Embedder:     emb,
		Retriever:    r,
		Runner:       &prodRunner{runner: runner, cm: cm},
		Agent:        ag,
		Store:        st,
		Wizard:       skill.NewWizard(cm, "skills/"),
		Knowledge:    knowledge.New(r),
		EvalReporter: server.NewFilesystemEvalReporter(evalReportsDir, evalFrozenDir),
		ChatModel:    cm,
		// Plan 260815-cos: PipelineChatModel = cm activates Step 8
		// streaming (text-delta-draft + text-delta-draft-end per token
		// before grounding). model.ToolCallingChatModel embeds
		// model.BaseChatModel so Stream is reachable on cm directly.
		PipelineChatModel: cm,
		// Plan 10-02 Task 4 (B2 fix): wire the Phase 10 history-aware
		// pipeline wrappers. Rewriter + RefusalClassifier share the same
		// `cm` ChatModel (one model, three call shapes). When
		// DISABLE_HISTORY_PIPELINE=true both fields stay nil (the B2
		// nil-gate table in internal/server/server.go documents the
		// "history prefix STILL flows" behavior).
		Rewriter:          rewriter,
		RefusalClassifier: refusalClassifier,
		// Phase 15 — data-import surface. nil Importer means import endpoints
		// return 503 (SKILLFORGE_DATA_SALT unset fail-closed posture).
		Importer:              importer,
		ImportMaxBodyBytes:    importMaxBody,
		NonImportMaxBodyBytes: nonImportMaxBody,
		// Phase 16 — KPI aggregation channel. Both fields are always
		// non-nil here (6c-3 is unconditional); a data-channel Skill
		// reaches kpi_call through them and GET /api/admin/kpis reads
		// the registry catalog + kpi_calls audit rows.
		KPIExecutor: kpiExecutor,
		KPIRegistry: kpiRegistry,
	}

	// 8. Hertz server (Pattern 1).
	h := server.New(serverAddr, deps)
	log.Printf("[OK] cmd/server: SkillForge listening on %s (backend=%s)", serverAddr, backendLabel)
	log.Printf("     POST /api/chat                — verdict pipeline (verdict or structured refusal)")
	log.Printf("     GET  /api/session/:id/history — Phase 10 history restore (Plan 03)")
	log.Printf("     GET  /api/sessions            — session roster (newest-first, rename override merged)")
	log.Printf("     PATCH /api/session/:id        — session rename (session_meta upsert)")
	log.Printf("     DELETE /api/session/:id       — session delete (turns+meta, audit untouched)")
	log.Printf("     GET  /api/skills              — list loaded Skills")
	log.Printf("     POST /api/skills              — dry-run Skill validator")
	log.Printf("     POST /api/skills/wizard       — SC-3 wizard (5-step interview -> preview)")
	log.Printf("     POST /api/skills/wizard/commit — atomic write (fsnotify closes the loop)")
	log.Printf("     GET  /api/knowledge/graph     — SC-5 graph browser (tier -> doc -> chunk)")
	log.Printf("     GET  /api/eval/reports        — SC-1 eval dashboard (INDEX.json + per-report fetch)")
	log.Printf("     GET  /api/eval/frozen         — D6-13 frozen-set metadata (skill + lock)")
	log.Printf("     GET  /api/audit               — SC-3 audit query (with retention redaction)")
	log.Printf("     POST /api/import/preview      — Phase 15 import preview (admin + salt required)")
	log.Printf("     POST /api/import/commit       — Phase 15 import commit (admin + salt required)")
	log.Printf("     POST /api/import/push         — Phase 15 import push (admin + salt required)")
	log.Printf("     DELETE /api/import/dataset/:name — Phase 15 dataset delete (admin + salt required)")
	log.Printf("     GET  /api/admin/kpis          — Phase 16 KPI catalog (admin, read-only)")
	// h.Spin() blocks until SIGINT/SIGTERM; the return is unreachable
	// in production but the lint guards us from a future "if err" insert.
	h.Spin()
}

// buildRetriever constructs the retriever.Retriever specified by backend
// ("milvus" or "mongo") and seeds the knowledge corpus into it. Both
// paths log.Fatalf on a misconfigured URI / unreachable server — the
// operator must see the failure at startup, not at request time.
//
// Returns the constructed Retriever plus a short label for the startup
// log line (e.g. "milvus(10.x.x.x:19530)" / "mongo(10.x.x.x:27017)").
func buildRetriever(ctx context.Context, backend string, emb *bge.BGE) (retriever.Retriever, string) {
	switch backend {
	case "mongo":
		mURI := envRequired("MONGO_URI")
		mc, err := mongo.New(ctx, mongo.Config{
			URI:        mURI,
			Database:   "skillforge",
			Collection: "knowledge",
		})
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: mongo dial %s: %v", mURI, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/server: mongo ensure collection: %v", err)
		}
		// Mongo path: the milvus-coupled knowledge.Index cannot run here
		// (it requires a milvus client.Client). Use the mongo backend's
		// own SeedFromKnowledgeDir, which mirrors the indexer's scan +
		// strip + embed + Insert flow. See mongo/seed.go godoc for the
		// Rule 3 deviation rationale.
		if err := mc.SeedFromKnowledgeDir(ctx, "knowledge/", emb); err != nil {
			log.Printf("[WARN] cmd/server: mongo knowledge seed failed (retrieval will return no_authorized_source until re-seeded): %v", err)
		} else {
			log.Printf("[OK] knowledge corpus seeded into mongo %s.%s", mc.DatabaseName(), mc.CollectionName())
		}
		return mc, "mongo(" + mURI + ")"
	default:
		if backend != "milvus" {
			log.Printf("[WARN] cmd/server: unknown RETRIEVER_BACKEND=%q, falling back to milvus", backend)
		}
		milvusAddr := envRequired("MILVUS_ADDRESS")
		mcfg := milvus.Config{
			Addr:           milvusAddr,
			CollectionName: defaultCollection,
		}
		mc, err := milvus.New(ctx, mcfg)
		if err != nil {
			log.Fatalf("[FAIL] cmd/server: milvus dial %s: %v", milvusAddr, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/server: milvus ensure collection: %v", err)
		}
		// Milvus path: unchanged — uses the milvus-coupled indexer.
		if err := knowledge.Index(ctx, "knowledge/", emb, mc.ClientOf(), mcfg); err != nil {
			log.Printf("[WARN] cmd/server: knowledge index failed (retrieval will return no_authorized_source until re-indexed): %v", err)
		} else {
			log.Printf("[OK] knowledge corpus indexed into %s", defaultCollection)
		}
		return mc, "milvus(" + milvusAddr + ")"
	}
}

// buildStore constructs the audit + session-memory backend (Plan 03-02).
// It mirrors buildRetriever's shape: read the env var, construct the
// backend, run a Health check to fail-fast on misconfiguration, and
// return the store.Store interface. Default SQLITE_DSN="skillforge.db"
// (file-backed) so a plain `go run cmd/server` produces a durable audit
// trail next to the binary; tests pass ":memory:" via the test harness.
//
// The returned store.Store is owned by the caller (cmd/server); Close
// is wired to SIGINT in a future hardening pass (Phase 3 ships without
// graceful-shutdown Close — the WAL journal_mode means a process kill
// still leaves the database consistent).
func buildStore(ctx context.Context) (store.Store, error) {
	dsn := envOr("SQLITE_DSN", "skillforge.db")
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	if err != nil {
		return nil, fmt.Errorf("sqlite.New(%q): %w", dsn, err)
	}
	if err := s.Health(ctx); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("store Health check: %w", err)
	}
	log.Printf("[OK] cmd/server: store open (dsn=%s, WAL mode)", dsn)
	return s, nil
}

// firstManifest returns the first loaded Skill's manifest. Phase 2 ships
// exactly one Skill (compliance); Phase 5 introduces semantic routing
// across multiple Skills. Aborts startup if zero Skills loaded — the
// agent requires a manifest to close over (ResolvedTiers → retrieve
// tool's PRE-similarity filter; KNOW-03).
func firstManifest(loader *skill.Loader) *skill.Manifest {
	for _, m := range loader.Skills() {
		return m
	}
	log.Fatalf("[FAIL] cmd/server: zero Skills loaded — cannot construct agent (the agent requires a manifest with resolved tiers; check skills/ directory)")
	return nil
}

// storeDatasetLister adapts store.Store to kpi.DatasetLister: the loader
// only needs the set of LOGICAL dataset names (e.g. "loans"), while the
// registry rows are composite ("loans__2026-07"). Pre-Phase-15 rows carry
// an empty Dataset column and are skipped — they cannot back a KPI file's
// dataset declaration anyway (D16-A2 requires a registered logical name).
//
// Interface isolation (kpi/executor.go:39-51): internal/kpi must NOT import
// internal/store's full surface, so the adapter lives here in the assembly.
type storeDatasetLister struct {
	st store.Store
}

func (l storeDatasetLister) ListDatasets(ctx context.Context) ([]string, error) {
	rows, err := l.st.ListDatasets(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(rows))
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Dataset == "" || seen[r.Dataset] {
			continue
		}
		seen[r.Dataset] = true
		out = append(out, r.Dataset)
	}
	return out, nil
}

// storeDatasetMeta adapts store.Store to kpi.DatasetMetaSource: the as_of
// period watermark (D16-B4) is the NEWEST registered row for the logical
// dataset. "Newest" is decided by CreatedAt (the registry's registration
// instant) — the same ordering an `ORDER BY created_at DESC LIMIT 1` query
// would produce, computed in Go over ListDatasets so no new store method
// (and no new SQL) is introduced for a bounded registry table.
//
// A dataset with zero rows returns an empty Period, which the executor
// turns into a dataset_not_available refusal (never a fabricated as_of).
type storeDatasetMeta struct {
	st store.Store
}

func (m storeDatasetMeta) LatestDatasetVersion(ctx context.Context, dataset string) (kpi.DatasetVersion, error) {
	rows, err := m.st.ListDatasets(ctx)
	if err != nil {
		return kpi.DatasetVersion{}, err
	}
	var (
		best  store.DatasetRow
		found bool
	)
	for _, r := range rows {
		if r.Dataset != dataset {
			continue
		}
		if !found || r.CreatedAt.After(best.CreatedAt) {
			best, found = r, true
		}
	}
	if !found {
		return kpi.DatasetVersion{}, nil
	}
	return kpi.DatasetVersion{
		Period:       best.Period,
		FilePath:     best.FilePath,
		KMinOverride: best.KMinOverride,
	}, nil
}

// shortSignature renders the KPI registry content-hash for the startup log:
// the first 12 hex chars, or an explicit bootstrap marker when the registry
// loaded zero definitions (empty kpis/ directory — the one legal degraded
// state).
func shortSignature(sig kpi.Signature) string {
	s := string(sig)
	if s == "" {
		return "(empty registry — no KPI definitions loaded)"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// prodRunner wraps adk.Runner for the Runner seam. Each Query call
// also passes model.WithTemperature(skill.Temperature) via the pipeline's
// adk.WithChatModelOptions wrapper — we don't add anything here, the
// pipeline owns the option construction so the temperature passthrough
// test can assert on it directly.
type prodRunner struct {
	runner *adk.Runner
	cm     model.ChatModel
	mu     sync.Mutex
	count  int
}

func (r *prodRunner) Query(ctx context.Context, query string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
	return r.runner.Query(ctx, query, opts...)
}

func (r *prodRunner) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}
