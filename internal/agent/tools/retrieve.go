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

// Package agenttools is the Phase 11 / Plan 11-01 namespace for the
// retrieve_compliance_chunks Tool. It is the D11-A3 / D11-B1 file
// re-home of internal/agent/retrieve_tool.go — the Tool now lives in
// its own package so the Runner-path Tool whitelist
// (BuildToolsForSkill) can compose it alongside the future HITL
// Tools (memory:facts, review_report — ships 11-02).
//
// The package path was chosen to mirror eino-ext's
// components/retriever/<vendor> subpackage pattern: one Tool per file,
// one package per Tool surface. The legacy agent.BuildRetrieveTool
// wrapper is preserved (internal/agent/retrieve_tool.go now delegates
// here) so existing call sites in internal/agent/agent.go keep
// compiling without churn.
package agenttools

import (
	"context"
	"fmt"
	"strings"
	"time"

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
//
// Mirrors internal/agent/retrieve_tool.go:38.
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
// through the full adk.Runner loop. BuildRunner (internal/agent/runner_setup.go)
// delegates to this function for the production wiring.
//
// This is a verbatim lift of internal/agent/retrieve_tool.go's
// BuildRetrieveTool — the only changes are the package name (agent
// → agenttools) and the inline chunkKey helper (which used to live in
// internal/agent/trust_floor.go). The chunkKey here is a local copy
// that produces the same decimal-string rendering the trust floor
// uses — a 3-line trivial function, no behavior change.
func BuildRetrieveTool(emb embedder.Embedder, ret retriever.Retriever, m *skill.Manifest) tool.InvokableTool {
	if emb == nil {
		panic("agenttools.BuildRetrieveTool: embedder is nil (caller bug — NewBuildRunner should pre-validate)")
	}
	if ret == nil {
		panic("agenttools.BuildRetrieveTool: retriever is nil (caller bug — NewBuildRunner should pre-validate)")
	}
	if m == nil {
		panic("agenttools.BuildRetrieveTool: manifest is nil (caller bug — NewBuildRunner should pre-validate)")
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
		Desc: "Retrieve compliance-regulation chunks from the knowledge bases this Skill is authorized to read (the reads declaration resolves to knowledge-base slugs via the registry; only authority-role material is citable). Call this tool when the user's question requires grounding in actual policy text. The query should be a focused compliance question or a refined search phrase; avoid single-word queries.",
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
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: query is required (cannot be empty or whitespace)")
		}

		vec, err := emb.Embed(ctx, query)
		if err != nil {
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: embed query: %w", err)
		}

		now := time.Now().Unix()
		hits, err := ret.Search(ctx, vec, tiers, retrieveTopK, now)
		if err != nil {
			return retrieveResponse{}, fmt.Errorf("retrieve_compliance_chunks: retriever.Search: %w", err)
		}

		return retrieveResponse{
			Chunks: FormatChunksAsData(hits),
			Count:  len(hits),
		}, nil
	}

	// utils.NewTool takes the explicit *schema.ToolInfo above plus the
	// typed invoke closure; the response JSON shape is inferred from the
	// generic InvokeFunc type parameters. Verified at
	// eino@v0.9.14/components/tool/utils/invokable_func.go:139-143.
	return utils.NewTool(desc, invoke)
}

// FormatChunksAsData renders a slice of retriever.Hit as the canonical
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
//
// Exported so the legacy agent.formatChunksAsData can be removed in
// 11-08 (final acceptance gate) without a re-implementation — the
// same byte-identical renderer lives here.
func FormatChunksAsData(hits []retriever.Hit) string {
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "[CHUNK %d] tier=%s source=%s\n%s\n---\n", h.ID, h.Tier, h.Source, h.Content)
	}
	return b.String()
}

// ChunkKey renders a retriever.Hit.ID (int64) as the canonical map key
// the accumulator uses for dedup and ChunksByID lookup. Centralized here
// so Add / TierMatched / ChunksByID / GroundingChecker all agree on the
// key format (the retrieve tool emits `[CHUNK %d]` using the same
// decimal rendering — T-2-21 framing + chunk ID format are paired).
//
// Avoids strconv to keep imports minimal; int64 → decimal is small.
//
// Duplicated from internal/agent/trust_floor.go:chunkKey (3-line
// trivial function; the duplication is intentional — see
// internal/agent/trust_floor.go godoc for the "deferred merge" note).
func ChunkKey(id int64) string {
	return fmt.Sprintf("%d", id)
}
