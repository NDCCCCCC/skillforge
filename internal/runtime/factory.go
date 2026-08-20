// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0

// Package runtime owns the archetype dispatch layer for SkillForge's
// per-session agent construction. It is Phase 12 / Plan 12-01's
// foundational package — every Skill runtime path (chat / deep / future
// sub-agent archetypes) funnels through DispatchAgent here, and every
// final-message verification runs through ApplyTrustStage (the unified
// floor → grounding → critique exit hoisted from internal/verdict).
//
// Architectural boundary (P12-L1):
//
//   - internal/agent keeps Phase 11 ChatModelAgent construction +
//     HITL Tools + callbacks.
//   - internal/verdict is downgraded to a library of pure functions
//     (ApplyTrustFloor + ApplyGrounding + ApplyCritique) consumed by
//     internal/runtime.ApplyTrustStage.
//   - internal/runtime owns: archetype dispatch + ApplyTrustStage exit
//   - MaxIteration hard cap (P12-L2).
//
// This file declares the DispatchConfig + DispatchAgent entry point
// and the AgentArchetype enum. DeepAgent-specific construction lives
// in deep_agent.go; trust-stage hoisting lives in trust_stage.go;
// budget callbacks (P12-L9) live in budget.go.
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	agentcallbacks "github.com/skillforge/skillforge/internal/agent/callbacks"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// AgentArchetype is the exported enum-style type returned by
// DispatchConfig.Archetype(). The string values match the SKILL.md
// `agent:` frontmatter values Phase 12 accepts — see
// internal/skill/manifest.go validAgentTypes whitelist. Exported so
// 12-04 verification + future skill-wizard UI can refer to a typed
// constant instead of a string literal.
type AgentArchetype string

// ArchetypeChat is the Phase 11 ChatModelAgent path — single LLM +
// tool whitelist + D11-A2 trust staircase. Selected when manifest.Agent
// is empty or "chat". Maps to adk.NewChatModelAgent under the hood.
const ArchetypeChat AgentArchetype = "chat"

// ArchetypeDeep is the Phase 12 DeepAgent path — multi-iteration
// agentic reasoning + plan_todos (12-02) + plan_approve (12-03) +
// budget callbacks (12-05). Maps to eino/adk/prebuilt/deep.New under
// the hood.
const ArchetypeDeep AgentArchetype = "deep"

// ArchetypeOrchestrator is declared in orchestrator.go (Phase 13 /
// P13-L1) — the third archetype in the dispatch surface. Kept here
// as a forward reference comment so the dispatch table at the
// bottom of this file is the canonical read of all three archetypes.

// DispatchConfig is the per-session construction envelope passed to
// DispatchAgent. Mirrors the surface area of agent.RunnerBuildConfig
// without inheriting its server-only fields (RuleMatcher,
// CheckPointStore, Handlers) — those live at the chatHandler boundary
// and are NOT consumed by the runtime dispatch.
//
// Manifest, ChatModel, SessionID are REQUIRED for every archetype.
// Embedder + Retriever are REQUIRED for the chat archetype (the
// retrieve Tool closes over them) and IGNORED for the deep
// archetype's ToolsConfig slice (the deep archetype's Tools slice is
// already pre-built by the caller via BuildToolsForSkill +
// agenttools.BuildRetrieveTool).
//
// Instruction is the per-archetype system prompt. The caller
// (agent.BuildRunner) computes it via agent.SystemPrompt for the
// chat archetype and via runtime.renderDeepSystemPrompt for the deep
// archetype; passing it via DispatchConfig breaks the import cycle
// (internal/runtime cannot import internal/agent; the prompts live
// in internal/agent/prompts.go).
//
// Tools is the pre-computed per-turn whitelist (D11-B1 —
// BuildToolsForSkill in internal/agent) reused by every archetype;
// the dispatch does NOT recompute it. Declared as []tool.BaseTool
// (the interface Compose.ToolsNodeConfig.Tools consumes) so callers
// can pass InvokableTool (the BuildToolsForSkill return shape) —
// InvokableTool embeds BaseTool so the slice is assignable.
//
// BudgetStore + BudgetEmitter are the Phase 12 / Plan 12-05 wiring
// seams consumed ONLY by the deep archetype (BuildDeepAgent →
// NewBudgetCallbacks → agentcallbacks.NewBudgetCallback): the audit
// INSERT target for action_budget_triggered rows + the per-Run SSE
// emitter for the data-decision{decision:"forced_stop"} chunk. Both
// may be nil (tests / no-SSE path) — the gate still fires; only the
// durable row / live-stream surface is suppressed. The chat archetype
// ignores them.
type DispatchConfig struct {
	Manifest    *skill.Manifest
	ChatModel   model.ToolCallingChatModel
	Embedder    embedder.Embedder
	Retriever   retriever.Retriever
	SessionID   string
	Tools       []tool.BaseTool
	Instruction string // per-archetype system prompt (caller-supplied)

	BudgetStore   agentcallbacks.BudgetAuditStore // 12-05 — deep-archetype budget audit INSERT target
	BudgetEmitter agentcallbacks.BudgetEmitter    // 12-05 — deep-archetype forced_stop SSE emitter
}

