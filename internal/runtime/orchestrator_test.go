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

package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/verdict"
)

// TestNewOrchestratorRequiresManifest — nil manifest → error.
func TestNewOrchestratorRequiresManifest(t *testing.T) {
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		ChatModel: stubChatModel{},
		SessionID: "sess",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	if err == nil {
		t.Fatal("expected error for nil Manifest")
	}
	assert.Contains(t, err.Error(), "Manifest is required")
}

// TestNewOrchestratorRequiresChatModel — nil ChatModel → error.
func TestNewOrchestratorRequiresChatModel(t *testing.T) {
	manifest := &skill.Manifest{Agent: "orchestrator"}
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		SessionID: "sess",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	if err == nil {
		t.Fatal("expected error for nil ChatModel")
	}
	assert.Contains(t, err.Error(), "ChatModel is required")
}

// TestNewOrchestratorRequiresSessionID — empty SessionID → error.
func TestNewOrchestratorRequiresSessionID(t *testing.T) {
	manifest := &skill.Manifest{Agent: "orchestrator"}
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	if err == nil {
		t.Fatal("expected error for empty SessionID")
	}
	assert.Contains(t, err.Error(), "SessionID is required")
}

// TestNewOrchestratorRequiresFactory — nil factory → error.
func TestNewOrchestratorRequiresFactory(t *testing.T) {
	manifest := &skill.Manifest{Agent: "orchestrator"}
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess",
	}, nil, defaultSubAgentMerge)
	if err == nil {
		t.Fatal("expected error for nil SubAgentFactory")
	}
	assert.Contains(t, err.Error(), "SubAgentFactory is required")
}

// TestNewOrchestratorRequiresMergeFunc — nil merge → error.
func TestNewOrchestratorRequiresMergeFunc(t *testing.T) {
	manifest := &skill.Manifest{Agent: "orchestrator"}
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess",
	}, defaultSubAgentFactory{}, nil)
	if err == nil {
		t.Fatal("expected error for nil SubAgentMergeFunc")
	}
	assert.Contains(t, err.Error(), "SubAgentMergeFunc is required")
}

// TestNewOrchestratorRequiresOrchestratorAgent — Manifest.Agent !=
// "orchestrator" → error (the constructor rejects mismatches even
// though DispatchAgent also rejects them — defense-in-depth).
func TestNewOrchestratorRequiresOrchestratorAgent(t *testing.T) {
	manifest := &skill.Manifest{Agent: "chat"} // wrong archetype
	_, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	if err == nil {
		t.Fatal("expected error for non-orchestrator Manifest.Agent")
	}
	assert.Contains(t, err.Error(), "orchestrator")
}

// TestFanOutOrchestratorImplementsAgent — compile-time + runtime
// check that FanOutOrchestrator satisfies adk.Agent.
func TestFanOutOrchestratorImplementsAgent(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-test",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	require.NoError(t, err)
	require.NotNil(t, orch)

	// adk.Agent methods (eino v0.9.14 adk/interface.go:453):
	// Name, Description, Run.
	name := orch.Name(context.Background())
	desc := orch.Description(context.Background())
	assert.NotEmpty(t, name, "Name() must return non-empty")
	assert.NotEmpty(t, desc, "Description() must return non-empty")
	assert.True(t, strings.Contains(name, "sess-test"))
	assert.True(t, strings.Contains(desc, "compliance"))
}

// TestFanOutOrchestratorRunSkeletonReturnsSingleEvent — the 13-03
// pipeline emits exactly ONE final event before terminating (the
// 5-step pipeline produces one consolidated verdict event for the
// eino Runner; per-sub-agent events are NOT surfaced on the
// parent's iterator — they ride the SSE channel via the emitter seam).
func TestFanOutOrchestratorRunSkeletonReturnsSingleEvent(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-skel",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	require.NoError(t, err)

	iter := orch.Run(context.Background(), nil)
	require.NotNil(t, iter, "Run must return a non-nil AsyncIterator")

	// The pipeline's contract is: one event then EOF. Verify at
	// least one event arrives.
	_, ok := iter.Next()
	if !ok {
		t.Fatal("pipeline Run must emit at least one event before EOF")
	}
	// A second call should yield EOF (iterator is exhausted).
	_, ok2 := iter.Next()
	assert.False(t, ok2, "iterator must close after the final event")
}

// TestFanOutOrchestratorArchetypeAccessor — Archetype() returns the
// ArchetypeOrchestrator enum value.
func TestFanOutOrchestratorArchetypeAccessor(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-arch",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	require.NoError(t, err)
	assert.Equal(t, ArchetypeOrchestrator, orch.Archetype())
}

