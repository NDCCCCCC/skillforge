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

	"github.com/skillforge/skillforge/internal/kb"
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

// auditWriterHook is the seam the AuditWriter (11-03) satisfies. The
// interface declaration lives here (not in audit_writer) because
// MemoryFactsTool is the CONSUMER.
//
// Contract:
//
//   - InsertPendingApproval persists the ApprovalInfo row to the audit
//     table BEFORE the tool returns from Pass 1 (D11-C3 fail-closed).
//   - MarkApproved / MarkRejected write the SECOND lifecycle row on
//     Pass 2 (their signatures mirror AuditWriter's — the snap
//     SkillSnapshot travels with the call so the Tool-side seam and
//     the server-side approval endpoint share ONE method shape).
//   - MarkAutoPassedRule writes the single-row auto-pass audit record
//     when a D11-D1 rule match skips the interrupt (11-05 / HITL-08).
type auditWriterHook interface {
	InsertPendingApproval(ctx context.Context, approvalID string, info *ApprovalInfo) error
	MarkApproved(ctx context.Context, sessionID, userID string, snap SkillSnapshot, toolCallID, approvalID, target string, frozenArgs []byte, reason string) (int64, error)
	MarkRejected(ctx context.Context, sessionID, userID string, snap SkillSnapshot, toolCallID, approvalID, target string, frozenArgs []byte, reason string) (int64, error)
	MarkAutoPassedRule(ctx context.Context, sessionID, userID string, toolCallID, target string, frozenArgs []byte) (int64, error)
}

// noopAuditWriter is the default when MemoryFactsTool.audit is nil
// (tests + degraded construction). Every method is a success no-op;
// the production runner_setup wires the real AuditWriter.
type noopAuditWriter struct{}

// InsertPendingApproval satisfies auditWriterHook by returning nil.
func (noopAuditWriter) InsertPendingApproval(_ context.Context, _ string, _ *ApprovalInfo) error {
	return nil
}

// MarkApproved satisfies auditWriterHook by returning (0, nil).
func (noopAuditWriter) MarkApproved(_ context.Context, _, _ string, _ SkillSnapshot, _, _, _ string, _ []byte, _ string) (int64, error) {
	return 0, nil
}

// MarkRejected satisfies auditWriterHook by returning (0, nil).
func (noopAuditWriter) MarkRejected(_ context.Context, _, _ string, _ SkillSnapshot, _, _, _ string, _ []byte, _ string) (int64, error) {
	return 0, nil
}

// MarkAutoPassedRule satisfies auditWriterHook by returning (0, nil).
func (noopAuditWriter) MarkAutoPassedRule(_ context.Context, _, _ string, _, _ string, _ []byte) (int64, error) {
	return 0, nil
}

// MemoryFactsTool is the HITL-06 first write target. The Tool is
// stateless beyond its wiring fields — every other piece of state
// (the interrupt JSON, the ApprovalDecision) lives in the eino
// Runner's checkpoint, not on the Tool instance.
//
// EXPORTED (11-05): agent.BuildToolsForSkill type-asserts
// *hitl.MemoryFactsTool across the package boundary to inject the
// per-session context via WithContext.
type MemoryFactsTool struct {
	// audit may be nil; nil-safe via auditOrDefault(). The production
	// wiring (runner_setup) injects the real AuditWriter.
	audit auditWriterHook

	// ruleMatcher is the D11-D1 approve-and-remember lookup. nil-safe
	// (defaults to NoopRuleMatcher) so a zero-value Tool always falls
	// back to the interrupt path — fail-closed toward human review.
	ruleMatcher RuleMatcher

	// sessionID / skillID are the per-session identity bound when
	// BuildToolsForSkill asks the registry for a request-scoped copy
	// (cloneForSession, CR-04 26-REVIEW — D11-B1 per-turn whitelist
	// recompute carries the session context with it — Skill-A's rules
	// never leak into Skill-B's session). They are NEVER written on a
	// shared instance: every request gets its own copy.
	sessionID string
	skillID   string

	// tierEnum is the LLM-visible write-target vocabulary: the kb
	// registry's ActiveSlugs snapshot taken at construction (Plan 26-06
	// / T-26-06-2). It is deliberately a SNAPSHOT, not a live view — a
	// knowledge base created at runtime does not backfill an already
	// constructed agent's tool schema; the new slug takes effect when
	// the Runner is rebuilt (restart), which is the same
	// snapshot-per-construction semantic the retrieve tool's tier
	// closure and the skill loader's resolved reads follow.
	tierEnum []string

	// defaultTier is the write target used when the LLM omits the tier
	// argument: the first slug-ascending active authority-role KB. The
	// authority role is the citable evidence role, so an unspecified
	// fact lands where it can be cited.
	defaultTier string
}

