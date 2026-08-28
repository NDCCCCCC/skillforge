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

// hitl_plan_approve.go is Phase 12 / Plan 12-03's plan confirmation
// HITL Tool (PLAN-03 / Plan 8-3 deep_agent plan approval gate). It
// mirrors hitl_review_report.go's two-pass interrupt/resume contract
// verbatim with target="plan_approve" — reuses ApprovalDecision schema
// (D11-B2), audit_writer seam (D11-C3), dual-table snapshot (D11-C2),
// fail-closed posture (P12-L7 zero-schema-divergence).
//
// The deny semantics = "forced_stop" data-decision (D16 overload):
// when the operator denies a plan, the Tool returns a message that
// signals the upstream chatHandler to emit data-decision{decision:
// "forced_stop", reason:"plan denied"}. The agent then rewrites the
// plan and re-submits by issuing another plan_approve call (capped
// by the budget gate's MaxIteration: 10).
//
// Architectural note: the unexported planApproveTool struct mirrors
// reviewReportTool exactly — the auditWriterHook interface +
// noopAuditWriter are shared with hitl_memory_facts.go (D11-B3 single
// source of truth). The registry (registry.go) registers the Tool via
// `hitlHandlers["plan_approve"] = &planApproveTool{}` — the
// D11-B3 single-source-of-truth pattern.
package hitl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// PlanApproveToolName is the Eino tool name emitted by the LLM in
// tool_calls. Distinct from PlanApproveTarget ("plan_approve") on
// purpose (D11-B2): the Tool NAME is what the LLM writes to
// tool_calls[i].Function.Name; the TARGET ("plan_approve") is the
// policy routing key carried in ApprovalInfo.Target and echoed back
// through ApprovalDecision.Target. They live in different namespaces
// (LLM-visible vs operator-visible) and must not collide.
const PlanApproveToolName = "plan_approve_review"

// PlanApproveTarget is the canonical target string for the
// plan_approve write (the policy string in ApprovalInfo.Target and
// the audit row's target column). Matches the AllowedWriteTargets
// entry added in Plan 12-03.
const PlanApproveTarget = "plan_approve"

// PlanApproveArgs is the typed payload schema for the plan_approve
// Tool (PLAN-03 SC-3 / 11-CONTEXT §Claude's Discretion E — schema name
// + target whitelist are locked upstream; the concrete field shape
// is planner's call).
//
//   - PlanID: stable identifier for the proposed plan. The agent
//     reuses the same PlanID on rewrite-and-re-submit so the audit
//     chain (Pass 1 + Pass 2) is linkable across iterations.
//   - Steps: ordered plan steps. Empty Steps is rejected at parse time.
//   - EstimatedIterations: self-reported budget for the run. Caps the
//     deep_agent loop's MaxIteration (P12-L2 budget gate).
//   - RiskAssessment: free-form text the operator reviews at approval
//     time ("low"/"medium"/"high"/"catastrophic" — the schema does not
//     enforce an enum; the Tool passes the value verbatim).
type PlanApproveArgs struct {
	PlanID              string     `json:"planId"`
	Steps               []PlanStep `json:"steps"`
	EstimatedIterations int        `json:"estimatedIterations"`
	RiskAssessment      string     `json:"riskAssessment"`
	// D18-D2: previous run context for plan_approve inheritance.
	// When non-nil, the ApprovalCard shows "based on v{N} version" notice
	// and modification notes are injected into section narratives.
	PrevRunContext *PrevRunContext `json:"prevRunContext,omitempty"`
}

// PrevRunContext carries the previous run metadata for D18-D2 plan_approve inheritance.
type PrevRunContext struct {
	Period          string `json:"period"`           // previous run period
	TemplateVersion string `json:"template_version"` // template sha256 prefix
	PrevVersion     int    `json:"prev_version"`    // previous report version
	ModNotes        string `json:"modification_notes"` // user modification notes
}

// PlanStep is a single step in a PlanApproveArgs.Steps list. Mirrors
// the shape of wire.PlanTodo (12-02) for consistency — both phases
// agree on the {ID, Content, ActiveForm, Status} quartet so the FE
// can render plan_approve ApprovalCards and plan_todos TodoCards with
// the same component primitive.
//
// Phase 12 owns the wire.PlanTodo type (12-02 is in flight); this
// file declares its own struct because:
//
//  1. hitl_plan_approve.go is in package hitl (not wire); wire imports
//     would create a dependency cycle since hitl is the D11-B3
//     source-of-truth and wire depends on hitl.
//  2. The two structs share a structural shape but live at different
//     edges (audit-row frozen_args vs SSE data-plan-trace chunk) —
//     keeping them as parallel types prevents cross-package drift.
type PlanStep struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm,omitempty"`
	Status     string `json:"status"`
}

