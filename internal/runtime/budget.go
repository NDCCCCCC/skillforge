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

// Package runtime — budget.go is the Phase 12 budget-gate wiring seam.
// BudgetConfig (12-01) + the 12-05 factory that turns it into the
// 3-layer budget middleware slice deep.Config.Handlers consumes.
//
// P12-L2 lock: DefaultMaxIteration = 10. The hard cap on DeepAgent
// reasoning iterations lives in deep_agent.go's `MaxIteration: 10`
// literal inside the deep.New call — this constant exists for the test
// reflection lock (TestMaxIterationHardCap_Literal10 source-greps the
// literal in the factory file). The two values MUST stay in sync; the
// test would catch a mismatch via source-grep, not via runtime
// reference.
//
// Plan 12-05 replaced the 12-01 empty placeholder: NewBudgetCallbacks
// now returns exactly ONE middleware —
// agentcallbacks.NewBudgetCallback — which layers (1) the middleware-
// side max_iter counter (dual enforcement with the factory literal),
// (2) the atomic token accumulator on Usage.TotalTokens, and (3) the
// fnv64a rolling-window loop detector (P12-L9). On ANY layer trigger
// the middleware writes the action_budget_triggered audit row FIRST,
// then emits the data-decision{decision:"forced_stop"} SSE chunk
// (D11-C3 audit-first fail-closed posture).
package runtime

import (
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	agentcallbacks "github.com/skillforge/skillforge/internal/agent/callbacks"
)

// DefaultMaxIteration is the P12-L2 hard cap on DeepAgent reasoning
// iterations. Source-grep locked via TestMaxIterationHardCap_Literal10
// in deep_agent_test.go. DO NOT raise this without an explicit user
// decision — Pitfall 8 (DeepAgent default 50 = cost explosion on
// GLM-5.2).
//
// The literal `10` ALSO appears inline in deep_agent.go's deep.New
// call (`MaxIteration: 10`) — both are intentional. The constant here
// is the exported knob name (for tests + 12-04 spike calibration); the
// inline literal in the factory is the source-grep lock that makes the
// cap structural (config-driven knobs can drift; a code literal
// cannot).
const DefaultMaxIteration = 10

// DefaultTokenBudgetPerRun is the Phase 12 initial value for per-Run
// token accumulation. Calibration lives in 12-08 acceptance gate.
// Production writes the actual usage to the audit table; 50,000 is
// a conservative ceiling that prevents runaway loops before the
// token-accumulator middleware fires `forced_stop`.
const DefaultTokenBudgetPerRun = 50000

// DefaultLoopDetectWindow is the P12-L9 loop detection window.
// Window=3 means: if the same fnv64a(tool_name + first 200 chars of
// args) hash is seen 3 times within a single Run, fire forced_stop.
// The hash-based design tolerates legitimate multi-hop variation
// (truncation moves the boundary; refined queries hash differently).
const DefaultLoopDetectWindow = 3

// BudgetConfig carries the per-session budget knobs that get passed
// into DeepAgent via deep.Config.Handlers. Zero values fall back to
// the package defaults via Resolve().
//
// SessionID is the canonical per-session identifier used as the
// handlers' audit + emit key. Empty SessionID is rejected at the
// BuildDeepAgent call site (cfg.SessionID == "" returns an error).
//
// Store + Emitter are the 12-05 wiring seams: the audit INSERT target
// (T-12-05-4 operator visibility) and the per-Run SSE emitter for the
// data-decision{decision:"forced_stop"} chunk. Both may be nil for
// tests / the no-SSE path — the middleware still fires the gate
// (budget protection must not depend on audit availability); only the
// durable row / live-stream surface is suppressed.
type BudgetConfig struct {
	SessionID    string
	MaxIteration int
	TokenBudget  int
	LoopWindow   int
	Store        agentcallbacks.BudgetAuditStore
	Emitter      agentcallbacks.BudgetEmitter
}

// Resolve returns a copy of c with zero-valued knobs replaced by the
// package defaults. The original c is unchanged.
//
// Used by NewBudgetCallbacks at the bottom of the DeepAgent
// construction path so callers can override ONE knob without having
// to re-state all three. Pure function — no side effects, safe to
// call from any goroutine.
func (c BudgetConfig) Resolve() BudgetConfig {
	out := c
	if out.MaxIteration <= 0 {
		out.MaxIteration = DefaultMaxIteration
	}
	if out.TokenBudget <= 0 {
		out.TokenBudget = DefaultTokenBudgetPerRun
	}
	if out.LoopWindow <= 0 {
		out.LoopWindow = DefaultLoopDetectWindow
	}
	return out
}

// NewBudgetCallbacks returns the slice of TypedChatModelAgentMiddleware
// that deep.Config.Handlers consumes. Phase 12 / Plan 12-05 returns
// exactly ONE middleware — the 3-layer budget gate:
//
//   - Layer 1 (max_iter): BeforeModelRewriteState counts model
//     invocations and fires forced_stop on MaxIteration+1 (dual
//     enforcement with the factory's structural cap; either side
//     drifting leaves the other intact).
//   - Layer 2 (token): AfterModelRewriteState accumulates
//     Usage.TotalTokens atomically; BeforeModelRewriteState refuses
//     the next call once the budget is exceeded.
//   - Layer 3 (loop): WrapInvokableToolCall hashes
//     fnv64a(tool_name + first 200 chars of args) into a rolling
//     LoopWindow-sized window and fires forced_stop on N consecutive
//     identical hashes.
//
// On ANY trigger: audit INSERT (action_budget_triggered +
// budget_layer) FIRST, then the SSE data-decision emit (D11-C3);
// a failed audit INSERT suppresses the emit (fail-closed).
//
// Type parameter: adk.TypedChatModelAgentMiddleware[*schema.Message]
// matches deep.Config.Handlers's element type (the DeepAgent factory
// uses the standard message type, not the agentic one).
func NewBudgetCallbacks(cfg BudgetConfig) []adk.TypedChatModelAgentMiddleware[*schema.Message] {
	return []adk.TypedChatModelAgentMiddleware[*schema.Message]{
		agentcallbacks.NewBudgetCallback(agentcallbacks.BudgetConfig{
			TokenBudget:  cfg.TokenBudget,
			LoopWindow:   cfg.LoopWindow,
			MaxIteration: cfg.MaxIteration,
			RunID:        cfg.SessionID,
			Store:        cfg.Store,
			Emitter:      cfg.Emitter,
		}),
	}
}