// NewMemoryFactsTool constructs the tool with its tier vocabulary
// snapshotted from the registry (Plan 26-06). Both failure modes fail
// closed rather than degrading to a compiled-in tier list:
//
//   - reg == nil → "kb registry required" (a wiring bug, never an
//     empty vocabulary — the 26-04 nil-interface posture);
//   - no active authority-role KB → the default write target is not
//     derivable, so construction refuses.
func NewMemoryFactsTool(reg kb.Registry) (*MemoryFactsTool, error) {
	t := &MemoryFactsTool{}
	if err := t.bindKBRegistry(reg); err != nil {
		return nil, err
	}
	return t, nil
}

// bindKBRegistry snapshots the tier vocabulary and the default write
// target from reg. Validation completes BEFORE any field is assigned,
// so a failed bind leaves the tool in its unbound state instead of
// half-configured.
func (t *MemoryFactsTool) bindKBRegistry(reg kb.Registry) error {
	if reg == nil {
		return errors.New("hitl: memory_facts_write: kb registry required (the tier vocabulary is a registry snapshot, not a compiled-in list)")
	}
	slugs := reg.ActiveSlugs()
	authority := reg.SlugsByRole(kb.RoleAuthority)
	if len(authority) == 0 {
		return errors.New("hitl: memory_facts_write: kb registry has no authority-role knowledge base — the default write tier is not derivable (fail-closed)")
	}
	t.tierEnum = slugs
	t.defaultTier = authority[0]
	return nil
}

// cloneForSession returns a per-request COPY of the tool with the
// session identity + rule matcher bound and the tier vocabulary
// snapshotted from reg (CR-04, 26-REVIEW).
//
// WHY A COPY: the instances in hitlHandlers are package-global
// singletons, but BuildToolsForSkill runs per HTTP request (Hertz serves
// each request on its own goroutine). Mutating the shared instance per
// request was both a genuine Go data race (concurrent requests write
// sessionID/skillID/ruleMatcher while others read them in Info() and
// InvokableRun) and an audit-integrity defect — with two in-flight write
// requests the last WithContext won, so a memory_facts_write interrupt
// and its action_pending_approval row could be attributed to the wrong
// session/skill. A clone is cheap (a struct copy plus two registry
// snapshot slices) and makes the bindable state private to one request.
//
// Validation is fail-closed and happens BEFORE any state is used: a nil
// registry (or one without an authority-role KB) refuses the clone, the
// same contract as NewMemoryFactsTool.
func (t *MemoryFactsTool) cloneForSession(reg kb.Registry, sessionID, skillID string, ruleMatcher RuleMatcher) (tool.InvokableTool, error) {
	c := *t // shallow copy: carries the audit hook and any bound vocabulary
	if err := c.bindKBRegistry(reg); err != nil {
		return nil, err
	}
	c.sessionID = sessionID
	c.skillID = skillID
	c.ruleMatcher = ruleMatcher
	return &c, nil
}

// auditOrDefault returns the configured audit writer, or a noop if
// nil. Keeps the call site readable (no `if t.audit != nil { ... }`
// boilerplate at every InvokableRun call).
func (t *MemoryFactsTool) auditOrDefault() auditWriterHook {
	if t.audit == nil {
		return noopAuditWriter{}
	}
	return t.audit
}

