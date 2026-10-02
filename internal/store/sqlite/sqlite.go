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
// It uses github.com/mattn/go-sqlite3 (the CGO SQLite3 driver, bundled
// SQLite 3.53.4). Building this package requires a C compiler — run
// `make check-cgo` before `go build` to verify gcc is on PATH.
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

	// Blank-import the CGO SQLite3 driver. It registers itself under
	// the name "sqlite3" with database/sql — no init-time network calls
	// (only sql.Register). Verified legitimate in Plan 14-03 Task 1
	// (MIT, source github.com/mattn/go-sqlite3 v1.14.50).
	_ "github.com/mattn/go-sqlite3"
)

// Config configures the SQLiteStore. DSN defaults to ":memory:" when
// empty (useful for tests); MaxOpenConns defaults to 1 for in-memory
// DSNs (bare ":memory:" or "file:...?mode=memory") and 4 for file-backed
// DSNs (SQLite serializes writes through its own file lock — 4 reader
// connections is a safe conservative pool size).
type Config struct {
	DSN          string
	MaxOpenConns int
}

func (cfg Config) withDefaults() Config {
	if cfg.DSN == "" {
		cfg.DSN = ":memory:"
	}
	if cfg.MaxOpenConns == 0 {
		// In-memory databases (bare ":memory:" or "file:...?mode=memory")
		// use a single connection. With shared-cache in-memory DBs, multiple
		// pooled connections can deadlock on table locks even with
		// busy_timeout; one connection avoids the class of failures without
		// changing the test concurrency posture at the goroutine level.
		// File-backed DSNs keep the conservative pool size of 4.
		if isMemoryDSN(cfg.DSN) {
			cfg.MaxOpenConns = 1
		} else {
			cfg.MaxOpenConns = 4
		}
	}
	return cfg
}

// SQLiteStore implements store.Store against a github.com/mattn/go-sqlite3
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
// runs schema migrations, and returns a ready store. busy_timeout,
// WAL journal_mode and foreign_keys are applied through DSN parameters
// so every connection opened by the pool inherits them — post-open
// PRAGMAs only affect one pooled connection (Pitfall 2). WAL is a
// no-op for :memory: databases, which remain in-memory.
func New(ctx context.Context, cfg Config) (*SQLiteStore, error) {
	cfg = cfg.withDefaults()
	db, err := sql.Open("sqlite3", dsnWithPragmas(cfg.DSN))
	if err != nil {
		return nil, fmt.Errorf("sqlite.New: sql.Open(%q): %w", cfg.DSN, err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	s := &SQLiteStore{cfg: cfg, db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// dsnWithPragmas appends the canonical SQLite pragmas to the DSN.
// Using DSN parameters instead of post-open PRAGMAs ensures every
// connection in the pool sees busy_timeout=5000 (waits on lock rather
// than returning SQLITE_BUSY), WAL mode (no-op for :memory:), and
// foreign-key enforcement.
func dsnWithPragmas(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=1"
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
