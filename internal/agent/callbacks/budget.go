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

// Package agentcallbacks — budget.go is Phase 12 / Plan 12-05's 3-layer
// budget gate middleware for the DeepAgent archetype (PLAN-04).
//
// The three layers (defense-in-depth; ANY layer tripping fires
// forced_stop conservatively — never run-to-exhaustion):
//
//	Layer 1 — max_iter: per-run model-invocation counter. The STRUCTURAL
//	          cap is deep.Config.MaxIteration: 10 hardcoded in
//	          internal/runtime/deep_agent.go (P12-L2 source-grep lock);
//	          this middleware independently counts BeforeModelRewriteState
//	          calls and fires max_iter forced_stop on invocation
//	          MaxIteration+1. Dual enforcement means a drift in either
//	          the factory literal or the middleware leaves the other
//	          intact.
//	Layer 2 — token:   atomic.Int64 accumulator on
//	          msg.ResponseMeta.Usage.TotalTokens (OpenAI-compatible
//	          standard field; GLM-5.2 reports it). Checked BEFORE each
//	          model call in BeforeModelRewriteState; accumulated AFTER
//	          each model call in AfterModelRewriteState.
//	Layer 3 — loop:    fnv64a(tool_name + first 200 chars of args)
//	          rolling-window hash detector on consecutive identical tool
//	          calls (P12-L9). The 200-char truncation tolerates
//	          legitimate multi-hop refinement — Phase 3's pattern of
//	          retrieve("差旅费") → retrieve("差旅费 报销标准 v2") hashes
//	          differently past the shared prefix (Pitfall 5 mitigation).
//
// On ANY gate trigger, the middleware (in this exact order — D11-C3
// audit-first fail-closed posture, mirroring Phase 11's
// hitl/audit_writer.go):
//
//	(1) AUDIT  — Store.PutAudit(action="action_budget_triggered",
//	           budget_layer=<max_iter|token|loop>). If the INSERT
//	           fails, the middleware returns the error WITHOUT
//	           emitting the SSE chunk — the user must never see a
//	           forced_stop that the durable record does not back.
//	(2) SSE    — Emitter.EmitForcedStopDecision(reason) writes the
//	           data-decision{decision:"forced_stop", reason:...}
//	           chunk (P12-L3: existing data-decision discriminator,
//	           D16 overload — NO new wire chunk type).
//	(3) ERROR  — the middleware returns a non-nil error, which the
//	           adk runner surfaces as the run's terminal error. The
//	           model call / tool call that tripped the gate NEVER
//	           executes.
//
// D8-04 invariant: the middleware does NOT call tool.StatefulInterrupt
// — forced_stop is an SSE data-decision emit + a returned error, not a
// HITL interrupt.
package agentcallbacks

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// Budget defaults (Phase 12 initial calibration; 12-08 acceptance gate
// tunes TokenBudget against the 50-case golden set — Open Question #5).
//
// These mirror runtime.DefaultTokenBudgetPerRun /
// runtime.DefaultLoopDetectWindow / runtime.DefaultMaxIteration. They
// are duplicated here (not imported) because the dependency arrow is
// runtime → agentcallbacks (internal/runtime/budget.go calls
// NewBudgetCallback); importing runtime from here would cycle.
const (
	// defaultTokenBudgetPerRun covers ~10 LLM calls with 5K context
	// each — the conservative ceiling that prevents a runaway loop from
	// burning $5+ in API cost before the gate fires.
	defaultTokenBudgetPerRun = 50000
	// defaultLoopDetectWindow is the P12-L9 rolling-window size: 3
	// consecutive identical fnv64a hashes → loop forced_stop.
	defaultLoopDetectWindow = 3
	// defaultMaxIteration mirrors the P12-L2 structural cap. See the
	// Layer 1 note above for why the middleware ALSO enforces it.
	defaultMaxIteration = 10
)

// loopHashTruncateLen is the args truncation length for the loop hash
// (P12-L9 / Pitfall 5). First 200 chars: long enough that a refined
// multi-hop query (which differs within the first 200 chars of the
// args JSON) hashes differently; short enough that hash-stable
// formatting noise past char 200 does not mask a true loop.
const loopHashTruncateLen = 200

// BudgetAuditStore is the audit-INSERT seam the budget middleware
// consumes. *sqlite.SQLiteStore satisfies it; tests substitute a fake
// that records call order (the D11-C3 audit-before-emit assertion
// needs a shared event log, which the concrete store cannot provide).
type BudgetAuditStore interface {
	// PutAudit appends an immutable audit row (T-3-AUDIT INSERT-only).
	PutAudit(ctx context.Context, row store.AuditRow) (int64, error)
}

