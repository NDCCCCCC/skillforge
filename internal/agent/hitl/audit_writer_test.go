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

// Package hitl_test black-box tests the HITL-04 audit writer against a
// real (in-process, file-backed) SQLiteStore. The package is external
// (hitl_test) so the tests exercise ONLY the exported surface — the same
// call shape 11-04's dual approve/deny endpoint will consume.
//
// Every test here is hermetic: a temp-dir SQLite file, no live model, no
// network.
package hitl_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/agent/hitl"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSnapshot builds the D11-C2 Skill snapshot the writer stamps onto
// every audit row.
//
// DEVIATION note: plan 11-03 called for a *skill.Manifest here.
// internal/skill imports internal/agent/hitl (validate.go), so the
// production writer cannot take a manifest — it takes hitl.SkillSnapshot
// instead, which the caller layer builds from the manifest. The test
// mirrors that production shape.
func testSnapshot(t *testing.T, version string) hitl.SkillSnapshot {
	t.Helper()
	return hitl.NewSkillSnapshot("compliance", version, "You are a strict compliance auditor. (test prompt body)")
}

// newTestStore opens a fresh file-backed SQLiteStore under t.TempDir().
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "hitl-test.db")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestFailClosedAudit — HITL-04 P0 no-waiver gate (D11-C3).
//
// The store is closed BEFORE the write, so every INSERT fails. The
// invariant under test: the failure ESCAPES via the error return, wrapped
// in the fail-closed marker, and no row id is produced. The caller (the
// HITL Tool) then returns that error from InvokableRun, which means the
// Runner never signals the interrupt, never writes a checkpoint, and
// never executes the write tool.
//
// If anyone ever copies the store read-path's swallow posture into
// audit_writer.go, this test is what fails.
func TestFailClosedAudit(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Close(), "close the store FIRST so every INSERT fails")

	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")

	id, err := w.WritePendingApproval(context.Background(),
		"sess-1", "user-1", snap,
		"tc-failclosed", "memory:facts",
		[]byte(`{"key":"value"}`), "agent reason")
	require.Error(t, err, "audit INSERT must fail when the store is closed — fail-closed invariant")
	assert.Contains(t, err.Error(), "audit INSERT failed (fail-closed)",
		"the error must carry the fail-closed wrapper, not a bare driver error")
	assert.Equal(t, int64(0), id, "no audit row id may be returned on failure")

	// The seam the HITL Tool actually calls must fail closed too — it is
	// the SAME INSERT path, not a parallel one.
	seamErr := w.WithIdentity("sess-1", "user-1", snap).InsertPendingApproval(
		context.Background(), "tc-failclosed-seam", &hitl.ApprovalInfo{
			ToolName:        hitl.MemoryFactsToolName,
			ArgumentsInJSON: `{"key":"value"}`,
			ToolCallID:      "tc-failclosed-seam",
			Target:          hitl.MemoryFactsTarget,
		})
	require.Error(t, seamErr, "the auditWriterHook seam must fail closed as well")
	assert.Contains(t, seamErr.Error(), "audit INSERT failed (fail-closed)")
}

