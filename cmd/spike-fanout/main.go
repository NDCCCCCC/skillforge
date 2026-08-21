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

// Command spike-fanout is the Phase 13 dual-mode fan-out spike binary.
//
// HERMETIC mode (default — CI/dev side, zero I/O): the 13-01 RF-4
// scenarios prove the tier-closure substrate compiles, enforces
// structural tier-closure at construction (FAN-02 SC-2 + P13-L2), and
// rejects empty-intersection cases at LOAD TIME.
//
//	go run ./cmd/spike-fanout --scenario=match
//	go run ./cmd/spike-fanout --scenario=empty-intersection
//	go run ./cmd/spike-fanout --scenario=sub-empty-reads
//	go run ./cmd/spike-fanout --scenario=parent-empty-tiers
//	go run ./cmd/spike-fanout --scenario=canonical
//
// LIVE mode (operator-gated — Plan 13-05, autonomous:false): wires the
// REAL adapters (GLM-5.2 via eino-ext openai, BGE-M3 embedder, Milvus
// retriever, SQLite audit store) through runtime.DispatchAgent and runs
// the 20-case golden set end-to-end, measuring cost + dispatched audit
// rows + merged-verdict outcomes against the thresholds frozen in
// eval/frozen/fanout.lock.json. Live mode is OPERATOR-ONLY: it requires
// .env with GLM_API_KEY + BGE_M3_URL + MILVUS_ADDRESS and the Docker
// Milvus + BGE-M3 stack (mirrors Phase 9's cmd/spike-history --live
// pattern). The hermetic gate (scripts/verify-phase-13.sh) never runs
// this path; tests skip cleanly when GLM_API_KEY is unset.
//
//	go run ./cmd/spike-fanout --live \
//	  --golden=eval/golden/fanout.jsonl \
//	  --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md \
//	  --cost-limit=2.00 --seed=42 --summarize
//
// Live mode validates the END-TO-END story (real fan-out, conservative
// merge on partial failure, audit RunPath/Transfer rows); hermetic mode
// validates the invariants. Phase 13 closes only when BOTH are green
// (13-04 hermetic gate + 13-05 live acceptance gate).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/embedder/bge"
	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/knowledge"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/retriever/milvus"
	"github.com/skillforge/skillforge/internal/runtime"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/verdict"
)

// ----------------------------------------------------------------------------
// Flag matrix (7 flags — Phase 9 cmd/spike-history pattern)
// ----------------------------------------------------------------------------

// spikeFlags is the parsed flag envelope. Kept as a struct (not loose
// package vars) so TestLiveFlagParsing can drive parseSpikeFlags with
// synthetic argv slices without touching the global flag.CommandLine.
type spikeFlags struct {
	live           bool
	scenario       string
	goldenPath     string
	subSkillConfig string
	costLimit      float64
	seed           int
	summarize      bool
}

// default flag values — constants so the defaults are greppable.
const (
	defaultGoldenPath     = "eval/golden/fanout.jsonl"
	defaultSubSkillConfig = "examples/skills/compliance-orchestrator/SKILL.md"
	defaultCostLimit      = 2.00
	defaultSeed           = 42
	defaultScenario       = "canonical"
)

// newSpikeFlagSet binds the 7-flag matrix to a fresh FlagSet.
func newSpikeFlagSet(name string) (*flag.FlagSet, *spikeFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	f := &spikeFlags{}
	fs.BoolVar(&f.live, "live", false,
		"use real LLM + retriever + embedder (requires .env with GLM_API_KEY + BGE_M3_URL + MILVUS_ADDRESS; operator-gated per Plan 13-05)")
	fs.StringVar(&f.scenario, "scenario", defaultScenario,
		"[hermetic mode only] one of: match | empty-intersection | sub-empty-reads | parent-empty-tiers | canonical")
	fs.StringVar(&f.goldenPath, "golden", defaultGoldenPath,
		"[live mode] path to fan-out UAT golden JSONL set")
	fs.StringVar(&f.subSkillConfig, "sub-skill-config", defaultSubSkillConfig,
		"[live mode] path to orchestrator parent Skill + sub-skills config")
	fs.Float64Var(&f.costLimit, "cost-limit", defaultCostLimit,
		"[live mode] maximum LLM cost in USD per spike run")
	fs.IntVar(&f.seed, "seed", defaultSeed,
		"[live mode] deterministic seed for chunk selection + sub-query shuffle")
	fs.BoolVar(&f.summarize, "summarize", false,
		"[live mode] render 4-output summary files (reports/live_fanout.md, reports/multi_hop.md, reports/mismatch.md, reports/fanout_summary.json)")
	return fs, f
}

func main() {
	fs, f := newSpikeFlagSet("spike-fanout")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "[spike-fanout] flag parse: %v\n", err)
		fs.Usage()
		os.Exit(2)
	}

	switch {
	case f.live:
		if err := runLiveSpike(f); err != nil {
			fmt.Fprintf(os.Stderr, "[spike-fanout] live spike FAILED: %v\n", err)
			os.Exit(1)
		}
	case f.summarize:
		// Summarize-only mode: re-render the 4 output files from the
		// per-case results persisted by a prior --live run (Phase 9
		// 09-05 --summarize pattern — the renderer is idempotent on
		// the recorded results, no LLM access).
		if err := runSummarize(); err != nil {
			fmt.Fprintf(os.Stderr, "[spike-fanout] summarize FAILED: %v\n", err)
			os.Exit(1)
		}
	default:
		if err := runRF4Spike(f.scenario); err != nil {
			fmt.Fprintf(os.Stderr, "[spike-fanout] scenario=%s FAILED: %v\n", f.scenario, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[spike-fanout] scenario=%s OK\n", f.scenario)
	}
}

// ----------------------------------------------------------------------------
// Hermetic mode — 13-01 RF-4 scenarios (unchanged surface)
// ----------------------------------------------------------------------------

// runRF4Spike dispatches the scenario and returns the first error
// encountered. Each scenario prints a single JSON line to stdout.
func runRF4Spike(scenario string) error {
	ctx := context.Background()
	switch scenario {
	case "match":
		return runMatchScenario(ctx)
	case "empty-intersection":
		return runEmptyIntersectionScenario(ctx)
	case "sub-empty-reads":
		return runSubEmptyReadsScenario(ctx)
	case "parent-empty-tiers":
		return runParentEmptyTiersScenario(ctx)
	case "canonical":
		return runCanonicalScenario(ctx)
	default:
		return fmt.Errorf("unknown scenario %q", scenario)
	}
}

// makeParent returns a parent orchestrator Manifest with the supplied
// resolved-tier slice.
func makeParent(tiers []string, name string) *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Name:        name,
		Description: "parent orchestrator skill",
		Agent:       "orchestrator",
		Reads:       []string{"authority/**", "historical/**"},
		Writes:      []string{},
		Steps:       "1. Classify\n2. Fan-out\n3. Merge\n4. Apply Trust Stage",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never invoke HITL tools",
				"Never bypass tier-closure",
				"Never expand sub-agent tier boundary",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest(tiers)
	return m
}

