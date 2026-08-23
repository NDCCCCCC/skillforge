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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/verdict"
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

// TestShippedGateFileIsValid: the shipped compliance gate file at
// examples/skills/compliance/eval-gate.json must parse and declare 0.9
// (D6-10 + Q2 path resolution). The repo default Skill is simple-qa
// (quick task 260814-vga); the compliance Skill — and its gate
// threshold — ship as the examples/ template that a `cp -r
// examples/skills/compliance skills/` restore copies back into skills/
// for eval runs. A typo in the shipped file would silently change the
// gate after such a restore.
func TestShippedGateFileIsValid(t *testing.T) {
	thresholds, err := eval.LoadThresholds(filepath.Join("..", "..", "examples", defaultSkillsDir))
	require.NoError(t, err)
	require.Contains(t, thresholds, "compliance",
		"examples/skills/compliance/eval-gate.json must exist at the Q2-resolved path (NOT .claude/skills/)")
	assert.Equal(t, 0.9, thresholds["compliance"])
}

// TestClassifyRunOutcome locks the outcome modalities of the scoring-path
// classifier (14-11 Task 1 — Gap 1 in 14-VERIFICATION.md).
//
// The load-bearing row is "in-schema refused verdict": under the 1.1.0
// floor tail the model refuses INSIDE the verdict schema — a well-formed
// verdict with status:"refused", a verdict.ValidStatuses member. The
// pre-14-11 scoring path never recognized it, so refusal prose was judged
// by faithfulness against weakly-related chunks instead of taking the
// refusal conventions — the root cause of the 14-10 double-window replay
// FAIL (0.6876 / 0.6763 < 0.9 with refusal 0/20 twice; see
// 14-10-SUMMARY "Operator-Gate Addendum"). The fix routes it into the
// SAME frozen conventions as the Run-error / structured-refusal paths:
// refused=true + answer="" → Faithfulness empty-answer 1.0 +
// ContextRecall REFUSAL sentinel (refusal_accuracy derivation).
//
// Boundary guards: status:"needs_input" / "error" deliberately do NOT
// enter refusal accounting — only "refused" is semantically a refusal
// per verdict.ValidStatuses. Priority guard: err != nil wins over an
// in-schema refused verdict (no double counting).
func TestClassifyRunOutcome(t *testing.T) {
	citations := []verdict.Citation{{ChunkID: "7", Span: "数据铁律条款", Tier: "权威"}}
	refusalProse := "该请求超出平台数据边界：无授权数据源可支撑该查询，无法提供个体记录。"

	refusedVerdict := verdict.Verdict{
		Status:     "refused",
		Confidence: 1.0,
		Citations:  citations,
		Answer:     refusalProse,
	}
	okVerdict := verdict.Verdict{
		Status:     "ok",
		Confidence: 0.95,
		Citations:  citations,
		Answer:     "年假为 5 天。",
	}
	needsInputVerdict := verdict.Verdict{
		Status:     "needs_input",
		Confidence: 0.5,
		Citations:  citations,
	}

	tests := []struct {
		name          string
		err           error
		refusal       *verdict.Refusal
		v             verdict.Verdict
		wantRefused   bool
		wantAnswer    string // exact expectation ("" for every refusal row)
		wantAnswerSub string // substring expectation for scored-verdict rows
		wantReason    string
	}{
		{
			name:        "agent.Run error scores as refusal (existing modality)",
			err:         errors.New("context deadline exceeded"),
			v:           refusedVerdict, // v is returned alongside err in reality; err must win
			wantRefused: true,
			wantAnswer:  "",
			wantReason:  "agent_error",
		},
		{
			name:        "structured Refusal scores as refusal (existing modality)",
			refusal:     &verdict.Refusal{Reason: "no_authorized_source"},
			v:           okVerdict,
			wantRefused: true,
			wantAnswer:  "",
			wantReason:  "no_authorized_source",
		},
		{
			name:        "in-schema status:refused verdict scores as refusal (RED core — 14-11 fix target)",
			v:           refusedVerdict,
			wantRefused: true,
			wantAnswer:  "",
			wantReason:  "verdict_status_refused",
		},
		{
			name:          "ok verdict is scored, not refused (boundary guard)",
			v:             okVerdict,
			wantRefused:   false,
			wantAnswerSub: `"status":"ok"`,
			wantReason:    "",
		},
		{
			name:          "needs_input verdict is scored, not refused (boundary guard)",
			v:             needsInputVerdict,
			wantRefused:   false,
			wantAnswerSub: `"status":"needs_input"`,
			wantReason:    "",
		},
		{
			name:        "priority guard: err wins over in-schema refused verdict",
			err:         errors.New("judge transport reset"),
			v:           refusedVerdict,
			wantRefused: true,
			wantAnswer:  "",
			wantReason:  "agent_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			refused, answer, reason := classifyRunOutcome(tc.err, tc.refusal, tc.v)
			assert.Equal(t, tc.wantRefused, refused, "refused")
			assert.Equal(t, tc.wantReason, reason, "reason")
			if tc.wantAnswerSub != "" {
				assert.False(t, refused, "scored row must not be refused")
				assert.Contains(t, answer, tc.wantAnswerSub, "answer carries the serialized verdict JSON")
			} else {
				assert.Equal(t, tc.wantAnswer, answer, "refusal rows take the frozen empty-answer convention")
			}
		})
	}
}