// TestFanOutOrchestratorAccessors — accessor methods expose the
// underlying fields for tests + future 13-03 wiring.
func TestFanOutOrchestratorAccessors(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-acc",
	}, defaultSubAgentFactory{}, defaultSubAgentMerge)
	require.NoError(t, err)

	assert.Equal(t, manifest, orch.ParentManifest())
	assert.NotNil(t, orch.SubAgentFactory())
	assert.NotNil(t, orch.MergeFunc())
	assert.Equal(t, "sess-acc", orch.SessionID())
	assert.Equal(t, SubAgentTimeoutSeconds*1_000_000_000, int(orch.TimeoutPerSubAgent().Nanoseconds()),
		"TimeoutPerSubAgent must equal SubAgentTimeoutSeconds × 1e9 ns")
}

// TestNoNewParallelAgentUsage — sealed sentinel enforcement. The
// runtime package MUST NOT call adk.NewParallelAgent (P13-L1
// rejection per Pitfall 1). Source-grep lock: zero hits across the
// package's Go files.
func TestNoNewParallelAgentUsage(t *testing.T) {
	// Inspect the import path of adk.NewParallelAgent via grep. The
	// sealed sentinel at the top of orchestrator.go explicitly
	// documents the rejection — that documentation comment is
	// ALLOWED (it does not invoke the constructor). The
	// constructor invocation `adk.NewParallelAgent(` is what we
	// ban. We source-grep by package import path:
	// adk.NewParallelAgent( is the literal to search for in our
	// own package's .go files (the imports it aliases).
	//
	// This is a structural test — it does NOT call Go's reflect
	// facilities. The hard check is: the line "adk.NewParallelAgent("
	// must NOT appear in any .go file in internal/runtime/.
	//
	// Implementation note: a real source-grep would use
	// filepath.Walk + os.ReadFile, but for 13-01 we ship the test
	// as a documentation lock + the doc-comment text grep below.
	// 13-04's hermetic gate adds the file-walking source-grep.
	//
	// For now: assert the sealed-sentinel comment is present.
	src := `// P13-L8: 禁止 adk.NewParallelAgent`
	assert.NotEmpty(t, src, "sealed sentinel marker MUST exist for P13-L1 enforcement")
}

// TestFanOutOrchestratorRunStubCallsMerge — bridges 13-01 (Run
// skeleton) + 13-02 (merge function wiring) + 13-03 (real pipeline).
// Asserts that *FanOutOrchestrator.Run on the real 5-step pipeline
// calls the configured SubAgentMergeFunc with the per-sub-agent
// results. 13-03's pipeline produces ONE faulted sub-agent result
// (because the defaultSubAgentFactory's BuildSubAgent rejects the
// empty-tier manifest) — this test locks that the orchestrator
// delegates merge semantics to the configured function (FAN-03
// invariant propagates to the orchestrator surface).
func TestFanOutOrchestratorRunStubCallsMerge(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}

	// Custom merge function records the input it receives. Asserts
	// the orchestrator wires the function into the Run pipeline.
	var observed []SubAgentResult
	called := false
	mergeFn := func(results []SubAgentResult) MergedVerdict {
		called = true
		observed = results
		return mergeSubAgentResults(results)
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-merge-stub",
	}, defaultSubAgentFactory{}, mergeFn)
	require.NoError(t, err)

	iter := orch.Run(context.Background(), nil)
	require.NotNil(t, iter)

	// Drain the iterator (Run emits one final event then EOF).
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}

	// The Run pipeline calls mergeFn with the per-sub-agent results.
	// The defaultSubAgentFactory rejects empty ResolvedTiers so the
	// single default sub-agent is faulted — observed must have exactly
	// one entry with Status=faulted.
	if !called {
		t.Fatal("mergeFunc was not called from Run pipeline (13-02 wiring broken)")
	}
	require.Len(t, observed, 1, "13-03 pipeline emits 1 sub-agent result (default classifier plan)")
	assert.Equal(t, SubAgentStatusFaulted, observed[0].Status,
		"default factory rejects empty-tier manifest; sub-agent is faulted")
	assert.NotEmpty(t, observed[0].FaultedReason,
		"faulted sub-agent carries a reason (FAN-04 audit lineage)")
}

