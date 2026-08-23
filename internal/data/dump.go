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

// Dump channel: accept-list parser for sqlite3 .dump output.
//
// This file intentionally imports ZERO database drivers and executes ZERO SQL.
// The dump bytes are the only input; parsing is the only consumption surface
// (D15-B3 / T-15-01). Anything outside an explicit allow-list is rejected
// fail-closed, including DROP/UPDATE/DELETE/ATTACH/CREATE TRIGGER/ALTER/
// PRAGMAs other than the three boilerplate forms.
package data

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// DumpMaxInputBytes is the v1 dump-channel input envelope.
//
// Dump materializes the entire file as [][]string in memory, so it keeps a
// much smaller limit than the 2 GB CSV/push upload envelope enforced by the
// HTTP layer (15-06). Exceeding this limit is rejected by byte counting before
// materialization completes — the fail-closed channel guard for dump.
//
// The literal is source-grep locked (scripts/verify-phase-15.sh Stage 9).
const DumpMaxInputBytes = 64 << 20

// DumpNullValue is the in-band sentinel used for SQL NULL in DumpResult.Rows.
// It is intentionally not a printable string so it cannot collide with a real
// cell value; the consumer (importer.go) must map it to the target column's
// NULL representation before persistence.
const DumpNullValue = "\x00NULL\x00"

// DumpColumn is one column parsed from CREATE TABLE.
type DumpColumn struct {
	Name         string
	DeclaredType string
}

// DumpResult is the parsed content of a single-table sqlite .dump.
type DumpResult struct {
	Table   string
	Columns []DumpColumn
	Rows    [][]string
}

// ParseDump parses a sqlite3 .dump stream with a three-layer accept-list:
//
//   1. Skip boilerplate: PRAGMA foreign_keys=OFF, PRAGMA writable_schema=ON/OFF,
//      BEGIN TRANSACTION, COMMIT, CREATE INDEX, and the sqlite_sequence trio.
//   2. Accept exactly one CREATE TABLE (v1 single-table channel).
//   3. Accept INSERT INTO that same table; reject INSERT INTO other tables.
//   4. Reject everything else fail-closed, quoting the offending statement.
//
// It never executes the SQL it parses.
func ParseDump(r io.Reader) (*DumpResult, error) {
	s := newDumpScanner(r)
	res := &DumpResult{}
	seenCreate := false
	for {
		stmt, err := s.nextStatement()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmtError("ParseDump", err)
		}
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		lowerNoSemi := strings.TrimSuffix(lower, ";")

		// Layer 1: boilerplate whitelist.
		switch {
		case isExactPragma(lower, "foreign_keys", "off"):
			continue
		case isExactPragma(lower, "writable_schema", "on"):
			continue
		case isExactPragma(lower, "writable_schema", "off"):
			continue
		case lowerNoSemi == "begin transaction" || lowerNoSemi == "commit":
			continue
		case strings.HasPrefix(lower, "create index") || strings.HasPrefix(lower, "create unique index"):
			continue
		case isSQLiteSequenceStatement(lower):
			continue
		}

		// Layer 2: CREATE TABLE.
		if strings.HasPrefix(lower, "create table") {
			if seenCreate {
				return nil, fmtError("ParseDump", fmt.Errorf(
					"multi-table dumps unsupported — dump the single table (v1); rejected statement: %s", snippet(stmt)))
			}
			name, cols, err := parseCreateTable(stmt)
			if err != nil {
				return nil, fmtError("ParseDump", err)
			}
			res.Table = name
			res.Columns = cols
			seenCreate = true
			continue
		}

		// Layer 3: INSERT INTO.
		if strings.HasPrefix(lower, "insert into") {
			if !seenCreate {
				return nil, fmtError("ParseDump", fmt.Errorf(
					"INSERT before CREATE TABLE — malformed dump; rejected statement: %s", snippet(stmt)))
			}
			if err := parseInsertInto(res, stmt); err != nil {
				return nil, fmtError("ParseDump", err)
			}
			continue
		}

		// Layer 4: default-deny.
		return nil, fmtError("ParseDump", fmt.Errorf(
			"unauthorized statement in dump — only CREATE TABLE and INSERT INTO %q are allowed; rejected statement: %s", res.Table, snippet(stmt)))
	}
	return res, nil
}

// dumpScanner reads statement-by-statement, respecting single-quoted strings
// and x'/X' blob literals. Statements are terminated by a top-level semicolon.
type dumpScanner struct {
	br       *bufio.Reader
	consumed int64
}

func newDumpScanner(r io.Reader) *dumpScanner {
	return &dumpScanner{br: bufio.NewReader(r)}
}

