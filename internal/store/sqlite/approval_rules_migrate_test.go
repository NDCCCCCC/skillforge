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

// Phase 56 / Plan 56-01 / D-56-01 — the approval_rules legacy rebuild
// regression locks.
//
// The project's FIRST table-rebuild migration (PRAGMA-gated ALTER
// cannot rewrite a PRIMARY KEY). Mirrors the migrate_legacy_test.go
// file-backed pattern and guards the same UAT 14-09 landmine in its
// 56-01 form: the new-shape approval_rules indexes live at the
// migrate() tail AFTER the rebuild — inlined into schemaSQL they would
// reference match_type/scope/args_pattern before the rebuild introduces
// them, and New() would die with "no such column: match_type" on every
// pre-56 deployment.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyApprovalRulesDDL is the pre-56 approval_rules table exactly as
// Plan 11-05 shipped it: the four-tuple PK, binary scope encoded in
// scope_session_id (” vs non-empty), and the two legacy indexes.
const legacyApprovalRulesDDL = `
CREATE TABLE approval_rules (
    target            TEXT NOT NULL,
    args_hash         TEXT NOT NULL,
    skill_id          TEXT NOT NULL,
    scope_session_id  TEXT NOT NULL DEFAULT '',
    created_at        INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    last_used_at      INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    PRIMARY KEY (target, args_hash, skill_id, scope_session_id)
);
CREATE INDEX IF NOT EXISTS idx_approval_rules_scope_session ON approval_rules(scope_session_id);
CREATE INDEX IF NOT EXISTS idx_approval_rules_skill ON approval_rules(skill_id);
`

// Fixed unix seconds so the legacy last_used_at → last_hit_at mapping
// is assertable exactly (the rebuild copies the value, not the clock).
const (
	legacySessCreatedAt  = int64(1700000000)
	legacySessLastUsed   = int64(1700000100)
	legacySkillCreatedAt = int64(1700000200)
	legacySkillLastUsed  = int64(1700000300)
)

// seedLegacyApprovalRulesDB creates a file-backed database holding ONLY
// the legacy approval_rules table plus two surviving rows — one
// session-scope, one skill-scope — the minimal reproduction of a
// pre-56 deployment's rule table.
func seedLegacyApprovalRulesDB(t *testing.T, dsn string) {
	t.Helper()
	raw, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err, "raw sqlite3 open for legacy seeding should succeed")
	_, err = raw.Exec(legacyApprovalRulesDDL)
	require.NoError(t, err, "legacy four-tuple-PK approval_rules DDL should apply")
	_, err = raw.Exec(`INSERT INTO approval_rules (target, args_hash, skill_id, scope_session_id, created_at, last_used_at)
VALUES
  ('memory:facts', 'legacy-hash-sess', 'compliance', 'sess-legacy', ?, ?),
  ('memory:facts', 'legacy-hash-skill', 'compliance', '', ?, ?)`,
		legacySessCreatedAt, legacySessLastUsed,
		legacySkillCreatedAt, legacySkillLastUsed)
	require.NoError(t, err, "legacy rule rows insert should succeed")
	require.NoError(t, raw.Close(), "raw seeding connection should close cleanly")
}

