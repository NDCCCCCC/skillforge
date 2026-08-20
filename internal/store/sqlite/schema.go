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
    faithfulness     REAL,
    context_precision REAL,
    answer_relevance  REAL,
    context_recall    REAL
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
}

var checkpointSnapshotColumns = []struct{ column, stmt string }{
	{"skill_version", alterCheckpointAddSkillVersionSQL},
	{"prompt_hash", alterCheckpointAddPromptHashSQL},
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

	// D11-C4 partial unique index. Created AFTER the ALTERs so a
	// migrated DB has the tool_call_id column by the time the index
	// references it.
	if _, err := s.db.ExecContext(ctx, createAuditToolCallIDPendingUniqueIdxSQL); err != nil {
		return fmtError("migrate.audit.tcid_unique", err)
	}
	return nil
}
