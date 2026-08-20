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

// Package agentcallbacks — budget_test.go is Phase 12 / Plan 12-05
// Task 2: the hermetic test suite for the 3-layer budget gate
// middleware (PLAN-04 / SC-4).
//
// Coverage map (plan's 11 required tests + the D11-C3 ordering locks):
//
//	Layer 1 (max_iter):
//	  - TestBudgetMaxIterationCap — P12-L2 source-grep + the 11th
//	    BeforeModel call fires max_iter forced_stop.
//	  - TestBudgetAuditRowOnTrigger/max_iter — audit row layer value.
//	Layer 2 (token):
//	  - TestBudgetTokenCounterAccumulation — atomic sum across calls.
//	  - TestBudgetTokenBudgetExceeded — strictly-greater semantics.
//	  - TestBudgetTokenBudgetAuditRow — audit row layer value.
//	  - TestBudgetAuditRowOnTrigger/token — same, subtest form.
//	Layer 3 (loop):
//	  - TestBudgetLoopDetection3InRow — 3 identical → 3rd trips.
//	  - TestBudgetLoopDetectionLegitimateMultiHop — Pitfall 5.
//	  - TestBudgetLoopDetection4InRowThreshold — configurable window.
//	  - TestBudgetAuditRowOnTrigger/loop — audit row layer value.
//	Emit + audit seams:
//	  - TestBudgetForcedStopEmitsDataDecision — reason contract.
//	  - TestBudgetTriggerOrderAuditBeforeEmit — D11-C3 order lock.
//	  - TestBudgetFailClosedClosedStoreSuppressesSSE — closed store →
//	    NO SSE chunk (the fail-closed half of D11-C3).
//	Config:
//	  - TestBudgetConfigResolveDefaults
//	  - TestBudgetConfigResolvePreservesExplicit
//	  - TestBudgetLoopWindowFloor — LoopWindow=1 clamped to 2 (a
//	    1-window would flag the first call as a loop).
//
// The tests are white-box (same package) because the middleware state
// (BudgetState) and the wrapped-endpoint seam are unexported by design
// — the ONLY production consumer is deep.Config.Handlers via
// runtime.NewBudgetCallbacks.
package agentcallbacks

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// ---------------------------------------------------------------------
// Test seams
// ---------------------------------------------------------------------

// eventLog is the shared ordered event recorder the D11-C3 ordering
// assertion reads. The budget middleware calls Store BEFORE Emitter on
// trigger; the shared log makes the order observable from one place.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.events))
	copy(out, l.events)
	return out
}

// recordingStore is the BudgetAuditStore fake. err != nil simulates a
// failed / closed store (the fail-closed D11-C3 path).
type recordingStore struct {
	log  *eventLog
	rows []store.AuditRow
	err  error
}

func (s *recordingStore) PutAudit(_ context.Context, row store.AuditRow) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.rows = append(s.rows, row)
	if s.log != nil {
		s.log.add("audit:" + row.BudgetLayer)
	}
	return int64(len(s.rows)), nil
}

// recordingEmitter is the BudgetEmitter fake. err != nil simulates a
// dead SSE channel.
type recordingEmitter struct {
	log     *eventLog
	reasons []string
	err     error
}

func (e *recordingEmitter) EmitForcedStopDecision(_ context.Context, reason string) error {
	if e.err != nil {
		return e.err
	}
	e.reasons = append(e.reasons, reason)
	if e.log != nil {
		e.log.add("emit:" + reason)
	}
	return nil
}

// usageState builds a TypedChatModelAgentState whose LAST message
// carries Usage.TotalTokens — the AfterModelRewriteState accumulator
// input shape (adk handler.go:174-176: the input state includes the
// model's response as the last message).
func usageState(totalTokens int) *adk.TypedChatModelAgentState[*schema.Message] {
	return &adk.TypedChatModelAgentState[*schema.Message]{
		Messages: []*schema.Message{
			{ResponseMeta: &schema.ResponseMeta{
				Usage: &schema.TokenUsage{TotalTokens: totalTokens},
			}},
		},
	}
}

