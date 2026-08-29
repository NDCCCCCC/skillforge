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
// (Phase 19 / Plan 19-02 / GE VL-11).
package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skillforge/skillforge/internal/eval"
)

// EvalGate checks whether a Skill's latest eval faithfulness score is above
// the threshold before a cron-triggered report run fires (GEVL-11).
type EvalGate struct {
	evalReportsDir string // e.g. "eval/reports"
	thresholdsDir  string // e.g. "skills"
}

// NewEvalGate constructs an EvalGate with the given directories.
func NewEvalGate(evalReportsDir, thresholdsDir string) *EvalGate {
	return &EvalGate{evalReportsDir: evalReportsDir, thresholdsDir: thresholdsDir}
}

// Check checks the eval gate for skillID. Returns (pass, reason, error).
// Pass=true means the skill passes the gate (score >= threshold or no eval data).
// Pass=false with reason means blocked by eval gate.
func (g *EvalGate) Check(skillID string) (bool, string, error) {
	threshold, err := g.loadThreshold(skillID)
	if err != nil {
		return false, "", fmt.Errorf("eval gate: load threshold: %w", err)
	}

	score, err := g.getLatestFaithfulness(skillID)
	if err != nil {
		// No eval data — allow the run (fail open for v1; eval may not have run yet)
		return true, "no_eval_data", nil
	}

	if score < threshold {
		return false, fmt.Sprintf("faithfulness %.4f < threshold %.4f", score, threshold), nil
	}
	return true, "", nil
}

func (g *EvalGate) loadThreshold(skillID string) (float64, error) {
	// Try skill-specific eval-gate.json first
	gatePath := filepath.Join(g.thresholdsDir, skillID, eval.GateThresholdFileName)
	b, err := os.ReadFile(gatePath)
	if err == nil {
		var cfg struct {
			FaithfulnessThreshold *float64 `json:"faithfulness_threshold"`
		}
		if err := json.Unmarshal(b, &cfg); err == nil && cfg.FaithfulnessThreshold != nil {
			return *cfg.FaithfulnessThreshold, nil
		}
	}
	// Fall back to default
	return eval.DefaultFaithfulnessThreshold, nil
}

func (g *EvalGate) getLatestFaithfulness(skillID string) (float64, error) {
	entries, err := os.ReadDir(g.evalReportsDir)
	if err != nil {
		return 0, fmt.Errorf("read eval reports dir: %w", err)
	}

	type candidate struct {
		faith float64
		time  os.FileInfo
		path  string
	}
	var best candidate

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(g.evalReportsDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var report struct {
			Skill   string             `json:"skill"`
			Overall map[string]float64 `json:"overall"`
		}
		if err := json.Unmarshal(b, &report); err != nil {
			continue
		}
		if report.Skill != skillID {
			continue
		}
		faith, ok := report.Overall["faithfulness"]
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best.time == nil || info.ModTime().After(best.time.ModTime()) {
			best = candidate{faith: faith, time: info, path: path}
		}
	}

	if best.time == nil {
		return 0, fmt.Errorf("no faithfulness for skill %s", skillID)
	}
	return best.faith, nil
}
