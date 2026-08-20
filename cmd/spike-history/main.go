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

// Command spike-history is the spike-only binary for the Phase 9
// history-injection trust-floor spike (GEVL-08, Plan 09-03).
//
// Why a separate binary:
//
//   - Spike rows (history_injection_*) are NOT in the CI gate (D11):
//     adding them to `cmd/eval --frozen` would silently lower the
//     refusal-accuracy gate. The spike is a one-shot product, not a
//     regression suite.
//   - Per D11 (CONTEXT.md), spike runner must NOT reuse `cmd/eval`.
//     This binary deliberately re-implements the bootstrap mirror
//     (Planner Ruling 1) without extracting it to a shared internal
//     package — that refactor is deferred to Phase 10.
//   - Plan 09-04 extends --history-length=long; Plan 09-05 adds
//     --summarize. Both plans operate on this binary's JSONL stream.
//
// Flag matrix (D9-CONTEXT §F + Plan 09-03 §Action(1)):
//
//	--golden          path to JSONL golden set (default eval/golden/compliance.jsonl)
//	--out             output directory for JSONL rows (default reports/)
//	--category        comma list of categories to run (default adversarial,followup)
//	--judge-model     JUDGE_MODEL env override (default glm-5.2)
//	--limit           cap number of cases (0 = no cap)
//	--seed            reserved for Plan 09-04 long-history noise fill (default 42)
//	--with-history    true / false — both are recorded into the JSONL
//	                  (Plan 09-03 only implements true; false is a stub that
//	                   still records baseline-only numbers)
//	--history-length  "short" (Plan 09-03) | "long" (Plan 09-04 stub gate)
//	--summarize       non-empty triggers Plan 09-05 stub gate
//	--cost-limit      hard USD cap for live mode (default 5.00; dry-run ignores)
//	--baseline-lock   path to the Phase 8 frozen compliance.lock.json
//	                  (used by --summarize to reference baseline model_ver
//	                   + prompt_hash; default eval/frozen/compliance.lock.json)
//	--dry-run         run the pipeline with all externals stubbed
//	--via-pipeline    route RunEntry through the production chat path
//	                  (history.Project + rewrite + classifier + multi-turn
//	                   messages prefix). Legacy direct path is the default
//	                   for Phase 9 row-level reproducibility.
//	--frozen          Phase 11 acceptance-gate mode: re-run the spike
//	                  against eval/frozen/compliance.lock.json, compute
//	                  metric drift vs the Phase 8 baselines (compiled
//	                  into frozen.go), and emit reports/spike_<date>.json
//	                  + reports/frozen_lock_<date>.sha256. Exits 0 with
//	                  [FROZEN-RELOCK-PASS] on pass, 1 with
//	                  [FROZEN-RELOCK-FAIL] on drift.
//	--archive         Ship-evidence bundle (requires --frozen=true).
//	                  Additionally writes reports/metrics_diff_<date>.json
//	                  so the operator's audit trail carries the per-metric
//	                  drift table. This is the Phase 11 sign-off artifact.
//
// Per the planner's stub-error discipline (Planner Ruling 2 + 3),
// --history-length=long and --summarize exit with a grep-friendly
// prefix that points to the owning successor plan. Operators who
// hit either gate know exactly which plan owns the feature.
//
// Bootstrap mirrors cmd/eval/main.go:498-609 verbatim (5 helpers:
// buildEmbedder / buildRetriever / buildChatModel / firstManifest /
// buildStore). Each helper carries a 'mirrored from cmd/eval/main.go:L###'
// comment so a Phase 10+ refactor that lifts these into an
// internal/evalcli package can grep for the original line numbers.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
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
	"github.com/skillforge/skillforge/internal/server/classifier"
	"github.com/skillforge/skillforge/internal/server/rewrite"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// Default flag values. Constants (not flag.Defaults) so a future
// refactor can re-bind via env vars without hunting magic strings.
const (
	defaultGoldenPath = "eval/golden/compliance.jsonl"
	defaultOutDir     = "reports"
	defaultJudgeModel = "glm-5.2"
	defaultCostLimit  = 5.00

	defaultSkillsDir   = "skills/"
	defaultCollection  = "skillforge_knowledge"
	defaultSeed        = 42
	defaultHistoryLen  = "short"
	defaultWithHistory = true
	defaultDryRun      = false

	defaultTopK = 5
)

