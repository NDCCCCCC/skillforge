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
// write is the id-keyed upsert (INSERT ... ON CONFLICT(id), i.e.
// replace-on-replay) so a re-interrupted run replaces its own
// checkpoint. session_id / skill_id
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
    -- Phase 58-05 (ROI-01): the retrieval channel distribution (JSON).
    -- NULL == no stats for this turn (pre-58 rows and turns with
    -- nothing to report), which keeps every existing row readable.
    retrieval_stats           TEXT,
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
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    owner_token TEXT NOT NULL DEFAULT ''
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

-- approval_rules (Phase 56 / Plan 56-01 / D-56-01): the policy-entry
-- rule table, rebuilt from the Plan 11-05 four-tuple-PK form. The PK
-- is now the surrogate id (AUTOINCREMENT) — PRAGMA-gated ALTER
-- cannot rewrite a PRIMARY KEY, so pre-56 deployments upgrade via the
-- rebuild step in migrate() (see legacyApprovalRulesRebuild), NOT via
-- an ALTER. The four-tuple survives as the UNIQUE natural key
-- idx_approval_rules_natural — but with match_type and args_pattern
-- folded in.
--
-- Match tiers (D-56-01), matched in priority order session > skill >
-- global by the 56-02 engine:
--
--   - exact:    arguments match the rule's args_hash byte-for-byte
--               (the Plan 11-05 semantics — args_hash is now an INDEX
--               COLUMN for this tier, no longer part of the key).
--   - prefix:   args_pattern locks a JSON-object field subset; the
--               remaining fields stay free (naming follows the Claude
--               Code Bash(cmd:*) intuition, landed as a field-level
--               template).
--   - template: matches at the TARGET level only (every call to the
--               target auto-passes).
--
-- Deny semantics are UNCHANGED by design (D-56-01): deny /
-- request_change stay one-shot operator decisions — there is NO deny
-- rule, rules only ever AUTO-PASS a write. The rule table therefore
-- has no verdict column.
--
-- Three-layer scope (replaces the D11-D3 binary scope): scope is now a
-- STORED column ('session' | 'skill' | 'global'). scope_session_id is
-- retained as the VALUE column for scope='session' rows only. A
-- 'global' rule applies across every Skill (skill_id may be '').
--
-- Origin ('approve_remember' | 'manual' | 'seed'): where the rule came
-- from — an operator's approve-and-remember click, a panel-authored
-- entry, or a seeded policy. Legacy rows migrated by the rebuild step
-- carry 'approve_remember' (the only possible legacy source).
--
-- last_used_at is a COMPAT COLUMN kept in lockstep with last_hit_at
-- (HitRule writes both in one statement) so the pre-56 FE shim
-- (last_used_at wire field) keeps rendering until the 56-03 panel
-- switches to last_hit_at; it retires in the next milestone.
--
-- NOTE: the approval_rules INDEXES deliberately live at the migrate()
-- tail (after the legacy rebuild), NOT here — this is the UAT 14-09
-- "no such column" landmine: on a pre-56 database this CREATE TABLE is
-- a no-op (the legacy four-tuple table already exists), so an index
-- inlined here would reference match_type before the rebuild adds it.
CREATE TABLE IF NOT EXISTS approval_rules (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    target           TEXT NOT NULL,
    match_type       TEXT NOT NULL DEFAULT 'exact'
                     CHECK (match_type IN ('exact','prefix','template')),
    args_hash        TEXT NOT NULL DEFAULT '',
    args_pattern     TEXT NOT NULL DEFAULT '',
    skill_id         TEXT NOT NULL DEFAULT '',
    scope            TEXT NOT NULL DEFAULT 'session'
                     CHECK (scope IN ('session','skill','global')),
    scope_session_id TEXT NOT NULL DEFAULT '',
    enabled          INTEGER NOT NULL DEFAULT 1,
    expires_at       INTEGER,
    origin           TEXT NOT NULL DEFAULT 'approve_remember'
                     CHECK (origin IN ('approve_remember','manual','seed')),
    hit_count        INTEGER NOT NULL DEFAULT 0,
    last_hit_at      INTEGER,
    created_at       INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    last_used_at     INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

-- approval_rule_hits (Phase 56 / Plan 56-01 / D-56-01): the INSERT-only
-- hit ledger — one row per rule auto-pass, written by the 56-02 engine
-- (InsertRuleHit) as the fine-grained companion to the rule row's
-- hit_count / last_hit_at counters. append-only like audit: there is
-- no UPDATE / DELETE path for hit rows anywhere in this package, so
-- the per-rule hit history stays replayable (the 56-03 panel's hit
-- history view reads it via QueryRuleHits).
CREATE TABLE IF NOT EXISTS approval_rule_hits (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id           INTEGER NOT NULL,
    ts                INTEGER NOT NULL,
    session_id        TEXT NOT NULL,
    target            TEXT NOT NULL,
    tool_call_id      TEXT NOT NULL DEFAULT '',
    match_type        TEXT NOT NULL,
    matched_args_hash TEXT NOT NULL DEFAULT ''
);

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

-- report_runs (Phase 19 / Plan 19-01 / PORT-01/PORT-02/PORT-03):
-- INSERT-only audit table for scheduler-triggered report runs. Each
-- scheduled firing produces one row: run_id (UUID), skill_id, period,
-- cron_expr (the schedule that fired it), fired_at, kpi_snapshots (JSON
-- freeze of KPI values at run time), output_url, status (enum:
-- success|failed|skipped_duplicate|eval_gate_blocked), drift_detected
-- (bool flag for GEVL-11 drift detection), and created_at.
CREATE TABLE IF NOT EXISTS report_runs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          TEXT    NOT NULL UNIQUE,
    skill_id        TEXT    NOT NULL,
    period          TEXT    NOT NULL,
    cron_expr       TEXT    NOT NULL DEFAULT '',
    fired_at        DATETIME NOT NULL,
    kpi_snapshots   TEXT    NOT NULL DEFAULT '{}',
    output_url      TEXT    NOT NULL DEFAULT '',
    status          TEXT    NOT NULL DEFAULT 'success',
    drift_detected  INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL
);

