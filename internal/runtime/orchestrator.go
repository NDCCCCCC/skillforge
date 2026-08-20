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

// P13-L8 sealed sentinel:
//
//	DO NOT introduce `adk.NewParallelAgent` calls anywhere in this
//	package. Eino's NewParallelAgent shares the parent's
//	runSession.Values map (verified at adk/runctx.go:478-508 +
//	forkRunCtx at lines 489-491 — Values are SHARED). That sharing
//	directly conflicts with FAN-02's structural tier-closure
//	invariant: a sub-agent built via NewParallelAgent could read
//	parent's session values that encode tier hints or future
//	fields. Phase 13 ships Option B (manual errgroup + per-Run
//	agent construction) precisely to AVOID this fan-out shape.
//	Use runtime.BuildSubAgent + the 5-step pipeline in Run().
//	The TestNoNewParallelAgentUsage source-grep guard is in 13-04's
//	hermetic phase.
//
// Package runtime — orchestrator.go is Phase 13 / Plan 13-01's
// FanOutOrchestrator agent type. It extends the P12-L1 dispatch
// surface with a THIRD archetype ("orchestrator") that owns the
// 5-step fan-out pipeline:
//
//	Step 1: Classify (LLM call — fills in 13-03)
//	Step 2: Build   (construct sub-agents via runtime.BuildSubAgent — fills in 13-03)
//	Step 3: Fan-out (errgroup with SetLimit(4) — fills in 13-03)
//	Step 4: Merge   (key-sorted pure function — fills in 13-03 + 13-02)
//	Step 5: Apply Trust Stage (reuses runtime.ApplyTrustStage — fills in 13-03)
//
// 13-01 ships the SKELETON: NewOrchestrator constructor, the
// adk.Agent interface (Name/Description/Run), and the structural
// constants. The per-step bodies land in 13-03.
//
// Why a NEW archetype (vs a special Tool on DeepAgent)?
// Phase 12 P12-L2 set WithoutGeneralSubAgent: true to prevent
// sub-agent injection. Re-enabling it for Phase 13 would require
// DeepAgent's auto-injected task tool to enforce tier-closure +
// audit RunPath + per-sub-agent grounding + structural read-only —
// too many cross-cutting concerns packed into one tool call. A
// dedicated FanOutOrchestrator makes the 5-step pipeline explicit,
// testable, auditable.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/skill"
)

// ArchetypeOrchestrator is the Phase 13 / P13-L1 third archetype
// dispatched by runtime.DispatchAgent. Maps to runtime.NewOrchestrator
// (a NEW *FanOutOrchestrator instance per-Run).
//
// Recursion guard (P13-L8): runtime.BuildSubAgent does NOT route
// through the orchestrator case — sub-agents always dispatch to
// chat or deep. Adding a recursive orchestrator inside an
// orchestrator is structurally impossible.
const ArchetypeOrchestrator AgentArchetype = "orchestrator"

// SubAgentDecision is the per-event envelope emitted by the
// orchestrator's emitter seam. The wire payload lands in 13-03
// (D16 overload on data-decision.decision enum with
// fanout_dispatched | fanout_completed | fanout_failed values).
type SubAgentDecision struct {
	RunID       string
	RunPath     string
	TransferFrom string
	TransferTo   string
	Event       string // "fanout_dispatched" | "fanout_completed" | "fanout_failed"
}

// SubAgentEmitter is the SSE emitter seam the orchestrator uses to
// publish per-sub-agent events. 13-03 wires the production emitter;
// 13-01 declares the interface so the orchestrator compiles.
type SubAgentEmitter interface {
	EmitFanoutDecision(ctx context.Context, decision SubAgentDecision) error
}

// SubAgentMergeFunc is the merge pure function signature (FAN-03
// contract). 13-02 supplies the production merge.go implementation
// in merge.go. The orchestrator holds a func-typed reference so 13-01
// can declare the seam without an import cycle.
type SubAgentMergeFunc func(results []SubAgentResult) MergedVerdict

// (SubAgentResult + MergedVerdict types live in merge.go — moved from
// this file in 13-02 so the merge pure function owns the type
// definitions alongside the byte-identity contract test. The merge
// types reuse verdict.Verdict / verdict.Refusal / verdict.Citation
// per FAN-04 reuse — NOT new types.)