// TestAuditChainThreeRowLifecycle — the HITL-04 chain is INSERT-only:
// action_pending_approval → action_approved | action_rejected, each a
// SEPARATE row. No stage ever mutates an earlier row (T-3-AUDIT).
func TestAuditChainThreeRowLifecycle(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	frozen := []byte(`{"key":"audit-chain"}`)
	ctx := context.Background()

	pendingID, err := w.WritePendingApproval(ctx,
		"sess-1", "user-1", snap, "tc-chain-1", "memory:facts", frozen, "agent reason")
	require.NoError(t, err)
	require.Greater(t, pendingID, int64(0))

	approvedID, err := w.MarkApproved(ctx,
		"sess-1", "user-1", snap, "tc-chain-1", "tc-chain-1", "memory:facts", frozen, "operator reason")
	require.NoError(t, err)
	require.Greater(t, approvedID, int64(0))
	assert.NotEqual(t, pendingID, approvedID,
		"the approved row is a NEW row, not an UPDATE of the pending row (T-3-AUDIT)")

	// Reject branch — a different session and a different tool_call_id.
	pendingID2, err := w.WritePendingApproval(ctx,
		"sess-2", "user-1", snap, "tc-chain-2", "review_report", frozen, "agent reason")
	require.NoError(t, err)

	rejectedID, err := w.MarkRejected(ctx,
		"sess-2", "user-1", snap, "tc-chain-2", "tc-chain-2", "review_report", frozen, "operator reject reason")
	require.NoError(t, err)
	require.Greater(t, rejectedID, int64(0))
	assert.NotEqual(t, pendingID2, rejectedID)

	// HITL-02: a reject without a reason is refused at the writer, so no
	// reasonless rejection can reach the audit table.
	_, err = w.MarkRejected(ctx,
		"sess-2", "user-1", snap, "tc-chain-2", "tc-chain-2", "review_report", frozen, "")
	require.Error(t, err, "MarkRejected must require a reason (HITL-02)")

	rows, err := s.QueryAudit(ctx, store.AuditFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, rows, 4, "2 pending + 1 approved + 1 rejected")

	var pendingRow *store.AuditRow
	actions := map[string]int{}
	for i := range rows {
		actions[rows[i].Action]++
		if rows[i].ToolCallID == "tc-chain-1" && rows[i].Action == "action_pending_approval" {
			pendingRow = &rows[i]
		}
	}
	assert.Equal(t, 2, actions["action_pending_approval"])
	assert.Equal(t, 1, actions["action_approved"])
	assert.Equal(t, 1, actions["action_rejected"])

	require.NotNil(t, pendingRow, "the pending row must exist")
	assert.Equal(t, frozen, pendingRow.FrozenArgs,
		"frozen_args must be byte-identical to what WritePendingApproval received")
	assert.Equal(t, "tc-chain-1", pendingRow.ApprovalID, "approval_id == tool_call_id (D11-C4)")
	assert.Equal(t, snap.Version, pendingRow.SkillVersion, "D11-C2 skill_version snapshot")
	assert.Equal(t, snap.PromptHash, pendingRow.PromptHash, "D11-C2 prompt_hash snapshot")
}

// TestApprovalArgsByteEquality — the 3-way byte-identity invariant:
// ApprovalCard preview bytes == Tool argumentsInJSON == audit
// frozen_args. sha256(frozenArgs) is the D11-D1 rule-match key, so any
// in-flight mutation must be visible as a hash change.
func TestApprovalArgsByteEquality(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	ctx := context.Background()

	// The 3 representations of the same args.
	previewJSON := []byte(`{"key":"employee_count=150","reason":"approved budget"}`)
	toolArgsJSON := previewJSON // the Tool sees the bytes the LLM emitted
	auditFrozen := previewJSON  // the audit row stores those same bytes

	expectedHex := hexSum(previewJSON)
	assert.Equal(t, expectedHex, hexSum(toolArgsJSON), "preview == tool sha256")
	assert.Equal(t, expectedHex, hexSum(auditFrozen), "preview == audit sha256")

	_, err := w.WritePendingApproval(ctx, "sess-byte", "user-1", snap,
		"tc-byte-1", "memory:facts", auditFrozen, "reason")
	require.NoError(t, err)
	approvedID, err := w.MarkApproved(ctx, "sess-byte", "user-1", snap,
		"tc-byte-1", "tc-byte-1", "memory:facts", toolArgsJSON, "operator reason")
	require.NoError(t, err)
	require.Greater(t, approvedID, int64(0))

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-byte", Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, auditFrozen, r.FrozenArgs,
			"frozen_args must be byte-identical across the pending + approved rows")
		assert.Equal(t, expectedHex, hexSum(r.FrozenArgs),
			"the persisted bytes must hash to the same D11-D1 rule key")
	}

	// An edit-before-approve (HITL-07) MUST change the hash — that is
	// what invalidates a remembered rule.
	edited := []byte(`{"key":"employee_count=151","reason":"approved budget"}`)
	assert.NotEqual(t, expectedHex, hexSum(edited),
		"edited args must produce a different rule key (D11-D1)")
}

