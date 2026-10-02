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

package report

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValueFormatter transforms a KPI numeric value into its final display string.
// Applies thousand-separator + 2 decimal places + unit suffix.
// Also applies markdown cell escaping to prevent GFM table injection.
type ValueFormatter func(v any) string

// MarkdownCellEscaper escapes GFM table special characters.
var mdCellEscaper = strings.NewReplacer(
	"|", `\|`, "`", "\\`", "*", `\*`, "_", `\_`,
	"\n", " ", "<", `\<`, ">", `\>`,
	"[", `\[`, "]", `\]`,
)

// DefaultValueFormatter is the standard formatter used in render pipeline.
func DefaultValueFormatter(v any) string {
	switch val := v.(type) {
	case float64:
		return formatFloat(val)
	case int64:
		return formatInt(val)
	case string:
		return mdCellEscaper.Replace(val)
	default:
		return mdCellEscaper.Replace(fmt.Sprintf("%v", val))
	}
}

func formatFloat(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	integer := int64(v)
	decimal := (v - float64(integer)) * 100
	return fmt.Sprintf("%s%d.%02d", sign, integer, int64(decimal+0.5))
}

func formatInt(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	s := fmt.Sprintf("%d", v)
	var result strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(r)
	}
	return sign + result.String()
}

// ConsistencyReport captures the result of a narrative-consistency check.
// Mismatches are non-blocking per D18-A3 "标记" semantics.
type ConsistencyReport struct {
	Mismatches []ConsistencyMismatch
	Clean      bool
}

type ConsistencyMismatch struct {
	SectionIdx    int
	SlotIdx      int
	NarrativeText string
	ExpectedValue string
	ActualValue   string
	Tolerance     float64
}

// allowedExpressions lists number patterns that are allowed without KPI binding.
var allowedExpressions = []string{
	"第一", "第二", "第三", "第四", "第五",
	"一季度", "二季度", "三季度", "四季度",
	"环比", "同比",
}

var numberRE = regexp.MustCompile(`-?\d+(?:\.\d+)?`)

// isoDateRE strips YYYY-MM period stamps before number scanning so the
// "2026" / "08" halves of a date never surface as narrative numbers.
var isoDateRE = regexp.MustCompile(`\d{4}-\d{2}`)

// consistencyTolerance is the ±0.5% relative tolerance (D18-A3).
const consistencyTolerance = 0.005

// CheckNarrativeConsistency verifies that numbers in slot narrative text
// match a KPI value of the SAME section (per the SKILL.md contract:
// insight/risk_note numbers must come from the section's KPI snapshot;
// narrative slots must not cite numbers at all). A number is a mismatch
// when NO section KPI value matches it within ±0.5% relative tolerance
// (a percent-style narration like "1.23%" also matches a 0.0123 snapshot
// value via the ×100 form).
//
// WR-04 (18-REVIEW): the previous build was a stub — it extracted
// numbers, discarded every comparison input with `_ =`, and always
// returned Clean. The allowed-expression exemption is now PER-NUMBER
// (keyword immediately preceding the matched number), not per-text.
// Render emits each mismatch as a visible markdown blockquote marker
// that survives goldmark + bluemonday (HTML comments were stripped by
// both and never reached the published HTML).
//
// A nil snapshot checks nothing (clean) — Render's fail-safe posture.
func CheckNarrativeConsistency(tmpl *ReportTemplate, slots SlotTexts,
	snap interface{ Get(string) any }, fmtFn ValueFormatter) *ConsistencyReport {
	report := &ConsistencyReport{Clean: true}
	if snap == nil || tmpl == nil {
		return report
	}

	for si, slotMap := range slots {
		if si >= len(tmpl.Sections) {
			continue
		}
		// Candidate values: this section's KPI snapshot values, numeric
		// only. Non-numeric KPI values are skipped as candidates (they
		// cannot anchor a numeric narrative claim).
		candidates := make([]float64, 0, len(tmpl.Sections[si].KPIs))
		for _, kpiName := range tmpl.Sections[si].KPIs {
			if v := toFloat(snap.Get(kpiName)); v != nil {
				candidates = append(candidates, *v)
			}
		}
		for slotIdx, text := range slotMap {
			// Strip ISO period stamps first (2026-08 → not numbers).
			scanText := isoDateRE.ReplaceAllString(text, "")
			for _, loc := range numberRE.FindAllStringIndex(scanText, -1) {
				numStr := scanText[loc[0]:loc[1]]
				if isAllowedNumber(scanText, loc[0]) {
					continue
				}
				n, err := strconv.ParseFloat(numStr, 64)
				if err != nil {
					continue
				}
				// 4-digit years are dates, not narrative claims.
				if !strings.Contains(numStr, ".") && n >= 1900 && n <= 2100 && len(numStr) == 4 {
					continue
				}
				if matchesAny(n, candidates) {
					continue
				}
				expected := "（无可用 KPI 值）"
				if len(candidates) > 0 {
					formatted := make([]string, 0, len(candidates))
					for _, c := range candidates {
						formatted = append(formatted, fmtFn(c))
					}
					expected = strings.Join(formatted, " / ")
				}
				report.Mismatches = append(report.Mismatches, ConsistencyMismatch{
					SectionIdx:    si,
					SlotIdx:       slotIdx,
					NarrativeText: text,
					ExpectedValue: expected,
					ActualValue:   numStr,
					Tolerance:     consistencyTolerance,
				})
			}
		}
	}
	report.Clean = len(report.Mismatches) == 0
	return report
}

// isAllowedNumber reports whether the number starting at idx is exempt
// from the KPI-binding check: an allowed-expression keyword immediately
// precedes it (e.g. "环比增长5%" — the 5 is a comparative, not a KPI
// citation). PER-NUMBER, not per-text (WR-04). The window is 24 bytes
// (~8 CJK runes) so 2-rune keywords like 环比 / 同比 survive in front
// of phrases like "增长5%".
func isAllowedNumber(text string, idx int) bool {
	start := idx - 24
	if start < 0 {
		start = 0
	}
	prefix := text[start:idx]
	for _, expr := range allowedExpressions {
		if strings.Contains(prefix, expr) {
			return true
		}
	}
	return false
}

// matchesAny reports whether n matches any candidate within ±0.5%
// relative tolerance, testing the ×100 form too (percent narration of a
// ratio stored as a fraction).
func matchesAny(n float64, candidates []float64) bool {
	for _, c := range candidates {
		if withinTolerance(n, c) || withinTolerance(n, c*100) {
			return true
		}
	}
	return len(candidates) == 0 // no candidates → nothing to contradict
}

func withinTolerance(a, b float64) bool {
	if b == 0 {
		return a == 0
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= consistencyTolerance*absFloat(b)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// toFloat attempts to extract a numeric KPI value for comparison;
// returns nil for non-numeric values (the candidate is silently
// dropped — non-numeric KPI values can't anchor numeric narrative
// claims anyway).
func toFloat(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case float32:
		f := float64(n)
		return &f
	case int:
		f := float64(n)
		return &f
	case int64:
		f := float64(n)
		return &f
	case string:
		if f, err := strconv.ParseFloat(n, 64); err == nil {
			return &f
		}
	}
	return nil
}
