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
	// BudgetLayer is the Phase 12 / Plan 12-05 PLAN-04 budget event
	// discriminator. Populated ONLY for action_budget_triggered rows;
	// one of "max_iter" | "token" | "loop" (whitelist enforced by the
	// sqlite package's BudgetLayerNames validator). Empty string for
	// every other row — verdict rows, HITL-04 lifecycle rows, and the
	// plan_todo cross-reference rows from 12-02 leave this column NULL.
	// Mirrors HITL-04 fail-closed posture: budget events are INSERT-only,
	// queryable via AuditFilter.Action="action_budget_triggered" +
	// a per-layer scan (the column is a single TEXT column, not a JOINed
	// table, because the audit row count grows by ~3 rows per trigger and
	// a JOIN would be heavier than a WHERE filter on a nullable TEXT).
	BudgetLayer string
	// The four Ragas metrics — NULL until the eval sidecar backfills.
	Faithfulness     *float64
	ContextPrecision *float64
	AnswerRelevance  *float64
	ContextRecall    *float64
	// RunPath is the Phase 13 / Plan 13-03 P13-L6 audit substrate.
	// Slash-separated agent path (parent_run/sub_query_id) populated
	// ONLY for action_fanout_dispatched | action_fanout_completed |
	// action_fanout_failed rows. Empty for every other row (verdict,
	// HITL-04 lifecycle, plan_todo cross-reference, budget). The
	// QueryFanoutByRunPath helper (internal/store/sqlite/fanout_events.go)
	// uses RunPath LIKE 'parent_run/%' for the trace query.
	RunPath string
	// TransferFrom is the sub-agent invocation source. Populated for
	// fanout rows to identify the parent run ID (the "transfer source"
	// in agent-path semantics). Empty for non-fanout rows.
	TransferFrom string
	// TransferTo is the sub-agent invocation target. Populated for
	// fanout rows to identify the sub_query_id (the "transfer
	// destination" in agent-path semantics). Empty for non-fanout rows.
	TransferTo string
	// PlatformFloorVersion is the PLAT-10 platform-floor semver that
	// wrote this row. Empty string means "pre-floor era" (the DDL
	// DEFAULT '' carries this semantic). Every INSERT path — chat
	// verdict, HITL lifecycle, budget, fanout — populates it so a SQL
	// `SELECT DISTINCT platform_floor_version FROM audit` reveals the
	// drift window.
	PlatformFloorVersion string
}

// FanoutEvent is one row of the Phase 13 / Plan 13-03 FAN-04 SC-4
// audit trace. Captures the lifecycle of a single sub-agent's Run
// (dispatched → completed | failed). Mirrors the sqlite package's
// FanoutEvent struct field-for-field; promoted to the shared store
// package so the Store interface can reference it.
//
// Field semantics:
//   - ID:      audit row auto-increment ID (for stable ordering when
//              multiple events share the same ts).
//   - Ts:      wall-clock time the audit row was written (UTC).
//   - Action:  one of the 3 action_fanout_* enum values.
//   - RunPath: slash-separated agent path (parent_run/sub_query_id).
//   - TransferFrom: parent run ID (the "transfer source").
//   - TransferTo:   sub_query_id (the "transfer destination").
//   - Reason: populated for action_fanout_failed rows; empty for
//              dispatched + completed.
type FanoutEvent struct {
	ID           int64
	Ts           time.Time
	Action       string
	RunPath      string
	TransferFrom string
	TransferTo   string
	Reason       string
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
	// PlatformFloorVersion is the PLAT-10 platform-floor semver that
	// wrote this row. HITL rows MUST carry a non-empty value
	// (ErrMissingFloorVersion fail-closed).
	PlatformFloorVersion string
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
	// PlatformFloorVersion is the PLAT-10 platform-floor semver that
	// wrote this row. HITL rows MUST carry a non-empty value
	// (ErrMissingFloorVersion fail-closed).
	PlatformFloorVersion string
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
	// PlatformFloorVersion is the PLAT-10 platform-floor semver that
	// wrote this row. HITL rows MUST carry a non-empty value
	// (ErrMissingFloorVersion fail-closed).
	PlatformFloorVersion string
}