// FanOutOrchestrator is the per-Run orchestrator instance. Mirrors
// the *chatmodel.TypedChatModelAgent[*schema.Message] surface area
// (Name/Description/Run) but owns the 5-step fan-out pipeline instead
// of a single LLM loop. Recursion is structurally impossible (P13-L8
// — BuildSubAgent never routes through the orchestrator case).
//
// 13-01 ships the skeleton. The Step 1..5 bodies land in 13-03.
// The merge function (Step 4) is supplied in 13-02; the orchestrator
// holds a SubAgentMergeFunc reference and calls it at the seam.
type FanOutOrchestrator struct {
	name                string
	description         string
	parentManifest      *skill.Manifest
	classifierChatModel model.ToolCallingChatModel
	subAgentFactory     SubAgentFactory
	mergeFunc           SubAgentMergeFunc
	budgetStore         SubAgentEmitter // re-uses the seam type (audit+emit dual purpose)
	emitter             SubAgentEmitter
	runID               string
	sessionID           string
	tools               []tool.BaseTool
	instruction         string
	timeoutPerSubAgent  time.Duration
}

// SubAgentFactory is the seam BuildSubAgent (sub_agent_builder.go)
// satisfies. Defining it as an interface lets tests inject a stub
// without re-wiring the orchestrator. The orchestrator DOES NOT
// import internal/runtime.BuildSubAgent directly — it calls the
// factory through this seam.
type SubAgentFactory interface {
	BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error)
}

// NewOrchestrator constructs the per-Run FanOutOrchestrator.
// Returns an error if any required field is nil/empty.
//
// The cfg.SubAgentFactory + cfg.MergeFunc are required (the
// orchestrator's behavior depends on them). They are passed via
// DispatchConfig-extended fields (NOT yet on the existing
// DispatchConfig — a follow-up patch in 13-03 widens the surface).
// 13-01 declares the constructor with explicit parameters so the
// seam compiles even before DispatchConfig is extended.
func NewOrchestrator(ctx context.Context, cfg DispatchConfig, factory SubAgentFactory, merge SubAgentMergeFunc) (*FanOutOrchestrator, error) {
	if cfg.Manifest == nil {
		return nil, errors.New("runtime.NewOrchestrator: Manifest is required")
	}
	if cfg.ChatModel == nil {
		return nil, errors.New("runtime.NewOrchestrator: classifier ChatModel is required")
	}
	if cfg.SessionID == "" {
		return nil, errors.New("runtime.NewOrchestrator: SessionID is required")
	}
	if factory == nil {
		return nil, errors.New("runtime.NewOrchestrator: SubAgentFactory is required (P13-L2 invariant)")
	}
	if merge == nil {
		return nil, errors.New("runtime.NewOrchestrator: SubAgentMergeFunc is required (FAN-03 invariant)")
	}
	if cfg.Manifest.Agent != string(ArchetypeOrchestrator) {
		return nil, fmt.Errorf("runtime.NewOrchestrator: Manifest.Agent must be %q for the orchestrator archetype, got %q", ArchetypeOrchestrator, cfg.Manifest.Agent)
	}

	return &FanOutOrchestrator{
		name:                "skillforge-orchestrator-" + cfg.SessionID,
		description:   fmt.Sprintf("SkillForge FanOutOrchestrator for skill %q (session %s)", cfg.Manifest.Name, cfg.SessionID),
		parentManifest:      cfg.Manifest,
		classifierChatModel: cfg.ChatModel,
		subAgentFactory:     factory,
		mergeFunc:           merge,
		runID:               cfg.SessionID,
		sessionID:           cfg.SessionID,
		tools:               cfg.Tools,
		instruction:         cfg.Instruction,
		timeoutPerSubAgent:  time.Duration(SubAgentTimeoutSeconds) * time.Second,
	}, nil
}

// Name returns the orchestrator's agent name. adk.Agent interface
// compliance — the eino Runner uses this for logging + audit.
func (o *FanOutOrchestrator) Name(_ context.Context) string {
	return o.name
}

// Description returns the orchestrator's human-readable description.
// adk.Agent interface compliance.
func (o *FanOutOrchestrator) Description(_ context.Context) string {
	return o.description
}

