// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/externalauth"
)

// extAuthFixtureView is the controlled view file the wiring tests load
// through the REAL loader (LoadViewsDir) so the registry the server
// boots with is the production shape.
const extAuthFixtureView = `
views:
  - view_name: v_user_consumption
    source: test fixture — read-only consumption replica
    columns:
      - name: user_id
        type: string
      - name: amount
        type: float
      - name: account_no
        type: string
      - name: phone
        type: string
      - name: email
        type: string
    field_masks:
      - field_name: account_no
        kind: account
        keep_len: 4
      - field_name: phone
        kind: phone
        keep_len: 4
      - field_name: email
        kind: email
        keep_len: 2
`

// newExternalAuthManager builds a fully wired Manager over the real
// MemoryStore + MemorySource + a registry loaded from a temp-dir
// controlled view file, seeded with two fixture rows.
func newExternalAuthManager(t *testing.T) *externalauth.Manager {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v_user_consumption.yaml"), []byte(extAuthFixtureView), 0o644))
	reg, err := externalauth.LoadViewsDir(dir)
	require.NoError(t, err)

	src := externalauth.NewMemorySource(reg)
	require.NoError(t, src.UpsertView("v_user_consumption", []map[string]string{
		{"user_id": "u-001", "amount": "1500.00", "account_no": "6225880137799001", "phone": "13812345678", "email": "alice@example.com"},
		{"user_id": "u-002", "amount": "2300.00", "account_no": "6225880137799002", "phone": "13987654321", "email": "bob@example.com"},
	}))
	return externalauth.NewManager(externalauth.NewMemoryStore(), src, reg)
}

// newExternalAuthServer builds the FULL production route table over the
// given Deps — the external-auth routes under test are the real ones
// bound by New, not a private re-binding.
func newExternalAuthServer(t *testing.T, deps *Deps) *server.Hertz {
	t.Helper()
	return New(":0", deps)
}

