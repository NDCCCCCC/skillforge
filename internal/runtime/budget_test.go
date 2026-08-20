// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	agentcallbacks "github.com/skillforge/skillforge/internal/agent/callbacks"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// TestDefaultMaxIteration pins the P12-L2 source-grep lock value at
// 10. The actual cap lives as the literal `10` in deep_agent.go
// (TestMaxIterationHardCap_Literal10 asserts that side); the constant
// here is the exported knob name for tests + future calibration.
func TestDefaultMaxIteration(t *testing.T) {
	if DefaultMaxIteration != 10 {
		t.Fatalf("DefaultMaxIteration = %d, want 10 (P12-L2)", DefaultMaxIteration)
	}
}

// TestDefaultTokenBudgetPerRun pins the Phase 12 initial token budget.
// Calibration lives in the 12-08 acceptance gate; if you bump this
// here, document the rationale + update eval/golden/deep.jsonl.
func TestDefaultTokenBudgetPerRun(t *testing.T) {
	if DefaultTokenBudgetPerRun != 50000 {
		t.Fatalf("DefaultTokenBudgetPerRun = %d, want 50000", DefaultTokenBudgetPerRun)
	}
}

// TestDefaultLoopDetectWindow pins the P12-L9 loop detection window.
// Window=3 means three same-hash tool calls in a row → forced_stop.
// Hash key: fnv64a(tool_name + first 200 chars of args).
func TestDefaultLoopDetectWindow(t *testing.T) {
	if DefaultLoopDetectWindow != 3 {
		t.Fatalf("DefaultLoopDetectWindow = %d, want 3 (P12-L9)", DefaultLoopDetectWindow)
	}
}

// TestBudgetConfig_Resolve_FillsDefaults verifies the zero-value fill.
func TestBudgetConfig_Resolve_FillsDefaults(t *testing.T) {
	c := BudgetConfig{SessionID: "sess-x"}.Resolve()
	if c.MaxIteration != DefaultMaxIteration {
		t.Errorf("MaxIteration = %d, want %d", c.MaxIteration, DefaultMaxIteration)
	}
	if c.TokenBudget != DefaultTokenBudgetPerRun {
		t.Errorf("TokenBudget = %d, want %d", c.TokenBudget, DefaultTokenBudgetPerRun)
	}
	if c.LoopWindow != DefaultLoopDetectWindow {
		t.Errorf("LoopWindow = %d, want %d", c.LoopWindow, DefaultLoopDetectWindow)
	}
	if c.SessionID != "sess-x" {
		t.Errorf("SessionID = %q, want %q (Resolve must preserve SessionID)", c.SessionID, "sess-x")
	}
}

// TestBudgetConfig_Resolve_PreservesExplicit checks explicit values
// are NOT overwritten by the defaults.
func TestBudgetConfig_Resolve_PreservesExplicit(t *testing.T) {
	c := BudgetConfig{
		SessionID:    "sess-y",
		MaxIteration: 5,
		TokenBudget:  10000,
		LoopWindow:   5,
	}.Resolve()
	if c.MaxIteration != 5 {
		t.Errorf("MaxIteration = %d, want 5 (explicit preserved)", c.MaxIteration)
	}
	if c.TokenBudget != 10000 {
		t.Errorf("TokenBudget = %d, want 10000 (explicit preserved)", c.TokenBudget)
	}
	if c.LoopWindow != 5 {
		t.Errorf("LoopWindow = %d, want 5 (explicit preserved)", c.LoopWindow)
	}
}

