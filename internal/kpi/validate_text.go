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

package kpi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// kpiNamePattern is the lexical grammar for KPI names and dataset identifiers
// in KPI files. Mirrors the intent of internal/duckdb/connector.go:109-117
// ValidateDatasetID (^  [A-Za-z0-9_-]{1,64}$) but stricter: KPI names
// forbid hyphens to avoid ambiguity with the ds_{dataset}__{period} composite
// registration name separator. The CGO boundary means this package cannot
// import the duckdb package, so this is an intentional one-line mirror; WSL
// tests (16-06) assert that both grammars agree on common corpus.
var kpiNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// dsTokenPattern scans SQL text for ds_{identifier} alias tokens.
// Used by the Layer 1 cross-dataset-join pre-check: all matched tokens
// must be exactly "ds_{declared dataset}".
var dsTokenPattern = regexp.MustCompile(`ds_[A-Za-z0-9_-]+`)

// paramPlaceholderPattern extracts :param named placeholders from SQL text.
// Used for the bidirectional :param↔params coverage check (D16-A2).
var paramPlaceholderPattern = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

// aggFuncPattern detects aggregate function calls in SQL text (Pitfall 7 rule 1).
// KPI SQL must aggregate (not return row-level detail).
var aggFuncPattern = regexp.MustCompile(`(?i)\b(sum|count|min|max|avg)\s*\(`)

// ValidateKPIFile is the Layer 1 pure-Go fail-closed validator for a KPI
// definition file (hermetic — no CGO, no engine dependencies).
//
// Checks (all fail-closed; errors are wrapped via fmtError as kpi.%s: %w
// and name the failing field/rule/decision anchor):
//
//  1. Dataset: non-empty, passes kpiNamePattern, no "__" substring
//  2. KPIs: non-empty; per KPI: Name passes kpiNamePattern and is unique
//     within file; Description non-empty; SQL non-empty
//  3. Alias consistency: all ds_ tokens in SQL must be exactly "ds_{Dataset}"
//  4. Table reference form: SQL contains at least one "ds_{Dataset}.data" literal
//  5. CountColumn: non-empty and appears as "AS {CountColumn}" in SQL
//  6. Placeholder ↔ params bidirectional 1:1 (full diff, not first-error)
//  7. Per ParamSpec: Type ∈ {string, enum}; enum→Values non-empty, no dupes;
//     string+Pattern→regexp.Compile success; enum and Pattern mutually exclusive
//  8. Aggregation heuristic: SQL contains GROUP BY or an aggregate function
//  9. KMin >= 0; MaxRows >= 0
//
// First-error short-circuit applies to file-own checks (rules 1-2, 4-5, 8-9).
// Full diff applies to set-comparison checks (rules 3, 6) — the complete
// mismatch is listed, not just the first item. This division mirrors
// internal/data/rules.go:126-129 comments: file validation is first-error,
// column/placeholder diff is full-set.
func ValidateKPIFile(m *KPIFile) error {
	if m == nil {
		return fmtError("ValidateKPIFile", fmt.Errorf("kpi file is nil — a KPI file must declare dataset and kpis"))
	}

	// Rule 1: Dataset
	if strings.TrimSpace(m.Dataset) == "" {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"dataset is required (D16-A1: the dataset this file's KPIs reference)"))
	}
	if !kpiNamePattern.MatchString(m.Dataset) {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"dataset %q fails the kpi-name grammar (must match ^[A-Za-z][A-Za-z0-9_]{0,63}$; mirrors internal/duckdb/connector.go:109-117 ValidateDatasetID intent but stricter — hyphens forbidden to avoid ambiguity with composite names)",
			m.Dataset))
	}
	if strings.Contains(m.Dataset, "__") {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"dataset %q contains \"__\" — \"__\" is the reserved separator of the \"{dataset}__{period}\" composite registry name; use single underscores (D15-C2 / D16-A2)",
			m.Dataset))
	}

	// Rule 2: KPIs non-empty; per-KPI checks
	if len(m.KPIs) == 0 {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"kpis must declare at least one KPI definition — an empty kpis list is a configuration error (D16-A1)"))
	}
	seen := make(map[string][]int) // name → indices for duplicate detection
	for i, kpi := range m.KPIs {
		if strings.TrimSpace(kpi.Name) == "" {
			return fmtError("ValidateKPIFile", fmt.Errorf(
				"kpis[%d].name is required", i))
		}
		if !kpiNamePattern.MatchString(kpi.Name) {
			return fmtError("ValidateKPIFile", fmt.Errorf(
				"kpis[%d].name %q fails the kpi-name grammar (must match ^[A-Za-z][A-Za-z0-9_]{0,63}$)",
				i, kpi.Name))
		}
		seen[kpi.Name] = append(seen[kpi.Name], i)
		if strings.TrimSpace(kpi.Description) == "" {
			return fmtError("ValidateKPIFile", fmt.Errorf(
				"kpis[%d] (%s): description is required (shown in kpi_call tool catalog, D16-B1)",
				i, kpi.Name))
		}
		if strings.TrimSpace(kpi.SQL) == "" {
			return fmtError("ValidateKPIFile", fmt.Errorf(
				"kpis[%d] (%s): sql is required (D16-A2: SQL template with :param placeholders)",
				i, kpi.Name))
		}
	}
	// Report all duplicate names at once
	var dupes []string
	for name, indices := range seen {
		if len(indices) > 1 {
			dupes = append(dupes, name)
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"KPI names must be unique within file; duplicates: %v (D16-A1: every KPI must be unambiguously callable by name)",
			dupes))
	}

	// Per-KPI deeper checks (rules 3-8)
	for i, kpi := range m.KPIs {
		if err := validateKPIDef(m, &kpi, i); err != nil {
			return err
		}
	}

	// Rule 9: KMin >= 0; MaxRows checked per-KPI above
	if m.KMin < 0 {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"k_min must be >= 0 (got %d; 0 = use dataset k_min_override, then KMinDefault=5 — three-tier chain, D16-D1)",
			m.KMin))
	}

	return nil
}