// DatasetRow is one registered analytic dataset (Phase 14 / D14-D2). The
// registry lives in the SAME sqlite database as audit/checkpoint; the
// dataset payload itself is a .duckdb file under data/datasets/ whose
// on-disk name derives from Name (see internal/duckdb.CommitStagedDataset —
// FilePath is written by that commit path, never hand-edited).
type DatasetRow struct {
	// Name is the dataset identifier — the same allowlist grammar the
	// duckdb ATTACH layer enforces (^[A-Za-z0-9_-]{1,64}$).
	Name string
	// Tier is the trust tier the dataset is exposed under (Skill `reads`
	// vocabulary: "权威" / "历史" / "个人").
	Tier string
	// SchemaJSON is the table/column descriptor JSON for admin surfaces.
	SchemaJSON string
	// FilePath is the on-disk .duckdb location (under data/datasets/).
	FilePath string
	// ContentSHA256 is the content hash for integrity + TOCTOU checks
	// (Phase 19 pattern).
	ContentSHA256 string
	// BatchID is the import batch that produced this row.
	BatchID string
	// CreatedAt is the registration time (UTC).
	CreatedAt time.Time
	// Dataset is the logical dataset name (Phase 15 / D15-C2). For the
	// composite registry name "loans__2026-07" this is "loans". Empty for
	// pre-Phase-15 rows (DEFAULT '') and old callers that do not set it.
	Dataset string
	// Period is the data period, e.g. "2026-07" (Phase 15 / D15-C2). Empty
	// for pre-Phase-15 rows and old callers that do not set it.
	Period string
	// KMinOverride is the per-dataset k-anonymity floor override (Phase 15 /
	// D15-D3). Zero means "use the global default KMinDefault"; non-zero
	// overrides the default for this dataset.
	KMinOverride int
}

// ImportBatchRow is one INSERT-only audit record for a data-import batch
// (Phase 15 / Plan 15-02 / DATA-17). It carries the proof that raw data was
// discarded (DiscardTS), the rule-set hash applied (RulesSHA256, D15-A4),
// and the supersede chain (SupersedesBatchID, D15-C1).
type ImportBatchRow struct {
	// BatchID is the crypto/rand hex identifier of the batch.
	BatchID string
	// Dataset is the logical dataset name.
	Dataset string
	// Period is the data period, e.g. "2026-07".
	Period string
	// Tier is the trust tier the dataset is exposed under.
	Tier string
	// ContentSHA256 is the SHA-256 of the raw upload bytes.
	ContentSHA256 string
	// RulesSHA256 is the SHA-256 of the rule file bytes used for
	// desensitization (D15-A4).
	RulesSHA256 string
	// RawSizeBytes is the byte length of the raw upload.
	RawSizeBytes int64
	// DesensitizedRowCount is the number of rows written to the desensitized
	// dataset.
	DesensitizedRowCount int64
	// DiscardTS is the RFC3339 timestamp when the raw stream was discarded
	// (DATA-17). Stored as a string to preserve the exact moment.
	DiscardTS string
	// SupersedesBatchID is the previous batch ID for this (dataset, period)
	// key, or empty if this is the first batch (D15-C1).
	SupersedesBatchID string
	// CreatedAt is the batch registration time (UTC).
	CreatedAt time.Time
}

// DatasetDeleteRow is one INSERT-only audit record for a dataset deletion
// (Phase 15 / Plan 15-02 / D15-C3). The state-side deletion happens in the
// delete endpoint; this row records the action.
type DatasetDeleteRow struct {
	// Name is the exact registry name that was deleted.
	Name string
	// Dataset is the logical dataset name (empty for pre-Phase-15 deletions).
	Dataset string
	// DeletedAt is the deletion time (UTC).
	DeletedAt time.Time
}

