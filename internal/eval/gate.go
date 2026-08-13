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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultFaithfulnessThreshold is the gate bar a Skill must clear when
// it ships no eval-gate.json of its own (D6-10). Different Skill types
// warrant different quality bars — a compliance auditor is held to 0.9,
// a future creative Skill might be held to 0.7 — so the threshold is
// per-Skill data, not a constant in the gate.
const DefaultFaithfulnessThreshold = 0.9

// DefaultGateSkill is the Skill a report is attributed to when it
// carries no Skill field (reports written before Phase 6 added it).
const DefaultGateSkill = "compliance"

// GateThresholdFileName is the per-Skill threshold file, co-located
// with the SKILL.md it gates.
//
// Q2 RESOLUTION: the file lives at skills/<skill>/eval-gate.json —
// alongside the PRODUCT Skill. NOT .claude/skills/<skill>/, which holds
// this repository's own meta-skills (spike-findings, sketch-findings)
// and has nothing to do with SkillForge's runtime Skills. This also
// matches the D6-09 CI path filter (skills/*/SKILL.md).
const GateThresholdFileName = "eval-gate.json"

// SkillResult is one Skill's gate outcome. It carries the Threshold
// alongside the score because SC-2 requires the PR comment to name BOTH
// ("the PR comment names the Skill and the threshold") — a bare
// pass/fail would force the reader to go hunting for the bar.
type SkillResult struct {
	Skill        string  `json:"skill"`
	Faithfulness float64 `json:"faithfulness"`
	Threshold    float64 `json:"threshold"`
	Pass         bool    `json:"pass"`
}

// gateThresholds is the per-Skill threshold map read from disk.
type gateThresholds = map[string]float64

// gateDecision evaluates a report's per-Skill faithfulness against the
// per-Skill thresholds and returns one SkillResult per Skill evaluated.
//
// PURE FUNCTION — no I/O, no logging, no process exit. This is the
// Pitfall 6 counter-measure: the gate's decision logic is the part that
// must be unit-testable and CI-independent, so it is separated from the
// GitHub Actions plumbing (which formats markdown and sets an exit
// code) and from cmd/eval (which must keep its exit-0 contract).
//
// Callers determine pass/fail by inspecting the results: a run passes
// when every SkillResult.Pass is true. FailedSkills is the convenience
// filter for that.
//
// Phase 6 ships exactly one Skill, so the returned slice has one entry;
// the shape generalizes to N Skills without a signature change.
func gateDecision(report Report, thresholds gateThresholds) []SkillResult {
	skill := report.Skill
	if skill == "" {
		skill = DefaultGateSkill
	}

	threshold, ok := thresholds[skill]
	if !ok {
		threshold = DefaultFaithfulnessThreshold
	}

	faithfulness := report.Overall["faithfulness"]

	return []SkillResult{{
		Skill:        skill,
		Faithfulness: faithfulness,
		Threshold:    threshold,
		Pass:         faithfulness >= threshold,
	}}
}

// GateDecision is the exported seam over the pure gateDecision function
// so cmd/eval (a different package) can run the gate without the logic
// leaving internal/eval, where its unit tests live.
func GateDecision(report Report, thresholds map[string]float64) []SkillResult {
	return gateDecision(report, thresholds)
}

// FailedSkills filters a gate decision down to the Skills that missed
// their bar. An empty result means the gate passes.
func FailedSkills(results []SkillResult) []SkillResult {
	var failed []SkillResult
	for _, r := range results {
		if !r.Pass {
			failed = append(failed, r)
		}
	}
	return failed
}

// LoadThresholds scans skillsDir for per-Skill eval-gate.json files and
// returns the Skill→threshold map that gateDecision consumes.
//
// A Skill with no eval-gate.json is simply absent from the map, which
// gateDecision reads as DefaultFaithfulnessThreshold — "no declared bar
// means the default bar", never "no gate".
//
// A malformed or unreadable eval-gate.json IS an error: silently
// falling back to the default would let a typo quietly lower a Skill's
// quality bar, which is the failure mode a promotion gate exists to
// prevent.
func LoadThresholds(skillsDir string) (map[string]float64, error) {
	out := map[string]float64{}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("eval.LoadThresholds: read dir %q: %w", skillsDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillsDir, e.Name(), GateThresholdFileName)
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // no declared bar → gateDecision applies the default
			}
			return nil, fmt.Errorf("eval.LoadThresholds: read %q: %w", path, err)
		}
		var cfg struct {
			FaithfulnessThreshold *float64 `json:"faithfulness_threshold"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return nil, fmt.Errorf("eval.LoadThresholds: parse %q: %w", path, err)
		}
		if cfg.FaithfulnessThreshold == nil {
			return nil, fmt.Errorf("eval.LoadThresholds: %q missing required field %q", path, "faithfulness_threshold")
		}
		out[e.Name()] = *cfg.FaithfulnessThreshold
	}
	return out, nil
}

// FormatGateMarkdown renders a gate decision as the markdown table that
// gets posted to the PR (D6-11 / SC-2). The table names the Skill, its
// faithfulness, its threshold and a ✅/❌ verdict — SC-2 requires the
// comment to identify the failing Skill and the bar it missed.
//
// Rows are sorted by Skill name so the comment is diff-stable across
// runs (map iteration order would otherwise reshuffle it).
func FormatGateMarkdown(results []SkillResult) string {
	sorted := make([]SkillResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Skill < sorted[j].Skill })

	var b strings.Builder
	b.WriteString("### SkillForge eval gate\n\n")
	b.WriteString("| Skill | faithfulness | threshold | verdict |\n")
	b.WriteString("|-------|--------------|-----------|---------|\n")
	for _, r := range sorted {
		verdict := "✅"
		if !r.Pass {
			verdict = "❌"
		}
		fmt.Fprintf(&b, "| %s | %.4f | %.4f | %s |\n", r.Skill, r.Faithfulness, r.Threshold, verdict)
	}

	failed := FailedSkills(sorted)
	b.WriteString("\n")
	if len(failed) == 0 {
		b.WriteString("All Skills meet their faithfulness threshold.\n")
		return b.String()
	}
	names := make([]string, 0, len(failed))
	for _, f := range failed {
		names = append(names, fmt.Sprintf("`%s` (%.4f < %.4f)", f.Skill, f.Faithfulness, f.Threshold))
	}
	fmt.Fprintf(&b, "**Gate FAILED** — below threshold: %s\n", strings.Join(names, ", "))
	return b.String()
}

// LoadReport reads a report.json written by WriteReport. Used by the
// --gate-check path, which runs in CI against an already-produced
// report and therefore needs no LLM, no retriever and no API key.
func LoadReport(path string) (Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return Report{}, fmt.Errorf("eval.LoadReport: open %q: %w", path, err)
	}
	defer f.Close()

	var r Report
	if err := json.NewDecoder(f).Decode(&r); err != nil {
		return Report{}, fmt.Errorf("eval.LoadReport: decode %q: %w", path, err)
	}
	return r, nil
}
