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
// retriever, skill, knowledge} packages into a single binary.
//
// Startup sequence (fail fast on any error — the operator must see the
// problem at startup, never at request time):
//
//  1. Load skills via skill.NewLoader("skills/").Scan(). An invalid
//     Skill is fatal — the operator should see the SYSK-02 violation
//     in the startup log, not as a confusing 500 later.
//  2. Dial Milvus via retriever.NewMilvusClient (bounded retries).
//     BGE-M3 outage is NOT fatal at startup (Pitfall 8 — production
//     refuses on BGE-M3 outage at request time, not startup time, so
//     the server can boot into a known-broken state and operators can
//     see the failure mode in the first chat request).
//  3. EnsureCollection + Index the knowledge/ corpus into Milvus
//     (one-doc-one-chunk Pattern 7). Idempotent: re-running inserts
//     duplicates (Milvus INT64 PK is not auto-deduped), but the
//     startup hook is safe for a fresh deployment.
//  4. Build the GLM-5.2 ChatModel via openai.NewChatModel pointed at
//     the Coding Plan endpoint (Pitfall 6 — NOT /api/paas/v4, the
//     standard endpoint rejects Coding Plan keys with a confusing
//     "余额不足" error).
//  5. Wrap the ChatModel in adk.NewChatModelAgent + adk.NewRunner.
//  6. Build *server.Deps, h := server.New(addr, deps), h.Spin().
//
// Environment variables (the deployment surface):
//
//   - MILVUS_ADDRESS  (required) — gRPC host:port of internal Milvus
//   - BGE_M3_URL      (required) — internal BGE-M3 HTTP endpoint
//   - GLM_API_KEY     (required) — Zhipu Coding Plan subscription key
//   - GLM_BASE_URL    (optional) — default https://open.bigmodel.cn/api/coding/paas/v4
//   - SERVER_ADDR     (optional) — default ":8888" (Pitfall 7)
//
// All five are read at startup; missing required vars are fatal with
// an actionable error message. The Skill author / operator should be
// able to fix the issue from the log alone.
package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/milvus-io/milvus-sdk-go/v2/client"

	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/server"
	"github.com/skillforge/skillforge/internal/skill"
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

	// 1. Env var parsing.
	milvusAddr := envRequired("MILVUS_ADDRESS")
	bgeURL := envRequired("BGE_M3_URL")
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

	// 3. Milvus client.
	mc, err := retriever.NewMilvusClient(ctx, retriever.MilvusConfig{
		Addr:           milvusAddr,
		CollectionName: defaultCollection,
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: milvus dial %s: %v", milvusAddr, err)
	}
	mcfg := retriever.MilvusConfig{
		Addr:           milvusAddr,
		CollectionName: defaultCollection,
		BGEURL:         bgeURL,
	}

	if err := retriever.EnsureCollection(ctx, mc, mcfg); err != nil {
		log.Fatalf("[FAIL] cmd/server: ensure collection: %v", err)
	}

	// 4. Index knowledge/ corpus (one-doc-one-chunk, Pattern 7).
	if err := knowledge.Index(ctx, "knowledge/", mc, mcfg, bgeURL); err != nil {
		// Non-fatal — Index errors are most often BGE-M3 outages, and
		// the retrieval path will refuse with no_authorized_source when
		// the collection is empty. We log and continue; an operator can
		// manually re-index once the embed service is back.
		log.Printf("[WARN] cmd/server: knowledge index failed (retrieval will return no_authorized_source until re-indexed): %v", err)
	} else {
		log.Printf("[OK] knowledge corpus indexed into %s", defaultCollection)
	}

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
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "skillforge-compliance",
		Description: "SkillForge compliance-review ChatModelAgent",
		Instruction: "You are a strict compliance auditor. Output ONLY the JSON object specified by the runtime system prompt.",
		Model:       cm,
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/server: adk agent init: %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})

	// 7. Build *server.Deps. Embedder + Searcher wrap the production
	// retriever clients; Runner wraps the Eino Runner.
	deps := &server.Deps{
		Loader:    loader,
		Milvus:    mc,
		MilvusCfg: mcfg,
		BGEURL:    bgeURL,
		Embedder:  prodEmbedder{url: bgeURL},
		Searcher:  prodSearcher{mc: mc, cfg: mcfg},
		Runner:    &prodRunner{runner: runner, cm: cm},
	}

	// 8. Hertz server (Pattern 1).
	h := server.New(serverAddr, deps)
	log.Printf("[OK] cmd/server: SkillForge listening on %s", serverAddr)
	log.Printf("     POST /api/chat       — verdict pipeline (verdict or structured refusal)")
	log.Printf("     GET  /api/skills     — list loaded Skills (added in 02-04)")
	log.Printf("     POST /api/skills     — dry-run Skill validator (added in 02-04)")
	// h.Spin() blocks until SIGINT/SIGTERM; the return is unreachable
	// in production but the lint guards us from a future "if err" insert.
	h.Spin()
}

// prodEmbedder wraps retriever.EmbedQuery for the Embedder seam. NO
// fallback to EmbedQueryDeterministic — production refuses on BGE-M3
// outage (Pitfall 8 / T-2-25).
type prodEmbedder struct {
	url string
}

func (e prodEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return retriever.EmbedQuery(ctx, e.url, text)
}

// prodSearcher wraps retriever.SearchWithTierFilter for the Searcher
// seam. The pipeline passes a filter expression it built from
// retriever.BuildMilvusFilter; we forward verbatim to the SDK.
type prodSearcher struct {
	mc  client.Client
	cfg retriever.MilvusConfig
}

func (s prodSearcher) SearchWithFilter(ctx context.Context, queryVec []float32, filter string, topK int) ([]retriever.TierSearchResult, error) {
	return retriever.SearchWithTierFilter(ctx, s.mc, s.cfg, queryVec, filter, topK)
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