// Archetype returns the archetype enum for cfg. Empty Agent field or
// "chat" returns ArchetypeChat (the Phase 11 default path); "deep"
// returns ArchetypeDeep; "orchestrator" returns ArchetypeOrchestrator
// (Phase 13 / P13-L1). Other values return empty AgentArchetype —
// callers should treat empty as "rejected by Validate" (manifest
// validator guards against unknown agent types at SKILL.md load time,
// but the dispatch layer is defense-in-depth).
func (c DispatchConfig) Archetype() AgentArchetype {
	if c.Manifest == nil {
		return AgentArchetype("")
	}
	switch c.Manifest.Agent {
	case "", "chat":
		return ArchetypeChat
	case "deep":
		return ArchetypeDeep
	case "orchestrator":
		return ArchetypeOrchestrator
	default:
		return AgentArchetype("")
	}
}

// DispatchAgent is the P12-L1 pure dispatcher. It maps cfg.Manifest.Agent
// to the per-archetype constructor (chat → ChatModelAgent; deep →
// DeepAgent; orchestrator → FanOutOrchestrator) and returns the
// constructed adk.Agent ready to hand to adk.NewRunner.
//
// Architectural contract:
//
//   - chat path → buildChatModelAgent (Phase 11 ChatModelAgent — the
//     previous adk.NewChatModelAgent call site, lifted into the runtime
//     package unchanged so agent.BuildRunner can delegate).
//   - deep path → buildDeepAgent → BuildDeepAgent (deep_agent.go —
//     P12-L2 MaxIteration: 10 hardcoded, WithoutWriteTodos: true,
//     WithoutGeneralSubAgent: true).
//   - orchestrator path → buildOrchestrator → NewOrchestrator
//     (orchestrator.go — Phase 13 / P13-L1; 5-step pipeline with
//     sub-agent tier-closure; adk.NewParallelAgent REJECTED per
//     P13-L8).
//   - unknown values → error with substring "unknown agent type".
//
// The dispatcher is PURE: no audit write, no SSE emit, no checkpoint
// write — those happen at the chatHandler boundary AFTER the runner
// is constructed. The dispatch is the single funnel all archetypes
// pass through; that funnel is the test-as-gate anchor for P12-L1 +
// P13-L1.
//
// The orchestrator archetype requires the BuildSubAgent factory +
// the merge function (FAN-03 invariant). These are wired via package
// singletons (BuildSubAgent + mergeSubAgentResults) — see
// buildOrchestrator below. 13-03 widens this seam for production
// wiring (audit store + emitter injection).
func DispatchAgent(ctx context.Context, cfg DispatchConfig) (adk.Agent, error) {
	if cfg.Manifest == nil {
		return nil, errors.New("runtime.DispatchAgent: Manifest is required")
	}
	if cfg.ChatModel == nil {
		return nil, errors.New("runtime.DispatchAgent: ChatModel is required")
	}
	if cfg.SessionID == "" {
		return nil, errors.New("runtime.DispatchAgent: SessionID is required")
	}

	switch cfg.Manifest.Agent {
	case "", "chat", "ChatModelAgent":
		// Phase 11 legacy alias — "ChatModelAgent" was the original
		// Eino ADK constructor name. SKILL.md frontmatter that has
		// not migrated to the short form ("chat") still works. The
		// dispatch layer maps all three to the same chat archetype.
		return buildChatModelAgent(ctx, cfg)
	case "deep":
		return buildDeepAgent(ctx, cfg)
	case "orchestrator":
		return buildOrchestrator(ctx, cfg)
	default:
		return nil, fmt.Errorf("runtime.DispatchAgent: unknown agent type %q (valid: chat, deep, orchestrator)", cfg.Manifest.Agent)
	}
}

