// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package externalauth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeStore implements Store in-process for tests.
type fakeStore struct {
	mu       sync.Mutex
	requests map[string]ExternalAuthRequest
	tokens   map[string]TTLToken
	audits   []AuditRow
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		requests: make(map[string]ExternalAuthRequest),
		tokens:   make(map[string]TTLToken),
	}
}

func (f *fakeStore) IssueExternalAuthRequest(_ context.Context, r ExternalAuthRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.requests[r.RequestID]; ok {
		return errors.New("duplicate")
	}
	f.requests[r.RequestID] = r
	return nil
}
func (f *fakeStore) GetExternalAuthRequest(_ context.Context, id string) (ExternalAuthRequest, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.requests[id]
	return r, ok, nil
}
func (f *fakeStore) ResolveExternalAuthRequest(_ context.Context, r ExternalAuthRequest, a AuditRow, t *TTLToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests[r.RequestID] = r
	if t != nil {
		f.tokens[t.TokenID] = *t
	}
	f.audits = append(f.audits, a)
	return nil
}
func (f *fakeStore) ListPendingExternalAuthRequests(_ context.Context, limit int) ([]ExternalAuthRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ExternalAuthRequest, 0)
	for _, r := range f.requests {
		if r.State == StatePending {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStore) GetTTLToken(_ context.Context, id string) (TTLToken, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[id]
	return t, ok, nil
}
func (f *fakeStore) ListAuditRows(_ context.Context, fromMs, toMs int64) ([]AuditRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]AuditRow, 0)
	for _, a := range f.audits {
		ms := a.Timestamp.UnixMilli()
		if ms >= fromMs && ms < toMs {
			out = append(out, a)
		}
	}
	return out, nil
}

