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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/skillforge/skillforge/internal/scheduler"
)

// RerenderRequest is the POST body for /api/reports/rerender.
type RerenderRequest struct {
	SkillID string `json:"skill_id"`
	Period  string `json:"period"`
}

// rerenderHandler triggers an immediate report rerender for the given skill/period.
// POST /api/reports/rerender {skill_id, period}
// Returns 202 Accepted with {run_id} on success.
// Per D-05: reuses the DeepAgent run path via scheduler.RunReportJob with IsManual=true.
func (deps *Deps) rerenderHandler(ctx context.Context, c *app.RequestContext) {
	var req RerenderRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.SkillID == "" || req.Period == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "skill_id and period are required"})
		return
	}

	if deps.Scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "scheduler not available"})
		return
	}

	input := scheduler.ReportJobInput{
		SkillID:  req.SkillID,
		Period:   req.Period,
		CronExpr: "",
		IsManual: true,
	}

	// Run asynchronously — return 202 immediately with a run_id
	runID := fmt.Sprintf("manual-%d", time.Now().UnixNano())
	go func() {
		_ = scheduler.RunReportJob(context.Background(), input, deps.Store, deps.Scheduler.EvalGate(), deps.KPIExecutor, nil)
	}()

	c.JSON(http.StatusAccepted, map[string]string{"run_id": runID})
}
