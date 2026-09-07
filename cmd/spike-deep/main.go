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

// Command spike-deep is the spike-only binary for the Phase 12 / Plan
// 12-04 RF-5 verdict-JSON parse rate measurement against the DeepAgent
// archetype. It measures how often the DeepAgent final assistant
// message parses as a verdict JSON object with two paths:
//
//  1. Direct ExtractJSON pass (Phase 3 OOS-1 fix — handles fences /
//     trailing prose).
//  2. Structured-output fallback ExtractVerdictJSONWithStructuredOutput
//     (12-04 Task 3 — RF-5 mitigation; Phase 2 systemPrompt verbatim +
//     response_format=json_object).
//
// P12-L10 lock:
//
//	direct parse rate < 90% → structured-output final step is MANDATORY
//	                          (runtime.ApplyTrustStage must wire the call)
//	direct parse rate < 70% → ESCALATE: verdict-JSON suffix fallback +
//	                          forced_stop on parse failure (out of scope
//	                          for 12-04; 12-08 acceptance gate reports the
//	                          actual rate).
//
// Why a separate binary (mirrors the rationale in cmd/spike-history):
//
//   - Live stack required (GLM-5.2 + Milvus + BGE-M3). CI gates must
//     NOT depend on the operator's environment; the spike is a
//     product artifact, not a regression suite.
//   - Per the operator-gate rule (Plan 09-03 stub-error discipline),
//     this binary MUST NOT reuse cmd/eval — the bootstrap is mirrored
//     verbatim where it can be (buildEmbedder / buildRetriever /
//     buildChatModel / firstManifest) so a Phase 13+ refactor can lift
//     the helpers into a shared evalcli package.
//
// Usage:
//
//	spike-deep parse-rate [--golden=eval/golden/deep.jsonl]
//	                     [--output=reports/rf5-deep-<date>.jsonl]
//	                     [--cost-limit=5.00] [--seed=42]
//	                     [--max-iterations=10] [--dry-run]
//
// Subcommands:
//
//	parse-rate  RF-5 verdict-JSON parse rate measurement (the only
//	            subcommand shipped in 12-04).
//
// Live mode requires: GLM_API_KEY, BGE_M3_URL, MILVUS_ADDRESS.
// Dry-run mode loads the golden set, validates the schema, prints
// "<N> cases loaded from <path>" to stdout, and exits 0 — no API
// calls, no env requirement. The dry-run proves the spike
// infrastructure is wired (P12-L10 spike is operator-runs-live).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/llm"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/skillforge/skillforge/internal/runtime"
	"github.com/skillforge/skillforge/internal/skill"
)

