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

// Phase 16 / Plan 16-03 — kpi_calls INSERT-only audit tests. Lives in
// internal/store/sqlite (white-box) because it asserts the exact DDL shape,
// column order, index set, and additive-migration behavior of the table,
// plus (Task 2) a source-grep structural lock on the audit file itself.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// expectedKPICallColumns mirrors the CREATE TABLE DDL in schema.go —
// TestKPICallsDDL asserts PRAGMA table_info returns EXACTLY this name
// sequence in this order (column-order drift would silently break every
// positional Scan in kpi_calls.go).
var expectedKPICallColumns = []string{
	"id", "ts", "session_id", "skill_id", "func",
	"args_json", "as_of_period", "k_min_applied", "result_snapshot", "signature",
}

// fullKPICallRow returns a fully-populated KPICallRow whose ResultSnapshot
// matches the executor's real shape: all rows INCLUDING suppressed-marker
// rows (D16-D1 row-retention + D16-D3 full-freeze).
func fullKPICallRow(ts time.Time) store.KPICallRow {
	return store.KPICallRow{
		TS:          ts,
		SessionID:   "sess-kpi-1",
		SkillID:     "compliance",
		Func:        "overdue_by_region",
		ArgsJSON:    `{"period":"2026-07","region":"华东"}`,
		ASOfPeriod:  "2026-07",
		KMinApplied: 5,
		ResultSnapshot: `[{"region":"华东","row_count":42,"overdue_rate":0.187},` +
			`{"region":"零星区","row_count":3,"value":"suppressed"}]`,
		Signature: "d16a4c0ffee",
	}
}

