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

// Package agent test — Plan 12-02 Task 4 (TDD) —
// writeTodosAuditedTool + BuildWriteTodosTool + DeepAgentTools.
// Hermetic tests cover the 3-effect contract documented in
// deep_audit.go:
//
//	(1) AUDIT  — PutPlanTodo INSERT (fail-closed)
//	(2) SSE    — EmitDataPlanTrace chunk (fail-closed)
//	(3) VALUE  — adk.AddSessionValue (best-effort, void)
package agent

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/wire"
)

// captureWriteTodosEmitter is the test seam: it captures the payload
// passed to EmitDataPlanTrace so the test can assert (a) the call
// happened, (b) the payload carries the expected RunID + Todos + Ts.
// The fail field forces EmitDataPlanTrace to return a non-nil error
// so the SSE-fail-closed tests can exercise the failure path.
type captureWriteTodosEmitter struct {
	mu        sync.Mutex
	payloads  []wire.PlanTracePayload
	fail      error
	callCount int
}

func (c *captureWriteTodosEmitter) EmitDataPlanTrace(_ context.Context, p wire.PlanTracePayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callCount++
	if c.fail != nil {
		return c.fail
	}
	c.payloads = append(c.payloads, p)
	return nil
}

// payloadsSnapshot returns a copy of the captured payloads so the
// test can assert without holding the mutex during the assertion.
func (c *captureWriteTodosEmitter) payloadsSnapshot() []wire.PlanTracePayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]wire.PlanTracePayload, len(c.payloads))
	copy(out, c.payloads)
	return out
}

func (c *captureWriteTodosEmitter) callCountSnapshot() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callCount
}

// newTestStore creates a hermetic :memory: SQLiteStore for the test.
// The 7 hermetic tests all share this pattern.
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestWriteTodosAuditedToolInvokableRun — Task 4: happy path. Write 3
// todos with mixed statuses; assert (a) 3 plan_todos rows inserted
// (QueryPlanTodos returns 3 rows with op='upsert'), (b) the emitter
// was called once with the payload carrying all 3 todos + the same
// RunID, (c) the Tool returns a success string with the count.
func TestWriteTodosAuditedToolInvokableRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	t1 := BuildWriteTodosTool(store, "sess-deep-1", emitter)

	args := `{"todos":[
		{"id":"t1","content":"retrieve policy","status":"pending"},
		{"id":"t2","content":"draft answer","activeForm":"drafting","status":"in_progress"},
		{"id":"t3","content":"verify","status":"pending"}
	]}`
	out, err := t1.InvokableRun(ctx, args)
	require.NoError(t, err)
	assert.Contains(t, out, "Updated todo list with 3 items",
		"Tool must return the canonical success string with the count")

	// (a) 3 plan_todos rows inserted with op='upsert'.
	rows, err := store.QueryPlanTodos(ctx, "sess-deep-1")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, r := range rows {
		assert.Equal(t, sqlite.PlanTodoOpUpsert, r.Op,
			"every audit row must carry op='upsert' on a write_todos call")
	}
	assert.Equal(t, "t1", rows[0].TodoID)
	assert.Equal(t, "retrieve policy", rows[0].Content)
	assert.Equal(t, "pending", rows[0].Status)
	assert.Equal(t, "t2", rows[1].TodoID)
	assert.Equal(t, "drafting", rows[1].ActiveForm,
		"activeForm must round-trip into the audit row (FE renders the active indicator)")
	assert.Equal(t, "in_progress", rows[1].Status)
	assert.Equal(t, "t3", rows[2].TodoID)

	// (b) emitter called once with the full payload.
	assert.Equal(t, 1, emitter.callCountSnapshot(),
		"emitter.EmitDataPlanTrace must be called exactly once per write_todos call")
	payloads := emitter.payloadsSnapshot()
	require.Len(t, payloads, 1)
	assert.Equal(t, "sess-deep-1", payloads[0].RunID,
		"emitter payload RunID must match the per-session identity")
	require.Len(t, payloads[0].Todos, 3)
	assert.Equal(t, "t1", payloads[0].Todos[0].ID)
	assert.Equal(t, "drafting", payloads[0].Todos[1].ActiveForm)
	assert.Equal(t, "t3", payloads[0].Todos[2].ID)
	assert.Greater(t, payloads[0].Ts, int64(0),
		"emitter payload Ts must be a non-zero epoch-seconds timestamp")
}

