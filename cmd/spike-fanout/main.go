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

// Command spike-fanout is Phase 13 / Plan 13-01's RF-4 hermetic
// smoke binary. It proves the tier-closure substrate compiles,
// enforces structural tier-closure at construction (FAN-02 SC-2 +
// P13-L2), and rejects empty-intersection cases at LOAD TIME — all
// without making any LLM call or any network call.
//
// Usage:
//
//	go run ./cmd/spike-fanout --scenario=match
//	go run ./cmd/spike-fanout --scenario=empty-intersection
//	go run ./cmd/spike-fanout --scenario=sub-empty-reads
//	go run ./cmd/spike-fanout --scenario=parent-empty-tiers
//	go run ./cmd/spike-fanout --scenario=canonical
//
// Scenarios:
//
//   - match              — parent [权威] / sub [权威] → success
//   - empty-intersection — parent [权威] / sub [个人] → load-time error
//   - sub-empty-reads    — parent [权威] / sub []    → load-time error
//   - parent-empty-tiers — parent []   / sub [权威] → load-time error
//   - canonical          — demonstrates orchestrator's 5-step pipeline
//     skeleton with 3 sub-queries
//
// 13-05 extends this binary with --live + --golden flags for the
// end-to-end live fan-out UAT.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/runtime"
	"github.com/skillforge/skillforge/internal/skill"
)

