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

// Command eval_snapshot renders the published eval snapshot that lives
// in the README's "## Eval Results" section (DIST-09 / D7-11). It is a
// read-only consumer of the report + lock that cmd/eval --frozen
// produces; it never touches a live LLM, retriever, or API key.
//
// Why this command exists (RESEARCH §Open Questions Q1): the trust
// evaluator writes four metrics to eval/report.json (faithfulness /
// context_precision / answer_relevance / context_recall). DIST-09 also
// asks for "refusal accuracy", but that is NOT a field the evaluator
// emits — it is DERIVED from Report.Cases by filtering on
// Category=="refusal-required" and counting Refused==true. This command
// is the ONE place that derivation happens, so the README number always
// traces to the same logic a stranger can re-run.
//
// Output: a markdown block on stdout suitable for pasting into README's
// Eval Results section. It includes the run date, the lock metadata
// (eval_set_version / model_ver / prompt_hash), the Overall metric
// cells, the derived refusal_accuracy cell, and the case-composition
// breakdown.
//
// Usage:
//
//	go run ./scripts/eval_snapshot \
//	    -report eval/report.json \
//	    -lock  eval/frozen/compliance.lock.json
//
// Both flags default to the canonical paths; the typical invocation is
// just `go run ./scripts/eval_snapshot` after `cmd/eval --frozen`.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/eval"
)

const (
	defaultReportPath = "eval/report.json"
	defaultLockPath   = "eval/frozen/compliance.lock.json"

	// refusalCategory is the CaseResult.Category literal that marks a
	// case whose correct agent behavior is refusal (out-of-scope / PII
	// / insufficient context). Refusal accuracy is derived as the
	// fraction of refusal-required cases the agent actually refused.
	// Kept as a const so the literal string appears once and the
	// derivation reads as plain English.
	refusalCategory = "refusal-required"

	// shortHashLen is how many hex chars of the prompt_hash to show in
	// the snapshot header. The full hash is in the lock file; the
	// snapshot only needs enough to eyeball "did this move?".
	shortHashLen = 8
)

func main() {
	reportPath := flag.String("report", defaultReportPath, "path to the eval/report.json written by `cmd/eval --frozen`")
	lockPath := flag.String("lock", defaultLockPath, "path to the frozen .lock.json sidecar (eval_set_version / model_ver / prompt_hash)")
	runDate := flag.String("date", time.Now().UTC().Format("2006-01-02"), "run date stamp (YYYY-MM-DD UTC) for the snapshot header")
	flag.Parse()

	if err := run(*reportPath, *lockPath, *runDate, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] eval_snapshot: %v\n", err)
		os.Exit(1)
	}
}

// run is the testable seam: it loads the two files, renders the
// snapshot, and writes it to w. main() is the only thing that touches
// flag / os.Exit; everything else flows through here.
func run(reportPath, lockPath, runDate string, w *os.File) error {
	report, err := eval.LoadReport(reportPath)
	if err != nil {
		return fmt.Errorf("load report: %w", err)
	}
	lock, err := eval.LoadLock(lockPath)
	if err != nil {
		return fmt.Errorf("load lock: %w", err)
	}
	fmt.Fprint(w, renderSnapshot(report, lock, runDate))
	return nil
}

// deriveRefusalAccuracy is the DIST-09 derivation (RESEARCH Q1).
//
// refusal_accuracy = count(Cases where Category=="refusal-required"
// && Refused==true) / count(Cases where Category=="refusal-required")
//
// Returns (ratio, total, refused). When total == 0 (a degenerate
// fixture with zero refusal-required cases) the ratio is 0 and the
// renderer is expected to surface "N/A" rather than "0%" — a missing
// category is not a 0% score.
func deriveRefusalAccuracy(report eval.Report) (ratio float64, total, refused int) {
	for _, c := range report.Cases {
		if c.Category == refusalCategory {
			total++
			if c.Refused {
				refused++
			}
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	return float64(refused) / float64(total), total, refused
}

// countByCategory tallies cases per Category for the "50 (20
// adversarial / 20 refusal-required / 10 multi-hop)" composition cell.
func countByCategory(report eval.Report) map[string]int {
	counts := map[string]int{}
	for _, c := range report.Cases {
		counts[c.Category]++
	}
	return counts
}

// renderSnapshot produces the markdown block to paste into README's
// "## Eval Results" section. The format mirrors RESEARCH Example 6:
//
//	Frozen run on **<date>** · `eval_set_version: <v>` · `model_ver:
//	<m>` · `prompt_hash: <short>…`
//
//	| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |
//	|-------|-------------|-------------------|------------------|-------|
//	| compliance | 92% | 85% | 95% (19/20) | 50 (20 adversarial / 20 refusal-required / 10 multi-hop) |
//
//	**Reproduce:** `go run ./cmd/eval --frozen=compliance`
//
// Every value traces to either the report (Overall + Cases) or the
// lock file. Nothing is invented.
func renderSnapshot(report eval.Report, lock eval.LockFile, runDate string) string {
	skill := report.Skill
	if skill == "" {
		skill = lock.SkillName
	}
	if skill == "" {
		skill = eval.DefaultGateSkill
	}

	ratio, totalRefusal, refused := deriveRefusalAccuracy(report)
	counts := countByCategory(report)

	faith := report.Overall["faithfulness"]
	cprec := report.Overall["context_precision"]

	refusalCell := "N/A"
	if totalRefusal > 0 {
		refusalCell = fmt.Sprintf("%.0f%% (%d/%d)", ratio*100, refused, totalRefusal)
	}

	promptShort := lock.PromptHash
	if len(promptShort) > shortHashLen {
		promptShort = promptShort[:shortHashLen]
	}

	var b strings.Builder
	fmt.Fprintf(&b,
		"Frozen run on **%s** · `eval_set_version: %s` · `model_ver: %s` · `prompt_hash: %s…`\n\n",
		runDate, lock.EvalSetVersion, lock.ModelVersion, promptShort)
	b.WriteString("| Skill | Faithfulness | Context Precision | Refusal Accuracy | Cases |\n")
	b.WriteString("|-------|-------------|-------------------|------------------|-------|\n")
	fmt.Fprintf(&b, "| %s | %.0f%% | %.0f%% | %s | %d (%s) |\n",
		skill,
		faith*100,
		cprec*100,
		refusalCell,
		len(report.Cases),
		categoryBreakdown(counts))
	b.WriteString("\n**Reproduce:** `go run ./cmd/eval --frozen=compliance`\n")
	return b.String()
}

// categoryBreakdown renders the parenthetical case-composition cell in
// the canonical order (adversarial, refusal-required, multi-hop) from
// internal/eval/dataset.go's Case doc. Unknown categories are skipped —
// the published golden set is fixed at three categories, and inventing
// a sorted "extras" section would render one-off spike fixtures as if
// they were a stable contract.
func categoryBreakdown(counts map[string]int) string {
	order := []string{"adversarial", refusalCategory, "multi-hop"}
	parts := make([]string, 0, len(order))
	for _, cat := range order {
		if n := counts[cat]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, cat))
		}
	}
	return strings.Join(parts, " / ")
}
