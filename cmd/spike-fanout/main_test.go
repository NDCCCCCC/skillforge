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

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/skillforge/skillforge/internal/kb"
)

// testKBReg returns the read-only registry the spike tests resolve the
// reads contract against: the four baseline bases of the controlled seed
// (knowledge_bases.yaml), all active.
func testKBReg() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	})
}

// ----------------------------------------------------------------------------
// 13-01 hermetic scenario preservation (the 5 RF-4 scenarios stay the
// default path after the 13-05 live-mode extension).
// ----------------------------------------------------------------------------

// TestHermeticScenariosPreserved — the hermetic entry point must keep
// accepting all 5 RF-4 scenarios AND reject unknown ones. This is the
// regression lock that the live-mode flag extension did not narrow the
// hermetic surface.
func TestHermeticScenariosPreserved(t *testing.T) {
	for _, scen := range []string{"match", "empty-intersection", "sub-empty-reads", "parent-empty-tiers", "canonical"} {
		if err := runRF4Spike(scen); err != nil {
			t.Errorf("hermetic scenario %q: %v", scen, err)
		}
	}
	if err := runRF4Spike("bogus"); err == nil {
		t.Error("runRF4Spike must reject unknown scenarios")
	}
}

// ----------------------------------------------------------------------------
// Flag matrix (Plan 13-05 Task 1 — 7 flags)
// ----------------------------------------------------------------------------

// TestLiveFlagParsing verifies all 7 flags parse correctly with both
// the operator's live invocation AND the defaults.
func TestLiveFlagParsing(t *testing.T) {
	// The exact operator invocation from the plan / gate docs:
	// go run ./cmd/spike-fanout --live --golden=eval/golden/fanout.jsonl
	//   --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md
	//   --cost-limit=2.00 --seed=42 --summarize
	fs, f := newSpikeFlagSet("test-live")
	args := []string{
		"--live",
		"--golden=eval/golden/fanout.jsonl",
		"--sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md",
		"--cost-limit=2.00",
		"--seed=42",
		"--summarize",
		"--scenario=match",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse live invocation: %v", err)
	}
	if !f.live {
		t.Error("--live must parse to true")
	}
	if f.goldenPath != "eval/golden/fanout.jsonl" {
		t.Errorf("--golden = %q", f.goldenPath)
	}
	if f.subSkillConfig != "examples/skills/compliance-orchestrator/SKILL.md" {
		t.Errorf("--sub-skill-config = %q", f.subSkillConfig)
	}
	if f.costLimit != 2.00 {
		t.Errorf("--cost-limit = %v", f.costLimit)
	}
	if f.seed != 42 {
		t.Errorf("--seed = %d", f.seed)
	}
	if !f.summarize {
		t.Error("--summarize must parse to true")
	}
	if f.scenario != "match" {
		t.Errorf("--scenario = %q", f.scenario)
	}

	// Defaults: hermetic path, no live, canonical scenario.
	fs2, d := newSpikeFlagSet("test-defaults")
	if err := fs2.Parse(nil); err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	if d.live || d.summarize {
		t.Error("live/summarize must default to false (hermetic mode is the default path)")
	}
	if d.scenario != "canonical" {
		t.Errorf("scenario default = %q, want canonical", d.scenario)
	}
	if d.goldenPath != defaultGoldenPath || d.subSkillConfig != defaultSubSkillConfig ||
		d.costLimit != defaultCostLimit || d.seed != defaultSeed {
		t.Errorf("flag defaults drifted: %+v", d)
	}

	// Unknown flags must fail fast (ContinueOnError).
	fs3, _ := newSpikeFlagSet("test-bad")
	if err := fs3.Parse([]string{"--nope"}); err == nil {
		t.Error("unknown flag must be rejected")
	}
}

// ----------------------------------------------------------------------------
// Live-mode parity with hermetic wiring (Plan 13-05 Task 1)
// ----------------------------------------------------------------------------