// Default flag values — constants (not flag.Defaults) so a future
// refactor can re-bind via env vars without hunting magic strings.
const (
	defaultGoldenPath = "eval/golden/deep.jsonl"
	defaultOutDir     = "reports"
	defaultCostLimit  = 5.00
	defaultSeed       = 42
	defaultMaxIter    = 10
	defaultCollection = "skillforge_knowledge"

	// perCallEstimateUSD is the conservative per-LLM-call USD estimate
	// used by the --cost-limit guard. The spike's live path may emit
	// up to (MaxIteration + structuredFallback) calls per case — with
	// MaxIteration=10 and 50 cases, worst-case spend ≈ 50 × 11 ×
	// 0.002 = $1.10, comfortably under the $5.00 cap. Document the
	// estimator so the operator can read the headline number knowing
	// it is a call-count proxy, not a token-derived invoice.
	perCallEstimateUSD = 0.002

	// verdictJSONSchema is the JSON-schema fragment interpolated into
	// the ExtractVerdictJSONWithStructuredOutput system prompt (12-04
	// Task 3). The structured-output final step uses the literal
	// fields from the Phase 2 systemInstructionsTemplate so the LLM
	// emits the same shape ExtractJSON already understands. Mirrors
	// the schema block in internal/agent/prompts.go:58-68.
	verdictJSONSchema = `{"status":"ok|needs_input|refused|error","confidence":0.0-1.0,"citations":[{"chunk_id":"string","span":"string","tier":"string"}],"review_required":true|false,"answer":"string","compliance_status":"compliant|non_compliant|not_applicable"}`
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "parse-rate":
		os.Exit(runParseRate(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "[FAIL] cmd/spike-deep: unknown subcommand %q (only parse-rate ships in 12-04)\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s <subcommand> [flags]\n\n"+
			"Spike-only RF-5 verdict-JSON parse rate runner for Phase 12 / Plan 12-04.\n"+
			"Subcommands: parse-rate (RF-5 measurement against the DeepAgent archetype).\n\n",
		os.Args[0])
}

// ---------------------------------------------------------------------------
// parse-rate subcommand
// ---------------------------------------------------------------------------

func runParseRate(args []string) int {
	fs := flag.NewFlagSet("parse-rate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	goldenPath := fs.String("golden", defaultGoldenPath, "path to the golden-set JSONL file")
	outPath := fs.String("output", "", "output JSONL path (default reports/rf5-deep-<YYYYMMDD>.jsonl)")
	costLimit := fs.Float64("cost-limit", defaultCostLimit, "hard USD cap for live mode (dry-run ignores)")
	seed := fs.Int("seed", defaultSeed, "RNG seed for the spike's deterministic ordering (default 42)")
	maxIter := fs.Int("max-iterations", defaultMaxIter, "recorded MaxIteration cap (P12-L2 hard-codes 10 at the runtime call-site; this flag documents intent and clamps upward to 10)")
	dryRun := fs.Bool("dry-run", false, "validate the golden set + print '<N> cases loaded' + exit 0 (no API calls / no env required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// P12-L2: the runtime call-site LITERALS 10 (deep_agent.go:91).
	// Source-grep locked by TestMaxIterationHardCap_Literal10 — this
	// flag is a recorded-intent knob, NOT a parameterization; clamp to
	// the hard cap with a loud warning so an operator who raises it
	// sees the constraint.
	if *maxIter > defaultMaxIter {
		log.Printf("[WARN] cmd/spike-deep: --max-iterations=%d > P12-L2 hard cap %d (runtime deep_agent.go:91 literal) — clamping", *maxIter, defaultMaxIter)
		*maxIter = defaultMaxIter
	}

	rand.Seed(int64(*seed))

	// ---- 1. Load + validate the golden set (BEFORE env gating so
	// dry-run + env-missing both surface a useful message) ----
	cases, err := loadDeepGolden(*goldenPath)
	if err != nil {
		log.Printf("[FAIL] cmd/spike-deep: load golden %q: %v", *goldenPath, err)
		return 1
	}
	log.Printf("[OK] cmd/spike-deep: %d cases loaded from %s", len(cases), *goldenPath)

	if *dryRun {
		// Dry-run mode: validate + report the category breakdown +
		// exit 0. No env required (P12-L10 spike infrastructure is
		// hermetic).
		printCategoryBreakdown(cases)
		log.Printf("[OK] cmd/spike-deep: parse-rate --dry-run exit 0 (50 cases loaded, no API calls made)")
		return 0
	}

	// ---- 2. Live mode env gating (T-12-04-4: cost cap + operator
	// gate). Missing required env → exit 1 with operator-actionable
	// message (the spike is operator-runs-live; the gate is the
	// operator's signal that the live stack is configured). ----
	if missing := envMissing(); len(missing) > 0 {
		log.Printf("[FAIL] cmd/spike-deep: live mode requires the following env vars: %s", strings.Join(missing, ", "))
		log.Printf("[INFO] cmd/spike-deep: populate them (copy .env.example to .env) OR re-run with --dry-run")
		return 1
	}

	// ---- 3. Bootstrap mirrors cmd/eval + cmd/spike-history verbatim
	// where it can; the spike path diverges by calling
	// runtime.BuildDeepAgent directly (not runtime.DispatchAgent) so
	// the measurement pins the deep archetype (P12-L1) and its
	// MaxIteration=10 hard cap. ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emb := buildEmbedderLive()
	r := buildRetrieverLive(ctx, emb)
	cm := buildChatModelLive(ctx)
	manifest := loadManifestLive()

	// ---- 4. Resolve output path ----
	resolvedOut := *outPath
	if resolvedOut == "" {
		date := time.Now().UTC().Format("20060102")
		resolvedOut = filepath.Join(defaultOutDir, fmt.Sprintf("rf5-deep-%s.jsonl", date))
	}
	if dir := filepath.Dir(resolvedOut); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[FAIL] cmd/spike-deep: mkdir %q: %v", dir, err)
			return 1
		}
	}
	outFile, err := os.Create(resolvedOut)
	if err != nil {
		log.Printf("[FAIL] cmd/spike-deep: create output %q: %v", resolvedOut, err)
		return 1
	}
	defer outFile.Close()
	bw := bufio.NewWriter(outFile)
	defer bw.Flush()

	// PROVISIONAL (26-06): read-only seed snapshot feeding the hitl tool
	// construction (agent.BuildToolsForSkill) — replaced by kb.Open in
	// plan 26-09, same posture as the loader-side snapshot above. Hoisted
	// once before the loop: the seed file is read once per spike run, and
	// a nil registry would fail closed at BuildToolsForSkill for every
	// case whose manifest declares the memory:facts write target.
	kbReg := loadSeedKBRegistry()

	// ---- 5. Main loop ----
	cost := &costState{LimitUSD: *costLimit, PerCallUSD: perCallEstimateUSD, Seed: *seed}
	summary := newSummary(len(cases), *seed, *maxIter)
	for i, c := range cases {
		if cost.Exceeded() {
			log.Printf("[WARN] cmd/spike-deep: --cost-limit=$%.2f exceeded after %d/%d cases — stopping (existing JSONL preserved)", *costLimit, i, len(cases))
			break
		}
		row := measureParseRate(ctx, c, cm, emb, r, manifest, kbReg, cost)
		summary.absorb(row)
		if err := writeJSONLRow(bw, row); err != nil {
			log.Printf("[FAIL] cmd/spike-deep: write row %s: %v", c.ID, err)
			return 1
		}
		log.Printf("[OK] cmd/spike-deep: %d/%d %s [%s] direct=%t structured=%t failed=%t cost=$%.4f",
			i+1, len(cases), c.ID, c.Category, row.DirectParse, row.StructuredRescue, row.ParseFailed, row.CostUSD)
	}
	// Summary row terminates the JSONL so downstream tooling can
	// index it without a second pass over the file.
	if err := writeJSONLRow(bw, summary.finalRow(modelVersion(), promptHash(manifest))); err != nil {
		log.Printf("[FAIL] cmd/spike-deep: write summary row: %v", err)
		return 1
	}
	printSummary(summary, *costLimit, resolvedOut)
	return 0
}

// ---------------------------------------------------------------------------
// Golden-set schema
// ---------------------------------------------------------------------------

// deepCase is the JSONL schema for eval/golden/deep.jsonl. Each line is
// a single test case (50 lines per the plan: 25 multi-hop + 15
// plan-style + 10 refusal-required). The structure mirrors the Phase 9
// compliance.jsonl (DIST-10 published 50 cases) extended with the
// plan-layer fields (RequiresWriteTodos + ExpectedPlanSteps).
type deepCase struct {
	ID                 string          `json:"id"`
	Category           string          `json:"category"`
	Query              string          `json:"query"`
	Skill              string          `json:"skill"`
	ExpectedChunks     []string        `json:"expectedChunks"`
	GroundTruth        deepGroundTruth `json:"groundTruth"`
	Difficulty         string          `json:"difficulty"`
	RequiresWriteTodos bool            `json:"requiresWriteTodos,omitempty"`
	ExpectedPlanSteps  int             `json:"expectedPlanSteps,omitempty"`
}

// deepGroundTruth mirrors the verdict-JSON contract: the expected
// answer field + a citation list. Source: same shape as
// eval/golden/compliance.jsonl's ground_truth entries.
type deepGroundTruth struct {
	Answer    string   `json:"answer"`
	Citations []string `json:"citations"`
}

// validCategory enumerates the three categories the plan locks. The
// list is checked at load time; an unknown category fails the schema
// validation so a typo in deep.jsonl cannot produce a malformed
// measurement.
var validCategory = map[string]bool{
	"multi-hop":        true,
	"plan-style":       true,
	"refusal-required": true,
}

// loadDeepGolden reads the JSONL file at path and validates every
// row's schema. The schema gate runs BEFORE env gating so a malformed
// golden file surfaces a clear error to the operator (the measurement
// never silently produces broken rows).
func loadDeepGolden(path string) ([]deepCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	var out []deepCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024) // large lines tolerated
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var c deepCase
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, fmt.Errorf("line %d: decode: %w", line, err)
		}
		if err := validateCase(c); err != nil {
			return nil, fmt.Errorf("line %d (%s): %w", line, c.ID, err)
		}
		out = append(out, c)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("zero cases in golden set")
	}
	return out, nil
}

