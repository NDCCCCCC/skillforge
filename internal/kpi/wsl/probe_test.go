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

// Package wsl contains WSL-gated tests that require CGO (go-duckdb).
// These tests are NOT compiled by `go test ./internal/kpi/` on Windows
// because this directory contains only _test.go files with package wsl.
// Run via: wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/kpi/wsl/ -v'
package wsl

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/marcboeker/go-duckdb/v2"
)

// sqlEnvelope mirrors the json_serialize_sql top-level JSON structure.
// Pitfall 1: json_serialize_sql returns error:true in the JSON envelope
// rather than throwing a Go error — the caller MUST check envelope.Error
// explicitly or invalid SQL silently passes validation.
type sqlEnvelope struct {
	Error        bool      `json:"error"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Statements   []sqlStmt `json:"statements"`
}

type sqlStmt struct {
	Node sqlNode `json:"node"`
}

type sqlNode struct {
	Type     string `json:"type"`      // e.g. "SELECT_NODE"
	FromList []any  `json:"from_list"` // join tree / table references
}

// serializeSQL calls json_serialize_sql with the given SQL string.
// It uses string interpolation because go-duckdb's parameterized query
// does not preserve the VARCHAR type hint that json_serialize_sql requires
// (first argument must be VARCHAR, not a prepared-statement parameter).
// The result is CAST to VARCHAR because json_serialize_sql returns a native
// DuckDB STRUCT/JSON type that go-duckdb cannot Scan into a string directly.
// The sql argument is a literal SQL string for testing — never user input.
func serializeSQL(t *testing.T, db *sql.DB, sqlStr string) string {
	t.Helper()
	// Use string concatenation to build the query since json_serialize_sql
	// requires a VARCHAR argument and prepared statement parameters don't
	// satisfy DuckDB's type inference. This is safe because sqlStr is a
	// test-controlled literal, not user input.
	escaped := strings.ReplaceAll(sqlStr, "'", "''")
	query := "SELECT CAST(json_serialize_sql('" + escaped + "') AS VARCHAR)"
	var ast string
	if err := db.QueryRow(query).Scan(&ast); err != nil {
		t.Fatalf("json_serialize_sql query failed for %q: %v", sqlStr, err)
	}
	return ast
}

// TestProbeJSONSerializeSQL locks assumptions A1/A2/A3 about json_serialize_sql
// behavior by testing 6 representative SQL strings and asserting on their AST
// output. The raw JSON envelopes are written to testdata/probe-N.json as
// fixtures for 16-05 hermetic AST walk tests.
//
// Assertions locked:
//   - A1: Non-SELECT statements either have error==true or node.type != "SELECT_NODE"
//   - A2: json_serialize_sql is available in go-duckdb v2.4.3 (statically linked JSON extension)
//   - A3: Cross-alias JOIN table references appear as substrings in the serialized JSON
func TestProbeJSONSerializeSQL(t *testing.T) {
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		t.Fatalf("open duckdb :memory: %v", err)
	}
	defer db.Close()

	probeCases := []struct {
		n    int    // probe number (1-based, used for fixture filename)
		sql  string // SQL string to serialize
		desc string // human description
	}{
		{
			n:    1,
			sql:  `SELECT status AS status, count(*) AS row_count FROM ds_loans.data GROUP BY status`,
			desc: "valid SELECT with ds_loans.data alias",
		},
		{
			n:    2,
			sql:  `TOTALLY NOT VALID SQL`,
			desc: "invalid SQL (syntax error)",
		},
		{
			n:    3,
			sql:  `INSERT INTO ds_loans.data VALUES (1)`,
			desc: "INSERT statement (non-SELECT DML)",
		},
		{
			n:    4,
			sql:  `SELECT count(*) FROM ds_loans.data JOIN ds_cards.data ON true`,
			desc: "cross-alias JOIN (ds_loans + ds_cards)",
		},
		{
			n:    5,
			sql:  `SELECT 42`,
			desc: "empty FROM (no table reference)",
		},
		{
			n:    6,
			sql:  `SELECT (SELECT count(*) FROM ds_loans.data) AS c`,
			desc: "subquery in select list",
		},
	}

	// Ensure testdata directory exists
	testdataDir := filepath.Join(".", "testdata")
	if err := os.MkdirAll(testdataDir, 0o755); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}

	for _, tc := range probeCases {
		t.Run(fmt.Sprintf("probe_%d", tc.n), func(t *testing.T) {
			ast := serializeSQL(t, db, tc.sql)
			t.Logf("SQL: %s", tc.sql)
			t.Logf("AST: %s", ast)

			// Write fixture file
			fixturePath := filepath.Join(testdataDir, fmt.Sprintf("probe-%d.json", tc.n))
			if err := os.WriteFile(fixturePath, []byte(ast), 0o644); err != nil {
				t.Errorf("write fixture %s: %v", fixturePath, err)
			}

			// Parse envelope for assertions
			var env sqlEnvelope
			if err := json.Unmarshal([]byte(ast), &env); err != nil {
				t.Fatalf("unmarshal envelope: %v", err)
			}

			switch tc.n {
			case 1:
				// Valid SELECT: must contain dataset alias "ds_loans" and table name "data"
				if !strings.Contains(ast, `"ds_loans"`) {
					t.Errorf("probe 1: AST does not contain ds_loans — actual table reference form may differ; full JSON logged above")
				}
				if !strings.Contains(ast, `"data"`) {
					t.Errorf("probe 1: AST does not contain 'data' — actual table reference form may differ; full JSON logged above")
				}
				if env.Error {
					t.Errorf("probe 1: valid SELECT returned error=true: %s", env.ErrorMessage)
				}

			case 2:
				// Invalid SQL: must return error==true in envelope (Pitfall 1)
				if !env.Error {
					t.Errorf("probe 2: invalid SQL returned error=false — json_serialize_sql must flag syntax errors in envelope")
				}

			case 3:
				// INSERT: must NOT appear as a valid SELECT_NODE.
				// A1 dual-path assertion: either error==true OR node.type != "SELECT_NODE"
				// (both paths lead to rejection in validate_sql.go)
				isSelectNode := false
				if !env.Error && len(env.Statements) > 0 {
					isSelectNode = env.Statements[0].Node.Type == "SELECT_NODE"
				}
				if !env.Error && isSelectNode {
					t.Errorf("probe 3: INSERT returned error=false AND node.type=SELECT_NODE — must fail at least one path")
				}

			case 4:
				// Cross-alias JOIN: serialized JSON must contain both "ds_loans" and "ds_cards"
				// as substrings (A3: join tree table references are walkable)
				if !strings.Contains(ast, `"ds_loans"`) {
					t.Errorf("probe 4: AST does not contain ds_loans — join tree walkability unconfirmed")
				}
				if !strings.Contains(ast, `"ds_cards"`) {
					t.Errorf("probe 4: AST does not contain ds_cards — join tree walkability unconfirmed")
				}

			case 5:
				// Empty FROM: must be a valid SELECT_NODE with zero table references
				if env.Error {
					t.Errorf("probe 5: SELECT 42 returned error=true — empty FROM should be valid AST: %s", env.ErrorMessage)
				}
				if len(env.Statements) == 0 {
					t.Fatalf("probe 5: no statements in envelope")
				}
				if env.Statements[0].Node.Type != "SELECT_NODE" {
					t.Errorf("probe 5: node.type=%q, expected SELECT_NODE", env.Statements[0].Node.Type)
				}
				// Empty FROM means no table references — the validator must
				// independently reject "zero table references" (not the parser)

			case 6:
				// Subquery in select list: just log the full AST for 16-05
				// to understand subquery table reference structure
				t.Logf("probe 6 subquery AST logged for 16-05 inspection")
			}
		})
	}
}