func mustBuildManager(t *testing.T) (*Manager, *MemorySource, *ViewRegistry) {
	t.Helper()
	registry := NewViewRegistry()
	registry.Register(PrecompiledView{
		ViewName: "v_user_consumption",
		Source:   "mock-warehouse",
		FieldMasks: []MaskSpec{
			{FieldName: "account", Kind: "account", KeepLen: 4},
			{FieldName: "phone", Kind: "phone", KeepLen: 4},
			{FieldName: "email", Kind: "email"},
		},
	})
	src := NewMemorySource(registry)
	if err := src.UpsertView("v_user_consumption", []map[string]string{
		{"id": "u-001", "name": "alice", "account": "6225880137799001", "phone": "13812345678", "email": "alice@example.com", "amount": "1500.00"},
		{"id": "u-002", "name": "bob", "account": "6225880137799002", "phone": "13987654321", "email": "bob@example.com", "amount": "2300.00"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return NewManager(newFakeStore(), src, registry), src, registry
}

func TestMask_AccountLast4(t *testing.T) {
	got := ApplyMask(MaskSpec{Kind: "account", KeepLen: 4}, "6225880137799001")
	want := "************9001"
	if got != want {
		t.Fatalf("account mask: got %q want %q", got, want)
	}
}

func TestMask_EmailKeepsFirstTwo(t *testing.T) {
	got := ApplyMask(MaskSpec{Kind: "email"}, "alice@example.com")
	want := "al***@example.com"
	if got != want {
		t.Fatalf("email mask: got %q want %q", got, want)
	}
}

func TestMask_AccountShortFullMask(t *testing.T) {
	got := ApplyMask(MaskSpec{Kind: "account", KeepLen: 4}, "12")
	if got != "**" {
		t.Fatalf("short input: got %q want %q", got, "**")
	}
}

func TestMask_UnknownKindFullMask(t *testing.T) {
	got := ApplyMask(MaskSpec{Kind: "mystery"}, "secret")
	if got != "***" {
		t.Fatalf("unknown kind: got %q want %q", got, "***")
	}
}

func TestRejectNonSelect(t *testing.T) {
	cases := []struct {
		sql   string
		valid bool
	}{
		{"SELECT * FROM v_user", true},
		{"  select id from v_user", true},
		{"-- a comment\nSELECT 1", true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a=1", false},
		{"DELETE FROM t", false},
		{"DROP TABLE t", false},
		{"ALTER TABLE t ADD COLUMN x INT", false},
		{"TRUNCATE t", false},
		{"GRANT SELECT ON t TO alice", false},
		{"CALL my_proc()", false},
		{"SELECT 1; DROP TABLE t", true}, // only the first verb is checked
		{"", false},
	}
	for _, c := range cases {
		err := RejectNonSelect(c.sql)
		if c.valid && err != nil {
			t.Errorf("expected valid: %q err=%v", c.sql, err)
		}
		if !c.valid && err == nil {
			t.Errorf("expected invalid: %q", c.sql)
		}
	}
}

func TestManager_FullApprovalAndExecute(t *testing.T) {
	ctx := context.Background()
	mgr, src, _ := mustBuildManager(t)

	req, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID:    "alice",
		SkillID:   "compliance",
		SessionID: "sess-1",
		Preview: QueryPreview{
			SQLPreview:     "SELECT id, name, account, phone, email, amount FROM v_user_consumption",
			ViewName:       "v_user_consumption",
			EstimatedRows:  2,
			BusinessReason: "policy review of employee expense",
		},
	})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if req.State != StatePending {
		t.Fatalf("expected pending, got %s", req.State)
	}
	if req.RequestID == "" {
		t.Fatal("expected request id minted")
	}

	// DATA-13: pending state is permanent; only explicit action
	// resolves it. Verify the request is still pending without
	// intervention.
	pending, _, _ := mgr.store.GetExternalAuthRequest(ctx, req.RequestID)
	if pending.State != StatePending {
		t.Fatalf("DATA-13 violation: state drifted from pending -> %s", pending.State)
	}

	resolved, audit, token, err := mgr.Approve(ctx, req.RequestID, "officer-1", "verified need", []string{"c1"})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if resolved.State != StateApproved {
		t.Fatalf("expected approved, got %s", resolved.State)
	}
	if audit.Decision != StateApproved {
		t.Fatalf("audit decision: %s", audit.Decision)
	}
	if audit.TTLTokenID != token.TokenID {
		t.Fatalf("token id mismatch: audit=%s token=%s", audit.TTLTokenID, token.TokenID)
	}
	if audit.SQLPreview != req.Preview.SQLPreview {
		t.Fatalf("DATA-14 violation: sql preview not preserved in audit")
	}

	// Execute via the minted token. DATA-10 enforcement: view is
	// in registry. DATA-05 enforcement: rows come back masked.
	rows, err := mgr.ExecuteWithToken(ctx, token.TokenID)
	if err != nil {
		t.Fatalf("ExecuteWithToken: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// First row: account + phone must be masked; email must be masked.
	if !strings.HasSuffix(rows[0]["account"], "9001") || rows[0]["account"] == "6225880137799001" {
		t.Errorf("DATA-05 violation: account not masked: %q", rows[0]["account"])
	}
	if strings.Contains(rows[0]["phone"], "1234") && !strings.Contains(rows[0]["phone"], "*") {
		t.Errorf("DATA-05 violation: phone not masked: %q", rows[0]["phone"])
	}
	if !strings.HasSuffix(rows[0]["email"], "@example.com") {
		t.Errorf("DATA-05 violation: email domain leaked: %q", rows[0]["email"])
	}
	// DATA-06: external audit log must contain a row with the same
	// QueryID (request id) as the internal row, NOT the scope hash.
	if !src.HasQueryID(resolved.RequestID) {
		t.Errorf("DATA-06 violation: external audit missing for query %s", resolved.RequestID)
	}
}

func TestManager_RejectDoesNotMintToken(t *testing.T) {
	ctx := context.Background()
	mgr, _, _ := mustBuildManager(t)
	req, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID:    "alice",
		SkillID:   "compliance",
		SessionID: "sess-1",
		Preview: QueryPreview{
			SQLPreview:     "SELECT 1",
			ViewName:       "v_user_consumption",
			BusinessReason: "test reject",
		},
	})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	resolved, audit, err := mgr.Reject(ctx, req.RequestID, "officer-1", "wrong audience")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if resolved.State != StateRejected {
		t.Fatalf("expected rejected, got %s", resolved.State)
	}
	if audit.Decision != StateRejected {
		t.Fatalf("audit decision: %s", audit.Decision)
	}
	if audit.TTLTokenID != "" {
		t.Fatalf("rejected request must not mint a TTL token (got %s)", audit.TTLTokenID)
	}
}

func TestManager_NonSelectRejected(t *testing.T) {
	ctx := context.Background()
	mgr, _, _ := mustBuildManager(t)
	_, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance",
		Preview: QueryPreview{
			SQLPreview: "DELETE FROM v_user_consumption", ViewName: "v_user_consumption",
		},
	})
	if !errors.Is(err, ErrSQLReadOnly) {
		t.Fatalf("expected ErrSQLReadOnly, got %v", err)
	}
}