func main() {
	// ---- 1. Flag parsing ----
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s [flags]\n\n"+
				"Spike-only history-injection runner for the Phase 9 trust-floor\n"+
				"hard gate (GEVL-08). Plan 09-03 owns the short-history path; Plan\n"+
				"09-04 owns --history-length=long; Plan 09-05 owns --summarize.\n\nFlags:\n",
			os.Args[0])
		flag.PrintDefaults()
	}
	goldenPath := flag.String("golden", defaultGoldenPath, "path to the golden-set JSONL file")
	outDir := flag.String("out", defaultOutDir, "output directory for per-entry JSONL rows")
	category := flag.String("category", "adversarial,followup", "comma list of categories to run")
	judgeModel := flag.String("judge-model", defaultJudgeModel, "JUDGE_MODEL env override (default glm-5.2)")
	limit := flag.Int("limit", 0, "cap number of cases (0 = no cap)")
	seed := flag.Int("seed", defaultSeed, "RNG seed for Plan 09-04 long-history noise (default 42)")
	withHistory := flag.Bool("with-history", defaultWithHistory, "feed projected history into the prompt (true | false)")
	historyLength := flag.String("history-length", defaultHistoryLen, "short | long (long is Plan 09-04's territory)")
	summarize := flag.String("summarize", "", "non-empty triggers Plan 09-05 summarize stub gate")
	costLimit := flag.Float64("cost-limit", defaultCostLimit, "hard USD cap for live mode (dry-run ignores)")
	baselineLock := flag.String("baseline-lock", "eval/frozen/compliance.lock.json",
		"path to the Phase 8 frozen compliance.lock.json (used by --summarize to reference baseline model_ver + prompt_hash)")
	viaPipeline := flag.Bool("via-pipeline", false, "route RunEntry through the production chat path (history.Project + rewrite + classifier + multi-turn messages prefix)")
	dryRun := flag.Bool("dry-run", defaultDryRun, "run with all externals stubbed (no API keys / no Milvus / no BGE-M3)")
	frozen := flag.Bool("frozen", false, "Phase 11 acceptance gate: re-run against eval/frozen/compliance.lock.json + emit drift report; [FROZEN-RELOCK-PASS] on success")
	archive := flag.Bool("archive", false, "Phase 11 ship-evidence bundle (requires --frozen=true): additionally writes reports/metrics_diff_<date>.json")
	flag.Parse()

	// --archive requires --frozen (the metrics-diff report is a
	// frozen-gate artifact; archiving without the gate would write a
	// meaningless diff against an unset baseline).
	if *archive && !*frozen {
		log.Fatalf("[FAIL] cmd/spike-history: --archive requires --frozen=true")
	}

	// --frozen forces dry-run + memory retriever so the acceptance
	// gate is hermetic on a developer laptop. The gate's purpose is
	// reproducible metric-drift detection, NOT live-LLM re-validation
	// (live re-validation is the quarterly judge-drift sanity check
	// per D6-14 cadence).
	if *frozen {
		*dryRun = true
		if err := os.Setenv("ANTHROPIC_API_KEY", "mock"); err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: set ANTHROPIC_API_KEY=mock: %v", err)
		}
		if err := os.Setenv("RETRIEVER_BACKEND", "memory"); err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: set RETRIEVER_BACKEND=memory: %v", err)
		}
		log.Printf("[INFO] cmd/spike-history: --frozen active — forcing --dry-run + RETRIEVER_BACKEND=memory (hermetic)")
	}

	// Honour --seed across the package (used by future Plan 09-04 long
	// history noise fill). Plan 09-03 records --seed into the JSONL via
	// costState seed-time but does not yet consume it.
	rand.Seed(int64(*seed))

	// Honour --judge-model by setting JUDGE_MODEL before any chat-model
	// construction. NewGLMChatModel reads it at construction time.
	if err := os.Setenv("JUDGE_MODEL", *judgeModel); err != nil {
		log.Fatalf("[FAIL] cmd/spike-history: set JUDGE_MODEL=%s: %v", *judgeModel, err)
	}

	// ---- 2. Stub error gates (Planner Rulings 2 + 3) ----
	// Hit these BEFORE bootstrap so a fast failure does not require
	// GLM_API_KEY / Milvus / BGE-M3 to be present.
	//
	// --summarize routes to runSummarize (Plan 09-05). The flag's
	// value is interpreted as either:
	//   - empty  → summarize skipped.
	//   - "true" / "1" → use --out as the input directory.
	//   - any other non-empty string → that string IS the input
	//     directory (so a developer can `go run ./cmd/spike-history
	//     --summarize=reports/` without changing --out).
	//
	// runSummarize returns 0 on GO / 1 on NO-GO; we os.Exit here so
	// defer cancel() runs (defer fires across os.Exit at the top of a
	// function — but not from arbitrary positions inside main, so the
	// exit must be at the same scope as the defer).
	if *summarize != "" {
		out := *outDir
		if *summarize != "true" && *summarize != "1" {
			out = *summarize
		}
		code := runSummarize(out, *baselineLock)
		os.Exit(code)
	}

	// ---- 3. Bootstrap selection ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var deps *runnerDeps
	if *dryRun {
		// Hermetic — no API keys, no network. StubJudge gives the
		// metrics deterministic scores; embedder / retriever / chat
		// model stay nil (the dry-run branch never touches them).
		deps = &runnerDeps{
			DryRun: true,
		}
		// Plan 10-04: --via-pipeline in dry-run enables the production
		// composition path with inert stub pointers. RunEntry stubs the
		// rewriter (returns query unchanged) and classifier (returns allow)
		// internally when DryRun=true, so zero external calls are made.
		if *viaPipeline {
			deps.Rewriter = &rewrite.QueryRewriter{}
			deps.Classifier = &classifier.RefusalClassifier{}
		}
		log.Printf("[INFO] cmd/spike-history: dry-run mode — all externals stubbed (no API keys / no Milvus / no BGE-M3)")
	} else {
		// Live mode mirrors cmd/eval's bootstrap. envRequired aborts
		// with a clear message if GLM_API_KEY is missing.
		envRequired("GLM_API_KEY")
		emb := buildEmbedder()
		r := buildRetriever(ctx, emb)
		manifest := firstManifest()
		cm := buildChatModel(ctx)
		judge := eval.NewJudge(cm)
		deps = &runnerDeps{
			Embedder:  emb,
			Retriever: r,
			Manifest:  manifest,
			ChatModel: cm,
			Judge:     judge,
			DryRun:    false,
		}
		// Plan 10-04 (B5): when --via-pipeline is set, wire the production
		// query rewriter + refusal classifier so the spike runner exercises
		// the same chat-path composition the running product uses.
		if *viaPipeline {
			rewriter, err := rewrite.NewQueryRewriter(cm)
			if err != nil {
				log.Fatalf("[FAIL] cmd/spike-history: query rewriter init: %v", err)
			}
			refusalClassifier, err := classifier.NewRefusalClassifier(cm)
			if err != nil {
				log.Fatalf("[FAIL] cmd/spike-history: refusal classifier init: %v", err)
			}
			deps.Rewriter = rewriter
			deps.Classifier = refusalClassifier
			log.Printf("[INFO] cmd/spike-history: via-pipeline ACTIVE (rewriter + classifier wired)")
		}
		// buildStore is unused in the spike runner (spike never writes
		// audit / session) but the helper is mirrored for completeness
		// so Phase 10+ can adopt it without a re-derivation.
		_ = buildStore(ctx)
	}

	// ---- 4. Load + filter rows ----
	// Long mode (Plan 09-04) takes a different path: loadLongHistoryRows
	// synthesizes 10 SpikeRows with 8/12/15 history turns. The resulting
	// rows have Category="history_injection_long" so writeResult routes
	// them to long.jsonl (planner ruling 5). The --limit / --cost-limit
	// / main-loop plumbing below is shared between modes.
	var rows []SpikeRow
	if *historyLength == "long" {
		longRows, err := loadLongHistoryRows(*goldenPath, int64(*seed))
		if err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: loadLongHistoryRows: %v", err)
		}
		rows = longRows
		// Override --category so writeResult emits long.jsonl. We
		// deliberately mutate the flag string (not a copy) — there
		// is no other reader of *category after this point.
		*category = "history_injection_long"
		log.Printf("[INFO] cmd/spike-history: long mode: synthesized %d spike rows from seed=%d", len(rows), *seed)
	} else {
		loaded, err := loadSpikeGolden(*goldenPath)
		if err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: load golden %q: %v", *goldenPath, err)
		}
		categories := strings.Split(*category, ",")
		loaded = filterByCategory(loaded, categories)
		rows = loaded
	}
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
		log.Printf("[INFO] cmd/spike-history: --limit=%d capped to %d", *limit, len(rows))
	}
	if len(rows) == 0 {
		log.Fatalf("[FAIL] cmd/spike-history: zero rows after filterByCategory (history-length=%s, categories=%v)", *historyLength, *category)
	}
	log.Printf("[OK] cmd/spike-history: loaded %d rows from %s (history-length=%s, categories=%v)", len(rows), *goldenPath, *historyLength, *category)

	// ---- 5. Main loop ----
	cost := &costState{LimitUSD: *costLimit}
	results := make([]*SpikeResult, 0, len(rows))
	for i, row := range rows {
		if cost.Exceeded() {
			log.Printf("[WARN] cmd/spike-history: cost limit $%.2f exceeded after %d/%d rows — stopping (existing JSONL preserved)",
				*costLimit, i, len(rows))
			break
		}
		res, err := RunEntry(ctx, row, *withHistory, *historyLength, deps, cost)
		if err != nil {
			// Stub gates surface here as explicit errors. We log but
			// do NOT abort — the loop is allowed to continue over
			// subsequent rows that may not hit the gate (in practice
			// only the first stub gate row would error; subsequent
			// rows keep failing the same way, so this is acceptable).
			log.Printf("[WARN] cmd/spike-history: row %d/%d %s: %v", i+1, len(rows), row.ID, err)
			continue
		}
		if err := writeResult(*outDir, row.Category, res); err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: write row %s: %v", row.ID, err)
		}
		results = append(results, res)
		log.Printf("[OK] cmd/spike-history: %d/%d %s [%s] faith=%.3f arel=%.3f cprec=%.3f crecall=%.3f refused=%t grounded=%t inj=%t cost=$%.4f",
			i+1, len(rows), row.ID, row.Category,
			res.Metrics["faithfulness"], res.Metrics["answer_relevancy"],
			res.Metrics["context_precision"], res.Metrics["context_recall"],
			res.Refused, res.GroundingPass, res.InjectionPenetrated, res.CostUSD)
	}
	log.Printf("[OK] cmd/spike-history: wrote %d rows to %s (cost=$%.4f)", len(rows), *outDir, cost.TotalUSD)
	_ = time.Now() // keep time import for header time-stamping later

	// ---- 6. Frozen acceptance gate (Plan 11-08 Task 1) ----
	// Runs AFTER the main loop so the per-row JSONL is already on
	// disk. The gate aggregates the in-memory results, computes drift
	// vs the Phase 8 baseline constants, and emits the ship-evidence
	// bundle. Exit code is the operator's pass/fail signal.
	if *frozen {
		report, err := runFrozenGate(*baselineLock, rows, results, *archive, *dryRun)
		if err != nil {
			log.Fatalf("[FROZEN-LOCK-MISMATCH] cmd/spike-history: %v", err)
		}
		if err := writeFrozenReports(*outDir, report, *archive); err != nil {
			log.Fatalf("[FROZEN-RELOCK-FAIL] cmd/spike-history: write reports: %v", err)
		}
		if !report.Pass {
			for _, line := range report.DriftedMetrics {
				log.Printf("[FROZEN-RELOCK-FAIL] drifted: %s", line)
			}
			log.Fatalf("[FROZEN-RELOCK-FAIL] cmd/spike-history: %d metric(s) drifted beyond tolerance", len(report.DriftedMetrics))
		}
		log.Printf("[FROZEN-RELOCK-PASS] cmd/spike-history: %d cases, lock_sha256=%s, drift within tolerance", report.CaseCount, report.LockSHA256)
	}
}

