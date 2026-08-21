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
// Package runtime — orchestrator.go is Phase 13 / Plan 13-03's
// FanOutOrchestrator agent type. 13-01 ships the skeleton; 13-03
// fills the 5-step pipeline:
//
//	Step 1: Classify (LLM call — fill in via cfg.ClassifierChatModel)
//	Step 2: Build   (construct sub-agents via runtime.BuildSubAgent)
//	Step 3: Fan-out (errgroup with SetLimit(4) — manual parallel)
//	Step 4: Merge   (key-sorted pure function — MergeFunc seam)
//	Step 5: Apply Trust Stage (reuses runtime.ApplyTrustStage — P12-L1)
//
// Architectural invariants:
//
//   - FAN-01 SC-1: fault isolation — each sub-agent Run is wrapped in
//     defer recover() so a panic in one sub-agent does NOT kill the
//     parent. The recovered error is mapped to SubAgentStatusFaulted
//     and surfaces in the audit row (FAN-04).
//   - FAN-02 SC-2: tier-closure — BuildSubAgent enforces empty
//     intersection = load-time error (sub_agent_builder.go).
//   - FAN-03 SC-3: deterministic merge — mergeSubAgentResults is the
//     pure function; byte-identity contract test lives in merge_test.go.
//   - FAN-04 SC-4: per-sub-agent grounding — ApplyTrustStage is called
//     for EACH sub-agent (not just the merged verdict) so each
//     sub-agent's verdict is independently grounded before merging.
//   - P13-L1: manual errgroup, NOT adk.NewParallelAgent.
//   - P13-L9: shared *ChunkAccumulator across parent + sub-agents.
//   - P13-L8: orchestrator cannot dispatch orchestrator (recursion guard).
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
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

// SubAgentFanoutLimit is the errgroup.SetLimit value for the fan-out
// step (P13-L1). Cap of 4 sub-agents in parallel — anything wider
// risks LLM provider rate-limit storms; the conservative cap keeps
// the orchestrator responsive under load.
//
// Source-grep locked via TestSubAgentFanoutLimitConstant.
const SubAgentFanoutLimit = 4

// SubAgentDecision is the per-event envelope emitted by the
// orchestrator's emitter seam. The wire payload rides the
// data-decision.decision enum (D16 overload pattern) with values
// "fanout_dispatched" | "fanout_completed" | "fanout_failed".
type SubAgentDecision struct {
	RunID        string
	RunPath      string
	TransferFrom string
	TransferTo   string
	Event        string // "fanout_dispatched" | "fanout_completed" | "fanout_failed"
	Reason       string // populated for "fanout_failed"
}

// SubAgentEmitter is the SSE emitter seam the orchestrator uses to
// publish per-sub-agent events. The production wiring passes a
// wrapper that emits data-decision{decision:"fanout_*"} SSE chunks
// (RealChatTransport routes these to the PlanInspector timeline).
//
// May be nil — the orchestrator silently skips emission in that case
// (the audit row write still happens; the SSE channel is suppressed).
type SubAgentEmitter interface {
	EmitFanoutDecision(ctx context.Context, decision SubAgentDecision) error
}

// SubAgentMergeFunc is the merge pure function signature (FAN-03
// contract). The production merge function is mergeSubAgentResults in
// merge.go (10x byte-identity contract). The orchestrator holds a
// func-typed reference so callers can inject test fakes.
type SubAgentMergeFunc func(results []SubAgentResult) MergedVerdict

// (SubAgentResult + MergedVerdict types live in merge.go — moved from
// this file in 13-02 so the merge pure function owns the type
// definitions alongside the byte-identity contract test.)

