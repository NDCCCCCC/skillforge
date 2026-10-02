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

package sqlite

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// TestReportsPeriodVersionUnique — WR-09 (18-REVIEW): the vN race
// backstop. Two inserts with the same (period, version) must reject
// the second at the DB boundary — the previous build had no UNIQUE
// index and the race silently overwrote the first row's vN HTML.
func TestReportsPeriodVersionUnique(t *testing.T) {
	ctx := context.Background()
	st, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = st.Close() }()

	row := store.ReportRow{
		Period:   "2026-08",
		SkillID:  "monthly-report",
		Version:  1,
		Title:    "2026-08 业务月报",
		FilePath: "reports/2026-08/v1.html",
	}
	require.NoError(t, st.PutReport(ctx, row))

	dup := row
	require.Error(t, st.PutReport(ctx, dup), "duplicate (period, version) must be rejected by the UNIQUE index")
}

// TestReportsPeriodVersionAllowsDifferentVersion — the UNIQUE index
// must NOT block legitimate vN increments on the same period (v1 +
// v2 coexist for the same period; the index is (period, version), not
// period alone).
func TestReportsPeriodVersionAllowsDifferentVersion(t *testing.T) {
	ctx := context.Background()
	st, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = st.Close() }()

	v1 := store.ReportRow{Period: "2026-08", SkillID: "monthly-report", Version: 1, FilePath: "p/v1.html"}
	v2 := v1
	v2.Version = 2
	v2.FilePath = "p/v2.html"
	require.NoError(t, st.PutReport(ctx, v1))
	require.NoError(t, st.PutReport(ctx, v2),
		"different version on the same period must be allowed")

	got, err := st.ListReports(ctx, "2026-08", 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

// TestReportsIndexPresentAfterMigrate — the migrate() tail must
// register the UNIQUE index so a fresh database carries it from the
// start (no separate ALTER pass needed).
func TestReportsIndexPresentAfterMigrate(t *testing.T) {
	ctx := context.Background()
	st, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = st.Close() }()

	// sqlite_master query (UNIQUE index name); presence is the assertion.
	rows, err := st.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_reports_period_version'")
	require.NoError(t, err)
	defer rows.Close()
	var found []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		found = append(found, n)
	}
	require.NoError(t, rows.Err())
	assert.True(t, strings.Contains(strings.Join(found, ","), "idx_reports_period_version"),
		"migrate() must register the WR-09 UNIQUE index; saw %v", found)
}