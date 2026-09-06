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
// Bootstrap mirrors cmd/server: same env vars (BGE_M3_URL,
// LOCAL_*/GLM_API_KEY + GLM_BASE_URL, MILVUS_ADDRESS / MONGO_URI,
// SQLITE_DSN), same bge + retriever + agent + store construction. The
// judge uses LOCAL_MODEL / JUDGE_MODEL (default glm-5.2) — the D3-07
// slot for Phase 6 cross-evaluation.
//
// Per-case flow:
//
//  1. embedder.Embed(query) → queryVec.
//  2. retriever.Search(ctx, queryVec, tiers, topK, now) → scored Hits (for
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
// Phase 6 flag matrix (D6-02 / D6-13 / D6-14 / GEVL-07):
//
//	--frozen=<skill>   read eval/frozen/<skill>.jsonl + .lock.json and
//	                   ASSERT the pin (model_ver + prompt_hash) BEFORE
//	                   scoring. Mutually exclusive with --golden.
//	--archive          also append the run to eval/reports/<ts>.json +
//	                   eval/reports/INDEX.json (the dashboard's trend
//	                   history).
//	--judge-models a,b run the eval once per judge model, writing
//	                   per-judge reports under eval/reports/<judge>/.
//	                   Surfaces the Pitfall 8 self-eval-bias signal.
//	--gate-check=path  do NOT run an eval at all: load an existing
//	                   report, apply the per-Skill thresholds, print the
//	                   markdown table and exit non-zero on regression.
//
// Exit codes: 0 on success, 1 on configuration / runtime error.
//
// EXIT-CODE CONTRACT (load-bearing): a low faithfulness score does NOT
// make the DEFAULT eval mode exit non-zero. Scoring and gating are
// deliberately separate concerns — the eval CLI emits data, and the
// promotion decision belongs to the CI workflow, which invokes the
// dedicated --gate-check mode. Conflating them would make every local
// exploratory run look like a build failure and would make the gate's
// threshold un-overridable. --gate-check is the ONE mode that exits
// non-zero on a quality regression, because gating is its entire job.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
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

	// Phase 6 paths (D6-02 / D6-13).
	defaultArchiveDir = "eval/reports"
	frozenDir         = "eval/frozen"
	defaultSkillsDir  = "skills"
)

// frozenSetPath / frozenLockPath resolve a Skill's frozen eval set and
// its pin sidecar. Kept as functions (not fmt.Sprintf at the call site)
// so the layout is stated once — the CI workflow, the dashboard's
// /api/eval/frozen handler and this CLI must agree on it.
func frozenSetPath(skill string) string  { return filepath.Join(frozenDir, skill+".jsonl") }
func frozenLockPath(skill string) string { return filepath.Join(frozenDir, skill+".lock.json") }

// archiveDirFor returns the archive directory for a judge model.
// Cross-eval runs (--judge-models) are partitioned per judge so their
// INDEX.json histories stay separate — mixing two judges' scores into
// one trend line would render the line meaningless.
func archiveDirFor(judge string, crossEval bool) string {
	if !crossEval {
		return defaultArchiveDir
	}
	return filepath.Join(defaultArchiveDir, judge)
}

// validateFlagMatrix rejects contradictory flag combinations before any
// expensive bootstrap happens.
//
// --golden and --frozen are mutually exclusive: they are two different
// answers to "which eval set?", and silently preferring one would mean
// the operator who passed both gets scored against a set they did not
// choose — while a frozen run's whole promise is knowing exactly what
// was scored.
func validateFlagMatrix(explicitGolden bool, frozen string) error {
	if explicitGolden && frozen != "" {
		return fmt.Errorf("--golden and --frozen are mutually exclusive (they name two different eval sets); pass --frozen=<skill> to score the pinned set, or --golden=<path> to score an arbitrary set")
	}
	return nil
}

