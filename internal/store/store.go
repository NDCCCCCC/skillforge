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

// Package store defines the abstraction over the audit + session-memory
// backend that every Phase 3 subsystem consumes. Plan 03-05 (server
// integration) writes audit rows and session turns here; Plan 03-06
// (eval sidecar) reads audit rows from here for the JSONL contract.
//
// This file holds ONLY the interface + the row/message/filter value
// types. The SQLite default implementation lives in
// internal/store/sqlite; future backends (Postgres, MySQL) will sit
// alongside it, mirroring the D11 retriever.Retriever + mongo/milvus
// pattern — interface-first, SQLite default, PG-ready drop-in.
package store

import (
	"context"
	"time"
)

// AuditRow is a single immutable compliance-review audit record. Rows
// are INSERT-only (there is deliberately no UPDATE path anywhere in the
// store package — anti-tampering, T-3-AUDIT). The schema is denormalized
// so SC-7's "queryable via SQL filter" holds as FIRST-CLASS columns:
//
//   - ChunkIDs is a CSV of verdict.Citation.ChunkID so a SQL
//     `WHERE chunk_ids LIKE '%<id>%'` finds the row without parsing
//     VerdictJSON. The caller populates it from the verdict's citations
//     at insert time.
//   - RefusalReason is a top-level column so the operator can filter
//     "show every query the floor refused" with a trivial SQL predicate.
//
// The four metrics pointers (Faithfulness etc.) are NULLable: the agent
// writes NULL at query time, the eval sidecar (Plan 03-06) backfills
// them via a one-shot UPDATE — the ONLY exception to the no-UPDATE rule,
// owned by the sidecar.
type AuditRow struct {
	ID int64
	// Timestamp is the wall-clock time of the query (UTC). Indexed.
	Timestamp time.Time
	// SessionID partitions a user's conversation history — the WHERE
	// clause on this column is the SC-9 cross-session isolation boundary.
	SessionID string
	UserID    string
	SkillID   string
	Query     string
	// PromptHash is hex SHA-256 of (rendered system prompt + skill_id +
	// model_version). Collision-resistant to 2^128 (T-3-HASH); the hash
	// input includes skill_id + model_version so cross-skill reuse does
	// not collide. The caller computes the hash; the store just persists
	// the string.
	PromptHash   string
	ModelVersion string
	// VerdictJSON is the JSON-serialized verdict.Verdict for the query.
	VerdictJSON string
	// RefusalReason is "" for an answer or the refusal code
	// ("no_authorized_source", "low_faithfulness", ...) for a refusal.
	RefusalReason string
	// ChunkIDs is a CSV of cited chunk IDs (e.g. "c1,c2,c3").
	ChunkIDs string
	// The four Ragas metrics — NULL until the eval sidecar backfills.
	Faithfulness     *float64
	ContextPrecision *float64
	AnswerRelevance  *float64
	ContextRecall    *float64
}

// SessionMessage is one turn-half (user OR assistant) of a conversation
// history. AppendTurn writes a (user, assistant) pair atomically.
type SessionMessage struct {
	// Role is "user" or "assistant" — the schema CHECK-enforces these
	// two values.
	Role    string
	Content string
	Ts      time.Time
}

// AuditMeta is the Phase 10 sibling-table audit row that carries the
// rewriter / classifier / history-projection signals NOT covered by the
// canonical audit row. The relationship to AuditRow is 1:1 by query_id
// (the audit row's auto-increment ID). Both rows are INSERT-only
// (T-3-AUDIT invariant); a future "backfill metrics" path that UPDATEs
// the meta row would destroy the trust property this machinery exists
// to protect.
//
// The fields captured here are the Phase 10 trust-floor escape's
// audit lineage: when the spike runner re-runs an adversarial session
// against the same seed, it must reproduce the same rewriter output,
// the same classifier decision, and the same history projection — the
// AuditMeta row is the single source of truth.
//
// Field semantics:
//
//   - QueryID            1:1 join key back to AuditRow.ID. NOT auto-
//                        generated here; the caller wires it after
//                        PutAudit returns the audit row's ID.
//   - RewrittenQuery     nullable; empty when no rewrite happened (first
//                        turn / no-anaphora / rewriter nil-gated).
//   - ClassifierDecision one of "allow" | "refuse" | "n/a". "n/a" when
//                        the classifier was nil-gated; the audit row
//                        itself carries the code-level short-circuit
//                        refusal reason (no_authorized_source /
//                        low_retrieval_score).
//   - ClassifierReason   nullable; one of verdict.RefusalReason* when
//                        ClassifierDecision="refuse".
//   - ClassifierConfidence nullable; float64 in [0.0, 1.0].
//   - HistoryProjectedNTurns post-sanitization message count that
//                        flowed into the verdict messages prefix (0
//                        when no history).
//   - HistoryBudgetTruncated 1 when TruncateToCharBudget dropped at
//                        least one message, else 0.
//   - HistoryDroppedCount number of messages dropped by the budget
//                        window (0 when none).
type AuditMeta struct {
	QueryID                 int64
	RewrittenQuery          string
	ClassifierDecision      string
	ClassifierReason        string
	ClassifierConfidence    *float64
	HistoryProjectedNTurns  int
	HistoryBudgetTruncated  int
	HistoryDroppedCount     int
}

