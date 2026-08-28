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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// audit_writer.go is the SOLE fail-closed write path for the HITL-04
// audit chain (D11-C3).
//
// It is the EXPLICIT INVERSION of the store read-path posture: the read
// path (store.QueryAudit and its server-side consumers) swallows and
// redacts, because a failed audit READ must never break a user-facing
// response. The write path does the opposite — a failed audit INSERT
// returns immediately, with no retry layer and no swallow, so the HITL
// Tool aborts BEFORE tool.StatefulInterrupt is reached:
//
//   - the interrupt signal is never raised,
//   - the Runner-internal CheckPointStore.Set never runs,
//   - the write tool never executes.
//
// The two paths deliberately share no helper. A copy-paste of the
// swallow posture into this file surfaces as a TestFailClosedAudit
// failure (the P0 no-waiver gate).
//
// ApprovalID == ToolCallID is a SINGLE identity (D11-C4). The partial
// unique index idx_audit_pending_tcid on
// audit(tool_call_id) WHERE action='action_pending_approval' turns a
// cross-device double-approve race into a UNIQUE constraint error:
// exactly one INSERT wins.
//
// Dependency posture (D18 interface-first): this file consumes the
// store.Store INTERFACE only. It never imports internal/store/sqlite,
// so a Postgres backend drops in without touching the writer.
//
// PLAN-12-03 / Phase 12 plan_approve audit rows:
//
//	plan_approve audit rows use the SAME action='action_pending_approval'
//	as every other HITL target (memory:facts, review_report). The
//	discriminator is the audit row's `target` column (= "plan_approve")
//	— the AuditTimelineTab (12-07) filters by target for plan rows.
//
//	WHY this design (vs. a per-target enum value):
//	  1. No migration. Adding an enum value would require an ALTER on
//	     audit.action or a discriminator convention; reusing the
//	     existing action keeps the table schema frozen.
//	  2. Uniform 3-row lifecycle. The pending → approved/rejected chain
//	     is identical across all HITL Tools (HITL-04 contract); the
//	     plan_approve Tool's audit chain looks the same as memory:facts
//	     from the table's perspective.
//	  3. The target column is already indexed (idx_audit_target) — the
//	     timeline filter is O(matched) not O(table).
//
//	The AuditWriterHook interface is target-agnostic on purpose: the
//	plan_approve Tool passes ApprovalInfo{Target: "plan_approve"} and
//	the same INSERT path stamps the row with action='action_pending_approval'
//	+ the target column carrying the discriminator. Zero interface
//	extension. Zero migration.

// AuditWriter is the single entry point for HITL-04 audit INSERTs. Each
// of its four methods writes exactly ONE row of the lifecycle:
//
//	WritePendingApproval → MarkApproved | MarkRejected
//	MarkRuleRevoked (D11-D2 revoke; its own single-row chain)
//
// There is no method that updates or deletes an audit row (T-3-AUDIT).
type AuditWriter struct {
	store store.Store

	// floorVersion is the PLAT-10 platform-floor semver that every HITL
	// audit row MUST carry. It is injected at construction so this package
	// never imports internal/agent (the agent→hitl edge already exists;
	// reversing it would create an import cycle).
	floorVersion string

	// now is the clock. Production uses time.Now().UTC(); hermetic tests
	// inject a fixed clock via SetNow so ts values are deterministic.
	now func() time.Time

	// sessionID / userID / snapshot are the ambient identity used by the
	// InsertPendingApproval seam (the narrow interface MemoryFactsTool
	// consumes, which carries no Skill context of its own). They are
	// optional: the explicit WritePendingApproval / MarkApproved /
	// MarkRejected / MarkRuleRevoked methods take everything they need as
	// arguments.
	sessionID string
	userID    string
	snapshot  SkillSnapshot
}

