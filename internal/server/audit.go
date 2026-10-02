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

// Plan 06-02 / SC-3 + SC-5 — the GET /api/audit HTTP surface.
//
// Wire contract:
//
//	GET /api/audit?user=&skill=&since=&until=&refusal_reason=&limit=&offset=
//	    → 200 {rows: [...AuditRow]}     success (retention-projected)
//	    → 400 {error: "..."}             bad limit/offset (negative / non-int)
//	    → 500 {error: "..."}             QueryAudit failed (W-3, generic message)
//	    → 503 {error: "..."}             Deps.Store == nil (mirror of wizard / graph)
//
// Threat model (06-02 threat_model):
//
//   - T-3-SQLI  — query params are mapped 1:1 to store.AuditFilter;
//                 QueryAudit builds the SQL with `?` placeholders only.
//                 The DROP TABLE regression in store_test.go must stay
//                 green (and is re-asserted by audit_test.go).
//
//   - T-6-IDOR  — /api/audit exposes ALL users' queries. Raw access
//                 requires a valid X-Admin-Token; without one, the
//                 retention redaction replaces Query + VerdictJSON on
//                 old rows. v2 SCALE-* owns real RBAC.
//
//   - T-6-TOKEN — the admin token comparison uses
//                 crypto/subtle.ConstantTimeCompare (ASVS V6.2.1).
//                 The token value is NEVER logged.
//
//   - T-3-AUDIT — retention redaction is a READ-TIME projection
//                 between QueryAudit and c.JSON. The store layer
//                 gains NO UPDATE / DELETE path; the immutable audit
//                 row is preserved (Pitfall 4 / T-3-AUDIT).
//
// Why the projection lives here, not in the store: the store's
// contract is "give me the raw rows"; the access policy (admin
// vs non-admin) is an HTTP concern. Putting it in the handler
// keeps the policy testable in pure Go (TestRetentionRedact_*)
// and prevents a future "give me the redacted rows" feature
// from accidentally bypassing it.

package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

const (
	// defaultAuditLimit is the AuditFilter.Limit default (mirrors the
	// store-side default so the handler + the store agree on 100).
	defaultAuditLimit = 100
	// maxAuditLimit caps an explicit limit to a safe upper bound —
	// anything larger is silently clamped (V5 ASVS input validation;
	// not an error, just a noisy client).
	maxAuditLimit = 1000
	// defaultRetentionDays is the SKILLFORGE_RETENTION_DAYS fallback
	// when the env is unset (D6-06).
	defaultRetentionDays = 90
)

// auditRetention reads SKILLFORGE_RETENTION_DAYS (default 90). The
// env is read at request time, not at construction, so an operator
// can change the policy without restarting the server. Returns a
// duration (the store works in rows, the handler in time).
func auditRetention() time.Duration {
	v := os.Getenv("SKILLFORGE_RETENTION_DAYS")
	if v == "" {
		return defaultRetentionDays * 24 * time.Hour
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultRetentionDays * 24 * time.Hour
	}
	return time.Duration(n) * 24 * time.Hour
}

