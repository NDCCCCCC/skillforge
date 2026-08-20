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

// Package agent — fanout_tools.go is Phase 13 / Plan 13-01's
// orchestrator LLM-facing Tool. The Tool declares the sub-queries
// the parent orchestrator's classifier LLM emits; the Tool itself
// is the FIRST layer of the FAN-02 SC-2 two-layer defense (the
// second layer is runtime.BuildSubAgent's intersection check).
//
// Why a Tool (vs a JSON-config injection point)?
//   - The orchestrator's LLM is the natural source of sub-queries —
//     a multi-hop query naturally splits into 2..N sub-queries by
//     LLM-driven classification. Injecting the sub-queries via a Tool
//     makes the structure explicit (the LLM sees the schema) and
//     auditable (every Tool invocation lands in the existing audit
//     trail via the eino v0.9.14 callback chain).
//
// Two-layer defense (Pitfall 2):
//   - Layer 1 (this Tool): rejects sub-queries whose tier_filter
//     contains a tier NOT in the parent's ResolvedTiers(). The Tool
//     sees the parent manifest at construction; it cannot be
//     re-bound by the LLM.
//   - Layer 2 (runtime.BuildSubAgent): rejects sub-queries whose
//     intersection with sub_skill.Reads is empty (FAN-02 SC-2
//     load-time error). Defense-in-depth — even if a future caller
//     bypasses the Tool and calls BuildSubAgent directly, the
//     intersection check still fires.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/skill"
)

// FanOutDispatchToolName is the orchestrator LLM-facing Tool's
// canonical name. Distinct from any HITL Tool name (D11-B3 shared
// source of truth) and from the retrieve_compliance_chunks Tool
// (the sub-agent's read-only Tool).
const FanOutDispatchToolName = "fan_out_dispatch"

// FanOutSubQuery is a single sub-query declaration emitted by the
// orchestrator's classifier LLM. ID must be unique within a single
// Tool invocation. Query is the sub-agent's per-Run prompt.
// TierFilter is the set of tiers the sub-agent may query (Layer 1
// validation enforces subset-of-parent.ResolvedTiers()).
type FanOutSubQuery struct {
	ID         string   `json:"id"`
	Query      string   `json:"query"`
	TierFilter []string `json:"tier_filter"`
}

// FanOutDispatchArgs is the Tool-args envelope. The orchestrator LLM
// emits one Tool call per fan-out with N sub-queries.
type FanOutDispatchArgs struct {
	SubQueries []FanOutSubQuery `json:"sub_queries"`
}

// fanOutDispatchTool is the LLM-facing Tool the orchestrator
// invokes. It is constructed with a reference to the parent
// manifest (the tier-set validator). The Tool is NOT a HITL Tool
// (it does NOT call tool.StatefulInterrupt — FAN-01 SC-1 invariant:
// sub-agents return compressed results, no HITL inside sub-agents).
type fanOutDispatchTool struct {
	parentManifest *skill.Manifest
}

// Compile-time interface compliance check: fanOutDispatchTool MUST
// satisfy tool.InvokableTool (eino v0.9.14 components/tool/interface.go:42).
var _ tool.InvokableTool = (*fanOutDispatchTool)(nil)

// BuildFanOutDispatchTool returns the orchestrator's classifier
// LLM-facing Tool. The parent manifest is REQUIRED — the Tool
// validates every tier_filter value against the parent's
// ResolvedTiers(). nil parentManifest is rejected with a panic
// (consistent with the P13-L2 invariant: the orchestrator pattern
// never passes nil).
func BuildFanOutDispatchTool(parentManifest *skill.Manifest) tool.InvokableTool {
	if parentManifest == nil {
		panic("agent.BuildFanOutDispatchTool: parentManifest is required (P13-L2 invariant)")
	}
	return &fanOutDispatchTool{parentManifest: parentManifest}
}