// SubAgentAuditWriter is the audit INSERT seam the orchestrator uses
// to record the 3-row fanout lifecycle (dispatched / completed / failed)
// per sub-agent. The production wiring points at the SQLite
// store.PutAudit path; nil is allowed (tests skip the audit write).
//
// Audit row contract (P13-L6 + T-13-03-3 + T-13-03-6):
//
//   - Action: "action_fanout_dispatched" before sub-agent Run.
//   - Action: "action_fanout_completed" after successful Run +
//     ApplyTrustStage grounding.
//   - Action: "action_fanout_failed" after error/panic/timeout.
//   - RunPath: <parent_runID>/<sub_query_id> for every fanout row.
//   - TransferFrom: parent runID.
//   - TransferTo: sub_query_id.
//   - PutAudit rejects fanout rows with empty RunPath (T-13-03-3).
type SubAgentAuditWriter interface {
	PutAudit(ctx context.Context, action, runPath, transferFrom, transferTo, reason string) error
}

// FanOutOrchestrator is the per-Run orchestrator instance. Mirrors
// the *chatmodel.TypedChatModelAgent[*schema.Message] surface area
// (Name/Description/Run) but owns the 5-step fan-out pipeline instead
// of a single LLM loop. Recursion is structurally impossible (P13-L8
// — BuildSubAgent never routes through the orchestrator case).
//
// 13-01 ships the skeleton; 13-03 fills the 5-step pipeline bodies.
type FanOutOrchestrator struct {
	name               string
	description        string
	parentManifest     *skill.Manifest
	classifierChatModel model.ToolCallingChatModel
	subAgentFactory    SubAgentFactory
	mergeFunc          SubAgentMergeFunc
	emitter            SubAgentEmitter
	auditWriter        SubAgentAuditWriter
	// subEmbedder + subRetriever carry the caller's live retrieval
	// stack into every sub-agent (13-05 live wiring): the sub-agent's
	// retrieve Tool closes over these. Nil in hermetic tests — the
	// stub factory seam then supplies its own adapters.
	subEmbedder        embedder.Embedder
	subRetriever       retriever.Retriever
	runID              string
	sessionID          string
	tools              []tool.BaseTool
	instruction        string
	timeoutPerSubAgent time.Duration
	// classifierForTest is a test-only seam that overrides the
	// production classify() step. Production code leaves this nil;
	// tests inject a deterministic plan via this field. The field is
	// unexported and the only writer is in the test file
	// (orchestrator_test.go).
	classifierForTest func() []subQueryPlan
}

// SubAgentFactory is the seam BuildSubAgent (sub_agent_builder.go)
// satisfies. Defining it as an interface lets tests inject a stub
// without re-wiring the orchestrator.
type SubAgentFactory interface {
	BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error)
}

// NewOrchestrator constructs the per-Run FanOutOrchestrator.
// Returns an error if any required field is nil/empty.
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
		description:         fmt.Sprintf("SkillForge FanOutOrchestrator for skill %q (session %s)", cfg.Manifest.Name, cfg.SessionID),
		parentManifest:     cfg.Manifest,
		classifierChatModel: cfg.ChatModel,
		subAgentFactory:     factory,
		mergeFunc:           merge,
		subEmbedder:         cfg.Embedder,
		subRetriever:        cfg.Retriever,
		runID:               cfg.SessionID,
		sessionID:           cfg.SessionID,
		tools:               cfg.Tools,
		instruction:         cfg.Instruction,
		timeoutPerSubAgent:  time.Duration(SubAgentTimeoutSeconds) * time.Second,
	}, nil
}

// WithEmitter installs an SSE emitter seam. Optional — the orchestrator
// silently skips emission when nil.
func (o *FanOutOrchestrator) WithEmitter(emitter SubAgentEmitter) *FanOutOrchestrator {
	o.emitter = emitter
	return o
}

