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

// hitl_review_report.go is Phase 11 / Plan 11-04's second write Tool
// (HITL-06 completion). It mirrors hitl_memory_facts.go's two-pass
// interrupt/resume contract: Pass 1 persists the pending audit row via
// the shared auditWriterHook seam THEN signals tool.StatefulInterrupt;
// Pass 2 validates the injected *ApprovalDecision and either executes
// the (11-04 stub) write or returns the operator's deny reason.
//
// The audit-writer seam (auditWriterHook), the wire type (ApprovalInfo),
// the noop default (noopAuditWriter), and the approval-ID helper
// (newApprovalID) are all shared with hitl_memory_facts.go — this file
// declares NONE of them again (Go forbids package-level redeclaration).
package hitl

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ReviewReportToolName is the Eino tool name emitted by the LLM in
// tool_calls. Distinct from ReviewReportTarget ("review_report") per
// D11-B2 + HITL-06: the Tool NAME is what the LLM writes to
// tool_calls[i].Function.Name; the TARGET is the policy routing key
// carried in ApprovalInfo.Target and echoed back through
// ApprovalDecision.Target. The two namespaces must not collide.
const ReviewReportToolName = "review_report_write"

// ReviewReportTarget is the canonical target string for the
// review_report write (the policy string in ApprovalInfo.Target and
// the audit row's target column).
const ReviewReportTarget = "review_report"

// init registers *ApprovalInfo with encoding/gob under the literal
// name "hitl.ApprovalInfo" — the SAME dual-registration posture as
// ApprovalDecision (D11-B2, approval_decision.go init).
//
// WHY THIS LIVES IN 11-04's FILE: every HITL Tool passes *ApprovalInfo
// as the tool.StatefulInterrupt info payload. The adk Runner's
// checkpoint save gob-encodes that payload; without registration the
// save fails with `gob: type not registered for interface:
// hitl.ApprovalInfo` and the interrupt CANNOT be checkpointed
// (verified empirically against eino v0.9.14: interrupt event fires,
// then `failed to save checkpoint: failed to encode checkpoint: gob:
// type not registered for interface: hitl.ApprovalInfo`). 11-02
// shipped ApprovalInfo without the registration because no Runner
// round-trip exercised it; 11-04's approval endpoint is the first
// production path that persists + resumes these checkpoints.
//
// gob.RegisterName (not schema.RegisterName) is used on purpose:
// encoding/gob tolerates a duplicate same-name same-type registration
// (idempotent), while schema.RegisterName panics on a duplicate
// GenericRegister. If a sibling plan adds the same registration the
// process must not crash at init. The gob boundary is the one the
// checkpoint encode/decode actually consults.
func init() {
	gob.RegisterName("hitl.ApprovalInfo", &ApprovalInfo{})
}

// ReviewReportArgs is the typed payload schema for the review_report
// write (11-CONTEXT §Claude's Discretion E — planner/executor fix the
// concrete fields; only the schema name + target whitelist are locked
// upstream).
//
//   - Report: the report body (free-form markdown). This is the
//     customer-facing / counterparty-discoverable artifact.
//   - Citations: the chunk IDs / source references the report cites,
//     so the operator can verify every claim against the original
//     sources before approving.
type ReviewReportArgs struct {
	Report    string   `json:"report"`
	Citations []string `json:"citations"`
}

// reviewReportTool is the HITL-06 second write target. Stateless
// beyond the `audit` field — the interrupt JSON and the
// ApprovalDecision live in the eino Runner's checkpoint, not on the
// Tool instance (same posture as MemoryFactsTool).
type reviewReportTool struct {
	// audit may be nil; nil-safe via auditOrDefault(). The production
	// wiring injects the real AuditWriter; tests use the noop default
	// or a spy.
	audit auditWriterHook
}

// auditOrDefault helper moved to audit_writer.go (Phase 34 HITL-01).

// Info returns the LLM-visible tool schema. Both params are REQUIRED:
// a report without a body is meaningless, and a report without
// citations is unverifiable (the operator cannot ground-check the
// claims at approval time).
func (h *reviewReportTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ReviewReportToolName,
		Desc: "Generate a customer-facing or counterparty-discoverable review report " +
			"(regulatory-discoverable long-lived document). The report is timestamped + " +
			"actor-attributed. Requires human approval before write.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"report": {
				Type:     schema.String,
				Required: true,
				Desc:     "Report body (markdown). This text becomes a long-lived, discoverable document — the wording is permanent unless manually corrected.",
			},
			"citations": {
				Type:     schema.Array,
				Required: true,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Desc:     "Chunk IDs / source references the report cites. The operator verifies every claim against these sources before approving.",
			},
		}),
	}, nil
}

