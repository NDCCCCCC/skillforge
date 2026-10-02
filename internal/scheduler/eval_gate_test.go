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

package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvalGate_Check_NoEvalData(t *testing.T) {
	// Temp dir with no eval reports — should fail open (return pass=true)
	tmpDir := t.TempDir()
	g := NewEvalGate(tmpDir, tmpDir)

	pass, reason, err := g.Check("nonexistent-skill")
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !pass {
		t.Errorf("Check() pass = false, want true when no eval data")
	}
	if reason != "no_eval_data" {
		t.Errorf("Check() reason = %q, want %q", reason, "no_eval_data")
	}
}

func TestEvalGate_Check_BlockedByThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	reportsDir := filepath.Join(tmpDir, "reports")
	skillsDir := filepath.Join(tmpDir, "skills")
	_ = os.MkdirAll(reportsDir, 0755)
	_ = os.MkdirAll(filepath.Join(skillsDir, "test-skill"), 0755)

	// Write a low-faithfulness eval report
	reportJSON := `{"skill":"test-skill","overall":{"faithfulness":0.5}}`
	_ = os.WriteFile(filepath.Join(reportsDir, "2026-08-01T09-00-00.json"), []byte(reportJSON), 0644)

	// Write threshold that requires 0.9
	thresholdJSON := `{"faithfulness_threshold":0.9}`
	_ = os.WriteFile(filepath.Join(skillsDir, "test-skill", "eval-gate.json"), []byte(thresholdJSON), 0644)

	g := NewEvalGate(reportsDir, skillsDir)
	pass, reason, err := g.Check("test-skill")

	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if pass {
		t.Errorf("Check() pass = true, want false when faithfulness 0.5 < threshold 0.9")
	}
	if reason == "" {
		t.Errorf("Check() reason is empty, want descriptive reason")
	}
}

func TestEvalGate_Check_PassesThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	reportsDir := filepath.Join(tmpDir, "reports")
	skillsDir := filepath.Join(tmpDir, "skills")
	_ = os.MkdirAll(reportsDir, 0755)
	_ = os.MkdirAll(filepath.Join(skillsDir, "good-skill"), 0755)

	// Write a high-faithfulness eval report
	reportJSON := `{"skill":"good-skill","overall":{"faithfulness":0.95}}`
	_ = os.WriteFile(filepath.Join(reportsDir, "2026-08-01T09-00-00.json"), []byte(reportJSON), 0644)

	g := NewEvalGate(reportsDir, skillsDir) // default threshold 0.9
	pass, reason, err := g.Check("good-skill")

	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !pass {
		t.Errorf("Check() pass = false, want true when faithfulness 0.95 >= threshold 0.9")
	}
	if reason != "" {
		t.Errorf("Check() reason = %q, want empty", reason)
	}
}

func TestEvalGate_loadThreshold_Default(t *testing.T) {
	tmpDir := t.TempDir()
	g := NewEvalGate(tmpDir, tmpDir) // no eval-gate.json file

	threshold, err := g.loadThreshold("any-skill")
	if err != nil {
		t.Fatalf("loadThreshold() error = %v, want nil", err)
	}
	if threshold != 0.9 {
		t.Errorf("loadThreshold() = %v, want default 0.9", threshold)
	}
}