// TestWriteTodosAuditedToolAuditFailClosed — Task 4: store closed
// before the Tool call → InvokableRun returns an error containing
// "audit INSERT fail-closed" (P12-L4 — fail-closed posture, NO retry,
// NO swallow). The SSE chunk is NOT emitted in the failure case
// (audit comes BEFORE SSE in the 3-effect order).
func TestWriteTodosAuditedToolAuditFailClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := filepath.Join(t.TempDir(), "audit-fail-closed.db")
	store, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	require.NoError(t, store.Close(), "close the store to force PutPlanTodo → sql.ErrConnDone")

	emitter := &captureWriteTodosEmitter{}
	tool := BuildWriteTodosTool(store, "sess-audit-fail", emitter)

	args := `{"todos":[{"id":"t1","content":"x","status":"pending"}]}`
	out, err := tool.InvokableRun(ctx, args)
	require.Error(t, err, "audit INSERT failure must surface as a Tool error (P12-L4 fail-closed)")
	assert.Contains(t, err.Error(), "audit INSERT fail-closed",
		"error must carry the 'audit INSERT fail-closed' marker for log triage")
	assert.Empty(t, out, "no success string on failure")
	assert.Equal(t, 0, emitter.callCountSnapshot(),
		"SSE emit must NOT be attempted when audit INSERT fails (audit is BEFORE SSE in 3-effect order)")
}

// TestWriteTodosAuditedToolSSEFailClosed — Task 4: emitter returns
// error → InvokableRun returns error containing "SSE emit
// fail-closed". The audit rows ARE written (durable record survives)
// but the Tool surfaces the SSE failure to the LLM run.
func TestWriteTodosAuditedToolSSEFailClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{fail: errors.New("SSE channel closed")}
	tool := BuildWriteTodosTool(store, "sess-sse-fail", emitter)

	args := `{"todos":[{"id":"t1","content":"x","status":"pending"}]}`
	out, err := tool.InvokableRun(ctx, args)
	require.Error(t, err, "SSE emit failure must surface as a Tool error (P12-L4 fail-closed)")
	assert.Contains(t, err.Error(), "SSE emit fail-closed",
		"error must carry the 'SSE emit fail-closed' marker for log triage")
	assert.Empty(t, out, "no success string on SSE failure")

	// Audit row IS written — durable substrate survives the SSE
	// failure so the next write_todos call from the LLM can fix
	// the SSE gap with a fresh audit row.
	rows, err := store.QueryPlanTodos(ctx, "sess-sse-fail")
	require.NoError(t, err)
	require.Len(t, rows, 1,
		"audit row MUST persist even when SSE emit fails (durable substrate for replay)")
	assert.Equal(t, sqlite.PlanTodoOpUpsert, rows[0].Op)
}

