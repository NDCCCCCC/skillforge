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

package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/skill"
)

// validParentForFanOut returns a parent manifest whose ResolvedTiers
// include ["权威", "历史"] so the validation tests have a known
// allowed set.
func validParentForFanOut() *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Description: "parent orchestrator",
		Agent:       "orchestrator",
		Reads:       []string{"authority/**", "historical/**"},
		Writes:      []string{},
		Steps:       "1. Classify\n2. Fan-out",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never invoke HITL tools",
				"Never bypass tier-closure",
				"Never expand tier boundary",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest([]string{"权威", "历史"})
	return m
}

// TestFanOutDispatchToolInvokableRun — happy path: 3 sub-queries with
// valid tier_filter ⊆ parent; tool returns success.
func TestFanOutDispatchToolInvokableRun(t *testing.T) {
	manifest := validParentForFanOut()
	tool := BuildFanOutDispatchTool(manifest)
	require.NotNil(t, tool)

	args := `{
		"sub_queries": [
			{"id": "sa1", "query": "first sub-query", "tier_filter": ["权威"]},
			{"id": "sa2", "query": "second sub-query", "tier_filter": ["历史"]},
			{"id": "sa3", "query": "third sub-query", "tier_filter": ["权威", "历史"]}
		]
	}`
	out, err := tool.InvokableRun(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, out, "declared_sub_queries")
	assert.Contains(t, out, "3")
}

// TestFanOutDispatchToolInvalidArgs — malformed JSON → error.
func TestFanOutDispatchToolInvalidArgs(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	_, err := tool.InvokableRun(context.Background(), "{not valid json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid args")
}