// BudgetEmitter is the per-Run SSE emitter seam for the forced_stop
// data-decision chunk. Production (12-07) closes over the Hertz SSE
// writer; 12-05 ships NoopBudgetEmitter so the middleware is testable
// before the FE surface lands.
type BudgetEmitter interface {
	// EmitForcedStopDecision writes one
	// data-decision{decision:"forced_stop", reason:...} SSE chunk
	// (P12-L3: existing discriminator; the decision string enum extends,
	// no new chunk type).
	EmitForcedStopDecision(ctx context.Context, reason string) error
}

// NoopBudgetEmitter is the nil-receiver fallback for BudgetConfig.
// Always returns nil — the audit row is still written (fail-closed is
// about ORDER, not about suppressing the durable record), only the
// live-stream surface is suppressed.
type NoopBudgetEmitter struct{}

// EmitForcedStopDecision satisfies BudgetEmitter by returning nil.
func (NoopBudgetEmitter) EmitForcedStopDecision(_ context.Context, _ string) error {
	return nil
}

// BudgetConfig carries the per-Run budget gate knobs + wiring. Zero
// values are replaced by Resolve()'s defaults. RunID + Store are the
// minimum wiring for the audit INSERT; Emitter falls back to
// NoopBudgetEmitter when nil.
type BudgetConfig struct {
	// TokenBudget is the Layer 2 per-run ceiling on accumulated
	// Usage.TotalTokens. Default 50000.
	TokenBudget int
	// LoopWindow is the Layer 3 rolling-window size (consecutive
	// identical tool-call hashes before forced_stop). Default 3.
	LoopWindow int
	// MaxIteration is the Layer 1 per-run model-invocation ceiling.
	// Mirrors deep.Config.MaxIteration (P12-L2). Default 10.
	MaxIteration int
	// RunID is the per-run identity written to audit.session_id (the
	// same value the plan_todos table + the replay endpoint key on).
	RunID string
	// SkillID labels the audit row (audit.skill_id is NOT NULL).
	SkillID string
	// ModelVersion labels the audit row (audit.model_version is NOT
	// NULL); e.g. "glm-5.2".
	ModelVersion string
	// Store is the audit INSERT target. Required for trigger semantics;
	// when nil the middleware logs-and-continues on trigger (the gate
	// still fires — budget protection must not depend on audit
	// availability).
	Store BudgetAuditStore
	// Emitter is the per-Run SSE seam. nil → NoopBudgetEmitter.
	Emitter BudgetEmitter
}

// Resolve returns a copy of c with zero-valued knobs replaced by the
// package defaults. Pure function — no side effects.
func (c BudgetConfig) Resolve() BudgetConfig {
	out := c
	if out.TokenBudget <= 0 {
		out.TokenBudget = defaultTokenBudgetPerRun
	}
	if out.LoopWindow <= 0 {
		out.LoopWindow = defaultLoopDetectWindow
	} else if out.LoopWindow == 1 {
		// Window of 1 would flag EVERY call as a loop (a single hash
		// trivially equals itself) — a self-inflicted denial of service
		// on the gate. Clamp up to 2 (the documented floor).
		out.LoopWindow = 2
	}
	if out.MaxIteration <= 0 {
		out.MaxIteration = defaultMaxIteration
	}
	if out.Emitter == nil {
		out.Emitter = NoopBudgetEmitter{}
	}
	return out
}

// BudgetState is the per-Run mutable gate state. One instance is
// shared by all three layers (the middleware is constructed once per
// Run; adk invokes its hooks per model/tool call).
//
// Concurrency: TotalTokens is atomic (AfterModelRewriteState may run
// on the streaming path); recentHashes + modelCalls are guarded by mu
// (tool calls within one iteration execute sequentially in adk's
// ToolsNode, but the wrapper API does not promise single-threaded
// invocation — the mutex keeps the window math race-free under -race).
type BudgetState struct {
	TotalTokens atomic.Int64

	mu           sync.Mutex
	recentHashes []uint64
	modelCalls   int
}

// budgetMiddleware is the adk.TypedChatModelAgentMiddleware
// implementation. It embeds the no-op base so only the budget-relevant
// hooks are overridden (eino v0.9.14 adk/handler.go:261 —
// TypedBaseChatModelAgentMiddleware provides default no-ops for all
// interface methods).
type budgetMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]

	config BudgetConfig
	state  *BudgetState
}