// TestBudgetConfig_Resolve_DoesNotMutate verifies Resolve() is a pure
// function — the input BudgetConfig is unchanged after the call.
func TestBudgetConfig_Resolve_DoesNotMutate(t *testing.T) {
	in := BudgetConfig{SessionID: "sess-z"}
	_ = in.Resolve()
	if in.MaxIteration != 0 {
		t.Errorf("Resolve mutated input: MaxIteration now %d, want 0", in.MaxIteration)
	}
	if in.TokenBudget != 0 {
		t.Errorf("Resolve mutated input: TokenBudget now %d, want 0", in.TokenBudget)
	}
	if in.LoopWindow != 0 {
		t.Errorf("Resolve mutated input: LoopWindow now %d, want 0", in.LoopWindow)
	}
	if in.SessionID != "sess-z" {
		t.Errorf("Resolve mutated input: SessionID now %q, want %q", in.SessionID, "sess-z")
	}
}

// ---------------------------------------------------------------------
// 12-05 — NewBudgetCallbacks wiring tests (behavioral, via the adk
// middleware interface: the agentcallbacks state is unexported by
// design, so the runtime tests observe the resolved knobs through the
// gate's behavior rather than reading its config).
// ---------------------------------------------------------------------

// rtBudgetStore is the runtime-test BudgetAuditStore fake.
type rtBudgetStore struct {
	rows []store.AuditRow
}

// Compile-time proof the runtime-test fakes satisfy the agentcallbacks
// seams (BudgetConfig.Store / BudgetConfig.Emitter field types).
var (
	_ agentcallbacks.BudgetAuditStore = (*rtBudgetStore)(nil)
	_ agentcallbacks.BudgetEmitter    = (*rtBudgetEmitter)(nil)
)

func (s *rtBudgetStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	s.rows = append(s.rows, row)
	return int64(len(s.rows)), nil
}

// rtBudgetEmitter is the runtime-test BudgetEmitter fake.
type rtBudgetEmitter struct {
	reasons []string
}

func (e *rtBudgetEmitter) EmitForcedStopDecision(_ context.Context, reason string) error {
	e.reasons = append(e.reasons, reason)
	return nil
}

// rtUsageState builds the AfterModelRewriteState input whose last
// message carries Usage.TotalTokens (the accumulator input shape).
func rtUsageState(totalTokens int) *adk.TypedChatModelAgentState[*schema.Message] {
	return &adk.TypedChatModelAgentState[*schema.Message]{
		Messages: []*schema.Message{
			{ResponseMeta: &schema.ResponseMeta{
				Usage: &schema.TokenUsage{TotalTokens: totalTokens},
			}},
		},
	}
}

// TestNewBudgetCallbacks_ReturnsOneMiddleware — the 12-05 ship: the
// placeholder empty slice is gone; exactly ONE middleware (the 3-layer
// budget gate) is returned for deep.Config.Handlers.
func TestNewBudgetCallbacks_ReturnsOneMiddleware(t *testing.T) {
	got := NewBudgetCallbacks(BudgetConfig{SessionID: "sess-1mw"})
	if got == nil {
		t.Fatalf("NewBudgetCallbacks returned nil; expected 1 middleware")
	}
	if len(got) != 1 {
		t.Fatalf("NewBudgetCallbacks returned %d middlewares; want 1 (the 3-layer budget gate)", len(got))
	}
	if got[0] == nil {
		t.Fatalf("NewBudgetCallbacks[0] is nil")
	}
}

