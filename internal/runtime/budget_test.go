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
	"testing"
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

// TestNewBudgetCallbacks_EmptyPlaceholder locks the 12-01 ship: the
// placeholder returns an empty slice. Plan 12-05 fills in the token
// + loop middleware. Until then, deep.Config.Handlers is a
// compilation-only consumer.
func TestNewBudgetCallbacks_EmptyPlaceholder(t *testing.T) {
	got := NewBudgetCallbacks(BudgetConfig{SessionID: "sess-p"})
	if got == nil {
		t.Fatalf("NewBudgetCallbacks returned nil; expected empty slice (12-05 will replace body)")
	}
	if len(got) != 0 {
		t.Fatalf("NewBudgetCallbacks returned %d middlewares; want 0 (12-05 fills)", len(got))
	}
}