// Info returns the Tool's metadata for the LLM. The schema declares
// a single top-level `sub_queries` field (array of objects). The
// per-sub-query schema (id unique + query non-empty + tier_filter
// non-empty subset-of-parent) is enforced at InvokableRun time
// (the schema's nested-object representation is loose — runtime
// validation is the structural guarantee).
func (t *fanOutDispatchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: FanOutDispatchToolName,
		Desc: "Declare sub-queries for parallel fan-out. The orchestrator builds sub-agents per sub-query with tier-closure inheritance (sub-agent's tier scope = intersection of this Skill's tiers and the sub-query's tier_filter). Each sub-query MUST have: (a) a unique id, (b) a non-empty query, (c) a non-empty tier_filter where every tier is in this Skill's ResolvedTiers().",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"sub_queries": {
				Type: schema.Array,
				Desc: "The list of sub-queries to fan out. Each sub-query has {id, query, tier_filter}.",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"id": {
							Type:     schema.String,
							Desc:     "Unique sub-query identifier (e.g. \"sa1\").",
							Required: true,
						},
						"query": {
							Type:     schema.String,
							Desc:     "The sub-agent's per-Run prompt (compressed sub-query text).",
							Required: true,
						},
						"tier_filter": {
							Type:     schema.Array,
							Desc:     "Tiers the sub-agent may query. EVERY element MUST be in this Skill's ResolvedTiers().",
							Required: true,
							ElemInfo: &schema.ParameterInfo{
								Type: schema.String,
							},
						},
					},
				},
			},
		}),
	}, nil
}

// InvokableRun validates the LLM's Tool-call args. Two-layer defense
// (Pitfall 2):
//
//  1. Reject malformed JSON (the LLM emitted bad syntax).
//  2. Reject empty sub_queries (FAN-01 SC-1 requires >=1 sub-query).
//  3. Reject duplicate sub-query IDs (orchestrator cannot route
//     duplicated sub-queries).
//  4. Reject empty query or empty tier_filter per sub-query.
//  5. Reject tier values NOT in parent's ResolvedTiers() — the
//     first layer of FAN-02 SC-2 defense. The second layer is
//     runtime.BuildSubAgent's intersection check.
//
// Returns a JSON-encoded success string summarizing the sub-query
// count; error on any validation failure.
func (t *fanOutDispatchTool) InvokableRun(_ context.Context, argsInJSON string, _ ...tool.Option) (string, error) {
	var args FanOutDispatchArgs
	if err := json.Unmarshal([]byte(argsInJSON), &args); err != nil {
		return "", fmt.Errorf("fan_out_dispatch: invalid args: %w", err)
	}
	if len(args.SubQueries) == 0 {
		return "", fmt.Errorf("fan_out_dispatch: at least one sub-query required (FAN-01 SC-1 requires >=1 multi-hop sub-queries)")
	}
	parentTiers := t.parentManifest.ResolvedTiers()
	parentTierSet := make(map[string]bool, len(parentTiers))
	for _, tier := range parentTiers {
		parentTierSet[tier] = true
	}
	seenIDs := make(map[string]bool, len(args.SubQueries))
	for i, sq := range args.SubQueries {
		if sq.ID == "" {
			return "", fmt.Errorf("fan_out_dispatch: sub_query[%d] missing id", i)
		}
		if seenIDs[sq.ID] {
			return "", fmt.Errorf("fan_out_dispatch: duplicate sub_query id %q", sq.ID)
		}
		seenIDs[sq.ID] = true
		if sq.Query == "" {
			return "", fmt.Errorf("fan_out_dispatch: sub_query[%d] missing query", i)
		}
		if len(sq.TierFilter) == 0 {
			return "", fmt.Errorf("fan_out_dispatch: sub_query[%d] empty tier_filter", i)
		}
		for _, tier := range sq.TierFilter {
			if !parentTierSet[tier] {
				return "", fmt.Errorf("fan_out_dispatch: sub_query[%d] tier %q not in parent ResolvedTiers() %v (two-layer defense per P13-L2; intersects with sub_skill Reads at BuildSubAgent)",
					i, tier, parentTiers)
			}
		}
	}
	return fmt.Sprintf(`{"declared_sub_queries": %d, "status": "ok"}`, len(args.SubQueries)), nil
}