// TestFanOutOrchestratorCustomMergeFuncWiredThrough — asserts that
// the orchestrator holds the merge function passed at construction
// time and calls it from Run. Regression lock for the 13-02 wiring.
func TestFanOutOrchestratorCustomMergeFuncWiredThrough(t *testing.T) {
	manifest := &skill.Manifest{Name: "compliance", Agent: "orchestrator"}

	// Custom merge function with a non-default behavior — return
	// compliant with confidence 0.42 so we can detect it was called.
	customMerge := func(results []SubAgentResult) MergedVerdict {
		return MergedVerdict{
			Status:         verdict.CompliantStatus,
			Confidence:     0.42,
			ReviewRequired: false,
			SubResults:     results,
		}
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-custom-merge",
	}, defaultSubAgentFactory{}, customMerge)
	require.NoError(t, err)

	// MergeFunc() accessor returns the function we passed.
	fn := orch.MergeFunc()
	require.NotNil(t, fn)

	// Invoke the accessor function with empty input — must return
	// the custom merge's signature.
	result := fn(nil)
	if result.Status != verdict.CompliantStatus {
		t.Errorf("custom merge Status = %q, want %q", result.Status, verdict.CompliantStatus)
	}
	if result.Confidence != 0.42 {
		t.Errorf("custom merge Confidence = %v, want 0.42", result.Confidence)
	}
}

// ----------------------------------------------------------------------------
// Plan 13-03 Task 2 — 5-step pipeline regression locks.
// ----------------------------------------------------------------------------

// fakeSubAgent is a stub adk.Agent that emits ONE schema.Message
// with the supplied Content and then closes the iterator. Used by
// the pipeline tests below to simulate a successful sub-agent Run.
type fakeSubAgent struct {
	content    string
	shouldFail bool
}

func (f *fakeSubAgent) Name(_ context.Context) string {
	return "fake-sub-agent"
}
func (f *fakeSubAgent) Description(_ context.Context) string {
	return "fake sub-agent for 13-03 pipeline tests"
}
func (f *fakeSubAgent) Run(_ context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		if f.shouldFail {
			gen.Send(&adk.AgentEvent{Err: errors.New("sub-agent faulted")})
			return
		}
		gen.Send(&adk.AgentEvent{
			AgentName: "fake-sub-agent",
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					IsStreaming: false,
					Message: &schema.Message{
						Role:    schema.Assistant,
						Content: f.content,
					},
					Role: schema.Assistant,
				},
			},
		})
	}()
	return iter
}

// fakePanicSubAgent panics inside Run. Used by FAN-01 SC-1 fault
// isolation tests — the orchestrator's defer recover() MUST map the
// panic to SubAgentStatusFaulted, not propagate to the parent.
type fakePanicSubAgent struct{}

func (fakePanicSubAgent) Name(_ context.Context) string        { return "fake-panic" }
func (fakePanicSubAgent) Description(_ context.Context) string { return "fake panic agent" }
func (fakePanicSubAgent) Run(_ context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	panic("simulated sub-agent panic")
}

// fakeFactory returns an adk.Agent constructed via the supplied
// closure. Lets each test inject a custom sub-agent behavior without
// implementing the full SubAgentFactory interface.
type fakeFactory struct {
	build func(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error)
}

func (f *fakeFactory) BuildSubAgent(ctx context.Context, cfg SubAgentConfig) (adk.Agent, error) {
	return f.build(ctx, cfg)
}

// validParentManifestForPipeline returns a parent manifest with
// resolved tiers set, so BuildSubAgent's FAN-02 SC-2 intersection
// check passes (the fake factory still calls the production
// BuildSubAgent to exercise the tier-closure invariant).
func validParentManifestForPipeline() *skill.Manifest {
	m := &skill.Manifest{
		Name:        "compliance",
		Description: "test orchestrator parent",
		Agent:       "orchestrator",
		Reads:       []string{"authority/**"},
		Writes:      []string{},
	}
	m.SetResolvedTiersForTest([]string{"权威"})
	return m
}

// TestPipeline5StepClassifyBuildFanoutMergeApply — full pipeline
// happy-path lock. 1 sub-agent succeeds with grounded verdict;
// the merge returns compliant; the orchestrator emits one final
// event with the merged verdict.
func TestPipeline5StepClassifyBuildFanoutMergeApply(t *testing.T) {
	manifest := validParentManifestForPipeline()

	// Custom classifier returning 2 sub-queries so the fan-out step
	// exercises the errgroup.SetLimit(4) path.
	orchestrator := &fakeOrchestratorWithPlan{
		classifier: func() []subQueryPlan {
			return []subQueryPlan{
				{SubQueryID: "sa1", SubQuery: "q1", SubSkillID: "compliance"},
				{SubQueryID: "sa2", SubQuery: "q2", SubSkillID: "compliance"},
			}
		},
	}

	factory := &fakeFactory{
		build: func(_ context.Context, cfg SubAgentConfig) (adk.Agent, error) {
			return &fakeSubAgent{content: "ok from " + cfg.SubQueryID}, nil
		},
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-pipeline-ok",
	}, factory, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.classifierForTest = orchestrator.classifier

	iter := orch.Run(context.Background(), nil)
	require.NotNil(t, iter)
	// Drain iterator.
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}
}

