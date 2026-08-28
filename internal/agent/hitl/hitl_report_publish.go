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

// hitl_report_publish.go is Phase 18 / Plan 18-01's report_publish HITL Tool.
// It mirrors hitl_plan_approve.go's two-pass interrupt/resume contract:
// Pass 1 validates + interrupts for operator approval; Pass 2 renders
// and publishes the report. The tool is registered in hitlHandlers
// (D11-B3 pattern) so it is reachable via hitl.LookupHandler.
//
// The actual render + vN increment + os.WriteFile orchestration
// lives in internal/report/publish.go (Task 2). This tool delegates to
// report.Publish after operator approval.
package hitl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/report"
)

// ReportPublishToolName is the Eino tool name emitted by the LLM.
// Distinct from the write target ("report:publish") on purpose (D11-B2).
const ReportPublishToolName = "report_publish"

// ReportPublishTarget is the canonical target string for the report:publish
// write (the policy string in ApprovalInfo.Target). Matches the
// AllowedWriteTargets entry.
const ReportPublishTarget = "report:publish"

// ReportPublishArgs is the typed payload for the report_publish Tool.
// The args encode the slot texts and KPI snapshot needed to render the report.
type ReportPublishArgs struct {
	Period     string                  `json:"period"`              // e.g. "2026-08"
	Template   any                   `json:"template"`           // *report.Template — type-asserted at publish time
	Slots      map[int]map[int]string `json:"slots"`             // sectionIdx -> slotIdx -> text
	SkillID    string                  `json:"skillId"`           // for audit
	SessionID  string                  `json:"sessionId"`         // for audit
	KPICallIDs []int64                `json:"kpiCallIds"`        // KPI call audit IDs
}

// reportPublishTool is the HITL Tool for publishing a rendered report.
// Pass 1: validate + interrupt for operator approval.
// Pass 2: after approval, render + publish via report.Publish.
type reportPublishTool struct {
	audit auditWriterHook
}

// Compile-time interface compliance check.
var _ tool.InvokableTool = (*reportPublishTool)(nil)

func (h *reportPublishTool) auditOrDefault() auditWriterHook {
	if h.audit == nil {
		return noopAuditWriter{}
	}
	return h.audit
}

// Info returns the LLM-visible tool schema.
func (h *reportPublishTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ReportPublishToolName,
		Desc: "Submit a rendered report for operator approval and publication. " +
			"The report is rendered from the template + slot texts + KPI snapshot. " +
			"Requires human approval before publication.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"period": {
				Type:     schema.String,
				Required: true,
				Desc:     "Report period in YYYY-MM format (e.g. 2026-08).",
			},
			"template": {
				Type:     schema.Object,
				Required: true,
				Desc:     "Report template structure (sections, KPIs, slot types).",
			},
			"slots": {
				Type:     schema.Object,
				Required: true,
				Desc:     "Slot texts: map of sectionIdx -> slotIdx -> text.",
			},
			"skillId": {
				Type:     schema.String,
				Required: true,
				Desc:     "Skill ID that generated this report.",
			},
			"sessionId": {
				Type:     schema.String,
				Required: true,
				Desc:     "Session ID for audit tracing.",
			},
			"kpiCallIds": {
				Type:     schema.Array,
				Required: false,
				Desc:     "KPI call audit IDs used to produce the report.",
				ElemInfo: &schema.ParameterInfo{Type: schema.Integer},
			},
		}),
	}, nil
}