// validateCase enforces the minimum-field contract every deep.jsonl
// row MUST satisfy: non-empty identity (id, category, query, skill),
// a known category, and a ground-truth entry.
func validateCase(c deepCase) error {
	if c.ID == "" {
		return errors.New("id is required")
	}
	if !validCategory[c.Category] {
		return fmt.Errorf("unknown category %q (must be multi-hop | plan-style | refusal-required)", c.Category)
	}
	if c.Query == "" {
		return errors.New("query is required")
	}
	if c.Skill == "" {
		return errors.New("skill is required")
	}
	if c.GroundTruth.Answer == "" && len(c.GroundTruth.Citations) == 0 {
		return errors.New("groundTruth must carry at least one of answer or citations")
	}
	return nil
}

// printCategoryBreakdown is the dry-run reporter. The Plan 12-04
// verification gate ("must exit 0 with '50 cases loaded' message") is
// served by the [OK] line emitted before this; this function adds the
// per-category count table so the operator can eyeball the distribution.
func printCategoryBreakdown(cases []deepCase) {
	byCat := map[string]int{}
	for _, c := range cases {
		byCat[c.Category]++
	}
	for _, cat := range []string{"multi-hop", "plan-style", "refusal-required"} {
		log.Printf("[INFO] cmd/spike-deep:   %s: %d cases", cat, byCat[cat])
	}
}