// buildChatModelAgent is the lifted Phase 11 ChatModelAgent path.
// Behavior is byte-identical to the previous agent.BuildRunner's
// inline adk.NewChatModelAgent call — the only change is that it
// lives in the runtime package now so the dispatch site is the
// single funnel.
//
// Per-turn Tool whitelist (D11-B1) is computed by the caller (agent.
// BuildRunner) and passed in via cfg.Tools; this function does NOT
// recompute it. The Tool whitelist is the load-bearing invariant for
// HITL-05 (a read-only Skill cannot reach a write Tool); recomputing
// inside the factory would duplicate BuildToolsForSkill and create
// drift risk.
//
// Compose options: empty (Phase 11 / Plan 11-04 reserved the
// Handlers field for the D11-A2 trust staircase wiring; runtime
// delegates that to the chatHandler boundary).
func buildChatModelAgent(ctx context.Context, cfg DispatchConfig) (adk.Agent, error) {
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "skillforge-session-" + cfg.SessionID,
		Description: "SkillForge ChatModelAgent (Plan 12-01 runtime dispatcher; archetype=chat)",
		Instruction: cfg.Instruction,
		Model:       cfg.ChatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: cfg.Tools},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("runtime.buildChatModelAgent: NewChatModelAgent: %w", err)
	}
	return chatAgent, nil
}

// buildDeepAgent is the deep archetype entry point. Delegates to
// BuildDeepAgent (deep_agent.go) — the split keeps factory.go focused
// on dispatch logic and deep_agent.go focused on DeepAgent-specific
// config wiring (MaxIteration hard cap, WithoutWriteTodos, etc.).
func buildDeepAgent(ctx context.Context, cfg DispatchConfig) (adk.Agent, error) {
	deepAgent, err := BuildDeepAgent(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("runtime.buildDeepAgent: %w", err)
	}
	return deepAgent, nil
}

// buildOrchestrator is the Phase 13 / P13-L1 orchestrator archetype
// entry point. Delegates to NewOrchestrator (orchestrator.go) with
// the BuildSubAgent factory + merge function (FAN-03 invariant).
//
// 13-01 ships the dispatch path with the package-level factory +
// merge singletons wired inline; 13-03 widens this seam for
// production injection (audit store + emitter).
func buildOrchestrator(ctx context.Context, cfg DispatchConfig) (adk.Agent, error) {
	orch, err := NewOrchestrator(ctx, cfg, defaultSubAgentFactory{}, defaultSubAgentMerge)
	if err != nil {
		return nil, fmt.Errorf("runtime.buildOrchestrator: %w", err)
	}
	return orch, nil
}

// defaultSubAgentFactory is the package-level SubAgentFactory singleton
// wired into DispatchAgent for the orchestrator archetype. 13-01
// ships a stub that delegates to BuildSubAgent (the real implementation
// lands in 13-01 Task 3 / sub_agent_builder.go); 13-03 may widen this
// seam for production injection (audit store + emitter per-call).
type defaultSubAgentFactory struct{}

// BuildSubAgent satisfies runtime.SubAgentFactory. Delegates to the
// package-level BuildSubAgent function defined in sub_agent_builder.go.
// 13-01 ships this delegation; the tier-closure + read-only invariant
// lives in sub_agent_builder.go.
func (defaultSubAgentFactory) BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error) {
	return BuildSubAgent(ctx, cfg)
}

// defaultSubAgentMerge is the package-level SubAgentMergeFunc singleton.
// 13-01 ships a passthrough that returns an empty MergedVerdict; 13-02
// replaces it with the production merge function (FAN-03 10× byte-
// identity contract). Returning an empty verdict here keeps the
// orchestrator's Run skeleton from panicking when 13-03's 5-step
// pipeline hands results to the merge function.
func defaultSubAgentMerge(results []SubAgentResult) MergedVerdict {
	_ = results
	return MergedVerdict{}
}
