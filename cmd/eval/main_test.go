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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/eval"
)

// TestFlagMatrix_GoldenAndFrozenMutuallyExclusive: naming two different
// eval sets is a contradiction the CLI must refuse rather than resolve
// by preference — a frozen run's value is knowing exactly what was
// scored.
func TestFlagMatrix_GoldenAndFrozenMutuallyExclusive(t *testing.T) {
	err := validateFlagMatrix(true, "compliance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.Contains(t, err.Error(), "--frozen")
	assert.Contains(t, err.Error(), "--golden")
}

// TestFlagMatrix_ValidCombinations: each set source alone is fine, and
// the DEFAULT --golden value (not explicitly passed) does not conflict
// with --frozen.
func TestFlagMatrix_ValidCombinations(t *testing.T) {
	assert.NoError(t, validateFlagMatrix(false, ""), "neither flag → default golden set")
	assert.NoError(t, validateFlagMatrix(true, ""), "--golden alone")
	assert.NoError(t, validateFlagMatrix(false, "compliance"),
		"--frozen alone: the un-passed --golden default must NOT count as a conflict")
}

// TestFrozenPaths pins the D6-13 layout that the CLI, the CI workflow
// and the dashboard's frozen-set endpoint must all agree on.
func TestFrozenPaths(t *testing.T) {
	assert.Equal(t, filepath.Join("eval", "frozen", "compliance.jsonl"), frozenSetPath("compliance"))
	assert.Equal(t, filepath.Join("eval", "frozen", "compliance.lock.json"), frozenLockPath("compliance"))
}

// TestArchiveDirFor: cross-eval runs are partitioned per judge so two
// judges' scores never share one trend line.
func TestArchiveDirFor(t *testing.T) {
	assert.Equal(t, defaultArchiveDir, archiveDirFor("glm-5.2", false))
	assert.Equal(t, filepath.Join(defaultArchiveDir, "deepseek-chat"), archiveDirFor("deepseek-chat", true))
}

// TestParseJudgeModels covers the comma list, whitespace tolerance and
// the empty case (which means "one run with the ambient JUDGE_MODEL").
func TestParseJudgeModels(t *testing.T) {
	assert.Nil(t, parseJudgeModels(""))
	assert.Nil(t, parseJudgeModels("   "))
	assert.Equal(t, []string{"glm-5.2"}, parseJudgeModels("glm-5.2"))
	assert.Equal(t, []string{"glm-5.2", "deepseek-chat"}, parseJudgeModels("glm-5.2,deepseek-chat"))
	assert.Equal(t, []string{"glm-5.2", "deepseek-chat"}, parseJudgeModels(" glm-5.2 , deepseek-chat , "))
}

// writeTestReport writes a report.json for the gate-check tests.
func writeTestReport(t *testing.T, dir string, skill string, faithfulness float64) string {
	t.Helper()
	path := filepath.Join(dir, "report.json")
	require.NoError(t, eval.WriteReport(path, eval.Report{
		Skill:   skill,
		Overall: map[string]float64{"faithfulness": faithfulness},
	}))
	return path
}

// writeTestSkillsDir writes skills/<skill>/eval-gate.json (the
// Q2-resolved path).
func writeTestSkillsDir(t *testing.T, skill string, threshold float64) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, skill), 0o755))
	b, err := json.Marshal(map[string]float64{"faithfulness_threshold": threshold})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, skill, eval.GateThresholdFileName), b, 0o644))
	return dir
}

// TestRunGateCheck_PassAndFail: --gate-check is the ONE mode allowed to
// exit non-zero on a quality regression (the exit-code contract). It
// returns 0 above the bar and 1 below it.
func TestRunGateCheck_PassAndFail(t *testing.T) {
	skillsDir := writeTestSkillsDir(t, "compliance", 0.9)

	passReport := writeTestReport(t, t.TempDir(), "compliance", 0.95)
	assert.Equal(t, 0, runGateCheck(passReport, skillsDir), "0.95 >= 0.9 → gate passes")

	failReport := writeTestReport(t, t.TempDir(), "compliance", 0.85)
	assert.Equal(t, 1, runGateCheck(failReport, skillsDir), "0.85 < 0.9 → gate fails")
}

// TestRunGateCheck_MissingInputs: an unreadable report or a malformed
// threshold file fails the gate rather than passing it. A gate that
// cannot evaluate must never report success.
func TestRunGateCheck_MissingInputs(t *testing.T) {
	skillsDir := writeTestSkillsDir(t, "compliance", 0.9)
	assert.Equal(t, 1, runGateCheck(filepath.Join(t.TempDir(), "nope.json"), skillsDir))

	badSkills := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(badSkills, "compliance"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(badSkills, "compliance", eval.GateThresholdFileName), []byte(`{`), 0o644))
	assert.Equal(t, 1, runGateCheck(writeTestReport(t, t.TempDir(), "compliance", 0.99), badSkills))
}

// TestShippedGateFileIsValid: the repo's real skills/compliance/eval-gate.json
// must parse and declare 0.9 (D6-10 + Q2 path resolution). This is the
// file CI reads, so a typo in it would silently change the gate.
func TestShippedGateFileIsValid(t *testing.T) {
	thresholds, err := eval.LoadThresholds(filepath.Join("..", "..", defaultSkillsDir))
	require.NoError(t, err)
	require.Contains(t, thresholds, "compliance",
		"skills/compliance/eval-gate.json must exist at the Q2-resolved path (NOT .claude/skills/)")
	assert.Equal(t, 0.9, thresholds["compliance"])
}
