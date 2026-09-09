// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// external_auth.go — Phase 21 (v1.1.1 wiring) External Authorized
// Channel HTTP surface.
//
// Surface map — who calls what, and who is gated:
//
//   - POST /api/external-auth/request — the AGENT's path. It is
//     intentionally NOT admin-token-gated: the agent reaches the
//     manager in-process (needs_external_query calls
//     Manager.RequestApproval directly), and the HTTP route exists as
//     the observable / audit surface for that same call. It is bounded
//     by the DATA-10 view allowlist (an unknown view is rejected before
//     anything persists) and writes only a pending audit row — no data
//     leaves the warehouse through it. Unauthenticated request-flood
//     hardening (shared secret + rate limit) is queued for v1.2 with
//     real auth (threat register T-W62-06); abuse is operator-visible
//     as dual-audit rows via reconcile.
//
//   - GET /api/external-auth/pending, POST approve, POST reject,
//     POST execute, GET audit/reconcile — operator / data-officer
//     surfaces. Each requires the X-Admin-Token gate
//     (isAdminRequest: SKILLFORGE_ADMIN_TOKEN + constant-time compare)
//     and answers 403 without it.
//
// CHECK ORDER invariant (locked by the tests): every gated handler
// runs the admin check FIRST and only then the nil-manager 503 check —
// a 403 must never leak whether the channel is configured.
package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/externalauth"
)

// externalAuthRequestBody is the wire shape of
// POST /api/external-auth/request. The agent fills business_reason +
// the NL2SQL preview; the handler validates + persists.
type externalAuthRequestBody struct {
	UserID         string `json:"user_id"`
	SkillID        string `json:"skill_id"`
	SessionID      string `json:"session_id"`
	SQLPreview     string `json:"sql_preview"`
	ViewName       string `json:"view_name"`
	EstimatedRows  int    `json:"estimated_rows"`
	BusinessReason string `json:"business_reason"`
}

// externalAuthResolveBody is the wire shape of the approve/reject
// actions (POST /api/external-auth/approve + /reject).
type externalAuthResolveBody struct {
	RequestID        string   `json:"request_id"`
	DataOfficerID    string   `json:"data_officer_id"`
	BusinessReason   string   `json:"business_reason"`
	ChunkIDsReturned []string `json:"chunk_ids_returned,omitempty"`
}

// postExternalAuthRequest: agent creates a pending approval
// (DATA-11 NL2SQL preview + DATA-10 view_name). Returns the
// canonical request (with RequestID minted) so the agent can echo
// it back through the streaming flow.
func postExternalAuthRequest(deps *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if deps == nil || mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		var body externalAuthRequestBody
		if err := c.Bind(&body); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
			return
		}
		req, err := mgr.RequestApproval(ctx, externalauth.ExternalAuthRequest{
			UserID:    body.UserID,
			SkillID:   body.SkillID,
			SessionID: body.SessionID,
			Preview: externalauth.QueryPreview{
				SQLPreview:     body.SQLPreview,
				ViewName:       body.ViewName,
				EstimatedRows:  body.EstimatedRows,
				BusinessReason: body.BusinessReason,
			},
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}

// getExternalAuthPending: data officer dashboard (DATA-13). FIFO
// list of pending requests. Admin-gated.
func getExternalAuthPending(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !requireAdmin(c) {
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		limit := 100
		if s := c.Query("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err == nil && n > 0 {
				limit = n
			}
		}
		reqs, err := mgr.Store().ListPendingExternalAuthRequests(ctx, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]any{"requests": reqs, "count": len(reqs)})
	}
}

// postExternalAuthApprove + postExternalAuthReject: DATA-13
// human resolution paths. Both admin-gated.
func postExternalAuthApprove(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !requireAdmin(c) {
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		var body externalAuthResolveBody
		if err := c.Bind(&body); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		_, audit, token, err := mgr.Approve(ctx, body.RequestID, body.DataOfficerID, body.BusinessReason, body.ChunkIDsReturned)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]any{
			"audit": audit,
			"token": token,
		})
	}
}

func postExternalAuthReject(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !requireAdmin(c) {
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		var body externalAuthResolveBody
		if err := c.Bind(&body); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		_, audit, err := mgr.Reject(ctx, body.RequestID, body.DataOfficerID, body.BusinessReason)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]any{"audit": audit})
	}
}

// postExternalAuthExecute: agent's post-approval query. Uses the
// minted TTL token to call Source.Execute; rows come back masked.
// Admin-gated — executing against the warehouse is an operator /
// data-officer-auditable surface, not an open one.
func postExternalAuthExecute(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !requireAdmin(c) {
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		var body struct {
			TokenID string `json:"token_id"`
		}
		if err := c.Bind(&body); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		rows, err := mgr.ExecuteWithToken(ctx, body.TokenID)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]any{
			"rows":  rows,
			"count": len(rows),
		})
	}
}

// getExternalAuthAuditReconcile: DATA-06 dual-audit pair
// mismatch surfacing. InternalOnly / ExternalOnly rows are
// security events. Admin-gated.
func getExternalAuthAuditReconcile(_ *Deps, mgr *externalauth.Manager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// CHECK ORDER: admin check FIRST — a 403 must not leak whether
		// the channel is configured.
		if !requireAdmin(c) {
			return
		}
		if mgr == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external auth not configured"})
			return
		}
		from, err := strconv.ParseInt(c.Query("from"), 10, 64)
		if err != nil || from <= 0 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "from (epoch ms) required"})
			return
		}
		to, err := strconv.ParseInt(c.Query("to"), 10, 64)
		if err != nil || to <= 0 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "to (epoch ms) required"})
			return
		}
		report, err := mgr.Reconcile(ctx, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, report)
	}
}
