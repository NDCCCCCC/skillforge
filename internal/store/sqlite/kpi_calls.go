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

// Phase 16 / Plan 16-03 — INSERT-only KPI-call audit storage (KPI-02 /
// D16-D4). Wholesale mirror of import_batches.go with a new column set:
// PutKPICall is the single append-only write path, every value rides in a
// `?` placeholder, and this file deliberately ships no row-mutation or
// row-removal verb at all (T-3-AUDIT + T-16-03-01 anti-repudiation;
// locked by TestKPICallsInsertOnly's source-grep assertion).
package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/skillforge/skillforge/internal/store"
)

// putKPICallSQL is the single write path for the kpi_calls audit table
// (KPI-02 + D16-D4). Column order mirrors the CREATE TABLE DDL in
// schema.go one-to-one; id is omitted (AUTOINCREMENT assigns it) and
// signature carries the DEFAULT '' safety net.
const putKPICallSQL = `
INSERT INTO kpi_calls (
    ts, session_id, skill_id, func, args_json,
    as_of_period, k_min_applied, result_snapshot, signature
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// PutKPICall appends one INSERT-only KPI-call audit row. Consumed by the
// 16-06 executor's AuditSink (the audit leg of its three output legs);
// each invocation freezes {func, args, as_of, result_snapshot,
// k_min_applied} exactly as observed, never truncated (D16-D3).
func (s *SQLiteStore) PutKPICall(ctx context.Context, row store.KPICallRow) error {
	_, err := s.db.ExecContext(ctx, putKPICallSQL,
		row.TS.UTC().Format(time.RFC3339Nano),
		row.SessionID,
		row.SkillID,
		row.Func,
		row.ArgsJSON,
		row.ASOfPeriod,
		row.KMinApplied,
		row.ResultSnapshot,
		row.Signature,
	)
	if err != nil {
		return fmtError("PutKPICall", err)
	}
	return nil
}

// defaultKPICallsListLimit mirrors QueryAudit's LIMIT default.
const defaultKPICallsListLimit = 100

// listKPICallsBaseSQL selects the full kpi_calls column set. The WHERE
// clause (if any) is assembled ONLY from the fixed clause constants below
// — the column names are code, never caller input, and every VALUE rides
// in a `?` placeholder. No fmt.Sprintf with user input, ever
// (T-16-03-02; same posture as QueryAudit).
const listKPICallsBaseSQL = `
SELECT id, ts, session_id, skill_id, func, args_json,
       as_of_period, k_min_applied, result_snapshot, signature
FROM kpi_calls
`

// kpiCallsClauseFunc / kpiCallsClauseSessionID are the two sanctioned
// filter clauses — hardcoded constants so strings.Join can never see a
// caller-controlled string (T-16-03-02).
const (
	kpiCallsClauseFunc      = "func = ?"
	kpiCallsClauseSessionID = "session_id = ?"
)

// listKPICallsTailSQL orders newest-first by insertion id and caps the
// result set; the limit value is appended to args, not into SQL text.
const listKPICallsTailSQL = " ORDER BY id DESC LIMIT ?"

// ListKPICalls returns KPI-call audit rows matching the filter,
// newest-first. Zero-value filter fields are not filtered; a zero Limit
// defaults to 100.
func (s *SQLiteStore) ListKPICalls(ctx context.Context, filter store.KPICallsFilter) ([]store.KPICallRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	var (
		clauses []string
		args    []any
	)
	if filter.Func != "" {
		clauses = append(clauses, kpiCallsClauseFunc)
		args = append(args, filter.Func)
	}
	if filter.SessionID != "" {
		clauses = append(clauses, kpiCallsClauseSessionID)
		args = append(args, filter.SessionID)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultKPICallsListLimit
	}

	q := listKPICallsBaseSQL
	if len(clauses) > 0 {
		// strings.Join is safe — the only strings in `clauses` are the
		// hardcoded parameterized literals above, NEVER user input.
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += listKPICallsTailSQL
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmtError("ListKPICalls", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]store.KPICallRow, 0)
	for rows.Next() {
		var r store.KPICallRow
		var ts string
		if err := rows.Scan(
			&r.ID,
			&ts,
			&r.SessionID,
			&r.SkillID,
			&r.Func,
			&r.ArgsJSON,
			&r.ASOfPeriod,
			&r.KMinApplied,
			&r.ResultSnapshot,
			&r.Signature,
		); err != nil {
			return nil, fmtError("ListKPICalls", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmtError("ListKPICalls", err)
		}
		r.TS = parsed.UTC()
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("ListKPICalls", err)
	}
	return out, nil
}

// latestKPICallByFuncSQL implements the ADM-03 "last call time per KPI"
// view as ONE aggregation over kpi_calls — no JOIN against the chat audit
// main table.
//
// MAX(id), not MAX(ts): AUTOINCREMENT makes id strictly monotonic with
// insertion order (the table's replay-order guarantee, stated in the DDL
// comment in schema.go), whereas RFC3339Nano TEXT comparison has a
// fraction-trim edge — a zero-nanosecond stamp ("…00Z") sorts ABOVE a
// later same-second fractional stamp ("…00.25Z"), which would misreport
// the latest call when two invocations land in one wall-clock second.
const latestKPICallByFuncSQL = `
SELECT func, ts FROM kpi_calls
WHERE id IN (SELECT MAX(id) FROM kpi_calls GROUP BY func)
`

// LatestKPICallByFunc returns, per KPI name, the timestamp of its most
// recent call. Empty (non-nil) map when kpi_calls has no rows.
func (s *SQLiteStore) LatestKPICallByFunc(ctx context.Context) (map[string]time.Time, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	rows, err := s.db.QueryContext(ctx, latestKPICallByFuncSQL)
	if err != nil {
		return nil, fmtError("LatestKPICallByFunc", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]time.Time)
	for rows.Next() {
		var fn, ts string
		if err := rows.Scan(&fn, &ts); err != nil {
			return nil, fmtError("LatestKPICallByFunc", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmtError("LatestKPICallByFunc", err)
		}
		out[fn] = parsed.UTC()
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("LatestKPICallByFunc", err)
	}
	return out, nil
}
