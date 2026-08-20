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
	// Action is the Phase 11 HITL-04 lifecycle enum. One of
	// "action_pending_approval" | "action_approved" | "action_rejected" |
	// "action_rule_created" | "action_rule_revoked", or the empty string
	// for every verdict row written by the chat path (and for rows that
	// pre-date the column). The lifecycle rows are written exclusively by
	// InsertPendingApproval / MarkApproved / MarkRejected /
	// MarkRuleRevoked — each writes ONE new row, never an UPDATE.
	Action string
	// SkillVersion is the D11-C2 snapshot of the active Skill's version
	// at the moment the row was written. Paired with PromptHash it forms
	// the resume expire-check: a checkpoint whose snapshot no longer
	// matches the live Skill is expired rather than silently resumed.
	SkillVersion string
	// FrozenArgs is the byte-identical snapshot of the LLM-emitted
	// tool-call JSON arguments (HITL-04 byte-identity invariant: "批的 A
	// 不会执行成 B"). NULL for verdict rows.
	FrozenArgs []byte
	// ToolCallID is the eino tool-call identity of the approval. It is
	// the SAME value as ApprovalID (D11-C4 single identity) and is
	// constrained by the idx_audit_pending_tcid partial unique index for
	// action_pending_approval rows.
	ToolCallID string
	// ApprovalID == ToolCallID (D11-C4). Kept as a distinct column so the
	// approve/deny HTTP route can key on the operator-facing name without
	// leaking the eino-internal vocabulary into the API.
	ApprovalID string
	// ScopeSessionID binds an approve-and-remember rule to a session
	// (D11-D2 rule scope). Empty for verdict rows.
	ScopeSessionID string
	// The four Ragas metrics — NULL until the eval sidecar backfills.
	Faithfulness     *float64
	ContextPrecision *float64
	AnswerRelevance  *float64
	ContextRecall    *float64
}

// PendingApprovalRow is the audit row written synchronously BEFORE the
// HITL Tool reaches tool.StatefulInterrupt (D11-C3 fail-closed). It is
// the FIRST row of the HITL-04 3-row lifecycle.
//
// FrozenArgs is the byte-identical snapshot of the LLM-emitted tool-call
// JSON args — the anchor of the "批的 A 不会执行成 B" invariant.
// ToolCallID == ApprovalID (D11-C4); the partial unique index on
// audit(tool_call_id) WHERE action='action_pending_approval' makes a
// duplicate pending INSERT fail, which is exactly the cross-device
// double-approve moat.
type PendingApprovalRow struct {
	Timestamp     time.Time
	SessionID     string
	UserID        string
	SkillID       string
	Query         string
	PromptHash    string
	SkillVersion  string // D11-C2 snapshot
	ModelVersion  string
	VerdictJSON   string
	RefusalReason string // empty for pending
	ChunkIDs      string
	FrozenArgs    []byte // HITL-04 byte snapshot
	ToolCallID    string // D11-C4 == ApprovalID (PK UNIQUE for pending)
	ApprovalID    string // D11-C4 == ToolCallID
	// ScopeSessionID binds the row to the session an approve-and-remember
	// rule would be scoped to (D11-D2).
	ScopeSessionID string
}

// ApprovalResolutionRow is the SECOND row of the HITL-04 lifecycle
// (action_approved | action_rejected). RefusalReason carries the
// operator's explanation — REQUIRED on the reject path (HITL-02).
//
// FrozenArgs may differ from the pending row's bytes when the operator
// used edit-before-approve (HITL-07); when no edit happened the two are
// byte-identical, which is what TestApprovalArgsByteEquality locks.
type ApprovalResolutionRow struct {
	Timestamp      time.Time
	SessionID      string
	UserID         string
	SkillID        string
	Query          string
	PromptHash     string
	SkillVersion   string
	ModelVersion   string
	VerdictJSON    string
	RefusalReason  string // operator reason (required when reject)
	ChunkIDs       string
	FrozenArgs     []byte
	ToolCallID     string // == the pending row's tool_call_id (D11-C4)
	ApprovalID     string // == the pending row's approval_id
	ScopeSessionID string
}