// SkillSnapshot is the D11-C2 dual-table snapshot of the active Skill:
// identity + version + prompt hash, frozen into every audit row (and
// into the checkpoint row via SetCheckpointWithSnapshot). On resume, a
// mismatch between the stored snapshot and the live Skill expires the
// checkpoint instead of silently resuming against drifted instructions.
//
// DEVIATION (documented, load-bearing): plan 11-03 specified passing a
// *skill.Manifest into every writer method. That is impossible in this
// package: internal/skill imports internal/agent/hitl (validate.go, the
// D11-B3 shared write-target source), so hitl importing skill is an
// import cycle. SkillSnapshot is the dependency-free value object
// carrying exactly the fields the audit rows need; the caller layer
// (internal/agent / internal/server, which may import skill) builds it
// via NewSkillSnapshot.
type SkillSnapshot struct {
	// SkillID is the Skill manifest identity (manifest.Name, e.g.
	// "compliance"). Persisted to audit.skill_id.
	SkillID string

	// Version is the Skill version persisted to audit.skill_version and
	// checkpoint.skill_version.
	//
	// DEVIATION (documented): the plan said manifest.Version, but
	// skill.Manifest has no Version field — the only version in the Skill
	// subsystem is skill.Entry.Version, a per-PROCESS monotonic counter
	// that resets on restart (so a checkpoint written before a restart
	// could falsely match, or falsely expire, after one). Callers should
	// pass a value that changes exactly when the Skill content changes;
	// NewSkillSnapshot derives such a value from the prompt body when the
	// caller has no better source.
	Version string

	// PromptHash is the hex SHA-256 of the rendered system prompt body
	// (see ComputePromptHash). Persisted to audit.prompt_hash and
	// checkpoint.prompt_hash.
	PromptHash string
}

// NewSkillSnapshot builds a snapshot from the Skill identity, an
// explicit version, and the rendered system-prompt body. When version is
// empty it is derived from the prompt body ("sha256:<16 hex>") so the
// snapshot still changes exactly when the rendered prompt changes — a
// content version, stable across process restarts.
func NewSkillSnapshot(skillID, version, promptBody string) SkillSnapshot {
	hash := ComputePromptHash(promptBody)
	if version == "" && hash != "" {
		version = "sha256:" + hash[:16]
	}
	return SkillSnapshot{SkillID: skillID, Version: version, PromptHash: hash}
}

// IsZero reports whether the snapshot carries no Skill context at all.
// Every lifecycle method rejects a zero snapshot: an audit row without
// the D11-C2 snapshot cannot support the resume expire check.
func (s SkillSnapshot) IsZero() bool {
	return s.SkillID == "" && s.Version == "" && s.PromptHash == ""
}

// Compile-time check that *AuditWriter satisfies the seam declared by
// 11-02 in hitl_memory_facts.go. The Tool consumes the narrow
// auditWriterHook interface; this assertion is what makes 11-03's writer
// droppable into MemoryFactsTool.audit without a shim.
var _ auditWriterHook = (*AuditWriter)(nil)

// NewAuditWriter constructs the writer with the default clock.
// floorVersion is the PLAT-10 PlatformFloorVersion semver stamped on
// every HITL audit row; an empty value makes every write fail-closed.
func NewAuditWriter(s store.Store, floorVersion string) *AuditWriter {
	return &AuditWriter{store: s, floorVersion: floorVersion, now: func() time.Time { return time.Now().UTC() }}
}

// ErrMissingFloorVersion is the fail-closed sentinel returned by every
// HITL write method when the writer was constructed with an empty
// PlatformFloorVersion. An empty value would create a new audit row that
// masquerades as "pre-floor era" and falsifies the drift window
// (T-14-06b mitigation).
var ErrMissingFloorVersion = errors.New("hitl: AuditWriter PlatformFloorVersion required (PLAT-10)")

// guardFloorVersion fail-closes the writer when no PlatformFloorVersion
// was injected. Every write method calls this before building the row.
func (w *AuditWriter) guardFloorVersion() error {
	if w.floorVersion == "" {
		return ErrMissingFloorVersion
	}
	return nil
}

// SetNow injects a deterministic clock for hermetic tests. Production
// callers do not invoke this method.
func (w *AuditWriter) SetNow(now func() time.Time) { w.now = now }

// WithIdentity binds the ambient session / user / Skill identity used by
// the InsertPendingApproval seam (the interface MemoryFactsTool calls).
// The Runner wiring (11-04) calls this once per turn, before handing the
// writer to BuildToolsForSkill. Returns the receiver so the call can be
// chained onto NewAuditWriter.
func (w *AuditWriter) WithIdentity(sessionID, userID string, snap SkillSnapshot) *AuditWriter {
	w.sessionID = sessionID
	w.userID = userID
	w.snapshot = snap
	return w
}