// NewBudgetCallback constructs the 3-layer budget gate middleware.
// The returned value is consumed by deep.Config.Handlers (the
// DeepAgent factory in internal/runtime/deep_agent.go via
// internal/runtime/budget.go::NewBudgetCallbacks).
//
// cfg is resolved (defaults filled) internally; the caller may pass a
// zero-value TokenBudget/LoopWindow/MaxIteration and get the package
// defaults.
func NewBudgetCallback(cfg BudgetConfig) adk.TypedChatModelAgentMiddleware[*schema.Message] {
	cfg = cfg.Resolve()
	return &budgetMiddleware{
		TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.Message]{},
		config:                            cfg,
		state:                             &BudgetState{},
	}
}

// BeforeModelRewriteState is the pre-model-call gate for Layers 1 + 2.
// Fires BEFORE each ChatModel invocation (eino v0.9.14
// adk/handler.go:160-171).
//
//	Layer 1: increment modelCalls; if modelCalls > MaxIteration →
//	         forced_stop(max_iter). Dual enforcement with the factory's
//	         deep.Config.MaxIteration (P12-L2).
//	Layer 2: if accumulated TotalTokens > TokenBudget →
//	         forced_stop(token).
//
// On trigger the middleware returns a non-nil error — the adk runner
// fails fast and the model call never executes (fail-fast contract per
// handler.go:156-157: "if any handler returns an error, subsequent
// handlers are NOT called").
func (m *budgetMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.TypedChatModelAgentState[*schema.Message],
	mc *adk.TypedModelContext[*schema.Message],
) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	_ = mc

	// Layer 1 — max_iter (middleware half of the dual enforcement).
	m.state.mu.Lock()
	m.state.modelCalls++
	calls := m.state.modelCalls
	m.state.mu.Unlock()
	if calls > m.config.MaxIteration {
		reason := fmt.Sprintf("PLAN-04 budget exhausted: max_iter (model invocation %d exceeds cap %d)", calls, m.config.MaxIteration)
		if err := m.triggerForcedStop(ctx, "max_iter", reason); err != nil {
			return ctx, state, err
		}
		return ctx, state, errors.New(reason)
	}

	// Layer 2 — token budget (checked BEFORE the model call so an
	// already-exhausted budget cannot fund one more invocation).
	if used := m.state.TotalTokens.Load(); used > int64(m.config.TokenBudget) {
		reason := fmt.Sprintf("PLAN-04 budget exhausted: token (used %d > budget %d)", used, m.config.TokenBudget)
		if err := m.triggerForcedStop(ctx, "token", reason); err != nil {
			return ctx, state, err
		}
		return ctx, state, errors.New(reason)
	}

	return ctx, state, nil
}

// AfterModelRewriteState is the post-model-call accumulator for
// Layer 2. Reads Usage.TotalTokens off the model's response message
// (the LAST message in state.Messages — the input state "includes the
// model's response as the last message" per eino v0.9.14
// adk/handler.go:174-176) and adds it to the atomic accumulator.
//
// Nil-safe on every hop of the ResponseMeta → Usage chain: providers
// that omit usage metadata (some streaming paths) contribute 0 rather
// than panicking the run.
func (m *budgetMiddleware) AfterModelRewriteState(
	ctx context.Context,
	state *adk.TypedChatModelAgentState[*schema.Message],
	mc *adk.TypedModelContext[*schema.Message],
) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	_ = mc
	if state != nil && len(state.Messages) > 0 {
		if msg := state.Messages[len(state.Messages)-1]; msg != nil &&
			msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
			m.state.TotalTokens.Add(int64(msg.ResponseMeta.Usage.TotalTokens))
		}
	}
	return ctx, state, nil
}

// WrapInvokableToolCall is the Layer 3 loop-detection wrapper for
// InvokableTools (write_todos / retrieve / plan_approve are all
// Invokable). The wrapper runs the hash check INSIDE the returned
// endpoint closure so the actual argumentsInJSON (available only at
// call time, not wrap time) feed the hash.
//
// On loop detection the endpoint returns an error WITHOUT invoking the
// wrapped endpoint — the tripped tool call never executes.
func (m *budgetMiddleware) WrapInvokableToolCall(
	_ context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	if tCtx == nil {
		return endpoint, nil
	}
	toolName := tCtx.Name
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		if err := m.checkLoop(ctx, toolName, argumentsInJSON); err != nil {
			return "", err
		}
		return endpoint(ctx, argumentsInJSON, opts...)
	}, nil
}

// WrapEnhancedInvokableToolCall mirrors WrapInvokableToolCall for
// EnhancedInvokableTools (schema.ToolArgument form). Same Layer 3
// semantics; hash input is the argument's JSON encoding.
func (m *budgetMiddleware) WrapEnhancedInvokableToolCall(
	_ context.Context,
	endpoint adk.EnhancedInvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.EnhancedInvokableToolCallEndpoint, error) {
	if tCtx == nil {
		return endpoint, nil
	}
	toolName := tCtx.Name
	return func(ctx context.Context, toolArgument *schema.ToolArgument, opts ...tool.Option) (*schema.ToolResult, error) {
		if toolArgument != nil {
			if err := m.checkLoop(ctx, toolName, toolArgument.Text); err != nil {
				return nil, err
			}
		}
		return endpoint(ctx, toolArgument, opts...)
	}, nil
}

