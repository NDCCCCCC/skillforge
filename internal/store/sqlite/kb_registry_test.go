// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

package sqlite

// Phase 26 / Plan 26-02 — persistence half of the knowledge-base registry
// (KBM-01/02). These tests lock the three kb.Store verbs over the mutable
// knowledge_bases config table, the role CHECK constraint that backstops
// the closed vocabulary at the storage layer, and the additive migration
// path (fresh DB, re-migrate, legacy DB upgrade). The switch/structure
// split ("structure belongs to config, the switch belongs to ops") is
// asserted one layer up in internal/kb — here every persisted field must
// simply round-trip byte-for-byte.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/kb"
)

// kbTestEntry builds a fully-populated registry row for the tests.
func kbTestEntry(slug string, role kb.Role, display string, enabled bool, at time.Time) kb.Entry {
	return kb.Entry{
		Slug:             slug,
		DisplayName:      display,
		Role:             role,
		Description:      "test base " + slug,
		RetrievalEnabled: enabled,
		CreatedAt:        at,
		UpdatedAt:        at,
	}
}

// newKBTestStore opens a fresh :memory: store (single connection — the
// in-memory posture of approval_rules_test.go).
func newKBTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestKBRegistryRoundTrip — UpsertBase writes two rows; LoadBases reads
// them back slug-ascending with every field intact, including the
// retrieval switch and both timestamps.
func TestKBRegistryRoundTrip(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	first := kbTestEntry("authority", kb.RoleAuthority, "Authority", true, at)
	second := kbTestEntry("personal", kb.RolePersonal, "Personal", false, at.Add(time.Minute))
	second.UpdatedAt = at.Add(2 * time.Minute)

	require.NoError(t, s.UpsertBase(ctx, first))
	require.NoError(t, s.UpsertBase(ctx, second))

	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "authority", got[0].Slug, "LoadBases must return rows slug-ascending")
	assert.Equal(t, "personal", got[1].Slug)

	want := []kb.Entry{first, second}
	for i := range want {
		want[i].CreatedAt = want[i].CreatedAt.UTC()
		want[i].UpdatedAt = want[i].UpdatedAt.UTC()
		assert.Equal(t, want[i], got[i], "row %d must round-trip field-for-field", i)
	}
}

// TestUpsertBaseIdempotentUpdate — a second UpsertBase for the same slug
// takes the ON CONFLICT path: the row is updated in place, the row count
// stays at one, and no duplicate appears.
func TestUpsertBaseIdempotentUpdate(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpsertBase(ctx, kbTestEntry("reports", kb.RoleHistorical, "Reports", true, at)))

	changed := kb.Entry{
		Slug:             "reports",
		DisplayName:      "Monthly Reports",
		Role:             kb.RolePersonal,
		Description:      "re-scoped",
		RetrievalEnabled: false,
		CreatedAt:        at,
		UpdatedAt:        at.Add(time.Hour),
	}
	require.NoError(t, s.UpsertBase(ctx, changed))

	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1, "the ON CONFLICT path must update, never duplicate")
	changed.CreatedAt = changed.CreatedAt.UTC()
	changed.UpdatedAt = changed.UpdatedAt.UTC()
	assert.Equal(t, changed, got[0])
}

// TestSetRetrievalEnabled — the deactivation switch flips in isolation:
// only the switch and updated_at move, and an unknown slug is an error
// rather than a silent zero-row update.
func TestSetRetrievalEnabled(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpsertBase(ctx, kbTestEntry("authority", kb.RoleAuthority, "Authority", true, at)))

	later := at.Add(30 * time.Minute)
	require.NoError(t, s.SetRetrievalEnabled(ctx, "authority", false, later))

	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].RetrievalEnabled, "the switch must flip to false")
	assert.Equal(t, later.UTC(), got[0].UpdatedAt, "the switch flip must stamp updated_at")
	assert.Equal(t, at.UTC(), got[0].CreatedAt, "created_at must not move")

	// re-enabling round-trips too
	require.NoError(t, s.SetRetrievalEnabled(ctx, "authority", true, later.Add(time.Minute)))
	got, err = s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].RetrievalEnabled)

	// an unknown slug must fail closed, never report success on 0 rows
	err = s.SetRetrievalEnabled(ctx, "kb-nope", false, later)
	require.Error(t, err, "SetRetrievalEnabled on an unknown slug must error")
	assert.True(t, strings.Contains(err.Error(), "kb-nope"), "error %q must name the slug", err)
}

// TestLoadBasesEmpty — an empty registry is a legal state: no rows, no
// error (26-01 posture — the boot path decides whether emptiness is fatal).
func TestLoadBasesEmpty(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	assert.NotNil(t, got, "LoadBases must return a non-nil slice for an empty registry")
	assert.Empty(t, got)
}

// TestRoleCheckConstraint — the closed role vocabulary is enforced at the
// storage layer by the CHECK constraint, not only by Go-side validation: a
// row carrying a role outside the vocabulary is refused by sqlite itself.
func TestRoleCheckConstraint(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
INSERT INTO knowledge_bases (slug, display_name, role, description, retrieval_enabled, created_at, updated_at)
VALUES ('rogue', 'Rogue', 'admin', '', 1, '2026-09-07T12:00:00Z', '2026-09-07T12:00:00Z')`)
	require.Error(t, err, "a role outside the closed vocabulary must be refused by the CHECK constraint")
	assert.Contains(t, err.Error(), "CHECK constraint", "the refusal must come from the constraint layer")
}

// TestMigrateIdempotent — migrate() runs on every New(), so calling it a
// second time on the same connection must be a clean no-op (CREATE TABLE
// IF NOT EXISTS plus the tail index).
func TestMigrateIdempotent(t *testing.T) {
	s := newKBTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, s.migrate(ctx), "a second migrate() on the same connection must be a no-op")

	// the table stays usable after the re-migration
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpsertBase(ctx, kbTestEntry("authority", kb.RoleAuthority, "Authority", true, at)))
	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// TestExistingDBUpgrades — the additive forward-compatibility direction
// (mirrors migrate_legacy_test.go): a database created before
// knowledge_bases existed must gain the table and its role index through
// New() without losing the pre-existing rows.
func TestExistingDBUpgrades(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn := t.TempDir() + "/legacy-kb-upgrade.db"
	seedLegacyAuditDB(t, dsn) // pre-Phase-26 schema: no knowledge_bases table

	s, err := New(ctx, Config{DSN: dsn})
	require.NoError(t, err, "a legacy DB must upgrade via New() (additive migration, D14-E1)")
	defer func() { _ = s.Close() }()

	var tblCount int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_bases'`).Scan(&tblCount))
	require.Equal(t, 1, tblCount, "knowledge_bases must be created on the upgraded DB")

	var idxCount int
	require.NoError(t, s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_knowledge_bases_role'`).Scan(&idxCount))
	require.Equal(t, 1, idxCount, "idx_knowledge_bases_role must exist after the legacy upgrade")

	// the legacy table the DB already carried is untouched
	var auditRows int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&auditRows))
	require.Equal(t, 1, auditRows, "legacy audit rows must survive the additive migration")

	// and the new table is immediately usable
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpsertBase(ctx, kbTestEntry("authority", kb.RoleAuthority, "Authority", true, at)))
	got, err := s.LoadBases(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
}
