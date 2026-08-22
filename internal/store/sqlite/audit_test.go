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

// Package sqlite_test verifies the PLAT-10 audit-floor column behavior
// against the real SQLite implementation.
package sqlite_test

import (
	"context"
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFloorTestStore opens an isolated file-backed SQLiteStore for tests
// that need to issue raw SQL alongside the Store interface.
func newFloorTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	dsn := t.TempDir() + "/floor.db"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := sqlite.New(ctx, sqlite.Config{DSN: dsn})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestAuditPlatformFloorVersionColumn — PLAT-10 D14-B2:
// every audit row carries the platform-floor semver, and a SQL DISTINCT
// query over the column exposes the drift window (pre-floor empty string
// vs current "1.0.0").
func TestAuditPlatformFloorVersionColumn(t *testing.T) {
	ctx := context.Background()
	s := newFloorTestStore(t)

	const floor = "1.0.0"

	// PutAudit row with the floor version.
	row := store.AuditRow{
		Timestamp:            time.Now().UTC(),
		SessionID:            "sess-floor",
		UserID:               "u1",
		SkillID:              "compliance",
		Query:                "q1",
		PromptHash:           "h1",
		ModelVersion:         "glm-5.2",
		VerdictJSON:          `{}`,
		PlatformFloorVersion: floor,
	}
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// QueryAudit round-trips the new column.
	rows, err := s.QueryAudit(ctx, store.AuditFilter{SessionID: "sess-floor"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, floor, rows[0].PlatformFloorVersion,
		"QueryAudit must return the platform_floor_version written by PutAudit")

	// Raw SQL DISTINCT exposes the current floor value.
	assertDistinct(t, s, []string{floor})

	// Simulate a legacy pre-floor row with an empty string.
	legacy := store.AuditRow{
		Timestamp:            time.Now().UTC().Add(-time.Hour),
		SessionID:            "sess-legacy",
		UserID:               "u1",
		SkillID:              "compliance",
		Query:                "q-legacy",
		PromptHash:           "h-legacy",
		ModelVersion:         "glm-5.2",
		VerdictJSON:          `{}`,
		PlatformFloorVersion: "",
	}
	_, err = s.PutAudit(ctx, legacy)
	require.NoError(t, err)

	// DISTINCT now returns both the pre-floor era and the current floor.
	assertDistinct(t, s, []string{"", floor})
}

// assertDistinct queries `SELECT DISTINCT platform_floor_version FROM audit`
// and asserts the result matches want (order-independent).
func assertDistinct(t *testing.T, s *sqlite.SQLiteStore, want []string) {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(),
		"SELECT DISTINCT platform_floor_version FROM audit")
	require.NoError(t, err)
	defer rows.Close()

	var got []string
	for rows.Next() {
		var v sql.NullString
		require.NoError(t, rows.Scan(&v))
		got = append(got, v.String)
	}
	require.NoError(t, rows.Err())

	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got,
		"SELECT DISTINCT platform_floor_version must expose the drift window")
}