// TestWriteTodosAuditedToolEmptyTodos — Task 4: write 0 todos →
// error "at least one todo required". The LLM must always emit at
// least one todo; the Tool rejects empty lists before any side
// effects fire.
func TestWriteTodosAuditedToolEmptyTodos(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	tool := BuildWriteTodosTool(store, "sess-empty", emitter)

	out, err := tool.InvokableRun(ctx, `{"todos":[]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one todo required",
		"empty todo list must be rejected with a clear message")
	assert.Empty(t, out, "no success string on empty todo list")

	// No audit rows + no SSE emit on the empty-list rejection.
	rows, err := store.QueryPlanTodos(ctx, "sess-empty")
	require.NoError(t, err)
	assert.Empty(t, rows, "no audit row on empty todo list (rejection is pre-side-effect)")
	assert.Equal(t, 0, emitter.callCountSnapshot(),
		"no SSE emit on empty todo list")
}

// TestWriteTodosAuditedToolInvalidArgs — Task 4: argsInJSON="not-json"
// → error "invalid args". The Tool rejects malformed JSON before
// any side effects fire.
func TestWriteTodosAuditedToolInvalidArgs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	tool := BuildWriteTodosTool(store, "sess-invalid", emitter)

	out, err := tool.InvokableRun(ctx, "not-json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid args",
		"invalid JSON must be rejected with the 'invalid args' marker")
	assert.Empty(t, out, "no success string on invalid args")

	rows, err := store.QueryPlanTodos(ctx, "sess-invalid")
	require.NoError(t, err)
	assert.Empty(t, rows, "no audit row on invalid args (rejection is pre-side-effect)")
	assert.Equal(t, 0, emitter.callCountSnapshot(),
		"no SSE emit on invalid args")
}

// TestWriteTodosAuditedToolInvalidStatus — Task 4: a todo with an
// invalid status enum value must be rejected. The Tool enforces
// pending|in_progress|completed at parse time so the LLM cannot
// drift from the canonical schema (mirrors eino's deep.TODO jsonschema
// enum: jsonschema:"enum=pending,enum=in_progress,enum=completed").
func TestWriteTodosAuditedToolInvalidStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	tool := BuildWriteTodosTool(store, "sess-bad-status", emitter)

	cases := []struct {
		name string
		args string
	}{
		{"unknown status", `{"todos":[{"id":"t1","content":"x","status":"done"}]}`},
		{"empty status", `{"todos":[{"id":"t1","content":"x","status":""}]}`},
		{"missing id", `{"todos":[{"content":"x","status":"pending"}]}`},
		{"empty content", `{"todos":[{"id":"t1","status":"pending"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.InvokableRun(ctx, tc.args)
			require.Error(t, err, "validation must reject malformed todo: %s", tc.name)
		})
	}

	// No audit rows + no SSE emit on any validation failure.
	rows, err := store.QueryPlanTodos(ctx, "sess-bad-status")
	require.NoError(t, err)
	assert.Empty(t, rows, "no audit row on validation failure (rejection is pre-side-effect)")
	assert.Equal(t, 0, emitter.callCountSnapshot())
}

// TestBuildWriteTodosToolRequiresAllParams — Task 4: nil store / nil
// emitter (via passing nil) → panic (factory-time invariant). The
// Tool is mandatory for the DeepAgent archetype; the runtime factory
// wires it via DeepAgentTools and a nil parameter is a wiring bug,
// not a runtime error.
func TestBuildWriteTodosToolRequiresAllParams(t *testing.T) {
	store := newTestStore(t)

	t.Run("nil store panics", func(t *testing.T) {
		assert.Panics(t, func() {
			BuildWriteTodosTool(nil, "sess", &captureWriteTodosEmitter{})
		}, "BuildWriteTodosTool(nil, ...) must panic (factory-time invariant)")
	})

	t.Run("empty runID panics", func(t *testing.T) {
		assert.Panics(t, func() {
			BuildWriteTodosTool(store, "", &captureWriteTodosEmitter{})
		}, "BuildWriteTodosTool(store, '', ...) must panic (per-session identity required)")
	})

	t.Run("nil emitter falls back to noop", func(t *testing.T) {
		// nil emitter is a NON-fatal fall-back to noopWriteTodosEmitter
		// so unit tests + the no-SSE path don't need a wiring
		// ceremony. Production wiring (12-06) passes a real emitter.
		assert.NotPanics(t, func() {
			tool := BuildWriteTodosTool(store, "sess-noop", nil)
			assert.NotNil(t, tool)
		}, "BuildWriteTodosTool(store, runID, nil) must NOT panic (falls back to noop emitter)")
	})
}

// TestDeepAgentToolsReturnsWriteTodosOnly — Task 4: DeepAgentTools
// returns a slice with exactly the write_todos Tool (the chat
// archetype's retrieve Tool is added by the runtime dispatcher in
// internal/agent/runner_setup.go, NOT by this factory). The slice
// is in the order the LLM should see the Tools in its schema; 12-03
// appends plan_approve at index 1.
func TestDeepAgentToolsReturnsWriteTodosOnly(t *testing.T) {
	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	tools := DeepAgentTools(store, "sess-deep-factory", emitter)
	require.Len(t, tools, 1, "DeepAgentTools must return exactly 1 Tool in Plan 12-02 (write_todos only)")

	// Verify the Tool is the audited write_todos by checking Info().
	info, err := tools[0].Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, WriteTodosToolName, info.Name,
		"DeepAgentTools must return the write_todos Tool (name matches eino auto-injected default)")
	assert.NotNil(t, info.ParamsOneOf,
		"write_todos Tool must have a ParamsOneOf schema so the LLM sees the canonical fields")

	// The slice must satisfy tool.InvokableTool (compile-time check
	// + runtime smoke test).
	var _ tool.InvokableTool = tools[0]
}