func extAuthRequest(t *testing.T, h *server.Hertz, method, path, adminToken string, body any) *ut.ResponseRecorder {
	t.Helper()
	if body == nil {
		return ut.PerformRequest(h.Engine, method, path, nil, extAuthHeaders(adminToken)...)
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	// Hertz's c.Bind routes on Content-Type: without application/json the
	// body is never decoded and every field silently arrives empty.
	headers := append([]ut.Header{{Key: "Content-Type", Value: "application/json"}}, extAuthHeaders(adminToken)...)
	return ut.PerformRequest(h.Engine, method, path, &ut.Body{Body: bytes.NewReader(raw), Len: len(raw)}, headers...)
}

func extAuthHeaders(adminToken string) []ut.Header {
	if adminToken == "" {
		return nil
	}
	return []ut.Header{{Key: "X-Admin-Token", Value: adminToken}}
}

// TestExternalAuth_WiredPath — the channel is live behind a wired
// manager: request answers 201 with an xreq- id, the data-officer
// surfaces gate on X-Admin-Token (403 without, 200 with), approve
// mints an xtok- token, and reconcile answers 200.
func TestExternalAuth_WiredPath(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
	deps := &Deps{ExternalAuthManager: newExternalAuthManager(t)}
	h := newExternalAuthServer(t, deps)

	// ① POST /request — un-gated agent surface → 201 + xreq- id.
	resp := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/request", "", map[string]any{
		"user_id":         "sess-e2e-1",
		"skill_id":        "external-lookup",
		"session_id":      "sess-e2e-1",
		"sql_preview":     "SELECT user_id, account_no FROM v_user_consumption",
		"view_name":       "v_user_consumption",
		"estimated_rows":  2,
		"business_reason": "wiring test",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "body: %s", resp.Body.String())
	var created externalauth.ExternalAuthRequest
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	assert.True(t, strings.HasPrefix(created.RequestID, "xreq-"), "request id must be xreq-prefixed, got %q", created.RequestID)
	assert.Equal(t, externalauth.StatePending, created.State)

	// ② GET /pending without a token → 403; with the token → 200.
	noToken := extAuthRequest(t, h, http.MethodGet, "/api/external-auth/pending", "", nil)
	assert.Equal(t, http.StatusForbidden, noToken.Code, "pending must be admin-gated")
	assert.Contains(t, noToken.Body.String(), "admin token required")

	withToken := extAuthRequest(t, h, http.MethodGet, "/api/external-auth/pending", "the-real-token", nil)
	require.Equal(t, http.StatusOK, withToken.Code)
	var pending struct {
		Requests []externalauth.ExternalAuthRequest `json:"requests"`
		Count    int                                `json:"count"`
	}
	require.NoError(t, json.Unmarshal(withToken.Body.Bytes(), &pending))
	assert.Equal(t, 1, pending.Count, "exactly one pending request")

	// ③ Approve with a WRONG token → 403 (and it must stay pending).
	wrongToken := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/approve", "wrong-token", map[string]any{
		"request_id":      created.RequestID,
		"data_officer_id": "officer-1",
		"business_reason": "should be rejected",
	})
	assert.Equal(t, http.StatusForbidden, wrongToken.Code)

	// ④ Approve with the correct token → 200 + approved audit + xtok- id.
	approve := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/approve", "the-real-token", map[string]any{
		"request_id":      created.RequestID,
		"data_officer_id": "officer-1",
		"business_reason": "verified need",
	})
	require.Equal(t, http.StatusOK, approve.Code, "body: %s", approve.Body.String())
	var approved struct {
		Audit externalauth.AuditRow `json:"audit"`
		Token externalauth.TTLToken `json:"token"`
	}
	require.NoError(t, json.Unmarshal(approve.Body.Bytes(), &approved))
	assert.Equal(t, externalauth.StateApproved, approved.Audit.Decision)
	assert.True(t, strings.HasPrefix(approved.Token.TokenID, "xtok-"), "token id must be xtok-prefixed, got %q", approved.Token.TokenID)
	assert.Equal(t, approved.Audit.TTLTokenID, approved.Token.TokenID)

	// ⑤ Reconcile over a wide window → 200 (the approve row pairs with
	// nothing external yet; the shape is what matters here).
	from := time.Now().Add(-time.Hour).UnixMilli()
	to := time.Now().Add(time.Hour).UnixMilli()
	reconcile := extAuthRequest(t, h, http.MethodGet,
		fmt.Sprintf("/api/external-auth/audit/reconcile?from=%d&to=%d", from, to), "the-real-token", nil)
	require.Equal(t, http.StatusOK, reconcile.Code, "body: %s", reconcile.Body.String())

	returnedToken := approved.Token
	returnedID := created.RequestID

	// ⑥ Execute with the minted token (admin-gated surface) → 200 +
	// masked rows (see TestExternalAuth_EndToEndMasking for the mask
	// assertion detail — kept here for the status-code matrix).
	execute := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/execute", "the-real-token", map[string]any{
		"token_id": returnedToken.TokenID,
	})
	require.Equal(t, http.StatusOK, execute.Code, "body: %s", execute.Body.String())
	assert.NotEmpty(t, returnedID)
}

// TestExternalAuth_DegradedPath — a nil manager answers 503 on ALL six
// routes, even with a CORRECT admin token. The admin check passing must
// not make a disabled channel look like anything other than 503.
func TestExternalAuth_DegradedPath(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
	deps := &Deps{} // ExternalAuthManager nil — gate unset / disabled
	h := newExternalAuthServer(t, deps)

	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/external-auth/request", map[string]any{"view_name": "v_user_consumption"}},
		{http.MethodGet, "/api/external-auth/pending", nil},
		{http.MethodPost, "/api/external-auth/approve", map[string]any{"request_id": "xreq-00000001"}},
		{http.MethodPost, "/api/external-auth/reject", map[string]any{"request_id": "xreq-00000001"}},
		{http.MethodPost, "/api/external-auth/execute", map[string]any{"token_id": "xtok-00000001"}},
		{http.MethodGet, "/api/external-auth/audit/reconcile?from=1&to=2", nil},
	}
	for _, r := range routes {
		resp := extAuthRequest(t, h, r.method, r.path, "the-real-token", r.body)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code,
			"%s %s must answer 503 when the channel is not wired; body: %s", r.method, r.path, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "external auth not configured")
	}
}