// TestApprovalIDIdentityInvariant — approval_id == tool_call_id (D11-C4)
// holds across every lifecycle method the writer exposes.
func TestApprovalIDIdentityInvariant(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	ctx := context.Background()
	frozen := []byte(`{"x":1}`)

	_, err := w.WritePendingApproval(ctx, "sess-id", "user-1", snap,
		"tc-identity", "memory:facts", frozen, "r")
	require.NoError(t, err)
	_, err = w.MarkApproved(ctx, "sess-id", "user-1", snap,
		"tc-identity", "tc-identity", "memory:facts", frozen, "r")
	require.NoError(t, err)
	_, err = w.MarkRuleRevoked(ctx, "sess-id", "user-1", snap,
		"tc-identity-rule", "memory:facts", frozen, "r")
	require.NoError(t, err)

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-id", Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	for _, r := range rows {
		assert.Equal(t, r.ToolCallID, r.ApprovalID, "approval_id == tool_call_id (D11-C4)")
		assert.Equal(t, "sess-id", r.ScopeSessionID, "rule scope binds to the session (D11-D2)")
	}

	var foundRuleRevoke bool
	for _, r := range rows {
		if r.Action == "action_rule_revoked" && r.ToolCallID == "tc-identity-rule" {
			foundRuleRevoke = true
		}
	}
	assert.True(t, foundRuleRevoke, "the action_rule_revoked row must carry its own tool_call_id")
}

// TestAuditWriterPreconditions — the writer refuses to emit an audit row
// that could not support the D11-C2 expire check or the D11-C4 identity.
func TestAuditWriterPreconditions(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	ctx := context.Background()

	_, err := w.WritePendingApproval(ctx, "sess", "user", testSnapshot(t, "v1"),
		"", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err, "an empty toolCallID breaks the D11-C4 single identity")

	_, err = w.WritePendingApproval(ctx, "sess", "user", hitl.SkillSnapshot{},
		"tc-1", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err, "a zero Skill snapshot breaks the D11-C2 expire check")
}

// TestAuditPlanApproveTargetDiscriminator — Plan 12-03 / D11-B3
// invariant: plan_approve audit rows share the action='action_pending_approval'
// discriminator with every other HITL target; the per-target
// discriminator is the audit row's `target` column. InsertPendingApproval
// + MarkApproved with Target="plan_approve" produces 2 rows whose
// action="action_pending_approval" / "action_approved" both carry
// target="plan_approve" (the timeline-tab filter key).
func TestAuditPlanApproveTargetDiscriminator(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	ctx := context.Background()

	pendingID, err := w.WritePendingApproval(ctx,
		"sess-plan", "user-1", snap, "tc-plan-1",
		hitl.PlanApproveTarget,
		[]byte(`{"planId":"plan-discriminator","steps":[{"id":"s1","content":"x","status":"pending"}]}`),
		"agent reason",
	)
	require.NoError(t, err)
	require.Greater(t, pendingID, int64(0))

	approvedID, err := w.MarkApproved(ctx,
		"sess-plan", "user-1", snap, "tc-plan-1", "tc-plan-1",
		hitl.PlanApproveTarget,
		[]byte(`{"planId":"plan-discriminator","steps":[{"id":"s1","content":"x","status":"pending"}]}`),
		"operator reason",
	)
	require.NoError(t, err)
	require.Greater(t, approvedID, int64(0))
	assert.NotEqual(t, pendingID, approvedID,
		"the approved row is a NEW row, not an UPDATE of the pending row (T-3-AUDIT)")

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-plan", Limit: 100})
	require.NoError(t, err)
	require.Len(t, rows, 2,
		"the session-scoped query must return the 2-row chain (pending + approved) for plan_approve")

	// Action enum is uniform across HITL targets — the discriminator
	// is the target column, NOT a per-target action value.
	var pendingRow, approvedRow *store.AuditRow
	for i := range rows {
		switch rows[i].Action {
		case "action_pending_approval":
			pendingRow = &rows[i]
		case "action_approved":
			approvedRow = &rows[i]
		}
		assert.Equal(t, hitl.PlanApproveTarget, rows[i].Query,
			"every plan_approve audit row must carry target=\"plan_approve\"")
	}
	require.NotNil(t, pendingRow, "the pending row must exist")
	require.NotNil(t, approvedRow, "the approved row must exist")
	assert.Equal(t, "tc-plan-1", pendingRow.ToolCallID,
		"D11-C4 single identity on the pending row")
	assert.Equal(t, "tc-plan-1", pendingRow.ApprovalID,
		"approval_id == tool_call_id (D11-C4)")
	assert.Equal(t, "tc-plan-1", approvedRow.ToolCallID)
	assert.Equal(t, "tc-plan-1", approvedRow.ApprovalID)
}

