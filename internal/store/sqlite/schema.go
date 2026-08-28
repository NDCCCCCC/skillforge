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
	"fmt"
)

// schemaSQL creates the tables the store owns. CREATE TABLE IF NOT
// EXISTS makes migrate() idempotent — safe to call on every New().
//
// audit (T-3-AUDIT anti-tampering): INSERT + SELECT paths only — there
// is NO row-modification path for audit rows (the eval sidecar owns the
// one-shot metrics backfill, the only sanctioned exception).
// The four Ragas metric columns are NULL until the sidecar backfills them.
//
// audit_meta (Plan 10-02): sibling-table INSERT-only audit row that
// carries the rewriter / classifier / history-projection signals
// alongside the canonical audit row. 1:1 join via query_id (FK to
// audit.id) + ON DELETE CASCADE. The columns are documented in the
// AuditMeta godoc (internal/store/store.go).
//
// session_turn: the UNIQUE(session_id, role, ts) constraint is the
// AppendTurn idempotency mechanism — a replayed (sessionID, user.Ts,
// assistant.Ts) tuple hits ON CONFLICT DO NOTHING on both INSERTs.
//
// checkpoint (Plan 08-02): opaque eino CheckPointStore bytes. INSERT +
// SELECT passthrough — the store never parses the payload; the only
// write is the id-keyed upsert (ON CONFLICT(id) DO UPDATE) so a
// re-interrupted run replaces its own checkpoint. session_id / skill_id
// / expires_at are reserved for the Phase 11 lifecycle (bind-to-session,
// TTL) — this phase neither writes nor filters on them (all-NULL rows
// are indistinguishable from having no filter).
const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    ts               DATETIME NOT NULL,
    session_id       TEXT    NOT NULL,
    user_id          TEXT,
    skill_id         TEXT    NOT NULL,
    query            TEXT    NOT NULL,
    prompt_hash      TEXT    NOT NULL,
    model_version    TEXT    NOT NULL,
    verdict_json     TEXT    NOT NULL,
    refusal_reason   TEXT    NOT NULL DEFAULT '',
    chunk_ids        TEXT    NOT NULL DEFAULT '',
    action           TEXT    NOT NULL DEFAULT '',
    skill_version    TEXT    NOT NULL DEFAULT '',
    frozen_args      BLOB,
    tool_call_id     TEXT    NOT NULL DEFAULT '',
    approval_id      TEXT    NOT NULL DEFAULT '',
    scope_session_id TEXT    NOT NULL DEFAULT '',
    budget_layer     TEXT    NOT NULL DEFAULT '',
    faithfulness     REAL,
    context_precision REAL,
    answer_relevance  REAL,
    context_recall    REAL,
    -- Phase 13 / Plan 13-03 — P13-L6 audit RunPath/Transfer columns
    -- (FAN-04 SC-4 audit substrate). The 3 nullable TEXT columns are
    -- populated for action_fanout_dispatched | action_fanout_completed
    -- | action_fanout_failed rows to identify the slash-separated
    -- agent-path (run_path = parent_run/sub_query_id) + the transfer
    -- endpoints (transfer_from = parent_run, transfer_to = sub_query_id).
    -- Pre-existing rows have NULL in these columns (additive migration
    -- preserves history — T-13-03-7 mitigation).
    run_path         TEXT,
    transfer_from    TEXT,
    transfer_to      TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_session_id ON audit(session_id);
CREATE INDEX IF NOT EXISTS idx_audit_skill_id   ON audit(skill_id);
CREATE INDEX IF NOT EXISTS idx_audit_refusal    ON audit(refusal_reason);
CREATE INDEX IF NOT EXISTS idx_audit_ts         ON audit(ts);

