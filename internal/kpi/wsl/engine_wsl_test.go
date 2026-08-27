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

// WSL-gated engine integration tests (16-06 Task 3).
//
// These tests are the AUTHORITATIVE leg of the executor↔engine contract:
// the hermetic tests under internal/kpi/executor_test.go lock the
// executor's policy (refusal shapes, snapshot field set, audit-write
// discipline) with stub engines; the tests below prove the END-TO-END
// behavior against a real DuckDB session, real AttachLRU, real staging→
// commit fixture, real DuckDBSQLSerializer for LoadKPIs' Layer 2 AST
// walk, and a real meta adapter that reads the sqlite registry.

package wsl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/data"
	"github.com/skillforge/skillforge/internal/duckdb"
	"github.com/skillforge/skillforge/internal/kpi"
	"github.com/skillforge/skillforge/internal/store"
)

// captureAudit records every KPICallRow (or fails with canned err).
// Mirrors the hermetic captureAudit in internal/kpi/executor_test.go so
// the WSL audit assertions read identically.
type captureAudit struct {
	rows []store.KPICallRow
	err  error
}

func (a *captureAudit) PutKPICall(_ context.Context, row store.KPICallRow) error {
	if a.err != nil {
		return a.err
	}
	a.rows = append(a.rows, row)
	return nil
}

