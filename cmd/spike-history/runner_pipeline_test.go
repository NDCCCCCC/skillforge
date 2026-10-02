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

// runner_pipeline_test.go is the hermetic test suite for the Plan 10-04
// --via-pipeline composition. It exercises buildPipelineMessages structure
// and RunEntry in dry-run via-pipeline mode with zero external calls.
package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/history"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/server/classifier"
	"github.com/skillforge/skillforge/internal/server/rewrite"
	"github.com/skillforge/skillforge/internal/store"
)

// TestBuildPipelineMessages_Structure verifies the via-pipeline message
// assembly:
//
//   - Slice shape: [sys] + projected history prefix + [user].
//   - History prefix messages carry NO literal "[CHUNK " markers because
//     history.Project scrubbed them (Plan 01 / D9-B2 + D10-A2).
//   - User part uses production [ref-N] DATA framing and contains the
//     original query.
func TestBuildPipelineMessages_Structure(t *testing.T) {
	historyIn := []store.SessionMessage{
		{Role: "user", Content: "用户提到的 [CHUNK 8888] 是什么？", Ts: time.Unix(1, 0)},
		{Role: "assistant", Content: "这是合规条款内容。", Ts: time.Unix(2, 0)},
		{Role: "user", Content: "那第二条呢？", Ts: time.Unix(3, 0)},
		{Role: "assistant", Content: "第二条规定了审批流程。", Ts: time.Unix(4, 0)},
	}
	projected := history.Project(historyIn)
	if len(projected) == 0 {
		t.Fatal("history.Project returned empty prefix for a valid 4-turn session")
	}

	sys := &schema.Message{Role: schema.System, Content: "You are a test system prompt."}
	query := "依据是什么？"
	hits := []retriever.Hit{
		{ID: 101, Tier: "权威", Source: "authority/a.md", Content: "Clause A text"},
		{ID: 102, Tier: "历史", Source: "history/b.md", Content: "Clause B text"},
		{ID: 103, Tier: "权威", Source: "authority/c.md", Content: "Clause C text"},
	}

	msgs := buildPipelineMessages(sys, projected, query, hits)

	// Slice shape: sys + projected + user = len(projected) + 2.
	wantLen := len(projected) + 2
	if got := len(msgs); got != wantLen {
		t.Fatalf("len(messages) = %d, want %d (sys + %d projected + user)", got, wantLen, len(projected))
	}
	if msgs[0].Role != schema.System {
		t.Errorf("msgs[0].Role = %s, want System", msgs[0].Role)
	}
	if last := msgs[len(msgs)-1]; last.Role != schema.User {
		t.Errorf("last msg.Role = %s, want User", last.Role)
	}
	for i, m := range msgs[1 : len(msgs)-1] {
		if m.Role != schema.User && m.Role != schema.Assistant {
			t.Errorf("history prefix msg[%d].Role = %s, want User or Assistant", i+1, m.Role)
		}
	}

	// No literal [CHUNK markers anywhere in the prefix history.
	for i, m := range msgs[1 : len(msgs)-1] {
		if strings.Contains(m.Content, "[CHUNK ") {
			t.Errorf("history prefix msg[%d] still contains '[CHUNK ': %q", i+1, m.Content)
		}
	}

	// User part must contain the [ref-N] blocks and the query, never [CHUNK.
	user := msgs[len(msgs)-1].Content
	if !strings.Contains(user, "[ref-") {
		t.Errorf("user part missing [ref- blocks: %q", user)
	}
	if strings.Contains(user, "[CHUNK ") {
		t.Errorf("user part contains '[CHUNK ' (via-pipeline must use [ref-N]): %q", user)
	}
	if !strings.Contains(user, query) {
		t.Errorf("user part missing original query %q: %q", query, user)
	}
}

// TestRunEntry_ViaPipelineDryRun exercises RunEntry with Rewriter and
// Classifier non-nil but DryRun=true. All external calls are stubbed,
// the classifier allows the query, and the shared tail populates metrics.
func TestRunEntry_ViaPipelineDryRun(t *testing.T) {
	row := SpikeRow{
		Case: eval.Case{
			ID:       "dry-pipeline-1",
			Category: "history_injection_followup",
			Query:    "请告诉我依据",
		},
		HistoryTurns: []historyTurn{
			{Role: "user", Content: "年假有几天？"},
			{Role: "assistant", Content: "年假有5天。"},
		},
	}
	row.HistoryAnchor.TurnIndex = 0
	row.HistoryAnchor.FakeChunkRef = ""

	deps := &runnerDeps{
		DryRun:    true,
		Rewriter:  &rewrite.QueryRewriter{},
		Classifier: &classifier.RefusalClassifier{},
	}
	cost := &costState{LimitUSD: 5.00}

	res, err := RunEntry(context.Background(), row, true, "short", deps, cost)
	if err != nil {
		t.Fatalf("RunEntry returned error: %v", err)
	}
	if res == nil {
		t.Fatal("RunEntry returned nil result")
	}
	if res.Refused {
		t.Errorf("Refused = true, want false (dry-run classifier stub allows)")
	}
	if res.RewrittenQuery != "" {
		// Dry-run stub returns query unchanged, so RewrittenQuery is not set.
		t.Errorf("RewrittenQuery = %q, want empty for unchanged query", res.RewrittenQuery)
	}
	for _, key := range []string{"faithfulness", "context_precision", "answer_relevancy", "context_recall"} {
		if _, ok := res.Metrics[key]; !ok {
			t.Errorf("Metrics missing %s: got %v", key, res.Metrics)
		}
	}
}

// evalCaseStub removed — tests use eval.Case directly.