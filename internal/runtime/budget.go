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

// Package runtime — budget.go is Phase 12 / Plan 12-01's BudgetConfig
// + default constants + NewBudgetCallbacks factory.
//
// P12-L2 lock: DefaultMaxIteration = 10. The hard cap on DeepAgent
// reasoning iterations lives in deep_agent.go's `const maxIter = 10`
// literal — this constant exists for the test reflection lock
// (TestMaxIterationHardCap source-greps the literal in the factory
// file). The two values MUST stay in sync; the test would catch a
// mismatch via source-grep, not via runtime reference.
//
// Plan 12-01 ships the placeholder empty slice for Handlers; Plan
// 12-05 fills in the token-budget accumulator + the fnv64a loop-
// detection middleware (P12-L9). The placeholder keeps the
// deep.Config.Handlers wiring compilable so the runtime substrate can
// be tested in isolation before 12-05 ships.
package runtime

import (
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// DefaultMaxIteration is the P12-L2 hard cap on DeepAgent reasoning
// iterations. Source-grep locked via TestMaxIterationHardCap in
// deep_agent_test.go. DO NOT raise this without an explicit user
// decision — Pitfall 8 (DeepAgent default 50 = cost explosion on
// GLM-5.2).
//
// The hardcoded literal `10` ALSO appears in deep_agent.go's
// `const maxIter = 10` — both are intentional. The constant here is
// the exported knob name (for tests + 12-04 spike calibration); the
// literal in the factory is the source-grep lock that makes the cap
// structural (config-driven knobs can drift; a code literal cannot).
const DefaultMaxIteration = 10

// DefaultTokenBudgetPerRun is the Phase 12 initial value for per-Run
// token accumulation. Calibration lives in 12-08 acceptance gate.
// Production writes the actual usage to the audit table; 50,000 is
// a conservative ceiling that prevents runaway loops before 12-05's
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
type BudgetConfig struct {
	SessionID    string
	MaxIteration int
	TokenBudget  int
	LoopWindow   int
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
// that deep.Config.Handlers consumes. Phase 12 / Plan 12-01 returns an
// EMPTY slice (the placeholder); Plan 12-05 fills in:
//
//   - BeforeModel middleware that increments a per-SessionID token
//     accumulator and fires forced_stop when TokenBudget is exceeded.
//   - AfterModel middleware that hashes tool calls into the loop
//     detector (fnv64a(tool_name + first 200 chars of args)) and
//     fires forced_stop when DefaultLoopDetectWindow duplicates are
//     observed within a single Run.
//
// The empty placeholder exists so the deep.Config.Handlers wiring
// compiles + tests can assert via reflection that the slice has the
// expected (empty-for-now) shape. TestNewBudgetCallbacksEmpty locks
// the empty-slice invariant until 12-05 replaces this body.
//
// Type parameter: adk.TypedChatModelAgentMiddleware[*schema.Message]
// matches deep.Config.Handlers's element type (the DeepAgent factory
// uses the standard message type, not the agentic one).
func NewBudgetCallbacks(cfg BudgetConfig) []adk.TypedChatModelAgentMiddleware[*schema.Message] {
	_ = cfg // 12-05 consumes cfg.SessionID + cfg.TokenBudget + cfg.LoopWindow
	return []adk.TypedChatModelAgentMiddleware[*schema.Message]{}
}