// InsertPendingApproval satisfies the auditWriterHook seam that 11-02's
// MemoryFactsTool consumes. It is a thin adapter over
// WritePendingApproval: the Tool knows its ApprovalInfo but not the
// session / Skill identity, so the ambient values bound by WithIdentity
// fill the gap.
//
// D11-C3 fail-closed: the error is returned verbatim to the Tool, which
// returns it from InvokableRun BEFORE reaching tool.StatefulInterrupt.
func (w *AuditWriter) InsertPendingApproval(ctx context.Context, approvalID string, info *ApprovalInfo) error {
	if info == nil {
		return errors.New("hitl: InsertPendingApproval: info required")
	}
	// The seam's approvalID is authoritative — it is the value the Tool
	// stamped into info.ToolCallID (D11-C4 single identity).
	if approvalID == "" {
		approvalID = info.ToolCallID
	}
	// Stamp the D11-C2 snapshot onto the wire object so the FE's
	// ApprovalCard and the audit row agree (11-02 ships these fields
	// empty by contract; the writer is what fills them).
	if info.SkillVersion == "" {
		info.SkillVersion = w.snapshot.Version
	}
	if info.PromptHash == "" {
		info.PromptHash = w.snapshot.PromptHash
	}
	_, err := w.writePending(ctx, pendingArgs{
		sessionID:    w.sessionID,
		userID:       w.userID,
		skillID:      w.snapshot.SkillID,
		skillVersion: info.SkillVersion,
		promptHash:   info.PromptHash,
		toolCallID:   approvalID,
		target:       info.Target,
		frozenArgs:   []byte(info.ArgumentsInJSON),
	})
	return err
}

// WritePendingApproval inserts the FIRST row of the HITL-04 lifecycle
// synchronously. The HITL Tool MUST call it (directly or through the
// InsertPendingApproval seam) BEFORE returning tool.StatefulInterrupt —
// D11-C3 fail-closed.
//
// On ANY INSERT error (sqlite busy / disk full / schema drift / ctx
// cancelled / UNIQUE collision on the pending tool_call_id) the wrapped
// error is returned. The caller propagates it to the Tool's
// InvokableRun return value; the Runner then never signals the
// interrupt, never writes a checkpoint, and never executes the write.
//
// Arguments:
//
//	ctx        — the Tool's InvokableRun ctx (cancellation propagates)
//	sessionID  — the active session (audit partition key + rule scope)
//	userID     — operator identity (from the auth context in 11-04)
//	snap       — the active Skill snapshot (D11-C2 skill_version + prompt_hash)
//	toolCallID — == ApprovalID (D11-C4)
//	target     — the write target (e.g. "memory:facts")
//	frozenArgs — byte snapshot of the LLM-emitted tool-call JSON args
//	reason     — the agent's rationale. It travels to the operator on the
//	             ApprovalInfo wire object (the ApprovalCard renders it);
//	             the pending audit row leaves refusal_reason empty because
//	             that column is reserved for the OPERATOR's reason, which
//	             only exists once the decision is made.
//
// Returns the audit row id (>0) on success.
func (w *AuditWriter) WritePendingApproval(
	ctx context.Context,
	sessionID, userID string,
	snap SkillSnapshot,
	toolCallID, target string,
	frozenArgs []byte,
	reason string,
) (int64, error) {
	if toolCallID == "" {
		return 0, errors.New("hitl: WritePendingApproval: toolCallID required (D11-C4 single identity)")
	}
	if snap.IsZero() {
		return 0, errors.New("hitl: WritePendingApproval: skill snapshot required (D11-C2)")
	}
	_ = reason // carried to the operator on ApprovalInfo, not on the pending row
	return w.writePending(ctx, pendingArgs{
		sessionID:    sessionID,
		userID:       userID,
		skillID:      snap.SkillID,
		skillVersion: snap.Version,
		promptHash:   snap.PromptHash,
		toolCallID:   toolCallID,
		target:       target,
		frozenArgs:   frozenArgs,
	})
}

// pendingArgs is the internal parameter bag shared by the exported
// WritePendingApproval and the InsertPendingApproval seam, so both go
// through EXACTLY the same fail-closed INSERT (one code path, one error
// posture — the property TestFailClosedAudit locks).
type pendingArgs struct {
	sessionID    string
	userID       string
	skillID      string
	skillVersion string
	promptHash   string
	toolCallID   string
	target       string
	frozenArgs   []byte
}

func (w *AuditWriter) writePending(ctx context.Context, a pendingArgs) (int64, error) {
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	row := store.PendingApprovalRow{
		Timestamp:    w.clock(),
		SessionID:    a.sessionID,
		UserID:       a.userID,
		SkillID:      a.skillID,
		Query:        a.target, // the write target; the args live in frozen_args
		PromptHash:   a.promptHash,
		SkillVersion: a.skillVersion,
		// ModelVersion / VerdictJSON / ChunkIDs belong to the verdict
		// row that server.writeAuditAndTurn writes; an approval row has
		// no model output yet.
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        "",
		ChunkIDs:             "",
		FrozenArgs:           a.frozenArgs,
		ToolCallID:           a.toolCallID,
		ApprovalID:           a.toolCallID, // D11-C4: one identity, two columns
		ScopeSessionID:       a.sessionID,
		PlatformFloorVersion: w.floorVersion,
	}
	// D11-C3: returned IMMEDIATELY — no retry, no swallow, no fallback.
	id, err := w.store.InsertPendingApproval(ctx, row)
	if err != nil {
		return 0, fmt.Errorf("hitl: WritePendingApproval: audit INSERT failed (fail-closed): %w", err)
	}
	return id, nil
}

