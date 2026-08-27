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
