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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store/sqlite"

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

// scriptedApprovalModel is a ToolCallingChatModel scripted on the
// conversation history (mirrors internal/server/approval_api_test.go's
// scriptedApprovalModel):
//
//   - no tool message in history → emit a memory_facts_write tool call
//     (drives the HITL Tool's Pass-1 interrupt);
//   - a tool message present     → emit the final answer.
//
// Used by TestCancelPropagationMidToolRunner's ResumePath sub-test to
// drive a real interrupt → resume cycle hermetically.
type scriptedApprovalModel struct{}

var _ model.ToolCallingChatModel = (*scriptedApprovalModel)(nil)

const (
	cancelSmokeToolCallID = "tc-cancel-smoke-000001"
	cancelSmokeToolArgs   = `{"key":"employee_count","value":"150"}`
)

func hasToolResultInMsgs(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if m != nil && m.Role == schema.Tool {
			return true
		}
	}
	return false
}

func (m *scriptedApprovalModel) scripted(msgs []*schema.Message) *schema.Message {
	if !hasToolResultInMsgs(msgs) {
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   cancelSmokeToolCallID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      hitl.MemoryFactsToolName,
					Arguments: cancelSmokeToolArgs,
				},
			}},
		}
	}
	return &schema.Message{Role: schema.Assistant, Content: "final answer after approval"}
}

func (m *scriptedApprovalModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.scripted(msgs), nil
}