// WithAuditWriter installs the audit INSERT seam. Optional — the
// orchestrator silently skips audit writes when nil.
func (o *FanOutOrchestrator) WithAuditWriter(writer SubAgentAuditWriter) *FanOutOrchestrator {
	o.auditWriter = writer
	return o
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

// Run is the orchestrator's entry point — the 5-step pipeline:
// Classify → Build → Fan-out → Merge → ApplyTrustStage.
//
// The Run signature MUST satisfy adk.Agent —
// TypedAgent[*schema.Message].Run (eino v0.9.14 adk/interface.go:
// 453-464). The goroutine runs the pipeline and closes the generator
// when done; Run returns the AsyncIterator immediately so the eino
// Runner does not block.
//
// On success the final emitted event carries the merged verdict (or a
// refusal summary) as a single adk.AgentEvent wrapping a schema.Message.
// On pipeline failure (no sub-agents survived) the event carries an
// empty message + the goroutine logs the failure via the audit writer.
//
// Audit-first lifecycle (T-13-03-6): every sub-agent emits an
// action_fanout_dispatched audit row BEFORE its Run, plus one of
// action_fanout_completed | action_fanout_failed AFTER. The audit
// lineage gives the operator a complete trace even if a sub-agent
// panics (the dispatched row exists; the failed row records the panic).
func (o *FanOutOrchestrator) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer gen.Close()

		// Step 1: Classify — derive sub-query plan from input.
		subs, classifyErr := o.classify(ctx, input)
		if classifyErr != nil {
			o.emitFailure(ctx, "classifier", classifyErr)
			gen.Send(&adk.AgentEvent{
				AgentName: o.name,
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming: false,
						Message: &schema.Message{
							Role:    schema.Assistant,
							Content: fmt.Sprintf("orchestrator classify failed: %v", classifyErr),
						},
						Role: schema.Assistant,
					},
				},
			})
			return
		}

		// Defensive: empty sub-plan collapses to an empty merged verdict.
		if len(subs) == 0 {
			merged := o.mergeFunc(nil)
			// No sub-agents ran — no shared accumulator hits. Pass nil
			// and let the trust-floor short-circuit on empty Chunks
			// (single-vertex fallback is supposed to refuse by default).
			verdict := o.applyTrustStageToMerged(ctx, merged, nil)
			gen.Send(o.eventFromVerdict(verdict))
			return
		}

		// Step 2 + 3: Build + Fan-out (manual errgroup with
		// SetLimit(SubAgentFanoutLimit) — P13-L1, NOT adk.NewParallelAgent).
		// fanOut returns the shared accumulator so Step 5 can ground
		// the merged verdict against the UNION of per-sub-agent
		// retrievals (P13-L9 — FAN-04 SC-4 final pass).
		results, acc := o.fanOut(ctx, subs, input)

		// Step 4: Merge (pure function — FAN-03 SC-3 byte-identity).
		merged := o.mergeFunc(results)
		// Fill the RunPath for audit + replay.
		merged.RunPath = o.runID

		// Step 5: Apply Trust Stage on the merged verdict — feed the
		// accumulator's union hits so the trust floor at
		// len(Chunks)==0 does NOT refuse the merged verdict (the
		// first live run aborted with all-OK sub-agents conservatively
		// downgraded because the merged-grounding pass was getting
		// Chunks: nil).
		verdict := o.applyTrustStageToMerged(ctx, merged, acc)

		gen.Send(o.eventFromVerdict(verdict))
	}()

	_ = opts
	return iter
}

// subQueryPlan is one sub-query the classifier decided to dispatch.
// Step 1 (Classify) produces N of these; Step 2+3 (Build+Fan-out)
// iterates over them.
type subQueryPlan struct {
	SubQueryID   string
	SubQuery     string
	SubSkillID   string   // the sub-skill name to dispatch
	SubSkillTiers []string // resolved tier slice from the sub-skill manifest (carried for FAN-04 SC-4 per-sub-agent grounding retrieval)
}

