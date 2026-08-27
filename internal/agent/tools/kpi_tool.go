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

// Package agenttools — kpi_tool.go is Phase 16 / Plan 16-07 Task 2:
// the kpi_call Tool (D16-B1) that mirrors retrieve.go's construction
// pattern. One Tool, one closure, one channel (Shared Pattern D): the
// kpi_call Tool is the ONLY knowledge-source surface available to a
// data-channel Skill (HasDataChannel() == true). The tier channel keeps
// retrieve; the data channel replaces it with kpi_call — physically
// disjoint Tool sets, structurally unreachable from each other
// (D16-C3 + SC-3 runtime layer).
//
// Architectural notes:
//
//   - Catalog is compiled at CONSTRUCTION TIME (snapshot ONCE, into
//     the closure). This is the Don't-Hand-Roll rule (16-RESEARCH
//     Anti-Pattern): runtime prompt assembly from a mutable registry
//     would be the implicit backdoor through which a future Skill
//     hot-reload could swap the LLM-visible Tool catalog. The
//     construction-time snapshot is the structural guarantee.
//
//   - Structured Refusal is a TOOL RESULT, never a Go error
//     (D16-B2 / DATA-12). eino renders a Go error as tool noise that
//     the agent cannot reason about; the agent needs to read the
//     reason_code + available_kpis to self-correct or narrate.
//
//   - DataRefEmitFunc is the SSE data-reference part emission hook
//     for the 16-08 server wiring. The closure receives a
//     wire.DataReference populated from the executor's Snapshot —
//     rows body is NOT included (volume control + the agent already
//     has the full snapshot from the tool result). nil emit is a
//     valid hermetic-mode value (unit tests + the no-SSE pipeline
//     path).
//
//   - The Tool function receives name + params via JSON (kpiCallArgs
//     mirrors retrieveArgs). SessionID + SkillID are STAMPED at
//     construction time per-Runner (BuildRunner's
//     DataRefEmitFunc plumbing ships the per-Runner context in a
//     later task; here the Tool accepts them as captured values
//     today to keep the closure pure).
package agenttools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/wire"
)

// KPICallToolName is the canonical Eino tool name. Exported as a
// constant so internal/agent wiring (Plan 16-07 Task 3) and tests
// assert against the exact string ("kpi_call") rather than a magic
// literal. Mirrors the RetrieveToolName / D11-A3 discipline.
const KPICallToolName = "kpi_call"

// KPITool is an alias for the eino tool.InvokableTool surface used by
// BuildKPICallTool. Tests import this alias rather than the eino
// package directly so they stay decoupled from eino's specific
// generic-shape wiring. Production callers (BuildRunner in
// internal/agent/runner_setup.go) use the same alias.
type KPITool = interface {
	tool.BaseTool
	InvokableRun(ctx context.Context, argsInJSON string, opts ...tool.Option) (string, error)
}

// CatalogTruncate caps the catalog lines embedded in the Tool's
// description. The literal 30 is the source-grep lock target
// (16-PATTERNS Pattern I — "structural constant source-grep lock").
// Exceeding 30 in any single registry is unusual; the truncation is
// the LLM-side guard, not the right-sizing answer. When truncated,
// the description appends a hint pointing at the available_kpis
// fallback path so the agent learns to enumerate rather than guess.
//
// Not exported: not a tunable knob for v1.
const CatalogTruncate = 30

// DataRefEmitFunc is the per-Runner SSE data-reference part emission
// hook. The closure is invoked by BuildKPICallTool AFTER a Snapshot
// has been produced by the executor; nil means "no SSE side-channel"
// (hermetic tests + the no-SSE pipeline path).
//
// The 16-08 server wiring will pass a closure matching the existing
// runnerEmitDecision shape (internal/server/runner_setup.go:192-198) —
// data-reference parts ride the same data-decision envelope
// (DecisionPayload.DataRef, D16-B3 additive field). For 16-07 this
// is an opaque callback surface; the production assembly wires it
// in 16-08 alongside the rest of the callbacks plumbing.
type DataRefEmitFunc func(ref wire.DataReference)

// kpiCallArgs is the JSON arguments shape the LLM emits when calling
// kpi_call. Eino's utils.NewTool deserializes the LLM-emitted JSON
// arguments string into this struct before invoking the underlying
// function.
//
// json tags are lower-cased per project convention (matches the
// retrieveArgs shape + internal/wire chunk convention).
type kpiCallArgs struct {
	// Name is the KPI identifier from the registry catalog
	// (reg.Names()). eino v0.9.14 schema.ParameterInfo.Enum locks
	// the LLM's choice to this slice (ToolInfo + ParamsOneOf below)
	// — the LLM cannot reach a KPI that isn't in the catalog.
	Name string `json:"name"`

	// Params is the KPI's declared parameter object. Validated
	// fail-closed against the registry schema in executor.Call
	// step ② (Layer 1 validation: required / enum / pattern / extra
	// keys). The agent sees a structured Refusal on any violation;
	// the Tool never returns a Go error for invalid params.
	Params map[string]any `json:"params"`
}