// callModel drives one BeforeModel → AfterModel round trip. Returns
// the BeforeModel error (nil = gate passed). A nil AfterModel error is
// asserted inline — the accumulator has no failure mode.
func callModel(
	t *testing.T,
	m *budgetMiddleware,
	totalTokens int,
) error {
	t.Helper()
	ctx := context.Background()
	_, _, err := m.BeforeModelRewriteState(ctx, &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
	if err != nil {
		return err
	}
	_, _, err = m.AfterModelRewriteState(ctx, usageState(totalTokens), &adk.TypedModelContext[*schema.Message]{})
	if err != nil {
		t.Fatalf("AfterModelRewriteState returned error: %v", err)
	}
	return nil
}

// toolDriver wraps the middleware's Invokable-tool seam with a
// passthrough endpoint that counts real invocations, so tests can
// assert a tripped call NEVER reaches the wrapped tool.
type toolDriver struct {
	m           *budgetMiddleware
	invocations int
}

func (d *toolDriver) call(toolName, args string) error {
	wrapped, err := d.m.WrapInvokableToolCall(
		context.Background(),
		func(_ context.Context, _ string, _ ...tool.Option) (string, error) {
			d.invocations++
			return "ok", nil
		},
		&adk.ToolContext{Name: toolName},
	)
	if err != nil {
		return err
	}
	_, err = wrapped(context.Background(), args)
	return err
}

// callTool drives one wrapped Invokable tool call through a fresh
// driver and returns its error.
func callTool(t *testing.T, m *budgetMiddleware, toolName, args string) error {
	t.Helper()
	d := &toolDriver{m: m}
	return d.call(toolName, args)
}

// ---------------------------------------------------------------------
// Config resolution
// ---------------------------------------------------------------------

// TestBudgetConfigResolveDefaults — empty BudgetConfig resolves to the
// package defaults (50000 / 3 / 10) + NoopBudgetEmitter.
func TestBudgetConfigResolveDefaults(t *testing.T) {
	got := BudgetConfig{RunID: "sess-r"}.Resolve()
	if got.TokenBudget != 50000 {
		t.Errorf("TokenBudget = %d, want 50000 (default)", got.TokenBudget)
	}
	if got.LoopWindow != 3 {
		t.Errorf("LoopWindow = %d, want 3 (default)", got.LoopWindow)
	}
	if got.MaxIteration != 10 {
		t.Errorf("MaxIteration = %d, want 10 (default)", got.MaxIteration)
	}
	if _, ok := got.Emitter.(NoopBudgetEmitter); !ok {
		t.Errorf("Emitter = %T, want NoopBudgetEmitter (default)", got.Emitter)
	}
	if got.RunID != "sess-r" {
		t.Errorf("RunID = %q, want %q (Resolve must preserve RunID)", got.RunID, "sess-r")
	}
}

// TestBudgetConfigResolvePreservesExplicit — explicit knobs are NOT
// overwritten by the defaults.
func TestBudgetConfigResolvePreservesExplicit(t *testing.T) {
	got := BudgetConfig{TokenBudget: 10000, LoopWindow: 4, MaxIteration: 5}.Resolve()
	if got.TokenBudget != 10000 {
		t.Errorf("TokenBudget = %d, want 10000 (explicit preserved)", got.TokenBudget)
	}
	if got.LoopWindow != 4 {
		t.Errorf("LoopWindow = %d, want 4 (explicit preserved)", got.LoopWindow)
	}
	if got.MaxIteration != 5 {
		t.Errorf("MaxIteration = %d, want 5 (explicit preserved)", got.MaxIteration)
	}
}

// TestBudgetLoopWindowFloor — LoopWindow=1 would flag the FIRST tool
// call as a loop (a single hash trivially equals itself). Resolve()
// must clamp it to the documented floor of 2.
func TestBudgetLoopWindowFloor(t *testing.T) {
	got := BudgetConfig{LoopWindow: 1}.Resolve().LoopWindow
	if got != 2 {
		t.Errorf("LoopWindow=1 resolved to %d, want 2 (floor)", got)
	}
}

// ---------------------------------------------------------------------
// Layer 2 — token accumulator + gate
// ---------------------------------------------------------------------

// TestBudgetTokenCounterAccumulation — 3 ChatModel calls of 1000
// Usage.TotalTokens each accumulate to 3000 on the atomic counter.
func TestBudgetTokenCounterAccumulation(t *testing.T) {
	m := NewBudgetCallback(BudgetConfig{}).(*budgetMiddleware)
	for i := 0; i < 3; i++ {
		if err := callModel(t, m, 1000); err != nil {
			t.Fatalf("call %d: unexpected gate error: %v", i+1, err)
		}
	}
	if got := m.state.TotalTokens.Load(); got != 3000 {
		t.Fatalf("TotalTokens = %d, want 3000", got)
	}
}

// TestBudgetTokenBudgetExceeded — strictly-greater semantics: the gate
// refuses the model call AFTER accumulated usage exceeds the budget
// (the run gets its full budget; the next call is refused). Budget
// 1500, calls of 1000: call 3 sees used=2000 > 1500 → forced_stop.
func TestBudgetTokenBudgetExceeded(t *testing.T) {
	m := NewBudgetCallback(BudgetConfig{TokenBudget: 1500}).(*budgetMiddleware)
	if err := callModel(t, m, 1000); err != nil { // used: 0 → 1000
		t.Fatalf("call 1: unexpected gate error: %v", err)
	}
	if err := callModel(t, m, 1000); err != nil { // used: 1000 → 2000
		t.Fatalf("call 2: unexpected gate error: %v", err)
	}
	err := callModel(t, m, 1000) // before call 3: 2000 > 1500 → trip
	if err == nil {
		t.Fatalf("call 3 BeforeModel: expected token budget error, got nil")
	}
	if !strings.Contains(err.Error(), "PLAN-04 budget exhausted: token") {
		t.Fatalf("error = %q, want substring %q", err.Error(), "PLAN-04 budget exhausted: token")
	}
	if !strings.Contains(err.Error(), "2000 > budget 1500") {
		t.Fatalf("error = %q, want usage detail %q", err.Error(), "2000 > budget 1500")
	}
}

// TestBudgetTokenBudgetAuditRow — the token trigger writes an
// action_budget_triggered audit row with budget_layer=token keyed to
// the RunID.
func TestBudgetTokenBudgetAuditRow(t *testing.T) {
	st := &recordingStore{}
	m := NewBudgetCallback(BudgetConfig{
		TokenBudget: 1500,
		RunID:       "sess-tok-audit",
		Store:       st,
		Emitter:     &recordingEmitter{},
	}).(*budgetMiddleware)
	_ = callModel(t, m, 1000)
	_ = callModel(t, m, 1000)
	if err := callModel(t, m, 1000); err == nil {
		t.Fatalf("expected token budget trigger on call 3")
	}
	if len(st.rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(st.rows))
	}
	row := st.rows[0]
	if row.Action != sqlite.ActionBudgetTriggered {
		t.Errorf("Action = %q, want %q", row.Action, sqlite.ActionBudgetTriggered)
	}
	if row.BudgetLayer != "token" {
		t.Errorf("BudgetLayer = %q, want %q", row.BudgetLayer, "token")
	}
	if row.SessionID != "sess-tok-audit" {
		t.Errorf("SessionID = %q, want %q", row.SessionID, "sess-tok-audit")
	}
}