// TestFanOutDispatchToolEmptySubQueries — 0 sub-queries → error.
func TestFanOutDispatchToolEmptySubQueries(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	_, err := tool.InvokableRun(context.Background(), `{"sub_queries": []}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one sub-query required")
}

// TestFanOutDispatchToolDuplicateID — duplicate sub-query id → error.
func TestFanOutDispatchToolDuplicateID(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	args := `{"sub_queries": [
		{"id": "sa1", "query": "q1", "tier_filter": ["权威"]},
		{"id": "sa1", "query": "q2", "tier_filter": ["历史"]}
	]}`
	_, err := tool.InvokableRun(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// TestFanOutDispatchToolEmptyTierFilter — empty tier_filter → error.
func TestFanOutDispatchToolEmptyTierFilter(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	args := `{"sub_queries": [
		{"id": "sa1", "query": "q1", "tier_filter": []}
	]}`
	_, err := tool.InvokableRun(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty tier_filter")
}

// TestFanOutDispatchToolTierFilterOutsideParent — tier not in parent →
// error naming the parent's tier set (the two-layer defense).
func TestFanOutDispatchToolTierFilterOutsideParent(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	args := `{"sub_queries": [
		{"id": "sa1", "query": "q1", "tier_filter": ["个人"]}
	]}`
	_, err := tool.InvokableRun(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in parent ResolvedTiers")
}

// TestFanOutDispatchToolEmptyQuery — empty query string → error.
func TestFanOutDispatchToolEmptyQuery(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	args := `{"sub_queries": [
		{"id": "sa1", "query": "", "tier_filter": ["权威"]}
	]}`
	_, err := tool.InvokableRun(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing query")
}

// TestFanOutDispatchToolMissingID — sub query without id → error.
func TestFanOutDispatchToolMissingID(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	args := `{"sub_queries": [
		{"id": "", "query": "q1", "tier_filter": ["权威"]}
	]}`
	_, err := tool.InvokableRun(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing id")
}

// TestBuildFanOutDispatchToolRequiresParentManifest — nil parent → panic.
func TestBuildFanOutDispatchToolRequiresParentManifest(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("BuildFanOutDispatchTool(nil) must panic")
		}
	}()
	_ = BuildFanOutDispatchTool(nil)
}

// TestFanOutDispatchToolInfo — Info returns canonical Tool metadata.
func TestFanOutDispatchToolInfo(t *testing.T) {
	tool := BuildFanOutDispatchTool(validParentForFanOut())
	info, err := tool.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, FanOutDispatchToolName, info.Name)
	assert.True(t, strings.Contains(info.Desc, "sub-queries"),
		"the Tool description must mention sub-queries")
}

// ---------------------------------------------------------------------------
// CR-02 (18-REVIEW) — the data-channel V2 variant: extraction, V2 args
// handling, and the fan_out_success path. The tier-channel
// BuildFanOutDispatchTool must NEVER be wired for data-channel Skills
// (its ResolvedTiers validation rejects everything there).
// ---------------------------------------------------------------------------

// stubKPICaller is a hermetic KPICaller seam stub returning one
// canned snapshot per call.
type stubKPICaller struct {
	calls    []kpi.CallRequest
	snapshot *kpi.Snapshot
}

func (s *stubKPICaller) Call(_ context.Context, req kpi.CallRequest) (kpi.CallOutcome, error) {
	s.calls = append(s.calls, req)
	return kpi.CallOutcome{Snapshot: s.snapshot}, nil
}

// TestSectionTitlesAndKPIs — extraction from the manifest template's
// post-YAML map shape; index alignment titles[i] ↔ kpis[i] is the
// load-bearing property.
func TestSectionTitlesAndKPIs(t *testing.T) {
	var tmpl any
	raw := `{
		"title": "2026-08 业务月报",
		"sections": [
			{"title": "业务概览", "kpis": ["KPI_LOAN_BALANCE", "KPI_CUSTOMER_COUNT"], "slots": [{"type": "narrative"}]},
			{"title": "贷款质量", "kpis": ["KPI_NPL_RATIO"]},
			{"title": "风险提示"}
		]
	}`
	require.NoError(t, json.Unmarshal([]byte(raw), &tmpl))

	titles, kpis := sectionTitlesAndKPIs(tmpl)
	assert.Equal(t, []string{"业务概览", "贷款质量", "风险提示"}, titles)
	require.Len(t, kpis, 3)
	assert.Equal(t, []string{"KPI_LOAN_BALANCE", "KPI_CUSTOMER_COUNT"}, kpis[0])
	assert.Equal(t, []string{"KPI_NPL_RATIO"}, kpis[1])
	assert.Empty(t, kpis[2])

	// Defensive shapes → empty, never panic.
	defTitles, defKPIs := sectionTitlesAndKPIs(nil)
	assert.Nil(t, defTitles)
	assert.Nil(t, defKPIs)
	defTitles, defKPIs = sectionTitlesAndKPIs("not-a-map")
	assert.Nil(t, defTitles)
	assert.Nil(t, defKPIs)
	defTitles, defKPIs = sectionTitlesAndKPIs(map[string]any{"sections": "oops"})
	assert.Nil(t, defTitles)
	assert.Nil(t, defKPIs)
}

// TestDataChannelFanOutV2Success — the wired monthly-report shape:
// V2 args whose boundaries exactly match the extracted template
// titles fan out per section and respond fan_out_success, calling the
// executor once per section KPI.
func TestDataChannelFanOutV2Success(t *testing.T) {
	stub := &stubKPICaller{snapshot: &kpi.Snapshot{Func: "KPI_X", Rows: []map[string]any{}}}
	titles := []string{"业务概览", "贷款质量"}
	kpis := [][]string{{"KPI_LOAN_BALANCE", "KPI_CUSTOMER_COUNT"}, {"KPI_NPL_RATIO"}}
	fanOut := BuildFanOutDispatchToolDataChannel(titles, kpis, stub)

	args := `{"should_fan_out": true, "num_subtasks": 2, "task_boundaries": ["业务概览", "贷款质量"], "reason": "sections independent"}`
	out, err := fanOut.InvokableRun(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, out, `"fan_out_success"`)
	assert.Contains(t, out, `"num_subtasks":2`)

	require.Len(t, stub.calls, 3, "one executor call per section KPI (2+1)")
	// Sections run concurrently (errgroup) — collect + sort the names.
	gotNames := make([]string, 0, len(stub.calls))
	for _, c := range stub.calls {
		gotNames = append(gotNames, c.Name)
	}
	sort.Strings(gotNames)
	assert.Equal(t,
		[]string{"KPI_CUSTOMER_COUNT", "KPI_LOAN_BALANCE", "KPI_NPL_RATIO"}, gotNames)
}

// TestDataChannelFanOutV2Fallbacks — D18-C4 fallback paths never
// touch the executor.
func TestDataChannelFanOutV2Fallbacks(t *testing.T) {
	stub := &stubKPICaller{}
	fanOut := BuildFanOutDispatchToolDataChannel([]string{"业务概览"}, [][]string{{"KPI_LOAN_BALANCE"}}, stub)

	cases := []struct {
		name string
		args string
		want string
	}{
		{"invalid json", `{broken`, `"invalid_json"`},
		{"missing boundaries", `{"should_fan_out": true, "num_subtasks": 1}`, `"missing_field"`},
		{"boundary not in template", `{"should_fan_out": true, "num_subtasks": 1, "task_boundaries": ["不存在的章节"]}`, `"boundary_not_in_template"`},
		{"num mismatch", `{"should_fan_out": true, "num_subtasks": 2, "task_boundaries": ["业务概览"]}`, `"num_subtasks_mismatch"`},
		{"classifier declined", `{"should_fan_out": false, "num_subtasks": 0, "task_boundaries": []}`, `"classifier_declined_fan_out"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := fanOut.InvokableRun(context.Background(), tc.args)
			require.NoError(t, err, "fallbacks are tool RESULTS, not Go errors")
			assert.Contains(t, out, `"fallback_sequential"`)
			assert.Contains(t, out, tc.want)
		})
	}
	assert.Empty(t, stub.calls, "no KPI call may fire on any fallback path")
}