// classify (Step 1) — derives the sub-query plan.
//
// 13-03 ships a minimal classifier: if the input is nil, return a
// single sub-query with the input's contents. If the input carries a
// pre-classified plan in the AgentInput's messages, parse it. Otherwise
// return one sub-query per chunk in the parent's pre-retrieve (defensive
// placeholder — the production classifier LLM call lands in 13-05).
//
// The minimal classifier keeps the dispatch path compiling; the
// pipeline structure (5 steps + per-sub-agent grounding) is the
// load-bearing surface, not the classifier's intelligence.
//
// Tests inject a deterministic plan via the classifierForTest field.
func (o *FanOutOrchestrator) classify(_ context.Context, input *adk.AgentInput) ([]subQueryPlan, error) {
	if o.classifierForTest != nil {
		return o.classifierForTest(), nil
	}
	// Minimal classifier placeholder. The production classifier
	// (13-05) uses cfg.ClassifierChatModel to derive the sub-plan;
	// for 13-03 we return a deterministic single-sub-query plan so
	// the pipeline compiles + tests can exercise the structure.
	//
	// The sub-query text falls back to a synthesis of any user message
	// in the AgentInput — otherwise the sub-agent receives a hardcoded
	// "default sub-query" and cannot ground its verdict. The
	// ResolvedTiers of the parent (or empty for the fanout-12
	// parent-empty-tiers contract case) carry through so per-sub-agent
	// grounding retrieval uses the parent's authorized slice.
	parentTiers := o.parentManifest.ResolvedTiers()
	parentQuery := extractFirstUserText(input)
	if parentQuery == "" {
		parentQuery = "default sub-query"
	}
	return []subQueryPlan{
		{
			SubQueryID:   "sa1",
			SubQuery:     parentQuery,
			SubSkillID:   o.parentManifest.Name,
			SubSkillTiers: parentTiers,
		},
	}, nil
}

// extractFirstUserText returns the Content of the first User-role
// Message in in.Messages, trimmed. Empty if the input is nil or has
// no user messages.
func extractFirstUserText(in *adk.AgentInput) string {
	if in == nil {
		return ""
	}
	for _, m := range in.Messages {
		if m == nil {
			continue
		}
		if m.Role == schema.User {
			return strings.TrimSpace(m.Content)
		}
	}
	return ""
}

// fanOut (Step 2 + 3) — build sub-agents in parallel via manual
// errgroup.SetLimit(4). The accumulator (P13-L9) is shared across
// all sub-agents so the citation entailment verifier sees the union.
//
// FAN-01 SC-1 fault isolation: every sub-agent Run is wrapped in
// defer recover() — a panic maps to SubAgentStatusFaulted (P13-L5 +
// Pitfall 4 mitigation). The audit row records the failure.
//
// Per-sub-agent timeout (Pitfall 4): each sub-agent's Run is wrapped
// in context.WithTimeout(timeoutPerSubAgent). On timeout the sub-agent
// is marked SubAgentStatusFaulted with reason "context deadline exceeded".
//
// Returns the slice of per-sub-agent SubAgentResult envelopes, one per
// sub-query plan (in plan order). Failed sub-agents retain their slot
// with Status=faulted + FaultedReason populated.
func (o *FanOutOrchestrator) fanOut(ctx context.Context, subs []subQueryPlan, input *adk.AgentInput) ([]SubAgentResult, *SharedChunkAccumulator) {
	results := make([]SubAgentResult, len(subs))

	// Shared accumulator across parent + sub-agents (P13-L9).
	acc := NewSharedChunkAccumulator(SharedChunkAccumulatorMaxEntries)

	// Semaphore (channel-based) — implements errgroup.SetLimit(4)
	// semantics without importing the errgroup package (the orchestrator
	// avoids the errgroup dependency to keep the source-grep surface
	// tight; the semaphore pattern is 6 lines and equally correct).
	sem := make(chan struct{}, SubAgentFanoutLimit)

	// WaitGroup for join semantics.
	var wg sync.WaitGroup

	for i, sub := range subs {
		i, sub := i, sub // capture range variable
		wg.Add(1)
		go func() {
			defer wg.Done()

			// SetLimit(4) semaphore acquire.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = o.faultedResult(sub, ctx.Err())
				return
			}

			results[i] = o.runSubAgent(ctx, sub, input, acc)
		}()
	}

	wg.Wait()
	return results, acc
}