// AuditFilter is the parameter bag for QueryAudit. Every field is
// optional; a zero-value AuditFilter returns the most-recent rows up to
// Limit (default 100). Every field is applied as a parameterized `?`
// placeholder — the WHERE clause is built dynamically but NEVER via
// fmt.Sprintf with user input (T-3-SQLI).
type AuditFilter struct {
	UserID        string
	SkillID       string
	RefusalReason string
	// SessionID filters by the session partition (useful for "show the
	// full audit trail of one conversation").
	SessionID string
	// Since / Until bound the Timestamp range. A zero time.Time (the
	// default value) means "no bound on this side".
	Since time.Time
	Until time.Time
	// Limit caps the number of returned rows. Zero defaults to 100.
	Limit int
	// Offset skips this many rows from the start of the result set.
	// Phase 6 (D6-05) adds LIMIT/OFFSET pagination to /api/audit; the
	// store layer is unchanged in posture (still 100% `?`-parameterized)
	// — the new value is appended to args, not fmt.Sprintf'd into SQL.
	Offset int
}

// Store is the abstraction over the audit + session-memory backend.
//
// Implementations MUST be safe for concurrent use across goroutines
// (the SC-9 concurrency test exercises two same-user sessions in
// parallel under -race). All methods are ctx-aware for AGNT-04 cancel
// propagation — a cancelled ctx must surface as ctx.Err() promptly.
//
// Implementations:
//
//   - internal/store/sqlite — the zero-ops default (pure-Go
//     modernc.org/sqlite, no cgo). Production-swappable for Postgres
//     without changing any caller.
type Store interface {
	// PutAudit appends an immutable audit row. GEVL-03: the row MUST
	// exist before the response ends — synchronous write, SQLite WAL
	// keeps this sub-ms. Returns the assigned auto-increment ID (>0).
	PutAudit(ctx context.Context, row AuditRow) (int64, error)

	// QueryAudit returns audit rows matching the filter, newest-first.
	// The filter is applied via parameterized placeholders (T-3-SQLI).
	QueryAudit(ctx context.Context, filter AuditFilter) ([]AuditRow, error)

	// GetSession returns the message history for sessionID in
	// chronological order. Returns an empty (non-nil) slice for an
	// unknown session_id — callers treat empty as "start fresh". The
	// WHERE clause on session_id is the AGNT-05 isolation enforcement.
	GetSession(ctx context.Context, sessionID string) ([]SessionMessage, error)

	// AppendTurn atomically appends a (user, assistant) turn pair for
	// sessionID. Idempotent on (sessionID, user.Ts, assistant.Ts) via
	// a UNIQUE constraint + ON CONFLICT DO NOTHING.
	AppendTurn(ctx context.Context, sessionID string, user, assistant SessionMessage) error

	// UpdateAuditMetrics backfills the 4 metric columns on an existing
	// audit row. This is the ONLY sanctioned UPDATE path — Plan 03-06's
	// eval sidecar owns this exception; every other path is INSERT-only
	// per T-3-AUDIT. The metric pointers are NULLable: pass nil for any
	// metric you do not want to backfill (the column keeps its prior
	// value, typically NULL from the original INSERT).
	//
	// Returns sql.ErrNoRows if the row does not exist (the caller treats
	// this as "eval CLI ran before the chat path wrote the row" — a
	// sequencing issue, surfaced loudly).
	UpdateAuditMetrics(ctx context.Context, id int64, faithfulness, contextPrecision, answerRelevance, contextRecall *float64) error

	// PutAuditMeta appends the Phase 10 sibling-table audit row that
	// carries the rewriter / classifier / history-projection signals
	// alongside the canonical audit row. INSERT-only (T-3-AUDIT
	// invariant); the audit_meta table is the singular write surface,
	// no UPDATE / DELETE path exists. The meta row's lifecycle is tied
	// to the audit row via FOREIGN KEY (query_id) REFERENCES
	// audit(id) ON DELETE CASCADE — deleting the audit row drops the
	// meta row along with it. Returns an error when the referenced
	// query_id does not exist (the caller treats this as a sequencing
	// bug — PutAudit must have returned the ID before PutAuditMeta is
	// called).
	PutAuditMeta(ctx context.Context, meta AuditMeta) error

	// Health reports whether the backend is reachable. Used by
	// cmd/server startup to fail fast on a misconfigured DSN.
	Health(ctx context.Context) error

	// Close releases the underlying connection pool. After Close every
	// method returns a meaningful error (sql.ErrConnDone-equivalent).
	Close() error
}