// planApproveTool is the HITL Tool for PLAN-03 plan confirmation.
// Stateless beyond the `audit` field — the interrupt JSON and the
// ApprovalDecision live in the eino Runner's checkpoint, not on the
// Tool instance (same posture as reviewReportTool).
type planApproveTool struct {
	// audit may be nil; nil-safe via auditOrDefault(). The production
	// wiring (runner_setup via hitl.InjectDeps — WR-01) injects the
	// real AuditWriter.
	audit auditWriterHook
}

// setAudit wires the production audit hook (WR-01); called via the
// auditBinder interface from hitl.InjectDeps.
func (h *planApproveTool) setAudit(hook auditWriterHook) { h.audit = hook }

// auditOrDefault returns the configured audit writer, or a noop if
// nil (same helper shape as reviewReportTool.auditOrDefault).
func (h *planApproveTool) auditOrDefault() auditWriterHook {
	if h.audit == nil {
		return noopAuditWriter{}
	}
	return h.audit
}

// Info returns the LLM-visible tool schema. PlanID + Steps are
// REQUIRED; EstimatedIterations and RiskAssessment are optional but
// strongly encouraged — the operator reviewing the ApprovalCard
// (12-07) uses both to decide approve vs deny.
func (h *planApproveTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: PlanApproveToolName,
		Desc: "Submit a proposed plan for user approval before execution. " +
			"The plan includes N steps with estimated iterations and risk assessment. " +
			"Requires human approval before the agent run proceeds. " +
			"On deny, the agent rewrites the plan and re-submits (capped by MaxIteration).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"planId": {
				Type:     schema.String,
				Required: true,
				Desc:     "Stable plan identifier. The agent reuses the same PlanID on rewrite-and-re-submit so the audit chain links across iterations.",
			},
			"steps": {
				Type:     schema.Array,
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"id":         {Type: schema.String, Required: true, Desc: "Step identifier (e.g. \"step-1\")."},
						"content":    {Type: schema.String, Required: true, Desc: "Step body (markdown). The operator reviews this text at approval time."},
						"activeForm": {Type: schema.String, Required: false, Desc: "Present-tense verb form (e.g. \"Analyzing chunks\"). Optional."},
						"status":     {Type: schema.String, Required: true, Desc: "Step status at submission time (\"pending\" / \"in_progress\" / \"done\")."},
					},
				},
				Desc: "Ordered plan steps. Empty list is rejected at parse time.",
			},
			"estimatedIterations": {
				Type:     schema.Integer,
				Required: false,
				Desc:     "Self-reported budget for the run. Caps the deep_agent loop's MaxIteration (P12-L2 budget gate).",
			},
			"riskAssessment": {
				Type:     schema.String,
				Required: false,
				Desc:     "Free-form risk level (\"low\" / \"medium\" / \"high\" / \"catastrophic\"). Surface to the operator verbatim.",
			},
		}),
	}, nil
}

// InvokableRun is the two-pass HITL Tool body.
//
// Pass 1 (no prior interrupt on this address):
//
//   1. Parse + validate PlanApproveArgs (PlanID + Steps non-empty).
//   2. Resolve approvalID from compose.GetToolCallID, fall back to
//      args.PlanID (D11-C4 single identity invariant).
//   3. Build ApprovalInfo with target=PlanApproveTarget.
//   4. audit_writer.InsertPendingApproval — D11-C3 sync INSERT
//      BEFORE tool.StatefulInterrupt (fail-closed posture).
//   5. tool.StatefulInterrupt — signals the interrupt.
//
// Pass 2 (resumed after operator decision): three branches — not the
// resume target → re-interrupt (leaf contract preservation); is target
// + no data → framework glitch (reject); is target + data → Validate
// the decision, then deny (forced_stop message) or approve (execute).
//
// The Tool does NOT call any chat model — deterministic logic only.