// MarkApproved inserts the SECOND row of the lifecycle
// (action_approved) after the operator approves via the dual endpoint
// (11-04). frozenArgs may be the edited bytes (HITL-07
// edit-before-approve); when no edit happened it is byte-identical to
// the pending row's frozen_args — the invariant
// TestApprovalArgsByteEquality locks.
func (w *AuditWriter) MarkApproved(
	ctx context.Context,
	sessionID, userID string,
	snap SkillSnapshot,
	toolCallID, approvalID, target string,
	frozenArgs []byte,
	reason string,
) (int64, error) {
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	if snap.IsZero() {
		return 0, errors.New("hitl: MarkApproved: skill snapshot required (D11-C2)")
	}
	return w.store.MarkApproved(ctx, store.ApprovalResolutionRow{
		Timestamp:            w.clock(),
		SessionID:            sessionID,
		UserID:               userID,
		SkillID:              snap.SkillID,
		Query:                target,
		PromptHash:           snap.PromptHash,
		SkillVersion:         snap.Version,
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        reason,
		ChunkIDs:             "",
		FrozenArgs:           frozenArgs,
		ToolCallID:           toolCallID,
		ApprovalID:           approvalID,
		ScopeSessionID:       sessionID,
		PlatformFloorVersion: w.floorVersion,
	})
}

// MarkRejected mirrors MarkApproved with action='action_rejected'.
// HITL-02 requires a reason on the deny path; the HTTP layer (11-04)
// validates it via ApprovalDecision.Validate() and this method
// re-checks it so no code path can write a reasonless rejection.
func (w *AuditWriter) MarkRejected(
	ctx context.Context,
	sessionID, userID string,
	snap SkillSnapshot,
	toolCallID, approvalID, target string,
	frozenArgs []byte,
	reason string,
) (int64, error) {
	if snap.IsZero() {
		return 0, errors.New("hitl: MarkRejected: skill snapshot required (D11-C2)")
	}
	if reason == "" {
		return 0, errors.New("hitl: MarkRejected: reason required (HITL-02)")
	}
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	return w.store.MarkRejected(ctx, store.ApprovalResolutionRow{
		Timestamp:            w.clock(),
		SessionID:            sessionID,
		UserID:               userID,
		SkillID:              snap.SkillID,
		Query:                target,
		PromptHash:           snap.PromptHash,
		SkillVersion:         snap.Version,
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        reason,
		ChunkIDs:             "",
		FrozenArgs:           frozenArgs,
		ToolCallID:           toolCallID,
		ApprovalID:           approvalID,
		ScopeSessionID:       sessionID,
		PlatformFloorVersion: w.floorVersion,
	})
}

// MarkChangeRequested inserts the D18-D4 third-state resolution row
// (action_change_requested) when the operator sends a draft back to
// the LLM with a change reason (要求修改). The reason is REQUIRED
// (HITL-02 posture extended to the third state — the LLM's revision
// loop and the audit chain both need it) and is stored in
// RefusalReason, mirroring MarkRejected's column usage.
func (w *AuditWriter) MarkChangeRequested(
	ctx context.Context,
	sessionID, userID string,
	snap SkillSnapshot,
	toolCallID, approvalID, target string,
	frozenArgs []byte,
	reason string,
) (int64, error) {
	if snap.IsZero() {
		return 0, errors.New("hitl: MarkChangeRequested: skill snapshot required (D11-C2)")
	}
	if reason == "" {
		return 0, errors.New("hitl: MarkChangeRequested: reason required (D18-D4)")
	}
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	return w.store.MarkChangeRequested(ctx, store.ApprovalResolutionRow{
		Timestamp:            w.clock(),
		SessionID:            sessionID,
		UserID:               userID,
		SkillID:              snap.SkillID,
		Query:                target,
		PromptHash:           snap.PromptHash,
		SkillVersion:         snap.Version,
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        reason,
		ChunkIDs:             "",
		FrozenArgs:           frozenArgs,
		ToolCallID:           toolCallID,
		ApprovalID:           approvalID,
		ScopeSessionID:       sessionID,
		PlatformFloorVersion: w.floorVersion,
	})
}

