// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Phase 26 / Plan 26-02 — the persistence layer of the knowledge-base
// registry (KBM-01/02). kb_registry.go adapts the kb.Store seam (declared
// in internal/kb, the leaf contract package) over the mutable
// knowledge_bases config table:
//
//   - the table is MUTABLE configuration (the datasets precedent for the
//     "config is not evidence" split): UpsertBase writes the declared
//     fields in place and SetRetrievalEnabled flips the deactivation
//     switch — deactivation is an update of retrieval_enabled, never a
//     delete (KBM-02: deactivate, never destroy; this file has no delete
//     path for knowledge_bases and must never grow one).
//   - the audit evidence for every change lives in the EXISTING audit
//     table as insert-only rows written by the CALLER (server layer,
//     Plan 26-05). This file has no audit-row write path and must never
//     grow one — the kb package itself never writes audit rows either.
//   - every value rides a `?` placeholder; no statement in this file is
//     ever built from caller-controlled strings (T-26-02-3).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/skillforge/skillforge/internal/kb"
)

// compile-time proof that the sqlite store carries the whole kb.Store
// contract (the D11 conformance-check discipline; the seam is consumed by
// kb.Open in 26-02 and wired at boot in 26-09).
var _ kb.Store = (*SQLiteStore)(nil)

// loadBasesSQL returns every registry row — active and deactivated (the
// admin view and the boot merge both need the full set) — slug-ascending
// for a deterministic registry order.
const loadBasesSQL = `
SELECT slug, display_name, role, description, retrieval_enabled, created_at, updated_at
FROM knowledge_bases
ORDER BY slug ASC
`

// upsertBaseSQL is the single write path for a registry row. The ON
// CONFLICT clause updates the DECLARED fields (display_name, role,
// description) plus the switch and updated_at, keyed on the slug primary
// key — a replayed seed upsert is idempotent. Timestamps are supplied by
// the caller: the seed-merge path in kb.Open preserves a row's existing
// created_at/updated_at by passing them straight back through, so a
// structure-only change never rewrites the switch's history.
const upsertBaseSQL = `
INSERT INTO knowledge_bases (
    slug, display_name, role, description, retrieval_enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
    display_name = excluded.display_name,
    role = excluded.role,
    description = excluded.description,
    retrieval_enabled = excluded.retrieval_enabled,
    updated_at = excluded.updated_at
`

// setRetrievalEnabledSQL moves ONLY the deactivation switch and its
// updated_at stamp. The slug is the `?`-bound key; a slug that matches no
// row reports through RowsAffected (see SetRetrievalEnabled) instead of
// succeeding silently on zero rows.
const setRetrievalEnabledSQL = `
UPDATE knowledge_bases
SET retrieval_enabled = ?, updated_at = ?
WHERE slug = ?
`

// LoadBases returns every persisted registry row, active and deactivated,
// sorted by slug ascending. An empty table is a legal state: it returns a
// non-nil empty slice and no error (26-01 posture — the boot path decides
// whether an empty registry is fatal, not the store).
func (s *SQLiteStore) LoadBases(ctx context.Context) ([]kb.Entry, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, loadBasesSQL)
	if err != nil {
		return nil, fmtError("LoadBases", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]kb.Entry, 0)
	for rows.Next() {
		var (
			e                    kb.Entry
			role                 string
			enabled              int
			createdAt, updatedAt string
		)
		if err := rows.Scan(
			&e.Slug,
			&e.DisplayName,
			&role,
			&e.Description,
			&enabled,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmtError("LoadBases.Scan", err)
		}
		created, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmtError("LoadBases.created_at", err)
		}
		updated, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmtError("LoadBases.updated_at", err)
		}
		e.Role = kb.Role(role)
		e.RetrievalEnabled = enabled == 1
		e.CreatedAt = created.UTC()
		e.UpdatedAt = updated.UTC()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("LoadBases.Rows", err)
	}
	return out, nil
}

// UpsertBase writes the declared fields of one registry row, creating it
// when the slug is new and updating it in place when it already exists
// (KBM-01 seed merge: structure belongs to config). The row's switch and
// timestamps are whatever the caller passes — the seed-merge path in
// kb.Open is the component that keeps an admin's deactivation by passing
// the persisted value back through unchanged.
func (s *SQLiteStore) UpsertBase(ctx context.Context, e kb.Entry) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	_, err := s.db.ExecContext(ctx, upsertBaseSQL,
		e.Slug,
		e.DisplayName,
		string(e.Role),
		e.Description,
		boolToInt(e.RetrievalEnabled),
		e.CreatedAt.UTC().Format(time.RFC3339Nano),
		e.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("UpsertBase", err)
	}
	return nil
}

// SetRetrievalEnabled flips only the deactivation switch of one registry
// row and stamps updated_at (KBM-02 admin deactivate / re-enable). An
// unknown slug is an ERROR, never a silent zero-row success — an admin
// action against a base that does not exist must be visible to the caller
// (which turns it into a 404) rather than disappearing.
func (s *SQLiteStore) SetRetrievalEnabled(ctx context.Context, slug string, enabled bool, updatedAt time.Time) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, setRetrievalEnabledSQL,
		boolToInt(enabled),
		updatedAt.UTC().Format(time.RFC3339Nano),
		slug,
	)
	if err != nil {
		return fmtError("SetRetrievalEnabled", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmtError("SetRetrievalEnabled.RowsAffected", err)
	}
	if n == 0 {
		return fmtError("SetRetrievalEnabled", fmt.Errorf("knowledge base %q is not registered", slug))
	}
	return nil
}

// boolToInt maps a Go bool onto the table's INTEGER switch column.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
