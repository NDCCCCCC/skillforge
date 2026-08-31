// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Plan 20-04 / ADM-04 — admin read-only skill registry view.
//
// Wire contract:
//
//	GET /api/admin/skills
//	    → 200 {skills: [...]}     success (admin sees all skills + hot-reload status)
//	    → 403 {error}             missing/wrong X-Admin-Token
//
// Threat model (20-04 threat_model):
//
//   - T-20-04-MUTATE — zero mutation verbs source-grep gate. This handler
//       is retrieval-only; the skill registry is a read-only snapshot and
//       eval results are immutable artifacts.
//
//   - T-20-04-TOOLTIP — eval result is shown as a hover tooltip in the
//       FE only (SkillManagementList), not as an inline badge. The 200ms
//       delay prevents flash on mouse pass-through.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

// evalIndexEntry is one run entry in eval/reports/INDEX.json.
type evalIndexEntry struct {
	Timestamp       string  `json:"ts"`
	Skill          string  `json:"skill"`
	JudgeModel     string  `json:"judge_model"`
	Faithfulness   float64 `json:"faithfulness"`
	ContextPrecision float64 `json:"context_precision"`
	ReportPath     string  `json:"report_path"`
}

// evalIndex is the top-level structure of eval/reports/INDEX.json.
type evalIndex struct {
	Version int               `json:"version"`
	Runs    []evalIndexEntry `json:"runs"`
}

// evalSummary holds the subset of eval metrics surfaced in the admin
// skill list tooltip. The full eval report lives in eval/reports/ and
// is not served through this endpoint.
type evalSummary struct {
	Faithfulness     float64 `json:"faithfulness"`
	ContextPrecision float64 `json:"context_precision"`
	EvalTS          string  `json:"eval_ts"`
}

// skillAdminEntry is one skill in the GET /api/admin/skills response.
type skillAdminEntry struct {
	Name            string       `json:"name"`
	Version         int64        `json:"version"`
	Status          string       `json:"status"` // "ok" | "failed" | "pending"
	LastEvalSummary *evalSummary `json:"last_eval_summary"`
	LastChangeAt    string       `json:"last_change_at,omitempty"`
}

// skillAdminResponse is the 200 envelope for GET /api/admin/skills.
type skillAdminResponse struct {
	Skills []skillAdminEntry `json:"skills"`
}

// adminSkillsHandler implements GET /api/admin/skills.
//
// The handler composes data from two sources:
//   - Skill registry (s.Registry): name, version, hot-reload status
//   - eval/reports/INDEX.json + latest report: last eval summary for each skill
//
// If eval reports are not available (eval not yet run), the
// corresponding skill carries last_eval_summary: null.
func (s *Deps) adminSkillsHandler(ctx context.Context, c *app.RequestContext) {
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, map[string]string{"error": "admin token required"})
		return
	}

	// Collect skills from the registry snapshot.
	var entries []skillAdminEntry
	if s.Registry != nil {
		snap := s.Registry.Snapshot()
		failed := s.Registry.Failed()
		ids := make([]string, 0, len(snap))
		for id := range snap {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			entry := skillAdminEntry{Name: id, Version: 0, Status: "ok"}
			if e := snap[id]; e != nil {
				entry.Version = e.Version
			}
			if _, ok := failed[id]; ok {
				entry.Status = "failed"
			}
			entries = append(entries, entry)
		}
		// Surface skills that failed before entering the snapshot.
		for name := range failed {
			if _, ok := snap[name]; ok {
				continue
			}
			entries = append(entries, skillAdminEntry{
				Name:   name,
				Status: "failed",
			})
		}
	} else if s.Loader != nil {
		for _, m := range s.Loader.Skills() {
			if m == nil {
				continue
			}
			entries = append(entries, skillAdminEntry{
				Name:   m.Name,
				Status: "ok",
			})
		}
	}

	// Overlay eval summaries from eval/reports/INDEX.json.
	evalReportsDir := os.Getenv("SKILLFORGE_EVAL_REPORTS_DIR")
	if evalReportsDir == "" {
		evalReportsDir = "eval/reports"
	}
	indexPath := filepath.Join(evalReportsDir, "INDEX.json")
	latestBySkill := loadLatestEval(indexPath)

	for i := range entries {
		if summary, ok := latestBySkill[entries[i].Name]; ok {
			entries[i].LastEvalSummary = summary
		}
	}

	c.JSON(http.StatusOK, skillAdminResponse{Skills: entries})
}

// loadLatestEval reads eval/reports/INDEX.json and returns a map from
// skill name to the evalSummary of the most recent run for that skill.
// Returns an empty map if the index cannot be read (eval never run).
func loadLatestEval(indexPath string) map[string]*evalSummary {
	file, err := os.Open(indexPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var index evalIndex
	if err := json.NewDecoder(file).Decode(&index); err != nil {
		return nil
	}

	// Find the latest run per skill (runs are in reverse-chronological order).
	latest := make(map[string]*evalSummary)
	for _, run := range index.Runs {
		if _, ok := latest[run.Skill]; ok {
			continue // already have a newer run for this skill
		}
		ts, err := time.Parse("2006-01-02T15-04-05Z", run.Timestamp)
		if err != nil {
			continue
		}
		latest[run.Skill] = &evalSummary{
			Faithfulness:     run.Faithfulness,
			ContextPrecision: run.ContextPrecision,
			EvalTS:          ts.UTC().Format(time.RFC3339),
		}
	}
	return latest
}