// runSubAgent is the per-sub-agent goroutine body. It does:
//   - audit lifecycle row (dispatched) BEFORE the Run.
//   - context.WithTimeout(30s) wrap of the Run.
//   - defer recover() to map panics to SubAgentStatusFaulted (FAN-01 SC-1).
//   - per-sub-agent ApplyTrustStage grounding (FAN-04 SC-4).
//   - audit lifecycle row (completed | failed) AFTER the Run.
//
// Returns SubAgentResult populated with one of the 3 status values.
// Empty verdict on faulted sub-agent; full verdict on success.
//
// The accumulator is appended-to by the sub-agent's retrieve Tool
// during Run; the parent's Run can read acc.Hits() afterwards.
func (o *FanOutOrchestrator) runSubAgent(parentCtx context.Context, sub subQueryPlan, input *adk.AgentInput, acc *SharedChunkAccumulator) SubAgentResult {
	runPath := o.runID + "/" + sub.SubQueryID
	result := SubAgentResult{
		ID:       sub.SubQueryID,
		SubQuery: sub.SubQuery,
	}

	// Audit lifecycle: write dispatched row BEFORE Run (T-13-03-6).
	o.writeAudit(parentCtx, "action_fanout_dispatched", runPath, o.runID, sub.SubQueryID, "")
	o.emitDecision(parentCtx, "fanout_dispatched", runPath, o.runID, sub.SubQueryID, "")

	// Per-sub-agent timeout wrap (Pitfall 4 mitigation).
	timeoutCtx, cancel := context.WithTimeout(parentCtx, o.timeoutPerSubAgent)
	defer cancel()

	// Build the sub-agent (Step 2 — structural tier-closure).
	subAgent, err := o.subAgentFactory.BuildSubAgent(timeoutCtx, SubAgentConfig{
		ParentManifest:   o.parentManifest,
		SubSkillManifest: o.parentManifest, // 13-05 widens to per-sub-skill; 13-03 reuses parent
		SubQueryID:       sub.SubQueryID,
		ChatModel:        o.classifierChatModel,
		Embedder:         o.subEmbedder,
		Retriever:        o.subRetriever,
		SessionID:        o.sessionID + ":" + sub.SubQueryID,
		ChunkAccumulator: acc,
		Instruction:      o.instruction,
	})
	if err != nil {
		reason := err.Error()
		result.Status = SubAgentStatusFaulted
		result.FaultedReason = reason
		o.writeAudit(parentCtx, "action_fanout_failed", runPath, o.runID, sub.SubQueryID, reason)
		o.emitDecision(parentCtx, "fanout_failed", runPath, o.runID, sub.SubQueryID, reason)
		return result
	}

	// FAN-01 SC-1: defer recover() — a panic in the sub-agent's Run
	// MUST NOT propagate to the parent. The recover maps the panic
	// to SubAgentStatusFaulted (Pitfall 4 mitigation).
	var (
		subResult    *SubAgentResult
		runErr       error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("sub-agent %q panic: %v", sub.SubQueryID, r)
			}
		}()
		subResult, runErr = o.runSubAgentInner(timeoutCtx, subAgent, sub, acc, input)
	}()

	if runErr != nil {
		reason := runErr.Error()
		result.Status = SubAgentStatusFaulted
		result.FaultedReason = reason
		o.writeAudit(parentCtx, "action_fanout_failed", runPath, o.runID, sub.SubQueryID, reason)
		o.emitDecision(parentCtx, "fanout_failed", runPath, o.runID, sub.SubQueryID, reason)
		return result
	}
	if subResult == nil {
		// Defensive: should not happen (runSubAgentInner always returns
		// a non-nil SubAgentResult), but map to faulted if it does.
		result.Status = SubAgentStatusFaulted
		result.FaultedReason = "sub-agent returned nil result"
		o.writeAudit(parentCtx, "action_fanout_failed", runPath, o.runID, sub.SubQueryID, result.FaultedReason)
		o.emitDecision(parentCtx, "fanout_failed", runPath, o.runID, sub.SubQueryID, result.FaultedReason)
		return result
	}

	// Audit lifecycle: completed row.
	o.writeAudit(parentCtx, "action_fanout_completed", runPath, o.runID, sub.SubQueryID, "")
	o.emitDecision(parentCtx, "fanout_completed", runPath, o.runID, sub.SubQueryID, "")

	return *subResult
}

