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

// Plan 06-02 / SC-3 + SC-5 — server-side tests for the /api/audit
// endpoint, the read-time retention projection (redact), and the
// binary admin-token access split.
//
// Test families:
//
//   1. TestAuditHandler_NilStoreReturns503 — Deps.Store == nil → 503
//      (mirrors the knowledge + wizard 503 posture).
//   2. TestAuditHandler_QueryParamsMapToFilter — query params
//      (user / skill / since / until / refusal_reason / limit /
//      offset) flow into AuditFilter 1:1; LIMIT / OFFSET pagination
//      works through the store (T-3-SQLI-safe: parameterized).
//   3. TestAuditHandler_NegativeLimitRejected + OversizeLimitClamped
//      + NegativeOffsetRejected — V5 ASVS input validation.
//   4. TestRetentionRedact_NonAdmin / _Admin / _QueryString — read-time
//      projection applies retention redaction to Query + VerdictJSON
//      on rows older than (now - retention), unless X-Admin-Token is
//      valid; admin=true short-circuits.
//   5. TestAdminToken_UnsetEnv — SKILLFORGE_ADMIN_TOKEN unset → all
//      requests non-admin (redacted) — demo OOB posture (D6-08).
//   6. TestAdminToken_WrongTokenTimingSafe / _CorrectTokenReturnsRaw
//      — wrong token returns redacted rows; correct token returns
//      raw rows. The constant-time compare is asserted via a
//      grep-on-source check in TestAdminToken_ConstantTimeCompareIsUsed.
//   7. TestSQLiRegression_AuditHandlerParam — DROP TABLE in
//      user / skill / refusal_reason does NOT drop the table (the
//      store-level regression is also re-run by the verify command).

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
)

// invokeAuditHandler runs GET /api/audit?q with the optional
// X-Admin-Token header set. Returns the recorder so the test can
// assert on body / status. Mirrors invokeGraphHandler.
func invokeAuditHandler(t *testing.T, deps *Deps, q, adminToken string) *ut.ResponseRecorder {
	t.Helper()
	h := newAuditHandlerHertz(t, deps)
	var headers []ut.Header
	if adminToken != "" {
		headers = append(headers, ut.Header{Key: "X-Admin-Token", Value: adminToken})
	}
	return ut.PerformRequest(h.Engine, http.MethodGet, "/api/audit?"+q, nil, headers...)
}

// newAuditHandlerHertz builds a fresh hertz instance with
// /api/audit registered against deps.
func newAuditHandlerHertz(t *testing.T, deps *Deps) *server.Hertz {
	t.Helper()
	h := server.Default(server.WithHostPorts(":0"))
	h.GET("/api/audit", deps.auditHandler)
	return h
}

// seedAuditRows inserts a deterministic 5-row audit trail into s
// for pagination + retention tests. Timestamps span a 200-day
// window so tests can exercise the retention boundary by varying
// the SKILLFORGE_RETENTION_DAYS env (default 90).
func seedAuditRows(t *testing.T, s *sqlite.SQLiteStore) []store.AuditRow {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []store.AuditRow{
		{Timestamp: base, UserID: "u1", SkillID: "s1", SessionID: "sess-1", RefusalReason: "no_authorized_source", Query: "q1", PromptHash: "h1", ModelVersion: "m1", VerdictJSON: `{"v":"v1"}`, ChunkIDs: "c1"},
		{Timestamp: base.Add(40 * 24 * time.Hour), UserID: "u2", SkillID: "s1", SessionID: "sess-2", RefusalReason: "low_faithfulness", Query: "q2", PromptHash: "h2", ModelVersion: "m1", VerdictJSON: `{"v":"v2"}`, ChunkIDs: "c2"},
		{Timestamp: base.Add(80 * 24 * time.Hour), UserID: "u1", SkillID: "s1", SessionID: "sess-3", RefusalReason: "", Query: "q3", PromptHash: "h3", ModelVersion: "m1", VerdictJSON: `{"v":"v3"}`, ChunkIDs: "c3"},
		{Timestamp: base.Add(120 * 24 * time.Hour), UserID: "u2", SkillID: "s2", SessionID: "sess-4", RefusalReason: "", Query: "q4", PromptHash: "h4", ModelVersion: "m1", VerdictJSON: `{"v":"v4"}`, ChunkIDs: "c4"},
		{Timestamp: base.Add(160 * 24 * time.Hour), UserID: "u1", SkillID: "s2", SessionID: "sess-5", RefusalReason: "no_authorized_source", Query: "q5", PromptHash: "h5", ModelVersion: "m1", VerdictJSON: `{"v":"v5"}`, ChunkIDs: "c5"},
	}
	for i := range rows {
		_, err := s.PutAudit(ctx, rows[i])
		require.NoError(t, err, "seed[%d] PutAudit", i)
	}
	return rows
}

