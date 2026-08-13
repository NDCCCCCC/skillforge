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

package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateReport builds a minimal Report carrying a Skill + faithfulness.
func gateReport(skill string, faithfulness float64) Report {
	return Report{
		Skill:   skill,
		Overall: map[string]float64{"faithfulness": faithfulness},
	}
}

// TestGateDecision_PassWhenAboveThreshold: 0.95 clears a 0.9 bar, so no
// Skill fails.
func TestGateDecision_PassWhenAboveThreshold(t *testing.T) {
	results := gateDecision(gateReport("compliance", 0.95), map[string]float64{"compliance": 0.9})

	require.Len(t, results, 1)
	assert.True(t, results[0].Pass)
	assert.Empty(t, FailedSkills(results), "an empty failed set is what 'the gate passes' means")
}

// TestGateDecision_FailWhenBelowThreshold: 0.85 misses a 0.9 bar and
// the failing Skill is NAMED (SC-2).
func TestGateDecision_FailWhenBelowThreshold(t *testing.T) {
	results := gateDecision(gateReport("compliance", 0.85), map[string]float64{"compliance": 0.9})

	require.Len(t, results, 1)
	assert.Equal(t, SkillResult{Skill: "compliance", Faithfulness: 0.85, Threshold: 0.9, Pass: false}, results[0])

	failed := FailedSkills(results)
	require.Len(t, failed, 1)
	assert.Equal(t, "compliance", failed[0].Skill)
}

// TestGateDecision_BoundaryIsInclusive: exactly-at-threshold passes.
// A Skill sitting precisely on its declared bar has met it.
func TestGateDecision_BoundaryIsInclusive(t *testing.T) {
	results := gateDecision(gateReport("compliance", 0.9), map[string]float64{"compliance": 0.9})
	assert.True(t, results[0].Pass, "faithfulness == threshold MUST pass")
}

// TestGateDecision_DefaultThreshold: a Skill absent from the map is
// held to 0.9 (D6-10) — "no declared bar" means the default bar, never
// "no gate".
func TestGateDecision_DefaultThreshold(t *testing.T) {
	for _, tc := range []struct {
		name       string
		thresholds map[string]float64
	}{
		{"nil map", nil},
		{"empty map", map[string]float64{}},
		{"other Skill declared", map[string]float64{"creative": 0.7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := gateDecision(gateReport("compliance", 0.88), tc.thresholds)
			require.Len(t, results, 1)
			assert.Equal(t, DefaultFaithfulnessThreshold, results[0].Threshold)
			assert.False(t, results[0].Pass, "0.88 < default 0.9")
		})
	}
}

// TestGateDecision_NamesTheSkill: every result carries all four fields
// the PR comment needs, and a report with no Skill field (pre-Phase-6
// shape) is attributed to the default Skill rather than to "".
func TestGateDecision_NamesTheSkill(t *testing.T) {
	results := gateDecision(gateReport("compliance", 0.93), map[string]float64{"compliance": 0.9})
	require.Len(t, results, 1)
	assert.Equal(t, "compliance", results[0].Skill)
	assert.InDelta(t, 0.93, results[0].Faithfulness, 1e-9)
	assert.InDelta(t, 0.9, results[0].Threshold, 1e-9)
	assert.True(t, results[0].Pass)

	legacy := gateDecision(Report{Overall: map[string]float64{"faithfulness": 0.93}}, nil)
	assert.Equal(t, DefaultGateSkill, legacy[0].Skill, "a report without a Skill field still names a Skill")
}

// TestGateDecision_IsPure: the function must not mutate its inputs —
// CI calls it against a report it also uploads as an artifact.
func TestGateDecision_IsPure(t *testing.T) {
	report := gateReport("compliance", 0.85)
	thresholds := map[string]float64{"compliance": 0.9}

	_ = gateDecision(report, thresholds)

	assert.Equal(t, "compliance", report.Skill)
	assert.InDelta(t, 0.85, report.Overall["faithfulness"], 1e-9)
	assert.Equal(t, map[string]float64{"compliance": 0.9}, thresholds)
}

// TestFormatGateMarkdown names the Skill AND the threshold on failure —
// the literal SC-2 requirement for the PR comment.
func TestFormatGateMarkdown(t *testing.T) {
	pass := FormatGateMarkdown(gateDecision(gateReport("compliance", 0.95), nil))
	assert.Contains(t, pass, "| compliance |")
	assert.Contains(t, pass, "✅")
	assert.Contains(t, pass, "All Skills meet their faithfulness threshold")

	fail := FormatGateMarkdown(gateDecision(gateReport("compliance", 0.85), nil))
	assert.Contains(t, fail, "❌")
	assert.Contains(t, fail, "Gate FAILED")
	assert.Contains(t, fail, "compliance", "the comment MUST name the Skill (SC-2)")
	assert.Contains(t, fail, "0.9000", "the comment MUST name the threshold (SC-2)")
}