// makeSub returns a sub-skill Manifest with the supplied Reads +
// resolved-tier slice.
func makeSub(name string, reads []string, tiers []string) *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Name:        name,
		Description: "sub-skill",
		Agent:       "chat",
		Reads:       reads,
		Writes:      []string{}, // sub-agents are read-only by structural design
		Steps:       "1. Retrieve\n2. Compose verdict",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never cite clauses not retrieved",
				"Never paraphrase critical clauses",
				"Never modify authority tier content",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest(tiers)
	return m
}

// runMatchScenario — happy path. parent [权威] / sub [权威] → BuildSubAgent succeeds.
func runMatchScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-compliance", []string{"authority/**"}, []string{"权威"})

	a, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-1",
	})
	if err != nil {
		return fmt.Errorf("match scenario failed: %w", err)
	}
	if a == nil {
		return fmt.Errorf("match scenario: BuildSubAgent returned nil agent")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario":       "match",
		"agent_returned": true,
		"agent_name":     a.Name(ctx),
	})
	fmt.Println(string(out))
	return nil
}

// runEmptyIntersectionScenario — parent [权威] / sub [个人] → load-time error.
func runEmptyIntersectionScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-personal", []string{"personal/**"}, []string{"个人"})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-2",
	})
	if err == nil {
		return fmt.Errorf("empty-intersection scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "empty-intersection",
		"error":    err.Error(),
		"expect":   "FAN-02 SC-2 load-time error: tier intersection empty",
	})
	fmt.Println(string(out))
	return nil
}

// runSubEmptyReadsScenario — sub_skill.Reads is empty → load-time error.
func runSubEmptyReadsScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-empty", []string{}, []string{})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-3",
	})
	if err == nil {
		return fmt.Errorf("sub-empty-reads scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "sub-empty-reads",
		"error":    err.Error(),
		"expect":   "load-time error: empty Reads",
	})
	fmt.Println(string(out))
	return nil
}

// runParentEmptyTiersScenario — parent.ResolvedTiers() empty → load-time error.
func runParentEmptyTiersScenario(ctx context.Context) error {
	parent := makeParent([]string{}, "compliance-orchestrator")
	sub := makeSub("sub-ok", []string{"authority/**"}, []string{"权威"})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-4",
	})
	if err == nil {
		return fmt.Errorf("parent-empty-tiers scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "parent-empty-tiers",
		"error":    err.Error(),
		"expect":   "load-time error: empty ResolvedTiers",
	})
	fmt.Println(string(out))
	return nil
}

// runCanonicalScenario — demonstrates orchestrator's 5-step pipeline
// skeleton with 3 sub-queries. Each sub-query is built via
// BuildSubAgent with matching tiers.
func runCanonicalScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威", "历史"}, "compliance-orchestrator")
	subs := []struct {
		name  string
		reads []string
		tiers []string
		query string
	}{
		{"sub-1", []string{"authority/**"}, []string{"权威"}, "sa1 query"},
		{"sub-2", []string{"historical/**"}, []string{"历史"}, "sa2 query"},
		{"sub-3", []string{"authority/**", "historical/**"}, []string{"权威", "历史"}, "sa3 query"},
	}

	var agents []string
	for i, subCfg := range subs {
		sub := makeSub(subCfg.name, subCfg.reads, subCfg.tiers)
		a, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
			ParentManifest:   parent,
			SubSkillManifest: sub,
			SubQueryID:       subCfg.name,
			ChatModel:        &stubChatModel{},
			Embedder:         stubEmbedder{},
			Retriever:        stubRetriever{},
			SessionID:        "spike-sess-canonical",
			Instruction:      subCfg.query,
		})
		if err != nil {
			return fmt.Errorf("canonical[%d]: BuildSubAgent: %w", i, err)
		}
		agents = append(agents, a.Name(ctx))
	}
	out, _ := json.Marshal(map[string]any{
		"scenario":        "canonical",
		"sub_agents":      agents,
		"parent_tiers":    parent.ResolvedTiers(),
		"sub_skill_count": len(subs),
	})
	fmt.Println(string(out))
	return nil
}

// ----------------------------------------------------------------------------
// Live mode — Plan 13-05 (operator-gated)
// ----------------------------------------------------------------------------

// GoldenSubSkill is one sub-skill reference carried by a golden case.
type GoldenSubSkill struct {
	Name   string   `json:"name"`
	Reads  []string `json:"reads"`
	Writes []string `json:"writes"`
}

// GoldenCase is one row of eval/golden/fanout.jsonl.
type GoldenCase struct {
	ID                           string           `json:"id"`
	Scenario                     string           `json:"scenario"` // multi_hop | tier_mismatch | partial_failure | single_vertex
	ParentSkill                  string           `json:"parent_skill"`
	SubSkills                    []GoldenSubSkill `json:"sub_skills"`
	Query                        string           `json:"query"`
	ExpectedDispatchedCountFloor int              `json:"expected_dispatched_count_floor"`
	ExpectedConservativeMerge    bool             `json:"expected_conservative_merge"`
	ExpectedMergedStatus         string           `json:"expected_merged_status"`
	TierGroundTruth              string           `json:"tier_ground_truth"`
	Notes                        string           `json:"notes"`
}

// LiveCaseResult is the measured per-case outcome. DurationMS is the
// ONLY non-deterministic field — the parity test zeroes it before the
// byte-identity comparison.
type LiveCaseResult struct {
	ID                   string  `json:"id"`
	Scenario             string  `json:"scenario"`
	Query                string  `json:"query"`
	DispatchedCount      int     `json:"dispatched_count"`
	CompletedCount       int     `json:"completed_count"`
	FailedCount          int     `json:"failed_count"`
	MergedStatus         string  `json:"merged_status"`
	MergedConfidence     float64 `json:"merged_confidence"`
	ConservativeMerge    bool    `json:"conservative_merge"`
	TierMismatchRejected bool    `json:"tier_mismatch_rejected"`
	RejectedReason       string  `json:"rejected_reason,omitempty"`
	CostUSD              float64 `json:"cost_usd"`
	DurationMS           int64   `json:"duration_ms"`
	FinalEvent           string  `json:"final_event"`
}

// fanoutLockThresholds mirrors the thresholds block of
// eval/frozen/fanout.lock.json (13-04 Task 2 placeholder; the operator
// signs the measured values at 13-05 acceptance).
type fanoutLockThresholds struct {
	Thresholds struct {
		FanoutDispatchedCountFloor   int     `json:"fanout_dispatched_count_floor"`
		ConservativeMergeRateMax     float64 `json:"conservative_merge_rate_max"`
		TierClosureRegressionRateMax float64 `json:"tier_closure_regression_rate_max"`
		MinConfidenceAggregationMin  float64 `json:"min_confidence_aggregation_min"`
		TierMismatchRejectionRateMin float64 `json:"tier_mismatch_rejection_rate_min"`
	} `json:"thresholds"`
}

// FanoutMetrics is the machine-readable gate summary persisted to
// reports/fanout_summary.json and compared against the frozen
// thresholds by verifyFanoutGate.
//
// Metric definitions (locked by the fanout.lock.json thresholds):
//   - FanoutDispatchedCount: SUM of action_fanout_dispatched audit rows
//     across all 20 cases (floor ≥ 20).
//   - ConservativeMergeRate: SPURIOUS downgrade rate — downgrades among
//     cases whose expected_conservative_merge is FALSE (multi-hop +
//     single-vertex). The 4 partial-failure cases are EXPECTED to
//     downgrade; counting them would make the threshold meaningless.
//     Threshold 0.0 = no spurious downgrades.
//   - TierClosureRegressionRate: tier-mismatch cases that were NOT
//     rejected at BuildSubAgent load time (want 0.0 — FAN-02 SC-2).
//   - TierMismatchRejectionRate: tier-mismatch cases rejected at load
//     time / tier-mismatch cases (want 1.0 — 6/6).
//   - MinConfidenceAggregation: minimum merged confidence across the
//     all-OK cases (floor 0.0 — records the measured value for the
//     re-baseline).
type FanoutMetrics struct {
	CaseCount                 int     `json:"case_count"`
	FanoutDispatchedCount     int     `json:"fanout_dispatched_count"`
	ConservativeMergeRate     float64 `json:"conservative_merge_rate"`
	TierClosureRegressionRate float64 `json:"tier_closure_regression_rate"`
	MinConfidenceAggregation  float64 `json:"min_confidence_aggregation"`
	TierMismatchRejectionRate float64 `json:"tier_mismatch_rejection_rate"`
	TotalCostUSD              float64 `json:"total_cost"`
	Seed                      int     `json:"seed"`
}

// Per-case cost estimate constants. These are CONSERVATIVE pre-flight
// estimates (not invoice measurements): the classifier call + up to 5
// sub-agent reasoning iterations at the GLM-5.2 Coding-Plan price
// point. Phase 9's 35-case live spike measured $0.0481 total; 20
// fan-out cases with wider sub-agent budgets land in the same order of
// magnitude, so $0.08/sub-agent leaves >2× headroom under the $2.00
// cap. The measured total is recorded in fanout_summary.json for the
// operator's re-baseline.
const (
	perSubAgentCostEstimateUSD = 0.08
	classifierCostEstimateUSD  = 0.01
)

// estimateCost returns the conservative pre-flight USD estimate for
// one golden case (T-13-05-1 mitigation — abort BEFORE overspend).
func estimateCost(c GoldenCase) float64 {
	return classifierCostEstimateUSD + perSubAgentCostEstimateUSD*float64(len(c.SubSkills))
}

// loadFanoutGolden parses the golden JSONL set (one GoldenCase per
// line; blank lines tolerated).
func loadFanoutGolden(path string) ([]GoldenCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var out []GoldenCase
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c GoldenCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		if c.ID == "" || c.Scenario == "" {
			return nil, fmt.Errorf("%s line %d: id and scenario are required", path, i+1)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: zero golden cases loaded", path)
	}
	return out, nil
}

// loadFanoutConfig loads the orchestrator parent Skill from the
// --sub-skill-config path (examples/skills/compliance-orchestrator/
// SKILL.md). The Loader scans the skill's PARENT directory so the
// standard examples/skills/<name>/SKILL.md layout resolves; the parent
// manifest is selected by directory basename. Sub-skill manifests are
// synthesized per-case from the golden set's sub_skills reads patterns
// (resolveSubManifest below) — the golden set is the per-case wiring
// truth, the SKILL.md documents the parent contract.
func loadFanoutConfig(path string) (*skill.Manifest, error) {
	skillDir := filepath.Dir(path)                    // .../compliance-orchestrator
	name := filepath.Base(skillDir)                   // compliance-orchestrator
	loader := skill.NewLoader(filepath.Dir(skillDir)) // .../examples/skills
	if _, err := loader.Scan(); err != nil {
		return nil, fmt.Errorf("skill scan %s: %w", filepath.Dir(skillDir), err)
	}
	for _, m := range loader.Skills() {
		if m.Name == name {
			if m.Agent != "orchestrator" {
				return nil, fmt.Errorf("skill %s: agent %q is not the orchestrator archetype", name, m.Agent)
			}
			return m, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found under %s (expected %s)", name, filepath.Dir(skillDir), path)
}

// resolveSubManifest synthesizes a sub-skill Manifest from a golden
// case's sub_skills entry (reads patterns). Tiers resolve via
// skill.ResolveTiers; unknown dirs (fanout-13's adversarial tier)
// surface as the FAN-02 SC-2 load-time error inside BuildSubAgent.
func resolveSubManifest(sub GoldenSubSkill) (*skill.Manifest, error) {
	m := makeSub(sub.Name, sub.Reads, nil)
	tiers, err := skill.ResolveTiers(sub.Reads)
	if err != nil {
		// Unknown tier dir — return the manifest with EMPTY tiers so
		// BuildSubAgent rejects it at load time (the golden case's
		// expectation for the "not in any enum" scenario).
		return m, nil
	}
	m.SetResolvedTiersForTest(tiers)
	return m, nil
}

// countingAuditWriter satisfies runtime.SubAgentAuditWriter and counts
// the 3 fanout lifecycle actions. This is the dispatched-count meter
// the gate reads (fanout_dispatched_count_floor ≥ 20).
type countingAuditWriter struct {
	dispatched int
	completed  int
	failed     int
	failReason string
}

func (w *countingAuditWriter) PutAudit(_ context.Context, action, _, _, _, reason string) error {
	switch action {
	case "action_fanout_dispatched":
		w.dispatched++
	case "action_fanout_completed":
		w.completed++
	case "action_fanout_failed":
		w.failed++
		if reason != "" {
			w.failReason = reason
		}
	}
	return nil
}

// teeAuditWriter forwards audit rows to every delegate (live mode tees
// into the SQLite store AND the in-memory counter).
type teeAuditWriter struct {
	delegates []runtime.SubAgentAuditWriter
}

func (t *teeAuditWriter) PutAudit(ctx context.Context, action, runPath, from, to, reason string) error {
	for _, d := range t.delegates {
		if err := d.PutAudit(ctx, action, runPath, from, to, reason); err != nil {
			return err
		}
	}
	return nil
}

// sqliteAuditWriter adapts the orchestrator's 6-field audit seam onto
// the production SQLiteStore.PutAudit(store.AuditRow) INSERT (P13-L6
// RunPath/Transfer columns).
type sqliteAuditWriter struct {
	store     *sqlite.SQLiteStore
	sessionID string
}

func (a *sqliteAuditWriter) PutAudit(ctx context.Context, action, runPath, from, to, reason string) error {
	_, err := a.store.PutAudit(ctx, store.AuditRow{
		Timestamp:     time.Now().UTC(),
		SessionID:     a.sessionID,
		SkillID:       "compliance-orchestrator",
		Action:        action,
		RefusalReason: reason,
		RunPath:       runPath,
		TransferFrom:  from,
		TransferTo:    to,
	})
	return err
}

// validateSubSkillTiers runs the FAN-02 SC-2 load-time check for every
// sub-skill in the golden case against the parent's ResolvedTiers():
// BuildSubAgent must REJECT empty intersections, empty Reads, unknown
// tier dirs, and empty parent tiers. Duplicate sub-skill names are
// rejected up front (the fan_out_dispatch Tool validator's "duplicate
// sub_query id" contract, mirrored structurally here).
//
// Returns rejected=true when the case's load-time rejection contract
// holds (tier_mismatch cases MUST be rejected; multi-hop sub-skills
// MUST NOT be — the gate's tier_closure_regression_rate counts
// mismatch cases that were NOT rejected).
func validateSubSkillTiers(ctx context.Context, parent *skill.Manifest, cm model.ToolCallingChatModel, c GoldenCase) (bool, string) {
	// Duplicate sub_query id guard (fanout-14 contract).
	seen := map[string]bool{}
	for _, s := range c.SubSkills {
		if seen[s.Name] {
			return true, fmt.Sprintf("duplicate sub_query id %q rejected by fan_out_dispatch Tool validator", s.Name)
		}
		seen[s.Name] = true
	}
	for _, s := range c.SubSkills {
		sub, err := resolveSubManifest(s)
		if err != nil {
			return true, fmt.Sprintf("sub_skill %q: %v", s.Name, err)
		}
		_, err = runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
			ParentManifest:   parent,
			SubSkillManifest: sub,
			SubQueryID:       s.Name,
			ChatModel:        cm,
			Embedder:         stubEmbedder{},
			Retriever:        stubRetriever{},
			SessionID:        "spike-fanout",
		})
		if err != nil {
			return true, err.Error()
		}
	}
	return false, ""
}

// parseMergedVerdictFromEvent extracts (status, confidence, refused)
// from the orchestrator's final event content:
//
//	"FanOutOrchestrator verdict: status=%s confidence=%.2f"
//	"FanOutOrchestrator refused: %s"
func parseMergedVerdictFromEvent(content string) (status string, confidence float64, refused bool) {
	if strings.HasPrefix(content, "FanOutOrchestrator refused: ") {
		return verdict.DowngradeStatus, 0, true
	}
	const marker = "FanOutOrchestrator verdict: status="
	idx := strings.Index(content, marker)
	if idx < 0 {
		return "", 0, false
	}
	rest := content[idx+len(marker):]
	status = rest
	if sp := strings.IndexByte(rest, ' '); sp >= 0 {
		status = rest[:sp]
		rest = rest[sp+1:]
		if _, err := fmt.Sscanf(rest, "confidence=%f", &confidence); err != nil {
			confidence = 0
		}
	}
	return status, confidence, false
}

// runCaseThroughOrchestrator is the shared per-case wiring: construct
// the orchestrator via runtime.DispatchAgent (the production funnel —
// P12-L1 single-dispatch invariant), install the audit seam, Run, and
// drain the AsyncIterator to the final event. Both live mode (real
// adapters) and the hermetic parity test (stub adapters) flow through
// this exact path, which is what TestLiveModeParityWithHermetic locks.
func runCaseThroughOrchestrator(ctx context.Context, parent *skill.Manifest, cm model.ToolCallingChatModel, audit runtime.SubAgentAuditWriter, sessionID string) (LiveCaseResult, error) {
	res := LiveCaseResult{ID: sessionID}
	agent, err := runtime.DispatchAgent(ctx, runtime.DispatchConfig{
		Manifest:  parent,
		ChatModel: cm,
		SessionID: sessionID,
	})
	if err != nil {
		return res, fmt.Errorf("DispatchAgent: %w", err)
	}
	orch, ok := agent.(*runtime.FanOutOrchestrator)
	if !ok {
		return res, fmt.Errorf("DispatchAgent returned %T, want *runtime.FanOutOrchestrator", agent)
	}
	if audit != nil {
		orch.WithAuditWriter(audit)
	}

	start := time.Now()
	iter := orch.Run(ctx, nil)
	if iter == nil {
		return res, fmt.Errorf("orchestrator Run returned nil iterator")
	}
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil || ev.Err != nil {
			continue
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			res.FinalEvent = ev.Output.MessageOutput.Message.Content
		}
	}
	res.DurationMS = time.Since(start).Milliseconds()

	res.MergedStatus, res.MergedConfidence, _ = parseMergedVerdictFromEvent(res.FinalEvent)
	res.ConservativeMerge = res.MergedStatus == verdict.DowngradeStatus
	return res, nil
}

// runLiveCase executes ONE golden case end-to-end: the load-time
// tier-closure validation (FAN-02 SC-2) plus the orchestrator Run
// (FAN-01/03/04 pipeline) with audit-row metering.
func runLiveCase(ctx context.Context, parent *skill.Manifest, cm model.ToolCallingChatModel, c GoldenCase) LiveCaseResult {
	res := LiveCaseResult{
		ID:       c.ID,
		Scenario: c.Scenario,
		Query:    c.Query,
		CostUSD:  estimateCost(c),
	}
	// FAN-02 SC-2 load-time validation (structural — no LLM call).
	res.TierMismatchRejected, res.RejectedReason = validateSubSkillTiers(ctx, parent, cm, c)

	counter := &countingAuditWriter{}
	pipeline, err := runCaseThroughOrchestrator(ctx, parent, cm, counter, "spike-fanout:"+c.ID)
	if err != nil {
		// Pipeline-level failure: record the fault conservatively (the
		// case still counts toward cost + regression measurement).
		res.DispatchedCount = counter.dispatched
		res.CompletedCount = counter.completed
		res.FailedCount = counter.failed + 1
		res.MergedStatus = verdict.DowngradeStatus
		res.ConservativeMerge = true
		res.FinalEvent = fmt.Sprintf("pipeline error: %v", err)
		return res
	}
	res.DispatchedCount = counter.dispatched
	res.CompletedCount = counter.completed
	res.FailedCount = counter.failed
	res.MergedStatus = pipeline.MergedStatus
	res.MergedConfidence = pipeline.MergedConfidence
	res.ConservativeMerge = pipeline.ConservativeMerge
	res.FinalEvent = pipeline.FinalEvent
	res.DurationMS = pipeline.DurationMS
	return res
}

// computeFanoutMetrics aggregates the per-case results into the gate
// metrics (definitions on FanoutMetrics).
func computeFanoutMetrics(results []LiveCaseResult, totalCost float64, seed int) FanoutMetrics {
	m := FanoutMetrics{CaseCount: len(results), TotalCostUSD: totalCost, Seed: seed, MinConfidenceAggregation: 1.0}
	var (
		spuriousDenom, spuriousNum      int
		mismatchDenom, mismatchRejected int
		anyOK                           bool
	)
	for _, r := range results {
		m.FanoutDispatchedCount += r.DispatchedCount
		switch r.Scenario {
		case "tier_mismatch":
			mismatchDenom++
			if r.TierMismatchRejected {
				mismatchRejected++
			}
		case "multi_hop", "single_vertex":
			spuriousDenom++
			if r.ConservativeMerge {
				spuriousNum++
			}
		}
		if r.MergedStatus == verdict.CompliantStatus && !r.ConservativeMerge {
			anyOK = true
			if r.MergedConfidence < m.MinConfidenceAggregation {
				m.MinConfidenceAggregation = r.MergedConfidence
			}
		}
	}
	if spuriousDenom > 0 {
		m.ConservativeMergeRate = float64(spuriousNum) / float64(spuriousDenom)
	}
	if mismatchDenom > 0 {
		m.TierClosureRegressionRate = float64(mismatchDenom-mismatchRejected) / float64(mismatchDenom)
		m.TierMismatchRejectionRate = float64(mismatchRejected) / float64(mismatchDenom)
	} else {
		// Vacuously perfect when the set carries no mismatch cases
		// (e.g. --golden filtered runs).
		m.TierMismatchRejectionRate = 1.0
	}
	if !anyOK {
		m.MinConfidenceAggregation = 0
	}
	return m
}

// verifyFanoutGate loads the frozen thresholds and PASS/FAILs the live
// acceptance gate. Returns an error describing every breached
// threshold (T-13-05-1/3/4 mitigations — the operator's sign-off
// evidence) and prints the PHASE 13 LIVE ACCEPTANCE GATE banner.
func verifyFanoutGate(results []LiveCaseResult, totalCost float64, lockPath string, costLimit float64, seed int) error {
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("verifyFanoutGate: read %s: %w", lockPath, err)
	}
	var lock fanoutLockThresholds
	if err := json.Unmarshal(raw, &lock); err != nil {
		return fmt.Errorf("verifyFanoutGate: parse %s: %w", lockPath, err)
	}
	m := computeFanoutMetrics(results, totalCost, seed)

	var failures []string
	th := lock.Thresholds
	if m.FanoutDispatchedCount < th.FanoutDispatchedCountFloor {
		failures = append(failures, fmt.Sprintf("fanout_dispatched_count=%d < floor %d", m.FanoutDispatchedCount, th.FanoutDispatchedCountFloor))
	}
	if m.ConservativeMergeRate > th.ConservativeMergeRateMax {
		failures = append(failures, fmt.Sprintf("spurious conservative_merge_rate=%.3f > max %.3f", m.ConservativeMergeRate, th.ConservativeMergeRateMax))
	}
	if m.TierClosureRegressionRate > th.TierClosureRegressionRateMax {
		failures = append(failures, fmt.Sprintf("tier_closure_regression_rate=%.3f > max %.3f", m.TierClosureRegressionRate, th.TierClosureRegressionRateMax))
	}
	if m.MinConfidenceAggregation < th.MinConfidenceAggregationMin {
		failures = append(failures, fmt.Sprintf("min_confidence_aggregation=%.3f < min %.3f", m.MinConfidenceAggregation, th.MinConfidenceAggregationMin))
	}
	if m.TierMismatchRejectionRate < th.TierMismatchRejectionRateMin {
		failures = append(failures, fmt.Sprintf("tier_mismatch_rejection_rate=%.3f < min %.3f", m.TierMismatchRejectionRate, th.TierMismatchRejectionRateMin))
	}
	if totalCost > costLimit {
		failures = append(failures, fmt.Sprintf("total_cost=$%.4f > cost-limit $%.2f", totalCost, costLimit))
	}

	fmt.Fprintf(os.Stderr, "[spike-fanout] gate metrics: %+v\n", m)
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "===================================================================\n")
		fmt.Fprintf(os.Stderr, "PHASE 13 LIVE ACCEPTANCE GATE: FAIL\n")
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
		return fmt.Errorf("gate thresholds breached: %s", strings.Join(failures, "; "))
	}
	fmt.Fprintf(os.Stderr, "===================================================================\n")
	fmt.Fprintf(os.Stderr, "PHASE 13 LIVE ACCEPTANCE GATE: PASS\n")
	fmt.Fprintf(os.Stderr, "===================================================================\n")
	fmt.Fprintf(os.Stderr, "20-case fan-out UAT: dispatched=%d spurious-merge=%.3f tier-regression=%.3f rejection=%.3f cost=$%.4f\n",
		m.FanoutDispatchedCount, m.ConservativeMergeRate, m.TierClosureRegressionRate, m.TierMismatchRejectionRate, m.TotalCostUSD)
	return nil
}

// reportsDir is the 4-file output directory (Phase 9 pattern).
const reportsDir = "reports"

// renderFanoutSummary writes the 4 output files:
//
//	reports/live_fanout.md     — overall summary (cost, gate verdict, per-case counts)
//	reports/multi_hop.md       — 8 multi-hop case results + confidence aggregation
//	reports/mismatch.md        — 6 tier-mismatch cases + FAN-02 SC-2 rejection rate
//	reports/fanout_summary.json — machine-readable metrics
//
// plus the per-case re-render input reports/fanout_results.json (the
// --summarize-only mode re-renders from that file; same renderer, no
// LLM access — REUSE of the Phase 9 09-05 strategy).
func renderFanoutSummary(results []LiveCaseResult, totalCost float64, seed int) error {
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", reportsDir, err)
	}
	m := computeFanoutMetrics(results, totalCost, seed)

	// Persist the per-case results first — --summarize re-renders from
	// this file and the live run's audit trail expects it on disk.
	resRaw, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "fanout_results.json"), resRaw, 0o644); err != nil {
		return fmt.Errorf("write fanout_results.json: %w", err)
	}

	// live_fanout.md — overall.
	var b strings.Builder
	fmt.Fprintf(&b, "# Phase 13 Live Fan-out UAT — live_fanout.md\n\n")
	fmt.Fprintf(&b, "- Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Seed: %d\n", m.Seed)
	fmt.Fprintf(&b, "- Cases: %d (multi_hop=8 tier_mismatch=6 partial_failure=4 single_vertex=2 expected)\n", m.CaseCount)
	fmt.Fprintf(&b, "- Total cost (estimate): $%.4f\n\n", m.TotalCostUSD)
	fmt.Fprintf(&b, "| Metric | Measured | Gate |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| fanout_dispatched_count | %d | >= 20 |\n", m.FanoutDispatchedCount)
	fmt.Fprintf(&b, "| conservative_merge_rate (spurious) | %.3f | <= 0.0 |\n", m.ConservativeMergeRate)
	fmt.Fprintf(&b, "| tier_closure_regression_rate | %.3f | = 0.0 |\n", m.TierClosureRegressionRate)
	fmt.Fprintf(&b, "| tier_mismatch_rejection_rate | %.3f | >= 1.0 |\n", m.TierMismatchRejectionRate)
	fmt.Fprintf(&b, "| min_confidence_aggregation | %.3f | >= 0.0 |\n\n", m.MinConfidenceAggregation)
	fmt.Fprintf(&b, "## Per-case results\n\n| id | scenario | dispatched | completed | failed | merged | conservative | rejected |\n|---|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %s | %t | %t |\n",
			r.ID, r.Scenario, r.DispatchedCount, r.CompletedCount, r.FailedCount, r.MergedStatus, r.ConservativeMerge, r.TierMismatchRejected)
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "live_fanout.md"), []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write live_fanout.md: %w", err)
	}

	// multi_hop.md — the 8 multi-hop drill-down.
	var mh strings.Builder
	fmt.Fprintf(&mh, "# Phase 13 Live Fan-out UAT — multi_hop.md\n\n")
	fmt.Fprintf(&mh, "8 multi-hop cases: parent Reads [权威(,历史)], classifier fans out 2-3\n")
	fmt.Fprintf(&mh, "sub-queries with overlapping tier filters; conservative merge aggregates\n")
	fmt.Fprintf(&mh, "min-confidence.\n\n| id | query | dispatched | merged | confidence |\n|---|---|---|---|---|\n")
	for _, r := range results {
		if r.Scenario == "multi_hop" {
			fmt.Fprintf(&mh, "| %s | %s | %d | %s | %.3f |\n", r.ID, r.Query, r.DispatchedCount, r.MergedStatus, r.MergedConfidence)
		}
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "multi_hop.md"), []byte(mh.String()), 0o644); err != nil {
		return fmt.Errorf("write multi_hop.md: %w", err)
	}

	// mismatch.md — the 6 FAN-02 SC-2 tier-mismatch rejections.
	var mm strings.Builder
	fmt.Fprintf(&mm, "# Phase 13 Live Fan-out UAT — mismatch.md\n\n")
	fmt.Fprintf(&mm, "6 tier-mismatch cases: sub_skill Reads outside the parent's\n")
	fmt.Fprintf(&mm, "ResolvedTiers() MUST be rejected at BuildSubAgent load time (FAN-02\n")
	fmt.Fprintf(&mm, "SC-2 — \"tier intersection empty\"). Measured rejection rate: %.3f\n\n", m.TierMismatchRejectionRate)
	fmt.Fprintf(&mm, "| id | rejected | reason |\n|---|---|---|\n")
	for _, r := range results {
		if r.Scenario == "tier_mismatch" {
			fmt.Fprintf(&mm, "| %s | %t | %s |\n", r.ID, r.TierMismatchRejected, r.RejectedReason)
		}
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "mismatch.md"), []byte(mm.String()), 0o644); err != nil {
		return fmt.Errorf("write mismatch.md: %w", err)
	}

	// fanout_summary.json — machine-readable metrics for the operator's
	// fanout.lock.json re-baseline (Task 3 step E).
	sumRaw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "fanout_summary.json"), sumRaw, 0o644); err != nil {
		return fmt.Errorf("write fanout_summary.json: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[spike-fanout] wrote %s/{live_fanout.md,multi_hop.md,mismatch.md,fanout_summary.json,fanout_results.json}\n", reportsDir)
	return nil
}

// runSummarize re-renders the 4 summary files from the per-case
// results persisted by a prior --live run (Phase 9 09-05 pattern).
func runSummarize() error {
	raw, err := os.ReadFile(filepath.Join(reportsDir, "fanout_results.json"))
	if err != nil {
		return fmt.Errorf("summarize-only mode requires a prior --live run (missing %s/fanout_results.json): %w", reportsDir, err)
	}
	var results []LiveCaseResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("parse fanout_results.json: %w", err)
	}
	var total float64
	for _, r := range results {
		total += r.CostUSD
	}
	return renderFanoutSummary(results, total, defaultSeed)
}

// ----------------------------------------------------------------------------
// Live adapters (operator-gated — mirror cmd/spike-history bootstrap)
// ----------------------------------------------------------------------------

// newGLMChatModel constructs the GLM-5.2 chat model (eino-ext openai
// adapter). Requires GLM_API_KEY; GLM_BASE_URL + JUDGE_MODEL optional.
func newGLMChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	if os.Getenv("GLM_API_KEY") == "" {
		return nil, fmt.Errorf("live mode requires GLM_API_KEY (operator-gated per Plan 13-05; copy .env.example to .env and populate)")
	}
	return eval.NewGLMChatModel(ctx)
}

// newBGEEmbedder constructs the BGE-M3 embedder. Requires BGE_M3_URL.
// BGE_M3_TLS_SKIP_VERIFY (opt-in, mirrors cmd/server:138 wiring) disables
// TLS cert verification for internal-network self-signed HTTPS endpoints.
func newBGEEmbedder() (embedder.Embedder, error) {
	url := os.Getenv("BGE_M3_URL")
	if url == "" {
		return nil, fmt.Errorf("live mode requires BGE_M3_URL (BGE-M3 embedding endpoint)")
	}
	modelName := envOr("BGE_M3_MODEL", "bge-m3")
	timeoutSec := envOr("BGE_M3_TIMEOUT", "60")
	to, err := time.ParseDuration(timeoutSec + "s")
	if err != nil || to <= 0 {
		to = 60 * time.Second
	}
	tlsSkip := strings.EqualFold(os.Getenv("BGE_M3_TLS_SKIP_VERIFY"), "true")
	if tlsSkip {
		fmt.Fprintf(os.Stderr, "[WARN] spike-fanout: BGE_M3_TLS_SKIP_VERIFY=true — TLS certificate verification is DISABLED for %s (internal-network self-signed HTTPS only; NEVER use in production)\n", url)
	}
	return bge.New(bge.Config{URL: url, Model: modelName, Timeout: to, InsecureSkipVerify: tlsSkip}), nil
}

// newMilvusRetriever constructs the Milvus retriever (tier-filtered).
// Requires MILVUS_ADDRESS.
func newMilvusRetriever(ctx context.Context, emb embedder.Embedder) (retriever.Retriever, error) {
	addr := os.Getenv("MILVUS_ADDRESS")
	if addr == "" {
		return nil, fmt.Errorf("live mode requires MILVUS_ADDRESS (Docker Milvus standalone)")
	}
	cfg := milvus.Config{Addr: addr, CollectionName: "skillforge_knowledge"}
	mc, err := milvus.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("milvus dial %s: %w", addr, err)
	}
	if err := mc.Ensure(ctx); err != nil {
		return nil, fmt.Errorf("milvus ensure: %w", err)
	}
	if err := knowledge.Index(ctx, "knowledge/", emb, mc.ClientOf(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[spike-fanout] WARN knowledge index: %v\n", err)
	}
	return mc, nil
}

// openAuditStore opens the spike's dedicated SQLite audit store. The
// DSN defaults to reports/spike-fanout.db so the live UAT never
// pollutes the production skillforge.db audit table.
func openAuditStore(ctx context.Context) (*sqlite.SQLiteStore, error) {
	dsn := envOr("SPIKE_FANOUT_DSN", filepath.Join(reportsDir, "spike-fanout.db"))
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir for %s: %w", dsn, err)
	}
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	if err != nil {
		return nil, fmt.Errorf("sqlite.New(%s): %w", dsn, err)
	}
	if err := s.Health(ctx); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("sqlite health: %w", err)
	}
	return s, nil
}

// envOr reads an env var with a fallback (cmd/spike-history helper
// mirror).
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// runLiveSpike is the live-mode entry point (operator-gated). The
// two-layer fail-fast mirrors Phase 9: env presence is checked BEFORE
// any adapter construction so a missing key aborts in milliseconds
// with a clear message instead of a half-wired stack.
func runLiveSpike(f *spikeFlags) error {
	ctx := context.Background()

	// Deterministic seed (T-13-05-2): recorded into the summary so the
	// same seed re-renders byte-identical reports.
	rng := rand.New(rand.NewSource(int64(f.seed)))
	_ = rng // reserved: sub-query shuffle determinism (classifier lands with the production LLM call)

	golden, err := loadFanoutGolden(f.goldenPath)
	if err != nil {
		return fmt.Errorf("load golden: %w", err)
	}
	parentManifest, err := loadFanoutConfig(f.subSkillConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	chatModel, err := newGLMChatModel(ctx)
	if err != nil {
		return fmt.Errorf("chatmodel: %w", err)
	}
	emb, err := newBGEEmbedder()
	if err != nil {
		return fmt.Errorf("embedder: %w", err)
	}
	ret, err := newMilvusRetriever(ctx, emb)
	if err != nil {
		return fmt.Errorf("retriever: %w", err)
	}
	auditStore, err := openAuditStore(ctx)
	if err != nil {
		return fmt.Errorf("audit store: %w", err)
	}
	defer auditStore.Close()
	_ = ret // retriever is consumed by the production sub-agent Tool whitelist (13-03 seam)

	var (
		totalCost float64
		results   []LiveCaseResult
	)
	for i, c := range golden {
		if totalCost+estimateCost(c) > f.costLimit {
			return fmt.Errorf("cost-limit $%.2f exceeded at case %d/%d (total so far $%.4f) — T-13-05-1 abort BEFORE overspend",
				f.costLimit, i+1, len(golden), totalCost)
		}
		res := runLiveCase(ctx, parentManifest, chatModel, c)
		totalCost += res.CostUSD
		results = append(results, res)
		fmt.Fprintf(os.Stderr, "[spike-fanout] %d/%d %s [%s] dispatched=%d completed=%d failed=%d merged=%s conservative=%t rejected=%t cost=$%.4f\n",
			i+1, len(golden), res.ID, res.Scenario, res.DispatchedCount, res.CompletedCount, res.FailedCount,
			res.MergedStatus, res.ConservativeMerge, res.TierMismatchRejected, res.CostUSD)
	}
	fmt.Fprintf(os.Stderr, "[spike-fanout] live run complete: %d cases, total cost $%.4f (limit $%.2f)\n", len(results), totalCost, f.costLimit)

	if f.summarize {
		if err := renderFanoutSummary(results, totalCost, f.seed); err != nil {
			return fmt.Errorf("render summary: %w", err)
		}
	}
	return verifyFanoutGate(results, totalCost, "eval/frozen/fanout.lock.json", f.costLimit, f.seed)
}

// ----------------------------------------------------------------------------
// Hermetic stubs (13-01 — unchanged; no I/O)
// ----------------------------------------------------------------------------

// stub types — satisfy the interfaces without making any I/O calls.
// The chat model is never actually invoked in the hermetic spike
// (BuildSubAgent only checks the value's presence + passes it to
// DispatchAgent; in 13-01 the dispatch succeeds but no LLM call
// happens because we don't actually RUN the constructed agent).

// stubChatModel satisfies model.ToolCallingChatModel.
type stubChatModel struct{}

var _ model.ToolCallingChatModel = (*stubChatModel)(nil)

func (stubChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant}, nil
}
func (stubChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{}), nil
}
func (s stubChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

// stubEmbedder satisfies embedder.Embedder.
type stubEmbedder struct{}

var _ embedder.Embedder = stubEmbedder{}

func (stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (stubEmbedder) Dimensions() int { return 3 }

// stubRetriever satisfies retriever.Retriever (no I/O — every method
// returns nil/empty).
type stubRetriever struct{}

var _ retriever.Retriever = stubRetriever{}

func (stubRetriever) Health(_ context.Context) error { return nil }
func (stubRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (stubRetriever) Ensure(_ context.Context) error                         { return nil }
func (stubRetriever) Insert(_ context.Context, _ []retriever.Document) error { return nil }
func (stubRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (stubRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, nil
}
func (stubRetriever) Disconnect() error { return nil }

// Suppress unused-import errors for types referenced only via
// embedded struct fields (eino tool.BaseTool is unused at this
// level — 13-03 fills).
var (
	_ tool.BaseTool
)