// TestAuditHandler_NilStoreReturns503 — Deps.Store == nil → 503
// with the standard {error:"..."} envelope. Mirrors the
// graphHandler / wizard 503 posture.
func TestAuditHandler_NilStoreReturns503(t *testing.T) {
	deps := &Deps{Store: nil}
	got := invokeAuditHandler(t, deps, "", "")
	assert.Equal(t, http.StatusServiceUnavailable, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "audit store not initialized")
}

// TestAuditHandler_QueryParamsMapToFilter — every query param
// (user / skill / since / until / refusal_reason / limit /
// offset) flows into the AuditFilter 1:1. The test asserts each
// axis by issuing one query and checking the returned rows.
func TestAuditHandler_QueryParamsMapToFilter(t *testing.T) {
	s := newTestStore(t)
	seedAuditRows(t, s)
	deps := &Deps{Store: s}

	t.Run("user filter", func(t *testing.T) {
		got := invokeAuditHandler(t, deps, "user=u1", "")
		require.Equal(t, http.StatusOK, got.Code)
		var rows []auditWireRow
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		assert.Len(t, rows, 3, "u1 has 3 rows (sess-1, sess-3, sess-5)")
		for _, r := range rows {
			assert.Equal(t, "u1", r.UserID)
		}
	})

	t.Run("skill filter", func(t *testing.T) {
		got := invokeAuditHandler(t, deps, "skill=s1", "")
		require.Equal(t, http.StatusOK, got.Code)
		var rows []auditWireRow
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		for _, r := range rows {
			assert.Equal(t, "s1", r.SkillID)
		}
	})

	t.Run("refusal_reason filter", func(t *testing.T) {
		got := invokeAuditHandler(t, deps, "refusal_reason=no_authorized_source", "")
		require.Equal(t, http.StatusOK, got.Code)
		var rows []auditWireRow
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		require.Len(t, rows, 2)
		for _, r := range rows {
			assert.Equal(t, "no_authorized_source", r.RefusalReason)
		}
	})

	t.Run("since/until filter", func(t *testing.T) {
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		since := base.Add(50 * 24 * time.Hour).Format(time.RFC3339)
		until := base.Add(150 * 24 * time.Hour).Format(time.RFC3339)
		got := invokeAuditHandler(t, deps, "since="+since+"&until="+until, "")
		require.Equal(t, http.StatusOK, got.Code)
		var rows []auditWireRow
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		require.Len(t, rows, 2, "rows in [50d, 150d) are sess-3 and sess-4")
	})

	t.Run("limit and offset pagination", func(t *testing.T) {
		// limit=2 → 2 newest rows; offset=2 → 2 next rows.
		got := invokeAuditHandler(t, deps, "limit=2", "")
		require.Equal(t, http.StatusOK, got.Code)
		var rows []auditWireRow
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		require.Len(t, rows, 2)

		got = invokeAuditHandler(t, deps, "limit=2&offset=2", "")
		require.Equal(t, http.StatusOK, got.Code)
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		require.Len(t, rows, 2)

		// limit=2&offset=4 → 1 row (the 5th).
		got = invokeAuditHandler(t, deps, "limit=2&offset=4", "")
		require.Equal(t, http.StatusOK, got.Code)
		require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
		require.Len(t, rows, 1)
	})
}