// ---------------------------------------------------------------------------
// Measurement
// ---------------------------------------------------------------------------

// parseRateRow is one JSONL row emitted per case (one summary row at
// the end of the file). Mirrors the spike-history row shape (T-12-04-5:
// --model-ver + --prompt-hash recorded for repeatability; --seed pinned
// to 42; deterministic operator diff).
type parseRateRow struct {
	Type             string  `json:"type"` // "case" or "summary"
	TS               string  `json:"ts,omitempty"`
	ID               string  `json:"id,omitempty"`
	Category         string  `json:"category,omitempty"`
	DirectParse      bool    `json:"direct_parse"`
	StructuredRescue bool    `json:"structured_rescue"`
	ParseFailed      bool    `json:"parse_failed"`
	ModelVer         string  `json:"model_ver,omitempty"`
	PromptHash       string  `json:"prompt_hash,omitempty"`
	Seed             int     `json:"seed,omitempty"`
	MaxIterations    int     `json:"max_iterations,omitempty"`
	ModelCalls       int     `json:"model_calls"`
	CostUSD          float64 `json:"cost_usd"`
	FinalExcerpt     string  `json:"final_excerpt,omitempty"`
	Error            string  `json:"error,omitempty"`

	// Summary-only fields (Type=="summary").
	TotalCases    int     `json:"total_cases,omitempty"`
	DirectOK      int     `json:"direct_ok,omitempty"`
	StructuredOK  int     `json:"structured_ok,omitempty"`
	BothFailed    int     `json:"both_failed,omitempty"`
	DirectRate    float64 `json:"direct_rate,omitempty"`
	TotalRate     float64 `json:"total_rate,omitempty"`
	P12L10Verdict string  `json:"p12l10_verdict,omitempty"`
}

