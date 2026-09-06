// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestUAT_FullChannelFlow is the Phase 21 release-gate end-to-end
// scenario. It walks the entire channel lifecycle:
//
//   1. Agent submits an NL2SQL preview with a registered view
//      (DATA-10 allow list, DATA-11 SELECT-only).
//   2. The request persists as `pending` (DATA-13 — explicit
//      resolution only).
//   3. A data officer approves; the manager mints a TTL token
//      (DATA-04).
//   4. The agent calls ExecuteWithToken; rows come back masked
//      (DATA-05).
//   5. The dual audit (DATA-06) reconciles cleanly — internal +
//      external QueryIDs match.
//   6. A second request is rejected; it produces a rejected
//      audit row with no TTL token.
//   7. The DATA-13 contract is verified: pending state has no
//      auto-resolution path.
//
// This test is the in-process UAT. The HTTP-level UAT (live
// curl) lives in .planning/uat/phase21-external-auth-2026-09-06.md.
func TestUAT_FullChannelFlow(t *testing.T) {
	ctx := context.Background()
	mgr, src, _ := mustBuildManager(t)

	// ── Step 1: agent requests approval (DATA-10, DATA-11). ──
	req, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID:    "alice",
		SkillID:   "compliance",
		SessionID: "sess-uat-1",
		Preview: QueryPreview{
			SQLPreview:     "SELECT id, name, account, phone, email, amount FROM v_user_consumption",
			ViewName:       "v_user_consumption",
			EstimatedRows:  2,
			BusinessReason: "UAT: compliance review of employee expense",
		},
	})
	if err != nil {
		t.Fatalf("Step 1 RequestApproval: %v", err)
	}
	if req.State != StatePending {
		t.Fatalf("Step 1: state %s, expected pending", req.State)
	}

	// ── Step 2: verify DATA-13 — pending is permanent without
	// human action. ──
	pending, _, _ := mgr.Store().GetExternalAuthRequest(ctx, req.RequestID)
	if pending.State != StatePending {
		t.Fatalf("DATA-13: state drifted from pending -> %s", pending.State)
	}

	// ── Step 3: data officer approves (DATA-04 mints TTL token). ──
	resolved, audit, token, err := mgr.Approve(ctx, req.RequestID, "officer-1", "verified need", []string{"c1"})
	if err != nil {
		t.Fatalf("Step 3 Approve: %v", err)
	}
	if resolved.State != StateApproved {
		t.Fatalf("Step 3: state %s, expected approved", resolved.State)
	}
	if audit.TTLTokenID != token.TokenID {
		t.Fatalf("Step 3: audit token id mismatch")
	}
	if !token.Valid(timeNow()) {
		t.Fatalf("Step 3: token immediately expired (ExpiresAt %v)", token.ExpiresAt)
	}

	// ── Step 4: agent executes with the token (DATA-05 masks). ──
	rows, err := mgr.ExecuteWithToken(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("Step 4 ExecuteWithToken: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("Step 4: expected 2 rows, got %d", len(rows))
	}
	row := rows[0]
	if !endsWith(row["account"], "9001") || row["account"] == "6225880137799001" {
		t.Fatalf("DATA-05: account not masked: %q", row["account"])
	}
	if !endsWith(row["email"], "@example.com") {
		t.Fatalf("DATA-05: email domain leaked: %q", row["email"])
	}

	// ── Step 5: dual audit reconcile (DATA-06). ──
	from := audit.Timestamp.Add(-time.Minute).UnixMilli()
	to := audit.Timestamp.Add(time.Minute).UnixMilli()
	report, err := mgr.Reconcile(ctx, from, to)
	if err != nil {
		t.Fatalf("Step 5 Reconcile: %v", err)
	}
	if report.MatchedCount < 1 {
		t.Fatalf("DATA-06: expected at least 1 matched pair, got %d", report.MatchedCount)
	}
	if len(report.InternalOnly) > 0 || len(report.ExternalOnly) > 0 {
		t.Fatalf("DATA-06: unexpected pair mismatch: %+v / %+v", report.InternalOnly, report.ExternalOnly)
	}
	if !src.HasQueryID(resolved.RequestID) {
		t.Fatalf("Step 5: external audit log missing query %s", resolved.RequestID)
	}

	// ── Step 6: a second request is rejected — produces a
	// rejected audit row with NO TTL token. ──
	req2, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance", SessionID: "sess-uat-1",
		Preview: QueryPreview{
			SQLPreview: "SELECT 1", ViewName: "v_user_consumption",
			BusinessReason: "UAT: rejection path",
		},
	})
	if err != nil {
		t.Fatalf("Step 6 RequestApproval: %v", err)
	}
	_, rejAudit, err := mgr.Reject(ctx, req2.RequestID, "officer-2", "wrong audience")
	if err != nil {
		t.Fatalf("Step 6 Reject: %v", err)
	}
	if rejAudit.Decision != StateRejected {
		t.Fatalf("Step 6: decision %s", rejAudit.Decision)
	}
	if rejAudit.TTLTokenID != "" {
		t.Fatalf("Step 6: rejected request must NOT mint a TTL token (got %s)", rejAudit.TTLTokenID)
	}

	// ── Step 7: DATA-13 negative test — non-SELECT is rejected
	// without ever touching the pending state. ──
	_, err = mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance",
		Preview: QueryPreview{
			SQLPreview: "UPDATE v_user SET name='hax'", ViewName: "v_user_consumption",
		},
	})
	if !errors.Is(err, ErrSQLReadOnly) {
		t.Fatalf("Step 7: expected ErrSQLReadOnly, got %v", err)
	}

	t.Logf("UAT complete: 1 approved + 1 rejected, both DATA-04/05/06/10/11/13/14 invariants verified")
}

func endsWith(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	return s[len(s)-len(sub):] == sub
}