// InvokableRun is the two-pass HITL Tool body (mirrors
// MemoryFactsTool.InvokableRun; the audit row's target is
// "review_report" instead of "memory:facts").
//
// Pass 1 (no prior interrupt on this address): InsertPendingApproval
// via the audit-writer seam (D11-C3 fail-closed: an audit INSERT error
// returns BEFORE the interrupt is signalled), then
// tool.StatefulInterrupt carrying the ApprovalInfo + verbatim args.
//
// Pass 2 (resumed after operator decision): three branches — not the
// resume target → re-interrupt (leaf contract preservation); is target
// + no data → framework glitch (reject); is target + data → Validate
// the decision, then deny (message) or approve (execute stub).
//
// The Tool does NOT call any chat model — deterministic logic only.
func (h *reviewReportTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// Pass 1 — first execution. The audit row MUST be persisted
		// BEFORE the interrupt (D11-C3 fail-closed: a SQL error
		// surfaces to the operator, not as a silent lost interrupt).
		approvalID := compose.GetToolCallID(ctx)
		if approvalID == "" {
			approvalID = newApprovalID()
		}
		info := &ApprovalInfo{
			ToolName:        ReviewReportToolName,
			ArgumentsInJSON: args,
			ToolCallID:      approvalID,
			Target:          ReviewReportTarget,
			Reason:          "",
			SkillVersion:    "",
			PromptHash:      "",
		}
		if _, err := writePendingAndInterrupt(ctx, h.audit, approvalID, info, args); err != nil {
			// D11-C3 fail-closed: audit write failure → return error
			// BEFORE interrupt. The operator (not the LLM) decides
			// how to retry.
			return "", fmt.Errorf("review_report_write: audit INSERT failed (fail-closed): %w", err)
		}
		return "", nil
	}

	// Pass 2 — resumed execution.
	isTarget, hasData, data := tool.GetResumeContext[*ApprovalDecision](ctx)
	if !isTarget {
		// Leaf contract preservation (adk/runner.go:137-147): not our
		// turn → re-interrupt with the stored args so the checkpoint
		// state survives for the eventual targeted resume.
		return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
			ToolName:        ReviewReportToolName,
			ArgumentsInJSON: storedArgs,
			ToolCallID:      "",
			Target:          ReviewReportTarget,
			Reason:          "resume-not-target",
		}, storedArgs)
	}
	if !hasData {
		return "", errors.New("review_report_write: resumed without data (no ApprovalDecision injected)")
	}
	if err := data.Validate(); err != nil {
		return "", fmt.Errorf("review_report_write: invalid ApprovalDecision: %w", err)
	}
	if data.Decision == "deny" {
		// The approval endpoint (11-04) writes the MarkRejected audit
		// row; the Tool returns the deny reason so the chat-visible
		// response carries the operator's rationale.
		return fmt.Sprintf("user denied: %s", data.Reason), nil
	}

	// Approve path. HITL-07 edit-before-approve uses data.FrozenArgs
	// when the operator rewrote the args; else fall back to the Pass-1
	// storedArgs (byte-equality invariant for unchanged approvals).
	writeArgs := data.FrozenArgs
	if len(writeArgs) == 0 {
		writeArgs = json.RawMessage(storedArgs)
	}
	// ROBUST-04 (Phase 37): 3-way frozen_args byte-equality check.
	// previewBytes = data.FrozenArgs (the operator-saw-and-edited bytes
	// in the ApprovalCard). auditFrozen = storedArgs (Pass-1 audit row's
	// frozen_args column, byte-identical to what we wrote via
	// InsertPendingApproval). executedBytes = writeArgs (what the Tool
	// is about to execute). The 3-way assertion catches any drift
	// between operator intent (previewBytes) and runtime audit
	// (auditFrozen) and the live execution (executedBytes).
	if err := assertFrozenArgsEqual([]byte(data.FrozenArgs), []byte(storedArgs), writeArgs); err != nil {
		return "", fmt.Errorf("review_report_write: %w", err)
	}
	var rrArgs ReviewReportArgs
	if err := json.Unmarshal(writeArgs, &rrArgs); err != nil {
		return "", fmt.Errorf("review_report_write: parse args: %w", err)
	}

	// 11-04 ships a structured success string; the real review_report
	// persistence ships in Phase 12+. The string carries the report's
	// char count + citation count so the chatHandler can render the
	// verdict deterministically.
	return fmt.Sprintf(
		"review_report_write OK: report=%d_chars citations=%d",
		len(rrArgs.Report), len(rrArgs.Citations),
	), nil
}

// Compile-time check: reviewReportTool MUST satisfy tool.InvokableTool
// so the eino compose.ToolsNodeConfig accepts it.
var _ tool.InvokableTool = (*reviewReportTool)(nil)