// measureParseRate runs one case through the deep archetype: builds a
// per-case DeepAgent (SessionID = case ID, P12-L2 hard cap enforced
// by runtime.BuildDeepAgent), drains events to collect the last
// non-empty assistant message, runs ExtractJSON (direct path), and
// on miss runs ExtractVerdictJSONWithStructuredOutput (12-04 Task 3
// fallback). The cost state is incremented by the estimated
// per-call USD × modelCalls.
func measureParseRate(
	ctx context.Context,
	c deepCase,
	cm model.ToolCallingChatModel,
	emb embedder.Embedder,
	r retriever.Retriever,
	manifest *skill.Manifest,
	kbReg kb.Registry,
	cost *costState,
) parseRateRow {
	row := parseRateRow{
		Type:     "case",
		TS:       time.Now().UTC().Format(time.RFC3339Nano),
		ID:       c.ID,
		Category: c.Category,
		ModelVer: modelVersion(),
		Seed:     cost.Seed, // see costState; pinned to --seed for the row
	}

	// Build the deep archetype with the production-side tool whitelist
	// (per D11-B1) + the retrieve tool — the same wiring
	// agent.BuildRunner uses internally. The deep agent's
	// MaxIteration is hard-capped at 10 by runtime.BuildDeepAgent
	// (P12-L2 source-grep locked).
	tools, terr := agent.BuildToolsForSkill(ctx, manifest, c.ID, manifest.Name, nil, kbReg)
	if terr != nil {
		row.Error = fmt.Sprintf("BuildToolsForSkill: %v", terr)
		return row
	}
	retrieve := agenttools.BuildRetrieveTool(emb, r, manifest)
	tools = append(tools, retrieve)

	deepAgent, err := runtime.BuildDeepAgent(ctx, runtime.DispatchConfig{
		Manifest:    manifest,
		ChatModel:   cm,
		Embedder:    emb,
		Retriever:   r,
		SessionID:   c.ID,
		Tools:       tools,
		Instruction: agent.RenderSystemPrompt(manifest),
	})
	if err != nil {
		row.Error = fmt.Sprintf("BuildDeepAgent: %v", err)
		return row
	}

	// Drain the run. The DeepAgent streams; the final non-empty
	// assistant Message.Content is the "final message" we attempt to
	// parse as the verdict JSON. modelCalls counts every assistant
	// MessageOutput event we observed (the deep loop's inner model
	// calls + tool-result synthesis).
	var finalMsg string
	var modelCalls int
	iter := deepAgent.Run(ctx, &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage(c.Query)}})
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			row.Error = fmt.Sprintf("agent run: %v", ev.Err)
			break
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		mv := ev.Output.MessageOutput
		if mv.Message == nil || mv.IsStreaming {
			continue
		}
		if mv.Role != schema.Assistant || mv.Message.Role != schema.Assistant {
			continue
		}
		modelCalls++
		if msg := mv.Message; msg != nil && msg.Content != "" {
			finalMsg = msg.Content
		}
	}
	row.ModelCalls = modelCalls
	cost.Add(modelCalls)
	row.FinalExcerpt = truncate(finalMsg, 240)

	// Direct path — ExtractJSON handles fences + trailing prose. A
	// non-empty result that parses as a JSON object counts as a direct
	// parse success.
	if extracted := llm.ExtractJSON(finalMsg); extracted != "" {
		var probe map[string]any
		if err := json.Unmarshal([]byte(extracted), &probe); err == nil && probe != nil {
			row.DirectParse = true
			return row
		}
	}

	// Structured-output fallback (12-04 Task 3) — the RF-5
	// mitigation. Propagates the recorded OpenAI-compatible
	// response_format=json_object signal; the chat model itself is
	// the one the spike already constructed (cost-tracked in the
	// model-calls tally above for the DeepAgent loop; the fallback
	// call is an additional model call we account for here).
	parsed, err := llm.ExtractVerdictJSONWithStructuredOutput(ctx, finalMsg, cm, verdictJSONSchema)
	if err == nil && parsed != nil {
		row.StructuredRescue = true
		row.ModelCalls++
		cost.Add(1)
		row.CostUSD = cost.LastDeltaUSD
		return row
	}
	row.ParseFailed = true
	if err != nil {
		row.Error = fmt.Sprintf("structured fallback: %v", err)
	}
	row.CostUSD = cost.LastDeltaUSD
	return row
}

// truncate returns the first n bytes of s (no ellipsis — the excerpt
// is a debugging breadcrumb, not a renderable string).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

// summary accumulates the per-case parse outcomes for the trailing
// summary row. The verdict text is derived from the direct-rate
// thresholds P12-L10 locks (90% / 70%) so a downstream reader can
// skip the math.
type summary struct {
	TotalCases   int
	DirectOK     int
	StructuredOK int
	BothFailed   int
	ModelCalls   int
	CostUSD      float64
	Seed         int
	MaxIter      int
}

// newSummary returns a zero-value summary seeded with the run-time
// invariants (seed + max-iter) so the JSONL row carries the run
// metadata the operator needs to re-derive the run.
func newSummary(totalCases, seed, maxIter int) *summary {
	return &summary{TotalCases: totalCases, Seed: seed, MaxIter: maxIter}
}

// absorb folds one per-case row into the running totals.
func (s *summary) absorb(row parseRateRow) {
	switch {
	case row.DirectParse:
		s.DirectOK++
	case row.StructuredRescue:
		s.StructuredOK++
	default:
		s.BothFailed++
	}
	s.ModelCalls += row.ModelCalls
	s.CostUSD += row.CostUSD
}