// TestExternalAuth_AdminGateOrderedBeforeDegraded — on the gated
// surfaces a missing token answers 403 (NOT 503) even when the channel
// is not wired: the admin check runs first so a 403 never leaks channel
// configuration state.
func TestExternalAuth_AdminGateOrderedBeforeDegraded(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
	deps := &Deps{} // manager nil
	h := newExternalAuthServer(t, deps)

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/external-auth/pending"},
		{http.MethodPost, "/api/external-auth/approve"},
		{http.MethodPost, "/api/external-auth/reject"},
		{http.MethodPost, "/api/external-auth/execute"},
		{http.MethodGet, "/api/external-auth/audit/reconcile?from=1&to=2"},
	} {
		resp := extAuthRequest(t, h, r.method, r.path, "", nil)
		assert.Equal(t, http.StatusForbidden, resp.Code,
			"%s %s must 403 before the 503 can leak channel state", r.method, r.path)
	}
}

// TestExternalAuth_EndToEndMasking — request → approve → execute over
// the real route table returns rows masked by the view's field masks:
// the account value must keep no raw digits beyond its last 4.
func TestExternalAuth_EndToEndMasking(t *testing.T) {
	t.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")
	deps := &Deps{ExternalAuthManager: newExternalAuthManager(t)}
	h := newExternalAuthServer(t, deps)

	created := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/request", "", map[string]any{
		"user_id":         "sess-mask-1",
		"skill_id":        "external-lookup",
		"session_id":      "sess-mask-1",
		"sql_preview":     "SELECT user_id, amount, account_no, phone, email FROM v_user_consumption",
		"view_name":       "v_user_consumption",
		"estimated_rows":  2,
		"business_reason": "e2e masking",
	})
	require.Equal(t, http.StatusCreated, created.Code, "body: %s", created.Body.String())
	var req externalauth.ExternalAuthRequest
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &req))

	approved := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/approve", "the-real-token", map[string]any{
		"request_id":      req.RequestID,
		"data_officer_id": "officer-9",
		"business_reason": "e2e masking",
	})
	require.Equal(t, http.StatusOK, approved.Code)
	var res struct {
		Audit externalauth.AuditRow `json:"audit"`
		Token externalauth.TTLToken `json:"token"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &res))
	require.NotEmpty(t, res.Token.TokenID)

	// DATA-14: the approval audit row carries the masked-field list.
	assert.Contains(t, res.Audit.MaskApplied, "account_no")
	assert.Contains(t, res.Audit.MaskApplied, "phone")

	execute := extAuthRequest(t, h, http.MethodPost, "/api/external-auth/execute", "the-real-token", map[string]any{
		"token_id": res.Token.TokenID,
	})
	require.Equal(t, http.StatusOK, execute.Code, "body: %s", execute.Body.String())
	var rows struct {
		Rows  []map[string]string `json:"rows"`
		Count int                 `json:"count"`
	}
	require.NoError(t, json.Unmarshal(execute.Body.Bytes(), &rows))
	require.Equal(t, 2, rows.Count)
	require.NotEmpty(t, rows.Rows)

	// The masked account: everything before the last 4 chars must be
	// '*' — zero raw digits beyond the visible last 4.
	acct := rows.Rows[0]["account_no"]
	require.Len(t, acct, 16, "masked account must keep the original length: %q", acct)
	assert.Equal(t, "9001", acct[len(acct)-4:], "the last 4 digits stay visible")
	assert.True(t, strings.Trim(acct[:len(acct)-4], "*") == "",
		"no raw digits beyond the last 4 (masked %q, seeded 6225880137799001)", acct)

	// Phone masked the same way; email keeps only the first 2 local chars.
	assert.True(t, strings.Trim(rows.Rows[0]["phone"][:len(rows.Rows[0]["phone"])-4], "*") == "",
		"phone must be masked beyond the last 4: %q", rows.Rows[0]["phone"])
	assert.True(t, strings.HasPrefix(rows.Rows[0]["email"], "al***@example.com"),
		"email must keep only the first 2 local chars + domain: %q", rows.Rows[0]["email"])
}