// KPICallRow is one INSERT-only audit record of a single KPI invocation
// (Phase 16 / Plan 16-03 / KPI-02 / D16-D4). It freezes the FULL answer the
// agent saw — {func, args, as_of, result_snapshot, k_min_applied} plus the
// ts/session/skill context — into the dedicated kpi_calls table, so that
// even after the upstream DuckDB dataset is superseded or cleaned the exact
// call-time result stays replayable from this row alone. Written exclusively
// through PutKPICall; no mutation or removal path exists (T-3-AUDIT +
// T-16-03-01). The large ResultSnapshot JSON deliberately does NOT enter the
// chat audit main table (T-16-03-03 physical isolation).
type KPICallRow struct {
	// ID is the assigned auto-increment ID (>0); insertion order IS replay
	// order. Populated on reads; ignored by PutKPICall.
	ID int64
	// TS is the call moment (UTC) — the same instant the executor stamps
	// into its as_of payload.
	TS time.Time
	// SessionID ties the call to the conversation partition (the audit
	// table's same-named column precedent).
	SessionID string
	// SkillID identifies the Skill whose agent invoked the KPI.
	SkillID string
	// Func is the invoked KPI name — KPI-02's literal func field and the
	// kpi registry key (e.g. "overdue_by_region").
	Func string
	// ArgsJSON snapshots the ACTUAL arguments used, including which optional
	// parameters were passed vs defaulted — replay never invents defaults.
	ArgsJSON string
	// ASOfPeriod records as_of.period — the datasets-registry period
	// watermark the answer was computed from.
	ASOfPeriod string
	// KMinApplied is the effective k-anonymity floor for THIS call — the
	// final value of the file > dataset-override > global-default chain.
	KMinApplied int
	// ResultSnapshot is the complete result-set JSON — every row INCLUDING
	// suppressed-marker rows, never truncated (D16-D3 full-freeze).
	ResultSnapshot string
	// Signature is the registry definition hash (D16-A4) proving WHICH
	// definition version produced the snapshot on replay.
	Signature string
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
	// Action filters by the Phase 11 HITL lifecycle enum
	// ("action_pending_approval" | "action_approved" |
	// "action_rejected" | "action_rule_revoked"). Empty = no action
	// filter. Added by Plan 11-04 for the approval dual endpoint and
	// the audit-timeline endpoint; applied as a parameterized `?`
	// placeholder like every other field (T-3-SQLI).
	Action string
	// ToolCallID filters by the D11-C4 approval identity (==
	// ApprovalID). Used by the approval endpoint's pending-row lookup
	// and idempotency probe. Empty = no filter.
	ToolCallID string
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

	// QueryFanoutByRunPath returns the Phase 13 / Plan 13-03 FAN-04
	// SC-4 audit trace reconstruction: the ordered list of fanout
	// lifecycle events for the supplied runPathPrefix. Filters on
	// action LIKE 'action_fanout_%' AND run_path LIKE <prefix>.
	// Returns empty slice for no matches; sql.ErrConnDone on closed
	// store; error on empty runPathPrefix.
	//
	// Read-only — does NOT write any audit rows. The audit table is the
	// source of truth (P13-L6 — additive ALTER COLUMN migration).
	QueryFanoutByRunPath(ctx context.Context, runPathPrefix string) ([]FanoutEvent, error)

	// ListFanoutEvents returns fanout lifecycle events for sessionID,
	// newest-first, up to limit rows. limit <= 0 defaults to 100.
	// Used by the AgentInspector timeline (Phase 5 SC-4) to render the
	// fanout trace alongside the existing decision timeline.
	ListFanoutEvents(ctx context.Context, sessionID string, limit int) ([]FanoutEvent, error)

	// PutDataset upserts one dataset registry row (INSERT OR REPLACE by
	// name — idempotent, Phase 14 / D14-D2). Called by the staging commit
	// path (internal/duckdb.CommitStagedDataset) after the atomic rename.
	PutDataset(ctx context.Context, row DatasetRow) error

	// ListDatasets returns every registered dataset row, ordered by name.
	// Empty (non-nil) slice when the registry is empty.
	ListDatasets(ctx context.Context) ([]DatasetRow, error)

	// DeleteDataset removes the dataset registry row with the exact name.
	// It is the state-side half of the deletion action; the caller is
	// responsible for the file removal and the dataset_deletes audit row
	// (D15-C3). Missing rows are idempotent (no error).
	DeleteDataset(ctx context.Context, name string) error

	// PutImportBatch appends one INSERT-only import batch audit row
	// (DATA-17 + D15-A4 + D15-C1). Duplicate batch_id values surface as a
	// PRIMARY KEY constraint error, never silent replacement.
	PutImportBatch(ctx context.Context, row ImportBatchRow) error

	// LatestImportBatch returns the newest batch for the (dataset, period)
	// key, or (nil, nil) when no batch exists (DATA-16 idempotency probe).
	LatestImportBatch(ctx context.Context, dataset, period string) (*ImportBatchRow, error)

	// ListImportBatches returns every batch audit row for dataset, newest
	// first. Empty (non-nil) slice when there are none (ADM-02 batch view
	// substrate).
	ListImportBatches(ctx context.Context, dataset string) ([]ImportBatchRow, error)

	// PutDatasetDelete appends one INSERT-only dataset deletion audit row
	// (D15-C3). The state-side deletion is performed separately by the
	// caller.
	PutDatasetDelete(ctx context.Context, row DatasetDeleteRow) error

	// PutKPICall appends one INSERT-only KPI-call audit row (KPI-02 /
	// D16-D4) — the ONLY write path into kpi_calls. Consumed by the 16-06
	// executor's AuditSink (the audit leg of its three output legs); the
	// ADM-03 admin view (Phase 20) reads the table via ListKPICalls /
	// LatestKPICallByFunc. Append-only by construction: every call gets a
	// fresh AUTOINCREMENT id and frozen rows are never rewritten.
	PutKPICall(ctx context.Context, row KPICallRow) error

	// Health reports whether the backend is reachable. Used by
	// cmd/server startup to fail fast on a misconfigured DSN.
	Health(ctx context.Context) error

	// Close releases the underlying connection pool. After Close every
	// method returns a meaningful error (sql.ErrConnDone-equivalent).
	Close() error
}