// ---------------------------------------------------------------------
// Layer 1 — max_iter cap (dual enforcement with the factory literal)
// ---------------------------------------------------------------------

// findModuleRoot walks upward from the test's cwd until it finds a
// go.mod, then joins relPath. Mirrors the helper in
// internal/runtime/deep_agent_test.go — tests may run from any cwd
// (gopls variant, terminal, CI), so a fixed relative path is not
// portable on Windows.
func findModuleRoot(relPath string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir + "/" + relPath, nil
		}
		parent := dir
		if dir == "/" || dir[len(dir)-1] == ':' {
			return "", os.ErrNotExist
		}
		for i := len(dir) - 1; i >= 0; i-- {
			if dir[i] == '/' || dir[i] == '\\' {
				dir = dir[:i]
				if dir == "" {
					dir = "/"
				}
				break
			}
		}
		if parent == dir {
			return "", os.ErrNotExist
		}
	}
}

// TestBudgetMaxIterationCap — (a) the P12-L2 source-grep lock: the
// runtime factory MUST carry the literal `MaxIteration: 10`; (b) the
// middleware half: the 11th BeforeModel call fires max_iter
// forced_stop even if the factory's structural cap were to drift.
func TestBudgetMaxIterationCap(t *testing.T) {
	// (a) source-grep via the module-root walk (cwd-portable).
	path, err := findModuleRoot("internal/runtime/deep_agent.go")
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read deep_agent.go: %v", err)
	}
	if !strings.Contains(string(src), "MaxIteration: 10") {
		t.Fatalf("P12-L2 lock broken: internal/runtime/deep_agent.go must contain the literal `MaxIteration: 10`")
	}

	// (b) middleware enforcement — default MaxIteration=10.
	st := &recordingStore{}
	m := NewBudgetCallback(BudgetConfig{Store: st, Emitter: &recordingEmitter{}}).(*budgetMiddleware)
	for i := 1; i <= 10; i++ {
		_, _, err := m.BeforeModelRewriteState(context.Background(), &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
		if err != nil {
			t.Fatalf("invocation %d: unexpected max_iter error: %v", i, err)
		}
	}
	_, _, err = m.BeforeModelRewriteState(context.Background(), &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
	if err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted: max_iter") {
		t.Fatalf("invocation 11: want max_iter forced_stop error, got %v", err)
	}
	if len(st.rows) != 1 || st.rows[0].BudgetLayer != "max_iter" {
		t.Fatalf("max_iter trigger must write exactly one audit row with layer max_iter; got %+v", st.rows)
	}
}

