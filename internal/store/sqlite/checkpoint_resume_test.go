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

// SC-5 real-interrupt round-trip (Phase 8 / Plan 08-04). Unlike the
// store-layer tests in checkpoint_test.go (raw Get/Set bytes), this file
// drives the FULL chain: a scripted stub ChatModel emits a tool call, the
// tool interrupts via the v0.9 tool-layer interrupt API, the adk Runner
// checkpoints the interrupt into the 08-02 SQLiteStore, then a simulated
// "process restart" (Close + reopen the SAME database file + a brand-new
// Agent and Runner) resumes from that checkpoint with an injected approval
// decision and runs to a scripted final answer.
//
// Hermetic: zero network, zero external services — the only I/O is the
// file-backed sqlite database inside t.TempDir().
//
// Event-chain source of truth (eino@v0.9.14 module cache, cross-checked
// against the official adk tests):
//   - adk/runner.go:310-341 — Runner converts an internal interrupt into
//     AgentEvent.Action.Interrupted and persists the checkpoint via the
//     configured CheckPointStore.
//   - adk/interrupt.go:219-242 — resume loads the checkpoint bytes back
//     through store.Get, so the interrupt state crosses the serialization
//     boundary (this is why the registration below must be the DOUBLE
//     registration, per RESEARCH Pitfall 1).
//   - components/tool/interrupt.go:44-77 — tool.Interrupt /
//     StatefulInterrupt are the ONLY sanctioned interrupt entry points for
//     tools (mirrors the Phase 11 write-tool shape; the 08-03 ban gate
//     forbids the legacy un-addressed APIs everywhere, tests included).
package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// approvalDecision is the custom state type that crosses the checkpoint
// serialization boundary: it is attached to the tool's StatefulInterrupt,
// gob-encoded inside the checkpoint payload, persisted to sqlite, and
// decoded back in the "restarted" process. Round-trip fidelity of this
// type IS one of the truths under test.
type approvalDecision struct {
	Approved bool
	Reason   string
}

// Registration uses schema.RegisterName[T](stableName) — the official
// entry point (schema/serialization.go:83-90) that performs BOTH halves of
// the registration (gob + the v0.9 InternalSerializer registry) in one
// call. A bare encoding/gob registration would only cover half the
// surface and detonate later at resume-decode time (Pitfall 1: 延迟引爆).
// The name is permanent once shipped — "skillforge_" prefix reserves the
// namespace for the product (Phase 11 will register its real approval
// state the same way).
func init() {
	schema.RegisterName[*approvalDecision]("skillforge_test_approval_decision")
}

// Constants for the scripted scenario. Exported nowhere; test-local.
const (
	approvalToolName = "request_write_approval"
	approvalQuery    = "please delete policy-42.md from the authority tier"
	checkPointID     = "test-cp-roundtrip"

	// pendingReason is carried by the ORIGINAL interrupt state (before any
	// human decision). The restarted tool must see exactly this value via
	// GetInterruptState — proof the interrupt-time state survived the
	// serialize → sqlite → deserialize round trip.
	pendingReason = "awaiting human decision"

	// resumeReason is the decision injected via ResumeWithParams Targets.
	// It must be observable BOTH in the tool's resumed execution AND in
	// the final answer text — proof the injected value reached the
	// execution flow, not just the framework plumbing.
	resumeReason   = "human approved via resume"
	finalAnswerPfx = "审批决策已执行:"
)

// ---------------------------------------------------------------------------
// Test double: interruptible write-approval tool
// ---------------------------------------------------------------------------