// Run is the orchestrator's entry point — the 5-step pipeline.
// 13-01 ships a SKELETON: Step 1 (Classify) + Step 2 (Build) are
// stubs that return an empty MergedVerdict so the dispatch path
// compiles. 13-02 wires the merge function (Step 4) into the stub
// so the orchestrator's Run invokes mergeSubAgentResults at the
// seam. 13-03 fills the actual LLM-driven classifier call,
// errgroup fan-out, and per-sub-agent ApplyTrustStage grounding.
//
// The Run signature MUST satisfy adk.Agent — TypedAgent[*schema.Message].Run
// (eino v0.9.14 adk/interface.go:453-464). The skeleton's return
// value is an AsyncIterator wrapping a single skeleton-message
// event so the eino Runner can drive the dispatch without errors.
//
// The skeleton emits ONE event then closes the generator so the
// eino Runner's iterator sees EOF after the skeleton. A goroutine
// runs the emit-then-close sequence so Run returns immediately (the
// eino Runner does NOT block on the orchestrator's completion).
//
// 13-02 wiring: the goroutine calls mergeSubAgentResults(nil) so
// the merge pure function is exercised on every Run call. The
// merged verdict is discarded (the skeleton does not consume it);
// 13-03 replaces this with the real 5-step pipeline where the
// merged verdict feeds Step 5 (ApplyTrustStage).
func (o *FanOutOrchestrator) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		// 13-02 stub: call the merge function with an empty result
		// slice so the seam is exercised on every Run. 13-03 wires
		// the real per-sub-agent results here.
		_ = o.mergeFunc(nil)

		// 13-01 skeleton contract: emit ONE event carrying the
		// placeholder text (so the eino Runner sees a non-zero
		// completion). The real pipeline emits SSE chunks
		// (data-decision, data-verdict, etc.) via the Emitter seam.
		gen.Send(&adk.AgentEvent{
			AgentName: o.name,
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					IsStreaming: false,
					Message: &schema.Message{
						Role:    schema.Assistant,
						Content: "FanOutOrchestrator skeleton — 13-03 will fill the 5-step pipeline (Classify → Build → Fan-out → Merge → ApplyTrustStage)",
					},
					Role: schema.Assistant,
				},
			},
		})
		// Close the generator so the iterator sees EOF. 13-03's
		// real pipeline closes only after the merge function
		// returns (Step 4 complete).
		gen.Close()
	}()

	// Step 1+2 skeleton: in 13-03, this becomes:
	//   subs := o.classify(ctx, input)
	//   for _, sub := range subs { go o.runSubAgent(ctx, sub, ...) }
	//   merged := o.mergeFunc(results)
	//   verdict := o.applyTrustStage(ctx, merged)
	// For 13-02 we just call mergeFunc(nil) so the seam is wired.
	_ = ctx
	_ = input
	_ = opts
	return iter
}

// Archetype returns ArchetypeOrchestrator. Convenience accessor for
// tests + future tooling that introspects the dispatch surface.
func (o *FanOutOrchestrator) Archetype() AgentArchetype {
	return ArchetypeOrchestrator
}

// ParentManifest exposes the parent manifest for tests + the
// BuildSubAgent seam (the factory needs the parent's ResolvedTiers()
// to compute the intersection). NOT exposed as a public field —
// callers should use the accessor so the field can change shape
// without breaking the public API.
func (o *FanOutOrchestrator) ParentManifest() *skill.Manifest {
	return o.parentManifest
}

// SubAgentFactory returns the orchestrator's SubAgentFactory. Used
// by 13-03's per-Run goroutines to construct sub-agents on demand.
func (o *FanOutOrchestrator) SubAgentFactory() SubAgentFactory {
	return o.subAgentFactory
}

// MergeFunc returns the orchestrator's SubAgentMergeFunc.
func (o *FanOutOrchestrator) MergeFunc() SubAgentMergeFunc {
	return o.mergeFunc
}

// SessionID returns the orchestrator's session ID.
func (o *FanOutOrchestrator) SessionID() string {
	return o.sessionID
}

// TimeoutPerSubAgent returns the per-sub-agent timeout (Pitfall 4).
func (o *FanOutOrchestrator) TimeoutPerSubAgent() time.Duration {
	return o.timeoutPerSubAgent
}

// Compile-time interface compliance check: FanOutOrchestrator
// MUST satisfy adk.Agent (eino v0.9.14 adk/interface.go:453).
var _ adk.Agent = (*FanOutOrchestrator)(nil)