// ---------------------------------------------------------------------
// Layer 3 — fnv64a loop detection
// ---------------------------------------------------------------------

// TestBudgetLoopDetection3InRow — 3 consecutive identical tool calls
// (same name + same args): the 3rd trips the loop gate, and the
// wrapped tool endpoint is NEVER invoked for the tripped call.
func TestBudgetLoopDetection3InRow(t *testing.T) {
	m := NewBudgetCallback(BudgetConfig{}).(*budgetMiddleware)
	d := &toolDriver{m: m}
	sameArgs := `{"query":"差旅费 报销标准"}`
	for i := 1; i <= 2; i++ {
		if err := d.call("retrieve_compliance_chunks", sameArgs); err != nil {
			t.Fatalf("call %d: unexpected loop error: %v", i, err)
		}
	}
	if d.invocations != 2 {
		t.Fatalf("endpoint invocations = %d, want 2 before the trip", d.invocations)
	}
	err := d.call("retrieve_compliance_chunks", sameArgs)
	if err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted: loop") {
		t.Fatalf("call 3: want loop forced_stop error, got %v", err)
	}
	if !strings.Contains(err.Error(), "3 consecutive identical tool calls: retrieve_compliance_chunks") {
		t.Fatalf("error = %q, want tool-name detail", err.Error())
	}
	if d.invocations != 2 {
		t.Fatalf("endpoint invocations = %d after trip, want 2 (the tripped call MUST NOT reach the tool)", d.invocations)
	}
}

