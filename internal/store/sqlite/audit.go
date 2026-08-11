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

import (
	"context"
	"database/sql"
	"strings"

	"github.com/skillforge/skillforge/internal/store"
)

// putAuditSQL is the single INSERT path for the audit table. There is
// deliberately no row-modification statement anywhere in this package —
// the schema is INSERT + SELECT only (T-3-AUDIT anti-tampering). Every
// value is a `?` placeholder; the row never touches fmt.Sprintf.
const putAuditSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, query, prompt_hash,
    model_version, verdict_json, refusal_reason, chunk_ids,
    faithfulness, context_precision, answer_relevance, context_recall
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// PutAudit appends an immutable audit row. GEVL-03: the row MUST exist
// before the response ends — synchronous write, SQLite WAL keeps this
// sub-ms. Returns the assigned auto-increment ID (>0).
func (s *SQLiteStore) PutAudit(ctx context.Context, row store.AuditRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, putAuditSQL,
		row.Timestamp.UTC(),
		row.SessionID,
		row.UserID,
		row.SkillID,
		row.Query,
		row.PromptHash,
		row.ModelVersion,
		row.VerdictJSON,
		row.RefusalReason,
		row.ChunkIDs,
		toNullFloat64(row.Faithfulness),
		toNullFloat64(row.ContextPrecision),
		toNullFloat64(row.AnswerRelevance),
		toNullFloat64(row.ContextRecall),
	)
	if err != nil {
		return 0, fmtError("PutAudit", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("PutAudit.LastInsertId", err)
	}
	return id, nil
}

// queryAuditCols enumerates the SELECT column list once so PutAudit and
// QueryAudit can't drift on column order.
const queryAuditCols = `
    id, ts, session_id, user_id, skill_id, query, prompt_hash,
    model_version, verdict_json, refusal_reason, chunk_ids,
    faithfulness, context_precision, answer_relevance, context_recall
`

// QueryAudit returns rows matching filter, newest-first. The WHERE
// clause is built dynamically, but EVERY field is a `?` placeholder —
// no fmt.Sprintf with user input (T-3-SQLI). Empty filter fields are
// omitted from the WHERE so a zero-value filter returns the most-recent
// rows up to Limit (default 100).
func (s *SQLiteStore) QueryAudit(ctx context.Context, filter store.AuditFilter) ([]store.AuditRow, error) {
	if s.isClosed() {
		return nil, sql.ErrConnDone
	}
	var (
		clauses []string
		args    []any
	)
	if filter.UserID != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.SkillID != "" {
		clauses = append(clauses, "skill_id = ?")
		args = append(args, filter.SkillID)
	}
	if filter.RefusalReason != "" {
		clauses = append(clauses, "refusal_reason = ?")
		args = append(args, filter.RefusalReason)
	}
	if filter.SessionID != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if !filter.Since.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, filter.Since.UTC())
	}
	if !filter.Until.IsZero() {
		clauses = append(clauses, "ts < ?")
		args = append(args, filter.Until.UTC())
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	q := "SELECT " + queryAuditCols + " FROM audit"
	if len(clauses) > 0 {
		// strings.Join is safe — the only strings in `clauses` are the
		// hardcoded parameterized literals above, NEVER user input.
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY ts DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmtError("QueryAudit", err)
	}
	defer rows.Close()

	var out []store.AuditRow
	for rows.Next() {
		var (
			r            store.AuditRow
			faith        sql.NullFloat64
			ctxPrec      sql.NullFloat64
			ansRel       sql.NullFloat64
			ctxRecall    sql.NullFloat64
		)
		if err := rows.Scan(
			&r.ID, &r.Timestamp, &r.SessionID, &r.UserID, &r.SkillID,
			&r.Query, &r.PromptHash, &r.ModelVersion, &r.VerdictJSON,
			&r.RefusalReason, &r.ChunkIDs,
			&faith, &ctxPrec, &ansRel, &ctxRecall,
		); err != nil {
			return nil, fmtError("QueryAudit.Scan", err)
		}
		r.Faithfulness = fromNullFloat64(faith)
		r.ContextPrecision = fromNullFloat64(ctxPrec)
		r.AnswerRelevance = fromNullFloat64(ansRel)
		r.ContextRecall = fromNullFloat64(ctxRecall)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmtError("QueryAudit.Rows", err)
	}
	if out == nil {
		out = []store.AuditRow{}
	}
	return out, nil
}

// toNullFloat64 converts a *float64 metric to sql.NullFloat64 so NULL
// is persisted when the metric is unset (the common case at query time;
// the eval sidecar backfills later).
func toNullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Valid: true, Float64: *v}
}

// fromNullFloat64 is the scan-side inverse of toNullFloat64.
func fromNullFloat64(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}