// ruleMatcherOrDefault returns the configured rule matcher, or the
// NoopRuleMatcher when nil — a nil matcher MUST behave as "no rule
// matched" so the Tool falls back to the interrupt path (fail-closed
// toward human review, never toward silent execution).
func (t *MemoryFactsTool) ruleMatcherOrDefault() RuleMatcher {
	if t.ruleMatcher == nil {
		return NoopRuleMatcher{}
	}
	return t.ruleMatcher
}

// snapshot returns the D11-C2 Skill snapshot derived from the bound
// skillID. Version + PromptHash are populated by the AuditWriter's
// ambient identity (WithIdentity) on the InsertPendingApproval seam;
// the resolution rows (MarkApproved / MarkRejected) carry what the
// Tool knows — the Skill identity.
func (t *MemoryFactsTool) snapshot() SkillSnapshot {
	return SkillSnapshot{SkillID: t.skillID}
}

// Info returns the LLM-visible tool schema. Params are declared via
// schema.NewParamsOneOfByParams (typed map form) rather than the
// auto-reflected form because the Tool's InvokableRun takes a raw
// `args string` and parses it manually — the schema is what the LLM
// is FORBIDDEN to drift from.
//
// The tier parameter's Enum and default are the registry snapshot bound
// at construction (Plan 26-06). An unbound tool has no vocabulary to
// publish, so Info fails closed instead of advertising an empty or
// stale Enum.
func (t *MemoryFactsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	if t.tierEnum == nil {
		return nil, errors.New("hitl: memory_facts_write: kb registry required — construct via NewMemoryFactsTool (or hitl.BindKBRegistry) before publishing the schema")
	}
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
				Enum:     t.tierEnum,
				Desc:     "Tier (knowledge-base slug) to write into. Defaults to " + t.defaultTier + " (the first active authority-role knowledge base).",
			},
		}),
	}, nil
}

// memFactsArgs is the typed parse target for ArgumentsInJSON. An empty
// Tier resolves through the registry-bound authority-role default at
// write time (executeMemoryFactsWrite).
type memFactsArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Tier  string `json:"tier,omitempty"`
}

