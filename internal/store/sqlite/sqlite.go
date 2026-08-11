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

// Package sqlite is the zero-ops default implementation of store.Store.
// It uses modernc.org/sqlite (a pure-Go transpilation of SQLite3 — no
// cgo, no postinstall scripts, BSD-3-Clause). The driver registers
// under the name "sqlite" via a blank import below.
//
// The implementation mirrors the D11 retriever pattern (interface-first,
// cfg+conn struct shape, compile-time conformance check, Config.withDefaults).
// Production-swappable for a Postgres backend without changing any caller.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/skillforge/skillforge/internal/store"

	// Blank-import the pure-Go SQLite driver. It registers itself under
	// the name "sqlite" with database/sql — no init-time network calls
	// (only sql.Register). Verified legitimate in Plan 03-02 Task 1
	// (BSD-3-Clause, source gitlab.com/cznic/sqlite).
	_ "modernc.org/sqlite"
)

// Config configures the SQLiteStore. DSN defaults to ":memory:" when
// empty (useful for tests); MaxOpenConns defaults to 1 for ":memory:"
// (every connection otherwise gets its own private in-memory DB) and
// 4 for a file-backed DSN (SQLite serializes writes through its own
// file lock — 4 reader connections is a safe conservative pool size).
type Config struct {
	DSN        string
	MaxOpenConns int
}

func (cfg Config) withDefaults() Config {
	if cfg.DSN == "" {
		cfg.DSN = ":memory:"
	}
	if cfg.MaxOpenConns == 0 {
		if cfg.DSN == ":memory:" {
			cfg.MaxOpenConns = 1
		} else {
			cfg.MaxOpenConns = 4
		}
	}
	return cfg
}

// SQLiteStore implements store.Store against a modernc.org/sqlite
// database. It owns the underlying *sql.DB pool. The closed flag is
// mutex-guarded so concurrent Close/method calls behave deterministically
// (the SC-9 -race test exercises this path).
type SQLiteStore struct {
	cfg    Config
	db     *sql.DB
	closed bool
	mu     sync.Mutex // guards `closed`
}

// Compile-time check that *SQLiteStore satisfies store.Store. Drift in
// the interface will fail the build here, not at runtime — the D11
// contract.
var _ store.Store = (*SQLiteStore)(nil)

// New opens the SQLite database via database/sql, configures the pool,
// runs schema migrations, and returns a ready store. WAL journal_mode
// is enabled on file-backed DSNs (no-op for :memory:); foreign_keys is
// ON so future FK constraints are enforced.
func New(ctx context.Context, cfg Config) (*SQLiteStore, error) {
	cfg = cfg.withDefaults()
	db, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlite.New: sql.Open(%q): %w", cfg.DSN, err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	// PRAGMAs — best-effort; :memory: tolerates WAL as a no-op.
	// foreign_keys=ON is forward-looking (no FKs in the v1 schema).
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		if !isMemoryDSN(cfg.DSN) {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite.New: PRAGMA journal_mode=WAL: %w", err)
		}
		// :memory: rejects WAL — tolerate and continue.
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite.New: PRAGMA foreign_keys=ON: %w", err)
	}

	s := &SQLiteStore{cfg: cfg, db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Health runs SELECT 1 against the pool. Used by cmd/server startup to
// fail fast on a misconfigured DSN.
func (s *SQLiteStore) Health(ctx context.Context) error {
	if s.isClosed() {
		return sql.ErrConnDone
	}
	var one int
	if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmtError("Health SELECT 1", err)
	}
	if one != 1 {
		return fmt.Errorf("sqlite.Health: unexpected probe result %d", one)
	}
	return nil
}

// Close releases the underlying connection pool. After Close, every
// method returns sql.ErrConnDone (or the driver's equivalent).
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	return s.db.Close()
}

// DB exposes the underlying *sql.DB for operator-side SQL forensics
// (e.g. the SC-7 regression test that runs a raw `chunk_ids LIKE` query
// to prove the column is queryable without JSON_EXTRACT). Production
// callers (cmd/server, eval sidecar) MUST NOT use this — go through
// Store methods instead.
func (s *SQLiteStore) DB() *sql.DB { return s.db }

// isClosed is the read-side of the closed flag, mutex-guarded so the
// -race detector stays green under concurrent Close/method calls.
func (s *SQLiteStore) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// fmtError wraps a driver error with a method-name prefix. Centralized
// so the wrapping convention stays consistent across PutAudit /
// QueryAudit / GetSession / AppendTurn / Health.
func fmtError(method string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlite.%s: %w", method, err)
}

// isMemoryDSN detects whether the DSN targets an in-memory database
// (either the bare ":memory:" form or the URI form with mode=memory).
// Used to tolerate WAL-rejection on :memory: databases.
func isMemoryDSN(dsn string) bool {
	if dsn == ":memory:" {
		return true
	}
	return strings.Contains(dsn, "mode=memory")
}

// errEmptySessionID guards AppendTurn / GetSession against an empty
// session_id, which would otherwise silently partition on "" and mix
// sessions that should never share history.
var errEmptySessionID = errors.New("sqlite: session_id must be non-empty")