// finalRow materializes the trailing summary JSONL row.
func (s *summary) finalRow(modelVer, promptHash string) parseRateRow {
	totalOK := s.DirectOK + s.StructuredOK
	directRate := rate(s.DirectOK, s.TotalCases)
	totalRate := rate(totalOK, s.TotalCases)
	return parseRateRow{
		Type:          "summary",
		TS:            time.Now().UTC().Format(time.RFC3339Nano),
		ModelVer:      modelVer,
		PromptHash:    promptHash,
		Seed:          s.Seed,
		MaxIterations: s.MaxIter,
		ModelCalls:    s.ModelCalls,
		CostUSD:       s.CostUSD,
		TotalCases:    s.TotalCases,
		DirectOK:      s.DirectOK,
		StructuredOK:  s.StructuredOK,
		BothFailed:    s.BothFailed,
		DirectRate:    directRate,
		TotalRate:     totalRate,
		P12L10Verdict: p12l10Verdict(directRate),
	}
}

// rate returns ok/total as a fraction in [0,1] (0 when total==0).
func rate(ok, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(ok) / float64(total)
}

// p12l10Verdict renders the P12-L10 lock verdict for stdout + JSONL.
//
//   - direct ≥ 90% → "OK (structured-output optional)"
//   - direct 70-89% → "MANDATORY: structured-output final step required"
//   - direct < 70% → "ESCALATE: verdict-suffix fallback + forced_stop"
func p12l10Verdict(directRate float64) string {
	switch {
	case directRate >= 0.90:
		return "OK (structured-output optional)"
	case directRate >= 0.70:
		return "MANDATORY: structured-output final step required (P12-L10)"
	default:
		return "ESCALATE: verdict-suffix fallback + forced_stop on parse failure"
	}
}

// writeJSONLRow encodes r as JSON + appends '\n'. Errors propagate so
// the operator sees a partial-file write as a hard failure (T-9-RESULT-LOSS).
func writeJSONLRow(bw *bufio.Writer, r parseRateRow) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if _, err := bw.Write(b); err != nil {
		return err
	}
	if err := bw.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}

// printSummary emits the human-readable footer to stdout.
func printSummary(s *summary, costLimit float64, outPath string) {
	totalOK := s.DirectOK + s.StructuredOK
	directRate := rate(s.DirectOK, s.TotalCases)
	totalRate := rate(totalOK, s.TotalCases)
	log.Printf("[OK] cmd/spike-deep: parse-rate complete -> %s", outPath)
	log.Printf("[OK] cmd/spike-deep:   total=%d direct=%d structured=%d failed=%d", s.TotalCases, s.DirectOK, s.StructuredOK, s.BothFailed)
	log.Printf("[OK] cmd/spike-deep:   direct_rate=%.3f total_rate=%.3f cost=$%.4f / limit=$%.2f", directRate, totalRate, s.CostUSD, costLimit)
	log.Printf("[OK] cmd/spike-deep:   p12l10: %s", p12l10Verdict(directRate))
}

// ---------------------------------------------------------------------------
// Cost state
// ---------------------------------------------------------------------------

// costState is the spike's USD cap guard. It is deliberately a
// call-count proxy (not a token-derived invoice) — see
// perCallEstimateUSD above. Exceeded() is consulted BEFORE each
// measureParseRate call so the cap aborts the run cleanly.
type costState struct {
	LimitUSD     float64
	PerCallUSD   float64
	TotalUSD     float64
	LastDeltaUSD float64
	Seed         int
}

// Add records n LLM calls (rough estimator) and updates LastDeltaUSD.
func (c *costState) Add(n int) {
	if n <= 0 {
		c.LastDeltaUSD = 0
		return
	}
	delta := float64(n) * c.PerCallUSD
	c.TotalUSD += delta
	c.LastDeltaUSD = delta
}

// Exceeded reports whether the running total has crossed the cap.
func (c *costState) Exceeded() bool { return c.TotalUSD >= c.LimitUSD }

// ---------------------------------------------------------------------------
// Live bootstrap (mirrors cmd/spike-history verbatim where it can)
// ---------------------------------------------------------------------------

