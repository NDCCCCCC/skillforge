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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuckDBBootQueries asserts that the two hermetic SET statements are
// replayed on every new connection, not just the first one. Four concurrent
// connections each query the settings; all must report false.
func TestDuckDBBootQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 4})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	c.DB().SetMaxOpenConns(4)

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for _, key := range []string{"autoinstall_known_extensions", "autoload_known_extensions"} {
				var val string
				row := c.DB().QueryRowContext(ctx, "SELECT current_setting(?)", key)
				if err := row.Scan(&val); err != nil {
					errs <- fmt.Errorf("conn %d %s scan: %w", idx, key, err)
					return
				}
				if val != "false" {
					errs <- fmt.Errorf("conn %d %s = %q, want false", idx, key, val)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// TestDuckDBCloseFlushesWAL writes rows through the Appender to a file-backed
// database, closes both the connector and the db pool, then re-opens the same
// file and asserts that all rows are present. This is the Pitfall 5 WAL-flush
// discipline lock.
func TestDuckDBCloseFlushesWAL(t *testing.T) {
	if runtime.GOOS == "windows" {
		// DuckDB on Windows keeps the file mapped while the connector is open;
		// closing and immediately reopening the same path in one test process is
		// supported, but we keep the defer close discipline to avoid surprises.
		t.Log("running on windows; close+reopen uses file mapping")
	}

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal-test.duckdb")

	const n = 10000

	// Write phase.
	{
		c, err := New(ctx, Config{DSN: dbPath, MaxOpenConns: 1})
		require.NoError(t, err)
		_, err = c.DB().ExecContext(ctx, "CREATE TABLE t (id INTEGER, val DOUBLE)")
		require.NoError(t, err)

		driverConn, err := c.connector.Connect(ctx)
		require.NoError(t, err)
		app, err := duckdb.NewAppender(driverConn, "", "main", "t")
		require.NoError(t, err)

		for i := 0; i < n; i++ {
			require.NoError(t, app.AppendRow(int32(i), float64(i)*0.1))
		}
		require.NoError(t, app.Flush())
		require.NoError(t, app.Close())
		require.NoError(t, driverConn.Close())

		// Close both the pool and the connector (the discipline under test).
		require.NoError(t, c.Close())
	}

	// Re-open and count.
	{
		c, err := New(ctx, Config{DSN: dbPath, MaxOpenConns: 1})
		require.NoError(t, err)
		defer func() { require.NoError(t, c.Close()) }()

		var count int
		require.NoError(t, c.DB().QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count))
		assert.Equal(t, n, count, "WAL did not flush: expected %d rows after close+reopen", n)
	}
}

// TestValidateDatasetID asserts the allowlist used before any dataset
// identifier is allowed into an ATTACH statement. It covers path traversal,
// whitespace, semicolon injection, empty, and overlong inputs (T-14-12).
func TestValidateDatasetID(t *testing.T) {
	t.Parallel()
	valid := []string{"ds_001", "sales-2026", "a", strings.Repeat("a", 64)}
	for _, id := range valid {
		assert.NoError(t, ValidateDatasetID(id), "expected %q to be valid", id)
	}

	invalid := []string{
		"../escape",
		"a b",
		"ds;ATTACH",
		"ds'OR'1'='1",
		"",
		"a/b",
		"a\\b",
		"a.b",
		strings.Repeat("a", 65),
	}
	for _, id := range invalid {
		assert.Error(t, ValidateDatasetID(id), "expected %q to be invalid", id)
	}
}

// TestDuckDBAttachDatasetRejectsInvalidID verifies that AttachDataset refuses
// to build SQL when the identifier fails the allowlist.
func TestDuckDBAttachDatasetRejectsInvalidID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c, err := New(ctx, Config{DSN: ":memory:"})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	// Use a raw driver connection as the ExecerContext.
	driverConn, err := c.connector.Connect(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, driverConn.Close()) }()

	execer, ok := driverConn.(driver.ExecerContext)
	require.True(t, ok, "driver conn must implement ExecerContext")

	err = AttachDataset(execer, "../escape", "data/datasets/x.duckdb")
	require.Error(t, err)
	assert.ErrorIs(t, err, errInvalidDatasetID)
}

// AppenderMinRowsPerSec is the SC-3 throughput floor. It can be overridden via
// DUCKDB_APPENDER_MIN_RPS for CI environments with slower I/O.
//
// Source-grep locked via TestDuckDBAppenderThroughput.
const AppenderMinRowsPerSec = 100_000