// TestAuditPlanApproveFullLifecycle — Plan 12-03 / HITL-04 mirror:
// the plan_approve audit chain is the same 2-row (pending → approved/
// rejected) shape as every other HITL target. This test exercises
// the full lifecycle with Target="plan_approve" + verifies the
// approval_id invariant holds across both rows.
func TestAuditPlanApproveFullLifecycle(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	ctx := context.Background()
	frozen := []byte(`{"planId":"plan-lifecycle","steps":[{"id":"s1","content":"x","status":"pending"},{"id":"s2","content":"y","status":"pending"}]}`)

	// Pass 1: InsertPendingApproval with target="plan_approve".
	pendingID, err := w.WritePendingApproval(ctx,
		"sess-lifecycle", "user-1", snap, "tc-lifecycle-1",
		hitl.PlanApproveTarget, frozen, "agent reason")
	require.NoError(t, err)
	require.Greater(t, pendingID, int64(0))

	// Pass 2: MarkApproved completes the 2-row chain.
	approvedID, err := w.MarkApproved(ctx,
		"sess-lifecycle", "user-1", snap, "tc-lifecycle-1", "tc-lifecycle-1",
		hitl.PlanApproveTarget, frozen, "operator reason")
	require.NoError(t, err)
	require.Greater(t, approvedID, int64(0))
	assert.NotEqual(t, pendingID, approvedID,
		"approved row is a NEW row, not an UPDATE of pending (T-3-AUDIT INSERT-only)")

	// D11-C4 identity invariant across both rows.
	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-lifecycle", Limit: 10})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, r.ToolCallID, r.ApprovalID,
			"approval_id == tool_call_id must hold for plan_approve rows (D11-C4)")
		assert.Equal(t, hitl.PlanApproveTarget, r.Query,
			"target column must carry the plan_approve discriminator")
		assert.Equal(t, frozen, r.FrozenArgs,
			"frozen_args must byte-equal across pending + approved (HITL-07)")
		assert.Equal(t, snap.Version, r.SkillVersion,
			"D11-C2 skill_version snapshot stamped on plan_approve audit rows")
		assert.Equal(t, snap.PromptHash, r.PromptHash,
			"D11-C2 prompt_hash snapshot stamped on plan_approve audit rows")
	}
}

