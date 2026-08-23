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

package data

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestDumpParsesBasicFixture(t *testing.T) {
	r, err := os.Open("testdata/dump_basic.sql")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer r.Close()

	res, err := ParseDump(r)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if res.Table != "t" {
		t.Errorf("table = %q, want t", res.Table)
	}
	wantCols := []DumpColumn{
		{Name: "a", DeclaredType: "INTEGER"},
		{Name: "name", DeclaredType: "TEXT"},
		{Name: "bal", DeclaredType: "REAL"},
		{Name: "data", DeclaredType: "BLOB"},
		{Name: "note", DeclaredType: "TEXT"},
	}
	if len(res.Columns) != len(wantCols) {
		t.Fatalf("columns = %d, want %d: %+v", len(res.Columns), len(wantCols), res.Columns)
	}
	for i, wc := range wantCols {
		if res.Columns[i] != wc {
			t.Errorf("column[%d] = %+v, want %+v", i, res.Columns[i], wc)
		}
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	row := res.Rows[0]
	want := []string{"1", "张三'quote", "1234.56", "616263", DumpNullValue}
	if len(row) != len(want) {
		t.Fatalf("row len = %d, want %d", len(row), len(want))
	}
	for i, w := range want {
		if row[i] != w {
			t.Errorf("row[%d] = %q, want %q", i, row[i], w)
		}
	}
}

func TestDumpAutoincrementBoilerplateSkipped(t *testing.T) {
	r, err := os.Open("testdata/dump_autoincrement.sql")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer r.Close()

	res, err := ParseDump(r)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if res.Table != "u" {
		t.Errorf("table = %q, want u", res.Table)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	if res.Rows[0][1] != "alice" {
		t.Errorf("name = %q, want alice", res.Rows[0][1])
	}
	for _, row := range res.Rows {
		if len(row) > 0 && row[0] == "sqlite_sequence" {
			t.Errorf("sqlite_sequence row leaked into result: %v", row)
		}
	}
}

func TestDumpNonInsertStatementRejected(t *testing.T) {
	base := "PRAGMA foreign_keys=OFF; BEGIN TRANSACTION; CREATE TABLE t(a INTEGER PRIMARY KEY);"
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"DROP TABLE", base + " DROP TABLE t; COMMIT;", "DROP TABLE"},
		{"UPDATE", base + " UPDATE t SET a=1; COMMIT;", "UPDATE"},
		{"PRAGMA user_version", base + " PRAGMA user_version=2; COMMIT;", "PRAGMA user_version"},
		{"ATTACH", base + " ATTACH 'x' AS y; COMMIT;", "ATTACH"},
		{"CREATE TRIGGER", base + " CREATE TRIGGER trg AFTER INSERT ON t BEGIN SELECT 1; END; COMMIT;", "CREATE TRIGGER"},
		{"CREATE VIEW", base + " CREATE VIEW v AS SELECT a FROM t; COMMIT;", "CREATE VIEW"},
		{"DELETE FROM", base + " DELETE FROM t; COMMIT;", "DELETE FROM"},
		{"INSERT other table", base + " INSERT INTO other VALUES(1); COMMIT;", "other"},
		{"second CREATE TABLE", base + " CREATE TABLE s(b INTEGER); COMMIT;", "multi-table"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDump(strings.NewReader(tc.payload))
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestDumpControlCaseRealDumpPasses(t *testing.T) {
	r, err := os.Open("testdata/dump_basic.sql")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer r.Close()
	// Full fixture includes boilerplate that a naive "non CREATE/INSERT -> reject"
	// parser would block. This test is the Pitfall 2 control case.
	if _, err := ParseDump(r); err != nil {
		t.Fatalf("full real dump should parse: %v", err)
	}
}

func TestDumpUnistrRejected(t *testing.T) {
	r, err := os.Open("testdata/dump_unistr.sql")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer r.Close()

	_, err = ParseDump(r)
	if err == nil {
		t.Fatalf("expected error for unistr literal")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "unistr") {
		t.Errorf("error does not mention unistr: %q", msg)
	}
	if !strings.Contains(msg, "导出端清理") || !strings.Contains(msg, "控制字符") {
		t.Errorf("error does not guide export cleanup: %q", msg)
	}
}

func TestDumpBlobCaseInsensitive(t *testing.T) {
	payload := "PRAGMA foreign_keys=OFF; BEGIN TRANSACTION; CREATE TABLE t(a INTEGER, data BLOB); INSERT INTO t VALUES(1,X'6162'); COMMIT;"
	res, err := ParseDump(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][1] != "6162" {
		t.Errorf("got %+v, want hex 6162", res.Rows)
	}
}

// infiniteReader yields an endless stream of ASCII letters so the scanner
// keeps looking for the next statement terminator and crosses the limit.
type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}

func TestDumpOversizeFailClosed(t *testing.T) {
	// Seed a valid first statement, then an unterminated stream of garbage.
	r := io.MultiReader(strings.NewReader("PRAGMA foreign_keys=OFF;"), infiniteReader{})
	_, err := ParseDump(r)
	if err == nil {
		t.Fatalf("expected oversize error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "64 MiB") {
		t.Errorf("error does not mention 64 MiB limit: %q", msg)
	}
	if !strings.Contains(msg, "CSV/push") {
		t.Errorf("error does not suggest CSV/push channel: %q", msg)
	}
}

func TestDumpMaliciousFixtureRejected(t *testing.T) {
	r, err := os.Open("testdata/dump_malicious.sql")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer r.Close()

	_, err = ParseDump(r)
	if err == nil {
		t.Fatalf("expected malicious fixture to be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "DROP TABLE") && !strings.Contains(msg, "ATTACH") {
		t.Errorf("error does not reference rejected statement: %q", msg)
	}
}
