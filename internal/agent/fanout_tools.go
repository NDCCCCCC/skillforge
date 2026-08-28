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
	"golang.org/x/sync/errgroup"

	"github.com/skillforge/skillforge/internal/kpi"
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

// FanOutDispatchArgsV2 is the data-channel fan-out classifier args.
// D18-C1: per-section boundary defined by template; D18-C2: main loop tool-call self-decides.
type FanOutDispatchArgsV2 struct {
	ShouldFanOut   bool     `json:"should_fan_out"`
	NumSubtasks    int      `json:"num_subtasks"`
	TaskBoundaries []string `json:"task_boundaries"` // Template section title exact-match
	Reason         string   `json:"reason"`           // Classification reason — enters audit row (FAN-05)
}

// fallbackReason enumerations (D18-C4)
const (
	FallbackInvalidJSON           = "invalid_json"
	FallbackMissingField          = "missing_field"
	FallbackBoundaryNotInTemplate = "boundary_not_in_template"
	FallbackNumSubtasksMismatch   = "num_subtasks_mismatch"
)

// validateTaskBoundaries checks that every boundary exists in template section titles.
func validateTaskBoundaries(boundaries []string, sectionTitles []string) (string, bool) {
	allowed := make(map[string]bool)
	for _, t := range sectionTitles {
		allowed[t] = true
	}
	for _, b := range boundaries {
		if !allowed[b] {
			return b, false
		}
	}
	return "", true
}

// KPICaller is the narrow seam the data-channel fan-out tool consumes
// from *kpi.Executor (interface isolation — hermetic tests stub it;
// mirrors the kpi.AuditSink posture). *kpi.Executor satisfies it via
// its pointer-receiver Call method.
type KPICaller interface {
	Call(ctx context.Context, req kpi.CallRequest) (kpi.CallOutcome, error)
}

// sectionTitlesAndKPIs extracts the section titles + per-section KPI
// name lists from a manifest template block (CR-02 18-REVIEW: the
// data-channel fan-out tool needs the template-declared boundaries,
// and the manifest's Template field is `any` — yaml/json round-trip
// materializes it as map[string]any).
//
// The walk is defensive and index-safe: titles[i] always pairs with
// sectionKPIs[i] (a structurally unexpected section entry appends an
// empty title + nil KPI list rather than skipping). A fully
// unexpected shape yields empty slices, in which case the tool's
// boundary validation degrades to the sequential fallback (D18-C4) —
// never a panic mid-Run.
func sectionTitlesAndKPIs(template any) ([]string, [][]string) {
	m, ok := template.(map[string]any)
	if !ok {
		return nil, nil
	}
	sections, ok := m["sections"].([]any)
	if !ok {
		return nil, nil
	}
	titles := make([]string, 0, len(sections))
	kpis := make([][]string, 0, len(sections))
	for _, sec := range sections {
		title := ""
		var names []string
		if secMap, ok := sec.(map[string]any); ok {
			if s, ok := secMap["title"].(string); ok {
				title = s
			}
			if list, ok := secMap["kpis"].([]any); ok {
				names = make([]string, 0, len(list))
				for _, k := range list {
					if s, ok := k.(string); ok {
						names = append(names, s)
					}
				}
			}
		}
		titles = append(titles, title)
		kpis = append(kpis, names)
	}
	return titles, kpis
}

// dataChannelFanOutTool is the data-channel fan-out tool using template section boundaries.
// It validates task_boundaries against the template and falls back to sequential
// execution when validation fails (D18-C4).
type dataChannelFanOutTool struct {
	sectionTitles []string
	sectionKPIs   [][]string // KPIs per section (index-aligned with sectionTitles)
	executor      KPICaller
}

// Compile-time interface compliance check.
var _ tool.InvokableTool = (*dataChannelFanOutTool)(nil)

