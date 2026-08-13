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

import "sort"

// DefaultCrossEvalDeltaPct is the self-eval-bias alarm threshold
// (D6-14). It is not an arbitrary number: eval/README.md already
// documents ">5% faithfulness delta between judge models" as the
// Pitfall 8 signal that the judge is grading its own homework. Phase 6
// turns that documented heuristic into executable code.
const DefaultCrossEvalDeltaPct = 5.0

// crossEvalEpsilon absorbs binary floating-point error at the exact
// threshold boundary. Without it, two judges scoring 0.92 and 0.87 —
// a delta of exactly 5 points — compute to 5.000000000000004 and would
// trip a ">5%" alarm. An alarm that fires on the 15th decimal place
// trains operators to ignore it, which costs more than the alarm is
// worth.
const crossEvalEpsilon = 1e-9

// CrossEvalDelta is one Skill's cross-judge comparison.
//
// DeltaPct is expressed in PERCENTAGE POINTS of the 0..1 faithfulness
// scale (0.92 vs 0.86 → 6.0), matching how the threshold is written and
// discussed ("more than 5%").
type CrossEvalDelta struct {
	Skill               string             `json:"skill"`
	FaithfulnessByJudge map[string]float64 `json:"faithfulness_by_judge"`
	DeltaPct            float64            `json:"delta_pct"`
	Flagged             bool               `json:"flagged"`
}

// crossEvalDiff compares per-Skill faithfulness across judge models and
// flags the Skills whose scores disagree by more than
// deltaThresholdPct percentage points.
//
// PURE FUNCTION — mirrors Calibrate's shape (data in, structured result
// out, no I/O). The CLI decides which judges to run and where reports
// land; this function only compares them.
//
// A large delta does NOT mean one judge is right and the other wrong.
// It means the number is judge-dependent and therefore should not be
// published as if it were a property of the system. That is the whole
// point of the Pitfall 8 counter-measure.
//
// Skills scored by fewer than two judges have DeltaPct 0 and are never
// flagged — there is nothing to disagree with.
//
// deltaThresholdPct <= 0 falls back to DefaultCrossEvalDeltaPct so a
// zero-value caller gets the documented behavior rather than flagging
// everything.
func crossEvalDiff(reportsByJudge map[string]Report, deltaThresholdPct float64) []CrossEvalDelta {
	if deltaThresholdPct <= 0 {
		deltaThresholdPct = DefaultCrossEvalDeltaPct
	}

	// Collect faithfulness per (skill, judge). Judges are visited in
	// sorted order so the output is deterministic (map iteration is not).
	judges := make([]string, 0, len(reportsByJudge))
	for judge := range reportsByJudge {
		judges = append(judges, judge)
	}
	sort.Strings(judges)

	bySkill := map[string]map[string]float64{}
	var skillOrder []string
	for _, judge := range judges {
		r := reportsByJudge[judge]
		skill := r.Skill
		if skill == "" {
			skill = DefaultGateSkill
		}
		if _, seen := bySkill[skill]; !seen {
			bySkill[skill] = map[string]float64{}
			skillOrder = append(skillOrder, skill)
		}
		bySkill[skill][judge] = r.Overall["faithfulness"]
	}
	sort.Strings(skillOrder)

	out := make([]CrossEvalDelta, 0, len(skillOrder))
	for _, skill := range skillOrder {
		scores := bySkill[skill]
		delta := CrossEvalDelta{Skill: skill, FaithfulnessByJudge: scores}
		if len(scores) >= 2 {
			first := true
			var min, max float64
			for _, v := range scores {
				if first {
					min, max, first = v, v, false
					continue
				}
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
			delta.DeltaPct = (max - min) * 100
			delta.Flagged = delta.DeltaPct > deltaThresholdPct+crossEvalEpsilon
		}
		out = append(out, delta)
	}
	return out
}

// CrossEvalDiff is the exported seam over crossEvalDiff so cmd/eval can
// run the comparison while the logic (and its tests) stay in this
// package.
func CrossEvalDiff(reportsByJudge map[string]Report, deltaThresholdPct float64) []CrossEvalDelta {
	return crossEvalDiff(reportsByJudge, deltaThresholdPct)
}