// TestBudgetLoopDetectionLegitimateMultiHop — Pitfall 5 mitigation:
// the same tool called 3 times with REFINED queries (different within
// the first 200 chars) hashes differently → NO false positive.
func TestBudgetLoopDetectionLegitimateMultiHop(t *testing.T) {
	m := NewBudgetCallback(BudgetConfig{}).(*budgetMiddleware)
	queries := []string{
		`{"query":"差旅费"}`,
		`{"query":"差旅费 报销标准 v2 限额"}`,
		`{"query":"差旅费 报销标准 v2 限额 高铁 二等座"}`,
	}
	for i, args := range queries {
		if err := callTool(t, m, "retrieve_compliance_chunks", args); err != nil {
			t.Fatalf("multi-hop call %d: false-positive loop trigger: %v", i+1, err)
		}
	}
}

// TestBudgetLoopDetection4InRowThreshold — the window is configurable:
// LoopWindow=4 requires 4 consecutive identical calls before the trip.
func TestBudgetLoopDetection4InRowThreshold(t *testing.T) {
	m := NewBudgetCallback(BudgetConfig{LoopWindow: 4}).(*budgetMiddleware)
	sameArgs := `{"query":"identical"}`
	for i := 1; i <= 3; i++ {
		if err := callTool(t, m, "retrieve", sameArgs); err != nil {
			t.Fatalf("call %d with LoopWindow=4: premature loop trigger: %v", i, err)
		}
	}
	if err := callTool(t, m, "retrieve", sameArgs); err == nil || !strings.Contains(err.Error(), "PLAN-04 budget exhausted: loop") {
		t.Fatalf("call 4 with LoopWindow=4: want loop trigger, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Emit + audit seams (D11-C3 posture)
// ---------------------------------------------------------------------

// TestBudgetForcedStopEmitsDataDecision — every trigger calls
// EmitForcedStopDecision exactly once with the reason contract
// "PLAN-04 budget exhausted: <layer> (...)".
func TestBudgetForcedStopEmitsDataDecision(t *testing.T) {
	em := &recordingEmitter{}
	m := NewBudgetCallback(BudgetConfig{TokenBudget: 1500, Store: &recordingStore{}, Emitter: em}).(*budgetMiddleware)
	_ = callModel(t, m, 1000)
	_ = callModel(t, m, 1000)
	_ = callModel(t, m, 1000) // trips token gate
	if len(em.reasons) != 1 {
		t.Fatalf("emit count = %d, want 1", len(em.reasons))
	}
	if !strings.Contains(em.reasons[0], "PLAN-04 budget exhausted") {
		t.Fatalf("reason = %q, want prefix %q", em.reasons[0], "PLAN-04 budget exhausted")
	}
}

// TestBudgetAuditRowOnTrigger — all three layers write the audit row
// with their own budget_layer value (T-12-05-4 operator visibility).
func TestBudgetAuditRowOnTrigger(t *testing.T) {
	t.Run("max_iter", func(t *testing.T) {
		st := &recordingStore{}
		m := NewBudgetCallback(BudgetConfig{Store: st, Emitter: &recordingEmitter{}}).(*budgetMiddleware)
		for i := 0; i < 11; i++ {
			_, _, _ = m.BeforeModelRewriteState(context.Background(), &adk.TypedChatModelAgentState[*schema.Message]{}, &adk.TypedModelContext[*schema.Message]{})
		}
		if len(st.rows) != 1 || st.rows[0].BudgetLayer != "max_iter" {
			t.Fatalf("want one max_iter audit row, got %+v", st.rows)
		}
	})
	t.Run("token", func(t *testing.T) {
		st := &recordingStore{}
		m := NewBudgetCallback(BudgetConfig{TokenBudget: 1500, Store: st, Emitter: &recordingEmitter{}}).(*budgetMiddleware)
		_ = callModel(t, m, 1000)
		_ = callModel(t, m, 1000)
		_ = callModel(t, m, 1000)
		if len(st.rows) != 1 || st.rows[0].BudgetLayer != "token" {
			t.Fatalf("want one token audit row, got %+v", st.rows)
		}
	})
	t.Run("loop", func(t *testing.T) {
		st := &recordingStore{}
		m := NewBudgetCallback(BudgetConfig{Store: st, Emitter: &recordingEmitter{}}).(*budgetMiddleware)
		for i := 0; i < 3; i++ {
			_ = callTool(t, m, "retrieve", `{"q":"same"}`)
		}
		if len(st.rows) != 1 || st.rows[0].BudgetLayer != "loop" {
			t.Fatalf("want one loop audit row, got %+v", st.rows)
		}
	})
}

// TestBudgetTriggerOrderAuditBeforeEmit — the D11-C3 ordering lock:
// the audit INSERT event MUST precede the SSE emit event in the shared
// ordered log (mirrors Phase 11 hitl/audit_writer.go posture).
func TestBudgetTriggerOrderAuditBeforeEmit(t *testing.T) {
	log := &eventLog{}
	st := &recordingStore{log: log}
	em := &recordingEmitter{log: log}
	m := NewBudgetCallback(BudgetConfig{TokenBudget: 1500, Store: st, Emitter: em}).(*budgetMiddleware)
	_ = callModel(t, m, 1000)
	_ = callModel(t, m, 1000)
	_ = callModel(t, m, 1000) // trips token gate

	events := log.snapshot()
	if len(events) != 2 {
		t.Fatalf("events = %v, want exactly 2 (one audit + one emit)", events)
	}
	if !strings.HasPrefix(events[0], "audit:token") {
		t.Fatalf("event[0] = %q, want the audit INSERT to fire FIRST (D11-C3)", events[0])
	}
	if !strings.HasPrefix(events[1], "emit:") {
		t.Fatalf("event[1] = %q, want the SSE emit to fire SECOND (D11-C3)", events[1])
	}
}

// TestBudgetFailClosedClosedStoreSuppressesSSE — the fail-closed half
// of D11-C3: when the audit INSERT fails (here: a CLOSED real
// SQLiteStore, whose PutAudit returns sql.ErrConnDone), the
// middleware returns the error WITHOUT emitting any SSE chunk — the
// user must never see a forced_stop the durable record does not back.
func TestBudgetFailClosedClosedStoreSuppressesSSE(t *testing.T) {
	s, err := sqlite.New(context.Background(), sqlite.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("store close: %v", err)
	}

	log := &eventLog{}
	em := &recordingEmitter{log: log}
	m := NewBudgetCallback(BudgetConfig{Store: s, Emitter: em}).(*budgetMiddleware)
	d := &toolDriver{m: m}
	if err := d.call("retrieve", `{"q":"same"}`); err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}
	if err := d.call("retrieve", `{"q":"same"}`); err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}
	err = d.call("retrieve", `{"q":"same"}`) // 3rd identical → loop trigger
	if err == nil {
		t.Fatalf("expected fail-closed error on loop trigger with closed store")
	}
	if !strings.Contains(err.Error(), "audit INSERT fail-closed") {
		t.Fatalf("error = %q, want %q substring", err.Error(), "audit INSERT fail-closed")
	}
	if !strings.Contains(err.Error(), "(SSE suppressed)") {
		t.Fatalf("error = %q, want SSE-suppressed marker", err.Error())
	}
	if len(em.reasons) != 0 {
		t.Fatalf("SSE emit count = %d, want 0 (closed store MUST suppress the emit)", len(em.reasons))
	}
	if events := log.snapshot(); len(events) != 0 {
		t.Fatalf("events = %v, want empty (no audit, no emit)", events)
	}
}
