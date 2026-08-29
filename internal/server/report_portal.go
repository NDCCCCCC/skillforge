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

	"github.com/cloudwego/hertz/pkg/app"
)

// reportsListHandler handles GET /reports with optional ?skill_id= and ?period= filters.
// Returns JSON list of report run rows.
// No auth required (matches /eval pattern per PORT-01).
func (deps *Deps) reportsListHandler(ctx context.Context, c *app.RequestContext) {
	skillID := string(c.Query("skill_id"))
	period := string(c.Query("period"))
	limitStr := string(c.Query("limit", "20"))
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	if deps.Store == nil {
		c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
		return
	}

	rows, err := deps.Store.ListReportRuns(ctx, skillID, period, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}