-- knowledge_bases (Phase 26 / Plan 26-02 / KBM-01/02): the mutable
-- registry-config table behind the knowledge-base registry. Deliberate
-- asymmetry with kpi_calls: kpi_calls is an insert-only audit table
-- because a KPI call is evidence; a knowledge base is OPERATIONAL
-- CONFIGURATION, so this table carries the current state only and is
-- updated in place (datasets is the established precedent for the
-- "config is not evidence" split, store.go PutDataset/DeleteDataset).
-- The irreproducible history still exists — every structure or switch
-- change is recorded by the CALLER as an insert-only audit row in the
-- EXISTING audit table (action_kb_created / action_kb_deactivated,
-- server layer, Plan 26-05). No new audit table is created here, and
-- this file ships no audit-row write path of its own.
--
-- Schema rationale:
--   - slug PRIMARY KEY: one registry row per knowledge base; the slug is
--     also the knowledge/<slug>/ directory name and the retriever filter
--     value (whitelist-validated upstream by internal/kb).
--   - role CHECK: the closed four-value vocabulary is platform semantics
--     (internal/kb Roles() is its single home), so the storage layer
--     backstops it — a typo'd role is refused by sqlite, not merely by
--     Go-side validation.
--   - retrieval_enabled: the deactivation switch (0 = deactivated).
--     Deactivation NEVER deletes the row or its data (KBM-02: deactivate,
--     never destroy) — there is no delete path for this table anywhere in
--     the store.
--   - Additive migration only (D14-E1): a brand-new table created with
--     IF NOT EXISTS; its index is created at the migrate() tail, never
--     inlined here (see idxKnowledgeBasesRole).
CREATE TABLE IF NOT EXISTS knowledge_bases (
    slug              TEXT PRIMARY KEY,
    display_name      TEXT NOT NULL,
    role              TEXT NOT NULL CHECK (role IN ('authority','historical','personal','draft')),
    description       TEXT NOT NULL DEFAULT '',
    retrieval_enabled INTEGER NOT NULL DEFAULT 1,
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);

-- feedback (Phase 29 / Plan 29-02 / FBK-01 / D-29-02): the one-vote-per-
-- message user-feedback table. UNIQUE(session_id, turn_id) IS the
-- idempotency mechanism — a replayed vote hits ON CONFLICT DO NOTHING and
-- the caller reads the FIRST row back (the FE lock state is only a cache;
-- the DB constraint is the sole source of truth for "one vote per turn").
--
-- Schema rationale:
--   - turn_id is session_turn.id (D-29-01) — the server-side message
--     identity, never a client-generated uuid.
--   - verdict CHECK: the closed three-value vocabulary is storage-layer
--     backstopped the same way knowledge_bases.role is (a typo'd verdict
--     is refused by sqlite, not merely by Go-side validation).
--   - comment: the optional D-29-06 outdated note (empty for the other
--     two verdicts; surfacing it is the review queue's business).
--   - skill_id / chunk_ids: server-side copies from the turn's audit row
--     (D-29-05 — never trusted from the FE). chunk_ids is a CSV snapshot
--     in the audit.chunk_ids precedent; the normalized per-chunk join
--     column lives in feedback_chunk_refs below.
--   - INSERT-only discipline: rows are appended by PutFeedback; this
--     table has no mutation or removal path anywhere (source-grep
--     structural lock in feedback_test.go).
CREATE TABLE IF NOT EXISTS feedback (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ts         DATETIME NOT NULL,
    session_id TEXT    NOT NULL,
    turn_id    INTEGER NOT NULL,
    verdict    TEXT    NOT NULL CHECK (verdict IN ('helpful','unhelpful','outdated')),
    comment    TEXT    NOT NULL DEFAULT '',
    skill_id   TEXT    NOT NULL DEFAULT '',
    chunk_ids  TEXT    NOT NULL DEFAULT '',
    UNIQUE(session_id, turn_id)
);

-- feedback_chunk_refs (Phase 29 / Plan 29-02 / FBK-02/03): the
-- normalization of one vote's cited chunks, one row per (feedback,
-- chunk). audit.chunk_ids is a CSV and splitting CSV for aggregation
-- inside sqlite is awkward — writing these rows once at vote time buys
-- two pure-SQL aggregations forever after: the per-chunk pending-review
-- queue (GROUP BY chunk_id) and the per-KB metrics (GROUP BY kb_slug).
--
--   - kb_slug is the retriever chunk's Tier (Phase 26 semantics: Tier IS
--     the knowledge_bases registry slug). Populated by the 29-05 write
--     path; this plan only stores and reads it.
--   - One vote citing two KBs produces two rows — that is the Open Q2
--     accounting (per-KB metrics count a multi-KB vote once PER KB; the
--     per-Skill and total dimensions still count it exactly once).
CREATE TABLE IF NOT EXISTS feedback_chunk_refs (
    feedback_id INTEGER NOT NULL,
    chunk_id    INTEGER NOT NULL,
    kb_slug     TEXT    NOT NULL DEFAULT '',
    UNIQUE(feedback_id, chunk_id)
);

