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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// stubChatModel satisfies model.ToolCallingChatModel so DispatchAgent
// can build the underlying ChatModelAgent. We never invoke it in these
// unit tests — the goal is construction + dispatch behavior — so the
// methods just return zero values that compile against the eino v0.9.14
// interface signature.
type stubChatModel struct{}

// Generate satisfies model.BaseChatModel[model.ToolCallingChatModel].
// Empty body — tests do not exercise the LLM.
func (stubChatModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: ""}, nil
}

// Stream satisfies model.BaseChatModel.
func (stubChatModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{}), nil
}

// WithTools satisfies model.ToolCallingChatModel — must return the
// receiver unchanged because the test stub does not need a tools-aware
// chat model. The real ChatModelAgent never invokes WithTools during
// construction (it captures the tools via ToolsConfig).
func (s stubChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

// TestDispatchAgent_RequiresManifest locks the cfg.Manifest nil-guard.
func TestDispatchAgent_RequiresManifest(t *testing.T) {
	_, err := DispatchAgent(context.Background(), DispatchConfig{
		ChatModel: nil, // ensures Manifest guard runs before ChatModel guard
		SessionID: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "Manifest is required") {
		t.Fatalf("expected Manifest-required error, got %v", err)
	}
}

// TestDispatchAgent_RequiresChatModel locks the cfg.ChatModel nil-guard.
func TestDispatchAgent_RequiresChatModel(t *testing.T) {
	_, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "chat"},
		SessionID: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "ChatModel is required") {
		t.Fatalf("expected ChatModel-required error, got %v", err)
	}
}

// TestDispatchAgent_RequiresSessionID locks the cfg.SessionID empty-guard.
func TestDispatchAgent_RequiresSessionID(t *testing.T) {
	_, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "chat"},
		ChatModel: stubChatModel{},
	})
	if err == nil || !strings.Contains(err.Error(), "SessionID is required") {
		t.Fatalf("expected SessionID-required error, got %v", err)
	}
}

// TestDispatchAgent_UnknownArchetype returns the "unknown agent type"
// error string that the manifest validator message aligns with.
func TestDispatchAgent_UnknownArchetype(t *testing.T) {
	_, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "bogus-archetype"},
		ChatModel: stubChatModel{},
		SessionID: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown agent type") {
		t.Fatalf("expected unknown-agent-type error, got %v", err)
	}
}

// TestDispatchAgent_Chat locks the chat archetype construction.
func TestDispatchAgent_Chat(t *testing.T) {
	agent, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: "chat"},
		ChatModel: stubChatModel{},
		SessionID: "sess-1",
		Tools:     []tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("DispatchAgent(chat): %v", err)
	}
	if agent == nil {
		t.Fatalf("DispatchAgent(chat) returned nil agent")
	}
	// Sanity: chat path's Name() should reflect the session ID.
	if name := agent.Name(context.Background()); name == "" {
		t.Fatalf("DispatchAgent(chat) returned agent with empty name")
	}
}

// TestDispatchAgent_EmptyAgent_DefaultsToChat mirrors Phase 11 behavior:
// empty agent frontmatter means the legacy ChatModelAgent path.
func TestDispatchAgent_EmptyAgent_DefaultsToChat(t *testing.T) {
	agent, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  &skill.Manifest{Name: "compliance", Agent: ""},
		ChatModel: stubChatModel{},
		SessionID: "sess-1",
		Tools:     []tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("DispatchAgent(empty): %v", err)
	}
	if agent == nil {
		t.Fatalf("DispatchAgent(empty) returned nil")
	}
}