// buildEmbedder mirrors cmd/server's BGE-M3 construction. Required
// env: BGE_M3_URL. Optional: BGE_M3_MODEL (default "bge-m3"),
// BGE_M3_TIMEOUT (default 60s).
//
// Mirrored from cmd/eval/main.go:501-510.
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

// buildRetriever mirrors cmd/server's RETRIEVER_BACKEND switch
// (milvus default, mongo optional).
//
// Mirrored from cmd/eval/main.go:513-550.
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
			log.Fatalf("[FAIL] cmd/spike-history: mongo dial %s: %v", mURI, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: mongo ensure: %v", err)
		}
		if err := mc.SeedFromKnowledgeDir(ctx, "knowledge/", emb); err != nil {
			log.Printf("[WARN] cmd/spike-history: mongo seed: %v", err)
		}
		return mc
	default:
		if backend != "milvus" {
			log.Printf("[WARN] cmd/spike-history: unknown RETRIEVER_BACKEND=%q, falling back to milvus", backend)
		}
		addr := envRequired("MILVUS_ADDRESS")
		cfg := milvus.Config{Addr: addr, CollectionName: defaultCollection}
		mc, err := milvus.New(ctx, cfg)
		if err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: milvus dial %s: %v", addr, err)
		}
		if err := mc.Ensure(ctx); err != nil {
			log.Fatalf("[FAIL] cmd/spike-history: milvus ensure: %v", err)
		}
		if err := knowledge.Index(ctx, "knowledge/", emb, mc.ClientOf(), cfg); err != nil {
			log.Printf("[WARN] cmd/spike-history: knowledge index: %v", err)
		}
		return mc
	}
}