// TestKPIEngineIntegration is the 16-06 WSL lock: real DuckDB execution +
// composite-alias substitution + k_min chain + suppression via the
// executor segment.
//
// Fixture:
//   - 12 rows of "正常", 3 rows of "逾期", 12 rows of "已结清"
//   - dataset k_min_override = 10
//   - three buckets: 12, 3, 12 → only "逾期" (count 3) is suppressed
//
// Assertions:
//   - nil optional status binding works → all three buckets returned
//   - KMinApplied == 10 (dataset override chain beats KMinDefault=5)
//   - suppressed row has overdue_rate="suppressed" but status/row_count
//     preserved (D16-D1 executor segment in the WSL authority leg)
//   - enum status filter binds to one bucket (sanity for $status binding)
//   - the composite alias substitution produced a working query (no
//     sentinel ErrAliasCountMismatch)
func TestKPIEngineIntegration(t *testing.T) {
	ctx := context.Background()

	ef := newEngineFixture(t, 4)

	// ---- fixture data ----
	rows := []loanRow{}
	for i := 0; i < 12; i++ {
		rows = append(rows, loanRow{Status: "正常", Balance: float64(1000 + i*100), OverdueDays: 0})
	}
	for i := 0; i < 3; i++ {
		rows = append(rows, loanRow{Status: "逾期", Balance: float64(5000 + i*100), OverdueDays: int64(10 + i)})
	}
	for i := 0; i < 12; i++ {
		rows = append(rows, loanRow{Status: "已结清", Balance: float64(800 + i*50), OverdueDays: 0})
	}

	staging := ef.writeLoansStaging(t, "loans__2026-07", rows)
	ef.commitLoansDataset(t, "loans", "2026-07", staging, 10)

	// ---- KPI YAML + registry ----
	kpiDir := t.TempDir()
	writeLoansKPIYAML(t, kpiDir, false /*status not required → nil binding exercised*/)
	reg := loadWSLRegistry(t, ef, kpiDir)

	// Sanity: the registry's CompiledKPI for the fixture KPI has
	// TableRefCount == 1 and PreserveColumns has status + row_count.
	// Note: PreserveColumns is the GROUP BY dimensions + the CountColumn
	// (per kmin.Apply D16-D1 contract); select-list value columns like
	// overdue_rate are SUPPRESSED for small buckets, not preserved.
	ck, ok := reg.Lookup("loan_overdue_rate_by_status")
	require.True(t, ok)
	assert.Equal(t, 1, ck.TableRefCount, "single BASE_TABLE reference, AST count must be 1")
	assert.Contains(t, ck.PreserveColumns, "status", "GROUP BY dimension must be preserved")
	assert.Contains(t, ck.PreserveColumns, "row_count", "CountColumn must be preserved")
	assert.NotContains(t, ck.PreserveColumns, "overdue_rate",
		"select-list value columns are SUPPRESSED, not preserved (kmin.Apply D16-D1)")

	// ---- executor ----
	audit := &captureAudit{}
	adapter := duckdb.NewEngineAdapter(ef.lru)
	meta := wsMeta{store: ef.store}
	fixedNow := time.Date(2026, 8, 27, 12, 30, 15, 250_000_000, time.UTC)
	exec := kpi.NewExecutor(reg, meta, adapter, audit, func() time.Time { return fixedNow })

	t.Run("nil_optional_status_returns_all_buckets", func(t *testing.T) {
		audit.rows = nil
		outcome, err := exec.Call(ctx, kpi.CallRequest{
			Name:      "loan_overdue_rate_by_status",
			SessionID: "sess-nil",
			SkillID:   "compliance",
		})
		require.NoError(t, err, "WSL nil-binding call must succeed")
		require.NotNil(t, outcome.Snapshot, "expected snapshot, got refusal %+v", outcome.Refusal)
		snap := outcome.Snapshot
		assert.Equal(t, "2026-07", snap.ASOf.Period, "period = registry watermark")
		assert.Equal(t, fixedNow.UTC().Format(time.RFC3339Nano), snap.ASOf.Ts)
		assert.Equal(t, 3, len(snap.Rows), "all three status buckets must be returned (nil binding)")

		// KMinApplied == 10 (dataset override chain).
		assert.Equal(t, 10, snap.KMinApplied,
			"k_min chain: file(0) > dataset override(10) > default(5); expect 10")

		// Suppressed bucket: only 逾期 (count=3 < 10).
		assert.Equal(t, 1, snap.Suppressed, "exactly one bucket (逾期) is below k_min")

		// Bucket-by-bucket assertions.
		buckets := map[string]map[string]any{}
		for _, r := range snap.Rows {
			buckets[r["status"].(string)] = r
		}
		require.Contains(t, buckets, "正常")
		require.Contains(t, buckets, "逾期")
		require.Contains(t, buckets, "已结清")

		// 正常 bucket: count=12 ≥ 10, all columns original.
		assert.EqualValues(t, 12, buckets["正常"]["row_count"])
		assert.NotEqual(t, "suppressed", buckets["正常"]["overdue_rate"], "healthy bucket keeps overdue_rate")

		// 逾期 bucket: count=3 < 10, value column suppressed, dimension + count preserved.
		assert.EqualValues(t, 3, buckets["逾期"]["row_count"], "small-bucket count PRESERVED")
		assert.Equal(t, "逾期", buckets["逾期"]["status"], "small-bucket status PRESERVED")
		assert.Equal(t, "suppressed", buckets["逾期"]["overdue_rate"],
			"small-bucket overdue_rate must be the suppression marker (D16-D1 executor segment WSL authority)")

		// 已结清 bucket: count=12 ≥ 10, all columns original.
		assert.EqualValues(t, 12, buckets["已结清"]["row_count"])
		assert.NotEqual(t, "suppressed", buckets["已结清"]["overdue_rate"])

		// Audit row: exactly one written, full freeze.
		require.Len(t, audit.rows, 1, "successful call must write exactly one audit row")
		assert.Equal(t, "loan_overdue_rate_by_status", audit.rows[0].Func)
		assert.Equal(t, 10, audit.rows[0].KMinApplied)
	})

	t.Run("enum_status_filter_binds", func(t *testing.T) {
		audit.rows = nil
		outcome, err := exec.Call(ctx, kpi.CallRequest{
			Name:      "loan_overdue_rate_by_status",
			Params:    map[string]any{"status": "逾期"},
			SessionID: "sess-enum",
			SkillID:   "compliance",
		})
		require.NoError(t, err)
		require.NotNil(t, outcome.Snapshot)
		assert.Equal(t, 1, len(outcome.Snapshot.Rows), "enum filter must narrow to one bucket")
		assert.Equal(t, "逾期", outcome.Snapshot.Rows[0]["status"])
		// 3 < 10 → still suppressed under KMinApplied=10.
		assert.Equal(t, "suppressed", outcome.Snapshot.Rows[0]["overdue_rate"])
		assert.EqualValues(t, 3, outcome.Snapshot.Rows[0]["row_count"])
	})
}