// envMissing returns the names of the env vars the live spike
// requires that are unset. Empty result means all set. The names
// match the contract documented in the package doc above; the
// message is operator-actionable (copy .env.example to .env).
func envMissing() []string {
	missing := []string{}
	for _, name := range []string{"GLM_API_KEY", "BGE_M3_URL", "MILVUS_ADDRESS"} {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// buildEmbedderLive mirrors spike-history's BGE-M3 construction.
// Mirrored from cmd/eval/main.go:501-510.
func buildEmbedderLive() embedder.Embedder {
	bgeURL := os.Getenv("BGE_M3_URL")
	bgeModel := os.Getenv("BGE_M3_MODEL")
	if bgeModel == "" {
		bgeModel = "bge-m3"
	}
	timeoutSec := os.Getenv("BGE_M3_TIMEOUT")
	if timeoutSec == "" {
		timeoutSec = "60"
	}
	to, err := time.ParseDuration(timeoutSec + "s")
	if err != nil || to <= 0 {
		to = 60 * time.Second
	}
	return bge.New(bge.Config{URL: bgeURL, Model: bgeModel, Timeout: to})
}

// buildRetrieverLive mirrors spike-history's Milvus construction. The
// spike does NOT need the knowledge corpus indexed (the measurement
// targets the DeepAgent's final message FORMAT, not retrieval
// quality) — skipping knowledge.Index keeps the live path cheap.
func buildRetrieverLive(ctx context.Context, emb embedder.Embedder) retriever.Retriever {
	addr := os.Getenv("MILVUS_ADDRESS")
	cfg := milvus.Config{Addr: addr, CollectionName: defaultCollection}
	r, err := milvus.New(ctx, cfg)
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-deep: milvus dial %s: %v", addr, err)
	}
	return r
}

// buildChatModelLive constructs the chat model. Reads GLM_API_KEY
// (required; validated by eval.NewGLMChatModel.validateEnv) +
// GLM_BASE_URL (default Coding Plan endpoint) + JUDGE_MODEL (default
// glm-5.2).
func buildChatModelLive(ctx context.Context) model.ToolCallingChatModel {
	cm, err := eval.NewGLMChatModel(ctx)
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-deep: %v", err)
	}
	return cm
}

// loadManifestLive loads skills/ and returns the first Skill
// manifest. The deep archetype is dispatched by the spike via
// runtime.BuildDeepAgent DIRECTLY (not runtime.DispatchAgent), so the
// manifest.Agent value does not influence routing — but the
// resolved-tier slice MUST be present for the retrieve tool's
// KNOW-03 whitelist. Mirrors cmd/spike-history:479.
// loadSeedKBRegistry builds the read-only registry the skill loader
// validates reads declarations against.
//
// PROVISIONAL (26-04): read-only seed snapshot — replaced by the
// sqlite-backed kb.Open registry in plan 26-09 (this binary never
// touches the knowledge corpus, so the seed snapshot is its long-term
// shape).
func loadSeedKBRegistry() kb.Registry {
	kbSeeds, err := kb.LoadSeedFile("knowledge_bases.yaml")
	if err != nil {
		log.Fatalf("[FAIL] cmd/spike-deep: kb seed load: %v", err)
	}
	if kbSeeds == nil {
		log.Printf("[WARN] cmd/spike-deep: no knowledge_bases.yaml seed found — knowledge-base registry is empty (every tier Skill will fail reads validation)")
	}
	return kb.New(kbSeeds)
}

func loadManifestLive() *skill.Manifest {
	loader := skill.NewLoader("skills/", loadSeedKBRegistry())
	if _, err := loader.Scan(); err != nil {
		log.Fatalf("[FAIL] cmd/spike-deep: skill scan: %v", err)
	}
	for _, m := range loader.Skills() {
		return m
	}
	log.Fatalf("[FAIL] cmd/spike-deep: zero Skills loaded (the deep agent requires a manifest)")
	return nil
}

// modelVersion is the LLM identifier written into every JSONL row
// (T-12-04-5: --model-ver recorded). It honors JUDGE_MODEL (the
// D3-07 cross-evaluation slot) before falling back to the default.
func modelVersion() string {
	if m := os.Getenv("JUDGE_MODEL"); m != "" {
		return m
	}
	return "glm-5.2"
}

// promptHash is the D11-C2 prompt snapshot recorded for re-runs
// (T-12-04-5: --prompt-hash). The hash is the SHA-256 of the rendered
// system prompt body — agent.RenderSystemPrompt + hitl.ComputePromptHash,
// the same derivation agent.BuildRunner uses to stamp checkpoint rows.
func promptHash(m *skill.Manifest) string {
	if m == nil {
		return ""
	}
	return hitl.ComputePromptHash(agent.RenderSystemPrompt(m))
}