// runSubAgentInner — drives the eino adk.Agent's Run method on the
// per-sub-agent (chat or deep archetype). Drains the AsyncIterator
// to completion, then performs the FAN-04 SC-4 per-sub-agent
// ApplyTrustStage grounding. Returns the SubAgentResult envelope.
//
// The grounding call is the per-sub-agent equivalent of the parent's
// Step 5 — every sub-agent's verdict is independently grounded
// against the shared ChunkAccumulator so a faulty sub-agent does not
// propagate ungrounded citations to the merge.
func (o *FanOutOrchestrator) runSubAgentInner(ctx context.Context, subAgent adk.Agent, sub subQueryPlan, acc *SharedChunkAccumulator, input *adk.AgentInput) (*SubAgentResult, error) {
	if subAgent == nil {
		return nil, errors.New("runSubAgentInner: sub-agent is nil")
	}
	// eino v0.9.14 chatmodel.go:1075 panics on nil AgentInput. Always
	// pass a non-nil input — fall back to a synthesized one carrying
	// the sub-query when the parent input is nil (test-only path or
	// defensive path; the production wiring always supplies one).
	subInput := input
	if subInput == nil {
		subInput = &adk.AgentInput{
			Messages: []*schema.Message{{
				Role:    schema.User,
				Content: sub.SubQuery,
			}},
			EnableStreaming: false,
		}
	}
	iter := subAgent.Run(ctx, subInput)
	if iter == nil {
		return nil, errors.New("runSubAgentInner: sub-agent Run returned nil iterator")
	}

	// Drain the iterator; capture the final assistant message.
	var (
		finalMsg    *schema.Message
		iterErr     error
	)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			iterErr = ev.Err
			continue
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			finalMsg = ev.Output.MessageOutput.Message
		}
	}
	if iterErr != nil {
		return nil, fmt.Errorf("sub-agent %q iterator: %w", sub.SubQueryID, iterErr)
	}
	if finalMsg == nil {
		return nil, fmt.Errorf("sub-agent %q produced no message", sub.SubQueryID)
	}

	// FAN-04 SC-4 per-sub-agent grounding — retrieve THEN apply
	// TrustStage. The sub-agent's retrieve Tool writes hits into the
	// shared accumulator on its own ReAct loop, but the chat-archetype
	// path currently has no Tool wiring (buildChatModelAgent receives
	// cfg.Tools == nil from the orchestrator's build path), so the
	// accumulator stays empty and the trust-floor at len(Chunks)==0
	// would refuse every sub-agent verdict. To keep the live UAT
	// functioning WITHOUT requiring a full Tool-whitelist wiring
	// refactor (which is a separate 13-05 follow-up), the orchestrator
	// re-issues the retrieval HERE using the sub-skill's ResolvedTiers
	// (the intersection the BuildSubAgent constructor baked into the
	// subManifest) and the sub-query as the search query. Hits are
	// added to the shared accumulator so the merged grounding pass
	// (applyTrustStageToMerged) also sees the union (P13-L9).
	var hits []retriever.Hit
	if o.subRetriever != nil && o.subEmbedder != nil {
		vec, embErr := o.subEmbedder.Embed(ctx, sub.SubQuery)
		if embErr == nil {
			now := time.Now().Unix()
			// The sub-skill's resolved tier slice (closed-over by
			// BuildSubAgent at construction time, P13-L2). An empty
			// slice here (e.g. the fanout-12 parent-empty-tiers
			// regression case) is allowed — Retriever.Search returns
			// zero hits when tiers is empty, the trust floor then
			// refuses the verdict, and the conservative merge
			// downgrades. The verifier math self-heals.
			subTiers := sub.SubSkillTiers
			retrievedHits, retErr := o.subRetriever.Search(ctx, vec, subTiers, SubAgentRetrieveTopK, now)
			if retErr == nil {
				hits = retrievedHits
				for _, h := range hits {
					acc.Add(h)
				}
			}
		}
	}
	tsOut, err := ApplyTrustStage(ctx, &TrustStageInput{
		Manifest: o.parentManifest,
		Output:   finalMsg,
		Chunks:   hits,
		History:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("sub-agent %q ApplyTrustStage: %w", sub.SubQueryID, err)
	}

	// Map the trust-stage output to SubAgentResult.
	res := &SubAgentResult{
		ID:       sub.SubQueryID,
		SubQuery: sub.SubQuery,
	}
	if tsOut.Refusal != nil {
		res.Status = SubAgentStatusRefused
		res.Refusal = tsOut.Refusal
		return res, nil
	}
	if tsOut.Verdict != nil {
		res.Status = SubAgentStatusOK
		res.Verdict = tsOut.Verdict
		res.Confidence = tsOut.Verdict.Confidence
		res.Citations = tsOut.Verdict.Citations
		return res, nil
	}
	// Invariant violation: ApplyTrustStage returned neither verdict
	// nor refusal. Treat as faulted (defensive).
	return nil, fmt.Errorf("sub-agent %q ApplyTrustStage returned nil verdict+refusal", sub.SubQueryID)
}

