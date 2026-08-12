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

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
)

// RetrieveToolName is the canonical Eino tool name. Exported as a
// constant so internal/server wiring (Plan 03-05) and tests assert
// against the exact string ("retrieve_compliance_chunks") rather than
// a magic literal.
const RetrieveToolName = "retrieve_compliance_chunks"

// retrieveTopK is the production default for Retriever.Search's topK
// argument. 5 hits balances context coverage (compliance reviewers
// typically need 2-3 grounded clauses per answer) against token-budget
// pressure (each chunk consumes ~250 tokens; 5 chunks = ~1.25K tokens,
// leaving the model ~14K of context for reasoning under GLM-5.2's 16K
// compliance-mode budget).
//
// Not exported: not a tunable knob for v1 — exposed as a constant so a
// future rag-eval spike can override at construction time without
// hunting for magic numbers.
const retrieveTopK = 5

// retrieveArgs is the JSON arguments shape the ReAct LLM emits when
// calling retrieve_compliance_chunks. Eino's utils.NewTool deserializes
// the LLM-emitted JSON arguments string into this struct before invoking
// the underlying function.
//
// json tags are lower-cased per the project convention (matches the
// retrieveResponse shape and internal/wire chunk convention).
type retrieveArgs struct {
	Query string `json:"query"`
}

// retrieveResponse is the JSON shape returned to the LLM. The Chunks
// field is a single multi-line string with the [CHUNK N] ... \n---\n
// framing copied character-for-character from
// internal/server/pipeline.go buildPrompt (lines 234-241) — T-2-21
// prompt-injection mitigation: the framing tells the LLM that the
// retrieved content is DATA, not instructions. Modifying the framing
// here breaks TestRetrieveToolFramesChunksAsData.
type retrieveResponse struct {
	Chunks string `json:"chunks"`
	Count  int    `json:"count"`
}

// BuildRetrieveTool constructs the retrieve_compliance_chunks
// InvokableTool. The tool closes over the manifest's ResolvedTiers
// slice (KNOW-03 — the agent LLM cannot override tiers; only the
// Skill's reads field can, via the Loader).
//
// The tool invokes the embedder → retriever.Search → format pipeline
// on every call. It is stateless beyond the closed-over manifest;
// concurrent InvokableRun calls are safe (no shared mutable state in
// the tool itself — the embedder and retriever implementations carry
// their own thread-safety guarantees).
//
// Exported because tests in internal/agent (TestBuildRetrieveTool,
// TestRetrieveToolFramesChunksAsData, TestRetrieveToolEnforcesResolvedTiers)
// call it directly to assert on the tool's output without going
// through the full react.Agent loop. NewAgent delegates to this
// function for the production wiring.
func BuildRetrieveTool(emb embedder.Embedder, ret retriever.Retriever, m *skill.Manifest) tool.InvokableTool {
	if emb == nil {
		// Fail loudly at construction rather than returning a tool that
		// panics on first InvokableRun. The error is wrapped in the
		// returned tool's first invocation via a closure that produces
		// the error message — we cannot return (nil, error) here because
		// the function signature is `tool.InvokableTool` (no error).
		// NewAgent pre-validates non-nil Embedder, so reaching this
		// branch is a programmer error.
		panic("agent.BuildRetrieveTool: embedder is nil (caller bug — NewAgent should pre-validate)")
	}
	if ret == nil {
		panic("agent.BuildRetrieveTool: retriever is nil (caller bug — NewAgent should pre-validate)")
	}
	if m == nil {
		panic("agent.BuildRetrieveTool: manifest is nil (caller bug — NewAgent should pre-validate)")
	}

	// Snapshot the tier slice ONCE at construction. This is the KNOW-03
	// closure: the agent LLM has no way to mutate the tiers that
	// Retriever.Search receives because the tool function does not read
	// from any LLM-supplied field that maps to tiers (retrieveArgs has
	// only `query`). The closure captures `tiers` by value (a slice
	// header copy) so even subsequent mutations to the manifest's
	// resolvedTiers field do not affect an already-built tool.
	tiers := m.ResolvedTiers()

	desc := &schema.ToolInfo{
		Name: RetrieveToolName,
		Desc: "Retrieve compliance-regulation chunks from the knowledge tiers this Skill is authorized to read (authority / historical / personal). Call this tool when the user's question requires grounding in actual policy text. The query should be a focused compliance question or a refined search phrase; avoid single-word queries.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "The compliance question or refined search query to retrieve supporting chunks for.",
				Required: true,
			},
		}),
	}

	invoke := func(ctx context.Context, args retrieveArgs) (retrieveResponse, error) {
		query := strings.TrimSpace(args.Query)
		if query == "" {
			// Empty-query guard: the LLM may emit {"query": ""} when it
			// hallucinates a retrieve call without a concrete question.
			// Returning an error here causes the ReAct loop to feed the
			// error back to the LLM, which typically retries with a
			// real query on the next turn (within MaxStep budget).
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: query is required (cannot be empty or whitespace)")
		}

		vec, err := emb.Embed(ctx, query)
		if err != nil {
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: embed query: %w", err)
		}

		hits, err := ret.Search(ctx, vec, tiers, retrieveTopK)
		if err != nil {
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: retriever.Search: %w", err)
		}

		return retrieveResponse{
			Chunks: formatChunksAsData(hits),
			Count:  len(hits),
		}, nil
	}

	// utils.NewTool infers the JSON-args shape from the generic
	// InvokeFunc type parameters. Verified at
	// eino@v0.7.7/components/tool/utils/invokable_func.go:101-105.
	return utils.NewTool(desc, invoke)
}

// formatChunksAsData renders a slice of retriever.Hit as the canonical
// [CHUNK N] ... \n---\n envelope. The framing is copied
// character-for-character from internal/server/pipeline.go buildPrompt
// (lines 234-241) — both production code paths MUST stay byte-identical
// (T-2-21 prompt-injection mitigation; TestRetrieveToolFramesChunksAsData
// locks the framing).
//
// The format is:
//
//	[CHUNK <id>] tier=<tier> source=<source>
//	<content>
//	---
//
// The trailing newline + "---\n" separator after every chunk (including
// the last) is intentional: it gives the LLM a stable "end of DATA"
// marker and prevents the model from confusing chunk content with its
// own scratchpad reasoning.
//
// Pure function (no I/O, no globals) — trivially unit-testable; tests
// in TestRetrieveToolFramesChunksAsData exercise this via the public
// BuildRetrieveTool path.
func formatChunksAsData(hits []retriever.Hit) string {
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "[CHUNK %d] tier=%s source=%s\n%s\n---\n", h.ID, h.Tier, h.Source, h.Content)
	}
	return b.String()
}
