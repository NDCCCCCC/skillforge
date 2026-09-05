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

// audit_prov.go — Phase 24 Task 24-01: GET /api/session/:id/audit/prov.
//
// The session-scoped W3C PROV-JSON projection of the audit chain (the
// standards-export half of the Semantica对标). Isolation matches
// /api/session/:id/audit exactly: session_id WHERE clause (SC-9), no admin
// token — the export reveals nothing the session's own audit tab doesn't.
// Retention redaction is a GLOBAL /api/audit concern; the session face
// exports what the session owns (parity with auditListHandler), and any
// redaction sentinel already stored in a row passes through verbatim
// (auditprov never decodes it).
package server

import (
	"context"
	"log"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/auditprov"
	"github.com/skillforge/skillforge/internal/store"
)

// sessionProvHandler implements GET /api/session/:id/audit/prov.
func (s *Deps) sessionProvHandler(ctx context.Context, c *app.RequestContext) {
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "audit store not initialized (set Deps.Store at server construction)",
		})
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, graphResponseError{Error: "session id is required"})
		return
	}

	// SC-9 — the session_id filter is the isolation boundary; the cap is
	// the same clamp /api/audit uses (an export is a read, not a dump).
	rows, err := s.Store.QueryAudit(ctx, store.AuditFilter{SessionID: sessionID, Limit: maxAuditLimit})
	if err != nil {
		log.Printf("[WARN] server: audit prov query failed: %v", err)
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "audit prov query failed — see server logs",
		})
		return
	}

	// An empty session is a legal empty document (200, not 404) — the
	// provenance of "nothing happened yet" is an empty graph.
	c.JSON(http.StatusOK, auditprov.FromSessionRows(rows))
}
