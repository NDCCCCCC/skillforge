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

// Package agent — runner_setup.go is Phase 11 / Plan 11-01's
// BuildRunner construction site, extended in Phase 12 / Plan 12-01 to
// delegate agent construction to internal/runtime.DispatchAgent. It
// is the D11-A1 + D11-B1 + D11-B4 + P12-L1 substrate: chatHandler
// drives a single adk.NewRunner().Run(ctx, msgs) one-line dispatch;
// per-session Agent singleton; per-turn Tool whitelist re-pure-
// computed; archetype dispatch funnels through runtime.
//
// Architectural boundary (D11-A2 — HITL ships 11-02; P12-L1 —
// runtime substrate ships 12-01):
//
//   - 11-01 lands the empty HITL Tool seam. The Tool whitelist for
//     Skills with writes:[] is empty (the LLM never sees a write Tool
//     schema for them).
//   - 11-02 lands the real HITL Tools (memory:facts, review_report)
//     and replaces hitlBuildToolsForSkillStub with hitl.BuildToolsForSkill.
//   - 12-01 lifts the adk.NewChatModelAgent call site into
//     internal/runtime.DispatchAgent. The chat archetype is the
//     Phase 11 default; the deep archetype ships via
//     runtime.BuildDeepAgent (P12-L2 MaxIteration: 10 hard cap).
//   - NO legacy interrupt API is introduced here. D8-04 invariant
//     holds: the only call sites for tool.StatefulInterrupt are
//     inside 11-02's hitl/ files. Phase 12's runtime package does
//     NOT call StatefulInterrupt (12-01 ships the substrate; 12-03
//     ships plan_approve which DOES introduce a new HITL call site
//     — TestNoLegacyInterruptAPI is extended to scan runtime/ +
//     agent/plan/ in 12-03).
//
// eino v0.9.14 API note: the v0.9 ChatModelAgentConfig uses the
// `model.BaseModel[M]` Model field (not the v0.7 `ToolCallingChatModel`).
// Both embed the same interface surface so a v0.7 ToolCallingChatModel
// value is assignable to the v0.9 BaseModel[*schema.Message] field.
// The Handlers field in v0.9 takes []TypedChatModelAgentMiddleware[M]
// (not the v0.7 []callbacks.Handler) — for 11-01 we ship the empty
// default; 11-04 wires the D11-A2 trust staircase via the new interface.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/skillforge/skillforge/internal/agent/hitl"
	agenttools "github.com/skillforge/skillforge/internal/agent/tools"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/runtime"
	"github.com/skillforge/skillforge/internal/skill"
)

// RunnerCheckPointID is the canonical format for per-session Runner
// checkpoint IDs. Exported so internal/store/sqlite/checkpoint_test.go
// can pin it as a regression test (TestCheckpointIDFormat).
//
// Format: "sess:<sessionID>:cp" (D11-A1 + D11-C1). The colon prefix
// isolates SkillForge IDs from any future eino-internal ID namespace
// (eino uses random UUIDs by default; the explicit "sess:" prefix
// makes the namespace ownership unambiguous in audit logs).
func RunnerCheckPointID(sessionID string) string {
	return "sess:" + sessionID + ":cp"
}

// RunnerBuildConfig is the per-session production configuration for
// adk.NewRunner. Mirrors AgentConfig but switches the agent type
// from react.Agent (legacy Phase 3) to adk.ChatModelAgent
// (Phase 11).
//
// Per D11-B4: Agent is per-session singleton. Per D11-B1: Tools are
// re-pure-computed every construction (so Skill-A's HITL Tool cannot
// leak into Skill-B's session even via Runner cross-reuse).
//
// 11-01 ships the trust-staircase Callbacks field for forward-
// compatibility with the D11-A2 plan; 11-04 wires the actual
// BeforeAgent / BeforeModel / AfterModel handlers (the eino v0.9
// Handlers field takes []TypedChatModelAgentMiddleware[M], so the
// legacy callbacks.Handler shape needs an adapter — 11-04).
type RunnerBuildConfig struct {
	Manifest  *skill.Manifest
	Embedder  embedder.Embedder
	Retriever retriever.Retriever
	ChatModel model.ToolCallingChatModel
	MaxHops   int    // 0 → DefaultMaxHops (2)
	SkillID   string // for HITL registry target whitelist
	SessionID string // for checkPointID

	// RuleMatcher is the 11-05 / HITL-08 approve-and-remember lookup
	// injected into each whitelisted HITL Tool (D11-D1). Production
	// wires hitl.NewSqlRuleMatcher(store) at the server boundary;
	// nil degrades to hitl.NoopRuleMatcher (every write interrupts —
	// the fail-closed direction). Kept as an INTERFACE field (not a
	// store dependency) so BuildRunner stays pure and unit tests can
	// construct a Runner without a DB — see the BuildRunner doc.
	RuleMatcher hitl.RuleMatcher

	// CheckPointStore is the 11-04 / D11-C1 checkpoint persistence
	// binding. eino v0.9.14 exposes NO adk.Runner.WithCheckPointStore
	// setter — the store can only be attached via
	// adk.RunnerConfig.CheckPointStore at construction time, so it
	// travels through this config. nil → the Runner runs without
	// checkpoint persistence (unit-test posture; ResumeWithParams on
	// such a Runner fails fast with "store is nil"). The approval
	// endpoint (internal/server/approval_api.go) passes the session
	// store so the resume path can load the interrupt checkpoint.
	CheckPointStore adk.CheckPointStore

	// Handlers is the D11-A2 trust staircase (eino v0.9 Handler
	// interface slice, NOT the legacy callbacks.Handler). Reserved
	// for 11-04 wiring; 11-01 passes nil (the chatHandler boundary
	// still runs the trust floor / classifier / grounding checks
	// inline at their existing positions).
	Handlers []any
}

