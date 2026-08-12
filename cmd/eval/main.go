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

// Command eval is the Plan 03-06 Go-native trust evaluator CLI. It
// drives the SkillForge agent over a golden-set JSONL file, computes the
// 4 RAG-trust metrics (faithfulness / answer_relevancy / context_precision
// / context_recall), writes a JSON report + calibration data, and
// backfills the metric columns on any matching audit rows.
//
// Why a separate binary (vs. an HTTP endpoint on cmd/server):
//
//   - The eval run is a BATCH job (50+ LLM calls), not a streaming
//     request. Running it in the same process as cmd/server would
//     block the chat path.
//   - D3-08 sidecar shape: the eval is a CLI batch + JSON report,
//     NOT an always-on HTTP service. The Phase 6 CI promotion gate
//     (faithfulness < 0.9 blocks Skill promotion — GEVL-07) consumes
//     the JSON report this binary writes.
//
// Bootstrap mirrors cmd/server: same env vars (BGE_M3_URL, GLM_API_KEY,
// GLM_BASE_URL, MILVUS_ADDRESS / MONGO_URI, SQLITE_DSN), same bge +
// retriever + agent + store construction. The judge uses JUDGE_MODEL
// (default glm-5.2) — the D3-07 slot for Phase 6 cross-evaluation.
//
// Per-case flow:
//
//  1. embedder.Embed(query) → queryVec.
//  2. retriever.Search(ctx, queryVec, tiers, topK) → scored Hits (for
//     context_precision/recall inputs + calibration MaxScore).
//  3. agent.Run(ctx, sessionID, query) → verdict (+ refusal);
//     agent.LastRetrieved() → the agent's actual chunks (primary metric
//     input — these are what the agent saw, not a parallel retrieval).
//  4. Run the 4 metrics over (verdict, contexts, hits, query).
//  5. If a matching AuditRow exists (same session_id+query), backfill
//     the metric columns via store.UpdateAuditMetrics.
//  6. Accumulate CaseScore{MaxScore, Refused} for calibration.
//
// At end: WriteReport + Calibrate → eval/report.json + eval/calibration.json.
//
// Exit codes: 0 on success, 1 on configuration / runtime error. A
// high-faithfulness deviation (e.g., faithfulness < 0.9 average) does
// NOT exit non-zero in Phase 3 — the CI promotion gate is Phase 6; here
// we only emit the data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/skillforge/skillforge/internal/retriever/mongo"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/verdict"
)

// Default flag values. Kept as constants (not flag.Defaults) so a future
// refactor can re-bind them via env vars without hunting for magic
// strings.
const (
	defaultGoldenPath      = "eval/golden/compliance.jsonl"
	defaultReportPath      = "eval/report.json"
	defaultCalibrationPath = "eval/calibration.json"
	defaultTopK            = 5
	defaultCollection      = "skillforge_knowledge"
)

