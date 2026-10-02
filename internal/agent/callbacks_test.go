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
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
)

// toolRunInfo builds the RunInfo eino passes to a Tool OnStart callback.
// eino sets Name to the tool name and Component to ComponentOfTool
// (eino@v0.9.14/compose/tool_node.go:896-899); the utils/callbacks
// template routes on Component (template.go:208-209), so both fields are
// load-bearing for this test to reach toolHandler.OnStart at all.
func toolRunInfo(name string) *callbacks.RunInfo {
	return &callbacks.RunInfo{Name: name, Component: components.ComponentOfTool}
}

// TestCallbackToolDispatch — Phase 16 / Plan 16-08 Task 1 (Pitfall 5 /
// D16-B3): the Tool OnStart callback must dispatch on tool name so a
// kpi_call hop is NEVER labelled `retrieve` on the SSE timeline. The
// kpi_reference decision (with DataRef) is emitted by the kpi_call
// closure instead — the two paths are mutually exclusive.
func TestCallbackToolDispatch(t *testing.T) {
	ctx := context.Background()

	// ① kpi_call OnStart → zero decisions emitted from the callback layer.
	t.Run("kpi_call emits no retrieve decision", func(t *testing.T) {
		var got []DecisionRecord
		handler, sc := newDecisionCallback(func(step int, decision, toolCallID, query string, confidence float64) {
			got = append(got, DecisionRecord{Step: step, Decision: decision, ToolCallID: toolCallID, Query: query, Confidence: confidence})
		})
		require.NotNil(t, handler)
		require.NotNil(t, sc)

		handler.OnStart(ctx, toolRunInfo(agenttools.KPICallToolName),
			&tool.CallbackInput{ArgumentsInJSON: `{"name":"loan_overdue_rate_by_status","params":{"status":"逾期"}}`})

		assert.Empty(t, got,
			"kpi_call OnStart MUST emit zero decisions (its kpi_reference decision rides the tool closure — Pitfall 5)")
		assert.Equal(t, 1, sc.current(),
			"the hop counter MUST still advance for a kpi_call hop (shared timeline coordinate)")
	})

	// ② retrieve tool OnStart → DecisionRetrieve still fires (regression).
	t.Run("retrieve tool still emits retrieve decision", func(t *testing.T) {
		var got []DecisionRecord
		handler, _ := newDecisionCallback(func(step int, decision, toolCallID, query string, confidence float64) {
			got = append(got, DecisionRecord{Step: step, Decision: decision, ToolCallID: toolCallID, Query: query, Confidence: confidence})
		})
		require.NotNil(t, handler)

		handler.OnStart(ctx, toolRunInfo(agenttools.RetrieveToolName),
			&tool.CallbackInput{ArgumentsInJSON: `{"query":"年假 结转"}`})

		require.Len(t, got, 1, "retrieve OnStart MUST emit exactly one decision (Phase 3 behavior preserved)")
		assert.Equal(t, DecisionRetrieve, got[0].Decision)
		assert.Equal(t, 1, got[0].Step)
		assert.Equal(t, "年假 结转", got[0].Query,
			"the retrieve decision keeps carrying the parsed query argument")
	})

	// ③ Mutual exclusion across a mixed timeline: only the retrieve hop
	// produces a decision chunk; the kpi_call hops leave the timeline
	// unpolluted while the step numbers still count every hop.
	t.Run("mixed timeline stays unpolluted", func(t *testing.T) {
		var got []DecisionRecord
		handler, sc := newDecisionCallback(func(step int, decision, toolCallID, query string, confidence float64) {
			got = append(got, DecisionRecord{Step: step, Decision: decision, ToolCallID: toolCallID, Query: query, Confidence: confidence})
		})
		require.NotNil(t, handler)

		kpiArgs := &tool.CallbackInput{ArgumentsInJSON: `{"name":"loan_overdue_rate_by_status","params":{}}`}
		handler.OnStart(ctx, toolRunInfo(agenttools.KPICallToolName), kpiArgs)
		handler.OnStart(ctx, toolRunInfo(agenttools.RetrieveToolName),
			&tool.CallbackInput{ArgumentsInJSON: `{"query":"制度依据"}`})
		handler.OnStart(ctx, toolRunInfo(agenttools.KPICallToolName), kpiArgs)

		require.Len(t, got, 1,
			"exactly one decision for three hops: only the retrieve hop emits from the callback layer")
		assert.Equal(t, DecisionRetrieve, got[0].Decision)
		assert.Equal(t, 2, got[0].Step, "the retrieve hop is hop #2 — kpi_call hops still advance the counter")
		assert.Equal(t, 3, sc.current())

		for _, r := range got {
			assert.NotEqual(t, "kpi_reference", r.Decision,
				"the callback layer NEVER emits kpi_reference — that leg belongs to the tool closure (DataRefEmitFunc)")
		}
	})

	// ④ Level-2 shape probe: when RunInfo carries no name (hand-rolled
	// dispatch / future eino change), the kpiCallArgs shape still routes
	// to the kpi_call branch, and a retrieve-shaped payload does not.
	t.Run("shape probe fallback without RunInfo name", func(t *testing.T) {
		var got []DecisionRecord
		handler, _ := newDecisionCallback(func(step int, decision, toolCallID, query string, confidence float64) {
			got = append(got, DecisionRecord{Step: step, Decision: decision, ToolCallID: toolCallID, Query: query, Confidence: confidence})
		})
		require.NotNil(t, handler)

		nameless := &callbacks.RunInfo{Component: components.ComponentOfTool}
		handler.OnStart(ctx, nameless,
			&tool.CallbackInput{ArgumentsInJSON: `{"name":"loan_overdue_rate_by_status","params":{"status":"逾期"}}`})
		assert.Empty(t, got, "kpiCallArgs shape MUST route to the kpi_call branch even without RunInfo.Name")

		handler.OnStart(ctx, nameless, &tool.CallbackInput{ArgumentsInJSON: `{"query":"年假"}`})
		require.Len(t, got, 1, "retrieve-shaped arguments MUST keep emitting a retrieve decision (conservative default)")
		assert.Equal(t, DecisionRetrieve, got[0].Decision)
	})
}
