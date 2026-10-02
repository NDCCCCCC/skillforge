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

// audit_api.go is Phase 11 / Plan 11-04's audit-timeline endpoint —
// the back-end data source for the FE AuditTimelineTab (Plan 11-07).
//
//	GET /api/session/:id/audit?type=<action>&limit=<N>
//
// Returns the session's audit chain rows newest-first (the store's
// canonical order), optionally filtered to one HITL lifecycle action.
// Every query parameter is validated at the HTTP boundary and reaches
// the store exclusively as a `?`-parameterized value (T-3-SQLI).
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// auditActionsAllowlist enumerates the HITL lifecycle action values
// the `type` query parameter accepts (plus "" = all actions). The
// enum mirrors the action constants in approval_api.go plus the
// D11-D2 rule-revoke row.
var auditActionsAllowlist = map[string]bool{
	auditActionPending:    true,
	auditActionApproved:   true,
	auditActionRejected:   true,
	"action_rule_revoked": true,
}

// auditEntry is the wire shape of one audit row, scoped to the
// columns the FE AuditTimelineTab renders (11-07 consumer).
type auditEntry struct {
	ID             int64           `json:"id"`
	Ts             time.Time       `json:"ts"`
	Action         string          `json:"action"`
	Target         string          `json:"target,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	FrozenArgs     json.RawMessage `json:"frozen_args,omitempty"`
	ToolCallID     string          `json:"tool_call_id,omitempty"`
	ApprovalID     string          `json:"approval_id,omitempty"`
	ScopeSessionID string          `json:"scope_session_id,omitempty"`
	SkillVersion   string          `json:"skill_version"`
	PromptHash     string          `json:"prompt_hash"`
	UserID         string          `json:"user_id,omitempty"`
}

// auditListResponse is the GET /api/session/:id/audit envelope.
type auditListResponse struct {
	Rows  []auditEntry `json:"rows"`
	Total int          `json:"total"`
}

// auditListHandler implements GET /api/session/:id/audit.
//
// Query parameters:
//
//	type  — one of action_pending_approval | action_approved |
//	        action_rejected | action_rule_revoked; omitted = all
//	        actions. Unknown value → 400.
//	limit — 1..200, default 50. Non-numeric → default; <1 → 400;
//	        >200 → capped at 200.
//
// Status codes: 200 (empty or populated — existence-leak posture,
// unknown sessions return 200 + empty) | 400 (malformed session id /
// bad type / bad limit) | 404 (Deps.Store nil) | 500 (store error).
func (s *Deps) auditListHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusNotFound, map[string]string{
			"error": "audit store not initialized",
		})
		return
	}
	sessionID := c.Param("id")
	if !sessionIDRegex.MatchString(sessionID) {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid session id",
		})
		return
	}

	typeFilter := c.Query("type")
	if typeFilter != "" && !auditActionsAllowlist[typeFilter] {
		c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid type filter",
		})
		return
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			// Non-numeric limit degrades to the default rather than
			// erroring — the FE's initial page load must not 400 on a
			// malformed hand-typed URL.
			n = 50
		}
		if n < 1 {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "limit must be >= 1",
			})
			return
		}
		if n > 200 {
			n = 200
		}
		limit = n
	}

	// T-3-SQLI: SessionID + Action + Limit all reach the store as
	// `?`-parameterized values (QueryAudit never fmt.Sprintfs user
	// input into SQL).
	rows, err := s.Store.QueryAudit(ctx, store.AuditFilter{
		SessionID: sessionID,
		Action:    typeFilter,
		Limit:     limit,
	})
	if err != nil {
		log.Printf("[WARN] server: audit list (sess=%s type=%q) failed: %v", sessionID, typeFilter, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "audit list failed",
		})
		return
	}

	out := make([]auditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, auditEntry{
			ID:             r.ID,
			Ts:             r.Timestamp,
			Action:         r.Action,
			Target:         r.Query, // the audit writer stores the write target in the query column
			Reason:         r.RefusalReason,
			FrozenArgs:     json.RawMessage(r.FrozenArgs),
			ToolCallID:     r.ToolCallID,
			ApprovalID:     r.ApprovalID,
			ScopeSessionID: r.ScopeSessionID,
			SkillVersion:   r.SkillVersion,
			PromptHash:     r.PromptHash,
			UserID:         r.UserID,
		})
	}

	c.JSON(http.StatusOK, auditListResponse{Rows: out, Total: len(out)})
}
