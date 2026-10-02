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

// Package duckdb adapter for KPI SQL validation.
//
// This file is ONE of the CGO boundary crossing points between internal/kpi
// (hermetic, CGO-free) and internal/duckdb (CGO, go-duckdb). The other
// crossing point is the executor adapter (16-06).
//
// DuckDBSQLSerializer implements kpi.SQLSerializer by calling
//   SELECT json_serialize_sql(?) → CAST to VARCHAR
// using the Connector's DB pool. The function belongs to the JSON module,
// which go-duckdb v2.4.3 statically links (16-01 probe confirmed).
// Hermetic boot (autoinstall/autoload=false) does NOT prevent this function
// from working — it is a built-in, not a loadable extension.
package duckdb

import (
	"context"
	"fmt"
	"strings"
)

// DuckDBSQLSerializer implements kpi.SQLSerializer using a live DuckDB connection.
// It is the production implementation; hermetic tests use fake fixtures instead.
type DuckDBSQLSerializer struct {
	conn *Connector
}

// NewDuckDBSQLSerializer creates a new serializer backed by the given Connector.
// The Connector must be initialized and non-nil.
func NewDuckDBSQLSerializer(conn *Connector) *DuckDBSQLSerializer {
	if conn == nil {
		panic("duckdb: NewDuckDBSQLSerializer called with nil Connector")
	}
	return &DuckDBSQLSerializer{conn: conn}
}

// SerializeSQL calls json_serialize_sql on the DuckDB engine and returns the
// raw JSON string.
//
// Binding shape: DuckDB's json_serialize_sql requires a VARCHAR literal as
// its first argument — it does NOT accept a prepared-statement parameter
// binding (returns "Invalid type Error: json_serialize_sql first argument
// must be a VARCHAR"). The 16-01 probe validated the literal form; the
// production code shipped a parameterized form, so the Layer 2 AST walk
// was dead code and cmd/server could not boot with kpis/loans.yaml.
// Fix: interpolate the statement as a single-quoted literal with strict
// `'` → `''` escape.
//
// Why this is safe in the validation-only path:
//   - The statement comes from a KPI YAML file authored by the data team
//     (no agent / user / network input at this stage).
//   - KPI SQL for v1 uses simple SELECT + named placeholders; embedded
//     single quotes are exceedingly rare in regulated reporting queries.
//   - The downstream ValidateSQLAST walker re-parses the returned AST and
//     would reject any syntactically malformed input regardless.
//   - The result is CAST to VARCHAR because json_serialize_sql returns a
//     native DuckDB STRUCT/JSON type that go-duckdb cannot Scan into a
//     string directly.
func (s *DuckDBSQLSerializer) SerializeSQL(ctx context.Context, statement string) (string, error) {
	escaped := strings.ReplaceAll(statement, "'", "''")
	var out string
	err := s.conn.DB().QueryRowContext(ctx,
		"SELECT CAST(json_serialize_sql('"+escaped+"') AS VARCHAR)").Scan(&out)
	if err != nil {
		return "", fmtError("SerializeSQL", fmt.Errorf("json_serialize_sql query failed: %w", err))
	}
	return out, nil
}