func main() {
	goldenPath := flag.String("golden", defaultGoldenPath, "path to the golden-set JSONL file")
	reportPath := flag.String("report", defaultReportPath, "output path for the JSON report")
	calibrationPath := flag.String("calibration", defaultCalibrationPath, "output path for the calibration JSON")
	limit := flag.Int("limit", 0, "cap the number of cases processed (0 = no cap; debug aid)")
	topK := flag.Int("topk", defaultTopK, "top-K retriever hits per case (mirrors cmd/server)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Bootstrap: mirror cmd/server's wiring (env vars + construction).
	// Any failure here is fatal — same fail-fast contract as cmd/server.
	emb := buildEmbedder()
	r := buildRetriever(ctx, emb)
	manifest := firstManifest()
	cm := buildChatModel(ctx)         // agent's chat model (GLM_API_KEY)
	judgeCM := buildChatModel(ctx)    // judge's chat model (same key; JUDGE_MODEL slot via NewGLMChatModel)
	ag := buildAgent(ctx, manifest, emb, r, cm)
	st := buildStore(ctx)
	judge := eval.NewJudge(judgeCM)

	// 2. Load the golden set.
	cases, err := eval.LoadGolden(*goldenPath)
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: load golden %q: %v", *goldenPath, err)
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
		log.Printf("[INFO] cmd/eval: --limit=%d capped cases to %d", *limit, len(cases))
	}
	log.Printf("[OK] cmd/eval: loaded %d golden cases from %s", len(cases), *goldenPath)

	// 3. Per-case loop.
	tiers := manifest.ResolvedTiers()
	results := make([]eval.CaseResult, 0, len(cases))
	scores := make([]eval.CaseScore, 0, len(cases))
	verdictBytes, _ := json.Marshal(verdict.Verdict{})
	_ = verdictBytes

	for i, c := range cases {
		caseCtx, caseCancel := context.WithTimeout(ctx, 90*time.Second)
		sessionID := "eval-" + c.ID

		// 3a. Parallel retriever.Search for the calibration MaxScore +
		// the context_precision/recall inputs. This is the SAME search
		// the agent's retrieve tool would do internally; we re-run it
		// here to get the scored Hit slice (the agent does not expose
		// scores on its public API, only via LastRetrieved).
		queryVec, err := emb.Embed(caseCtx, c.Query)
		var hits []retriever.Hit
		var maxScore float32
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s embed: %v — metrics will score 0 context", c.ID, err)
		} else {
			hits, err = r.Search(caseCtx, queryVec, tiers, *topK)
			if err != nil {
				log.Printf("[WARN] cmd/eval: case %s search: %v — metrics will score 0 context", c.ID, err)
			} else {
				for _, h := range hits {
					if h.Score > maxScore {
						maxScore = h.Score
					}
				}
			}
		}

		// 3b. Run the agent. The agent's retrieve tool does its OWN
		// embed+search internally; LastRetrieved exposes what it saw.
		v, refusal, err := ag.Run(caseCtx, sessionID, c.Query)
		refused := false
		var answer string
		var contexts []string
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s agent.Run: %v — scoring as refusal", c.ID, err)
			refused = true
		} else if refusal != nil {
			refused = true
		} else {
			// Serialize the verdict to an answer string for the judge.
			// The verdict's Citations carry the load-bearing claims;
			// including the full JSON lets the judge see status +
			// confidence too.
			if b, mErr := json.Marshal(v); mErr == nil {
				answer = string(b)
			}
			// The agent's actual retrieved chunks — primary metric input.
			for _, ch := range ag.LastRetrieved() {
				contexts = append(contexts, ch.Content)
			}
		}
		caseCancel()

		// 3c. Compute the 4 metrics. Errors here are per-case, not
		// fatal — log and score 0 for the failing metric.
		faith, err := eval.Faithfulness(ctx, answer, contexts, judge)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s faithfulness: %v — scoring 0", c.ID, err)
			faith = 0
		}
		arel, err := eval.AnswerRelevancy(ctx, c.Query, answer, emb, judge)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s answer_relevancy: %v — scoring 0", c.ID, err)
			arel = 0
		}
		cprec, err := eval.ContextPrecision(ctx, c.Query, hits, judge)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s context_precision: %v — scoring 0", c.ID, err)
			cprec = 0
		}
		crecall, err := eval.ContextRecall(ctx, c.GroundTruth, hits, judge, refused)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s context_recall: %v — scoring 0", c.ID, err)
			crecall = 0
		}

		// 3d. Backfill the audit row if one exists for this session.
		backfillAuditMetrics(ctx, st, sessionID, c.Query, faith, cprec, arel, crecall)

		// 3e. Accumulate.
		sample := eval.Sample{
			Case:          c,
			Answer:        answer,
			Contexts:      contexts,
			RefusalReason: refusalReasonStr(refusal, refused),
		}
		results = append(results, eval.SampleToResult(sample, faith, cprec, arel, crecall))
		scores = append(scores, eval.CaseScore{MaxScore: maxScore, Refused: refused})

		log.Printf("[OK] cmd/eval: case %d/%d %s [%s] faith=%.3f arel=%.3f cprec=%.3f crecall=%.3f refused=%t maxScore=%.3f",
			i+1, len(cases), c.ID, c.Category, faith, arel, cprec, crecall, refused, maxScore)
	}

	// 4. Aggregate + write report + calibration.
	report := buildReport(results)
	if err := eval.WriteReport(*reportPath, report); err != nil {
		log.Fatalf("[FAIL] cmd/eval: write report %q: %v", *reportPath, err)
	}
	rec := eval.Calibrate(scores)
	if err := writeCalibration(*calibrationPath, rec); err != nil {
		log.Fatalf("[FAIL] cmd/eval: write calibration %q: %v", *calibrationPath, err)
	}

	log.Printf("[OK] cmd/eval: %d cases scored. Report: %s. Calibration: %s (recommended_threshold=%.4f).",
		len(cases), *reportPath, *calibrationPath, rec.RecommendedThreshold)
}