func (s *dumpScanner) nextStatement() (string, error) {
	var sb strings.Builder
	inString := false
	inBlob := false
	pendingBlob := false

	for {
		if s.consumed > DumpMaxInputBytes {
			return "", fmt.Errorf("dump input exceeds 64 MiB v1 limit — export a smaller single-table dump or use the CSV/push channel")
		}
		b, err := s.br.ReadByte()
		if err != nil {
			if err == io.EOF {
				if strings.TrimSpace(sb.String()) == "" {
					return "", io.EOF
				}
				return "", fmt.Errorf("unexpected EOF inside statement: %s", snippet(sb.String()))
			}
			return "", err
		}
		s.consumed++
		sb.WriteByte(b)

		if inString {
			if b == '\'' {
				next, perr := s.br.Peek(1)
				if perr == nil && next[0] == '\'' {
					// Escaped quote: consume the second one too.
					b2, _ := s.br.ReadByte()
					sb.WriteByte(b2)
					s.consumed++
				} else {
					inString = false
				}
			}
			continue
		}

		if inBlob {
			if b == '\'' {
				inBlob = false
			}
			continue
		}

		// Normal state.
		if b == '\'' {
			inString = true
			pendingBlob = false
			continue
		}

		if pendingBlob && b == '\'' {
			inBlob = true
			pendingBlob = false
			continue
		}
		pendingBlob = false
		if b == 'x' || b == 'X' {
			pendingBlob = true
			continue
		}

		if b == ';' {
			return sb.String(), nil
		}
	}
}

// isExactPragma matches the three whitelisted PRAGMA forms with optional spaces
// around '='. Only lower-case names/values are passed in via the caller's
// ToLower result, and the fixture forms are exactly those spellings.
func isExactPragma(lowerStmt, name, value string) bool {
	lowerStmt = strings.TrimSuffix(lowerStmt, ";")
	prefix := "pragma " + name
	if !strings.HasPrefix(lowerStmt, prefix) {
		return false
	}
	rest := strings.TrimSpace(lowerStmt[len(prefix):])
	rest = strings.TrimPrefix(rest, "=")
	rest = strings.TrimSpace(rest)
	return rest == value
}

func isSQLiteSequenceStatement(lower string) bool {
	if strings.HasPrefix(lower, "create table if not exists sqlite_sequence") {
		return true
	}
	if strings.HasPrefix(lower, "create table sqlite_sequence") {
		return true
	}
	if strings.HasPrefix(lower, "delete from sqlite_sequence") {
		return true
	}
	if strings.HasPrefix(lower, "insert into sqlite_sequence") {
		return true
	}
	return false
}

func parseCreateTable(stmt string) (string, []DumpColumn, error) {
	lower := strings.ToLower(stmt)
	idx := strings.Index(lower, "create table")
	rest := stmt[idx+len("create table"):]
	lowerRest := lower[idx+len("create table"):]
	// Skip optional IF NOT EXISTS (not used for main tables in v1 fixtures, but
	// robustness costs nothing).
	lowerRest = strings.TrimSpace(lowerRest)
	if strings.HasPrefix(lowerRest, "if not exists") {
		rest = rest[strings.Index(strings.ToLower(rest), "if not exists")+len("if not exists"):]
		lowerRest = strings.ToLower(rest)
	}
	lowerRest = strings.TrimSpace(lowerRest)
	rest = strings.TrimSpace(rest)

	paren := strings.Index(rest, "(")
	if paren == -1 {
		return "", nil, fmt.Errorf("malformed CREATE TABLE (no opening paren): %s", snippet(stmt))
	}
	name := strings.TrimSpace(rest[:paren])
	name = strings.Trim(name, "\"`")
	body := rest[paren+1:]
	body = strings.TrimSuffix(strings.TrimSpace(body), ";")
	body = strings.TrimSuffix(body, ")")

	cols, err := splitTopLevel(body)
	if err != nil {
		return "", nil, fmt.Errorf("malformed CREATE TABLE column list: %w", err)
	}

	var out []DumpColumn
	for _, col := range cols {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		lowerCol := strings.ToLower(col)
		if strings.HasPrefix(lowerCol, "primary key(") ||
			strings.HasPrefix(lowerCol, "unique(") ||
			strings.HasPrefix(lowerCol, "check(") ||
			strings.HasPrefix(lowerCol, "foreign key(") {
			continue
		}
		fields := strings.Fields(col)
		if len(fields) < 2 {
			return "", nil, fmt.Errorf("malformed column definition %q", col)
		}
		out = append(out, DumpColumn{Name: fields[0], DeclaredType: fields[1]})
	}
	if len(out) == 0 {
		return "", nil, fmt.Errorf("CREATE TABLE has no usable columns: %s", snippet(stmt))
	}
	return name, out, nil
}