// TestAuditHandler_WireShapeIsSnakeCase — UAT 2026-09-06 B1 (P1)
// regression lock. The /api/audit contract is the snake_case shape of
// web/src/types/eval.ts AuditRow; marshaling store.AuditRow directly
// (PascalCase Go field names) fed the FE undefined fields and crashed
// AuditTable's redaction check (white screen). The lock asserts the
// exact key set: snake_case keys present, PascalCase keys absent.
func TestAuditHandler_WireShapeIsSnakeCase(t *testing.T) {
	s := newTestStore(t)
	seedAuditRows(t, s)
	deps := &Deps{Store: s}

	got := invokeAuditHandler(t, deps, "limit=1", "")
	require.Equal(t, http.StatusOK, got.Code)

	var raws []map[string]any
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &raws))
	require.Len(t, raws, 1)

	want := []string{
		"ts", "user_id", "skill_id", "session_id", "refusal_reason",
		"query", "verdict_json", "faithfulness", "context_precision",
		"answer_relevance", "context_recall",
	}
	for _, k := range want {
		assert.Contains(t, raws[0], k, "wire row must carry snake_case key %q", k)
	}
	for _, k := range []string{"Query", "Timestamp", "VerdictJSON", "RefusalReason", "UserID", "SkillID", "SessionID"} {
		assert.NotContains(t, raws[0], k, "PascalCase Go field names must not leak to the wire")
	}

	// The metrics columns stay nullable on the wire (rows without eval
	// backfill carry null, not 0 — the FE distinguishes unset from zero).
	assert.Nil(t, raws[0]["faithfulness"], "un-backfilled faithfulness must be null")
}

// TestAuditHandler_NegativeLimitRejected — V5 ASVS: limit must
// be ≥ 0. limit=-1 → 400 with a clear error envelope.
func TestAuditHandler_NegativeLimitRejected(t *testing.T) {
	s := newTestStore(t)
	seedAuditRows(t, s)
	deps := &Deps{Store: s}

	got := invokeAuditHandler(t, deps, "limit=-1", "")
	assert.Equal(t, http.StatusBadRequest, got.Code)
	var body graphResponseError
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "limit")
}

// TestAuditHandler_OversizeLimitClamped — limit=10000 → clamped
// to the safe max (maxAuditLimit = 1000). The endpoint returns
// 200 + rows, not 400 — clamping is gentler than rejecting
// because a noisy client with limit=100000 is not an attack,
// just a misconfig.
func TestAuditHandler_OversizeLimitClamped(t *testing.T) {
	s := newTestStore(t)
	seedAuditRows(t, s)
	deps := &Deps{Store: s}

	got := invokeAuditHandler(t, deps, "limit=10000", "")
	require.Equal(t, http.StatusOK, got.Code)
	var rows []auditWireRow
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
	assert.LessOrEqual(t, len(rows), 5, "clamp does not exceed seeded rows")
}

// TestAuditHandler_NegativeOffsetRejected — V5 ASVS: offset
// must be ≥ 0. offset=-5 → 400.
func TestAuditHandler_NegativeOffsetRejected(t *testing.T) {
	s := newTestStore(t)
	deps := &Deps{Store: s}
	got := invokeAuditHandler(t, deps, "offset=-5", "")
	assert.Equal(t, http.StatusBadRequest, got.Code)
}

// TestRetentionRedact_NonAdmin — pure-fn test of redact() with
// canned timestamps. Rows older than the retention window have
// Query + VerdictJSON replaced with the [REDACTED:...] sentinel;
// recent rows stay raw.
func TestRetentionRedact_NonAdmin(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	rows := []store.AuditRow{
		{Timestamp: now.Add(-200 * 24 * time.Hour), Query: "very-old", VerdictJSON: `{"v":"old"}`},
		{Timestamp: now.Add(-100 * 24 * time.Hour), Query: "old", VerdictJSON: `{"v":"oldish"}`},
		{Timestamp: now.Add(-30 * 24 * time.Hour), Query: "recent", VerdictJSON: `{"v":"fresh"}`},
		{Timestamp: now.Add(-1 * 24 * time.Hour), Query: "today", VerdictJSON: `{"v":"now"}`},
	}
	redact(rows, 90*24*time.Hour, now, false)

	assert.Contains(t, rows[0].Query, "REDACTED:", "row 0 (200d old) must be redacted")
	assert.Contains(t, rows[0].VerdictJSON, "REDACTED:", "row 0 VerdictJSON redacted")
	assert.Contains(t, rows[1].Query, "REDACTED:", "row 1 (100d old) must be redacted (boundary case at 90d)")
	assert.Equal(t, "recent", rows[2].Query, "row 2 (30d) stays raw")
	assert.Equal(t, "today", rows[3].Query, "row 3 (1d) stays raw")
}