// isAdminRequest returns true iff the request carries a valid
// X-Admin-Token header AND SKILLFORGE_ADMIN_TOKEN is set in the
// environment. Unset env → isAdmin=false for ALL requests
// (demo-OOB posture, D6-08); the operator must explicitly opt in
// to raw-row access.
//
// ASVS V6.2.1 — the token compare uses crypto/subtle.ConstantTimeCompare
// to neutralize timing attacks. The token value is never logged
// (only the boolean result, in TestAdminToken_*) — see T-6-TOKEN.
//
// Hertz exposes headers through the request context as a low-level
// iteration API (no direct map access). The handler is hot path
// (one request = one header walk), so the O(n) cost is fine.
func isAdminRequest(c *app.RequestContext) bool {
	expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
	if expected == "" {
		return false
	}
	var got string
	c.Request.Header.VisitAll(func(k, v []byte) {
		if string(k) == "X-Admin-Token" {
			got = string(v)
		}
	})
	// ConstantTimeCompare returns 1 iff the two byte slices are
	// equal in length AND content. The token is short-lived per
	// request, so the time it takes is bounded by len(expected).
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// requireAdmin checks the X-Admin-Token header against
// SKILLFORGE_ADMIN_TOKEN. On mismatch it writes 403 with the standard
// graphResponseError envelope and returns false; the caller should
//   if !requireAdmin(c) { return }
// Phase 35 (REPEATHTTP-02): replaces the 7+ repeated
//   if !isAdminRequest(c) {
//       c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
//       return
//   }
// template at every admin-gated handler entry. The 403 MUST NOT
// leak KB registry state or any store state per D6-03 / T-3-AUDIT.
func requireAdmin(c *app.RequestContext) bool {
	if isAdminRequest(c) {
		return true
	}
	c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
	return false
}

// redact applies the SC-5 retention projection to audit rows IN
// PLACE. Rows older than (now - retention) have Query + VerdictJSON
// replaced with a "[REDACTED:...]" sentinel that includes the day
// count, the row's timestamp, and the retention boundary — the FE
// reads these three pieces to render the hover detail (D6-07)
// without an extra API call.
//
// The isAdmin=true short-circuit returns immediately: raw-row access
// is the access policy of the X-Admin-Token path, and bypassing the
// projection is the whole point of carrying the token.
//
// INVARIANT (T-3-AUDIT, Pitfall 4): this function mutates a slice
// in place. The store layer has no write surface for retention —
// the row stored on disk is byte-identical to what the chat path
// wrote. A future "soft-delete" or "TTL" path that re-encodes the
// row would destroy the audit-trail property this machinery exists
// to protect.
func redact(rows []store.AuditRow, retention time.Duration, now time.Time, isAdmin bool) {
	if isAdmin {
		return
	}
	boundary := now.Add(-retention)
	days := int(retention.Hours() / 24)
	for i := range rows {
		if rows[i].Timestamp.Before(boundary) {
			reason := fmt.Sprintf("REDACTED:%dd+ (ts=%s, boundary=%s)",
				days,
				rows[i].Timestamp.UTC().Format(time.RFC3339),
				boundary.UTC().Format(time.RFC3339),
			)
			rows[i].Query = reason
			rows[i].VerdictJSON = reason
		}
	}
}

// atoiOrZero parses a query param as a non-negative int. Returns
// (val, ok) where ok=false signals a malformed value the handler
// should surface as 400. Empty string returns (0, true) — the
// filter default.
func atoiOrZero(raw string) (int, bool) {
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

// auditHandler implements GET /api/audit. The handler:
//
//  1. 503 if Deps.Store is nil (mirrors the graph / wizard posture).
//  2. Parse + clamp query params (limit ≥ 0, limit ≤ maxAuditLimit,
//     offset ≥ 0). Negative values → 400.
//  3. Call store.QueryAudit — 100% parameterized (T-3-SQLI).
//  4. Apply the read-time retention redaction (T-3-AUDIT) based on
//     the admin-token policy (T-6-IDOR + T-6-TOKEN).
//  5. 200 with the projected rows.
func (s *Deps) auditHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "audit store not initialized (set Deps.Store at server construction)",
		})
		return
	}

	limit, ok := atoiOrZero(string(c.Query("limit")))
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "limit must be a non-negative integer",
		})
		return
	}
	if limit < 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "limit must be a non-negative integer",
		})
		return
	}
	if limit > maxAuditLimit {
		limit = maxAuditLimit
	}
	if limit == 0 {
		limit = defaultAuditLimit
	}

	offset, ok := atoiOrZero(string(c.Query("offset")))
	if !ok {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "offset must be a non-negative integer",
		})
		return
	}
	if offset < 0 {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "offset must be a non-negative integer",
		})
		return
	}

	since, err := parseAuditTime(string(c.Query("since")))
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "since must be RFC3339: " + err.Error(),
		})
		return
	}
	until, err := parseAuditTime(string(c.Query("until")))
	if err != nil {
		c.JSON(http.StatusBadRequest, graphResponseError{
			Error: "until must be RFC3339: " + err.Error(),
		})
		return
	}

	filter := store.AuditFilter{
		UserID:        string(c.Query("user")),
		SkillID:       string(c.Query("skill")),
		RefusalReason: string(c.Query("refusal_reason")),
		Since:         since,
		Until:         until,
		Limit:         limit,
		Offset:        offset,
	}

	rows, err := s.Store.QueryAudit(ctx, filter)
	if err != nil {
		// W-3 (mirrors graphHandler / chunkHandler): log the full
		// err.Error() server-side at WARN so the operator can
		// diagnose, return a generic envelope to the FE.
		log.Printf("[WARN] server: audit query failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "audit query failed — see server logs",
		})
		return
	}

	// T-3-AUDIT invariant: redaction runs in the HTTP layer, NEVER
	// in the store. The store returned RAW rows; the policy is
	// applied between QueryAudit and c.JSON.
	redact(rows, auditRetention(), time.Now(), isAdminRequest(c))

	// UAT 2026-09-06 B1 (P1): project onto the snake_case wire shape
	// the FE contract documents (web/src/types/eval.ts AuditRow).
	// Marshaling store.AuditRow directly leaked Go field names
	// (PascalCase) — every FE field read came back undefined and
	// AuditTable's redaction check crashed on `.startsWith`,
	// white-screening /eval/audit.
	c.JSON(http.StatusOK, toAuditWireRows(rows))
}

