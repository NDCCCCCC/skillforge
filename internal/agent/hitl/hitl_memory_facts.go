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

package hitl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// MemoryFactsToolName is the Eino tool name emitted by the LLM in
// tool_calls. Distinct from MemoryFactsTarget ("memory:facts") on
// purpose (D11-B2): the Tool NAME is what the LLM writes to
// tool_calls[i].Function.Name; the TARGET ("memory:facts") is the
// policy string carried in ApprovalInfo.Target and echoed back
// through ApprovalDecision.Target. They live in different namespaces
// (LLM-visible vs operator-visible) and must not collide.
const MemoryFactsToolName = "memory_facts_write"

// MemoryFactsTarget is the canonical target string for the
// memory:facts write. Used by ApprovalInfo.Target on the wire and
// by AllowList lookups in the audit writer (11-03). Kept here
// (alongside MemoryFactsToolName) so both names live next to the
// Tool that owns them.
const MemoryFactsTarget = "memory:facts"

// ApprovalInfo is the user-facing summary the ApprovalCard renders
// (11-04 / FE Plan). Fields carry the operator's full decision
// context — what the Tool was called for, what the arguments were,
// WHY the operator should care. SkillVersion + PromptHash are
// reserved for 11-03 audit_writer to populate; 11-02 leaves them
// empty so the wire shape is stable across 11-02 → 11-03.
type ApprovalInfo struct {
	// ToolName — the LLM-facing tool name (MemoryFactsToolName =
	// "memory_facts_write"). Echoed by the FE so the ApprovalCard
	// title can render "Review memory_facts_write call".
	ToolName string `json:"tool_name"`

	// ArgumentsInJSON — verbatim JSON string from the LLM tool call
	// (matches tool.InvokableRun signature's second arg). The FE
	// renders this as a key/value table for operator review.
	ArgumentsInJSON string `json:"arguments_in_json"`

	// ToolCallID — the eino ToolCall ID (== ApprovalID == audit row
	// PK per D11-B2 + D11-C4 single-identity contract). v0.9.14's
	// compose.GetToolCallID may return "" inside InvokableRun (the
	// Tool's own tool_call_id is set by the OUTER call site, not the
	// inner Tool ctx); the newApprovalID hex fallback covers the gap.
	ToolCallID string `json:"tool_call_id"`

	// Target — the policy string ("memory:facts" for this Tool). Echoes
	// back through ApprovalDecision.Target so the audit row's target
	// column matches the FE's claim.
	Target string `json:"target"`

	// Reason — the agent's rationale extracted from query context if
	// present (e.g., LLM-emitted reasoning pre-tool-call), else empty.
	// Surface to operator via ApprovalCard; not validated here (the
	// audit_writer 11-03 stamps it regardless of content).
	Reason string `json:"reason"`

	// SkillVersion — reserved for 11-03 audit_writer to populate from
	// manifest.Version. Empty in 11-02.
	SkillVersion string `json:"skill_version"`

	// PromptHash — reserved for 11-03 audit_writer to populate from
	// sha256(RenderSystemPrompt). Empty in 11-02.
	PromptHash string `json:"prompt_hash"`
}

// auditWriterHook is the forward-compat seam that 11-03's audit_writer.go
// satisfies. The interface declaration lives here (not in audit_writer)
// because memoryFactsTool is the CONSUMER; 11-03's plan produces an
// implementation that fits this shape.
//
// Contract: InsertPendingApproval persists the ApprovalInfo row to
// the audit table BEFORE the tool returns from Pass 1 (D11-C3
// fail-closed: if the insert fails, the tool returns an error
// rather than triggering an interrupt — the user can re-invoke the
// tool after the audit store recovers).
//
// The interface is intentionally unexported (auditWriterHook) because
// no other package consumes it yet; 11-03 will export it (as
// AuditWriter) when it lands.
type auditWriterHook interface {
	InsertPendingApproval(ctx context.Context, approvalID string, info *ApprovalInfo) error
}