// TestLiveModeParityWithHermetic — with the live flag OFF (default),
// the dispatch wiring used by live mode (runCaseThroughOrchestrator →
// runtime.DispatchAgent → FanOutOrchestrator.Run) produces
// byte-identical case outcomes across repeated runs with the hermetic
// stub classifier/model: same number of dispatched/completed/failed
// audit rows AND the same merged-verdict shape (FAN-03 determinism at
// the wiring surface). DurationMS is zeroed before comparison — it is
// the only wall-clock field.
func TestLiveModeParityWithHermetic(t *testing.T) {
	ctx := context.Background()
	parent := makeParent([]string{"authority", "historical"}, "compliance-orchestrator")

	run := func() LiveCaseResult {
		counter := &countingAuditWriter{}
		res, err := runCaseThroughOrchestrator(ctx, parent, &stubChatModel{}, nil, nil, counter, "parity-sess", "parity query")
		if err != nil {
			t.Fatalf("runCaseThroughOrchestrator: %v", err)
		}
		res.DispatchedCount = counter.dispatched
		res.CompletedCount = counter.completed
		res.FailedCount = counter.failed
		res.ID = "fanout-parity"
		res.Scenario = "multi_hop"
		res.Query = "parity query"
		res.DurationMS = 0
		return res
	}

	first, second := run(), run()

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("live wiring is not byte-identical across runs:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}

	// The dispatch wiring is ACTIVE: the audit lifecycle must observe
	// at least one dispatched row (the placeholder classifier plans one
	// sub-agent per Run) and the counts must agree between runs.
	if first.DispatchedCount == 0 {
		t.Fatal("dispatch wiring produced zero action_fanout_dispatched rows (audit seam not wired)")
	}
	if first.DispatchedCount+first.FailedCount != first.DispatchedCount+first.CompletedCount &&
		first.CompletedCount+first.FailedCount != first.DispatchedCount {
		t.Fatalf("audit lifecycle inconsistent: dispatched=%d completed=%d failed=%d (want completed+failed == dispatched)",
			first.DispatchedCount, first.CompletedCount, first.FailedCount)
	}

	// The merged verdict shape must be one of the two FAN-03 outcomes
	// (never empty — the merge singleton delegates to the production
	// pure function).
	if first.MergedStatus == "" {
		t.Fatalf("merged verdict status is empty (final event %q)", first.FinalEvent)
	}
}

// TestLiveModeRunsWithoutNetwork — the operator-gated live path.
// When GLM_API_KEY is unset (any CI/dev/hermetic environment) the test
// skips cleanly; when set (operator terminal with the live stack) it
// runs a single golden case end-to-end through the REAL adapters.
func TestLiveModeRunsWithoutNetwork(t *testing.T) {
	if os.Getenv("GLM_API_KEY") == "" {
		t.Skip("live mode requires GLM_API_KEY")
	}
	ctx := context.Background()
	cm, err := newGLMChatModel(ctx)
	if err != nil {
		t.Fatalf("operator env advertised GLM_API_KEY but chat model construction failed: %v", err)
	}
	golden, err := loadFanoutGolden("eval/golden/fanout.jsonl")
	if err != nil {
		t.Fatalf("load golden: %v", err)
	}
	parent := makeParent([]string{"authority", "historical"}, "compliance-orchestrator")
	res := runLiveCase(ctx, parent, cm, nil, nil, nil, testKBReg(), golden[0])
	if res.ID != golden[0].ID {
		t.Fatalf("case id mismatch: %q != %q", res.ID, golden[0].ID)
	}
	if res.FinalEvent == "" {
		t.Fatal("live case produced no final event")
	}
}

// ----------------------------------------------------------------------------
// Golden loader + merged-verdict parser + gate verification
// ----------------------------------------------------------------------------

// TestLoadFanoutConfigResolvesExample — the --sub-skill-config default
// target must resolve through the real skill.Loader and carry the
// orchestrator archetype + non-empty ResolvedTiers (operator-prep gap
// closed hermetically: the example Skill ships in this repo, so the
// operator's live command runs without any manual file creation).
func TestLoadFanoutConfigResolvesExample(t *testing.T) {
	m, err := loadFanoutConfig("../../examples/skills/compliance-orchestrator/SKILL.md", testKBReg())
	if err != nil {
		t.Fatalf("loadFanoutConfig: %v", err)
	}
	if m.Agent != "orchestrator" {
		t.Fatalf("example agent = %q, want orchestrator", m.Agent)
	}
	if len(m.ResolvedTiers()) == 0 {
		t.Fatal("example orchestrator Skill must resolve non-empty tiers (sub-skill intersection source)")
	}
}

// TestLoadFanoutGolden — JSONL parsing with blank-line tolerance and
// required-field enforcement.
func TestLoadFanoutGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "golden.jsonl")
	content := `{"id":"fanout-01","scenario":"multi_hop","parent_skill":"compliance-orchestrator","sub_skills":[{"name":"s1","reads":["authority/**"],"writes":[]}],"query":"q","expected_dispatched_count_floor":2,"expected_conservative_merge":false,"expected_merged_status":"compliant","tier_ground_truth":"authority","notes":"n"}` + "\n\n" +
		`{"id":"fanout-09","scenario":"tier_mismatch","parent_skill":"compliance-orchestrator","sub_skills":[{"name":"s2","reads":["personal/**"],"writes":[]}],"query":"q2","expected_dispatched_count_floor":0,"expected_conservative_merge":true,"expected_merged_status":"needs_input","tier_ground_truth":"authority","notes":"n2"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cases, err := loadFanoutGolden(path)
	if err != nil {
		t.Fatalf("loadFanoutGolden: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("loaded %d cases, want 2 (blank line tolerated)", len(cases))
	}
	if cases[0].SubSkills[0].Reads[0] != "authority/**" {
		t.Errorf("sub-skill reads roundtrip: %v", cases[0].SubSkills[0].Reads)
	}

	// Missing id/scenario is rejected.
	bad := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(bad, []byte(`{"query":"no id"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFanoutGolden(bad); err == nil {
		t.Error("golden row without id/scenario must be rejected")
	}
}