// InvokableRun is the two-pass HITL Tool body.
// Pass 1: validate + InsertPendingApproval + StatefulInterrupt.
// Pass 2: after approval, render and publish the report.
func (h *reportPublishTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// Pass 1 — first execution. Parse + validate before audit.
		var parsed ReportPublishArgs
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return "", fmt.Errorf("report_publish: invalid args: %w", err)
		}
		if parsed.Period == "" {
			return "", errors.New("report_publish: period is required")
		}
		if parsed.Template == nil {
			return "", errors.New("report_publish: template with sections is required")
		}

		approvalID := compose.GetToolCallID(ctx)
		if approvalID == "" {
			approvalID = fmt.Sprintf("report-%s", parsed.Period)
		}
		info := &ApprovalInfo{
			ToolName:        ReportPublishToolName,
			ArgumentsInJSON: args,
			ToolCallID:      approvalID,
			Target:          ReportPublishTarget,
			Reason: fmt.Sprintf(
				"Report publication request for period %s (skill=%s)",
				parsed.Period, parsed.SkillID,
			),
		}
		if err := h.auditOrDefault().InsertPendingApproval(ctx, approvalID, info); err != nil {
			return "", fmt.Errorf("report_publish: audit INSERT failed (fail-closed): %w", err)
		}
		return "", tool.StatefulInterrupt(ctx, info, args)
	}

	// Pass 2 — resumed execution.
	isTarget, hasData, data := tool.GetResumeContext[*ApprovalDecision](ctx)
	if !isTarget {
		return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
			ToolName:        ReportPublishToolName,
			ArgumentsInJSON: storedArgs,
			ToolCallID:      "",
			Target:          ReportPublishTarget,
			Reason:          "resume-not-target",
		}, storedArgs)
	}
	if !hasData {
		return "", errors.New("report_publish: resumed without data (no ApprovalDecision injected)")
	}
	if err := data.Validate(); err != nil {
		return "", fmt.Errorf("report_publish: invalid ApprovalDecision: %w", err)
	}

	// Parse stored args to extract publish parameters.
	var parsed ReportPublishArgs
	if err := json.Unmarshal([]byte(storedArgs), &parsed); err != nil {
		return "", fmt.Errorf("report_publish: parse stored args: %w", err)
	}

	if data.Decision == "deny" {
		return fmt.Sprintf(
			"user denied report publication for %s: %s",
			parsed.Period, data.Reason,
		), nil
	}

	if data.Decision == "request_change" {
		// D18-D4: 要求修改第三态 — return to LLM with change request
		return fmt.Sprintf("change requested: %s", data.Reason), nil
	}

	// Approve path: render and publish the report.
	result, err := h.publishReport(ctx, &parsed)
	if err != nil {
		return "", fmt.Errorf("report_publish: publish failed: %w", err)
	}
	return fmt.Sprintf(
		"Report published: period=%s, version=v%d, url=%s",
		parsed.Period, result.Version, result.URL,
	), nil
}

// publishReport converts the generic template payload into the typed
// struct and delegates to report.Publish.
//
// CR-01 (18-REVIEW): Template is `any` and ALWAYS crosses a JSON
// boundary — Pass 1 unmarshals the LLM's tool-call JSON, and Pass 2
// unmarshals the frozen args stored by StatefulInterrupt — so
// json.Unmarshal materializes it as map[string]any. A direct
// *report.ReportTemplate type assertion can therefore NEVER succeed
// (the approve path was unreachable). The fix re-encodes the generic
// map and decodes it into the typed struct, then validates it
// fail-closed before any publish side effect (WR-06's publish-side
// half: an LLM-supplied template with arbitrary slot types must be
// rejected here, not rendered).
func (h *reportPublishTool) publishReport(ctx context.Context, args *ReportPublishArgs) (*report.PublishResult, error) {
	raw, err := json.Marshal(args.Template)
	if err != nil {
		return nil, fmt.Errorf("report_publish: re-encode template: %w", err)
	}
	var tmpl report.ReportTemplate
	if err := json.Unmarshal(raw, &tmpl); err != nil {
		return nil, fmt.Errorf("report_publish: decode template: %w", err)
	}
	if err := tmpl.Validate(); err != nil {
		return nil, fmt.Errorf("report_publish: template invalid: %w", err)
	}
	return report.Publish(ctx, &tmpl, args.Slots, args.Period, nil, "", args.KPICallIDs, args.SkillID)
}