func (m *scriptedApprovalModel) Stream(ctx context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *scriptedApprovalModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// blockingAfterToolModel behaves like scriptedApprovalModel until the
// post-tool turn, where Stream blocks until ctx is cancelled — the
// ResumePath sub-test uses it to measure cancel propagation DURING
// the resume (the model is mid-Stream when cancel fires).
type blockingAfterToolModel struct {
	inner scriptedApprovalModel
}

var _ model.ToolCallingChatModel = (*blockingAfterToolModel)(nil)

func (m *blockingAfterToolModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if hasToolResultInMsgs(msgs) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return m.inner.Generate(ctx, msgs, opts...)
}

func (m *blockingAfterToolModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if hasToolResultInMsgs(msgs) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	msg, err := m.inner.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *blockingAfterToolModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// cancelMeasureBudget is the D11-A4 + SC-8 + AGNT-04 tolerance:
// cancel-to-SSE-close must complete within this budget. The
// CombinedTolerance sub-test asserts max(RunPath, ResumePath) <=
// cancelMeasureBudget across 50 iterations each.
const cancelMeasureBudget = 200 * time.Millisecond

// cancelMeasureIterations is the per-sub-test iteration count. The
// 200ms budget applies to the MAX observed value across iterations,
// not the mean — a single p99 outlier above 200ms is a lock
// violation.
const cancelMeasureIterations = 50

// TestCancelPropagationMidToolRunner — Plan 11-08 Task 5 / D11-A4 +
// SC-8 / AGNT-04 <200ms lock, restructured into 3 sub-tests:
//
//   - RunPath: BuildRunner + runner.Run + cancel mid-run; assert
//     drain returns within 200ms.
//   - ResumePath: BuildRunner + Run (interrupt) → stamp checkpoint →
//     ResumeWithParams (blocking post-tool model) + cancel mid-resume;
//     assert drain returns within 200ms.
//   - CombinedTolerance: run BOTH paths in sequence and assert
//     max(RunPathElapsed, ResumePathElapsed) <= 200ms. This is the
//     final acceptance-gate measurement.
//
// Each sub-test runs 50 iterations and reports max / p50 / p99. The
// 200ms budget is on the MAX observed value (a single outlier is a
// lock violation). The log line `[CANCEL-MEASURE]` is grep-friendly
// for scripts/verify-phase-11.sh Stage 4.
//
// Plan note: the plan specified agent_test.go; the test already lives
// here (runner_setup_test.go) alongside the helpers it consumes
// (hangChatModel + the Runner construction path). Both files are in
// package agent_test so the test NAME + sub-test names match the plan
// exactly.
func TestCancelPropagationMidToolRunner(t *testing.T) {
	t.Run("RunPath", testCancelMeasureRunPath)
	t.Run("ResumePath", testCancelMeasureResumePath)
	t.Run("CombinedTolerance", testCancelMeasureCombined)
}

// testCancelMeasureRunPath measures cancel propagation on the
// runner.Run path: the chat model blocks on ctx, cancel fires, drain
// must return within 200ms.
func testCancelMeasureRunPath(t *testing.T) {
	elapsed := measureCancelRunPath(t, cancelMeasureIterations)
	reportCancelMeasure(t, "run-path", elapsed)
	maxV := maxDuration(elapsed)
	require.LessOrEqual(t, maxV, cancelMeasureBudget,
		"D11-A4: RunPath cancel did not propagate within %v (max=%v)", cancelMeasureBudget, maxV)
}

// testCancelMeasureResumePath measures cancel propagation on the
// runner.ResumeWithParams path: a real interrupt seeds the checkpoint,
// the resume uses a blocking post-tool model, cancel fires mid-resume,
// drain must return within 200ms.
func testCancelMeasureResumePath(t *testing.T) {
	elapsed := measureCancelResumePath(t, cancelMeasureIterations)
	reportCancelMeasure(t, "resume-path", elapsed)
	maxV := maxDuration(elapsed)
	require.LessOrEqual(t, maxV, cancelMeasureBudget,
		"D11-A4: ResumePath cancel did not propagate within %v (max=%v)", cancelMeasureBudget, maxV)
}

// testCancelMeasureCombined is the acceptance-gate measurement: it
// runs BOTH paths in sequence and asserts max(Run, Resume) <= 200ms.
// This is the final invariant scripts/verify-phase-11.sh Stage 4
// greps for.
func testCancelMeasureCombined(t *testing.T) {
	runElapsed := measureCancelRunPath(t, cancelMeasureIterations)
	resumeElapsed := measureCancelResumePath(t, cancelMeasureIterations)
	reportCancelMeasure(t, "combined-run", runElapsed)
	reportCancelMeasure(t, "combined-resume", resumeElapsed)
	maxRun := maxDuration(runElapsed)
	maxResume := maxDuration(resumeElapsed)
	combined := maxRun
	if maxResume > combined {
		combined = maxResume
	}
	require.LessOrEqual(t, combined, cancelMeasureBudget,
		"D11-A4 CombinedTolerance: max(Run=%v, Resume=%v) = %v exceeds budget %v",
		maxRun, maxResume, combined, cancelMeasureBudget)
}

// measureCancelRunPath runs n iterations of the RunPath measurement
// and returns the per-iteration elapsed times (cancel-to-drain-return).
func measureCancelRunPath(t *testing.T, n int) []time.Duration {
	t.Helper()
	elapsed := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		cm := &hangChatModel{}
		emb := newStubEmbedder()
		ret := &stubRetriever{}
		manifest := &skill.Manifest{}
		manifest.SetResolvedTiersForTest([]string{"权威"})

		ctx, cancel := context.WithCancel(context.Background())
		runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
			Manifest:  manifest,
			Embedder:  emb,
			Retriever: ret,
			ChatModel: cm,
			SessionID: fmt.Sprintf("test-sess-cancel-run-%d", i),
		})
		require.NoError(t, err, "iteration %d: BuildRunner", i)

		iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("any query")})
		require.NotNil(t, iter, "iteration %d: runner.Run", i)

		resCh := make(chan error, 1)
		go func() {
			resCh <- agent.DrainRunnerEvents(ctx, iter, nil)
		}()

		// Warm-up: let the request reach chatModel.Generate.
		time.Sleep(20 * time.Millisecond)
		start := time.Now()
		cancel()
		select {
		case <-resCh:
			elapsed = append(elapsed, time.Since(start))
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatalf("iteration %d: drain did not return within 2s of cancel", i)
		}
		cancel()
	}
	return elapsed
}

