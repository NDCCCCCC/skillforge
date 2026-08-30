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

// Phase 19 / Plan 19-01 / PORT-01: INSERT-only report_runs table for
// scheduler-triggered report run audit (PORT-02 cron + PORT-03 portal).
// Mirror of reports.go with a different column set: PutReportRun is the
// single append-only write path, every value rides in a `?` placeholder,
// and this file deliberately ships no row-mutation or row-removal verb
// at all (T-3-AUDIT anti-repudiation).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putReportRunSQL is the single write path for the report_runs audit table
// (PORT-01). Column order mirrors the CREATE TABLE DDL in schema.go
// one-to-one; id is omitted (AUTOINCREMENT assigns it).
const putReportRunSQL = `
INSERT INTO report_runs (
    run_id, skill_id, period, cron_expr, fired_at,
    kpi_snapshots, output_url, status, drift_detected, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// PutReportRun appends one INSERT-only report run audit row. Consumed by
// the scheduler on each cron firing to record the run outcome.
func (s *SQLiteStore) PutReportRun(ctx context.Context, row store.ReportRunRow) error {
	_, err := s.db.ExecContext(ctx, putReportRunSQL,
		row.RunID,
		row.SkillID,
		row.Period,
		row.CronExpr,
		row.FiredAt.UTC().Format(time.RFC3339Nano),
		row.KPISnapshots,
		row.OutputURL,
		row.Status,
		btof(row.DriftDetected),
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmtError("PutReportRun", err)
	}
	return nil
}

// listReportRunsBaseSQL selects the full report_runs column set.
const listReportRunsBaseSQL = `
SELECT id, run_id, skill_id, period, cron_expr, fired_at,
       kpi_snapshots, output_url, status, drift_detected, created_at
FROM report_runs
`

// ListReportRuns returns report run rows filtered by skill_id and/or period,
// newest-first (ORDER BY created_at DESC). Empty skillID or period means
// no filter on that dimension.
func (s *SQLiteStore) ListReportRuns(ctx context.Context, skillID, period string, limit int) ([]store.ReportRunRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if limit <= 0 {
		limit = 20
	}

	// Build WHERE clause dynamically from fixed column names (T-3-SQLI:
	// only column names are in the query string; filter values go through
	// ? placeholders exclusively).
	where := " WHERE 1=1"
	args := []any{}
	if skillID != "" {
		where += " AND skill_id=?"
		args = append(args, skillID)
	}
	if period != "" {
		where += " AND period=?"
		args = append(args, period)
	}
	query := listReportRunsBaseSQL + where + " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmtError("ListReportRuns", err)
	}
	defer rows.Close()

	out := make([]store.ReportRunRow, 0)
	for rows.Next() {
		var r store.ReportRunRow
		var firedAt, createdAt string
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.SkillID, &r.Period, &r.CronExpr,
			&firedAt, &r.KPISnapshots, &r.OutputURL, &r.Status,
			&r.DriftDetected, &createdAt,
		); err != nil {
			return nil, fmtError("ListReportRuns.Scan", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, firedAt)
		if err != nil {
			return nil, fmtError("ListReportRuns.ParseFiredAt", err)
		}
		r.FiredAt = parsed.UTC()
		parsed, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmtError("ListReportRuns.ParseCreatedAt", err)
		}
		r.CreatedAt = parsed.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListReportRuns.Rows", err)
	}
	return out, nil
}

// CountRows returns the total row count for a named table. Used by the
// admin console capacity monitor (Phase 20 / ADM-01). Table names are
// allowlisted — unknown names return an error (caller → 500).
func (s *SQLiteStore) CountRows(ctx context.Context, table string) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	// Allowlist: only pre-defined table names are accepted.
	switch table {
	case "audit", "kpi_calls", "report_runs":
		// allowed
	default:
		return 0, fmt.Errorf("CountRows: unknown table %q", table)
	}
	var count int64
	query := "SELECT COUNT(*) FROM " + table
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmtError("CountRows."+table, err)
	}
	return count, nil
}

// GetReportRun returns the report run for a specific run_id.
func (s *SQLiteStore) GetReportRun(ctx context.Context, runID string) (*store.ReportRunRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	row := &store.ReportRunRow{}
	var firedAt, createdAt string
	err := s.db.QueryRowContext(ctx,
		listReportRunsBaseSQL+" WHERE run_id=?",
		runID).
		Scan(
			&row.ID, &row.RunID, &row.SkillID, &row.Period, &row.CronExpr,
			&firedAt, &row.KPISnapshots, &row.OutputURL, &row.Status,
			&row.DriftDetected, &createdAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmtError("GetReportRun", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, firedAt)
	if err != nil {
		return nil, fmtError("GetReportRun.ParseFiredAt", err)
	}
	row.FiredAt = parsed.UTC()
	parsed, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmtError("GetReportRun.ParseCreatedAt", err)
	}
	row.CreatedAt = parsed.UTC()
	return row, nil
}

// GetLatestReportRunBySkill returns the most recent report run for a
// (skill_id, period) pair, or nil if none exists.
func (s *SQLiteStore) GetLatestReportRunBySkill(ctx context.Context, skillID, period string) (*store.ReportRunRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	row := &store.ReportRunRow{}
	var firedAt, createdAt string
	err := s.db.QueryRowContext(ctx,
		listReportRunsBaseSQL+" WHERE skill_id=? AND period=? ORDER BY created_at DESC LIMIT 1",
		skillID, period).
		Scan(
			&row.ID, &row.RunID, &row.SkillID, &row.Period, &row.CronExpr,
			&firedAt, &row.KPISnapshots, &row.OutputURL, &row.Status,
			&row.DriftDetected, &createdAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmtError("GetLatestReportRunBySkill", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, firedAt)
	if err != nil {
		return nil, fmtError("GetLatestReportRunBySkill.ParseFiredAt", err)
	}
	row.FiredAt = parsed.UTC()
	parsed, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmtError("GetLatestReportRunBySkill.ParseCreatedAt", err)
	}
	row.CreatedAt = parsed.UTC()
	return row, nil
}

// btof converts bool to int (SQLite stores booleans as 0/1).
func btof(b bool) int {
	if b {
		return 1
	}
	return 0
}

// suppress unused-variable warnings for strings import
var _ = strings.TrimSpace