// validateKPIDef checks rules 3-8 for a single KPIDef within its parent KPIFile.
func validateKPIDef(file *KPIFile, kpi *KPIDef, idx int) error {
	prefix := fmt.Sprintf("kpis[%d] (%s)", idx, kpi.Name)

	// Rule 3: Alias consistency — all ds_ tokens in SQL must be exactly ds_{Dataset}
	tokens := dsTokenPattern.FindAllString(kpi.SQL, -1)
	expectedAlias := "ds_" + file.Dataset
	var illegalTokens []string
	for _, tok := range tokens {
		if tok != expectedAlias {
			illegalTokens = append(illegalTokens, tok)
		}
	}
	if len(illegalTokens) > 0 {
		unique := uniqueSorted(illegalTokens)
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: SQL contains alias token(s) %v not matching declared dataset %q — cross-dataset join is forbidden (D14-D1 physical isolation; Layer 1 text pre-check, Layer 2 AST in 16-05 authoritative)",
			prefix, unique, file.Dataset))
	}

	// Rule 4: Table reference form — SQL must contain "ds_{Dataset}.data" at least once
	expectedTableRef := expectedAlias + ".data"
	if !strings.Contains(kpi.SQL, expectedTableRef) {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: SQL must reference table %q (staging table name is always \"data\" — internal/data/importer.go:1049; Layer 1 pre-check, Layer 2 AST in 16-05)",
			prefix, expectedTableRef))
	}

	// Rule 5: CountColumn non-empty and appears as "AS {CountColumn}" in SQL
	if strings.TrimSpace(kpi.CountColumn) == "" {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: count_column is required (k_min row-level suppression needs the aggregate count column, D16-D1)",
			prefix))
	}
	asPattern := regexp.MustCompile(`(?i)\bAS\s+` + regexp.QuoteMeta(kpi.CountColumn) + `\b`)
	if !asPattern.MatchString(kpi.SQL) {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: count_column %q does not appear as \"AS %s\" in SQL (k_min needs this column to determine row count; Layer 1 text check, Layer 2 AST in 16-05)",
			prefix, kpi.CountColumn, kpi.CountColumn))
	}

	// Rule 6: Placeholder ↔ params bidirectional 1:1 (full diff, D16-A2)
	placeholders := paramPlaceholderPattern.FindAllStringSubmatch(kpi.SQL, -1)
	sqlParams := make(map[string]bool)
	for _, m := range placeholders {
		sqlParams[m[1]] = true
	}
	declaredParams := make(map[string]bool)
	for name := range kpi.Params {
		declaredParams[name] = true
	}

	// Diff: SQL has but params doesn't
	var inSQLNotDeclared []string
	for p := range sqlParams {
		if !declaredParams[p] {
			inSQLNotDeclared = append(inSQLNotDeclared, p)
		}
	}
	// Diff: params declares but SQL doesn't
	var declaredNotInSQL []string
	for p := range declaredParams {
		if !sqlParams[p] {
			declaredNotInSQL = append(declaredNotInSQL, p)
		}
	}
	if len(inSQLNotDeclared) > 0 || len(declaredNotInSQL) > 0 {
		sort.Strings(inSQLNotDeclared)
		sort.Strings(declaredNotInSQL)
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: SQL placeholder↔params mismatch — SQL has params missing: %v; params has SQL missing: %v (D16-A2: bidirectional 1:1 coverage required)",
			prefix, inSQLNotDeclared, declaredNotInSQL))
	}

	// Rule 7: Per ParamSpec validation
	paramNames := make([]string, 0, len(kpi.Params))
	for name := range kpi.Params {
		paramNames = append(paramNames, name)
	}
	sort.Strings(paramNames)
	for _, name := range paramNames {
		ps := kpi.Params[name]
		switch ps.Type {
		case ParamTypeString:
			if len(ps.Values) > 0 {
				return fmtError("ValidateKPIFile", fmt.Errorf(
					"%s: params.%s: string type must not declare values (meaningless combination — string parameters use pattern for value-domain constraint, D16-A3)",
					prefix, name))
			}
			if ps.Pattern != "" {
				if _, err := regexp.Compile(ps.Pattern); err != nil {
					return fmtError("ValidateKPIFile", fmt.Errorf(
						"%s: params.%s: pattern %q does not compile: %v (fail-closed: a bad regex cannot enforce the value-domain constraint at call time)",
						prefix, name, ps.Pattern, err))
				}
			}
		case ParamTypeEnum:
			if len(ps.Values) == 0 {
				return fmtError("ValidateKPIFile", fmt.Errorf(
					"%s: params.%s: enum type requires non-empty values (D16-A3: the enum value set is the differential whitelist, D16-D2)",
					prefix, name))
			}
			if ps.Pattern != "" {
				return fmtError("ValidateKPIFile", fmt.Errorf(
					"%s: params.%s: enum type must not declare pattern (enum and pattern are mutually exclusive — enum values are an explicit allowlist, pattern is a regex constraint, D16-A3)",
					prefix, name))
			}
			// Check for duplicate enum values
			seen := make(map[string]bool)
			var dupes []string
			for _, v := range ps.Values {
				if seen[v] {
					dupes = append(dupes, v)
				}
				seen[v] = true
			}
			if len(dupes) > 0 {
				return fmtError("ValidateKPIFile", fmt.Errorf(
					"%s: params.%s: enum values contain duplicates: %v (fail-closed: duplicate values are a declaration error)",
					prefix, name, dupes))
			}
		default:
			return fmtError("ValidateKPIFile", fmt.Errorf(
				"%s: params.%s: type %q is not supported (valid types: %q, %q — vocabulary default-deny, D16-A3)",
				prefix, name, ps.Type, ParamTypeString, ParamTypeEnum))
		}
	}

	// Rule 8: Aggregation heuristic (Pitfall 7 rule 1)
	hasGroupBy := strings.Contains(strings.ToUpper(kpi.SQL), "GROUP BY")
	hasAggFunc := aggFuncPattern.MatchString(kpi.SQL)
	if !hasGroupBy && !hasAggFunc {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: SQL must contain GROUP BY or an aggregate function (sum/count/min/max/avg) — KPI must aggregate, not return row-level detail (Pitfall 7 rule 1; heuristic, Layer 2 AST in 16-05 complements)",
			prefix))
	}

	// Rule 9 (per-KPI): MaxRows >= 0
	if kpi.MaxRows < 0 {
		return fmtError("ValidateKPIFile", fmt.Errorf(
			"%s: max_rows must be >= 0 (got %d; 0 = use package default, D16-D1)",
			prefix, kpi.MaxRows))
	}

	return nil
}

// uniqueSorted returns the sorted unique elements of s.
func uniqueSorted(s []string) []string {
	m := make(map[string]bool)
	for _, v := range s {
		m[v] = true
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// fmtError wraps an error with the kpi package prefix (Shared Pattern H).
func fmtError(fn string, err error) error {
	return fmt.Errorf("kpi.%s: %w", fn, err)
}