func main() {
	scenario := flag.String("scenario", "match", "RF-4 scenario: match | empty-intersection | sub-empty-reads | parent-empty-tiers | canonical")
	flag.Parse()

	if err := runRF4Spike(*scenario); err != nil {
		fmt.Fprintf(os.Stderr, "[spike-fanout] scenario=%s FAILED: %v\n", *scenario, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[spike-fanout] scenario=%s OK\n", *scenario)
}

// runRF4Spike dispatches the scenario and returns the first error
// encountered. Each scenario prints a single JSON line to stdout.
func runRF4Spike(scenario string) error {
	ctx := context.Background()
	switch scenario {
	case "match":
		return runMatchScenario(ctx)
	case "empty-intersection":
		return runEmptyIntersectionScenario(ctx)
	case "sub-empty-reads":
		return runSubEmptyReadsScenario(ctx)
	case "parent-empty-tiers":
		return runParentEmptyTiersScenario(ctx)
	case "canonical":
		return runCanonicalScenario(ctx)
	default:
		return fmt.Errorf("unknown scenario %q", scenario)
	}
}

// makeParent returns a parent orchestrator Manifest with the supplied
// resolved-tier slice.
func makeParent(tiers []string, name string) *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Name:        name,
		Description: "parent orchestrator skill",
		Agent:       "orchestrator",
		Reads:       []string{"authority/**", "historical/**"},
		Writes:      []string{},
		Steps:       "1. Classify\n2. Fan-out\n3. Merge\n4. Apply Trust Stage",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never invoke HITL tools",
				"Never bypass tier-closure",
				"Never expand sub-agent tier boundary",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest(tiers)
	return m
}

// makeSub returns a sub-skill Manifest with the supplied Reads +
// resolved-tier slice.
func makeSub(name string, reads []string, tiers []string) *skill.Manifest {
	temp := float32(0.0)
	m := &skill.Manifest{
		Name:        name,
		Description: "sub-skill",
		Agent:       "chat",
		Reads:       reads,
		Writes:      []string{}, // sub-agents are read-only by structural design
		Steps:       "1. Retrieve\n2. Compose verdict",
		Guardrails: skill.Guardrails{
			Prohibitions: []string{
				"Never cite clauses not retrieved",
				"Never paraphrase critical clauses",
				"Never modify authority tier content",
			},
		},
		Temperature: &temp,
	}
	m.SetResolvedTiersForTest(tiers)
	return m
}

// runMatchScenario — happy path. parent [权威] / sub [权威] → BuildSubAgent succeeds.
func runMatchScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-compliance", []string{"authority/**"}, []string{"权威"})

	a, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-1",
	})
	if err != nil {
		return fmt.Errorf("match scenario failed: %w", err)
	}
	if a == nil {
		return fmt.Errorf("match scenario: BuildSubAgent returned nil agent")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario":      "match",
		"agent_returned": true,
		"agent_name":    a.Name(ctx),
	})
	fmt.Println(string(out))
	return nil
}

// runEmptyIntersectionScenario — parent [权威] / sub [个人] → load-time error.
func runEmptyIntersectionScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-personal", []string{"personal/**"}, []string{"个人"})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-2",
	})
	if err == nil {
		return fmt.Errorf("empty-intersection scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "empty-intersection",
		"error":    err.Error(),
		"expect":   "FAN-02 SC-2 load-time error: tier intersection empty",
	})
	fmt.Println(string(out))
	return nil
}

// runSubEmptyReadsScenario — sub_skill.Reads is empty → load-time error.
func runSubEmptyReadsScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威"}, "compliance-orchestrator")
	sub := makeSub("sub-empty", []string{}, []string{})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-3",
	})
	if err == nil {
		return fmt.Errorf("sub-empty-reads scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "sub-empty-reads",
		"error":    err.Error(),
		"expect":   "load-time error: empty Reads",
	})
	fmt.Println(string(out))
	return nil
}

// runParentEmptyTiersScenario — parent.ResolvedTiers() empty → load-time error.
func runParentEmptyTiersScenario(ctx context.Context) error {
	parent := makeParent([]string{}, "compliance-orchestrator")
	sub := makeSub("sub-ok", []string{"authority/**"}, []string{"权威"})

	_, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
		ParentManifest:   parent,
		SubSkillManifest: sub,
		SubQueryID:       "sa1",
		ChatModel:        &stubChatModel{},
		Embedder:         stubEmbedder{},
		Retriever:        stubRetriever{},
		SessionID:        "spike-sess-4",
	})
	if err == nil {
		return fmt.Errorf("parent-empty-tiers scenario: BuildSubAgent must FAIL but succeeded")
	}
	out, _ := json.Marshal(map[string]any{
		"scenario": "parent-empty-tiers",
		"error":    err.Error(),
		"expect":   "load-time error: empty ResolvedTiers",
	})
	fmt.Println(string(out))
	return nil
}

// runCanonicalScenario — demonstrates orchestrator's 5-step pipeline
// skeleton with 3 sub-queries. Each sub-query is built via
// BuildSubAgent with matching tiers.
func runCanonicalScenario(ctx context.Context) error {
	parent := makeParent([]string{"权威", "历史"}, "compliance-orchestrator")
	subs := []struct {
		name  string
		reads []string
		tiers []string
		query string
	}{
		{"sub-1", []string{"authority/**"}, []string{"权威"}, "sa1 query"},
		{"sub-2", []string{"historical/**"}, []string{"历史"}, "sa2 query"},
		{"sub-3", []string{"authority/**", "historical/**"}, []string{"权威", "历史"}, "sa3 query"},
	}

	var agents []string
	for i, subCfg := range subs {
		sub := makeSub(subCfg.name, subCfg.reads, subCfg.tiers)
		a, err := runtime.BuildSubAgent(ctx, runtime.SubAgentConfig{
			ParentManifest:   parent,
			SubSkillManifest: sub,
			SubQueryID:       subCfg.name,
			ChatModel:        &stubChatModel{},
			Embedder:         stubEmbedder{},
			Retriever:        stubRetriever{},
			SessionID:        "spike-sess-canonical",
			Instruction:      subCfg.query,
		})
		if err != nil {
			return fmt.Errorf("canonical[%d]: BuildSubAgent: %w", i, err)
		}
		agents = append(agents, a.Name(ctx))
	}
	out, _ := json.Marshal(map[string]any{
		"scenario":        "canonical",
		"sub_agents":      agents,
		"parent_tiers":    parent.ResolvedTiers(),
		"sub_skill_count": len(subs),
	})
	fmt.Println(string(out))
	return nil
}

// stub types — satisfy the interfaces without making any I/O calls.
// The chat model is never actually invoked in the hermetic spike
// (BuildSubAgent only checks the value's presence + passes it to
// DispatchAgent; in 13-01 the dispatch succeeds but no LLM call
// happens because we don't actually RUN the constructed agent).

// stubChatModel satisfies model.ToolCallingChatModel.
type stubChatModel struct{}

var _ model.ToolCallingChatModel = (*stubChatModel)(nil)

func (stubChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant}, nil
}
func (stubChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{}), nil
}
func (s stubChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

// stubEmbedder satisfies embedder.Embedder.
type stubEmbedder struct{}

var _ embedder.Embedder = stubEmbedder{}

func (stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (stubEmbedder) Dimensions() int { return 3 }

// stubRetriever satisfies retriever.Retriever (no I/O — every method
// returns nil/empty).
type stubRetriever struct{}

var _ retriever.Retriever = stubRetriever{}

func (stubRetriever) Health(_ context.Context) error                                  { return nil }
func (stubRetriever) Search(_ context.Context, _ []float32, _ []string, _ int, _ int64) ([]retriever.Hit, error) {
	return nil, nil
}
func (stubRetriever) Ensure(_ context.Context) error                                 { return nil }
func (stubRetriever) Insert(_ context.Context, _ []retriever.Document) error         { return nil }
func (stubRetriever) ListMetadata(_ context.Context) ([]retriever.DocumentMeta, error) {
	return nil, nil
}
func (stubRetriever) GetChunk(_ context.Context, _ int64) (retriever.ChunkContent, error) {
	return retriever.ChunkContent{}, nil
}
func (stubRetriever) Disconnect() error                                              { return nil }

// Suppress unused-import errors for types referenced only via
// embedded struct fields (eino tool.BaseTool is unused at this
// level — 13-03 fills).
var (
	_ tool.BaseTool
)