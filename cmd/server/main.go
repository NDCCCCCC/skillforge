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
//       "milvus" (default) — milvus.New (bounded retries) + Ensure + Index.
//       "mongo"            — mongo.New (Connect+Ping) + Ensure +
//                            SeedFromKnowledgeDir. Used when the internal
//                            Milvus is unavailable; MongoDB 4.4+ has no
//                            native vector search, so cosine is computed
//                            in Go against the tier-filtered rows.
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
//   - GLM_API_KEY       (required) — Zhipu Coding Plan subscription key
//   - GLM_BASE_URL      (optional) — default https://open.bigmodel.cn/api/coding/paas/v4
//   - SERVER_ADDR       (optional) — default ":8888" (Pitfall 7)
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
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/skillforge/skillforge/internal/retriever/mongo"
	"github.com/skillforge/skillforge/internal/server"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// env-or-default helpers. Each required var aborts startup on empty.
const (
	defaultGLMBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	defaultServerAddr = ":8888"
	defaultCollection = "skillforge_knowledge"
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

func main() {
	ctx := context.Background()

	// 1. Env var parsing. MILVUS_ADDRESS / MONGO_URI are validated inside
	// the backend switch (only the configured backend's URI is required).
	bgeURL := envRequired("BGE_M3_URL")
	bgeModel := envOr("BGE_M3_MODEL", "bge-m3")
	bgeTimeoutSec := envOr("BGE_M3_TIMEOUT", "60")
	glmKey := envRequired("GLM_API_KEY")
	glmBase := envOr("GLM_BASE_URL", defaultGLMBaseURL)
	serverAddr := envOr("SERVER_ADDR", defaultServerAddr)

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
	emb := bge.New(bge.Config{URL: bgeURL, Model: bgeModel, Timeout: bgeTimeout})

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

	// 6b. Phase 3 ReAct Agent (Plan 03-03 — supersedes adk.NewChatModelAgent
	// for the /api/chat path). The adk agent + runner above stay in scope
	// because the eval golden-set runner (Plan 03-06) may use the simpler
	// single-turn Runner.Query to drive 50 cases without the full ReAct
	// loop; keeping both available preserves the option (per Plan 03-05).
	//
	// The agent's Manifest is the first loaded Skill (Phase 2 has exactly
	// one Skill — compliance; multi-Skill routing is Phase 5). MaxHops=2
	// is the D3-02 hard cap (ComputeMaxStep(2)=5 LLM turns max).
	//
	// Emit is NOT bound here — internal/agent's DecisionEmitFunc requires
	// the per-request *app.RequestContext, which does not exist at server
	// startup. The chatHandler wraps emitDecision(c, ...) into a closure
	// per request (see internal/server/chat.go runAgentPath).
	ag, err := agent.NewAgent(ctx, agent.AgentConfig{
		Manifest:  firstManifest(loader),
		Embedder:  emb,
		Retriever: r,
		ChatModel: cm,
		MaxHops:   2, // D3-02 hard cap — see agent.ComputeMaxStep
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: agent init: %v", err)
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
	deps := &server.Deps{
		Loader:    loader,
		Embedder:  emb,
		Retriever: r,
		Runner:    &prodRunner{runner: runner, cm: cm},
		Agent:     ag,
		Store:     st,
		Wizard:    skill.NewWizard(cm, "skills/"),
	}

	// 8. Hertz server (Pattern 1).
	h := server.New(serverAddr, deps)
	log.Printf("[OK] cmd/server: SkillForge listening on %s (backend=%s)", serverAddr, backendLabel)
	log.Printf("     POST /api/chat                — verdict pipeline (verdict or structured refusal)")
	log.Printf("     GET  /api/skills              — list loaded Skills")
	log.Printf("     POST /api/skills              — dry-run Skill validator")
	log.Printf("     POST /api/skills/wizard       — SC-3 wizard (5-step interview -> preview)")
	log.Printf("     POST /api/skills/wizard/commit — atomic write (fsnotify closes the loop)")
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