// TestNoLegacyInterruptAPI_DeepAudit — Task 4: invariant from D8-04.
// internal/agent/deep_audit.go is a NEW Tool that does NOT call
// tool.StatefulInterrupt (write_todos is NOT a HITL Tool). The
// canonical check is grep-based — TestNoLegacyInterruptAPI in
// integration/contract scans the source files for the pattern.
// This local test pins the package-level invariant for the new
// file (defense-in-depth; the source-grep test is the canonical
// gate).
func TestNoLegacyInterruptAPI_DeepAudit(t *testing.T) {
	// The deep_audit.go file's InvokableRun MUST NOT call
	// tool.StatefulInterrupt. We assert this via the runtime
	// behavior: the Tool's InvokableRun must complete normally on
	// a valid call (a HITL Tool would have returned a non-nil
	// resume-required marker instead of a success string).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	emitter := &captureWriteTodosEmitter{}
	tool := BuildWriteTodosTool(store, "sess-no-interrupt", emitter)

	out, err := tool.InvokableRun(ctx, `{"todos":[{"id":"t1","content":"x","status":"pending"}]}`)
	require.NoError(t, err, "write_todos Tool must complete normally (NOT a HITL Tool — no StatefulInterrupt)")
	assert.Contains(t, out, "Updated todo list with 1 item",
		"Tool must return the success string (not a resume-required marker)")
}

// TestWriteTodosAuditedToolNoopEmitterFallback — Task 4: when the
// caller passes nil as the emitter, BuildWriteTodosTool falls back
// to noopWriteTodosEmitter (the production runtime passes a real
// Hertz-backed emitter; the noop fallback is for tests + the no-SSE
// path). The Tool must complete successfully with no SSE side effect
// but the audit row is still written.
func TestWriteTodosAuditedToolNoopEmitterFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := newTestStore(t)
	tool := BuildWriteTodosTool(store, "sess-noop-emit", nil)

	out, err := tool.InvokableRun(ctx, `{"todos":[{"id":"t1","content":"x","status":"pending"}]}`)
	require.NoError(t, err)
	assert.Contains(t, out, "Updated todo list with 1 item")

	rows, err := store.QueryPlanTodos(ctx, "sess-noop-emit")
	require.NoError(t, err)
	require.Len(t, rows, 1, "audit row MUST persist even with noop emitter (P12-L4 INSERT-only substrate)")
}

// TestNoLegacyInterruptAPI_DeepAudit_Panic — Test that the panic
// guards in BuildWriteTodosTool don't catch a recovered panic and
// silently proceed. This is a tiny assertion on the noop case.
func TestBuildWriteTodosTool_RecoversNoPanic(t *testing.T) {
	store := newTestStore(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildWriteTodosTool must NOT recover nil-emitter panic: %v", r)
		}
	}()
	tool := BuildWriteTodosTool(store, "sess-recover", nil)
	if tool == nil {
		t.Fatal("BuildWriteTodosTool must return a non-nil Tool when emitter is nil")
	}
}

// _ ensures the unused import in test compilation doesn't fail.
var _ = sql.ErrConnDone