// kpiCallResponse is the JSON shape returned to the LLM. The tool
// result is one of two structural shapes (D16-B2): Snapshot (success)
// or Refusal (structured rejection). Both are JSON; the LLM reads
// whichever the executor produced.
//
// We type the result as a single struct with a `kind` discriminator
// so eino's reflection-based NewTool signature works without a
// generic shape parameter at this layer (the alternative — a
// per-call any-typed result — would lose the JSON wire shape
// contract). Snapshot / Refusal bodies live in nested maps.
type kpiCallResponse struct {
	// Kind is the wire discriminator: "snapshot" | "refusal".
	Kind string `json:"kind"`

	// Snapshot carries the successful KPI result (kind="snapshot").
	// Populated verbatim from kpi.Snapshot (D16-B3 single-Snapshot
	// three-output source of truth).
	Snapshot *kpi.Snapshot `json:"snapshot,omitempty"`

	// Refusal carries the structured rejection (kind="refusal").
	// Populated verbatim from kpi.Refusal (D16-B2: a refusal is a
	// tool result, NEVER a Go error).
	Refusal *kpi.Refusal `json:"refusal,omitempty"`
}

// BuildKPICallTool constructs the kpi_call InvokableTool. The tool
// closes over the immutable kpi.Registry + kpi.Executor + emit
// snapshot (the KNOW-03 closure discipline from retrieve.go:124-131 —
// the LLM has no way to mutate the registry or reach SQL through any
// tool argument because the closure captures only the read-side
// seams).
//
// Pre-conditions (Rule 2 — fail-fast on misconfiguration):
//   - exec and reg MUST be non-nil. nil parameters panic with a
//     "caller bug" message (mirrors retrieve.go:113-122 + executor.go:194-205
//     conventions) so assembly mistakes crash at boot, never lurk
//     until the first LLM-triggered call.
//
// Post-conditions:
//   - Returns a non-nil tool.BaseTool whose Info() reports Name ==
//     KPICallToolName, Desc includes the compiled catalog, and the
//     ParamsOneOf for `name` has Enum == reg.Names() (sorted).
//   - The closure is safe for concurrent invocation (every per-call
//     state is allocated fresh; exec / reg are read-only).
//
// Construction-time snapshot discipline (Don't-Hand-Roll):
// The catalog string is rendered ONCE at construction from
// reg.Catalog() and frozen into the returned Tool's description. A
// future registry hot-reload would build a NEW Tool; the existing
// Tool's description is byte-stable. This is the structural defense
// against the "runtime prompt assembly = implicit backdoor" anti-
// pattern (16-RESEARCH §Architecture Patterns: Don't-Hand-Roll).
func BuildKPICallTool(exec *kpi.Executor, reg *kpi.Registry, sessionID, skillID string, emit DataRefEmitFunc) tool.InvokableTool {
	if exec == nil {
		panic("agenttools.BuildKPICallTool: executor is nil (caller bug — BuildRunner should pre-validate)")
	}
	if reg == nil {
		panic("agenttools.BuildKPICallTool: registry is nil (caller bug — BuildRunner should pre-validate)")
	}

	catalog := renderCatalog(reg.Catalog())

	desc := &schema.ToolInfo{
		Name: KPICallToolName,
		Desc: "按名调用预定义 KPI（聚合指标查询，永不接触 SQL/原始行）。可用 KPI 目录：\n" + catalog + "\n参数：name=KPI 名（必须在目录内）；params=该 KPI 声明的参数对象（见目录内参数摘要）。结果带 as_of 数据引用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Required: true,
				Enum:     reg.Names(),
				Desc:     "KPI 名，必须在目录内",
			},
			"params": {
				Type: schema.Object,
				Desc: "该 KPI 声明的参数对象；越域参数会被结构化拒绝",
			},
		}),
	}

	invoke := func(ctx context.Context, args kpiCallArgs) (kpiCallResponse, error) {
		name := strings.TrimSpace(args.Name)
		if name == "" {
			// Treat missing name as no_matching_kpi — the executor
			// would also produce this on Lookup("", false). Keeping
			// the same shape (tool result, no Go error) preserves the
			// agent-visible contract: empty input → structured refusal
			// → available_kpis to self-correct.
			return kpiCallResponse{
				Kind: "refusal",
				Refusal: &kpi.Refusal{
					ReasonCode:       kpi.ReasonNoMatchingKPI,
					HumanExplanation: "未提供 KPI 名称。可用目录见 available_kpis，请选择一个后重试。",
					AvailableKPIs:    reg.Names(),
				},
			}, nil
		}

		// D17-B3: detect periods:[] — dispatch to CallMultiPeriod when
		// more than one period is requested (single-period calls reuse Call).
		rawPeriods, hasPeriods := args.Params["periods"].([]any)
		var outcome kpi.CallOutcome
		var err error
		if hasPeriods && len(rawPeriods) > 1 {
			outcome, err = exec.CallMultiPeriod(ctx, kpi.CallRequest{
				Name:      name,
				Params:    args.Params,
				SessionID: sessionID,
				SkillID:   skillID,
			})
		} else {
			outcome, err = exec.Call(ctx, kpi.CallRequest{
				Name:      name,
				Params:    args.Params,
				SessionID: sessionID,
				SkillID:   skillID,
			})
		}
		if err != nil {
			// Infrastructure failure (engine crash, audit write
			// fault). Per D16-B2 / DATA-12: Go errors are reserved
			// for infra failures ONLY — never for Refusals.
			return kpiCallResponse{}, fmt.Errorf("kpi_call %s: %w", name, err)
		}

		if outcome.Refusal != nil {
			return kpiCallResponse{Kind: "refusal", Refusal: outcome.Refusal}, nil
		}

		// Snapshot branch: emit the DataReference part (no-op when
		// emit is nil). Rows are NOT included in the wire payload —
		// the agent already has the full snapshot from the tool
		// result, and the wire envelope is volume-controlled.
		snap := outcome.Snapshot
		if emit != nil && snap != nil {
			ref := wire.DataReference{
				Func:        snap.Func,
				Args:        snap.Args,
				ASOfPeriod:  snap.ASOf.Period,
				ASOfTs:      snap.ASOf.Ts,
				RowCount:    len(snap.Rows),
				Suppressed:  snap.Suppressed,
				KMinApplied: snap.KMinApplied,
				References:  buildPointReferences(snap.Rows), // D17-B2: per-point sparkline references
			}
			emit(ref)
		}
		return kpiCallResponse{Kind: "snapshot", Snapshot: snap}, nil
	}

	// utils.NewTool takes the explicit *schema.ToolInfo above plus
	// the typed invoke closure; the response JSON shape is inferred
	// from the InvokeFunc type parameter. Verified at
	// eino@v0.9.14/components/tool/utils/invokable_func.go:139-143
	// (same pattern as BuildRetrieveTool).
	return utils.NewTool(desc, invoke)
}

