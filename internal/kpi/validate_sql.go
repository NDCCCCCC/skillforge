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

package kpi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SQLSerializer is the narrow interface for json_serialize_sql — DuckDB's
// built-in SQL-to-JSON-AST parser. The production implementation lives in
// internal/duckdb/kpi_validate.go (the ONLY CGO crossing point for Layer 2).
// Hermetic tests inject a fake implementation that returns pre-recorded probe
// fixtures (CGO boundary discipline: internal/kpi MUST NOT import
// internal/duckdb or go-duckdb).
type SQLSerializer interface {
	// SerializeSQL calls json_serialize_sql(statement) and returns the raw
	// JSON string. The caller unmarshals the envelope and applies fail-closed
	// rules. Infrastructure errors (connection, timeout) are returned as Go
	// errors; SQL-level errors are returned inside the JSON envelope with
	// error:true (Pitfall 1 — the caller MUST check envelope.Error explicitly).
	SerializeSQL(ctx context.Context, statement string) (string, error)
}

// sqlEnvelope is the top-level JSON structure returned by json_serialize_sql.
// Pitfall 1: json_serialize_sql returns error:true in the JSON envelope
// rather than throwing a Go error. The caller MUST check Error explicitly or
// invalid SQL silently passes validation.
//
// Probe-confirmed structure (16-01 WSL probe, A1/A2/A3 locked):
//   - Valid SELECT: error=false, statements[].node.type="SELECT_NODE",
//     from_table has schema_name + table_name (A3: split form, not compound).
//   - Invalid SQL: error=true, error_type="parser" (A1).
//   - Non-SELECT (INSERT): error=true, error_type="not implemented" (A1: stronger
//     than assumed — it errors at the envelope level, not just non-SELECT_NODE).
//   - Cross-alias JOIN: from_table.type="JOIN" with left/right BASE_TABLE sub-objects
//     each containing schema_name + table_name (A3: walkable).
//   - Empty FROM: from_table.type="EMPTY" (zero table refs — validator must reject).
//   - Subquery: select_list[].class="SUBQUERY" with nested subquery.node.from_table
//     containing schema_name (A3: walkable via recursion).
type sqlEnvelope struct {
	Error       bool    `json:"error"`
	ErrorType   string  `json:"error_type"`
	ErrorMsg    string  `json:"error_message"`
	Statements  []sqlStmt `json:"statements"`
}

type sqlStmt struct {
	Node json.RawMessage `json:"node"` // preserved as RawMessage for lazy parsing
}

// sqlNode is the minimal recursive model for AST walk. The type field
// determines the node kind; from_table, select_list, and other children
// are walked recursively to collect all BASE_TABLE references.
//
// Fields are populated based on probe-confirmed AST structure (16-01).
// Additional fields are ignored via json:",omitempty" or silently skipped.
type sqlNode struct {
	Type      string          `json:"type"`       // "SELECT_NODE" or others
	FromTable json.RawMessage `json:"from_table"` // BASE_TABLE, JOIN, EMPTY, or SUBQUERY
	SelectList []json.RawMessage `json:"select_list"` // may contain SUBQUERY entries
}

// sqlBaseTable represents a BASE_TABLE reference inside the AST.
// Probe-confirmed: table references use schema_name + table_name (split form),
// not a single compound "ds_loans.data" identifier.
type sqlBaseTable struct {
	Type       string `json:"type"`        // "BASE_TABLE"
	SchemaName string `json:"schema_name"` // e.g. "ds_loans"
	TableName  string `json:"table_name"`  // e.g. "data"
}

// sqlJoinTable represents a JOIN node in the AST.
// Probe-confirmed: has left and right sub-objects, each a BASE_TABLE (or nested JOIN).
type sqlJoinTable struct {
	Type  string          `json:"type"` // "JOIN"
	Left  json.RawMessage `json:"left"`
	Right json.RawMessage `json:"right"`
}

// sqlSubqueryInSelect represents a SUBQUERY entry in the select_list.
// Probe-confirmed: class="SUBQUERY" with nested subquery.node containing from_table.
type sqlSubqueryInSelect struct {
	Class    string          `json:"class"` // "SUBQUERY"
	Subquery json.RawMessage `json:"subquery"`
}

// sqlSubqueryNode wraps the subquery's inner node.
type sqlSubqueryNode struct {
	Node json.RawMessage `json:"node"`
}