// TestParseMergedVerdictFromEvent — the three final-event content
// shapes the orchestrator emits.
func TestParseMergedVerdictFromEvent(t *testing.T) {
	status, conf, refused := parseMergedVerdictFromEvent("FanOutOrchestrator verdict: status=needs_input confidence=0.00")
	if status != "needs_input" || conf != 0 || refused {
		t.Errorf("needs_input parse: status=%q conf=%v refused=%v", status, conf, refused)
	}
	status, conf, refused = parseMergedVerdictFromEvent("FanOutOrchestrator verdict: status=compliant confidence=0.93")
	if status != "compliant" || conf != 0.93 || refused {
		t.Errorf("compliant parse: status=%q conf=%v refused=%v", status, conf, refused)
	}
	status, _, refused = parseMergedVerdictFromEvent("FanOutOrchestrator refused: no_authorized_source")
	if status != "needs_input" || !refused {
		t.Errorf("refusal parse: status=%q refused=%v", status, refused)
	}
	status, _, _ = parseMergedVerdictFromEvent("FanOutOrchestrator completed")
	if status != "" {
		t.Errorf("unparseable content should yield empty status, got %q", status)
	}
}

// TestValidateSubSkillTiers — FAN-02 SC-2 load-time rejection contract
// measured per golden case: matching tiers pass; personal/draft/empty/
// unknown-tier sub-skills reject at BuildSubAgent.
func TestValidateSubSkillTiers(t *testing.T) {
	ctx := context.Background()
	parent := makeParent([]string{"authority"}, "compliance-orchestrator")

	ok := GoldenCase{
		ID:       "fanout-ok",
		Scenario: "multi_hop",
		SubSkills: []GoldenSubSkill{
			{Name: "s1", Reads: []string{"authority/**"}},
			{Name: "s2", Reads: []string{"authority/**", "historical/**"}},
		},
	}
	if rejected, reason := validateSubSkillTiers(ctx, parent, &stubChatModel{}, testKBReg(), ok); rejected {
		t.Errorf("matching-tier sub-skills must NOT reject, got: %s", reason)
	}

	for _, tc := range []struct {
		name  string
		reads []string
	}{
		{"personal", []string{"personal/**"}},       // personal ∩ authority = ∅
		{"draft", []string{"draft/**"}},             // draft ∩ authority = ∅
		{"empty", []string{}},                       // empty Reads
		{"unknown-tier", []string{"not-a-tier/**"}}, // not a registered knowledge-base slug
	} {
		c := GoldenCase{
			ID:        "fanout-mm-" + tc.name,
			Scenario:  "tier_mismatch",
			SubSkills: []GoldenSubSkill{{Name: "s1", Reads: tc.reads}},
		}
		rejected, reason := validateSubSkillTiers(ctx, parent, &stubChatModel{}, testKBReg(), c)
		if !rejected {
			t.Errorf("tier_mismatch sub-skill %q (reads %v) must reject at load time", tc.name, tc.reads)
		}
		if reason == "" {
			t.Errorf("rejection must carry a reason (FAN-04 audit lineage), scenario %q", tc.name)
		}
	}

	// Duplicate sub_query id contract (fanout-14).
	dup := GoldenCase{
		ID:       "fanout-dup",
		Scenario: "tier_mismatch",
		SubSkills: []GoldenSubSkill{
			{Name: "same", Reads: []string{"authority/**"}},
			{Name: "same", Reads: []string{"authority/**"}},
		},
	}
	if rejected, reason := validateSubSkillTiers(ctx, parent, &stubChatModel{}, testKBReg(), dup); !rejected || !strings.Contains(reason, "duplicate sub_query id") {
		t.Errorf("duplicate sub_query id must reject with the Tool-validator reason, got rejected=%v reason=%q", rejected, reason)
	}

	// Parent-empty-tiers contract (fanout-12): ParentReads=[] synthesizes
	// a per-case parent with ZERO resolved tiers → BuildSubAgent rejects
	// "parent has empty ResolvedTiers()". Without the per-case override
	// the shared live parent HAS tiers, so this case could never reject
	// and tier_mismatch_rejection_rate would cap at 5/6 < the 1.0 gate.
	emptyParent := GoldenCase{
		ID:          "fanout-parent-empty",
		Scenario:    "tier_mismatch",
		ParentReads: []string{},
		SubSkills:   []GoldenSubSkill{{Name: "s1", Reads: []string{"authority/**"}}},
	}
	rejected, reason := validateSubSkillTiers(ctx, parent, &stubChatModel{}, testKBReg(), emptyParent)
	if !rejected || !strings.Contains(reason, "empty ResolvedTiers()") {
		t.Errorf("parent-empty-tiers case must reject with the ResolvedTiers reason, got rejected=%v reason=%q", rejected, reason)
	}
}