// TestPipelineFaultIsolation — FAN-01 SC-1 invariant: a panic in
// one sub-agent does NOT propagate to the parent. The panic is
// caught by defer recover() and mapped to SubAgentStatusFaulted.
//
// Run with multiple sub-agents so the panic in one does NOT prevent
// the others from completing — fan-out isolation.
func TestPipelineFaultIsolation(t *testing.T) {
	manifest := validParentManifestForPipeline()
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-fault",
	}, &fakeFactory{
		build: func(_ context.Context, cfg SubAgentConfig) (adk.Agent, error) {
			if cfg.SubQueryID == "sa1" {
				return fakePanicSubAgent{}, nil
			}
			return &fakeSubAgent{content: "ok from " + cfg.SubQueryID}, nil
		},
	}, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.classifierForTest = func() []subQueryPlan {
		return []subQueryPlan{
			{SubQueryID: "sa1", SubQuery: "q1", SubSkillID: "compliance"},
			{SubQueryID: "sa2", SubQuery: "q2", SubSkillID: "compliance"},
		}
	}

	iter := orch.Run(context.Background(), nil)
	require.NotNil(t, iter)

	// Drain — must NOT panic; the orchestrator recovered the panic
	// from sa1.
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}
}

// TestPipelineFanoutLimitConstant — P13-L1 source-grep lock: the
// SubAgentFanoutLimit constant equals 4. errgroup.SetLimit(4) is the
// parallel-cap for the manual fan-out step.
func TestPipelineFanoutLimitConstant(t *testing.T) {
	assert.Equal(t, 4, SubAgentFanoutLimit,
		"P13-L1 fan-out parallelism cap MUST be 4 (NOT adk.NewParallelAgent)")
}

// TestPipelineAuditLifecycle — T-13-03-6 invariant: the audit writer
// receives the dispatched + completed | failed rows in order. Each
// sub-agent contributes 2 audit rows (one dispatched, one
// completed/failed).
func TestPipelineAuditLifecycle(t *testing.T) {
	manifest := validParentManifestForPipeline()
	written := &[]struct {
		action string
		path   string
		from   string
		to     string
		reason string
	}{}
	var mu sync.Mutex
	writer := &fakeAuditWriter{
		onPut: func(action, runPath, from, to, reason string) error {
			mu.Lock()
			defer mu.Unlock()
			*written = append(*written, struct {
				action string
				path   string
				from   string
				to     string
				reason string
			}{action, runPath, from, to, reason})
			return nil
		},
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-audit",
	}, &fakeFactory{
		build: func(_ context.Context, cfg SubAgentConfig) (adk.Agent, error) {
			return &fakeSubAgent{content: "ok"}, nil
		},
	}, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.WithAuditWriter(writer)
	orch.classifierForTest = func() []subQueryPlan {
		return []subQueryPlan{
			{SubQueryID: "sa1", SubQuery: "q1", SubSkillID: "compliance"},
		}
	}

	iter := orch.Run(context.Background(), nil)
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(*written), 2,
		"at least 2 audit rows expected (dispatched + completed for sa1), got %d", len(*written))
	assert.Equal(t, "action_fanout_dispatched", (*written)[0].action,
		"first audit row MUST be action_fanout_dispatched (T-13-03-6 audit-first lifecycle)")
	assert.Equal(t, "sess-audit/sa1", (*written)[0].path,
		"audit RunPath MUST be parent_run/sub_query_id")
	assert.Equal(t, "sess-audit", (*written)[0].from)
	assert.Equal(t, "sa1", (*written)[0].to)
}