// WrapStreamableToolCall mirrors WrapInvokableToolCall for
// StreamableTools. The loop check runs before the stream is opened —
// a tripped call returns an error stream rather than invoking the
// wrapped endpoint.
func (m *budgetMiddleware) WrapStreamableToolCall(
	_ context.Context,
	endpoint adk.StreamableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.StreamableToolCallEndpoint, error) {
	if tCtx == nil {
		return endpoint, nil
	}
	toolName := tCtx.Name
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
		if err := m.checkLoop(ctx, toolName, argumentsInJSON); err != nil {
			return nil, err
		}
		return endpoint(ctx, argumentsInJSON, opts...)
	}, nil
}

// checkLoop is the Layer 3 gate body: hash the (toolName + first 200
// chars of args) pair, push onto the rolling window, and fire
// forced_stop(loop) when the last LoopWindow hashes are all identical.
//
// The mutex covers the append + window-trim + equality check as one
// critical section (a torn read between append and check could miss a
// trigger under concurrent tool calls).
func (m *budgetMiddleware) checkLoop(ctx context.Context, toolName, args string) error {
	h := fnv.New64a()
	h.Write([]byte(toolName))
	argsTrunc := args
	if len(argsTrunc) > loopHashTruncateLen {
		argsTrunc = argsTrunc[:loopHashTruncateLen]
	}
	h.Write([]byte(argsTrunc))
	hash := h.Sum64()

	m.state.mu.Lock()
	m.state.recentHashes = append(m.state.recentHashes, hash)
	if len(m.state.recentHashes) > m.config.LoopWindow {
		m.state.recentHashes = m.state.recentHashes[len(m.state.recentHashes)-m.config.LoopWindow:]
	}
	allSame := len(m.state.recentHashes) == m.config.LoopWindow
	if allSame {
		for i := 1; i < len(m.state.recentHashes); i++ {
			if m.state.recentHashes[i] != m.state.recentHashes[0] {
				allSame = false
				break
			}
		}
	}
	m.state.mu.Unlock()

	if allSame {
		reason := fmt.Sprintf("PLAN-04 budget exhausted: loop (%d consecutive identical tool calls: %s)", m.config.LoopWindow, toolName)
		if err := m.triggerForcedStop(ctx, "loop", reason); err != nil {
			return err
		}
		return errors.New(reason)
	}
	return nil
}

// triggerForcedStop executes the D11-C3 audit-first trigger sequence:
//
//	(1) AUDIT INSERT — Store.PutAudit(action=action_budget_triggered,
//	    budget_layer=layer). Failure → return the error WITHOUT
//	    emitting: the user must never see a forced_stop the durable
//	    record does not back (mirrors Phase 11 hitl/audit_writer.go's
//	    fail-closed posture).
//	(2) SSE EMIT — Emitter.EmitForcedStopDecision(reason). Failure →
//	    return the error (the audit row is already durable; the run
//	    still aborts).
//
// The NOT NULL audit columns (skill_id / query / prompt_hash /
// model_version / verdict_json) are filled with the budget-gate
// identity values — the reason text doubles as the query column so the
// 12-07 AuditTimelineTab renders the trigger detail without a JOIN.
func (m *budgetMiddleware) triggerForcedStop(ctx context.Context, layer, reason string) error {
	// (1) AUDIT INSERT FIRST (D11-C3).
	if m.config.Store != nil {
		if _, err := m.config.Store.PutAudit(ctx, store.AuditRow{
			Timestamp:    time.Now().UTC(),
			SessionID:    m.config.RunID,
			SkillID:      m.config.SkillID,
			Query:        reason,
			PromptHash:   "budget-gate",
			ModelVersion: m.config.ModelVersion,
			VerdictJSON:  "{}",
			Action:       sqlite.ActionBudgetTriggered,
			BudgetLayer:  layer,
		}); err != nil {
			// Fail-closed: NO SSE emit when the audit INSERT failed.
			return fmt.Errorf("budget gate %s: audit INSERT fail-closed (SSE suppressed): %w", layer, err)
		}
	}

	// (2) SSE EMIT SECOND (only after the durable record exists).
	if err := m.config.Emitter.EmitForcedStopDecision(ctx, reason); err != nil {
		return fmt.Errorf("budget gate %s: SSE emit failed: %w", layer, err)
	}
	return nil
}