// parseJudgeModels splits the --judge-models comma list, trimming
// blanks. An empty flag yields nil, meaning "one run with the ambient
// JUDGE_MODEL" (the pre-Phase-6 behavior).
func parseJudgeModels(list string) []string {
	if strings.TrimSpace(list) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(list, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runGateCheck is the GEVL-07 / SC-2 promotion gate (D6-11). It loads an
// already-written report, applies each Skill's declared threshold, and
// prints the markdown table CI posts as a PR comment.
//
// This is the ONE mode that exits non-zero on a quality regression —
// see the exit-code contract in the package doc. It runs no LLM and
// needs no API key, so the gate step in CI stays cheap and cannot leak
// a secret.
func runGateCheck(reportPath, skillsDir string) int {
	report, err := eval.LoadReport(reportPath)
	if err != nil {
		log.Printf("[FAIL] cmd/eval: gate-check: %v", err)
		return 1
	}
	thresholds, err := eval.LoadThresholds(skillsDir)
	if err != nil {
		log.Printf("[FAIL] cmd/eval: gate-check: %v", err)
		return 1
	}

	results := eval.GateDecision(report, thresholds)
	// The markdown table goes to STDOUT so CI can redirect it into a
	// file for `gh pr comment --body-file`; diagnostics go to stderr via
	// log so they never corrupt the comment body.
	fmt.Print(eval.FormatGateMarkdown(results))

	if failed := eval.FailedSkills(results); len(failed) > 0 {
		for _, f := range failed {
			log.Printf("[FAIL] cmd/eval: gate: Skill %q faithfulness %.4f < threshold %.4f", f.Skill, f.Faithfulness, f.Threshold)
		}
		return 1
	}
	log.Printf("[OK] cmd/eval: gate passed (%d Skill(s) at or above threshold)", len(results))
	return 0
}

// ---------------------------------------------------------------------------
// KPI eval support (GEVL-09/10).
// ---------------------------------------------------------------------------

// KPILockFile pins a KPI eval run. Extends LockFile with KPI-specific fields.
type KPILockFile struct {
	ModelVersion       string `json:"model_ver"`
	PromptHash        string `json:"prompt_hash"`
	Seed              int64  `json:"seed"`
	EvalSetVersion    string `json:"eval_set_version"`
	SkillName         string `json:"skill_name"`
	EvalSetSHA256     string `json:"eval_set_sha256"`
	KpiRegistrySHA256 string `json:"kpi_registry_sha256"`
}

// loadKPICases reads eval/golden/kpi.jsonl and returns the KPI cases.
func loadKPICases(path string) ([]eval.KPICase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loadKPICases: open %q: %w", path, err)
	}
	defer f.Close()

	var cases []eval.KPICase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c eval.KPICase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("loadKPICases: line %d: parse JSON: %w", lineNo, err)
		}
		if c.ID == "" {
			return nil, fmt.Errorf("loadKPICases: line %d: case missing id", lineNo)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("loadKPICases: scan: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("loadKPICases: %q loaded 0 cases", path)
	}
	return cases, nil
}

// loadKPILock reads the KPI lock file.
func loadKPILock(path string) (KPILockFile, error) {
	var lock KPILockFile
	f, err := os.Open(path)
	if err != nil {
		return lock, fmt.Errorf("loadKPILock: open %q: %w", path, err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&lock); err != nil {
		return lock, fmt.Errorf("loadKPILock: decode %q: %w", path, err)
	}
	// Validate required fields.
	for _, req := range []struct {
		field string
		value string
	}{
		{"model_ver", lock.ModelVersion},
		{"prompt_hash", lock.PromptHash},
		{"eval_set_version", lock.EvalSetVersion},
		{"skill_name", lock.SkillName},
		{"eval_set_sha256", lock.EvalSetSHA256},
		{"kpi_registry_sha256", lock.KpiRegistrySHA256},
	} {
		if req.value == "" {
			return lock, fmt.Errorf("loadKPILock: %q missing required field %q", path, req.field)
		}
	}
	return lock, nil
}

// computeKPIRegistrySHA computes a SHA-256 hex digest of all files under a directory tree.
func computeKPIRegistrySHA(dir string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("computeKPIRegistrySHA: walk %q: %w", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// computeEvalSetSHA256 computes SHA-256 of a JSONL file.
func computeEvalSetSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("computeEvalSetSHA256: open %q: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("computeEvalSetSHA256: copy: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// judgeKPI runs the GEVL-09/10 programmatic judge for one KPI case.
// snapRows should be the KPI snapshot data for the period being tested.
func judgeKPI(answerText string, c eval.KPICase, snapRows []map[string]any) eval.KPIJudgment {
	return eval.JudgeKPIReferenceConsistency(&c, answerText, snapRows)
}

// runKPIMode executes the KPI-specific frozen eval path (GEVL-09/10).
func runKPIMode(ctx context.Context, frozen string, ag *agent.Agent, emb *bge.BGE, r retriever.Retriever) {
	// Build paths.
	goldenSetPath := frozenSetPath(frozen) // eval/golden/kpi.jsonl
	lockPath := frozenLockPath(frozen)     // eval/frozen/kpi.lock.json

	// Load lock and assert pins.
	lock, err := loadKPILock(lockPath)
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: kpi: %v", err)
	}

	// Assert kpi_registry_sha256 pin.
	regSHA, err := computeKPIRegistrySHA("kpis/")
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: kpi: computeKPIRegistrySHA: %v", err)
	}
	if regSHA != lock.KpiRegistrySHA256 {
		log.Fatalf("[FAIL] cmd/eval: kpi: kpi_registry_sha256 mismatch: got %s, want %s", regSHA, lock.KpiRegistrySHA256)
	}
	log.Printf("[OK] cmd/eval: kpi: kpi_registry_sha256 pin holds (%s)", regSHA)

	// Assert eval_set_sha256 pin.
	setSHA, err := computeEvalSetSHA256(goldenSetPath)
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: kpi: computeEvalSetSHA256: %v", err)
	}
	if setSHA != lock.EvalSetSHA256 {
		log.Fatalf("[FAIL] cmd/eval: kpi: eval_set_sha256 mismatch: got %s, want %s", setSHA, lock.EvalSetSHA256)
	}
	log.Printf("[OK] cmd/eval: kpi: eval_set_sha256 pin holds (%s)", setSHA)

	// Load golden cases.
	cases, err := loadKPICases(goldenSetPath)
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: kpi: loadKPICases: %v", err)
	}
	log.Printf("[INFO] cmd/eval: kpi: loaded %d cases from %s", len(cases), goldenSetPath)

	// Load KPI snapshot data for the eval period.
	// For now, build empty snapshot rows (actual KPI data would come from DuckDB or KPI registry).
	var snapRows []map[string]any
	// In production, snapRows would be populated from the KPI registry with the
	// actual computed values for the expected period.

	// Accumulate results.
	var passCount, failCount int
	catPass := map[string]int{}
	catTotal := map[string]int{}

	for i, c := range cases {
		// Run the agent for this query.
		sessionID := "eval-kpi-" + c.ID
		v, refusal, err := ag.Run(ctx, sessionID, c.Query)
		_, answer, refusalReason := classifyRunOutcome(err, refusal, v)

		// Judge the answer.
		j := judgeKPI(answer, c, snapRows)

		catTotal[c.Category]++
		if j.Passed {
			passCount++
			catPass[c.Category]++
		} else {
			failCount++
			log.Printf("[FAIL] cmd/eval: kpi: case %d/%d %s [%s] reason=%s",
				i+1, len(cases), c.ID, c.Category, j.Reason)
		}
		log.Printf("[OK] cmd/eval: kpi: case %d/%d %s [%s] passed=%t reason=%s",
			i+1, len(cases), c.ID, c.Category, j.Passed, j.Reason)
		_ = refusalReason // logged but not used in GEVL-09/10 scoring
	}

	total := passCount + failCount
	accuracy := 0.0
	if total > 0 {
		accuracy = float64(passCount) / float64(total)
	}
	log.Printf("[OK] cmd/eval: kpi: %d/%d passed (accuracy=%.4f)", passCount, total, accuracy)
	for cat, totalCat := range catTotal {
		passCat := catPass[cat]
		log.Printf("[INFO] cmd/eval: kpi: category %s: %d/%d passed", cat, passCat, totalCat)
	}
}

func main() {
	goldenPath := flag.String("golden", defaultGoldenPath, "path to the golden-set JSONL file")
	reportPath := flag.String("report", defaultReportPath, "output path for the JSON report")
	calibrationPath := flag.String("calibration", defaultCalibrationPath, "output path for the calibration JSON")
	limit := flag.Int("limit", 0, "cap the number of cases processed (0 = no cap; debug aid)")
	topK := flag.Int("topk", defaultTopK, "top-K retriever hits per case (mirrors cmd/server)")
	archive := flag.Bool("archive", false, "append the run to "+defaultArchiveDir+"/<ts>.json + INDEX.json (trend history)")
	frozen := flag.String("frozen", "", "Skill name; scores "+frozenDir+"/<skill>.jsonl and asserts the .lock.json pin")
	judgeModels := flag.String("judge-models", "", "comma list (e.g. glm-5.2,deepseek-chat); runs the eval once per judge (cross-eval)")
	gateCheck := flag.String("gate-check", "", "path to a report.json; applies per-Skill thresholds, prints a markdown table and exits non-zero on regression")
	skillsDir := flag.String("skills-dir", defaultSkillsDir, "directory holding <skill>/eval-gate.json threshold files")
	caseDelay := flag.String("case-delay", envOr("EVAL_CASE_DELAY", ""), "pause between cases (Go duration, e.g. 3s); rate-limit courtesy for throttled endpoints (Coding Plan 429 / single-seat local server). EVAL_CASE_DELAY env sets the default")
	casesFilter := flag.String("cases", "", "comma list of case IDs to run (e.g. adv-006,adv-007); diagnostic subset — report/calibration still reflect ONLY the cases run")
	dumpPath := flag.String("dump", "", "write per-case diagnostics (query, raw verdict, refusal, contexts, scores) to this JSON file; for investigating model-vs-harness disputes — report.json deliberately omits answer text")
	flag.Parse()
	caseDelayDur := 0 * time.Second
	if s := strings.TrimSpace(*caseDelay); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d < 0 {
			log.Fatalf("[FAIL] cmd/eval: --case-delay %q: invalid duration (try e.g. 3s, 500ms)", *caseDelay)
		}
		caseDelayDur = d
	}
	// Per-case budget (default 90s = GLM-tuned). EVAL_CASE_TIMEOUT=300s
	// for a slow self-hosted thinking model — an abort here scores the
	// case as refusal, which would silently poison refusal_accuracy.
	if s := envOr("EVAL_CASE_TIMEOUT", ""); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			log.Fatalf("[FAIL] cmd/eval: EVAL_CASE_TIMEOUT=%q invalid (try e.g. 300s)", s)
		}
		evalCaseTimeout = d
		log.Printf("[INFO] cmd/eval: per-case budget = %s (EVAL_CASE_TIMEOUT)", d)
	}

	// --gate-check short-circuits BEFORE any bootstrap: gating reads a
	// file, so it must not require GLM_API_KEY / Milvus / BGE-M3.
	if *gateCheck != "" {
		os.Exit(runGateCheck(*gateCheck, *skillsDir))
	}

	explicitGolden := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "golden" {
			explicitGolden = true
		}
	})
	if err := validateFlagMatrix(explicitGolden, *frozen); err != nil {
		log.Fatalf("[FAIL] cmd/eval: %v", err)
	}

	// Frozen mode: swap the eval set and load the pin. The pin is
	// ASSERTED after bootstrap (below), once the live prompt body is
	// available to re-hash.
	goldenSetPath := *goldenPath
	frozenVersion := ""
	var lock eval.LockFile
	if *frozen != "" {
		goldenSetPath = frozenSetPath(*frozen)
		var err error
		lock, err = eval.LoadLock(frozenLockPath(*frozen))
		if err != nil {
			log.Fatalf("[FAIL] cmd/eval: %v", err)
		}
		frozenVersion = lock.EvalSetVersion
		log.Printf("[INFO] cmd/eval: frozen mode: set=%s lock=%s (model_ver=%s, eval_set_version=%s)",
			goldenSetPath, frozenLockPath(*frozen), lock.ModelVersion, lock.EvalSetVersion)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Bootstrap: mirror cmd/server's wiring (env vars + construction).
	// Any failure here is fatal — same fail-fast contract as cmd/server.
	// The KPI frozen set is a programmatic-judge mode, not a named Skill —
	// fall back to the first manifest for it.
	kpiMode := *frozen == "kpi"
	emb := buildEmbedder()
	r := buildRetriever(ctx, emb)
	manifestName := *frozen
	if kpiMode {
		// KPI frozen set is a programmatic-judge mode, not a named Skill —
		// fall back to the first manifest for it.
		manifestName = ""
	} else if manifestName == "" && explicitGolden {
		// Non-frozen explicit --golden: the golden set is Skill-bound (the
		// frozen sets live at eval/frozen/<skill>.jsonl, so the stem IS the
		// intended Skill name). Resolving it here closes the UAT 2026-09-06
		// gap where `--golden eval/frozen/compliance.jsonl` silently ran the
		// FIRST manifest (calculator): all 50 compliance questions were then
		// correctly refused as out-of-scope and the report scored a skill
		// the questions never exercised. manifestFor fails loud when the
		// stem matches no manifest — right contract for a mis-targeted run.
		manifestName = strings.TrimSuffix(filepath.Base(*goldenPath), filepath.Ext(*goldenPath))
	}
	manifest := manifestFor(*skillsDir, manifestName)
	cm := buildChatModel(ctx) // agent's chat model (GLM_API_KEY)
	ag := buildAgent(ctx, manifest, emb, r, cm)
	st := buildStore(ctx)

	// SC-6 / D6-13: assert the pin BEFORE scoring. A drifted model or
	// prompt makes the resulting numbers incomparable to the published
	// baseline, so failing loud here is strictly better than emitting
	// scores that look authoritative and are not.
	skillName := *frozen
	if kpiMode {
		// KPI eval path: uses programmatic judge, not LLM judge.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		emb := buildEmbedder()
		r := buildRetriever(ctx, emb)
		cm := buildChatModel(ctx)
		ag := buildAgent(ctx, manifest, emb, r, cm)
		_ = st
		runKPIMode(ctx, *frozen, ag, emb, r)
		return
	}
	if *frozen != "" {
		if err := eval.AssertPin(lock, eval.ActiveModelVersion(), skillName, agent.RenderSystemPrompt(manifest)); err != nil {
			log.Fatalf("[FAIL] cmd/eval: %v", err)
		}
		log.Printf("[OK] cmd/eval: frozen pin holds (model_ver=%s, prompt_hash=%s)", lock.ModelVersion, lock.PromptHash)
	}
	if skillName == "" {
		skillName = manifest.Name
	}

	// 2. Load the eval set.
	cases, err := eval.LoadGolden(goldenSetPath)
	if err != nil {
		log.Fatalf("[FAIL] cmd/eval: load eval set %q: %v", goldenSetPath, err)
	}
	if *casesFilter != "" {
		want := map[string]bool{}
		for _, id := range strings.Split(*casesFilter, ",") {
			want[strings.TrimSpace(id)] = true
		}
		var kept []eval.Case
		for _, c := range cases {
			if want[c.ID] {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			log.Fatalf("[FAIL] cmd/eval: --cases %q matched none of the %d loaded cases", *casesFilter, len(cases))
		}
		log.Printf("[INFO] cmd/eval: --cases filter kept %d case(s)", len(kept))
		cases = kept
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
		log.Printf("[INFO] cmd/eval: --limit=%d capped cases to %d", *limit, len(cases))
	}
	log.Printf("[OK] cmd/eval: loaded %d cases from %s", len(cases), goldenSetPath)

	// 3. Judge selection. --judge-models runs the whole eval once per
	// judge so their scores can be diffed (D6-14); the default is a
	// single run with the ambient JUDGE_MODEL.
	judges := parseJudgeModels(*judgeModels)
	crossEval := len(judges) > 1
	if len(judges) == 0 {
		judges = []string{eval.ActiveModelVersion()}
	}

	reportsByJudge := make(map[string]eval.Report, len(judges))
	for _, judgeModel := range judges {
		var dumpRows []map[string]any
		// NewGLMChatModel reads the model slot at construction (via
		// ResolveLLM: LOCAL_MODEL > JUDGE_MODEL); setting BOTH here is
		// how a per-judge model is selected without duplicating the
		// chat-model wiring — LOCAL_MODEL alone would win the precedence
		// and silently pin every cross-eval judge to one model. Safe:
		// this CLI is single-threaded batch.
		if err := os.Setenv("JUDGE_MODEL", judgeModel); err != nil {
			log.Fatalf("[FAIL] cmd/eval: set JUDGE_MODEL=%s: %v", judgeModel, err)
		}
		if err := os.Setenv("LOCAL_MODEL", judgeModel); err != nil {
			log.Fatalf("[FAIL] cmd/eval: set LOCAL_MODEL=%s: %v", judgeModel, err)
		}
		judge := eval.NewJudge(buildChatModel(ctx))
		log.Printf("[INFO] cmd/eval: scoring %d cases with judge=%s", len(cases), judgeModel)

		report, scores := runEvalPass(ctx, evalPass{
			cases:   cases,
			agent:   ag,
			emb:     emb,
			r:       r,
			store:   st,
			judge:   judge,
			tiers:   manifest.ResolvedTiers(),
			topK:    *topK,
			skill:   skillName,
			nJudges: len(judges),
			caseDelay: caseDelayDur,
			dump:    &dumpRows,
		})
		reportsByJudge[judgeModel] = report

		// --dump: persist the raw per-case evidence next to the run.
		if *dumpPath != "" {
			if b, mErr := json.MarshalIndent(dumpRows, "", "  "); mErr == nil {
				if wErr := os.WriteFile(*dumpPath, b, 0o644); wErr != nil {
					log.Printf("[WARN] cmd/eval: write dump %q: %v", *dumpPath, wErr)
				} else {
					log.Printf("[OK] cmd/eval: case diagnostics dumped to %s (%d cases)", *dumpPath, len(dumpRows))
				}
			}
		}

		// The canonical eval/report.json + calibration.json reflect the
		// FIRST judge (the primary). Cross-eval judges are published only
		// under their own archive directory so they cannot silently
		// overwrite the headline numbers.
		if judgeModel == judges[0] {
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

		if *archive {
			dir := archiveDirFor(judgeModel, crossEval)
			if err := eval.WriteArchive(dir, report, skillName, judgeModel, frozenVersion); err != nil {
				log.Fatalf("[FAIL] cmd/eval: archive: %v", err)
			}
			log.Printf("[OK] cmd/eval: archived run to %s (judge=%s)", dir, judgeModel)
		}
	}

	// 4. Cross-eval diff (D6-14 / Pitfall 8). Reported, never fatal: a
	// judge disagreement is information about the measurement, not a
	// build failure.
	if crossEval {
		for _, d := range eval.CrossEvalDiff(reportsByJudge, eval.DefaultCrossEvalDeltaPct) {
			if d.Flagged {
				log.Printf("[WARN] cmd/eval: cross-eval: Skill %q faithfulness varies %.2f%% across judges %v — the score is judge-dependent (self-eval bias signal)",
					d.Skill, d.DeltaPct, d.FaithfulnessByJudge)
				continue
			}
			log.Printf("[OK] cmd/eval: cross-eval: Skill %q judges agree within %.2f%% %v", d.Skill, d.DeltaPct, d.FaithfulnessByJudge)
		}
	}
}

// evalPass bundles the per-run inputs of runEvalPass. Grouped into a
// struct because --judge-models turned a straight-line main() into a
// loop, and a 10-positional-argument call would be unreadable.
type evalPass struct {
	cases   []eval.Case
	agent   *agent.Agent
	emb     *bge.BGE
	r       retriever.Retriever
	store   store.Store
	judge   eval.Judge
	tiers   []string
	topK    int
	skill   string
	nJudges int
	// caseDelay sleeps this long after each case (except the last).
	// Rationale (2026-09-06): the GLM Coding Plan 429s at back-to-back
	// case rates; a self-hosted LOCAL_* endpoint may also be
	// single-seat. Each case issues ~4-6 LLM calls (agent verdict +
	// 4 metric judge calls), so even 2-3s of delay meaningfully lowers
	// the request rate.
	caseDelay time.Duration
	// dump, when non-nil, collects per-case raw diagnostics (--dump).
	dump *[]map[string]any
}

// evalCaseTimeout is the per-case budget (agent verdict + 4 metric judge
// calls). EVAL_CASE_TIMEOUT raises it for slow self-hosted models — a
// thinking 27B on ollama spends minutes per LLM call, and the 90s default
// (tuned for GLM) aborts mid-verdict (2026-09-06).
var evalCaseTimeout = 90 * time.Second

// runEvalPass scores every case with ONE judge and returns the report +
// the calibration inputs. Extracted verbatim from the Phase 3 main()
// loop so a cross-eval run can execute it once per judge model.
func runEvalPass(ctx context.Context, p evalPass) (eval.Report, []eval.CaseScore) {
	results := make([]eval.CaseResult, 0, len(p.cases))
	scores := make([]eval.CaseScore, 0, len(p.cases))

	for i, c := range p.cases {
		caseCtx, caseCancel := context.WithTimeout(ctx, evalCaseTimeout)
		sessionID := "eval-" + c.ID

		// 3a. Parallel retriever.Search for the calibration MaxScore +
		// the context_precision/recall inputs. This is the SAME search
		// the agent's retrieve tool would do internally; we re-run it
		// here to get the scored Hit slice (the agent does not expose
		// scores on its public API, only via LastRetrieved).
		queryVec, err := p.emb.Embed(caseCtx, c.Query)
		var hits []retriever.Hit
		var maxScore float32
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s embed: %v — metrics will score 0 context", c.ID, err)
		} else {
			// Plan 05-02 D5-15: compute now once per case (consistent with
			// the agent's per-Retrieve-call semantics in internal/agent).
			now := time.Now().Unix()
			hits, err = p.r.Search(caseCtx, queryVec, p.tiers, p.topK, now)
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
		v, refusal, err := p.agent.Run(caseCtx, sessionID, c.Query)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s agent.Run: %v — scoring as refusal", c.ID, err)
		}
		refused, answer, refusalReason := classifyRunOutcome(err, refusal, v)
		var contexts []string
		if !refused {
			// The agent's actual retrieved chunks — primary metric input.
			for _, ch := range p.agent.LastRetrieved() {
				contexts = append(contexts, ch.Content)
			}
		}
		caseCancel()

		// --dump: capture the raw evidence for offline inspection (the
		// report intentionally stays metric-only). faith is patched in
		// after the metrics run below.
		var dumpIdx = -1
		if p.dump != nil {
			raw, _ := json.Marshal(v)
			hits := p.agent.LastRetrieved()
			scores := make([]map[string]any, 0, len(hits))
			for _, h := range hits {
				scores = append(scores, map[string]any{"id": h.ID, "score": h.Score})
			}
			*p.dump = append(*p.dump, map[string]any{
				"id":             c.ID,
				"category":       c.Category,
				"query":          c.Query,
				"ground_truth":   c.GroundTruth,
				"expected":       c.ExpectedChunks,
				"agent_error":    err != nil,
				"err":            errString(err),
				"refused":        refused,
				"refusal_reason": refusalReason,
				"verdict":        json.RawMessage(raw),
				"chunk_scores":   scores,
				"faith":          0.0,
			})
			dumpIdx = len(*p.dump) - 1
		}

		// 3c. Compute the 4 metrics via the explicit exemption helper.
		// Errors here are per-case, not fatal — log and score 0 for the
		// failing metric.
		faith, arel, cprec, crecall := computeCaseMetrics(ctx, c.ID, refused, answer, c.Query, c.GroundTruth, contexts, hits, p.judge, p.emb)
		if dumpIdx >= 0 {
			(*p.dump)[dumpIdx]["faith"] = faith
			(*p.dump)[dumpIdx]["answer"] = answer
		}

		// 3d. Backfill the audit row if one exists for this session.
		// Skipped under cross-eval: N judges scoring the same session
		// would each overwrite the previous judge's metrics, leaving the
		// audit row holding whichever judge happened to run last.
		if p.nJudges == 1 {
			backfillAuditMetrics(ctx, p.store, sessionID, c.Query, faith, cprec, arel, crecall)
		}

		// 3e. Accumulate.
		sample := eval.Sample{
			Case:          c,
			Answer:        answer,
			Contexts:      contexts,
			RefusalReason: refusalReason,
		}
		results = append(results, eval.SampleToResult(sample, faith, cprec, arel, crecall))
		scores = append(scores, eval.CaseScore{MaxScore: maxScore, Refused: refused})

		log.Printf("[OK] cmd/eval: case %d/%d %s [%s] faith=%.3f arel=%.3f cprec=%.3f crecall=%.3f refused=%t maxScore=%.3f",
			i+1, len(p.cases), c.ID, c.Category, faith, arel, cprec, crecall, refused, maxScore)

		// Rate-limit courtesy: sleep between cases (not after the last)
		// so a throttled endpoint (Coding Plan 429 / single-seat local
		// server) is not hammered back-to-back.
		if p.caseDelay > 0 && i < len(p.cases)-1 {
			log.Printf("[..] cmd/eval: case pacing — sleeping %s before next case", p.caseDelay)
			select {
			case <-time.After(p.caseDelay):
			case <-ctx.Done():
				log.Printf("[WARN] cmd/eval: pacing sleep aborted: %v", ctx.Err())
			}
		}
	}

	report := buildReport(results)
	report.Skill = p.skill
	return report, scores
}

// computeCaseMetrics calculates the four RAG-trust metrics for a single
// eval case. It is the home of the 14-12 re-baseline semantics: an
// in-schema refused verdict (refused=true) is explicitly exempt from
// faithfulness judging against retrieved chunks and instead takes the
// frozen empty-answer convention (faithfulness=1.0). The other three
// metrics keep their existing conventions so the 1.0.0-era baseline
// numbers remain reproducible.
//
// The exemption is explicit here rather than implicit via
// internal/eval/faithfulness.go:75-78, so a future refactor of the
// Faithfulness metric cannot accidentally route refusal prose back to
// the chunk judge.
func computeCaseMetrics(ctx context.Context, caseID string, refused bool, answer, query, groundTruth string, contexts []string, hits []retriever.Hit, judge eval.Judge, emb embedder.Embedder) (faith, arel, cprec, crecall float64) {
	var err error
	if refused {
		// 14-12 re-baseline: refused verdicts are judged against the frozen
		// empty-answer convention, not against retrieved chunks.
		faith = 1.0
	} else {
		faith, err = eval.Faithfulness(ctx, answer, contexts, judge)
		if err != nil {
			log.Printf("[WARN] cmd/eval: case %s faithfulness: %v — scoring 0", caseID, err)
			faith = 0
		}
	}
	arel, err = eval.AnswerRelevancy(ctx, query, answer, emb, judge)
	if err != nil {
		log.Printf("[WARN] cmd/eval: case %s answer_relevancy: %v — scoring 0", caseID, err)
		arel = 0
	}
	cprec, err = eval.ContextPrecision(ctx, query, hits, judge)
	if err != nil {
		log.Printf("[WARN] cmd/eval: case %s context_precision: %v — scoring 0", caseID, err)
		cprec = 0
	}
	crecall, err = eval.ContextRecall(ctx, groundTruth, hits, judge, refused)
	if err != nil {
		log.Printf("[WARN] cmd/eval: case %s context_recall: %v — scoring 0", caseID, err)
		crecall = 0
	}
	return
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

// buildChatModel constructs the agent's chat model via eval.NewGLMChatModel
// (resolves LOCAL_* > GLM_* — see eval.ResolveLLM). Returns the
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

// manifestFor loads Skills from skillsDir and resolves the manifest for
// the frozen set. Resolution contract (UAT 2026-09-06 fix): when a frozen
// set names a Skill (e.g. --frozen=compliance → "compliance"), the manifest
// with THAT Name is required — the compliance golden set is meaningless
// against any other Skill, and picking "the first skill" stopped working
// when skills/ grew data-channel entries (kpi-dashboard, Phase 16: the
// experimental agent path rejects them) and compliance moved to
// examples/skills (run with --skills-dir=examples/skills). With no frozen
// name the first tier-compatible manifest is returned (legacy posture).
// Aborts on error.
func manifestFor(skillsDir, name string) *skill.Manifest {
	loader := skill.NewLoader(skillsDir)
	if _, err := loader.Scan(); err != nil {
		log.Fatalf("[FAIL] cmd/eval: skill scan (%s): %v", skillsDir, err)
	}
	all := loader.Skills()
	if len(all) == 0 {
		log.Fatalf("[FAIL] cmd/eval: zero Skills loaded from %s (the agent requires a manifest)", skillsDir)
	}
	if name != "" {
		for _, m := range all {
			if m.Name == name {
				return m
			}
		}
		log.Fatalf("[FAIL] cmd/eval: frozen set names Skill %q but no manifest with that Name exists in %s (known: %v)", name, skillsDir, manifestNames(all))
	}
	for _, m := range all {
		return m
	}
	return nil
}

func manifestNames(ms []*skill.Manifest) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
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

// classifyRunOutcome is the SINGLE exit point for the scoring-path
// outcome classification of one agent.Run call. It folds the branch chain
// previously inlined in runEvalPass 3b together with the old
// refusalReasonStr helper (absorbed here; its only call site was 3e).
//
// Background (14-10 double-window replay FAIL — see 14-10-SUMMARY
// "Operator-Gate Addendum"): under the 1.1.0 floor tail the model refuses
// INSIDE the verdict schema — a well-formed verdict with
// status:"refused", a verdict.ValidStatuses member (validate.go) — which
// the pre-14-11 scoring path never recognized, so refusal prose was
// judged by faithfulness against weakly-related chunks instead of taking
// the refusal conventions. The 1.0.0 baseline (0.972 / 95%) was carried
// by the agent.Run-ERROR path (prose refusals failing verdict-JSON
// parse → empty answer → vacuous faith 1.0). 14-11 fixes the accounting
// here; the fix is deliberately minimal — it only routes the in-schema
// refusal into the SAME frozen conventions the two existing refusal
// paths take, with no new metrics and no arel/cprec semantics change.
//
// Outcome modalities (priority order — err wins, no double counting):
//   - err != nil      → (true, "", "agent_error")
//   - refusal != nil  → (true, "", refusal.Reason)
//   - v.Status=="refused" → (true, "", "verdict_status_refused")  [14-11]
//   - otherwise       → (false, verdict JSON, "")  (marshal failure
//     keeps answer="")
//
// Downstream conventions for refused=true + answer="" (frozen with the
// 1.0.0-era baseline; do NOT change unilaterally):
//   - Faithfulness: empty answer → 1.0 ("Refusal answer — nothing to
//     refute", internal/eval/faithfulness.go:75-78)
//   - ContextRecall: REFUSAL sentinel → 1.0 when groundTruth=="REFUSAL"
//     && refused (internal/eval/metrics.go:189-195); refusal_accuracy is
//     derived from it in internal/eval/archive.go (RefusalAccuracy)
//   - answer_relevancy / context_precision keep their existing
//     conventions (arel judges the empty answer; cprec judges the
//     retrieved hits) — the 1.0.0 baseline numbers are the product of
//     exactly this accounting.
//
// 14-12 update: the exemption is now explicit in the scoring loop via
// computeCaseMetrics, which short-circuits faithfulness to 1.0 when
// refused=true. This makes the re-baseline semantics a code-level
// contract rather than relying solely on the implicit empty-answer
// short-circuit in internal/eval/faithfulness.go:75-78.
func classifyRunOutcome(err error, refusal *verdict.Refusal, v verdict.Verdict) (refused bool, answer string, reason string) {
	if err != nil {
		return true, "", "agent_error"
	}
	if refusal != nil {
		return true, "", refusal.Reason
	}
	// 14-11: "refused" is a verdict.ValidStatuses member (validate.go).
	// Under the 1.1.0 floor tail the model refuses INSIDE the verdict
	// schema (well-formed JSON, Answer carrying refusal prose); route it
	// into the same refusal conventions as the two paths above instead
	// of judging the prose against retrieved chunks. status
	// "needs_input"/"error" deliberately do NOT enter here — this fix
	// only adopts the modality ValidStatuses already defines as a
	// refusal.
	if v.Status == "refused" {
		return true, "", "verdict_status_refused"
	}
	// Serialize the verdict to an answer string for the judge. The
	// verdict's Citations carry the load-bearing claims; including the
	// full JSON lets the judge see status + confidence too.
	if b, mErr := json.Marshal(v); mErr == nil {
		answer = string(b)
	}
	return false, answer, ""
}

// errString flattens an error for the --dump JSON (nil → "").
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