// hexSum is the sha256 rule-key helper (D11-D1).
func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestHITLPlatformFloorVersionRows — PLAT-10 D14-B2: the five HITL
// audit INSERT sites (pending / approved / rejected / rule-revoked /
// auto-passed-rule) all stamp platform_floor_version on the audit row.
// The value is injected via NewAuditWriter so the hitl package never
// imports internal/agent (import-cycle avoidance).
func TestHITLPlatformFloorVersionRows(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, agent.PlatformFloorVersion)
	snap := testSnapshot(t, "v1")
	ctx := context.Background()
	frozen := []byte(`{"key":"floor-test"}`)

	pendingID, err := w.WritePendingApproval(ctx,
		"sess-floor", "user-1", snap, "tc-floor-1", "memory:facts", frozen, "agent reason")
	require.NoError(t, err)
	require.Greater(t, pendingID, int64(0))

	approvedID, err := w.MarkApproved(ctx,
		"sess-floor", "user-1", snap, "tc-floor-1", "tc-floor-1", "memory:facts", frozen, "operator reason")
	require.NoError(t, err)
	require.Greater(t, approvedID, int64(0))

	rejectedID, err := w.MarkRejected(ctx,
		"sess-floor", "user-1", snap, "tc-floor-2", "tc-floor-2", "memory:facts", frozen, "operator reject reason")
	require.NoError(t, err)
	require.Greater(t, rejectedID, int64(0))

	revokedID, err := w.MarkRuleRevoked(ctx,
		"sess-floor", "user-1", snap, "tc-floor-3", "memory:facts", frozen, "operator revoke reason")
	require.NoError(t, err)
	require.Greater(t, revokedID, int64(0))

	autoPassID, err := w.MarkAutoPassedRule(ctx,
		"sess-floor", "user-1", "tc-floor-4", "memory:facts", frozen)
	require.NoError(t, err)
	require.Greater(t, autoPassID, int64(0))

	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-floor", Limit: 50})
	require.NoError(t, err)
	require.Len(t, rows, 5, "pending + approved + rejected + revoked + auto-pass = 5 rows")

	for _, r := range rows {
		assert.Equal(t, agent.PlatformFloorVersion, r.PlatformFloorVersion,
			"every HITL row must carry the platform floor version (action=%s)", r.Action)
	}

	// The auto-pass row intentionally leaves SkillID/PromptHash empty,
	// but platform_floor_version is still mandatory.
	for _, r := range rows {
		if r.Action == "action_approved" && r.RefusalReason == "auto_passed_rule" {
			assert.Empty(t, r.SkillID, "auto-pass row keeps SkillID empty")
			assert.Equal(t, agent.PlatformFloorVersion, r.PlatformFloorVersion,
				"auto-pass row still carries platform_floor_version")
		}
	}
}

// TestHITLPlatformFloorVersionFailClosed — an empty floorVersion
// refuses to write any audit row, preventing new rows from forging a
// pre-floor drift window.
func TestHITLPlatformFloorVersionFailClosed(t *testing.T) {
	s := newTestStore(t)
	w := hitl.NewAuditWriter(s, "")
	snap := testSnapshot(t, "v1")
	ctx := context.Background()

	_, err := w.WritePendingApproval(ctx,
		"sess", "user", snap, "tc-1", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err)
	assert.ErrorIs(t, err, hitl.ErrMissingFloorVersion)

	_, err = w.MarkApproved(ctx,
		"sess", "user", snap, "tc-2", "tc-2", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err)
	assert.ErrorIs(t, err, hitl.ErrMissingFloorVersion)

	_, err = w.MarkRejected(ctx,
		"sess", "user", snap, "tc-3", "tc-3", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err)
	assert.ErrorIs(t, err, hitl.ErrMissingFloorVersion)

	_, err = w.MarkRuleRevoked(ctx,
		"sess", "user", snap, "tc-4", "memory:facts", []byte(`{}`), "r")
	require.Error(t, err)
	assert.ErrorIs(t, err, hitl.ErrMissingFloorVersion)

	_, err = w.MarkAutoPassedRule(ctx,
		"sess", "user", "tc-5", "memory:facts", []byte(`{}`))
	require.Error(t, err)
	assert.ErrorIs(t, err, hitl.ErrMissingFloorVersion)
}
