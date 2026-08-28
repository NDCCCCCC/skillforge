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
	"github.com/skillforge/skillforge/internal/wire"
)

// ReportPublishToolName is the Eino tool name emitted by the LLM.
// Distinct from the write target ("report:publish") on purpose (D11-B2).
const ReportPublishToolName = "report_publish"

// ReportPublishTarget is the canonical target string for the report:publish
// write (the policy string in ApprovalInfo.Target). Matches the
// AllowedWriteTargets entry.
const ReportPublishTarget = "report:publish"

// ReportPublishEmitFunc is the WR-02 (18-REVIEW) SSE emission hook the
// server wires at Runner construction (mirroring the DataRefEmit
// pattern): after a successful publish the tool invokes it once with a
// data-decision payload whose Decision is wire.DecisionReportPublished
// and whose ReportCard carries the artifact metadata. Without it the
// FE report card is dead UI — nothing on the wire ever carries the
// card. Emission failures are the HOOK's concern (log, never abort the
// publish — the reports DB row + file are the source of truth).
type ReportPublishEmitFunc func(p wire.DecisionPayload)

// ReportPublishArgs is the typed payload for the report_publish Tool.
// The args encode the slot texts and KPI snapshot needed to render the report.
//
// Template and Snapshot are `any` because they ALWAYS cross a JSON
// boundary (LLM tool-call args in Pass 1; frozen args round-trip in
// Pass 2): Template is re-encoded into report.ReportTemplate at
// publish time (CR-01), and Snapshot is adapted via report.MapSnapshot
// (CR-06).
type ReportPublishArgs struct {
	Period     string                 `json:"period"`             // e.g. "2026-08"
	Template   any                    `json:"template"`           // report template structure (JSON object)
	Slots      map[int]map[int]string `json:"slots"`              // sectionIdx -> slotIdx -> text
	SkillID    string                 `json:"skillId"`            // for audit
	SessionID  string                 `json:"sessionId"`          // for audit
	KPICallIDs []int64                `json:"kpiCallIds"`         // KPI call audit IDs
	// Snapshot maps KPI name → value, captured from the run's kpi_call
	// results (CR-06). Pass 2 binds it into the render via
	// report.MapSnapshot; a template that declares KPIs without any
	// snapshot data fails closed (insufficient_kpi_data), it never
	// panics on a nil Snapshot.
	Snapshot map[string]any `json:"snapshot,omitempty"`
	// TemplateVersion is the server-derived template content hash
	// (WR-03) — NOT an LLM-visible schema param. Pass 1 computes it
	// (report.TemplateHash) and freezes it into the args the operator
	// reviews; the reports row later carries the SAME identifier.
	TemplateVersion string `json:"templateVersion,omitempty"`
}

// reportPublishTool is the HITL Tool for publishing a rendered report.
// Pass 1: validate + interrupt for operator approval.
// Pass 2: after approval, render + publish via report.Publish.
type reportPublishTool struct {
	audit auditWriterHook

	// reports is the CR-04 reports-table audit sink (INSERT-only
	// PutReport writer). nil-safe at construction; publishReport
	// FAILS CLOSED when it is nil — a published file with no
	// reports row breaks the audit linkage the sink exists for.
	reports report.ReportAuditSink

	// emit is the WR-02 SSE hook; nil disables the data-decision
	// report_published emission (hermetic tests).
	emit ReportPublishEmitFunc
}

// Compile-time interface compliance check.
var _ tool.InvokableTool = (*reportPublishTool)(nil)

// setAudit wires the production audit hook (WR-01); called via the
// auditBinder interface from hitl.InjectDeps.
func (h *reportPublishTool) setAudit(hook auditWriterHook) { h.audit = hook }

// setReportSink wires the production reports audit sink (CR-04). The
// unexported setter is invoked by hitl.InjectDeps at Runner
// construction — see registry.go.
func (h *reportPublishTool) setReportSink(sink report.ReportAuditSink) {
	h.reports = sink
}