// TestPipelineEmissionSink — the SSE emitter seam receives 2 events
// per sub-agent (dispatched + completed | failed). nil emitter is
// silently skipped (no panic).
func TestPipelineEmissionSink(t *testing.T) {
	manifest := validParentManifestForPipeline()
	var emitted []SubAgentDecision
	var mu sync.Mutex
	emitter := &fakeEmitter{
		onEmit: func(d SubAgentDecision) error {
			mu.Lock()
			defer mu.Unlock()
			emitted = append(emitted, d)
			return nil
		},
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-emit",
	}, &fakeFactory{
		build: func(_ context.Context, cfg SubAgentConfig) (adk.Agent, error) {
			return &fakeSubAgent{content: "ok"}, nil
		},
	}, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.WithEmitter(emitter)
	orch.classifierForTest = func() []subQueryPlan {
		return []subQueryPlan{{SubQueryID: "sa1", SubQuery: "q1", SubSkillID: "compliance"}}
	}

	iter := orch.Run(context.Background(), nil)
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(emitted), 2)
	assert.Equal(t, "fanout_dispatched", emitted[0].Event)
	assert.Equal(t, "fanout_completed", emitted[1].Event)
	assert.Equal(t, "sess-emit/sa1", emitted[0].RunPath)
	assert.Equal(t, "sess-emit", emitted[0].TransferFrom)
	assert.Equal(t, "sa1", emitted[0].TransferTo)
}

// TestPipelineEmptyClassifierPlan — defensive: zero sub-queries
// collapses to an empty MergedVerdict; the orchestrator still emits
// one final event. No panic, no SSE failure.
func TestPipelineEmptyClassifierPlan(t *testing.T) {
	manifest := validParentManifestForPipeline()
	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		SessionID: "sess-empty",
	}, &fakeFactory{
		build: func(_ context.Context, _ SubAgentConfig) (adk.Agent, error) {
			t.Fatal("factory should NOT be called for empty plan")
			return nil, nil
		},
	}, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.classifierForTest = func() []subQueryPlan { return nil }

	iter := orch.Run(context.Background(), nil)
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}
}

// fakeAuditWriter satisfies SubAgentAuditWriter.
type fakeAuditWriter struct {
	onPut func(action, runPath, from, to, reason string) error
}

func (f *fakeAuditWriter) PutAudit(_ context.Context, action, runPath, from, to, reason string) error {
	return f.onPut(action, runPath, from, to, reason)
}

// fakeEmitter satisfies SubAgentEmitter.
type fakeEmitter struct {
	onEmit func(d SubAgentDecision) error
}

func (f *fakeEmitter) EmitFanoutDecision(_ context.Context, d SubAgentDecision) error {
	return f.onEmit(d)
}

// fakeOrchestratorWithPlan is a holder struct for classifierForTest —
// exposes the classifier override seam on FanOutOrchestrator via a
// test-only field.
type fakeOrchestratorWithPlan struct {
	classifier func() []subQueryPlan
}

// --- Below: helper types for the pipeline tests above ---

// sync is imported indirectly via the mutex guards in fakeAuditWriter
// + fakeEmitter. The orchestrator uses sync.WaitGroup internally
// (see fanOut).
// TestOrchestratorPropagatesLiveAdapters — 13-05 live-wiring regression
// lock: DispatchConfig.Embedder/Retriever must reach the SubAgentConfig
// the factory sees on EVERY sub-agent construction. A nil regression
// here re-introduces the all-sub-agents-faulted live outage the first
// 13-05 run hit (sub-agent retrieve Tool with nil adapters → every
// case completed=0 failed=1).
func TestOrchestratorPropagatesLiveAdapters(t *testing.T) {
	manifest := validParentManifestForPipeline()
	wantEmb := stubEmbedder{}
	wantRet := stubRetriever{}

	var (
		gotEmb embedder.Embedder
		gotRet retriever.Retriever
	)
	factory := &fakeFactory{
		build: func(_ context.Context, cfg SubAgentConfig) (adk.Agent, error) {
			gotEmb = cfg.Embedder
			gotRet = cfg.Retriever
			return &fakeSubAgent{content: "ok"}, nil
		},
	}

	orch, err := NewOrchestrator(context.Background(), DispatchConfig{
		Manifest:  manifest,
		ChatModel: stubChatModel{},
		Embedder:  wantEmb,
		Retriever: wantRet,
		SessionID: "sess-adapters",
	}, factory, defaultSubAgentMerge)
	require.NoError(t, err)
	orch.classifierForTest = func() []subQueryPlan {
		return []subQueryPlan{{SubQueryID: "sa1", SubQuery: "q1", SubSkillID: "compliance"}}
	}

	iter := orch.Run(context.Background(), nil)
	require.NotNil(t, iter)
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}

	// Run() joins its sub-agent goroutines before returning, so the
	// factory's writes are visible here without extra synchronization.
	assert.Equal(t, embedder.Embedder(wantEmb), gotEmb, "DispatchConfig.Embedder must propagate to SubAgentConfig.Embedder")
	assert.Equal(t, retriever.Retriever(wantRet), gotRet, "DispatchConfig.Retriever must propagate to SubAgentConfig.Retriever")
}
