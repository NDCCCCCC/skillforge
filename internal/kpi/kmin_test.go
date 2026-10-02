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
	"reflect"
	"testing"
)

// TestKMinRowSuppression locks the D16-D1 row-level suppression contract.
//
// PITFALL 4 WARNING (16-RESEARCH): if any assertion below expects FEWER rows
// in the output than the input, the implementation has the SQL HAVING bug —
// the correct behavior is ROW RETENTION with value replacement ("suppressed"),
// never row removal and never whole-call refusal. The output row count MUST
// equal the input row count in every case.
func TestKMinRowSuppression(t *testing.T) {
	// loans-shaped fixture: status (GROUP BY dimension) + row_count
	// (aggregate count column) + overdue_rate (value column).
	//
	// 7 rows total; the "逾期" rows have row_count < 10 → suppressed;
	// the "正常" rows have row_count >= 10 → returned unmodified.
	input := []map[string]any{
		{"status": "正常", "row_count": int64(120), "overdue_rate": 3.5},
		{"status": "正常", "row_count": int64(88), "overdue_rate": 1.2},
		{"status": "逾期", "row_count": int64(7), "overdue_rate": 42.86},
		{"status": "正常", "row_count": int64(64), "overdue_rate": 0.9},
		{"status": "逾期", "row_count": int64(4), "overdue_rate": 55.1},
		{"status": "正常", "row_count": int64(15), "overdue_rate": 2.75},
		{"status": "正常", "row_count": int64(301), "overdue_rate": 0.3},
	}
	kmin := 10

	out, suppressed := Apply(input, "row_count", []string{"status"}, kmin)

	// LOCK ①a: output row count == input row count (anti-HAVING).
	if len(out) != len(input) {
		t.Fatalf("Pitfall 4 violation: output has %d rows, input has %d — rows must be RETAINED, not removed (output must not shrink: fewer rows = HAVING bug)",
			len(out), len(input))
	}
	// LOCK ①b: exactly the two small buckets were suppressed.
	if suppressed != 2 {
		t.Fatalf("suppressed = %d, want 2", suppressed)
	}

	// Suppressed rows: status + row_count keep ORIGINAL values,
	// overdue_rate is replaced with the literal string "suppressed".
	if got := out[2]["status"]; got != "逾期" {
		t.Errorf("suppressed row 2 status = %v, want original 逾期", got)
	}
	if got := out[2]["row_count"]; got != int64(7) {
		t.Errorf("suppressed row 2 row_count = %v (%T), want original int64(7)", got, got)
	}
	if got := out[2]["overdue_rate"]; got != "suppressed" {
		t.Errorf("suppressed row 2 overdue_rate = %v, want \"suppressed\"", got)
	}
	if got := out[4]["status"]; got != "逾期" {
		t.Errorf("suppressed row 4 status = %v, want original 逾期", got)
	}
	if got := out[4]["row_count"]; got != int64(4) {
		t.Errorf("suppressed row 4 row_count = %v (%T), want original int64(4)", got, got)
	}
	if got := out[4]["overdue_rate"]; got != "suppressed" {
		t.Errorf("suppressed row 4 overdue_rate = %v, want \"suppressed\"", got)
	}

	// Normal rows: all three columns pass through byte-identically.
	for _, i := range []int{0, 1, 3, 5, 6} {
		want := input[i]
		got := out[i]
		for _, col := range []string{"status", "row_count", "overdue_rate"} {
			if got[col] != want[col] {
				t.Errorf("normal row %d col %s = %v, want %v (unmodified rows must be returned unchanged)", i, col, got[col], want[col])
			}
		}
	}

	// LOCK ⑥: input slice NOT mutated (snapshot immutability discipline —
	// callers freeze the OUTPUT; the input must stay untouched for retry/audit).
	for i, row := range input {
		if row["status"] == "逾期" && row["overdue_rate"] == "suppressed" {
			t.Errorf("input row %d was mutated in place — Apply must copy, never mutate (caller snapshot immutability)", i)
		}
	}
	if input[2]["overdue_rate"] != 42.86 {
		t.Errorf("input row 2 overdue_rate = %v, want original 42.86 (no in-place mutation)", input[2]["overdue_rate"])
	}
}

// TestKMinAllRowsSuppressed locks case ③: when every bucket is below kmin,
// ALL value columns become "suppressed" but NO row disappears and the call
// is still a success (whole-result refusal would violate D16-D1's
// "行保留、非整体拒答" contract — suppression is per-row, not per-KPI).
func TestKMinAllRowsSuppressed(t *testing.T) {
	input := []map[string]any{
		{"status": "逾期", "row_count": int64(3), "overdue_rate": 60.0},
		{"status": "核销", "row_count": int64(1), "overdue_rate": 100.0},
		{"status": "展期", "row_count": int64(2), "overdue_rate": 85.5},
	}

	out, suppressed := Apply(input, "row_count", []string{"status"}, 5)

	if len(out) != len(input) {
		t.Fatalf("Pitfall 4 violation: all-small-buckets output has %d rows, want %d (rows NEVER shrink)", len(out), len(input))
	}
	if suppressed != len(input) {
		t.Fatalf("suppressed = %d, want %d (every row below kmin)", suppressed, len(input))
	}
	for i, row := range out {
		if row["status"] != input[i]["status"] {
			t.Errorf("row %d status = %v, want dimension column preserved", i, row["status"])
		}
		if row["row_count"] != input[i]["row_count"] {
			t.Errorf("row %d row_count = %v, want count column preserved", i, row["row_count"])
		}
		if row["overdue_rate"] != "suppressed" {
			t.Errorf("row %d overdue_rate = %v, want \"suppressed\" (all rows below kmin)", i, row["overdue_rate"])
		}
	}
}