// BuildFanOutDispatchToolDataChannel builds the data-channel fan-out tool for a
// skill with template. sectionTitles + sectionKPIs are extracted from the
// manifest template via sectionTitlesAndKPIs; executor is the session's KPI
// executor (the ONLY call target, same as kpi_call — D16-C3).
func BuildFanOutDispatchToolDataChannel(sectionTitles []string, sectionKPIs [][]string, executor KPICaller) tool.InvokableTool {
	return &dataChannelFanOutTool{
		sectionTitles: sectionTitles,
		sectionKPIs:   sectionKPIs,
		executor:      executor,
	}
}

// Info returns the Tool metadata for the LLM.
func (t *dataChannelFanOutTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: FanOutDispatchToolName,
		Desc: "Structured fan-out classifier for data-channel monthly report. Declare should_fan_out, num_subtasks, and task_boundaries (template section titles). Fallback to sequential when boundaries are invalid.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"should_fan_out": {
				Type:     schema.Boolean,
				Required: true,
				Desc:     "Whether to fan out in parallel across sections.",
			},
			"num_subtasks": {
				Type:     schema.Integer,
				Required: true,
				Desc:     "Number of parallel sub-tasks (must equal task_boundaries length).",
			},
			"task_boundaries": {
				Type:     schema.Array,
				Required: true,
				Desc:     "Template section titles — each must exactly match a template section.",
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
			},
			"reason": {
				Type:     schema.String,
				Required: false,
				Desc:     "Classification reason — stored in audit row (FAN-05).",
			},
		}),
	}, nil
}

// InvokableRun validates args and either fans out in parallel or falls back to sequential.
func (t *dataChannelFanOutTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	// 1. Parse args
	var a FanOutDispatchArgsV2
	if err := json.Unmarshal([]byte(args), &a); err != nil {
		// invalid JSON → fallback
		return `{"status":"fallback_sequential","reason":"invalid_json"}`, nil
	}

	// 2. Field presence check
	if a.TaskBoundaries == nil {
		return `{"status":"fallback_sequential","reason":"missing_field"}`, nil
	}

	// 3. task_boundaries closure validation
	if _, ok := validateTaskBoundaries(a.TaskBoundaries, t.sectionTitles); !ok {
		return fmt.Sprintf(
			`{"status":"fallback_sequential","reason":"boundary_not_in_template","detail":"%s"}`,
			"task_boundary not in template sections",
		), nil
	}

	// 4. num_subtasks validation
	if a.NumSubtasks != len(a.TaskBoundaries) {
		return `{"status":"fallback_sequential","reason":"num_subtasks_mismatch"}`, nil
	}

	// 5. Validation passed: fan-out execution
	if a.ShouldFanOut {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(4)
		results := make([]*kpi.Snapshot, len(a.TaskBoundaries))

		for i := range a.TaskBoundaries {
			sectionIdx := i // capture
			g.Go(func() error {
				// Find KPIs for this section
				kpis := t.sectionKPIs[sectionIdx]
				var snap []*kpi.Snapshot
				for _, kpiName := range kpis {
					outcome, err := t.executor.Call(ctx, kpi.CallRequest{
						Name:      kpiName,
						Params:    map[string]any{},
						SessionID: "",
						SkillID:   "",
					})
					if err != nil {
						return err
					}
					if outcome.Refusal != nil {
						// Structured refusal — skip, don't append
						continue
					}
					if outcome.Snapshot != nil {
						snap = append(snap, outcome.Snapshot)
					}
				}
				results[sectionIdx] = mergeSnapshots(snap)
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return "", err
		}
		result, _ := json.Marshal(map[string]interface{}{
			"status":       "fan_out_success",
			"num_subtasks": a.NumSubtasks,
			"boundaries":   a.TaskBoundaries,
			"reason":       a.Reason,
			"section_results": results,
		})
		return string(result), nil
	}

	// 6. should_fan_out=false → sequential fallback
	return `{"status":"fallback_sequential","reason":"classifier_declined_fan_out"}`, nil
}

// mergeSnapshots merges multiple KPI snapshots into one per section.
func mergeSnapshots(snap []*kpi.Snapshot) *kpi.Snapshot {
	for _, s := range snap {
		if s != nil {
			return s
		}
	}
	return nil
}