// buildEmbedder mirrors cmd/server's BGE-M3 construction. The same env
// vars (BGE_M3_URL required; BGE_M3_MODEL default "bge-m3"; BGE_M3_TIMEOUT
// default 60s).
func buildEmbedder() *bge.BGE {
	bgeURL := envRequired("BGE_M3_URL")
	bgeModel := envOr("BGE_M3_MODEL", "bge-m3")
	bgeTimeoutSec := envOr("BGE_M3_TIMEOUT", "60")
	to, err := time.ParseDuration(bgeTimeoutSec + "s")
	if err != nil || to <= 0 {
		to = 60 * time.Second
	}
	return bge.New(bge.Config{URL: bgeURL, Model: bgeModel, Timeout: to})
}

// buildRetriever mirrors cmd/server's RETRIEVER_BACKEND switch.
func buildRetriever(ctx context.Context, emb *bge.BGE) retriever.Retriever {
	backend := strings.ToLower(os.Getenv("RETRIEVER_BACKEND"))
	if backend == "" {
		backend = "milvus"
	}
	switch backend {
	case "mongo":
		mURI := envRequired("MONGO_URI")
		mc, err := mongo.New(ctx, mongo.Config{URI: mURI, Database: "skillforge", Collection: "knowledge"})
		if err != nil {
			log.Fatalf("[FAIL] cmd/eval: mongo dial %s: %v", mURI, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/eval: mongo ensure: %v", err)
		}
		if err := mc.SeedFromKnowledgeDir(ctx, "knowledge/", emb); err != nil {
			log.Printf("[WARN] cmd/eval: mongo seed: %v", err)
		}
		return mc
	default:
		if backend != "milvus" {
			log.Printf("[WARN] cmd/eval: unknown RETRIEVER_BACKEND=%q, falling back to milvus", backend)
		}
		addr := envRequired("MILVUS_ADDRESS")
		cfg := milvus.Config{Addr: addr, CollectionName: defaultCollection}
		mc, err := milvus.New(ctx, cfg)
		if err != nil {
			log.Fatalf("[FAIL] cmd/eval: milvus dial %s: %v", addr, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/eval: milvus ensure: %v", err)
		}
		if err := knowledge.Index(ctx, "knowledge/", emb, mc.ClientOf(), cfg); err != nil {
			log.Printf("[WARN] cmd/eval: knowledge index: %v", err)
		}
		return mc
	}
}

// buildChatModel constructs the agent's chat model. Reads GLM_API_KEY
// (required) + GLM_BASE_URL (default Coding Plan endpoint). Returns the
// model.ToolCallingChatModel that agent.NewAgent requires.
func buildChatModel(ctx context.Context) model.ToolCallingChatModel {
	cm, err := eval.NewGLMChatModel(ctx)
	if err != nil {
		// NewGLMChatModel already wrapped the env-var-missing case in a
		// clear message — surface it directly.
		log.Fatalf("[FAIL] cmd/eval: %v", err)
	}
	return cm
}

// buildAgent mirrors cmd/server's agent construction (MaxHops=2 D3-02
// hard cap). The AgentConfig.Emit field is left nil — the eval CLI runs
// in batch mode and does NOT emit data-decision SSE chunks.
func buildAgent(ctx context.Context, manifest *skill.Manifest, emb embedder.Embedder, r retriever.Retriever, cm model.ToolCallingChatModel) *agent.Agent {
	ag, err := agent.NewAgent(ctx, agent.AgentConfig{
		Manifest:  manifest,
		Embedder:  emb,
		Retriever: r,
		ChatModel: cm,
		MaxHops:   2,
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: agent init: %v", err)
	}
	return ag
}

// buildStore mirrors cmd/server's SQLite store construction.
func buildStore(ctx context.Context) store.Store {
	dsn := envOr("SQLITE_DSN", "skillforge.db")
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: sqlite.New(%q): %v", dsn, err)
	}
	if err := s.Health(ctx); err != nil {
		_ = s.Close()
		log.Fatalf("[FAIL] cmd/eval: store health: %v", err)
	}
	return s
}

// firstManifest loads skills/ and returns the first Skill manifest
// (Phase 3 has exactly one Skill — compliance; multi-Skill routing is
// Phase 5). Aborts on error.
func firstManifest() *skill.Manifest {
	loader := skill.NewLoader("skills/")
	if _, err := loader.Scan(); err != nil {
		log.Fatalf("[FAIL] cmd/eval: skill scan: %v", err)
	}
	for _, m := range loader.Skills() {
		return m
	}
	log.Fatalf("[FAIL] cmd/eval: zero Skills loaded (the agent requires a manifest)")
	return nil
}

// backfillAuditMetrics queries the store for an audit row matching the
// (sessionID, query) tuple and — if exactly one is found — backfills
// the 4 metric columns. Multiple matches → the most recent is updated
// (newest-first QueryAudit order). Zero matches → no-op (the eval CLI
// is NOT responsible for inserting audit rows; it only backfills).
//
// A failure here is logged but not fatal — the report+calibration are
// still written. The audit backfill is a convenience for cross-run
// forensics, not a deliverable.
func backfillAuditMetrics(ctx context.Context, st store.Store, sessionID, query string, faith, cprec, arel, crecall float64) {
	rows, err := st.QueryAudit(ctx, store.AuditFilter{SessionID: sessionID, Limit: 50})
	if err != nil {
		log.Printf("[WARN] cmd/eval: audit query (%s): %v — skipping backfill", sessionID, err)
		return
	}
	// Find the most recent row whose Query matches exactly. The eval
	// session_id is unique per case, so there is typically 0 or 1.
	for _, r := range rows {
		if r.Query == query {
			if err := st.UpdateAuditMetrics(ctx, r.ID, &faith, &cprec, &arel, &crecall); err != nil {
				log.Printf("[WARN] cmd/eval: audit backfill (%s, id=%d): %v", sessionID, r.ID, err)
			}
			return
		}
	}
}

// buildReport aggregates the per-case results into an eval.Report:
// overall means + per-category means. Faithfulness on refusal cases is
// included in the overall mean (faithfulness=1.0 for refusals by
// convention, so a healthy eval shows faithfulness ≈ 1.0).
func buildReport(results []eval.CaseResult) eval.Report {
	metrics := []string{"faithfulness", "context_precision", "answer_relevance", "context_recall"}
	sumOverall := map[string]float64{}
	sumPerCat := map[string]map[string]float64{}
	countPerCat := map[string]int{}

	for _, r := range results {
		countPerCat[r.Category]++
		if _, ok := sumPerCat[r.Category]; !ok {
			sumPerCat[r.Category] = map[string]float64{}
		}
		sumOverall["faithfulness"] += r.Faithfulness
		sumOverall["context_precision"] += r.ContextPrecision
		sumOverall["answer_relevance"] += r.AnswerRelevance
		sumOverall["context_recall"] += r.ContextRecall
		sumPerCat[r.Category]["faithfulness"] += r.Faithfulness
		sumPerCat[r.Category]["context_precision"] += r.ContextPrecision
		sumPerCat[r.Category]["answer_relevance"] += r.AnswerRelevance
		sumPerCat[r.Category]["context_recall"] += r.ContextRecall
	}

	overall := map[string]float64{}
	for _, m := range metrics {
		if len(results) > 0 {
			overall[m] = round4(sumOverall[m] / float64(len(results)))
		}
	}

	perCat := map[string]map[string]float64{}
	for cat, n := range countPerCat {
		per := map[string]float64{}
		for _, m := range metrics {
			per[m] = round4(sumPerCat[cat][m] / float64(n))
		}
		perCat[cat] = per
	}

	return eval.Report{
		Overall:     overall,
		PerCategory: perCat,
		Cases:       results,
	}
}

// writeCalibration serializes the Recommendation to JSON. Pretty-
// printed so the operator can read the rationale without piping through
// jq.
func writeCalibration(path string, r eval.Recommendation) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// refusalReasonStr flattens a *verdict.Refusal to its reason code for
// the Sample. Returns "" for non-refusals.
func refusalReasonStr(refusal *verdict.Refusal, refused bool) string {
	if !refused {
		return ""
	}
	if refusal != nil {
		return refusal.Reason
	}
	return "agent_error"
}

// round4 rounds to 4 decimal places — enough precision for a trust
// metric (0.9234 reads as "92.34% faithful") without floating-point
// noise (0.92340000000001).
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// envRequired reads a required env var; aborts with a clear message on
// missing. Mirrors cmd/server's helper of the same name.
func envRequired(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("[FAIL] cmd/eval: %s env var not set (copy .env.example to .env and populate)", name)
	}
	return v
}

// envOr reads an env var with a fallback. Mirrors cmd/server's helper.
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// Ensure errors / verdict packages are referenced (lint guards against
// future import removal by automated tooling).
var (
	_ = errors.New
	_ = verdict.Verdict{}
)