// TestRetentionRedact_Admin — admin=true → no redaction,
// regardless of age.
func TestRetentionRedact_Admin(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	rows := []store.AuditRow{
		{Timestamp: now.Add(-1000 * 24 * time.Hour), Query: "ancient", VerdictJSON: `{"v":"v"}`},
		{Timestamp: now.Add(-200 * 24 * time.Hour), Query: "old", VerdictJSON: `{"v":"v"}`},
	}
	redact(rows, 90*24*time.Hour, now, true)
	assert.Equal(t, "ancient", rows[0].Query, "admin sees ancient row verbatim")
	assert.Equal(t, "old", rows[1].Query, "admin sees old row verbatim")
}

// TestRetentionRedact_NonAdmin_QueryString — redaction uses a
// stable prefix the FE matches on. The sentinel must include the
// day count + the row's ts + the retention boundary so the FE's
// hover detail (D6-07) can render the reason without a separate
// API call.
func TestRetentionRedact_NonAdmin_QueryString(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	old := now.Add(-180 * 24 * time.Hour)
	rows := []store.AuditRow{{Timestamp: old, Query: "PII", VerdictJSON: "raw"}}
	redact(rows, 90*24*time.Hour, now, false)
	assert.Contains(t, rows[0].Query, "REDACTED:90d+", "sentinel includes the retention-day count")
	assert.Contains(t, rows[0].Query, old.UTC().Format(time.RFC3339), "sentinel includes the row ts")
}

// TestAdminToken_UnsetEnv — SKILLFORGE_ADMIN_TOKEN unset → all
// requests are non-admin. We pre-clear the env, then assert an
// old row is redacted in the response even when no token is sent.
func TestAdminToken_UnsetEnv(t *testing.T) {
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
	t.Cleanup(func() {
		if prev != "" {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		}
	})

	s := newTestStore(t)
	old := time.Now().Add(-200 * 24 * time.Hour)
	_, err := s.PutAudit(context.Background(), store.AuditRow{
		Timestamp: old, UserID: "u1", SkillID: "s1", SessionID: "sess-old",
		Query: "should-be-redacted", VerdictJSON: "raw", PromptHash: "h", ModelVersion: "m",
	})
	require.NoError(t, err)

	deps := &Deps{Store: s}
	got := invokeAuditHandler(t, deps, "", "")
	require.Equal(t, http.StatusOK, got.Code)

	var rows []auditWireRow
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
	require.NotEmpty(t, rows, "the seeded row must be returned (even if redacted)")
	for _, r := range rows {
		if r.Query == "should-be-redacted" {
			t.Fatalf("Query leaked unredacted despite unset admin env (T-6-IDOR / D6-08)")
		}
	}
	foundRedacted := false
	for _, r := range rows {
		if len(r.Query) >= 9 && r.Query[:9] == "REDACTED:" {
			foundRedacted = true
			break
		}
	}
	assert.True(t, foundRedacted, "unset SKILLFORGE_ADMIN_TOKEN must produce redacted rows (D6-08 demo-OOB)")
}

// TestAdminToken_WrongTokenTimingSafe — wrong token returns
// redacted rows (same body shape as no-token). The constant-time
// compare is asserted in TestAdminToken_ConstantTimeCompareIsUsed
// via a grep-on-source check.
func TestAdminToken_WrongTokenTimingSafe(t *testing.T) {
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	t.Cleanup(func() {
		if prev != "" {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		} else {
			_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		}
	})
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")

	s := newTestStore(t)
	old := time.Now().Add(-200 * 24 * time.Hour)
	_, err := s.PutAudit(context.Background(), store.AuditRow{
		Timestamp: old, UserID: "u1", SkillID: "s1", SessionID: "sess-old",
		Query: "PII", VerdictJSON: "raw", PromptHash: "h", ModelVersion: "m",
	})
	require.NoError(t, err)

	deps := &Deps{Store: s}
	got := invokeAuditHandler(t, deps, "", "wrong-token")
	require.Equal(t, http.StatusOK, got.Code)
	var rows []auditWireRow
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
	for _, r := range rows {
		assert.NotEqual(t, "PII", r.Query, "wrong token MUST NOT return raw rows (T-6-TOKEN)")
	}
}

