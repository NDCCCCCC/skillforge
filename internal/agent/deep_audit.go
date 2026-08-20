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

// Package agent — deep_audit.go is Phase 12 / Plan 12-02's audited
// write_todos Tool substrate for the DeepAgent archetype.
//
// Why a CUSTOM Tool (not intercepting the auto-injection)?
//
//	github.com/cloudwego/eino/adk/prebuilt/deep.New auto-injects
//	write_todos via its buildTypedBuiltinAgentMiddlewares (deep.go:
//	244-282 in eino v0.9.14). The middleware is opaque — it calls
//	adk.AddSessionValue(ctx, SessionKeyTodos, input.Todos) directly
//	and SkillForge never sees the tool call to emit audit + SSE.
//
//	The Option C path (deep.Config.WithoutWriteTodos: true — set in
//	12-01's internal/runtime/deep_agent.go) trades the auto-injection
//	for full control: we register our own InvokableTool that does
//	(a) PutPlanTodo INSERT, (b) EmitDataPlanTrace SSE chunk, (c)
//	adk.AddSessionValue(..., SessionKeyTodos, ...). The Tool name is
//	"write_todos" so the LLM-facing schema is identical to the
//	auto-injected default (no prompt drift).
//
// 3-EFFECT CONTRACT — every write_todos call MUST execute in this
// order, and any effect failure MUST short-circuit the Tool with a
// non-nil error (P12-L4 fail-closed posture, mirrors HITL-04):
//
//	(1) AUDIT  — PutPlanTodo INSERT for each todo row. Failure
//	             surfaces as "audit INSERT fail-closed"; the SSE
//	             chunk is NOT emitted, the session value is NOT
//	             set, the LLM sees an error.
//	(2) SSE    — EmitDataPlanTrace chunk via the per-Run emitter.
//	             Failure surfaces as "SSE emit fail-closed"; the
//	             audit rows are committed (durable record survives)
//	             but the LLM sees an error. The next write_todos
//	             call from the LLM must re-emit the audit row to
//	             fix the SSE gap (no automatic retry — the audit
//	             table is the durable source of truth).
//	(3) VALUE  — adk.AddSessionValue(SessionKeyTodos, todos) to
//	             preserve the default DeepAgent behavior (the
//	             auto-injected Tool's sole side effect). This call
//	             is the LAST step and is best-effort (eino's
//	             AddSessionValue is a void function — no failure
//	             surface), but a nil session is tolerated.
//
// INVARIANTS:
//
//   - write_todos is NOT a HITL Tool. It does NOT call
//     tool.StatefulInterrupt (D8-04 invariant — only HITL Tools
//     inside internal/agent/hitl/ are allowed to interrupt).
//   - The audit INSERT happens BEFORE the SSE emit so the durable
//     record is written even if the SSE channel is closed mid-tool.
//     Mirrors Phase 11 D11-C3 (audit_writer sync INSERT BEFORE
//     StatefulInterrupt).
//   - The audit row, the SSE chunk, and the session value all carry
//     the SAME todos list (the post-write snapshot) — drift between
//     these three surfaces would break the audit-vs-live parity
//     invariant that the FE relies on for the planning trace UI.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/skillforge/skillforge/internal/wire"
)

// WriteTodosToolName is the Eino tool name emitted by the LLM in
// tool_calls[i].Function.Name. Matches eino/adk/prebuilt/deep's
// auto-injected Tool name verbatim (deep.go:254 in eino v0.9.14 uses
// utils.InferTool with the literal "write_todos") so the LLM-facing
// schema is identical to the default behavior — no prompt drift, no
// SKILL.md update needed.
const WriteTodosToolName = "write_todos"

// writeTodosArgs is the typed parse target for the Tool's argsInJSON.
// Mirrors eino/adk/prebuilt/deep.writeTodosArguments (deep.go:240 in
// eino v0.9.14) field-for-field — Status is constrained to
// pending|in_progress|completed via the json.RawMessage-level
// jsonschema enum registered on wire.PlanTodo.Status.
//
// The Info() method surfaces a richer schema (with enum + Required)
// via schema.NewParamsOneOfByParams so the LLM is FORBIDDEN to drift
// from the canonical shape.
type writeTodosArgs struct {
	Todos []wire.PlanTodo `json:"todos"`
}