// TestLoadThresholds reads per-Skill eval-gate.json files from the
// Q2-resolved location (skills/<skill>/eval-gate.json).
func TestLoadThresholds(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "compliance"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "creative"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "compliance", GateThresholdFileName),
		[]byte(`{"faithfulness_threshold": 0.9}`), 0o644))
	// "creative" ships no eval-gate.json → absent from the map → default.

	thresholds, err := LoadThresholds(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"compliance": 0.9}, thresholds)

	missing, err := LoadThresholds(filepath.Join(dir, "does-not-exist"))
	require.NoError(t, err, "a missing skills dir yields an empty map, not an error")
	assert.Empty(t, missing)
}

// TestLoadThresholds_Malformed: a typo in a gate file is an ERROR. A
// silent fallback to the default would let a broken file quietly change
// a Skill's quality bar — the exact failure the gate exists to prevent.
func TestLoadThresholds_Malformed(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "compliance"), 0o755))
	path := filepath.Join(dir, "compliance", GateThresholdFileName)

	require.NoError(t, os.WriteFile(path, []byte(`{"faithfulness_threshold":`), 0o644))
	_, err := LoadThresholds(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")

	require.NoError(t, os.WriteFile(path, []byte(`{"other_field": 1}`), 0o644))
	_, err = LoadThresholds(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "faithfulness_threshold")
}

// TestCrossEvalDiff_FlaggedDelta: 0.92 vs 0.86 is a 6-point spread,
// above the documented 5% self-eval-bias signal (D6-14 / Pitfall 8).
func TestCrossEvalDiff_FlaggedDelta(t *testing.T) {
	deltas := crossEvalDiff(map[string]Report{
		"glm-5.2":       gateReport("compliance", 0.92),
		"deepseek-chat": gateReport("compliance", 0.86),
	}, DefaultCrossEvalDeltaPct)

	require.Len(t, deltas, 1)
	assert.Equal(t, "compliance", deltas[0].Skill)
	assert.InDelta(t, 6.0, deltas[0].DeltaPct, 1e-9)
	assert.True(t, deltas[0].Flagged, ">5% delta MUST be flagged")
	assert.Equal(t, map[string]float64{"glm-5.2": 0.92, "deepseek-chat": 0.86}, deltas[0].FaithfulnessByJudge)
}

// TestCrossEvalDiff_NoFlag: a delta at or below 5% is agreement.
func TestCrossEvalDiff_NoFlag(t *testing.T) {
	for _, tc := range []struct {
		name    string
		a, b    float64
		wantPct float64
	}{
		{"small delta", 0.92, 0.90, 2.0},
		// 0.92-0.87 computes to 5.000000000000004 in binary floating
		// point; the epsilon in crossEvalDiff keeps the ">5%" alarm from
		// firing on the 15th decimal place.
		{"exactly 5pct is not >5pct", 0.92, 0.87, 5.0},
		{"identical judges", 0.92, 0.92, 0.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deltas := crossEvalDiff(map[string]Report{
				"glm-5.2":       gateReport("compliance", tc.a),
				"deepseek-chat": gateReport("compliance", tc.b),
			}, DefaultCrossEvalDeltaPct)

			require.Len(t, deltas, 1)
			assert.InDelta(t, tc.wantPct, deltas[0].DeltaPct, 1e-9)
			assert.False(t, deltas[0].Flagged)
		})
	}
}

// TestCrossEvalDiff_SingleJudgeNeverFlags: one judge has nothing to
// disagree with.
func TestCrossEvalDiff_SingleJudgeNeverFlags(t *testing.T) {
	deltas := crossEvalDiff(map[string]Report{"glm-5.2": gateReport("compliance", 0.92)}, 0)

	require.Len(t, deltas, 1)
	assert.Equal(t, 0.0, deltas[0].DeltaPct)
	assert.False(t, deltas[0].Flagged)
}

// TestCrossEvalDiff_DefaultThresholdOnZero: a zero threshold falls back
// to the documented 5%, rather than flagging every non-identical pair.
func TestCrossEvalDiff_DefaultThresholdOnZero(t *testing.T) {
	deltas := crossEvalDiff(map[string]Report{
		"glm-5.2":       gateReport("compliance", 0.92),
		"deepseek-chat": gateReport("compliance", 0.90),
	}, 0)

	require.Len(t, deltas, 1)
	assert.False(t, deltas[0].Flagged, "2% delta with a zero (→ default 5%) threshold must not flag")
}

// TestCrossEvalDiff_Empty: no reports, no deltas, no panic.
func TestCrossEvalDiff_Empty(t *testing.T) {
	assert.Empty(t, crossEvalDiff(nil, DefaultCrossEvalDeltaPct))
}