// RuleRevokeRow is the audit row written when an approve-and-remember
// rule is DELETEd (D11-D2). The rule row itself is removed from the
// approval_rules table; the audit chain only ever grows (T-3-AUDIT).
// FrozenArgs captures the rule key (target + args hash + skill_id +
// scope_session_id) as bytes so the revoke is reconstructible.
type RuleRevokeRow struct {
	Timestamp      time.Time
	SessionID      string
	UserID         string
	SkillID        string
	Query          string
	PromptHash     string
	SkillVersion   string
	ModelVersion   string
	VerdictJSON    string
	RefusalReason  string // operator reason for the revoke
	ChunkIDs       string
	FrozenArgs     []byte
	ToolCallID     string
	ApprovalID     string
	ScopeSessionID string
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

// SessionSummary is one roster row for ListSessions (session-management
// phase). The session roster is DERIVED — sessions exist implicitly as
// distinct session_id values in session_turn; session_meta contributes
// only the optional rename override.
type SessionSummary struct {
	// ID is the session_id partition key.
	ID string
	// DerivedTitle is the first user turn's content truncated to 40
	// runes — the default display title. Empty when the session has no
	// user turn (cannot happen via AppendTurn today; defensive).
	DerivedTitle string
	// Title is the session_meta rename override; empty string means
	// "no override — use DerivedTitle".
	Title string
	// LastActive is the newest session_turn ts for the session (UTC).
	LastActive time.Time
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
//     generated here; the caller wires it after
//     PutAudit returns the audit row's ID.
//   - RewrittenQuery     nullable; empty when no rewrite happened (first
//     turn / no-anaphora / rewriter nil-gated).
//   - ClassifierDecision one of "allow" | "refuse" | "n/a". "n/a" when
//     the classifier was nil-gated; the audit row
//     itself carries the code-level short-circuit
//     refusal reason (no_authorized_source /
//     low_retrieval_score).
//   - ClassifierReason   nullable; one of verdict.RefusalReason* when
//     ClassifierDecision="refuse".
//   - ClassifierConfidence nullable; float64 in [0.0, 1.0].
//   - HistoryProjectedNTurns post-sanitization message count that
//     flowed into the verdict messages prefix (0
//     when no history).
//   - HistoryBudgetTruncated 1 when TruncateToCharBudget dropped at
//     least one message, else 0.
//   - HistoryDroppedCount number of messages dropped by the budget
//     window (0 when none).
type AuditMeta struct {
	QueryID                int64
	RewrittenQuery         string
	ClassifierDecision     string
	ClassifierReason       string
	ClassifierConfidence   *float64
	HistoryProjectedNTurns int
	HistoryBudgetTruncated int
	HistoryDroppedCount    int
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

	// ListSessions returns the session roster derived from session_turn
	// (one row per distinct session_id), newest-activity-first, LEFT
	// JOINed onto session_meta for the rename override. DerivedTitle is
	// the first user turn truncated; Title (the override) is empty when
	// no rename happened — callers prefer Title, falling back to
	// DerivedTitle. Audit rows are NOT consulted (the immutable trail
	// stays decoupled from the user-facing roster).
	ListSessions(ctx context.Context) ([]SessionSummary, error)

	// RenameSession upserts the session_meta title override for
	// sessionID (INSERT ... ON CONFLICT DO UPDATE). It does not require
	// existing turns — renaming before any chat is legal and simply
	// pre-seeds the override.
	RenameSession(ctx context.Context, sessionID, title string) error

	// DeleteSessionTurns removes sessionID's turns AND its session_meta
	// row in one transaction (both are user-owned data). Audit rows are
	// deliberately NOT deleted — T-3-AUDIT's immutable trail survives
	// user-visible deletion. Idempotent: an unknown sessionID is a no-op.
	DeleteSessionTurns(ctx context.Context, sessionID string) error

	// InsertPendingApproval writes the FIRST row of the HITL-04 audit
	// lifecycle (action_pending_approval). Called synchronously by the
	// HITL audit writer BEFORE the Tool interrupts (D11-C3 fail-closed):
	// on error the Tool must return the error, so no interrupt is
	// signalled, no checkpoint is written, and no write executes.
	//
	// A UNIQUE constraint error means a pending row for the same
	// tool_call_id already exists (D11-C4 cross-device race). The error
	// is returned, never swallowed — the caller decides whether that is
	// benign (a second POST for an already-pending approval).
	InsertPendingApproval(ctx context.Context, row PendingApprovalRow) (int64, error)

	// MarkApproved / MarkRejected write the SECOND row of the lifecycle.
	// Both are INSERTs — the pending row is never mutated (T-3-AUDIT).
	MarkApproved(ctx context.Context, row ApprovalResolutionRow) (int64, error)
	MarkRejected(ctx context.Context, row ApprovalResolutionRow) (int64, error)

	// MarkRuleRevoked records the revoke of an approve-and-remember rule
	// (D11-D2). The rule row is deleted from its own table; the audit
	// chain records the revoke as a new row.
	MarkRuleRevoked(ctx context.Context, row RuleRevokeRow) (int64, error)

	// SetCheckpointWithSnapshot persists a checkpoint together with the
	// D11-C2 (skill_version, prompt_hash) snapshot. The Runner owns the
	// plain Set path; this is the snapshot-carrying facade the HITL
	// resume machinery writes through.
	SetCheckpointWithSnapshot(ctx context.Context, id string, payload []byte, skillVersion, promptHash string) error

	// GetCheckpointByVersionFilter returns the checkpoint payload ONLY
	// when the stored snapshot matches (skillVersion, promptHash). A
	// mismatch returns (nil, false, nil) — the D11-C2 expire state; the
	// caller then Deletes the row and reports state="expired" rather than
	// silently resuming against a drifted Skill.
	GetCheckpointByVersionFilter(ctx context.Context, id, skillVersion, promptHash string) ([]byte, bool, error)

	// Delete removes the checkpoint stored under id (the D11-C2 expire
	// path; also the adk.CheckPointDeleter contract). Idempotent: an
	// unknown id is a no-op. Audit rows are NEVER deleted (T-3-AUDIT).
	Delete(ctx context.Context, id string) error

	// Health reports whether the backend is reachable. Used by
	// cmd/server startup to fail fast on a misconfigured DSN.
	Health(ctx context.Context) error

	// Close releases the underlying connection pool. After Close every
	// method returns a meaningful error (sql.ErrConnDone-equivalent).
	Close() error
}