// writeApprovalTool mirrors the Phase 11 write-tool shape: a mutating
// action that MUST obtain a human approval decision before executing. On
// first pass it interrupts (state = the pending decision); once resumed
// it reads the injected decision via GetResumeContext and reports it as
// its output.
//
// First pass  : GetInterruptState → wasInterrupted=false → StatefulInterrupt.
// Resumed     : GetInterruptState → wasInterrupted=true (state restored
//
//	from the sqlite checkpoint), GetResumeContext → the
//	decision injected by ResumeWithParams.
//
// Not target  : re-interrupt (leaf components MUST preserve their state —
//
//	adk/runner.go:137-147 contract).
type writeApprovalTool struct {
	mu sync.Mutex

	firstPass         int               // count of interrupting invocations
	resumedDecision   *approvalDecision // decision injected via resume (nil until resumed)
	restoredState     *approvalDecision // state recovered via GetInterruptState on resume
	sawInterruptState bool              // wasInterrupted flag seen on the resumed pass
}

var _ tool.InvokableTool = (*writeApprovalTool)(nil)

func (w *writeApprovalTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: approvalToolName,
		Desc: "Requests human approval before performing a write action",
	}, nil
}

func (w *writeApprovalTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	wasInterrupted, hasState, state := tool.GetInterruptState[*approvalDecision](ctx)
	if !wasInterrupted {
		w.mu.Lock()
		w.firstPass++
		w.mu.Unlock()
		// Tool-layer interrupt carrying the custom state type (the state
		// is what crosses the serialization boundary into sqlite).
		return "", tool.StatefulInterrupt(ctx,
			"write action requires human approval",
			&approvalDecision{Approved: false, Reason: pendingReason},
		)
	}

	w.mu.Lock()
	w.sawInterruptState = true
	if hasState {
		w.restoredState = state
	}
	w.mu.Unlock()

	isTarget, hasData, data := tool.GetResumeContext[*approvalDecision](ctx)
	if !isTarget {
		// Leaf contract: not our turn → preserve state by re-interrupting.
		return "", tool.Interrupt(ctx, "not the resume target — re-interrupt")
	}
	if !hasData {
		return "resumed without data", nil
	}

	w.mu.Lock()
	w.resumedDecision = data
	w.mu.Unlock()
	return fmt.Sprintf("approved=%v; reason=%s", data.Approved, data.Reason), nil
}

// Accessors (mutex-guarded) keep the post-drain assertions race-detector
// clean even though the tool executes on runner goroutines.
func (w *writeApprovalTool) snapshot() (firstPass int, resumed, restored *approvalDecision, sawState bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.firstPass, w.resumedDecision, w.restoredState, w.sawInterruptState
}

// ---------------------------------------------------------------------------
// Test double: scripted ChatModel (multiTurnStubChatModel shape)
// ---------------------------------------------------------------------------

// scriptedApprovalChatModel follows the internal/agent multiTurnStubChatModel
// shape (var _ model.ToolCallingChatModel assertion, mutex-guarded call
// counter, Stream → not implemented, WithTools → self). The script keys on
// the CONVERSATION HISTORY rather than an instance-local turn counter:
//
//   - no tool message in history → emit the approval tool call (turn 1 of
//     process 1 — the interrupt trigger);
//   - a tool message present    → emit the final answer, echoing the LAST
//     tool result so the final text provably carries whatever the resumed
//     tool produced (the resume-injected decision becomes assertable).
//
// History-keying (not counter-keying) is REQUIRED by the restart design:
// process 2 gets a brand-new instance with a fresh counter but a RESTORED
// history, so a counter-keyed script would re-emit the tool call and loop.
type scriptedApprovalChatModel struct {
	mu    sync.Mutex
	calls int
}

var _ model.ToolCallingChatModel = (*scriptedApprovalChatModel)(nil)

// lastToolResult returns the content of the last schema.Tool message in
// the conversation, or "" when none exists.
func lastToolResult(msgs []*schema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == schema.Tool {
			return msgs[i].Content
		}
	}
	return ""
}