// buildChatModel constructs the chat model. Reads GLM_API_KEY
// (required) + GLM_BASE_URL (default Coding Plan endpoint).
//
// Mirrored from cmd/eval/main.go:555-563.
func buildChatModel(ctx context.Context) model.ToolCallingChatModel {
	cm, err := eval.NewGLMChatModel(ctx)
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-history: %v", err)
	}
	return cm
}

// buildAgent is the spike runner's full agent (kept in main for the
// eventual Phase 10 wiring that lets the spike re-use the production
// agent end-to-end). NOT called by Plan 09-03 — Plan 09-03 needs
// preRetrieve outside the ReAct loop (Planner Ruling 6 — agent.Run
// does not accept history). The helper is included so a future plan
// that flips to agent.Run has the bootstrap ready.
//
// Mirrored from cmd/eval/main.go:568-580.
func buildAgent(ctx context.Context, manifest *skill.Manifest, emb embedder.Embedder, r retriever.Retriever, cm model.ToolCallingChatModel) *agent.Agent {
	ag, err := agent.NewAgent(ctx, agent.AgentConfig{
		Manifest:  manifest,
		Embedder:  emb,
		Retriever: r,
		ChatModel: cm,
		MaxHops:   2,
	})
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-history: agent init: %v", err)
	}
	return ag
}