CREATE TABLE IF NOT EXISTS audit_meta (
    query_id                 INTEGER PRIMARY KEY,
    rewritten_query          TEXT,
    classifier_decision      TEXT    NOT NULL,
    classifier_reason        TEXT,
    classifier_confidence    REAL,
    history_projected_n_turns INTEGER NOT NULL DEFAULT 0,
    history_budget_truncated  INTEGER NOT NULL DEFAULT 0,
    history_dropped_count     INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (query_id) REFERENCES audit(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_audit_meta_decision ON audit_meta(classifier_decision);

CREATE TABLE IF NOT EXISTS session_turn (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL,
    role        TEXT    NOT NULL CHECK(role IN ('user','assistant')),
    content     TEXT    NOT NULL,
    ts          DATETIME NOT NULL,
    UNIQUE(session_id, role, ts)
);
CREATE INDEX IF NOT EXISTS idx_session_turn_sid ON session_turn(session_id);

-- session_meta (session-management phase): optional per-session metadata
-- override. A row exists ONLY after a user rename — the authoritative
-- session roster is derived from session_turn (ListSessions aggregates
-- turns + LEFT JOINs this table). Deleting a session drops both its
-- turns and its meta row; audit rows are NEVER touched (T-3-AUDIT).
CREATE TABLE IF NOT EXISTS session_meta (
    id    TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS checkpoint (
    id            TEXT PRIMARY KEY,
    session_id    TEXT NOT NULL DEFAULT '',
    skill_id      TEXT NOT NULL DEFAULT '',
    skill_version TEXT NOT NULL DEFAULT '',
    prompt_hash   TEXT NOT NULL DEFAULT '',
    payload       BLOB NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at    TEXT
);

-- approval_rules (Plan 11-05 / HITL-08 / D11-D2): the independent
-- approve-and-remember rule table. PK is the full rule key
-- (target, args_hash, skill_id, scope_session_id) per D11-D1/D11-D2.
-- D11-D3 binary scope: a session-scope rule has scope_session_id set to
-- the originating session; a skill-scope rule has scope_session_id = ''
-- with skill_id set to the Skill identity. There is NO global scope.
-- Revoke DELETEs the row here and INSERTs an action_rule_revoked audit
-- row in the same transaction (D11-D4 + T-3-AUDIT INSERT-only).
CREATE TABLE IF NOT EXISTS approval_rules (
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

-- plan_todos (Phase 12 / Plan 12-02 / P12-L5 / P12-L4): the SEPARATE
-- INSERT-only audit table that backs the DeepAgent write_todos Tool
-- (internal/agent/deep_audit.go::writeTodosAuditedTool). This table is
-- INTENTIONALLY DISTINCT from audit.action — replay-query is the hot
-- path per PlanReplayView render (12-07), and a dedicated table keeps
-- the SELECT lean (no WHERE filter, no audit row JOINs).
--
-- Schema rationale:
--   - PRIMARY KEY (run_id, todo_id): idempotent upserts (re-writing
--     the same todo updates content + status + ts via INSERT OR
--     REPLACE semantics, last-write-wins for replay consistency).
--   - op: 'upsert' | 'complete' discriminator — replay reconstructs
--     todo state transitions from this column (a 'complete' row
--     records a "this todo was finished" timestamp; future phases
--     may query the gap between upsert and complete).
--   - active_form: optional plan_todos schema field (mirrors eino's
--     deep.TODO.ActiveForm; FE renders the active step indicator).
--   - ts: epoch-seconds; index-less ORDER BY ts ASC keeps the replay
--     scan cheap for typical run sizes (<200 todos per run).
--   - NO ALTER on existing tables — additive migration; existing data
--     is untouched (P12-L5: separate table, not extension).
CREATE TABLE IF NOT EXISTS plan_todos (
    run_id      TEXT NOT NULL,
    todo_id     TEXT NOT NULL,
    content     TEXT NOT NULL,
    active_form TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL,
    ts          INTEGER NOT NULL,
    op          TEXT NOT NULL,
    PRIMARY KEY (run_id, todo_id)
);
CREATE INDEX IF NOT EXISTS idx_plan_todos_run_id_ts ON plan_todos(run_id, ts);

-- datasets (Phase 14 / Plan 14-06 / D14-D2): the dataset registry for the
-- data-support track (DuckDB analytic datasets). Metadata lives in the SAME
-- sqlite database as audit/checkpoint so registration participates in OLTP
-- transaction semantics (D14-D2: DuckDB central catalog was rejected because
-- cross-engine transactional consistency cannot be guaranteed).
--
-- Schema rationale:
--   - PRIMARY KEY (name): one registry row per dataset; the import pipeline
--     commits via INSERT OR REPLACE (PutDataset) so replays are idempotent.
--   - tier: the trust tier the dataset is exposed under ("权威"/"历史"/"个人")
--     — same vocabulary as the Skill reads field.
--   - schema_json: table/column descriptor JSON for admin/preview surfaces.
--   - file_path: on-disk location under data/datasets/ (the .duckdb file the
--     lazy ATTACH layer mounts; written by CommitStagedDataset).
--   - content_sha256: content hash for integrity checks + TOCTOU detection
--     (Phase 19 pattern).
--   - batch_id: the import batch that produced the row — replay detection
--     and audit trail.
CREATE TABLE IF NOT EXISTS datasets (
    name            TEXT PRIMARY KEY,
    tier            TEXT NOT NULL,
    schema_json     TEXT NOT NULL,
    file_path       TEXT NOT NULL,
    content_sha256  TEXT NOT NULL,
    batch_id        TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    dataset         TEXT NOT NULL DEFAULT '',
    period          TEXT NOT NULL DEFAULT '',
    k_min_override  INTEGER NOT NULL DEFAULT 0
);

-- import_batches (Phase 15 / Plan 15-02 / DATA-17): the SEPARATE
-- INSERT-only batch-audit table that proves raw data was discarded
-- (discard_ts) and carries the rule-set hash (rules_sha256) for
-- reproducibility (D15-A4). The supersedes_batch_id field chains
-- monthly replacement batches (D15-C1). This table is INTENTIONALLY
-- DISTINCT from audit.action — batch-audit queries are the hot path
-- for the admin import view (ADM-02), and a dedicated table keeps the
-- SELECT lean and SQL-first.
--
-- INSERT-only discipline: rows are appended by PutImportBatch; there
-- is NO UPDATE / DELETE path. Replays of the same batch_id surface as
-- a PRIMARY KEY constraint error, never silent replacement.
CREATE TABLE IF NOT EXISTS import_batches (
    batch_id                TEXT PRIMARY KEY,
    dataset                 TEXT NOT NULL,
    period                  TEXT NOT NULL,
    tier                    TEXT NOT NULL,
    content_sha256          TEXT NOT NULL,
    rules_sha256            TEXT NOT NULL,
    raw_size_bytes          INTEGER NOT NULL,
    desensitized_row_count  INTEGER NOT NULL,
    discard_ts              TEXT NOT NULL,
    supersedes_batch_id     TEXT NOT NULL DEFAULT '',
    created_at              TEXT NOT NULL
);

-- dataset_deletes (Phase 15 / Plan 15-02 / D15-C3): INSERT-only audit
-- of dataset deletion actions. The state-side deletion (datasets row
-- + file) is performed by the delete endpoint; this table records the
-- action for compliance replay.
CREATE TABLE IF NOT EXISTS dataset_deletes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    dataset     TEXT NOT NULL DEFAULT '',
    deleted_at  TEXT NOT NULL
);

-- kpi_calls (Phase 16 / Plan 16-03 / KPI-02 / D16-D4): the SEPARATE
-- INSERT-only audit table that freezes EVERY KPI invocation as ONE full
-- row — {func, args, as_of, result_snapshot, k_min_applied, ts,
-- session/skill} — so that even after the upstream DuckDB dataset is
-- superseded or cleaned (dataset_deletes), the exact call-time answer
-- stays replayable from this row alone. This is the storage-layer half
-- of KPI-02; the replay BEHAVIOR itself lives in the 16-06 executor.
--
-- Schema rationale:
--   - SEPARATE INSERT-only KPI-call audit: the large result_snapshot
--     JSON deliberately does NOT enter the chat audit main table
--     (T-16-03-03 — keeping multi-KB snapshots out shrinks the main
--     table's exposure surface and keeps its queries lean).
--   - ADM-03 SQL-first admin reads (Phase 20): "calls of one func" /
--     "calls of one session" are indexed column lookups, and the
--     "last call time per func" view is ONE GROUP BY over this table —
--     no JOIN against the chat audit main table.
--   - id AUTOINCREMENT mirrors dataset_deletes: insertion order IS
--     replay order; ORDER BY id DESC returns newest-first.
--   - signature NOT NULL DEFAULT '': the registry definition hash
--     (D16-A4) recorded so a replay can prove WHICH definition version
--     produced the snapshot. Empty denotes rows written before the
--     executor started stamping it (defensive default; the executor
--     always sets it).
--
-- INSERT-only discipline: rows are appended by PutKPICall; there is NO
-- mutation or removal path anywhere in kpi_calls.go (source-grep
-- structural lock in kpi_calls_test.go, mirroring the import_batches
-- anti-tampering posture).
CREATE TABLE IF NOT EXISTS kpi_calls (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              TEXT    NOT NULL,
    session_id      TEXT    NOT NULL,
    skill_id        TEXT    NOT NULL,
    func            TEXT    NOT NULL,
    args_json       TEXT    NOT NULL,
    as_of_period    TEXT    NOT NULL,
    k_min_applied   INTEGER NOT NULL,
    result_snapshot TEXT    NOT NULL,
    signature       TEXT    NOT NULL DEFAULT ''
);

-- reports (Phase 18 / Plan 18-02 / RPT-04): INSERT-only metadata table for
-- published monthly reports. Mirrors kpi_calls posture (append-only audit).
-- GET /reports/* serves the rendered HTML; this table records metadata only.
CREATE TABLE IF NOT EXISTS reports (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    ts                  TEXT    NOT NULL,
    period              TEXT    NOT NULL,
    skill_id            TEXT    NOT NULL,
    version             INTEGER NOT NULL,
    title               TEXT    NOT NULL,
    file_path           TEXT    NOT NULL,
    approval_audit_id   TEXT    NOT NULL,
    kpi_call_ids        TEXT    NOT NULL DEFAULT '[]',
    template_version    TEXT    NOT NULL DEFAULT '',
    result_digest       TEXT    NOT NULL DEFAULT ''
);
`

// HITL-04 audit lifecycle + D11-C2 snapshot dual-table DDL extensions.
//
// Fresh databases get these columns straight from schemaSQL's CREATE
// TABLE above (the source of truth). Databases created before Phase 11
// get them via the ALTER TABLE statements below, each gated by a
// PRAGMA table_info() column-existence check — the SAME idempotency
// posture as CREATE TABLE IF NOT EXISTS, so migrate() stays safe to
// call on every New().
//
// The `action` column is the HITL-04 lifecycle enum. Allowed values:
// action_pending_approval | action_approved | action_rejected |
// action_rule_created | action_rule_revoked | the empty string (rows that pre-date
// the column, plus every verdict row written by PutAudit). NOT NULL
// DEFAULT empty-string keeps the migration non-destructive for existing rows.
const (
	alterAuditAddSkillVersionSQL   = "ALTER TABLE audit ADD COLUMN skill_version TEXT NOT NULL DEFAULT ''"
	alterAuditAddActionSQL         = "ALTER TABLE audit ADD COLUMN action TEXT NOT NULL DEFAULT ''"
	alterAuditAddFrozenArgsSQL     = "ALTER TABLE audit ADD COLUMN frozen_args BLOB"
	alterAuditAddToolCallIDSQL     = "ALTER TABLE audit ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT ''"
	alterAuditAddApprovalIDSQL     = "ALTER TABLE audit ADD COLUMN approval_id TEXT NOT NULL DEFAULT ''"
	alterAuditAddScopeSessionIDSQL = "ALTER TABLE audit ADD COLUMN scope_session_id TEXT NOT NULL DEFAULT ''"

	// Phase 12 / Plan 12-05 — PLAN-04 budget event discriminator column.
	// Nullable TEXT (NOT NULL DEFAULT ''); populated ONLY for
	// action_budget_triggered rows. Idempotent migration via the same
	// PRAGMA table_info() gate as the existing audit columns — running
	// this on a fresh DB is a no-op because schemaSQL already created
	// the column.
	alterAuditAddBudgetLayerSQL = "ALTER TABLE audit ADD COLUMN budget_layer TEXT NOT NULL DEFAULT ''"

	// Phase 13 / Plan 13-03 — P13-L6 audit RunPath/Transfer columns.
	// Nullable TEXT (NULL-allowed, no DEFAULT) — populated for the
	// action_fanout_* lifecycle rows. Pre-existing rows have NULL in
	// these columns (additive migration — T-13-03-7 mitigation).
	alterAuditAddRunPathSQL      = "ALTER TABLE audit ADD COLUMN run_path TEXT"
	alterAuditAddTransferFromSQL = "ALTER TABLE audit ADD COLUMN transfer_from TEXT"
	alterAuditAddTransferToSQL   = "ALTER TABLE audit ADD COLUMN transfer_to TEXT"

	// Phase 14 / Plan 14-02 — PLAT-10 platform-floor version column.
	// NOT NULL DEFAULT '' mirrors the budget_layer precedent; empty
	// string denotes pre-floor era rows.
	alterAuditAddPlatformFloorVersionSQL = "ALTER TABLE audit ADD COLUMN platform_floor_version TEXT NOT NULL DEFAULT ''"

	// Phase 15 / Plan 15-02 — DATA-16 idempotency key + D15-C2 period
	// + D15-D3 k_min override. NOT NULL DEFAULT '' / 0 keeps the
	// migration additive for existing datasets rows; the import pipeline
	// populates the columns on every new/updated row.
	alterDatasetsAddDatasetSQL        = "ALTER TABLE datasets ADD COLUMN dataset TEXT NOT NULL DEFAULT ''"
	alterDatasetsAddPeriodSQL         = "ALTER TABLE datasets ADD COLUMN period TEXT NOT NULL DEFAULT ''"
	alterDatasetsAddKMinOverrideSQL   = "ALTER TABLE datasets ADD COLUMN k_min_override INTEGER NOT NULL DEFAULT 0"

	alterCheckpointAddSkillVersionSQL = "ALTER TABLE checkpoint ADD COLUMN skill_version TEXT NOT NULL DEFAULT ''"
	alterCheckpointAddPromptHashSQL   = "ALTER TABLE checkpoint ADD COLUMN prompt_hash TEXT NOT NULL DEFAULT ''"
)

// createAuditToolCallIDPendingUniqueIdxSQL is the D11-C4 idempotency
// moat: approval_id == tool_call_id is a SINGLE identity, and only ONE
// action_pending_approval row may exist per tool_call_id. Two devices
// racing the same approval both attempt the INSERT; exactly one wins,
// the other gets a UNIQUE constraint error.
//
// The partial WHERE clause scopes the uniqueness to the pending row so
// the downstream lifecycle rows (approved / rejected / rule_revoked)
// stay free to carry the same tool_call_id — that is what makes the
// audit chain joinable.
//
// CREATE UNIQUE INDEX IF NOT EXISTS is idempotent at the SQLite level;
// no PRAGMA gating needed.
const createAuditToolCallIDPendingUniqueIdxSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_pending_tcid
ON audit(tool_call_id) WHERE action = 'action_pending_approval'
`

// createAuditRunPathIdxSQL is the Phase 13 / Plan 13-03 P13-L6 fanout
// trace reconstruction index. QueryFanoutByRunPath uses LIKE
// 'parent_run/%' for the trace query; the index keeps the prefix scan
// O(log N) instead of a full table scan.
//
// Executed in migrate() AFTER the audit ALTERs — same ordering contract
// as createAuditToolCallIDPendingUniqueIdxSQL (UAT 14-09: on a legacy
// DB the run_path column is added by an ALTER, so an index created
// before the ALTER loop dies with "no such column: run_path").
//
// CREATE INDEX IF NOT EXISTS is idempotent at the SQLite level; no
// PRAGMA gating needed.
const createAuditRunPathIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_audit_run_path ON audit(run_path)
`

// createImportBatchesDatasetPeriodIdxSQL is the Phase 15 / Plan 15-02
// DATA-16 idempotency query index (LatestImportBatch). Created AFTER the
// datasets ALTERs so a migrated DB has the dataset / period columns by
// the time the index references them (D14-E1 ordering contract, UAT
// 14-09 blocker precedent).
const createImportBatchesDatasetPeriodIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_import_batches_dataset_period ON import_batches(dataset, period)
`

// kpi_calls admin read-path indexes (Phase 16 / Plan 16-03 / ADM-03).
// Three independent consts executed at the TAIL of migrate() — the same
// shape discipline as idx_import_batches_dataset_period. The brand-new
// kpi_calls table has no ALTER dependency (additive CREATE TABLE IF NOT
// EXISTS in schemaSQL is enough), but every post-schemaSQL index stays
// in the migrate()-tail convention so the D14-E1 ordering contract has
// exactly one shape to remember.
//
//   - idx_kpi_calls_func:       "calls of one KPI" list reads.
//   - idx_kpi_calls_session_id: per-conversation KPI audit trail.
//   - idx_kpi_calls_ts:         time-window scans for the admin view.
const (
	createKPICallsFuncIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_kpi_calls_func ON kpi_calls(func)
`

	createKPICallsSessionIDIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_kpi_calls_session_id ON kpi_calls(session_id)
`

	createKPICallsTsIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_kpi_calls_ts ON kpi_calls(ts)
`
)

// reports (Phase 18 / Plan 18-02 / RPT-04): read-path indexes for
// ListReports (by period) and ListReportsBySkill (by skill_id).
const (
	createReportsPeriodIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_reports_period ON reports(period)
`
	createReportsSkillIDIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_reports_skill_id ON reports(skill_id)
`
)

// auditLifecycleColumns / checkpointSnapshotColumns pair each new column
// with the ALTER that adds it, so migrate() can loop instead of
// repeating the same gate eight times.
var auditLifecycleColumns = []struct{ column, stmt string }{
	{"action", alterAuditAddActionSQL},
	{"skill_version", alterAuditAddSkillVersionSQL},
	{"frozen_args", alterAuditAddFrozenArgsSQL},
	{"tool_call_id", alterAuditAddToolCallIDSQL},
	{"approval_id", alterAuditAddApprovalIDSQL},
	{"scope_session_id", alterAuditAddScopeSessionIDSQL},
	// Phase 12 / Plan 12-05 — budget event discriminator column.
	{"budget_layer", alterAuditAddBudgetLayerSQL},
	// Phase 13 / Plan 13-03 — P13-L6 audit RunPath/Transfer columns.
	{"run_path", alterAuditAddRunPathSQL},
	{"transfer_from", alterAuditAddTransferFromSQL},
	{"transfer_to", alterAuditAddTransferToSQL},
	// Phase 14 / Plan 14-02 — PLAT-10 platform-floor version column.
	{"platform_floor_version", alterAuditAddPlatformFloorVersionSQL},
}

var checkpointSnapshotColumns = []struct{ column, stmt string }{
	{"skill_version", alterCheckpointAddSkillVersionSQL},
	{"prompt_hash", alterCheckpointAddPromptHashSQL},
}

// datasetImportColumns pairs each Phase 15 additive datasets column with
// the ALTER that adds it, so migrate() can loop instead of repeating the
// same PRAGMA gate three times. On a fresh DB these are no-ops because
// schemaSQL already created the columns.
var datasetImportColumns = []struct{ column, stmt string }{
	{"dataset", alterDatasetsAddDatasetSQL},
	{"period", alterDatasetsAddPeriodSQL},
	{"k_min_override", alterDatasetsAddKMinOverrideSQL},
}

// columnExists reports whether the named column is already present in
// table. PRAGMA table_info() is a read-only metadata query — it never
// blocks and never mutates. The table name is NOT caller-controlled:
// every call site below passes a hardcoded literal ("audit" /
// "checkpoint"), so the fmt.Sprintf here never sees user input
// (T-3-SQLI posture — PRAGMA does not accept `?` placeholders for the
// table name, which is why this one statement is formatted).
func (s *SQLiteStore) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmtError("columnExists", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid          int
			name, ctype  string
			notnull, pk  int
			defaultValue sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &defaultValue, &pk); err != nil {
			return false, fmtError("columnExists.Scan", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmtError("columnExists.Rows", err)
	}
	return false, nil
}

// migrate runs schemaSQL against the store's *sql.DB, then applies the
// Phase 11 idempotent ALTER TABLE extensions and the D11-C4 partial
// unique index. Idempotent — safe to call on every New(). Wrapped in
// ExecContext so a cancelled ctx surfaces promptly (AGNT-04).
func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmtError("migrate", err)
	}

	// HITL-04 audit lifecycle columns (no-op on a fresh DB — schemaSQL
	// already created them).
	for _, c := range auditLifecycleColumns {
		exists, err := s.columnExists(ctx, "audit", c.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.db.ExecContext(ctx, c.stmt); err != nil {
			return fmtError("migrate.audit."+c.column, err)
		}
	}

	// D11-C2 checkpoint snapshot columns.
	for _, c := range checkpointSnapshotColumns {
		exists, err := s.columnExists(ctx, "checkpoint", c.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.db.ExecContext(ctx, c.stmt); err != nil {
			return fmtError("migrate.checkpoint."+c.column, err)
		}
	}

	// Phase 15 / Plan 15-02 — datasets additive columns for the import
	// pipeline (DATA-16 idempotency key + period + k_min override).
	for _, c := range datasetImportColumns {
		exists, err := s.columnExists(ctx, "datasets", c.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.db.ExecContext(ctx, c.stmt); err != nil {
			return fmtError("migrate.datasets."+c.column, err)
		}
	}

	// D11-C4 partial unique index. Created AFTER the ALTERs so a
	// migrated DB has the tool_call_id column by the time the index
	// references it.
	if _, err := s.db.ExecContext(ctx, createAuditToolCallIDPendingUniqueIdxSQL); err != nil {
		return fmtError("migrate.audit.tcid_unique", err)
	}

	// P13-L6 fanout trace index. Created AFTER the ALTERs so a migrated
	// (pre-Phase-12 legacy) DB has the run_path column by the time the
	// index references it (UAT 14-09 blocker).
	if _, err := s.db.ExecContext(ctx, createAuditRunPathIdxSQL); err != nil {
		return fmtError("migrate.audit.run_path_idx", err)
	}

	// Phase 15 / Plan 15-02 — import_batches (dataset, period) index.
	// Created AFTER the datasets ALTERs so a legacy DB has the columns
	// referenced by the index. Fresh DBs already have the columns from
	// schemaSQL and the index from this statement.
	if _, err := s.db.ExecContext(ctx, createImportBatchesDatasetPeriodIdxSQL); err != nil {
		return fmtError("migrate.import_batches.dataset_period_idx", err)
	}

	// Phase 16 / Plan 16-03 — kpi_calls admin read-path indexes (func /
	// session_id / ts). Executed AFTER the ALTER loops per the D14-E1
	// ordering posture; kpi_calls itself is brand-new (additive, no
	// ALTER dependency) but keeps the same migrate()-tail convention.
	if _, err := s.db.ExecContext(ctx, createKPICallsFuncIdxSQL); err != nil {
		return fmtError("migrate.kpi_calls.func_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createKPICallsSessionIDIdxSQL); err != nil {
		return fmtError("migrate.kpi_calls.session_id_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createKPICallsTsIdxSQL); err != nil {
		return fmtError("migrate.kpi_calls.ts_idx", err)
	}

	// Phase 18 / Plan 18-02 — reports read-path indexes (period / skill_id).
	// Fresh table (additive CREATE TABLE) has no ALTER dependency; same
	// migrate()-tail convention as kpi_calls.
	if _, err := s.db.ExecContext(ctx, createReportsPeriodIdxSQL); err != nil {
		return fmtError("migrate.reports.period_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createReportsSkillIDIdxSQL); err != nil {
		return fmtError("migrate.reports.skill_id_idx", err)
	}
	return nil
}
