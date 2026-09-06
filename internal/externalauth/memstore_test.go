// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// memstoreLifecycleClock gives the lifecycle test a deterministic
// CreatedAt ordering (each issued request is 1 minute apart) so the
// FIFO assertion below is not racing the wall clock.
var memstoreLifecycleClock = func(start time.Time) func() time.Time {
	cur := start
	return func() time.Time {
		cur = cur.Add(time.Minute)
		return cur
	}
}(time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC))

// TestMemoryStore_Lifecycle walks the full Store contract with the
// production MemoryStore: issue -> get -> FIFO pending list -> approve
// resolution (token persisted + AuditRowID stamped) -> reject
// resolution (audit row with no token id).
func TestMemoryStore_Lifecycle(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()

	mkPreview := func(sql, view string) QueryPreview {
		return QueryPreview{SQLPreview: sql, ViewName: view}
	}

	// ① Issue three requests at increasing CreatedAt — ids minted the
	// way the Manager mints them.
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		r := ExternalAuthRequest{
			RequestID: FormatRequestID(int64(i + 1)),
			UserID:    "alice",
			SkillID:   "external-lookup",
			Preview:   mkPreview("SELECT 1", "v_user_consumption"),
			State:     StatePending,
			CreatedAt: memstoreLifecycleClock().UTC(),
		}
		if err := st.IssueExternalAuthRequest(ctx, r); err != nil {
			t.Fatalf("issue #%d: %v", i+1, err)
		}
		ids = append(ids, r.RequestID)
	}

	// ② Get round-trips the stored row; a missing id is (zero,false,nil).
	got, ok, err := st.GetExternalAuthRequest(ctx, ids[1])
	if err != nil || !ok {
		t.Fatalf("get %s: ok=%v err=%v", ids[1], ok, err)
	}
	if got.RequestID != ids[1] || got.State != StatePending {
		t.Fatalf("get round-trip mismatch: %+v", got)
	}
	if missing, ok, err := st.GetExternalAuthRequest(ctx, "xreq-does-not-exist"); ok || err != nil || missing.RequestID != "" {
		t.Fatalf("missing lookup must be (zero,false,nil), got (%+v,%v,%v)", missing, ok, err)
	}

	// ③ Pending list is FIFO oldest-first and honours a positive limit.
	pending, err := st.ListPendingExternalAuthRequests(ctx, 0)
	if err != nil {
		t.Fatalf("pending list: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}
	for i, want := range ids {
		if pending[i].RequestID != want {
			t.Fatalf("FIFO violation at %d: got %s want %s", i, pending[i].RequestID, want)
		}
	}
	truncated, err := st.ListPendingExternalAuthRequests(ctx, 2)
	if err != nil {
		t.Fatalf("pending list (limit): %v", err)
	}
	if len(truncated) != 2 || truncated[0].RequestID != ids[0] {
		t.Fatalf("limit must keep the oldest head: %+v", truncated)
	}

	// ④ Approve resolution persists the token and stamps AuditRowID.
	token := TTLToken{
		TokenID:   FormatTokenID(1),
		ViewName:  "v_user_consumption",
		UserID:    "alice",
		ScopeHash: "scope",
		IssuedAt:  time.Date(2026, 9, 1, 9, 10, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 9, 1, 9, 15, 0, 0, time.UTC),
	}
	approved := got
	approved.State = StateApproved
	approved.TTLTokenID = token.TokenID
	if err := st.ResolveExternalAuthRequest(ctx, approved, AuditRow{
		QueryID:        approved.RequestID,
		UserID:         approved.UserID,
		Decision:       StateApproved,
		TTLTokenID:     token.TokenID,
		Timestamp:      token.IssuedAt,
		SQLPreview:     approved.Preview.SQLPreview,
		BusinessReason: "lifecycle test approval",
	}, &token); err != nil {
		t.Fatalf("resolve approve: %v", err)
	}
	stored, ok, err := st.GetExternalAuthRequest(ctx, approved.RequestID)
	if err != nil || !ok {
		t.Fatalf("post-approve get: ok=%v err=%v", ok, err)
	}
	if stored.State != StateApproved {
		t.Fatalf("expected approved, got %s", stored.State)
	}
	if stored.AuditRowID <= 0 {
		t.Fatalf("DATA-14: AuditRowID must be stamped > 0, got %d", stored.AuditRowID)
	}
	persistedToken, ok, err := st.GetTTLToken(ctx, token.TokenID)
	if err != nil || !ok {
		t.Fatalf("approved token must be retrievable: ok=%v err=%v", ok, err)
	}
	if persistedToken.TokenID != token.TokenID || persistedToken.ViewName != "v_user_consumption" {
		t.Fatalf("token round-trip mismatch: %+v", persistedToken)
	}

	// ⑤ Reject resolution writes an audit row and mints NO token.
	rejected := pending[2]
	rejected.State = StateRejected
	if err := st.ResolveExternalAuthRequest(ctx, rejected, AuditRow{
		QueryID:    rejected.RequestID,
		UserID:     rejected.UserID,
		Decision:   StateRejected,
		Timestamp:  time.Date(2026, 9, 1, 9, 20, 0, 0, time.UTC),
		SQLPreview: rejected.Preview.SQLPreview,
	}, nil); err != nil {
		t.Fatalf("resolve reject: %v", err)
	}
	rows, err := st.ListAuditRows(ctx, 0, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC).UnixMilli())
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 audit rows (approve + reject), got %d", len(rows))
	}
	var rejectRow *AuditRow
	for i := range rows {
		if rows[i].Decision == StateRejected {
			rejectRow = &rows[i]
		}
	}
	if rejectRow == nil {
		t.Fatalf("reject audit row missing: %+v", rows)
	}
	if rejectRow.TTLTokenID != "" {
		t.Fatalf("rejected request must not carry a token id, got %q", rejectRow.TTLTokenID)
	}

	// ⑥ A second resolution of the same request is refused — the
	// pending -> resolved transition is a single write (insert-only
	// discipline), never a rewrite of history.
	approved.State = StateRejected
	if err := st.ResolveExternalAuthRequest(ctx, approved, AuditRow{QueryID: approved.RequestID, Timestamp: time.Now()}, nil); err == nil {
		t.Fatal("re-resolving a resolved request must fail (single-write discipline)")
	}
}

// TestMemoryStoreNoSQL is the structural lock for the store's
// insert-only posture: memstore.go must contain no mutation verbs in
// code or string literals. Comment lines are stripped BEFORE the match
// so the lock is structurally unable to trip on its own prose or the
// godoc's vocabulary — the assertion targets the executable text only.
//
// This package never executes SQL against a warehouse; the warehouse
// side of the channel is the Source interface (v1.2 drivers). A future
// edit that smuggles a statement string into the store breaks this
// test first.
func TestMemoryStoreNoSQL(t *testing.T) {
	src, err := os.ReadFile("memstore.go")
	if err != nil {
		t.Fatalf("read memstore.go: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		kept = append(kept, line)
	}
	code := strings.Join(kept, "\n")
	for _, verb := range []string{"INSERT INTO", "UPDATE ", "DELETE FROM"} {
		if strings.Contains(code, verb) {
			t.Errorf("memstore.go contains %q outside comments — the store must stay insert-only (no mutation verbs in code or string literals)", verb)
		}
	}
}