// TestMergeSnapshotsPreservesAll — WR-07 regression: every per-KPI
// snapshot survives the merge (the previous implementation returned
// only the first non-nil snapshot and dropped the rest — a section
// with N KPIs silently lost N-1 results).
func TestMergeSnapshotsPreservesAll(t *testing.T) {
	mk := func(name string) *kpi.Snapshot {
		return &kpi.Snapshot{Func: name, Rows: []map[string]any{{"v": 1}}}
	}
	got := mergeSnapshots([]*kpi.Snapshot{mk("A"), nil, mk("B"), mk("C")})
	require.Len(t, got, 3, "no snapshot may be silently dropped (WR-07)")
	assert.Equal(t, "A", got[0].Func)
	assert.Equal(t, "B", got[1].Func)
	assert.Equal(t, "C", got[2].Func)

	// All-nil input yields an empty slice (not nil — JSON encodes as []).
	assert.NotNil(t, mergeSnapshots(nil))
}

// TestDataChannelFanOutV2SectionResultsPreserveAllKPIs — WR-07 end
// to end: the success-path JSON response surfaces a per-section
// slice of EVERY KPI the section produced.
func TestDataChannelFanOutV2SectionResultsPreserveAllKPIs(t *testing.T) {
	stub := &stubKPICaller{snapshot: &kpi.Snapshot{Func: "KPI_X", Rows: []map[string]any{{"v": 1}}}}
	titles := []string{"业务概览", "贷款质量"}
	kpis := [][]string{{"KPI_LOAN_BALANCE", "KPI_CUSTOMER_COUNT"}, {"KPI_NPL_RATIO"}}
	fanOut := BuildFanOutDispatchToolDataChannel(titles, kpis, stub)

	args := `{"should_fan_out": true, "num_subtasks": 2, "task_boundaries": ["业务概览", "贷款质量"], "reason": "r"}`
	out, err := fanOut.InvokableRun(context.Background(), args)
	require.NoError(t, err)

	// Decode section_results as [][]map[string]any and assert length:
	// section 0 has 2 KPI entries, section 1 has 1.
	var resp struct {
		SectionResults [][]map[string]any `json:"section_results"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.SectionResults, 2)
	assert.Len(t, resp.SectionResults[0], 2, "section 0 must carry BOTH KPI snapshots")
	assert.Len(t, resp.SectionResults[1], 1)
}