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

// Package scheduler implements the cron-driven report regeneration service
// (Phase 19 / Plan 19-02).
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/skillforge/skillforge/internal/store"
)

// ReportJobInput is the input for triggering a report run (from cron or manual trigger).
type ReportJobInput struct {
	SkillID  string
	Period   string
	CronExpr string
	IsManual bool // true = manual trigger, false = cron trigger
}

// RunReportJob executes a single report run for the given skill and period.
// It checks the eval gate, acquires the in-flight slot, runs the DeepAgent path,
// captures KPI snapshots, detects drift, and writes the report_runs audit row.
func RunReportJob(ctx context.Context, input ReportJobInput, storeDB store.Store, evalGate *EvalGate) error {
	key := SkillPeriodKey(input.SkillID, input.Period)

	// 1. Eval gate check (GEVL-11) — skip cron runs if faithfulness regressed
	if !input.IsManual && evalGate != nil {
		pass, reason, err := evalGate.Check(input.SkillID)
		if err != nil {
			log.Printf("[WARN] scheduler: eval gate check failed for %s: %v", key, err)
			// Fail open: log and proceed
		} else if !pass {
			log.Printf("[INFO] scheduler: eval gate blocked %s: %s", key, reason)
			writeBlockedRun(ctx, input, storeDB, "eval_gate_blocked", reason)
			return nil
		}
	}

	// 2. Concurrent guard — acquire in-flight slot
	if !tryAcquireInFlight(key) {
		log.Printf("[INFO] scheduler: skipping %s (already in flight)", key)
		writeBlockedRun(ctx, input, storeDB, "skipped_duplicate", "concurrent run in progress")
		return nil
	}
	defer releaseInFlight(key)

	// 3. Get previous KPI snapshot for drift detection
	var prevSnapshots map[string]any
	if storeDB != nil {
		prevRow, err := storeDB.GetLatestReportRunBySkill(ctx, input.SkillID, input.Period)
		if err != nil {
			log.Printf("[WARN] scheduler: GetLatestReportRunBySkill failed: %v", err)
		} else if prevRow != nil {
			prevSnapshots, _ = ParseKPISnapshots(prevRow.KPISnapshots)
		}
	}

	// 4. Run the DeepAgent report generation path (stub — full integration in 19-03)
	runID := uuid.New().String()
	firedAt := time.Now().UTC()
	currSnapshots, runErr := executeReportRun(ctx, input.SkillID, input.Period)

	// 5. Detect drift
	driftDetected := false
	if runErr == nil && len(currSnapshots) > 0 {
		driftDetected, _ = DetectDrift(prevSnapshots, currSnapshots, 0.05) // 5% default threshold
	}

	// 6. Determine status
	status := "success"
	outputURL := ""
	if runErr != nil {
		status = "failed"
		log.Printf("[ERROR] scheduler: report run failed for %s: %v", key, runErr)
	}

	// 7. Write report_runs audit row
	snapshotJSON, _ := SerializeKPISnapshots(currSnapshots)
	row := store.ReportRunRow{
		RunID:         runID,
		SkillID:       input.SkillID,
		Period:        input.Period,
		CronExpr:      input.CronExpr,
		FiredAt:       firedAt,
		KPISnapshots:  snapshotJSON,
		OutputURL:     outputURL,
		Status:        status,
		DriftDetected: driftDetected,
		CreatedAt:     time.Now().UTC(),
	}
	if err := storeDB.PutReportRun(ctx, row); err != nil {
		log.Printf("[ERROR] scheduler: PutReportRun failed: %v", err)
	}

	return runErr
}

// writeBlockedRun writes a report_runs row for a blocked/skipped run.
func writeBlockedRun(ctx context.Context, input ReportJobInput, storeDB store.Store, status, reason string) {
	if storeDB == nil {
		return
	}
	row := store.ReportRunRow{
		RunID:        uuid.New().String(),
		SkillID:      input.SkillID,
		Period:       input.Period,
		CronExpr:     input.CronExpr,
		FiredAt:      time.Now().UTC(),
		Status:       status,
		DriftDetected: false,
		CreatedAt:    time.Now().UTC(),
	}
	row.KPISnapshots, _ = SerializeKPISnapshots(map[string]any{"skip_reason": reason})
	_ = storeDB.PutReportRun(ctx, row)
}

// executeReportRun calls the KPI executor to get current KPI values.
// This is the same kpi_call logic used by the DeepAgent during report generation.
// Returns a map of KPI name -> value. Stub — full integration in 19-03.
func executeReportRun(ctx context.Context, skillID, period string) (map[string]any, error) {
	return make(map[string]any), nil
}

// tryAcquireInFlight is a package-level helper that delegates to s.inFlight.
func tryAcquireInFlight(key string) bool {
	return inFlightSingleton.tryAcquire(key)
}

// releaseInFlight is a package-level helper.
func releaseInFlight(key string) {
	inFlightSingleton.release(key)
}

var inFlightSingleton = &inFlight{m: make(map[string]time.Time)}
