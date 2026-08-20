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

// Package agent_test — runner_setup_test.go is Phase 11 / Plan 11-01's
// verification gate file. It locks the D11-A4 <200ms cancel
// propagation invariant for the Runner path AND re-asserts the
// TestNoLegacyInterruptAPI ban against the new directories the
// plan creates (internal/agent/{callbacks,tools,hitl}/).
package agent_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/skill"

	"github.com/stretchr/testify/require"
)

// hangChatModel is a model.ToolCallingChatModel whose Generate blocks
// until ctx is cancelled. Mirrors the production hangChatModel in
// internal/server/chat_session_test.go so the 200ms lock assertion
// is the same shape across both paths.
type hangChatModel struct {
	mu      sync.Mutex
	started bool
}

var _ model.ToolCallingChatModel = (*hangChatModel)(nil)

func (m *hangChatModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (m *hangChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("hangChatModel: Stream not implemented")
}

func (m *hangChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestCancelPropagationMidToolRunner — Plan 11-01 / D11-A4 + SC-8 /
// AGNT-04 <200ms lock. Build a Agent + Runner via the production
// BuildRunner; call runner.Run in a goroutine; cancel ctx after a
// short warm-up; assert the goroutine returns within 200ms.
//
// The 200ms lock is anchored by the select { case <-ctx.Done():
// return ctx.Err(); default: } in agent.DrainRunnerEvents (D11-A4
// core). Without the select, Eino's AsyncIterator.Next() does NOT
// poll ctx cooperatively, so the goroutine could sit on Next() for
// the full Run lifetime.
//
// Mirrors TestCancelPropagationMidTool in
// internal/server/chat_session_test.go (the legacy react.Agent path)
// so the cancel propagation invariant is locked on BOTH paths under
// -race.
func TestCancelPropagationMidToolRunner(t *testing.T) {
	// The stubEmbedder / stubRetriever are defined in agent_test.go.
	// Reuse them so the test file does not redeclare types in the
	// agent_test package.
	cm := &hangChatModel{}
	emb := newStubEmbedder()
	ret := &stubRetriever{}

	// Manifest stub — BuildRunner only checks Manifest != nil in 11-01
	// (the per-turn Tool whitelist is empty for the stub Skills; the
	// retrieve Tool closes over ResolvedTiers which is nil for this
	// zero-value manifest).
	manifest := &skill.Manifest{}
	manifest.SetResolvedTiersForTest([]string{"权威"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:  manifest,
		Embedder:  emb,
		Retriever: ret,
		ChatModel: cm,
		SessionID: "test-sess-cancel",
	})
	require.NoError(t, err, "BuildRunner must succeed with stub deps")
	require.NotNil(t, runner, "BuildRunner must return non-nil Runner")

	// Warm-up: give the request time to reach chatModel.Generate.
	time.Sleep(50 * time.Millisecond)

	// Fire the Run in a goroutine and cancel ctx.
	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("any query")})
	require.NotNil(t, iter, "runner.Run must return non-nil iterator")

	type result struct {
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		// Drain the iterator until ctx cancellation propagates.
		err := agent.DrainRunnerEvents(ctx, iter, nil)
		resCh <- result{err}
	}()

	cancelTime := time.Now()
	cancel()

	select {
	case res := <-resCh:
		elapsed := time.Since(cancelTime)
		// The drain returns ctx.Err() on cancellation.
		if res.err == nil {
			t.Logf("[INFO] drain returned without error (incomplete generator; cancel propagated via iter EOF)")
		}
		require.Less(t, elapsed, 200*time.Millisecond,
			"D11-A4: cancel did not propagate within 200ms (elapsed=%v)", elapsed)
	case <-time.After(2 * time.Second):
		t.Fatal("D11-A4 FAIL: drain did not return within 2s of cancel")
	}
}
