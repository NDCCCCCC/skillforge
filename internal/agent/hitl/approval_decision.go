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

// Package hitl houses the SkillForge Human-in-the-Loop (HITL) Tool
// substrate for Phase 11 / Plan 11-02. Every file under
// internal/agent/hitl/ is owned by Phase 11 and the D11-B*
// invariants — any 11-03+ plan that needs to extend the registry
// OR add a new write target reads (and writes) its source of truth
// from this directory.
//
// D11-B2 invariant lives in approval_decision.go (this file):
// ApprovalDecision must be addressable BOTH by the eino internal
// serializer (schema.RegisterName) AND by the encoding/gob registry
// (the on-disk checkpoint payload in internal/store/sqlite is gob).
// Mismatch between the two registered names triggers
// `gob: name not registered` on resume — Pitfall 4 of the Phase 11
// AI-SPEC. We pin BOTH calls to the literal string
// "hitl.ApprovalDecision" so the dual registration is structurally
// bound to one source of truth.
package hitl

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cloudwego/eino/schema"
)

// validDecisions enumerates the legal Decision values accepted by the
// ApprovalDecision validator. Phase 11 ships two: "approve" and "deny".
// "edit" was considered (HITL-07 edit-before-approve path) but lives
// as a fe-side affordance that rewrites FrozenArgs before submit,
// never as a Decision enum value.
var validDecisions = map[string]bool{
	"approve": true,
	"deny":    true,
}

// validScopes enumerates the legal scope values for approve-and-remember
// rules (11-05 ships the actual rule table). Empty string is allowed
// because the FE may omit scope for a one-off approval (rule never
// persists in that case).
var validScopes = map[string]bool{
	"":        true,
	"session": true,
	"skill":   true,
}

// ValidationError is the typed error returned by ApprovalDecision.Validate.
// Carrying the failing field name (vs a free-form message) lets the FE
// surface "Reason required when denying" directly on the form input
// without re-parsing an error string.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return "hitl.ApprovalDecision: field " + e.Field + ": " + e.Msg
}

// ApprovalDecision is the typed schema the FE returns on the approval
// endpoint (11-04 /api/approvals/:id). It crosses THREE serialization
// boundaries in Phase 11:
//
//  1. JSON over HTTP (the FE → backend wire shape, governed by the
//     json tags below).
//  2. encoding/gob into the sqlite checkpoint (the Runner pause
//     payload, governed by the schema.RegisterName + gob.RegisterName
//     dual registration in this file's init()).
//  3. The Runner's InternalSerializer registry (the typed-state
//     handoff handled by eino ADK at resume time, covered by the
//     schema.RegisterName call here).
//
// All three boundaries MUST use the literal name "hitl.ApprovalDecision"
// — see D11-B2.
type ApprovalDecision struct {
	// Decision is "approve" or "deny". Required.
	Decision string `json:"decision"`

	// Reason is mandatory when Decision == "deny" (HITL-02 — deny-without-
	// reason is the audit's most-actionable gap; the operator reviewing
	// the audit log needs to know WHY the write was rejected).
	Reason string `json:"reason,omitempty"`

	// Target echoes ApprovalInfo.Target ("memory:facts", "review_report",
	// future "plan_approve"). Lets the audit_writer cross-check the
	// FE-claimed target against the audit row's target column
	// (mismatch = spoof-reject; see TargetMatches).
	Target string `json:"target"`

	// FrozenArgs preserves the (possibly operator-edited) arguments the
	// write Tool should execute with on Pass 2. json.RawMessage — NOT
	// string — preserves bytes verbatim so the byte-equality invariant
	// `sha256(audit_row.frozen_args) == sha256(data.FrozenArgs)` holds
	// through the HTTP + gob + JSON edges (HITL-07).
	FrozenArgs json.RawMessage `json:"frozen_args,omitempty"`

	// Scope declares whether the approval persists as a rule
	// ("session" or "skill") or is a one-off ("" / omitted). 11-05
	// owns the rule table; this field is the wire-side payload today.
	Scope string `json:"scope,omitempty"`

	// ApprovalID is the audit row's PK that this decision attaches to
	// (== tool_call_id per D11-C4 + D11-B2 single-identity contract).
	// Immutable across an operator edit — HITL-07 rewrites FrozenArgs
	// but ApprovalID stays bound to the same PendingApproval row.
	ApprovalID string `json:"approval_id"`
}

