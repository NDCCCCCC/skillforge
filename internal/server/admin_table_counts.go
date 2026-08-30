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

// Plan 20-01 (ADM-01) — admin console capacity monitor backend.
//
// GET /api/admin/table-counts returns per-table row counts and thresholds
// for the capacity monitor bars in the admin dashboard.
//
// Threat model:
//   - T-20-01 Spoofing — X-Admin-Token validated via isAdminRequest
//     (crypto/subtle.ConstantTimeCompare, timing-attack safe).
//   - T-20-02 Information Disclosure — 403 on missing/invalid token;
//     no information leakage about table structure or data.

package server

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/skillforge/skillforge/internal/store"
)

// tableCapacity holds a count and its operational threshold.
type tableCapacity struct {
	Count     int64 `json:"count"`
	Threshold int64 `json:"threshold"`
}

// tableCountsResponse is the wire shape for GET /api/admin/table-counts.
type tableCountsResponse struct {
	Audit      tableCapacity `json:"audit"`
	KPICalls   tableCapacity `json:"kpi_calls"`
	ReportRuns tableCapacity `json:"report_runs"`
}

// capacityThreshold reads an env var with a default. Returns the default
// when the env is unset or unparseable.
func capacityThreshold(envKey string, fallback int64) int64 {
	v := os.Getenv(envKey)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// tableCountsHandler implements GET /api/admin/table-counts.
func (s *Deps) tableCountsHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "store not initialized",
		})
		return
	}

	auditThreshold := capacityThreshold("SKILLFORGE_AUDIT_CAPACITY", 10_000)
	kpiThreshold := capacityThreshold("SKILLFORGE_KPI_CALLS_CAPACITY", 50_000)
	reportThreshold := capacityThreshold("SKILLFORGE_REPORT_RUNS_CAPACITY", 5_000)

	auditCount, err := s.Store.CountRows(ctx, "audit")
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "failed to count audit rows"})
		return
	}
	kpiCount, err := s.Store.CountRows(ctx, "kpi_calls")
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "failed to count kpi_calls rows"})
		return
	}
	reportCount, err := s.Store.CountRows(ctx, "report_runs")
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{Error: "failed to count report_runs rows"})
		return
	}

	c.JSON(http.StatusOK, tableCountsResponse{
		Audit:      tableCapacity{Count: auditCount, Threshold: auditThreshold},
		KPICalls:   tableCapacity{Count: kpiCount, Threshold: kpiThreshold},
		ReportRuns: tableCapacity{Count: reportCount, Threshold: reportThreshold},
	})
}