-- feedback_dispositions (Phase 29 / Plan 29-02 / FBK-02 / D-29-07..09):
-- the INSERT-only review-queue disposition log. Deliberately NO UNIQUE
-- constraint — multiple dispositions of one chunk are legal and the
-- NEWEST timestamp wins ("latest-disposition-wins", Open Q1). The
-- pending view is DERIVED (a chunk is pending when it has an outdated
-- vote with no later disposition) — there is no second write path and
-- no status column anywhere.
CREATE TABLE IF NOT EXISTS feedback_dispositions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    chunk_id    INTEGER NOT NULL,
    disposition TEXT    NOT NULL CHECK (disposition IN ('updated','false_positive','ignored')),
    note        TEXT    NOT NULL DEFAULT '',
    actor       TEXT    NOT NULL DEFAULT 'admin'
);

-- tickets (Phase 29 / Plan 29-02 / TKT-02 / D-29-14): the one-ticket-per-
-- message ticket request table. UNIQUE(session_id, turn_id) is the same
-- idempotency mechanism as feedback — a duplicate click returns the FIRST
-- row (existing ticket number) instead of a second row.
--
--   - INSERT-only: PutTicket appends; there is NO write-back path for
--     the outbound dispatch outcome (the dispatch result lands in an
--     action_ticket_dispatch_* audit row instead — TKT-02 keeps the
--     INSERT-only posture intact, source-grep lock in tickets_test.go).
--   - dispatch_status CHECK ('none' | 'queued') records the state AT
--     INSERT TIME only: 'none' = no provider configured (store-only
--     receipt), 'queued' = handed to the background dispatcher. It is
--     never flipped afterwards.
--   - The acceptance number is NOT a column — the 29-06 handler derives
--     "TKT-{row id}" from the returned TicketRow.ID (INSERT-only
--     discipline forbids a write-back number column).
CREATE TABLE IF NOT EXISTS tickets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              DATETIME NOT NULL,
    session_id      TEXT    NOT NULL,
    turn_id         INTEGER NOT NULL,
    description     TEXT    NOT NULL,
    chunk_ids       TEXT    NOT NULL DEFAULT '',
    skill_id        TEXT    NOT NULL DEFAULT '',
    provider        TEXT    NOT NULL DEFAULT 'none',
    dispatch_status TEXT    NOT NULL DEFAULT 'none' CHECK (dispatch_status IN ('none','queued')),
    UNIQUE(session_id, turn_id)
);

-- ecl_failures (Phase 39 / Plan 39-05 / ONT-12 / D-DL-01): the ECL
-- pipeline's dead-letter ledger — one row per note (note_id PRIMARY KEY),
-- stage records the MOST RECENT failed stage (extract | kg-load |
-- milvus-load), attempt_count the failed attempts so far, and status the
-- lifecycle pending | dead | resolved.
--
-- MUTABLE STATUS COLUMN IS SANCTIONED HERE — this is NOT an audit table
-- (CONTEXT D-DL-01 note): retry semantics REQUIRE in-place status
-- transitions (pending → dead on the 3rd attempt, → resolved on a manual
-- re-run success), so the T-3-AUDIT INSERT-only discipline deliberately
-- does not apply. UpsertFailure is the single write path
-- (INSERT ... ON CONFLICT(note_id) DO UPDATE, ecl_failures.go).
-- updated_at is an RFC3339 UTC string SET BY THE CALLER (the ecl manager)
-- — never a SQL default — so retry timing stays testable.
CREATE TABLE IF NOT EXISTS ecl_failures (
    note_id       TEXT PRIMARY KEY,
    stage         TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',
    updated_at    TEXT NOT NULL DEFAULT ''
);