// BuildRunner constructs the per-session ChatModelAgent + adk.Runner.
// Returns a *adk.Runner ready for runner.Run(...) or
// runner.ResumeWithParams(...).
//
// Per D11-B4: the Agent is per-session singleton (returned Runner is
// reused across all turns of the session). Per D11-B1: the Tool
// whitelist is re-pure-computed every construction as
// hitlBuildToolsForSkillStub(ctx, manifest) — the 11-02 seam.
// Read-only Skills (writes:[]) get NO write Tools; write-Skills get
// ONLY the intersection of AllowedWriteTargets ∩ manifest.Writes.
//
// The CheckPointStore / CheckPointDeleter are bound by the CALLER
// via adk.Runner.WithCheckPointStore (added in the chatHandler
// boundary, internal/server/runner_chat.go). BuildRunner returns
// the runner pre-AI; the chatHandler attaches the store. This
// keeps BuildRunner pure (no store dependency in the agent
// construction seam) so unit tests can construct a Runner without
// a DB.
//
// CRITICAL: NO legacy interrupt API calls appear in this file.
// The HITL Tool (which uses tool.StatefulInterrupt) ships in 11-02;
// 11-01 only registers the empty placeholder via
// hitlBuildToolsForSkillStub.
func BuildRunner(ctx context.Context, cfg RunnerBuildConfig) (*adk.Runner, error) {
	if cfg.Manifest == nil {
		return nil, errors.New("agent.BuildRunner: Manifest is required")
	}
	if cfg.ChatModel == nil {
		return nil, errors.New("agent.BuildRunner: ChatModel is required")
	}
	if cfg.Embedder == nil {
		return nil, errors.New("agent.BuildRunner: Embedder is required")
	}
	if cfg.Retriever == nil {
		return nil, errors.New("agent.BuildRunner: Retriever is required")
	}
	if cfg.SessionID == "" {
		return nil, errors.New("agent.BuildRunner: SessionID is required")
	}
	if cfg.MaxHops <= 0 {
		cfg.MaxHops = DefaultMaxHops
	}

	// D11-B1 per-turn Tool whitelist. For Skills with writes:[] this
	// returns an empty slice (the LLM never sees a write Tool
	// schema). The function lives in this same package (agent) to
	// break the skill<->hitl import cycle — see hitl_tools.go's
	// file-level comment for the architectural rationale.
	//
	// 11-05: the per-session context (SessionID, SkillID, RuleMatcher)
	// travels with the whitelist computation so each HITL Tool's
	// D11-D1 rule lookup is scoped to THIS session + Skill. nil
	// RuleMatcher → NoopRuleMatcher (fail-closed: every write
	// interrupts).
	ruleMatcher := cfg.RuleMatcher
	if ruleMatcher == nil {
		ruleMatcher = hitl.NoopRuleMatcher{}
	}
	tools, err := BuildToolsForSkill(ctx, cfg.Manifest, cfg.SessionID, cfg.SkillID, ruleMatcher)
	if err != nil {
		return nil, fmt.Errorf("agent.BuildRunner: BuildToolsForSkill: %w", err)
	}

	// Always include the read-only retrieve Tool (it is not a write
	// Tool and is not gated by the writes: frontmatter). The Tool
	// closes over the manifest's resolved tier slice (KNOW-03 — the
	// LLM cannot override tiers via Tool arguments).
	retrieveTool := agenttools.BuildRetrieveTool(cfg.Embedder, cfg.Retriever, cfg.Manifest)
	tools = append(tools, retrieveTool)

	sysPrompt := SystemPrompt(cfg.Manifest)

	// P12-L1: archetype dispatch funnels through runtime.DispatchAgent.
	// The chat archetype (Phase 11 default; manifest.Agent empty or
	// "chat") constructs the same adk.NewChatModelAgent as before —
	// just lifted into the runtime package. The deep archetype
	// (Phase 12; manifest.Agent "deep") routes through
	// runtime.BuildDeepAgent (P12-L2 MaxIteration: 10 hard cap +
	// WithoutGeneralSubAgent: true).
	//
	// D11-B1 / D11-B4 / D11-C4 invariants are preserved: BuildToolsForSkill
	// is called here, the Tools slice is pre-computed and passed via
	// DispatchConfig.Tools, and the per-session identity (SessionID) is
	// propagated unchanged. The dispatch itself does NOT re-compute
	// tools (no drift risk).
	dispatchedAgent, err := runtime.DispatchAgent(ctx, runtime.DispatchConfig{
		Manifest:    cfg.Manifest,
		ChatModel:   cfg.ChatModel,
		Embedder:    cfg.Embedder,
		Retriever:   cfg.Retriever,
		SessionID:   cfg.SessionID,
		Tools:       tools,
		Instruction: sysPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("agent.BuildRunner: runtime.DispatchAgent: %w", err)
	}

	return adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           dispatchedAgent,
		EnableStreaming: true,
		CheckPointStore: cfg.CheckPointStore,
	}), nil
}
