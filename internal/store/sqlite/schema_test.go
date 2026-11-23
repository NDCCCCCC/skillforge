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

// Plan 10-02 Task 2 — audit_meta sibling-table DDL + INSERT + CASCADE
// + nullable column coverage. The tests live in internal/store/sqlite
// (white-box) because they inspect the table shape directly via PRAGMA
// + raw SELECT — a black-box test through Store would not observe the
// DDL ordering or the ON DELETE CASCADE behavior.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/store"
)

// expectedAuditMetaCols is the canonical column list asserted by
// TestAuditMetaDDL. Order matches the schemaSQL CREATE TABLE statement
// so a future migration that adds / reorders columns fails loudly.
var expectedAuditMetaCols = []string{
	"query_id",
	"rewritten_query",
	"classifier_decision",
	"classifier_reason",
	"classifier_confidence",
	"history_projected_n_turns",
	"history_budget_truncated",
	"history_dropped_count",
}

// expectedDatasetsCols is the canonical column list asserted by
// TestDatasetsDDL. Order matches the schemaSQL CREATE TABLE statement
// so a future migration that adds / reorders columns fails loudly.
var expectedDatasetsCols = []string{
	"name",
	"tier",
	"schema_json",
	"file_path",
	"content_sha256",
	"batch_id",
	"created_at",
	"dataset",
	"period",
	"k_min_override",
}

// expectedImportBatchesCols is the canonical column list asserted by
// TestImportBatchesDDL.
var expectedImportBatchesCols = []string{
	"batch_id",
	"dataset",
	"period",
	"tier",
	"content_sha256",
	"rules_sha256",
	"raw_size_bytes",
	"desensitized_row_count",
	"discard_ts",
	"supersedes_batch_id",
	"created_at",
}

// expectedDatasetDeletesCols is the canonical column list asserted by
// TestDatasetDeletesDDL.
var expectedDatasetDeletesCols = []string{
	"id",
	"name",
	"dataset",
	"deleted_at",
}

// expectedApprovalRulesCols is the canonical column list asserted by
// TestApprovalRulesDDL (Phase 56 / Plan 56-01 / D-56-01). Order matches
// the schemaSQL CREATE TABLE statement so a future migration that adds
// / reorders columns fails loudly. This ONE list also pins the legacy
// rebuild: approval_rules_migrate_test.go asserts the SAME list against
// a rebuilt pre-56 database, so any drift between the schemaSQL DDL and
// legacyApprovalRulesV2DDL fails in whichever test runs first.
var expectedApprovalRulesCols = []string{
	"id",
	"target",
	"match_type",
	"args_hash",
	"args_pattern",
	"skill_id",
	"scope",
	"scope_session_id",
	"enabled",
	"expires_at",
	"origin",
	"hit_count",
	"last_hit_at",
	"created_at",
	"last_used_at",
}

// expectedApprovalRuleHitsCols is the canonical column list asserted by
// TestApprovalRuleHitsDDL.
var expectedApprovalRuleHitsCols = []string{
	"id",
	"rule_id",
	"ts",
	"session_id",
	"target",
	"tool_call_id",
	"match_type",
	"matched_args_hash",
}

// tableColumns returns the ordered column names from PRAGMA table_info.
func tableColumns(t *testing.T, s *SQLiteStore, table string) []string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), fmt.Sprintf("PRAGMA table_info(%s)", table))
	require.NoError(t, err, "PRAGMA table_info(%s)", table)
	defer rows.Close()

	got := []string{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		got = append(got, name)
	}
	require.NoError(t, rows.Err())
	return got
}

// TestDatasetsDDL — after migrate(), the datasets table has the 10
// expected columns including the Phase 15 additive dataset/period/k_min_override.
func TestDatasetsDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedDatasetsCols, tableColumns(t, s, "datasets"),
		"datasets must have the expected 10 columns in canonical order (Phase 15 Plan 15-02 spec)")
}

