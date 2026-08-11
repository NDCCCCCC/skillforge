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

import "context"

// schemaSQL creates the two tables the store owns. CREATE TABLE IF NOT
// EXISTS makes migrate() idempotent — safe to call on every New().
//
// audit (T-3-AUDIT anti-tampering): INSERT + SELECT paths only — there
// is NO row-modification path anywhere in this package (the eval sidecar
// owns the one-shot metrics backfill, the only sanctioned exception).
// The four Ragas metric columns are NULL until the sidecar backfills them.
//
// session_turn: the UNIQUE(session_id, role, ts) constraint is the
// AppendTurn idempotency mechanism — a replayed (sessionID, user.Ts,
// assistant.Ts) tuple hits ON CONFLICT DO NOTHING on both INSERTs.
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
    faithfulness     REAL,
    context_precision REAL,
    answer_relevance  REAL,
    context_recall    REAL
);
CREATE INDEX IF NOT EXISTS idx_audit_session_id ON audit(session_id);
CREATE INDEX IF NOT EXISTS idx_audit_skill_id   ON audit(skill_id);
CREATE INDEX IF NOT EXISTS idx_audit_refusal    ON audit(refusal_reason);
CREATE INDEX IF NOT EXISTS idx_audit_ts         ON audit(ts);

CREATE TABLE IF NOT EXISTS session_turn (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL,
    role        TEXT    NOT NULL CHECK(role IN ('user','assistant')),
    content     TEXT    NOT NULL,
    ts          DATETIME NOT NULL,
    UNIQUE(session_id, role, ts)
);
CREATE INDEX IF NOT EXISTS idx_session_turn_sid ON session_turn(session_id);
`

// migrate runs schemaSQL against the store's *sql.DB. Idempotent —
// safe to call on every New(). Wrapped in ExecContext so a cancelled
// ctx surfaces promptly (AGNT-04).
func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmtError("migrate", err)
	}
	return nil
}