// setEmit wires the WR-02 SSE emission hook; invoked via the
// reportEmitBinder interface from hitl.InjectDeps.
func (h *reportPublishTool) setEmit(fn ReportPublishEmitFunc) { h.emit = fn }

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
			"snapshot": {
				Type:     schema.Object,
				Required: false,
				Desc: "KPI values keyed by KPI name, copied VERBATIM from the " +
					"kpi_call results of this run (CR-06). Required when the " +
					"template declares KPIs — publication without data is refused " +
					"(insufficient_kpi_data).",
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
		// CR-05 (18-REVIEW): the period rides LLM-controlled args into
		// filepath.Join("reports", period) — reject every traversal /
		// absolute / drive form BEFORE the interrupt (fail-closed at
		// the HITL boundary; report.Publish re-checks as defense in
		// depth). Same character class as the GET route allowlist.
		if !report.IsValidPeriod(parsed.Period) {
			return "", fmt.Errorf("report_publish: invalid period %q (must match ^[A-Za-z0-9_-]+$)", parsed.Period)
		}
			if parsed.Template == nil {
			return "", errors.New("report_publish: template with sections is required")
		}
		// WR-03: convert + validate the template EARLY (fail-fast before
		// the interrupt, not at approve time) and freeze the stable
		// template content hash into the args. The hash is what the
		// ApprovalCard structure summary renders AND what the reports
		// row will carry (report.TemplateHash) — the operator reviews
		// the same identifier the audit trail records. The rewritten
		// args bytes flow to BOTH the audit row and StatefulInterrupt,
		// so the D11 byte-equality invariant stays intact.
		tmplConv, convErr := report.ParseTemplate(parsed.Template)
		if convErr != nil {
			return "", fmt.Errorf("report_publish: %w", convErr)
		}
		parsed.TemplateVersion = report.TemplateHash(tmplConv)
		frozenBytes, mErr := json.Marshal(parsed)
		if mErr != nil {
			return "", fmt.Errorf("report_publish: re-marshal frozen args: %w", mErr)
		}
		args = string(frozenBytes)

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

	// Approve path: render and publish the report. data.ApprovalID is
	// the D11-C4 identity of the pending-approval audit row this Pass 2
	// resolves — it rides the reports row so the published artifact is
	// linked to its approval chain (CR-04).
	result, err := h.publishReport(ctx, &parsed, data.ApprovalID)
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
func (h *reportPublishTool) publishReport(ctx context.Context, args *ReportPublishArgs, approvalAuditID string) (*report.PublishResult, error) {
	// CR-01: convert the JSON-round-trip template map into the typed
	// struct; WR-06: Validate runs inside ParseTemplate — fail-closed
	// before any publish side effect.
	tmpl, err := report.ParseTemplate(args.Template)
	if err != nil {
		return nil, fmt.Errorf("report_publish: %w", err)
	}
	// CR-06 (18-REVIEW): bind the KPI snapshot the render reads. The
	// frozen args carry the kpi_call-produced values keyed by KPI name
	// (captured in Pass 1). A template that declares KPIs with NO
	// snapshot data is a hard error — the previous build passed a
	// literal nil Snapshot into report.Publish, so buildMarkdown's
	// snap.Get(...) deterministically panicked mid-resume. Surface
	// "no KPI data" as an error (the Skill's required_refusal_phrases
	// vocabulary: insufficient_kpi_data), never a panic.
	var snap report.Snapshot
	if len(args.Snapshot) > 0 {
		snap = report.MapSnapshot(args.Snapshot)
	} else {
		for _, sec := range tmpl.Sections {
			if len(sec.KPIs) > 0 {
				return nil, errors.New(
					"report_publish: no KPI snapshot in frozen args but the template declares KPIs (insufficient_kpi_data — CR-06 fail-closed)")
			}
		}
	}
	// CR-04 fail-closed: no reports sink → no publish. A file without
	// its append-only audit row would silently break the
	// artifact↔approval↔KPI-call linkage.
	if h.reports == nil {
		return nil, errors.New(
			"report_publish: reports audit sink not wired (CR-04 fail-closed) — " +
				"inject the store-backed ReportAuditSink via hitl.InjectDeps")
	}
	result, err := report.Publish(ctx, tmpl, args.Slots, args.Period, snap,
		approvalAuditID, args.KPICallIDs, args.SkillID, h.reports)
	if err != nil {
		return nil, err
	}

	// WR-02 (18-REVIEW): emit the report_published card on the existing
	// data-decision channel so the FE renders it (previously the
	// ReportCard struct + Decision constant had ZERO emitters and the
	// FE card was dead UI). Emission is best-effort — the hook logs
	// failures; the file + reports row are the source of truth.
	if h.emit != nil {
		h.emit(wire.DecisionPayload{
			Step:       0,
			Decision:   wire.DecisionReportPublished,
			ToolCallID: approvalAuditID,
			ReportCard: &wire.ReportCard{
				Title:   tmpl.Title,
				Period:  args.Period,
				Version: result.Version,
				URL:     result.URL,
			},
		})
	}
	return result, nil
}
