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
	"encoding/json"
	"errors"
)

// jsonNumber aliases encoding/json.Number — numeric literals scanned from
// JSON payloads arrive as this type (e.g. rows unmarshalled for replay
// paths). The alias keeps the WIP call-site readable.
type jsonNumber = json.Number

// errNonNumericCount is the fail-closed sentinel returned by toFloat64 when
// a row's count column is missing or non-numeric: Apply cannot PROVE the
// bucket holds >= kmin members, so the row must be treated as suppressed
// (D16-D1 — cannot-prove-safe means suppressed, never let through).
var errNonNumericCount = errors.New("kpi: count column value is missing or non-numeric")

// KMinDefault is the Phase 16 query-side k-anonymity floor (D16-D1).
// It mirrors internal/data/desensitize.go:39 KMinDefault — the CGO
// boundary (internal/data transitively depends on go-duckdb) forces the
// kpi package to mirror the constant instead of importing it. Equality
// of the two constants is locked by WSL test TestNameLexiconParity
// (16-06 Task 3); drift makes that test red.
//
// Three-tier chain (D15-D3 / 16-PATTERNS §7):
//   file k_min > 0 ? : dataset k_min_override > 0 ? : KMinDefault
//
// The literal is the call-site source-grep target (NOT a config knob).
const KMinDefault = 5

// ResolveKMin returns the effective k_min from the three-tier chain:
//
//	fileKMin > 0 ? fileKMin : datasetOverride > 0 ? datasetOverride : KMinDefault
//
// Negative override values are treated as "not set" (same as 0), matching
// the datarules/datasets convention where 0 = defer (D15-D3 semantics).
// This is a pure function with no side effects — hermetic testable.
func ResolveKMin(fileKMin, datasetOverride int) int {
	if fileKMin > 0 {
		return fileKMin
	}
	if datasetOverride > 0 {
		return datasetOverride
	}
	return KMinDefault
}

// Apply performs k_min row-level suppression post-filter on KPI result rows.
//
// BEHAVIOR CONTRACT (D16-D1 — this is NOT SQL HAVING semantics):
//
//  1. For each row where row[countColumn] (numeric) < kmin:
//     - The row is RETAINED (row count does NOT change — anti-HAVING).
//     - All value columns are replaced with the string "suppressed".
//     - The countColumn AND every column listed in preserveColumns
//       (the GROUP BY dimension identifiers + the CountColumn, per
//       CompiledKPI.PreserveColumns) retain their original values.
//       countColumn is preserved implicitly whether or not the caller
//       included it in preserveColumns ("除 countColumn 与 preserveColumns
//       外的全部列值替换" — the suppressed row still truthfully reports its
//       bucket size, which is what makes the suppression auditable).
//
//  2. Rows with count >= kmin are returned unmodified.
//
//  3. When count column value is missing or non-numeric: fail-closed —
//     the row is treated as suppressed (cannot prove ≥ kmin → suppressed).
//
//  4. kmin <= 0: zero suppression (defensive — ResolveKMin should always
//     produce a positive value, but Apply is safe for any input).
//
//  5. The input slice is NOT mutated — each returned row is a shallow copy
//     of the input row with value columns replaced. Callers can safely
//     freeze the output into audit snapshots.
//
// Pitfall 4 (HAVING trap): TestKMinRowSuppression asserts output row count
// == input row count. Any test that asserts fewer rows in the output has
// the HAVING bug — the correct behavior is row retention with value
// replacement, not row removal.
func Apply(rows []map[string]any, countColumn string, preserveColumns []string, kmin int) (out []map[string]any, suppressed int) {
	if kmin <= 0 || len(rows) == 0 {
		return rows, 0
	}

	preserveSet := make(map[string]bool, len(preserveColumns)+1)
	preserveSet[countColumn] = true // count column is ALWAYS preserved (D16-D1)
	for _, c := range preserveColumns {
		preserveSet[c] = true
	}

	out = make([]map[string]any, len(rows))
	for i, row := range rows {
		countVal, err := toFloat64(row[countColumn])
		if err != nil || countVal < float64(kmin) {
			// Suppressed row: retain preserveColumns, replace everything else
			newRow := make(map[string]any, len(row))
			for k, v := range row {
				if preserveSet[k] {
					newRow[k] = v
				} else {
					newRow[k] = "suppressed"
				}
			}
			out[i] = newRow
			suppressed++
		} else {
			// Unmodified row: shallow copy for snapshot immutability
			newRow := make(map[string]any, len(row))
			for k, v := range row {
				newRow[k] = v
			}
			out[i] = newRow
		}
	}
	return out, suppressed
}

// toFloat64 converts a numeric value from DuckDB scan to float64.
// Supported types: float64, int64, int, json.Number.
// Returns error for non-numeric or missing values (fail-closed).
func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int64:
		return float64(n), nil
	case int:
		return float64(n), nil
	case jsonNumber:
		return n.Float64()
	default:
		return 0, errNonNumericCount
	}
}