// auditWireRow is the /api/audit wire projection — snake_case JSON keys
// mirroring web/src/types/eval.ts AuditRow (the contract the eval
// dashboard's AuditTable consumes). Deliberately a DISTINCT type from
// store.AuditRow: store structs have no JSON tags by design (they are
// a persistence shape, not a wire shape), and marshaling them directly
// couples the FE to Go field-name capitalization.
type auditWireRow struct {
	Ts               string   `json:"ts"`
	UserID           string   `json:"user_id"`
	SkillID          string   `json:"skill_id"`
	SessionID        string   `json:"session_id"`
	RefusalReason    string   `json:"refusal_reason"`
	Query            string   `json:"query"`
	VerdictJSON      string   `json:"verdict_json"`
	Faithfulness     *float64 `json:"faithfulness"`
	ContextPrecision *float64 `json:"context_precision"`
	AnswerRelevance  *float64 `json:"answer_relevance"`
	ContextRecall    *float64 `json:"context_recall"`
}

// toAuditWireRows projects store rows onto the /api/audit wire shape.
// Ts is RFC3339 UTC (the FE parses it with Date.parse / string compare).
func toAuditWireRows(rows []store.AuditRow) []auditWireRow {
	out := make([]auditWireRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, auditWireRow{
			Ts:               r.Timestamp.UTC().Format(time.RFC3339),
			UserID:           r.UserID,
			SkillID:          r.SkillID,
			SessionID:        r.SessionID,
			RefusalReason:    r.RefusalReason,
			Query:            r.Query,
			VerdictJSON:      r.VerdictJSON,
			Faithfulness:     r.Faithfulness,
			ContextPrecision: r.ContextPrecision,
			AnswerRelevance:  r.AnswerRelevance,
			ContextRecall:    r.ContextRecall,
		})
	}
	return out
}

// parseAuditTime parses an RFC3339 timestamp. Empty string is the
// "no bound" signal — a zero time.Time, which the store layer
// treats as "filter unset". Any non-empty unparseable value → an
// error the handler surfaces as 400.
func parseAuditTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

// writeAuditMeta is the Phase 10 helper that writes the audit_meta
// sibling-table row alongside the canonical audit row. Called from
// writeAuditAndTurn (extended in Plan 02 Task 3) after the audit row
// is INSERTed and the auto-increment ID is known.
//
// Contract (best-effort, matches writeAuditAndTurn's posture):
//
//   - nil s     → no-op (degraded mode: the audit row was already
//                 silently skipped, the meta row follows suit).
//   - nil meta  → no-op (the caller chose not to write a meta row —
//                 legitimate on a first-turn / empty-history path
//                 when no Phase 10 signals exist).
//   - error     → log + swallow (the response has already streamed;
//                 failing the request over a meta-write error is
//                 worse than degraded durability — T-3-AUDIT-WRITE
//                 accept disposition).
//
// The function deliberately does NOT return the error so the call
// site can fire-and-forget without scattering error-handling. The
// audit row carries the query_id the meta row needs; if PutAudit
// failed the call site also swallowed that error, so the meta row
// cannot be written (query_id=0 fails the FK constraint). The caller
// pattern is:
//
//	id, err := s.Store.PutAudit(ctx, row)
//	if err == nil {  // audit row exists
//	    writeAuditMeta(ctx, s.Store, &store.AuditMeta{QueryID: id, ...})
//	}
//
// Returns nothing — observability-only on error.
func writeAuditMeta(ctx context.Context, s store.Store, meta *store.AuditMeta) {
	if s == nil || meta == nil {
		return
	}
	if err := s.PutAuditMeta(ctx, *meta); err != nil {
		log.Printf("[WARN] server: audit_meta write (query_id=%d): %v (response already streamed; non-fatal)", meta.QueryID, err)
	}
}