func (m *scriptedApprovalChatModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()

	if lastToolResult(msgs) == "" {
		// Turn 1: request the write approval → drives the tool interrupt.
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_approval_1",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      approvalToolName,
						Arguments: `{"action":"delete_doc","doc":"policy-42.md"}`,
					},
				},
			},
		}, nil
	}

	// Post-tool turn: final answer echoing the tool result — the injected
	// decision is now part of the assistant-visible execution flow.
	return &schema.Message{
		Role:    schema.Assistant,
		Content: finalAnswerPfx + lastToolResult(msgs),
	}, nil
}

func (m *scriptedApprovalChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("scriptedApprovalChatModel: Stream not implemented")
}

func (m *scriptedApprovalChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *scriptedApprovalChatModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// ---------------------------------------------------------------------------
// The round-trip test
// ---------------------------------------------------------------------------

// TestInterruptResumeRoundTrip — SC-5: one interrupt run, checkpointed to
// the real sqlite store, then resumed to completion by a fully separate
// "process" (new DB handle + new Agent + new Runner over the same file).
//
// Truths locked (plan must_haves):
//  1. adk Runner surfaces Action.Interrupted with the interrupt ID
//     capturable from InterruptContexts[0].ID.
//  2. After the interrupt, checkpoint bytes are durably persisted in
//     sqlite (store Get existed=true — NOT an in-memory artifact).
//  3. A "restarted" process (new SQLiteStore handle on the same DSN, new
//     ChatModelAgent, new Runner) resumes via ResumeWithParams(Targets)
//     and the run completes to the scripted final answer.
//  4. The custom state type registered via schema.RegisterName survives
//     the serialization round trip with semantics intact (the resumed
//     tool recovers the ORIGINAL pending state and receives the injected
//     decision).
func TestInterruptResumeRoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "checkpoint-resume.db")

	// ------------------------------------------------------------------
	// Process 1: query → tool interrupt → checkpoint persisted.
	// ------------------------------------------------------------------
	s1, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err, "process 1: sqlite.New should open the store")
	defer func() { _ = s1.Close() }() // idempotent; explicit Close below

	model1 := &scriptedApprovalChatModel{}
	tool1 := &writeApprovalTool{}
	agent1, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "write-approval-agent",
		Description: "agent whose write tool requires human approval",
		Instruction: "You are a write-gated assistant. Use the approval tool for any write action.",
		Model:       model1,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{tool1},
			},
		},
	})
	require.NoError(t, err, "process 1: NewChatModelAgent")

	runner1 := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent1,
		CheckPointStore: s1, // 08-02 SQLiteStore IS the adk CheckPointStore
	})

	iter := runner1.Query(ctx, approvalQuery, adk.WithCheckPointID(checkPointID))

	// Drain the iterator (drainFinalContent shape from
	// internal/server/chat.go:914-939) plus the interrupt branch.
	var interruptID string
	var sawInterrupt bool
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		require.NoError(t, ev.Err, "process 1: events must be error-free")
		if ev.Action != nil && ev.Action.Interrupted != nil {
			sawInterrupt = true
			require.NotEmpty(t, ev.Action.Interrupted.InterruptContexts,
				"the interrupt event must carry at least one InterruptContext")
			ic := ev.Action.Interrupted.InterruptContexts[0]
			interruptID = ic.ID
			require.NotEmpty(t, ic.Address,
				"InterruptContext must carry a non-empty address (indexing ic.Address[len-1] would panic otherwise)")
			assert.Equal(t, approvalToolName, ic.Address[len(ic.Address)-1].ID,
				"the root interrupt address must point at the approval tool")
		}
	}
	require.True(t, sawInterrupt, "the scripted tool call MUST surface Action.Interrupted")
	require.NotEmpty(t, interruptID, "interrupt ID must be captured from InterruptContexts[0].ID")

	// Truth 2: the checkpoint bytes are durably in sqlite BEFORE restart.
	persisted, existed, err := s1.Get(ctx, checkPointID)
	require.NoError(t, err, "store Get after interrupt must not error")
	require.True(t, existed, "checkpoint must be persisted to sqlite after the interrupt (existed=true)")
	require.NotEmpty(t, persisted, "persisted checkpoint payload must be non-empty")

	fp1, resumed1, _, _ := tool1.snapshot()
	assert.Equal(t, 1, fp1, "process 1: exactly one interrupting tool pass")
	assert.Nil(t, resumed1, "process 1: the tool never saw a resume decision")
	assert.Equal(t, 1, model1.callCount(), "process 1: one model turn (the tool call)")

	// ------------------------------------------------------------------
	// "Process restart": close the DB handle, reopen the SAME file, and
	// rebuild EVERY runtime object (model, tool, agent, runner). Instance
	// level restart — nothing from process 1 is reused except the file.
	// ------------------------------------------------------------------
	require.NoError(t, s1.Close(), "Close simulates the process exiting")

	s2, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err, "process 2: sqlite.New on the same DSN must reopen the store")
	defer func() { _ = s2.Close() }()

	model2 := &scriptedApprovalChatModel{} // fresh call counter
	tool2 := &writeApprovalTool{}          // fresh tool state
	agent2, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "write-approval-agent",
		Description: "agent whose write tool requires human approval",
		Instruction: "You are a write-gated assistant. Use the approval tool for any write action.",
		Model:       model2,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{tool2},
			},
		},
	})
	require.NoError(t, err, "process 2: NewChatModelAgent")

	runner2 := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent2,
		CheckPointStore: s2,
	})

	iter2, err := runner2.ResumeWithParams(ctx, checkPointID, &adk.ResumeParams{
		Targets: map[string]any{
			interruptID: &approvalDecision{Approved: true, Reason: resumeReason},
		},
	})
	require.NoError(t, err, "ResumeWithParams must load the checkpoint from the reopened store")

	// Drain the resumed run to completion — no further interrupt allowed.
	var finalContent string
	for {
		ev, ok := iter2.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		require.NoError(t, ev.Err, "process 2: resumed run must complete without error")
		require.Nil(t, ev.Action,
			"the resumed run must NOT re-interrupt when the tool is the resume target")
		if ev.Output != nil && ev.Output.MessageOutput != nil {
			msg, mErr := ev.Output.MessageOutput.GetMessage()
			require.NoError(t, mErr)
			if msg != nil && msg.Content != "" {
				finalContent += msg.Content
			}
		}
	}

	// Truth 3 (completion): the final answer is the scripted echo and it
	// carries the injected decision — the value reached the execution flow.
	assert.Contains(t, finalContent, finalAnswerPfx,
		"final answer must come from the scripted post-tool turn")
	assert.Contains(t, finalContent, "approved=true",
		"the injected Approved=true must be visible in the final answer")
	assert.Contains(t, finalContent, resumeReason,
		"the injected Reason must be visible in the final answer (SC-5 batch A)")

	// Truth 4 (registration + state semantics): the restarted tool saw the
	// ORIGINAL pending state (serialized into sqlite at interrupt time and
	// decoded back) AND the injected resume decision.
	fp2, resumed2, restored2, sawState2 := tool2.snapshot()
	assert.Equal(t, 0, fp2, "process 2: the tool must NOT run a fresh interrupting pass")
	assert.True(t, sawState2, "process 2: GetInterruptState must report wasInterrupted=true")
	require.NotNil(t, restored2, "the interrupt-time state must round-trip through the sqlite checkpoint")
	assert.False(t, restored2.Approved,
		"restored state is the ORIGINAL pending decision (Approved=false at interrupt time)")
	assert.Equal(t, pendingReason, restored2.Reason,
		"restored state Reason must be byte-equal to the interrupt-time value")
	require.NotNil(t, resumed2, "the resumed tool must receive the injected decision")
	assert.True(t, resumed2.Approved)
	assert.Equal(t, resumeReason, resumed2.Reason)

	assert.GreaterOrEqual(t, model2.callCount(), 1,
		"process 2: the fresh model instance must produce the final answer")
}
