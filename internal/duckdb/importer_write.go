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

package duckdb

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"iter"
	"regexp"

	"github.com/marcboeker/go-duckdb/v2"
)

// Allowed staging column types. These are the only DuckDB types the Appender
// path is willing to create; types like BIT/ANY/INVALID are rejected up front
// because go-duckdb's Appender cannot create them (T-15-16).
var allowedStagingTypes = map[string]struct{}{
	"VARCHAR":   {},
	"BIGINT":    {},
	"DOUBLE":    {},
	"TIMESTAMP": {},
}

// ErrUnsupportedColumnType is returned when a StagingTable declares a DuckDB
// type outside the allowed staging allowlist.
var ErrUnsupportedColumnType = errors.New("duckdb: unsupported staging column type")

// ErrInvalidIdentifier is returned when a table or column name contains
// characters outside the defensive identifier allowlist.
var ErrInvalidIdentifier = errors.New("duckdb: invalid staging identifier")

// ErrTableExists is returned when WriteStagingBatch targets a staging file
// that already contains the named table. Import batches must write fresh
// staging files (SC-1 / STACK-VERIFICATION §2.8).
var ErrTableExists = errors.New("duckdb: staging table already exists")

// identifierRegexp is a defensive allowlist for table/column identifiers.
// It is intentionally narrower than dataset ids: no hyphens and a generous
// length cap so SQL identifiers stay simple and quote-free.
var identifierRegexp = regexp.MustCompile(`^[A-Za-z0-9_]{1,128}$`)

// ColumnSpec describes one column of the staging table.
type ColumnSpec struct {
	// Name is the column identifier. It must satisfy identifierRegexp.
	Name string
	// DuckDBType is the target DuckDB type. It must be one of the
	// allowedStagingTypes values.
	DuckDBType string
}

// StagingTable describes the table to create in a staging .duckdb file.
type StagingTable struct {
	// Name is the table identifier. It must satisfy identifierRegexp.
	Name string
	// Columns is the ordered list of column specifications.
	Columns []ColumnSpec
}

// WriteStagingBatch creates a fresh staging .duckdb file at stagingPath and
// writes rows to it using the Appender bulk path. It is the production
// import-pipeline's only allowed way to persist desensitized rows
// (SC-1 / STACK-VERIFICATION §2.8): no row-by-row INSERT loops.
//
// Semantics:
//   - Column types are checked against an allowlist before any DDL/Appender
//     work; unsupported types (e.g. BIT) return ErrUnsupportedColumnType.
//   - Table and column names are checked against identifierRegexp as defense
//     in depth; callers (the rules layer) are expected to sanitize them too.
//   - The target table must NOT already exist in the staging file; otherwise
//     ErrTableExists is returned. Import batches always write new staging
//     files, so an existing table signals reuse/corruption.
//   - A dedicated Connector with MaxOpenConns=1 is used; closing it flushes
//     the WAL so the file is ready for CommitStagedDataset.
//   - The returned int64 is the number of rows appended.
func WriteStagingBatch(ctx context.Context, stagingPath string, table StagingTable, rows iter.Seq[[]any]) (int64, error) {
	if !identifierRegexp.MatchString(table.Name) {
		return 0, fmt.Errorf("%w: table name %q", ErrInvalidIdentifier, table.Name)
	}
	if len(table.Columns) == 0 {
		return 0, fmtError("WriteStagingBatch", errors.New("table must have at least one column"))
	}

	var colDefs []string
	for _, col := range table.Columns {
		if !identifierRegexp.MatchString(col.Name) {
			return 0, fmt.Errorf("%w: column name %q", ErrInvalidIdentifier, col.Name)
		}
		if _, ok := allowedStagingTypes[col.DuckDBType]; !ok {
			return 0, fmt.Errorf("%w: %q (allowed: VARCHAR, BIGINT, DOUBLE, TIMESTAMP)", ErrUnsupportedColumnType, col.DuckDBType)
		}
		colDefs = append(colDefs, fmt.Sprintf("%s %s", col.Name, col.DuckDBType))
	}

	c, err := New(ctx, Config{DSN: stagingPath, MaxOpenConns: 1})
	if err != nil {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("open staging connector: %w", err))
	}
	defer func() {
		_ = c.Close()
	}()

	// Reject existing table — import batches must target fresh staging files.
	var existing int
	if err := c.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'main' AND table_name = ?",
		table.Name).Scan(&existing); err != nil {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("check existing table: %w", err))
	}
	if existing != 0 {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("%w: %s", ErrTableExists, table.Name))
	}

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", table.Name, joinColumnDefs(colDefs))
	if _, err := c.DB().ExecContext(ctx, createSQL); err != nil {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("create table: %w", err))
	}

	driverConn, err := c.connector.Connect(ctx)
	if err != nil {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("connect for appender: %w", err))
	}
	defer func() {
		_ = driverConn.Close()
	}()

	app, err := duckdb.NewAppenderFromConn(driverConn, "main", table.Name)
	if err != nil {
		return 0, fmtError("WriteStagingBatch", fmt.Errorf("create appender: %w", err))
	}
	defer func() {
		_ = app.Close()
	}()

	var written int64
	for row := range rows {
		values := make([]driver.Value, len(row))
		for i, v := range row {
			values[i] = driver.Value(v)
		}
		if err := app.AppendRow(values...); err != nil {
			return written, fmtError("WriteStagingBatch", fmt.Errorf("append row %d: %w", written, err))
		}
		written++
	}

	if err := app.Flush(); err != nil {
		return written, fmtError("WriteStagingBatch", fmt.Errorf("flush appender: %w", err))
	}

	return written, nil
}

// joinColumnDefs joins column definitions with a comma. It exists only to keep
// the CREATE TABLE text construction readable; the values are already validated.
func joinColumnDefs(defs []string) string {
	out := ""
	for i, d := range defs {
		if i > 0 {
			out += ", "
		}
		out += d
	}
	return out
}