// WriteTodosEmitter is the per-Run SSE emitter seam. The production
// implementation (added in 12-06) closes over *app.RequestContext and
// writes via wire.EmitChunk; tests + the no-SSE path use
// noopWriteTodosEmitter. Returns a non-nil error on any SSE failure
// (P12-L4 fail-closed — Tool returns error, LLM run aborts).
type WriteTodosEmitter interface {
	// EmitDataPlanTrace writes one data-plan-trace SSE chunk carrying
	// the post-write todos snapshot. The Type discriminator and
	// PlanTracePayload marshalling are the emitter's responsibility;
	// the Tool only passes the typed payload.
	EmitDataPlanTrace(ctx context.Context, payload wire.PlanTracePayload) error
}

// noopWriteTodosEmitter is the default when BuildWriteTodosTool.emitter
// is nil (tests + the no-SSE path). Always returns nil — the audit
// row is still written, the session value is still set, only the
// live-stream surface is suppressed.
type noopWriteTodosEmitter struct{}

// EmitDataPlanTrace satisfies WriteTodosEmitter by returning nil.
func (noopWriteTodosEmitter) EmitDataPlanTrace(_ context.Context, _ wire.PlanTracePayload) error {
	return nil
}

// writeTodosAuditedTool is the Option C audited replacement for the
// eino auto-injected write_todos middleware (deep.go:244 in eino
// v0.9.14). Every Tool call executes the 3-effect contract documented
// at the top of this file:
//
//	(1) PutPlanTodo audit INSERT (fail-closed)
//	(2) EmitDataPlanTrace SSE chunk (fail-closed)
//	(3) adk.AddSessionValue(SessionKeyTodos, todos) (best-effort)
//
// The Tool is stateless beyond its wiring fields (store + runID +
// emitter) — every piece of session state lives in the eino Runner's
// session-value map, not on the Tool instance.
type writeTodosAuditedTool struct {
	store   *sqlite.SQLiteStore
	runID   string
	emitter WriteTodosEmitter
}

// Info returns the LLM-visible tool schema. Params are declared via
// schema.NewParamsOneOfByParams (typed map form, parallel to
// hitl.MemoryFactsTool.Info) because the Tool's InvokableRun takes a
// raw `args string` and parses it manually — the schema is what the
// LLM is FORBIDDEN to drift from.
//
// Status enum: "pending" | "in_progress" | "completed" — mirrors
// eino's deep.TODO.Status jsonschema tag enum
// (jsonschema:"enum=pending,enum=in_progress,enum=completed") so the
// LLM emits identical JSON to the auto-injected default.
func (t *writeTodosAuditedTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: WriteTodosToolName,
		Desc: "Update the todo list for this agent run. The todo list is persisted to the audit log and streamed live to the UI as a planning trace. Use this BEFORE the first tool call to declare your plan, and UPDATE it whenever the plan changes.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"todos": {
				Type:     schema.Array,
				Required: true,
				Desc:     "Full todo snapshot (replaces the prior snapshot). Each todo has id, content, optional activeForm, and status.",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"id": {
							Type:     schema.String,
							Required: true,
							Desc:     "Stable todo identifier (e.g. 't1'). Reusing an id across calls updates the existing todo.",
						},
						"content": {
							Type:     schema.String,
							Required: true,
							Desc:     "Human-readable todo description.",
						},
						"activeForm": {
							Type:     schema.String,
							Required: false,
							Desc:     "Optional present-continuous label rendered while the todo is in_progress.",
						},
						"status": {
							Type:     schema.String,
							Required: true,
							Enum:     []string{"pending", "in_progress", "completed"},
							Desc:     "Todo status. pending = not started; in_progress = currently working; completed = done.",
						},
					},
				},
			},
		}),
	}, nil
}

