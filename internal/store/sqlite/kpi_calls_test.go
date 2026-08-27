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