// TestDuckDBAppenderThroughput writes 500k rows to a wide table and asserts
// that the Appender path sustains at least AppenderMinRowsPerSec rows/sec.
func TestDuckDBAppenderThroughput(t *testing.T) {
	ctx := context.Background()
	c, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 1})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	_, err = c.DB().ExecContext(ctx, `CREATE TABLE wide (
		i0 INTEGER, i1 INTEGER, i2 INTEGER, i3 INTEGER, i4 INTEGER,
		d0 DOUBLE, d1 DOUBLE,
		v0 VARCHAR, v1 VARCHAR, v2 VARCHAR
	)`)
	require.NoError(t, err)

	driverConn, err := c.connector.Connect(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, driverConn.Close()) }()

	app, err := duckdb.NewAppender(driverConn, "", "main", "wide")
	require.NoError(t, err)
	defer func() { require.NoError(t, app.Close()) }()

	const rows = 500_000
	start := time.Now()
	for i := 0; i < rows; i++ {
		require.NoError(t, app.AppendRow(
			int32(i), int32(i%100), int32(i%1000), int32(i%10), int32(i%2),
			float64(i)*0.1, float64(i)*0.01,
			fmt.Sprintf("row-%d", i), fmt.Sprintf("val-%d", i%1000), "fixed",
		))
	}
	require.NoError(t, app.Flush())
	elapsed := time.Since(start)

	rps := float64(rows) / elapsed.Seconds()
	t.Logf("Appender throughput: %.0f rows/sec over %d rows (%s)", rps, rows, elapsed)

	min := float64(AppenderMinRowsPerSec)
	if v := os.Getenv("DUCKDB_APPENDER_MIN_RPS"); v != "" {
		var parsed float64
		if _, err := fmt.Sscanf(v, "%f", &parsed); err == nil {
			min = parsed
		}
	}
	assert.GreaterOrEqual(t, rps, min, "Appender throughput below threshold")
}

// ReaderBlockFactor is the multiplier used in TestDuckDBConcurrentReadNoBlockWrite.
// The test asserts that writing with 50 concurrent readers takes less than
// (write-alone time * ReaderBlockFactor). Overridable via DUCKDB_READER_BLOCK_FACTOR.
const ReaderBlockFactor = 3.0

// TestDuckDBConcurrentReadNoBlockWrite writes a large batch while 50 goroutines
// run count(*) queries, then compares the wall-clock write time against a
// baseline write with no readers.
func TestDuckDBConcurrentReadNoBlockWrite(t *testing.T) {
	ctx := context.Background()

	// Prepopulate a table in a fresh in-memory database.
	c, err := New(ctx, Config{DSN: ":memory:", MaxOpenConns: 4})
	require.NoError(t, err)
	defer func() { require.NoError(t, c.Close()) }()

	_, err = c.DB().ExecContext(ctx, "CREATE TABLE counters (id INTEGER, bucket INTEGER)")
	require.NoError(t, err)

	driverConn, err := c.connector.Connect(ctx)
	require.NoError(t, err)

	app, err := duckdb.NewAppender(driverConn, "", "main", "counters")
	require.NoError(t, err)

	const prepopulate = 100_000
	for i := 0; i < prepopulate; i++ {
		require.NoError(t, app.AppendRow(int32(i), int32(i%50)))
	}
	require.NoError(t, app.Flush())
	require.NoError(t, app.Close())
	require.NoError(t, driverConn.Close())

	// Baseline: write alone.
	const batches = 10
	const batchSize = 50_000
	writeAlone := time.Duration(0)
	{
		driverConn, err := c.connector.Connect(ctx)
		require.NoError(t, err)
		app, err := duckdb.NewAppender(driverConn, "", "main", "counters")
		require.NoError(t, err)
		start := time.Now()
		for b := 0; b < batches; b++ {
			for i := 0; i < batchSize; i++ {
				require.NoError(t, app.AppendRow(int32(b*batchSize+i), int32(i%50)))
			}
			require.NoError(t, app.Flush())
		}
		writeAlone = time.Since(start)
		require.NoError(t, app.Close())
		require.NoError(t, driverConn.Close())
	}

	// Concurrent: one writer plus 50 readers.
	writeWithReaders := time.Duration(0)
	readErrors := make(chan error, 50)
	{
		var wg sync.WaitGroup
		driverConn, err := c.connector.Connect(ctx)
		require.NoError(t, err)
		app, err := duckdb.NewAppender(driverConn, "", "main", "counters")
		require.NoError(t, err)

		// Start readers.
		readerCtx, cancelReaders := context.WithCancel(ctx)
		defer cancelReaders()
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for readerCtx.Err() == nil {
					var n int
					err := c.DB().QueryRowContext(readerCtx, "SELECT count(*) FROM counters").Scan(&n)
					if err != nil {
						if readerCtx.Err() != nil {
							return
						}
						readErrors <- fmt.Errorf("reader %d: %w", idx, err)
						return
					}
				}
			}(i)
		}

		start := time.Now()
		for b := 0; b < batches; b++ {
			for i := 0; i < batchSize; i++ {
				require.NoError(t, app.AppendRow(int32(b*batchSize+i), int32(i%50)))
			}
			require.NoError(t, app.Flush())
		}
		writeWithReaders = time.Since(start)

		cancelReaders()
		wg.Wait()
		require.NoError(t, app.Close())
		require.NoError(t, driverConn.Close())
	}
	close(readErrors)
	for e := range readErrors {
		t.Error(e)
	}

	factor := ReaderBlockFactor
	if v := os.Getenv("DUCKDB_READER_BLOCK_FACTOR"); v != "" {
		var parsed float64
		if _, err := fmt.Sscanf(v, "%f", &parsed); err == nil {
			factor = parsed
		}
	}

	t.Logf("write alone: %s; with 50 readers: %s; factor: %.2fx (threshold %.2fx)",
		writeAlone, writeWithReaders, float64(writeWithReaders)/float64(writeAlone), factor)

	assert.Less(t, float64(writeWithReaders), float64(writeAlone)*factor,
		"concurrent readers severely blocked the writer")
}
