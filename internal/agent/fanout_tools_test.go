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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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