// MarkRuleRevoked inserts the action_rule_revoked row when an
// approve-and-remember rule is DELETEd (D11-D2 / D11-D4). The rule row
// itself is removed by internal/store/sqlite/approval_rules.go (11-05);
// the audit chain only ever grows (T-3-AUDIT INSERT-only).
func (w *AuditWriter) MarkRuleRevoked(
	ctx context.Context,
	sessionID, userID string,
	snap SkillSnapshot,
	toolCallID, target string,
	frozenArgs []byte,
	reason string,
) (int64, error) {
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	if snap.IsZero() {
		return 0, errors.New("hitl: MarkRuleRevoked: skill snapshot required (D11-C2)")
	}
	return w.store.MarkRuleRevoked(ctx, store.RuleRevokeRow{
		Timestamp:            w.clock(),
		SessionID:            sessionID,
		UserID:               userID,
		SkillID:              snap.SkillID,
		Query:                target,
		PromptHash:           snap.PromptHash,
		SkillVersion:         snap.Version,
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        reason,
		ChunkIDs:             "",
		FrozenArgs:           frozenArgs,
		ToolCallID:           toolCallID,
		ApprovalID:           toolCallID, // D11-C4
		ScopeSessionID:       sessionID,
		PlatformFloorVersion: w.floorVersion,
	})
}

// MarkAutoPassedRule inserts an action_approved audit row when the
// HITL Tool executed a write directly because an approve-and-remember
// rule matched (D11-D1 auto-pass path — HITL-08). There is NO pending
// row on this path: the rule match happens BEFORE
// tool.StatefulInterrupt, so no interrupt ever occurred (T-3-AUDIT
// INSERT-only invariant preserved — the auto-pass chain is a single
// row, not the 3-row operator chain).
//
// RefusalReason = "auto_passed_rule" (LITERAL) is the flag that
// distinguishes a rule-matched execution from an operator-approved one
// in audit queries. SkillID / PromptHash / SkillVersion are left empty:
// the auto-pass row's identity is (tool_call_id, target, frozen_args)
// plus the scope_session_id, mirroring the rule key; the Skill snapshot
// belongs to the operator-interrupt chain.
func (w *AuditWriter) MarkAutoPassedRule(
	ctx context.Context,
	sessionID, userID string,
	toolCallID, target string,
	frozenArgs []byte,
) (int64, error) {
	if err := w.guardFloorVersion(); err != nil {
		return 0, err
	}
	if toolCallID == "" {
		return 0, errors.New("hitl: MarkAutoPassedRule: toolCallID required (D11-C4 single identity)")
	}
	row := store.ApprovalResolutionRow{
		Timestamp:            w.clock(),
		SessionID:            sessionID,
		UserID:               userID,
		SkillID:              "",
		Query:                target,
		PromptHash:           "",
		SkillVersion:         "",
		ModelVersion:         "",
		VerdictJSON:          "",
		RefusalReason:        "auto_passed_rule", // LITERAL: D11-D1 auto-pass flag
		ChunkIDs:             "",
		FrozenArgs:           frozenArgs,
		ToolCallID:           toolCallID,
		ApprovalID:           toolCallID, // D11-C4: one identity, two columns
		ScopeSessionID:       sessionID,
		PlatformFloorVersion: w.floorVersion,
	}
	return w.store.MarkApproved(ctx, row)
}

// clock returns the writer's timestamp, tolerating a zero-value
// AuditWriter (a caller that built the struct literally rather than
// through NewAuditWriter).
func (w *AuditWriter) clock() time.Time {
	if w.now == nil {
		return time.Now().UTC()
	}
	return w.now()
}

// ComputePromptHash returns the hex SHA-256 of the rendered system
// prompt body — the D11-C2 prompt snapshot.
//
// DEVIATION (documented): the plan specified
// ComputePromptHash(m *skill.Manifest) calling m.RenderSystemPrompt().
// internal/skill imports THIS package (validate.go), so hitl cannot
// import skill and the manifest never reaches this file. Taking the
// already-rendered body as a string keeps the digest byte-identical to
// what a manifest-aware caller would compute, and leaves the rendering
// where it belongs (internal/agent.RenderSystemPrompt).
//
// An empty body hashes to the empty string rather than to a digest of
// the empty string, so a caller that forgot to render the prompt
// produces a visibly-empty snapshot instead of a plausible-looking hash.
func ComputePromptHash(promptBody string) string {
	if promptBody == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(promptBody))
	return hex.EncodeToString(sum[:])
}