// measureCancelResumePath runs n iterations of the ResumePath
// measurement and returns the per-iteration elapsed times. Each
// iteration is independent (fresh in-memory store, fresh Runner).
func measureCancelResumePath(t *testing.T, n int) []time.Duration {
	t.Helper()
	elapsed := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		elapsed = append(elapsed, measureOneResumeCancel(t, i))
	}
	return elapsed
}

// measureOneResumeCancel drives one full interrupt → resume → cancel
// cycle and returns the cancel-to-drain-return elapsed time.
func measureOneResumeCancel(t *testing.T, iter int) time.Duration {
	t.Helper()
	ctx := context.Background()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err, "iteration %d: sqlite.New", iter)
	defer func() { _ = s.Close() }()

	sessionID := fmt.Sprintf("sess-cancel-resume-%d", iter)
	manifest := &skill.Manifest{
		Name:        "compliance",
		Description: "cancel-resume smoke",
		Agent:       "ChatModelAgent",
		Writes:      []string{"memory:facts"},
	}
	manifest.SetResolvedTiersForTest([]string{"权威"})
	snap := hitl.NewSkillSnapshot(manifest.Name, "", agent.RenderSystemPrompt(manifest))
	cpID := agent.RunnerCheckPointID(sessionID)

	// Phase 1: interrupt + checkpoint.
	interruptID := driveInterruptPhase(t, ctx, s, manifest, sessionID, iter)
	stampCheckpointSnapshot(t, ctx, s, cpID, snap)

	// Phase 2: resume with the blocking post-tool model; cancel mid-resume.
	resumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner, err := agent.BuildRunner(resumeCtx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        newStubEmbedder(),
		Retriever:       &stubRetriever{},
		ChatModel:       &blockingAfterToolModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
	})
	require.NoError(t, err, "iteration %d: BuildRunner (resume)", iter)

	resumeIter, err := runner.ResumeWithParams(resumeCtx, cpID, &adk.ResumeParams{
		Targets: map[string]any{interruptID: &hitl.ApprovalDecision{
			Decision:   "approve",
			Target:     "memory:facts",
			ApprovalID: cancelSmokeToolCallID,
		}},
	})
	require.NoError(t, err, "iteration %d: ResumeWithParams", iter)

	resCh := make(chan error, 1)
	go func() {
		resCh <- agent.DrainRunnerEvents(resumeCtx, resumeIter, nil)
	}()

	// Warm-up: let the resume reach the blocking model call.
	time.Sleep(20 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case <-resCh:
		return time.Since(start)
	case <-time.After(2 * time.Second):
		t.Fatalf("iteration %d: resume drain did not return within 2s of cancel", iter)
		return 0
	}
}

// driveInterruptPhase runs Pass 1 (interrupt) and returns the
// interrupt UUID. Helper for measureOneResumeCancel.
func driveInterruptPhase(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, manifest *skill.Manifest, sessionID string, iter int) string {
	t.Helper()
	runner, err := agent.BuildRunner(ctx, agent.RunnerBuildConfig{
		Manifest:        manifest,
		Embedder:        newStubEmbedder(),
		Retriever:       &stubRetriever{},
		ChatModel:       &scriptedApprovalModel{},
		SessionID:       sessionID,
		SkillID:         manifest.Name,
		CheckPointStore: s,
	})
	require.NoError(t, err, "iteration %d: BuildRunner (interrupt)", iter)

	runIter := runner.Run(ctx, []*schema.Message{schema.UserMessage("persist the employee count")},
		adk.WithCheckPointID(agent.RunnerCheckPointID(sessionID)))
	var interruptID string
	var sawInterrupt bool
	for {
		ev, ok := runIter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		require.NoError(t, ev.Err, "iteration %d: phase 1 events", iter)
		if ev.Action != nil && ev.Action.Interrupted != nil {
			sawInterrupt = true
			require.NotEmpty(t, ev.Action.Interrupted.InterruptContexts)
			interruptID = ev.Action.Interrupted.InterruptContexts[0].ID
		}
	}
	require.True(t, sawInterrupt, "iteration %d: scripted tool call must interrupt", iter)
	require.NotEmpty(t, interruptID, "iteration %d: interrupt UUID", iter)
	return interruptID
}

