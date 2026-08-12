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

// Package agent implements the Phase 3 ReAct agent that wraps Phase 2's
// linear VerdictPipeline into an Eino ReAct loop with three trust anchors
// (D3-01 / D3-04 / D3-03):
//
//   - **Trust floor (D3-01, D-09)** — a deterministic Go post-check that
//     forces refusal when the agent returned an answer but the retrieve
//     tool never produced any tier-matched chunks. Implemented in
//     trust_floor.go (Task 2). This invariant CANNOT be removed by the
//     LLM — it is structural, regression-locked by TestTrustFloorForcesRefusal.
//
//   - **Self-RAG critique (D3-04 / D3-05)** — a second LLM pass that
//     re-examines the agent's answer against the retrieved chunks. The
//     critique runs only when there is at least one grounded citation
//     (D3-04 gating). Critique can ONLY add strictness — it can set
//     ReviewRequired=true or downgrade Confidence/Status — never raise
//     them. Implemented in critique.go (Task 2), regression-locked by
//     TestCritiqueCannotRaiseConfidence.
//
//   - **Decision observability (D3-03 / SC-1)** — Eino callbacks emit a
//     data-decision SSE chunk per agent hop so the React UI can render
//     the agent's autonomous decisions (retrieve / answer / refuse /
//     forced_stop). Implemented in callbacks.go (Task 3), regression-locked
//     by TestDecisionSSEEmission.
//
// The retrieve tool (retrieve_tool.go) is the ONLY knowledge-source
// surface available to the agent loop. It closes over the Skill manifest's
// resolved tier slice (KNOW-03) — the agent LLM cannot override tiers.
// Chunks are framed with the [CHUNK N] ... \n---\n envelope verbatim
// from internal/server/pipeline.go buildPrompt (T-2-21 prompt-injection
// mitigation).
//
// API adaptation note (Rule 3 deviation from PLAN.md):
//
//	The plan assumed `adk.NewRunner(ctx, adk.RunnerConfig{Agent: reactAgent})`
//	+ iterator drain (per RESEARCH.md §Code Examples). Verified at compile
//	time against eino@v0.7.7: `react.Agent` does NOT satisfy the `adk.Agent`
//	interface (different Generate/Stream signatures), so `adk.NewRunner`
//	cannot wrap it. The actual API is `react.NewAgent` →
//	`reactAgent.Generate(ctx, msgs, agent.WithComposeOptions(...))` which
//	returns `*schema.Message` synchronously (no iterator). This file
//	uses the actual API directly; the deviation is documented in SUMMARY.md.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// DefaultMaxHops is the production default for AgentConfig.MaxHops. Two
// hops covers the canonical compliance-review flow: hop 1 retrieves the
// most-likely chunk, hop 2 retrieves a complementary chunk if hop 1 was
// insufficient. The D3-02 MaxStep cap (ComputeMaxStep) bounds the total
// LLM turns to MaxHops*2 + 2 = 6 — see 03-RESEARCH.md §Pitfall 1.
const DefaultMaxHops = 2

// AgentConfig is the production configuration for the compliance-review
// ReAct agent. All pointer/interface fields are REQUIRED; NewAgent
// rejects nil values up front (Rule 2 — fail-fast on misconfiguration).
type AgentConfig struct {
	// Manifest is the loaded SKILL.md manifest. Its ResolvedTiers slice
	// closes over the retrieve tool (KNOW-03 — the agent LLM cannot
	// override tiers).
	Manifest *skill.Manifest

	// Embedder converts the user's query (and refined queries emitted by
	// the LLM) into the vector passed to Retriever.Search.
	Embedder embedder.Embedder

	// Retriever is the tiered vector-search backend (Milvus in production,
	// stubRetriever in tests).
	Retriever retriever.Retriever

	// ChatModel is the underlying LLM. Typed as
	// model.ToolCallingChatModel (NOT the deprecated model.ChatModel)
	// because react.NewAgent's ToolCallingModel field requires the
	// WithTools method. BindTools-only models (the deprecated
	// model.ChatModel variant) cannot drive a ReAct loop and are
	// rejected at this field type.
	ChatModel model.ToolCallingChatModel

	// MaxHops bounds the number of retrieve_compliance_chunks
	// invocations the agent may make per Run. Default if zero:
	// DefaultMaxHops (2). Translated to react.MaxStep via ComputeMaxStep.
	MaxHops int

	// Emit (optional) is invoked once per agent hop with a populated
	// wire.DecisionPayload. Production wires this to a data-decision SSE
	// emitter; tests assert on the captured payloads. When nil, no
	// decision chunks are emitted (agent runs silently — used by unit
	// tests that do not exercise SC-1 observability).
	Emit DecisionEmitFunc
}