// InvokableRun is the audited write_todos Tool body. Three effects,
// three failure modes — see the file-level godoc for the contract.
//
// Note: NO StatefulInterrupt call. write_todos is NOT a HITL Tool
// (D8-04 invariant — only HITL Tools inside internal/agent/hitl/
// are allowed to interrupt). This is a regular audit Tool that
// emits a chunk + writes audit + sets a session value.
func (t *writeTodosAuditedTool) InvokableRun(ctx context.Context, argsInJSON string, _ ...tool.Option) (string, error) {
	var args writeTodosArgs
	if err := json.Unmarshal([]byte(argsInJSON), &args); err != nil {
		return "", fmt.Errorf("write_todos: invalid args: %w", err)
	}
	if len(args.Todos) == 0 {
		return "", fmt.Errorf("write_todos: at least one todo required")
	}
	for i, todo := range args.Todos {
		if todo.ID == "" {
			return "", fmt.Errorf("write_todos: todos[%d].id is required", i)
		}
		if todo.Content == "" {
			return "", fmt.Errorf("write_todos: todos[%d].content is required", i)
		}
		if todo.Status != "pending" && todo.Status != "in_progress" && todo.Status != "completed" {
			return "", fmt.Errorf("write_todos: todos[%d].status=%q must be one of pending|in_progress|completed", i, todo.Status)
		}
	}

	now := time.Now().UTC()

	// (1) AUDIT INSERT — T-3-AUDIT fail-closed (P12-L4). Each todo
	// row gets its own PutPlanTodo call. PRIMARY KEY (run_id, todo_id)
	// UNIQUE makes a same-key call an UPDATE (INSERT OR REPLACE) — the
	// replay substrate stays "latest content wins" per
	// TestPlanTodosDuplicatePKUpsert.
	for _, todo := range args.Todos {
		if err := t.store.PutPlanTodo(ctx, sqlite.PlanTodoRow{
			RunID:      t.runID,
			TodoID:     todo.ID,
			Content:    todo.Content,
			ActiveForm: todo.ActiveForm,
			Status:     todo.Status,
			Ts:         now,
			Op:         sqlite.PlanTodoOpUpsert,
		}); err != nil {
			return "", fmt.Errorf("write_todos: audit INSERT fail-closed: %w", err)
		}
	}

	// (2) SSE EMIT — live trace for PLAN-02 (Pitfall 6: must be
	// visible live). The emitter owns the wire marshalling + SSE
	// write — the Tool only passes the typed payload. Failure
	// surfaces as "SSE emit fail-closed"; audit rows are durable
	// even when SSE is down.
	if err := t.emitter.EmitDataPlanTrace(ctx, wire.PlanTracePayload{
		RunID: t.runID,
		Todos: args.Todos,
		Ts:    now.Unix(),
	}); err != nil {
		return "", fmt.Errorf("write_todos: SSE emit fail-closed: %w", err)
	}

	// (3) SESSION VALUE — preserve the default DeepAgent behavior
	// (eino's auto-injected write_todos sole side effect is
	// adk.AddSessionValue(SessionKeyTodos, todos)). eino's
	// AddSessionValue is void (runctx.go:214-221) — no error to
	// surface, no failure mode to handle.
	adk.AddSessionValue(ctx, deep.SessionKeyTodos, args.Todos)

	return fmt.Sprintf("Updated todo list with %d items", len(args.Todos)), nil
}

// BuildWriteTodosTool constructs the audited write_todos Tool for the
// DeepAgent archetype. The Tool replaces the auto-injected middleware
// (Option C, set via deep.Config.WithoutWriteTodos: true in
// internal/runtime/deep_agent.go).
//
// Required fields: store (for the audit INSERT), runID (the
// per-session identity), emitter (the per-Run SSE seam). nil →
// panic (factory-time invariant — the Tool is mandatory for
// DeepAgent; the runtime factory wires it via DeepAgentTools in
// internal/agent/deep_registry.go).
//
// Returns a tool.InvokableTool so the caller's []tool.BaseTool slice
// (compose.ToolsNodeConfig.Tools) accepts it without further
// conversion. The 3-effect contract is enforced inside InvokableRun;
// this constructor is a pure wiring seam.
func BuildWriteTodosTool(store *sqlite.SQLiteStore, runID string, emitter WriteTodosEmitter) tool.InvokableTool {
	if store == nil {
		panic("agent.BuildWriteTodosTool: store is required (audit INSERT target)")
	}
	if runID == "" {
		panic("agent.BuildWriteTodosTool: runID is required (per-session identity)")
	}
	if emitter == nil {
		// Fall back to the noop emitter so tests + the no-SSE path
		// don't need a wiring ceremony. Production wiring (12-06)
		// passes a real Hertz-backed emitter.
		emitter = noopWriteTodosEmitter{}
	}
	return &writeTodosAuditedTool{
		store:   store,
		runID:   runID,
		emitter: emitter,
	}
}