// TestKPICallsDDL locks the kpi_calls schema contract three ways:
//
//  1. A fresh file-backed DB carries exactly the 10 columns in DDL order
//     plus the three admin read-path indexes (func / session_id / ts).
//  2. migrate() is idempotent — re-running it on the migrated DB is a no-op.
//  3. The legacy upgrade path: a pre-Phase-16 deployment (datasets registry
//     WITHOUT any kpi_calls table) gains the table via New() additively and
//     its existing rows survive untouched.
func TestKPICallsDDL(t *testing.T) {
	ctx := context.Background()

	// --- Act 1: fresh file-backed DB ---
	s, err := New(ctx, Config{DSN: filepath.Join(t.TempDir(), "kpi-calls-ddl.db")})
	require.NoError(t, err, "fresh file DB must open via New()")

	var gotNames []string
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(kpi_calls)`)
	require.NoError(t, err)
	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notnull, pk  int
			defaultValue sql.NullString
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &defaultValue, &pk))
		gotNames = append(gotNames, name)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, expectedKPICallColumns, gotNames,
		"kpi_calls must carry exactly the 10 DDL columns in DDL order")

	for _, idx := range []string{
		"idx_kpi_calls_func",
		"idx_kpi_calls_session_id",
		"idx_kpi_calls_ts",
	} {
		var idxCount int
		require.NoError(t, s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
		assert.Equal(t, 1, idxCount, "%s must exist on a fresh DB", idx)
	}

	// --- Act 2: migrate() idempotency on the already-migrated DB ---
	require.NoError(t, s.migrate(ctx), "re-running migrate must be a no-op")
	require.NoError(t, s.Close())

	// --- Act 3: legacy upgrade path (pre-Phase-16 DB without kpi_calls) ---
	dsn := filepath.Join(t.TempDir(), "legacy-kpi-upgrade.db")
	seedLegacyDatasetsDB(t, dsn)

	legacy, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "pre-Phase-16 DB must upgrade via New() (additive kpi_calls)")
	defer func() { _ = legacy.Close() }()

	var colCount int
	require.NoError(t, legacy.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('kpi_calls')`).Scan(&colCount))
	assert.Equal(t, len(expectedKPICallColumns), colCount,
		"kpi_calls must be created with all 10 columns on the upgraded legacy DB")

	for _, idx := range []string{"idx_kpi_calls_func", "idx_kpi_calls_session_id", "idx_kpi_calls_ts"} {
		var idxCount int
		require.NoError(t, legacy.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
		assert.Equal(t, 1, idxCount, "%s must exist after legacy migration", idx)
	}

	// Additive discipline: the pre-existing datasets rows survive untouched.
	var dsCount int
	require.NoError(t, legacy.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datasets`).Scan(&dsCount))
	assert.Equal(t, 2, dsCount, "legacy datasets rows must survive the kpi_calls migration")
}

// TestKPICallsInsertOnly locks the kpi_calls anti-tampering contract in two
// segments (T-16-03-01 + T-16-03-02, mirroring the import_batches audit
// posture):
//
//  1. source-grep: the audit file itself must contain no row-mutation or
//     row-removal statement — a structural lock that survives future edits
//     better than any behavioral test (a mutation verb cannot sneak in
//     without tripping this assertion).
//  2. round-trip: three frozen calls written via PutKPICall read back
//     field-for-field through the read-only query face — zero-filter list,
//     per-func filter, per-session filter, combined filter, and the
//     ADM-03 "last call time" aggregation.
func TestKPICallsInsertOnly(t *testing.T) {
	ctx := context.Background()

	// --- Segment 1: source-grep structural lock ---
	src, err := os.ReadFile("kpi_calls.go")
	require.NoError(t, err, "kpi_calls.go must be readable from the package dir")
	source := string(src)
	assert.NotContains(t, source, "UPDATE ",
		"kpi_calls.go must contain no row-mutation verb (T-16-03-01)")
	assert.NotContains(t, source, "DELETE ",
		"kpi_calls.go must contain no row-removal verb (T-16-03-01)")

	// --- Segment 2: round-trip through the read-only query face ---
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Three frozen calls: two share func, two share session. Timestamps
	// carry sub-second precision to prove RFC3339Nano round-trip fidelity.
	r1 := fullKPICallRow(time.Date(2026, 8, 27, 10, 0, 1, 500000000, time.UTC))
	r1.SessionID = "sess-a"
	r2 := fullKPICallRow(time.Date(2026, 8, 27, 10, 0, 2, 250000000, time.UTC))
	r2.SessionID = "sess-b"
	r3 := fullKPICallRow(time.Date(2026, 8, 27, 10, 0, 3, 123456789, time.UTC))
	r3.SessionID = "sess-b"
	r3.Func = "approval_rate_by_month"
	for _, r := range []store.KPICallRow{r1, r2, r3} {
		require.NoError(t, s.PutKPICall(ctx, r))
	}

	assertRowsEqual := func(t *testing.T, want store.KPICallRow, got store.KPICallRow) {
		t.Helper()
		assert.Equal(t, want.SessionID, got.SessionID)
		assert.Equal(t, want.SkillID, got.SkillID)
		assert.Equal(t, want.Func, got.Func)
		assert.Equal(t, want.ArgsJSON, got.ArgsJSON, "ArgsJSON must round-trip byte-equal")
		assert.Equal(t, want.ResultSnapshot, got.ResultSnapshot,
			"ResultSnapshot must round-trip byte-equal (full-freeze D16-D3)")
		assert.Equal(t, want.ASOfPeriod, got.ASOfPeriod)
		assert.Equal(t, want.KMinApplied, got.KMinApplied)
		assert.Equal(t, want.Signature, got.Signature)
		assert.True(t, want.TS.Equal(got.TS), "TS must round-trip with nanosecond fidelity: want %v got %v", want.TS, got.TS)
		assert.Greater(t, got.ID, int64(0), "auto-increment ID must be populated on reads")
	}

	// Zero-value filter: all 3 rows, newest-first by insertion id.
	all, err := s.ListKPICalls(ctx, store.KPICallsFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3)
	assertRowsEqual(t, r3, all[0])
	assertRowsEqual(t, r2, all[1])
	assertRowsEqual(t, r1, all[2])

	// Filter by Func: only that KPI's rows, still newest-first.
	byFunc, err := s.ListKPICalls(ctx, store.KPICallsFilter{Func: "overdue_by_region"})
	require.NoError(t, err)
	require.Len(t, byFunc, 2)
	assertRowsEqual(t, r2, byFunc[0])
	assertRowsEqual(t, r1, byFunc[1])

	missing, err := s.ListKPICalls(ctx, store.KPICallsFilter{Func: "no_such_kpi"})
	require.NoError(t, err)
	assert.Empty(t, missing)

	// Filter by SessionID.
	bySession, err := s.ListKPICalls(ctx, store.KPICallsFilter{SessionID: "sess-b"})
	require.NoError(t, err)
	require.Len(t, bySession, 2)
	assertRowsEqual(t, r3, bySession[0])
	assertRowsEqual(t, r2, bySession[1])

	// Combined filters AND together.
	combined, err := s.ListKPICalls(ctx, store.KPICallsFilter{
		Func:      "overdue_by_region",
		SessionID: "sess-b",
	})
	require.NoError(t, err)
	require.Len(t, combined, 1)
	assertRowsEqual(t, r2, combined[0])

	// LatestKPICallByFunc: one aggregation over the table returns each
	// func's most-recent call timestamp (ADM-03 substrate).
	latest, err := s.LatestKPICallByFunc(ctx)
	require.NoError(t, err)
	require.Len(t, latest, 2)
	assert.True(t, latest["overdue_by_region"].Equal(r2.TS),
		"overdue_by_region last call = r2 (%v), got %v", r2.TS, latest["overdue_by_region"])
	assert.True(t, latest["approval_rate_by_month"].Equal(r3.TS),
		"approval_rate_by_month last call = r3 (%v), got %v", r3.TS, latest["approval_rate_by_month"])

	// Empty table → empty map.
	fresh, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = fresh.Close() }()
	emptyLatest, err := fresh.LatestKPICallByFunc(ctx)
	require.NoError(t, err)
	assert.NotNil(t, emptyLatest)
	assert.Empty(t, emptyLatest)
}

// ---------------------------------------------------------------------------
// Test helpers for multi-period KPI integration tests (Phase 17 / Plan 17-07)
// ---------------------------------------------------------------------------

// testStoreForKPI is a shared test helper that creates a temp-file-backed
// SQLite store so that multiple helper calls (InsertKPICall, CountKPICalls,
// LoadKPICall) share the same persistent database.
func testStoreForKPI(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "kpi-calls-test.db")
	s, err := New(context.Background(), Config{DSN: dbPath})
	require.NoError(t, err)
	cleanup := func() { _ = s.Close() }
	return s, cleanup
}

// InsertKPICall is a test-helper that inserts one kpi_call row into a
// shared temp-file store and returns the auto-assigned ID.
func InsertKPICall(t *testing.T, s *SQLiteStore, argsJSON, resultSnapshot string) int64 {
	t.Helper()
	ctx := context.Background()
	ts := time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC)
	row := store.KPICallRow{
		TS:             ts,
		SessionID:      "sess-multi-period",
		SkillID:        "kpi-dashboard",
		Func:           "loan_balance",
		ArgsJSON:       argsJSON,
		ASOfPeriod:     "2026-08",
		KMinApplied:    5,
		ResultSnapshot: resultSnapshot,
		Signature:      "p17-multi-period",
	}
	err := s.PutKPICall(ctx, row)
	require.NoError(t, err)

	rows, err := s.ListKPICalls(ctx, store.KPICallsFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	return rows[0].ID
}

// CountKPICalls returns the total number of kpi_calls rows in the given store.
func CountKPICalls(t *testing.T, s *SQLiteStore) int {
	t.Helper()
	ctx := context.Background()
	all, err := s.ListKPICalls(ctx, store.KPICallsFilter{Limit: 10000})
	require.NoError(t, err)
	return len(all)
}

// LoadKPICall loads a single kpi_calls row by ID from the given store.
func LoadKPICall(t *testing.T, s *SQLiteStore, id int64) store.KPICallRow {
	t.Helper()
	ctx := context.Background()
	all, err := s.ListKPICalls(ctx, store.KPICallsFilter{Limit: 100})
	require.NoError(t, err)
	for _, r := range all {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("LoadKPICall: row with ID %d not found", id)
	return store.KPICallRow{}
}

// TestMultiPeriodKPIPeriodsArray verifies that a KPI call with a periods
// array argument produces one audit row whose result_snapshot contains
// exactly N result rows (one per period). This locks the multi-period
// call → N-row snapshot → 1 kpi_calls audit row invariant
// (Phase 17 / Plan 17-07 Task 2).
func TestMultiPeriodKPIPeriodsArray(t *testing.T) {
	s, cleanup := testStoreForKPI(t)
	defer cleanup()

	// Insert a kpi_call with periods:["2026-05","2026-06","2026-07"].
	args := `{"periods":["2026-05","2026-06","2026-07"]}`
	result := `{"rows":[{"period":"2026-05","balance":42315000000},{"period":"2026-06","balance":42100000000},{"period":"2026-07","balance":41950000000}],"as_of":"2026-08-01T00:00:00Z"}`
	callID := InsertKPICall(t, s, args, result)

	// Verify audit row count: exactly 1.
	count := CountKPICalls(t, s)
	assert.Equal(t, 1, count)

	// Verify snapshot contains 3 rows.
	snap := LoadKPICall(t, s, callID)
	var snapData struct {
		Rows []map[string]any `json:"rows"`
	}
	err := json.Unmarshal([]byte(snap.ResultSnapshot), &snapData)
	require.NoError(t, err)
	assert.Len(t, snapData.Rows, 3)
}