// DecisionEmitFunc is the signature for emitting one wire.DecisionPayload
// as a data-decision SSE chunk (D3-03 / SC-1). The alias keeps
// AgentConfig self-documenting and avoids importing internal/wire here
// (this package deals only in raw payloads — the wire-layer conversion
// happens at the call site, in internal/server).
type DecisionEmitFunc func(step int, decision, toolCallID, query string, confidence float64)

// Agent wraps a constructed *react.Agent plus the post-run trust floor
// and critique pipeline. Built by NewAgent; consumed by Run (Task 3).
//
// The react.Agent is the Eino primitive that runs the actual
// model→tools→model→... ReAct loop. We do NOT wrap it in an adk.Runner
// (see package docs: API adaptation note).
type Agent struct {
	react    *react.Agent
	cfg      AgentConfig
	retrieve tool.InvokableTool
}

// ComputeMaxStep implements the D3-02 MaxStep formula. Given MaxHops N:
//
//	MaxStep = N*2 + 2
//
// Rationale (03-RESEARCH.md §Pitfall 1): each hop consumes two pregel
// vertices (model call → tool call), so N hops = 2N model+tool pairs.
// The +2 reserves one vertex for the final answer and one as a safety
// margin. With DefaultMaxHops=2 this yields MaxStep=6 — empirically the
// smallest value that allows two retrieve calls plus a final grounded
// answer without the react loop bailing early.
//
// Exported so agent_test.go (TestComputeMaxStepFormula) can lock the
// formula regression-style without forcing a behavioral test.
func ComputeMaxStep(maxHops int) int {
	return maxHops*2 + 2
}