// InvokableRun is the two-pass HITL Tool body.
//
// Pass 1 (no prior interrupt on this address): D11-D1 rule-match
// check FIRST — a matched approve-and-remember rule skips the
// interrupt entirely (MarkAutoPassedRule audit row + direct execute).
// No match → InsertPendingApproval (D11-C3 fail-closed) +
// tool.StatefulInterrupt.
//
// Pass 2 (resumed after operator decision): ApplyFrozenArgs picks the
// executed bytes (data.FrozenArgs when the operator edited, else the
// stored args), assertFrozenArgsEqual enforces the HITL-07 3-way
// byte-identity invariant, then MarkApproved + execute (or
// MarkRejected + deny message).
func (t *MemoryFactsTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	wasInterrupted, hasState, storedArgs := tool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		// Pass 1 — first execution.
		approvalID := compose.GetToolCallID(ctx)
		if approvalID == "" {
			approvalID = newApprovalID()
		}

		// D11-D1 auto-pass (HITL-08): if an approve-and-remember rule
		// matches (target, sha256(args), skillID, sessionID), skip the
		// interrupt — audit the auto-pass and execute directly. The
		// argsHash is computed from the VERBATIM args bytes, never
		// re-marshaled (HITL-07 byte-identity anchor shared with the
		// rule key).
		argsHash := ComputeFrozenArgsHash([]byte(args))
		if _, matched := t.ruleMatcherOrDefault().Match(ctx, MemoryFactsTarget, argsHash, t.skillID, t.sessionID); matched {
			userID := userIDFromCtx(ctx)
			if _, err := t.auditOrDefault().MarkAutoPassedRule(ctx, t.sessionID, userID, approvalID, MemoryFactsTarget, []byte(args)); err != nil {
				// Fail-closed, same posture as the pending-row insert:
				// an audit failure aborts the write.
				return "", fmt.Errorf("memory_facts_write: MarkAutoPassedRule failed (fail-closed): %w", err)
			}
			return t.executeMemoryFactsWrite(ctx, []byte(args))
		}

		// No rule matched — the audit row MUST be persisted BEFORE the
		// interrupt (D11-C3 fail-closed: a SQL error surfaces to the
		// operator, not as a silent lost interrupt).
		info := &ApprovalInfo{
			ToolName:        MemoryFactsToolName,
			ArgumentsInJSON: args,
			ToolCallID:      approvalID,
			Target:          MemoryFactsTarget,
			Reason:          "",
			SkillVersion:    "",
			PromptHash:      "",
		}
		if err := t.auditOrDefault().InsertPendingApproval(ctx, approvalID, info); err != nil {
			return "", fmt.Errorf("memory_facts_write: audit INSERT failed (fail-closed): %w", err)
		}

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
	//   - is target + data → Validate the decision, assert the HITL-07
	//     3-way byte equality, then approve or deny.
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

	// HITL-07 edit-before-approve: data.FrozenArgs (possibly edited by
	// the operator) wins over the Pass-1 storedArgs when non-empty.
	writeArgs := ApplyFrozenArgs(data, storedArgs)

	// HITL-07 3-way byte-identity: the ApprovalCard preview bytes, the
	// audit frozen_args bytes, and the about-to-be-executed bytes must
	// agree. On the resume path the framework re-invokes the Tool with
	// the original args, so `args` anchors the preview + audit sides;
	// writeArgs is the executed side. Drift → ErrFrozenArgsDrift
	// (wrapped) — the operator must re-review, the write NEVER
	// executes against drifted bytes.
	previewBytes := []byte(args)
	auditFrozen := []byte(args)
	if err := assertFrozenArgsEqual(previewBytes, auditFrozen, []byte(writeArgs)); err != nil {
		return "", fmt.Errorf("memory_facts_write: %w", err)
	}

	userID := userIDFromCtx(ctx)
	if data.Decision == "deny" {
		if _, err := t.auditOrDefault().MarkRejected(ctx, t.sessionID, userID, t.snapshot(), data.ApprovalID, data.ApprovalID, MemoryFactsTarget, []byte(writeArgs), data.Reason); err != nil {
			return "", fmt.Errorf("memory_facts_write: MarkRejected failed: %w", err)
		}
		return fmt.Sprintf("user denied: %s", data.Reason), nil
	}

	// Approve path.
	if _, err := t.auditOrDefault().MarkApproved(ctx, t.sessionID, userID, t.snapshot(), data.ApprovalID, data.ApprovalID, MemoryFactsTarget, []byte(writeArgs), data.Reason); err != nil {
		return "", fmt.Errorf("memory_facts_write: MarkApproved failed: %w", err)
	}
	return t.executeMemoryFactsWrite(ctx, []byte(writeArgs))
}

// executeMemoryFactsWrite performs the actual write side of the Tool.
// 11-05 keeps the 11-02 no-op success message (the real memory:facts
// INSERT ships in Phase 12 with the memory store's Persist). The
// success string is structured ("memory_facts_write OK: tier=X key=Y
// value=Z") so the chatHandler can render the verdict deterministically.
// The tier default is the registry-bound authority-role slug (Plan
// 26-06) and is enforced here so BOTH the Pass-1 auto-pass path and the
// Pass-2 approve path share one behavior; an unbound tool refuses to
// invent a target rather than writing into an empty slug.
func (t *MemoryFactsTool) executeMemoryFactsWrite(_ context.Context, writeArgs []byte) (string, error) {
	var parsed memFactsArgs
	if err := json.Unmarshal(writeArgs, &parsed); err != nil {
		return "", fmt.Errorf("memory_facts_write: parse args: %w", err)
	}
	if parsed.Tier == "" {
		if t.defaultTier == "" {
			return "", errors.New("memory_facts_write: no default tier bound (kb registry missing at construction — fail-closed)")
		}
		parsed.Tier = t.defaultTier
	}
	return fmt.Sprintf(
		"memory_facts_write OK: tier=%s key=%s value=%s",
		parsed.Tier, parsed.Key, parsed.Value,
	), nil
}

// userIDFromCtx extracts the operator identity from the request ctx.
// 11-05 reads the "user_id" ctx value (set by the chatHandler
// boundary); Phase 12 wires real auth. Absent → "" (the audit row's
// user_id column tolerates empty).
func userIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("user_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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