// noopAuditWriter is the 11-02 default when memoryFactsTool.audit is
// nil. 11-03 wires the real &audit_writer.AuditWriter{} via
// internal/agent/runner_setup.go::BuildRunner (replacing the noop
// with the production writer). Until then, the audit chain is
// structurally incomplete — accepted deviation per the Phase 11
// CONTEXT (T-11-02-B5).
type noopAuditWriter struct{}

// InsertPendingApproval satisfies auditWriterHook by returning nil.
// Used when memoryFactsTool.audit == nil (the 11-02 default).
func (noopAuditWriter) InsertPendingApproval(_ context.Context, _ string, _ *ApprovalInfo) error {
	return nil
}

// memoryFactsTool is the HITL-06 first write target. The Tool is
// stateless beyond the `audit` field — every other piece of state
// (the interrupt JSON, the ApprovalDecision) lives in the eino
// Runner's checkpoint, not on the Tool instance.
type memoryFactsTool struct {
	// audit may be nil; nil-safe via auditOrDefault(). 11-03 wires
	// the real AuditWriter here.
	audit auditWriterHook
}

// auditOrDefault returns the configured audit writer, or a noop if
// nil. Keeps the call site readable (no `if h.audit != nil { ... }`
// boilerplate at every InvokableRun call).
func (h *memoryFactsTool) auditOrDefault() auditWriterHook {
	if h.audit == nil {
		return noopAuditWriter{}
	}
	return h.audit
}

// Info returns the LLM-visible tool schema. Params are declared via
// schema.NewParamsOneOfByParams (typed map form) rather than the
// auto-reflected form because the Tool's InvokableRun takes a raw
// `args string` and parses it manually — the schema is what the LLM
// is FORBIDDEN to drift from.
//
// tier defaults to "权威" when omitted (matches the ComplianceSkill
// canonical behavior; the JSON-schema `default` is informational —
// the InvokableRun enforces the default at parse time).
func (h *memoryFactsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: MemoryFactsToolName,
		Desc: "Persist an authoritative fact into the memory:facts corpus (long-lived retrieval). " +
			"The fact will surface in every future query against this Skill. " +
			"Requires human approval before write.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"key": {
				Type:     schema.String,
				Required: true,
				Desc:     "Fact identifier (e.g. employee_count, fiscal_year). Must be stable — renaming a fact loses the retrieval link.",
			},
			"value": {
				Type:     schema.String,
				Required: true,
				Desc:     "Fact body to persist. Operator-readable text — the fact is what gets surfaced on every future query, so the wording is permanent unless manually corrected.",
			},
			"tier": {
				Type:     schema.String,
				Required: false,
				Enum:     []string{"权威", "历史", "个人"},
				Desc:     "Tier to write into. Defaults to \"权威\" (canonical ComplianceSkill tier). Use \"历史\" for archived facts and \"个人\" for per-session notes.",
			},
		}),
	}, nil
}

// memFactsArgs is the typed parse target for ArgumentsInJSON. Default
// tier = "权威" matches ComplianceSkill's canonical behavior.
type memFactsArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Tier  string `json:"tier,omitempty"`
}