// NewAgent constructs the compliance-review ReAct agent.
//
// Pre-conditions (Rule 2 — fail-fast on misconfiguration):
//   - cfg.Manifest, cfg.Embedder, cfg.Retriever, cfg.ChatModel MUST be non-nil.
//   - cfg.MaxHops MUST be > 0 (zero falls back to DefaultMaxHops).
//   - cfg.ChatModel MUST implement model.ToolCallingChatModel (react's
//     ToolCallingModel field requires this interface — verified at the
//     type assertion below).
//
// Post-conditions:
//   - Returns a non-nil *Agent whose react field is a fully-constructed
//     *react.Agent.
//   - The retrieve tool is built once and reused across all Run calls
//     (the tool is stateless beyond the closed-over manifest).
//
// Errors are wrapped with the failing field name so the caller can fix
// the misconfiguration without spelunking through stack frames.
func NewAgent(ctx context.Context, cfg AgentConfig) (*Agent, error) {
	if cfg.Manifest == nil {
		return nil, errors.New("agent: AgentConfig.Manifest is required")
	}
	if cfg.Embedder == nil {
		return nil, errors.New("agent: AgentConfig.Embedder is required")
	}
	if cfg.Retriever == nil {
		return nil, errors.New("agent: AgentConfig.Retriever is required")
	}
	if cfg.ChatModel == nil {
		return nil, errors.New("agent: AgentConfig.ChatModel is required")
	}
	if cfg.MaxHops <= 0 {
		// Apply the production default rather than rejecting zero — this
		// mirrors how internal/server/pipeline.go treats MaxHops. Tests
		// that need a specific value set it explicitly.
		cfg.MaxHops = DefaultMaxHops
	}

	retrieveTool := BuildRetrieveTool(cfg.Embedder, cfg.Retriever, cfg.Manifest)

	// NOTE: the field name `ToolCallingModel` is verified against
	// eino@v0.7.7/flow/agent/react/react.go:50 — it is the recommended
	// field. The deprecated `Model model.ChatModel` field exists at
	// react.go:53 but does NOT accept ToolCallingChatModel-typed values
	// cleanly (the loss of WithTools capability breaks the ReAct loop).
	// AgentConfig.ChatModel is already typed as model.ToolCallingChatModel
	// so no runtime type assertion is needed here.
	reactCfg := &react.AgentConfig{
		ToolCallingModel: cfg.ChatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			// compose.ToolsNodeConfig.Tools is `[]tool.BaseTool` (verified
			// at eino@v0.7.7/compose/tool_node.go:121) — NOT
			// `InvokableTools` as the plan's research note A2 assumed.
			// tool.InvokableTool satisfies tool.BaseTool (BaseTool is the
			// minimal Info() interface; InvokableTool adds InvokableRun).
			Tools: []tool.BaseTool{retrieveTool},
		},
		MaxStep: ComputeMaxStep(cfg.MaxHops),
	}

	reactAgent, err := react.NewAgent(ctx, reactCfg)
	if err != nil {
		return nil, fmt.Errorf("agent: react.NewAgent: %w", err)
	}

	return &Agent{
		react:    reactAgent,
		cfg:      cfg,
		retrieve: retrieveTool,
	}, nil
}

// React exposes the underlying *react.Agent. Task 3's Run method uses
// this to call Generate with compose options (compose.WithCallbacks for
// SC-1 decision emission, compose.WithChatModelOption for the
// structured-output option slice). Exposed as a method (not a field) so
// future refactors can swap the underlying primitive without breaking
// callers.
func (a *Agent) React() *react.Agent { return a.react }

// RetrieveTool exposes the retrieve_compliance_chunks InvokableTool.
// Exposed for the trust floor (Task 2): the post-run check inspects
// the tool's chunkAccumulator to decide whether the agent actually saw
// any tier-matched context before answering.
func (a *Agent) RetrieveTool() tool.InvokableTool { return a.retrieve }

// Config returns a shallow copy of the AgentConfig the agent was built
// with. Shallow because the pointer/interface fields are shared with
// the underlying react.Agent — mutating them post-construction is
// unsupported (the changes would NOT propagate to the already-built
// react agent).
func (a *Agent) Config() AgentConfig { return a.cfg }

// Compile-time interface assertion. tool.InvokableTool embeds
// tool.BaseTool, so any value assignable to InvokableTool is also
// assignable to compose.ToolsNodeConfig.Tools ([]tool.BaseTool).
// The assertion below guarantees this at compile time; removing
// the embedding relationship in a future Eino upgrade would surface
// here as a build break before any production deploy.
//
// NOTE: model.ChatModel and model.ToolCallingChatModel are SIBLING
// interfaces (both extend BaseChatModel) — neither is a subtype of the
// other. ChatModel exposes BindTools; ToolCallingChatModel exposes
// WithTools. They are NOT assignable to each other, so we do NOT
// assert a subtyping relationship. react.AgentConfig.ToolCallingModel
// requires the WithTools variant, and NewAgent enforces that via a
// runtime type assertion (intentionally not compile-time — the
// AgentConfig.ChatModel field accepts the wider model.ChatModel
// interface for ergonomic reasons; rejecting a BindTools-only model
// only at NewAgent call time is fine).
var _ tool.BaseTool = (tool.InvokableTool)(nil)