// buildStore mirrors cmd/server's SQLite store construction. NOT
// called by the spike runner (spike never writes audit / session
// rows; Plan 09-04/09-05 may adopt it). Kept here for completeness
// so the bootstrap set is exhaustive across future plans.
//
// Mirrored from cmd/eval/main.go:583-594.
func buildStore(ctx context.Context) *sqlite.SQLiteStore {
	dsn := envOr("SQLITE_DSN", "skillforge.db")
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-history: sqlite.New(%q): %v", dsn, err)
	}
	if err := s.Health(ctx); err != nil {
		_ = s.Close()
		log.Fatalf("[FAIL] cmd/spike-history: store health: %v", err)
	}
	return s
}

// firstManifest loads skills/ and returns the first Skill manifest
// (the compliance Skill in Phase 8/9).
//
// Mirrored from cmd/eval/main.go:599-609.
func firstManifest() *skill.Manifest {
	loader := skill.NewLoader(defaultSkillsDir)
	if _, err := loader.Scan(); err != nil {
		log.Fatalf("[FAIL] cmd/spike-history: skill scan: %v", err)
	}
	for _, m := range loader.Skills() {
		return m
	}
	log.Fatalf("[FAIL] cmd/spike-history: zero Skills loaded (the agent requires a manifest)")
	return nil
}

// envRequired reads a required env var; aborts with a clear message
// on missing. Mirrors cmd/server's helper.
func envRequired(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("[FAIL] cmd/spike-history: %s env var not set (copy .env.example to .env and populate)", name)
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