// TestNameLexiconParity locks two cross-package lexical contracts:
//
//  1. kpi package names (kpi.KPINamePattern = ^[A-Za-z][A-Za-z0-9_]{0,63}$)
//     and duckdb dataset ids (ValidateDatasetID = ^[A-Za-z0-9_-]{1,64}$)
//     agree on the SHARED grammar subset. Every string that passes the
//     stricter kpi grammar must pass duckdb's looser grammar; the
//     difference (hyphens) is what locks "kpi names forbid hyphens".
//
//  2. kpi.KMinDefault == data.KMinDefault — the mirror constant lands
//     equality across the CGO boundary (16-06 Task 1 godoc). Drift makes
//     this test red.
//
// Both contracts are ONLY observable from a package that can import all
// three (kpi, data, duckdb) — that's exactly the WSL subpackage's role.
func TestNameLexiconParity(t *testing.T) {
	t.Run("kpi_vs_duckdb_lexicon_agreement", func(t *testing.T) {
		cases := []struct {
			in     string
			kpiOK  bool
			dsOK   bool
			reason string
		}{
			// Shared grammar subset: pass BOTH.
			{"loans", true, true, "vanilla identifier"},
			{"overdue_rate_by_status", true, true, "snake_case"},
			{"a", true, true, "single letter"},
			{"A1_b2_C3", true, true, "mixed case + digits"},

			// kpi stricter: rejects (starts with digit) → rejects both.
			{"1loans", false, true, "starts with digit — kpi rejects, duckdb accepts"},

			// duckdb looser only: hyphen is legal in dataset id but not
			// in KPI name. The lexical gap that the engine adapter quotes
			// its way around (composite alias ds_loans__2026-07).
			{"loans-2026-07", false, true, "hyphen — kpi rejects, duckdb accepts"},

			// Rejected by both.
			{"", false, false, "empty"},
			{"loans x", false, false, "whitespace"},
			{"loans;DROP", false, false, "metacharacter"},
			{"loans\x00null", false, false, "NUL byte"},

			// Length boundary. The kpi regex ^[A-Za-z][A-Za-z0-9_]{0,63}$
			// (max 64 chars) and the duckdb regex ^[A-Za-z0-9_-]{1,64}$
			// (max 64 chars) both reject a 65-char input.
			{
				in:     "a234567890123456789012345678901234567890123456789012345678901234", // 64 chars (max accepted)
				kpiOK:  true,
				dsOK:   true,
				reason: "64 chars at the cap — both sides accept",
			},
			{
				in:     "a2345678901234567890123456789012345678901234567890123456789012345", // 65 chars over the cap
				kpiOK:  false,
				dsOK:   false,
				reason: "65 chars over the 64 cap on both sides",
			},
		}
		kpiName := kpi.KPINamePattern()
		require.NotNil(t, kpiName, "kpi.KPINamePattern must return the production regex")
		for _, c := range cases {
			gotKpi := kpiName.MatchString(c.in)
			gotDs := duckdb.ValidateDatasetID(c.in) == nil
			assert.Equal(t, c.kpiOK, gotKpi, "kpi.MatchString(%q): %s", c.in, c.reason)
			assert.Equal(t, c.dsOK, gotDs, "ValidateDatasetID(%q): %s", c.in, c.reason)
		}
	})

	t.Run("kmin_default_constant_parity", func(t *testing.T) {
		// Mirror constant must match across the CGO boundary. Drift
		// (either side changed without the other) is an immediate red.
		assert.Equal(t, kpi.KMinDefault, data.KMinDefault,
			"kpi.KMinDefault and data.KMinDefault must be byte-identical (CGO mirror constant — D16-D1)")
	})
}