// ValidateSQLAST performs Layer 2 SQL verification using the DuckDB AST.
// It calls ser.SerializeSQL to get the JSON AST, then applies four fail-closed
// rules:
//
//	Rule 1: envelope.Error == true → reject (Pitfall 1: syntax/semantic errors
//	        are returned in the envelope, not as Go errors).
//	Rule 2: len(envelope.Statements) != 1 → reject (only single statements allowed).
//	Rule 3: statements[0].node.type != "SELECT_NODE" → reject (SELECT-only;
//	        INSERT/DELETE are caught by both rule 1 and/or rule 3 — dual safety,
//	        A1 lock).
//	Rule 4: Recursive walk collecting all BASE_TABLE references; zero references
//	        → reject (no dataset binding); any reference not matching
//	        ds_{declaredDataset}.data → reject (unregistered dataset or cross-dataset
//	        join, D14-D1 application-layer enforcement).
//
// Returns the list of valid table references on success (caller records TableRefCount).
func ValidateSQLAST(ctx context.Context, ser SQLSerializer, statement, declaredDataset string) ([]string, error) {
	// Call the serializer
	raw, err := ser.SerializeSQL(ctx, statement)
	if err != nil {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf("SerializeSQL infra error: %w", err))
	}

	// Unmarshal envelope
	var envelope sqlEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf("unmarshal json_serialize_sql output: %w", err))
	}

	// Rule 1: envelope.Error == true → reject (Pitfall 1)
	if envelope.Error {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf(
			"SQL parse error (type=%s): %s (Pitfall 1: json_serialize_sql returns error:true, not a Go error)",
			envelope.ErrorType, envelope.ErrorMsg))
	}

	// Rule 2: exactly one statement
	if len(envelope.Statements) != 1 {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf(
			"expected exactly 1 statement, got %d (D16-A2: single SELECT only)", len(envelope.Statements)))
	}

	// Parse the statement node
	var node sqlNode
	if err := json.Unmarshal(envelope.Statements[0].Node, &node); err != nil {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf("unmarshal statement node: %w", err))
	}

	// Rule 3: must be SELECT_NODE
	if node.Type != "SELECT_NODE" {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf(
			"statement type is %q, not SELECT_NODE (D16-A2: only SELECT statements are allowed)", node.Type))
	}

	// Rule 4: collect all BASE_TABLE references and validate
	expectedSchema := "ds_" + declaredDataset
	var refs []string
	collectBaseTables(node.FromTable, &refs)
	// Also check select_list for subqueries
	for _, item := range node.SelectList {
		collectSubqueryTables(item, &refs)
	}

	if len(refs) == 0 {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf(
			"SQL has zero table references (e.g. SELECT 42) — every KPI must bind to a dataset (D16-A2)"))
	}

	// Validate each reference matches ds_{declaredDataset}.data
	var illegal []string
	for _, ref := range refs {
		// ref format is "schema_name.table_name" (split form from AST)
		parts := strings.SplitN(ref, ".", 2)
		if len(parts) != 2 || parts[0] != expectedSchema || parts[1] != "data" {
			illegal = append(illegal, ref)
		}
	}
	if len(illegal) > 0 {
		return nil, fmtError("ValidateSQLAST", fmt.Errorf(
			"SQL references table(s) %v not matching declared dataset %q (must be %s.data only — unregistered dataset or cross-dataset join forbidden, D14-D1)",
			illegal, declaredDataset, expectedSchema))
	}

	return refs, nil
}

// collectBaseTables recursively walks a from_table JSON node, collecting
// all BASE_TABLE references as "schema_name.table_name" strings.
func collectBaseTables(raw json.RawMessage, refs *[]string) {
	if raw == nil || string(raw) == "null" {
		return
	}

	// Try BASE_TABLE first
	var bt sqlBaseTable
	if err := json.Unmarshal(raw, &bt); err == nil && bt.Type == "BASE_TABLE" {
		if bt.SchemaName != "" && bt.TableName != "" {
			*refs = append(*refs, bt.SchemaName+"."+bt.TableName)
		}
		return
	}

	// Try JOIN
	var jt sqlJoinTable
	if err := json.Unmarshal(raw, &jt); err == nil && jt.Type == "JOIN" {
		collectBaseTables(jt.Left, refs)
		collectBaseTables(jt.Right, refs)
		return
	}

	// EMPTY or other types: nothing to collect
	// (EMPTY has type:"EMPTY" — no table references)
}

// collectSubqueryTables checks a select_list item for subqueries and
// collects their BASE_TABLE references.
func collectSubqueryTables(raw json.RawMessage, refs *[]string) {
	if raw == nil || string(raw) == "null" {
		return
	}

	var sq sqlSubqueryInSelect
	if err := json.Unmarshal(raw, &sq); err != nil || sq.Class != "SUBQUERY" {
		return
	}

	// Unwrap subquery.node
	var sqNode sqlSubqueryNode
	if err := json.Unmarshal(sq.Subquery, &sqNode); err != nil {
		return
	}

	var innerNode sqlNode
	if err := json.Unmarshal(sqNode.Node, &innerNode); err != nil {
		return
	}

	// Collect from the inner SELECT's from_table
	collectBaseTables(innerNode.FromTable, refs)
	// Recurse into inner select_list for nested subqueries
	for _, item := range innerNode.SelectList {
		collectSubqueryTables(item, refs)
	}
}