// InvokableRun is the two-pass HITL Tool body. Pass 1 (no prior
// interrupt on this address) inserts the audit row via the seam,
// then interrupts with the stored args. Pass 2 (resumed after
// operator decision) validates the ApprovalDecision and either
// executes the write or returns a deny message.
//
// Quoting the canonical pattern from
// internal/store/sqlite/checkpoint_resume_test.go:140-174 (the
// writeApprovalTool example).
func (h *memoryFactsTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	wasInterrupted, hasState, storedArgs := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// Pass 1 — first execution. The audit row MUST be persisted
		// BEFORE the interrupt (D11-C3 fail-closed: a SQL error
		// surfaces to the operator, not as a silent lost interrupt).
		approvalID := compose.GetToolCallID(ctx)
		if approvalID == "" {
			approvalID = newApprovalID()
		}
		info := &ApprovalInfo{
			ToolName:        MemoryFactsToolName,
			ArgumentsInJSON: args,
			ToolCallID:      approvalID,
			Target:          MemoryFactsTarget,
			Reason:          "",
			SkillVersion:    "",
			PromptHash:      "",
		}
		if err := h.auditOrDefault().InsertPendingApproval(ctx, approvalID, info); err != nil {
			// D11-C3 fail-closed: audit write failure → return error
			// BEFORE interrupt. The operator (not the LLM) decides
			// how to retry.
			return "", fmt.Errorf("memory_facts_write: audit INSERT failed (fail-closed): %w", err)
		}

		// D11-D1 + D11-A2: approve-and-remember rule match is deferred
		// to 11-05 (approval_rules table). 11-02 always interrupts;
		// 11-05 inserts the rule check BEFORE this StatefulInterrupt
		// so an auto-approving rule never reaches the interrupt path.
		//
		// hasState at this point is always false (first pass), but
		// the underscore explicitly documents that the stored-args
		// path is the Pass 2 contract — see below.
		_ = hasState
		return "", tool.StatefulInterrupt(ctx, info, args)
	}

	// Pass 2 — resumed execution. Three branches:
	//   - not the resume target → leaf contract preservation
	//     (adk/runner.go:137-147).
	//   - is target + no data → framework glitch (reject).
	//   - is target + data → Validate the decision, then approve
	//     or deny.
	isTarget, hasData, data := tool.GetResumeContext[*ApprovalDecision](ctx)
	if !isTarget {
		return "", tool.StatefulInterrupt(ctx, &ApprovalInfo{
			ToolName:        MemoryFactsToolName,
			ArgumentsInJSON: storedArgs,
			ToolCallID:      "",
			Target:          MemoryFactsTarget,
			Reason:          "resume-not-target",
		}, storedArgs)
	}
	if !hasData {
		return "", errors.New("memory_facts_write: resumed without data (no ApprovalDecision injected)")
	}
	if err := data.Validate(); err != nil {
		return "", fmt.Errorf("memory_facts_write: invalid ApprovalDecision: %w", err)
	}
	if data.Decision == "deny" {
		// D11-C3: the audit_writer (11-03) marks the row as Rejected
		// here. The Tool returns the deny reason so the chatHandler
		// can include it in the chat-visible response.
		return fmt.Sprintf("user denied: %s", data.Reason), nil
	}

	// Approve path. HITL-07 edit-before-approve uses data.FrozenArgs
	// if non-empty (operator rewrote the args); else fall back to
	// Pass-1 storedArgs (the byte-equality invariant for unchanged
	// approvals).
	writeArgs := data.FrozenArgs
	if len(writeArgs) == 0 {
		writeArgs = json.RawMessage(storedArgs)
	}
	var parsed memFactsArgs
	if err := json.Unmarshal(writeArgs, &parsed); err != nil {
		return "", fmt.Errorf("memory_facts_write: parse args: %w", err)
	}
	if parsed.Tier == "" {
		parsed.Tier = "权威"
	}

	// 11-02 ships a no-op success message. The real memory:facts
	// INSERT ships in 12+ (the audit_writer's MarkApproved followed
	// by the memory store's Persist). The success string is
	// structured ("memory_facts_write OK: tier=X key=Y value=Z") so
	// the chatHandler can render the verdict deterministically.
	return fmt.Sprintf(
		"memory_facts_write OK: tier=%s key=%s value=%s",
		parsed.Tier, parsed.Key, parsed.Value,
	), nil
}

// newApprovalID returns a 16-byte crypto/rand hex string (32 chars).
// v0.9.14's compose.GetToolCallID may return "" inside the Tool's
// own InvokableRun (the outer's tool_call_id doesn't propagate);
// this fallback covers the gap. Stdlib only — no new dependencies.
func newApprovalID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on Linux/macOS/Windows pulls from
		// /dev/urandom or CryptGenRandom; failure is exceptional
		// (entropy pool exhaustion). Fall back to a fixed-length
		// non-secret ID so the tool can still interrupt — the
		// audit_writer will surface a row with a degenerate ID
		// and a separate alarm.
		return "fallback-approval-id-no-entropy"
	}
	return hex.EncodeToString(b)
}