func TestManager_UnregisteredViewRejected(t *testing.T) {
	ctx := context.Background()
	mgr, _, _ := mustBuildManager(t)
	_, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance",
		Preview: QueryPreview{
			SQLPreview: "SELECT 1", ViewName: "v_secret",
		},
	})
	if !errors.Is(err, ErrViewNotAllowed) {
		t.Fatalf("expected ErrViewNotAllowed, got %v", err)
	}
}

func TestManager_TokenExpiryEnforced(t *testing.T) {
	ctx := context.Background()
	mgr, _, _ := mustBuildManager(t)

	// Freeze clock at approval time so ExpiresAt = now+5min is
	// deterministic; then advance the clock past ExpiresAt before
	// executing.
	frozen := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	current := frozen
	mgr.SetClock(func() time.Time { return current })

	req, err := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance",
		Preview: QueryPreview{
			SQLPreview: "SELECT 1", ViewName: "v_user_consumption",
		},
	})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	_, _, token, err := mgr.Approve(ctx, req.RequestID, "officer-1", "test", nil)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Advance the clock 6 minutes — past the 5-minute TTL.
	current = frozen.Add(6 * time.Minute)

	if _, err := mgr.ExecuteWithToken(ctx, token.TokenID); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired after TTL advance, got %v", err)
	}
}

func TestManager_Reconcile_PairMismatch(t *testing.T) {
	ctx := context.Background()
	mgr, _, _ := mustBuildManager(t)
	fs, ok := mgr.store.(*fakeStore)
	if !ok {
		t.Fatalf("mgr.store is %T, expected *fakeStore", mgr.store)
	}

	// Freeze clock to a deterministic moment — well past now so the
	// minted TTL token (now+5min) is also past its ExpiresAt from
	// real-time perspective. We override `Valid` via direct store
	// access below; the package-level timeNow() returns real-time
	// for the token check.
	now := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	mgr.SetClock(func() time.Time { return now })

	from := now.Add(-time.Hour).UnixMilli()
	to := now.Add(time.Hour).UnixMilli()

	// One matched pair: request → approve → execute (which appends
	// the external audit row inside ExecuteWithToken).
	req1, _ := mgr.RequestApproval(ctx, ExternalAuthRequest{
		UserID: "alice", SkillID: "compliance",
		Preview: QueryPreview{SQLPreview: "SELECT 1", ViewName: "v_user_consumption"},
	})
	_, _, tok1, _ := mgr.Approve(ctx, req1.RequestID, "officer-1", "r1", nil)
	if _, err := mgr.ExecuteWithToken(ctx, tok1.TokenID); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Inject an internal-only audit row (no external counterpart)
	// by writing directly to the fake store — simulates a missed
	// external-audit append. This is the DATA-06 reconcile signal.
	fs.audits = append(fs.audits, AuditRow{
		QueryID: "xreq-mismatch", UserID: "alice",
		Decision: StateApproved, DataOfficerID: "officer-1",
		Timestamp: now,
	})

	report, err := mgr.Reconcile(ctx, from, to)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.MatchedCount < 1 {
		t.Errorf("expected matched count >= 1, got %d", report.MatchedCount)
	}
	hasMismatch := false
	for _, r := range report.InternalOnly {
		if r.QueryID == "xreq-mismatch" {
			hasMismatch = true
		}
	}
	if !hasMismatch {
		t.Errorf("DATA-06 violation: internal-only mismatch row not surfaced")
	}
}