// init is the D11-B2 dual registration. BOTH calls must use the
// LITERAL string "hitl.ApprovalDecision"; a mismatch (or omitting the
// gob half) detonates on resume-decode with `gob: name not registered`.
//
// schema.RegisterName internally calls gob.RegisterName with
// generic.NewInstance[*ApprovalDecision]() — so the FIRST call is
// structurally sufficient. The EXPLICIT gob.RegisterName here is the
// documented redundant safety per the Phase 11 AI-SPEC §4b.1
// (Pitfall 4 — 延迟引爆). If schema.RegisterName ever changes its
// internal registration in a future eino release, this explicit
// call keeps the on-disk gob registry bound to the same string.
func init() {
	schema.RegisterName[*ApprovalDecision]("hitl.ApprovalDecision")
	// Defensive redundant call — gob.RegisterName requires a value,
	// pass &ApprovalDecision{} (idempotent with the schema.RegisterName
	// internal call since both register *ApprovalDecision under the
	// same name).
	gob.RegisterName("hitl.ApprovalDecision", &ApprovalDecision{})
}

// Validate enforces the four field-level invariants on the
// ApprovalDecision struct. Returns *ValidationError on failure (with
// the failing field name so the FE can highlight the right form
// input) and nil on pass.
//
// Order of checks: decision (cheapest) → reason (depends on decision)
// → scope → approval_id. The order matters because Validate is
// called at the HTTP boundary (11-04 approval_api.go) BEFORE the
// RunnerResume; a struct that fails the FIRST check should never
// see a WorkerPoolService.Enqueue.
func (d *ApprovalDecision) Validate() error {
	if !validDecisions[d.Decision] {
		return &ValidationError{
			Field: "decision",
			Msg: fmt.Sprintf(
				"must be one of %v, got %q",
				keysSorted(validDecisions), d.Decision,
			),
		}
	}
	if d.Decision == "deny" && d.Reason == "" {
		return &ValidationError{
			Field: "reason",
			Msg:   "is required when decision is deny (HITL-02 — audit must capture the denial rationale)",
		}
	}
	if !validScopes[d.Scope] {
		return &ValidationError{
			Field: "scope",
			Msg: fmt.Sprintf(
				"must be one of %v (or empty for one-off approval), got %q",
				keysSorted(validScopes), d.Scope,
			),
		}
	}
	if d.ApprovalID == "" {
		return &ValidationError{
			Field: "approval_id",
			Msg:   "is required (== audit row PK; rename the FE form's hidden input if blank)",
		}
	}
	return nil
}

// TargetMatches lets the audit_writer (11-03) cross-check the
// FE-claimed target against the audit row's stored target.
// Returns true when d.Target is empty (FE omitted it — backfill
// allowed) OR when d.Target == expected. False otherwise.
//
// The "empty is fine" carve-out exists because the v1 FE may be
// lazy about echoing the target it received in the ApprovalInfo;
// the audit_writer still has the canonical target as the audit row's
// target column, so an empty claim is recoverable. A NON-EMPTY
// mismatch is a spoof signal that must be rejected.
func (d *ApprovalDecision) TargetMatches(expected string) bool {
	if d.Target == "" {
		return true
	}
	return d.Target == expected
}

// keysSorted returns the keys of a map[string]bool in deterministic
// order. Used by Validate's error messages so test assertions
// (and operator logs) see stable text.
func keysSorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Stable for messages — sort.Slice is fine; the slice size is
	// 2-3 elements and called only on failure paths.
	sort.Strings(out)
	return out
}

// ApplyFrozenArgs returns the args the Tool should execute on Pass 2:
//
//   - data.FrozenArgs when non-empty (HITL-07 edit-before-approve —
//     the operator rewrote the ApprovalCard preview before approving).
//   - otherwise json.RawMessage(storedArgs) — the original LLM-emitted
//     args stored at interrupt time.
//
// The returned bytes are the EXECUTED side of the 3-way byte-equality
// assertion (assertFrozenArgsEqual); they are passed to sha256 via
// ComputeFrozenArgsHash verbatim, never re-marshaled.
func ApplyFrozenArgs(data *ApprovalDecision, storedArgs string) json.RawMessage {
	if data != nil && len(data.FrozenArgs) > 0 {
		return data.FrozenArgs
	}
	return json.RawMessage(storedArgs)
}

// assertFrozenArgsEqual wraps frozen_args.go's assertThreeWayByteEqual
// under the naming the Pass-2 call sites read most naturally. Kept as
// a thin wrapper so the 3-way assertion lives in exactly one place
// (frozen_args.go) while the ApprovalDecision-adjacent call sites read
// against this file's vocabulary.
func assertFrozenArgsEqual(previewBytes, auditFrozen, executedBytes []byte) error {
	return assertThreeWayByteEqual(previewBytes, auditFrozen, executedBytes)
}