// TestImportBatchesDDL — after migrate(), the import_batches table exists
// with the 11 expected columns (DATA-17 + D15-A4 + D15-C1).
func TestImportBatchesDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedImportBatchesCols, tableColumns(t, s, "import_batches"),
		"import_batches must have the expected 11 columns")
}

// TestDatasetDeletesDDL — after migrate(), the dataset_deletes table exists
// with the expected columns (D15-C3).
func TestDatasetDeletesDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedDatasetDeletesCols, tableColumns(t, s, "dataset_deletes"),
		"dataset_deletes must have the expected columns")
}

// TestImportBatchesIndexExists — after migrate(), the (dataset, period)
// index exists for the LatestImportBatch query path (DATA-16).
func TestImportBatchesIndexExists(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	var idxCount int
	require.NoError(t,
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_import_batches_dataset_period'`).Scan(&idxCount))
	require.Equal(t, 1, idxCount, "idx_import_batches_dataset_period must exist after migration")
}

// TestAuditMetaDDL — after migrate(), the audit_meta table exists
// with the 8 expected columns in the expected order.
func TestAuditMetaDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(audit_meta)`)
	require.NoError(t, err, "PRAGMA table_info(audit_meta)")
	defer rows.Close()

	got := []string{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		got = append(got, name)
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, expectedAuditMetaCols, got,
		"audit_meta must have the expected 8 columns in canonical order (Plan 10-02 Task 2 spec)")
}

// TestAuditMetaInsertQuery — PUT an audit row, PUT a meta row
// referencing its query_id, SELECT the meta row back, and assert
// every field round-trips.
func TestAuditMetaInsertQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Seed an audit row so the meta row has a valid FK target.
	row := store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-audit-meta",
		SkillID:     "compliance",
		Query:       "年假有几天",
		PromptHash:  "deadbeef",
		VerdictJSON: "{}",
	}
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// Insert the meta row.
	confidence := 0.95
	meta := store.AuditMeta{
		QueryID:                id,
		RewrittenQuery:         "年假制度的内容",
		ClassifierDecision:     "allow",
		ClassifierReason:       "",
		ClassifierConfidence:   &confidence,
		HistoryProjectedNTurns: 4,
		HistoryBudgetTruncated: 1,
		HistoryDroppedCount:    2,
	}
	require.NoError(t, s.PutAuditMeta(ctx, meta))

	// Round-trip the meta row via raw SQL (the Store has no GetAuditMeta
	// reader — only PutAuditMeta is in the interface for now).
	var (
		rewritten       sql.NullString
		decision        string
		reason          sql.NullString
		conf            sql.NullFloat64
		projectedN      int
		budgetTruncated int
		droppedCount    int
	)
	err = s.db.QueryRowContext(ctx,
		`SELECT rewritten_query, classifier_decision, classifier_reason,
		        classifier_confidence, history_projected_n_turns,
		        history_budget_truncated, history_dropped_count
		 FROM audit_meta WHERE query_id = ?`, id,
	).Scan(&rewritten, &decision, &reason, &conf, &projectedN, &budgetTruncated, &droppedCount)
	require.NoError(t, err)

	assert.Equal(t, "年假制度的内容", rewritten.String)
	assert.Equal(t, "allow", decision)
	assert.False(t, reason.Valid, "classifier_reason is nullable; absent → NULL")
	require.True(t, conf.Valid)
	assert.InDelta(t, 0.95, conf.Float64, 0.001)
	assert.Equal(t, 4, projectedN)
	assert.Equal(t, 1, budgetTruncated)
	assert.Equal(t, 2, droppedCount)
}

// TestAuditMetaCascadeDelete — DELETE the audit row; the meta row
// drops with it (FK ON DELETE CASCADE).
func TestAuditMetaCascadeDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	row := store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-cascade",
		SkillID:     "compliance",
		Query:       "q",
		PromptHash:  "ph",
		VerdictJSON: "{}",
	}
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)
	require.NoError(t, s.PutAuditMeta(ctx, store.AuditMeta{
		QueryID:            id,
		ClassifierDecision: "n/a",
	}))

	// DELETE the audit row directly (T-3-AUDIT forbids a runtime
	// path; this is a test-only raw SQL operation).
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit WHERE id = ?`, id)
	require.NoError(t, err)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "DELETE must affect the seeded row")

	// Assert the meta row is gone via CASCADE.
	var count int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_meta WHERE query_id = ?`, id,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "ON DELETE CASCADE must remove the meta row")
}

// TestAuditMetaNullableOptionalColumns — PUT a meta row with the
// three nullable columns (rewritten_query, classifier_reason,
// classifier_confidence) set to nil; assert the row is stored and
// the columns are NULL in the database.
func TestAuditMetaNullableOptionalColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	row := store.AuditRow{
		Timestamp:   time.Now().UTC(),
		SessionID:   "sess-nullable",
		SkillID:     "compliance",
		Query:       "q",
		PromptHash:  "ph",
		VerdictJSON: "{}",
	}
	id, err := s.PutAudit(ctx, row)
	require.NoError(t, err)

	// Nil confidence — no pointer supplied (AuditMeta.ClassifierConfidence is *float64).
	require.NoError(t, s.PutAuditMeta(ctx, store.AuditMeta{
		QueryID:            id,
		ClassifierDecision: "n/a",
		// RewrittenQuery, ClassifierReason, ClassifierConfidence all zero-value.
	}))

	var (
		rewritten sql.NullString
		reason    sql.NullString
		conf      sql.NullFloat64
	)
	err = s.db.QueryRowContext(ctx,
		`SELECT rewritten_query, classifier_reason, classifier_confidence
		 FROM audit_meta WHERE query_id = ?`, id,
	).Scan(&rewritten, &reason, &conf)
	require.NoError(t, err)
	assert.False(t, rewritten.Valid, "rewritten_query NULL when unset")
	assert.False(t, reason.Valid, "classifier_reason NULL when unset")
	assert.False(t, conf.Valid, "classifier_confidence NULL when unset")
}

// TestApprovalRulesDDL — after migrate(), the approval_rules table has
// the 15 expected D-56-01 columns in canonical order (Phase 56 / Plan
// 56-01). The table is the project's FIRST rebuild migration; the same
// list is asserted against a REBUILT legacy DB in
// approval_rules_migrate_test.go, pinning schemaSQL and
// legacyApprovalRulesV2DDL to one shape.
func TestApprovalRulesDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedApprovalRulesCols, tableColumns(t, s, "approval_rules"),
		"approval_rules must have the expected 15 D-56-01 columns in canonical order")
}

// TestApprovalRuleHitsDDL — after migrate(), the approval_rule_hits
// INSERT-only ledger exists with the 8 expected columns.
func TestApprovalRuleHitsDDL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	assert.Equal(t, expectedApprovalRuleHitsCols, tableColumns(t, s, "approval_rule_hits"),
		"approval_rule_hits must have the expected 8 columns in canonical order")
}

// TestApprovalRulesIndexesExist — after migrate(), the five Phase 56
// indexes exist: the UNIQUE natural key (the four-tuple PK's
// successor, with match_type/args_pattern folded in), the exact-tier
// args_hash index, the two scope listing indexes, and the hit-ledger
// history index. These live at the migrate() tail AFTER the legacy
// rebuild — the UAT 14-09 ordering contract — so their existence here
// also proves the fresh-DB path lands them.
func TestApprovalRulesIndexesExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	for _, idx := range []string{
		"idx_approval_rules_natural",
		"idx_approval_rules_args_hash",
		"idx_approval_rules_scope",
		"idx_approval_rules_skill",
		"idx_rule_hits_rule",
	} {
		var idxCount int
		require.NoError(t,
			s.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
		require.Equal(t, 1, idxCount, "%s must exist after migration", idx)
	}
}