// TestKMinMissingCountFailClosed locks case ④: a row whose count column is
// missing or non-numeric CANNOT prove membership >= kmin, so it is treated
// as suppressed (fail-closed orientation — cannot-prove-safe means suppressed).
func TestKMinMissingCountFailClosed(t *testing.T) {
	input := []map[string]any{
		{"status": "正常", "row_count": int64(50), "overdue_rate": 1.1}, // healthy control row
		{"status": "缺失计数", "overdue_rate": 9.9},                     // row_count key ABSENT
		{"status": "字符串计数", "row_count": "many", "overdue_rate": 8.8}, // non-numeric count
	}

	out, suppressed := Apply(input, "row_count", []string{"status"}, 5)

	if len(out) != len(input) {
		t.Fatalf("fail-closed case: output rows = %d, want %d (suppression replaces values, never removes rows)", len(out), len(input))
	}
	if suppressed != 2 {
		t.Fatalf("suppressed = %d, want 2 (missing-count AND non-numeric-count rows both fail closed)", suppressed)
	}
	for _, i := range []int{1, 2} {
		if got := out[i]["overdue_rate"]; got != "suppressed" {
			t.Errorf("unprovable row %d overdue_rate = %v, want \"suppressed\" (fail-closed: cannot prove >= kmin → suppress)", i, got)
		}
		if got := out[i]["status"]; got != input[i]["status"] {
			t.Errorf("unprovable row %d status = %v, want preserved", i, got)
		}
	}
	if out[0]["overdue_rate"] != 1.1 {
		t.Errorf("healthy control row was wrongly touched: overdue_rate = %v", out[0]["overdue_rate"])
	}
}

// TestKMinBelowFloorZeroSuppression locks case ⑤: kmin==1 lets every row
// through untouched (any real count >= 1 passes a k of 1 — zero suppression),
// proving the comparison is >= kmin rather than some off-by-one floor.
func TestKMinBelowFloorZeroSuppression(t *testing.T) {
	input := []map[string]any{
		{"status": "正常", "row_count": int64(1), "overdue_rate": 12.5},
		{"status": "逾期", "row_count": int64(2), "overdue_rate": 50.0},
	}

	out, suppressed := Apply(input, "row_count", []string{"status"}, 1)

	if suppressed != 0 {
		t.Fatalf("kmin=1: suppressed = %d, want 0 (count >= 1 always passes k=1)", suppressed)
	}
	if !reflect.DeepEqual(out, input) {
		t.Errorf("kmin=1: output should equal input (deep-equal), got %+v want %+v", out, input)
	}
}

// TestApplyCountColumnImplicitlyPreserved pins the contract nuance: even when
// the caller forgets to list countColumn in preserveColumns, Apply preserves
// it (the suppressed row's bucket size is what makes suppression auditable).
func TestApplyCountColumnImplicitlyPreserved(t *testing.T) {
	input := []map[string]any{
		{"bucket": "tiny", "n": int64(2), "avg_balance": 999.99},
	}
	out, suppressed := Apply(input, "n", nil, 5)
	if suppressed != 1 || len(out) != 1 {
		t.Fatalf("unexpected outcome: suppressed=%d rows=%d", suppressed, len(out))
	}
	if got := out[0]["n"]; got != int64(2) {
		t.Errorf("count column n = %v, want implicitly preserved int64(2)", got)
	}
	if got := out[0]["avg_balance"]; got != "suppressed" {
		t.Errorf("value column avg_balance = %v, want \"suppressed\"", got)
	}
}

// TestResolveKMin locks the three-tier chain (D15-D3 / 16-PATTERNS §7):
// file k_min > 0 wins; else dataset k_min_override > 0 wins; else the global
// KMinDefault. A negative override behaves identically to unset (same as 0).
func TestResolveKMin(t *testing.T) {
	cases := []struct {
		name            string
		fileKMin        int
		datasetOverride int
		want            int
	}{
		{name: "both unset -> global default", fileKMin: 0, datasetOverride: 0, want: KMinDefault},
		{name: "override set, file unset -> override", fileKMin: 0, datasetOverride: 10, want: 10},
		{name: "file set wins over override", fileKMin: 3, datasetOverride: 10, want: 3},
		{name: "negative override treated as unset", fileKMin: 0, datasetOverride: -1, want: KMinDefault},
		{name: "negative file k_min treated as unset", fileKMin: -7, datasetOverride: 12, want: 12},
		{name: "both negative -> global default", fileKMin: -7, datasetOverride: -1, want: KMinDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveKMin(tc.fileKMin, tc.datasetOverride)
			if got != tc.want {
				t.Errorf("ResolveKMin(%d, %d) = %d, want %d", tc.fileKMin, tc.datasetOverride, got, tc.want)
			}
		})
	}
}