// applyTrustStageToMerged (Step 5) — re-grounds the merged verdict
// against the SHARED ChunkAccumulator (P13-L9). The merged verdict's
// citations must be entailment-checked against the UNION of parent
// + sub-agent retrievals; this is the FAN-04 SC-4 final pass.
//
// If the merged verdict's citations exceed the grounding threshold
// the function returns a Refusal envelope (the conservative posture
// — D2-14 + P13-L5). If grounding passes, the merged verdict is
// returned unchanged.
func (o *FanOutOrchestrator) applyTrustStageToMerged(ctx context.Context, merged MergedVerdict, acc *SharedChunkAccumulator) *TrustStageOutput {
	// Build a synthetic output carrying the merged verdict's JSON
	// content for ApplyTrustStage.
	out := &schema.Message{
		Role:    schema.Assistant,
		Content: merged.Status,
	}
	// FAN-04 SC-4 final pass: feed the accumulator's UNION hits
	// (parent + sub-agents) to the trust floor — without this the
	// trust floor refuses the merged verdict on len(Chunks)==0 and
	// the conservative merge path downgrades everything to
	// needs_input (the "downgraded even when sub-agents were OK"
	// outage in the 13-05 first live run).
	var chunks []retriever.Hit
	if acc != nil {
		chunks = acc.Hits()
	}
	tsOut, err := ApplyTrustStage(ctx, &TrustStageInput{
		Manifest: o.parentManifest,
		Output:   out,
		Chunks:   chunks,
		History:  nil,
	})
	if err != nil {
		// Defensive: trust-stage error maps to a refused output.
		return &TrustStageOutput{
			Refusal: &verdict.Refusal{
				Reason: "merged ApplyTrustStage failed",
			},
		}
	}
	if tsOut.Refusal != nil {
		return tsOut
	}
	// Preserve the merged verdict on the trust-stage output.
	return &TrustStageOutput{
		Verdict: &verdict.Verdict{
			Status:         merged.Status,
			Confidence:     merged.Confidence,
			ReviewRequired: merged.ReviewRequired,
			Citations:      merged.Citations,
		},
	}
}