// TestNewBudgetCallbacks_IncludesStoreAndEmitter — Store + Emitter
// passed via runtime.BudgetConfig flow THROUGH to the middleware: a
// token-gate trigger writes the audit row (keyed to SessionID) AND
// emits the forced_stop reason.
func TestNewBudgetCallbacks_IncludesStoreAndEmitter(t *testing.T) {
	st := &rtBudgetStore{}
	em := &rtBudgetEmitter{}
	mw := NewBudgetCallbacks(BudgetConfig{
		SessionID:   "sess-wire",
		TokenBudget: 1500,
		Store:       st,
		Emitter:     em,
	})[0]

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, _, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{}); err != nil {
			t.Fatalf("model call %d: unexpected gate error: %v", i+1, err)
		}
		if _, _, err := mw.AfterModelRewriteState(ctx, rtUsageState(1000), &adk.TypedModelContext[*schema.Message]{}); err != nil {
			t.Fatalf("model call %d accumulate: %v", i+1, err)
		}
	}
	_, _, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
	if err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted: token") {
		t.Fatalf("3rd model call: want token gate error, got %v", err)
	}

	if len(st.rows) != 1 {
		t.Fatalf("audit rows = %d, want 1 (Store must flow through to the middleware)", len(st.rows))
	}
	if st.rows[0].Action != sqlite.ActionBudgetTriggered {
		t.Errorf("audit Action = %q, want %q", st.rows[0].Action, sqlite.ActionBudgetTriggered)
	}
	if st.rows[0].BudgetLayer != "token" {
		t.Errorf("audit BudgetLayer = %q, want token", st.rows[0].BudgetLayer)
	}
	if st.rows[0].SessionID != "sess-wire" {
		t.Errorf("audit SessionID = %q, want sess-wire (BudgetConfig.SessionID → RunID mapping)", st.rows[0].SessionID)
	}
	if len(em.reasons) != 1 || !strings.Contains(em.reasons[0], "PLAN-04 budget exhausted") {
		t.Fatalf("emitter reasons = %v, want one forced_stop reason (Emitter must flow through)", em.reasons)
	}
}

// TestNewBudgetCallbacks_DefaultsApplied — empty knobs resolve to the
// package defaults INSIDE the middleware: exactly 50000 tokens pass
// (strictly-greater semantics), 50001 trips; 2 identical tool calls
// pass, the 3rd trips (LoopWindow=3).
func TestNewBudgetCallbacks_DefaultsApplied(t *testing.T) {
	mw := NewBudgetCallbacks(BudgetConfig{SessionID: "sess-defaults"})[0]
	ctx := context.Background()

	// Token default = 50000: accumulate exactly 50000 (2 calls), the
	// next BeforeModel still passes (50000 > 50000 is false); one more
	// token trips it.
	for i := 0; i < 2; i++ {
		if _, _, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{}); err != nil {
			t.Fatalf("model call %d: unexpected gate error: %v", i+1, err)
		}
		if _, _, err := mw.AfterModelRewriteState(ctx, rtUsageState(25000), &adk.TypedModelContext[*schema.Message]{}); err != nil {
			t.Fatalf("model call %d accumulate: %v", i+1, err)
		}
	}
	if _, _, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{}); err != nil {
		t.Fatalf("at-budget model call (50000 used, 50000 budget) must still pass: %v", err)
	}
	if _, _, err := mw.AfterModelRewriteState(ctx, rtUsageState(1), &adk.TypedModelContext[*schema.Message]{}); err != nil {
		t.Fatalf("over-budget accumulate: %v", err)
	}
	_, _, err := mw.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
	if err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted") {
		t.Fatalf("over-budget model call (50001 used) must trip, got %v", err)
	}

	// LoopWindow default = 3: fresh middleware (the token gate above
	// already consumed the model-call budget headroom).
	mw2 := NewBudgetCallbacks(BudgetConfig{})[0]
	endpoint := func(_ context.Context, _ string, _ ...tool.Option) (string, error) {
		return "ok", nil
	}
	wrapped, werr := mw2.WrapInvokableToolCall(ctx, endpoint, &adk.ToolContext{Name: "retrieve"})
	if werr != nil {
		t.Fatalf("WrapInvokableToolCall: %v", werr)
	}
	for i := 0; i < 2; i++ {
		if _, err := wrapped(ctx, `{"q":"same"}`); err != nil {
			t.Fatalf("identical tool call %d with default window=3 must pass: %v", i+1, err)
		}
	}
	if _, err := wrapped(ctx, `{"q":"same"}`); err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted: loop") {
		t.Fatalf("3rd identical tool call with default window=3 must trip, got %v", err)
	}
}