// stampCheckpointSnapshot rewrites the checkpoint row with the D11-C2
// snapshot columns (payload bytes carried over verbatim). Helper for
// measureOneResumeCancel.
func stampCheckpointSnapshot(t *testing.T, ctx context.Context, s *sqlite.SQLiteStore, cpID string, snap hitl.SkillSnapshot) {
	t.Helper()
	payload, existed, err := s.Get(ctx, cpID)
	require.NoError(t, err)
	require.True(t, existed, "checkpoint must exist before stamping")
	require.NoError(t, s.SetCheckpointWithSnapshot(ctx, cpID, payload, snap.Version, snap.PromptHash))
}

// maxDuration returns the maximum value in ds (zero value on empty).
func maxDuration(ds []time.Duration) time.Duration {
	var m time.Duration
	for _, d := range ds {
		if d > m {
			m = d
		}
	}
	return m
}

// percentileDuration returns the p-th percentile (0-100) of ds using
// the nearest-rank method. Caller must pass a non-empty slice.
func percentileDuration(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := (p*len(sorted) + 99) / 100 // nearest-rank, 1-based
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// reportCancelMeasure emits the [CANCEL-MEASURE] log line that
// scripts/verify-phase-11.sh Stage 4 greps for.
func reportCancelMeasure(t *testing.T, label string, elapsed []time.Duration) {
	t.Helper()
	t.Logf("[CANCEL-MEASURE] %s: max=%v p50=%v p99=%v (budget=%v, iterations=%d)",
		label, maxDuration(elapsed), percentileDuration(elapsed, 50), percentileDuration(elapsed, 99),
		cancelMeasureBudget, len(elapsed))
}

// TestNoLegacyInterruptAPI scans the Phase 11 directories for legacy or
// misplaced interrupt API usages.
//
// tool.StatefulInterrupt() is allowed ONLY in the real HITL Tool files
// (internal/agent/hitl/*), which ship in 11-02 and use the current eino
// v0.9.14 API. The audit_writer.go file in particular must NOT contain
// StatefulInterrupt(); its job is the sync INSERT before the interrupt
// is reached (D11-C3).
//
// Deprecated Runner.Query( / Runner.Resume( forms are banned outright:
// the Runner path uses runner.Run() and (when needed) the v0.9.14
// ResumeWithParams flow.
func TestNoLegacyInterruptAPI(t *testing.T) {
	roots := []string{
		"internal/agent",
		"internal/agent/callbacks",
		"internal/agent/tools",
		"internal/agent/hitl",
	}

	var hits []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(b)
			if strings.Contains(s, "tool.StatefulInterrupt(") {
				// 11-02's HITL Tools legitimately use the current
				// StatefulInterrupt API; audit_writer.go does not.
				if !(strings.Contains(path, "hitl_memory_facts.go") ||
					strings.Contains(path, "hitl_review_report.go") ||
					strings.Contains(path, "hitl_plan_approve.go")) {
					hits = append(hits, path+": tool.StatefulInterrupt(")
				}
			}
			if strings.Contains(s, "Runner.Query(") || strings.Contains(s, "Runner.Resume(") {
				hits = append(hits, path+": legacy Runner.Query/Resume usage")
			}
			return nil
		})
	}

	if len(hits) > 0 {
		t.Fatalf("legacy or misplaced interrupt API found:\n%s", strings.Join(hits, "\n"))
	}
}