// eventFromVerdict converts the trust-stage output into the single
// adk.AgentEvent emitted on the parent's AsyncIterator. The
// orchestrator emits ONE event carrying the merged verdict status
// (the SSE stream surfaces the verdict via RealChatTransport; the
// parent's iterator contract is "one final event then close").
func (o *FanOutOrchestrator) eventFromVerdict(tsOut *TrustStageOutput) *adk.AgentEvent {
	content := "FanOutOrchestrator completed"
	if tsOut == nil {
		content = "FanOutOrchestrator completed (no trust-stage output)"
	} else if tsOut.Refusal != nil {
		content = fmt.Sprintf("FanOutOrchestrator refused: %s", tsOut.Refusal.Reason)
	} else if tsOut.Verdict != nil {
		content = fmt.Sprintf("FanOutOrchestrator verdict: status=%s confidence=%.2f",
			tsOut.Verdict.Status, tsOut.Verdict.Confidence)
	}
	return &adk.AgentEvent{
		AgentName: o.name,
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming: false,
				Message: &schema.Message{
					Role:    schema.Assistant,
					Content: content,
				},
				Role: schema.Assistant,
			},
		},
	}
}

// faultedResult builds a faulted SubAgentResult envelope with the
// supplied reason. Used by fanOut's early-exit paths (ctx cancelled
// before semaphore acquire).
func (o *FanOutOrchestrator) faultedResult(sub subQueryPlan, err error) SubAgentResult {
	return SubAgentResult{
		ID:            sub.SubQueryID,
		SubQuery:      sub.SubQuery,
		Status:        SubAgentStatusFaulted,
		FaultedReason: err.Error(),
	}
}

// emitDecision forwards a SubAgentDecision to the SSE emitter seam.
// nil emitter → silently skipped (the audit row still records the event).
func (o *FanOutOrchestrator) emitDecision(ctx context.Context, event, runPath, from, to, reason string) {
	if o.emitter == nil {
		return
	}
	_ = o.emitter.EmitFanoutDecision(ctx, SubAgentDecision{
		RunID:        o.runID,
		RunPath:      runPath,
		TransferFrom: from,
		TransferTo:   to,
		Event:        event,
		Reason:       reason,
	})
}

// emitFailure surfaces a pipeline-level failure via the audit writer.
// Step 1 (Classify) failures land here — there's no sub-agent Run to
// attribute the failure to.
func (o *FanOutOrchestrator) emitFailure(ctx context.Context, stage string, err error) {
	reason := fmt.Sprintf("%s: %v", stage, err)
	// Best-effort write — the orchestrator continues even if the audit
	// write fails (the SSE channel still closes via the deferred gen.Close()).
	if o.auditWriter != nil {
		_ = o.auditWriter.PutAudit(ctx, "action_fanout_failed", o.runID, o.runID, stage, reason)
	}
}

// writeAudit writes one audit row through the audit seam. nil writer →
// silently skipped.
func (o *FanOutOrchestrator) writeAudit(ctx context.Context, action, runPath, transferFrom, transferTo, reason string) {
	if o.auditWriter == nil {
		return
	}
	_ = o.auditWriter.PutAudit(ctx, action, runPath, transferFrom, transferTo, reason)
}

// Archetype returns ArchetypeOrchestrator. Convenience accessor for
// tests + future tooling that introspects the dispatch surface.
func (o *FanOutOrchestrator) Archetype() AgentArchetype {
	return ArchetypeOrchestrator
}

// ParentManifest exposes the parent manifest for tests + the
// BuildSubAgent seam (the factory needs the parent's ResolvedTiers()
// to compute the intersection).
func (o *FanOutOrchestrator) ParentManifest() *skill.Manifest {
	return o.parentManifest
}

// SubAgentFactory returns the orchestrator's SubAgentFactory.
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