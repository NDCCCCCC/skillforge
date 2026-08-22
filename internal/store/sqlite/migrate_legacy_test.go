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

// UAT Test 2 blocker regression lock (Plan 14-09).
//
// The hermetic suite exercised migrate() exclusively through fresh
// :memory: databases, where schemaSQL creates the full audit table in
// one shot — the legacy upgrade path (pre-existing audit table that
// predates the Phase 12/13/14 columns, upgraded in place by ALTERs)
// had ZERO coverage. The UAT 2026-08-23 re-run proved the gap was
// load-bearing: schemaSQL embedded CREATE INDEX idx_audit_run_path
// AHEAD of the ALTER loop, so on the real repo-root skillforge.db
// (21-column audit table) the index referenced a column that only the
// ALTERs would add, and New() died with "no such column: run_path".
//
// These tests build a file-backed legacy database with the exact
// 21-column audit schema observed in the wild (ends at the four Ragas
// metric columns; no budget_layer / run_path / transfer_from /
// transfer_to / platform_floor_version) and drive New() against it.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyAuditDDL is the pre-Phase-12 audit table exactly as observed on
// the repo-root skillforge.db during UAT (2026-08-23): 21 columns,
// stopping at the four Ragas metric backfill columns. Types mirror the
// corresponding schemaSQL declarations.
const legacyAuditDDL = `
CREATE TABLE audit (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    ts               DATETIME NOT NULL,
    session_id       TEXT    NOT NULL,
    user_id          TEXT,
    skill_id         TEXT    NOT NULL,
    query            TEXT    NOT NULL,
    prompt_hash      TEXT    NOT NULL,
    model_version    TEXT    NOT NULL,
    verdict_json     TEXT    NOT NULL,
    refusal_reason   TEXT    NOT NULL DEFAULT '',
    chunk_ids        TEXT    NOT NULL DEFAULT '',
    action           TEXT    NOT NULL DEFAULT '',
    skill_version    TEXT    NOT NULL DEFAULT '',
    frozen_args      BLOB,
    tool_call_id     TEXT    NOT NULL DEFAULT '',
    approval_id      TEXT    NOT NULL DEFAULT '',
    scope_session_id TEXT    NOT NULL DEFAULT '',
    faithfulness     REAL,
    context_precision REAL,
    answer_relevance  REAL,
    context_recall    REAL
)
`

// seedLegacyAuditDB creates a file-backed database holding ONLY the
// legacy 21-column audit table plus one surviving row — the minimal
// reproduction of a pre-Phase-12 deployment's store file.
func seedLegacyAuditDB(t *testing.T, dsn string) {
	t.Helper()
	raw, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err, "raw sqlite3 open for legacy seeding should succeed")
	_, err = raw.Exec(legacyAuditDDL)
	require.NoError(t, err, "legacy 21-column audit DDL should apply")
	_, err = raw.Exec(`INSERT INTO audit (id, ts, session_id, skill_id, query, prompt_hash, model_version, verdict_json)
VALUES (1, '2026-08-01T10:00:00Z', 'sess-legacy', 'compliance', '旧库行', 'legacy', 'glm-5.2', '{}')`)
	require.NoError(t, err, "legacy row insert should succeed")
	require.NoError(t, raw.Close(), "raw seeding connection should close cleanly")
}

// TestMigrateLegacyAuditUpgradePath locks the UAT Test 2 fix: a
// deployment holding the pre-Phase-12 audit schema must upgrade via
// New() — not die at boot — and the migration must be additive (index
// present, all five newer columns added, the legacy row intact).
func TestMigrateLegacyAuditUpgradePath(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy-upgrade.db")
	seedLegacyAuditDB(t, dsn)

	s, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "legacy 21-column audit DB must migrate via New() (was: no such column: run_path)")
	defer func() { _ = s.Close() }()

	// idx_audit_run_path must exist once the ALTERs have added run_path.
	var idxCount int
	require.NoError(t,
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_audit_run_path'`).Scan(&idxCount))
	require.Equal(t, 1, idxCount, "idx_audit_run_path must exist after legacy migration")

	// Every post-legacy column must have been added by the ALTER loop.
	for _, col := range []string{
		"budget_layer", "run_path", "transfer_from", "transfer_to", "platform_floor_version",
	} {
		exists, err := s.columnExists(ctx, "audit", col)
		require.NoError(t, err, "columnExists(%q) should not error", col)
		require.True(t, exists, "audit.%s must be present after legacy migration", col)
	}

	// Additive migration: the pre-existing row survives untouched.
	var count int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&count))
	require.Equal(t, 1, count, "legacy audit row must survive migration (additive, T-13-03-7)")
	var query string
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT query FROM audit WHERE id = 1`).Scan(&query))
	assert.Equal(t, "旧库行", query, "legacy row content must round-trip byte-for-byte")
}

// TestMigrateFreshFileDBRunPathIdx guards the fresh-database direction:
// a brand-new file DB must still carry idx_audit_run_path after the
// index creation moved out of schemaSQL.
func TestMigrateFreshFileDBRunPathIdx(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "fresh-runpath.db")

	s, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "fresh file DB must open via New()")
	defer func() { _ = s.Close() }()

	var idxCount int
	require.NoError(t,
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_audit_run_path'`).Scan(&idxCount))
	require.Equal(t, 1, idxCount, "fresh file DB must carry idx_audit_run_path")
}

// TestMigrateLegacyIdempotentReopen guards the reopen direction:
// migrate() runs on every New(), so a once-migrated legacy DB must open
// again cleanly (server restart on the upgraded file).
func TestMigrateLegacyIdempotentReopen(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy-reopen.db")
	seedLegacyAuditDB(t, dsn)

	first, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "first New() on legacy DB must migrate")
	require.NoError(t, first.Close())

	second, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "second New() on the migrated DB must succeed (idempotent migrate)")
	defer func() { _ = second.Close() }()

	var count int
	require.NoError(t, second.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&count))
	require.Equal(t, 1, count, "legacy row must still be intact after reopen")
}