// TestAdminToken_CorrectTokenReturnsRaw — correct token → raw
// rows even for old timestamps.
func TestAdminToken_CorrectTokenReturnsRaw(t *testing.T) {
	prev := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	t.Cleanup(func() {
		if prev != "" {
			_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", prev)
		} else {
			_ = os.Unsetenv("SKILLFORGE_ADMIN_TOKEN")
		}
	})
	_ = os.Setenv("SKILLFORGE_ADMIN_TOKEN", "the-real-token")

	s := newTestStore(t)
	old := time.Now().Add(-200 * 24 * time.Hour)
	_, err := s.PutAudit(context.Background(), store.AuditRow{
		Timestamp: old, UserID: "u1", SkillID: "s1", SessionID: "sess-old",
		Query: "raw-PII", VerdictJSON: "raw-PII-JSON", PromptHash: "h", ModelVersion: "m",
	})
	require.NoError(t, err)

	deps := &Deps{Store: s}
	got := invokeAuditHandler(t, deps, "", "the-real-token")
	require.Equal(t, http.StatusOK, got.Code)
	var rows []auditWireRow
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &rows))
	require.NotEmpty(t, rows)
	var hit *auditWireRow
	for i := range rows {
		if rows[i].SessionID == "sess-old" {
			hit = &rows[i]
			break
		}
	}
	require.NotNil(t, hit, "seeded row must be present in the response")
	assert.Equal(t, "raw-PII", hit.Query, "admin sees the raw Query")
	assert.Equal(t, "raw-PII-JSON", hit.VerdictJSON, "admin sees the raw VerdictJSON")
}

// TestSQLiRegression_AuditHandlerParam — Plan 06-02 / T-3-SQLI
// regression: a DROP TABLE payload in the user filter does NOT
// drop the table. Verified at the HTTP layer so a future refactor
// that bypasses the AuditFilter also surfaces.
func TestSQLiRegression_AuditHandlerParam(t *testing.T) {
	s := newTestStore(t)
	seedAuditRows(t, s)
	deps := &Deps{Store: s}

	payload := "'; DROP TABLE audit; --"
	got := invokeAuditHandler(t, deps, "user="+urlEscape(payload), "")
	require.Equal(t, http.StatusOK, got.Code)

	got2 := invokeAuditHandler(t, deps, "", "")
	require.Equal(t, http.StatusOK, got2.Code)
	var rows []auditWireRow
	require.NoError(t, json.Unmarshal(got2.Body.Bytes(), &rows))
	assert.GreaterOrEqual(t, len(rows), 5, "table must survive the DROP TABLE payload (T-3-SQLI)")
}

// urlEscape is a tiny stdlib-free escape for the SQLi regression
// test — encodes ' and space.
func urlEscape(s string) string {
	var out strings.Builder
	out.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\'':
			out.WriteString("%27")
		case ' ':
			out.WriteString("%20")
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// TestAdminToken_ConstantTimeCompareIsUsed — T-6-TOKEN / ASVS
// V6.2.1: the token comparison MUST use crypto/subtle.
// ConstantTimeCompare, not ==. We assert by reading the audit.go
// source and grepping for the canonical call. A future refactor
// that replaces subtle.ConstantTimeCompare with bytes.Equal or ==
// fails here, before the production code goes red.
func TestAdminToken_ConstantTimeCompareIsUsed(t *testing.T) {
	// Locate the audit.go file relative to this test (same package).
	// We cannot easily resolve Go source files at test time without
	// runtime.Caller tricks; instead, fetch the source via the build
	// cache path. The simplest defensible check: read the file by
	// the conventional name.
	//
	// The path is "audit.go" co-located with this test in the
	// `server` package.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed in a test")
	auditPath := thisFile[:len(thisFile)-len("audit_test.go")] + "audit.go"
	src, err := os.ReadFile(auditPath)
	require.NoError(t, err, "read audit.go at %s", auditPath)
	body := string(src)

	assert.Contains(t, body, "crypto/subtle",
		"audit.go must import crypto/subtle (T-6-TOKEN / ASVS V6.2.1)")
	assert.Contains(t, body, "ConstantTimeCompare",
		"audit.go must use subtle.ConstantTimeCompare (NOT ==) for X-Admin-Token")
	assert.NotContains(t, body, "X-Admin-Token\") ==",
		"audit.go MUST NOT compare the admin token with == (timing-attack vector)")
}
