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

// Phase 18 / Plan 18-02 — INSERT-only reports table for monthly report
// metadata (RPT-04). Mirror of kpi_calls.go with a different column set:
// PutReport is the single append-only write path, every value rides in a
// `?` placeholder, and this file deliberately ships no row-mutation or
// row-removal verb at all (T-3-AUDIT anti-repudiation).
package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putReportSQL is the single write path for the reports audit table
// (RPT-04). Column order mirrors the CREATE TABLE DDL in schema.go
// one-to-one; id is omitted (AUTOINCREMENT assigns it).
const putReportSQL = `
INSERT INTO reports (
    ts, period, skill_id, version, title,
    file_path, approval_audit_id, kpi_call_ids, template_version, result_digest
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// PutReport appends one INSERT-only report metadata row. Consumed by the
// hitl_report_publish tool after successful publication to record the
// report artifact.
func (s *SQLiteStore) PutReport(ctx context.Context, row store.ReportRow) error {
	_, err := s.db.ExecContext(ctx, putReportSQL,
		row.TS.UTC().Format(time.RFC3339Nano),
		row.Period,
		row.SkillID,
		row.Version,
		row.Title,
		row.FilePath,
		row.ApprovalAuditID,
		row.KPICallIDs,
		row.TemplateVersion,
		row.ResultDigest,
	)
	if err != nil {
		return fmtError("PutReport", err)
	}
	return nil
}

// listReportsBaseSQL selects the full reports column set.
const listReportsBaseSQL = `
SELECT id, ts, period, skill_id, version, title,
       file_path, approval_audit_id, kpi_call_ids, template_version, result_digest
FROM reports
`

// ListReports returns report metadata rows for the given period,
// newest-first.
func (s *SQLiteStore) ListReports(ctx context.Context, period string, limit int) ([]store.ReportRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		listReportsBaseSQL+" WHERE period=? ORDER BY version DESC LIMIT ?",
		period, limit)
	if err != nil {
		return nil, fmtError("ListReports", err)
	}
	defer rows.Close()

	out := make([]store.ReportRow, 0)
	for rows.Next() {
		var r store.ReportRow
		var ts string
		if err := rows.Scan(
			&r.ID, &ts, &r.Period, &r.SkillID, &r.Version,
			&r.Title, &r.FilePath, &r.ApprovalAuditID,
			&r.KPICallIDs, &r.TemplateVersion, &r.ResultDigest,
		); err != nil {
			return nil, fmtError("ListReports.Scan", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmtError("ListReports.ParseTS", err)
		}
		r.TS = parsed.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListReports.Rows", err)
	}
	return out, nil
}

// GetReport returns the report for a specific period and version,
// or nil if not found.
func (s *SQLiteStore) GetReport(ctx context.Context, period string, version int) (*store.ReportRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	row := &store.ReportRow{}
	var ts string
	err := s.db.QueryRowContext(ctx,
		listReportsBaseSQL+" WHERE period=? AND version=?",
		period, version).
		Scan(
			&row.ID, &ts, &row.Period, &row.SkillID, &row.Version,
			&row.Title, &row.FilePath, &row.ApprovalAuditID,
			&row.KPICallIDs, &row.TemplateVersion, &row.ResultDigest,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmtError("GetReport", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmtError("GetReport.ParseTS", err)
	}
	row.TS = parsed.UTC()
	return row, nil
}

// GetLatestReport returns the most recent report for a period.
func (s *SQLiteStore) GetLatestReport(ctx context.Context, period string) (*store.ReportRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	row := &store.ReportRow{}
	var ts string
	err := s.db.QueryRowContext(ctx,
		listReportsBaseSQL+" WHERE period=? ORDER BY version DESC LIMIT 1",
		period).
		Scan(
			&row.ID, &ts, &row.Period, &row.SkillID, &row.Version,
			&row.Title, &row.FilePath, &row.ApprovalAuditID,
			&row.KPICallIDs, &row.TemplateVersion, &row.ResultDigest,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmtError("GetLatestReport", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmtError("GetLatestReport.ParseTS", err)
	}
	row.TS = parsed.UTC()
	return row, nil
}

// ListReportsBySkill returns all reports for a given skill, newest-first.
func (s *SQLiteStore) ListReportsBySkill(ctx context.Context, skillID string, limit int) ([]store.ReportRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		listReportsBaseSQL+" WHERE skill_id=? ORDER BY ts DESC LIMIT ?",
		skillID, limit)
	if err != nil {
		return nil, fmtError("ListReportsBySkill", err)
	}
	defer rows.Close()

	out := make([]store.ReportRow, 0)
	for rows.Next() {
		var r store.ReportRow
		var ts string
		if err := rows.Scan(
			&r.ID, &ts, &r.Period, &r.SkillID, &r.Version,
			&r.Title, &r.FilePath, &r.ApprovalAuditID,
			&r.KPICallIDs, &r.TemplateVersion, &r.ResultDigest,
		); err != nil {
			return nil, fmtError("ListReportsBySkill.Scan", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmtError("ListReportsBySkill.ParseTS", err)
		}
		r.TS = parsed.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListReportsBySkill.Rows", err)
	}
	return out, nil
}

// suppress unused-variable warnings for strings import
var _ = strings.TrimSpace