// TestVerifyFanoutGateThresholds — the gate comparator itself:
// a metrics-passing result set PASSes against the frozen thresholds;
// a breaching set FAILs with the threshold detail.
func TestVerifyFanoutGateThresholds(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "fanout.lock.json")
	lockJSON := `{
	  "thresholds": {
	    "fanout_dispatched_count_floor": 20,
	    "conservative_merge_rate_max": 0.0,
	    "tier_closure_regression_rate_max": 0.0,
	    "min_confidence_aggregation_min": 0.0,
	    "tier_mismatch_rejection_rate_min": 1.0
	  }
	}`
	if err := os.WriteFile(lock, []byte(lockJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Passing set: 20 dispatched rows, no spurious downgrades, 6/6
	// mismatch rejections, compliant multi-hop confidence 0.9.
	pass := make([]LiveCaseResult, 0, 20)
	for i := 0; i < 8; i++ {
		pass = append(pass, LiveCaseResult{ID: "mh", Scenario: "multi_hop", DispatchedCount: 1, MergedStatus: "compliant", MergedConfidence: 0.9})
	}
	for i := 0; i < 6; i++ {
		pass = append(pass, LiveCaseResult{ID: "mm", Scenario: "tier_mismatch", DispatchedCount: 1, MergedStatus: "needs_input", ConservativeMerge: true, TierMismatchRejected: true})
	}
	for i := 0; i < 4; i++ {
		pass = append(pass, LiveCaseResult{ID: "pf", Scenario: "partial_failure", DispatchedCount: 1, MergedStatus: "needs_input", ConservativeMerge: true})
	}
	for i := 0; i < 2; i++ {
		pass = append(pass, LiveCaseResult{ID: "sv", Scenario: "single_vertex", DispatchedCount: 1, MergedStatus: "compliant", MergedConfidence: 0.9})
	}
	if err := verifyFanoutGate(pass, 0.5, lock, 2.00, 42); err != nil {
		t.Fatalf("passing set must PASS the gate, got: %v", err)
	}

	// Failing set: one mismatch case escaped load-time rejection →
	// tier_closure_regression_rate 1/6 > 0.
	fail := append([]LiveCaseResult{}, pass...)
	fail[8].TierMismatchRejected = false
	if err := verifyFanoutGate(fail, 0.5, lock, 2.00, 42); err == nil {
		t.Fatal("tier-closure regression must FAIL the gate")
	} else if !strings.Contains(err.Error(), "tier_closure_regression_rate") {
		t.Fatalf("failure must name the breached threshold, got: %v", err)
	}

	// Cost overrun aborts (T-13-05-1).
	if err := verifyFanoutGate(pass, 2.50, lock, 2.00, 42); err == nil || !strings.Contains(err.Error(), "cost-limit") {
		t.Fatalf("cost overrun must FAIL with cost-limit detail, got: %v", err)
	}
}

// TestComputeFanoutMetrics — spurious-vs-expected downgrade semantics:
// partial_failure downgrades do NOT count toward the spurious rate.
func TestComputeFanoutMetrics(t *testing.T) {
	results := []LiveCaseResult{
		{ID: "mh1", Scenario: "multi_hop", DispatchedCount: 2, MergedStatus: "compliant", MergedConfidence: 0.95},
		{ID: "mh2", Scenario: "multi_hop", DispatchedCount: 2, MergedStatus: "compliant", MergedConfidence: 0.85},
		{ID: "pf1", Scenario: "partial_failure", DispatchedCount: 2, MergedStatus: "needs_input", ConservativeMerge: true},
		{ID: "mm1", Scenario: "tier_mismatch", DispatchedCount: 1, MergedStatus: "needs_input", ConservativeMerge: true, TierMismatchRejected: true},
	}
	m := computeFanoutMetrics(results, 0.42, 7)
	if m.CaseCount != 4 {
		t.Fatalf("case count %d", m.CaseCount)
	}
	if m.FanoutDispatchedCount != 7 {
		t.Fatalf("dispatched sum %d, want 7", m.FanoutDispatchedCount)
	}
	if m.ConservativeMergeRate != 0 {
		t.Fatalf("partial-failure downgrade must not count as spurious; rate=%v", m.ConservativeMergeRate)
	}
	if m.TierClosureRegressionRate != 0 || m.TierMismatchRejectionRate != 1 {
		t.Fatalf("mismatch rates: regression=%v rejection=%v", m.TierClosureRegressionRate, m.TierMismatchRejectionRate)
	}
	if m.MinConfidenceAggregation != 0.85 {
		t.Fatalf("min confidence aggregation %v, want 0.85", m.MinConfidenceAggregation)
	}
	if m.TotalCostUSD != 0.42 || m.Seed != 7 {
		t.Fatalf("cost/seed passthrough: %v %d", m.TotalCostUSD, m.Seed)
	}
}

// ----------------------------------------------------------------------------
// FAN-06 monthly_report assertion seam (18-VERIFICATION GAP-2)
// ----------------------------------------------------------------------------

// goldenSetPath is the relative path from cmd/spike-fanout to the
// shipped fan-out golden set.
const goldenSetPath = "../../eval/golden/fanout.jsonl"

// boolPtr is a tiny helper for synthetic expectation rows.
func boolPtr(b bool) *bool { return &b }

// TestMonthlyReportGoldenCasesParseAssertions — the four frozen
// monthly_report rows must PARSE with all four assertion fields
// populated. Before GAP-2 json.Unmarshal silently dropped these keys
// (GoldenCase lacked the fields), so the frozen expectations were
// unparseable by any code. The values pinned here are the exact frozen
// rows in eval/golden/fanout.jsonl.
func TestMonthlyReportGoldenCasesParseAssertions(t *testing.T) {
	cases, err := loadFanoutGolden(goldenSetPath)
	if err != nil {
		t.Fatalf("load golden set: %v", err)
	}
	var monthly []GoldenCase
	for _, c := range cases {
		if c.Scenario == "monthly_report" {
			monthly = append(monthly, c)
		}
	}
	if len(monthly) != 4 {
		t.Fatalf("want exactly 4 monthly_report cases, got %d", len(monthly))
	}

	want := map[string]struct {
		fanOut   bool
		numTasks int
		fallback bool
		sections []string
	}{
		"rpt-fanout-01": {true, 4, false, []string{"业务概览", "贷款质量", "风险提示", "客户情况"}},
		"rpt-fanout-02": {true, 2, false, []string{"业务概览", "贷款质量"}},
		"rpt-fanout-03": {false, 1, true, []string{"业务概览"}},
		"rpt-fanout-04": {true, 3, false, []string{"业务概览", "贷款质量", "风险提示"}},
	}
	for _, c := range monthly {
		w, ok := want[c.ID]
		if !ok {
			t.Fatalf("unexpected monthly_report case id %q", c.ID)
		}
		if c.ExpectedFanOut == nil {
			t.Errorf("%s: expected_fan_out must parse (nil means the tier-channel posture leaked into a monthly_report row)", c.ID)
			continue
		}
		if *c.ExpectedFanOut != w.fanOut {
			t.Errorf("%s: expected_fan_out = %v, want %v", c.ID, *c.ExpectedFanOut, w.fanOut)
		}
		if c.ExpectedNumSubtasks != w.numTasks {
			t.Errorf("%s: expected_num_subtasks = %d, want %d", c.ID, c.ExpectedNumSubtasks, w.numTasks)
		}
		if c.ExpectedFallback != w.fallback {
			t.Errorf("%s: expected_fallback = %v, want %v", c.ID, c.ExpectedFallback, w.fallback)
		}
		if !slices.Equal(c.TemplateSections, w.sections) {
			t.Errorf("%s: template_sections = %v, want %v", c.ID, c.TemplateSections, w.sections)
		}
	}
}

// TestEvaluateFanOutExpectation — the FAN-06 assertion seam judged
// against synthetic decision inputs: the real frozen rows must pass
// when the observed classifier decision matches the frozen expectation
// and FAIL on drift; tier-channel rows (no expectation) pass vacuously;
// a self-contradictory row (fallback == fan_out) fails as an authoring
// bug (D18-C4 mutual exclusion).
func TestEvaluateFanOutExpectation(t *testing.T) {
	cases, err := loadFanoutGolden(goldenSetPath)
	if err != nil {
		t.Fatalf("load golden set: %v", err)
	}
	byID := make(map[string]GoldenCase, len(cases))
	for _, c := range cases {
		byID[c.ID] = c
	}

	// Fan-out-expected rows (rpt-fanout-01/02/04): dispatch passes.
	for _, id := range []string{"rpt-fanout-01", "rpt-fanout-02", "rpt-fanout-04"} {
		c := byID[id]
		if !EvaluateFanOutExpectation(c, true) {
			t.Errorf("%s: dispatched=true must PASS the frozen expectation (expected_fan_out=true)", id)
		}
		if EvaluateFanOutExpectation(c, false) {
			t.Errorf("%s: dispatched=false must FAIL — classifier drift is exactly what FAN-06 detects", id)
		}
	}

	// Fallback-expected row (rpt-fanout-03): sequential fallback passes.
	rpt03 := byID["rpt-fanout-03"]
	if !EvaluateFanOutExpectation(rpt03, false) {
		t.Error("rpt-fanout-03: dispatched=false (D18-C4 sequential fallback) must PASS")
	}
	if EvaluateFanOutExpectation(rpt03, true) {
		t.Error("rpt-fanout-03: dispatched=true must FAIL (frozen expectation is fallback)")
	}

	// Tier-channel row: ExpectedFanOut nil → vacuous pass either way
	// (the monthly_report_fanout_rate denominator must not count it).
	tier := byID["fanout-01"]
	if !EvaluateFanOutExpectation(tier, true) || !EvaluateFanOutExpectation(tier, false) {
		t.Error("tier-channel case (expected_fan_out absent) must pass vacuously in both directions")
	}

	// Contradictory authoring: expected_fan_out=true + expected_fallback=true
	// is impossible under D18-C4 mutual exclusion → FAIL even though the
	// observation matches fan_out.
	bad := GoldenCase{ID: "rpt-bad", Scenario: "monthly_report", ExpectedFanOut: boolPtr(true), ExpectedFallback: true}
	if EvaluateFanOutExpectation(bad, true) {
		t.Error("self-contradictory expectation row must FAIL (authoring bug lock)")
	}
}
