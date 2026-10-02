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

//go:build !race

// This file intentionally excludes itself from `go test -race` runs. The
// Appender throughput floor is a performance-regression lock; the race
// detector's per-access instrumentation (shadow memory on every cgo
// boundary crossing) inflates Appender wall-clock by 5-10x and makes the
// rows/sec number load-sensitive noise — on a full-repo parallel run the
// measured throughput hovers right at the 100k floor (observed 106k solo,
// sub-100k under load), so the assertion flaps. The floor is enforced by
// the non-race suite (verify-phase-14.sh Stage 04); the race gate (Stage
// 02) keeps the correctness locks only. See T-14-20 in
// docs/threat-model/STRIDE-14.md.

package duckdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AppenderMinRowsPerSec is the SC-3 throughput floor. It can be overridden via
// DUCKDB_APPENDER_MIN_RPS for CI environments with slower I/O.
//
// Source-grep locked via TestDuckDBAppenderThroughput (verify-phase-14.sh
// Stage 08).
const AppenderMinRowsPerSec = 100_000

// TestDuckDBAppenderThroughput writes 500k rows to a wide table and asserts
// that the Appender path sustains at least AppenderMinRowsPerSec rows/sec.
// Non-race only — see the file-level build-tag comment.
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