// TestMigrateLegacyApprovalRulesRebuild locks the 56-01 rebuild: a
// deployment holding the four-tuple-PK rule table must upgrade via
// New() — not die at boot — and every legacy row must land on the new
// shape with the documented mapping:
//
//	match_type='exact', origin='approve_remember', enabled=1,
//	hit_count=0, scope decoded from the binary encoding,
//	last_hit_at = legacy last_used_at.
func TestMigrateLegacyApprovalRulesRebuild(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy-rules-upgrade.db")
	seedLegacyApprovalRulesDB(t, dsn)

	s, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "legacy four-tuple-PK approval_rules DB must migrate via New() (was the risk: no such column: match_type)")
	defer func() { _ = s.Close() }()

	// The rebuilt table must be column-for-column the canonical
	// 56-01 shape (the same list schemaSQL's fresh-DB path is pinned
	// to — drift between the two DDLs fails here).
	assert.Equal(t, expectedApprovalRulesCols, tableColumns(t, s, "approval_rules"),
		"the rebuilt approval_rules must have the canonical 15 columns in canonical order")

	// The swap must be complete: the temporary v2 table is gone.
	var tblCount int
	require.NoError(t,
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='approval_rules_v2'`).Scan(&tblCount))
	require.Equal(t, 0, tblCount, "approval_rules_v2 must be renamed away after the rebuild")

	// The migrate-tail indexes exist on the rebuilt table; the LEGACY
	// indexes died with the DROP TABLE.
	for _, idx := range []string{
		"idx_approval_rules_natural",
		"idx_approval_rules_args_hash",
		"idx_approval_rules_scope",
		"idx_approval_rules_skill",
	} {
		var idxCount int
		require.NoError(t,
			s.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
		require.Equal(t, 1, idxCount, "%s must exist after the legacy rebuild", idx)
	}
	var legacyIdx int
	require.NoError(t,
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_approval_rules_scope_session'`).Scan(&legacyIdx))
	require.Equal(t, 0, legacyIdx, "the legacy scope_session index must NOT survive the rebuild")

	// Row count preserved: the mapping is injective, no merge loss.
	var count int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM approval_rules`).Scan(&count))
	require.Equal(t, 2, count, "both legacy rows must survive the rebuild")

	// Session-scope row: full field mapping.
	var target, matchType, argsHash, argsPattern, skillID, scope, scopeSessionID, origin string
	var enabled, hitCount int
	var expiresAt, lastHitAt, createdAt, lastUsed sql.NullInt64
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT target, match_type, args_hash, args_pattern, skill_id,
		        scope, scope_session_id, enabled, expires_at, origin,
		        hit_count, last_hit_at, created_at, last_used_at
		 FROM approval_rules WHERE args_hash = 'legacy-hash-sess'`).Scan(
		&target, &matchType, &argsHash, &argsPattern, &skillID,
		&scope, &scopeSessionID, &enabled, &expiresAt, &origin,
		&hitCount, &lastHitAt, &createdAt, &lastUsed))
	assert.Equal(t, "memory:facts", target)
	assert.Equal(t, "exact", matchType, "legacy rows are exact-tier (the only legacy matcher)")
	assert.Equal(t, "legacy-hash-sess", argsHash, "args_hash survives as the exact-tier index column")
	assert.Equal(t, "", argsPattern, "legacy rows carry no prefix pattern")
	assert.Equal(t, "compliance", skillID)
	assert.Equal(t, "session", scope, "non-empty scope_session_id decodes to session scope")
	assert.Equal(t, "sess-legacy", scopeSessionID, "the scope VALUE column survives")
	assert.Equal(t, 1, enabled, "legacy rows are enabled by default")
	assert.False(t, expiresAt.Valid, "legacy rows never expire")
	assert.Equal(t, "approve_remember", origin, "approve-and-remember is the only legacy origin")
	assert.Equal(t, 0, hitCount, "legacy rows start with zero hits")
	assert.True(t, lastHitAt.Valid && lastHitAt.Int64 == legacySessLastUsed,
		"legacy last_used_at (the 'last auto-pass' semantic) maps onto last_hit_at")
	assert.Equal(t, legacySessCreatedAt, createdAt.Int64, "created_at survives unchanged")

	// Skill-scope row: the ''-encoded binary scope decodes to 'skill'.
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT scope, scope_session_id, last_hit_at FROM approval_rules WHERE args_hash = 'legacy-hash-skill'`).Scan(
		&scope, &scopeSessionID, &lastHitAt))
	assert.Equal(t, "skill", scope, "empty scope_session_id decodes to skill scope")
	assert.Equal(t, "", scopeSessionID)
	assert.True(t, lastHitAt.Valid && lastHitAt.Int64 == legacySkillLastUsed,
		"skill-scope row maps last_used_at the same way")
}

// TestMigrateLegacyApprovalRulesIdempotentReopen guards the reopen
// direction: migrate() runs on every New(), so a once-rebuilt DB must
// open again cleanly and the probe gate (match_type present) must
// short-circuit the rebuild — no second copy of the rows.
func TestMigrateLegacyApprovalRulesIdempotentReopen(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "legacy-rules-reopen.db")
	seedLegacyApprovalRulesDB(t, dsn)

	first, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "first New() on legacy rules DB must rebuild")
	require.NoError(t, first.Close())

	second, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "second New() on the rebuilt DB must succeed (idempotent migrate)")
	defer func() { _ = second.Close() }()

	var count int
	require.NoError(t, second.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM approval_rules`).Scan(&count))
	require.Equal(t, 2, count, "rows must NOT duplicate across reopen (probe gate short-circuits)")

	var matchTypes int
	require.NoError(t, second.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM approval_rules WHERE match_type = 'exact'`).Scan(&matchTypes))
	require.Equal(t, 2, matchTypes, "rebuilt rows stay exact-tier after reopen (probe gate is keyed on match_type presence)")
}

// TestMigrateFreshFileDBApprovalRulesShape guards the fresh-database
// direction: a brand-new file DB gets the 56-01 shape directly from
// schemaSQL and must NOT enter the rebuild path (probe gate false from
// the very first boot).
func TestMigrateFreshFileDBApprovalRulesShape(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "fresh-rules.db")

	s, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "fresh file DB must open via New()")
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedApprovalRulesCols, tableColumns(t, s, "approval_rules"),
		"a fresh DB carries the canonical 56-01 shape without a rebuild")

	var count int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM approval_rules`).Scan(&count))
	require.Equal(t, 0, count, "a fresh DB starts with zero rules")
}