// buildPlanApproveReason builds the approval Reason string for the audit row.
// D18-D2: when PrevRunContext is non-nil, includes "based on v{N} version" notice.
func buildPlanApproveReason(args PlanApproveArgs) string {
	base := fmt.Sprintf(
		"Agent proposed a plan with %d steps (estimated %d iterations, risk: %s)",
		len(args.Steps), args.EstimatedIterations, args.RiskAssessment,
	)
	if args.PrevRunContext != nil {
		prev := args.PrevRunContext
		return fmt.Sprintf(
			"%s | Based on previous run: v%d (period=%s, template=%s) — %s",
			base, prev.PrevVersion, prev.Period, prev.TemplateVersion, prev.ModNotes,
		)
	}
	return base
}

func (h *planApproveTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// Pass 1 — first execution. Parse + validate before audit.
		var parsed PlanApproveArgs
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return "", fmt.Errorf("plan_approve_review: invalid args: %w", err)
		}
		if parsed.PlanID == "" {
			return "", errors.New("plan_approve_review: planId is required")
		}
		if len(parsed.Steps) == 0 {
			return "", errors.New("plan_approve_review: at least one step is required")
		}

		// D11-C4 — single identity invariant: approval_id == tool_call_id.
		// v0.9.14's compose.GetToolCallID may return "" inside the
		// Tool's own InvokableRun (the outer's tool_call_id doesn't
		// propagate); fall back to args.PlanID so the audit row's PK
		// is still a stable identifier across iterations of the
		// agent's rewrite-and-re-submit loop.
		approvalID := compose.GetToolCallID(ctx)
		if approvalID == "" {
			approvalID = parsed.PlanID
		}
		info := &ApprovalInfo{
			ToolName:        PlanApproveToolName,
			ArgumentsInJSON: args,
			ToolCallID:      approvalID,
			Target:          PlanApproveTarget,
			Reason: buildPlanApproveReason(parsed),
		}
		// D11-C3 — sync audit INSERT BEFORE StatefulInterrupt. A
		// fail-closed audit write returns BEFORE the interrupt is
		// signalled; the Runner never checkpoints, never executes.
		if err := h.auditOrDefault().InsertPendingApproval(ctx, approvalID, info); err != nil {
			return "", fmt.Errorf("plan_approve_review: audit INSERT failed (fail-closed): %w", err)
		}
		return "", tool.StatefulInterrupt(ctx, info, args)
	}

	// Pass 2 — resumed execution.
	isTarget, hasData, data := tool.GetResumeContext[*ApprovalDecision](ctx)
	if !isTarget {
		// Leaf contract preservation (adk/runner.go:137-147): not our
		// turn → re-interrupt with the stored args so the checkpoint
		// state survives for the eventual targeted resume.
		return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
			ToolName:        PlanApproveToolName,
			ArgumentsInJSON: storedArgs,
			ToolCallID:      "",
			Target:          PlanApproveTarget,
			Reason:          "resume-not-target",
		}, storedArgs)
	}
	if !hasData {
		return "", errors.New("plan_approve_review: resumed without data (no ApprovalDecision injected)")
	}
	if err := data.Validate(); err != nil {
		return "", fmt.Errorf("plan_approve_review: invalid ApprovalDecision: %w", err)
	}

	// Parse the stored args so the approve/deny message can echo
	// plan-shape data (step count + estimated iterations).
	var parsed PlanApproveArgs
	_ = json.Unmarshal([]byte(storedArgs), &parsed) // best-effort; empty args surface as 0/0

	if data.Decision == "deny" {
		// PLAN-03 deny semantics = forced_stop (D16 overload).
		// The approval endpoint (12-04) writes the MarkRejected audit
		// row; the Tool returns the deny reason so the chat-visible
		// response carries the operator's rationale + the "rewrite
		// and re-submit" instruction for the agent.
		return fmt.Sprintf(
			"user denied plan: %s. Agent will rewrite plan and re-submit.",
			data.Reason,
		), nil
	}

	// Approve path. The downstream deep_agent run (12-02 deep_audit.go)
	// resumes from the approved plan; this Tool returns a structured
	// success string so the chatHandler can render the verdict
	// deterministically.
	return fmt.Sprintf(
		"Plan approved: %d steps, %d estimated iterations",
		len(parsed.Steps), parsed.EstimatedIterations,
	), nil
}

// Compile-time check: planApproveTool MUST satisfy tool.InvokableTool
// so the eino compose.ToolsNodeConfig accepts it.
var _ tool.InvokableTool = (*planApproveTool)(nil)