-- ingest_jobs / ingest_steps / ingest_items (Phase 54 / Plan 54-01 /
-- ING-04/ING-05): the async ingest job model. Two-table shape chosen over
-- the RAGFlow single wide row: each step is a FIRST-CLASS row
-- ((job_id, step) PK) carrying its own status/attempt/timestamps/error —
-- step-level retry (54-03) needs row-granular CAS that an 8-step × 8-field
-- wide row cannot express without repeated job-table DDL churn.
--
-- MUTABLE STATUS COLUMNS ARE SANCTIONED HERE — same exemption argument as
-- ecl_failures: retry semantics REQUIRE in-place status transitions
-- (pending → running → done|failed|skipped|stale per step; queued →
-- running → done|failed|partial per job), so the T-3-AUDIT INSERT-only
-- discipline deliberately does not apply. The job/steps tables are
-- OPERATIONAL STATE (the knowledge_bases "config is not evidence"
-- precedent), not audit evidence — the audit trail for ingest actions
-- lives in the EXISTING audit table (action_doc_ingest_queued /
-- action_doc_uploaded rows written by the server layer).
--
-- TIMESTAMP DISCIPLINE: created_at / updated_at / started_at /
-- completed_at are RFC3339Nano UTC strings SET BY THE CALLER (the ingest
-- runner) — never a SQL default — so step-duration derivation and retry
-- timing stay testable (ecl_failures.updated_at precedent).
--
--   - job_id: "ing-" + crypto/rand 12-byte hex (24 chars total); the
--     handler passes it straight back to the FE polling loop.
--   - doc_id: parse step assigns it (the 53-01 persist relative path);
--     empty until parse completes.
--   - source_path: the 53-01 raw-bytes archive (data/uploads/<sha256><ext>)
--     — the original-byte source for 54-03 step retry (D-54-02 staging
--     contract lands there; this plan only records the pointer).
--   - parser_name: parse step captures ParseResult.ParserName (54-04
--     consumes; the action_doc_uploaded audit's parser_version now reads
--     the real registry name instead of the legacy "unknown" stub).
--   - file_size: NOT in the 54-01 plan DDL — added because the
--     action_doc_uploaded audit payload (migrated verbatim from upload.go
--     STEP 9 into the runner's done path) carries file_size, and the job
--     row is the only state the async runner has. Deviation recorded in
--     the plan handoff.
--   - status (job): queued | running | partial | done | failed. partial is
--     reserved for 54-03 (job not failed but some items failed — SC 7);
--     ClaimJob accepts it as a re-claimable state.
CREATE TABLE IF NOT EXISTS ingest_jobs (
    job_id      TEXT PRIMARY KEY,
    slug        TEXT NOT NULL,
    filename    TEXT NOT NULL,
    sha256      TEXT NOT NULL,
    mime        TEXT NOT NULL DEFAULT '',
    file_size   INTEGER NOT NULL DEFAULT 0,
    doc_id      TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    parser_name TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'queued',
    error       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT ''
);

-- ingest_steps: one row per (job, step) of the eight-step pipeline
-- (ingest.StepSequence is the ordering contract; this table stores no
-- sequence column — consumers order by StepSequence index so a future
-- step insertion cannot rewrite history).
--
--   - attempt: incremented by the runner at every step START, never reset
--     — 54-03 step retry reads it to know how many times a step ran.
--   - error: FIRST-CLASS failure field — the failure reason MUST be
--     queryable from the job surface, not buried in server logs (Dify
--     lesson; SC 1). Non-error notes also live here: skipped steps record
--     the reason ("ecl auto-trigger not wired (54-04)") in error.
--   - artifact_ref: per-step product locator. 54-02 fixes the on-disk
--     semantics (staging, D-54-02); this plan already writes the natural
--     refs where one exists (parse → doc_id, vector_insert → written PK
--     set) so the column is exercised from day one.
--   - stale: RESERVED for 54-03 — "result invalidated by an upstream
--     retry" (Dify re_segment analog). Only the constant is defined this
--     phase; no writer exists yet.
CREATE TABLE IF NOT EXISTS ingest_steps (
    job_id       TEXT NOT NULL,
    step         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    attempt      INTEGER NOT NULL DEFAULT 0,
    started_at   TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    error        TEXT NOT NULL DEFAULT '',
    artifact_ref TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (job_id, step)
);

-- ingest_items: the item-level ledger under one step (SC 7 / Bedrock
-- "COMPLETE but N docs failed" model). v1 writes ONLY embed rows
-- (item_key = chunk content_hash); vector_insert is batch-atomic (no
-- per-item decomposition), and the ECL item ledger is the EXISTING
-- ecl_failures table — a second one here would fork the dead-letter
-- semantics. 54-02 (embed cache / per-chunk retry) is the first writer;
-- this plan ships the schema only.
CREATE TABLE IF NOT EXISTS ingest_items (
    job_id     TEXT NOT NULL,
    step       TEXT NOT NULL,
    item_key   TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    error      TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (job_id, step, item_key)
);

-- embed_cache (Phase 54 / Plan 54-02 / ING-04 / SC 5): the vector lookaside
-- keyed by the Dify 'Embedding(model_name, hash)' pattern — a composite PK
-- of (model, content_hash) IS the uniqueness mechanism. A re-run (54-03 step
-- retry, re-upload of unchanged content) that reaches the embed step finds
-- every unchanged chunk's vector here and costs ZERO BGE-M3 calls.
--
-- IMMUTABLE CACHE ROWS — stronger than ecl_failures, weaker than audit:
-- the same (model, content_hash) pair is deterministic (same model ⇒ same
-- vector), so rows are INSERT-OR-IGNORE'd once and never updated. No
-- UPDATE path exists in embed_cache.go, and a PutEmbedding with a
-- different vector under the same key is a no-op by design — the first
-- write wins and reproducibility is preserved. Rows are deleted only by
-- manual operator SQL (a model swap does NOT purge: the old model's rows
-- simply stop being looked up, because every lookup carries the model).
--
--   - vector: float32 LITTLE-ENDIAN packed BLOB (4 bytes/dim; 1024-dim
--     BGE-M3 ≈ 4 KB/chunk, a thousand-chunk tier ≈ 4 MB — an acceptable
--     skillforge.db size/budget, and the SQLite backup path is the
--     proven recovery story). Byte order is pinned by
--     TestEmbedCacheBlobRoundTrip (encode/decode + a literal-bytes
--     assertion) so a backup/restored DB on another machine decodes
--     identically.
--   - dim: the row's dimensionality — the cache read path fails LOUDLY on
--     a stored-dim != embedder-dim mismatch instead of silently inserting
--     a mixed-dimension vector (the embedder.Embedder contract's "MUST
--     equal Dimensions()" moat).
--   - created_at: RFC3339Nano UTC string SET BY THE CALLER (never a SQL
--     default — the ecl_failures.updated_at timestamp discipline).
--   - No index beyond the PK: every read is an exact-key batch lookup.
CREATE TABLE IF NOT EXISTS embed_cache (
    model        TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    dim          INTEGER NOT NULL,
    vector       BLOB NOT NULL,          -- float32 little-endian packed
    created_at   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (model, content_hash)
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

	// 2026-11-09 最小身份层：session_meta.owner_token 记录会话归属。
	// 空串 = 无归属（本次改造之前创建的历史会话）。写操作对无归属会话
	// fail-closed 要求 admin —— 无法证明归属的东西不能被匿名裁决。
	alterSessionMetaAddOwnerTokenSQL = "ALTER TABLE session_meta ADD COLUMN owner_token TEXT NOT NULL DEFAULT ''"

	// Phase 20 / Plan 20-06 (ADM-06) — actor_type discriminator for the
	// admin audit sub-view. Admin handlers tag their audit rows with
	// actor_type='admin'; regular chat-path rows get the SQL DEFAULT
	// 'user'. ADDITIVE migration only — no backfill needed because the
	// DEFAULT 'user' covers existing rows.
	alterAuditAddActorTypeSQL = "ALTER TABLE audit ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user'"

	// Phase 15 / Plan 15-02 — DATA-16 idempotency key + D15-C2 period
	// + D15-D3 k_min override. NOT NULL DEFAULT '' / 0 keeps the
	// migration additive for existing datasets rows; the import pipeline
	// populates the columns on every new/updated row.
	alterDatasetsAddDatasetSQL      = "ALTER TABLE datasets ADD COLUMN dataset TEXT NOT NULL DEFAULT ''"
	alterDatasetsAddPeriodSQL       = "ALTER TABLE datasets ADD COLUMN period TEXT NOT NULL DEFAULT ''"
	alterDatasetsAddKMinOverrideSQL = "ALTER TABLE datasets ADD COLUMN k_min_override INTEGER NOT NULL DEFAULT 0"

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

// createReportsPeriodVersionUniqueIdxSQL is the WR-09 (18-REVIEW)
// uniqueness backstop against the vN race: two concurrent publishes of
// the same period would otherwise scan reports/<period>/ at nearly the
// same moment, both compute the same MAX(v)+1, and os.WriteFile the
// second copy on top of the first (file truncation — the second
// publish silently overwrites the first). The UNIQUE index rejects
// the duplicate PutReport row at the DB; report.Publish surfaces the
// UNIQUE violation (with the file path so the operator can recover
// the overwritten vN) instead of the previous fail-silent overwrite.
const createReportsPeriodVersionUniqueIdxSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_period_version ON reports(period, version)
`

// report_runs (Phase 19 / Plan 19-01 / PORT-01): indexes for the
// scheduler-triggered run audit table.
const (
	createReportRunsSkillPeriodIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_report_runs_skill_period ON report_runs(skill_id, period)
`
	createReportRunsFiredAtIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_report_runs_fired_at ON report_runs(fired_at)
`
)

// idxKnowledgeBasesRole (Phase 26 / Plan 26-02 / KBM-01): the role index
// for the knowledge_bases registry table — the read path filters by role
// (annotate's counter-evidence channel resolves the historical-role set,
// the role wildcard expands by role membership).
//
// Executed at the TAIL of migrate() AFTER the ALTER loops, per the D14-E1
// ordering contract: never inline an index into schemaSQL. knowledge_bases
// itself is brand-new (additive, no ALTER dependency) but keeps the same
// migrate()-tail convention so there is exactly one index shape to
// remember. CREATE INDEX IF NOT EXISTS is idempotent at the SQLite level;
// no PRAGMA gating needed.
const idxKnowledgeBasesRole = `
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_role ON knowledge_bases(role)
`

// feedback / feedback_chunk_refs / feedback_dispositions / tickets
// read-path indexes (Phase 29 / Plan 29-02 / FBK-01..03 + TKT-02). Six
// independent consts executed at the TAIL of migrate() — the same shape
// discipline as idx_kpi_calls_* and idxKnowledgeBasesRole. All four
// tables are brand-new (additive CREATE TABLE IF NOT EXISTS, zero ALTER
// dependency), but every post-schemaSQL index stays in the
// migrate()-tail convention so the D14-E1 ordering contract keeps
// exactly one shape to remember.
//
//   - idx_feedback_session: per-conversation vote history (29-03
//     VotedVerdictsBySession read).
//   - idx_feedback_ts: metrics time-window scans (D-29-10/11).
//   - idx_feedback_skill: per-Skill metric aggregation (FBK-03).
//   - idx_refs_chunk: per-chunk pending-review queue reads.
//   - idx_dispositions_chunk: latest-disposition-wins lookup per chunk.
//   - idx_tickets_session: per-conversation ticket lookups.
const (
	idxFeedbackSession = `
CREATE INDEX IF NOT EXISTS idx_feedback_session ON feedback(session_id)
`

	idxFeedbackTS = `
CREATE INDEX IF NOT EXISTS idx_feedback_ts ON feedback(ts)
`

	idxFeedbackSkill = `
CREATE INDEX IF NOT EXISTS idx_feedback_skill ON feedback(skill_id)
`

	idxFeedbackChunkRefsChunk = `
CREATE INDEX IF NOT EXISTS idx_refs_chunk ON feedback_chunk_refs(chunk_id)
`

	idxFeedbackDispositionsChunk = `
CREATE INDEX IF NOT EXISTS idx_dispositions_chunk ON feedback_dispositions(chunk_id)
`

	idxTicketsSession = `
CREATE INDEX IF NOT EXISTS idx_tickets_session ON tickets(session_id)
`
)

// approval_rules / approval_rule_hits indexes (Phase 56 / Plan 56-01 /
// D-56-01). Five consts executed at the TAIL of migrate() — AFTER the
// legacy rebuild step, per the D14-E1 ordering contract.
//
// The ordering is LOAD-BEARING, not stylistic: on a pre-56 database the
// schemaSQL `CREATE TABLE IF NOT EXISTS approval_rules` is a no-op (the
// legacy four-tuple-PK table already exists), so an index inlined into
// schemaSQL would reference match_type / scope / args_pattern before
// the rebuild step introduces them and New() would die with
// "no such column: match_type" — the exact UAT 14-09 landmine
// (idx_audit_run_path precedent). Execution order in migrate():
//
//		legacy rebuild (probe-gated) → these five CREATE INDEX statements
//
//	  - idx_approval_rules_natural: the UNIQUE natural key replacing the
//	    legacy four-tuple PK — (target, match_type, args_hash,
//	    args_pattern, skill_id, scope, scope_session_id). The legacy row
//	    set maps injectively onto it, so the rebuild INSERT cannot trip
//	    the UNIQUE constraint.
//	  - idx_approval_rules_args_hash: the exact-tier candidate lookup
//	    (args_hash is an index column now, not a key).
//	  - idx_approval_rules_scope:     scope listing (panel + engine).
//	  - idx_approval_rules_skill:     skill-scope listing.
//	  - idx_rule_hits_rule:           per-rule hit history reads.
const (
	createApprovalRulesNaturalIdxSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_approval_rules_natural
    ON approval_rules(target, match_type, args_hash, args_pattern, skill_id, scope, scope_session_id)
`

	createApprovalRulesArgsHashIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_approval_rules_args_hash ON approval_rules(args_hash)
`

	createApprovalRulesScopeIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_approval_rules_scope ON approval_rules(scope, scope_session_id)
`

	createApprovalRulesSkillIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_approval_rules_skill ON approval_rules(skill_id)
`

	createRuleHitsRuleIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_rule_hits_rule ON approval_rule_hits(rule_id, ts)
`
)

// createIngestJobsSlugSHAIdxSQL (Phase 54 / Plan 54-01 / ING-04) backs the
// extended upload dedup: the handler answers 409 when a NON-failed job for
// the same (slug, sha256) already exists (the in-flight-window dedup the
// audit-row-only check cannot see). ingest_steps / ingest_items need no
// extra index — their composite PKs ((job_id, step) / (job_id, step,
// item_key)) prefix-serve every job-keyed read this phase ships.
//
// Executed at the TAIL of migrate() per the D14-E1 ordering contract; all
// three ingest tables are brand-new (additive CREATE TABLE IF NOT EXISTS,
// zero ALTER dependency) but keep the one-shape migrate()-tail convention.
// CREATE INDEX IF NOT EXISTS is idempotent at the SQLite level.
const createIngestJobsSlugSHAIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_ingest_jobs_slug_sha ON ingest_jobs(slug, sha256)
`

// Phase 56 / Plan 56-01 / D-56-01 — the legacy approval_rules rebuild.
//
// This is the FIRST table-rebuild migration in the project (the house
// convention until now was PRAGMA-gated ALTER, which cannot rewrite a
// PRIMARY KEY). Legacy form: PK (target, args_hash, skill_id,
// scope_session_id), binary scope encoded as scope_session_id ” vs
// non-empty. New form: surrogate `id` PK + match_type/scope/enabled/
// expires_at/origin/hit_count/last_hit_at columns.
//
// legacyApprovalRulesV2DDL is the new-shape table under the temporary
// name. It MUST stay column-for-column identical to the schemaSQL
// approval_rules declaration — the DDL tests assert both fresh-DB and
// rebuilt-DB shapes against the same expectedApprovalRulesCols list,
// so a drift between the two declarations fails loudly.
const legacyApprovalRulesV2DDL = `
CREATE TABLE approval_rules_v2 (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    target           TEXT NOT NULL,
    match_type       TEXT NOT NULL DEFAULT 'exact'
                     CHECK (match_type IN ('exact','prefix','template')),
    args_hash        TEXT NOT NULL DEFAULT '',
    args_pattern     TEXT NOT NULL DEFAULT '',
    skill_id         TEXT NOT NULL DEFAULT '',
    scope            TEXT NOT NULL DEFAULT 'session'
                     CHECK (scope IN ('session','skill','global')),
    scope_session_id TEXT NOT NULL DEFAULT '',
    enabled          INTEGER NOT NULL DEFAULT 1,
    expires_at       INTEGER,
    origin           TEXT NOT NULL DEFAULT 'approve_remember'
                     CHECK (origin IN ('approve_remember','manual','seed')),
    hit_count        INTEGER NOT NULL DEFAULT 0,
    last_hit_at      INTEGER,
    created_at       INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    last_used_at     INTEGER NOT NULL DEFAULT (strftime('%s','now'))
)
`

// legacyApprovalRulesCopySQL maps each legacy row onto the new shape:
//
//   - match_type = 'exact'      — the only matching tier the legacy
//     engine had (matchRuleSQL was args_hash equality).
//   - scope = CASE              — the D11-D3 binary encoding decoded
//     back: scope_session_id != ” → 'session', ” → 'skill'. There is
//     no global scope in legacy data.
//   - last_hit_at = last_used_at — legacy last_used_at MEANT "when the
//     rule last auto-passed a write" (UpdateLastUsedAt); in the new
//     model that semantic belongs to last_hit_at.
//   - origin = DEFAULT ('approve_remember') — the only legacy source.
//   - hit_count = DEFAULT 0, enabled = DEFAULT 1, expires_at = NULL.
//
// The mapping is injective on the legacy PK (each legacy row lands on
// a distinct natural key), so the INSERT cannot trip
// idx_approval_rules_natural — no merge/dedup needed.
const legacyApprovalRulesCopySQL = `
INSERT INTO approval_rules_v2 (
    target, match_type, args_hash, skill_id, scope, scope_session_id,
    created_at, last_used_at, last_hit_at
) SELECT
    target, 'exact', args_hash, skill_id,
    CASE WHEN scope_session_id = '' THEN 'skill' ELSE 'session' END,
    scope_session_id, created_at, last_used_at, last_used_at
FROM approval_rules
`

// rebuildLegacyApprovalRules upgrades a pre-56 approval_rules table in
// place: create approval_rules_v2 (new DDL) → copy rows with the
// legacy→policy mapping → drop the legacy table → rename v2 into
// place, all inside ONE transaction. The legacy indexes
// (idx_approval_rules_scope_session / idx_approval_rules_skill) die
// with the DROP TABLE; the migrate() tail recreates the new index set
// right after this returns.
//
// Probe gate: PRAGMA table_info(approval_rules) must NOT contain
// match_type — a fresh 56+ database (or an already-rebuilt one) has
// the column, making the whole step a no-op (idempotent on every
// New(), same posture as the ALTER gates above).
//
// FK safety: no table holds a FOREIGN KEY into approval_rules (the
// audit table records lifecycle events by value, not by FK — D11-D2
// single-table reuse note in approval_rules_api.go), so DROP TABLE
// cannot orphan anything.
func (s *SQLiteStore) rebuildLegacyApprovalRules(ctx context.Context) error {
	hasMatchType, err := s.columnExists(ctx, "approval_rules", "match_type")
	if err != nil {
		return err
	}
	if hasMatchType {
		return nil // already the 56+ shape — nothing to rebuild
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmtError("rebuildLegacyApprovalRules.BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, legacyApprovalRulesV2DDL); err != nil {
		return fmtError("rebuildLegacyApprovalRules.v2_ddl", err)
	}
	if _, err := tx.ExecContext(ctx, legacyApprovalRulesCopySQL); err != nil {
		return fmtError("rebuildLegacyApprovalRules.copy", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE approval_rules`); err != nil {
		return fmtError("rebuildLegacyApprovalRules.drop", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE approval_rules_v2 RENAME TO approval_rules`); err != nil {
		return fmtError("rebuildLegacyApprovalRules.rename", err)
	}
	return tx.Commit()
}

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
	// Phase 20 / Plan 20-06 (ADM-06) — actor_type discriminator for admin audit.
	{"actor_type", alterAuditAddActorTypeSQL},
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

	// 2026-11-09 最小身份层：session_meta.owner_token（幂等，PRAGMA
	// table_info 门控，与上面几组 ALTER 同一套姿态）。
	if exists, err := s.columnExists(ctx, "session_meta", "owner_token"); err != nil {
		return err
	} else if !exists {
		if _, err := s.db.ExecContext(ctx, alterSessionMetaAddOwnerTokenSQL); err != nil {
			return fmtError("migrate.session_meta.owner_token", err)
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
	// WR-09 (18-REVIEW): UNIQUE backstop on (period, version). Reject the
	// vN race at the DB boundary instead of silently overwriting the
	// previously-published vN file.
	if _, err := s.db.ExecContext(ctx, createReportsPeriodVersionUniqueIdxSQL); err != nil {
		return fmtError("migrate.reports.period_version_idx", err)
	}

	// Phase 19 / Plan 19-01 / PORT-01: report_runs audit table indexes.
	if _, err := s.db.ExecContext(ctx, createReportRunsSkillPeriodIdxSQL); err != nil {
		return fmtError("migrate.report_runs.skill_period_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createReportRunsFiredAtIdxSQL); err != nil {
		return fmtError("migrate.report_runs.fired_at_idx", err)
	}

	// Phase 26 / Plan 26-02 / KBM-01: knowledge_bases role index. Executed
	// AFTER the ALTER loops per the D14-E1 ordering posture; the table
	// itself is brand-new (additive, no ALTER dependency) but keeps the
	// same migrate()-tail convention as kpi_calls / reports / report_runs.
	if _, err := s.db.ExecContext(ctx, idxKnowledgeBasesRole); err != nil {
		return fmtError("migrate.knowledge_bases.role_idx", err)
	}

	// Phase 29 / Plan 29-02 — feedback / feedback_chunk_refs /
	// feedback_dispositions / tickets read-path indexes. All four tables
	// are brand-new (additive, no ALTER dependency); the migrate()-tail
	// convention is kept so the D14-E1 ordering contract continues to
	// have exactly one index shape to remember.
	if _, err := s.db.ExecContext(ctx, idxFeedbackSession); err != nil {
		return fmtError("migrate.feedback.session_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, idxFeedbackTS); err != nil {
		return fmtError("migrate.feedback.ts_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, idxFeedbackSkill); err != nil {
		return fmtError("migrate.feedback.skill_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, idxFeedbackChunkRefsChunk); err != nil {
		return fmtError("migrate.feedback_chunk_refs.chunk_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, idxFeedbackDispositionsChunk); err != nil {
		return fmtError("migrate.feedback_dispositions.chunk_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, idxTicketsSession); err != nil {
		return fmtError("migrate.tickets.session_idx", err)
	}

	// Phase 56 / Plan 56-01 / D-56-01 — the approval_rules legacy
	// rebuild (project's FIRST table-rebuild migration; PRAGMA-gated
	// ALTER cannot rewrite a PRIMARY KEY). Probe-gated: a no-op on any
	// DB already carrying the 56+ shape. MUST run BEFORE the approval
	// rules index block below — on a pre-56 database the new-shape
	// columns (match_type / scope / args_pattern / ...) exist only
	// after this swap (UAT 14-09 "no such column" ordering contract).
	if err := s.rebuildLegacyApprovalRules(ctx); err != nil {
		return err
	}

	// Phase 56 / Plan 56-01 / D-56-01 — approval_rules /
	// approval_rule_hits read-path indexes. Executed AFTER the rebuild
	// per the ordering contract documented on the const block.
	if _, err := s.db.ExecContext(ctx, createApprovalRulesNaturalIdxSQL); err != nil {
		return fmtError("migrate.approval_rules.natural_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createApprovalRulesArgsHashIdxSQL); err != nil {
		return fmtError("migrate.approval_rules.args_hash_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createApprovalRulesScopeIdxSQL); err != nil {
		return fmtError("migrate.approval_rules.scope_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createApprovalRulesSkillIdxSQL); err != nil {
		return fmtError("migrate.approval_rules.skill_idx", err)
	}
	if _, err := s.db.ExecContext(ctx, createRuleHitsRuleIdxSQL); err != nil {
		return fmtError("migrate.approval_rule_hits.rule_idx", err)
	}

	// Phase 54 / Plan 54-01 / ING-04 — ingest_jobs (slug, sha256) dedup
	// index. Fresh additive tables (no ALTER dependency); migrate()-tail
	// convention per D14-E1.
	if _, err := s.db.ExecContext(ctx, createIngestJobsSlugSHAIdxSQL); err != nil {
		return fmtError("migrate.ingest_jobs.slug_sha_idx", err)
	}
	return nil
}
