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

package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/auditchain"
	"github.com/skillforge/skillforge/internal/store"
)

// sessionChainHandler implements GET /api/session/:id/decision-chain.
//
// Phase 24 Task 24-02 (decision-chain): every audit row for the
// session, classified into a chronological Link list the FE
// AuditTimelineTab can render as one timeline.
//
// Query params:
//   - limit=N (default 1000, max 5000) — caps Link count.
//   - at=RFC3339 — point-in-time bound (Phase 24 Task 24-03 reuse):
//     only rows with ts <= at are included. Empty → every row.
//
// 503 on nil store (same posture as audit_prov); 200 with empty
// array for sessions with no rows (so FE renders "no chain yet").
func sessionChainHandler(deps *Deps) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := c.Param("id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "session id required"})
			return
		}
		if deps == nil || deps.Store == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "audit store not configured"})
			return
		}
		limit := 1000
		if s := c.Query("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid limit"})
				return
			}
			limit = n
			if limit > 5000 {
				limit = 5000
			}
		}
		// at bound: empty → unbounded, otherwise parse as RFC3339.
		tUnixMs := int64(0)
		if s := c.Query("at"); s != "" {
			parsed, err := time.Parse(time.RFC3339, s)
			if err != nil {
				c.JSON(http.StatusBadRequest, map[string]string{"error": "at must be RFC3339"})
				return
			}
			tUnixMs = parsed.UnixMilli()
		}
		// Adapts the store.Store to auditchain.Store. The store's
		// AuditBySessionAt already returns id-ASC + ts-bounded rows;
		// auditchain.ChainForSession / ChainForSessionAt do the rest.
		adapter := &chainStoreAdapter{inner: deps.Store}
		var (
			chain []auditchain.Link
			err   error
		)
		if tUnixMs > 0 {
			chain, err = auditchain.ChainForSessionAt(ctx, adapter, sessionID, time.UnixMilli(tUnixMs))
		} else {
			chain, err = auditchain.ChainForSession(ctx, adapter, sessionID, limit)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "decision-chain: " + err.Error()})
			return
		}
		if chain == nil {
			chain = []auditchain.Link{}
		}
		c.JSON(http.StatusOK, map[string]any{
			"session_id": sessionID,
			"at":         queryAt(tUnixMs),
			"links":      chain,
			"count":      len(chain),
		})
	}
}

// sessionStateHandler implements GET /api/session/:id/state?at=RFC3339.
//
// Phase 24 Task 24-03 (time-travel state): reconstructs the session
// state at time T (visible turns + effective approval-rules) from the
// INSERT-only audit substrate. Revoked rules are rebuilt from their
// action_rule_revoked row's frozen_args column so a point-in-time
// query sees the rule that was live at T (T-3-AUDIT "audit row is
// evidence" guarantee).
//
// at is required (state without a bound is undefined — there's
// always a "now"). 400 on missing or malformed at.
func sessionStateHandler(deps *Deps) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := c.Param("id")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "session id required"})
			return
		}
		if deps == nil || deps.Store == nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "audit store not configured"})
			return
		}
		rawAt := c.Query("at")
		if rawAt == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "at (RFC3339) is required"})
			return
		}
		t, err := time.Parse(time.RFC3339, rawAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "at must be RFC3339"})
			return
		}
		adapter := &chainStoreAdapter{inner: deps.Store}
		state, err := auditchain.StateAtSession(ctx, adapter, sessionID, t)
		if err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "state: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, state)
	}
}

// chainStoreAdapter bridges store.Store to auditchain.Store. The
// auditchain package is a leaf (no upward import on internal/server),
// so the adapter lives on the server side.
//
// Implementation note: auditchain.Store's AuditBySessionAt returns
// []store.AuditRow; the underlying store.Store has the exact same
// signature in its interface, so the adapter is identity (it just
// holds a typed reference). Keeping it as a named type (rather than
// a type alias) lets the auditchain tests pass a fakeStore without
// pulling in the SQLite package.
type chainStoreAdapter struct {
	inner interface {
		AuditBySessionAt(ctx context.Context, sessionID string, tUnixMs int64) ([]store.AuditRow, error)
	}
}

func (a *chainStoreAdapter) AuditBySessionAt(ctx context.Context, sessionID string, tUnixMs int64) ([]store.AuditRow, error) {
	return a.inner.AuditBySessionAt(ctx, sessionID, tUnixMs)
}

// queryAt renders the "at" parameter in the response envelope. Empty
// string when no bound was supplied.
func queryAt(tUnixMs int64) string {
	if tUnixMs <= 0 {
		return ""
	}
	return time.UnixMilli(tUnixMs).UTC().Format(time.RFC3339Nano)
}

// SessionChainHandler returns the decision-chain handler bound to
// this Deps. Wired into the route table at server boot — one
// closure per process.
func (d *Deps) SessionChainHandler() app.HandlerFunc { return sessionChainHandler(d) }

// SessionStateHandler returns the time-travel state handler bound to
// this Deps.
func (d *Deps) SessionStateHandler() app.HandlerFunc { return sessionStateHandler(d) }
