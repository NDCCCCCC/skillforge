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

// Package duckdb owns the DuckDB connection mode for SkillForge v1.0 data
// support. It uses github.com/marcboeker/go-duckdb/v2 (CGO, DuckDB engine
// v1.4.1) and enforces three lifecycle disciplines:
//
//   1. Hermetic boot-queries: every new connection replays the two SET
//      constants that disable extension auto-install / auto-load. This keeps
//      the engine from reaching out to the network on first use of an
//      unbundled extension (STACK-VERIFICATION §3.4).
//   2. Temporary-object lifetime: db.SetMaxIdleConns(0) so that temporary
//      catalog state (attached databases, temporary tables) does not outlive
//      the connection that created it.
//   3. WAL flush on close: persistent databases must close both the connector
//      and the sql.DB pool so that the WAL is fully checkpointed and data is
//      not lost (STACK-VERIFICATION §2.1, Pitfall 5).
//
// Datasets are always attached under a sanitized alias; the default database
// stays :memory: and DETACH is never applied to it.
package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/marcboeker/go-duckdb/v2"
)

// bootHermetic contains the two hermetic SET statements. They are deliberately
// a single string that is split on ';' so the source-grep target "autoinstall"
// and "autoload" appear exactly once each and the statements are code, not
// config. Each fragment is executed independently via ExecContext.
//
// P14-SC3 source-grep locked via TestDuckDBBootQueries.
const bootHermetic = `SET autoinstall_known_extensions=false; SET autoload_known_extensions=false;`

// bootQueries returns the hermetic SET fragments as separate SQL strings.
// Callers may prepend ATTACH statements for datasets before passing the slice
// to a boot callback.
func bootQueries() []string {
	var out []string
	for _, q := range strings.Split(bootHermetic, ";") {
		q = strings.TrimSpace(q)
		if q != "" {
			out = append(out, q)
		}
	}
	return out
}

// Config configures a DuckDB Connector. DSN defaults to ":memory:"; use a
// file path for persistent databases. MaxOpenConns defaults to a conservative
// pool size for in-process DuckDB.
type Config struct {
	DSN          string
	MaxOpenConns int
}

func (cfg Config) withDefaults() Config {
	if cfg.DSN == "" {
		cfg.DSN = ":memory:"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 4
	}
	return cfg
}

// Connector wraps duckdb.Connector and the sql.DB pool created from it. It
// exists to enforce the Close() double-close discipline documented above.
type Connector struct {
	cfg       Config
	connector *duckdb.Connector
	db        *sql.DB
}

var (
	// datasetIDRegexp is the allowlist for externally supplied dataset
	// identifiers before they are allowed into an ATTACH SQL statement. It
	// rejects path traversal, whitespace, semicolons, and other SQL injection
	// payloads while still permitting readable identifiers.
	//
	// Threat: T-14-12 ATTACH path injection.
	datasetIDRegexp = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

	errInvalidDatasetID = errors.New("duckdb: invalid dataset identifier")
)

// ValidateDatasetID returns nil if id is safe to use in an ATTACH alias, or
// errInvalidDatasetID otherwise. Exported because callers (e.g. import
// pipeline, KPI registry) must validate identifiers before building SQL text.
func ValidateDatasetID(id string) error {
	if !datasetIDRegexp.MatchString(id) {
		return fmt.Errorf("%w: %q", errInvalidDatasetID, id)
	}
	return nil
}

// AttachDataset validates id and then executes ATTACH for path under the alias
// ds_{id} (READ_ONLY). The path argument must be a caller-trusted filesystem
// path; this function only sanitizes the SQL identifier side (T-14-12). The
// alias is always prefixed with "ds_" so it cannot collide with DuckDB
// reserved catalog names.
func AttachDataset(execer driver.ExecerContext, id, path string) error {
	if err := ValidateDatasetID(id); err != nil {
		return err
	}
	clean := filepath.Clean(path)
	q := fmt.Sprintf("ATTACH '%s' AS \"ds_%s\" (READ_ONLY)", clean, id)
	_, err := execer.ExecContext(context.Background(), q, nil)
	if err != nil {
		return fmtError("AttachDataset", err)
	}
	return nil
}

// New opens a DuckDB connector with hermetic boot-queries and returns the
// wrapper. The caller owns the returned Connector and must call Close when
// done.
func New(ctx context.Context, cfg Config) (*Connector, error) {
	cfg = cfg.withDefaults()
	queries := bootQueries()

	// The boot callback runs on every new pool connection, possibly long after
	// New has returned. Binding it to the caller's ctx would let a cancelled
	// request ctx poison every lazily-opened connection, so the hermetic SETs
	// always run under context.Background() (RESEARCH Pattern 2).
	connector, err := duckdb.NewConnector(cfg.DSN, func(execer driver.ExecerContext) error {
		for _, q := range queries {
			if _, err := execer.ExecContext(context.Background(), q, nil); err != nil {
				return fmtError("boot query", fmt.Errorf("%q: %w", q, err))
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmtError("NewConnector", err)
	}

	db := sql.OpenDB(connector)
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	return &Connector{
		cfg:       cfg,
		connector: connector,
		db:        db,
	}, nil
}

// DB exposes the underlying *sql.DB pool. Callers should prefer the methods on
// Connector when they exist; DB is exposed for query paths that do not need
// special lifecycle handling.
func (c *Connector) DB() *sql.DB {
	return c.db
}

// Close closes both the sql.DB pool and the duckdb.Connector. Closing both is
// a hard requirement for persistent databases so that the WAL is checkpointed
// and data is not lost (Pitfall 5). The calls are idempotent and safe to
// repeat.
func (c *Connector) Close() error {
	var errs []error
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.connector != nil {
		if err := c.connector.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmtError("Close", errors.Join(errs...))
	}
	return nil
}

// fmtError wraps a driver error with a method-name prefix. Mirrors the
// sqlite.fmtError convention (Shared Pattern B).
func fmtError(method string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("duckdb.%s: %w", method, err)
}
