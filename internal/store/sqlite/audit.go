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
	"errors"
	"strings"

	"github.com/skillforge/skillforge/internal/store"
)

// putAuditSQL is the single INSERT path for the audit table. There is
// deliberately no row-modification statement anywhere in this package —
// the schema is INSERT + SELECT only (T-3-AUDIT anti-tampering). Every
// value is a `?` placeholder; the row never touches fmt.Sprintf.
//
// Phase 12 / Plan 12-05: budget_layer column added for the PLAN-04
// 3-layer budget gate (internal/agent/callbacks/budget.go). Populated
// ONLY for action_budget_triggered rows; empty string for every other
// row (verdict, HITL-04 lifecycle, plan_todo cross-reference).
const putAuditSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, query, prompt_hash,
    model_version, verdict_json, refusal_reason, chunk_ids,
    action, skill_version, frozen_args, tool_call_id, approval_id,
    scope_session_id, budget_layer,
    faithfulness, context_precision, answer_relevance, context_recall
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// PutAudit appends an immutable audit row. GEVL-03: the row MUST exist
// before the response ends — synchronous write, SQLite WAL keeps this
// sub-ms. Returns the assigned auto-increment ID (>0).
//
// The Phase 11 HITL lifecycle columns (Action / SkillVersion /
// FrozenArgs / ToolCallID / ApprovalID / ScopeSessionID) are additive:
// a verdict row written by the chat path leaves them zero-valued, which
// persists as the empty string (NULL for frozen_args). The
// idx_audit_pending_tcid partial unique index only constrains rows with
// action='action_pending_approval', so a stream of verdict rows carrying
// action=(empty string) never collides.
//
// Phase 12 / Plan 12-05: rows with action="action_budget_triggered" MUST
// carry a BudgetLayer value in BudgetLayerNames (T-12-05-6 spoofing
// guard). Empty BudgetLayer on a non-budget row is the canonical state
// (the column is nullable; only budget events populate it).
func (s *SQLiteStore) PutAudit(ctx context.Context, row store.AuditRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	if row.Action == ActionBudgetTriggered && !BudgetLayerNames[row.BudgetLayer] {
		// T-12-05-6 spoofing guard — reject unknown budget_layer values.
		// The audit table is the durable record of what happened; an
		// invalid layer value would let an adversary understate or
		// misrepresent the budget event (e.g. "loop" mis-recorded as
		// "denied" to skip the loop anomaly detector).
		return 0, ErrInvalidBudgetLayer
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
		row.Action,
		row.SkillVersion,
		row.FrozenArgs,
		row.ToolCallID,
		row.ApprovalID,
		row.ScopeSessionID,
		row.BudgetLayer, // 12-05: empty for non-budget rows; validator rejects unknown layer values
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
    action, skill_version, frozen_args, tool_call_id, approval_id,
    scope_session_id, budget_layer,
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
	if filter.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.ToolCallID != "" {
		clauses = append(clauses, "tool_call_id = ?")
		args = append(args, filter.ToolCallID)
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
	// Phase 6 / D6-05: LIMIT/OFFSET pagination. The OFFSET value is
	// appended to args as a `?` placeholder — NEVER fmt.Sprintf'd into
	// the SQL string (T-3-SQLI). The existing DROP TABLE regression
	// (store_test.go::TestAuditRowSQLInjection) MUST stay green; the
	// payload is treated as a literal string by the parameterized
	// driver, never as SQL.
	if filter.Offset > 0 {
		q += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmtError("QueryAudit", err)
	}
	defer rows.Close()

	var out []store.AuditRow
	for rows.Next() {
		var (
			r         store.AuditRow
			faith     sql.NullFloat64
			ctxPrec   sql.NullFloat64
			ansRel    sql.NullFloat64
			ctxRecall sql.NullFloat64
		)
		if err := rows.Scan(
			&r.ID, &r.Timestamp, &r.SessionID, &r.UserID, &r.SkillID,
			&r.Query, &r.PromptHash, &r.ModelVersion, &r.VerdictJSON,
			&r.RefusalReason, &r.ChunkIDs,
			&r.Action, &r.SkillVersion, &r.FrozenArgs, &r.ToolCallID,
			&r.ApprovalID, &r.ScopeSessionID, &r.BudgetLayer,
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

// updateAuditMetricsSQL is the ONLY row-modification statement in the
// store package. It is owned by Plan 03-06's eval sidecar — the
// exception to the INSERT-only T-3-AUDIT invariant. The columns already
// exist (Plan 03-02 reserved them); the eval sidecar sets them after
// the chat path INSERTed the row with NULLs.
//
// Each metric argument is a `?` placeholder (T-3-SQLI: no fmt.Sprintf
// with input). The COALESCE keeps a NULL column NULL when the caller
// passes nil for that metric (the sidecar may compute only 3 of 4
// metrics for a refusal case — the 4th should stay NULL, not get
// overwritten with a spurious 0).
const updateAuditMetricsSQL = `
UPDATE audit SET
    faithfulness      = COALESCE(?, faithfulness),
    context_precision = COALESCE(?, context_precision),
    answer_relevance  = COALESCE(?, answer_relevance),
    context_recall    = COALESCE(?, context_recall)
WHERE id = ?
`

// UpdateAuditMetrics backfills the 4 metric columns on audit row id.
// See store.Store.UpdateAuditMetrics for the contract; see
// updateAuditMetricsSQL above for the SQL shape (COALESCE preserves
// prior values for nil metrics).
//
// Returns sql.ErrNoRows when the row does not exist — the driver-level
// rows-affected check surfaces this as ErrNoRows via res.RowsAffected()
// (the eval CLI treats this as a sequencing error and logs it).
func (s *SQLiteStore) UpdateAuditMetrics(ctx context.Context, id int64, faithfulness, contextPrecision, answerRelevance, contextRecall *float64) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, updateAuditMetricsSQL,
		toNullFloat64(faithfulness),
		toNullFloat64(contextPrecision),
		toNullFloat64(answerRelevance),
		toNullFloat64(contextRecall),
		id,
	)
	if err != nil {
		return fmtError("UpdateAuditMetrics", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmtError("UpdateAuditMetrics.RowsAffected", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// fromNullFloat64 is the scan-side inverse of toNullFloat64.
func fromNullFloat64(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

// putAuditMetaSQL is the single INSERT path for the audit_meta sibling
// table. Mirrors putAuditSQL's posture: ?-placeholders only (T-3-SQLI);
// no row-modification statement exists anywhere in the package
// (T-3-AUDIT INSERT-only invariant). The query_id FK ties meta
// lifecycle to the audit row; ON DELETE CASCADE removes the meta row
// when the audit row is removed by a future operator action.
//
// The classifier_decision column is NOT NULL with the "n/a" sentinel
// for nil-gated classifier paths (a value must always be present so
// spike / audit queries don't need NULL-handling logic).
const putAuditMetaSQL = `
INSERT INTO audit_meta (
    query_id, rewritten_query, classifier_decision, classifier_reason,
    classifier_confidence, history_projected_n_turns, history_budget_truncated,
    history_dropped_count
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`

// PutAuditMeta appends a sibling-table audit row that carries the
// rewriter / classifier / history-projection signals alongside the
// canonical audit row. The relationship is 1:1 by query_id (the
// audit row's auto-increment ID). When the audit row is removed the
// meta row drops via ON DELETE CASCADE — the lifecycle tie is the
// database's job, not the caller's.
//
// The QueryID MUST be a valid audit row id; otherwise the FK
// constraint fires a constraint error from the driver (the caller
// treats this as a sequencing bug — PutAudit must have returned the
// ID before PutAuditMeta is called). The classifier_decision MUST be
// one of "allow" | "refuse" | "n/a" — other values are coerced to
// "n/a" by the DB-layer default-check or rejected by the operator.
//
// Nullable column handling: empty-string RewrittenQuery and
// ClassifierReason are persisted as SQL NULL so spike queries can
// distinguish "no rewrite happened" from "rewrote to empty string".
// ClassifierConfidence follows toNullFloat64 (the canonical pattern
// across the store package — see UpdateAuditMetrics).
//
// On error the function wraps via fmtError so the caller can
// distinguish audit_meta failures from audit failures in the log.
func (s *SQLiteStore) PutAuditMeta(ctx context.Context, meta store.AuditMeta) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	_, err := s.db.ExecContext(ctx, putAuditMetaSQL,
		meta.QueryID,
		toNullString(meta.RewrittenQuery),
		meta.ClassifierDecision,
		toNullString(meta.ClassifierReason),
		toNullFloat64(meta.ClassifierConfidence),
		meta.HistoryProjectedNTurns,
		meta.HistoryBudgetTruncated,
		meta.HistoryDroppedCount,
	)
	if err != nil {
		return fmtError("PutAuditMeta", err)
	}
	return nil
}

// toNullString converts an empty string to sql.NullString{Valid:false}
// so the DB column receives NULL rather than "". Keeps the nullable
// contract uniform across PutAuditMeta's call sites (no caller-side
// sql.NullString wrapping needed).
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ---------------------------------------------------------------------
// HITL-04 audit lifecycle (INSERT-only, 3-row chain)
// ---------------------------------------------------------------------
//
// The canonical order is:
//
//	InsertPendingApproval → MarkApproved | MarkRejected
//	MarkRuleRevoked (D11-D2 rule revoke; a chain of its own)
//
// Every stage writes a NEW row — there is no UPDATE / DELETE path for
// audit rows (T-3-AUDIT; UpdateAuditMetrics stays the single sanctioned
// exception, owned by the eval sidecar). The pending row carries the
// frozen_args byte snapshot plus the tool_call_id identity that the
// idx_audit_pending_tcid partial unique index constrains (D11-C4).
//
// D11-C3 fail-closed is the CALLER's posture — see
// internal/agent/hitl/audit_writer.go. These methods simply return the
// driver error; they never retry and never swallow.
//
// All values are `?` placeholders (T-3-SQLI).

// Action enum constants (Phase 11 + Phase 12 expansion). The audit
// action column is TEXT — the values are STRING literals persisted
// verbatim, NOT a typed enum. Storing them as named constants here is
// for cross-package readability (the chat path / server layer /
// audit_writer seam import these so the row comparison stays
// syntactic instead of a typo-prone literal comparison).
//
// Phase 11 (D11-C3 / HITL-04 lifecycle) owns:
//
//	ActionPendingApproval  → ActionApproved | ActionRejected
//	ActionRuleRevoked      (D11-D2)
//
// Phase 12 / Plan 12-02 adds the plan_todo cross-reference enum
// values:
//
//	ActionPlanTodoUpsert   (parallel to a plan_todos INSERT)
//	ActionPlanTodoComplete (parallel to a plan_todos INSERT with op='complete')
//
// The plan_todos table is the PRIMARY substrate (P12-L5 separate
// table rationale); these audit.action enum values are the SECONDARY
// cross-reference that AuditTimelineTab (12-07) joins on to render
// "wrote todo" events alongside approval/rejection events in the
// timeline. The audit_writer hooks (12-02/12-03) emit these rows
// in lockstep with the plan_todos INSERT.
//
// No DDL change — the audit.action column is already TEXT. Adding
// new string values is a pure catalog extension; old rows keep
// their prior values; old consumers (chat path) are unaffected.
const (
	ActionPendingApproval string = "action_pending_approval"
	ActionApproved        string = "action_approved"
	ActionRejected        string = "action_rejected"
	ActionRuleCreated     string = "action_rule_created"
	ActionRuleRevoked     string = "action_rule_revoked"
	// Phase 12 / Plan 12-02 — plan_todos cross-reference
	ActionPlanTodoUpsert   string = "action_plan_todo_upsert"
	ActionPlanTodoComplete string = "action_plan_todo_complete"

	// Phase 12 / Plan 12-05 — PLAN-04 budget gate audit. Fires when the
	// 3-layer budget middleware (internal/agent/callbacks/budget.go)
	// trips a MaxIteration | Token | Loop threshold. Carries a non-empty
	// BudgetLayer value (one of the keys in BudgetLayerNames below).
	// INSERT-only (T-3-AUDIT); queryable via AuditFilter.Action.
	ActionBudgetTriggered string = "action_budget_triggered"
)

// BudgetLayerNames is the whitelist of valid BudgetLayer values for
// action_budget_triggered rows (Phase 12 / Plan 12-05). The middleware
// (internal/agent/callbacks/budget.go) populates this column with one
// of these three keys; the PutAudit path rejects any other value as a
// T-12-05-6 spoofing guard (the audit row is a durable record — a
// fabricated layer string would let an adversary understate what happened).
//
// T-12-05-6 mitigation posture:
//   - whitelist keys: max_iter | token | loop
//   - any other value → PutAudit returns an error (no row written)
//   - empty string is the canonical "non-budget row" value — verdict
//     rows, HITL-04 lifecycle rows, and plan_todo cross-reference
//     rows leave BudgetLayer = ""
var BudgetLayerNames = map[string]bool{
	"max_iter": true,
	"token":    true,
	"loop":     true,
}

// ErrInvalidBudgetLayer is the sentinel error PutAudit surfaces when
// row.Action == ActionBudgetTriggered but row.BudgetLayer is NOT in
// BudgetLayerNames. The caller (audit_writer seam + the budget
// middleware) treats this as a fail-closed signal: the row is NOT
// written, the budget gate proceeds without the audit (the SSE emit
// is suppressed on the same condition — see D11-C3 audit-first
// posture mirrored in budget.go's BeforeModel hook).
var ErrInvalidBudgetLayer = errors.New("sqlite.PutAudit: action_budget_triggered requires non-empty budget_layer in BudgetLayerNames (max_iter|token|loop)")

const putPendingApprovalSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, action,
    query, prompt_hash, skill_version, model_version,
    verdict_json, refusal_reason, chunk_ids,
    frozen_args, tool_call_id, approval_id, scope_session_id
) VALUES (?, ?, ?, ?, 'action_pending_approval', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

const putApprovedSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, action,
    query, prompt_hash, skill_version, model_version,
    verdict_json, refusal_reason, chunk_ids,
    frozen_args, tool_call_id, approval_id, scope_session_id
) VALUES (?, ?, ?, ?, 'action_approved', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

const putRejectedSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, action,
    query, prompt_hash, skill_version, model_version,
    verdict_json, refusal_reason, chunk_ids,
    frozen_args, tool_call_id, approval_id, scope_session_id
) VALUES (?, ?, ?, ?, 'action_rejected', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

const putRuleRevokedSQL = `
INSERT INTO audit (
    ts, session_id, user_id, skill_id, action,
    query, prompt_hash, skill_version, model_version,
    verdict_json, refusal_reason, chunk_ids,
    frozen_args, tool_call_id, approval_id, scope_session_id
) VALUES (?, ?, ?, ?, 'action_rule_revoked', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// InsertPendingApproval writes the FIRST row of the HITL-04 lifecycle
// (action_pending_approval). It is called SYNCHRONOUSLY by
// internal/agent/hitl/audit_writer.go BEFORE the HITL Tool reaches
// tool.StatefulInterrupt (D11-C3 fail-closed).
//
// row.ToolCallID is the D11-C4 identity (== row.ApprovalID). The
// partial unique index idx_audit_pending_tcid rejects a second pending
// row for the same tool_call_id with a UNIQUE constraint error — that
// error IS the cross-device idempotency signal, so it is returned to
// the caller rather than swallowed.
func (s *SQLiteStore) InsertPendingApproval(ctx context.Context, row store.PendingApprovalRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, putPendingApprovalSQL,
		row.Timestamp.UTC(), row.SessionID, row.UserID, row.SkillID,
		row.Query, row.PromptHash, row.SkillVersion, row.ModelVersion,
		row.VerdictJSON, row.RefusalReason, row.ChunkIDs,
		row.FrozenArgs, row.ToolCallID, row.ApprovalID, row.ScopeSessionID,
	)
	if err != nil {
		return 0, fmtError("InsertPendingApproval", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("InsertPendingApproval.LastInsertId", err)
	}
	return id, nil
}

// MarkApproved writes the SECOND row of the HITL-04 lifecycle
// (action_approved), after the operator approves via the dual endpoint
// (11-04). row.FrozenArgs is the possibly-edited bytes (HITL-07); it is
// byte-identical to the pending row's frozen_args when no edit was
// performed.
func (s *SQLiteStore) MarkApproved(ctx context.Context, row store.ApprovalResolutionRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, putApprovedSQL,
		row.Timestamp.UTC(), row.SessionID, row.UserID, row.SkillID,
		row.Query, row.PromptHash, row.SkillVersion, row.ModelVersion,
		row.VerdictJSON, row.RefusalReason, row.ChunkIDs,
		row.FrozenArgs, row.ToolCallID, row.ApprovalID, row.ScopeSessionID,
	)
	if err != nil {
		return 0, fmtError("MarkApproved", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("MarkApproved.LastInsertId", err)
	}
	return id, nil
}

// MarkRejected mirrors MarkApproved with action='action_rejected'.
// row.RefusalReason carries the operator's reason, which is REQUIRED
// per HITL-02 / D11-B2 — validated at the HTTP boundary (11-04) and by
// the audit writer, not here.
func (s *SQLiteStore) MarkRejected(ctx context.Context, row store.ApprovalResolutionRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, putRejectedSQL,
		row.Timestamp.UTC(), row.SessionID, row.UserID, row.SkillID,
		row.Query, row.PromptHash, row.SkillVersion, row.ModelVersion,
		row.VerdictJSON, row.RefusalReason, row.ChunkIDs,
		row.FrozenArgs, row.ToolCallID, row.ApprovalID, row.ScopeSessionID,
	)
	if err != nil {
		return 0, fmtError("MarkRejected", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("MarkRejected.LastInsertId", err)
	}
	return id, nil
}

// MarkRuleRevoked writes an action_rule_revoked row when an
// approve-and-remember rule is DELETEd (D11-D2). The rule row itself
// lives in the approval_rules table (11-05) and IS deleted there; the
// audit chain only ever grows (T-3-AUDIT INSERT-only).
func (s *SQLiteStore) MarkRuleRevoked(ctx context.Context, row store.RuleRevokeRow) (int64, error) {
	if s.isClosed() {
		return 0, sql.ErrConnDone
	}
	res, err := s.db.ExecContext(ctx, putRuleRevokedSQL,
		row.Timestamp.UTC(), row.SessionID, row.UserID, row.SkillID,
		row.Query, row.PromptHash, row.SkillVersion, row.ModelVersion,
		row.VerdictJSON, row.RefusalReason, row.ChunkIDs,
		row.FrozenArgs, row.ToolCallID, row.ApprovalID, row.ScopeSessionID,
	)
	if err != nil {
		return 0, fmtError("MarkRuleRevoked", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmtError("MarkRuleRevoked.LastInsertId", err)
	}
	return id, nil
}