func parseInsertInto(res *DumpResult, stmt string) error {
	lower := strings.ToLower(stmt)
	idx := strings.Index(lower, "insert into")
	rest := stmt[idx+len("insert into"):]
	lowerRest := lower[idx+len("insert into"):]
	lowerRest = strings.TrimSpace(lowerRest)
	rest = strings.TrimSpace(rest)

	nameEnd := strings.IndexAny(rest, " \t(")
	if nameEnd == -1 {
		return fmt.Errorf("malformed INSERT INTO: %s", snippet(stmt))
	}
	tableName := strings.TrimSpace(rest[:nameEnd])
	tableName = strings.Trim(tableName, "\"`")
	if !tableNameEqual(tableName, res.Table) {
		return fmt.Errorf("INSERT INTO %q references a different table (expected %q); rejected statement: %s", tableName, res.Table, snippet(stmt))
	}

	lowerRest = lowerRest[nameEnd:]
	rest = rest[nameEnd:]
	valIdx := strings.Index(strings.ToLower(rest), "values")
	if valIdx == -1 {
		return fmt.Errorf("INSERT INTO without VALUES: %s", snippet(stmt))
	}
	rest = rest[valIdx+len("values"):]
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "(") {
		return fmt.Errorf("INSERT INTO VALUES not followed by '(' : %s", snippet(stmt))
	}
	// Multiple row groups are supported: VALUES(...),(...). Keep the outer
	// '(' so indexMatchingParen can find the matching ')' while respecting
	// nested function calls such as unistr('...').
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" || rest == ";" {
			break
		}
		if !strings.HasPrefix(rest, "(") {
			return fmt.Errorf("expected '(' before row in VALUES: %s", snippet(stmt))
		}
		closeParen := indexMatchingParen(rest)
		if closeParen == -1 {
			return fmt.Errorf("unbalanced parentheses in VALUES: %s", snippet(stmt))
		}
		rowText := rest[1:closeParen]
		row, err := parseValuesRow(rowText)
		if err != nil {
			return err
		}
		res.Rows = append(res.Rows, row)
		rest = rest[closeParen+1:]
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, ",") {
			rest = rest[1:]
			continue
		}
		if rest == "" || rest == ";" {
			break
		}
		return fmt.Errorf("unexpected token after row in VALUES: %s", snippet(stmt))
	}
	return nil
}

func parseValuesRow(rowText string) ([]string, error) {
	parts, err := splitTopLevel(rowText)
	if err != nil {
		return nil, err
	}
	row := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		if lower == "null" {
			row = append(row, DumpNullValue)
			continue
		}
		if strings.HasPrefix(lower, "unistr(") {
			return nil, fmt.Errorf("dump contains unistr() literals (control characters in TEXT) — clean the export and retry (v1 limitation: 导出端清理控制字符后重试); rejected value: %s", p)
		}
		if strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'") {
			row = append(row, unquoteSQLString(p))
			continue
		}
		if strings.HasPrefix(lower, "x'") && strings.HasSuffix(p, "'") {
			row = append(row, p[2:len(p)-1])
			continue
		}
		// Numeric or default expression.
		row = append(row, p)
	}
	return row, nil
}

func unquoteSQLString(s string) string {
	s = s[1 : len(s)-1]
	return strings.ReplaceAll(s, "''", "'")
}

// splitTopLevel splits a comma-separated list at top level, ignoring commas
// inside single-quoted strings or parenthesized sub-expressions.
func splitTopLevel(s string) ([]string, error) {
	var out []string
	var buf bytes.Buffer
	depth := 0
	inString := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if inString {
			buf.WriteByte(b)
			if b == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					// Escaped quote: keep the next one too.
					buf.WriteByte(s[i+1])
					i++
				} else {
					inString = false
				}
			}
			continue
		}
		switch b {
		case '\'':
			inString = true
			buf.WriteByte(b)
		case '(':
			depth++
			buf.WriteByte(b)
		case ')':
			depth--
			buf.WriteByte(b)
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced ')' in %q", s)
			}
		case ',':
			if depth == 0 {
				out = append(out, buf.String())
				buf.Reset()
			} else {
				buf.WriteByte(b)
			}
		default:
			buf.WriteByte(b)
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string in %q", s)
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses in %q", s)
	}
	out = append(out, buf.String())
	return out, nil
}

// indexMatchingParen finds the ')' matching the opening '(' at the start of s.
func indexMatchingParen(s string) int {
	depth := 0
	inString := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if inString {
			if b == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
				} else {
					inString = false
				}
			}
			continue
		}
		if b == '\'' {
			inString = true
			continue
		}
		if b == '(' {
			depth++
			continue
		}
		if b == ')' {
			if depth == 1 {
				return i
			}
			depth--
		}
	}
	return -1
}

func snippet(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// tableNameEqual compares SQL identifiers case-insensitively and ignoring
// surrounding quotes. SQLite identifiers are case-insensitive unless quoted
// in a double-quote/backtick form; v1 accepts the common unquoted form.
func tableNameEqual(a, b string) bool {
	a = strings.Trim(a, "\"`")
	b = strings.Trim(b, "\"`")
	return strings.EqualFold(a, b)
}