// TestArchetypeAccessor covers the string switch with all valid forms.
func TestArchetypeAccessor(t *testing.T) {
	cases := []struct {
		agent string
		want  AgentArchetype
	}{
		{"chat", ArchetypeChat},
		{"", ArchetypeChat},
		{"deep", ArchetypeDeep},
		{"orchestrator", ArchetypeOrchestrator},
		{"bogus", AgentArchetype("")},
	}
	for _, tc := range cases {
		got := DispatchConfig{Manifest: &skill.Manifest{Agent: tc.agent}}.Archetype()
		if got != tc.want {
			t.Errorf("Archetype(%q) = %q, want %q", tc.agent, got, tc.want)
		}
	}
	// nil manifest → empty (defensive).
	if (DispatchConfig{}).Archetype() != AgentArchetype("") {
		t.Errorf("nil-manifest Archetype() should be empty")
	}
}

// TestDispatchAgent_Orchestrator — Phase 13 / P13-L1: Manifest.Agent
// == "orchestrator" routes through buildOrchestrator → NewOrchestrator
// and returns a *FanOutOrchestrator (single-vertex, NOT ResumableAgent
// in 13-01).
func TestDispatchAgent_Orchestrator(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}
	a, err := DispatchAgent(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-orch",
	})
	if err != nil {
		t.Fatalf("DispatchAgent(orchestrator): %v", err)
	}
	if a == nil {
		t.Fatalf("DispatchAgent(orchestrator) returned nil")
	}
	if _, ok := a.(*FanOutOrchestrator); !ok {
		t.Fatalf("DispatchAgent(orchestrator) returned %T, want *FanOutOrchestrator", a)
	}
}

// silence unused-import warnings if model is otherwise consumed.
var _ model.ToolCallingChatModel = stubChatModel{}

// TestDefaultSubAgentMergeDelegatesToProduction — 13-05 Rule-1
// regression lock. The defaultSubAgentMerge singleton wired into
// DispatchAgent's orchestrator path MUST forward to the production
// mergeSubAgentResults (FAN-03 pure function). 13-01 shipped a
// passthrough returning an empty MergedVerdict and the promised 13-02
// replacement never re-pointed the call site — every orchestrator
// constructed via DispatchAgent (the only funnel available outside the
// runtime package) produced empty merged verdicts. This test fails if
// the delegation ever regresses to a passthrough.
func TestDefaultSubAgentMergeDelegatesToProduction(t *testing.T) {
	// Conservative partial-failure input: one faulted + one OK sub-agent.
	// mergeSubAgentResults MUST downgrade to needs_input + confidence 0
	// (P13-L5 / D2-14 posture); the old passthrough returned Status ""
	// and Confidence 0 with ReviewRequired false.
	merged := defaultSubAgentMerge([]SubAgentResult{
		{ID: "sa1", Status: SubAgentStatusOK, Confidence: 0.95},
		{ID: "sa2", Status: SubAgentStatusFaulted, FaultedReason: "retriever timeout"},
	})
	if merged.Status != verdict.DowngradeStatus {
		t.Fatalf("defaultSubAgentMerge conservative status = %q, want %q (P13-L5)", merged.Status, verdict.DowngradeStatus)
	}
	if merged.Confidence != 0 {
		t.Fatalf("defaultSubAgentMerge conservative confidence = %v, want 0", merged.Confidence)
	}
	if !merged.ReviewRequired {
		t.Fatal("defaultSubAgentMerge conservative ReviewRequired = false, want true")
	}

	// All-OK input: min-confidence aggregation + compliant status.
	okMerged := defaultSubAgentMerge([]SubAgentResult{
		{ID: "sa1", Status: SubAgentStatusOK, Confidence: 0.95},
		{ID: "sa2", Status: SubAgentStatusOK, Confidence: 0.80},
	})
	if okMerged.Status != verdict.CompliantStatus {
		t.Fatalf("defaultSubAgentMerge all-OK status = %q, want %q", okMerged.Status, verdict.CompliantStatus)
	}
	if okMerged.Confidence != 0.80 {
		t.Fatalf("defaultSubAgentMerge all-OK confidence = %v, want 0.80 (min aggregation)", okMerged.Confidence)
	}
	if !okMerged.ReviewRequired {
		t.Fatal("defaultSubAgentMerge all-OK min=0.80 < MergeConfidenceFloor → ReviewRequired want true")
	}
}