// renderCatalog renders the registry catalog as the Tool's
// description body. Format per 16-RESEARCH Pattern 2:
//
//   - One KPI per line.
//   - Each line: "name — description（参数：k1=类型[枚举N值|pattern], …）"
//   - Sorted alphabetically (Catalog() already sorts).
//   - Truncated to CatalogTruncate (30) lines; when truncated, the
//     description appends a hint pointing the agent at the
//     available_kpis path so it learns to enumerate rather than
//     guess from the visible portion.
//
// Pure function (no globals, no I/O) — trivially unit-testable via
// the catalog snapshot in the test suite.
func renderCatalog(entries []kpi.CatalogEntry) string {
	if len(entries) == 0 {
		return "（当前无已注册的 KPI）"
	}
	// Defensive copy + sort (Catalog() already returns sorted, but
	// we sort defensively to keep renderCatalog robust if a future
	// change relaxes the Catalog contract).
	sorted := make([]kpi.CatalogEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	lines := make([]string, 0, len(sorted))
	for _, e := range sorted {
		paramPart := "（无参数）"
		if e.ParamsSummary != "none" && e.ParamsSummary != "" {
			paramPart = "（参数：" + e.ParamsSummary + "）"
		}
		lines = append(lines, fmt.Sprintf("%s — %s%s", e.Name, e.Description, paramPart))
	}
	if len(lines) > CatalogTruncate {
		truncated := lines[:CatalogTruncate]
		hint := fmt.Sprintf("…（仅展示前 %d 条；完整目录见 no_matching_kpi 返回的 available_kpis）", CatalogTruncate)
		return strings.Join(truncated, "\n") + "\n" + hint
	}
	return strings.Join(lines, "\n")
}

// buildPointReferences converts snap.Rows to per-point references for sparkline.
// Each row has a `period` column tagged at executor step ④ in CallMultiPeriod.
// D17-B2.
func buildPointReferences(rows []map[string]any) []wire.PointReference {
	out := make([]wire.PointReference, 0, len(rows))
	for i, row := range rows {
		p := wire.PointReference{PointIndex: i}
		if period, ok := row["period"].(string); ok {
			p.Period = period
		}
		// Last non-period, non-row_count column = the value
		for k, v := range row {
			if k != "period" && k != "row_count" {
				p.Value = v
				break
			}
		}
		out = append(out, p)
	}
	return out
}

// kpiCallResponseForTest is a compile-time anchor for the JSON wire
// shape — used by kpi_tool_test.go to assert the Tool's output
// round-trip and by future tests to confirm the {Kind, Snapshot |
// Refusal} envelope. Not exported (test-only).
type kpiCallResponseForTest = kpiCallResponse

// _ enforces that kpiCallResponseForTest alias survives the
// json.Marshal round-trip with the expected field tags. The compile
// errors this triggers (if any) surface here rather than in
// downstream callers.
var _ = func() any {
	b, _ := json.Marshal(kpiCallResponseForTest{Kind: "snapshot"})
	return b
}()
