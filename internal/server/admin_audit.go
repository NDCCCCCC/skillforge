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

// Plan 20-06 / ADM-06 — admin audit sub-view endpoint.
//
// Wire contract:
//
//	GET /api/admin/audit
//	    → 200 [...]       success (admin sees raw rows — isAdmin=true)
//	    → 403 {error}     missing/wrong X-Admin-Token
//	    → 503 {error}     Deps.Store == nil
//	    → 500 {error}     QueryAudit failed
//
// Threat model (20-06 threat_model):
//
//   - T-20-06-AUDIT  — actor_type column is ADDITIVE only. The store
//                       layer has no UPDATE/DELETE path for audit rows.
//                       INSERT-only invariant is preserved.
//
//   - T-20-06-REDACT  — Phase 6 redact() is applied with isAdmin=true,
//                       so the admin sees raw rows (the access policy
//                       of the X-Admin-Token path is to bypass the
//                       retention projection).
//
//   - T-6-TOKEN       — isAdminRequest gate uses
//                       crypto/subtle.ConstantTimeCompare (ASVS V6.2.1).
//                       The token value is never logged.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// adminAuditHandler implements GET /api/admin/audit. The handler:
//
//  1. 403 if X-Admin-Token is missing/wrong (isAdminRequest gate).
//  2. 503 if Deps.Store is nil (mirrors the graph / wizard posture).
//  3. Call store.QueryAudit with ActorType="admin" filter — 100%
//     parameterized (T-3-SQLI).
//  4. Apply Phase 6 redact() with isAdmin=true (admin bypasses
//     retention projection — T-20-06-REDACT).
//  5. 200 with the filtered rows.
func (s *Deps) adminAuditHandler(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(c) {
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "audit store not initialized"})
		return
	}

	filter := store.AuditFilter{ActorType: "admin"}
	rows, err := s.Store.QueryAudit(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "audit query failed"})
		return
	}

	// T-20-06-REDACT: isAdmin=true bypasses the retention projection.
	// This is the whole point of the X-Admin-Token path — the operator
	// has already authenticated and is explicitly opting into raw-row
	// access by carrying the token.
	redact(rows, auditRetention(), time.Now(), true)

	c.JSON(http.StatusOK, rows)
}
