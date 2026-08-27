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
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// KPICase is one case in the GEVL-09/10 golden set.
type KPICase struct {
	ID                  string   `json:"id"`
	Category            string   `json:"category"`
	Query               string   `json:"query"`
	ExpectedKPI         string   `json:"expected_kpi"`           // single KPI name
	ExpectedKPIs       []string `json:"expected_kpis"`         // multi-KPI names
	ExpectedValue       float64 `json:"expected_value"`        // single KPI expected value
	ExpectedValues     []float64 `json:"expected_values"`      // multi-KPI expected values
	AllowedExpressions []string   `json:"allowed_expressions"`  // shared fallback strings
	AllowedExpressionsMap map[string][]string `json:"allowed_expressions_map"` // per-KPI for multi-KPI cases
	ExpectedPeriod     string   `json:"expected_period"`
	Source             string   `json:"source"` // "synthetic"
}

// KPIJudgment is the result of judging one GEVL-09/10 case.
type KPIJudgment struct {
	CaseID     string
	Passed     bool
	Reason     string // "missing_as_of" | "missing_channel" | "value_mismatch" | "ok"
	MatchedKPI string
}

// JudgeKPIReferenceConsistency implements GEVL-10 (faithfulness extension to data).
//
// Three-step programmatic value match (D17-D2):
//   - Step 1: as_of token present in answer text.
//   - Step 2: channel-name token "data:aggregated_only" present in answer text.
//   - Step 3: every number in answer matches a snapshot row (within 0.5% tolerance)
//     or matches an allowed expression.
//
// Returns one KPIJudgment per case.
func JudgeKPIReferenceConsistency(c *KPICase, answerText string, snapRows []map[string]any) KPIJudgment {
	// Step 1: as_of token present.
	if !containsASOf(answerText) {
		return KPIJudgment{CaseID: c.ID, Passed: false, Reason: "missing_as_of"}
	}

	// Step 2: channel-name token present.
	if !strings.Contains(answerText, "data:aggregated_only") {
		return KPIJudgment{CaseID: c.ID, Passed: false, Reason: "missing_channel"}
	}

	// Step 3: every number in answer matches a snapshot row or an allowed expression.
	numbers := ExtractNumbers(answerText)
	if len(numbers) == 0 {
		return KPIJudgment{CaseID: c.ID, Passed: false, Reason: "value_mismatch: no numbers found"}
	}

	// Resolve expected KPI(s) and values.
	expectedKPIs := c.ExpectedKPIs
	if len(expectedKPIs) == 0 {
		expectedKPIs = []string{c.ExpectedKPI}
	}

	for _, n := range numbers {
		matched := false
		// Check if number matches any snapshot row within tolerance.
		for _, row := range snapRows {
			if numberInRow(n, row, 0.005) {
				matched = true
				break
			}
		}
		// Fallback: check allowed expressions.
		if !matched {
			allowed := c.AllowedExpressions
			if len(c.AllowedExpressionsMap) > 0 {
				allowed = c.AllowedExpressions
			}
			for _, expr := range allowed {
				f, ok := toFloat(expr)
				if ok && math.Abs(n-f)/math.Max(math.Abs(f), 1e-9) <= 0.005 {
					matched = true
					break
				}
			}
		}
		if !matched {
			return KPIJudgment{CaseID: c.ID, Passed: false, Reason: fmt.Sprintf("value_mismatch: %v not in snapshot", n)}
		}
	}

	return KPIJudgment{CaseID: c.ID, Passed: true, Reason: "ok", MatchedKPI: c.ExpectedKPI}
}

// containsASOf returns true if the answer text contains an as-of reference.
// Accepts Chinese and English variants.
func containsASOf(text string) bool {
	return strings.Contains(text, "as_of") ||
		strings.Contains(text, "数据截至") ||
		strings.Contains(text, "截至") ||
		strings.Contains(text, "截至") ||
		strings.Contains(text, "截止")
}

// getAllowedForKPI returns the allowed expression list for a specific KPI.
// Falls back to the shared AllowedExpressions list.
func getAllowedForKPI(c *KPICase, kpiName string) []string {
	if c.AllowedExpressionsMap != nil {
		if exprs, ok := c.AllowedExpressionsMap[kpiName]; ok {
			return exprs
		}
	}
	return c.AllowedExpressions
}

// matchesAnySnapshot checks if a number matches any value in snapshot rows
// (within tolerance) or matches any allowed expression.
func matchesAnySnapshot(num float64, rows []map[string]any, allowed []string, tol float64) bool {
	// Check snapshot rows.
	for _, row := range rows {
		for _, v := range row {
			f, ok := toFloat(v)
			if !ok {
				continue
			}
			if math.Abs(num-f)/math.Max(math.Abs(f), 1e-9) <= tol {
				return true
			}
		}
	}
	// Check allowed expressions.
	for _, expr := range allowed {
		f, ok := toFloat(expr)
		if ok && math.Abs(num-f)/math.Max(math.Abs(f), 1e-9) <= tol {
			return true
		}
	}
	return false
}

// ExtractNumbers extracts all numbers from text, stripping comma separators.
// Handles both integers and floats with optional units after the number.
func ExtractNumbers(text string) []float64 {
	re := regexp.MustCompile(`-?\d[\d,]*\.?\d*`)
	var out []float64
	for _, m := range re.FindAllString(text, -1) {
		cleaned := strings.ReplaceAll(m, ",", "")
		var f float64
		if _, err := fmt.Sscanf(cleaned, "%f", &f); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// toFloat converts a value of any numeric type (or numeric string) to float64.
func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// numberInRow checks if num matches any value in a single snapshot row within tolerance.
func numberInRow(num float64, row map[string]any, tol float64) bool {
	for _, v := range row {
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		if math.Abs(num-f)/math.Max(math.Abs(f), 1e-9) <= tol {
			return true
		}
	}
	return false
}
