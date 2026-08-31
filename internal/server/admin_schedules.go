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

// Plan 20-05 / ADM-05 — admin read-only view of the scheduled task list.
//
// READ-ONLY BY CONSTRUCTION (ADM-05). Schedule definitions live in Skill
// files under SKILLFORGE_SKILLS_DIR (report_schedule field). The scheduler
// writes audit rows to report_runs via PutReportRun; this handler reads
// them via ListReportRuns. No mutation path exists in this file.
//
// Trust model:
//   - X-Admin-Token gate via isAdminRequest (same crypto/subtle compare as
//     GET /api/audit and GET /api/admin/kpis; unset env → 403 for every
//     caller).
//   - No caller input reaches SQL: ListReportRuns uses ? placeholders for
//     skillID and period (empty strings → no filter, all runs returned).
//   - Fail-closed when the Store is nil (503).

package server

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// scheduleRow is the JSON-serialisable form of one report run as rendered
// for the admin schedule view.
type scheduleRow struct {
	RunID       string `json:"run_id"`
	SkillID     string `json:"skill_id"`
	CronExpr    string `json:"cron_expr"`
	Status      string `json:"status"`
	LastFiredAt string `json:"last_fired_at"`
	NextFireAt  string `json:"next_fire_at"`
	DriftStatus string `json:"drift_status"`
}

// schedulesResponse is the 200 envelope for GET /api/admin/schedules.
type schedulesResponse struct {
	Schedules []scheduleRow `json:"schedules"`
}

// adminSchedulesHandler serves GET /api/admin/schedules.
//
// Response envelope (200):
//
//	{
//	  "schedules": [
//	    {
//	      "run_id": "run-001",
//	      "skill_id": "compliance",
//	      "cron_expr": "0 0 * * *",
//	      "status": "completed",
//	      "last_fired_at": "2026-08-28T00:00:00Z",
//	      "next_fire_at": "2026-08-29T00:00:00Z",
//	      "drift_status": "ok"
//	    }
//	  ]
//	}
//
// Errors: 403 (admin token missing/wrong/unset), 503 (Store nil), 500
// (ListReportRuns failed).
func (s *Deps) adminSchedulesHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{
			"error": "admin token required",
		})
		return
	}
	if s.Store == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{
			Error: "report runs store not initialized",
		})
		return
	}

	// Empty skillID + period → no filter; limit 100 for admin overview.
	rows, err := s.Store.ListReportRuns(ctx, "", "", 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, graphResponseError{
			Error: "list report runs: " + err.Error(),
		})
		return
	}

	schedules := make([]scheduleRow, 0, len(rows))
	for _, r := range rows {
		driftStatus := "ok"
		if r.DriftDetected {
			driftStatus = "drifted"
		}
		lastFiredAt := ""
		if !r.FiredAt.IsZero() {
			lastFiredAt = r.FiredAt.UTC().Format(time.RFC3339)
		}
		nextFireAt := ""
		// NextFireAt is not stored in ReportRunRow; the scheduler
		// computes it at fire-time and stores it in KPISnapshots or
		// derives it from CronExpr. We render the CreatedAt as a proxy
		// for ordering; a future schema column could carry the field.
		// For now, leave next_fire_at empty unless a computed value
		// is available in a future extension.
		_ = nextFireAt // populated when the schema extends
		_ = r.CronExpr // used directly below

		schedules = append(schedules, scheduleRow{
			RunID:       r.RunID,
			SkillID:     r.SkillID,
			CronExpr:    r.CronExpr,
			Status:      r.Status,
			LastFiredAt: lastFiredAt,
			NextFireAt:  nextFireAt,
			DriftStatus: driftStatus,
		})
	}

	c.JSON(http.StatusOK, schedulesResponse{
		Schedules: schedules,
	})
}
